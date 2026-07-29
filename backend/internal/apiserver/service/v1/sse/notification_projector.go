package sse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/notificationsanitize"
)

const notificationProjectorConsumerGroup = "sse-notification-projector"

// NotificationProjector 将 Notification Outbox 映射到统一 UserEvent，并在投影持久化后确认 outbox。
type NotificationProjector struct {
	events    store.UserEventStore
	outbox    store.NotificationOutboxStore
	retention time.Duration
	subscribe SubscribeFunc
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func NewNotificationProjector(
	eventStore store.UserEventStore,
	outboxStore store.NotificationOutboxStore,
	retention time.Duration,
	subscribe SubscribeFunc,
) *NotificationProjector {
	if retention <= 0 {
		retention = 24 * time.Hour
	}
	return &NotificationProjector{events: eventStore, outbox: outboxStore, retention: retention, subscribe: subscribe}
}

// Start 为四种 Notification Outbox 事件建立独立于其他领域 projector 的 durable subscription。
func (p *NotificationProjector) Start(parent context.Context) error {
	if p.events == nil || p.outbox == nil || p.subscribe == nil {
		return fmt.Errorf("notification SSE projector dependencies are not configured")
	}
	ctx, cancel := context.WithCancel(parent)
	p.cancel = cancel
	for _, topic := range []string{
		iapiserver.NotificationEventCreated,
		iapiserver.NotificationEventUpdated,
		iapiserver.NotificationEventDeleted,
		iapiserver.NotificationEventUnreadCountChanged,
	} {
		messages, err := p.subscribe(ctx, topic, notificationProjectorConsumerGroup)
		if err != nil {
			cancel()
			p.wg.Wait()
			return fmt.Errorf("subscribe notification SSE topic %s: %w", topic, err)
		}
		p.wg.Add(1)
		go p.consume(ctx, topic, messages)
	}
	return nil
}

func (p *NotificationProjector) consume(ctx context.Context, topic string, messages <-chan *message.Message) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, open := <-messages:
			if !open {
				return
			}
			if err := p.project(ctx, topic, msg.Payload); err != nil {
				msg.Nack()
				continue
			}
			msg.Ack()
		}
	}
}

type notificationOutboxEvent struct {
	NotificationOutboxID string          `json:"notification_outbox_id"`
	NotificationID       string          `json:"notification_id"`
	NotificationTopic    string          `json:"notification_topic"`
	Category             string          `json:"category"`
	Severity             string          `json:"severity"`
	InboxStatus          string          `json:"inbox_status"`
	AttentionStatus      string          `json:"attention_status"`
	SourceType           string          `json:"source_type"`
	SourceID             string          `json:"source_id"`
	OccurrenceCount      int             `json:"occurrence_count"`
	ChangedFields        []string        `json:"changed_fields"`
	NavigationAvailable  bool            `json:"navigation_available"`
	UnreadCount          int             `json:"unread_count"`
	CriticalCount        int             `json:"critical_count"`
	ActionRequiredCount  int             `json:"action_required_count"`
	RecipientUserID      string          `json:"recipient_user_id"`
	AggregateType        string          `json:"aggregate_type"`
	AggregateID          string          `json:"aggregate_id"`
	AggregateVersion     int64           `json:"aggregate_version"`
	ResourceVersion      int64           `json:"resource_version"`
	CounterVersion       int64           `json:"counter_version"`
	DeletedAt            imachinery.Time `json:"deleted_at"`
	OccurredAt           imachinery.Time `json:"occurred_at"`
}

func (p *NotificationProjector) project(ctx context.Context, topic string, raw []byte) (resultErr error) {
	var source notificationOutboxEvent
	if err := json.Unmarshal(raw, &source); err != nil {
		return fmt.Errorf("decode notification outbox event: %w", err)
	}
	defer func() {
		if resultErr == nil || source.NotificationOutboxID == "" {
			return
		}
		if err := p.outbox.MarkNotificationOutboxFailed(
			ctx,
			source.NotificationOutboxID,
			time.Now().UTC().Add(time.Second),
			"ERR_NOTIFICATION_RULE_PROCESSING_FAILED",
			notificationsanitize.Text(resultErr.Error(), 500),
		); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("mark notification outbox failed: %w", err))
		}
	}()
	if source.NotificationOutboxID == "" || source.RecipientUserID == "" || source.AggregateID == "" || source.OccurredAt.IsZero() {
		return fmt.Errorf("notification outbox event is missing identity or routing fields")
	}
	eventType := map[string]string{
		iapiserver.NotificationEventCreated:            iapiserver.UserEventNotificationCreated,
		iapiserver.NotificationEventUpdated:            iapiserver.UserEventNotificationUpdated,
		iapiserver.NotificationEventDeleted:            iapiserver.UserEventNotificationDeleted,
		iapiserver.NotificationEventUnreadCountChanged: iapiserver.UserEventNotificationUnreadCountChanged,
	}[topic]
	if eventType == "" {
		return fmt.Errorf("unsupported notification outbox topic %s", topic)
	}
	if topic == iapiserver.NotificationEventUnreadCountChanged {
		if source.AggregateType != "notification_recipient_counter" {
			return fmt.Errorf("notification counter outbox aggregate type is invalid")
		}
	} else if source.AggregateType != "notification" || source.NotificationID == "" {
		return fmt.Errorf("notification outbox aggregate identity is invalid")
	}
	if topic == iapiserver.NotificationEventDeleted && source.DeletedAt.IsZero() {
		return fmt.Errorf("notification deleted outbox lacks deleted_at")
	}
	if topic == iapiserver.NotificationEventUpdated && len(source.ChangedFields) == 0 {
		return fmt.Errorf("notification updated outbox lacks changed_fields")
	}
	version := source.ResourceVersion
	if topic == iapiserver.NotificationEventUnreadCountChanged {
		version = source.CounterVersion
	}
	if version < 1 || source.AggregateVersion != version {
		return fmt.Errorf("notification outbox versions are inconsistent")
	}
	payload := notificationUserEventPayload(topic, source)
	event := &iapiserver.UserEvent{
		RecipientUserID: source.RecipientUserID, EventType: eventType, EventVersion: 1,
		AggregateType: source.AggregateType, AggregateID: source.AggregateID, AggregateVersion: version,
		NotificationID: source.NotificationID, Payload: payload,
		SourceDomain: iapiserver.SSESourceDomainNotificationCenter, SourceEventID: source.NotificationOutboxID,
		OccurredAt: source.OccurredAt, ExpiresAt: imachinery.NewTime(source.OccurredAt.Add(p.retention)),
	}
	event.ID, event.Name = uuid.NewString(), eventType
	if _, _, err := p.events.AddIdempotent(ctx, event); err != nil {
		return fmt.Errorf("persist notification UserEvent: %w", err)
	}
	if err := p.outbox.MarkNotificationOutboxPublished(ctx, source.NotificationOutboxID, time.Now().UTC()); err != nil {
		return fmt.Errorf("confirm notification outbox projection: %w", err)
	}
	return nil
}

func notificationUserEventPayload(topic string, source notificationOutboxEvent) map[string]any {
	switch topic {
	case iapiserver.NotificationEventCreated:
		return map[string]any{
			"notification_id": source.NotificationID, "notification_topic": source.NotificationTopic,
			"category": source.Category, "severity": source.Severity, "inbox_status": source.InboxStatus,
			"attention_status": source.AttentionStatus, "source_type": source.SourceType, "source_id": source.SourceID,
			"navigation_available": source.NavigationAvailable, "resource_version": source.ResourceVersion,
			"occurred_at": source.OccurredAt,
		}
	case iapiserver.NotificationEventUpdated:
		return map[string]any{
			"notification_id": source.NotificationID, "notification_topic": source.NotificationTopic,
			"category": source.Category, "severity": source.Severity, "inbox_status": source.InboxStatus,
			"attention_status": source.AttentionStatus, "occurrence_count": source.OccurrenceCount,
			"navigation_available": source.NavigationAvailable, "changed_fields": source.ChangedFields,
			"resource_version": source.ResourceVersion, "occurred_at": source.OccurredAt,
		}
	case iapiserver.NotificationEventDeleted:
		return map[string]any{
			"notification_id": source.NotificationID, "resource_version": source.ResourceVersion,
			"deleted_at": source.DeletedAt, "occurred_at": source.OccurredAt,
		}
	default:
		return map[string]any{
			"unread_count": source.UnreadCount, "critical_count": source.CriticalCount,
			"action_required_count": source.ActionRequiredCount,
		}
	}
}

// Close 取消四条订阅并等待当前投影完成。
func (p *NotificationProjector) Close() {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
}
