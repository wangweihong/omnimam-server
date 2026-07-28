package taskcenter

import (
	"context"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
)

type manualScheduleStoreStub struct {
	store.TaskCenterStore
	schedule *iapiserver.TaskSchedule
	record   *iapiserver.TaskScheduleExecution
}

func (s *manualScheduleStoreStub) GetTaskSchedule(context.Context, string) (*iapiserver.TaskSchedule, error) {
	return s.schedule, nil
}

func (s *manualScheduleStoreStub) ListLatestScheduleExecutions(context.Context, []string) (map[string]*iapiserver.TaskScheduleExecution, error) {
	return map[string]*iapiserver.TaskScheduleExecution{}, nil
}

func (s *manualScheduleStoreStub) AcquireScheduleExecution(_ context.Context, record *iapiserver.TaskScheduleExecution) (*iapiserver.TaskScheduleExecution, bool, error) {
	if s.record != nil && s.record.IdempotencyKey == record.IdempotencyKey {
		return s.record, false, nil
	}
	s.record = record
	return record, true, nil
}

func (s *manualScheduleStoreStub) UpdateScheduleExecution(_ context.Context, record *iapiserver.TaskScheduleExecution) (*iapiserver.TaskScheduleExecution, error) {
	s.record = record
	return record, nil
}

func (*manualScheduleStoreStub) GetAtomicTasksByIDs(context.Context, []string) ([]*iapiserver.AtomicTask, error) {
	return nil, nil
}

func (*manualScheduleStoreStub) GetTaskGroupsByIDs(context.Context, []string) ([]*iapiserver.TaskGroup, error) {
	return nil, nil
}

func (*manualScheduleStoreStub) GetDAGTaskGroupsByIDs(context.Context, []string) ([]*iapiserver.DAGTaskGroup, error) {
	return nil, nil
}

type manualScheduleRuntimeStub struct {
	workflowruntime.UnavailableRuntime
	definition workflowruntime.Definition
	start      workflowruntime.StartRequest
	starts     int
	err        error
}

func (r *manualScheduleRuntimeStub) RegisterDefinition(_ context.Context, definition workflowruntime.Definition) (workflowruntime.Binding, error) {
	r.definition = definition
	if r.err != nil {
		return workflowruntime.Binding{}, r.err
	}
	return workflowruntime.Binding{DefinitionName: definition.Name, DefinitionVersion: definition.Version}, nil
}

func (r *manualScheduleRuntimeStub) StartExecution(_ context.Context, start workflowruntime.StartRequest) (workflowruntime.Execution, error) {
	r.starts++
	r.start = start
	if r.err != nil {
		return workflowruntime.Execution{}, r.err
	}
	return workflowruntime.Execution{ID: "runtime-manual-1"}, nil
}

func TestRunTaskScheduleIsIdempotentAndPreservesPausedState(t *testing.T) {
	schedule := &iapiserver.TaskSchedule{
		ExecutionMode:  iapiserver.TaskScheduleModeMaterialized,
		ManagementMode: iapiserver.TaskScheduleManagementUser,
		Status:         iapiserver.TaskScheduleStatusPaused,
		Target: iapiserver.ScheduleTarget{
			Type:     iapiserver.TaskScheduleTargetAtomic,
			Template: map[string]any{"name": "manual target"},
		},
		ProjectID: "project", Namespace: "namespace", CreatedBy: iapiserver.DefaultTaskCenterCreatedBy,
	}
	schedule.ID, schedule.Name = "schedule-1", "Schedule"
	storage := &manualScheduleStoreStub{schedule: schedule}
	runtime := &manualScheduleRuntimeStub{}
	service := &taskCenterService{store: storage, runtime: runtime, functions: map[string]struct{}{}, reconciles: NewReconcileRegistry()}
	req := &iapiserver.TaskScheduleRunRequest{IdempotencyKey: "d871c8ce-f78e-4b23-ae61-981f343e24c8"}

	first, err := service.RunTaskSchedule(context.Background(), schedule.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.RunTaskSchedule(context.Background(), schedule.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || runtime.starts != 1 {
		t.Fatalf("ids = %q/%q, starts = %d", first.ID, second.ID, runtime.starts)
	}
	if first.TriggerSource != iapiserver.ScheduleExecutionTriggerManual || first.RuntimeExecutionID != "runtime-manual-1" {
		t.Fatalf("execution = %#v", first)
	}
	if runtime.definition.Name != ManualScheduleControllerDefinition || runtime.start.IdempotencyKey != first.ID {
		t.Fatalf("definition = %q, start = %#v", runtime.definition.Name, runtime.start)
	}
	if schedule.Status != iapiserver.TaskScheduleStatusPaused {
		t.Fatalf("schedule status changed to %q", schedule.Status)
	}
}

func TestRunTaskSchedulePersistsRuntimeStartFailure(t *testing.T) {
	schedule := &iapiserver.TaskSchedule{
		ExecutionMode:  iapiserver.TaskScheduleModeMaterialized,
		ManagementMode: iapiserver.TaskScheduleManagementUser,
		Status:         iapiserver.TaskScheduleStatusActive,
		Target:         iapiserver.ScheduleTarget{Type: iapiserver.TaskScheduleTargetAtomic},
		CreatedBy:      iapiserver.DefaultTaskCenterCreatedBy,
	}
	schedule.ID = "schedule-1"
	storage := &manualScheduleStoreStub{schedule: schedule}
	runtime := &manualScheduleRuntimeStub{err: workflowruntime.ErrUnavailable}
	service := &taskCenterService{store: storage, runtime: runtime, functions: map[string]struct{}{}, reconciles: NewReconcileRegistry()}

	record, err := service.RunTaskSchedule(context.Background(), schedule.ID, &iapiserver.TaskScheduleRunRequest{
		IdempotencyKey: "9d81b682-bc11-4d39-846c-b594748c01dc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Status != iapiserver.ScheduleExecutionStatusTriggerFailed || record.Reason == "" || record.CompletedAt.IsZero() {
		t.Fatalf("record = %#v", record)
	}
}
