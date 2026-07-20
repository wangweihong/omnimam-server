package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

const (
	topicAtomicTaskCreated        = "atomic_task_created"
	topicAtomicTaskStatusChanged  = "atomic_task_status_changed"
	topicTaskAttemptStatusChanged = "task_attempt_status_changed"
	topicTaskGroupStatusChanged   = "task_group_status_changed"
)

type SubscribeFunc func(context.Context, string, string) (<-chan *message.Message, error)

// Projector 消费 Task Center 可靠 outbox，幂等写入用户事件投影；失败只重试投影，不回滚任务事实。
type Projector struct {
	store     store.UserEventStore
	retention time.Duration
	subscribe SubscribeFunc
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func NewProjector(eventStore store.UserEventStore, retention time.Duration, subscribe SubscribeFunc) *Projector {
	if retention <= 0 {
		retention = 24 * time.Hour
	}
	return &Projector{store: eventStore, retention: retention, subscribe: subscribe}
}

func (p *Projector) Start(parent context.Context) error {
	if p.store == nil || p.subscribe == nil {
		return fmt.Errorf("SSE projector dependencies are not configured")
	}
	ctx, cancel := context.WithCancel(parent)
	p.cancel = cancel
	for _, topic := range []string{topicAtomicTaskCreated, topicAtomicTaskStatusChanged, topicTaskAttemptStatusChanged, topicTaskGroupStatusChanged} {
		messages, err := p.subscribe(ctx, topic, "sse-task-center-projector")
		if err != nil {
			cancel()
			p.wg.Wait()
			return fmt.Errorf("subscribe task-center topic %s: %w", topic, err)
		}
		p.wg.Add(1)
		go p.consume(ctx, topic, messages)
	}
	return nil
}

func (p *Projector) Close() {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
}

func (p *Projector) consume(ctx context.Context, topic string, messages <-chan *message.Message) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-messages:
			if !ok {
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

type taskCenterSourceEvent struct {
	SourceDomain     string          `json:"source_domain"`
	SourceEventID    string          `json:"source_event_id"`
	SSEEventType     string          `json:"sse_event_type"`
	CreatedBy        string          `json:"created_by"`
	ResourceVersion  int64           `json:"resource_version"`
	OccurredAt       imachinery.Time `json:"occurred_at"`
	CorrelationID    string          `json:"correlation_id"`
	ApplicationRunID string          `json:"application_run_id"`
	AtomicTaskID     string          `json:"atomic_task_id"`
	TaskAttemptID    string          `json:"task_attempt_id"`
	OwnerType        string          `json:"owner_type"`
	OwnerID          string          `json:"owner_id"`
	GroupType        string          `json:"group_type"`
	GroupID          string          `json:"group_id"`
	AggregateType    string          `json:"aggregate_type"`
	Payload          map[string]any  `json:"-"`
}

func (p *Projector) project(ctx context.Context, topic string, raw []byte) error {
	var source taskCenterSourceEvent
	if err := json.Unmarshal(raw, &source); err != nil {
		return fmt.Errorf("decode task-center source event: %w", err)
	}
	if err := json.Unmarshal(raw, &source.Payload); err != nil {
		return fmt.Errorf("decode task-center source payload: %w", err)
	}
	if source.SourceDomain != iapiserver.SSESourceDomainTaskCenter || source.SourceEventID == "" || source.CreatedBy == "" || source.ResourceVersion < 1 || source.OccurredAt.IsZero() {
		return fmt.Errorf("task-center source event is missing routing or version fields")
	}
	event := &iapiserver.UserEvent{
		RecipientUserID: source.CreatedBy, EventVersion: 1, AggregateVersion: source.ResourceVersion,
		CorrelationID: source.CorrelationID, ApplicationRunID: source.ApplicationRunID,
		SourceDomain: source.SourceDomain, SourceEventID: source.SourceEventID, OccurredAt: source.OccurredAt,
		ExpiresAt: imachinery.NewTime(source.OccurredAt.Add(p.retention)), Payload: sanitizeSourcePayload(source.Payload),
	}
	switch topic {
	case topicAtomicTaskCreated:
		event.EventType, event.AggregateType, event.AggregateID = iapiserver.UserEventAtomicTaskCreated, "atomic_task", source.AtomicTaskID
		event.AtomicTaskID = source.AtomicTaskID
	case topicAtomicTaskStatusChanged:
		event.EventType, event.AggregateType, event.AggregateID = source.SSEEventType, "atomic_task", source.AtomicTaskID
		event.AtomicTaskID = source.AtomicTaskID
	case topicTaskAttemptStatusChanged:
		event.EventType, event.AggregateType, event.AggregateID = source.SSEEventType, "task_attempt", source.TaskAttemptID
		event.AtomicTaskID, event.TaskAttemptID = source.AtomicTaskID, source.TaskAttemptID
	case topicTaskGroupStatusChanged:
		event.EventType, event.AggregateType, event.AggregateID = source.SSEEventType, source.AggregateType, source.GroupID
		if source.GroupType == iapiserver.TaskOwnerTypeDAGGroup {
			event.DAGTaskGroupID = source.GroupID
		} else {
			event.TaskGroupID = source.GroupID
		}
	default:
		return fmt.Errorf("unsupported task-center source topic %s", topic)
	}
	if event.EventType == "" || event.AggregateType == "" || event.AggregateID == "" {
		return fmt.Errorf("task-center source event cannot be mapped")
	}
	if source.OwnerType == iapiserver.TaskOwnerTypeGroup {
		event.TaskGroupID = source.OwnerID
	}
	if source.OwnerType == iapiserver.TaskOwnerTypeDAGGroup {
		event.DAGTaskGroupID = source.OwnerID
	}
	event.ID = uuid.NewString()
	event.Name = event.EventType
	_, _, err := p.store.AddIdempotent(ctx, event)
	return err
}

func sanitizeSourcePayload(payload map[string]any) map[string]any {
	result := make(map[string]any, len(payload))
	for key, value := range payload {
		switch key {
		case "source_domain", "source_event_id", "sse_event_type", "created_by", "project_id", "namespace", "correlation_id", "resource_version":
			continue
		default:
			result[key] = value
		}
	}
	return result
}
