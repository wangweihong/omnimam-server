package taskcenter

import (
	"context"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type TaskCenterSrv interface {
	ListDefinitions(ctx context.Context, req *iapiserver.TaskDefinitionListRequest) (*iapiserver.TaskDefinitionListResponse, error)
	CreateAtomicTask(ctx context.Context, req *iapiserver.AtomicTaskCreateRequest) (*iapiserver.AtomicTask, error)
	CreateTaskGroup(ctx context.Context, req *iapiserver.TaskGroupCreateRequest) (*iapiserver.TaskGroup, error)
	CreateDAGFlowTask(ctx context.Context, req *iapiserver.DAGFlowTaskCreateRequest) (*iapiserver.DAGFlowTask, error)
	ListRuns(ctx context.Context, req *iapiserver.TaskRunListRequest) (*iapiserver.TaskRunListResponse, error)
	CreateRun(ctx context.Context, req *iapiserver.TaskRunCreateRequest) (*iapiserver.TaskRun, error)
	GetRun(ctx context.Context, id string) (*iapiserver.TaskRun, error)
	DeleteRun(ctx context.Context, id string) (*iapiserver.SuccessResponse, error)
	ListAttempts(ctx context.Context, req *iapiserver.TaskAttemptListRequest) (*iapiserver.TaskAttemptListResponse, error)
	CancelRun(ctx context.Context, req *iapiserver.CancelTaskRunRequest) (*iapiserver.TaskRun, error)
	RetryRun(ctx context.Context, req *iapiserver.RetryTaskRunRequest) (*iapiserver.TaskRun, error)
	RegisterWorker(ctx context.Context, req *iapiserver.WorkerRegisterRequest) (*iapiserver.Worker, error)
	HeartbeatWorker(ctx context.Context, req *iapiserver.WorkerHeartbeatRequest) (*iapiserver.Worker, error)
	ClaimRun(ctx context.Context, req *iapiserver.ClaimTaskRunRequest) (*iapiserver.ClaimTaskRunResponse, error)
	UpdateProgress(ctx context.Context, req *iapiserver.ProgressUpdateRequest) (*iapiserver.TaskRun, error)
	CompleteRun(ctx context.Context, req *iapiserver.TaskRunCompleteRequest) (*iapiserver.TaskRun, error)
	FailRun(ctx context.Context, req *iapiserver.TaskRunFailRequest) (*iapiserver.TaskRun, error)
	RenewLease(ctx context.Context, req *iapiserver.LeaseRenewRequest) (*iapiserver.ExecutionLease, error)
	Health(ctx context.Context) (*iapiserver.TaskCenterHealth, error)
}

type taskCenterService struct {
	store store.Factory
}

func NewService(storeIns store.Factory) TaskCenterSrv {
	return &taskCenterService{store: storeIns}
}

func (s *taskCenterService) ListDefinitions(
	ctx context.Context,
	req *iapiserver.TaskDefinitionListRequest,
) (*iapiserver.TaskDefinitionListResponse, error) {
	items, total, err := s.store.TaskCenters().ListDefinitions(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.TaskDefinitionListResponse{Total: total, Items: items}, nil
}

// CreateAtomicTask 创建 AtomicTask 定义，只保存任务中心元数据，不执行具体 AppEngine 能力。
func (s *taskCenterService) CreateAtomicTask(
	ctx context.Context,
	req *iapiserver.AtomicTaskCreateRequest,
) (*iapiserver.AtomicTask, error) {
	if err := validateRetryPolicy(req.RetryPolicy, req.TimeoutPolicy); err != nil {
		return nil, err
	}
	definition := &iapiserver.TaskDefinition{
		DefinitionType:       iapiserver.TaskDefinitionTypeAtomic,
		FunctionRef:          req.FunctionRef,
		AppID:                req.AppID,
		EngineRef:            req.EngineRef,
		DefaultArguments:     req.DefaultArguments,
		TimeoutPolicy:        req.TimeoutPolicy,
		RetryPolicy:          req.RetryPolicy,
		CancelPolicy:         req.CancelPolicy,
		RequiredCapabilities: req.RequiredCapabilities,
		Tags:                 req.Tags,
		ProjectID:            req.ProjectID,
		Namespace:            req.Namespace,
		CreatedBy:            req.CreatedBy,
	}
	definition.Name = req.Name
	definition.Description = req.Description
	ret, err := s.store.TaskCenters().AddDefinition(ctx, definition)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return ret, nil
}

// CreateTaskGroup 创建 SERIAL/PARALLEL 任务组定义；不展开执行子任务。
func (s *taskCenterService) CreateTaskGroup(
	ctx context.Context,
	req *iapiserver.TaskGroupCreateRequest,
) (*iapiserver.TaskGroup, error) {
	if err := validateRetryPolicy(req.RetryPolicy, req.TimeoutPolicy); err != nil {
		return nil, err
	}
	if req.GroupType != iapiserver.TaskGroupTypeSerial && req.GroupType != iapiserver.TaskGroupTypeParallel {
		return nil, errors.NewStatusF(code.ErrTaskDefinitionInvalid, "task group type invalid")
	}
	if len(req.Children) == 0 {
		return nil, errors.NewStatusF(code.ErrTaskDefinitionInvalid, "task group children empty")
	}
	definition := &iapiserver.TaskDefinition{
		DefinitionType: iapiserver.TaskDefinitionTypeGroup,
		GroupType:      req.GroupType,
		Children:       req.Children,
		StrategyConfig: req.StrategyConfig,
		TimeoutPolicy:  req.TimeoutPolicy,
		RetryPolicy:    req.RetryPolicy,
		Tags:           req.Tags,
		ProjectID:      req.ProjectID,
		Namespace:      req.Namespace,
		CreatedBy:      req.CreatedBy,
	}
	definition.Name = req.Name
	definition.Description = req.Description
	ret, err := s.store.TaskCenters().AddDefinition(ctx, definition)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return ret, nil
}

// CreateDAGFlowTask 创建 DAGFlowTask 定义，并在保存前校验节点依赖无环。
func (s *taskCenterService) CreateDAGFlowTask(
	ctx context.Context,
	req *iapiserver.DAGFlowTaskCreateRequest,
) (*iapiserver.DAGFlowTask, error) {
	if err := validateRetryPolicy(req.RetryPolicy, req.TimeoutPolicy); err != nil {
		return nil, err
	}
	if err := validateDAG(req.Nodes, req.Edges); err != nil {
		return nil, err
	}
	definition := &iapiserver.TaskDefinition{
		DefinitionType: iapiserver.TaskDefinitionTypeDAGFlow,
		DAGNodes:       req.Nodes,
		DAGEdges:       req.Edges,
		InputMapping:   req.InputMapping,
		OutputMapping:  req.OutputMapping,
		StrategyConfig: req.StrategyConfig,
		TimeoutPolicy:  req.TimeoutPolicy,
		RetryPolicy:    req.RetryPolicy,
		Tags:           req.Tags,
		ProjectID:      req.ProjectID,
		Namespace:      req.Namespace,
		CreatedBy:      req.CreatedBy,
	}
	definition.Name = req.Name
	definition.Description = req.Description
	ret, err := s.store.TaskCenters().AddDefinition(ctx, definition)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return ret, nil
}

func (s *taskCenterService) ListRuns(
	ctx context.Context,
	req *iapiserver.TaskRunListRequest,
) (*iapiserver.TaskRunListResponse, error) {
	items, total, err := s.store.TaskCenters().ListRuns(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.TaskRunListResponse{Total: total, Items: items}, nil
}

// CreateRun 创建一次 TaskRun；具体业务执行由后续 Worker protocol 和 AppEngine 完成。
func (s *taskCenterService) CreateRun(
	ctx context.Context,
	req *iapiserver.TaskRunCreateRequest,
) (*iapiserver.TaskRun, error) {
	definition, err := s.store.TaskCenters().GetDefinition(ctx, req.DefinitionType, req.DefinitionID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	timeoutPolicy := req.TimeoutPolicy
	if timeoutPolicy == (iapiserver.TimeoutPolicy{}) {
		timeoutPolicy = definition.TimeoutPolicy
	}
	retryPolicy := req.RetryPolicy
	if retryPolicy == (iapiserver.RetryPolicy{}) {
		retryPolicy = definition.RetryPolicy
	}
	if err := validateRetryPolicy(retryPolicy, timeoutPolicy); err != nil {
		return nil, err
	}
	run := &iapiserver.TaskRun{
		DefinitionType: req.DefinitionType,
		DefinitionID:   req.DefinitionID,
		ParentRunID:    req.ParentRunID,
		RootRunID:      req.RootRunID,
		ScheduleAt:     req.ScheduleAt,
		Input:          req.Input,
		TimeoutAt:      timeoutAt(req.ScheduleAt, timeoutPolicy),
		MaxAttempts:    retryPolicyMaxAttempts(retryPolicy),
		ProjectID:      fallbackString(req.ProjectID, definition.ProjectID),
		Namespace:      fallbackString(req.Namespace, definition.Namespace),
		Tags:           req.Tags,
		CreatedBy:      req.CreatedBy,
	}
	run.Name = definition.Name + "-run"
	if !req.ScheduleAt.IsZero() && req.ScheduleAt.Time.After(time.Now()) {
		run.Status = iapiserver.TaskRunStatusPending
	} else {
		run.Status = iapiserver.TaskRunStatusReady
	}
	ret, err := s.store.TaskCenters().AddRun(ctx, run)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	s.recordTaskRunEvent(ctx, newTaskRunEvent(ret, iapiserver.TaskCenterEventRunCreated, "", ret.Status))
	return ret, nil
}

func (s *taskCenterService) GetRun(ctx context.Context, id string) (*iapiserver.TaskRun, error) {
	return s.store.TaskCenters().GetRun(ctx, id)
}

func (s *taskCenterService) DeleteRun(ctx context.Context, id string) (*iapiserver.SuccessResponse, error) {
	run, err := s.store.TaskCenters().GetRun(ctx, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if !isTerminalRunStatus(run.Status) {
		return nil, errors.NewStatusF(code.ErrTaskRunStateBlocked, "task run is not terminal")
	}
	if err := s.store.TaskCenters().SoftDeleteRun(ctx, id); err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.SuccessResponse{Success: true}, nil
}

func (s *taskCenterService) ListAttempts(
	ctx context.Context,
	req *iapiserver.TaskAttemptListRequest,
) (*iapiserver.TaskAttemptListResponse, error) {
	items, total, err := s.store.TaskCenters().ListAttempts(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.TaskAttemptListResponse{Total: total, Items: items}, nil
}

func (s *taskCenterService) CancelRun(
	ctx context.Context,
	req *iapiserver.CancelTaskRunRequest,
) (*iapiserver.TaskRun, error) {
	run, err := s.store.TaskCenters().GetRun(ctx, req.RunID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if isTerminalRunStatus(run.Status) {
		return nil, errors.NewStatusF(code.ErrTaskRunStateBlocked, "terminal task run cannot be canceled")
	}
	from := run.Status
	run.Status = iapiserver.TaskRunStatusCancelRequested
	run.CanceledAt = imachinery.NewTime(time.Now())
	ret, err := s.store.TaskCenters().UpdateRun(ctx, run)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	s.recordTaskRunEvent(ctx, newTaskRunEvent(ret, iapiserver.TaskCenterEventStatusChanged, from, ret.Status))
	return ret, nil
}

func (s *taskCenterService) RetryRun(
	ctx context.Context,
	req *iapiserver.RetryTaskRunRequest,
) (*iapiserver.TaskRun, error) {
	run, err := s.store.TaskCenters().GetRun(ctx, req.RunID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if run.Status != iapiserver.TaskRunStatusFailed &&
		run.Status != iapiserver.TaskRunStatusTimeout &&
		run.Status != iapiserver.TaskRunStatusLost {
		return nil, errors.NewStatusF(code.ErrTaskRunStateBlocked, "task run cannot be retried")
	}
	from := run.Status
	run.Status = iapiserver.TaskRunStatusReady
	run.Progress = 0
	run.CompletedAt = imachinery.Time{}
	ret, err := s.store.TaskCenters().UpdateRun(ctx, run)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	s.recordTaskRunEvent(ctx, newTaskRunEvent(ret, iapiserver.TaskCenterEventStatusChanged, from, ret.Status))
	return ret, nil
}

// RegisterWorker 注册 Worker 进程能力声明；不会执行具体业务任务。
func (s *taskCenterService) RegisterWorker(
	ctx context.Context,
	req *iapiserver.WorkerRegisterRequest,
) (*iapiserver.Worker, error) {
	worker := &iapiserver.Worker{
		WorkerType:     req.WorkerType,
		Capabilities:   req.Capabilities,
		Labels:         req.Labels,
		MaxConcurrency: req.MaxConcurrency,
		Status:         iapiserver.WorkerStatusOnline,
	}
	worker.Name = req.WorkerType
	return s.store.TaskCenters().RegisterWorker(ctx, worker)
}

func (s *taskCenterService) HeartbeatWorker(
	ctx context.Context,
	req *iapiserver.WorkerHeartbeatRequest,
) (*iapiserver.Worker, error) {
	return s.store.TaskCenters().HeartbeatWorker(ctx, req)
}

func (s *taskCenterService) ClaimRun(
	ctx context.Context,
	req *iapiserver.ClaimTaskRunRequest,
) (*iapiserver.ClaimTaskRunResponse, error) {
	return s.store.TaskCenters().ClaimRun(ctx, req)
}

func (s *taskCenterService) UpdateProgress(
	ctx context.Context,
	req *iapiserver.ProgressUpdateRequest,
) (*iapiserver.TaskRun, error) {
	return s.store.TaskCenters().UpdateProgress(ctx, req)
}

func (s *taskCenterService) CompleteRun(
	ctx context.Context,
	req *iapiserver.TaskRunCompleteRequest,
) (*iapiserver.TaskRun, error) {
	return s.store.TaskCenters().CompleteRun(ctx, req)
}

func (s *taskCenterService) FailRun(
	ctx context.Context,
	req *iapiserver.TaskRunFailRequest,
) (*iapiserver.TaskRun, error) {
	return s.store.TaskCenters().FailRun(ctx, req)
}

func (s *taskCenterService) RenewLease(
	ctx context.Context,
	req *iapiserver.LeaseRenewRequest,
) (*iapiserver.ExecutionLease, error) {
	return s.store.TaskCenters().RenewLease(ctx, req)
}

func (s *taskCenterService) Health(ctx context.Context) (*iapiserver.TaskCenterHealth, error) {
	return s.store.TaskCenters().Health(ctx)
}

func validateDAG(nodes []iapiserver.DAGNode, edges []iapiserver.DAGEdge) error {
	if len(nodes) == 0 {
		return errors.NewStatusF(code.ErrTaskDefinitionInvalid, "dag nodes empty")
	}
	graph := make(map[string][]string, len(nodes))
	visiting := make(map[string]bool, len(nodes))
	visited := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		nodeID := strings.TrimSpace(node.NodeID)
		if nodeID == "" {
			return errors.NewStatusF(code.ErrTaskDefinitionInvalid, "dag node id empty")
		}
		if _, ok := graph[nodeID]; ok {
			return errors.NewStatusF(code.ErrTaskDefinitionInvalid, "dag node id duplicated")
		}
		graph[nodeID] = nil
	}
	for _, edge := range edges {
		if _, ok := graph[edge.FromNodeID]; !ok {
			return errors.NewStatusF(code.ErrTaskDefinitionInvalid, "dag edge from node missing")
		}
		if _, ok := graph[edge.ToNodeID]; !ok {
			return errors.NewStatusF(code.ErrTaskDefinitionInvalid, "dag edge to node missing")
		}
		graph[edge.FromNodeID] = append(graph[edge.FromNodeID], edge.ToNodeID)
	}
	var visit func(string) bool
	visit = func(nodeID string) bool {
		if visiting[nodeID] {
			return true
		}
		if visited[nodeID] {
			return false
		}
		visiting[nodeID] = true
		for _, next := range graph[nodeID] {
			if visit(next) {
				return true
			}
		}
		visiting[nodeID] = false
		visited[nodeID] = true
		return false
	}
	for nodeID := range graph {
		if visit(nodeID) {
			return errors.NewStatusF(code.ErrTaskDAGCycleDetected, "dag flow task contains cycle")
		}
	}
	return nil
}

func isTerminalRunStatus(status string) bool {
	switch status {
	case iapiserver.TaskRunStatusSuccess,
		iapiserver.TaskRunStatusFailed,
		iapiserver.TaskRunStatusCanceled,
		iapiserver.TaskRunStatusTimeout,
		iapiserver.TaskRunStatusLost:
		return true
	default:
		return false
	}
}

func fallbackString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func retryPolicyMaxAttempts(policy iapiserver.RetryPolicy) int {
	if policy.MaxRetries < 0 {
		return -1
	}
	return policy.MaxRetries + 1
}

func validateRetryPolicy(policy iapiserver.RetryPolicy, timeoutPolicy iapiserver.TimeoutPolicy) error {
	if policy.MaxRetries >= 0 {
		return nil
	}
	if timeoutPolicy.OverallTimeout != "" || policy.MaxRetryDuration != "" {
		return nil
	}
	return errors.NewStatusF(code.ErrTaskRetryPolicyInvalid, "infinite retry requires exit protection")
}

func timeoutAt(scheduleAt imachinery.Time, policy iapiserver.TimeoutPolicy) imachinery.Time {
	if policy.OverallTimeout == "" {
		return imachinery.Time{}
	}
	duration, err := time.ParseDuration(policy.OverallTimeout)
	if err != nil {
		return imachinery.Time{}
	}
	base := time.Now()
	if !scheduleAt.IsZero() {
		base = scheduleAt.Time
	}
	return imachinery.NewTime(base.Add(duration))
}

func newTaskRunEvent(run *iapiserver.TaskRun, eventType, from, to string) *iapiserver.TaskRunEvent {
	event := &iapiserver.TaskRunEvent{
		RunID:      run.ID,
		EventType:  eventType,
		FromStatus: from,
		ToStatus:   to,
		Payload: map[string]any{
			"run_id":          run.ID,
			"definition_type": run.DefinitionType,
			"definition_id":   run.DefinitionID,
			"status":          run.Status,
			"project_id":      run.ProjectID,
			"namespace":       run.Namespace,
			"created_by":      run.CreatedBy,
		},
		OccurredAt: imachinery.NewTime(time.Now()),
	}
	event.Name = eventType
	return event
}

func (s *taskCenterService) recordTaskRunEvent(ctx context.Context, event *iapiserver.TaskRunEvent) {
	if _, err := s.store.TaskCenters().AddEvent(ctx, event); err != nil {
		log.Errorf("task center event record failed: run_id=%s event_type=%s err=%v", event.RunID, event.EventType, err)
	}
}
