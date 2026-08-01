package ollama

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/maputil"
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/provider"
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

func (Adapter) Check(ctx context.Context, engine *iapiserver.EngineInstance) (*iapiserver.EngineHealthCheckResult, error) {
	baseURL, err := nativeAPIBaseURL(engine)
	if err != nil {
		return nil, err
	}
	if err := provider.Probe(ctx, engine, http.MethodGet, "/api/tags", provider.WithBaseURL(baseURL)); err != nil {
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
	response, err := provider.Invoke(ctx, engine, http.MethodPost, e.path, payload)
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
