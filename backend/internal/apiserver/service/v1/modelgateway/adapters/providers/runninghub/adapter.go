package runninghub

import (
	"context"
	stderrors "errors"
	"net/http"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/maputil"
	"github.com/wangweihong/gotoolbox/pkg/typeutil"
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/transports/httpjson"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

const (
	AdapterID  = "runninghub_workflow"
	ExecutorID = "runninghub_workflow_execute"
)

// Adapter 对 RunningHub 官方服务地址执行可达性探测。
type Adapter struct{}

func (Adapter) ID() string { return AdapterID }

func (Adapter) Check(ctx context.Context, engine *iapiserver.EngineInstance) (*iapiserver.EngineHealthCheckResult, error) {
	if err := httpjson.Probe(ctx, engine, http.MethodGet, "/", httpjson.WithAPIKeyInPayload()); err != nil {
		return nil, err
	}
	return &iapiserver.EngineHealthCheckResult{EngineInstanceID: engine.ID, HealthStatus: iapiserver.EngineHealthOnline}, nil
}

// Executor 按 RunningHub 官方 create/outputs/cancel 协议执行固定工作流快照。
type Executor struct{}

func (Executor) ID() string { return ExecutorID }

// CancelExternalJob 取消 checkpoint 标识的 RunningHub 作业；没有外部 ID 时安全忽略。
func (Executor) CancelExternalJob(ctx context.Context, engine *iapiserver.EngineInstance, _ *iapiserver.ApplicationRun, checkpoint map[string]any) error {
	taskID := maputil.FirstString(checkpoint, "external_job_id", "taskId", "task_id")
	if taskID == "" {
		return nil
	}
	apiKey, err := addAPIKey(engine, map[string]any{})
	if err != nil {
		return err
	}
	return cancelTask(ctx, engine, apiKey, taskID)
}

// ExecuteCheckpoint 提交一次外部作业，并通过 runtime output 在延迟回调和自动重试间恢复 taskId。
func (Executor) ExecuteCheckpoint(ctx context.Context, engine *iapiserver.EngineInstance, run *iapiserver.ApplicationRun, checkpoint map[string]any) (map[string]any, error) {
	taskID := maputil.FirstString(checkpoint, "external_job_id", "taskId", "task_id")
	if taskID == "" {
		payload, _, err := submissionPayload(engine, run)
		if err != nil {
			return nil, err
		}
		submitted, err := httpjson.Invoke(ctx, engine, http.MethodPost, "/task/openapi/create", payload, httpjson.WithAPIKeyInPayload())
		if err != nil {
			return nil, err
		}
		taskID = taskIDFrom(submitted)
		if taskID == "" {
			return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "RunningHub response does not contain taskId")
		}
		return runningCheckpoint(taskID, "submitted"), nil
	}

	apiKey, err := addAPIKey(engine, map[string]any{})
	if err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, stderrors.Join(ctx.Err(), cancelTask(ctx, engine, apiKey, taskID))
	}
	result, err := httpjson.Invoke(ctx, engine, http.MethodPost, "/task/openapi/outputs", map[string]any{"apiKey": apiKey, "taskId": taskID}, httpjson.WithAPIKeyInPayload())
	if err != nil {
		if ctx.Err() != nil {
			return nil, stderrors.Join(ctx.Err(), cancelTask(ctx, engine, apiKey, taskID))
		}
		return nil, err
	}
	output, completed, err := runningResult(taskID, result)
	if err != nil || completed {
		return output, err
	}
	return runningCheckpoint(taskID, strings.ToLower(maputil.FirstString(result, "status", "state"))), nil
}

func (Executor) Execute(ctx context.Context, engine *iapiserver.EngineInstance, run *iapiserver.ApplicationRun) (map[string]any, error) {
	payload, apiKey, err := submissionPayload(engine, run)
	if err != nil {
		return nil, err
	}
	submitted, err := httpjson.Invoke(ctx, engine, http.MethodPost, "/task/openapi/create", payload, httpjson.WithAPIKeyInPayload())
	if err != nil {
		return nil, err
	}
	taskID := taskIDFrom(submitted)
	if taskID == "" {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "RunningHub response does not contain taskId")
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, stderrors.Join(ctx.Err(), cancelTask(ctx, engine, apiKey, taskID))
		case <-ticker.C:
			result, pollErr := httpjson.Invoke(ctx, engine, http.MethodPost, "/task/openapi/outputs", map[string]any{"apiKey": apiKey, "taskId": taskID}, httpjson.WithAPIKeyInPayload())
			if pollErr != nil {
				if ctx.Err() != nil {
					return nil, stderrors.Join(ctx.Err(), cancelTask(ctx, engine, apiKey, taskID))
				}
				return nil, pollErr
			}
			output, completed, resultErr := runningResult(taskID, result)
			if resultErr != nil || completed {
				return output, resultErr
			}
		}
	}
}

func submissionPayload(engine *iapiserver.EngineInstance, run *iapiserver.ApplicationRun) (map[string]any, string, error) {
	if run == nil {
		return nil, "", errors.NewStatus(code.ErrAIAppApplicationInputInvalid, "RunningHub application run is required")
	}
	payload := maputil.Clone(run.InputSnapshot)
	if payload["workflowId"] == nil {
		payload["workflowId"] = payload["workflow_id"]
	}
	delete(payload, "workflow_id")
	apiKey, err := addAPIKey(engine, payload)
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(typeutil.As[string](payload["workflowId"])) == "" {
		return nil, "", errors.NewStatus(code.ErrAIAppApplicationInputInvalid, "RunningHub workflowId is required")
	}
	return payload, apiKey, nil
}

func runningResult(taskID string, result map[string]any) (map[string]any, bool, error) {
	status := strings.ToLower(maputil.FirstString(result, "status", "state"))
	//nolint:misspell // RunningHub responses may use either US or British spelling.
	if status == "failed" || status == "error" || status == "canceled" || status == "cancelled" {
		return nil, true, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "RunningHub workflow task failed")
	}
	if result["data"] != nil || result["outputs"] != nil || status == "success" || status == "succeeded" || status == "completed" {
		return map[string]any{"values": result, "external_job_id": taskID}, true, nil
	}
	return nil, false, nil
}

func runningCheckpoint(taskID, state string) map[string]any {
	return map[string]any{"external_job_id": taskID, "provider_state": state, "in_progress": true, "callback_after_seconds": 1}
}

func cancelTask(ctx context.Context, engine *iapiserver.EngineInstance, apiKey, taskID string) error {
	cancelCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err := httpjson.Invoke(cancelCtx, engine, http.MethodPost, "/task/openapi/cancel", map[string]any{"apiKey": apiKey, "taskId": taskID}, httpjson.WithAPIKeyInPayload())
	return err
}

func addAPIKey(engine *iapiserver.EngineInstance, payload map[string]any) (string, error) {
	apiKey, _ := engine.AuthConfig["api_key"].(string)
	if strings.TrimSpace(apiKey) == "" {
		return "", errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "RunningHub api_key is required")
	}
	payload["apiKey"] = apiKey
	return apiKey, nil
}

func taskIDFrom(response map[string]any) string {
	if id := maputil.FirstString(response, "taskId", "task_id", "id"); id != "" {
		return id
	}
	return maputil.FirstString(typeutil.As[map[string]any](response["data"]), "taskId", "task_id", "id")
}
