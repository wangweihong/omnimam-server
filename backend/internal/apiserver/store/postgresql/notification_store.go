package postgresql

import (
	"context"
	stderrors "errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type notificationStore struct{ ds *datastore }

func newNotificationStore(ds *datastore) *notificationStore { return &notificationStore{ds: ds} }

func (s *notificationStore) ListNotifications(ctx context.Context, req *iapiserver.NotificationListRequest) ([]*iapiserver.Notification, int64, error) {
	query := s.ds.db.WithContext(ctx).Model(&iapiserver.Notification{}).
		Where("recipient_user_id = ? AND deleted_at IS NULL", req.RecipientUserID)
	if req.Keyword != "" {
		conditions, args := make([]string, 0, len(req.SearchFields)), make([]any, 0, len(req.SearchFields))
		for _, field := range req.SearchFields {
			column := map[string]string{"title": "name", "content": "description"}[field]
			conditions, args = append(conditions, column+" ILIKE ?"), append(args, "%"+req.Keyword+"%")
		}
		query = query.Where("("+strings.Join(conditions, " OR ")+")", args...)
	}
	query = applyNotificationCSVFilter(query, "inbox_status", req.InboxStatuses)
	query = applyNotificationCSVFilter(query, "category", req.Categories)
	query = applyNotificationCSVFilter(query, "severity", req.Severities)
	query = applyNotificationCSVFilter(query, "attention_status", req.AttentionStatuses)
	query = applyNotificationCSVFilter(query, "notification_topic", req.NotificationTopics)
	if req.CreatedAfterRFC3339 != "" {
		parsed, _ := time.Parse(time.RFC3339, req.CreatedAfterRFC3339)
		query = query.Where("created_at >= ?", parsed)
	}
	if req.CreatedBeforeRFC3339 != "" {
		parsed, _ := time.Parse(time.RFC3339, req.CreatedBeforeRFC3339)
		query = query.Where("created_at <= ?", parsed)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}
	window, err := req.PagingParams.Normalize()
	if err != nil {
		return nil, 0, err
	}
	orderColumn := map[string]string{"created_at": "created_at", "last_occurred_at": "last_occurred_at", "severity": "severity"}[req.SortField]
	if req.SortField == "severity" {
		orderColumn = "CASE severity WHEN 'info' THEN 0 WHEN 'success' THEN 1 WHEN 'warning' THEN 2 WHEN 'error' THEN 3 WHEN 'critical' THEN 4 END"
	}
	var items []*iapiserver.Notification
	if err := query.Order(orderColumn + " " + strings.ToUpper(req.SortOrder)).Offset(window.Offset).Limit(window.Limit).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}
	return items, total, nil
}

func applyNotificationCSVFilter(query *gorm.DB, column, raw string) *gorm.DB {
	if raw == "" {
		return query
	}
	values := strings.Split(raw, ",")
	for index := range values {
		values[index] = strings.TrimSpace(values[index])
	}
	return query.Where(column+" IN ?", values)
}

func (s *notificationStore) GetNotificationCounter(ctx context.Context, recipient string) (*iapiserver.NotificationRecipientCounter, error) {
	var counter iapiserver.NotificationRecipientCounter
	err := s.ds.db.WithContext(ctx).Where("recipient_user_id = ?", recipient).First(&counter).Error
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		counter.RecipientUserID = recipient
		counter.Name = recipient
		return &counter, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get notification counter: %w", err)
	}
	return &counter, nil
}

func (s *notificationStore) MutateNotificationInbox(ctx context.Context, recipient, id, action string) (*iapiserver.Notification, *iapiserver.NotificationRecipientCounter, error) {
	var result *iapiserver.Notification
	var counter *iapiserver.NotificationRecipientCounter
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item iapiserver.Notification
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND recipient_user_id = ? AND deleted_at IS NULL", id, recipient).First(&item).Error
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return store.ErrNotificationNotVisible
		}
		if err != nil {
			return fmt.Errorf("lock notification: %w", err)
		}
		before := item.DeepCopy()
		changed, err := applyInboxAction(&item, action, time.Now().UTC())
		if err != nil {
			return err
		}
		if changed {
			item.ExpiresAt = notificationExpiry(&item, time.Now().UTC())
			if err := tx.Save(&item).Error; err != nil {
				return fmt.Errorf("update notification inbox: %w", err)
			}
			if err := addNotificationOutbox(
				tx,
				&item,
				iapiserver.NotificationEventUpdated,
				notificationChangedFields(before, &item)...,
			); err != nil {
				return err
			}
		}
		counter, err = rebuildNotificationCounter(tx, recipient)
		if err != nil {
			return err
		}
		result = &item
		return nil
	})
	return result, counter, err
}

func notificationExpiry(item *iapiserver.Notification, now time.Time) imachinery.Time {
	days := 180
	switch item.InboxStatus {
	case iapiserver.NotificationInboxArchived, iapiserver.NotificationInboxRead:
		days = 90
	case iapiserver.NotificationInboxUnread:
		if item.Severity == iapiserver.NotificationSeverityError || item.Severity == iapiserver.NotificationSeverityCritical {
			days = 365
		}
	}
	return imachinery.NewTime(now.Add(time.Duration(days) * 24 * time.Hour))
}

func applyInboxAction(item *iapiserver.Notification, action string, now time.Time) (bool, error) {
	switch action {
	case "read":
		if item.InboxStatus == iapiserver.NotificationInboxArchived {
			return false, store.ErrNotificationStateConflict
		}
		if item.InboxStatus == iapiserver.NotificationInboxRead {
			return false, nil
		}
		item.InboxStatus, item.ReadAt = iapiserver.NotificationInboxRead, imachinery.NewTime(now)
		return true, nil
	case "unread":
		if item.InboxStatus == iapiserver.NotificationInboxArchived {
			return false, store.ErrNotificationStateConflict
		}
		if item.InboxStatus == iapiserver.NotificationInboxUnread {
			return false, nil
		}
		item.InboxStatus, item.ReadAt = iapiserver.NotificationInboxUnread, imachinery.Time{}
		return true, nil
	case "archive":
		if item.InboxStatus == iapiserver.NotificationInboxArchived {
			return false, nil
		}
		item.InboxStatus, item.ArchivedAt = iapiserver.NotificationInboxArchived, imachinery.NewTime(now)
		return true, nil
	case "unarchive":
		if item.InboxStatus == iapiserver.NotificationInboxUnread {
			return false, store.ErrNotificationStateConflict
		}
		if item.InboxStatus == iapiserver.NotificationInboxRead {
			return false, nil
		}
		item.InboxStatus, item.ArchivedAt = iapiserver.NotificationInboxRead, imachinery.Time{}
		if item.ReadAt.IsZero() {
			item.ReadAt = imachinery.NewTime(now)
		}
		return true, nil
	default:
		return false, fmt.Errorf("unsupported notification inbox action %q", action)
	}
}

func notificationChangedFields(before, after *iapiserver.Notification) []string {
	if before == nil || after == nil {
		return nil
	}
	changed := make([]string, 0, 12)
	if before.Name != after.Name {
		changed = append(changed, "title")
	}
	if before.Description != after.Description {
		changed = append(changed, "content")
	}
	if before.Severity != after.Severity {
		changed = append(changed, "severity")
	}
	if before.InboxStatus != after.InboxStatus {
		changed = append(changed, "inbox_status")
	}
	if before.AttentionStatus != after.AttentionStatus {
		changed = append(changed, "attention_status")
	}
	if before.OccurrenceCount != after.OccurrenceCount {
		changed = append(changed, "occurrence_count")
	}
	if !before.LastOccurredAt.Equal(&after.LastOccurredAt) {
		changed = append(changed, "last_occurred_at")
	}
	if !before.ReadAt.Equal(&after.ReadAt) {
		changed = append(changed, "read_at")
	}
	if !before.ArchivedAt.Equal(&after.ArchivedAt) {
		changed = append(changed, "archived_at")
	}
	if !before.ResolvedAt.Equal(&after.ResolvedAt) {
		changed = append(changed, "resolved_at")
	}
	if !reflect.DeepEqual(before.NavigationTarget, after.NavigationTarget) {
		changed = append(changed, "navigation_target")
	}
	if !equalOptionalString(before.ActionPath, after.ActionPath) {
		changed = append(changed, "action_path")
	}
	if !before.ExpiresAt.Equal(&after.ExpiresAt) {
		changed = append(changed, "expires_at")
	}
	return changed
}

func equalOptionalString(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func (s *notificationStore) ReadAllNotifications(ctx context.Context, recipient, category string) (int64, *iapiserver.NotificationRecipientCounter, error) {
	var affected int64
	var counter *iapiserver.NotificationRecipientCounter
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("recipient_user_id = ? AND deleted_at IS NULL AND inbox_status = ?", recipient, iapiserver.NotificationInboxUnread)
		if category != "" {
			query = query.Where("category = ?", category)
		}
		var items []*iapiserver.Notification
		if err := query.Find(&items).Error; err != nil {
			return fmt.Errorf("lock unread notifications: %w", err)
		}
		now := time.Now().UTC()
		for _, item := range items {
			before := item.DeepCopy()
			item.InboxStatus, item.ReadAt = iapiserver.NotificationInboxRead, imachinery.NewTime(now)
			item.ExpiresAt = notificationExpiry(item, now)
			if err := tx.Save(item).Error; err != nil {
				return fmt.Errorf("mark all notifications read: %w", err)
			}
			if err := addNotificationOutbox(
				tx,
				item,
				iapiserver.NotificationEventUpdated,
				notificationChangedFields(before, item)...,
			); err != nil {
				return err
			}
		}
		affected = int64(len(items))
		var err error
		counter, err = rebuildNotificationCounter(tx, recipient)
		return err
	})
	return affected, counter, err
}

func rebuildNotificationCounter(tx *gorm.DB, recipient string) (*iapiserver.NotificationRecipientCounter, error) {
	// 同一接收者可能由多个规则 Worker 并发创建不同通知。必须先串行化整个
	// “统计当前收件箱 + 写计数”过程，避免两个事务各自用旧快照覆盖较新的计数。
	if err := tx.Exec(
		"SELECT pg_advisory_xact_lock(hashtextextended(?, 0))",
		"notification-counter:"+recipient,
	).Error; err != nil {
		return nil, fmt.Errorf("lock notification counter projection: %w", err)
	}
	type counts struct{ Unread, Critical, ActionRequired int }
	var value counts
	err := tx.Model(&iapiserver.Notification{}).Select("COUNT(*) FILTER (WHERE inbox_status='unread') AS unread, COUNT(*) FILTER (WHERE inbox_status='unread' AND severity='critical') AS critical, COUNT(*) FILTER (WHERE inbox_status<>'archived' AND attention_status='action_required') AS action_required").Where("recipient_user_id = ? AND deleted_at IS NULL", recipient).Scan(&value).Error
	if err != nil {
		return nil, fmt.Errorf("rebuild notification counter facts: %w", err)
	}
	var counter iapiserver.NotificationRecipientCounter
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("recipient_user_id = ?", recipient).First(&counter).Error
	created := false
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		counter = iapiserver.NotificationRecipientCounter{RecipientUserID: recipient}
		counter.Name = recipient
		created = true
	} else if err != nil {
		return nil, fmt.Errorf("lock notification counter: %w", err)
	}
	changed := created || counter.UnreadCount != value.Unread || counter.CriticalCount != value.Critical || counter.ActionRequiredCount != value.ActionRequired
	counter.UnreadCount, counter.CriticalCount, counter.ActionRequiredCount = value.Unread, value.Critical, value.ActionRequired
	if created {
		err = tx.Create(&counter).Error
	} else if changed {
		err = tx.Save(&counter).Error
	}
	if err != nil {
		return nil, fmt.Errorf("persist notification counter: %w", err)
	}
	if changed {
		if err := addCounterOutbox(tx, &counter); err != nil {
			return nil, err
		}
	}
	return &counter, nil
}

func addNotificationOutbox(tx *gorm.DB, item *iapiserver.Notification, eventName string, changedFields ...string) error {
	if eventName == iapiserver.NotificationEventUpdated && len(changedFields) == 0 {
		return fmt.Errorf("notification_updated requires changed_fields")
	}
	outbox := &iapiserver.NotificationOutbox{EventName: eventName, AggregateType: "notification", AggregateID: item.ID, AggregateVersion: item.ResourceVersion, RecipientUserID: item.RecipientUserID, DeliveryStatus: iapiserver.NotificationOutboxPending, Payload: iapiserver.NotificationOutboxPayload{NotificationID: item.ID, NotificationTopic: item.NotificationTopic, Category: item.Category, Severity: item.Severity, InboxStatus: item.InboxStatus, AttentionStatus: item.AttentionStatus, SourceType: item.SourceType, SourceID: item.SourceID, OccurrenceCount: item.OccurrenceCount, ChangedFields: changedFields, NavigationAvailable: item.ActionPath != nil, ResourceVersion: item.ResourceVersion, DeletedAt: item.DeletedAt, OccurredAt: imachinery.NewTime(time.Now().UTC())}}
	outbox.ID = uuid.NewString()
	outbox.Payload.NotificationOutboxID = outbox.ID
	outbox.Name = eventName + ":" + item.ID
	if err := tx.Create(outbox).Error; err != nil {
		return fmt.Errorf("create notification outbox: %w", err)
	}
	return publishNotificationWatermillOutbox(tx, outbox)
}

func addCounterOutbox(tx *gorm.DB, counter *iapiserver.NotificationRecipientCounter) error {
	outbox := &iapiserver.NotificationOutbox{EventName: iapiserver.NotificationEventUnreadCountChanged, AggregateType: "notification_recipient_counter", AggregateID: counter.RecipientUserID, AggregateVersion: counter.ResourceVersion, RecipientUserID: counter.RecipientUserID, DeliveryStatus: iapiserver.NotificationOutboxPending, Payload: iapiserver.NotificationOutboxPayload{UnreadCount: counter.UnreadCount, CriticalCount: counter.CriticalCount, ActionRequiredCount: counter.ActionRequiredCount, CounterVersion: counter.ResourceVersion, OccurredAt: imachinery.NewTime(time.Now().UTC())}}
	outbox.ID = uuid.NewString()
	outbox.Payload.NotificationOutboxID = outbox.ID
	outbox.Name = iapiserver.NotificationEventUnreadCountChanged + ":" + counter.RecipientUserID
	if err := tx.Create(outbox).Error; err != nil {
		return fmt.Errorf("create notification counter outbox: %w", err)
	}
	return publishNotificationWatermillOutbox(tx, outbox)
}

func publishNotificationWatermillOutbox(tx *gorm.DB, outbox *iapiserver.NotificationOutbox) error {
	payload := map[string]any{
		"notification_outbox_id": outbox.ID, "notification_id": outbox.Payload.NotificationID,
		"notification_topic": outbox.Payload.NotificationTopic, "category": outbox.Payload.Category,
		"severity": outbox.Payload.Severity, "inbox_status": outbox.Payload.InboxStatus,
		"attention_status": outbox.Payload.AttentionStatus, "source_type": outbox.Payload.SourceType,
		"source_id": outbox.Payload.SourceID, "occurrence_count": outbox.Payload.OccurrenceCount,
		"changed_fields": outbox.Payload.ChangedFields, "navigation_available": outbox.Payload.NavigationAvailable,
		"unread_count": outbox.Payload.UnreadCount, "critical_count": outbox.Payload.CriticalCount,
		"action_required_count": outbox.Payload.ActionRequiredCount, "recipient_user_id": outbox.RecipientUserID,
		"aggregate_type": outbox.AggregateType, "aggregate_id": outbox.AggregateID,
		"aggregate_version": outbox.AggregateVersion, "resource_version": outbox.Payload.ResourceVersion,
		"counter_version": outbox.Payload.CounterVersion, "deleted_at": outbox.Payload.DeletedAt,
		"occurred_at": outbox.Payload.OccurredAt,
	}
	if err := publishOutbox(tx, outbox.EventName, outbox.ID, payload); err != nil {
		return fmt.Errorf("publish notification watermill outbox: %w", err)
	}
	return nil
}

func (s *notificationStore) ListNotificationTopics(ctx context.Context, enabledOnly bool) ([]*iapiserver.NotificationTopic, error) {
	query := s.ds.db.WithContext(ctx).Order("category ASC, topic ASC")
	if enabledOnly {
		query = query.Where("activation_status = ? AND enabled = TRUE", iapiserver.NotificationTopicActive)
	}
	var items []*iapiserver.NotificationTopic
	if err := query.Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list notification topics: %w", err)
	}
	return items, nil
}

func (s *notificationStore) ListNotificationPreferences(ctx context.Context, userID string) ([]*iapiserver.NotificationPreference, error) {
	var items []*iapiserver.NotificationPreference
	if err := s.ds.db.WithContext(ctx).Where("user_id = ?", userID).Order("category ASC, notification_topic ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list notification preferences: %w", err)
	}
	return items, nil
}

func (s *notificationStore) ReplaceNotificationPreferences(ctx context.Context, userID string, items []*iapiserver.NotificationPreference) ([]*iapiserver.NotificationPreference, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&iapiserver.NotificationPreference{}).Error; err != nil {
			return fmt.Errorf("delete notification preferences: %w", err)
		}
		for _, item := range items {
			item.UserID, item.Name = userID, item.Category+":"+item.NotificationTopic
			if err := createNotificationPreference(tx, item); err != nil {
				return fmt.Errorf("create notification preference: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.ListNotificationPreferences(ctx, userID)
}

func createNotificationPreference(tx *gorm.DB, item *iapiserver.NotificationPreference) error {
	if item == nil {
		return fmt.Errorf("notification preference is nil")
	}
	// GORM 会把带 default:true 标签的 false 当成未赋值并回填 true。偏好中的
	// false 是显式用户选择，因此在执行公共元数据 hook 后用 map 原样写入全部列。
	if err := item.BeforeCreate(tx); err != nil {
		return err
	}
	var muteUntil any
	if !item.MuteUntil.IsZero() {
		muteUntil = item.MuteUntil
	}
	return tx.Table(item.TableName()).Create(map[string]any{
		"id":                  item.ID,
		"name":                item.Name,
		"created_at":          item.CreatedAt,
		"updated_at":          item.UpdatedAt,
		"description":         item.Description,
		"extend_shadow":       item.ExtendShadow,
		"resource_version":    item.ResourceVersion,
		"user_id":             item.UserID,
		"category":            item.Category,
		"notification_topic":  item.NotificationTopic,
		"in_app_enabled":      item.InAppEnabled,
		"email_enabled":       item.EmailEnabled,
		"webhook_enabled":     item.WebhookEnabled,
		"mobile_push_enabled": item.MobilePushEnabled,
		"minimum_severity":    item.MinimumSeverity,
		"merge_repeated":      item.MergeRepeated,
		"digest_mode":         item.DigestMode,
		"mute_until":          muteUntil,
	}).Error
}
