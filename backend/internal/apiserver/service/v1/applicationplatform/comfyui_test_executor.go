package applicationplatform

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type ComfyUITestExecutor struct{ store store.Factory }

func NewComfyUITestExecutor(factory store.Factory) *ComfyUITestExecutor {
	return &ComfyUITestExecutor{store: factory}
}

func (e *ComfyUITestExecutor) Submit(ctx context.Context, testRunID string) (map[string]any, error) {
	run, engine, err := e.load(ctx, testRunID)
	if err != nil {
		return nil, err
	}
	if run.ExternalJobID != nil && *run.ExternalJobID != "" {
		return map[string]any{"prompt_id": *run.ExternalJobID, "external_job_id": *run.ExternalJobID}, nil
	}
	workflow := deepCopyMap(run.WorkflowSnapshot)
	for _, parameter := range run.Parameters {
		node := mapValue(workflow[parameter.NodeID])
		inputs := mapValue(node["inputs"])
		if inputs == nil {
			return nil, errors.NewStatus(code.ErrAIAppComfyUITestParameterInvalid, "parameter node is missing")
		}
		inputs[parameter.InputName] = parameter.Value
	}
	result, err := invokeProvider(ctx, engine, http.MethodPost, "/prompt", map[string]any{"prompt": workflow, "client_id": run.ID})
	if err != nil {
		return nil, err
	}
	promptID := firstString(result, "prompt_id")
	if promptID == "" {
		return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, "ComfyUI response does not contain prompt_id")
	}
	run.ExternalJobID = &promptID
	setRunStep(run, "submit", iapiserver.AtomicTaskStatusSuccess, 100, &promptID, nil, nil)
	_, err = e.store.ApplicationPlatforms().UpdateComfyUIWorkflowTestRun(ctx, run)
	return map[string]any{"prompt_id": promptID, "external_job_id": promptID}, err
}

func (e *ComfyUITestExecutor) Poll(ctx context.Context, testRunID string) (map[string]any, error) {
	run, engine, err := e.load(ctx, testRunID)
	if err != nil {
		return nil, err
	}
	if run.ExternalJobID == nil {
		return nil, errors.NewStatus(code.ErrAIAppComfyUITestRunStateBlocked, "prompt id is missing")
	}
	promptID := *run.ExternalJobID
	history, err := invokeProvider(ctx, engine, http.MethodGet, "/history/"+url.PathEscape(promptID), nil)
	if err != nil {
		return nil, err
	}
	entry := mapValue(history[promptID])
	if len(entry) == 0 {
		queuePosition := comfyQueuePosition(ctx, engine, promptID)
		state := "queued"
		setRunStep(run, "poll", iapiserver.AtomicTaskStatusRunning, 25, &promptID, &state, queuePosition)
		_, _ = e.store.ApplicationPlatforms().UpdateComfyUIWorkflowTestRun(ctx, run)
		return map[string]any{"in_progress": true, "callback_after_seconds": 2, "prompt_id": promptID, "queue_position": queuePosition}, nil
	}
	status := mapValue(entry["status"])
	if completed, _ := status["completed"].(bool); !completed {
		state := "running"
		setRunStep(run, "poll", iapiserver.AtomicTaskStatusRunning, 60, &promptID, &state, nil)
		_, _ = e.store.ApplicationPlatforms().UpdateComfyUIWorkflowTestRun(ctx, run)
		return map[string]any{"in_progress": true, "callback_after_seconds": 2, "prompt_id": promptID}, nil
	}
	state := "completed"
	setRunStep(run, "poll", iapiserver.AtomicTaskStatusSuccess, 100, &promptID, &state, nil)
	_, err = e.store.ApplicationPlatforms().UpdateComfyUIWorkflowTestRun(ctx, run)
	return map[string]any{"prompt_id": promptID, "history": entry}, err
}

func (e *ComfyUITestExecutor) Collect(ctx context.Context, testRunID string) (map[string]any, error) {
	run, engine, err := e.load(ctx, testRunID)
	if err != nil {
		return nil, err
	}
	if run.ExternalJobID == nil {
		return nil, errors.NewStatus(code.ErrAIAppComfyUITestRunStateBlocked, "prompt id is missing")
	}
	history, err := invokeProvider(ctx, engine, http.MethodGet, "/history/"+url.PathEscape(*run.ExternalJobID), nil)
	if err != nil {
		return nil, err
	}
	entry := mapValue(history[*run.ExternalJobID])
	outputs := collectTestOutputs(mapValue(entry["outputs"]), run.OutputSelections)
	run.Outputs = outputs
	run.Status = iapiserver.TaskGroupStatusSuccess
	run.Progress = 100
	setRunStep(run, "collect_preview", iapiserver.AtomicTaskStatusSuccess, 100, run.ExternalJobID, nil, nil)
	run.CurrentStep = nil
	_, err = e.store.ApplicationPlatforms().UpdateComfyUIWorkflowTestRun(ctx, run)
	return map[string]any{"prompt_id": *run.ExternalJobID, "output_count": len(outputs)}, err
}

func (e *ComfyUITestExecutor) load(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflowTestRun, *iapiserver.EngineInstance, error) {
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
func deepCopyMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		if nested, ok := item.(map[string]any); ok {
			result[key] = deepCopyMap(nested)
		} else {
			result[key] = item
		}
	}
	return result
}
func setRunStep(run *iapiserver.ComfyUIWorkflowTestRun, key, status string, progress int, jobID, state *string, queue *int) {
	for index := range run.Steps {
		if run.Steps[index].Key == key {
			run.Steps[index].Status = status
			run.Steps[index].Progress = progress
			run.Steps[index].ExternalJobID = jobID
			run.Steps[index].ProviderState = state
			run.Steps[index].QueuePosition = queue
			run.CurrentStep = &run.Steps[index].Key
			return
		}
	}
}
func comfyQueuePosition(ctx context.Context, engine *iapiserver.EngineInstance, promptID string) *int {
	queue, err := invokeProvider(ctx, engine, http.MethodGet, "/queue", nil)
	if err != nil {
		return nil
	}
	position := 0
	for _, key := range []string{"queue_running", "queue_pending"} {
		for _, raw := range anySlice(queue[key]) {
			item := anySlice(raw)
			if len(item) > 1 && fmt.Sprint(item[1]) == promptID {
				value := position
				return &value
			}
			position++
		}
	}
	return nil
}
func collectTestOutputs(outputs map[string]any, selections []iapiserver.ComfyUIWorkflowTestOutputSelection) []iapiserver.ComfyUIWorkflowTestOutput {
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
		output := mapValue(raw)
		for _, image := range anySlice(output["images"]) {
			item := mapValue(image)
			filename := stringValue(item["filename"])
			if filename == "" {
				continue
			}
			subfolder := stringValue(item["subfolder"])
			storageType := stringValue(item["type"])
			mimeType := mime.TypeByExtension(filepath.Ext(filename))
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}
			result = append(result, iapiserver.ComfyUIWorkflowTestOutput{ID: uuid.NewString(), Kind: "image", NodeID: nodeID, Filename: &filename, Subfolder: &subfolder, StorageType: &storageType, MimeType: &mimeType})
		}
		for _, textValue := range anySlice(output["text"]) {
			text := fmt.Sprint(textValue)
			result = append(result, iapiserver.ComfyUIWorkflowTestOutput{ID: uuid.NewString(), Kind: "text", NodeID: nodeID, Text: &text})
		}
	}
	return result
}
