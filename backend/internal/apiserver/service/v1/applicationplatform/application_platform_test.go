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
			name: "comfyui raw json primitive leaves",
			kind: iapiserver.AppTemplateKindComfyUI,
			config: map[string]any{
				"raw": `{"node":{"inputs":{"prompt":"hello","steps":20,"enabled":true}}}`,
			},
			want: []iapiserver.ParsedField{
				{SourcePath: "node.inputs.enabled", FieldType: "boolean", Required: true, LabelHint: "enabled"},
				{SourcePath: "node.inputs.prompt", FieldType: "string", Required: true, LabelHint: "prompt"},
				{SourcePath: "node.inputs.steps", FieldType: "number", Required: true, LabelHint: "steps"},
			},
		},
		{
			name: "saas request template primitive leaves",
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
			name:    "saas missing request template",
			kind:    iapiserver.AppTemplateKindSaaSAPI,
			config:  map[string]any{},
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
