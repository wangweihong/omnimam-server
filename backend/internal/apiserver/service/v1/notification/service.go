package notification

import (
	"context"
	stderrors "errors"
	"sort"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
)

// Service 提供当前认证用户的通知收件箱、计数和基础偏好能力。
type Service interface {
	List(context.Context, *iapiserver.NotificationListRequest) (*iapiserver.NotificationListResponse, error)
	UnreadCount(context.Context) (*iapiserver.NotificationUnreadCount, error)
	BatchRead(context.Context, *iapiserver.BatchNotificationRequest) (*iapiserver.BatchNotificationResponse, error)
	ReadAll(context.Context, *iapiserver.ReadAllNotificationsRequest) (*iapiserver.NotificationBulkActionSummary, error)
	Read(context.Context, string) (*iapiserver.NotificationResponse, error)
	Unread(context.Context, string) (*iapiserver.NotificationResponse, error)
	Archive(context.Context, string) (*iapiserver.NotificationResponse, error)
	Unarchive(context.Context, string) (*iapiserver.NotificationResponse, error)
	GetPreferences(context.Context) (*iapiserver.NotificationPreferenceSet, error)
	PutPreferences(context.Context, *iapiserver.NotificationPreferenceSetRequest) (*iapiserver.NotificationPreferenceSet, error)
}

type service struct{ store store.NotificationStore }

// New 使用消费方 NotificationStore 构造服务，测试无需依赖完整 Factory。
func New(notificationStore store.NotificationStore) Service {
	return &service{store: notificationStore}
}

func currentUserID(ctx context.Context) (string, error) {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err != nil || user == nil || user.ID == "" {
		return "", errors.NewStatus(code.ErrNotificationPermissionDenied, "authenticated user is required")
	}
	return user.ID, nil
}

func (s *service) List(ctx context.Context, req *iapiserver.NotificationListRequest) (*iapiserver.NotificationListResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	req.RecipientUserID = userID
	if req.NotificationTopics != "" {
		topics, topicErr := s.store.ListNotificationTopics(ctx, false)
		if topicErr != nil {
			return nil, topicErr
		}
		registered := make(map[string]struct{}, len(topics))
		for _, topic := range topics {
			registered[topic.Topic] = struct{}{}
		}
		for _, topic := range strings.Split(req.NotificationTopics, ",") {
			if _, ok := registered[strings.TrimSpace(topic)]; !ok {
				return nil, errors.NewStatus(code.ErrNotificationQueryInvalid, "notification topic filter is not registered")
			}
		}
	}
	items, total, err := s.store.ListNotifications(ctx, req)
	if err != nil {
		return nil, err
	}
	result := make([]*iapiserver.NotificationResponse, 0, len(items))
	for _, item := range items {
		result = append(result, notificationResponse(item))
	}
	return &iapiserver.NotificationListResponse{Total: total, Items: result}, nil
}

func (s *service) UnreadCount(ctx context.Context) (*iapiserver.NotificationUnreadCount, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	counter, err := s.store.GetNotificationCounter(ctx, userID)
	if err != nil {
		return nil, err
	}
	return counterResponse(counter), nil
}

func (s *service) BatchRead(ctx context.Context, req *iapiserver.BatchNotificationRequest) (*iapiserver.BatchNotificationResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	result := &iapiserver.BatchNotificationResponse{Total: len(req.Items), Results: make([]iapiserver.BatchNotificationItemResult, 0, len(req.Items))}
	for _, item := range req.Items {
		notification, _, itemErr := s.store.MutateNotificationInbox(ctx, userID, item.ID, "read")
		entry := iapiserver.BatchNotificationItemResult{ID: item.ID, Success: itemErr == nil}
		if itemErr == nil {
			entry.Data, result.Success = notificationResponse(notification), result.Success+1
		} else if stderrors.Is(itemErr, store.ErrNotificationNotVisible) || stderrors.Is(itemErr, store.ErrNotificationStateConflict) {
			entry.Error, result.Fail = batchError(itemErr), result.Fail+1
		} else {
			return nil, itemErr
		}
		result.Results = append(result.Results, entry)
	}
	counter, err := s.store.GetNotificationCounter(ctx, userID)
	if err != nil {
		return nil, err
	}
	result.UnreadCount = counter.UnreadCount
	return result, nil
}

func (s *service) ReadAll(ctx context.Context, req *iapiserver.ReadAllNotificationsRequest) (*iapiserver.NotificationBulkActionSummary, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	category := ""
	if req != nil {
		category = req.Category
	}
	affected, counter, err := s.store.ReadAllNotifications(ctx, userID, category)
	if err != nil {
		return nil, err
	}
	return &iapiserver.NotificationBulkActionSummary{AffectedCount: affected, UnreadCount: counter.UnreadCount, CriticalCount: counter.CriticalCount, ActionRequiredCount: counter.ActionRequiredCount, ResourceVersion: counter.ResourceVersion, UpdatedAt: counter.UpdatedAt}, nil
}

func (s *service) Read(ctx context.Context, id string) (*iapiserver.NotificationResponse, error) {
	return s.mutate(ctx, id, "read")
}
func (s *service) Unread(ctx context.Context, id string) (*iapiserver.NotificationResponse, error) {
	return s.mutate(ctx, id, "unread")
}
func (s *service) Archive(ctx context.Context, id string) (*iapiserver.NotificationResponse, error) {
	return s.mutate(ctx, id, "archive")
}
func (s *service) Unarchive(ctx context.Context, id string) (*iapiserver.NotificationResponse, error) {
	return s.mutate(ctx, id, "unarchive")
}

func (s *service) mutate(ctx context.Context, id, action string) (*iapiserver.NotificationResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	item, _, err := s.store.MutateNotificationInbox(ctx, userID, id, action)
	if stderrors.Is(err, store.ErrNotificationNotVisible) {
		return nil, errors.NewStatus(code.ErrNotificationNotVisible, "notification is not visible")
	}
	if stderrors.Is(err, store.ErrNotificationStateConflict) {
		return nil, errors.NewStatus(code.ErrNotificationStateConflict, "notification inbox state conflicts with action")
	}
	if err != nil {
		return nil, err
	}
	return notificationResponse(item), nil
}

func (s *service) GetPreferences(ctx context.Context) (*iapiserver.NotificationPreferenceSet, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.store.ListNotificationPreferences(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.preferenceSet(ctx, items)
}

func (s *service) PutPreferences(ctx context.Context, req *iapiserver.NotificationPreferenceSetRequest) (*iapiserver.NotificationPreferenceSet, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	topics, err := s.store.ListNotificationTopics(ctx, true)
	if err != nil {
		return nil, err
	}
	topicByName := make(map[string]*iapiserver.NotificationTopic, len(topics))
	for _, topic := range topics {
		topicByName[topic.Topic] = topic
	}
	items := make([]*iapiserver.NotificationPreference, 0, len(req.Items))
	for _, request := range req.Items {
		topicName := ""
		if request.NotificationTopic != nil {
			topicName = *request.NotificationTopic
			topic, ok := topicByName[topicName]
			if !ok || topic.Category != request.Category {
				return nil, errors.NewStatus(code.ErrNotificationPreferenceInvalid, "notification topic is unknown or does not match category")
			}
			if topic.MandatoryInApp && !*request.InAppEnabled {
				return nil, errors.NewStatus(code.ErrNotificationMandatoryTopicDisabled, "mandatory notification topic cannot disable in-app delivery")
			}
		}
		items = append(items, &iapiserver.NotificationPreference{Category: request.Category, NotificationTopic: topicName, InAppEnabled: *request.InAppEnabled, MinimumSeverity: request.MinimumSeverity, MergeRepeated: *request.MergeRepeated, DigestMode: "none"})
	}
	stored, err := s.store.ReplaceNotificationPreferences(ctx, userID, items)
	if err != nil {
		return nil, err
	}
	return s.preferenceSet(ctx, stored)
}

func (s *service) preferenceSet(ctx context.Context, preferences []*iapiserver.NotificationPreference) (*iapiserver.NotificationPreferenceSet, error) {
	topics, err := s.store.ListNotificationTopics(ctx, true)
	if err != nil {
		return nil, err
	}
	mandatory := make([]string, 0)
	topicByName := make(map[string]*iapiserver.NotificationTopic, len(topics))
	for _, topic := range topics {
		topicByName[topic.Topic] = topic
		if topic.MandatoryInApp {
			mandatory = append(mandatory, topic.Topic)
		}
	}
	sort.Strings(mandatory)
	items := make([]iapiserver.NotificationPreferenceItem, 0, len(preferences))
	for _, item := range preferences {
		var topic *string
		mandatoryItem := false
		if item.NotificationTopic != "" {
			value := item.NotificationTopic
			topic = &value
			mandatoryItem = topicByName[value] != nil && topicByName[value].MandatoryInApp
		}
		items = append(items, iapiserver.NotificationPreferenceItem{ID: item.ID, Category: item.Category, NotificationTopic: topic, InAppEnabled: item.InAppEnabled, MinimumSeverity: item.MinimumSeverity, MergeRepeated: item.MergeRepeated, Mandatory: mandatoryItem, ResourceVersion: item.ResourceVersion, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt})
	}
	return &iapiserver.NotificationPreferenceSet{Defaults: iapiserver.NotificationPreferenceDefaults{InAppEnabled: true, MinimumSeverity: iapiserver.NotificationSeverityInfo, MergeRepeated: true}, Capabilities: iapiserver.NotificationChannelCapabilities{InApp: true}, MandatoryTopics: mandatory, Total: len(items), Items: items}, nil
}

func notificationResponse(item *iapiserver.Notification) *iapiserver.NotificationResponse {
	if item == nil {
		return nil
	}
	return &iapiserver.NotificationResponse{ID: item.ID, Category: item.Category, NotificationTopic: item.NotificationTopic, Severity: item.Severity, Title: item.Name, Content: item.Description, InboxStatus: item.InboxStatus, AttentionStatus: item.AttentionStatus, SourceType: item.SourceType, SourceID: item.SourceID, NavigationTarget: item.NavigationTarget, ActionPath: item.ActionPath, OccurrenceCount: item.OccurrenceCount, FirstOccurredAt: item.FirstOccurredAt, LastOccurredAt: item.LastOccurredAt, ReadAt: optionalTime(item.ReadAt), ArchivedAt: optionalTime(item.ArchivedAt), ResolvedAt: optionalTime(item.ResolvedAt), ExpiresAt: optionalTime(item.ExpiresAt), ResourceVersion: item.ResourceVersion, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func optionalTime(value imachinery.Time) *imachinery.Time {
	if value.IsZero() {
		return nil
	}
	copied := value
	return &copied
}
func counterResponse(counter *iapiserver.NotificationRecipientCounter) *iapiserver.NotificationUnreadCount {
	return &iapiserver.NotificationUnreadCount{UnreadCount: counter.UnreadCount, CriticalCount: counter.CriticalCount, ActionRequiredCount: counter.ActionRequiredCount, ResourceVersion: counter.ResourceVersion, UpdatedAt: counter.UpdatedAt}
}

func batchError(err error) *iapiserver.NotificationAPIError {
	if stderrors.Is(err, store.ErrNotificationNotVisible) {
		return &iapiserver.NotificationAPIError{Code: "ERR_NOTIFICATION_NOT_VISIBLE", Value: code.ErrNotificationNotVisible, Message: "The notification does not exist or is not visible to the current user.", Messages: map[string]string{"zh-CN": "通知不存在或不属于当前用户。", "en-US": "The notification does not exist or is not visible to the current user."}}
	}
	if stderrors.Is(err, store.ErrNotificationStateConflict) {
		return &iapiserver.NotificationAPIError{Code: "ERR_NOTIFICATION_STATE_CONFLICT", Value: code.ErrNotificationStateConflict, Message: "The current notification state does not allow this inbox action.", Messages: map[string]string{"zh-CN": "当前通知状态不允许执行该收件箱操作。", "en-US": "The current notification state does not allow this inbox action."}}
	}
	return &iapiserver.NotificationAPIError{Code: "ERR_NOTIFICATION_RULE_PROCESSING_FAILED", Value: code.ErrNotificationRuleProcessingFailed, Message: "Notification rule processing, deduplication, aggregation, or inbox persistence has temporarily failed.", Messages: map[string]string{"zh-CN": "通知规则处理、去重、聚合或收件箱写入暂时失败。", "en-US": "Notification rule processing, deduplication, aggregation, or inbox persistence has temporarily failed."}, Retryable: true}
}
