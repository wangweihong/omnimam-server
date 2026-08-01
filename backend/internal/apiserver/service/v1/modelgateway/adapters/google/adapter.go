package google

import (
	"context"
	"net/http"

	"github.com/wangweihong/gotoolbox/pkg/maputil"
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/provider"
)

const (
	AdapterID  = "google_gemini"
	ExecutorID = "google_gemini_interactions"
)

// Adapter 对 Google Gemini 原生 API 使用 x-goog-api-key 执行健康探测。
type Adapter struct{}

func (Adapter) ID() string { return AdapterID }

func (Adapter) Check(ctx context.Context, engine *iapiserver.EngineInstance) (*iapiserver.EngineHealthCheckResult, error) {
	if err := provider.Probe(ctx, engine, http.MethodGet, "/models", provider.WithAPIKeyHeader("x-goog-api-key")); err != nil {
		return nil, err
	}
	return &iapiserver.EngineHealthCheckResult{EngineInstanceID: engine.ID, HealthStatus: iapiserver.EngineHealthOnline}, nil
}

// Executor 将标准运行输入映射到 Google Gemini Interactions API。
type Executor struct{}

func (Executor) ID() string { return ExecutorID }

func (Executor) Execute(ctx context.Context, engine *iapiserver.EngineInstance, run *iapiserver.ApplicationRun) (map[string]any, error) {
	payload := maputil.Clone(run.InputSnapshot)
	if _, ok := payload["model"]; !ok {
		payload["model"] = providerModelID(run, payload)
	}
	delete(payload, "model_id")
	response, err := provider.Invoke(ctx, engine, http.MethodPost, "/interactions", payload, provider.WithAPIKeyHeader("x-goog-api-key"))
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
