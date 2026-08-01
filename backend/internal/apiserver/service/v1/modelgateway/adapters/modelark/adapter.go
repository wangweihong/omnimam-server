package modelark

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/maputil"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/provider"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

const AdapterID = "byteplus_modelark"

var ExecutorIDs = []string{"byteplus_text_to_video", "byteplus_image_to_video", "byteplus_reference_to_video"}

// Adapter 对 BytePlus ModelArk API 执行协议级健康探测。
type Adapter struct{}

func (Adapter) ID() string { return AdapterID }

func (Adapter) Check(ctx context.Context, engine *iapiserver.EngineInstance) (*iapiserver.EngineHealthCheckResult, error) {
	if err := provider.Probe(ctx, engine, http.MethodGet, "/api/v3/contents/generations/tasks", provider.WithModelArkSigning()); err != nil {
		return nil, err
	}
	return &iapiserver.EngineHealthCheckResult{EngineInstanceID: engine.ID, HealthStatus: iapiserver.EngineHealthOnline}, nil
}

// Executor 将运行快照转换为指定的 ModelArk 视频生成操作。
type Executor struct{ id string }

func NewExecutor(id string) *Executor { return &Executor{id: id} }

func (e *Executor) ID() string { return e.id }

func (e *Executor) Execute(ctx context.Context, engine *iapiserver.EngineInstance, run *iapiserver.ApplicationRun) (map[string]any, error) {
	payload := maputil.Clone(run.InputSnapshot)
	if _, ok := payload["model"]; !ok {
		payload["model"] = providerModelID(run, payload)
	}
	if prompt, ok := payload["prompt"].(string); ok {
		payload["content"] = []map[string]any{{"type": "text", "text": prompt}}
		delete(payload, "prompt")
	}
	requestPath := "/api/v3/contents/generations/tasks"
	submitted, err := provider.Invoke(ctx, engine, http.MethodPost, requestPath, payload, provider.WithModelArkSigning())
	if err != nil {
		return nil, err
	}
	taskID := maputil.FirstString(submitted, "id", "task_id")
	if taskID == "" {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "ModelArk response does not contain task id")
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_, cancelErr := provider.Invoke(context.WithoutCancel(ctx), engine, http.MethodDelete, requestPath+"/"+url.PathEscape(taskID), nil, provider.WithModelArkSigning())
			return nil, stderrors.Join(ctx.Err(), cancelErr)
		case <-ticker.C:
			result, pollErr := provider.Invoke(ctx, engine, http.MethodGet, requestPath+"/"+url.PathEscape(taskID), nil, provider.WithModelArkSigning())
			if pollErr != nil {
				return nil, pollErr
			}
			switch strings.ToLower(maputil.FirstString(result, "status", "state")) {
			case "", "queued", "pending", "running", "processing":
				continue
			case "succeeded", "success", "completed":
				return map[string]any{"values": result, "artifacts": artifacts(result)}, nil
			default:
				return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "ModelArk rejected the generation task")
			}
		}
	}
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

func artifacts(result map[string]any) []map[string]any {
	items := []map[string]any{}
	for _, field := range []struct{ key, mediaType string }{{"video_url", "video"}, {"last_frame_url", "image"}, {"image_url", "image"}} {
		if ref := maputil.FirstString(result, field.key); ref != "" {
			items = append(items, map[string]any{"output_key": field.key, "media_type": field.mediaType, "content_ref": ref})
		}
	}
	if content, _ := result["content"].(map[string]any); content != nil {
		items = append(items, artifacts(content)...)
	}
	return items
}
