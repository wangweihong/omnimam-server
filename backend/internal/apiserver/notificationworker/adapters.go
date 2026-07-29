package notificationworker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/notificationsanitize"
)

const candidateRetention = 90 * 24 * time.Hour

type safeSourceError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type atomicTaskSourceEvent struct {
	SourceDomain     string          `json:"source_domain"`
	SourceEventID    string          `json:"source_event_id"`
	AtomicTaskID     string          `json:"atomic_task_id"`
	OwnerType        *string         `json:"owner_type"`
	OwnerID          *string         `json:"owner_id"`
	ApplicationRunID *string         `json:"application_run_id"`
	CanvasRunID      *string         `json:"canvas_run_id"`
	FromStatus       *string         `json:"from_status"`
	ToStatus         string          `json:"to_status"`
	Status           string          `json:"status"`
	ResourceVersion  int64           `json:"resource_version"`
	ProjectID        string          `json:"project_id"`
	Namespace        string          `json:"namespace"`
	CreatedBy        string          `json:"created_by"`
	OccurredAt       imachinery.Time `json:"occurred_at"`
	LastError        safeSourceError `json:"last_error"`
}

type canvasWarning struct {
	Code string `json:"code"`
}
type canvasRunSourceEvent struct {
	SourceDomain     string          `json:"source_domain"`
	SourceEventID    string          `json:"source_event_id"`
	CanvasRunID      string          `json:"canvas_run_id"`
	CanvasID         string          `json:"canvas_id"`
	FromStatus       string          `json:"from_status"`
	Status           string          `json:"status"`
	AggregateVersion int64           `json:"aggregate_version"`
	ProjectID        string          `json:"project_id"`
	Namespace        string          `json:"namespace"`
	CreatedBy        string          `json:"created_by"`
	OccurredAt       imachinery.Time `json:"occurred_at"`
	LastError        safeSourceError `json:"last_error"`
	Warnings         []canvasWarning `json:"warnings"`
}

type AtomicTaskSourceAdapter struct{}

func (AtomicTaskSourceAdapter) Normalize(_ context.Context, raw []byte) ([]*iapiserver.NotificationEvent, error) {
	var source atomicTaskSourceEvent
	if err := json.Unmarshal(raw, &source); err != nil {
		return nil, fmt.Errorf("decode atomic task notification source: %w", err)
	}
	if source.SourceDomain != iapiserver.SSESourceDomainTaskCenter || source.SourceEventID == "" || source.AtomicTaskID == "" || source.ResourceVersion < 1 || source.OccurredAt.IsZero() {
		return nil, fmt.Errorf("atomic task notification source is incomplete")
	}
	status := source.ToStatus
	if status == "" {
		status = source.Status
	}
	topic := map[string]string{iapiserver.AtomicTaskStatusSuccess: "task.atomic_task.succeeded", iapiserver.AtomicTaskStatusFailed: "task.atomic_task.failed", iapiserver.AtomicTaskStatusTimeout: "task.atomic_task.timed_out", iapiserver.AtomicTaskStatusBlocked: "task.atomic_task.action_required"}[status]
	if topic == "" {
		return []*iapiserver.NotificationEvent{}, nil
	}
	event := baseCandidate(source.SourceDomain, "atomic_task_status_changed", source.SourceEventID, "atomic_task", source.AtomicTaskID, source.ResourceVersion, topic, source.CreatedBy, source.ProjectID, source.Namespace, source.OccurredAt)
	event.PayloadSnapshot = iapiserver.NotificationPayloadSnapshot{AtomicTaskID: source.AtomicTaskID, OwnerType: dereference(source.OwnerType), OwnerID: dereference(source.OwnerID), ApplicationRunID: dereference(source.ApplicationRunID), CanvasRunID: dereference(source.CanvasRunID), FromStatus: dereference(source.FromStatus), Status: status, ErrorCode: safeText(source.LastError.Code, 128), ErrorSummary: safeText(source.LastError.Message, 500), Retryable: source.LastError.Retryable}
	return []*iapiserver.NotificationEvent{event}, nil
}

type CanvasRunSourceAdapter struct{}

func (CanvasRunSourceAdapter) Normalize(_ context.Context, raw []byte) ([]*iapiserver.NotificationEvent, error) {
	var source canvasRunSourceEvent
	if err := json.Unmarshal(raw, &source); err != nil {
		return nil, fmt.Errorf("decode canvas run notification source: %w", err)
	}
	if source.SourceDomain != iapiserver.SSESourceDomainWorkflowCanvas || source.SourceEventID == "" || source.CanvasRunID == "" || source.CanvasID == "" || source.AggregateVersion < 1 || source.OccurredAt.IsZero() {
		return nil, fmt.Errorf("canvas run notification source is incomplete")
	}
	topic := map[string]string{iapiserver.CanvasRunStatusSuccess: "canvas.run.succeeded", iapiserver.CanvasRunStatusPartialSuccess: "canvas.run.partially_succeeded", iapiserver.CanvasRunStatusFailed: "canvas.run.failed", iapiserver.CanvasRunStatusTimeout: "canvas.run.failed"}[source.Status]
	if topic == "" {
		return []*iapiserver.NotificationEvent{}, nil
	}
	event := baseCandidate(source.SourceDomain, "canvas_run_status_changed", source.SourceEventID, "canvas_run", source.CanvasRunID, source.AggregateVersion, topic, source.CreatedBy, source.ProjectID, source.Namespace, source.OccurredAt)
	warnings := make([]string, 0, len(source.Warnings))
	for _, warning := range source.Warnings {
		if value := safeText(warning.Code, 128); value != "" {
			warnings = append(warnings, value)
		}
	}
	event.PayloadSnapshot = iapiserver.NotificationPayloadSnapshot{CanvasRunID: source.CanvasRunID, CanvasID: source.CanvasID, FromStatus: safeText(source.FromStatus, 32), Status: source.Status, ErrorCode: safeText(source.LastError.Code, 128), ErrorSummary: safeText(source.LastError.Message, 500), Retryable: source.LastError.Retryable, WarningCodes: warnings}
	return []*iapiserver.NotificationEvent{event}, nil
}

func baseCandidate(domain, eventType, eventID, sourceType, sourceID string, version int64, topic, createdBy, projectID, namespace string, occurredAt imachinery.Time) *iapiserver.NotificationEvent {
	event := &iapiserver.NotificationEvent{SourceDomain: domain, SourceEventType: eventType, SourceEventID: eventID, SourceAggregateType: sourceType, SourceAggregateID: sourceID, SourceAggregateVersion: version, NotificationTopic: topic, SourceType: sourceType, SourceID: sourceID, RecipientBasis: iapiserver.NotificationRecipientBasis{CreatedBy: createdBy, ProjectID: projectID, Namespace: namespace}, OccurredAt: occurredAt, ProcessingStatus: iapiserver.NotificationEventPending, DeduplicationKey: domain + ":" + eventID + ":" + topic, RuleVersion: 1, ExpiresAt: imachinery.NewTime(occurredAt.Add(candidateRetention))}
	event.Name = eventID + ":" + topic
	return event
}

func dereference(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func safeText(value string, limit int) string {
	return notificationsanitize.Text(value, limit)
}
