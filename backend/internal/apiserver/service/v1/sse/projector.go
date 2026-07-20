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
	topicAtomicTaskCreated             = "atomic_task_created"
	topicAtomicTaskStatusChanged       = "atomic_task_status_changed"
	topicTaskAttemptStatusChanged      = "task_attempt_status_changed"
	topicTaskGroupStatusChanged        = "task_group_status_changed"
	topicArtifactCreated               = "artifact_created"
	topicArtifactProcessingChanged     = "artifact_processing_changed"
	topicArtifactRegistrationChanged   = "artifact_registration_changed"
	topicAssetVersionProcessingChanged = "asset_version_processing_changed"
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
	for _, topic := range []string{
		topicAtomicTaskCreated, topicAtomicTaskStatusChanged, topicTaskAttemptStatusChanged, topicTaskGroupStatusChanged,
		topicArtifactCreated, topicArtifactProcessingChanged, topicArtifactRegistrationChanged, topicAssetVersionProcessingChanged,
	} {
		consumerGroup := "sse-task-center-projector"
		if isAssetLibraryTopic(topic) {
			consumerGroup = "sse-asset-library-projector"
		}
		messages, err := p.subscribe(ctx, topic, consumerGroup)
		if err != nil {
			cancel()
			p.wg.Wait()
			return fmt.Errorf("subscribe SSE source topic %s: %w", topic, err)
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

type sourceEvent struct {
	SourceDomain       string          `json:"source_domain"`
	SourceEventID      string          `json:"source_event_id"`
	SSEEventType       string          `json:"sse_event_type"`
	CreatedBy          string          `json:"created_by"`
	ResourceVersion    int64           `json:"resource_version"`
	OccurredAt         imachinery.Time `json:"occurred_at"`
	CorrelationID      string          `json:"correlation_id"`
	ApplicationRunID   string          `json:"application_run_id"`
	AtomicTaskID       string          `json:"atomic_task_id"`
	TaskAttemptID      string          `json:"task_attempt_id"`
	OwnerType          string          `json:"owner_type"`
	OwnerID            string          `json:"owner_id"`
	GroupType          string          `json:"group_type"`
	GroupID            string          `json:"group_id"`
	TaskGroupID        string          `json:"task_group_id"`
	AggregateType      string          `json:"aggregate_type"`
	OwnerUserID        string          `json:"owner_user_id"`
	ArtifactID         string          `json:"artifact_id"`
	AssetID            string          `json:"asset_id"`
	AssetVersionID     string          `json:"asset_version_id"`
	ChangeType         string          `json:"change_type"`
	RegistrationStatus string          `json:"registration_status"`
	Status             string          `json:"status"`
	Payload            map[string]any  `json:"-"`
}

func (p *Projector) project(ctx context.Context, topic string, raw []byte) error {
	var source sourceEvent
	if err := json.Unmarshal(raw, &source); err != nil {
		return fmt.Errorf("decode task-center source event: %w", err)
	}
	if err := json.Unmarshal(raw, &source.Payload); err != nil {
		return fmt.Errorf("decode task-center source payload: %w", err)
	}
	recipient := source.CreatedBy
	expectedDomain := iapiserver.SSESourceDomainTaskCenter
	if isAssetLibraryTopic(topic) {
		recipient = source.OwnerUserID
		expectedDomain = iapiserver.SSESourceDomainAssetLibrary
	}
	if source.SourceDomain != expectedDomain || source.SourceEventID == "" || recipient == "" || source.ResourceVersion < 1 || source.OccurredAt.IsZero() {
		return fmt.Errorf("SSE source event is missing routing or version fields")
	}
	event := &iapiserver.UserEvent{
		RecipientUserID: recipient, EventVersion: 1, AggregateVersion: source.ResourceVersion,
		CorrelationID: source.CorrelationID, ApplicationRunID: source.ApplicationRunID,
		SourceDomain: source.SourceDomain, SourceEventID: source.SourceEventID, OccurredAt: source.OccurredAt,
		ExpiresAt: imachinery.NewTime(source.OccurredAt.Add(p.retention)), Payload: sanitizeSourcePayload(topic, source.Payload),
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
	case topicArtifactCreated:
		event.EventType, event.AggregateType, event.AggregateID = iapiserver.UserEventArtifactCreated, "artifact", source.ArtifactID
		event.ArtifactID, event.AssetID, event.AssetVersionID = source.ArtifactID, source.AssetID, source.AssetVersionID
	case topicArtifactProcessingChanged:
		event.EventType, event.AggregateType, event.AggregateID = artifactProcessingEventType(source.ChangeType), "artifact", source.ArtifactID
		event.ArtifactID, event.AssetID, event.AssetVersionID = source.ArtifactID, source.AssetID, source.AssetVersionID
	case topicArtifactRegistrationChanged:
		event.EventType, event.AggregateType, event.AggregateID = artifactRegistrationEventType(source.RegistrationStatus), "artifact", source.ArtifactID
		event.ArtifactID, event.AssetID, event.AssetVersionID = source.ArtifactID, source.AssetID, source.AssetVersionID
	case topicAssetVersionProcessingChanged:
		event.EventType, event.AggregateType, event.AggregateID = assetVersionProcessingEventType(source.Status, source.ChangeType, source.ResourceVersion), "asset_version", source.AssetVersionID
		event.AtomicTaskID, event.TaskGroupID = source.AtomicTaskID, source.TaskGroupID
		event.AssetID, event.AssetVersionID = source.AssetID, source.AssetVersionID
	default:
		return fmt.Errorf("unsupported SSE source topic %s", topic)
	}
	if event.EventType == "" || event.AggregateType == "" || event.AggregateID == "" {
		return fmt.Errorf("SSE source event cannot be mapped")
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

func sanitizeSourcePayload(topic string, payload map[string]any) map[string]any {
	result := make(map[string]any, len(payload))
	for key, value := range payload {
		switch key {
		case "source_domain", "source_event_id", "sse_event_type", "created_by", "owner_user_id", "project_id", "namespace", "correlation_id", "resource_version", "change_type":
			continue
		default:
			result[key] = value
		}
	}
	if topic == topicArtifactProcessingChanged {
		result["processing_progress"] = result["progress"]
		result["processing_retryable"] = result["retryable"]
		delete(result, "progress")
		delete(result, "retryable")
		delete(result, "error_code")
	}
	if topic == topicArtifactRegistrationChanged {
		result["registration_retryable"] = result["retryable"]
		delete(result, "retryable")
		delete(result, "error_code")
	}
	if topic == topicArtifactCreated || topic == topicArtifactProcessingChanged || topic == topicArtifactRegistrationChanged {
		return allowPayloadFields(result, "artifact_id", "application_run_id", "atomic_task_id", "output_key", "media_type",
			"processing_status", "processing_progress", "processing_phase", "preview_available", "preview_ref", "thumbnail_ref",
			"size_bytes", "processing_error_code", "processing_retryable", "registration_status", "asset_id",
			"registration_error_code", "registration_retryable", "ready_at", "occurred_at")
	}
	if topic == topicAssetVersionProcessingChanged {
		return allowPayloadFields(result, "asset_id", "asset_version_id", "status", "expected_count", "completed_count",
			"failed_count", "task_group_id", "atomic_task_id", "error_code", "occurred_at")
	}
	return result
}

func allowPayloadFields(payload map[string]any, fields ...string) map[string]any {
	result := make(map[string]any, len(fields))
	for _, field := range fields {
		if value, exists := payload[field]; exists {
			result[field] = value
		}
	}
	return result
}

func isAssetLibraryTopic(topic string) bool {
	switch topic {
	case topicArtifactCreated, topicArtifactProcessingChanged, topicArtifactRegistrationChanged, topicAssetVersionProcessingChanged:
		return true
	default:
		return false
	}
}

func artifactProcessingEventType(changeType string) string {
	return map[string]string{
		"transferring":  iapiserver.UserEventArtifactTransferring,
		"processing":    iapiserver.UserEventArtifactProcessing,
		"preview_ready": iapiserver.UserEventArtifactPreviewReady,
		"ready":         iapiserver.UserEventArtifactReady,
		"failed":        iapiserver.UserEventArtifactProcessingFailed,
		"deleted":       iapiserver.UserEventArtifactDeleted,
	}[changeType]
}

func artifactRegistrationEventType(status string) string {
	return map[string]string{
		"registered": iapiserver.UserEventArtifactRegistrationSucceeded,
		"failed":     iapiserver.UserEventArtifactRegistrationFailed,
	}[status]
}

func assetVersionProcessingEventType(status, changeType string, resourceVersion int64) string {
	switch status {
	case "processing":
		if changeType == "started" || (changeType == "" && resourceVersion == 1) {
			return iapiserver.UserEventAssetVersionProcessingStarted
		}
		return iapiserver.UserEventAssetVersionProcessingProgressed
	case "ready":
		return iapiserver.UserEventAssetVersionReady
	case "ready_with_warnings":
		return iapiserver.UserEventAssetVersionReadyWithWarnings
	case "failed":
		return iapiserver.UserEventAssetVersionProcessingFailed
	default:
		return ""
	}
}
