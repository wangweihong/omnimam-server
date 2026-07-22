package iapiserver

import (
	"encoding/json"
	"testing"
)

func TestWorkflowCanvasResponseIncludesS2Metadata(t *testing.T) {
	canvas := WorkflowCanvas{
		CanvasID:      "canvas-1",
		Visibility:    CanvasVisibilityPrivate,
		DraftGraph:    WorkflowCanvasGraph{Nodes: []WorkflowCanvasNode{}, Edges: []WorkflowCanvasEdge{}, Flows: []WorkflowCanvasFlow{}},
		DraftRevision: 1,
		ProjectID:     "project",
		Namespace:     "default",
		CreatedBy:     "user-1",
	}
	canvas.ID = "canvas-1"
	canvas.Name = "Example"
	raw, err := json.Marshal(canvas)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"canvas_id", "name", "description", "draft_graph", "resource_version", "created_at", "updated_at"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing %s in %s", key, raw)
		}
	}
	if _, ok := payload["id"]; ok {
		t.Fatalf("generic persistence id leaked: %s", raw)
	}
}

func TestCanvasNodeRunDetailResponseIsFlattened(t *testing.T) {
	node := &CanvasNodeRun{
		CanvasNodeRunID:      "node-run-1",
		CanvasRunID:          "run-1",
		NodeID:               "node-1",
		ExecutionKey:         "node-1",
		NodeType:             "test.node",
		DefinitionVersion:    "1",
		FlowIDs:              []string{},
		ExecutionFingerprint: "sha256:test",
		ResultMode:           CanvasResultExecuted,
		Status:               CanvasRunStatusRunning,
		Warnings:             []WorkflowRunWarning{},
	}
	node.ID = "node-run-1"
	detail := CanvasNodeRunDetail{CanvasNodeRun: node, TaskBindings: []*CanvasNodeRunTaskBinding{}, OutputBindings: []*CanvasNodeRunOutputBinding{}}
	raw, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["canvas_node_run_id"] != "node-run-1" || payload["task_bindings"] == nil || payload["output_bindings"] == nil {
		t.Fatalf("detail=%s", raw)
	}
	if _, nested := payload["canvas_node_run"]; nested {
		t.Fatalf("node detail must be flattened: %s", raw)
	}
}
