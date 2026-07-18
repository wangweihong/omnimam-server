package workflowcanvas

import (
	"testing"

	toolboxerrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

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
