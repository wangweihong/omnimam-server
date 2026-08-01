package xai

import (
	"context"
	"net/http"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/protocols/openaicompat"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/transports/httpjson"
)

const (
	AdapterID  = "xai_responses"
	ExecutorID = "xai_responses_create"
)

// Adapter 对 xAI 官方 Responses API 执行独立健康探测。
type Adapter struct{}

func (Adapter) ID() string { return AdapterID }

func (Adapter) Check(ctx context.Context, engine *iapiserver.EngineInstance) (*iapiserver.EngineHealthCheckResult, error) {
	if err := httpjson.Probe(ctx, engine, http.MethodGet, "/models"); err != nil {
		return nil, err
	}
	return &iapiserver.EngineHealthCheckResult{EngineInstanceID: engine.ID, HealthStatus: iapiserver.EngineHealthOnline}, nil
}

// NewExecutor 复用经 xAI 官方确认兼容的 Responses JSON 线协议。
func NewExecutor() modelgateway.OperationExecutor {
	return openaicompat.NewJSONExecutor(ExecutorID, "/responses")
}
