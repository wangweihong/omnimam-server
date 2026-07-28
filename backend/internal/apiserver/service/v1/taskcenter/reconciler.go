package taskcenter

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
)

// Reconciler periodically repairs Task Center projections from non-terminal runtime executions.
type Reconciler struct {
	store             store.TaskCenterStore
	runtime           workflowruntime.WorkflowRuntime
	interval          time.Duration
	terminalObservers []func(context.Context, *iapiserver.AtomicTask) error
}

func NewReconciler(factory store.Factory, runtime workflowruntime.WorkflowRuntime, interval time.Duration, terminalObservers ...func(context.Context, *iapiserver.AtomicTask) error) *Reconciler {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	return &Reconciler{store: factory.TaskCenters(), runtime: runtime, interval: interval, terminalObservers: terminalObservers}
}
func (r *Reconciler) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		if err := r.Reconcile(ctx); err != nil {
			// 单条历史投影损坏不能终止全部 Worker；后续周期仍需修复其他可恢复执行。
			log.Errorf("task center reconciliation failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
func (r *Reconciler) Reconcile(ctx context.Context) error {
	tasks, err := r.store.ListNonTerminalAtomicTasks(ctx, 200)
	if err != nil {
		return err
	}
	var errs []error
	for _, task := range tasks {
		execution, getErr := r.runtime.GetExecution(ctx, task.RuntimeExecutionID)
		if getErr != nil {
			if stderrors.Is(getErr, workflowruntime.ErrExecutionNotFound) {
				task.Status = iapiserver.AtomicTaskStatusFailed
				task.CompletedAt = imachinery.Now()
				task.LastError = iapiserver.TaskError{Message: getErr.Error(), OccurredAt: imachinery.Now()}
				_, getErr = r.store.UpdateAtomicTask(ctx, task)
			}
			errs = append(errs, getErr)
			continue
		}
		if _, applyErr := r.project(ctx, task, execution); applyErr != nil {
			errs = append(errs, applyErr)
		}
	}
	groups, groupErr := r.store.ListNonTerminalTaskGroups(ctx, 100)
	if groupErr != nil {
		errs = append(errs, groupErr)
	} else {
		for _, group := range groups {
			if err := r.reconcileOwner(ctx, iapiserver.TaskOwnerTypeGroup, group.ID, group.RuntimeExecutionID); err != nil {
				if stderrors.Is(err, workflowruntime.ErrExecutionNotFound) {
					group.Status = iapiserver.TaskGroupStatusFailed
					_, err = r.store.UpdateTaskGroup(ctx, group)
				}
				errs = append(errs, err)
			}
		}
	}
	dags, dagErr := r.store.ListNonTerminalDAGTaskGroups(ctx, 100)
	if dagErr != nil {
		errs = append(errs, dagErr)
	} else {
		for _, dag := range dags {
			if err := r.reconcileOwner(ctx, iapiserver.TaskOwnerTypeDAGGroup, dag.ID, dag.RuntimeExecutionID); err != nil {
				if stderrors.Is(err, workflowruntime.ErrExecutionNotFound) {
					dag.Status = iapiserver.TaskGroupStatusFailed
					_, err = r.store.UpdateDAGTaskGroup(ctx, dag)
				}
				errs = append(errs, err)
			}
		}
	}
	if err := r.reconcileScheduleExecutions(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := r.reconcileRuntimeRetention(ctx); err != nil {
		reconcileRetentionFailures.WithLabelValues("conductor").Inc()
		errs = append(errs, err)
	}
	return stderrors.Join(errs...)
}

func (r *Reconciler) reconcileOwner(ctx context.Context, ownerType, ownerID, executionID string) error {
	execution, err := r.runtime.GetExecution(ctx, executionID)
	if err != nil {
		return err
	}
	var tasks []*iapiserver.AtomicTask
	if ownerType == iapiserver.TaskOwnerTypeDAGGroup {
		tasks, err = r.store.ListDAGObservationTasks(ctx, ownerID)
		if err == nil {
			var group *iapiserver.DAGTaskGroup
			group, err = r.store.GetDAGTaskGroup(ctx, ownerID)
			if err == nil {
				dynamic := dynamicTasksFromExecution(group, tasks, execution)
				if len(dynamic) > 0 {
					err = r.store.AddOwnedAtomicTasks(ctx, ownerType, ownerID, dynamic)
					if err == nil {
						tasks, err = r.store.ListDAGObservationTasks(ctx, ownerID)
					}
				}
			}
		}
	} else {
		tasks, _, err = r.store.ListOwnedTasks(ctx, ownerType, ownerID, &iapiserver.AtomicTaskListRequest{BasicQueryParam: imachinery.BasicQueryParam{PagingParams: imachinery.PagingParams{PageSize: iapiserver.MaxTaskGraphNodes}}})
	}
	if err != nil {
		return err
	}
	var errs []error
	projectionApplied := false
	for _, task := range tasks {
		task.RuntimeExecutionID = executionID
		applied, projectErr := r.project(ctx, task, execution)
		if projectErr != nil {
			errs = append(errs, projectErr)
		} else if applied {
			projectionApplied = true
		}
	}
	if !projectionApplied {
		if err := r.store.RepairTerminalTaskOwner(ctx, ownerType, ownerID); err != nil {
			errs = append(errs, err)
		}
	}
	return stderrors.Join(errs...)
}

// dynamicTasksFromExecution materializes Conductor Dynamic Fork children that carry the required business identity envelope.
func dynamicTasksFromExecution(group *iapiserver.DAGTaskGroup, existing []*iapiserver.AtomicTask, execution workflowruntime.Execution) []*iapiserver.AtomicTask {
	known := make(map[string]bool, len(existing))
	declaredDynamic := make(map[string]bool)
	for _, task := range existing {
		known[task.ID] = true
	}
	for _, node := range group.Nodes {
		declaredDynamic[node.Key] = node.DynamicFork
	}
	result := make([]*iapiserver.AtomicTask, 0)
	for _, runtimeTask := range execution.Tasks {
		atomicTaskID, _ := runtimeTask.Input["atomic_task_id"].(string)
		nodeKey, _ := runtimeTask.Input["dag_node_key"].(string)
		functionRef, _ := runtimeTask.Input["function_ref"].(string)
		if atomicTaskID == "" || known[atomicTaskID] || !declaredDynamic[nodeKey] || functionRef == "" {
			continue
		}
		arguments, _ := runtimeTask.Input["arguments"].(map[string]any)
		childRef, _ := runtimeTask.Input["child_key"].(string)
		if childRef == "" {
			childRef = shortID(runtimeTask.ID)
		}
		childKey := nodeKey + ":" + childRef
		if len(childKey) > 128 {
			childKey = childKey[:128]
		}
		task := &iapiserver.AtomicTask{
			FunctionRef: functionRef, Arguments: arguments, Status: runtimeTaskStatus(runtimeTask.Status), Progress: runtimeTaskProgress(runtimeTask.Status),
			RootTaskID: atomicTaskID, OwnerType: iapiserver.TaskOwnerTypeDAGGroup, OwnerID: group.ID,
			ChildKey: childKey, ChildOrder: dynamicChildOrder(runtimeTask.Input["child_order"]), DAGNodeKey: nodeKey, RuntimeExecutionID: execution.ID, RuntimeTaskID: runtimeTask.ID,
			StartedAt: imachinery.NewTime(runtimeTask.StartedAt), CompletedAt: imachinery.NewTime(runtimeTask.CompletedAt),
			Output: runtimeTask.Output, ProjectID: group.ProjectID, Namespace: group.Namespace, CreatedBy: group.CreatedBy,
		}
		task.ID, task.Name = atomicTaskID, childKey
		if runtimeTask.FailureReason != "" {
			task.LastError = iapiserver.TaskError{Message: runtimeTask.FailureReason, OccurredAt: imachinery.Now()}
		}
		result = append(result, task)
		known[atomicTaskID] = true
	}
	return result
}

func runtimeTaskProgress(status string) float64 {
	if iapiserver.IsAtomicTaskTerminal(runtimeTaskStatus(status)) {
		return 1
	}
	return 0
}

func dynamicChildOrder(value any) int {
	switch order := value.(type) {
	case int:
		return order
	case int64:
		return int(order)
	case float64:
		return int(order)
	default:
		return 0
	}
}

func (r *Reconciler) reconcileScheduleExecutions(ctx context.Context) error {
	executions, err := r.store.ListActiveScheduleExecutions(ctx, 200)
	if err != nil {
		return err
	}
	var errs []error
	for _, execution := range executions {
		if execution.TriggerSource == iapiserver.ScheduleExecutionTriggerManual && execution.TargetID == "" {
			if err := r.recoverManualScheduleController(ctx, execution); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if execution.ExecutionMode == iapiserver.TaskScheduleModeReconcile {
			if err := r.recoverReconcileExecution(ctx, execution); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		status, terminal, statusErr := r.scheduleTargetStatus(ctx, execution)
		if statusErr != nil {
			errs = append(errs, statusErr)
			continue
		}
		if !terminal {
			continue
		}
		execution.Status = status
		execution.CompletedAt = imachinery.Now()
		if _, updateErr := r.store.UpdateScheduleExecution(ctx, execution); updateErr != nil {
			errs = append(errs, updateErr)
		}
	}
	return stderrors.Join(errs...)
}

func (r *Reconciler) recoverManualScheduleController(ctx context.Context, execution *iapiserver.TaskScheduleExecution) error {
	if execution.RuntimeExecutionID != "" {
		runtimeExecution, err := r.runtime.GetExecution(ctx, execution.RuntimeExecutionID)
		if err == nil {
			if runtimeExecution.Status == "RUNNING" || runtimeExecution.Status == "PAUSED" {
				return nil
			}
			execution.Status = iapiserver.ScheduleExecutionStatusTriggerFailed
			execution.Reason = "manual schedule controller terminated before creating its target"
			if runtimeExecution.FailureReason != "" {
				execution.Reason = runtimeExecution.FailureReason
			}
			execution.CompletedAt = imachinery.Now()
			_, err = r.store.UpdateScheduleExecution(ctx, execution)
			return err
		}
		if !stderrors.Is(err, workflowruntime.ErrExecutionNotFound) {
			return err
		}
	}
	binding, err := r.runtime.RegisterDefinition(ctx, manualScheduleControllerDefinition())
	if err != nil {
		return err
	}
	runtimeExecution, err := r.runtime.StartExecution(ctx, workflowruntime.StartRequest{
		DefinitionName: binding.DefinitionName, DefinitionVersion: binding.DefinitionVersion,
		CorrelationID: execution.ID, IdempotencyKey: execution.ID,
		Input: map[string]any{"schedule_execution_id": execution.ID},
	})
	if err != nil {
		return err
	}
	execution.RuntimeExecutionID = runtimeExecution.ID
	_, err = r.store.UpdateScheduleExecution(ctx, execution)
	return err
}

func (r *Reconciler) recoverReconcileExecution(ctx context.Context, execution *iapiserver.TaskScheduleExecution) error {
	runtimeExecution, err := r.runtime.GetExecution(ctx, execution.RuntimeExecutionID)
	missing := stderrors.Is(err, workflowruntime.ErrExecutionNotFound)
	if err != nil && !missing {
		return err
	}
	if !missing && (runtimeExecution.Status == "RUNNING" || runtimeExecution.Status == "PAUSED") {
		return nil
	}
	_, err = r.store.WithScheduleReconcileLock(ctx, execution.ScheduleID, func() error {
		current, getErr := r.store.GetScheduleExecution(ctx, execution.ID)
		if getErr != nil {
			return getErr
		}
		if current.Status != iapiserver.ScheduleExecutionStatusTriggered && current.Status != iapiserver.ScheduleExecutionStatusRunning {
			return nil
		}
		state, stateErr := r.store.GetScheduleReconcileState(ctx, current.ScheduleID)
		if stateErr != nil {
			return stateErr
		}
		now := imachinery.Now()
		reason := "reconcile controller terminated before committing its result"
		if missing {
			reason = "reconcile runtime execution no longer exists"
		}
		current.Status, current.Reason, current.CompletedAt = iapiserver.ScheduleExecutionStatusFailed, reason, now
		state.CurrentRuntimeExecutionID, state.LastCompletedAt = "", &now
		state.ConsecutiveFailures++
		state.TotalRuns++
		state.ResourceVersion++
		return r.store.CompleteScheduleReconcile(ctx, current, state)
	})
	return err
}

func (r *Reconciler) reconcileRuntimeRetention(ctx context.Context) error {
	items, err := r.runtime.ListTerminalExecutions(ctx, ReconcileControllerDefinition, time.Now().Add(-24*time.Hour), 200)
	if err != nil {
		return err
	}
	var errs []error
	for _, execution := range items {
		if err := r.runtime.DeleteTerminalExecution(ctx, execution.ID); err != nil {
			errs = append(errs, err)
		}
	}
	return stderrors.Join(errs...)
}

func (r *Reconciler) scheduleTargetStatus(ctx context.Context, execution *iapiserver.TaskScheduleExecution) (string, bool, error) {
	switch execution.TargetType {
	case iapiserver.TaskScheduleTargetAtomic:
		task, err := r.store.GetAtomicTask(ctx, execution.TargetID)
		if err != nil {
			return "", false, err
		}
		return scheduleExecutionStatus(task.Status)
	case iapiserver.TaskScheduleTargetGroup:
		group, err := r.store.GetTaskGroup(ctx, execution.TargetID)
		if err != nil {
			return "", false, err
		}
		return scheduleExecutionStatus(group.Status)
	case iapiserver.TaskScheduleTargetDAG:
		dag, err := r.store.GetDAGTaskGroup(ctx, execution.TargetID)
		if err != nil {
			return "", false, err
		}
		return scheduleExecutionStatus(dag.Status)
	default:
		return "", false, fmt.Errorf("unsupported schedule target %s", execution.TargetType)
	}
}
func (r *Reconciler) project(ctx context.Context, task *iapiserver.AtomicTask, execution workflowruntime.Execution) (bool, error) {
	attempts := make([]*iapiserver.TaskAttempt, 0)
	var latest *workflowruntime.ExecutionTask
	for _, runtimeTask := range execution.Tasks {
		atomicTaskID, _ := runtimeTask.Input["atomic_task_id"].(string)
		if atomicTaskID != task.ID {
			continue
		}
		current := runtimeTask
		latest = &current
		attemptNo := runtimeTask.RetryCount + 1
		executorType, executorName := taskExecutorSnapshot(task.FunctionRef)
		attempt := &iapiserver.TaskAttempt{AtomicTaskID: task.ID, AttemptNo: attemptNo, RuntimeTaskID: runtimeTask.ID, Status: attemptStatus(runtimeTask.Status), InputSnapshot: runtimeTask.Input, OutputSnapshot: runtimeTask.Output, ExecutorType: executorType, ExecutorDisplayName: executorName, StartedAt: imachinery.NewTime(runtimeTask.StartedAt), CompletedAt: imachinery.NewTime(runtimeTask.CompletedAt)}
		if externalJobID, ok := runtimeTask.Output["external_job_id"].(string); ok {
			attempt.ExternalJobID = externalJobID
		}
		attempt.ID = uuid.NewString()
		if !runtimeTask.StartedAt.IsZero() && !runtimeTask.CompletedAt.IsZero() {
			attempt.DurationMS = runtimeTask.CompletedAt.Sub(runtimeTask.StartedAt).Milliseconds()
		}
		if runtimeTask.FailureReason != "" {
			attempt.Error = iapiserver.TaskError{Message: runtimeTask.FailureReason, OccurredAt: imachinery.Now()}
		}
		attempt.LogsRef = iapiserver.TaskAttemptLogsRef(attempt.ID)
		r.appendRuntimeTerminalLog(ctx, runtimeTask)
		attempts = append(attempts, attempt)
		if attemptNo >= task.CurrentAttempt {
			task.CurrentAttempt = attemptNo
			task.RuntimeTaskID = runtimeTask.ID
		}
	}
	if latest == nil {
		return false, nil
	}
	task.RuntimeExecutionID = execution.ID
	task.Status = runtimeTaskStatus(latest.Status)
	task.StartedAt = imachinery.NewTime(latest.StartedAt)
	task.CompletedAt = imachinery.NewTime(latest.CompletedAt)
	task.Output = latest.Output
	if latest.FailureReason != "" {
		task.LastError = iapiserver.TaskError{Message: latest.FailureReason, OccurredAt: imachinery.Now()}
	}
	if iapiserver.IsAtomicTaskTerminal(task.Status) {
		task.Progress = 1
	}
	payload := map[string]any{
		"atomic_task_id": task.ID, "node_key": task.DAGNodeKey, "status": task.Status,
		"progress": task.Progress, "resource_version": task.ResourceVersion + 1,
		"attempt_no":   task.CurrentAttempt,
		"output_count": outputReferenceCount(task.Output, "artifact_refs") + outputReferenceCount(task.Output, "representation_refs"),
	}
	if len(attempts) > 0 {
		payload["task_attempt_id"] = attempts[len(attempts)-1].ID
	}
	event := &iapiserver.RuntimeProjectionEvent{RuntimeEventID: fmt.Sprintf("%s:%s:%s:%d", execution.ID, task.ID, latest.Status, task.CurrentAttempt), RuntimeExecutionID: execution.ID, RuntimeTaskID: task.RuntimeTaskID, EventType: iapiserver.TaskCenterEventProjectionReconciled, Payload: payload, OccurredAt: imachinery.Now(), ProjectionStatus: iapiserver.RuntimeProjectionStatusPending}
	event.ID = uuid.NewString()
	applied, err := r.store.ApplyRuntimeProjection(ctx, task, attempts, event)
	if err != nil || !applied || !iapiserver.IsAtomicTaskTerminal(task.Status) {
		return applied, err
	}
	for _, observer := range r.terminalObservers {
		if observerErr := observer(ctx, task); observerErr != nil {
			return applied, observerErr
		}
	}
	return applied, nil
}

func taskExecutorSnapshot(functionRef string) (string, string) {
	switch {
	case strings.HasPrefix(functionRef, "application"), strings.HasPrefix(functionRef, "comfyui"):
		return iapiserver.TaskExecutorApplication, "Application executor"
	case strings.HasPrefix(functionRef, "task."), strings.Contains(functionRef, "reconcile"):
		return iapiserver.TaskExecutorSystem, "Task Center system executor"
	default:
		return iapiserver.TaskExecutorWorker, "Task worker"
	}
}

func (r *Reconciler) appendRuntimeTerminalLog(ctx context.Context, task workflowruntime.ExecutionTask) {
	var entry workflowruntime.TaskLogEntry
	switch task.Status {
	case "CANCELED", "TERMINATED":
		entry = workflowruntime.LifecycleLog("attempt.canceled", workflowruntime.TaskLogLevelWarn, "Execution attempt was canceled.")
	case "TIMED_OUT":
		entry = workflowruntime.LifecycleLog("attempt.timed_out", workflowruntime.TaskLogLevelError, "Execution attempt timed out.")
	default:
		return
	}
	logCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if err := r.runtime.AppendTaskLog(logCtx, task.ID, entry); err != nil {
		log.Warnf("append reconciled task lifecycle log failed: error=%v", err)
	}
}

func runtimeTaskStatus(status string) string {
	switch status {
	case "COMPLETED":
		return iapiserver.AtomicTaskStatusSuccess
	case "FAILED", "FAILED_WITH_TERMINAL_ERROR":
		return iapiserver.AtomicTaskStatusFailed
	case "CANCELED", "TERMINATED":
		return iapiserver.AtomicTaskStatusCanceled
	case "TIMED_OUT":
		return iapiserver.AtomicTaskStatusTimeout
	case "SKIPPED":
		return iapiserver.AtomicTaskStatusSkipped
	case "SCHEDULED":
		return iapiserver.AtomicTaskStatusReady
	default:
		return iapiserver.AtomicTaskStatusRunning
	}
}

func scheduleExecutionStatus(status string) (string, bool, error) {
	switch status {
	case iapiserver.AtomicTaskStatusSuccess:
		return iapiserver.ScheduleExecutionStatusSuccess, true, nil
	case iapiserver.AtomicTaskStatusFailed, iapiserver.AtomicTaskStatusTimeout:
		return iapiserver.ScheduleExecutionStatusFailed, true, nil
	case iapiserver.AtomicTaskStatusCanceled:
		return iapiserver.ScheduleExecutionStatusCanceled, true, nil
	default:
		return "", false, nil
	}
}
func executionStatus(status string) string {
	switch status {
	case "COMPLETED":
		return iapiserver.AtomicTaskStatusSuccess
	case "FAILED", "FAILED_WITH_TERMINAL_ERROR":
		return iapiserver.AtomicTaskStatusFailed
	case "TERMINATED":
		return iapiserver.AtomicTaskStatusCanceled
	case "TIMED_OUT":
		return iapiserver.AtomicTaskStatusTimeout
	case "PAUSED":
		return iapiserver.AtomicTaskStatusBlocked
	default:
		return iapiserver.AtomicTaskStatusRunning
	}
}
func attemptStatus(status string) string {
	switch status {
	case "COMPLETED":
		return iapiserver.TaskAttemptStatusSuccess
	case "FAILED", "FAILED_WITH_TERMINAL_ERROR":
		return iapiserver.TaskAttemptStatusFailed
	case "CANCELED":
		return iapiserver.TaskAttemptStatusCanceled
	case "TIMED_OUT":
		return iapiserver.TaskAttemptStatusTimeout
	case "SCHEDULED_FOR_RETRY", "IN_PROGRESS":
		return iapiserver.TaskAttemptStatusRunning
	default:
		return iapiserver.TaskAttemptStatusScheduled
	}
}
