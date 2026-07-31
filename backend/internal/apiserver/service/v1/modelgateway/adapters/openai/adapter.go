package openai

import (
	"context"
	"net/http"

	"github.com/wangweihong/gotoolbox/pkg/maputil"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/provider"
)

// Adapter 对 OpenAI-compatible API 执行协议级健康探测。
type Adapter struct{ id string }

// NewAdapter 构造绑定到 Runtime Registry adapter ID 的 OpenAI-compatible 适配器。
func NewAdapter(id string) *Adapter { return &Adapter{id: id} }

func (a *Adapter) ID() string { return a.id }

func (*Adapter) Check(ctx context.Context, engine *iapiserver.EngineInstance) (*iapiserver.EngineHealthCheckResult, error) {
	if _, err := provider.Invoke(ctx, engine, http.MethodGet, "/models", nil); err != nil {
		return nil, err
	}
	return &iapiserver.EngineHealthCheckResult{EngineInstanceID: engine.ID, HealthStatus: iapiserver.EngineHealthOnline}, nil
}

// Executor 将运行快照转换为 OpenAI-compatible chat completions 请求。
type Executor struct{ id string }

// NewExecutor 构造绑定到 Runtime Registry operation ID 的 OpenAI-compatible 执行器。
func NewExecutor(id string) *Executor { return &Executor{id: id} }

func (e *Executor) ID() string { return e.id }

func (*Executor) Execute(ctx context.Context, engine *iapiserver.EngineInstance, run *iapiserver.ApplicationRun) (map[string]any, error) {
	payload := maputil.Clone(run.InputSnapshot)
	if _, ok := payload["model"]; !ok {
		if model := providerModelID(run, payload); model != "" {
			payload["model"] = model
		}
	}
	response, err := provider.Invoke(ctx, engine, http.MethodPost, "/chat/completions", payload)
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
