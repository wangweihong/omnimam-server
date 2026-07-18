package workflowcanvas

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/gowebpki/jcs"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	taskcentersvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
)

type Service interface {
	List(context.Context, *iapiserver.WorkflowCanvasListRequest) (*iapiserver.WorkflowCanvasListResponse, error)
	Create(context.Context, *iapiserver.WorkflowCanvasCreateRequest) (*iapiserver.WorkflowCanvas, error)
	Get(context.Context, string) (*iapiserver.WorkflowCanvas, error)
	Update(context.Context, *iapiserver.WorkflowCanvasUpdateRequest) (*iapiserver.WorkflowCanvas, error)
	Delete(context.Context, string) error
	Publish(context.Context, string, *iapiserver.WorkflowCanvasPublishRequest) (*iapiserver.CanvasVersion, error)
	ListVersions(context.Context, *iapiserver.CanvasVersionListRequest) (*iapiserver.CanvasVersionListResponse, error)
	GetVersion(context.Context, string) (*iapiserver.CanvasVersion, error)
	ListRuns(context.Context, *iapiserver.WorkflowCanvasRunListRequest) (*iapiserver.WorkflowCanvasRunListResponse, error)
	CreateRun(context.Context, *iapiserver.WorkflowCanvasRunCreateRequest) (*iapiserver.WorkflowCanvasRun, error)
	GetRun(context.Context, string) (*iapiserver.WorkflowCanvasRun, error)
	ListNodeRuns(context.Context, *iapiserver.CanvasNodeRunListRequest) (*iapiserver.CanvasNodeRunListResponse, error)
	CancelRun(context.Context, string) (*iapiserver.WorkflowCanvasRun, error)
	RetryRun(context.Context, string, *iapiserver.WorkflowCanvasRunRetryRequest) (*iapiserver.WorkflowCanvasRun, error)
}

type service struct {
	factory store.Factory
	store   store.WorkflowCanvasStore
	tasks   taskcentersvc.TaskCenterSrv
}

func New(factory store.Factory, tasks taskcentersvc.TaskCenterSrv) Service {
	return &service{factory: factory, store: factory.WorkflowCanvases(), tasks: tasks}
}

func (s *service) List(ctx context.Context, req *iapiserver.WorkflowCanvasListRequest) (*iapiserver.WorkflowCanvasListResponse, error) {
	user := actor(ctx)
	items, total, err := s.store.ListWorkflowCanvases(ctx, req, iapiserver.DefaultTaskCenterProjectID, iapiserver.DefaultTaskCenterNamespace, user)
	if err != nil {
		return nil, err
	}
	return &iapiserver.WorkflowCanvasListResponse{Total: total, Items: items}, nil
}
func (s *service) Create(ctx context.Context, req *iapiserver.WorkflowCanvasCreateRequest) (*iapiserver.WorkflowCanvas, error) {
	graph := req.DraftGraph
	if graph.Nodes == nil {
		graph.Nodes = []iapiserver.WorkflowCanvasNode{}
	}
	if graph.Edges == nil {
		graph.Edges = []iapiserver.WorkflowCanvasEdge{}
	}
	if err := validateGraph(graph, false); err != nil {
		return nil, err
	}
	item := &iapiserver.WorkflowCanvas{Visibility: req.Visibility, DraftGraph: graph, DraftRevision: 1, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace, CreatedBy: actor(ctx)}
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

func (s *service) Publish(ctx context.Context, id string, req *iapiserver.WorkflowCanvasPublishRequest) (*iapiserver.CanvasVersion, error) {
	canvas, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if canvas.DraftRevision != req.DraftRevision {
		return nil, errors.NewStatus(code.ErrCanvasRevisionConflict, "canvas draft revision changed")
	}
	if err := validateGraph(canvas.DraftGraph, true); err != nil {
		return nil, err
	}
	dagReq, err := s.dagRequest(ctx, canvas.DraftGraph, canvas.ProjectID, canvas.Namespace)
	if err != nil {
		return nil, err
	}
	next := canvas.LatestVersion + 1
	definitionName := "canvas_" + strings.ReplaceAll(canvas.ID, "-", "")
	binding, err := s.tasks.RegisterDAGDefinition(ctx, definitionName, next, dagReq)
	if err != nil {
		return nil, errors.NewStatus(code.ErrCanvasPublishFailed, err.Error())
	}
	digest, err := graphDigest(canvas.DraftGraph)
	if err != nil {
		return nil, errors.NewStatus(code.ErrCanvasPublishFailed, err.Error())
	}
	version := &iapiserver.CanvasVersion{CanvasID: canvas.ID, GraphSnapshot: canvas.DraftGraph, InputSchema: map[string]any{}, OutputSchema: map[string]any{}, ContentDigest: digest, CompiledDefinitionName: binding.Name, CompiledDefinitionVersion: binding.Version, NodeCount: len(canvas.DraftGraph.Nodes), EdgeCount: len(canvas.DraftGraph.Edges), PublishedBy: actor(ctx), PublishedAt: imachinery.Now()}
	version.ID = uuid.NewString()
	version.Name = fmt.Sprintf("%s v%d", canvas.Name, next)
	return s.store.PublishWorkflowCanvas(ctx, canvas, version, req.DraftRevision)
}
func (s *service) ListVersions(ctx context.Context, req *iapiserver.CanvasVersionListRequest) (*iapiserver.CanvasVersionListResponse, error) {
	if _, err := s.Get(ctx, req.CanvasID); err != nil {
		return nil, err
	}
	items, total, err := s.store.ListCanvasVersions(ctx, req)
	if err != nil {
		return nil, err
	}
	return &iapiserver.CanvasVersionListResponse{Total: total, Items: items}, nil
}
func (s *service) GetVersion(ctx context.Context, id string) (*iapiserver.CanvasVersion, error) {
	v, err := s.store.GetCanvasVersion(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := s.Get(ctx, v.CanvasID); err != nil {
		return nil, err
	}
	return v, nil
}
func (s *service) ListRuns(ctx context.Context, req *iapiserver.WorkflowCanvasRunListRequest) (*iapiserver.WorkflowCanvasRunListResponse, error) {
	items, total, err := s.store.ListWorkflowCanvasRuns(ctx, req, iapiserver.DefaultTaskCenterProjectID, iapiserver.DefaultTaskCenterNamespace, actor(ctx))
	if err != nil {
		return nil, err
	}
	return &iapiserver.WorkflowCanvasRunListResponse{Total: total, Items: items}, nil
}

func (s *service) CreateRun(ctx context.Context, req *iapiserver.WorkflowCanvasRunCreateRequest) (*iapiserver.WorkflowCanvasRun, error) {
	version, err := s.GetVersion(ctx, req.CanvasVersionID)
	if err != nil {
		return nil, err
	}
	digest, err := requestDigest(req.CanvasVersionID, req.Input)
	if err != nil {
		return nil, err
	}
	run := &iapiserver.WorkflowCanvasRun{CanvasID: version.CanvasID, CanvasVersionID: version.ID, IdempotencyKey: req.IdempotencyKey, RequestDigest: digest, InputSnapshot: req.Input, TaskCreationStatus: iapiserver.CanvasTaskCreationPending, Status: iapiserver.CanvasRunStatusPending, Progress: 0, Summary: map[string]any{}, Output: map[string]any{}, LastError: map[string]any{}, ProjectID: iapiserver.DefaultTaskCenterProjectID, Namespace: iapiserver.DefaultTaskCenterNamespace, CreatedBy: actor(ctx)}
	run.ID = uuid.NewString()
	run.Name = "Canvas run"
	created, isNew, err := s.store.AddWorkflowCanvasRunIdempotent(ctx, run)
	if err != nil || !isNew {
		return created, err
	}
	dagReq, err := s.dagRequest(ctx, version.GraphSnapshot, run.ProjectID, run.Namespace)
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
	nodeRuns := make([]*iapiserver.CanvasNodeRun, 0, len(version.GraphSnapshot.Nodes))
	for _, node := range version.GraphSnapshot.Nodes {
		nr := &iapiserver.CanvasNodeRun{CanvasRunID: run.ID, NodeKey: node.NodeKey, NodeType: node.NodeType, Status: iapiserver.AtomicTaskStatusBlocked, Output: map[string]any{}, LastError: map[string]any{}}
		nr.ID = uuid.NewString()
		nr.Name = node.NodeKey
		if task := byKey[node.NodeKey]; task != nil {
			nr.AtomicTaskID = &task.ID
			nr.Status = task.Status
			task.CanvasRunID = run.ID
			task.CanvasNodeRunID = nr.ID
			if _, updateErr := s.factory.TaskCenters().UpdateAtomicTask(ctx, task); updateErr != nil {
				return s.failRun(ctx, created, updateErr)
			}
		}
		nodeRuns = append(nodeRuns, nr)
	}
	return s.store.BindWorkflowCanvasRun(ctx, run.ID, group.ID, nodeRuns)
}
func (s *service) failRun(ctx context.Context, run *iapiserver.WorkflowCanvasRun, cause error) (*iapiserver.WorkflowCanvasRun, error) {
	run.TaskCreationStatus = iapiserver.CanvasTaskCreationFailed
	run.Status = iapiserver.CanvasRunStatusFailed
	run.LastError = map[string]any{"message": cause.Error()}
	updated, err := s.store.UpdateWorkflowCanvasRun(ctx, run)
	if err != nil {
		return nil, err
	}
	return updated, cause
}
func (s *service) GetRun(ctx context.Context, id string) (*iapiserver.WorkflowCanvasRun, error) {
	run, err := s.store.GetWorkflowCanvasRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if run.CreatedBy != actor(ctx) {
		return nil, errors.NewStatus(code.ErrCanvasRunNotFound, "canvas run not found")
	}
	return run, nil
}
func (s *service) ListNodeRuns(ctx context.Context, req *iapiserver.CanvasNodeRunListRequest) (*iapiserver.CanvasNodeRunListResponse, error) {
	if _, err := s.GetRun(ctx, req.CanvasRunID); err != nil {
		return nil, err
	}
	items, total, err := s.store.ListCanvasNodeRuns(ctx, req)
	if err != nil {
		return nil, err
	}
	return &iapiserver.CanvasNodeRunListResponse{Total: total, Items: items}, nil
}
func (s *service) CancelRun(ctx context.Context, id string) (*iapiserver.WorkflowCanvasRun, error) {
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
	return s.store.UpdateWorkflowCanvasRun(ctx, run)
}
func (s *service) RetryRun(ctx context.Context, id string, req *iapiserver.WorkflowCanvasRunRetryRequest) (*iapiserver.WorkflowCanvasRun, error) {
	source, err := s.GetRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if !isRunTerminal(source.Status) {
		return nil, errors.NewStatus(code.ErrCanvasRunStateBlocked, "canvas run cannot be retried")
	}
	created, err := s.CreateRun(ctx, &iapiserver.WorkflowCanvasRunCreateRequest{CanvasVersionID: source.CanvasVersionID, IdempotencyKey: req.IdempotencyKey, Input: source.InputSnapshot})
	if err == nil {
		created.RetryOfCanvasRunID = &source.ID
		created, err = s.store.UpdateWorkflowCanvasRun(ctx, created)
	}
	return created, err
}

func (s *service) dagRequest(ctx context.Context, graph iapiserver.WorkflowCanvasGraph, projectID, namespace string) (*iapiserver.DAGTaskGroupCreateRequest, error) {
	nodes := make([]iapiserver.DAGNode, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		ref, _ := node.Config["function_ref"].(string)
		args := map[string]any{"config": node.Config, "input_bindings": node.InputBindings}
		if node.NodeType == iapiserver.CanvasNodeTypeApplication {
			versionID, _ := node.Config["application_version_id"].(string)
			if versionID == "" {
				return nil, errors.NewStatusF(code.ErrCanvasNodeReferenceInvalid, "node %s has no application_version_id", node.NodeKey)
			}
			version, versionErr := s.factory.ApplicationPlatforms().GetApplicationVersion(ctx, versionID)
			if versionErr != nil || version.Status != iapiserver.VersionStatusPublished {
				return nil, errors.NewStatusF(code.ErrCanvasNodeReferenceInvalid, "node %s references an unavailable ApplicationVersion", node.NodeKey)
			}
			ref = "application-platform.run"
		}
		if ref == "" {
			return nil, errors.NewStatusF(code.ErrCanvasNodeReferenceInvalid, "node %s has no registered function_ref", node.NodeKey)
		}
		maxDynamic := 0
		if node.MaxDynamicTasks != nil {
			maxDynamic = *node.MaxDynamicTasks
		}
		nodes = append(nodes, iapiserver.DAGNode{Key: node.NodeKey, Task: iapiserver.AtomicTaskTemplate{Key: node.NodeKey, Name: node.NodeKey, FunctionRef: ref, Arguments: args}, InputMapping: node.InputBindings, DynamicFork: node.NodeType == iapiserver.CanvasNodeTypeDynamicFork, MaxDynamicTasks: maxDynamic})
	}
	edges := make([]iapiserver.DAGEdge, 0, len(graph.Edges))
	for _, edge := range graph.Edges {
		edges = append(edges, iapiserver.DAGEdge{FromNode: edge.FromNodeKey, ToNode: edge.ToNodeKey, DataMapping: map[string]any{edge.ToInput: edge.FromNodeKey + "." + edge.FromOutput}})
	}
	return &iapiserver.DAGTaskGroupCreateRequest{Name: "Canvas definition", Nodes: nodes, Edges: edges, Input: map[string]any{}, OutputMapping: map[string]any{}, ProjectID: projectID, Namespace: namespace}, nil
}

var nodeKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,99}$`)

func validateGraph(graph iapiserver.WorkflowCanvasGraph, requireNodes bool) error {
	if len(graph.Nodes) > iapiserver.MaxTaskGraphNodes || len(graph.Edges) > iapiserver.MaxTaskGraphEdges {
		return errors.NewStatus(code.ErrCanvasLimitExceeded, "canvas graph limit exceeded")
	}
	if requireNodes && len(graph.Nodes) == 0 {
		return errors.NewStatus(code.ErrCanvasGraphInvalid, "published canvas requires at least one node")
	}
	keys := map[string]struct{}{}
	indegree := map[string]int{}
	next := map[string][]string{}
	for _, node := range graph.Nodes {
		if !nodeKeyPattern.MatchString(node.NodeKey) {
			return errors.NewStatus(code.ErrCanvasGraphInvalid, "canvas node key is invalid")
		}
		if _, ok := keys[node.NodeKey]; ok {
			return errors.NewStatus(code.ErrCanvasGraphInvalid, "canvas node key is duplicated")
		}
		if node.NodeType != iapiserver.CanvasNodeTypeApplication && node.NodeType != iapiserver.CanvasNodeTypeFunction && node.NodeType != iapiserver.CanvasNodeTypeDynamicFork {
			return errors.NewStatus(code.ErrCanvasGraphInvalid, "canvas node type is invalid")
		}
		if containsUnsafeCanvasConfig(node.Config) {
			return errors.NewStatus(code.ErrCanvasGraphInvalid, "canvas node config contains forbidden runtime fields")
		}
		if node.NodeType == iapiserver.CanvasNodeTypeDynamicFork && (node.MaxDynamicTasks == nil || *node.MaxDynamicTasks < 1 || *node.MaxDynamicTasks > iapiserver.MaxDynamicForkTasks) {
			return errors.NewStatus(code.ErrCanvasLimitExceeded, "dynamic fork limit is invalid")
		}
		keys[node.NodeKey] = struct{}{}
		indegree[node.NodeKey] = 0
	}
	for _, edge := range graph.Edges {
		if _, ok := keys[edge.FromNodeKey]; !ok {
			return errors.NewStatus(code.ErrCanvasGraphInvalid, "edge source is missing")
		}
		if _, ok := keys[edge.ToNodeKey]; !ok {
			return errors.NewStatus(code.ErrCanvasGraphInvalid, "edge target is missing")
		}
		if edge.FromNodeKey == edge.ToNodeKey {
			return errors.NewStatus(code.ErrCanvasCycleDetected, "canvas graph contains a cycle")
		}
		next[edge.FromNodeKey] = append(next[edge.FromNodeKey], edge.ToNodeKey)
		indegree[edge.ToNodeKey]++
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
		return errors.NewStatus(code.ErrCanvasCycleDetected, "canvas graph contains a cycle")
	}
	return nil
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
func requestDigest(version string, input map[string]any) (string, error) {
	return graphDigest(iapiserver.WorkflowCanvasGraph{Nodes: []iapiserver.WorkflowCanvasNode{{NodeKey: "request", NodeType: iapiserver.CanvasNodeTypeFunction, Config: map[string]any{"version": version, "input": input}, InputBindings: map[string]any{}}}, Edges: []iapiserver.WorkflowCanvasEdge{}})
}
func actor(ctx context.Context) string {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err == nil && user != nil && user.ID != "" {
		return user.ID
	}
	return iapiserver.DefaultTaskCenterCreatedBy
}
func isRunTerminal(status string) bool {
	return status == iapiserver.CanvasRunStatusSuccess || status == iapiserver.CanvasRunStatusFailed || status == iapiserver.CanvasRunStatusCanceled || status == iapiserver.CanvasRunStatusTimeout
}

var _ Service = (*service)(nil)
