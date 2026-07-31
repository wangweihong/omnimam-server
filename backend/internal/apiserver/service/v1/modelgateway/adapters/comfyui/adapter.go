package comfyui

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/maputil"
	"github.com/wangweihong/gotoolbox/pkg/sliceutil"
	"github.com/wangweihong/gotoolbox/pkg/typeutil"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/provider"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

const (
	AdapterID  = "comfyui"
	ExecutorID = "comfyui_workflow"
)

// Adapter 对 ComfyUI API 执行健康探测并读取节点与版本元数据。
type Adapter struct{}

func (Adapter) ID() string { return AdapterID }

func (Adapter) Check(ctx context.Context, engine *iapiserver.EngineInstance) (*iapiserver.EngineHealthCheckResult, error) {
	if _, err := provider.Invoke(ctx, engine, http.MethodGet, "/system_stats", nil); err != nil {
		return nil, err
	}
	return &iapiserver.EngineHealthCheckResult{EngineInstanceID: engine.ID, HealthStatus: iapiserver.EngineHealthOnline}, nil
}

func (Adapter) ReadObjectInfo(ctx context.Context, engine *iapiserver.EngineInstance) (map[string]any, error) {
	result, err := provider.Invoke(ctx, engine, http.MethodGet, "/object_info", nil)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIObjectInfoUnavailable, err.Error())
	}
	if len(result) == 0 {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIObjectInfoUnavailable, "ComfyUI object_info is empty")
	}
	return result, nil
}

func (Adapter) ReadComfyUIVersion(ctx context.Context, engine *iapiserver.EngineInstance) (string, error) {
	result, err := provider.Invoke(ctx, engine, http.MethodGet, "/system_stats", nil)
	if err != nil {
		return "", err
	}
	if version := maputil.FirstString(result, "comfyui_version", "version"); version != "" {
		return version, nil
	}
	return maputil.FirstString(typeutil.As[map[string]any](result["system"]), "comfyui_version", "version"), nil
}

// Executor 将运行快照映射为 ComfyUI workflow，并等待历史结果完成。
type Executor struct{}

func (Executor) ID() string { return ExecutorID }

func (Executor) Execute(ctx context.Context, engine *iapiserver.EngineInstance, run *iapiserver.ApplicationRun) (map[string]any, error) {
	workflow, _ := run.CapabilitySourceSnapshot["comfyui_api_workflow"].(map[string]any)
	if len(workflow) == 0 {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "ComfyUI API workflow is missing")
	}
	workflow, err := ApplyInputs(workflow, run.InputSnapshot, typeutil.As[map[string]any](run.CapabilitySourceSnapshot["template_contract"]))
	if err != nil {
		return nil, err
	}
	submitted, err := provider.Invoke(ctx, engine, http.MethodPost, "/prompt", map[string]any{"prompt": workflow, "client_id": run.ID})
	if err != nil {
		return nil, err
	}
	promptID, _ := submitted["prompt_id"].(string)
	if promptID == "" {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "ComfyUI response does not contain prompt_id")
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_, cancelErr := provider.Invoke(context.WithoutCancel(ctx), engine, http.MethodPost, "/interrupt", map[string]any{})
			return nil, stderrors.Join(ctx.Err(), cancelErr)
		case <-ticker.C:
			history, pollErr := provider.Invoke(ctx, engine, http.MethodGet, "/history/"+url.PathEscape(promptID), nil)
			if pollErr != nil {
				return nil, pollErr
			}
			entry, _ := history[promptID].(map[string]any)
			if len(entry) == 0 {
				continue
			}
			if status, _ := entry["status"].(map[string]any); status != nil {
				if completed, _ := status["completed"].(bool); !completed {
					continue
				}
			}
			outputs, _ := entry["outputs"].(map[string]any)
			return map[string]any{"values": outputs, "artifacts": artifacts(engine.BaseURL, outputs, typeutil.As[map[string]any](run.CapabilitySourceSnapshot["template_contract"]))}, nil
		}
	}
}

// ApplyInputs 按模板契约将已解析运行输入映射到独立的 ComfyUI API workflow 副本。
func ApplyInputs(workflow, inputs, contract map[string]any) (map[string]any, error) {
	encoded, err := json.Marshal(workflow)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "ComfyUI workflow snapshot could not be copied")
	}
	var resolved map[string]any
	if err := json.Unmarshal(encoded, &resolved); err != nil {
		return nil, errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "ComfyUI workflow snapshot could not be copied")
	}
	mappings := typeutil.As[map[string]any](contract["request_mapping"])
	if len(mappings) == 0 && contract["parameter_mappings"] != nil {
		for _, raw := range sliceutil.ToInterfaceSlice(contract["fixed_parameters"]) {
			fixed := typeutil.As[map[string]any](raw)
			if err := setInput(resolved, typeutil.As[string](fixed["node_id"]), typeutil.As[string](fixed["input_name"]), fixed["value"]); err != nil {
				return nil, err
			}
		}
		for _, raw := range sliceutil.ToInterfaceSlice(contract["parameter_mappings"]) {
			mapping := typeutil.As[map[string]any](raw)
			value, exists := inputs[typeutil.As[string](mapping["input_key"])]
			if !exists {
				continue
			}
			value, err = convertValue(value, typeutil.As[string](mapping["conversion_type"]), typeutil.As[map[string]any](mapping["config"]))
			if err != nil {
				return nil, err
			}
			for _, targetRaw := range sliceutil.ToInterfaceSlice(mapping["targets"]) {
				target := typeutil.As[map[string]any](targetRaw)
				if err := setInput(resolved, typeutil.As[string](target["node_id"]), typeutil.As[string](target["input_name"]), value); err != nil {
					return nil, err
				}
			}
		}
		return resolved, nil
	}
	for field, raw := range mappings {
		value, exists := inputs[field]
		if !exists {
			continue
		}
		nodeID, inputName := "", ""
		switch mapping := raw.(type) {
		case string:
			parts := strings.Split(strings.TrimPrefix(mapping, "prompt."), ".")
			if len(parts) == 3 && parts[1] == "inputs" {
				nodeID, inputName = parts[0], parts[2]
			}
		case map[string]any:
			nodeID = maputil.FirstString(mapping, "node_id", "target_node_id")
			inputName = maputil.FirstString(mapping, "input_name", "target_input")
			if target := maputil.FirstString(mapping, "target"); nodeID == "" && target != "" {
				parts := strings.Split(strings.TrimPrefix(target, "prompt."), ".")
				if len(parts) == 3 && parts[1] == "inputs" {
					nodeID, inputName = parts[0], parts[2]
				}
			}
		}
		if nodeID == "" || inputName == "" {
			return nil, errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "ComfyUI request mapping is invalid for "+field)
		}
		if err := setInput(resolved, nodeID, inputName, value); err != nil {
			return nil, err
		}
	}
	return resolved, nil
}

func setInput(workflow map[string]any, nodeID, inputName string, value any) error {
	if nodeID == "" || inputName == "" {
		return errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "ComfyUI mapping target is incomplete")
	}
	node := typeutil.As[map[string]any](workflow[nodeID])
	if node == nil {
		return errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "ComfyUI mapping references missing node "+nodeID)
	}
	nodeInputs := typeutil.As[map[string]any](node["inputs"])
	if nodeInputs == nil {
		nodeInputs = map[string]any{}
		node["inputs"] = nodeInputs
	}
	nodeInputs[inputName] = value
	return nil
}

func convertValue(value any, conversion string, config map[string]any) (any, error) {
	switch conversion {
	case "", "DIRECT", "MULTI_TARGET_MAP":
		return value, nil
	case "FIXED_VALUE":
		return config["value"], nil
	case "ENUM_MAP", "ASPECT_RATIO_TO_SIZE":
		if mapped, ok := typeutil.As[map[string]any](config["values"])[fmt.Sprint(value)]; ok {
			return mapped, nil
		}
		return nil, errors.NewStatus(code.ErrAIAppApplicationInputInvalid, "ComfyUI mapping has no value for input")
	case "BOOLEAN_SWITCH":
		if enabled, _ := value.(bool); enabled {
			return config["true_value"], nil
		}
		return config["false_value"], nil
	case "RANGE_SCALE":
		number, ok := value.(float64)
		if !ok {
			return nil, errors.NewStatus(code.ErrAIAppApplicationInputInvalid, "range scale input is not numeric")
		}
		scale, _ := config["scale"].(float64)
		offset, _ := config["offset"].(float64)
		return number*scale + offset, nil
	case "CONCAT":
		var builder strings.Builder
		for _, part := range sliceutil.ToInterfaceSlice(config["parts"]) {
			if part == "$value" {
				builder.WriteString(fmt.Sprint(value))
			} else {
				builder.WriteString(fmt.Sprint(part))
			}
		}
		return builder.String(), nil
	case "TEMPLATE_STRING":
		return strings.ReplaceAll(typeutil.As[string](config["template"]), "{{value}}", fmt.Sprint(value)), nil
	case "CONDITIONAL":
		if mapped, ok := typeutil.As[map[string]any](config["cases"])[fmt.Sprint(value)]; ok {
			return mapped, nil
		}
		return config["default"], nil
	default:
		return nil, errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "unsupported ComfyUI conversion type "+conversion)
	}
}

func artifacts(baseURL string, outputs, contract map[string]any) []map[string]any {
	result := []map[string]any{}
	configured := sliceutil.ToInterfaceSlice(contract["outputs"])
	if len(configured) > 0 {
		for _, raw := range configured {
			definition := typeutil.As[map[string]any](raw)
			nodeID := typeutil.As[string](definition["node_id"])
			node := typeutil.As[map[string]any](outputs[nodeID])
			mediaType := typeutil.As[string](definition["media_type"])
			keys := map[string]string{"image": "images", "video": "videos", "audio": "audio"}
			for index, itemRaw := range sliceutil.ToInterfaceSlice(node[keys[mediaType]]) {
				item := typeutil.As[map[string]any](itemRaw)
				filename := maputil.FirstString(item, "filename")
				if filename == "" {
					continue
				}
				ref := artifactURL(baseURL, filename, maputil.FirstString(item, "subfolder"), "")
				result = append(result, map[string]any{"output_key": typeutil.As[string](definition["key"]), "media_type": mediaType, "content_ref": ref, "node_id": nodeID, "index": index})
			}
		}
		return result
	}
	for nodeID, raw := range outputs {
		node, _ := raw.(map[string]any)
		for _, media := range []struct{ key, mediaType string }{{"images", "image"}, {"videos", "video"}, {"audio", "audio"}} {
			for index, itemRaw := range sliceutil.ToInterfaceSlice(node[media.key]) {
				item := typeutil.As[map[string]any](itemRaw)
				filename := maputil.FirstString(item, "filename")
				if filename == "" {
					continue
				}
				result = append(result, map[string]any{"output_key": fmt.Sprintf("%s.%s.%d", nodeID, media.key, index), "media_type": media.mediaType, "content_ref": artifactURL(baseURL, filename, maputil.FirstString(item, "subfolder"), maputil.FirstString(item, "type"))})
			}
		}
	}
	return result
}

func artifactURL(baseURL, filename, subfolder, storageType string) string {
	query := url.Values{"filename": {filename}}
	if subfolder != "" {
		query.Set("subfolder", subfolder)
	}
	if storageType != "" {
		query.Set("type", storageType)
	}
	return strings.TrimRight(baseURL, "/") + "/view?" + query.Encode()
}
