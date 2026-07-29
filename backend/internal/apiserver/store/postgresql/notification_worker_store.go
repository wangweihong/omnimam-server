package postgresql

import (
	"context"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/notificationsanitize"
)

// AddNotificationCandidates 幂等保存源事件的最小安全候选；成功返回后上游消息才可 ACK。
func (s *notificationStore) AddNotificationCandidates(ctx context.Context, items []*iapiserver.NotificationEvent) error {
	if len(items) == 0 {
		return nil
	}
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			if item == nil {
				return fmt.Errorf("notification candidate is nil")
			}
			result := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "source_domain"}, {Name: "source_event_id"}, {Name: "notification_topic"}},
				DoNothing: true,
			}).Create(item)
			if result.Error != nil {
				return fmt.Errorf("add notification candidate: %w", result.Error)
			}
		}
		return nil
	})
}

// ClaimNotificationCandidates 使用有限租约和 SKIP LOCKED 支持多副本并发处理与崩溃恢复。
func (s *notificationStore) ClaimNotificationCandidates(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]*iapiserver.NotificationEvent, error) {
	if limit <= 0 {
		return []*iapiserver.NotificationEvent{}, nil
	}
	if lease <= 0 {
		return nil, fmt.Errorf("notification candidate lease must be positive")
	}
	var items []*iapiserver.NotificationEvent
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		eligible := `(processing_status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)) OR
			(processing_status = ? AND next_attempt_at <= ?)`
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where(eligible, iapiserver.NotificationEventPending, now, iapiserver.NotificationEventFailed, now).
			Order("created_at ASC").Limit(limit).Find(&items).Error; err != nil {
			return fmt.Errorf("claim notification candidates: %w", err)
		}
		leaseUntil := imachinery.NewTime(now.Add(lease))
		for _, item := range items {
			item.ProcessingStatus = iapiserver.NotificationEventFailed
			item.ProcessingAttemptCount++
			item.NextAttemptAt = leaseUntil
			item.LastErrorCode = ""
			item.LastErrorSummary = ""
			if err := tx.Save(item).Error; err != nil {
				return fmt.Errorf("lease notification candidate %s: %w", item.ID, err)
			}
		}
		return nil
	})
	return items, err
}

// FinishNotificationCandidate 只接受当前 claim attempt 的状态推进；过期 Worker 的完成结果幂等忽略。
func (s *notificationStore) FinishNotificationCandidate(ctx context.Context, id string, expectedAttempt int, status string, nextAttempt time.Time, errorCode, errorSummary string) error {
	allowed := map[string]bool{
		iapiserver.NotificationEventProcessed:  true,
		iapiserver.NotificationEventIgnored:    true,
		iapiserver.NotificationEventFailed:     true,
		iapiserver.NotificationEventDeadLetter: true,
	}
	if !allowed[status] {
		return fmt.Errorf("unsupported notification candidate status %q", status)
	}
	if expectedAttempt < 1 {
		return fmt.Errorf("notification candidate expected attempt must be positive")
	}
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item iapiserver.NotificationEvent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, "id = ?", id).Error; err != nil {
			return fmt.Errorf("lock notification candidate: %w", err)
		}
		if item.ProcessingAttemptCount != expectedAttempt ||
			item.ProcessingStatus != iapiserver.NotificationEventFailed {
			return nil
		}
		item.ProcessingStatus = status
		item.LastErrorCode = boundedNotificationText(errorCode, 128)
		item.LastErrorSummary = boundedNotificationText(errorSummary, 500)
		if status == iapiserver.NotificationEventFailed {
			item.NextAttemptAt = imachinery.NewTime(nextAttempt)
			item.ProcessedAt = imachinery.Time{}
		} else {
			item.NextAttemptAt = imachinery.Time{}
			item.ProcessedAt = imachinery.NewTime(time.Now().UTC())
		}
		if err := tx.Save(&item).Error; err != nil {
			return fmt.Errorf("finish notification candidate: %w", err)
		}
		return nil
	})
}

// MaterializeNotification 在同一事务中执行偏好门禁、版本单调聚合、计数和 Outbox 写入。
func (s *notificationStore) MaterializeNotification(ctx context.Context, event *iapiserver.NotificationEvent, draft store.NotificationMaterialization) (*iapiserver.Notification, bool, error) {
	if event == nil {
		return nil, false, fmt.Errorf("notification candidate is nil")
	}
	draft.Title = notificationsanitize.Text(draft.Title, 200)
	draft.Content = notificationsanitize.Text(draft.Content, 4000)
	if draft.Title == "" || draft.Content == "" {
		return nil, false, fmt.Errorf("notification title and content must remain non-empty after sanitization")
	}
	var result *iapiserver.Notification
	created := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var candidate iapiserver.NotificationEvent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&candidate, "id = ?", event.ID).Error; err != nil {
			return fmt.Errorf("lock notification candidate for materialization: %w", err)
		}
		var topic iapiserver.NotificationTopic
		if err := tx.Where("topic = ? AND activation_status = ? AND enabled = TRUE", candidate.NotificationTopic, iapiserver.NotificationTopicActive).First(&topic).Error; err != nil {
			return fmt.Errorf("load active notification topic: %w", err)
		}

		enabled, mergeRepeated, err := notificationPreferenceDecision(tx, draft.RecipientUserID, &topic, draft.Severity)
		if err != nil {
			return err
		}
		if !enabled {
			result = nil
			return nil
		}

		window := draft.AggregationWindow
		if window == "" {
			window = topic.AggregationMode
		}
		if !mergeRepeated {
			window = "immediate"
		}
		aggregateKey, bucket := notificationAggregation(candidate.NotificationTopic, candidate.SourceType, candidate.SourceID, draft.AggregateKey, window, candidate.OccurredAt.Time)
		deduplicationKey := draft.RecipientUserID + ":" + candidate.SourceDomain + ":" + candidate.SourceEventID + ":" + candidate.NotificationTopic
		lockKey := "notification-dedup:" + draft.RecipientUserID + ":" + deduplicationKey
		if aggregateKey != "" {
			lockKey = "notification-aggregate:" + draft.RecipientUserID + ":" + aggregateKey + ":" + window + ":" + bucket.UTC().Format(time.RFC3339)
		}
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", lockKey).Error; err != nil {
			return fmt.Errorf("lock notification materialization key: %w", err)
		}

		var existing iapiserver.Notification
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("recipient_user_id = ? AND deduplication_key = ?", draft.RecipientUserID, deduplicationKey).
			First(&existing).Error
		if err == nil {
			result = &existing
			return nil
		}
		if !stderrors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("lookup notification deduplication key: %w", err)
		}

		if aggregateKey != "" {
			err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("recipient_user_id = ? AND aggregate_key = ? AND aggregation_window = ? AND aggregation_bucket = ? AND deleted_at IS NULL", draft.RecipientUserID, aggregateKey, window, bucket).
				First(&existing).Error
			if err == nil {
				if candidate.SourceAggregateVersion <= existing.SourceAggregateVersion {
					result = &existing
					return nil
				}
				before := existing.DeepCopy()
				existing.Name = draft.Title
				existing.Description = draft.Content
				existing.Severity = draft.Severity
				existing.AttentionStatus = draft.AttentionStatus
				existing.SourceAggregateVersion = candidate.SourceAggregateVersion
				existing.NavigationTarget = draft.NavigationTarget
				existing.ActionPath = draft.ActionPath
				existing.OccurrenceCount++
				existing.LastOccurredAt = candidate.OccurredAt
				existing.ExpiresAt = draft.ExpiresAt
				if draft.AttentionStatus == iapiserver.NotificationAttentionResolved && existing.ResolvedAt.IsZero() {
					existing.ResolvedAt = imachinery.NewTime(time.Now().UTC())
				} else if draft.AttentionStatus != iapiserver.NotificationAttentionResolved {
					existing.ResolvedAt = imachinery.Time{}
				}
				if err := tx.Save(&existing).Error; err != nil {
					return fmt.Errorf("update aggregated notification: %w", err)
				}
				if err := addNotificationEventLink(tx, &existing, &candidate); err != nil {
					return err
				}
				if err := addNotificationOutbox(
					tx,
					&existing,
					iapiserver.NotificationEventUpdated,
					notificationChangedFields(before, &existing)...,
				); err != nil {
					return err
				}
				if _, err := rebuildNotificationCounter(tx, draft.RecipientUserID); err != nil {
					return err
				}
				result = &existing
				return nil
			}
			if !stderrors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("lookup notification aggregate: %w", err)
			}
		}

		item := &iapiserver.Notification{
			RecipientUserID:        draft.RecipientUserID,
			Category:               topic.Category,
			NotificationTopic:      candidate.NotificationTopic,
			Severity:               draft.Severity,
			InboxStatus:            iapiserver.NotificationInboxUnread,
			AttentionStatus:        draft.AttentionStatus,
			SourceType:             candidate.SourceType,
			SourceID:               candidate.SourceID,
			SourceAggregateVersion: candidate.SourceAggregateVersion,
			NavigationTarget:       draft.NavigationTarget,
			ActionPath:             draft.ActionPath,
			DeduplicationKey:       deduplicationKey,
			AggregateKey:           aggregateKey,
			AggregationWindow:      window,
			AggregationBucket:      bucket,
			OccurrenceCount:        1,
			FirstOccurredAt:        candidate.OccurredAt,
			LastOccurredAt:         candidate.OccurredAt,
			ExpiresAt:              draft.ExpiresAt,
		}
		item.Name, item.Description = draft.Title, draft.Content
		if item.AttentionStatus == iapiserver.NotificationAttentionResolved {
			item.ResolvedAt = imachinery.NewTime(time.Now().UTC())
		}
		if err := tx.Create(item).Error; err != nil {
			return fmt.Errorf("create notification: %w", err)
		}
		if err := addNotificationEventLink(tx, item, &candidate); err != nil {
			return err
		}
		if err := addNotificationOutbox(tx, item, iapiserver.NotificationEventCreated); err != nil {
			return err
		}
		if _, err := rebuildNotificationCounter(tx, draft.RecipientUserID); err != nil {
			return err
		}
		created, result = true, item
		return nil
	})
	return result, created, err
}

func notificationPreferenceDecision(tx *gorm.DB, userID string, topic *iapiserver.NotificationTopic, severity string) (bool, bool, error) {
	if topic.MandatoryInApp {
		return true, true, nil
	}
	var preference iapiserver.NotificationPreference
	err := tx.Where("user_id = ? AND category = ? AND notification_topic = ?", userID, topic.Category, topic.Topic).First(&preference).Error
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		err = tx.Where("user_id = ? AND category = ? AND notification_topic = ''", userID, topic.Category).First(&preference).Error
	}
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return true, true, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("load notification preference: %w", err)
	}
	return preference.InAppEnabled && notificationSeverityRank(severity) >= notificationSeverityRank(preference.MinimumSeverity), preference.MergeRepeated, nil
}

func notificationSeverityRank(value string) int {
	return map[string]int{"info": 0, "success": 1, "warning": 2, "error": 3, "critical": 4}[value]
}

func notificationAggregation(topic, sourceType, sourceID, requested, window string, occurredAt time.Time) (string, imachinery.Time) {
	if window == "immediate" {
		return "", imachinery.Time{}
	}
	var bucket time.Time
	switch window {
	case "per_source":
		bucket = time.Unix(0, 0).UTC()
	case "1_minute":
		bucket = occurredAt.UTC().Truncate(time.Minute)
	case "5_minutes":
		bucket = occurredAt.UTC().Truncate(5 * time.Minute)
	case "1_hour":
		bucket = occurredAt.UTC().Truncate(time.Hour)
	case "daily_digest":
		u := occurredAt.UTC()
		bucket = time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
	default:
		return "", imachinery.Time{}
	}
	key := topic + ":" + sourceType + ":" + sourceID + ":" + window + ":" + bucket.UTC().Format(time.RFC3339)
	if requested != "" && requested != sourceID {
		key += ":" + requested
	}
	return key, imachinery.NewTime(bucket)
}

func addNotificationEventLink(tx *gorm.DB, notification *iapiserver.Notification, event *iapiserver.NotificationEvent) error {
	link := &iapiserver.NotificationEventLink{
		NotificationID:         notification.ID,
		NotificationEventID:    event.ID,
		SourceAggregateVersion: event.SourceAggregateVersion,
		OccurrenceDelta:        1,
	}
	link.ID = uuid.NewString()
	link.Name = notification.ID + ":" + event.ID
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(link).Error; err != nil {
		return fmt.Errorf("create notification event link: %w", err)
	}
	return nil
}

// MarkNotificationOutboxPublished 在 UserEvent 已持久化后确认通知出站事件。
func (s *notificationStore) MarkNotificationOutboxPublished(ctx context.Context, id string, publishedAt time.Time) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item iapiserver.NotificationOutbox
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, "id = ?", id).Error; err != nil {
			return fmt.Errorf("lock notification outbox: %w", err)
		}
		if item.DeliveryStatus == iapiserver.NotificationOutboxPublished {
			return nil
		}
		item.DeliveryStatus = iapiserver.NotificationOutboxPublished
		item.PublishedAt = imachinery.NewTime(publishedAt)
		item.NextAttemptAt = imachinery.Time{}
		item.Description = ""
		if err := tx.Save(&item).Error; err != nil {
			return fmt.Errorf("publish notification outbox: %w", err)
		}
		return nil
	})
}

// MarkNotificationOutboxFailed 保留收件箱事实，只记录受控摘要并安排 SSE 投影重试。
func (s *notificationStore) MarkNotificationOutboxFailed(ctx context.Context, id string, nextAttempt time.Time, errorCode, errorSummary string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item iapiserver.NotificationOutbox
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, "id = ?", id).Error; err != nil {
			return fmt.Errorf("lock notification outbox: %w", err)
		}
		if item.DeliveryStatus == iapiserver.NotificationOutboxPublished {
			return nil
		}
		item.DeliveryStatus = iapiserver.NotificationOutboxFailed
		item.AttemptCount++
		item.NextAttemptAt = imachinery.NewTime(nextAttempt)
		item.Description = boundedNotificationText(errorCode+":"+errorSummary, 500)
		if err := tx.Save(&item).Error; err != nil {
			return fmt.Errorf("fail notification outbox: %w", err)
		}
		return nil
	})
}

// CleanupNotifications 逻辑删除到期通知，并清理已失去引用的候选和已投递 Outbox。
func (s *notificationStore) CleanupNotifications(ctx context.Context, now time.Time, limit int) (store.NotificationRetentionResult, error) {
	var result store.NotificationRetentionResult
	if limit <= 0 {
		return result, nil
	}
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var notifications []*iapiserver.Notification
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("deleted_at IS NULL AND expires_at IS NOT NULL AND expires_at <= ?", now).
			Order("expires_at ASC").Limit(limit).Find(&notifications).Error; err != nil {
			return fmt.Errorf("claim expired notifications: %w", err)
		}
		recipients := make(map[string]struct{})
		for _, item := range notifications {
			item.DeletedAt = imachinery.NewTime(now)
			if err := tx.Save(item).Error; err != nil {
				return fmt.Errorf("delete expired notification: %w", err)
			}
			if err := addNotificationOutbox(tx, item, iapiserver.NotificationEventDeleted); err != nil {
				return err
			}
			recipients[item.RecipientUserID] = struct{}{}
			result.Notifications++
		}
		for recipient := range recipients {
			if _, err := rebuildNotificationCounter(tx, recipient); err != nil {
				return err
			}
		}

		var tombstoneIDs []string
		if err := tx.Model(&iapiserver.Notification{}).Select("notifications.id").
			Where(`notifications.deleted_at IS NOT NULL
				AND EXISTS (
					SELECT 1 FROM notification_outbox
					WHERE notification_outbox.aggregate_type = 'notification'
					  AND notification_outbox.aggregate_id = notifications.id
					  AND notification_outbox.event_name = ?
					  AND notification_outbox.aggregate_version = notifications.resource_version
					  AND notification_outbox.delivery_status = ?
				)
				AND NOT EXISTS (
					SELECT 1 FROM notification_outbox
					WHERE notification_outbox.aggregate_type = 'notification'
					  AND notification_outbox.aggregate_id = notifications.id
					  AND notification_outbox.delivery_status <> ?
				)`,
				iapiserver.NotificationEventDeleted,
				iapiserver.NotificationOutboxPublished,
				iapiserver.NotificationOutboxPublished,
			).
			Order("notifications.deleted_at ASC").Limit(limit).Scan(&tombstoneIDs).Error; err != nil {
			return fmt.Errorf("list projected notification tombstones: %w", err)
		}
		if len(tombstoneIDs) > 0 {
			if err := tx.Where("notification_id IN ?", tombstoneIDs).Delete(&iapiserver.NotificationEventLink{}).Error; err != nil {
				return fmt.Errorf("delete projected notification links: %w", err)
			}
			if err := tx.Where("id IN ?", tombstoneIDs).Delete(&iapiserver.Notification{}).Error; err != nil {
				return fmt.Errorf("delete projected notification tombstones: %w", err)
			}
		}

		candidateIDs, err := notificationCleanupIDs(tx, &iapiserver.NotificationEvent{}, "expires_at <= ? AND NOT EXISTS (SELECT 1 FROM notification_event_links WHERE notification_event_links.notification_event_id = notification_events.id)", limit, now)
		if err != nil {
			return fmt.Errorf("list expired notification candidates: %w", err)
		}
		if len(candidateIDs) > 0 {
			cleanup := tx.Where("id IN ?", candidateIDs).Delete(&iapiserver.NotificationEvent{})
			if cleanup.Error != nil {
				return fmt.Errorf("delete expired notification candidates: %w", cleanup.Error)
			}
			result.Candidates = cleanup.RowsAffected
		}

		outboxIDs, err := notificationCleanupIDs(tx, &iapiserver.NotificationOutbox{}, "delivery_status = ? AND published_at IS NOT NULL AND published_at <= ?", limit, iapiserver.NotificationOutboxPublished, now)
		if err != nil {
			return fmt.Errorf("list published notification outbox: %w", err)
		}
		if len(outboxIDs) > 0 {
			cleanup := tx.Where("id IN ?", outboxIDs).Delete(&iapiserver.NotificationOutbox{})
			if cleanup.Error != nil {
				return fmt.Errorf("delete published notification outbox: %w", cleanup.Error)
			}
			result.Outbox = cleanup.RowsAffected
		}
		return nil
	})
	return result, err
}

func notificationCleanupIDs(tx *gorm.DB, model any, condition string, limit int, args ...any) ([]string, error) {
	var ids []string
	err := tx.Model(model).Select("id").Where(condition, args...).Order("created_at ASC").Limit(limit).Scan(&ids).Error
	return ids, err
}

func boundedNotificationText(value string, limit int) string {
	return notificationsanitize.Text(value, limit)
}
