package applicationplatform

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/httpcli"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/taskname"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

const (
	comfySubmitFunction  = "comfyui.submit"
	comfyPollFunction    = "comfyui.poll"
	comfyCollectFunction = "comfyui.collect_preview"
)

func (s *applicationPlatformService) ListComfyUIWorkflowTestRuns(ctx context.Context, req *iapiserver.ComfyUIWorkflowTestRunListRequest) (*iapiserver.ComfyUIWorkflowTestRunListResponse, error) {
	workflow, _, err := s.visibleComfyUIWorkflow(ctx, req.WorkflowID, "test")
	if err != nil {
		return nil, err
	}
	req.OwnerUserID = workflow.OwnerUserID
	items, total, err := s.Store.ApplicationPlatforms().ListComfyUIWorkflowTestRuns(ctx, req)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		_, _ = s.projectComfyTestRun(ctx, item)
		if !req.Detail {
			item.Parameters = nil
			item.Steps = nil
			item.Outputs = nil
		}
	}
	return &iapiserver.ComfyUIWorkflowTestRunListResponse{Total: total, Items: items}, nil
}

func (s *applicationPlatformService) CreateComfyUIWorkflowTestRun(ctx context.Context, workflowID string, req *iapiserver.ComfyUIWorkflowTestRunCreateRequest) (*iapiserver.ComfyUIWorkflowTestRun, error) {
	workflow, p, err := s.visibleComfyUIWorkflow(ctx, workflowID, "test")
	if err != nil {
		return nil, err
	}
	if workflow.APIConversionStatus != iapiserver.ComfyUIAPIConversionReady || len(workflow.APIWorkflow) == 0 {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIAPINotReady, "API workflow is not ready")
	}
	if existing, findErr := s.Store.ApplicationPlatforms().GetComfyUIWorkflowTestRunByIdempotency(ctx, workflow.OwnerUserID, req.IdempotencyKey); findErr == nil {
		existingParametersDigest, existingDigestErr := canonicalJSONDigest(existing.Parameters)
		requestParametersDigest, requestDigestErr := canonicalJSONDigest(req.Parameters)
		if existingDigestErr != nil || requestDigestErr != nil || existing.WorkflowID != workflowID || existing.EngineInstanceID != req.EngineInstanceID || existingParametersDigest != requestParametersDigest {
			return nil, errors.NewStatus(code.ErrAIAppComfyUITestRunStateBlocked, "idempotency key payload differs")
		}
		return s.projectComfyTestRun(ctx, existing)
	} else if !stderrors.Is(findErr, gorm.ErrRecordNotFound) {
		return nil, findErr
	}
	engine, catalog, err := s.usableComfyUIObjectInfo(ctx, req.EngineInstanceID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAIAppComfyUITestEngineUnavailable, err.Error())
	}
	parsed, err := parseComfyUIWorkflow(workflow.APIWorkflow, nil, catalog.ObjectInfo)
	if err != nil || parsed.status == iapiserver.ComfyUIParseUnsupported {
		return nil, errors.NewStatus(code.ErrAIAppComfyUITestIncompatible, "workflow is incompatible with target engine")
	}
	workflow.InputCandidates = parsed.inputs
	if err := validateTestParameters(workflow, req.Parameters); err != nil {
		return nil, errors.NewStatus(code.ErrAIAppComfyUITestParameterInvalid, err.Error())
	}
	validation := &iapiserver.ComfyUIWorkflowValidation{WorkflowID: workflow.ID, OwnerUserID: workflow.OwnerUserID, RequestedByUserID: p.UserID, EngineInstanceID: engine.ID, Status: iapiserver.ComfyUIValidationCompatible, ComfyUIVersion: catalog.ComfyUIVersion, NodeSummary: map[string]any{"total": parsed.summary.TotalNodes}, DependencySummary: map[string]any{"total": len(parsed.dependencies)}, Errors: []iapiserver.ComfyUIWorkflowDiagnostic{}, Warnings: []iapiserver.ComfyUIWorkflowDiagnostic{}}
	validation.ID = uuid.NewString()
	validation.Name = "Test run compatibility"
	validation.ValidatedAt = imachinery.Now()
	validation, err = s.Store.ApplicationPlatforms().AddComfyUIWorkflowValidation(ctx, validation)
	if err != nil {
		return nil, err
	}
	engineSnapshot := iapiserver.ComfyUIWorkflowTestEngineSnapshot{ID: engine.ID, Name: engine.Name}
	if engine.Region != "" {
		engineSnapshot.Region = &engine.Region
	}
	run := &iapiserver.ComfyUIWorkflowTestRun{WorkflowID: workflow.ID, OwnerUserID: workflow.OwnerUserID, RequestedByUserID: p.UserID, EngineInstanceID: engine.ID, EngineInstanceSnapshot: engineSnapshot, WorkflowValidationID: validation.ID, IdempotencyKey: req.IdempotencyKey, TaskCreationStatus: iapiserver.TaskCreationPending, WorkflowSnapshot: cloneMap(workflow.APIWorkflow), Parameters: req.Parameters, Status: iapiserver.TaskGroupStatusPending, Progress: 0, Steps: defaultTestSteps(), Outputs: []iapiserver.ComfyUIWorkflowTestOutput{}}
	run.ID = uuid.NewString()
	run.Name = "ComfyUI workflow test"
	run, err = s.Store.ApplicationPlatforms().AddComfyUIWorkflowTestRun(ctx, run)
	if err != nil {
		return nil, err
	}
	if s.Tasks == nil {
		return s.failComfyTestRun(ctx, run, fmt.Errorf("task center is unavailable"))
	}
	args := map[string]any{"test_run_id": run.ID}
	dag, err := s.Tasks.CreateDAGTaskGroup(ctx, &iapiserver.DAGTaskGroupCreateRequest{Name: "ComfyUI workflow test", SystemName: iapiserver.SystemNameSpec{Key: taskname.ComfyUIWorkflowTest}, Description: workflow.Name, Nodes: []iapiserver.DAGNode{{Key: "submit", Task: iapiserver.AtomicTaskTemplate{Key: "submit", Name: "Submit prompt", SystemName: iapiserver.SystemNameSpec{Key: taskname.ComfyUISubmit}, FunctionRef: comfySubmitFunction, Arguments: args}}, {Key: "poll", Task: iapiserver.AtomicTaskTemplate{Key: "poll", Name: "Poll queue and history", SystemName: iapiserver.SystemNameSpec{Key: taskname.ComfyUIPoll}, FunctionRef: comfyPollFunction, Arguments: args}}, {Key: "collect_preview", Task: iapiserver.AtomicTaskTemplate{Key: "collect_preview", Name: "Collect temporary preview", SystemName: iapiserver.SystemNameSpec{Key: taskname.ComfyUICollectPreview}, FunctionRef: comfyCollectFunction, Arguments: args}}}, Edges: []iapiserver.DAGEdge{{FromNode: "submit", ToNode: "poll"}, {FromNode: "poll", ToNode: "collect_preview"}}, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace, IdempotencyScope: "comfyui-workflow-test", IdempotencyKey: req.IdempotencyKey, CreatedBy: p.UserID})
	if err != nil {
		return s.failComfyTestRun(ctx, run, err)
	}
	run.DAGTaskGroupID = &dag.ID
	run.TaskCreationStatus = iapiserver.TaskCreationCreated
	run.Status = dag.Status
	run.Progress = int(dag.Progress * 100)
	return s.Store.ApplicationPlatforms().UpdateComfyUIWorkflowTestRun(ctx, run)
}

func (s *applicationPlatformService) GetComfyUIWorkflowTestRun(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflowTestRun, error) {
	run, err := s.visibleComfyTestRun(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.projectComfyTestRun(ctx, run)
}
func (s *applicationPlatformService) CancelComfyUIWorkflowTestRun(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflowTestRun, error) {
	run, err := s.visibleComfyTestRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if run.DAGTaskGroupID == nil {
		return nil, errors.NewStatus(code.ErrAIAppComfyUITestRunStateBlocked, "DAG was not created")
	}
	if _, err = s.Tasks.CancelDAGTaskGroup(ctx, *run.DAGTaskGroupID); err != nil {
		return nil, err
	}
	if run.ExternalJobID != nil {
		if engine, engineErr := s.Store.ApplicationPlatforms().GetEngineInstance(ctx, run.EngineInstanceID); engineErr == nil {
			_, _ = invokeProvider(context.WithoutCancel(ctx), engine, http.MethodPost, "/queue", map[string]any{"delete": []string{*run.ExternalJobID}})
		}
	}
	return s.projectComfyTestRun(ctx, run)
}
func (s *applicationPlatformService) GetComfyUIWorkflowTestOutputContent(ctx context.Context, id, outputID string) ([]byte, string, error) {
	run, err := s.visibleComfyTestRun(ctx, id)
	if err != nil {
		return nil, "", err
	}
	var output *iapiserver.ComfyUIWorkflowTestOutput
	for index := range run.Outputs {
		if run.Outputs[index].ID == outputID {
			output = &run.Outputs[index]
			break
		}
	}
	if output == nil || output.Kind != "image" || output.Filename == nil {
		return nil, "", errors.NewStatus(code.ErrAIAppComfyUITestPreviewUnavailable, "preview output not found")
	}
	engine, err := s.Store.ApplicationPlatforms().GetEngineInstance(ctx, run.EngineInstanceID)
	if err != nil {
		return nil, "", err
	}
	query := url.Values{"filename": {*output.Filename}}
	if output.Subfolder != nil && *output.Subfolder != "" {
		query.Set("subfolder", *output.Subfolder)
	}
	if output.StorageType != nil && *output.StorageType != "" {
		query.Set("type", *output.StorageType)
	}
	endpoint := strings.TrimRight(engine.BaseURL, "/") + "/view?" + query.Encode()
	builder := httpcli.NewHttpRequestBuilder().WithEndpoint(endpoint).WithMethod(http.MethodGet).AddHeaderParam("Accept", "image/*")
	if err := applyProviderAuthentication(builder, engine, http.MethodGet, "/view", nil); err != nil {
		return nil, "", err
	}
	timeout := time.Duration(engine.RequestTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	response, err := builder.Build().InvokeWithContext(ctx, httpcli.TimeoutCallOption(timeout))
	if err != nil || response.GetStatusCode() < 200 || response.GetStatusCode() >= 300 {
		return nil, "", errors.NewStatus(code.ErrAIAppComfyUITestPreviewUnavailable, "preview could not be read")
	}
	contentType := "application/octet-stream"
	if output.MimeType != nil {
		contentType = *output.MimeType
	}
	return []byte(response.GetBody()), contentType, nil
}
func (s *applicationPlatformService) visibleComfyTestRun(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflowTestRun, error) {
	p, err := s.principal(ctx, false)
	if err != nil {
		return nil, err
	}
	run, err := s.Store.ApplicationPlatforms().GetComfyUIWorkflowTestRun(ctx, id)
	if err != nil || (!p.Admin && run.OwnerUserID != p.UserID) {
		return nil, errors.NewStatus(code.ErrAIAppComfyUITestRunNotFound, "test run not found")
	}
	return run, nil
}
func (s *applicationPlatformService) projectComfyTestRun(ctx context.Context, run *iapiserver.ComfyUIWorkflowTestRun) (*iapiserver.ComfyUIWorkflowTestRun, error) {
	for index := range run.Outputs {
		if run.Outputs[index].Kind == "image" {
			value := "/api/v1/comfyui-workflow-test-runs/" + run.ID + "/outputs/" + run.Outputs[index].ID + "/content"
			run.Outputs[index].ContentURL = &value
		}
	}
	if run.DAGTaskGroupID == nil {
		return run, nil
	}
	dag, err := s.Tasks.GetDAGTaskGroup(ctx, *run.DAGTaskGroupID)
	if err != nil {
		return run, nil
	}
	tasksResp, _ := s.Tasks.ListDAGTaskGroupTasks(ctx, dag.ID, &iapiserver.AtomicTaskListRequest{BasicQueryParam: imachinery.BasicQueryParam{PagingParams: imachinery.PagingParams{PageSize: 100}}})
	run.Status = dag.Status
	run.Progress = int(dag.Progress * 100)
	for i := range run.Steps {
		for _, task := range tasksResp.Items {
			if task.ChildKey == run.Steps[i].Key {
				run.Steps[i].AtomicTaskID = &task.ID
				run.Steps[i].Status = task.Status
				run.Steps[i].Progress = int(task.Progress * 100)
				if run.Steps[i].Status == iapiserver.AtomicTaskStatusRunning {
					run.CurrentStep = &run.Steps[i].Key
				}
			}
		}
	}
	_, _ = s.Store.ApplicationPlatforms().UpdateComfyUIWorkflowTestRun(ctx, run)
	return run, nil
}
func (s *applicationPlatformService) failComfyTestRun(ctx context.Context, run *iapiserver.ComfyUIWorkflowTestRun, cause error) (*iapiserver.ComfyUIWorkflowTestRun, error) {
	message := cause.Error()
	run.TaskCreationStatus = iapiserver.TaskCreationFailed
	run.TaskCreationFailure = &message
	run.Status = iapiserver.TaskGroupStatusFailed
	updated, _ := s.Store.ApplicationPlatforms().UpdateComfyUIWorkflowTestRun(ctx, run)
	return updated, cause
}
func defaultTestSteps() []iapiserver.ComfyUIWorkflowTestStep {
	return []iapiserver.ComfyUIWorkflowTestStep{{Key: "submit", Label: "提交", Status: "PENDING"}, {Key: "poll", Label: "轮询", Status: "PENDING"}, {Key: "collect_preview", Label: "收集预览", Status: "PENDING"}}
}
func validateTestParameters(workflow *iapiserver.ComfyUIWorkflow, parameters []iapiserver.ComfyUIWorkflowTestParameter) error {
	allowed := map[string]iapiserver.ComfyUIWorkflowInputCandidate{}
	for _, item := range workflow.InputCandidates {
		if item.Classification == "exposable" || item.Classification == "fixed_only" {
			allowed[item.NodeID+"\x00"+item.InputName] = item
		}
	}
	seen := map[string]bool{}
	for _, item := range parameters {
		key := item.NodeID + "\x00" + item.InputName
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("parameter %s/%s cannot be overridden", item.NodeID, item.InputName)
		}
		if seen[key] {
			return fmt.Errorf("parameter %s/%s is duplicated", item.NodeID, item.InputName)
		}
		seen[key] = true
	}
	return nil
}
