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
	"github.com/wangweihong/gotoolbox/pkg/deepcopy"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/httpcli"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/transports/httpjson"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
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
	projectComfyTestRunSummaries(ctx, s.Tasks, items)
	for _, item := range items {
		if req.Detail {
			_, _ = s.projectComfyTestRun(ctx, item)
		}
		if !req.Detail {
			item.Parameters = nil
			item.OutputSelections = nil
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
		existingOutputsDigest, existingOutputsDigestErr := canonicalJSONDigest(existing.OutputSelections)
		requestOutputsDigest, requestOutputsDigestErr := canonicalJSONDigest(req.Outputs)
		if existingDigestErr != nil || requestDigestErr != nil || existingOutputsDigestErr != nil || requestOutputsDigestErr != nil || existing.WorkflowID != workflowID || existing.EngineInstanceID != req.EngineInstanceID || existingParametersDigest != requestParametersDigest || existingOutputsDigest != requestOutputsDigest {
			return nil, errors.NewStatus(code.ErrAIAppComfyUITestRunStateBlocked, "idempotency key payload differs")
		}
		return s.projectComfyTestRun(ctx, existing)
	} else if !stderrors.Is(findErr, gorm.ErrRecordNotFound) {
		return nil, findErr
	}
	engine, catalog, err := s.Srv.ResolveUsableObjectInfo(ctx, req.EngineInstanceID)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAIAppComfyUITestEngineUnavailable, err.Error())
	}
	parsed, err := parseComfyUIWorkflow(workflow.APIWorkflow, nil, catalog.ObjectInfo)
	if err != nil || parsed.status == iapiserver.ComfyUIParseUnsupported {
		return nil, errors.NewStatus(code.ErrAIAppComfyUITestIncompatible, "workflow is incompatible with target engine")
	}
	workflow.InputCandidates = parsed.inputs
	workflow.OutputCandidates = parsed.outputs
	if err := validateTestParameters(workflow, req.Parameters); err != nil {
		return nil, errors.NewStatus(code.ErrAIAppComfyUITestParameterInvalid, err.Error())
	}
	if err := validateTestOutputs(workflow, req.Outputs); err != nil {
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
	run := &iapiserver.ComfyUIWorkflowTestRun{WorkflowID: workflow.ID, OwnerUserID: workflow.OwnerUserID, RequestedByUserID: p.UserID, EngineInstanceID: engine.ID, EngineInstanceSnapshot: engineSnapshot, WorkflowValidationID: validation.ID, IdempotencyKey: req.IdempotencyKey, TaskCreationStatus: iapiserver.TaskCreationPending, WorkflowSnapshot: deepcopy.AnyMapClone(workflow.APIWorkflow), Parameters: req.Parameters, OutputSelections: req.Outputs, Outputs: []iapiserver.ComfyUIWorkflowTestOutput{}}
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
	bound, err := s.Store.ApplicationPlatforms().BindComfyUIWorkflowTestRunDAG(ctx, run.ID, dag.ID)
	if err != nil {
		return nil, err
	}
	return s.projectComfyTestRun(ctx, bound)
}

func (s *applicationPlatformService) GetComfyUIWorkflowTestRun(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflowTestRun, error) {
	run, err := s.visibleComfyTestRun(ctx, id, "read_test_run")
	if err != nil {
		return nil, err
	}
	return s.projectComfyTestRun(ctx, run)
}
func (s *applicationPlatformService) CancelComfyUIWorkflowTestRun(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflowTestRun, error) {
	run, err := s.visibleComfyTestRun(ctx, id, "cancel_test_run")
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
			_, _ = httpjson.Invoke(context.WithoutCancel(ctx), engine, http.MethodPost, "/queue", map[string]any{"delete": []string{*run.ExternalJobID}})
		}
	}
	return s.projectComfyTestRun(ctx, run)
}
func (s *applicationPlatformService) GetComfyUIWorkflowTestOutputContent(ctx context.Context, id, outputID string) ([]byte, string, error) {
	run, err := s.visibleComfyTestRun(ctx, id, "read_test_output")
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
	if err := httpjson.ApplyAuthentication(builder, engine, http.MethodGet, "/view", nil); err != nil {
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
func (s *applicationPlatformService) visibleComfyTestRun(ctx context.Context, id, action string) (*iapiserver.ComfyUIWorkflowTestRun, error) {
	p, err := s.principal(ctx, false)
	if err != nil {
		return nil, err
	}
	run, err := s.Store.ApplicationPlatforms().GetComfyUIWorkflowTestRun(ctx, id)
	if err != nil || (run.OwnerUserID != p.UserID && (!p.Admin || !p.HasPermission("aiapp.comfyui_workflow.manage_all"))) {
		return nil, errors.NewStatus(code.ErrAIAppComfyUITestRunNotFound, "test run not found")
	}
	workflow := &iapiserver.ComfyUIWorkflow{OwnerUserID: run.OwnerUserID}
	workflow.ID = run.WorkflowID
	if err := s.auditManagedWorkflow(ctx, p, workflow, action); err != nil {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIWorkflowAccessDenied, "managed workflow access could not be audited")
	}
	return run, nil
}
func (s *applicationPlatformService) projectComfyTestRun(ctx context.Context, run *iapiserver.ComfyUIWorkflowTestRun) (*iapiserver.ComfyUIWorkflowTestRun, error) {
	applyComfyTestRunCreationProjection(run)
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
	tasksResp, taskErr := s.Tasks.ListDAGTaskGroupTasks(ctx, dag.ID, &iapiserver.AtomicTaskListRequest{BasicQueryParam: imachinery.BasicQueryParam{PagingParams: imachinery.PagingParams{PageSize: 100}}})
	if taskErr != nil || tasksResp == nil {
		return run, nil
	}
	applyComfyTestRunProjection(run, dag, tasksResp.Items)
	return run, nil
}

func applyComfyTestRunProjection(run *iapiserver.ComfyUIWorkflowTestRun, dag *iapiserver.DAGTaskGroup, tasks []*iapiserver.AtomicTask) {
	run.Status = dag.Status
	run.Progress = dag.Progress
	run.CurrentStep = nil
	run.FailureSummary = nil
	run.Steps = make([]iapiserver.ComfyUIWorkflowTestStep, 0, len(dag.Nodes))

	byKey := make(map[string]*iapiserver.AtomicTask, len(tasks))
	for _, task := range tasks {
		if task != nil {
			key := task.DAGNodeKey
			if key == "" {
				key = task.ChildKey
			}
			byKey[key] = task
		}
	}
	currentPriority := 100
	for _, node := range dag.Nodes {
		step := iapiserver.ComfyUIWorkflowTestStep{Key: node.Key, Label: node.Task.Name, Status: iapiserver.AtomicTaskStatusPending}
		task := byKey[node.Key]
		if task == nil {
			run.Steps = append(run.Steps, step)
			continue
		}
		if task.Name != "" {
			step.Label = task.Name
		}
		step.AtomicTaskID = &task.ID
		step.Status = task.Status
		step.Progress = task.Progress
		if node.Key != "collect_preview" {
			step.ExternalJobID = comfyTestExternalJobID(run.ExternalJobID, task.Output)
		}
		if state, ok := task.Output["provider_state"].(string); ok && state != "" {
			step.ProviderState = &state
		}
		step.QueuePosition = comfyTestQueuePosition(task.Output["queue_position"])
		if priority := comfyTestCurrentStepPriority(task.Status); priority < currentPriority {
			currentPriority = priority
			run.CurrentStep = &step.Key
		}
		if task.Status == iapiserver.AtomicTaskStatusFailed || task.Status == iapiserver.AtomicTaskStatusTimeout {
			if summary := comfyTestTaskErrorSummary(task.LastError); summary != "" {
				step.Error = &summary
				if run.FailureSummary == nil {
					run.FailureSummary = &summary
				}
			}
		}
		run.Steps = append(run.Steps, step)
	}
	if isComfyTestRunTerminal(dag.Status) {
		run.CurrentStep = nil
	}
}

func projectComfyTestRunSummaries(ctx context.Context, tasks taskcenter.TaskCenterSrv, runs []*iapiserver.ComfyUIWorkflowTestRun) {
	ids := make([]string, 0, len(runs))
	for _, run := range runs {
		applyComfyTestRunCreationProjection(run)
		if run.DAGTaskGroupID != nil {
			ids = append(ids, *run.DAGTaskGroupID)
		}
	}
	if tasks == nil || len(ids) == 0 {
		return
	}
	summaries, err := tasks.GetDAGTaskGroupSummaries(ctx, ids)
	if err != nil {
		return
	}
	for _, run := range runs {
		if run.DAGTaskGroupID == nil {
			continue
		}
		if summary := summaries[*run.DAGTaskGroupID]; summary != nil {
			run.Status = summary.Status
			run.Progress = summary.Progress
		}
	}
}

func applyComfyTestRunCreationProjection(run *iapiserver.ComfyUIWorkflowTestRun) {
	run.Status = iapiserver.TaskGroupStatusPending
	run.Progress = 0
	run.CurrentStep = nil
	run.FailureSummary = nil
	if run.TaskCreationStatus == iapiserver.TaskCreationFailed {
		run.Status = iapiserver.TaskGroupStatusFailed
		run.FailureSummary = run.TaskCreationFailure
	}
}

func comfyTestQueuePosition(value any) *int {
	var position int
	switch typed := value.(type) {
	case int:
		position = typed
	case int64:
		position = int(typed)
	case float64:
		position = int(typed)
	case *int:
		if typed == nil {
			return nil
		}
		position = *typed
	default:
		return nil
	}
	if position <= 0 {
		return nil
	}
	return &position
}

func comfyTestExternalJobID(stored *string, output map[string]any) *string {
	if stored != nil && *stored != "" {
		return stored
	}
	for _, key := range []string{"external_job_id", "prompt_id"} {
		if value, ok := output[key].(string); ok && value != "" {
			return &value
		}
	}
	return nil
}

func comfyTestCurrentStepPriority(status string) int {
	switch status {
	case iapiserver.AtomicTaskStatusRunning, iapiserver.AtomicTaskStatusCancelRequested:
		return 0
	case iapiserver.AtomicTaskStatusRetrying:
		return 1
	case iapiserver.AtomicTaskStatusReady:
		return 2
	default:
		return 100
	}
}

func comfyTestTaskErrorSummary(taskError iapiserver.TaskError) string {
	for _, value := range []string{taskError.Message, taskError.Detail, taskError.Code} {
		if summary := strings.TrimSpace(value); summary != "" {
			return summary
		}
	}
	return ""
}

func isComfyTestRunTerminal(status string) bool {
	switch status {
	case iapiserver.TaskGroupStatusSuccess, iapiserver.TaskGroupStatusFailed, iapiserver.TaskGroupStatusTimeout, iapiserver.TaskGroupStatusCanceled:
		return true
	default:
		return false
	}
}
func (s *applicationPlatformService) failComfyTestRun(ctx context.Context, run *iapiserver.ComfyUIWorkflowTestRun, cause error) (*iapiserver.ComfyUIWorkflowTestRun, error) {
	message := cause.Error()
	updated, _ := s.Store.ApplicationPlatforms().FailComfyUIWorkflowTestRunCreation(ctx, run.ID, message)
	if updated != nil {
		applyComfyTestRunCreationProjection(updated)
	}
	return updated, cause
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

func validateTestOutputs(workflow *iapiserver.ComfyUIWorkflow, outputs []iapiserver.ComfyUIWorkflowTestOutputSelection) error {
	if len(outputs) == 0 {
		return fmt.Errorf("at least one output candidate is required")
	}
	allowed := map[string]struct{}{}
	for _, item := range workflow.OutputCandidates {
		if item.Extractable && (item.MediaType == "image" || item.MediaType == "text") {
			allowed[fmt.Sprintf("%s\x00%d", item.NodeID, item.OutputIndex)] = struct{}{}
		}
	}
	seen := map[string]bool{}
	for _, item := range outputs {
		key := fmt.Sprintf("%s\x00%d", item.NodeID, item.OutputIndex)
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("output %s/%d cannot be collected", item.NodeID, item.OutputIndex)
		}
		if seen[key] {
			return fmt.Errorf("output %s/%d is duplicated", item.NodeID, item.OutputIndex)
		}
		seen[key] = true
	}
	return nil
}
