package applicationplatform

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/httpcli"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

const providerPollInterval = 500 * time.Millisecond

type protocolAdapter struct {
	id string
}

type protocolExecutor struct {
	id       string
	provider string
}

// NewEngineAdapters constructs the immutable adapter set injected into Application Platform.
func NewEngineAdapters() map[string]EngineAdapter {
	return map[string]EngineAdapter{
		"comfyui":           &protocolAdapter{id: "comfyui"},
		"byteplus_modelark": &protocolAdapter{id: "byteplus_modelark"},
		"deepseek_official": &protocolAdapter{id: "deepseek_official"},
	}
}

// NewOperationExecutors constructs the immutable operation executor set referenced by Runtime Registry.
func NewOperationExecutors() map[string]OperationExecutor {
	return map[string]OperationExecutor{
		"comfyui_workflow":            &protocolExecutor{id: "comfyui_workflow", provider: "comfyui"},
		"byteplus_text_to_video":      &protocolExecutor{id: "byteplus_text_to_video", provider: "byteplus_modelark"},
		"byteplus_image_to_video":     &protocolExecutor{id: "byteplus_image_to_video", provider: "byteplus_modelark"},
		"byteplus_reference_to_video": &protocolExecutor{id: "byteplus_reference_to_video", provider: "byteplus_modelark"},
		"deepseek_chat_completions":   &protocolExecutor{id: "deepseek_chat_completions", provider: "deepseek_official"},
	}
}

func (a *protocolAdapter) ID() string { return a.id }

// ReadObjectInfo 获取 ComfyUI 实例的节点能力事实；该调用不会提交或执行工作流。
func (a *protocolAdapter) ReadObjectInfo(ctx context.Context, engine *iapiserver.EngineInstance) (map[string]any, error) {
	if a.id != "comfyui" {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIEngineTypeInvalid, "engine adapter is not ComfyUI")
	}
	result, err := invokeProvider(ctx, engine, http.MethodGet, "/object_info", nil)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIObjectInfoUnavailable, err.Error())
	}
	if len(result) == 0 {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIObjectInfoUnavailable, "ComfyUI object_info is empty")
	}
	return result, nil
}

// Check performs the provider-specific lightweight health request using EngineInstance credentials.
func (a *protocolAdapter) Check(ctx context.Context, engine *iapiserver.EngineInstance) (*iapiserver.EngineHealthCheckResult, error) {
	requestPath := map[string]string{
		"comfyui":           "/system_stats",
		"byteplus_modelark": "/api/v3/contents/generations/tasks",
		"deepseek_official": "/models",
	}[a.id]
	if requestPath == "" {
		return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "engine adapter is unavailable")
	}
	_, err := invokeProvider(ctx, engine, http.MethodGet, requestPath, nil)
	if err != nil {
		return nil, err
	}
	return &iapiserver.EngineHealthCheckResult{EngineInstanceID: engine.ID, HealthStatus: iapiserver.EngineHealthOnline}, nil
}

// Execute translates an immutable ApplicationRun snapshot to the selected external protocol.
func (e *protocolExecutor) ID() string { return e.id }

func (e *protocolExecutor) Execute(ctx context.Context, engine *iapiserver.EngineInstance, run *iapiserver.ApplicationRun) (map[string]any, error) {
	switch e.provider {
	case "comfyui":
		return executeComfyUI(ctx, engine, run)
	case "byteplus_modelark":
		return executeModelArk(ctx, engine, run)
	case "deepseek_official":
		return executeDeepSeek(ctx, engine, run)
	default:
		return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "engine adapter is unavailable")
	}
}

func executeDeepSeek(ctx context.Context, engine *iapiserver.EngineInstance, run *iapiserver.ApplicationRun) (map[string]any, error) {
	payload := copyMap(run.InputSnapshot)
	if _, ok := payload["model"]; !ok {
		if model := providerModelID(run, payload); model != "" {
			payload["model"] = model
		}
	}
	response, err := invokeProvider(ctx, engine, http.MethodPost, "/chat/completions", payload)
	if err != nil {
		return nil, err
	}
	return map[string]any{"values": response}, nil
}

func executeComfyUI(ctx context.Context, engine *iapiserver.EngineInstance, run *iapiserver.ApplicationRun) (map[string]any, error) {
	workflow, _ := run.CapabilitySourceSnapshot["comfyui_api_workflow"].(map[string]any)
	if len(workflow) == 0 {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "ComfyUI API workflow is missing")
	}
	workflow, err := applyComfyInputs(workflow, run.InputSnapshot, mapValue(run.CapabilitySourceSnapshot["template_contract"]))
	if err != nil {
		return nil, err
	}
	submitted, err := invokeProvider(ctx, engine, http.MethodPost, "/prompt", map[string]any{"prompt": workflow, "client_id": run.ID})
	if err != nil {
		return nil, err
	}
	promptID, _ := submitted["prompt_id"].(string)
	if promptID == "" {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "ComfyUI response does not contain prompt_id")
	}
	ticker := time.NewTicker(providerPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_, cancelErr := invokeProvider(context.WithoutCancel(ctx), engine, http.MethodPost, "/interrupt", map[string]any{})
			return nil, stderrors.Join(ctx.Err(), cancelErr)
		case <-ticker.C:
			history, pollErr := invokeProvider(ctx, engine, http.MethodGet, "/history/"+url.PathEscape(promptID), nil)
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
			return map[string]any{"values": outputs, "artifacts": comfyArtifacts(engine.BaseURL, outputs, mapValue(run.CapabilitySourceSnapshot["template_contract"]))}, nil
		}
	}
}

func executeModelArk(ctx context.Context, engine *iapiserver.EngineInstance, run *iapiserver.ApplicationRun) (map[string]any, error) {
	payload := copyMap(run.InputSnapshot)
	if _, ok := payload["model"]; !ok {
		payload["model"] = providerModelID(run, payload)
	}
	if prompt, ok := payload["prompt"].(string); ok {
		payload["content"] = []map[string]any{{"type": "text", "text": prompt}}
		delete(payload, "prompt")
	}
	requestPath := "/api/v3/contents/generations/tasks"
	submitted, err := invokeProvider(ctx, engine, http.MethodPost, requestPath, payload)
	if err != nil {
		return nil, err
	}
	taskID := firstString(submitted, "id", "task_id")
	if taskID == "" {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "ModelArk response does not contain task id")
	}
	ticker := time.NewTicker(providerPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_, cancelErr := invokeProvider(context.WithoutCancel(ctx), engine, http.MethodDelete, requestPath+"/"+url.PathEscape(taskID), nil)
			return nil, stderrors.Join(ctx.Err(), cancelErr)
		case <-ticker.C:
			result, pollErr := invokeProvider(ctx, engine, http.MethodGet, requestPath+"/"+url.PathEscape(taskID), nil)
			if pollErr != nil {
				return nil, pollErr
			}
			status := strings.ToLower(firstString(result, "status", "state"))
			switch status {
			case "", "queued", "pending", "running", "processing":
				continue
			case "succeeded", "success", "completed":
				return map[string]any{"values": result, "artifacts": modelArkArtifacts(result)}, nil
			default:
				return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "ModelArk rejected the generation task")
			}
		}
	}
}

func invokeProvider(ctx context.Context, engine *iapiserver.EngineInstance, method, requestPath string, payload any) (map[string]any, error) {
	if engine == nil || strings.TrimSpace(engine.BaseURL) == "" {
		return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "engine base URL is required")
	}
	endpoint := strings.TrimRight(engine.BaseURL, "/") + "/" + strings.TrimLeft(requestPath, "/")
	builder := httpcli.NewHttpRequestBuilder().WithEndpoint(endpoint).WithMethod(method).AddHeaderParam("Accept", "application/json")
	var raw json.RawMessage
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, errors.NewStatus(code.ErrAIAppApplicationInputInvalid, "provider request could not be encoded")
		}
		raw = encoded
		builder.WithBody("json", raw).AddHeaderParam("Content-Type", "application/json")
	}
	if err := applyProviderAuthentication(builder, engine, method, requestPath, raw); err != nil {
		return nil, err
	}
	timeout := time.Duration(engine.RequestTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	response, err := builder.Build().InvokeWithContext(ctx, httpcli.TimeoutCallOption(timeout))
	if err != nil {
		if stderrors.Is(err, context.DeadlineExceeded) || stderrors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "provider request timed out")
		}
		return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "provider request failed")
	}
	body := response.GetBody()
	if response.GetStatusCode() < 200 || response.GetStatusCode() >= 300 {
		switch response.GetStatusCode() {
		case http.StatusUnauthorized, http.StatusForbidden:
			return nil, errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "provider authentication was rejected")
		case http.StatusBadRequest, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity:
			return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, providerFailureSummary(body))
		default:
			return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, providerFailureSummary(body))
		}
	}
	if strings.TrimSpace(body) == "" {
		return map[string]any{}, nil
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "provider response could not be parsed")
	}
	return decoded, nil
}

func applyProviderAuthentication(builder *httpcli.HttpRequestBuilder, engine *iapiserver.EngineInstance, method, requestPath string, body []byte) error {
	switch engine.AuthType {
	case "none", "":
		return nil
	case "api_key":
		apiKey, _ := engine.AuthConfig["api_key"].(string)
		if apiKey == "" {
			return errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "api_key is required")
		}
		if engine.ApplicationEngineTypeID == "comfyui" {
			builder.AddHeaderParam("X-API-Key", apiKey)
		} else {
			builder.AddHeaderParam("Authorization", "Bearer "+apiKey)
		}
		return nil
	case "bearer_token":
		token, _ := engine.AuthConfig["bearer_token"].(string)
		if token == "" {
			return errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "bearer_token is required")
		}
		builder.AddHeaderParam("Authorization", "Bearer "+token)
		return nil
	case "ak_sk":
		return applyModelArkSignature(builder, engine, method, requestPath, body, time.Now().UTC())
	default:
		return errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "engine authentication type is unsupported")
	}
}

// ModelArk uses the BytePlus HMAC-SHA256 request signing profile.
func applyModelArkSignature(builder *httpcli.HttpRequestBuilder, engine *iapiserver.EngineInstance, method, requestPath string, body []byte, now time.Time) error {
	accessKey, _ := engine.AuthConfig["access_key"].(string)
	secretKey, _ := engine.AuthConfig["secret_key"].(string)
	if accessKey == "" || secretKey == "" {
		return errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "access_key and secret_key are required")
	}
	parsed, err := url.Parse(engine.BaseURL)
	if err != nil || parsed.Host == "" {
		return errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "engine base URL is invalid")
	}
	region := engine.Region
	if region == "" {
		region = "ap-southeast-1"
	}
	date := now.Format("20060102")
	xDate := now.Format("20060102T150405Z")
	bodyHash := sha256Hex(body)
	canonicalURI := path.Clean("/" + strings.TrimLeft(requestPath, "/"))
	signedHeaders := "content-type;host;x-content-sha256;x-date"
	canonicalHeaders := "content-type:application/json\n" + "host:" + parsed.Host + "\n" + "x-content-sha256:" + bodyHash + "\n" + "x-date:" + xDate + "\n"
	canonicalRequest := method + "\n" + canonicalURI + "\n\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + bodyHash
	scope := date + "/" + region + "/ark/request"
	stringToSign := "HMAC-SHA256\n" + xDate + "\n" + scope + "\n" + sha256Hex([]byte(canonicalRequest))
	kDate := hmacSHA256([]byte(secretKey), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, "ark")
	kSigning := hmacSHA256(kService, "request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))
	builder.AddHeaderParam("Host", parsed.Host)
	builder.AddHeaderParam("Content-Type", "application/json")
	builder.AddHeaderParam("X-Date", xDate)
	builder.AddHeaderParam("X-Content-Sha256", bodyHash)
	builder.AddHeaderParam("Authorization", fmt.Sprintf("HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", accessKey, scope, signedHeaders, signature))
	return nil
}

func providerModelID(run *iapiserver.ApplicationRun, inputs map[string]any) string {
	selected := firstString(inputs, "model", "model_id")
	capability, _ := run.CapabilitySourceSnapshot["provider_capability"].(map[string]any)
	models, _ := capability["models"].([]any)
	for _, raw := range models {
		model, _ := raw.(map[string]any)
		if selected == "" || firstString(model, "id") == selected {
			if providerID := firstString(model, "provider_model_id"); providerID != "" {
				return providerID
			}
		}
	}
	return selected
}

func applyComfyInputs(workflow, inputs, contract map[string]any) (map[string]any, error) {
	encoded, err := json.Marshal(workflow)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "ComfyUI workflow snapshot could not be copied")
	}
	var resolved map[string]any
	if err := json.Unmarshal(encoded, &resolved); err != nil {
		return nil, errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "ComfyUI workflow snapshot could not be copied")
	}
	mappings := mapValue(contract["request_mapping"])
	if len(mappings) == 0 && contract["parameter_mappings"] != nil {
		for _, raw := range anySlice(contract["fixed_parameters"]) {
			fixed := mapValue(raw)
			if err := setComfyInput(resolved, stringValue(fixed["node_id"]), stringValue(fixed["input_name"]), fixed["value"]); err != nil {
				return nil, err
			}
		}
		for _, raw := range anySlice(contract["parameter_mappings"]) {
			mapping := mapValue(raw)
			key := stringValue(mapping["input_key"])
			value, exists := inputs[key]
			if !exists {
				continue
			}
			value, err = convertComfyValue(value, stringValue(mapping["conversion_type"]), mapValue(mapping["config"]))
			if err != nil {
				return nil, err
			}
			for _, targetRaw := range anySlice(mapping["targets"]) {
				target := mapValue(targetRaw)
				if err := setComfyInput(resolved, stringValue(target["node_id"]), stringValue(target["input_name"]), value); err != nil {
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
			nodeID = firstString(mapping, "node_id", "target_node_id")
			inputName = firstString(mapping, "input_name", "target_input")
			if target := firstString(mapping, "target"); nodeID == "" && target != "" {
				parts := strings.Split(strings.TrimPrefix(target, "prompt."), ".")
				if len(parts) == 3 && parts[1] == "inputs" {
					nodeID, inputName = parts[0], parts[2]
				}
			}
		}
		if nodeID == "" || inputName == "" {
			return nil, errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "ComfyUI request mapping is invalid for "+field)
		}
		node := mapValue(resolved[nodeID])
		if node == nil {
			return nil, errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "ComfyUI request mapping references missing node "+nodeID)
		}
		nodeInputs := mapValue(node["inputs"])
		if nodeInputs == nil {
			nodeInputs = map[string]any{}
			node["inputs"] = nodeInputs
		}
		nodeInputs[inputName] = value
	}
	return resolved, nil
}

func setComfyInput(workflow map[string]any, nodeID, inputName string, value any) error {
	if nodeID == "" || inputName == "" {
		return errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "ComfyUI mapping target is incomplete")
	}
	node := mapValue(workflow[nodeID])
	if node == nil {
		return errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "ComfyUI mapping references missing node "+nodeID)
	}
	inputs := mapValue(node["inputs"])
	if inputs == nil {
		inputs = map[string]any{}
		node["inputs"] = inputs
	}
	inputs[inputName] = value
	return nil
}

func convertComfyValue(value any, conversion string, config map[string]any) (any, error) {
	switch conversion {
	case "", "DIRECT", "MULTI_TARGET_MAP":
		return value, nil
	case "FIXED_VALUE":
		return config["value"], nil
	case "ENUM_MAP", "ASPECT_RATIO_TO_SIZE":
		if mapped, ok := mapValue(config["values"])[fmt.Sprint(value)]; ok {
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
		parts := anySlice(config["parts"])
		var builder strings.Builder
		for _, part := range parts {
			if part == "$value" {
				builder.WriteString(fmt.Sprint(value))
			} else {
				builder.WriteString(fmt.Sprint(part))
			}
		}
		return builder.String(), nil
	case "TEMPLATE_STRING":
		template := stringValue(config["template"])
		return strings.ReplaceAll(template, "{{value}}", fmt.Sprint(value)), nil
	case "CONDITIONAL":
		if cases := mapValue(config["cases"]); cases != nil {
			if mapped, ok := cases[fmt.Sprint(value)]; ok {
				return mapped, nil
			}
		}
		return config["default"], nil
	default:
		return nil, errors.NewStatus(code.ErrAIAppTemplateSourceInvalid, "unsupported ComfyUI conversion type "+conversion)
	}
}

func comfyArtifacts(baseURL string, outputs, contract map[string]any) []map[string]any {
	artifacts := []map[string]any{}
	configured := anySlice(contract["outputs"])
	if len(configured) > 0 {
		for _, raw := range configured {
			definition := mapValue(raw)
			nodeID := stringValue(definition["node_id"])
			node := mapValue(outputs[nodeID])
			mediaType := stringValue(definition["media_type"])
			keys := map[string]string{"image": "images", "video": "videos", "audio": "audio"}
			items := anySlice(node[keys[mediaType]])
			for index, itemRaw := range items {
				item := mapValue(itemRaw)
				filename := firstString(item, "filename")
				if filename == "" {
					continue
				}
				subfolder := firstString(item, "subfolder")
				ref := strings.TrimRight(baseURL, "/") + "/view?filename=" + url.QueryEscape(filename)
				if subfolder != "" {
					ref += "&subfolder=" + url.QueryEscape(subfolder)
				}
				artifacts = append(artifacts, map[string]any{"output_key": stringValue(definition["key"]), "media_type": mediaType, "content_ref": ref, "node_id": nodeID, "index": index})
			}
		}
		return artifacts
	}
	for nodeID, raw := range outputs {
		node, _ := raw.(map[string]any)
		for _, media := range []struct{ key, mediaType string }{{"images", "image"}, {"videos", "video"}, {"audio", "audio"}} {
			items, _ := node[media.key].([]any)
			for index, itemRaw := range items {
				item, _ := itemRaw.(map[string]any)
				filename := firstString(item, "filename")
				if filename == "" {
					continue
				}
				query := url.Values{"filename": {filename}}
				if subfolder := firstString(item, "subfolder"); subfolder != "" {
					query.Set("subfolder", subfolder)
				}
				if kind := firstString(item, "type"); kind != "" {
					query.Set("type", kind)
				}
				artifacts = append(artifacts, map[string]any{"output_key": fmt.Sprintf("%s.%s.%d", nodeID, media.key, index), "media_type": media.mediaType, "content_ref": strings.TrimRight(baseURL, "/") + "/view?" + query.Encode()})
			}
		}
	}
	return artifacts
}

func modelArkArtifacts(result map[string]any) []map[string]any {
	artifacts := []map[string]any{}
	for _, field := range []struct{ key, mediaType string }{{"video_url", "video"}, {"last_frame_url", "image"}, {"image_url", "image"}} {
		if ref := firstString(result, field.key); ref != "" {
			artifacts = append(artifacts, map[string]any{"output_key": field.key, "media_type": field.mediaType, "content_ref": ref})
		}
	}
	if content, _ := result["content"].(map[string]any); content != nil {
		artifacts = append(artifacts, modelArkArtifacts(content)...)
	}
	return artifacts
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func copyMap(source map[string]any) map[string]any {
	target := make(map[string]any, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}

func providerFailureSummary(body string) string {
	body = strings.TrimSpace(body)
	if len(body) > 512 {
		body = body[:512]
	}
	if body == "" {
		return "provider request was rejected"
	}
	return body
}

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}
