package applicationplatform

import (
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func TestParseTemplateFields(t *testing.T) {
	tests := []struct {
		name    string
		kind    string
		config  map[string]any
		want    []iapiserver.ParsedField
		wantErr bool
	}{
		{
			name: "comfyui api raw json parses input primitives and skips links",
			kind: iapiserver.AppTemplateKindComfyUI,
			config: map[string]any{
				"raw": `{
					"3": {
						"class_type": "KSampler",
						"inputs": {
							"seed": 156680208700286,
							"steps": 20,
							"cfg": 8,
							"sampler_name": "euler",
							"scheduler": "normal",
							"denoise": 1,
							"model": ["4", 0],
							"positive": ["6", 0]
						},
						"_meta": {"title": "KSampler"}
					},
					"4": {
						"class_type": "CheckpointLoaderSimple",
						"inputs": {
							"ckpt_name": "v1-5-pruned-emaonly.safetensors"
						},
						"_meta": {"title": "Load Checkpoint"}
					}
				}`,
			},
			want: []iapiserver.ParsedField{
				{SourcePath: "3.inputs.cfg", FieldType: "number", Required: true, LabelHint: "KSampler.cfg"},
				{SourcePath: "3.inputs.denoise", FieldType: "number", Required: true, LabelHint: "KSampler.denoise"},
				{SourcePath: "3.inputs.sampler_name", FieldType: "string", Required: true, LabelHint: "KSampler.sampler_name"},
				{SourcePath: "3.inputs.scheduler", FieldType: "string", Required: true, LabelHint: "KSampler.scheduler"},
				{SourcePath: "3.inputs.seed", FieldType: "number", Required: true, LabelHint: "KSampler.seed"},
				{SourcePath: "3.inputs.steps", FieldType: "number", Required: true, LabelHint: "KSampler.steps"},
				{SourcePath: "4.inputs.ckpt_name", FieldType: "string", Required: true, LabelHint: "Load Checkpoint.ckpt_name"},
			},
		},
		{
			name: "comfyui api config object parses without raw wrapper",
			kind: iapiserver.AppTemplateKindComfyUI,
			config: map[string]any{
				"3": map[string]any{
					"class_type": "CLIPTextEncode",
					"inputs": map[string]any{
						"text": "a cat",
						"clip": []any{"4", float64(1)},
					},
					"_meta": map[string]any{"title": "Prompt"},
				},
			},
			want: []iapiserver.ParsedField{
				{SourcePath: "3.inputs.text", FieldType: "string", Required: true, LabelHint: "Prompt.text"},
			},
		},
		{
			name: "saas config root primitive leaves",
			kind: iapiserver.AppTemplateKindSaaSAPI,
			config: map[string]any{
				"model":  "z-image",
				"prompt": "cat",
				"size":   "1024x1024",
			},
			want: []iapiserver.ParsedField{
				{SourcePath: "model", FieldType: "string", Required: true, LabelHint: "model"},
				{SourcePath: "prompt", FieldType: "string", Required: true, LabelHint: "prompt"},
				{SourcePath: "size", FieldType: "string", Required: true, LabelHint: "size"},
			},
		},
		{
			name: "saas request template wrapper stays compatible",
			kind: iapiserver.AppTemplateKindSaaSAPI,
			config: map[string]any{
				"requestTemplate": map[string]any{
					"body": map[string]any{"query": "cat"},
				},
			},
			want: []iapiserver.ParsedField{
				{SourcePath: "body.query", FieldType: "string", Required: true, LabelHint: "query"},
			},
		},
		{
			name:    "invalid comfyui raw json",
			kind:    iapiserver.AppTemplateKindComfyUI,
			config:  map[string]any{"raw": `{"node":`},
			wantErr: true,
		},
		{
			name: "comfyui ui save workflow format is rejected",
			kind: iapiserver.AppTemplateKindComfyUI,
			config: map[string]any{
				"nodes": []any{map[string]any{"id": float64(1), "type": "KSampler"}},
				"links": []any{},
				"extra": map[string]any{},
			},
			wantErr: true,
		},
		{
			name: "comfyui node missing class type is rejected",
			kind: iapiserver.AppTemplateKindComfyUI,
			config: map[string]any{
				"3": map[string]any{
					"inputs": map[string]any{"text": "a cat"},
				},
			},
			wantErr: true,
		},
		{
			name:    "saas empty config",
			kind:    iapiserver.AppTemplateKindSaaSAPI,
			config:  map[string]any{},
			wantErr: true,
		},
		{
			name: "saas config without primitive leaves",
			kind: iapiserver.AppTemplateKindSaaSAPI,
			config: map[string]any{
				"body": map[string]any{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTemplateFields(tt.kind, tt.config)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseTemplateFields() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTemplateFields() error = %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parseTemplateFields() len = %d, want %d: %#v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("parseTemplateFields()[%d] = %#v, want %#v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestBuildFieldMappings(t *testing.T) {
	parsed := []iapiserver.ParsedField{
		{SourcePath: "body.prompt", FieldType: "string", Required: true},
		{SourcePath: "body.steps", FieldType: "number", Required: true},
	}

	tests := []struct {
		name    string
		inputs  []iapiserver.FieldMappingInput
		wantErr bool
	}{
		{
			name: "valid mappings inherit required flag",
			inputs: []iapiserver.FieldMappingInput{
				{FieldKey: "prompt", FieldLabel: "Prompt", FieldType: "string", SourcePath: "body.prompt", SortOrder: 1},
				{FieldKey: "steps", FieldLabel: "Steps", FieldType: "number", SourcePath: "body.steps", SortOrder: 2},
			},
		},
		{
			name: "duplicate field key",
			inputs: []iapiserver.FieldMappingInput{
				{FieldKey: "prompt", FieldLabel: "Prompt", FieldType: "string", SourcePath: "body.prompt"},
				{FieldKey: "prompt", FieldLabel: "Prompt 2", FieldType: "number", SourcePath: "body.steps"},
			},
			wantErr: true,
		},
		{
			name: "invalid source path",
			inputs: []iapiserver.FieldMappingInput{
				{FieldKey: "missing", FieldLabel: "Missing", FieldType: "string", SourcePath: "body.missing"},
			},
			wantErr: true,
		},
		{
			name: "invalid field type",
			inputs: []iapiserver.FieldMappingInput{
				{FieldKey: "prompt", FieldLabel: "Prompt", FieldType: "number", SourcePath: "body.prompt"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildFieldMappings("app-1", "tpl-1", parsed, tt.inputs)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("buildFieldMappings() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("buildFieldMappings() error = %v", err)
			}
			if len(got) != len(tt.inputs) {
				t.Fatalf("buildFieldMappings() len = %d, want %d", len(got), len(tt.inputs))
			}
			for i := range got {
				if got[i].ApplicationID != "app-1" || got[i].TemplateID != "tpl-1" {
					t.Fatalf("buildFieldMappings()[%d] ids = %s/%s", i, got[i].ApplicationID, got[i].TemplateID)
				}
				if !got[i].Required {
					t.Fatalf("buildFieldMappings()[%d].Required = false, want true", i)
				}
			}
		})
	}
}

func TestValidateTemplateSaaSConfig(t *testing.T) {
	tests := []struct {
		name    string
		req     *iapiserver.AppTemplateCreateRequest
		wantErr bool
	}{
		{
			name: "comfyui does not require saas fields",
			req:  &iapiserver.AppTemplateCreateRequest{Kind: iapiserver.AppTemplateKindComfyUI},
		},
		{
			name: "valid saas config",
			req: &iapiserver.AppTemplateCreateRequest{
				Kind:             iapiserver.AppTemplateKindSaaSAPI,
				SaaSPlatformType: iapiserver.SaaSPlatformModelScope,
				CapabilityType:   iapiserver.CapabilityImageGeneration,
				OperationKey:     "modelscope.image_generation",
			},
		},
		{
			name: "missing saas platform",
			req: &iapiserver.AppTemplateCreateRequest{
				Kind:           iapiserver.AppTemplateKindSaaSAPI,
				CapabilityType: iapiserver.CapabilityImageGeneration,
				OperationKey:   "modelscope.image_generation",
			},
			wantErr: true,
		},
		{
			name: "unsupported capability",
			req: &iapiserver.AppTemplateCreateRequest{
				Kind:             iapiserver.AppTemplateKindSaaSAPI,
				SaaSPlatformType: iapiserver.SaaSPlatformModelScope,
				CapabilityType:   "text_generation",
				OperationKey:     "modelscope.text_generation",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTemplateSaaSConfig(tt.req)
			if tt.wantErr && err == nil {
				t.Fatalf("validateTemplateSaaSConfig() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateTemplateSaaSConfig() error = %v", err)
			}
		})
	}
}

func TestValidateApplicationRunEngine(t *testing.T) {
	app := &iapiserver.Application{
		Kind:             iapiserver.AppTemplateKindSaaSAPI,
		SaaSPlatformType: iapiserver.SaaSPlatformModelScope,
		CapabilityType:   iapiserver.CapabilityImageGeneration,
	}
	engine := &iapiserver.AppEngine{
		EngineType:               iapiserver.AppEngineTypeSaaSAPI,
		SaaSPlatformType:         iapiserver.SaaSPlatformModelScope,
		Status:                   iapiserver.AppEngineStatusActive,
		HealthStatus:             iapiserver.AppEngineHealthHealthy,
		SupportedCapabilityTypes: []string{iapiserver.CapabilityImageGeneration},
	}
	if err := validateApplicationRunEngine(app, engine); err != nil {
		t.Fatalf("validateApplicationRunEngine() error = %v", err)
	}
	engine.SaaSPlatformType = iapiserver.SaaSPlatformCustomHTTP
	if err := validateApplicationRunEngine(app, engine); err == nil {
		t.Fatalf("validateApplicationRunEngine() error = nil, want platform mismatch")
	}
	engine.SaaSPlatformType = iapiserver.SaaSPlatformModelScope
	engine.SupportedCapabilityTypes = []string{iapiserver.CapabilityVideoGeneration}
	if err := validateApplicationRunEngine(app, engine); err == nil {
		t.Fatalf("validateApplicationRunEngine() error = nil, want capability unsupported")
	}
	engine.SupportedCapabilityTypes = []string{iapiserver.CapabilityImageGeneration}
	engine.HealthStatus = iapiserver.AppEngineHealthUnhealthy
	if err := validateApplicationRunEngine(app, engine); err == nil {
		t.Fatalf("validateApplicationRunEngine() error = nil, want unhealthy")
	}
}

func TestRenderApplicationPayload(t *testing.T) {
	app := &iapiserver.Application{
		FixedParameters: map[string]any{"body.model": "z-image"},
		FieldMappings: []*iapiserver.FieldMapping{
			{FieldKey: "prompt", SourcePath: "body.prompt", DefaultValue: "cat", Required: true},
			{FieldKey: "steps", SourcePath: "body.steps", DefaultValue: float64(20), Required: true},
		},
	}
	got, err := renderApplicationPayload(app, map[string]any{"prompt": "dog"})
	if err != nil {
		t.Fatalf("renderApplicationPayload() error = %v", err)
	}
	if got["body.model"] != "z-image" || got["body.prompt"] != "dog" || got["body.steps"] != float64(20) {
		t.Fatalf("renderApplicationPayload() = %#v", got)
	}
}
