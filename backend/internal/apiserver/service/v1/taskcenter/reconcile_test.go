package taskcenter

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	toolboxerrors "github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/taskname"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type reconcileHandlerStub struct{ ref string }

type misfireStoreStub struct {
	store.TaskCenterStore
	schedule *iapiserver.TaskSchedule
	existing *iapiserver.TaskScheduleExecution
}

func (s *misfireStoreStub) GetTaskSchedule(context.Context, string) (*iapiserver.TaskSchedule, error) {
	return s.schedule, nil
}

func (s *misfireStoreStub) GetScheduleExecutionAt(context.Context, string, time.Time) (*iapiserver.TaskScheduleExecution, error) {
	return s.existing, nil
}

type systemScheduleStoreStub struct {
	store.TaskCenterStore
	schedule        *iapiserver.TaskSchedule
	ensuredSchedule *iapiserver.TaskSchedule
}

type reconcileRecoveryStoreStub struct {
	store.TaskCenterStore
	state     *iapiserver.ScheduleReconcileState
	execution *iapiserver.TaskScheduleExecution
	completed *iapiserver.TaskScheduleExecution
}

type projectionStoreStub struct {
	store.TaskCenterStore
	applied  bool
	task     *iapiserver.AtomicTask
	attempts []*iapiserver.TaskAttempt
}

func (s *projectionStoreStub) ApplyRuntimeProjection(_ context.Context, task *iapiserver.AtomicTask, attempts []*iapiserver.TaskAttempt, _ *iapiserver.RuntimeProjectionEvent) (bool, error) {
	s.applied, s.task, s.attempts = true, task, attempts
	return true, nil
}

type failingTaskLogRuntime struct {
	workflowruntime.UnavailableRuntime
	appendCalls int
}

func (r *failingTaskLogRuntime) AppendTaskLog(context.Context, string, workflowruntime.TaskLogEntry) error {
	r.appendCalls++
	return workflowruntime.ErrUnavailable
}

func (s *reconcileRecoveryStoreStub) GetScheduleReconcileState(context.Context, string) (*iapiserver.ScheduleReconcileState, error) {
	return s.state, nil
}
func (s *reconcileRecoveryStoreStub) CompleteScheduleReconcile(_ context.Context, execution *iapiserver.TaskScheduleExecution, _ *iapiserver.ScheduleReconcileState) error {
	s.completed = execution
	return nil
}
func (s *reconcileRecoveryStoreStub) GetScheduleExecution(context.Context, string) (*iapiserver.TaskScheduleExecution, error) {
	return s.execution, nil
}
func (*reconcileRecoveryStoreStub) WithScheduleReconcileLock(_ context.Context, _ string, fn func() error) (bool, error) {
	return true, fn()
}

func (s *systemScheduleStoreStub) GetTaskSchedule(context.Context, string) (*iapiserver.TaskSchedule, error) {
	return s.schedule, nil
}
func (s *systemScheduleStoreStub) EnsureSystemTaskSchedule(_ context.Context, schedule *iapiserver.TaskSchedule, _ *iapiserver.ScheduleReconcileState) (*iapiserver.TaskSchedule, bool, error) {
	s.ensuredSchedule = schedule
	return schedule, true, nil
}
func (*systemScheduleStoreStub) ListLatestScheduleExecutions(context.Context, []string) (map[string]*iapiserver.TaskScheduleExecution, error) {
	return map[string]*iapiserver.TaskScheduleExecution{}, nil
}

func (h reconcileHandlerStub) Ref() string                       { return h.ref }
func (reconcileHandlerStub) DisplayName() string                 { return "Test reconcile" }
func (reconcileHandlerStub) ValidateConfig(map[string]any) error { return nil }
func (reconcileHandlerStub) Reconcile(context.Context, ReconcileRequest) (ReconcileResult, error) {
	return ReconcileResult{}, nil
}

func TestReconcileRegistryRejectsDuplicateRef(t *testing.T) {
	registry := NewReconcileRegistry()
	if err := registry.Register(reconcileHandlerStub{ref: "test.reconcile"}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(reconcileHandlerStub{ref: "test.reconcile"}); err == nil {
		t.Fatal("expected duplicate ref error")
	}
	if handler, ok := registry.Get("test.reconcile"); !ok || handler.DisplayName() != "Test reconcile" {
		t.Fatalf("handler = %#v, exists = %t", handler, ok)
	}
}

func TestEnsureSystemReconcileScheduleResolvesLocalizedName(t *testing.T) {
	registry := NewReconcileRegistry()
	if err := registry.Register(reconcileHandlerStub{ref: "test.reconcile"}); err != nil {
		t.Fatal(err)
	}
	storage := &systemScheduleStoreStub{}
	service := &taskCenterService{store: storage, runtime: workflowruntime.NewFake(), reconciles: registry}
	schedule := &iapiserver.TaskSchedule{
		ObjectMeta:     imachinery.ObjectMeta{Name: "internal-key"},
		TaskNameMeta:   iapiserver.TaskNameMeta{NameSource: iapiserver.TaskNameSourceSystem, SystemNameKey: taskname.EngineHealthReconcile},
		SystemKey:      "test.reconcile",
		CronExpression: "0 0 3 * * *",
		TimeZone:       "UTC",
		ReconcileSpec:  &iapiserver.ReconcileSpec{ReconcileRef: "test.reconcile", Config: map[string]any{}, MaxParallelism: 1, MaxItemsPerRun: 1, PerItemTimeoutSeconds: 1, OverallTimeoutSeconds: 1},
	}
	created, err := service.EnsureSystemReconcileSchedule(context.Background(), schedule)
	if err != nil {
		t.Fatal(err)
	}
	if storage.ensuredSchedule == nil || created.Name != "Application engine health reconcile" {
		t.Fatalf("created schedule = %#v", created)
	}
	if created.NameI18n[taskname.LanguageChinese] != "应用引擎健康巡检" {
		t.Fatalf("name_i18n = %#v", created.NameI18n)
	}
}

func TestProjectionKeepsCanceledResultWhenTerminalLogWriteFails(t *testing.T) {
	storeStub := &projectionStoreStub{}
	runtimeStub := &failingTaskLogRuntime{}
	reconciler := &Reconciler{store: storeStub, runtime: runtimeStub}
	task := &iapiserver.AtomicTask{Status: iapiserver.AtomicTaskStatusRunning, CurrentAttempt: 1}
	task.ID = "atomic-1"
	now := time.Date(2026, time.July, 22, 10, 0, 0, 0, time.UTC)
	execution := workflowruntime.Execution{ID: "execution-1", Tasks: []workflowruntime.ExecutionTask{{
		ID: "runtime-task-1", Status: "TERMINATED", Input: map[string]any{"atomic_task_id": task.ID},
		Output: map[string]any{}, StartedAt: now, CompletedAt: now.Add(time.Second),
	}}}

	applied, err := reconciler.project(context.Background(), task, execution)
	if err != nil {
		t.Fatal(err)
	}
	if !applied || !storeStub.applied || storeStub.task.Status != iapiserver.AtomicTaskStatusCanceled {
		t.Fatalf("projection applied=%t task=%#v", applied, storeStub.task)
	}
	if runtimeStub.appendCalls != 1 {
		t.Fatalf("append calls = %d", runtimeStub.appendCalls)
	}
	if len(storeStub.attempts) != 1 || storeStub.attempts[0].LogsRef != iapiserver.TaskAttemptLogsRef(storeStub.attempts[0].ID) {
		t.Fatalf("attempts = %#v", storeStub.attempts)
	}
}

func TestScheduleTriggerMisfireBoundary(t *testing.T) {
	scheduledAt := time.Date(2026, time.July, 18, 16, 44, 0, 0, time.UTC)
	tests := []struct {
		name        string
		scheduledAt time.Time
		triggeredAt time.Time
		want        bool
	}{
		{name: "normal jitter", scheduledAt: scheduledAt, triggeredAt: scheduledAt.Add(time.Second)},
		{name: "grace boundary", scheduledAt: scheduledAt, triggeredAt: scheduledAt.Add(scheduleMisfireGrace)},
		{name: "delayed scheduler recovery", scheduledAt: scheduledAt, triggeredAt: scheduledAt.Add(scheduleMisfireGrace + time.Nanosecond), want: true},
		{name: "clock skew before schedule", scheduledAt: scheduledAt, triggeredAt: scheduledAt.Add(-time.Second)},
		{name: "missing schedule time", triggeredAt: scheduledAt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ScheduleTriggerMisfired(tt.scheduledAt, tt.triggeredAt); got != tt.want {
				t.Fatalf("ScheduleTriggerMisfired() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestRunScheduleReconcileSkipsFirstMisfireWithoutHistory(t *testing.T) {
	schedule := &iapiserver.TaskSchedule{ExecutionMode: iapiserver.TaskScheduleModeReconcile, ReconcileSpec: &iapiserver.ReconcileSpec{ReconcileRef: "test.reconcile"}}
	schedule.ID = "schedule-1"
	registry := NewReconcileRegistry()
	if err := registry.Register(reconcileHandlerStub{ref: "test.reconcile"}); err != nil {
		t.Fatal(err)
	}
	service := &taskCenterService{store: &misfireStoreStub{schedule: schedule}, reconciles: registry}
	output, err := service.RunScheduleReconcile(context.Background(), schedule.ID, "runtime-1", time.Now().Add(-scheduleMisfireGrace-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if output["status"] != iapiserver.TaskSchedulePolicySkip {
		t.Fatalf("output = %#v", output)
	}
}

func TestValidateReconcileSpecBoundaries(t *testing.T) {
	handler := reconcileHandlerStub{ref: "test.reconcile"}
	tests := []struct {
		name  string
		spec  iapiserver.ReconcileSpec
		valid bool
	}{
		{name: "defaults", spec: iapiserver.ReconcileSpec{ReconcileRef: handler.Ref(), MaxParallelism: 16, MaxItemsPerRun: 1000, PerItemTimeoutSeconds: 4, OverallTimeoutSeconds: 5}, valid: true},
		{name: "parallelism too large", spec: iapiserver.ReconcileSpec{ReconcileRef: handler.Ref(), MaxParallelism: 65, MaxItemsPerRun: 1000, PerItemTimeoutSeconds: 4, OverallTimeoutSeconds: 5}},
		{name: "overall below item", spec: iapiserver.ReconcileSpec{ReconcileRef: handler.Ref(), MaxParallelism: 1, MaxItemsPerRun: 1, PerItemTimeoutSeconds: 5, OverallTimeoutSeconds: 4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReconcileSpec(&tt.spec, handler)
			if tt.valid && err != nil {
				t.Fatal(err)
			}
			if !tt.valid && err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestReconcileControllerDefinitionIsFixed(t *testing.T) {
	definition := reconcileControllerDefinition()
	if definition.Name != ReconcileControllerDefinition || definition.Version != 1 || len(definition.Tasks) != 1 || definition.Tasks[0].Name != ReconcileControllerTask {
		t.Fatalf("unexpected definition: %#v", definition)
	}
}

func TestApplySafeReconcileCheckpointPersistsCompletedChunksOnFailure(t *testing.T) {
	state := &iapiserver.ScheduleReconcileState{Checkpoint: map[string]any{"engine_instance_id": "engine-02"}}
	if advanced := applySafeReconcileCheckpoint(state, map[string]any{"engine_instance_id": "engine-04"}); !advanced {
		t.Fatal("completed chunk checkpoint was not marked advanced")
	}
	if got := state.Checkpoint["engine_instance_id"]; got != "engine-04" {
		t.Fatalf("checkpoint = %v", got)
	}
	if advanced := applySafeReconcileCheckpoint(state, nil); advanced {
		t.Fatal("nil checkpoint must not be treated as safe progress")
	}
	if got := state.Checkpoint["engine_instance_id"]; got != "engine-04" {
		t.Fatalf("nil result replaced safe checkpoint: %v", got)
	}
}

func TestReconcileActionFailureDoesNotAdvanceCheckpoint(t *testing.T) {
	state := &iapiserver.ScheduleReconcileState{Checkpoint: map[string]any{"cursor": "before"}}
	next := map[string]any{"cursor": "after"}
	if advanced := applySafeReconcileCheckpoint(state, checkpointAfterActions(next, false)); advanced {
		t.Fatal("failed repair action advanced checkpoint")
	}
	if state.Checkpoint["cursor"] != "before" {
		t.Fatalf("checkpoint = %#v", state.Checkpoint)
	}
	if advanced := applySafeReconcileCheckpoint(state, checkpointAfterActions(next, true)); !advanced {
		t.Fatal("completed repair actions did not advance checkpoint")
	}
}

func TestReconcileControllerRetryResumesOnlySameRuntimeExecution(t *testing.T) {
	existing := &iapiserver.TaskScheduleExecution{ExecutionMode: iapiserver.TaskScheduleModeReconcile, RuntimeExecutionID: "runtime-1", Status: iapiserver.ScheduleExecutionStatusRunning}
	if !canResumeScheduleExecution(existing, &iapiserver.TaskScheduleExecution{ExecutionMode: iapiserver.TaskScheduleModeReconcile, RuntimeExecutionID: "runtime-1"}) {
		t.Fatal("same Conductor execution must resume an interrupted reconcile round")
	}
	if canResumeScheduleExecution(existing, &iapiserver.TaskScheduleExecution{ExecutionMode: iapiserver.TaskScheduleModeReconcile, RuntimeExecutionID: "runtime-2"}) {
		t.Fatal("different runtime execution must be treated as overlap")
	}
	existing.Status = iapiserver.ScheduleExecutionStatusFailed
	if canResumeScheduleExecution(existing, &iapiserver.TaskScheduleExecution{ExecutionMode: iapiserver.TaskScheduleModeReconcile, RuntimeExecutionID: "runtime-1"}) {
		t.Fatal("terminal reconcile execution must not resume")
	}
}

func TestRecoverReconcileExecutionMarksMissingRuntimeFailed(t *testing.T) {
	state := &iapiserver.ScheduleReconcileState{ScheduleID: "schedule-1", CurrentRuntimeExecutionID: "missing-runtime"}
	execution := &iapiserver.TaskScheduleExecution{ScheduleID: "schedule-1", RuntimeExecutionID: "missing-runtime", Status: iapiserver.ScheduleExecutionStatusRunning}
	execution.ID = "execution-1"
	storage := &reconcileRecoveryStoreStub{state: state, execution: execution}
	reconciler := &Reconciler{store: storage, runtime: workflowruntime.NewFake()}
	if err := reconciler.recoverReconcileExecution(context.Background(), execution); err != nil {
		t.Fatal(err)
	}
	if storage.completed == nil || storage.completed.Status != iapiserver.ScheduleExecutionStatusFailed {
		t.Fatalf("execution = %#v", storage.completed)
	}
	if state.CurrentRuntimeExecutionID != "" || state.TotalRuns != 1 || state.ConsecutiveFailures != 1 {
		t.Fatalf("state = %#v", state)
	}
}

func TestScheduleReconcileStateDoesNotExposeCheckpoint(t *testing.T) {
	data, err := json.Marshal(&iapiserver.ScheduleReconcileState{ScheduleID: "schedule-1", Checkpoint: map[string]any{"engine_instance_id": "secret-cursor"}})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" || containsJSONField(data, "checkpoint") {
		t.Fatalf("checkpoint leaked: %s", data)
	}
}

func TestReconcileScheduleDoesNotExposeMaterializedTarget(t *testing.T) {
	schedule := &iapiserver.TaskSchedule{ExecutionMode: iapiserver.TaskScheduleModeReconcile}
	if summary := scheduleTemplateSummary(schedule); summary != nil {
		t.Fatalf("target summary = %#v", summary)
	}
	data, err := json.Marshal(schedule)
	if err != nil {
		t.Fatal(err)
	}
	if containsJSONField(data, "target") {
		t.Fatalf("materialized target leaked: %s", data)
	}
}

func TestSystemScheduleRejectsProtectedFieldUpdate(t *testing.T) {
	schedule := &iapiserver.TaskSchedule{ExecutionMode: iapiserver.TaskScheduleModeReconcile, ManagementMode: iapiserver.TaskScheduleManagementSystem, CreatedBy: iapiserver.DefaultTaskCenterCreatedBy, ReconcileSpec: &iapiserver.ReconcileSpec{ReconcileRef: "test.reconcile", MaxParallelism: 1, MaxItemsPerRun: 1, PerItemTimeoutSeconds: 1, OverallTimeoutSeconds: 1}}
	schedule.ID, schedule.Status = "schedule-1", iapiserver.TaskScheduleStatusActive
	registry := NewReconcileRegistry()
	_ = registry.Register(reconcileHandlerStub{ref: "test.reconcile"})
	service := &taskCenterService{store: &systemScheduleStoreStub{schedule: schedule}, runtime: workflowruntime.NewFake(), reconciles: registry, functions: map[string]struct{}{}}
	admin := &iapiserver.User{}
	admin.ID = "system-admin"
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, admin)
	name := "renamed"
	_, err := service.UpdateTaskSchedule(ctx, &iapiserver.TaskScheduleUpdateRequest{ID: schedule.ID, Name: &name})
	if err == nil {
		t.Fatal("expected protected field update to fail")
	}
	if status := toolboxerrors.ToStatus(err); status.Code != code.ErrTaskSystemScheduleOperationRestricted {
		t.Fatalf("code = %d", status.Code)
	}
}

func containsJSONField(data []byte, field string) bool {
	var value map[string]any
	_ = json.Unmarshal(data, &value)
	_, ok := value[field]
	return ok
}

func TestDynamicTasksFromExecutionMaterializesActualChildren(t *testing.T) {
	group := &iapiserver.DAGTaskGroup{
		Nodes:     []iapiserver.DAGNode{{Key: "images", DynamicFork: true}},
		ProjectID: "project", Namespace: "default", CreatedBy: "user-1",
	}
	group.ID = "dag-1"
	execution := workflowruntime.Execution{ID: "runtime-1", Tasks: []workflowruntime.ExecutionTask{{
		ID: "runtime-task-1", Status: "COMPLETED", Output: map[string]any{"artifact_refs": []any{}},
		Input: map[string]any{"atomic_task_id": "actual-1", "dag_node_key": "images", "function_ref": "image.generate", "child_key": "image_0", "child_order": float64(2), "arguments": map[string]any{"prompt": "test"}},
	}}}

	tasks := dynamicTasksFromExecution(group, nil, execution)
	if len(tasks) != 1 {
		t.Fatalf("tasks = %#v", tasks)
	}
	actual := tasks[0]
	if actual.ID != "actual-1" || actual.DAGNodeKey != "images" || actual.ChildKey != "images:image_0" || actual.ChildOrder != 2 || actual.Status != iapiserver.AtomicTaskStatusSuccess {
		t.Fatalf("actual task = %#v", actual)
	}
}
