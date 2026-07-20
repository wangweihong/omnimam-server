package postgresql

import (
	"fmt"
	"reflect"

	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

func projectAtomicTaskCreated(tx *gorm.DB, task *iapiserver.AtomicTask) error {
	payload := atomicTaskPayload(nil, task)
	sourceID := task.ID + ":created"
	payload["source_event_id"] = sourceID
	payload["source_domain"] = iapiserver.SSESourceDomainTaskCenter
	return publishOutbox(tx, OutboxTopicAtomicTaskCreated, sourceID, payload)
}

func projectAtomicTaskChanged(tx *gorm.DB, previous, task *iapiserver.AtomicTask) error {
	eventType := atomicTaskEventType(task.Status)
	if previous.Status == task.Status {
		if previous.Progress == task.Progress && previous.CurrentAttempt == task.CurrentAttempt && reflect.DeepEqual(previous.Output, task.Output) && reflect.DeepEqual(previous.LastError, task.LastError) {
			return nil
		}
		eventType = iapiserver.UserEventAtomicTaskProgressed
	}
	if eventType == "" {
		return nil
	}
	payload := atomicTaskPayload(previous, task)
	sourceID := fmt.Sprintf("%s:%d", task.ID, task.ResourceVersion)
	payload["source_event_id"] = sourceID
	payload["source_domain"] = iapiserver.SSESourceDomainTaskCenter
	payload["sse_event_type"] = eventType
	return publishOutbox(tx, OutboxTopicAtomicTaskStatusChanged, sourceID, payload)
}

func projectTaskAttemptChanged(tx *gorm.DB, task *iapiserver.AtomicTask, previous *iapiserver.TaskAttempt, attempt *iapiserver.TaskAttempt) error {
	eventType := taskAttemptEventType(attempt.Status)
	if previous == nil && attempt.Status == iapiserver.TaskAttemptStatusScheduled {
		eventType = iapiserver.UserEventTaskAttemptCreated
	}
	if eventType == "" || (previous != nil && previous.Status == attempt.Status && previous.ResourceVersion == attempt.ResourceVersion) {
		return nil
	}
	fromStatus := ""
	if previous != nil {
		fromStatus = previous.Status
	}
	payload := map[string]any{
		"task_attempt_id": attempt.ID, "atomic_task_id": task.ID, "attempt_no": attempt.AttemptNo,
		"from_status": nullableString(fromStatus), "to_status": attempt.Status, "status": attempt.Status,
		"error_code": nullableString(attempt.Error.Code), "retryable": attempt.Retryable,
		"started_at": nullableTime(attempt.StartedAt), "finished_at": nullableTime(attempt.CompletedAt),
		"resource_version": attempt.ResourceVersion, "atomic_task_resource_version": task.ResourceVersion,
		"project_id": task.ProjectID, "namespace": task.Namespace, "created_by": task.CreatedBy,
		"correlation_id": task.ID, "occurred_at": attempt.UpdatedAt,
	}
	sourceID := fmt.Sprintf("%s:%d", attempt.ID, attempt.ResourceVersion)
	payload["source_event_id"] = sourceID
	payload["source_domain"] = iapiserver.SSESourceDomainTaskCenter
	payload["sse_event_type"] = eventType
	payload["application_run_id"] = nullableString(task.ApplicationRunID)
	payload["owner_type"] = nullableString(task.OwnerType)
	payload["owner_id"] = nullableString(task.OwnerID)
	return publishOutbox(tx, OutboxTopicTaskAttemptStatusChanged, sourceID, payload)
}

func projectTaskGroupCreated(tx *gorm.DB, groupType string, group any) error {
	return projectTaskGroupChanged(tx, groupType, nil, group, true)
}

func projectTaskGroupChanged(tx *gorm.DB, groupType string, previous, current any, created bool) error {
	view := taskGroupEventView(current)
	var previousView groupEventView
	if previous != nil {
		previousView = taskGroupEventView(previous)
	}
	eventType := taskGroupEventType(groupType, view.Status, created)
	if !created && previousView.Status == view.Status && previousView.Progress == view.Progress && reflect.DeepEqual(previousView.Summary, view.Summary) {
		return nil
	}
	if !created && previousView.Status == view.Status {
		eventType = groupProgressEventType(groupType)
	}
	payload := map[string]any{
		"group_type": groupType, "group_id": view.ID, "from_status": nullableString(previousView.Status), "to_status": view.Status,
		"status": view.Status, "progress": view.Progress, "total": view.Summary.Total, "pending": view.Summary.Pending,
		"blocked": view.Summary.Blocked, "running": view.Summary.Running, "success": view.Summary.Success,
		"failed": view.Summary.Failed, "canceled": view.Summary.Canceled, "skipped": view.Summary.Skipped,
		"summary": view.Summary, "resource_version": view.Version, "project_id": view.ProjectID,
		"namespace": view.Namespace, "created_by": view.CreatedBy, "correlation_id": view.ID, "occurred_at": view.OccurredAt,
	}
	sourceID := fmt.Sprintf("%s:%s:%d", groupType, view.ID, view.Version)
	payload["source_event_id"] = sourceID
	payload["source_domain"] = iapiserver.SSESourceDomainTaskCenter
	payload["sse_event_type"] = eventType
	payload["aggregate_type"] = groupAggregateType(groupType)
	return publishOutbox(tx, OutboxTopicTaskGroupStatusChanged, sourceID, payload)
}

func atomicTaskPayload(previous, task *iapiserver.AtomicTask) map[string]any {
	fromStatus := ""
	if previous != nil {
		fromStatus = previous.Status
	}
	return map[string]any{
		"atomic_task_id": task.ID, "function_ref": task.FunctionRef, "owner_type": nullableString(task.OwnerType), "owner_id": nullableString(task.OwnerID),
		"root_task_id": task.RootTaskID, "application_run_id": nullableString(task.ApplicationRunID), "canvas_run_id": nullableString(task.CanvasRunID),
		"from_status": nullableString(fromStatus), "to_status": task.Status, "status": task.Status, "progress": task.Progress,
		"current_attempt": task.CurrentAttempt, "output_summary": summarizeTaskOutput(task.Output), "error_code": nullableString(task.LastError.Code),
		"retryable": task.LastError.Retryable, "resource_version": task.ResourceVersion, "project_id": task.ProjectID,
		"namespace": task.Namespace, "created_by": task.CreatedBy, "correlation_id": task.ID, "occurred_at": task.UpdatedAt,
	}
}

func summarizeTaskOutput(output map[string]any) map[string]any {
	result := map[string]any{}
	for _, key := range []string{"artifact_refs", "representation_refs"} {
		if value, ok := output[key]; ok {
			result[key] = value
		}
	}
	return result
}

func atomicTaskEventType(status string) string {
	return map[string]string{
		iapiserver.AtomicTaskStatusBlocked: iapiserver.UserEventAtomicTaskBlocked, iapiserver.AtomicTaskStatusReady: iapiserver.UserEventAtomicTaskReady,
		iapiserver.AtomicTaskStatusRunning: iapiserver.UserEventAtomicTaskStarted, iapiserver.AtomicTaskStatusRetrying: iapiserver.UserEventAtomicTaskRetrying,
		iapiserver.AtomicTaskStatusCancelRequested: iapiserver.UserEventAtomicTaskCancelRequested, iapiserver.AtomicTaskStatusSuccess: iapiserver.UserEventAtomicTaskSucceeded,
		iapiserver.AtomicTaskStatusFailed: iapiserver.UserEventAtomicTaskFailed, iapiserver.AtomicTaskStatusCanceled: iapiserver.UserEventAtomicTaskCanceled,
		iapiserver.AtomicTaskStatusTimeout: iapiserver.UserEventAtomicTaskTimedOut, iapiserver.AtomicTaskStatusSkipped: iapiserver.UserEventAtomicTaskSkipped,
	}[status]
}

func taskAttemptEventType(status string) string {
	return map[string]string{
		iapiserver.TaskAttemptStatusRunning: iapiserver.UserEventTaskAttemptStarted, iapiserver.TaskAttemptStatusSuccess: iapiserver.UserEventTaskAttemptSucceeded,
		iapiserver.TaskAttemptStatusFailed: iapiserver.UserEventTaskAttemptFailed, iapiserver.TaskAttemptStatusCanceled: iapiserver.UserEventTaskAttemptCanceled,
		iapiserver.TaskAttemptStatusTimeout: iapiserver.UserEventTaskAttemptTimedOut,
	}[status]
}

type groupEventView struct {
	ID, Status, ProjectID, Namespace, CreatedBy string
	Progress                                    float64
	Summary                                     iapiserver.TaskSummary
	Version                                     int64
	OccurredAt                                  imachinery.Time
}

func taskGroupEventView(value any) groupEventView {
	switch group := value.(type) {
	case *iapiserver.TaskGroup:
		return groupEventView{group.ID, group.Status, group.ProjectID, group.Namespace, group.CreatedBy, group.Progress, group.Summary, group.ResourceVersion, group.UpdatedAt}
	case *iapiserver.DAGTaskGroup:
		return groupEventView{group.ID, group.Status, group.ProjectID, group.Namespace, group.CreatedBy, group.Progress, group.Summary, group.ResourceVersion, group.UpdatedAt}
	default:
		return groupEventView{}
	}
}

func taskGroupEventType(groupType, status string, created bool) string {
	if created {
		if groupType == iapiserver.TaskOwnerTypeDAGGroup {
			return iapiserver.UserEventDAGTaskGroupCreated
		}
		return iapiserver.UserEventTaskGroupCreated
	}
	prefixDAG := groupType == iapiserver.TaskOwnerTypeDAGGroup
	switch status {
	case iapiserver.TaskGroupStatusRunning:
		if prefixDAG {
			return iapiserver.UserEventDAGTaskGroupStarted
		}
		return iapiserver.UserEventTaskGroupStarted
	case iapiserver.TaskGroupStatusSuccess:
		if prefixDAG {
			return iapiserver.UserEventDAGTaskGroupSucceeded
		}
		return iapiserver.UserEventTaskGroupSucceeded
	case iapiserver.TaskGroupStatusFailed, iapiserver.TaskGroupStatusTimeout:
		if prefixDAG {
			return iapiserver.UserEventDAGTaskGroupFailed
		}
		return iapiserver.UserEventTaskGroupFailed
	case iapiserver.TaskGroupStatusCanceled:
		if prefixDAG {
			return iapiserver.UserEventDAGTaskGroupCanceled
		}
		return iapiserver.UserEventTaskGroupCanceled
	default:
		return groupProgressEventType(groupType)
	}
}

func groupProgressEventType(groupType string) string {
	if groupType == iapiserver.TaskOwnerTypeDAGGroup {
		return iapiserver.UserEventDAGTaskGroupProgressed
	}
	return iapiserver.UserEventTaskGroupProgressed
}

func groupAggregateType(groupType string) string {
	if groupType == iapiserver.TaskOwnerTypeDAGGroup {
		return "dag_task_group"
	}
	return "task_group"
}

func ownerID(task *iapiserver.AtomicTask, ownerType string) string {
	if task.OwnerType == ownerType {
		return task.OwnerID
	}
	return ""
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func nullableTime(value imachinery.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
