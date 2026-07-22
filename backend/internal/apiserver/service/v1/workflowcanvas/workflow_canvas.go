package workflowcanvas

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gowebpki/jcs"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/sliceutil"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	taskcentersvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
)

type Service interface {
	ListNodeDefinitions(context.Context, *iapiserver.WorkflowNodeDefinitionListRequest) (*iapiserver.WorkflowNodeDefinitionListResponse, error)
	RegisterNodeDefinition(context.Context, *iapiserver.WorkflowNodeDefinitionRegisterRequest) (*iapiserver.WorkflowNodeDefinition, error)
	GetNodeDefinition(context.Context, string, string) (*iapiserver.WorkflowNodeDefinition, error)
	DeprecateNodeDefinition(context.Context, string, string, *iapiserver.WorkflowNodeDefinitionDeprecateRequest) (*iapiserver.WorkflowNodeDefinition, error)
	List(context.Context, *iapiserver.WorkflowCanvasListRequest) (*iapiserver.WorkflowCanvasListResponse, error)
	Create(context.Context, *iapiserver.WorkflowCanvasCreateRequest) (*iapiserver.WorkflowCanvas, error)
	Get(context.Context, string) (*iapiserver.WorkflowCanvas, error)
	Update(context.Context, *iapiserver.WorkflowCanvasUpdateRequest) (*iapiserver.WorkflowCanvas, error)
	Delete(context.Context, string) error
	ValidateDraft(context.Context, string, *iapiserver.WorkflowCanvasValidateRequest) (*iapiserver.WorkflowValidationResult, error)
	Publish(context.Context, string, *iapiserver.WorkflowCanvasPublishRequest) (*iapiserver.CanvasVersion, error)
	ListVersions(context.Context, *iapiserver.CanvasVersionListRequest) (*iapiserver.CanvasVersionListResponse, error)
	GetVersion(context.Context, string) (*iapiserver.CanvasVersion, error)
	ValidateRun(context.Context, string, *iapiserver.WorkflowCanvasRunValidateRequest) (*iapiserver.WorkflowCanvasRunValidationResult, error)
	ListRuns(context.Context, *iapiserver.WorkflowCanvasRunListRequest) (*iapiserver.WorkflowCanvasRunListResponse, error)
	CreateRun(context.Context, *iapiserver.WorkflowCanvasRunCreateRequest) (*iapiserver.WorkflowCanvasRun, error)
	GetRun(context.Context, string) (*iapiserver.WorkflowCanvasRun, error)
	ListFlowRuns(context.Context, *iapiserver.CanvasFlowRunListRequest) (*iapiserver.CanvasFlowRunListResponse, error)
	ListNodeRuns(context.Context, *iapiserver.CanvasNodeRunListRequest) (*iapiserver.CanvasNodeRunListResponse, error)
	GetNodeRun(context.Context, string) (*iapiserver.CanvasNodeRunDetail, error)
	CancelRun(context.Context, string, *iapiserver.WorkflowCanvasRunCancelRequest) (*iapiserver.WorkflowCanvasRun, error)
	RetryRun(context.Context, string, *iapiserver.WorkflowCanvasRunRetryRequest) (*iapiserver.WorkflowCanvasRun, error)
	GetCanvasRunSummaries(context.Context, string, []string) (map[string]*iapiserver.RelatedResourceSummary, error)
}

type service struct {
	factory store.Factory
	store   store.WorkflowCanvasStore
	tasks   taskcentersvc.TaskCenterSrv
}

func New(factory store.Factory, tasks taskcentersvc.TaskCenterSrv) Service {
	return &service{factory: factory, store: factory.WorkflowCanvases(), tasks: tasks}
}

// ListNodeDefinitions 返回当前默认 project/namespace 可用的受控节点定义目录。
func (s *service) ListNodeDefinitions(
	ctx context.Context,
	req *iapiserver.WorkflowNodeDefinitionListRequest,
) (*iapiserver.WorkflowNodeDefinitionListResponse, error) {
	items, total, err := s.store.ListWorkflowNodeDefinitions(ctx, req, iapiserver.DefaultTaskCenterProjectID, iapiserver.DefaultTaskCenterNamespace)
	if err != nil {
		return nil, err
	}
	result := make([]*iapiserver.WorkflowNodeDefinitionListItem, 0, len(items))
	for _, item := range items {
		result = append(
			result,
			&iapiserver.WorkflowNodeDefinitionListItem{
				NodeType:          item.NodeType,
				DefinitionVersion: item.DefinitionVersion,
				Title:             item.Title,
				Description:       item.Description,
				Category:          item.Category,
				NodeKind:          item.NodeKind,
				ExecutionMode:     item.ExecutionBinding.Mode,
				RendererKey:       item.RendererKey,
				AvailabilityScope: item.AvailabilityScope,
				Deprecated:        item.Deprecated,
				CreatedAt:         item.CreatedAt,
			},
		)
	}
	return &iapiserver.WorkflowNodeDefinitionListResponse{Total: total, Items: result}, nil
}

// RegisterNodeDefinition 幂等注册不可变定义版本，只允许受控执行绑定。
func (s *service) RegisterNodeDefinition(
	ctx context.Context,
	req *iapiserver.WorkflowNodeDefinitionRegisterRequest,
) (*iapiserver.WorkflowNodeDefinition, error) {
	if err := validateNodeDefinitionRequest(req); err != nil {
		return nil, err
	}
	item := &iapiserver.WorkflowNodeDefinition{
		NodeType:                req.NodeType,
		DefinitionVersion:       req.DefinitionVersion,
		Title:                   req.Title,
		Category:                req.Category,
		NodeKind:                req.NodeKind,
		Ports:                   req.Ports,
		ConfigSchema:            req.ConfigSchema,
		ControllerStateSchema:   req.ControllerStateSchema,
		ControllerSchemaVersion: req.ControllerSchemaVersion,
		ExecutionBinding:        req.ExecutionBinding,
		Renderer:                req.Renderer,
		CacheAllowed:            req.CacheAllowed,
		ReuseTTLSeconds:         req.ReuseTTLSeconds,
		AvailabilityScope:       req.AvailabilityScope,
		ProjectID:               req.ProjectID,
		Namespace:               req.Namespace,
		RegisteredBy:            actor(ctx),
	}
	item.ID = uuid.NewString()
	item.Name = req.NodeType + "@" + req.DefinitionVersion
	item.Description = req.Description
	result, _, err := s.store.AddWorkflowNodeDefinitionIdempotent(ctx, item)
	return result, err
}

// GetNodeDefinition 查询固定节点定义版本，包括已发布 CanvasVersion 所需历史定义。
func (s *service) GetNodeDefinition(ctx context.Context, nodeType, version string) (*iapiserver.WorkflowNodeDefinition, error) {
	return s.store.GetWorkflowNodeDefinition(ctx, nodeType, version, iapiserver.DefaultTaskCenterProjectID, iapiserver.DefaultTaskCenterNamespace, true)
}

// DeprecateNodeDefinition 阻止定义被新草稿引用，历史版本和运行仍可读。
func (s *service) DeprecateNodeDefinition(
	ctx context.Context,
	nodeType, version string,
	_ *iapiserver.WorkflowNodeDefinitionDeprecateRequest,
) (*iapiserver.WorkflowNodeDefinition, error) {
	return s.store.DeprecateWorkflowNodeDefinition(ctx, nodeType, version, actor(ctx))
}

// GetCanvasRunSummaries 按创建用户批量返回非敏感 CanvasRun 摘要。
func (s *service) GetCanvasRunSummaries(ctx context.Context, ownerUserID string, ids []string) (map[string]*iapiserver.RelatedResourceSummary, error) {
	result := make(map[string]*iapiserver.RelatedResourceSummary)
	items, err := s.store.GetWorkflowCanvasRunsByIDs(ctx, sliceutil.Unique(ids))
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item == nil || item.CreatedBy != ownerUserID {
			continue
		}
		name := item.Name
		if name == "" {
			name = "Canvas run"
		}
		result[item.ID] = &iapiserver.RelatedResourceSummary{Type: "canvas_run", ID: item.ID, Name: name, Status: item.Status}
	}
	return result, nil
}

func (s *service) List(ctx context.Context, req *iapiserver.WorkflowCanvasListRequest) (*iapiserver.WorkflowCanvasListResponse, error) {
	user := actor(ctx)
	items, total, err := s.store.ListWorkflowCanvases(ctx, req, iapiserver.DefaultTaskCenterProjectID, iapiserver.DefaultTaskCenterNamespace, user)
	if err != nil {
		return nil, err
	}
	result := make([]*iapiserver.WorkflowCanvasListItem, 0, len(items))
	for _, item := range items {
		result = append(
			result,
			&iapiserver.WorkflowCanvasListItem{
				CanvasID:                 item.ID,
				Name:                     item.Name,
				Description:              item.Description,
				Visibility:               item.Visibility,
				DraftRevision:            item.DraftRevision,
				LatestVersion:            item.LatestVersion,
				LatestPublishedVersionID: item.LatestPublishedVersionID,
				ProjectID:                item.ProjectID,
				Namespace:                item.Namespace,
				CreatedBy:                item.CreatedBy,
				CreatedAt:                item.CreatedAt,
				UpdatedAt:                item.UpdatedAt,
			},
		)
	}
	return &iapiserver.WorkflowCanvasListResponse{Total: total, Items: result}, nil
}
func (s *service) Create(ctx context.Context, req *iapiserver.WorkflowCanvasCreateRequest) (*iapiserver.WorkflowCanvas, error) {
	graph := req.DraftGraph
	if graph.Nodes == nil {
		graph.Nodes = []iapiserver.WorkflowCanvasNode{}
	}
	if graph.Edges == nil {
		graph.Edges = []iapiserver.WorkflowCanvasEdge{}
	}
	if graph.Flows == nil {
		graph.Flows = []iapiserver.WorkflowCanvasFlow{}
	}
	if err := validateGraph(graph, false); err != nil {
		return nil, err
	}
	item := &iapiserver.WorkflowCanvas{
		Visibility:    req.Visibility,
		DraftGraph:    graph,
		DraftRevision: 1,
		ProjectID:     iapiserver.DefaultTaskCenterProjectID,
		Namespace:     iapiserver.DefaultTaskCenterNamespace,
		CreatedBy:     actor(ctx),
	}
	item.ID = uuid.NewString()
	item.Name = req.Name
	item.Description = req.Description
	return s.store.AddWorkflowCanvas(ctx, item)
}
func (s *service) Get(ctx context.Context, id string) (*iapiserver.WorkflowCanvas, error) {
	item, err := s.store.GetWorkflowCanvas(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.CreatedBy != actor(ctx) {
		return nil, errors.NewStatus(code.ErrCanvasNotFound, "canvas not found")
	}
	return item, nil
}
func (s *service) Update(ctx context.Context, req *iapiserver.WorkflowCanvasUpdateRequest) (*iapiserver.WorkflowCanvas, error) {
	item, err := s.Get(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.DraftGraph != nil {
		if err := validateGraph(*req.DraftGraph, false); err != nil {
			return nil, err
		}
		item.DraftGraph = *req.DraftGraph
	}
	if req.Name != nil {
		item.Name = *req.Name
	}
	if req.Description != nil {
		item.Description = *req.Description
	}
	if req.Visibility != nil {
		item.Visibility = *req.Visibility
	}
	item.DraftRevision = req.ExpectedDraftRevision + 1
	return s.store.UpdateWorkflowCanvas(ctx, item, req.ExpectedDraftRevision)
}
func (s *service) Delete(ctx context.Context, id string) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	return s.store.DeleteWorkflowCanvas(ctx, id)
}

// ValidateDraft 在不创建版本或任务的前提下一次返回可定位的草稿问题。
func (s *service) ValidateDraft(ctx context.Context, id string, req *iapiserver.WorkflowCanvasValidateRequest) (*iapiserver.WorkflowValidationResult, error) {
	canvas, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if canvas.DraftRevision != req.ExpectedDraftRevision {
		return nil, errors.NewStatus(code.ErrCanvasRevisionConflict, "canvas draft revision changed")
	}
	issues := validateGraphIssues(canvas.DraftGraph, true)
	if _, definitionErr := s.resolveDefinitions(ctx, canvas.DraftGraph, false); definitionErr != nil {
		issues = append(
			issues,
			iapiserver.WorkflowValidationIssue{
				Code:     fmt.Sprint(code.ErrCanvasNodeReferenceInvalid),
				Severity: "error",
				Message:  "canvas contains an unavailable node definition",
			},
		)
	}
	digest, digestErr := graphDigest(canvas.DraftGraph)
	if digestErr != nil {
		return nil, errors.NewStatus(code.ErrCanvasGraphInvalid, "canvas graph cannot be canonicalized")
	}
	result := &iapiserver.WorkflowValidationResult{
		Valid:     len(issues) == 0,
		Issues:    issues,
		NodeCount: len(canvas.DraftGraph.Nodes),
		EdgeCount: len(canvas.DraftGraph.Edges),
	}
	if result.Valid {
		result.ContentDigest = &digest
	}
	return result, nil
}

func (s *service) Publish(ctx context.Context, id string, req *iapiserver.WorkflowCanvasPublishRequest) (*iapiserver.CanvasVersion, error) {
	canvas, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	expectedRevision := req.ExpectedDraftRevision
	if expectedRevision == 0 {
		expectedRevision = req.DraftRevision
	}
	if canvas.DraftRevision != expectedRevision {
		return nil, errors.NewStatus(code.ErrCanvasRevisionConflict, "canvas draft revision changed")
	}
	if err := validateGraph(canvas.DraftGraph, true); err != nil {
		return nil, err
	}
	definitions, err := s.resolveDefinitions(ctx, canvas.DraftGraph, false)
	if err != nil {
		return nil, err
	}
	dagReq, err := s.dagRequest(ctx, canvas.DraftGraph, definitions, canvas.ProjectID, canvas.Namespace)
	if err != nil {
		return nil, err
	}
	digest, err := graphDigest(canvas.DraftGraph)
	if err != nil {
		return nil, errors.NewStatus(code.ErrCanvasPublishFailed, err.Error())
	}
	definitionName := "canvas_" + strings.ReplaceAll(canvas.ID, "-", "") + "_" + strings.TrimPrefix(digest, "sha256:")[:16]
	binding, err := s.tasks.RegisterDAGDefinition(ctx, definitionName, workflowDefinitionVersion(digest), dagReq)
	if err != nil {
		return nil, errors.NewStatus(code.ErrCanvasPublishFailed, "workflow definition registration failed")
	}
	version := &iapiserver.CanvasVersion{
		CanvasID:                  canvas.ID,
		GraphSnapshot:             canvas.DraftGraph,
		DefinitionSnapshots:       definitions,
		InputSchema:               map[string]any{},
		OutputSchema:              map[string]any{},
		ContentDigest:             digest,
		ExecutionTemplateDigest:   digest,
		WorkflowDefinitionName:    binding.Name,
		WorkflowDefinitionVersion: fmt.Sprintf("%d", binding.Version),
		CompiledDefinitionName:    binding.Name,
		CompiledDefinitionVersion: binding.Version,
		CompileSummary:            map[string]any{"node_count": len(canvas.DraftGraph.Nodes), "edge_count": len(canvas.DraftGraph.Edges)},
		NodeCount:                 len(canvas.DraftGraph.Nodes),
		EdgeCount:                 len(canvas.DraftGraph.Edges),
		PublishedBy:               actor(ctx),
		PublishedAt:               imachinery.Now(),
	}
	version.ID = uuid.NewString()
	version.Name = fmt.Sprintf("%s v%d", canvas.Name, canvas.LatestVersion+1)
	result, err := s.store.PublishWorkflowCanvas(ctx, canvas, version, expectedRevision)
	if result != nil {
		result.Canvas = canvasSummary(canvas)
	}
	return result, err
}
func (s *service) ListVersions(ctx context.Context, req *iapiserver.CanvasVersionListRequest) (*iapiserver.CanvasVersionListResponse, error) {
	canvas, err := s.Get(ctx, req.CanvasID)
	if err != nil {
		return nil, err
	}
	items, total, err := s.store.ListCanvasVersions(ctx, req)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		item.Canvas = canvasSummary(canvas)
	}
	result := make([]*iapiserver.CanvasVersionListItem, 0, len(items))
	for _, item := range items {
		result = append(
			result,
			&iapiserver.CanvasVersionListItem{
				CanvasVersionID:         item.ID,
				CanvasID:                item.CanvasID,
				Canvas:                  item.Canvas,
				Version:                 item.Version,
				ContentDigest:           item.ContentDigest,
				ExecutionTemplateDigest: item.ExecutionTemplateDigest,
				NodeCount:               item.NodeCount,
				EdgeCount:               item.EdgeCount,
				PublishedBy:             item.PublishedBy,
				PublishedAt:             item.PublishedAt,
			},
		)
	}
	return &iapiserver.CanvasVersionListResponse{Total: total, Items: result}, nil
}
func (s *service) GetVersion(ctx context.Context, id string) (*iapiserver.CanvasVersion, error) {
	v, err := s.store.GetCanvasVersion(ctx, id)
	if err != nil {
		return nil, err
	}
	canvas, err := s.Get(ctx, v.CanvasID)
	if err != nil {
		return nil, err
	}
	v.Canvas = canvasSummary(canvas)
	return v, nil
}
func (s *service) ListRuns(ctx context.Context, req *iapiserver.WorkflowCanvasRunListRequest) (*iapiserver.WorkflowCanvasRunListResponse, error) {
	items, total, err := s.store.ListWorkflowCanvasRuns(ctx, req, iapiserver.DefaultTaskCenterProjectID, iapiserver.DefaultTaskCenterNamespace, actor(ctx))
	if err != nil {
		return nil, err
	}
	if err := s.attachCanvasRunRelations(ctx, items); err != nil {
		return nil, err
	}
	result := make([]*iapiserver.WorkflowCanvasRunListItem, 0, len(items))
	for _, item := range items {
		result = append(
			result,
			&iapiserver.WorkflowCanvasRunListItem{
				CanvasRunID:        item.ID,
				CanvasID:           item.CanvasID,
				Canvas:             item.Canvas,
				CanvasVersionID:    item.CanvasVersionID,
				CanvasVersion:      item.CanvasVersion,
				DAGTaskGroupID:     item.DAGTaskGroupID,
				DAGTaskGroup:       item.DAGTaskGroup,
				TaskCreationStatus: item.TaskCreationStatus,
				Status:             item.Status,
				Progress:           item.Progress,
				Summary:            item.Summary,
				Warnings:           item.Warnings,
				RetryOfCanvasRunID: item.RetryOfCanvasRunID,
				RetryOfCanvasRun:   item.RetryOfCanvasRun,
				RetryIntent:        item.RetryIntent,
				AggregateVersion:   item.AggregateVersion,
				ProjectID:          item.ProjectID,
				Namespace:          item.Namespace,
				CreatedBy:          item.CreatedBy,
				CreatedAt:          item.CreatedAt,
				FinishedAt:         nullableCanvasTime(item.FinishedAt),
			},
		)
	}
	return &iapiserver.WorkflowCanvasRunListResponse{Total: total, Items: result}, nil
}

// ValidateRun 预检固定版本、scope、输入和策略，不创建 CanvasRun 或 Task Center 资源。
func (s *service) ValidateRun(
	ctx context.Context,
	versionID string,
	req *iapiserver.WorkflowCanvasRunValidateRequest,
) (*iapiserver.WorkflowCanvasRunValidationResult, error) {
	version, err := s.GetVersion(ctx, versionID)
	if err != nil {
		return nil, err
	}
	plan, err := buildExecutionPlan(version, req.Scope, req.RunPolicy, req.RuntimeInputs)
	if err != nil {
		return nil, err
	}
	return &iapiserver.WorkflowCanvasRunValidationResult{
		WorkflowValidationResult: iapiserver.WorkflowValidationResult{
			Valid:     true,
			Issues:    []iapiserver.WorkflowValidationIssue{},
			NodeCount: version.NodeCount,
			EdgeCount: version.EdgeCount,
		},
		SelectedNodeCount:   len(plan.Graph.Nodes),
		ExecutableTaskCount: plan.ExecutableTasks,
		ReusedNodeCount:     0,
		FlowCount:           len(plan.Flows),
		ExecutionPlanDigest: &plan.Digest,
	}, nil
}

func (s *service) CreateRun(ctx context.Context, req *iapiserver.WorkflowCanvasRunCreateRequest) (*iapiserver.WorkflowCanvasRun, error) {
	return s.createRun(ctx, req, nil, nil)
}

func (s *service) createRun(
	ctx context.Context,
	req *iapiserver.WorkflowCanvasRunCreateRequest,
	retryOf, retryIntent *string,
) (*iapiserver.WorkflowCanvasRun, error) {
	version, err := s.GetVersion(ctx, req.CanvasVersionID)
	if err != nil {
		return nil, err
	}
	if req.RuntimeInputs == nil {
		req.RuntimeInputs = req.Input
	}
	if req.Scope.Mode == "" {
		req.Scope.Mode = iapiserver.CanvasRunScopeAll
	}
	if req.RunPolicy.ReusePolicy == "" {
		req.RunPolicy.ReusePolicy = iapiserver.CanvasReuseRerunAll
	}
	if req.RunPolicy.FailurePolicy == "" {
		req.RunPolicy.FailurePolicy = iapiserver.CanvasFailureContinueFlows
	}
	plan, err := buildExecutionPlan(version, req.Scope, req.RunPolicy, req.RuntimeInputs)
	if err != nil {
		return nil, err
	}
	digest, err := requestDigest(req.CanvasVersionID, req.Scope, req.RunPolicy, req.RuntimeInputs)
	if err != nil {
		return nil, err
	}
	if retryOf != nil {
		digest, err = digestValue(map[string]any{"request_digest": digest, "retry_of_canvas_run_id": retryOf, "retry_intent": retryIntent})
		if err != nil {
			return nil, err
		}
	}
	run := &iapiserver.WorkflowCanvasRun{
		CanvasID:            version.CanvasID,
		CanvasVersionID:     version.ID,
		IdempotencyKey:      req.IdempotencyKey,
		RequestDigest:       digest,
		InputSnapshot:       req.RuntimeInputs,
		Scope:               req.Scope,
		RunPolicy:           req.RunPolicy,
		ReuseDecisions:      []map[string]any{},
		ExecutionPlan:       plan.Snapshot,
		ExecutionPlanDigest: plan.Digest,
		TaskCreationStatus:  iapiserver.CanvasTaskCreationPending,
		Status:              iapiserver.CanvasRunStatusPending,
		Progress:            0,
		Summary: map[string]any{
			"progress":  0,
			"total":     len(plan.Graph.Nodes),
			"completed": 0,
			"success":   0,
			"failed":    0,
			"canceled":  0,
			"skipped":   0,
		},
		ResultSummary:      map[string]any{},
		Warnings:           []iapiserver.WorkflowRunWarning{},
		LastError:          map[string]any{},
		AggregateVersion:   1,
		RetryOfCanvasRunID: retryOf,
		RetryIntent:        retryIntent,
		ProjectID:          iapiserver.DefaultTaskCenterProjectID,
		Namespace:          iapiserver.DefaultTaskCenterNamespace,
		CreatedBy:          actor(ctx),
	}
	run.ID = uuid.NewString()
	run.Name = "Canvas run"
	run.Extend = map[string]any{
		"canvas_summary":         version.Canvas,
		"canvas_version_summary": canvasVersionSummary(version),
	}
	created, isNew, err := s.store.AddWorkflowCanvasRunIdempotent(ctx, run)
	if err != nil {
		return created, err
	}
	if !isNew {
		if relationErr := s.attachCanvasRunRelations(ctx, []*iapiserver.WorkflowCanvasRun{created}); relationErr != nil {
			return nil, relationErr
		}
		return created, nil
	}
	dagReq, err := s.dagRequest(ctx, plan.Graph, version.DefinitionSnapshots, run.ProjectID, run.Namespace)
	if err != nil {
		return s.failRun(ctx, created, err)
	}
	dagReq.Name = "Canvas run " + run.ID
	dagReq.CanvasVersionID = version.ID
	dagReq.IdempotencyScope = "canvas-run"
	dagReq.IdempotencyKey = req.IdempotencyKey
	group, err := s.tasks.CreateDAGTaskGroup(ctx, dagReq)
	if err != nil {
		return s.failRun(ctx, created, err)
	}
	taskList, err := s.tasks.ListDAGTaskGroupTasks(ctx, group.ID, &iapiserver.AtomicTaskListRequest{})
	if err != nil {
		return s.failRun(ctx, created, err)
	}
	byKey := map[string]*iapiserver.AtomicTask{}
	for _, task := range taskList.Items {
		byKey[task.ChildKey] = task
	}
	flowRuns := make([]*iapiserver.CanvasFlowRun, 0, len(plan.Flows))
	for _, flow := range plan.Flows {
		flowRun := &iapiserver.CanvasFlowRun{
			CanvasRunID:      run.ID,
			FlowID:           flow.FlowID,
			ExecutionKeys:    plan.FlowExecutionKeys[flow.FlowID],
			Status:           iapiserver.CanvasRunStatusPending,
			Summary:          iapiserver.WorkflowProgressSummary{},
			ResultSummary:    map[string]any{},
			Warnings:         []iapiserver.WorkflowRunWarning{},
			AggregateVersion: 1,
		}
		flowRun.ID = uuid.NewString()
		flowRun.Name = flow.Name
		flowRuns = append(flowRuns, flowRun)
	}
	definitions := definitionMap(version.DefinitionSnapshots)
	nodeRuns := make([]*iapiserver.CanvasNodeRun, 0, len(plan.Graph.Nodes))
	taskBindings := make([]*iapiserver.CanvasNodeRunTaskBinding, 0, len(taskList.Items))
	outputBindings := make([]*iapiserver.CanvasNodeRunOutputBinding, 0)
	for _, node := range plan.Graph.Nodes {
		id := canvasNodeID(node)
		def := definitions[node.NodeType+"@"+node.DefinitionVersion]
		resultMode, status := iapiserver.CanvasResultExecuted, iapiserver.AtomicTaskStatusBlocked
		if def != nil && def.ExecutionBinding.Mode == iapiserver.CanvasExecutionPassive {
			resultMode, status = iapiserver.CanvasResultPassive, iapiserver.CanvasRunStatusSuccess
		}
		fingerprint, _ := executionFingerprint(version.ID, node, req.RuntimeInputs)
		nr := &iapiserver.CanvasNodeRun{
			CanvasRunID:           run.ID,
			NodeID:                id,
			ExecutionKey:          id,
			NodeType:              node.NodeType,
			DefinitionVersion:     node.DefinitionVersion,
			ExecutionFingerprint:  fingerprint,
			ResolvedInputSnapshot: node.LiteralInputs,
			ResultMode:            resultMode,
			Status:                status,
			Warnings:              []iapiserver.WorkflowRunWarning{},
			LastError:             map[string]any{},
			AggregateVersion:      1,
		}
		nr.ID = uuid.NewString()
		nr.Name = id
		if task := byKey[id]; task != nil {
			nr.Status = task.Status
			nr.TaskCount = 1
			task.CanvasRunID = run.ID
			task.CanvasNodeRunID = nr.ID
			if _, updateErr := s.factory.TaskCenters().UpdateAtomicTask(ctx, task); updateErr != nil {
				return s.failRun(ctx, created, updateErr)
			}
			binding := &iapiserver.CanvasNodeRunTaskBinding{
				CanvasNodeRunID:     nr.ID,
				DAGTaskGroupID:      group.ID,
				AtomicTaskID:        task.ID,
				TaskChildKey:        task.ChildKey,
				BindingRole:         "primary",
				ShardKey:            "root",
				TaskResourceVersion: task.ResourceVersion,
			}
			binding.ID = uuid.NewString()
			binding.Name = task.ChildKey
			taskBindings = append(taskBindings, binding)
		}
		if def != nil {
			for _, port := range def.Ports {
				if port.Direction != "output" {
					continue
				}
				binding := &iapiserver.CanvasNodeRunOutputBinding{
					CanvasNodeRunID:    nr.ID,
					PortKey:            port.Key,
					Required:           port.RequiredForCompletion,
					ShardKey:           "root",
					ProducerKey:        run.ID + ":" + nr.ID + ":" + port.Key + ":root",
					AvailabilityStatus: "PENDING",
				}
				binding.ID = uuid.NewString()
				binding.Name = port.Key
				outputBindings = append(outputBindings, binding)
				nr.OutputCount++
				if port.RequiredForCompletion {
					nr.RequiredOutputCount++
				}
			}
		}
		nodeRuns = append(nodeRuns, nr)
	}
	result, err := s.store.BindWorkflowCanvasRun(ctx, run.ID, group.ID, flowRuns, nodeRuns, taskBindings, outputBindings)
	if err != nil {
		return nil, err
	}
	if err := s.attachCanvasRunRelations(ctx, []*iapiserver.WorkflowCanvasRun{result}); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *service) failRun(ctx context.Context, run *iapiserver.WorkflowCanvasRun, cause error) (*iapiserver.WorkflowCanvasRun, error) {
	run.TaskCreationStatus = iapiserver.CanvasTaskCreationFailed
	run.TaskCreationAttempts++
	run.LastError = map[string]any{"message": cause.Error()}
	updated, err := s.store.UpdateWorkflowCanvasRun(ctx, run)
	if err != nil {
		return nil, err
	}
	return updated, errors.NewStatus(code.ErrCanvasTaskCreationUnavailable, "task center temporarily unavailable")
}
func (s *service) GetRun(ctx context.Context, id string) (*iapiserver.WorkflowCanvasRun, error) {
	run, err := s.store.GetWorkflowCanvasRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if run.CreatedBy != actor(ctx) {
		return nil, errors.NewStatus(code.ErrCanvasRunNotFound, "canvas run not found")
	}
	if err := s.attachCanvasRunRelations(ctx, []*iapiserver.WorkflowCanvasRun{run}); err != nil {
		return nil, err
	}
	return run, nil
}

// ListFlowRuns 返回 CanvasRun 内显式流的分页投影。
func (s *service) ListFlowRuns(ctx context.Context, req *iapiserver.CanvasFlowRunListRequest) (*iapiserver.CanvasFlowRunListResponse, error) {
	if _, err := s.GetRun(ctx, req.CanvasRunID); err != nil {
		return nil, err
	}
	items, total, err := s.store.ListCanvasFlowRuns(ctx, req)
	if err != nil {
		return nil, err
	}
	return &iapiserver.CanvasFlowRunListResponse{Total: total, Items: items}, nil
}
func (s *service) ListNodeRuns(ctx context.Context, req *iapiserver.CanvasNodeRunListRequest) (*iapiserver.CanvasNodeRunListResponse, error) {
	if _, err := s.GetRun(ctx, req.CanvasRunID); err != nil {
		return nil, err
	}
	items, total, err := s.store.ListCanvasNodeRuns(ctx, req)
	if err != nil {
		return nil, err
	}
	s.attachCanvasNodeRunRelations(ctx, items)
	return &iapiserver.CanvasNodeRunListResponse{Total: total, Items: items}, nil
}

// GetNodeRun 返回单个执行实例及其有界 task/output bindings。
func (s *service) GetNodeRun(ctx context.Context, id string) (*iapiserver.CanvasNodeRunDetail, error) {
	node, err := s.store.GetCanvasNodeRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := s.GetRun(ctx, node.CanvasRunID); err != nil {
		return nil, errors.NewStatus(code.ErrCanvasNodeRunNotFound, "canvas node run not found")
	}
	tasks, outputs, err := s.store.GetCanvasNodeRunDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.tasks != nil {
		ids := make([]string, 0, len(tasks))
		for _, binding := range tasks {
			ids = append(ids, binding.AtomicTaskID)
		}
		summaries, summaryErr := s.tasks.GetAtomicTaskSummaries(ctx, sliceutil.Unique(ids))
		if summaryErr == nil {
			for _, binding := range tasks {
				binding.AtomicTask = summaries[binding.AtomicTaskID]
			}
		}
	}
	return &iapiserver.CanvasNodeRunDetail{CanvasNodeRun: node, TaskBindings: tasks, OutputBindings: outputs}, nil
}

func (s *service) CancelRun(ctx context.Context, id string, _ *iapiserver.WorkflowCanvasRunCancelRequest) (*iapiserver.WorkflowCanvasRun, error) {
	run, err := s.GetRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if run.DAGTaskGroupID == nil || isRunTerminal(run.Status) {
		return nil, errors.NewStatus(code.ErrCanvasRunStateBlocked, "canvas run cannot be canceled")
	}
	if _, err := s.tasks.CancelDAGTaskGroup(ctx, *run.DAGTaskGroupID); err != nil {
		return nil, err
	}
	run.Status = iapiserver.CanvasRunStatusCanceled
	run.FinishedAt = imachinery.Now()
	updated, err := s.store.UpdateWorkflowCanvasRun(ctx, run)
	if err != nil {
		return nil, err
	}
	if err := s.attachCanvasRunRelations(ctx, []*iapiserver.WorkflowCanvasRun{updated}); err != nil {
		return nil, err
	}
	return updated, nil
}
func (s *service) RetryRun(ctx context.Context, id string, req *iapiserver.WorkflowCanvasRunRetryRequest) (*iapiserver.WorkflowCanvasRun, error) {
	source, err := s.GetRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if !isRunTerminal(source.Status) {
		return nil, errors.NewStatus(code.ErrCanvasRunStateBlocked, "canvas run cannot be retried")
	}
	if err := validateRetryIntent(req); err != nil {
		return nil, err
	}
	scope := source.Scope
	switch req.Intent {
	case iapiserver.CanvasRetryNode:
		scope = iapiserver.WorkflowRunScope{Mode: iapiserver.CanvasRunScopeOnlyNodes, NodeIDs: req.NodeIDs}
	case iapiserver.CanvasRetryFromNode:
		scope = iapiserver.WorkflowRunScope{Mode: iapiserver.CanvasRunScopeFromNodes, NodeIDs: req.NodeIDs}
	case iapiserver.CanvasRetryFlow:
		scope = iapiserver.WorkflowRunScope{Mode: iapiserver.CanvasRunScopeFlows, FlowIDs: req.FlowIDs}
	case iapiserver.CanvasRetryAll:
		scope = iapiserver.WorkflowRunScope{Mode: iapiserver.CanvasRunScopeAll}
	}
	policy := source.RunPolicy
	if req.ReusePolicy != "" {
		policy.ReusePolicy = req.ReusePolicy
	}
	created, err := s.createRun(
		ctx,
		&iapiserver.WorkflowCanvasRunCreateRequest{
			CanvasVersionID: source.CanvasVersionID,
			IdempotencyKey:  req.IdempotencyKey,
			Scope:           scope,
			RunPolicy:       policy,
			RuntimeInputs:   source.InputSnapshot,
		},
		&source.ID,
		&req.Intent,
	)
	if err == nil {
		err = s.attachCanvasRunRelations(ctx, []*iapiserver.WorkflowCanvasRun{created})
	}
	return created, err
}

// attachCanvasRunRelations 在固定查询预算内组合 Canvas、版本、重跑来源和 Task Center DAG 摘要。
func (s *service) attachCanvasRunRelations(ctx context.Context, runs []*iapiserver.WorkflowCanvasRun) error {
	canvasIDs := make([]string, 0, len(runs))
	versionIDs := make([]string, 0, len(runs))
	retryIDs := make([]string, 0, len(runs))
	dagIDs := make([]string, 0, len(runs))
	for _, run := range runs {
		if run == nil {
			continue
		}
		run.Canvas = canvasRunSnapshot[iapiserver.CanvasSummary](run, "canvas_summary")
		run.CanvasVersion = canvasRunSnapshot[iapiserver.CanvasVersionSummary](run, "canvas_version_summary")
		if run.Canvas == nil {
			canvasIDs = append(canvasIDs, run.CanvasID)
		}
		if run.CanvasVersion == nil {
			versionIDs = append(versionIDs, run.CanvasVersionID)
		}
		if run.RetryOfCanvasRunID != nil {
			retryIDs = append(retryIDs, *run.RetryOfCanvasRunID)
		}
		if run.DAGTaskGroupID != nil {
			dagIDs = append(dagIDs, *run.DAGTaskGroupID)
		}
	}

	canvasByID := make(map[string]*iapiserver.WorkflowCanvas)
	if len(canvasIDs) > 0 {
		canvases, err := s.store.GetWorkflowCanvasesByIDs(ctx, sliceutil.Unique(canvasIDs))
		if err != nil {
			return err
		}
		for _, item := range canvases {
			canvasByID[item.ID] = item
		}
	}
	versionByID := make(map[string]*iapiserver.CanvasVersion)
	if len(versionIDs) > 0 {
		versions, err := s.store.GetCanvasVersionsByIDs(ctx, sliceutil.Unique(versionIDs))
		if err != nil {
			return err
		}
		for _, item := range versions {
			versionByID[item.ID] = item
		}
	}
	retryByID := make(map[string]*iapiserver.WorkflowCanvasRun)
	if len(retryIDs) > 0 {
		retries, err := s.store.GetWorkflowCanvasRunsByIDs(ctx, sliceutil.Unique(retryIDs))
		if err != nil {
			return err
		}
		for _, item := range retries {
			retryByID[item.ID] = item
		}
	}
	dagByID := map[string]*iapiserver.DAGTaskGroupSummary{}
	if s.tasks != nil {
		if summaries, summaryErr := s.tasks.GetDAGTaskGroupSummaries(ctx, sliceutil.Unique(dagIDs)); summaryErr == nil {
			dagByID = summaries
		}
	}
	for _, run := range runs {
		if run == nil {
			continue
		}
		if run.Canvas == nil && canvasByID[run.CanvasID] != nil {
			run.Canvas = canvasSummary(canvasByID[run.CanvasID])
		}
		if run.CanvasVersion == nil && versionByID[run.CanvasVersionID] != nil {
			run.CanvasVersion = canvasVersionSummary(versionByID[run.CanvasVersionID])
		}
		if run.RetryOfCanvasRunID != nil {
			source := retryByID[*run.RetryOfCanvasRunID]
			if source != nil && source.CreatedBy == run.CreatedBy && source.ProjectID == run.ProjectID && source.Namespace == run.Namespace {
				run.RetryOfCanvasRun = canvasRunSummary(source)
			}
		}
		if run.DAGTaskGroupID != nil {
			run.DAGTaskGroup = dagByID[*run.DAGTaskGroupID]
		}
	}
	return nil
}

func (s *service) attachCanvasNodeRunRelations(ctx context.Context, runs []*iapiserver.CanvasNodeRun) {
	if s.tasks == nil {
		return
	}
	ids := make([]string, 0, len(runs))
	for _, run := range runs {
		if run != nil && run.AtomicTaskID != nil {
			ids = append(ids, *run.AtomicTaskID)
		}
	}
	summaries, err := s.tasks.GetAtomicTaskSummaries(ctx, sliceutil.Unique(ids))
	if err != nil {
		return
	}
	for _, run := range runs {
		if run != nil && run.AtomicTaskID != nil {
			run.AtomicTask = summaries[*run.AtomicTaskID]
		}
	}
}

func canvasSummary(item *iapiserver.WorkflowCanvas) *iapiserver.CanvasSummary {
	return &iapiserver.CanvasSummary{CanvasID: item.ID, Name: item.Name, Visibility: item.Visibility}
}

func canvasVersionSummary(item *iapiserver.CanvasVersion) *iapiserver.CanvasVersionSummary {
	return &iapiserver.CanvasVersionSummary{
		CanvasVersionID: item.ID,
		CanvasID:        item.CanvasID,
		Version:         item.Version,
		ContentDigest:   item.ContentDigest,
		PublishedAt:     item.PublishedAt,
	}
}

func canvasRunSummary(item *iapiserver.WorkflowCanvasRun) *iapiserver.CanvasRunSummary {
	return &iapiserver.CanvasRunSummary{CanvasRunID: item.ID, Status: item.Status, Progress: item.Progress, CreatedAt: item.CreatedAt}
}
func nullableCanvasTime(value imachinery.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func canvasRunSnapshot[T any](run *iapiserver.WorkflowCanvasRun, key string) *T {
	if run == nil || run.Extend == nil || run.Extend[key] == nil {
		return nil
	}
	raw, err := json.Marshal(run.Extend[key])
	if err != nil {
		return nil
	}
	var result T
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil
	}
	return &result
}

func (s *service) dagRequest(
	ctx context.Context,
	graph iapiserver.WorkflowCanvasGraph,
	definitions []*iapiserver.WorkflowNodeDefinition,
	projectID, namespace string,
) (*iapiserver.DAGTaskGroupCreateRequest, error) {
	definitionByKey := definitionMap(definitions)
	nodes := make([]iapiserver.DAGNode, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		key := canvasNodeID(node)
		if node.Config == nil {
			node.Config = map[string]any{}
		}
		definition := definitionByKey[node.NodeType+"@"+node.DefinitionVersion]
		if definition != nil && definition.ExecutionBinding.Mode == iapiserver.CanvasExecutionPassive {
			continue
		}
		ref, _ := node.Config["function_ref"].(string)
		args := map[string]any{"config": node.Config, "literal_inputs": node.LiteralInputs, "controller_state": node.ControllerState}
		if definition != nil && definition.ExecutionBinding.FunctionRef != nil {
			ref = *definition.ExecutionBinding.FunctionRef
		}
		if definition != nil && definition.ExecutionBinding.ApplicationVersionID != nil {
			node.NodeType = iapiserver.CanvasNodeTypeApplication
			node.Config["application_version_id"] = *definition.ExecutionBinding.ApplicationVersionID
		}
		if node.NodeType == iapiserver.CanvasNodeTypeApplication {
			versionID, _ := node.Config["application_version_id"].(string)
			if versionID == "" {
				return nil, errors.NewStatusF(code.ErrCanvasNodeReferenceInvalid, "node %s has no application_version_id", key)
			}
			version, versionErr := s.factory.ApplicationPlatforms().GetApplicationVersion(ctx, versionID)
			if versionErr != nil || version.Status != iapiserver.VersionStatusPublished {
				return nil, errors.NewStatusF(code.ErrCanvasNodeReferenceInvalid, "node %s references an unavailable ApplicationVersion", key)
			}
			ref = "application-platform.run"
		}
		if ref == "" {
			return nil, errors.NewStatusF(code.ErrCanvasNodeReferenceInvalid, "node %s has no registered function_ref", key)
		}
		maxDynamic := 0
		if node.MaxDynamicTasks != nil {
			maxDynamic = *node.MaxDynamicTasks
		}
		nodes = append(
			nodes,
			iapiserver.DAGNode{
				Key:             key,
				Task:            iapiserver.AtomicTaskTemplate{Key: key, Name: key, FunctionRef: ref, Arguments: args},
				InputMapping:    node.InputBindings,
				DynamicFork:     definition != nil && definition.ExecutionBinding.Mode == iapiserver.CanvasExecutionExpanded,
				MaxDynamicTasks: maxDynamic,
			},
		)
	}
	executable := map[string]struct{}{}
	for _, node := range nodes {
		executable[node.Key] = struct{}{}
	}
	edges := make([]iapiserver.DAGEdge, 0, len(graph.Edges))
	for _, edge := range graph.Edges {
		from, to := canvasEdgeSource(edge), canvasEdgeTarget(edge)
		if _, ok := executable[from]; !ok {
			continue
		}
		if _, ok := executable[to]; !ok {
			continue
		}
		mapping := map[string]any{}
		if edge.ConnectionType != "control" {
			mapping[canvasEdgeTargetPort(edge)] = from + "." + canvasEdgeSourcePort(edge)
		}
		edges = append(edges, iapiserver.DAGEdge{FromNode: from, ToNode: to, DataMapping: mapping})
	}
	return &iapiserver.DAGTaskGroupCreateRequest{
		Name:          "Canvas definition",
		Nodes:         nodes,
		Edges:         edges,
		Input:         map[string]any{},
		OutputMapping: map[string]any{},
		ProjectID:     projectID,
		Namespace:     namespace,
	}, nil
}

var nodeKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,99}$`)
var nodeTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,199}$`)

func validateGraph(graph iapiserver.WorkflowCanvasGraph, requireNodes bool) error {
	issues := validateGraphIssues(graph, requireNodes)
	if len(issues) == 0 {
		return nil
	}
	for _, issue := range issues {
		if issue.Code == fmt.Sprint(code.ErrCanvasCycleDetected) {
			return errors.NewStatus(code.ErrCanvasCycleDetected, issue.Message)
		}
		if issue.Code == fmt.Sprint(code.ErrCanvasLimitExceeded) {
			return errors.NewStatus(code.ErrCanvasLimitExceeded, issue.Message)
		}
	}
	return errors.NewStatus(code.ErrCanvasGraphInvalid, issues[0].Message)
}

func validateGraphIssues(graph iapiserver.WorkflowCanvasGraph, requireNodes bool) []iapiserver.WorkflowValidationIssue {
	issues := make([]iapiserver.WorkflowValidationIssue, 0)
	add := func(value int, message string, nodeID, edgeID, flowID *string) {
		issues = append(
			issues,
			iapiserver.WorkflowValidationIssue{Code: fmt.Sprint(value), Severity: "error", Message: message, NodeID: nodeID, EdgeID: edgeID, FlowID: flowID},
		)
	}
	if len(graph.Nodes) > 1000 || len(graph.Edges) > 5000 {
		add(code.ErrCanvasLimitExceeded, "canvas graph limit exceeded", nil, nil, nil)
	}
	if requireNodes && len(graph.Nodes) == 0 {
		add(code.ErrCanvasGraphInvalid, "published canvas requires at least one node", nil, nil, nil)
	}
	keys := map[string]struct{}{}
	indegree := map[string]int{}
	next := map[string][]string{}
	for _, node := range graph.Nodes {
		key := canvasNodeID(node)
		if !nodeKeyPattern.MatchString(key) {
			value := key
			add(code.ErrCanvasGraphInvalid, "canvas node id is invalid", &value, nil, nil)
			continue
		}
		if _, ok := keys[key]; ok {
			value := key
			add(code.ErrCanvasGraphInvalid, "canvas node id is duplicated", &value, nil, nil)
			continue
		}
		if node.NodeType == "" || (node.DefinitionVersion == "" && node.NodeKey == "") {
			value := key
			add(code.ErrCanvasNodeReferenceInvalid, "canvas node definition reference is missing", &value, nil, nil)
		}
		if containsUnsafeCanvasConfig(node.Config) {
			value := key
			add(code.ErrWorkflowUnsafeNodeConfiguration, "canvas node config contains forbidden runtime fields", &value, nil, nil)
		}
		if node.MaxDynamicTasks != nil && (*node.MaxDynamicTasks < 1 || *node.MaxDynamicTasks > 1000) {
			value := key
			add(code.ErrCanvasLimitExceeded, "dynamic expansion limit is invalid", &value, nil, nil)
		}
		keys[key] = struct{}{}
		indegree[key] = 0
	}
	for _, edge := range graph.Edges {
		from, to, edgeID := canvasEdgeSource(edge), canvasEdgeTarget(edge), edge.EdgeID
		if edgeID == "" {
			edgeID = from + "->" + to
		}
		if _, ok := keys[from]; !ok {
			add(code.ErrCanvasGraphInvalid, "edge source is missing", nil, &edgeID, nil)
			continue
		}
		if _, ok := keys[to]; !ok {
			add(code.ErrCanvasGraphInvalid, "edge target is missing", nil, &edgeID, nil)
			continue
		}
		if from == to {
			add(code.ErrCanvasCycleDetected, "canvas graph contains a cycle", nil, &edgeID, nil)
			continue
		}
		next[from] = append(next[from], to)
		indegree[to]++
	}
	flowIDs := map[string]struct{}{}
	for _, flow := range graph.Flows {
		if flow.FlowID == "" {
			add(code.ErrCanvasGraphInvalid, "flow id is required", nil, nil, nil)
			continue
		}
		if _, exists := flowIDs[flow.FlowID]; exists {
			value := flow.FlowID
			add(code.ErrCanvasGraphInvalid, "flow id is duplicated", nil, nil, &value)
		}
		flowIDs[flow.FlowID] = struct{}{}
		if len(flow.OutputNodeIDs) == 0 {
			value := flow.FlowID
			add(code.ErrCanvasGraphInvalid, "flow requires an output node", nil, nil, &value)
		}
		for _, id := range append(append([]string{}, flow.EntryNodeIDs...), flow.OutputNodeIDs...) {
			if _, ok := keys[id]; !ok {
				value := flow.FlowID
				nodeID := id
				add(code.ErrCanvasGraphInvalid, "flow references a missing node", &nodeID, nil, &value)
			}
		}
	}
	queue := make([]string, 0)
	for key, n := range indegree {
		if n == 0 {
			queue = append(queue, key)
		}
	}
	sort.Strings(queue)
	visited := 0
	for len(queue) > 0 {
		key := queue[0]
		queue = queue[1:]
		visited++
		for _, child := range next[key] {
			indegree[child]--
			if indegree[child] == 0 {
				queue = append(queue, child)
			}
		}
	}
	if visited != len(keys) {
		add(code.ErrCanvasCycleDetected, "canvas graph contains a cycle", nil, nil, nil)
	}
	return issues
}
func containsUnsafeCanvasConfig(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalized := strings.ToLower(key)
			for _, forbidden := range []string{"url", "endpoint", "auth", "credential", "header", "script", "worker", "conductor"} {
				if strings.Contains(normalized, forbidden) {
					return true
				}
			}
			if containsUnsafeCanvasConfig(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsUnsafeCanvasConfig(child) {
				return true
			}
		}
	}
	return false
}
func graphDigest(graph iapiserver.WorkflowCanvasGraph) (string, error) {
	raw, err := json.Marshal(graph)
	if err != nil {
		return "", err
	}
	canonical, err := jcs.Transform(raw)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
func requestDigest(version string, scope iapiserver.WorkflowRunScope, policy iapiserver.WorkflowRunPolicy, input map[string]any) (string, error) {
	return digestValue(map[string]any{"canvas_version_id": version, "scope": scope, "run_policy": policy, "runtime_inputs": input})
}

func digestValue(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	canonical, err := jcs.Transform(raw)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func workflowDefinitionVersion(digest string) int {
	raw := strings.TrimPrefix(digest, "sha256:")
	if len(raw) < 8 {
		return 1
	}
	value, err := strconv.ParseUint(raw[:8], 16, 31)
	if err != nil || value == 0 {
		return 1
	}
	return int(value)
}

func validateNodeDefinitionRequest(req *iapiserver.WorkflowNodeDefinitionRegisterRequest) error {
	if req == nil || !nodeTypePattern.MatchString(req.NodeType) {
		return errors.NewStatus(code.ErrCanvasGraphInvalid, "node type is invalid")
	}
	binding := req.ExecutionBinding
	switch binding.Mode {
	case iapiserver.CanvasExecutionPassive:
		if binding.FunctionRef != nil || binding.ApplicationVersionID != nil {
			return errors.NewStatus(code.ErrCanvasNodeReferenceInvalid, "passive node cannot contain an execution reference")
		}
	case iapiserver.CanvasExecutionAtomic, iapiserver.CanvasExecutionExpanded:
		if (binding.FunctionRef == nil) == (binding.ApplicationVersionID == nil) {
			return errors.NewStatus(code.ErrCanvasNodeReferenceInvalid, "executable node requires exactly one execution reference")
		}
	default:
		return errors.NewStatus(code.ErrCanvasGraphInvalid, "execution mode is invalid")
	}
	if binding.BindingVersion == "" {
		return errors.NewStatus(code.ErrCanvasNodeReferenceInvalid, "binding version is required")
	}
	if binding.MaxDynamicTasks != nil && (*binding.MaxDynamicTasks < 1 || *binding.MaxDynamicTasks > 1000) {
		return errors.NewStatus(code.ErrCanvasLimitExceeded, "dynamic expansion limit is invalid")
	}
	if req.AvailabilityScope == iapiserver.CanvasAvailabilitySystem && (req.ProjectID != nil || req.Namespace != nil) {
		return errors.NewStatus(code.ErrCanvasGraphInvalid, "system definition cannot have project scope")
	}
	if req.AvailabilityScope == iapiserver.CanvasAvailabilityProject && (req.ProjectID == nil || req.Namespace == nil) {
		return errors.NewStatus(code.ErrCanvasGraphInvalid, "project definition requires project and namespace")
	}
	seen := map[string]string{}
	for _, port := range req.Ports {
		if !nodeKeyPattern.MatchString(port.Key) || (port.Direction != "input" && port.Direction != "output") ||
			(port.ConnectionType != "data" && port.ConnectionType != "control") ||
			(port.Cardinality != "single" && port.Cardinality != "multiple") {
			return errors.NewStatus(code.ErrCanvasGraphInvalid, "node definition port is invalid")
		}
		key := port.Direction + ":" + port.Key
		if _, ok := seen[key]; ok {
			return errors.NewStatus(code.ErrCanvasGraphInvalid, "node definition port is duplicated")
		}
		seen[key] = port.DataType
	}
	if _, err := compileJSONSchema(req.ConfigSchema, "config"); err != nil {
		return errors.NewStatus(code.ErrCanvasGraphInvalid, "config schema is invalid")
	}
	if req.ControllerStateSchema != nil {
		if req.ControllerSchemaVersion == nil {
			return errors.NewStatus(code.ErrWorkflowControllerStateInvalid, "controller schema version is required")
		}
		if _, err := compileJSONSchema(req.ControllerStateSchema, "controller"); err != nil {
			return errors.NewStatus(code.ErrWorkflowControllerStateInvalid, "controller schema is invalid")
		}
	}
	return nil
}

func compileJSONSchema(document map[string]any, name string) (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	location := "https://omnimam.local/workflow-canvas/" + name + ".schema.json"
	if err := compiler.AddResource(location, document); err != nil {
		return nil, err
	}
	return compiler.Compile(location)
}

func (s *service) resolveDefinitions(
	ctx context.Context,
	graph iapiserver.WorkflowCanvasGraph,
	includeDeprecated bool,
) ([]*iapiserver.WorkflowNodeDefinition, error) {
	result := make([]*iapiserver.WorkflowNodeDefinition, 0, len(graph.Nodes))
	seen := map[string]struct{}{}
	for _, node := range graph.Nodes {
		if node.DefinitionVersion == "" && node.NodeKey != "" {
			continue
		}
		key := node.NodeType + "@" + node.DefinitionVersion
		if _, ok := seen[key]; ok {
			continue
		}
		definition, err := s.store.GetWorkflowNodeDefinition(
			ctx,
			node.NodeType,
			node.DefinitionVersion,
			iapiserver.DefaultTaskCenterProjectID,
			iapiserver.DefaultTaskCenterNamespace,
			includeDeprecated,
		)
		if err != nil {
			return nil, errors.NewStatus(code.ErrCanvasNodeReferenceInvalid, "canvas node definition is unavailable")
		}
		configSchema, err := compileJSONSchema(definition.ConfigSchema, "config-"+strings.ReplaceAll(key, "@", "-"))
		if err != nil || configSchema.Validate(node.Config) != nil {
			return nil, errors.NewStatus(code.ErrCanvasGraphInvalid, "canvas node config does not match its definition")
		}
		if node.ControllerState != nil {
			if definition.ControllerStateSchema == nil {
				return nil, errors.NewStatus(code.ErrWorkflowControllerStateInvalid, "node does not accept controller state")
			}
			controllerSchema, schemaErr := compileJSONSchema(definition.ControllerStateSchema, "controller-"+strings.ReplaceAll(key, "@", "-"))
			if schemaErr != nil || controllerSchema.Validate(node.ControllerState) != nil {
				return nil, errors.NewStatus(code.ErrWorkflowControllerStateInvalid, "controller state does not match its definition")
			}
		}
		seen[key] = struct{}{}
		result = append(result, definition)
	}
	return result, nil
}

type executionPlan struct {
	Graph             iapiserver.WorkflowCanvasGraph
	Flows             []iapiserver.WorkflowCanvasFlow
	FlowExecutionKeys map[string][]string
	ExecutableTasks   int
	Digest            string
	Snapshot          map[string]any
}

func buildExecutionPlan(
	version *iapiserver.CanvasVersion,
	scope iapiserver.WorkflowRunScope,
	policy iapiserver.WorkflowRunPolicy,
	inputs map[string]any,
) (*executionPlan, error) {
	if policy.ReusePolicy == iapiserver.CanvasReuseRequired {
		return nil, errors.NewStatus(code.ErrCanvasReuseRequiredUnavailable, "reuse_required has no valid prior output")
	}
	if policy.ReusePolicy != iapiserver.CanvasReuseRerunAll && policy.ReusePolicy != iapiserver.CanvasReuseValidOutputs {
		return nil, errors.NewStatus(code.ErrCanvasRunScopeInvalid, "reuse policy is invalid")
	}
	if policy.FailurePolicy != iapiserver.CanvasFailureContinueFlows && policy.FailurePolicy != iapiserver.CanvasFailureFailFast {
		return nil, errors.NewStatus(code.ErrCanvasRunScopeInvalid, "failure policy is invalid")
	}
	graph := version.GraphSnapshot
	nodes := map[string]iapiserver.WorkflowCanvasNode{}
	parents := map[string][]string{}
	children := map[string][]string{}
	for _, node := range graph.Nodes {
		nodes[canvasNodeID(node)] = node
	}
	for _, edge := range graph.Edges {
		from, to := canvasEdgeSource(edge), canvasEdgeTarget(edge)
		parents[to] = append(parents[to], from)
		children[from] = append(children[from], to)
	}
	selected := map[string]struct{}{}
	selectedFlows := []iapiserver.WorkflowCanvasFlow{}
	if hasDuplicates(scope.FlowIDs) || hasDuplicates(scope.NodeIDs) {
		return nil, errors.NewStatus(code.ErrCanvasRunScopeInvalid, "run scope contains duplicate targets")
	}
	addClosure := func(start []string, adjacency map[string][]string) {
		queue := append([]string{}, start...)
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			if _, ok := nodes[id]; !ok {
				continue
			}
			if _, ok := selected[id]; ok {
				continue
			}
			selected[id] = struct{}{}
			queue = append(queue, adjacency[id]...)
		}
	}
	switch scope.Mode {
	case iapiserver.CanvasRunScopeAll:
		for id := range nodes {
			selected[id] = struct{}{}
		}
		selectedFlows = append(selectedFlows, graph.Flows...)
	case iapiserver.CanvasRunScopeFlows:
		if len(scope.FlowIDs) == 0 {
			return nil, errors.NewStatus(code.ErrCanvasRunScopeInvalid, "flow scope requires targets")
		}
		wanted := stringSet(scope.FlowIDs)
		for _, flow := range graph.Flows {
			if _, ok := wanted[flow.FlowID]; ok {
				selectedFlows = append(selectedFlows, flow)
				addClosure(flow.OutputNodeIDs, parents)
				delete(wanted, flow.FlowID)
			}
		}
		if len(wanted) > 0 {
			return nil, errors.NewStatus(code.ErrCanvasRunScopeInvalid, "flow scope contains an unknown flow")
		}
	case iapiserver.CanvasRunScopeOnlyNodes:
		if len(scope.NodeIDs) == 0 {
			return nil, errors.NewStatus(code.ErrCanvasRunScopeInvalid, "node scope requires targets")
		}
		for _, id := range scope.NodeIDs {
			if _, ok := nodes[id]; !ok {
				return nil, errors.NewStatus(code.ErrCanvasRunScopeInvalid, "node scope contains an unknown node")
			}
			selected[id] = struct{}{}
		}
	case iapiserver.CanvasRunScopeUntilNodes:
		if len(scope.NodeIDs) == 0 {
			return nil, errors.NewStatus(code.ErrCanvasRunScopeInvalid, "until scope requires targets")
		}
		addClosure(scope.NodeIDs, parents)
	case iapiserver.CanvasRunScopeFromNodes:
		if len(scope.NodeIDs) == 0 {
			return nil, errors.NewStatus(code.ErrCanvasRunScopeInvalid, "from scope requires targets")
		}
		addClosure(scope.NodeIDs, children)
	default:
		return nil, errors.NewStatus(code.ErrCanvasRunScopeInvalid, "run scope mode is invalid")
	}
	if len(selected) == 0 {
		return nil, errors.NewStatus(code.ErrCanvasRunScopeInvalid, "run scope is empty")
	}
	selectedGraph := iapiserver.WorkflowCanvasGraph{
		Nodes:    []iapiserver.WorkflowCanvasNode{},
		Edges:    []iapiserver.WorkflowCanvasEdge{},
		Flows:    selectedFlows,
		Groups:   graph.Groups,
		Viewport: graph.Viewport,
	}
	for _, node := range graph.Nodes {
		if _, ok := selected[canvasNodeID(node)]; ok {
			selectedGraph.Nodes = append(selectedGraph.Nodes, node)
		}
	}
	for _, edge := range graph.Edges {
		_, fromOK := selected[canvasEdgeSource(edge)]
		_, toOK := selected[canvasEdgeTarget(edge)]
		if fromOK && toOK {
			selectedGraph.Edges = append(selectedGraph.Edges, edge)
		}
	}
	executable := 0
	defs := definitionMap(version.DefinitionSnapshots)
	for _, node := range selectedGraph.Nodes {
		d := defs[node.NodeType+"@"+node.DefinitionVersion]
		if d == nil || d.ExecutionBinding.Mode != iapiserver.CanvasExecutionPassive {
			executable++
		}
	}
	flowKeys := map[string][]string{}
	for _, flow := range selectedFlows {
		flowSelected := map[string]struct{}{}
		queue := append([]string{}, flow.OutputNodeIDs...)
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			if _, ok := selected[id]; !ok {
				continue
			}
			if _, ok := flowSelected[id]; ok {
				continue
			}
			flowSelected[id] = struct{}{}
			queue = append(queue, parents[id]...)
		}
		keys := make([]string, 0, len(flowSelected))
		for id := range flowSelected {
			keys = append(keys, id)
		}
		sort.Strings(keys)
		flowKeys[flow.FlowID] = keys
	}
	snapshot := map[string]any{"graph": selectedGraph, "scope": scope, "run_policy": policy, "runtime_inputs": inputs, "flow_execution_keys": flowKeys}
	digest, err := digestValue(snapshot)
	if err != nil {
		return nil, err
	}
	return &executionPlan{
		Graph:             selectedGraph,
		Flows:             selectedFlows,
		FlowExecutionKeys: flowKeys,
		ExecutableTasks:   executable,
		Digest:            digest,
		Snapshot:          snapshot,
	}, nil
}

func definitionMap(definitions []*iapiserver.WorkflowNodeDefinition) map[string]*iapiserver.WorkflowNodeDefinition {
	result := make(map[string]*iapiserver.WorkflowNodeDefinition, len(definitions))
	for _, definition := range definitions {
		if definition != nil {
			result[definition.NodeType+"@"+definition.DefinitionVersion] = definition
		}
	}
	return result
}
func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, exists := result[value]; exists {
			continue
		}
		result[value] = struct{}{}
	}
	return result
}
func hasDuplicates(values []string) bool { return len(stringSet(values)) != len(values) }
func canvasNodeID(node iapiserver.WorkflowCanvasNode) string {
	if node.NodeID != "" {
		return node.NodeID
	}
	return node.NodeKey
}
func canvasEdgeSource(edge iapiserver.WorkflowCanvasEdge) string {
	if edge.SourceNodeID != "" {
		return edge.SourceNodeID
	}
	return edge.FromNodeKey
}
func canvasEdgeTarget(edge iapiserver.WorkflowCanvasEdge) string {
	if edge.TargetNodeID != "" {
		return edge.TargetNodeID
	}
	return edge.ToNodeKey
}
func canvasEdgeSourcePort(edge iapiserver.WorkflowCanvasEdge) string {
	if edge.SourcePortKey != "" {
		return edge.SourcePortKey
	}
	return edge.FromOutput
}
func canvasEdgeTargetPort(edge iapiserver.WorkflowCanvasEdge) string {
	if edge.TargetPortKey != "" {
		return edge.TargetPortKey
	}
	return edge.ToInput
}
func executionFingerprint(versionID string, node iapiserver.WorkflowCanvasNode, inputs map[string]any) (string, error) {
	return digestValue(map[string]any{"version": versionID, "node": node, "inputs": inputs})
}
func validateRetryIntent(req *iapiserver.WorkflowCanvasRunRetryRequest) error {
	switch req.Intent {
	case iapiserver.CanvasRetryNode, iapiserver.CanvasRetryFromNode:
		if len(req.NodeIDs) == 0 {
			return errors.NewStatus(code.ErrCanvasRetryTargetInvalid, "retry intent requires node targets")
		}
	case iapiserver.CanvasRetryFlow:
		if len(req.FlowIDs) == 0 {
			return errors.NewStatus(code.ErrCanvasRetryTargetInvalid, "retry intent requires flow targets")
		}
	case iapiserver.CanvasRetryFailed, iapiserver.CanvasRetryAll:
	default:
		return errors.NewStatus(code.ErrCanvasRetryTargetInvalid, "retry intent is invalid")
	}
	return nil
}
func actor(ctx context.Context) string {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err == nil && user != nil && user.ID != "" {
		return user.ID
	}
	return iapiserver.DefaultTaskCenterCreatedBy
}
func isRunTerminal(status string) bool {
	return status == iapiserver.CanvasRunStatusSuccess || status == iapiserver.CanvasRunStatusFailed || status == iapiserver.CanvasRunStatusCanceled ||
		status == iapiserver.CanvasRunStatusTimeout
}

var _ Service = (*service)(nil)
