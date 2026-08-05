package openaicompat

import (
	"context"
	"net/http"
	"sort"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/maputil"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/transports/httpjson"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

// Adapter 对 OpenAI-compatible API 执行协议级健康探测。
type Adapter struct{ id string }

// NewAdapter 构造绑定到 Runtime Registry adapter ID 的 OpenAI-compatible 适配器。
func NewAdapter(id string) *Adapter { return &Adapter{id: id} }

func (a *Adapter) ID() string { return a.id }

func (*Adapter) Check(ctx context.Context, engine *iapiserver.EngineInstance) (*iapiserver.EngineHealthCheckResult, error) {
	if err := httpjson.Probe(ctx, engine, http.MethodGet, "/models", providerOptions(engine)...); err != nil {
		return nil, err
	}
	return &iapiserver.EngineHealthCheckResult{EngineInstanceID: engine.ID, HealthStatus: iapiserver.EngineHealthOnline}, nil
}

// DiscoverProviderModels 从 OpenAI-compatible `/models` 的 `data[].id` 读取远端模型目录。
func (*Adapter) DiscoverProviderModels(ctx context.Context, engine *iapiserver.EngineInstance) ([]modelgateway.DiscoveredModel, error) {
	response, err := httpjson.Invoke(ctx, engine, http.MethodGet, "/models", nil, providerOptions(engine)...)
	if err != nil {
		return nil, err
	}
	data, ok := response["data"].([]any)
	if !ok {
		return nil, errors.NewStatus(code.ErrAIAppProviderResponseInvalid, "provider model list does not contain data")
	}
	byID := make(map[string]modelgateway.DiscoveredModel, len(data))
	for _, raw := range data {
		item, _ := raw.(map[string]any)
		id, _ := item["id"].(string)
		id = strings.TrimSpace(id)
		if id != "" {
			byID[id] = modelgateway.DiscoveredModel{RemoteModel: id, DisplayName: id}
		}
	}
	result := make([]modelgateway.DiscoveredModel, 0, len(byID))
	for _, item := range byID {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].RemoteModel < result[j].RemoteModel })
	return result, nil
}

// ProbeProviderModel 只查询模型目录并匹配远端 ID，不发送生成请求。
func (a *Adapter) ProbeProviderModel(ctx context.Context, engine *iapiserver.EngineInstance, remoteModel string) (*modelgateway.ModelProbeResult, error) {
	models, err := a.DiscoverProviderModels(ctx, engine)
	if err != nil {
		return nil, err
	}
	remoteModel = strings.TrimSpace(remoteModel)
	index := sort.Search(len(models), func(index int) bool { return models[index].RemoteModel >= remoteModel })
	if index == len(models) || models[index].RemoteModel != remoteModel {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "provider model was not found")
	}
	return &modelgateway.ModelProbeResult{RemoteModel: remoteModel, Available: true, StreamSupported: true}, nil
}

func providerOptions(engine *iapiserver.EngineInstance) []httpjson.InvokeOption {
	if engine == nil {
		return nil
	}
	options := make([]httpjson.InvokeOption, 0, 2)
	if organization, _ := engine.AuthConfig["organization"].(string); strings.TrimSpace(organization) != "" {
		options = append(options, httpjson.WithHeader("OpenAI-Organization", organization))
	}
	if project, _ := engine.AuthConfig["project"].(string); strings.TrimSpace(project) != "" {
		options = append(options, httpjson.WithHeader("OpenAI-Project", project))
	}
	return options
}

// Executor 将运行快照转换为指定的 OpenAI-compatible JSON 请求。
type Executor struct {
	id          string
	requestPath string
}

// NewChatCompletionsExecutor 构造绑定到 Runtime Registry operation ID 的 Chat Completions 执行器。
func NewChatCompletionsExecutor(id string) *Executor {
	return &Executor{id: id, requestPath: "/chat/completions"}
}

// NewJSONExecutor 构造绑定到指定 OpenAI JSON endpoint 的执行器。
func NewJSONExecutor(id, requestPath string) *Executor {
	return &Executor{id: id, requestPath: requestPath}
}

func (e *Executor) ID() string { return e.id }

func (e *Executor) Execute(ctx context.Context, engine *iapiserver.EngineInstance, run *iapiserver.ApplicationRun) (map[string]any, error) {
	payload := maputil.Clone(run.InputSnapshot)
	if _, ok := payload["model"]; !ok {
		if model := providerModelID(run, payload); model != "" {
			payload["model"] = model
		}
	}
	delete(payload, "model_id")
	response, err := httpjson.Invoke(ctx, engine, http.MethodPost, e.requestPath, payload)
	if err != nil {
		return nil, err
	}
	return map[string]any{"values": response}, nil
}

func providerModelID(run *iapiserver.ApplicationRun, inputs map[string]any) string {
	selected := maputil.FirstString(inputs, "model", "model_id")
	capability, _ := run.CapabilitySourceSnapshot["provider_capability"].(map[string]any)
	models, _ := capability["models"].([]any)
	for _, raw := range models {
		model, _ := raw.(map[string]any)
		if selected == "" || maputil.FirstString(model, "id") == selected {
			if providerID := maputil.FirstString(model, "provider_model_id"); providerID != "" {
				return providerID
			}
		}
	}
	return selected
}
