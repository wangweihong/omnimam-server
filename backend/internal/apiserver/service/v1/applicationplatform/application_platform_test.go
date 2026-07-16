package applicationplatform

import (
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func TestValidateRestrictionsRejectsCapabilityExpansion(t *testing.T) {
	capability := &iapiserver.AIAppProviderCapability{
		Models:     []iapiserver.ProviderCapabilityModel{{ID: "model-a"}},
		Operations: []iapiserver.ProviderCapabilityOperation{{ID: "operation-a"}},
		Variants:   []iapiserver.ProviderCapabilityVariant{{ID: "variant-a"}},
	}
	if err := validateRestrictions(capability, map[string]any{"model_ids": []any{"model-a"}}); err != nil {
		t.Fatalf("valid restriction rejected: %v", err)
	}
	if err := validateRestrictions(capability, map[string]any{"model_ids": []any{"model-b"}}); err == nil {
		t.Fatal("expanding restriction was accepted")
	}
	if err := validateRestrictions(capability, map[string]any{"unknown": []any{"x"}}); err == nil {
		t.Fatal("unknown restriction was accepted")
	}
}

func TestBuildRuntimeFieldsAppliesInvalidStrategies(t *testing.T) {
	properties := map[string]any{
		"resolution": map[string]any{"type": "string", "enum": []any{"720p", "1080p"}},
		"steps":      map[string]any{"type": "integer", "minimum": 1, "maximum": 10},
	}
	policies := map[string]any{
		"resolution": map[string]any{"on_invalid": "reset"},
		"steps":      map[string]any{"on_invalid": "clamp"},
	}
	fields, changes, violations := buildRuntimeFields(properties, []string{"resolution", "steps"}, policies, map[string]any{"resolution": "4k", "steps": 20})
	if len(violations) != 1 || violations[0].Field != "resolution" {
		t.Fatalf("violations = %#v, want required resolution after reset", violations)
	}
	if len(changes) != 2 {
		t.Fatalf("changes = %d, want 2", len(changes))
	}
	for _, field := range fields {
		if field.Name == "steps" && field.Value != 10 {
			t.Fatalf("clamped steps = %#v, want 10", field.Value)
		}
	}
}

func TestBuildRuntimeFieldsRejectsEmptyPolicyIntersection(t *testing.T) {
	properties := map[string]any{"resolution": map[string]any{"type": "string", "enum": []any{"720p"}}}
	policies := map[string]any{"resolution": map[string]any{"values": []any{"4k"}}}
	_, _, violations := buildRuntimeFields(properties, nil, policies, nil)
	if len(violations) != 1 || violations[0].Code != "NO_VALID_OPTION" {
		t.Fatalf("empty capability-policy intersection was not rejected: %#v", violations)
	}
}

func TestValidateComfyUIWorkflowRequiresObjectInfo(t *testing.T) {
	workflow := map[string]any{"1": map[string]any{"class_type": "KSampler", "inputs": map[string]any{}}}
	if err := validateComfyUIWorkflow(workflow, map[string]any{"KSampler": map[string]any{}}); err != nil {
		t.Fatalf("valid workflow rejected: %v", err)
	}
	if err := validateComfyUIWorkflow(workflow, map[string]any{}); err == nil {
		t.Fatal("missing object_info class was accepted")
	}
}

func TestValidateComfyUIContractRequiresMappingsAndOutputs(t *testing.T) {
	workflow := map[string]any{"1": map[string]any{"class_type": "KSampler", "inputs": map[string]any{}}}
	objectInfo := map[string]any{"KSampler": map[string]any{}}
	contract := map[string]any{
		"parameters":      map[string]any{"seed": map[string]any{"required": true}},
		"request_mapping": map[string]any{"seed": "1.inputs.seed"},
		"outputs":         map[string]any{"image": map[string]any{"node_id": "1"}},
	}
	if err := validateComfyUIContract(workflow, objectInfo, contract); err != nil {
		t.Fatalf("valid ComfyUI contract rejected: %v", err)
	}
	delete(contract, "outputs")
	if err := validateComfyUIContract(workflow, objectInfo, contract); err == nil {
		t.Fatal("ComfyUI contract without outputs was accepted")
	}
}

func TestResolvedRuntimeInputsUsesFinalFormValues(t *testing.T) {
	resolved := resolvedRuntimeInputs(map[string]any{"duration": 99, "removed": "old", "passthrough": true}, []iapiserver.RuntimeFormField{
		{Name: "duration", Value: 15},
		{Name: "removed", Value: nil},
		{Name: "model", Value: "seedance-2.0"},
	})
	if resolved["duration"] != 15 || resolved["model"] != "seedance-2.0" || resolved["passthrough"] != true {
		t.Fatalf("resolved inputs do not contain final form values: %#v", resolved)
	}
	if _, exists := resolved["removed"]; exists {
		t.Fatalf("reset field remained in resolved inputs: %#v", resolved)
	}
}

func TestSameApplicationRunRequestRejectsIdempotencyConflict(t *testing.T) {
	existing := &iapiserver.ApplicationRun{ApplicationID: "app-1", ApplicationVersionID: "version-1", EngineInstanceID: "engine-1", InputSnapshot: map[string]any{"prompt": "same"}}
	request := &iapiserver.ApplicationRunCreateRequest{ApplicationVersionID: "version-1", EngineInstanceID: "engine-1", Inputs: map[string]any{"prompt": "same"}}
	if !sameApplicationRunRequest(existing, "app-1", request) {
		t.Fatal("equivalent idempotent request was rejected")
	}
	request.Inputs["prompt"] = "different"
	if sameApplicationRunRequest(existing, "app-1", request) {
		t.Fatal("conflicting idempotent request was accepted")
	}
}
