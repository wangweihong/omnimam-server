package taskcenter

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

const (
	ReconcileControllerDefinition = "task_center_reconcile_controller"
	ReconcileControllerVersion    = 1
	ReconcileControllerTask       = "task.reconcile.control"
	scheduleMisfireGrace          = 5 * time.Second
)

var (
	reconcileRuns              = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "reconcile_runs_total", Help: "Task Center reconcile runs by bounded handler ref and status."}, []string{"ref", "status"})
	reconcileScanned           = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "reconcile_scanned_total", Help: "Resources scanned by reconcile handlers."}, []string{"ref"})
	reconcileFindings          = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "reconcile_findings_total", Help: "Findings produced by reconcile handlers."}, []string{"ref"})
	reconcileActions           = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "reconcile_actions_total", Help: "Atomic repair actions created by reconcile handlers."}, []string{"ref"})
	reconcileDuration          = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "reconcile_duration_seconds", Help: "Reconcile handler duration.", Buckets: prometheus.DefBuckets}, []string{"ref"})
	reconcileCheckpointAge     = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "reconcile_checkpoint_age_seconds", Help: "Age of the persisted reconcile checkpoint."}, []string{"ref"})
	reconcileOverlapSkipped    = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "reconcile_overlap_skipped_total", Help: "Overlapping reconcile runs skipped."}, []string{"ref"})
	reconcileRetentionFailures = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "reconcile_retention_failures_total", Help: "Reconcile history retention failures."}, []string{"backend"})
)

func init() {
	prometheus.MustRegister(reconcileRuns, reconcileScanned, reconcileFindings, reconcileActions, reconcileDuration, reconcileCheckpointAge, reconcileOverlapSkipped, reconcileRetentionFailures)
}

// ReconcileRequest 是 Task Center 传给受控巡检器的稳定执行契约。
type ReconcileRequest struct {
	ScheduleID     string
	ScheduledAt    time.Time
	Checkpoint     map[string]any
	Config         map[string]any
	MaxParallelism int
	MaxItemsPerRun int
	PerItemTimeout time.Duration
	OverallTimeout time.Duration
}

// ReconcileResult 返回可原子保存的 checkpoint、摘要和受控修复动作。
type ReconcileResult struct {
	NextCheckpoint map[string]any
	CycleCompleted bool
	Scanned        int
	Findings       int
	Deferred       int
	Actions        []*iapiserver.AtomicTaskCreateRequest
	Summary        map[string]any
}

// ReconcileHandler 由具体业务域实现；巡检器不能直接构造运行时任务。
type ReconcileHandler interface {
	Ref() string
	DisplayName() string
	ValidateConfig(map[string]any) error
	Reconcile(context.Context, ReconcileRequest) (ReconcileResult, error)
}

// ReconcileRegistry 是进程内只读 ref 路由表，注册完成后供固定 controller 调用。
type ReconcileRegistry struct {
	mu       sync.RWMutex
	handlers map[string]ReconcileHandler
}

func NewReconcileRegistry() *ReconcileRegistry {
	return &ReconcileRegistry{handlers: make(map[string]ReconcileHandler)}
}

func (r *ReconcileRegistry) Register(handler ReconcileHandler) error {
	if handler == nil || handler.Ref() == "" {
		return fmt.Errorf("reconcile handler ref is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[handler.Ref()]; exists {
		return fmt.Errorf("reconcile handler %s is already registered", handler.Ref())
	}
	r.handlers[handler.Ref()] = handler
	return nil
}

func (r *ReconcileRegistry) Get(ref string) (ReconcileHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.handlers[ref]
	return handler, ok
}

// EnsureSystemReconcileSchedule 原子补齐系统计划；已存在时绝不覆盖管理员参数。
func (s *taskCenterService) EnsureSystemReconcileSchedule(ctx context.Context, schedule *iapiserver.TaskSchedule) (*iapiserver.TaskSchedule, error) {
	if schedule == nil || schedule.SystemKey == "" || schedule.ReconcileSpec == nil {
		return nil, errors.NewStatus(code.ErrTaskReconcileConfigInvalid, "system reconcile schedule is invalid")
	}
	nameSpec := systemNameSpec(schedule.TaskNameMeta)
	if nameSpec.Key == "" {
		return nil, errors.NewStatus(code.ErrTaskReconcileConfigInvalid, "system reconcile schedule name is invalid")
	}
	if err := assignSystemName(&schedule.Name, &schedule.TaskNameMeta, nameSpec); err != nil {
		return nil, errors.NewStatus(code.ErrTaskReconcileConfigInvalid, err.Error())
	}
	handler, ok := s.reconciles.Get(schedule.ReconcileSpec.ReconcileRef)
	if !ok {
		return nil, errors.NewStatus(code.ErrTaskReconcileRefUnregistered, "reconcile handler is not registered")
	}
	if err := validateReconcileSpec(schedule.ReconcileSpec, handler); err != nil {
		return nil, err
	}
	schedule.ID = uuid.NewString()
	schedule.ExecutionMode, schedule.ManagementMode = iapiserver.TaskScheduleModeReconcile, iapiserver.TaskScheduleManagementSystem
	schedule.TriggerType, schedule.Status = iapiserver.TaskScheduleTriggerCron, iapiserver.TaskScheduleStatusActive
	schedule.Target = iapiserver.ScheduleTarget{}
	schedule.MisfirePolicy, schedule.OverlapPolicy = iapiserver.TaskSchedulePolicySkip, iapiserver.TaskSchedulePolicySkip
	schedule.RuntimeScheduleName = "task_schedule_" + schedule.ID
	if schedule.HistoryRetention == (iapiserver.HistoryRetention{}) {
		schedule.HistoryRetention = defaultHistoryRetention()
	}
	binding, err := s.runtime.RegisterDefinition(ctx, reconcileControllerDefinition())
	if err != nil {
		return nil, runtimeError(err)
	}
	state := &iapiserver.ScheduleReconcileState{Checkpoint: map[string]any{}}
	created, _, err := s.store.EnsureSystemTaskSchedule(ctx, schedule, state)
	if err != nil {
		return nil, err
	}
	if created.ReconcileSpec == nil || created.ReconcileSpec.ReconcileRef != handler.Ref() {
		return nil, errors.NewStatus(code.ErrTaskSystemScheduleOperationRestricted, "persisted system schedule has protected reconcile fields")
	}
	if err := validateReconcileSpec(created.ReconcileSpec, handler); err != nil {
		return nil, err
	}
	start := workflowruntime.StartRequest{DefinitionName: binding.DefinitionName, DefinitionVersion: binding.DefinitionVersion, CorrelationID: created.ID + "-${scheduledTime}", Input: map[string]any{"task_schedule_id": created.ID}}
	if err := s.runtime.SaveSchedule(ctx, runtimeSchedule(created, start)); err != nil {
		return nil, runtimeError(err)
	}
	projectLocalizedName(&created.TaskNameMeta)
	return s.decorateReconcileSchedule(created), nil
}

// RunScheduleReconcile 执行固定 controller 的一轮巡检，并原子提交 execution 与状态投影。
func (s *taskCenterService) RunScheduleReconcile(ctx context.Context, scheduleID, runtimeExecutionID string, scheduledAt time.Time) (map[string]any, error) {
	schedule, err := s.store.GetTaskSchedule(ctx, scheduleID)
	if err != nil {
		return nil, err
	}
	if schedule.ExecutionMode != iapiserver.TaskScheduleModeReconcile || schedule.ReconcileSpec == nil {
		return nil, errors.NewStatus(code.ErrTaskScheduleInvalid, "schedule is not reconcile mode")
	}
	handler, ok := s.reconciles.Get(schedule.ReconcileSpec.ReconcileRef)
	if !ok {
		return nil, errors.NewStatus(code.ErrTaskReconcileRefUnregistered, "reconcile handler is not registered")
	}
	if skip, err := shouldSkipScheduleMisfire(ctx, s.store, schedule.ID, scheduledAt, time.Now()); err != nil {
		return nil, err
	} else if skip {
		return scheduleMisfireOutput(scheduledAt), nil
	}
	execution := &iapiserver.TaskScheduleExecution{ScheduleID: schedule.ID, ExecutionMode: iapiserver.TaskScheduleModeReconcile, TriggerSource: iapiserver.ScheduleExecutionTriggerSchedule, TriggeredBy: schedule.CreatedBy, ScheduledAt: imachinery.NewTime(scheduledAt), TriggeredAt: imachinery.Now(), RuntimeExecutionID: runtimeExecutionID, Status: iapiserver.ScheduleExecutionStatusTriggered}
	execution.ID, execution.Name = uuid.NewString(), "Reconcile execution"
	record, acquired, err := s.store.AcquireScheduleExecution(ctx, execution)
	if err != nil {
		return nil, err
	}
	resuming := canResumeScheduleExecution(record, execution)
	if !acquired && !resuming {
		if record.Status == iapiserver.ScheduleExecutionStatusSkippedOverlap {
			reconcileOverlapSkipped.WithLabelValues(handler.Ref()).Inc()
			reconcileRuns.WithLabelValues(handler.Ref(), record.Status).Inc()
		}
		return map[string]any{"schedule_execution_id": record.ID, "status": record.Status}, nil
	}
	lockTimeout := time.Duration(schedule.ReconcileSpec.OverallTimeoutSeconds)*time.Second + 10*time.Second
	lockCtx, lockCancel := context.WithTimeout(context.WithoutCancel(ctx), lockTimeout)
	defer lockCancel()
	var output map[string]any
	var runErr error
	locked, err := s.store.WithScheduleReconcileLock(lockCtx, schedule.ID, func() error {
		current, getErr := s.store.GetScheduleExecution(lockCtx, record.ID)
		if getErr != nil {
			return getErr
		}
		if !canResumeScheduleExecution(current, execution) {
			output = map[string]any{"schedule_execution_id": current.ID, "status": current.Status}
			return nil
		}
		output, runErr = s.runAcquiredScheduleReconcile(ctx, schedule, handler, current, runtimeExecutionID, scheduledAt)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, errors.NewStatus(code.ErrWorkflowRuntimeRejected, "reconcile controller is already running")
	}
	return output, runErr
}

// ScheduleTriggerMisfired 判断首次触发是否已超过调度宽限。宽限吸收正常队列抖动，避免 Redis
// 恢复停机前的延迟 scheduler 消息后补发已经错过的 V1 轮次。
func ScheduleTriggerMisfired(scheduledAt, triggeredAt time.Time) bool {
	if scheduledAt.IsZero() || triggeredAt.IsZero() || triggeredAt.Before(scheduledAt) {
		return false
	}
	return triggeredAt.Sub(scheduledAt) > scheduleMisfireGrace
}

func shouldSkipScheduleMisfire(ctx context.Context, storage store.TaskCenterStore, scheduleID string, scheduledAt, triggeredAt time.Time) (bool, error) {
	if !ScheduleTriggerMisfired(scheduledAt, triggeredAt) {
		return false, nil
	}
	existing, err := storage.GetScheduleExecutionAt(ctx, scheduleID, scheduledAt)
	if err != nil {
		return false, err
	}
	// 已有轮次必须继续走 Acquire/恢复路径，不能把 Worker 重投误判为停机补发。
	return existing == nil, nil
}

func scheduleMisfireOutput(scheduledAt time.Time) map[string]any {
	return map[string]any{
		"status":       iapiserver.TaskSchedulePolicySkip,
		"scheduled_at": scheduledAt.UTC().Format(time.RFC3339Nano),
		"reason":       "misfire policy skipped delayed schedule trigger",
	}
}

func canResumeScheduleExecution(existing, incoming *iapiserver.TaskScheduleExecution) bool {
	return existing != nil && incoming != nil &&
		incoming.ExecutionMode == iapiserver.TaskScheduleModeReconcile &&
		existing.ExecutionMode == iapiserver.TaskScheduleModeReconcile &&
		existing.RuntimeExecutionID != "" && existing.RuntimeExecutionID == incoming.RuntimeExecutionID &&
		(existing.Status == iapiserver.ScheduleExecutionStatusTriggered || existing.Status == iapiserver.ScheduleExecutionStatusRunning)
}

func (s *taskCenterService) runAcquiredScheduleReconcile(ctx context.Context, schedule *iapiserver.TaskSchedule, handler ReconcileHandler, record *iapiserver.TaskScheduleExecution, runtimeExecutionID string, scheduledAt time.Time) (map[string]any, error) {
	state, err := s.store.GetScheduleReconcileState(ctx, schedule.ID)
	if err != nil {
		return nil, err
	}
	now := imachinery.Now()
	state.LastStartedAt, state.CurrentRuntimeExecutionID = &now, runtimeExecutionID
	state.ResourceVersion++
	record.Status = iapiserver.ScheduleExecutionStatusRunning
	startCtx, startCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	if err := s.store.CompleteScheduleReconcile(startCtx, record, state); err != nil {
		startCancel()
		return nil, err
	}
	startCancel()

	started := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(schedule.ReconcileSpec.OverallTimeoutSeconds)*time.Second)
	result, runErr := handler.Reconcile(runCtx, ReconcileRequest{ScheduleID: schedule.ID, ScheduledAt: scheduledAt, Checkpoint: state.Checkpoint, Config: schedule.ReconcileSpec.Config, MaxParallelism: schedule.ReconcileSpec.MaxParallelism, MaxItemsPerRun: schedule.ReconcileSpec.MaxItemsPerRun, PerItemTimeout: time.Duration(schedule.ReconcileSpec.PerItemTimeoutSeconds) * time.Second, OverallTimeout: time.Duration(schedule.ReconcileSpec.OverallTimeoutSeconds) * time.Second})
	cancel()
	duration := time.Since(started)
	actionsCreated := 0
	actionsComplete := true
	if runErr == nil {
		for _, action := range result.Actions {
			if action == nil || action.IdempotencyScope == "" || action.IdempotencyKey == "" {
				runErr = errors.NewStatus(code.ErrTaskReconcileConfigInvalid, "reconcile action requires a stable idempotency key")
				actionsComplete = false
				break
			}
			action.ProjectID, action.Namespace, action.CreatedBy = schedule.ProjectID, schedule.Namespace, schedule.CreatedBy
			if _, err := s.CreateAtomicTask(ctx, action); err != nil {
				runErr = err
				actionsComplete = false
				break
			}
			actionsCreated++
		}
	}
	checkpointAdvanced := applySafeReconcileCheckpoint(state, checkpointAfterActions(result.NextCheckpoint, actionsComplete))
	summary := iapiserver.ReconcileSummary{Scanned: result.Scanned, Findings: result.Findings, ActionsCreated: actionsCreated, Deferred: result.Deferred, DurationMS: duration.Milliseconds(), CheckpointAdvanced: checkpointAdvanced, CycleCompleted: result.CycleCompleted, Summary: result.Summary}
	record.ReconcileSummary, record.CompletedAt = summary, imachinery.Now()
	state.LastCompletedAt, state.LastSummary, state.CurrentRuntimeExecutionID = &record.CompletedAt, summary, ""
	state.TotalRuns++
	state.TotalScanned += int64(result.Scanned)
	state.TotalFindings += int64(result.Findings)
	state.TotalActionsCreated += int64(actionsCreated)
	state.ResourceVersion++
	if runErr != nil {
		record.Status, record.Reason = iapiserver.ScheduleExecutionStatusFailed, runErr.Error()
		state.ConsecutiveFailures++
	} else {
		record.Status = iapiserver.ScheduleExecutionStatusSuccess
		state.ConsecutiveFailures = 0
	}
	finalizeCtx, finalizeCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer finalizeCancel()
	if err := s.store.CompleteScheduleReconcile(finalizeCtx, record, state); err != nil {
		return nil, err
	}
	reconcileRuns.WithLabelValues(handler.Ref(), record.Status).Inc()
	reconcileScanned.WithLabelValues(handler.Ref()).Add(float64(result.Scanned))
	reconcileFindings.WithLabelValues(handler.Ref()).Add(float64(result.Findings))
	reconcileActions.WithLabelValues(handler.Ref()).Add(float64(actionsCreated))
	reconcileDuration.WithLabelValues(handler.Ref()).Observe(duration.Seconds())
	reconcileCheckpointAge.WithLabelValues(handler.Ref()).Set(0)
	if _, err := s.store.PruneReconcileExecutions(finalizeCtx, schedule.ID, schedule.HistoryRetention, time.Now()); err != nil {
		reconcileRetentionFailures.WithLabelValues("postgresql").Inc()
	}
	output := map[string]any{"schedule_execution_id": record.ID, "status": record.Status, "reconcile_summary": summary}
	if runErr != nil {
		return output, runErr
	}
	return output, nil
}

func checkpointAfterActions(next map[string]any, actionsComplete bool) map[string]any {
	if !actionsComplete {
		return nil
	}
	return next
}

func applySafeReconcileCheckpoint(state *iapiserver.ScheduleReconcileState, next map[string]any) bool {
	if state == nil || next == nil {
		return false
	}
	advanced := !reflect.DeepEqual(next, state.Checkpoint)
	// Handler 即使失败也可以返回最后一个完整分块的安全 checkpoint；非 nil 表示该游标之前的分块均已完整提交。
	state.Checkpoint = next
	return advanced
}

func (s *taskCenterService) GetScheduleReconcileState(ctx context.Context, scheduleID string) (*iapiserver.ScheduleReconcileState, error) {
	schedule, err := s.GetTaskSchedule(ctx, scheduleID)
	if err != nil {
		return nil, err
	}
	if schedule.ExecutionMode != iapiserver.TaskScheduleModeReconcile {
		return nil, errors.NewStatus(code.ErrTaskScheduleInvalid, "schedule is not reconcile mode")
	}
	return s.store.GetScheduleReconcileState(ctx, scheduleID)
}

func validateReconcileSpec(spec *iapiserver.ReconcileSpec, handler ReconcileHandler) error {
	if spec == nil || spec.MaxParallelism < 1 || spec.MaxParallelism > 64 || spec.MaxItemsPerRun < 1 || spec.MaxItemsPerRun > 1000 || spec.PerItemTimeoutSeconds < 1 || spec.PerItemTimeoutSeconds > 30 || spec.OverallTimeoutSeconds < 1 || spec.OverallTimeoutSeconds > 300 || spec.OverallTimeoutSeconds < spec.PerItemTimeoutSeconds {
		return errors.NewStatus(code.ErrTaskReconcileConfigInvalid, "reconcile limits are invalid")
	}
	if err := handler.ValidateConfig(spec.Config); err != nil {
		return errors.NewStatus(code.ErrTaskReconcileConfigInvalid, err.Error())
	}
	return nil
}

func defaultHistoryRetention() iapiserver.HistoryRetention {
	return iapiserver.HistoryRetention{SuccessCount: 4, FailureCount: 20, FailureDurationSeconds: 7 * 24 * 60 * 60, SkippedCount: 4, RuntimeRetentionSeconds: 24 * 60 * 60}
}

func reconcileControllerDefinition() workflowruntime.Definition {
	return workflowruntime.Definition{Name: ReconcileControllerDefinition, Version: ReconcileControllerVersion, Description: "Task Center fixed reconcile controller", TimeoutSeconds: 300, Tasks: []workflowruntime.Task{{Name: ReconcileControllerTask, ReferenceName: "reconcile", Type: "SIMPLE", Input: map[string]any{"arguments": map[string]any{"task_schedule_id": "${workflow.input.task_schedule_id}", "scheduled_at": "${workflow.input._scheduledTime}"}}}}}
}

func (s *taskCenterService) decorateReconcileSchedule(schedule *iapiserver.TaskSchedule) *iapiserver.TaskSchedule {
	if schedule != nil && schedule.ReconcileSpec != nil {
		if handler, ok := s.reconciles.Get(schedule.ReconcileSpec.ReconcileRef); ok {
			schedule.ReconcileSpec.DisplayName = handler.DisplayName()
		}
	}
	return schedule
}
