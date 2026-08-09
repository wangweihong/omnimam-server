package workflowcanvas

import (
	"context"
	"testing"

	toolboxerrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	taskcentersvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type canvasRelationStore struct {
	store.WorkflowCanvasStore
	canvasCalls  int
	versionCalls int
	retryCalls   int
	nodeRunCalls int
}

func (s *canvasRelationStore) GetWorkflowCanvas(context.Context, string) (*iapiserver.WorkflowCanvas, error) {
	item := &iapiserver.WorkflowCanvas{CreatedBy: iapiserver.DefaultTaskCenterCreatedBy}
	item.ID = "canvas-1"
	return item, nil
}

func (s *canvasRelationStore) GetWorkflowCanvasesByIDs(context.Context, []string) ([]*iapiserver.WorkflowCanvas, error) {
	s.canvasCalls++
	item := &iapiserver.WorkflowCanvas{Visibility: iapiserver.CanvasVisibilityPrivate}
	item.ID, item.Name = "canvas-1", "Campaign"
	return []*iapiserver.WorkflowCanvas{item}, nil
}

func (s *canvasRelationStore) GetCanvasVersionsByIDs(context.Context, []string) ([]*iapiserver.CanvasVersion, error) {
	s.versionCalls++
	item := &iapiserver.CanvasVersion{Version: 3, ContentDigest: "sha256:version"}
	item.ID = "version-1"
	return []*iapiserver.CanvasVersion{item}, nil
}

func (s *canvasRelationStore) GetCanvasVersion(context.Context, string) (*iapiserver.CanvasVersion, error) {
	item := &iapiserver.CanvasVersion{
		CanvasID: "canvas-1",
		GraphSnapshot: iapiserver.WorkflowCanvasGraph{
			Nodes: []iapiserver.WorkflowCanvasNode{{NodeID: "failed"}, {NodeID: "skipped"}},
			Edges: []iapiserver.WorkflowCanvasEdge{{SourceNodeID: "failed", TargetNodeID: "skipped"}},
		},
	}
	item.ID = "version-1"
	return item, nil
}

func (s *canvasRelationStore) GetWorkflowCanvasRunsByIDs(context.Context, []string) ([]*iapiserver.WorkflowCanvasRun, error) {
	s.retryCalls++
	item := &iapiserver.WorkflowCanvasRun{
		Status:    iapiserver.CanvasRunStatusFailed,
		Progress:  0.5,
		ProjectID: "project",
		Namespace: "default",
		CreatedBy: "user-1",
	}
	item.ID = "source-1"
	return []*iapiserver.WorkflowCanvasRun{item}, nil
}

func (s *canvasRelationStore) ListCanvasNodeRuns(
	_ context.Context,
	req *iapiserver.CanvasNodeRunListRequest,
) ([]*iapiserver.CanvasNodeRun, int64, error) {
	s.nodeRunCalls++
	if req.PageNum == 0 {
		return []*iapiserver.CanvasNodeRun{{NodeID: "failed", Status: iapiserver.AtomicTaskStatusFailed}}, 2, nil
	}
	return []*iapiserver.CanvasNodeRun{{NodeID: "skipped", Status: iapiserver.AtomicTaskStatusSkipped}}, 2, nil
}

type canvasRelationTasks struct {
	taskcentersvc.TaskCenterSrv
	dagCalls    int
	atomicCalls int
}

func (s *canvasRelationTasks) GetDAGTaskGroupSummaries(context.Context, []string) (map[string]*iapiserver.DAGTaskGroupSummary, error) {
	s.dagCalls++
	return map[string]*iapiserver.DAGTaskGroupSummary{
		"dag-1": {ID: "dag-1", Name: "Campaign DAG", Status: iapiserver.TaskGroupStatusRunning, Progress: 0.5},
	}, nil
}

func (s *canvasRelationTasks) GetAtomicTaskSummaries(context.Context, []string) (map[string]*iapiserver.AtomicTaskSummary, error) {
	s.atomicCalls++
	return map[string]*iapiserver.AtomicTaskSummary{"task-1": {ID: "task-1", Name: "Render", Status: iapiserver.AtomicTaskStatusRunning, Progress: 0.25}}, nil
}

func TestValidateGraphRejectsCycle(t *testing.T) {
	graph := iapiserver.WorkflowCanvasGraph{
		Nodes: []iapiserver.WorkflowCanvasNode{
			{NodeKey: "a", NodeType: iapiserver.CanvasNodeTypeFunction, Config: map[string]any{"function_ref": "test.a"}, InputBindings: map[string]any{}},
			{NodeKey: "b", NodeType: iapiserver.CanvasNodeTypeFunction, Config: map[string]any{"function_ref": "test.b"}, InputBindings: map[string]any{}},
		},
		Edges: []iapiserver.WorkflowCanvasEdge{
			{FromNodeKey: "a", FromOutput: "out", ToNodeKey: "b", ToInput: "in"},
			{FromNodeKey: "b", FromOutput: "out", ToNodeKey: "a", ToInput: "in"},
		},
	}
	err := validateGraph(graph, true)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if status := toolboxerrors.ToStatus(err); status.Code != code.ErrCanvasCycleDetected {
		t.Fatalf("code = %d", status.Code)
	}
}

func TestGetCanvasRunSummariesFiltersByCreator(t *testing.T) {
	store := &canvasRelationStore{}
	service := &service{store: store}
	summaries, err := service.GetCanvasRunSummaries(context.Background(), "user-1", []string{"source-1", "source-1"})
	if err != nil {
		t.Fatalf("get summaries: %v", err)
	}
	if store.retryCalls != 1 || summaries["source-1"] == nil || summaries["source-1"].Status != iapiserver.CanvasRunStatusFailed {
		t.Fatalf("calls=%d summaries=%#v", store.retryCalls, summaries)
	}
	invisible, err := service.GetCanvasRunSummaries(context.Background(), "user-2", []string{"source-1"})
	if err != nil {
		t.Fatalf("get invisible summaries: %v", err)
	}
	if len(invisible) != 0 {
		t.Fatalf("cross-user summary leaked: %#v", invisible)
	}
}
func TestValidateGraphRejectsUnsafeNodeType(t *testing.T) {
	graph := iapiserver.WorkflowCanvasGraph{
		Nodes: []iapiserver.WorkflowCanvasNode{
			{NodeKey: "request", NodeType: "HTTP", Config: map[string]any{"url": "http://127.0.0.1"}, InputBindings: map[string]any{}},
		},
	}
	err := validateGraph(graph, true)
	if err == nil {
		t.Fatal("expected unsafe node type to fail")
	}
	if status := toolboxerrors.ToStatus(err); status.Code != code.ErrCanvasGraphInvalid {
		t.Fatalf("code = %d", status.Code)
	}
}

func TestContainsUnsafeCanvasConfigMatchesFieldTokens(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		unsafe bool
	}{
		{name: "reject nested credential field", config: map[string]any{"provider": map[string]any{"auth_token": "secret"}}, unsafe: true},
		{name: "reject endpoint field", config: map[string]any{"service.endpoint": "internal"}, unsafe: true},
		{name: "reject camel case URL field", config: map[string]any{"callbackURL": "internal"}, unsafe: true},
		{name: "reject camel case authorization field", config: map[string]any{"authorizationHeader": "secret"}, unsafe: true},
		{name: "allow words containing denied substrings", config: map[string]any{"author": "Ada", "curl_mode": false, "duration": 10}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsUnsafeCanvasConfig(tt.config); got != tt.unsafe {
				t.Fatalf("containsUnsafeCanvasConfig() = %t, want %t", got, tt.unsafe)
			}
		})
	}
}

func TestGraphDigestIsStable(t *testing.T) {
	a := iapiserver.WorkflowCanvasGraph{
		Nodes: []iapiserver.WorkflowCanvasNode{
			{NodeKey: "node", NodeType: iapiserver.CanvasNodeTypeFunction, Config: map[string]any{"b": 2, "a": 1}, InputBindings: map[string]any{}},
		},
		Edges: []iapiserver.WorkflowCanvasEdge{},
	}
	b := iapiserver.WorkflowCanvasGraph{
		Nodes: []iapiserver.WorkflowCanvasNode{
			{NodeKey: "node", NodeType: iapiserver.CanvasNodeTypeFunction, Config: map[string]any{"a": 1, "b": 2}, InputBindings: map[string]any{}},
		},
		Edges: []iapiserver.WorkflowCanvasEdge{},
	}
	da, err := graphDigest(a)
	if err != nil {
		t.Fatal(err)
	}
	db, err := graphDigest(b)
	if err != nil {
		t.Fatal(err)
	}
	if da != db {
		t.Fatalf("digest mismatch: %s != %s", da, db)
	}
}

func TestRetryFailedNodeIDsIncludesOnlyFailureCausedSkippedDescendants(t *testing.T) {
	graph := iapiserver.WorkflowCanvasGraph{
		Nodes: []iapiserver.WorkflowCanvasNode{
			{NodeID: "failed"},
			{NodeID: "skipped_child"},
			{NodeID: "skipped_grandchild"},
			{NodeID: "successful_child"},
			{NodeID: "unrelated_skipped"},
		},
		Edges: []iapiserver.WorkflowCanvasEdge{
			{SourceNodeID: "failed", TargetNodeID: "skipped_child"},
			{SourceNodeID: "skipped_child", TargetNodeID: "skipped_grandchild"},
			{SourceNodeID: "failed", TargetNodeID: "successful_child"},
			{SourceNodeID: "successful_child", TargetNodeID: "unrelated_skipped"},
		},
	}
	nodeRuns := []*iapiserver.CanvasNodeRun{
		{NodeID: "failed", Status: iapiserver.AtomicTaskStatusFailed},
		{NodeID: "skipped_child", Status: iapiserver.AtomicTaskStatusSkipped},
		{NodeID: "skipped_grandchild", Status: iapiserver.AtomicTaskStatusSkipped},
		{NodeID: "successful_child", Status: iapiserver.AtomicTaskStatusSuccess},
		{NodeID: "unrelated_skipped", Status: iapiserver.AtomicTaskStatusSkipped},
	}
	want := []string{"failed", "skipped_child", "skipped_grandchild"}
	got := retryFailedNodeIDs(graph, nodeRuns)
	if len(got) != len(want) {
		t.Fatalf("retryFailedNodeIDs() = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("retryFailedNodeIDs() = %#v, want %#v", got, want)
		}
	}
}

func TestRetryFailedScopeLoadsAllSourceNodeRunPages(t *testing.T) {
	canvasStore := &canvasRelationStore{}
	service := &service{store: canvasStore}
	source := &iapiserver.WorkflowCanvasRun{CanvasVersionID: "version-1"}
	source.ID = "source-1"
	scope, err := service.retryFailedScope(t.Context(), source)
	if err != nil {
		t.Fatal(err)
	}
	if canvasStore.nodeRunCalls != 2 {
		t.Fatalf("node run page calls = %d, want 2", canvasStore.nodeRunCalls)
	}
	if scope.Mode != iapiserver.CanvasRunScopeOnlyNodes || len(scope.NodeIDs) != 2 || scope.NodeIDs[0] != "failed" || scope.NodeIDs[1] != "skipped" {
		t.Fatalf("scope = %#v", scope)
	}
}

func TestCanvasRelationsUseBoundedBatchQueries(t *testing.T) {
	canvasStore := &canvasRelationStore{}
	tasks := &canvasRelationTasks{}
	service := &service{store: canvasStore, tasks: tasks}
	dagID, sourceID := "dag-1", "source-1"
	runs := make([]*iapiserver.WorkflowCanvasRun, 50)
	for index := range runs {
		runs[index] = &iapiserver.WorkflowCanvasRun{
			CanvasID:           "canvas-1",
			CanvasVersionID:    "version-1",
			DAGTaskGroupID:     &dagID,
			RetryOfCanvasRunID: &sourceID,
			ProjectID:          "project",
			Namespace:          "default",
			CreatedBy:          "user-1",
		}
	}
	if err := service.attachCanvasRunRelations(t.Context(), runs); err != nil {
		t.Fatal(err)
	}
	if canvasStore.canvasCalls != 1 || canvasStore.versionCalls != 1 || canvasStore.retryCalls != 1 || tasks.dagCalls != 1 {
		t.Fatalf(
			"relation calls = canvas:%d version:%d retry:%d dag:%d",
			canvasStore.canvasCalls,
			canvasStore.versionCalls,
			canvasStore.retryCalls,
			tasks.dagCalls,
		)
	}
	if runs[0].Canvas == nil || runs[0].Canvas.Name != "Campaign" || runs[0].CanvasVersion == nil || runs[0].CanvasVersion.Version != 3 ||
		runs[0].RetryOfCanvasRun == nil ||
		runs[0].DAGTaskGroup == nil {
		t.Fatalf("run summaries are incomplete: %#v", runs[0])
	}
	nodes := []*iapiserver.CanvasNodeRun{{AtomicTaskID: stringPointer("task-1")}, {AtomicTaskID: stringPointer("task-1")}}
	service.attachCanvasNodeRunRelations(t.Context(), nodes)
	if tasks.atomicCalls != 1 || nodes[0].AtomicTask == nil || nodes[0].AtomicTask.Name != "Render" {
		t.Fatalf("node summaries = calls:%d first:%#v", tasks.atomicCalls, nodes[0].AtomicTask)
	}
}

func stringPointer(value string) *string { return &value }

func TestBuildExecutionPlanScopes(t *testing.T) {
	graph := iapiserver.WorkflowCanvasGraph{
		Nodes: []iapiserver.WorkflowCanvasNode{
			{
				NodeID:            "a",
				NodeType:          "test.node",
				DefinitionVersion: "1",
			}, {NodeID: "b", NodeType: "test.node", DefinitionVersion: "1"}, {NodeID: "c", NodeType: "test.node", DefinitionVersion: "1"}, {NodeID: "d", NodeType: "test.node", DefinitionVersion: "1"},
		},
		Edges: []iapiserver.WorkflowCanvasEdge{
			{EdgeID: "ab", SourceNodeID: "a", TargetNodeID: "b", ConnectionType: "data"},
			{EdgeID: "bc", SourceNodeID: "b", TargetNodeID: "c", ConnectionType: "data"},
		},
		Flows: []iapiserver.WorkflowCanvasFlow{
			{FlowID: "main", Name: "Main", EntryNodeIDs: []string{"a"}, OutputNodeIDs: []string{"c"}},
			{FlowID: "other", Name: "Other", EntryNodeIDs: []string{"d"}, OutputNodeIDs: []string{"d"}},
		},
	}
	definition := &iapiserver.WorkflowNodeDefinition{
		NodeType:          "test.node",
		DefinitionVersion: "1",
		ExecutionBinding:  iapiserver.WorkflowExecutionBinding{Mode: iapiserver.CanvasExecutionAtomic},
	}
	version := &iapiserver.CanvasVersion{GraphSnapshot: graph, DefinitionSnapshots: []*iapiserver.WorkflowNodeDefinition{definition}}
	policy := iapiserver.WorkflowRunPolicy{ReusePolicy: iapiserver.CanvasReuseRerunAll, FailurePolicy: iapiserver.CanvasFailureContinueFlows}
	tests := []struct {
		name      string
		scope     iapiserver.WorkflowRunScope
		wantNodes int
		wantFlows int
	}{
		{name: "all", scope: iapiserver.WorkflowRunScope{Mode: iapiserver.CanvasRunScopeAll}, wantNodes: 4, wantFlows: 2},
		{name: "flow closure", scope: iapiserver.WorkflowRunScope{Mode: iapiserver.CanvasRunScopeFlows, FlowIDs: []string{"main"}}, wantNodes: 3, wantFlows: 1},
		{name: "until node", scope: iapiserver.WorkflowRunScope{Mode: iapiserver.CanvasRunScopeUntilNodes, NodeIDs: []string{"c"}}, wantNodes: 3},
		{name: "from node", scope: iapiserver.WorkflowRunScope{Mode: iapiserver.CanvasRunScopeFromNodes, NodeIDs: []string{"b"}}, wantNodes: 2},
		{name: "only node", scope: iapiserver.WorkflowRunScope{Mode: iapiserver.CanvasRunScopeOnlyNodes, NodeIDs: []string{"b"}}, wantNodes: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := buildExecutionPlan(version, tt.scope, policy, map[string]any{})
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Graph.Nodes) != tt.wantNodes || len(plan.Flows) != tt.wantFlows {
				t.Fatalf("nodes=%d flows=%d", len(plan.Graph.Nodes), len(plan.Flows))
			}
			if plan.Digest == "" {
				t.Fatal("empty plan digest")
			}
		})
	}
}

func TestBuildExecutionPlanRejectsInvalidScopeAndRequiredReuse(t *testing.T) {
	version := &iapiserver.CanvasVersion{
		GraphSnapshot: iapiserver.WorkflowCanvasGraph{Nodes: []iapiserver.WorkflowCanvasNode{{NodeID: "a", NodeType: "test.node", DefinitionVersion: "1"}}},
	}
	_, err := buildExecutionPlan(
		version,
		iapiserver.WorkflowRunScope{Mode: iapiserver.CanvasRunScopeOnlyNodes, NodeIDs: []string{"a", "a"}},
		iapiserver.WorkflowRunPolicy{ReusePolicy: iapiserver.CanvasReuseRerunAll, FailurePolicy: iapiserver.CanvasFailureContinueFlows},
		map[string]any{},
	)
	if err == nil || toolboxerrors.ToStatus(err).Code != code.ErrCanvasRunScopeInvalid {
		t.Fatalf("duplicate scope error=%v", err)
	}
	_, err = buildExecutionPlan(
		version,
		iapiserver.WorkflowRunScope{Mode: iapiserver.CanvasRunScopeAll},
		iapiserver.WorkflowRunPolicy{ReusePolicy: iapiserver.CanvasReuseRequired, FailurePolicy: iapiserver.CanvasFailureContinueFlows},
		map[string]any{},
	)
	if err == nil || toolboxerrors.ToStatus(err).Code != code.ErrCanvasReuseRequiredUnavailable {
		t.Fatalf("reuse error=%v", err)
	}
}

func TestValidateNodeDefinitionRequestRejectsAmbiguousExecutionBinding(t *testing.T) {
	functionRef, versionID := "test.run", "version-1"
	req := &iapiserver.WorkflowNodeDefinitionRegisterRequest{
		NodeType:          "test.node",
		DefinitionVersion: "1",
		Title:             "Test",
		Category:          "test",
		NodeKind:          "processor",
		Ports:             []iapiserver.WorkflowPortDefinition{},
		ConfigSchema:      map[string]any{"type": "object"},
		ExecutionBinding: iapiserver.WorkflowExecutionBinding{
			Mode:                 iapiserver.CanvasExecutionAtomic,
			BindingVersion:       "1",
			FunctionRef:          &functionRef,
			ApplicationVersionID: &versionID,
		},
		AvailabilityScope: iapiserver.CanvasAvailabilitySystem,
	}
	err := validateNodeDefinitionRequest(req)
	if err == nil || toolboxerrors.ToStatus(err).Code != code.ErrCanvasNodeReferenceInvalid {
		t.Fatalf("error=%v", err)
	}
}
