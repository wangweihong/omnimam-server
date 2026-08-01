package openaicompat

import (
	"context"
	"net/http"

	"github.com/wangweihong/gotoolbox/pkg/maputil"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/transports/httpjson"
)

// Adapter 对 OpenAI-compatible API 执行协议级健康探测。
type Adapter struct{ id string }

// NewAdapter 构造绑定到 Runtime Registry adapter ID 的 OpenAI-compatible 适配器。
func NewAdapter(id string) *Adapter { return &Adapter{id: id} }

func (a *Adapter) ID() string { return a.id }

func (*Adapter) Check(ctx context.Context, engine *iapiserver.EngineInstance) (*iapiserver.EngineHealthCheckResult, error) {
	if err := httpjson.Probe(ctx, engine, http.MethodGet, "/models"); err != nil {
		return nil, err
	}
	return &iapiserver.EngineHealthCheckResult{EngineInstanceID: engine.ID, HealthStatus: iapiserver.EngineHealthOnline}, nil
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
