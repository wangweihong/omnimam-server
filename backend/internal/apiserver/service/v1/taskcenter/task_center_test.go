package taskcenter

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	toolboxerrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
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

type relationStoreStub struct {
	store.TaskCenterStore
	atomicCalls   int
	dagCalls      int
	scheduleCalls int
}

type attemptLogStoreStub struct {
	store.TaskCenterStore
	task            *iapiserver.AtomicTask
	attempt         *iapiserver.TaskAttempt
	attemptErr      error
	getAttemptCalls int
	seenTaskID      string
	seenAttemptID   string
}

type observationStoreStub struct {
	store.TaskCenterStore
	group       *iapiserver.DAGTaskGroup
	tasks       []*iapiserver.AtomicTask
	attempts    []*iapiserver.TaskAttempt
	projections []*iapiserver.RuntimeProjectionEvent
}

type artifactSummaryReaderStub struct {
	summaries map[string]*iapiserver.ArtifactReadableSummary
}

func (s *observationStoreStub) GetDAGTaskGroup(context.Context, string) (*iapiserver.DAGTaskGroup, error) {
	return s.group, nil
}

func (*observationStoreStub) ListScheduleSources(context.Context, string, []string) (map[string]*iapiserver.ScheduleSourceSummary, error) {
	return map[string]*iapiserver.ScheduleSourceSummary{}, nil
}

func (s *observationStoreStub) ListDAGObservationTasks(context.Context, string) ([]*iapiserver.AtomicTask, error) {
	return s.tasks, nil
}

func (s *observationStoreStub) ListAttemptsByTaskIDs(context.Context, []string) ([]*iapiserver.TaskAttempt, error) {
	return s.attempts, nil
}

func (s *observationStoreStub) ListRuntimeProjectionEvents(context.Context, string) ([]*iapiserver.RuntimeProjectionEvent, error) {
	return s.projections, nil
}

func (s *artifactSummaryReaderStub) ResolveArtifactSummaries(context.Context, string, []string) (map[string]*iapiserver.ArtifactReadableSummary, error) {
	return s.summaries, nil
}

func (s *attemptLogStoreStub) GetAtomicTask(context.Context, string) (*iapiserver.AtomicTask, error) {
	return s.task, nil
}

func (*attemptLogStoreStub) ListScheduleSources(context.Context, string, []string) (map[string]*iapiserver.ScheduleSourceSummary, error) {
	return map[string]*iapiserver.ScheduleSourceSummary{}, nil
}

func (s *attemptLogStoreStub) GetAttempt(_ context.Context, taskID, attemptID string) (*iapiserver.TaskAttempt, error) {
	s.getAttemptCalls++
	s.seenTaskID, s.seenAttemptID = taskID, attemptID
	return s.attempt, s.attemptErr
}

type attemptLogRuntimeStub struct {
	workflowruntime.UnavailableRuntime
	logs          []workflowruntime.TaskLogEntry
	err           error
	listCalls     int
	seenRuntimeID string
}

func (s *attemptLogRuntimeStub) ListTaskLogs(_ context.Context, runtimeTaskID string) ([]workflowruntime.TaskLogEntry, error) {
	s.listCalls++
	s.seenRuntimeID = runtimeTaskID
	return s.logs, s.err
}

func (s *relationStoreStub) GetAtomicTasksByIDs(context.Context, []string) ([]*iapiserver.AtomicTask, error) {
	s.atomicCalls++
	root := &iapiserver.AtomicTask{Status: iapiserver.AtomicTaskStatusSuccess, Progress: 1, FunctionRef: "asset.process", ProjectID: "project", Namespace: "default", CreatedBy: "user-1"}
	root.ID = "root-1"
	root.Name = "Process asset"
	return []*iapiserver.AtomicTask{root}, nil
}

func (s *relationStoreStub) GetTaskGroupsByIDs(context.Context, []string) ([]*iapiserver.TaskGroup, error) {
	return []*iapiserver.TaskGroup{}, nil
}

func (s *relationStoreStub) GetDAGTaskGroupsByIDs(context.Context, []string) ([]*iapiserver.DAGTaskGroup, error) {
	s.dagCalls++
	owner := &iapiserver.DAGTaskGroup{Status: iapiserver.TaskGroupStatusRunning, Progress: 0.5, ProjectID: "project", Namespace: "default", CreatedBy: "user-1"}
	owner.ID = "dag-1"
	owner.Name = "Representation build"
	return []*iapiserver.DAGTaskGroup{owner}, nil
}

func (s *relationStoreStub) GetTaskSchedulesByIDs(context.Context, []string) ([]*iapiserver.TaskSchedule, error) {
	s.scheduleCalls++
	return []*iapiserver.TaskSchedule{}, nil
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

func TestListAttemptLogsAuthorizesParentBeforeAttemptLookup(t *testing.T) {
	task := &iapiserver.AtomicTask{CreatedBy: "owner"}
	task.ID = "atomic-1"
	storeStub := &attemptLogStoreStub{task: task}
	runtimeStub := &attemptLogRuntimeStub{}
	service := &taskCenterService{store: storeStub, runtime: runtimeStub}
	user := &iapiserver.User{}
	user.ID = "other-user"
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, user)

	_, err := service.ListAttemptLogs(ctx, &iapiserver.TaskAttemptLogListRequest{AtomicTaskID: task.ID, TaskAttemptID: "attempt-1"})
	if status := toolboxerrors.ToStatus(err); status.Code != code.ErrAtomicTaskNotFound {
		t.Fatalf("code = %d, want %d", status.Code, code.ErrAtomicTaskNotFound)
	}
	if storeStub.getAttemptCalls != 0 || runtimeStub.listCalls != 0 {
		t.Fatalf("unauthorized lookup reached child/runtime: attempt=%d runtime=%d", storeStub.getAttemptCalls, runtimeStub.listCalls)
	}
}

func TestListAttemptLogsValidatesOwnershipAndPaginates(t *testing.T) {
	task := &iapiserver.AtomicTask{CreatedBy: "owner"}
	task.ID = "atomic-1"
	attempt := &iapiserver.TaskAttempt{RuntimeTaskID: "runtime-1"}
	attempt.ID = "attempt-1"
	base := time.Date(2026, time.July, 22, 10, 0, 0, 0, time.UTC)
	storeStub := &attemptLogStoreStub{task: task, attempt: attempt}
	runtimeStub := &attemptLogRuntimeStub{logs: []workflowruntime.TaskLogEntry{
		{Sequence: 1, Source: workflowruntime.TaskLogSourceLifecycle, Level: workflowruntime.TaskLogLevelInfo, Message: "started", OccurredAt: base},
		{Sequence: 2, Source: workflowruntime.TaskLogSourceWorker, Level: workflowruntime.TaskLogLevelInfo, Message: "running", OccurredAt: base.Add(time.Second)},
	}}
	service := &taskCenterService{store: storeStub, runtime: runtimeStub}
	user := &iapiserver.User{}
	user.ID = "owner"
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, user)

	response, err := service.ListAttemptLogs(ctx, &iapiserver.TaskAttemptLogListRequest{
		BasicQueryParam: imachinery.BasicQueryParam{PagingParams: imachinery.PagingParams{PageNum: 1, PageSize: 1}},
		AtomicTaskID:    task.ID,
		TaskAttemptID:   attempt.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if storeStub.seenTaskID != task.ID || storeStub.seenAttemptID != attempt.ID || runtimeStub.seenRuntimeID != attempt.RuntimeTaskID {
		t.Fatalf("lookup ids = %q/%q/%q", storeStub.seenTaskID, storeStub.seenAttemptID, runtimeStub.seenRuntimeID)
	}
	if response.Total != 2 || len(response.Items) != 1 || response.Items[0].Sequence != 2 || response.Items[0].Message != "running" {
		t.Fatalf("response = %#v", response)
	}
}

func TestListAttemptLogsMapsRuntimeFailures(t *testing.T) {
	task := &iapiserver.AtomicTask{CreatedBy: "owner"}
	task.ID = "atomic-1"
	attempt := &iapiserver.TaskAttempt{RuntimeTaskID: "runtime-1"}
	attempt.ID = "attempt-1"
	user := &iapiserver.User{}
	user.ID = "owner"
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, user)

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "retained history missing", err: workflowruntime.ErrTaskLogNotFound, want: code.ErrTaskAttemptLogUnavailable},
		{name: "runtime unavailable", err: workflowruntime.ErrUnavailable, want: code.ErrWorkflowRuntimeUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &taskCenterService{
				store:   &attemptLogStoreStub{task: task, attempt: attempt},
				runtime: &attemptLogRuntimeStub{err: tt.err},
			}
			_, err := service.ListAttemptLogs(ctx, &iapiserver.TaskAttemptLogListRequest{AtomicTaskID: task.ID, TaskAttemptID: attempt.ID})
			if status := toolboxerrors.ToStatus(err); status.Code != tt.want {
				t.Fatalf("code = %d, want %d", status.Code, tt.want)
			}
		})
	}
}

func TestListAttemptLogsReturnsEmptyBeforeRuntimeTaskExists(t *testing.T) {
	task := &iapiserver.AtomicTask{CreatedBy: "owner"}
	task.ID = "atomic-1"
	attempt := &iapiserver.TaskAttempt{}
	attempt.ID = "attempt-1"
	user := &iapiserver.User{}
	user.ID = "owner"
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, user)
	runtimeStub := &attemptLogRuntimeStub{}
	service := &taskCenterService{store: &attemptLogStoreStub{task: task, attempt: attempt}, runtime: runtimeStub}

	response, err := service.ListAttemptLogs(ctx, &iapiserver.TaskAttemptLogListRequest{AtomicTaskID: task.ID, TaskAttemptID: attempt.ID})
	if err != nil {
		t.Fatal(err)
	}
	if response.Total != 0 || len(response.Items) != 0 || runtimeStub.listCalls != 0 {
		t.Fatalf("response = %#v, runtime calls = %d", response, runtimeStub.listCalls)
	}
}

func TestAttemptLogFiltersCursorAndDownloadShareSemantics(t *testing.T) {
	task := &iapiserver.AtomicTask{CreatedBy: "owner"}
	task.ID = "atomic-1"
	attempt := &iapiserver.TaskAttempt{RuntimeTaskID: "runtime-1"}
	attempt.ID = "attempt-1"
	base := time.Date(2026, time.July, 22, 10, 0, 0, 0, time.UTC)
	runtimeStub := &attemptLogRuntimeStub{logs: []workflowruntime.TaskLogEntry{
		{Sequence: 1, Source: workflowruntime.TaskLogSourceLifecycle, Level: workflowruntime.TaskLogLevelInfo, Message: "started", OccurredAt: base},
		{Sequence: 2, Source: workflowruntime.TaskLogSourceWorker, Level: workflowruntime.TaskLogLevelWarn, Message: "waiting for output", OccurredAt: base.Add(time.Second)},
		{Sequence: 3, Source: workflowruntime.TaskLogSourceWorker, Level: workflowruntime.TaskLogLevelError, Message: "output failed", OccurredAt: base.Add(2 * time.Second)},
	}}
	service := &taskCenterService{store: &attemptLogStoreStub{task: task, attempt: attempt}, runtime: runtimeStub}
	user := &iapiserver.User{}
	user.ID = "owner"
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, user)
	req := &iapiserver.TaskAttemptLogListRequest{BasicQueryParam: imachinery.BasicQueryParam{PagingParams: imachinery.PagingParams{PageSize: 1}, Keyword: "output", SortOrder: "asc"}, AtomicTaskID: task.ID, TaskAttemptID: attempt.ID, Sources: "WORKER", Direction: "forward"}

	first, err := service.ListAttemptLogs(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if first.Total != 2 || len(first.Items) != 1 || first.Items[0].Sequence != 2 || first.NextCursor == nil {
		t.Fatalf("first page = %#v", first)
	}
	req.Cursor = *first.NextCursor
	second, err := service.ListAttemptLogs(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.Items[0].Sequence != 3 || second.PreviousCursor == nil {
		t.Fatalf("second page = %#v", second)
	}
	download, err := service.DownloadAttemptLogs(ctx, &iapiserver.TaskAttemptLogDownloadRequest{AtomicTaskID: task.ID, TaskAttemptID: attempt.ID, Keyword: "output", Sources: "WORKER", SortOrder: "asc"})
	if err != nil {
		t.Fatal(err)
	}
	text := string(download)
	if !strings.Contains(text, "waiting for output") || !strings.Contains(text, "output failed") || strings.Contains(text, "started") {
		t.Fatalf("download = %q", text)
	}
}

func TestAggregateDAGNodeUsesActivityThenTerminalPriority(t *testing.T) {
	base := imachinery.NewTime(time.Date(2026, time.July, 22, 10, 0, 0, 0, time.UTC))
	tasks := []*iapiserver.AtomicTask{
		{Status: iapiserver.AtomicTaskStatusSuccess, Progress: 1, StartedAt: base, CompletedAt: imachinery.NewTime(base.Add(time.Second)), Output: map[string]any{"artifact_refs": []any{map[string]any{"artifact_id": "a"}}}},
		{Status: iapiserver.AtomicTaskStatusFailed, Progress: 1, StartedAt: base, CompletedAt: imachinery.NewTime(base.Add(2 * time.Second)), LastError: iapiserver.TaskError{Message: "failed", OccurredAt: imachinery.NewTime(base.Add(2 * time.Second))}},
		{Status: iapiserver.AtomicTaskStatusRunning, Progress: 0.5, StartedAt: base},
	}
	for index, task := range tasks {
		task.ID = fmt.Sprintf("task-%d", index)
	}
	attempts := map[string][]*iapiserver.TaskAttempt{"task-1": {{AttemptNo: 1}, {AttemptNo: 2}}}
	result := aggregateDAGNode(iapiserver.DAGNode{Key: "render", DynamicFork: true}, tasks, attempts)
	if result.Status != iapiserver.AtomicTaskStatusRunning || result.AttemptCount != 2 || result.RetryCount != 1 || result.ArtifactCount != 1 || result.PrimaryAtomicTaskID != nil {
		t.Fatalf("active aggregate = %#v", result)
	}
	tasks[2].Status, tasks[2].Progress, tasks[2].CompletedAt = iapiserver.AtomicTaskStatusSuccess, 1, imachinery.NewTime(base.Add(3*time.Second))
	result = aggregateDAGNode(iapiserver.DAGNode{Key: "render", DynamicFork: true}, tasks, attempts)
	if result.Status != iapiserver.AtomicTaskStatusFailed {
		t.Fatalf("terminal status = %s, want FAILED", result.Status)
	}
}

func TestGetDAGTaskGroupDetailRebuildsEnrichedResult(t *testing.T) {
	group := &iapiserver.DAGTaskGroup{Nodes: []iapiserver.DAGNode{{Key: "render"}}, CreatedBy: "user-1"}
	group.ID = "dag-1"
	task := &iapiserver.AtomicTask{ChildKey: "render", DAGNodeKey: "render", Status: iapiserver.AtomicTaskStatusSuccess, Progress: 1, Output: map[string]any{"artifact_refs": []any{map[string]any{"artifact_id": "artifact-1"}}}}
	task.ID = "task-1"
	reader := &artifactSummaryReaderStub{summaries: map[string]*iapiserver.ArtifactReadableSummary{"artifact-1": {ID: "artifact-1", ArtifactType: "IMAGE"}}}
	service := &taskCenterService{store: &observationStoreStub{group: group, tasks: []*iapiserver.AtomicTask{task}}, artifacts: reader}
	user := &iapiserver.User{}
	user.ID = "user-1"
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, user)

	detail, err := service.GetDAGTaskGroupDetail(ctx, group.ID)
	if err != nil {
		t.Fatal(err)
	}
	output, ok := detail.Result["render"].(map[string]any)
	if !ok {
		t.Fatalf("DAG result = %#v", detail.Result)
	}
	refs := output["artifact_refs"].([]any)
	artifact, ok := refs[0].(map[string]any)["artifact"].(*iapiserver.ArtifactReadableSummary)
	if !ok || artifact.ID != "artifact-1" {
		t.Fatalf("enriched artifact = %#v", refs[0])
	}
}

func TestListDAGTaskGroupEventsRejectsInvertedTimeRange(t *testing.T) {
	group := &iapiserver.DAGTaskGroup{CreatedBy: "user-1"}
	group.ID = "dag-1"
	service := &taskCenterService{store: &observationStoreStub{group: group}}
	user := &iapiserver.User{}
	user.ID = "user-1"
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, user)
	_, err := service.ListDAGTaskGroupEvents(ctx, group.ID, &iapiserver.DAGExecutionEventListRequest{OccurredAfter: "2026-07-22T11:00:00Z", OccurredBefore: "2026-07-22T10:00:00Z"})
	if status := toolboxerrors.ToStatus(err); status.Code != code.ErrValidation {
		t.Fatalf("code = %d, want %d", status.Code, code.ErrValidation)
	}
}

func TestBuildTimelineRowMarksInvertedFactsIncomplete(t *testing.T) {
	base := imachinery.NewTime(time.Date(2026, time.July, 22, 10, 0, 0, 0, time.UTC))
	group := &iapiserver.DAGTaskGroup{TriggeredAt: base}
	task := &iapiserver.AtomicTask{DAGNodeKey: "render", Status: iapiserver.AtomicTaskStatusFailed}
	task.ID = "task-1"
	attempt := &iapiserver.TaskAttempt{AttemptNo: 1, StartedAt: imachinery.NewTime(base.Add(-time.Second)), CompletedAt: imachinery.NewTime(base.Add(-2 * time.Second))}
	row := buildTimelineRow(group, task, []*iapiserver.TaskAttempt{attempt}, nil, nil)
	if row.Complete || len(row.Segments) != 2 || row.Segments[0].Complete || row.Segments[1].Complete {
		t.Fatalf("timeline row = %#v", row)
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

func TestAttachAtomicTaskRelationsUsesBoundedTypedQueries(t *testing.T) {
	stub := &relationStoreStub{}
	service := &taskCenterService{store: stub}
	user := &iapiserver.User{}
	user.ID = "user-1"
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, user)
	task := &iapiserver.AtomicTask{RootTaskID: "root-1", RetryOfTaskID: "root-1", OwnerType: iapiserver.TaskOwnerTypeDAGGroup, OwnerID: "dag-1", ProjectID: "project", Namespace: "default", CreatedBy: "user-1"}
	task.ID = "task-2"

	if err := service.attachAtomicTaskRelations(ctx, []*iapiserver.AtomicTask{task}); err != nil {
		t.Fatal(err)
	}
	if stub.atomicCalls != 1 || stub.dagCalls != 1 || stub.scheduleCalls != 0 {
		t.Fatalf("relation calls = atomic:%d dag:%d schedule:%d", stub.atomicCalls, stub.dagCalls, stub.scheduleCalls)
	}
	if task.RootTask == nil || task.RootTask.Name != "Process asset" || task.RetryOfTask == nil {
		t.Fatalf("task summaries = root:%#v retry:%#v", task.RootTask, task.RetryOfTask)
	}
	if task.Owner == nil || task.Owner.Type != iapiserver.TaskOwnerTypeDAGGroup || task.Owner.Name != "Representation build" {
		t.Fatalf("owner summary = %#v", task.Owner)
	}
}

func TestBatchTaskSummariesFilterInvisibleResources(t *testing.T) {
	stub := &relationStoreStub{}
	service := &taskCenterService{store: stub}
	user := &iapiserver.User{}
	user.ID = "other-user"
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, user)

	atomic, err := service.GetAtomicTaskSummaries(ctx, []string{"root-1", "root-1"})
	if err != nil {
		t.Fatal(err)
	}
	dags, err := service.GetDAGTaskGroupSummaries(ctx, []string{"dag-1", "dag-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(atomic) != 0 || len(dags) != 0 {
		t.Fatalf("invisible task summaries leaked: atomic=%#v dags=%#v", atomic, dags)
	}
	if stub.atomicCalls != 1 || stub.dagCalls != 1 {
		t.Fatalf("batch calls = atomic:%d dag:%d", stub.atomicCalls, stub.dagCalls)
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
