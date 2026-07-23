package applicationplatform

import (
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func TestCollectTestOutputsFiltersSelectedNodes(t *testing.T) {
	outputs := map[string]any{
		"save-a": map[string]any{"images": []any{map[string]any{"filename": "a.png"}}},
		"save-b": map[string]any{"images": []any{map[string]any{"filename": "b.png"}}},
	}
	selections := []iapiserver.ComfyUIWorkflowTestOutputSelection{
		{NodeID: "save-b", OutputIndex: 0},
		{NodeID: "save-b", OutputIndex: 1},
	}

	result := collectTestOutputs(outputs, selections)
	if len(result) != 1 || result[0].NodeID != "save-b" {
		t.Fatalf("collected outputs = %#v, want only save-b", result)
	}
}

func TestCollectTestOutputsKeepsLegacyUnfilteredBehavior(t *testing.T) {
	outputs := map[string]any{
		"text-a": map[string]any{"text": []any{"first"}},
		"text-b": map[string]any{"text": []any{"second"}},
	}

	result := collectTestOutputs(outputs, nil)
	if len(result) != 2 {
		t.Fatalf("legacy collected output count = %d, want 2", len(result))
	}
}

func TestValidateTestOutputs(t *testing.T) {
	workflow := &iapiserver.ComfyUIWorkflow{OutputCandidates: []iapiserver.ComfyUIWorkflowOutputCandidate{
		{NodeID: "save", OutputIndex: 0, Extractable: true, MediaType: "image"},
		{NodeID: "model", OutputIndex: 0, Extractable: true, MediaType: "other"},
	}}
	tests := []struct {
		name    string
		outputs []iapiserver.ComfyUIWorkflowTestOutputSelection
		valid   bool
	}{
		{name: "valid output", outputs: []iapiserver.ComfyUIWorkflowTestOutputSelection{{NodeID: "save", OutputIndex: 0}}, valid: true},
		{name: "empty outputs"},
		{name: "unknown output", outputs: []iapiserver.ComfyUIWorkflowTestOutputSelection{{NodeID: "missing", OutputIndex: 0}}},
		{name: "non preview output", outputs: []iapiserver.ComfyUIWorkflowTestOutputSelection{{NodeID: "model", OutputIndex: 0}}},
		{name: "duplicate output", outputs: []iapiserver.ComfyUIWorkflowTestOutputSelection{{NodeID: "save", OutputIndex: 0}, {NodeID: "save", OutputIndex: 0}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTestOutputs(workflow, tt.outputs)
			if tt.valid && err != nil {
				t.Fatalf("valid outputs rejected: %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatal("invalid outputs accepted")
			}
		})
	}
}
