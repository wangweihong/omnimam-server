package taskcenter

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

var dagEventTypes = map[string]bool{
	"NODE_CREATED": true, "NODE_BLOCKED": true, "NODE_READY": true,
	"ATTEMPT_SCHEDULED": true, "ATTEMPT_STARTED": true, "PROGRESS_UPDATED": true,
	"RETRY_SCHEDULED": true, "OUTPUT_AVAILABLE": true, "NODE_SUCCEEDED": true,
	"NODE_FAILED": true, "NODE_CANCELED": true, "NODE_TIMED_OUT": true, "NODE_SKIPPED": true,
}

// GetDAGTaskGroupDetail 返回触发快照、运行时间和声明节点级确定性执行聚合。
func (s *taskCenterService) GetDAGTaskGroupDetail(ctx context.Context, id string) (*iapiserver.DAGTaskGroupDetail, error) {
	group, err := s.GetDAGTaskGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	tasks, attempts, _, err := s.dagObservationData(ctx, group)
	if err != nil {
		return nil, err
	}
	if err := s.attachAtomicTaskRelations(ctx, tasks); err != nil {
		return nil, err
	}
	if group.Result == nil {
		group.Result = make(map[string]any)
	}
	for _, task := range tasks {
		if task != nil && task.ChildKey != "" && len(task.Output) > 0 {
			group.Result[task.ChildKey] = task.Output
		}
	}
	byNode := groupTasksByNode(tasks)
	attemptsByTask := groupAttemptsByTask(attempts)
	nodes := make([]*iapiserver.DAGNodeExecutionSummary, 0, len(group.Nodes))
	for _, node := range group.Nodes {
		nodes = append(nodes, aggregateDAGNode(node, byNode[node.Key], attemptsByTask))
	}
	triggeredAt := group.TriggeredAt
	if triggeredAt.IsZero() {
		triggeredAt = group.CreatedAt
	}
	triggerType := group.TriggerType
	if triggerType == "" {
		triggerType = iapiserver.DAGTriggerAPI
	}
	return &iapiserver.DAGTaskGroupDetail{
		DAGTaskGroup: group, StartedAt: optionalTaskTime(group.StartedAt), CompletedAt: optionalTaskTime(group.CompletedAt),
		TriggerSummary: iapiserver.DAGTriggerSummary{Type: triggerType, SourceID: optionalString(group.TriggerSourceID), SourceName: optionalString(group.TriggerSourceName), TriggeredAt: triggeredAt},
		ExecutionNodes: nodes,
	}, nil
}

// ListDAGTaskGroupEvents 返回规范化白名单事件，不透传 runtime_projection_events.payload_json。
func (s *taskCenterService) ListDAGTaskGroupEvents(ctx context.Context, id string, req *iapiserver.DAGExecutionEventListRequest) (*iapiserver.DAGExecutionEventListResponse, error) {
	group, err := s.GetDAGTaskGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	tasks, attempts, projections, err := s.dagObservationData(ctx, group)
	if err != nil {
		return nil, err
	}
	events := buildDAGEvents(tasks, attempts, projections)
	allowed, err := parseDAGEventTypes(req.EventTypes)
	if err != nil {
		return nil, errors.NewStatus(code.ErrValidation, err.Error())
	}
	after, err := parseOptionalTime(req.OccurredAfter)
	if err != nil {
		return nil, errors.NewStatus(code.ErrValidation, "occurred_after is invalid")
	}
	before, err := parseOptionalTime(req.OccurredBefore)
	if err != nil {
		return nil, errors.NewStatus(code.ErrValidation, "occurred_before is invalid")
	}
	if after != nil && before != nil && after.After(*before) {
		return nil, errors.NewStatus(code.ErrValidation, "occurred_after must not be later than occurred_before")
	}
	filtered := events[:0]
	for _, event := range events {
		if req.NodeKey != "" && event.NodeKey != req.NodeKey || req.AtomicTaskID != "" && stringValue(event.AtomicTaskID) != req.AtomicTaskID || req.TaskAttemptID != "" && stringValue(event.TaskAttemptID) != req.TaskAttemptID || req.AttemptNo > 0 && intValue(event.AttemptNo) != req.AttemptNo {
			continue
		}
		if len(allowed) > 0 && !allowed[event.EventType] || after != nil && event.OccurredAt.Time.Before(*after) || before != nil && event.OccurredAt.Time.After(*before) {
			continue
		}
		filtered = append(filtered, event)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].OccurredAt.Equal(&filtered[j].OccurredAt) {
			if req.SortOrder == "desc" {
				return filtered[i].ID > filtered[j].ID
			}
			return filtered[i].ID < filtered[j].ID
		}
		if req.SortOrder == "desc" {
			return filtered[j].OccurredAt.Time.Before(filtered[i].OccurredAt.Time)
		}
		return filtered[i].OccurredAt.Time.Before(filtered[j].OccurredAt.Time)
	})
	window, err := req.PagingParams.Normalize()
	if err != nil {
		return nil, errors.NewStatus(code.ErrValidation, err.Error())
	}
	page := imachinery.PaginateSlice(filtered, window)
	return &iapiserver.DAGExecutionEventListResponse{Total: int64(len(filtered)), Items: page}, nil
}

// ListDAGTaskGroupTimeline 按实际任务返回可证明的规范化时间段，缺少边界时标记 complete=false。
func (s *taskCenterService) ListDAGTaskGroupTimeline(ctx context.Context, id string, req *iapiserver.DAGTimelineListRequest) (*iapiserver.DAGTimelineListResponse, error) {
	group, err := s.GetDAGTaskGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	tasks, attempts, _, err := s.dagObservationData(ctx, group)
	if err != nil {
		return nil, err
	}
	attemptsByTask := groupAttemptsByTask(attempts)
	completionByNode := latestCompletionByNode(tasks)
	dependencies := dagDependencies(group.Edges)
	rows := make([]*iapiserver.DAGTimelineRow, 0, len(tasks))
	for _, task := range tasks {
		if req.NodeKey != "" && task.DAGNodeKey != req.NodeKey {
			continue
		}
		rows = append(rows, buildTimelineRow(group, task, attemptsByTask[task.ID], dependencies[task.DAGNodeKey], completionByNode))
	}
	window, err := req.PagingParams.Normalize()
	if err != nil {
		return nil, errors.NewStatus(code.ErrValidation, err.Error())
	}
	page := imachinery.PaginateSlice(rows, window)
	return &iapiserver.DAGTimelineListResponse{Total: int64(len(rows)), Items: page}, nil
}

func (s *taskCenterService) dagObservationData(ctx context.Context, group *iapiserver.DAGTaskGroup) ([]*iapiserver.AtomicTask, []*iapiserver.TaskAttempt, []*iapiserver.RuntimeProjectionEvent, error) {
	tasks, err := s.store.ListDAGObservationTasks(ctx, group.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task.DAGNodeKey == "" {
			task.DAGNodeKey = task.ChildKey
		}
		ids = append(ids, task.ID)
	}
	attempts, err := s.store.ListAttemptsByTaskIDs(ctx, ids)
	if err != nil {
		return nil, nil, nil, err
	}
	projections, err := s.store.ListRuntimeProjectionEvents(ctx, group.RuntimeExecutionID)
	return tasks, attempts, projections, err
}

func aggregateDAGNode(node iapiserver.DAGNode, tasks []*iapiserver.AtomicTask, attemptsByTask map[string][]*iapiserver.TaskAttempt) *iapiserver.DAGNodeExecutionSummary {
	result := &iapiserver.DAGNodeExecutionSummary{NodeKey: node.Key, Dynamic: node.DynamicFork, Status: iapiserver.AtomicTaskStatusPending, TaskSummary: iapiserver.TaskSummary{Total: len(tasks)}}
	if len(tasks) == 0 {
		return result
	}
	result.Status = aggregateNodeStatus(tasks)
	var progress float64
	var startedAt, completedAt imachinery.Time
	var latestError *iapiserver.TaskError
	for _, task := range tasks {
		progress += task.Progress
		incrementTaskSummary(&result.TaskSummary, task.Status)
		if !task.StartedAt.IsZero() && (startedAt.IsZero() || task.StartedAt.Before(&startedAt)) {
			startedAt = task.StartedAt
		}
		if !task.CompletedAt.IsZero() && (completedAt.IsZero() || completedAt.Before(&task.CompletedAt)) {
			completedAt = task.CompletedAt
		}
		if task.LastError.Code != "" || task.LastError.Message != "" {
			candidate := task.LastError
			if latestError == nil || latestError.OccurredAt.Before(&candidate.OccurredAt) {
				latestError = &candidate
			}
		}
		attempts := attemptsByTask[task.ID]
		result.AttemptCount += len(attempts)
		if len(attempts) > 1 {
			result.RetryCount += len(attempts) - 1
		}
		result.ArtifactCount += outputReferenceCount(task.Output, "artifact_refs")
		result.RepresentationCount += outputReferenceCount(task.Output, "representation_refs")
	}
	result.Progress = progress / float64(len(tasks))
	result.StartedAt, result.CompletedAt, result.LatestError = optionalTaskTime(startedAt), optionalTaskTime(completedAt), latestError
	if !startedAt.IsZero() && !completedAt.IsZero() && completedAt.Time.After(startedAt.Time) {
		result.DurationMS = completedAt.Time.Sub(startedAt.Time).Milliseconds()
	}
	if !node.DynamicFork {
		primary := tasks[0]
		result.PrimaryAtomicTaskID = optionalString(primary.ID)
		result.PrimaryAtomicTask = atomicTaskSummary(primary)
	}
	return result
}

func aggregateNodeStatus(tasks []*iapiserver.AtomicTask) string {
	priority := []string{iapiserver.AtomicTaskStatusRunning, iapiserver.AtomicTaskStatusRetrying, iapiserver.AtomicTaskStatusReady, iapiserver.AtomicTaskStatusBlocked, iapiserver.AtomicTaskStatusPending, iapiserver.AtomicTaskStatusCancelRequested}
	for _, status := range priority {
		for _, task := range tasks {
			if task.Status == status {
				return status
			}
		}
	}
	for _, status := range []string{iapiserver.AtomicTaskStatusFailed, iapiserver.AtomicTaskStatusTimeout, iapiserver.AtomicTaskStatusCanceled, iapiserver.AtomicTaskStatusSuccess, iapiserver.AtomicTaskStatusSkipped} {
		for _, task := range tasks {
			if task.Status == status {
				return status
			}
		}
	}
	return iapiserver.AtomicTaskStatusPending
}

func incrementTaskSummary(summary *iapiserver.TaskSummary, status string) {
	switch status {
	case iapiserver.AtomicTaskStatusPending:
		summary.Pending++
	case iapiserver.AtomicTaskStatusBlocked:
		summary.Blocked++
	case iapiserver.AtomicTaskStatusReady:
		summary.Ready++
	case iapiserver.AtomicTaskStatusRunning:
		summary.Running++
	case iapiserver.AtomicTaskStatusRetrying:
		summary.Retrying++
	case iapiserver.AtomicTaskStatusCancelRequested:
		summary.CancelRequested++
	case iapiserver.AtomicTaskStatusSuccess:
		summary.Success++
	case iapiserver.AtomicTaskStatusFailed:
		summary.Failed++
	case iapiserver.AtomicTaskStatusCanceled:
		summary.Canceled++
	case iapiserver.AtomicTaskStatusTimeout:
		summary.Timeout++
	case iapiserver.AtomicTaskStatusSkipped:
		summary.Skipped++
	}
}

func buildDAGEvents(tasks []*iapiserver.AtomicTask, attempts []*iapiserver.TaskAttempt, projections []*iapiserver.RuntimeProjectionEvent) []*iapiserver.DAGExecutionEvent {
	byID := make(map[string]*iapiserver.AtomicTask, len(tasks))
	projectedStates := make(map[string]bool)
	result := make([]*iapiserver.DAGExecutionEvent, 0, len(tasks)+2*len(attempts)+len(projections))
	for _, task := range tasks {
		byID[task.ID] = task
		result = append(result, dagEvent("task:"+task.ID+":created", "NODE_CREATED", task, nil, task.CreatedAt))
		if !task.StartedAt.IsZero() && task.Progress > 0 && task.Progress < 1 {
			event := dagEvent("task:"+task.ID+":progress", "PROGRESS_UPDATED", task, nil, task.UpdatedAt)
			event.Progress = valuePtr(task.Progress)
			result = append(result, event)
		}
		if count := outputReferenceCount(task.Output, "artifact_refs") + outputReferenceCount(task.Output, "representation_refs"); count > 0 {
			event := dagEvent("task:"+task.ID+":output", "OUTPUT_AVAILABLE", task, nil, nonZeroTaskTime(task.CompletedAt, task.UpdatedAt))
			event.OutputCount = valuePtr(count)
			result = append(result, event)
		}
	}
	for _, attempt := range attempts {
		task := byID[attempt.AtomicTaskID]
		if task == nil {
			continue
		}
		result = append(result, dagEvent("attempt:"+attempt.ID+":scheduled", "ATTEMPT_SCHEDULED", task, attempt, attempt.CreatedAt))
		if attempt.AttemptNo > 1 {
			result = append(result, dagEvent("attempt:"+attempt.ID+":retry", "RETRY_SCHEDULED", task, attempt, attempt.CreatedAt))
		}
		if !attempt.StartedAt.IsZero() {
			result = append(result, dagEvent("attempt:"+attempt.ID+":started", "ATTEMPT_STARTED", task, attempt, attempt.StartedAt))
		}
	}
	for _, projection := range projections {
		taskID, _ := projection.Payload["atomic_task_id"].(string)
		task := byID[taskID]
		if task == nil {
			continue
		}
		status, _ := projection.Payload["status"].(string)
		if eventType := terminalDAGEventType(status); eventType != "" {
			projectedStates[task.ID+":"+eventType] = true
			event := dagEvent("projection:"+projection.ID, eventType, task, nil, projection.OccurredAt)
			event.Status = optionalString(status)
			if attemptID, ok := projection.Payload["task_attempt_id"].(string); ok {
				event.TaskAttemptID = optionalString(attemptID)
			}
			if attemptNo, ok := numericInt(projection.Payload["attempt_no"]); ok && attemptNo > 0 {
				event.AttemptNo = valuePtr(attemptNo)
			}
			if progress, ok := projection.Payload["progress"].(float64); ok {
				event.Progress = valuePtr(progress)
			}
			if count, ok := numericInt(projection.Payload["output_count"]); ok {
				event.OutputCount = valuePtr(count)
			}
			if task.LastError.Code != "" || task.LastError.Message != "" {
				errorValue := task.LastError
				event.Error = &errorValue
			}
			result = append(result, event)
		}
	}
	for _, task := range tasks {
		eventType := terminalDAGEventType(task.Status)
		if eventType == "" || projectedStates[task.ID+":"+eventType] {
			continue
		}
		event := dagEvent("task:"+task.ID+":state", eventType, task, nil, nonZeroTaskTime(task.CompletedAt, task.UpdatedAt))
		event.Status = optionalString(task.Status)
		if task.LastError.Code != "" || task.LastError.Message != "" {
			errorValue := task.LastError
			event.Error = &errorValue
		}
		result = append(result, event)
	}
	return result
}

func dagEvent(id, eventType string, task *iapiserver.AtomicTask, attempt *iapiserver.TaskAttempt, occurredAt imachinery.Time) *iapiserver.DAGExecutionEvent {
	event := &iapiserver.DAGExecutionEvent{ID: id, EventType: eventType, NodeKey: task.DAGNodeKey, AtomicTaskID: optionalString(task.ID), OccurredAt: occurredAt}
	if attempt != nil {
		event.TaskAttemptID, event.AttemptNo = optionalString(attempt.ID), valuePtr(attempt.AttemptNo)
	}
	return event
}

func terminalDAGEventType(status string) string {
	switch status {
	case iapiserver.AtomicTaskStatusBlocked:
		return "NODE_BLOCKED"
	case iapiserver.AtomicTaskStatusReady:
		return "NODE_READY"
	case iapiserver.AtomicTaskStatusSuccess:
		return "NODE_SUCCEEDED"
	case iapiserver.AtomicTaskStatusFailed:
		return "NODE_FAILED"
	case iapiserver.AtomicTaskStatusCanceled:
		return "NODE_CANCELED"
	case iapiserver.AtomicTaskStatusTimeout:
		return "NODE_TIMED_OUT"
	case iapiserver.AtomicTaskStatusSkipped:
		return "NODE_SKIPPED"
	default:
		return ""
	}
}

func buildTimelineRow(group *iapiserver.DAGTaskGroup, task *iapiserver.AtomicTask, attempts []*iapiserver.TaskAttempt, predecessors []string, completionByNode map[string]imachinery.Time) *iapiserver.DAGTimelineRow {
	row := &iapiserver.DAGTimelineRow{NodeKey: task.DAGNodeKey, AtomicTaskID: task.ID, AtomicTaskName: task.Name, AtomicTaskNameI18n: localizedName(task.TaskNameMeta), Status: task.Status, Complete: true, Segments: []*iapiserver.DAGTimelineSegment{}}
	start := group.TriggeredAt
	if start.IsZero() {
		start = group.CreatedAt
		row.Complete = false
	}
	queueStart := start
	if len(predecessors) > 0 {
		dependencyEnd := imachinery.Time{}
		dependencyComplete := true
		for _, predecessor := range predecessors {
			completed := completionByNode[predecessor]
			if completed.IsZero() {
				dependencyComplete = false
				continue
			}
			if dependencyEnd.IsZero() || dependencyEnd.Before(&completed) {
				dependencyEnd = completed
			}
		}
		segment := &iapiserver.DAGTimelineSegment{Phase: "DEPENDENCY_WAIT", StartedAt: start, CompletedAt: optionalTaskTime(dependencyEnd), Complete: dependencyComplete && !dependencyEnd.IsZero()}
		row.Segments = append(row.Segments, segment)
		row.Complete = row.Complete && segment.Complete
		if !dependencyEnd.IsZero() {
			queueStart = dependencyEnd
		}
	}
	if len(attempts) == 0 {
		row.Segments = append(row.Segments, &iapiserver.DAGTimelineSegment{Phase: "QUEUE_WAIT", StartedAt: queueStart, Complete: false})
		row.Complete = false
		return row
	}
	firstStart := attempts[0].StartedAt
	queueComplete := !firstStart.IsZero() && !firstStart.Before(&queueStart)
	queue := &iapiserver.DAGTimelineSegment{Phase: "QUEUE_WAIT", StartedAt: queueStart, CompletedAt: optionalTaskTime(firstStart), Complete: queueComplete}
	row.Segments = append(row.Segments, queue)
	row.Complete = row.Complete && queue.Complete
	for index, attempt := range attempts {
		if index > 0 {
			previous := attempts[index-1]
			retryComplete := !previous.CompletedAt.IsZero() && !attempt.StartedAt.IsZero() && !attempt.StartedAt.Before(&previous.CompletedAt)
			retry := &iapiserver.DAGTimelineSegment{Phase: "RETRY_WAIT", AttemptNo: valuePtr(attempt.AttemptNo), StartedAt: previous.CompletedAt, CompletedAt: optionalTaskTime(attempt.StartedAt), Complete: retryComplete}
			if retry.StartedAt.IsZero() {
				retry.StartedAt = previous.UpdatedAt
			}
			row.Segments = append(row.Segments, retry)
			row.Complete = row.Complete && retry.Complete
		}
		if attempt.StartedAt.IsZero() {
			row.Complete = false
			continue
		}
		runningComplete := !attempt.CompletedAt.IsZero() && !attempt.CompletedAt.Before(&attempt.StartedAt)
		running := &iapiserver.DAGTimelineSegment{Phase: "RUNNING", AttemptNo: valuePtr(attempt.AttemptNo), StartedAt: attempt.StartedAt, CompletedAt: optionalTaskTime(attempt.CompletedAt), Complete: runningComplete}
		row.Segments = append(row.Segments, running)
		row.Complete = row.Complete && running.Complete
	}
	return row
}

func groupTasksByNode(tasks []*iapiserver.AtomicTask) map[string][]*iapiserver.AtomicTask {
	result := make(map[string][]*iapiserver.AtomicTask)
	for _, task := range tasks {
		result[task.DAGNodeKey] = append(result[task.DAGNodeKey], task)
	}
	return result
}

func groupAttemptsByTask(attempts []*iapiserver.TaskAttempt) map[string][]*iapiserver.TaskAttempt {
	result := make(map[string][]*iapiserver.TaskAttempt)
	for _, attempt := range attempts {
		result[attempt.AtomicTaskID] = append(result[attempt.AtomicTaskID], attempt)
	}
	return result
}

func latestCompletionByNode(tasks []*iapiserver.AtomicTask) map[string]imachinery.Time {
	result := make(map[string]imachinery.Time)
	for _, task := range tasks {
		current := result[task.DAGNodeKey]
		if !task.CompletedAt.IsZero() && (current.IsZero() || current.Before(&task.CompletedAt)) {
			result[task.DAGNodeKey] = task.CompletedAt
		}
	}
	return result
}

func dagDependencies(edges []iapiserver.DAGEdge) map[string][]string {
	result := make(map[string][]string)
	for _, edge := range edges {
		result[edge.ToNode] = append(result[edge.ToNode], edge.FromNode)
	}
	return result
}

func outputReferenceCount(output map[string]any, key string) int {
	value := reflect.ValueOf(output[key])
	if value.IsValid() && (value.Kind() == reflect.Slice || value.Kind() == reflect.Array) {
		return value.Len()
	}
	return 0
}

func parseDAGEventTypes(raw string) (map[string]bool, error) {
	result := make(map[string]bool)
	if raw == "" {
		return result, nil
	}
	for _, value := range strings.Split(raw, ",") {
		value = strings.ToUpper(strings.TrimSpace(value))
		if !dagEventTypes[value] {
			return nil, fmt.Errorf("event_types contains an unsupported value")
		}
		result[value] = true
	}
	return result, nil
}

func parseOptionalTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	return &parsed, err
}

func optionalTaskTime(value imachinery.Time) *imachinery.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func nonZeroTaskTime(values ...imachinery.Time) imachinery.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return imachinery.Now()
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func numericInt(value any) (int, bool) {
	switch number := value.(type) {
	case int:
		return number, true
	case int64:
		return int(number), true
	case float64:
		return int(number), true
	default:
		return 0, false
	}
}

func valuePtr[T any](value T) *T { return &value }
