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

func (s *canvasRelationStore) GetWorkflowCanvasRunsByIDs(context.Context, []string) ([]*iapiserver.WorkflowCanvasRun, error) {
	s.retryCalls++
	item := &iapiserver.WorkflowCanvasRun{Status: iapiserver.CanvasRunStatusFailed, Progress: 0.5, ProjectID: "project", Namespace: "default", CreatedBy: "user-1"}
	item.ID = "source-1"
	return []*iapiserver.WorkflowCanvasRun{item}, nil
}

type canvasRelationTasks struct {
	taskcentersvc.TaskCenterSrv
	dagCalls    int
	atomicCalls int
}

func (s *canvasRelationTasks) GetDAGTaskGroupSummaries(context.Context, []string) (map[string]*iapiserver.DAGTaskGroupSummary, error) {
	s.dagCalls++
	return map[string]*iapiserver.DAGTaskGroupSummary{"dag-1": {ID: "dag-1", Name: "Campaign DAG", Status: iapiserver.TaskGroupStatusRunning, Progress: 0.5}}, nil
}

func (s *canvasRelationTasks) GetAtomicTaskSummaries(context.Context, []string) (map[string]*iapiserver.AtomicTaskSummary, error) {
	s.atomicCalls++
	return map[string]*iapiserver.AtomicTaskSummary{"task-1": {ID: "task-1", Name: "Render", Status: iapiserver.AtomicTaskStatusRunning, Progress: 0.25}}, nil
}

func TestValidateGraphRejectsCycle(t *testing.T) {
	graph := iapiserver.WorkflowCanvasGraph{Nodes: []iapiserver.WorkflowCanvasNode{{NodeKey: "a", NodeType: iapiserver.CanvasNodeTypeFunction, Config: map[string]any{"function_ref": "test.a"}, InputBindings: map[string]any{}}, {NodeKey: "b", NodeType: iapiserver.CanvasNodeTypeFunction, Config: map[string]any{"function_ref": "test.b"}, InputBindings: map[string]any{}}}, Edges: []iapiserver.WorkflowCanvasEdge{{FromNodeKey: "a", FromOutput: "out", ToNodeKey: "b", ToInput: "in"}, {FromNodeKey: "b", FromOutput: "out", ToNodeKey: "a", ToInput: "in"}}}
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
	graph := iapiserver.WorkflowCanvasGraph{Nodes: []iapiserver.WorkflowCanvasNode{{NodeKey: "request", NodeType: "HTTP", Config: map[string]any{"url": "http://127.0.0.1"}, InputBindings: map[string]any{}}}}
	err := validateGraph(graph, true)
	if err == nil {
		t.Fatal("expected unsafe node type to fail")
	}
	if status := toolboxerrors.ToStatus(err); status.Code != code.ErrCanvasGraphInvalid {
		t.Fatalf("code = %d", status.Code)
	}
}
func TestGraphDigestIsStable(t *testing.T) {
	a := iapiserver.WorkflowCanvasGraph{Nodes: []iapiserver.WorkflowCanvasNode{{NodeKey: "node", NodeType: iapiserver.CanvasNodeTypeFunction, Config: map[string]any{"b": 2, "a": 1}, InputBindings: map[string]any{}}}, Edges: []iapiserver.WorkflowCanvasEdge{}}
	b := iapiserver.WorkflowCanvasGraph{Nodes: []iapiserver.WorkflowCanvasNode{{NodeKey: "node", NodeType: iapiserver.CanvasNodeTypeFunction, Config: map[string]any{"a": 1, "b": 2}, InputBindings: map[string]any{}}}, Edges: []iapiserver.WorkflowCanvasEdge{}}
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

func TestCanvasRelationsUseBoundedBatchQueries(t *testing.T) {
	canvasStore := &canvasRelationStore{}
	tasks := &canvasRelationTasks{}
	service := &service{store: canvasStore, tasks: tasks}
	dagID, sourceID := "dag-1", "source-1"
	runs := make([]*iapiserver.WorkflowCanvasRun, 50)
	for index := range runs {
		runs[index] = &iapiserver.WorkflowCanvasRun{CanvasID: "canvas-1", CanvasVersionID: "version-1", DAGTaskGroupID: &dagID, RetryOfCanvasRunID: &sourceID, ProjectID: "project", Namespace: "default", CreatedBy: "user-1"}
	}
	if err := service.attachCanvasRunRelations(t.Context(), runs); err != nil {
		t.Fatal(err)
	}
	if canvasStore.canvasCalls != 1 || canvasStore.versionCalls != 1 || canvasStore.retryCalls != 1 || tasks.dagCalls != 1 {
		t.Fatalf("relation calls = canvas:%d version:%d retry:%d dag:%d", canvasStore.canvasCalls, canvasStore.versionCalls, canvasStore.retryCalls, tasks.dagCalls)
	}
	if runs[0].Canvas == nil || runs[0].Canvas.Name != "Campaign" || runs[0].CanvasVersion == nil || runs[0].CanvasVersion.Version != 3 || runs[0].RetryOfCanvasRun == nil || runs[0].DAGTaskGroup == nil {
		t.Fatalf("run summaries are incomplete: %#v", runs[0])
	}
	nodes := []*iapiserver.CanvasNodeRun{{AtomicTaskID: stringPointer("task-1")}, {AtomicTaskID: stringPointer("task-1")}}
	service.attachCanvasNodeRunRelations(t.Context(), nodes)
	if tasks.atomicCalls != 1 || nodes[0].AtomicTask == nil || nodes[0].AtomicTask.Name != "Render" {
		t.Fatalf("node summaries = calls:%d first:%#v", tasks.atomicCalls, nodes[0].AtomicTask)
	}
}

func stringPointer(value string) *string { return &value }
