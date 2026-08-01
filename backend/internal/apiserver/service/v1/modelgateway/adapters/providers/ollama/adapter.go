package ollama

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/maputil"
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/transports/httpjson"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

const (
	AdapterID           = "ollama"
	ChatExecutorID      = "ollama_chat_completions"
	ResponsesExecutorID = "ollama_responses_create"
)

// Adapter 对本地 Ollama OpenAI-compatible API 执行健康探测。
type Adapter struct{}

func (Adapter) ID() string { return AdapterID }

// DiscoverModels 返回当前 Ollama 实例已安装的真实模型名，不缓存或持久化结果。
func (Adapter) DiscoverModels(ctx context.Context, engine *iapiserver.EngineInstance) ([]string, error) {
	baseURL, err := nativeAPIBaseURL(engine)
	if err != nil {
		return nil, err
	}
	discoveryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	response, err := httpjson.Invoke(discoveryCtx, engine, http.MethodGet, "/api/tags", nil, httpjson.WithBaseURL(baseURL))
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	models := []string{}
	for _, raw := range responseModels(response["models"]) {
		name := maputil.FirstString(raw, "name", "model")
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		models = append(models, name)
	}
	sort.Strings(models)
	return models, nil
}

func responseModels(raw any) []map[string]any {
	values, _ := raw.([]any)
	models := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if model, ok := value.(map[string]any); ok {
			models = append(models, model)
		}
	}
	return models
}

type modelInstalledValidator struct{ discoverer Adapter }

func (modelInstalledValidator) ID() string { return "ollama.model-installed" }
func (v modelInstalledValidator) Validate(ctx context.Context, request modelgateway.CapabilityValidationRequest) error {
	if request.Engine == nil {
		return fmt.Errorf("engine instance is required")
	}
	selected := maputil.FirstString(request.Value, "model")
	models, err := v.discoverer.DiscoverModels(ctx, request.Engine)
	if err != nil {
		return err
	}
	for _, model := range models {
		if model == selected {
			return nil
		}
	}
	return fmt.Errorf("model %q is not installed on the selected Ollama instance", selected)
}

// NewModelInstalledValidator 构造 Ollama 执行前实例模型复检器。
func NewModelInstalledValidator() modelgateway.CapabilityValidator {
	return modelInstalledValidator{discoverer: Adapter{}}
}

func (Adapter) Check(ctx context.Context, engine *iapiserver.EngineInstance) (*iapiserver.EngineHealthCheckResult, error) {
	baseURL, err := nativeAPIBaseURL(engine)
	if err != nil {
		return nil, err
	}
	if err := httpjson.Probe(ctx, engine, http.MethodGet, "/api/tags", httpjson.WithBaseURL(baseURL)); err != nil {
		return nil, err
	}
	return &iapiserver.EngineHealthCheckResult{EngineInstanceID: engine.ID, HealthStatus: iapiserver.EngineHealthOnline}, nil
}

// Executor 使用请求中真实的本地模型名调用 Ollama，不维护伪造的全局模型目录。
type Executor struct {
	id   string
	path string
}

func NewExecutor(id, path string) *Executor { return &Executor{id: id, path: path} }
func (e *Executor) ID() string              { return e.id }

func (e *Executor) Execute(ctx context.Context, engine *iapiserver.EngineInstance, run *iapiserver.ApplicationRun) (map[string]any, error) {
	payload := maputil.Clone(run.InputSnapshot)
	model := maputil.FirstString(payload, "model", "model_id")
	if model == "" {
		return nil, errors.NewStatus(code.ErrAIAppApplicationInputInvalid, "Ollama model is required and must exist on the selected instance")
	}
	payload["model"] = model
	delete(payload, "model_id")
	response, err := httpjson.Invoke(ctx, engine, http.MethodPost, e.path, payload)
	if err != nil {
		return nil, err
	}
	return map[string]any{"values": response}, nil
}

func nativeAPIBaseURL(engine *iapiserver.EngineInstance) (string, error) {
	if engine == nil {
		return "", errors.NewStatus(code.ErrAIAppEngineUnavailable, "engine is required")
	}
	parsed, err := url.Parse(engine.BaseURL)
	if err != nil || parsed.Host == "" {
		return "", errors.NewStatus(code.ErrAIAppEngineUnavailable, "Ollama base URL is invalid")
	}
	parsed.Path = strings.TrimSuffix(strings.TrimRight(parsed.Path, "/"), "/v1")
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}
