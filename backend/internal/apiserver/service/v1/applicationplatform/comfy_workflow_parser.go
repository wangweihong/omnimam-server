package applicationplatform

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/richinsley/comfy2go/graphapi"
)

// ComfyWorkflowParser is the application-owned boundary around the upstream
// ComfyUI graph format. Tests can replace it without depending on comfy2go.
type ComfyWorkflowParser interface {
	VisualToAPI(visual, objectInfo map[string]any) (map[string]any, error)
}

type comfy2GoWorkflowParser struct{}

func (comfy2GoWorkflowParser) VisualToAPI(visual, objectInfo map[string]any) (api map[string]any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			api = nil
			err = fmt.Errorf("comfy2go rejected object_info: %v", recovered)
		}
	}()
	visualJSON, err := json.Marshal(visual)
	if err != nil {
		return nil, fmt.Errorf("encode visual workflow: %w", err)
	}
	objectJSON, err := json.Marshal(normalizeComfy2GoObjectInfo(objectInfo))
	if err != nil {
		return nil, fmt.Errorf("encode object_info: %w", err)
	}
	var objects graphapi.NodeObjects
	if err := json.Unmarshal(objectJSON, &objects.Objects); err != nil {
		return nil, fmt.Errorf("parse object_info with comfy2go: %w", err)
	}
	applyComfy2GoInputOrder(&objects, objectInfo)
	objects.PopulateInputProperties()
	graph, diagnostics, err := graphapi.NewGraphFromJsonReader(bytes.NewReader(visualJSON), &objects)
	if err != nil {
		return nil, fmt.Errorf("parse visual workflow with comfy2go: %w", err)
	}
	if graph == nil {
		return nil, fmt.Errorf("parse visual workflow with comfy2go: empty graph")
	}
	if diagnostics != nil && graph.HasErrors {
		return nil, fmt.Errorf("parse visual workflow with comfy2go: %v", *diagnostics)
	}
	prompt, err := graph.GraphToPrompt("omnimam-validation")
	if err != nil {
		return nil, fmt.Errorf("convert visual workflow with comfy2go: %w", err)
	}
	raw, err := json.Marshal(prompt.Nodes)
	if err != nil {
		return nil, fmt.Errorf("encode comfy2go prompt: %w", err)
	}
	api = map[string]any{}
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, fmt.Errorf("decode comfy2go prompt: %w", err)
	}
	if err := applyComfyVisualAdapters(api, visual); err != nil {
		return nil, err
	}
	return api, nil
}

func applyComfy2GoInputOrder(objects *graphapi.NodeObjects, objectInfo map[string]any) {
	for classType, object := range objects.Objects {
		if object == nil || object.Input == nil {
			continue
		}
		definition := mapValue(objectInfo[classType])
		inputOrder := mapValue(definition["input_order"])
		object.Input.OrderedRequired = orderedComfyInputNames(inputOrder["required"], object.Input.OrderedRequired, object.Input.Required)
		object.Input.OrderedOptional = orderedComfyInputNames(inputOrder["optional"], object.Input.OrderedOptional, object.Input.Optional)
	}
}

func orderedComfyInputNames(raw any, fallback []string, values map[string]*interface{}) []string {
	ordered := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, item := range anySlice(raw) {
		name := fmt.Sprint(item)
		if _, exists := values[name]; exists && !seen[name] {
			ordered = append(ordered, name)
			seen[name] = true
		}
	}
	// Older custom nodes may omit input_order. Preserve every property instead
	// of silently dropping it; comfy2go's decoded order is the fallback.
	for _, name := range fallback {
		if _, exists := values[name]; exists && !seen[name] {
			ordered = append(ordered, name)
			seen[name] = true
		}
	}
	return ordered
}

func applyComfyVisualAdapters(api, visual map[string]any) error {
	for _, rawNode := range anySlice(visual["nodes"]) {
		node := mapValue(rawNode)
		if stringValue(node["type"]) != "ZmlPowerLoraLoader" {
			continue
		}
		apiNode := mapValue(api[fmt.Sprint(node["id"])])
		inputs := mapValue(apiNode["inputs"])
		if inputs == nil {
			return fmt.Errorf("convert visual workflow with comfy2go: ZmlPowerLoraLoader prompt node is missing")
		}
		structured := mapValue(node["powerLoraLoader_data"])
		if len(anySlice(structured["entries"])) == 0 {
			return fmt.Errorf("convert visual workflow with comfy2go: ZmlPowerLoraLoader lora configuration is invalid")
		}
		encoded, err := json.Marshal(structured)
		if err != nil {
			return fmt.Errorf("encode ZmlPowerLoraLoader data: %w", err)
		}
		inputs["lora_loader_data"] = string(encoded)
	}
	return nil
}

func normalizeComfy2GoObjectInfo(source map[string]any) map[string]any {
	result := deepCopyMap(source)
	for _, rawDefinition := range result {
		definition := mapValue(rawDefinition)
		input := mapValue(definition["input"])
		for _, sectionName := range []string{"required", "optional"} {
			section := mapValue(input[sectionName])
			for _, rawSpec := range section {
				spec := anySlice(rawSpec)
				if len(spec) < 2 {
					continue
				}
				kind := fmt.Sprint(spec[0])
				options := mapValue(spec[1])
				if options == nil {
					continue
				}
				defaultValue, hasDefault := options["default"]
				if !hasDefault {
					continue
				}
				switch kind {
				case "BOOLEAN":
					if _, ok := defaultValue.(bool); !ok {
						delete(options, "default")
					}
				case "INT", "FLOAT":
					switch defaultValue.(type) {
					case float64, json.Number, int, int64:
					default:
						delete(options, "default")
					}
				}
			}
		}
	}
	return result
}

func detectComfyWorkflowSource(value map[string]any) string {
	if _, nodes := value["nodes"].([]any); nodes {
		if _, links := value["links"].([]any); links {
			return "visual_workflow"
		}
	}
	for _, raw := range value {
		node, ok := raw.(map[string]any)
		if !ok || stringValue(node["class_type"]) == "" || mapValue(node["inputs"]) == nil {
			return ""
		}
	}
	if len(value) > 0 {
		return "api_workflow"
	}
	return ""
}
