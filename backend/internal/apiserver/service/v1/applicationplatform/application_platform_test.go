package applicationplatform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
)

func TestValidateEngineAuthRejectsMismatchedUnion(t *testing.T) {
	runtime, err := appregistry.LoadRuntimeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	service := &applicationPlatformService{Dependencies: Dependencies{Runtime: runtime}}
	tests := []struct {
		name, engineType, authType string
		config                     map[string]any
		valid                      bool
	}{
		{name: "none without config", engineType: "comfyui", authType: "none", valid: true},
		{name: "none with config", engineType: "comfyui", authType: "none", config: map[string]any{"api_key": "secret"}},
		{name: "api key", engineType: "comfyui", authType: "api_key", config: map[string]any{"api_key": "secret"}, valid: true},
		{name: "api key extra field", engineType: "comfyui", authType: "api_key", config: map[string]any{"api_key": "secret", "token": "extra"}},
		{name: "missing bearer token", engineType: "comfyui", authType: "bearer_token", config: map[string]any{}},
		{name: "unsupported ak sk", engineType: "comfyui", authType: "ak_sk", config: map[string]any{"access_key": "a", "secret_key": "s"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateEngineAuth(tt.engineType, tt.authType, tt.config)
			if tt.valid && err != nil {
				t.Fatalf("valid auth rejected: %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatal("invalid auth accepted")
			}
		})
	}
}

func TestParseComfyUIWorkflowDerivesCandidatesAndRejectsBrokenReferences(t *testing.T) {
	workflow := map[string]any{"1": map[string]any{"class_type": "LoadImage", "inputs": map[string]any{"image": "input.png"}}, "2": map[string]any{"class_type": "SaveImage", "inputs": map[string]any{"images": []any{"1", float64(0)}}}}
	objectInfo := map[string]any{"LoadImage": map[string]any{"input": map[string]any{"required": map[string]any{"image": []any{"STRING", map[string]any{}}}}, "output": []any{"IMAGE"}, "output_name": []any{"IMAGE"}}, "SaveImage": map[string]any{"input": map[string]any{"required": map[string]any{"images": []any{"IMAGE", map[string]any{}}}}, "output": []any{}}}
	parsed, err := parseComfyUIWorkflow(workflow, nil, objectInfo)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.summary.TotalNodes != 2 || len(parsed.inputs) != 2 || len(parsed.outputs) != 1 {
		t.Fatalf("unexpected parse result: %#v", parsed)
	}
	workflow["2"].(map[string]any)["inputs"].(map[string]any)["images"] = []any{"missing", float64(0)}
	if _, err := parseComfyUIWorkflow(workflow, nil, objectInfo); err == nil {
		t.Fatal("broken node reference was accepted")
	}
}

func TestCompatibilityDiagnosticsDetectsRequiredAndRangeChanges(t *testing.T) {
	workflow := &iapiserver.ComfyUIWorkflow{ParsedNodes: []iapiserver.ComfyUIWorkflowNode{{NodeID: "1", ClassType: "Sampler", Inputs: []map[string]any{{"name": "steps", "value": float64(30), "classification": "exposable", "data_type": "int"}}, Outputs: []map[string]any{}}}}
	objectInfo := map[string]any{"Sampler": map[string]any{"input": map[string]any{"required": map[string]any{"steps": []any{"INT", map[string]any{"max": float64(20)}}, "seed": []any{"INT", map[string]any{}}}}, "output": []any{}}}
	diagnostics := compatibilityDiagnostics(workflow, objectInfo)
	codes := map[string]bool{}
	for _, item := range diagnostics {
		codes[item.Code] = true
	}
	if !codes["VALUE_ABOVE_MAXIMUM"] || !codes["REQUIRED_INPUT_ADDED"] {
		t.Fatalf("missing compatibility diagnostics: %#v", diagnostics)
	}
}

func TestCanonicalJSONDigestIgnoresObjectKeyOrder(t *testing.T) {
	first, err := canonicalJSONDigest(map[string]any{"b": 2, "a": 1})
	if err != nil {
		t.Fatal(err)
	}
	second, err := canonicalJSONDigest(map[string]any{"a": 1, "b": 2})
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("digest changed with key order: %s != %s", first, second)
	}
}

func TestCanonicalJSONDigestMatchesRFC8785Example(t *testing.T) {
	var value any
	raw := []byte(`{"numbers":[333333333.33333329,1E30,4.50,2e-3,0.000000000000000000000000001],"string":"\u20ac$\u000F\nA'B\"\\\\\"/","literals":[null,true,false]}`)
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	canonical := []byte(`{"literals":[null,true,false],"numbers":[333333333.3333333,1e+30,4.5,0.002,1e-27],"string":"€$\u000f\nA'B\"\\\\\"/"}`)
	digest := sha256.Sum256(canonical)
	want := "sha256:" + hex.EncodeToString(digest[:])
	got, err := canonicalJSONDigest(value)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("RFC 8785 digest=%s, want %s", got, want)
	}
}

func TestCanonicalJSONRawDigestRejectsDuplicateObjectKeys(t *testing.T) {
	if _, err := canonicalJSONRawDigest([]byte(`{"1":{"class_type":"A","class_type":"B","inputs":{}}}`), nil); err == nil {
		t.Fatal("duplicate JSON object keys were accepted")
	}
}

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
