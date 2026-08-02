package workflowcanvas

import (
	"context"

	"github.com/google/uuid"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

const builtInDefinitionVersion = "1.0.0"

var builtInCompilerKeys = map[string]string{
	"image":       iapiserver.CanvasCompilerMediaInput,
	"prompt":      iapiserver.CanvasCompilerPrompt,
	"loop":        iapiserver.CanvasCompilerLoop,
	"promptGroup": iapiserver.CanvasCompilerPromptGroup,
	"output":      iapiserver.CanvasCompilerOutput,
}

// ReconcileBuiltInNodeDefinitions 幂等写入当前 release 固定的 Canvas SYSTEM 节点目录。
func ReconcileBuiltInNodeDefinitions(ctx context.Context, target store.WorkflowCanvasStore) error {
	for _, definition := range builtInNodeDefinitions() {
		if _, _, err := target.AddWorkflowNodeDefinitionIdempotent(ctx, definition); err != nil {
			return err
		}
	}
	return nil
}

func builtInNodeDefinitions() []*iapiserver.WorkflowNodeDefinition {
	dataPort := func(key, dataType, direction, cardinality string, required bool) iapiserver.WorkflowPortDefinition {
		return iapiserver.WorkflowPortDefinition{
			Key: key, Label: key, Direction: direction, DataType: dataType, Required: required,
			Cardinality: cardinality, ConnectionType: "data",
		}
	}
	definition := func(nodeType, title, category, kind, mode, compilerKey, rendererKey string, ports []iapiserver.WorkflowPortDefinition, config map[string]any) *iapiserver.WorkflowNodeDefinition {
		binding := iapiserver.WorkflowExecutionBinding{Mode: mode, BindingVersion: builtInDefinitionVersion}
		if compilerKey != "" {
			compilerKeyCopy := compilerKey
			binding.CompilerKey = &compilerKeyCopy
		}
		item := &iapiserver.WorkflowNodeDefinition{
			NodeType: nodeType, DefinitionVersion: builtInDefinitionVersion, Title: title, Category: category,
			NodeKind: kind, Ports: ports, ConfigSchema: config, ExecutionBinding: binding,
			Renderer:          &iapiserver.WorkflowRendererCapability{RendererKey: rendererKey, RendererVersion: builtInDefinitionVersion},
			AvailabilityScope: iapiserver.CanvasAvailabilitySystem, RegisteredBy: "system",
		}
		item.ID = uuid.NewSHA1(uuid.NameSpaceOID, []byte("workflow-canvas:builtin:"+nodeType+"@"+builtInDefinitionVersion)).String()
		item.Name = nodeType + "@" + builtInDefinitionVersion
		item.Description = "OmniMAM built-in Canvas node"
		return item
	}
	emptyObject := func() map[string]any {
		return map[string]any{"type": "object", "additionalProperties": false}
	}
	loopPorts := make([]iapiserver.WorkflowPortDefinition, 0, 8)
	for _, item := range []struct{ key, dataType string }{{"prompt", "string"}, {"image", "image"}, {"video", "video"}, {"audio", "audio"}} {
		loopPorts = append(loopPorts,
			dataPort(item.key, item.dataType, "input", "single", false),
			dataPort(item.key, item.dataType, "output", "multiple", false),
		)
	}
	return []*iapiserver.WorkflowNodeDefinition{
		definition("image", "Media", "input", "data", iapiserver.CanvasExecutionCompileTime, iapiserver.CanvasCompilerMediaInput, "builtin.media_input", []iapiserver.WorkflowPortDefinition{
			dataPort("image", "image", "output", "single", false),
			dataPort("video", "video", "output", "single", false),
			dataPort("audio", "audio", "output", "single", false),
		}, map[string]any{
			"type": "object", "additionalProperties": false, "required": []any{"asset_id", "media_type"},
			"properties": map[string]any{
				"asset_id":   map[string]any{"type": "string", "minLength": 1},
				"media_type": map[string]any{"type": "string", "enum": []any{"image", "video", "audio"}},
			},
		}),
		definition("prompt", "Prompt", "input", "data", iapiserver.CanvasExecutionCompileTime, iapiserver.CanvasCompilerPrompt, "builtin.prompt", []iapiserver.WorkflowPortDefinition{
			dataPort("prompt", "string", "output", "single", false),
		}, map[string]any{
			"type": "object", "additionalProperties": false, "required": []any{"text"},
			"properties": map[string]any{"text": map[string]any{"type": "string", "maxLength": 20000}},
		}),
		definition("loop", "Loop", "orchestration", "orchestrator", iapiserver.CanvasExecutionCompileTime, iapiserver.CanvasCompilerLoop, "builtin.loop", loopPorts, map[string]any{
			"type": "object", "additionalProperties": false, "required": []any{"mode"},
			"properties": map[string]any{
				"count":                map[string]any{"type": "integer", "minimum": 1, "maximum": 99, "default": 1},
				"mode":                 map[string]any{"type": "string", "enum": []any{"serial", "batch", "cascade"}},
				"batch_inputs":         map[string]any{"type": "object", "default": map[string]any{}},
				"feedback_output_port": map[string]any{"type": "string", "minLength": 1},
				"feedback_input_port":  map[string]any{"type": "string", "minLength": 1},
			},
			"allOf": []any{map[string]any{
				"if":   map[string]any{"properties": map[string]any{"mode": map[string]any{"const": "cascade"}}, "required": []any{"mode"}},
				"then": map[string]any{"required": []any{"feedback_output_port", "feedback_input_port"}},
			}},
		}),
		definition("group", "Group", "orchestration", "orchestrator", iapiserver.CanvasExecutionPassive, "", "builtin.group", nil, map[string]any{
			"type": "object", "additionalProperties": false,
			"properties": map[string]any{"title": map[string]any{"type": "string"}},
		}),
		definition("promptGroup", "Prompt Group", "input", "data", iapiserver.CanvasExecutionCompileTime, iapiserver.CanvasCompilerPromptGroup, "builtin.prompt_group", []iapiserver.WorkflowPortDefinition{
			dataPort("prompts", "string", "input", "multiple", true),
			dataPort("prompt", "string", "output", "single", false),
		}, emptyObject()),
		definition("output", "Output", "output", "viewer", iapiserver.CanvasExecutionCompileTime, iapiserver.CanvasCompilerOutput, "builtin.output", []iapiserver.WorkflowPortDefinition{
			dataPort("image", "image", "input", "multiple", false),
			dataPort("video", "video", "input", "multiple", false),
			dataPort("audio", "audio", "input", "multiple", false),
		}, emptyObject()),
	}
}

func registeredBuiltInCompilerKey(nodeType, version, compilerKey string) bool {
	return version == builtInDefinitionVersion && builtInCompilerKeys[nodeType] == compilerKey
}
