package taskcenter

import (
	"context"
	stderrors "errors"
	"fmt"
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
	store    store.TaskCenterStore
	runtime  workflowruntime.WorkflowRuntime
	interval time.Duration
}

func NewReconciler(factory store.Factory, runtime workflowruntime.WorkflowRuntime, interval time.Duration) *Reconciler {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	return &Reconciler{store: factory.TaskCenters(), runtime: runtime, interval: interval}
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
	return stderrors.Join(errs...)
}

func (r *Reconciler) reconcileOwner(ctx context.Context, ownerType, ownerID, executionID string) error {
	execution, err := r.runtime.GetExecution(ctx, executionID)
	if err != nil {
		return err
	}
	tasks, _, err := r.store.ListOwnedTasks(ctx, ownerType, ownerID, &iapiserver.AtomicTaskListRequest{})
	if err != nil {
		return err
	}
	var errs []error
	for _, task := range tasks {
		task.RuntimeExecutionID = executionID
		if _, err := r.project(ctx, task, execution); err != nil {
			errs = append(errs, err)
		}
	}
	return stderrors.Join(errs...)
}

func (r *Reconciler) reconcileScheduleExecutions(ctx context.Context) error {
	executions, err := r.store.ListActiveScheduleExecutions(ctx, 200)
	if err != nil {
		return err
	}
	var errs []error
	for _, execution := range executions {
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
		attempt := &iapiserver.TaskAttempt{AtomicTaskID: task.ID, AttemptNo: attemptNo, RuntimeTaskID: runtimeTask.ID, Status: attemptStatus(runtimeTask.Status), InputSnapshot: runtimeTask.Input, OutputSnapshot: runtimeTask.Output, StartedAt: imachinery.NewTime(runtimeTask.StartedAt), CompletedAt: imachinery.NewTime(runtimeTask.CompletedAt)}
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
	event := &iapiserver.RuntimeProjectionEvent{RuntimeEventID: fmt.Sprintf("%s:%s:%s:%d", execution.ID, task.ID, latest.Status, task.CurrentAttempt), RuntimeExecutionID: execution.ID, RuntimeTaskID: task.RuntimeTaskID, EventType: iapiserver.TaskCenterEventProjectionReconciled, Payload: map[string]any{"atomic_task_id": task.ID, "status": task.Status, "resource_version": task.ResourceVersion + 1}, OccurredAt: imachinery.Now(), ProjectionStatus: iapiserver.RuntimeProjectionStatusPending}
	event.ID = uuid.NewString()
	return r.store.ApplyRuntimeProjection(ctx, task, attempts, event)
}

func runtimeTaskStatus(status string) string {
	switch status {
	case "COMPLETED":
		return iapiserver.AtomicTaskStatusSuccess
	case "FAILED", "FAILED_WITH_TERMINAL_ERROR":
		return iapiserver.AtomicTaskStatusFailed
	case "CANCELED":
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
