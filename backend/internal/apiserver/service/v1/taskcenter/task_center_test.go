package taskcenter

import (
	"context"
	"testing"
	"time"

	toolboxerrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type scheduleTargetStoreStub struct {
	store.TaskCenterStore
	atomicCalls     int
	groupCalls      int
	dagCalls        int
	seenAtomicList  *iapiserver.AtomicTaskListRequest
	scheduleSources map[string]*iapiserver.ScheduleSourceSummary
}

func (s *scheduleTargetStoreStub) ListAtomicTasks(_ context.Context, req *iapiserver.AtomicTaskListRequest) ([]*iapiserver.AtomicTask, int64, error) {
	s.seenAtomicList = req
	item := &iapiserver.AtomicTask{}
	item.ID = "atomic-1"
	return []*iapiserver.AtomicTask{item}, 1, nil
}

func (s *scheduleTargetStoreStub) ListScheduleSources(context.Context, string, []string) (map[string]*iapiserver.ScheduleSourceSummary, error) {
	return s.scheduleSources, nil
}

func (s *scheduleTargetStoreStub) GetAtomicTasksByIDs(context.Context, []string) ([]*iapiserver.AtomicTask, error) {
	s.atomicCalls++
	item := &iapiserver.AtomicTask{FunctionRef: "asset.thumbnail", Status: iapiserver.AtomicTaskStatusSuccess, Progress: 1}
	item.ID = "atomic-1"
	item.Name = "Generate thumbnail"
	return []*iapiserver.AtomicTask{item}, nil
}

func (s *scheduleTargetStoreStub) GetTaskGroupsByIDs(context.Context, []string) ([]*iapiserver.TaskGroup, error) {
	s.groupCalls++
	return []*iapiserver.TaskGroup{}, nil
}

func (s *scheduleTargetStoreStub) GetDAGTaskGroupsByIDs(context.Context, []string) ([]*iapiserver.DAGTaskGroup, error) {
	s.dagCalls++
	return []*iapiserver.DAGTaskGroup{}, nil
}

func TestValidateDAGRejectsCycle(t *testing.T) {
	service := &taskCenterService{functions: map[string]struct{}{"test.run": {}}}
	_, err := service.validateDAG([]iapiserver.DAGNode{{Key: "a", Task: iapiserver.AtomicTaskTemplate{Key: "a", FunctionRef: "test.run"}}, {Key: "b", Task: iapiserver.AtomicTaskTemplate{Key: "b", FunctionRef: "test.run"}}}, []iapiserver.DAGEdge{{FromNode: "a", ToNode: "b"}, {FromNode: "b", ToNode: "a"}})
	if err == nil {
		t.Fatal("expected cyclic DAG to fail")
	}
	if status := toolboxerrors.ToStatus(err); status.Code != code.ErrTaskDAGCycleDetected {
		t.Fatalf("code = %d, want %d", status.Code, code.ErrTaskDAGCycleDetected)
	}
}

func TestValidateDAGBuildsStableTopologicalLayers(t *testing.T) {
	service := &taskCenterService{functions: map[string]struct{}{"test.run": {}}}
	layers, err := service.validateDAG([]iapiserver.DAGNode{{Key: "b", Task: iapiserver.AtomicTaskTemplate{Key: "b", FunctionRef: "test.run"}}, {Key: "a", Task: iapiserver.AtomicTaskTemplate{Key: "a", FunctionRef: "test.run"}}, {Key: "c", Task: iapiserver.AtomicTaskTemplate{Key: "c", FunctionRef: "test.run"}}}, []iapiserver.DAGEdge{{FromNode: "a", ToNode: "c"}, {FromNode: "b", ToNode: "c"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 2 || len(layers[0]) != 2 || layers[0][0] != "a" || layers[0][1] != "b" || layers[1][0] != "c" {
		t.Fatalf("unexpected layers: %#v", layers)
	}
}

func TestValidateScheduleRequest(t *testing.T) {
	runAt := imachinery.NewTime(time.Now().Add(time.Hour))
	tests := []struct {
		name  string
		req   iapiserver.TaskScheduleCreateRequest
		valid bool
	}{{name: "cron", req: iapiserver.TaskScheduleCreateRequest{TriggerType: iapiserver.TaskScheduleTriggerCron, CronExpression: "*/30 * * * * *", TimeZone: "UTC", Target: iapiserver.ScheduleTarget{Type: iapiserver.TaskScheduleTargetAtomic}}, valid: true}, {name: "run at", req: iapiserver.TaskScheduleCreateRequest{TriggerType: iapiserver.TaskScheduleTriggerRunAt, RunAt: runAt, TimeZone: "Asia/Shanghai", Target: iapiserver.ScheduleTarget{Type: iapiserver.TaskScheduleTargetGroup}}, valid: true}, {name: "five field cron", req: iapiserver.TaskScheduleCreateRequest{TriggerType: iapiserver.TaskScheduleTriggerCron, CronExpression: "* * * * *", TimeZone: "UTC", Target: iapiserver.ScheduleTarget{Type: iapiserver.TaskScheduleTargetAtomic}}}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateScheduleRequest(&tt.req)
			if tt.valid && err != nil {
				t.Fatal(err)
			}
			if !tt.valid && err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestTaskScheduleScopeIncludesSystemSchedulesForSystemAdmin(t *testing.T) {
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, &iapiserver.User{})
	ctx.Value(iapiserver.GinContextKeyUser).(*iapiserver.User).ID = "system-admin"
	req := &iapiserver.TaskScheduleListRequest{}

	applyTaskScheduleScope(ctx, req)

	if req.CreatedBy != "system-admin" || !req.IncludeSystem {
		t.Fatalf("schedule scope = created_by %q, include_system %t", req.CreatedBy, req.IncludeSystem)
	}
	if !canReadTaskSchedule(ctx, &iapiserver.TaskSchedule{CreatedBy: iapiserver.DefaultTaskCenterCreatedBy}) {
		t.Fatal("system administrator cannot read system schedule")
	}
}

func TestTaskScheduleScopeKeepsOrdinaryUserOwnership(t *testing.T) {
	user := &iapiserver.User{}
	user.ID = "user-1"
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, user)
	req := &iapiserver.TaskScheduleListRequest{}

	applyTaskScheduleScope(ctx, req)

	if req.CreatedBy != "user-1" || req.IncludeSystem {
		t.Fatalf("schedule scope = created_by %q, include_system %t", req.CreatedBy, req.IncludeSystem)
	}
	if canReadTaskSchedule(ctx, &iapiserver.TaskSchedule{CreatedBy: iapiserver.DefaultTaskCenterCreatedBy}) {
		t.Fatal("ordinary user can read system schedule")
	}
}

func TestStableRuntimeKeyScopesIdempotency(t *testing.T) {
	got := stableRuntimeKey("project", "namespace", "asset-thumbnail", "thumbnail:a:v1", "fallback")
	if got != "project:namespace:asset-thumbnail:thumbnail:a:v1" {
		t.Fatalf("key = %q", got)
	}
	if got := stableRuntimeKey("project", "namespace", "", "", "fallback"); got != "fallback" {
		t.Fatalf("fallback key = %q", got)
	}
}

func TestAtomicDefinitionContainsOnlyAtomicHandler(t *testing.T) {
	task := &iapiserver.AtomicTask{FunctionRef: "test.run", ChildKey: "node", Arguments: map[string]any{"value": 1}, TimeoutPolicy: iapiserver.TimeoutPolicy{OverallTimeoutSeconds: 10}}
	task.ID = "task-1"
	definition := atomicDefinition(task)
	if len(definition.Tasks) != 1 || definition.Tasks[0].Name != "test.run" || definition.Tasks[0].Type != "SIMPLE" {
		t.Fatalf("definition = %#v", definition)
	}
}

func TestScheduleLauncherPassesSchedulerMetadataAsWorkerArguments(t *testing.T) {
	schedule := &iapiserver.TaskSchedule{TriggerType: iapiserver.TaskScheduleTriggerCron}
	definition := scheduleLauncherDefinition(schedule)
	if len(definition.Tasks) != 1 {
		t.Fatalf("tasks = %d, want 1", len(definition.Tasks))
	}
	arguments, ok := definition.Tasks[0].Input["arguments"].(map[string]any)
	if !ok {
		t.Fatalf("worker input does not contain arguments: %#v", definition.Tasks[0].Input)
	}
	if arguments["task_schedule_id"] != "${workflow.input.task_schedule_id}" || arguments["scheduled_at"] != "${workflow.input._scheduledTime}" {
		t.Fatalf("arguments = %#v", arguments)
	}
}

func TestScheduleTemplateSummary(t *testing.T) {
	schedule := &iapiserver.TaskSchedule{Target: iapiserver.ScheduleTarget{Type: iapiserver.TaskScheduleTargetAtomic, Template: map[string]any{"key": "thumbnail", "name": "Generate thumbnail", "function_ref": "asset.thumbnail", "application_run_id": "run-1"}}}
	summary := scheduleTemplateSummary(schedule)
	if summary.Type != iapiserver.TaskScheduleTargetAtomic || summary.Name != "Generate thumbnail" || summary.FunctionRef != "asset.thumbnail" || summary.ApplicationRunID != "run-1" {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestAttachExecutionTargetsUsesBoundedBatchQueriesAndFallback(t *testing.T) {
	stub := &scheduleTargetStoreStub{}
	service := &taskCenterService{store: stub}
	schedule := &iapiserver.TaskSchedule{Target: iapiserver.ScheduleTarget{Type: iapiserver.TaskScheduleTargetDAG, Template: map[string]any{"name": "Engine health", "nodes": []any{map[string]any{"key": "plan"}}}}}
	executions := []*iapiserver.TaskScheduleExecution{
		{TargetType: iapiserver.TaskScheduleTargetAtomic, TargetID: "atomic-1"},
		{TargetType: iapiserver.TaskScheduleTargetGroup, Status: iapiserver.ScheduleExecutionStatusTriggerFailed, Reason: "invalid target"},
		{TargetType: iapiserver.TaskScheduleTargetDAG, TargetID: "missing-dag"},
	}

	if err := service.attachExecutionTargets(context.Background(), schedule, executions); err != nil {
		t.Fatal(err)
	}
	if stub.atomicCalls != 1 || stub.groupCalls != 1 || stub.dagCalls != 1 {
		t.Fatalf("batch calls = atomic:%d group:%d dag:%d", stub.atomicCalls, stub.groupCalls, stub.dagCalls)
	}
	if executions[0].TargetSummary == nil || executions[0].TargetSummary.Name != "Generate thumbnail" || executions[0].TargetSummary.Status != iapiserver.AtomicTaskStatusSuccess {
		t.Fatalf("actual target summary = %#v", executions[0].TargetSummary)
	}
	if executions[1].TargetSummary == nil || executions[1].TargetSummary.ID != "" || executions[1].TargetSummary.Name != "Engine health" {
		t.Fatalf("trigger failure fallback = %#v", executions[1].TargetSummary)
	}
	if executions[2].TargetSummary == nil || executions[2].TargetSummary.ID != "missing-dag" || executions[2].TargetSummary.Name != "Engine health" {
		t.Fatalf("missing target fallback = %#v", executions[2].TargetSummary)
	}
}

func TestListAtomicTasksIncludesScheduleSourceAndKeepsUserScope(t *testing.T) {
	source := &iapiserver.ScheduleSourceSummary{ScheduleID: "schedule-1", ScheduleName: "Nightly", ScheduleExecutionID: "execution-1"}
	stub := &scheduleTargetStoreStub{scheduleSources: map[string]*iapiserver.ScheduleSourceSummary{"atomic-1": source}}
	service := &taskCenterService{store: stub}
	user := &iapiserver.User{}
	user.ID = "user-1"
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, user)

	response, err := service.ListAtomicTasks(ctx, &iapiserver.AtomicTaskListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if stub.seenAtomicList == nil || stub.seenAtomicList.CreatedBy != "user-1" || stub.seenAtomicList.IncludeSystem {
		t.Fatalf("atomic list scope = %#v", stub.seenAtomicList)
	}
	if len(response.Items) != 1 || response.Items[0].ScheduleSource != source {
		t.Fatalf("atomic list response = %#v", response)
	}
}

func TestListAtomicTasksIncludesSystemRunsForSystemAdmin(t *testing.T) {
	stub := &scheduleTargetStoreStub{}
	service := &taskCenterService{store: stub}
	admin := &iapiserver.User{}
	admin.ID = "system-admin"
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, admin)

	if _, err := service.ListAtomicTasks(ctx, &iapiserver.AtomicTaskListRequest{}); err != nil {
		t.Fatal(err)
	}
	if stub.seenAtomicList == nil || !stub.seenAtomicList.IncludeSystem {
		t.Fatalf("system admin atomic list scope = %#v", stub.seenAtomicList)
	}
	if !canReadTaskCreatedBy(ctx, iapiserver.DefaultTaskCenterCreatedBy) {
		t.Fatal("system administrator cannot open system schedule target")
	}
}

func TestDynamicDAGNodePassesPlannerOutputToFork(t *testing.T) {
	task := &iapiserver.AtomicTask{FunctionRef: "engine.plan", ChildKey: "plan"}
	task.ID = "planner-task"
	node := iapiserver.DAGNode{Key: "plan", DynamicFork: true, MaxDynamicTasks: 1000}
	tasks := runtimeTasksForDAGNode(node, task)
	if len(tasks) != 3 || tasks[1].Type != "FORK_JOIN_DYNAMIC" || tasks[2].Type != "JOIN" {
		t.Fatalf("dynamic tasks = %#v", tasks)
	}
	if tasks[1].DynamicTasksParam != "dynamic_tasks" || tasks[1].DynamicInputParam != "dynamic_inputs" {
		t.Fatalf("dynamic fork parameters = %#v", tasks[1])
	}
	if tasks[1].Input["dynamic_tasks"] != "${plan_plannertask.output.dynamic_tasks}" || tasks[1].Input["dynamic_inputs"] != "${plan_plannertask.output.dynamic_inputs}" {
		t.Fatalf("dynamic fork input = %#v", tasks[1].Input)
	}
}
