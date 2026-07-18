package taskcenter

import (
	"context"
	"testing"
	"time"

	toolboxerrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

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
