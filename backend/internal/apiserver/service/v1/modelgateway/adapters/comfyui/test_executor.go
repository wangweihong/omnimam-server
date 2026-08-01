package comfyui

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/deepcopy"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/maputil"
	"github.com/wangweihong/gotoolbox/pkg/sliceutil"
	"github.com/wangweihong/gotoolbox/pkg/typeutil"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/provider"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type TestExecutor struct{ store store.Factory }

// NewTestExecutor 构造执行 ComfyUI 测试运行 submit/poll/collect 步骤的 Worker executor。
func NewTestExecutor(factory store.Factory) *TestExecutor {
	return &TestExecutor{store: factory}
}

func (e *TestExecutor) Submit(ctx context.Context, testRunID string) (map[string]any, error) {
	run, engine, err := e.load(ctx, testRunID)
	if err != nil {
		return nil, err
	}
	if run.ExternalJobID != nil && *run.ExternalJobID != "" {
		return map[string]any{"prompt_id": *run.ExternalJobID, "external_job_id": *run.ExternalJobID}, nil
	}
	workflow := deepcopy.AnyMapClone(run.WorkflowSnapshot)
	for _, parameter := range run.Parameters {
		node := typeutil.As[map[string]any](workflow[parameter.NodeID])
		inputs := typeutil.As[map[string]any](node["inputs"])
		if inputs == nil {
			return nil, errors.NewStatus(code.ErrAIAppComfyUITestParameterInvalid, "parameter node is missing")
		}
		inputs[parameter.InputName] = parameter.Value
	}
	result, err := provider.Invoke(ctx, engine, http.MethodPost, "/prompt", map[string]any{"prompt": workflow, "client_id": run.ID}, provider.WithAPIKeyHeader("X-API-Key"))
	if err != nil {
		return nil, err
	}
	promptID := maputil.FirstString(result, "prompt_id")
	if promptID == "" {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "ComfyUI response does not contain prompt_id")
	}
	run.ExternalJobID = &promptID
	_, err = e.store.ApplicationPlatforms().SetComfyUIWorkflowTestRunExternalJob(ctx, run.ID, promptID)
	return map[string]any{"prompt_id": promptID, "external_job_id": promptID}, err
}

func (e *TestExecutor) Poll(ctx context.Context, testRunID string) (map[string]any, error) {
	run, engine, err := e.load(ctx, testRunID)
	if err != nil {
		return nil, err
	}
	if run.ExternalJobID == nil {
		return nil, errors.NewStatus(code.ErrAIAppComfyUITestRunStateBlocked, "prompt id is missing")
	}
	promptID := *run.ExternalJobID
	history, err := provider.Invoke(ctx, engine, http.MethodGet, "/history/"+url.PathEscape(promptID), nil, provider.WithAPIKeyHeader("X-API-Key"))
	if err != nil {
		return nil, err
	}
	entry := typeutil.As[map[string]any](history[promptID])
	if len(entry) == 0 {
		queuePosition := comfyQueuePosition(ctx, engine, promptID)
		return map[string]any{"in_progress": true, "callback_after_seconds": 2, "prompt_id": promptID, "provider_state": "queued", "queue_position": queuePosition}, nil
	}
	status := typeutil.As[map[string]any](entry["status"])
	if completed, _ := status["completed"].(bool); !completed {
		return map[string]any{"in_progress": true, "callback_after_seconds": 2, "prompt_id": promptID, "provider_state": "running"}, nil
	}
	return map[string]any{"prompt_id": promptID, "provider_state": "completed"}, nil
}

func (e *TestExecutor) Collect(ctx context.Context, testRunID string) (map[string]any, error) {
	run, engine, err := e.load(ctx, testRunID)
	if err != nil {
		return nil, err
	}
	if run.ExternalJobID == nil {
		return nil, errors.NewStatus(code.ErrAIAppComfyUITestRunStateBlocked, "prompt id is missing")
	}
	history, err := provider.Invoke(ctx, engine, http.MethodGet, "/history/"+url.PathEscape(*run.ExternalJobID), nil, provider.WithAPIKeyHeader("X-API-Key"))
	if err != nil {
		return nil, err
	}
	entry := typeutil.As[map[string]any](history[*run.ExternalJobID])
	outputs := CollectTestOutputs(typeutil.As[map[string]any](entry["outputs"]), run.OutputSelections)
	_, err = e.store.ApplicationPlatforms().SetComfyUIWorkflowTestRunOutputs(ctx, run.ID, outputs)
	return map[string]any{"prompt_id": *run.ExternalJobID, "output_count": len(outputs)}, err
}

func (e *TestExecutor) load(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflowTestRun, *iapiserver.EngineInstance, error) {
	run, err := e.store.ApplicationPlatforms().GetComfyUIWorkflowTestRun(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	engine, err := e.store.ApplicationPlatforms().GetEngineInstance(ctx, run.EngineInstanceID)
	if err != nil {
		return nil, nil, err
	}
	return run, engine, nil
}
func comfyQueuePosition(ctx context.Context, engine *iapiserver.EngineInstance, promptID string) *int {
	queue, err := provider.Invoke(ctx, engine, http.MethodGet, "/queue", nil, provider.WithAPIKeyHeader("X-API-Key"))
	if err != nil {
		return nil
	}
	position := 0
	for _, key := range []string{"queue_running", "queue_pending"} {
		for _, raw := range sliceutil.ToInterfaceSlice(queue[key]) {
			item := sliceutil.ToInterfaceSlice(raw)
			if len(item) > 1 && fmt.Sprint(item[1]) == promptID {
				value := position
				return &value
			}
			position++
		}
	}
	return nil
}

// CollectTestOutputs 将选中的 ComfyUI 历史输出转换为持久化测试输出快照。
func CollectTestOutputs(outputs map[string]any, selections []iapiserver.ComfyUIWorkflowTestOutputSelection) []iapiserver.ComfyUIWorkflowTestOutput {
	result := []iapiserver.ComfyUIWorkflowTestOutput{}
	selectedNodes := map[string]bool{}
	for _, selection := range selections {
		selectedNodes[selection.NodeID] = true
	}
	for nodeID, raw := range outputs {
		// Empty selections only occur on legacy runs created before output snapshots existed.
		if len(selectedNodes) > 0 && !selectedNodes[nodeID] {
			continue
		}
		output := typeutil.As[map[string]any](raw)
		for _, image := range sliceutil.ToInterfaceSlice(output["images"]) {
			item := typeutil.As[map[string]any](image)
			filename := typeutil.As[string](item["filename"])
			if filename == "" {
				continue
			}
			subfolder := typeutil.As[string](item["subfolder"])
			storageType := typeutil.As[string](item["type"])
			mimeType := mime.TypeByExtension(filepath.Ext(filename))
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}
			result = append(result, iapiserver.ComfyUIWorkflowTestOutput{ID: uuid.NewString(), Kind: "image", NodeID: nodeID, Filename: &filename, Subfolder: &subfolder, StorageType: &storageType, MimeType: &mimeType})
		}
		for _, textValue := range sliceutil.ToInterfaceSlice(output["text"]) {
			text := fmt.Sprint(textValue)
			result = append(result, iapiserver.ComfyUIWorkflowTestOutput{ID: uuid.NewString(), Kind: "text", NodeID: nodeID, Text: &text})
		}
	}
	return result
}
