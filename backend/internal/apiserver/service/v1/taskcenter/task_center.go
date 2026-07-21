package taskcenter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/sliceutil"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
)

type TaskCenterSrv interface {
	ListAtomicTasks(context.Context, *iapiserver.AtomicTaskListRequest) (*iapiserver.AtomicTaskListResponse, error)
	CreateAtomicTask(context.Context, *iapiserver.AtomicTaskCreateRequest) (*iapiserver.AtomicTask, error)
	GetAtomicTask(context.Context, string) (*iapiserver.AtomicTask, error)
	// GetAtomicTaskSummaries 批量返回当前主体可见的 AtomicTask 一跳摘要，供跨领域只读组合响应。
	GetAtomicTaskSummaries(context.Context, []string) (map[string]*iapiserver.AtomicTaskSummary, error)
	ListAttempts(context.Context, *iapiserver.TaskAttemptListRequest) (*iapiserver.TaskAttemptListResponse, error)
	CancelAtomicTask(context.Context, string, *iapiserver.ActionReasonRequest) (*iapiserver.AtomicTask, error)
	RetryAtomicTask(context.Context, string, *iapiserver.ActionReasonRequest) (*iapiserver.AtomicTask, error)
	ListTaskGroups(context.Context, *iapiserver.TaskGroupListRequest) (*iapiserver.TaskGroupListResponse, error)
	CreateTaskGroup(context.Context, *iapiserver.TaskGroupCreateRequest) (*iapiserver.TaskGroup, error)
	GetTaskGroup(context.Context, string) (*iapiserver.TaskGroup, error)
	ListTaskGroupTasks(context.Context, string, *iapiserver.AtomicTaskListRequest) (*iapiserver.AtomicTaskListResponse, error)
	CancelTaskGroup(context.Context, string) (*iapiserver.TaskGroup, error)
	RetryTaskGroup(context.Context, string) (*iapiserver.TaskGroup, error)
	ListDAGTaskGroups(context.Context, *iapiserver.DAGTaskGroupListRequest) (*iapiserver.DAGTaskGroupListResponse, error)
	CreateDAGTaskGroup(context.Context, *iapiserver.DAGTaskGroupCreateRequest) (*iapiserver.DAGTaskGroup, error)
	GetDAGTaskGroup(context.Context, string) (*iapiserver.DAGTaskGroup, error)
	// GetDAGTaskGroupSummaries 批量返回当前主体可见的 DAGTaskGroup 一跳摘要。
	GetDAGTaskGroupSummaries(context.Context, []string) (map[string]*iapiserver.DAGTaskGroupSummary, error)
	ListDAGTaskGroupTasks(context.Context, string, *iapiserver.AtomicTaskListRequest) (*iapiserver.AtomicTaskListResponse, error)
	CancelDAGTaskGroup(context.Context, string) (*iapiserver.DAGTaskGroup, error)
	RetryDAGTaskGroup(context.Context, string) (*iapiserver.DAGTaskGroup, error)
	ListTaskSchedules(context.Context, *iapiserver.TaskScheduleListRequest) (*iapiserver.TaskScheduleListResponse, error)
	CreateTaskSchedule(context.Context, *iapiserver.TaskScheduleCreateRequest) (*iapiserver.TaskSchedule, error)
	GetTaskSchedule(context.Context, string) (*iapiserver.TaskSchedule, error)
	UpdateTaskSchedule(context.Context, *iapiserver.TaskScheduleUpdateRequest) (*iapiserver.TaskSchedule, error)
	DeleteTaskSchedule(context.Context, string) error
	PauseTaskSchedule(context.Context, string) (*iapiserver.TaskSchedule, error)
	ResumeTaskSchedule(context.Context, string) (*iapiserver.TaskSchedule, error)
	ListScheduleExecutions(context.Context, *iapiserver.ScheduleExecutionListRequest) (*iapiserver.ScheduleExecutionListResponse, error)
	GetScheduleReconcileState(context.Context, string) (*iapiserver.ScheduleReconcileState, error)
	EnsureSystemReconcileSchedule(context.Context, *iapiserver.TaskSchedule) (*iapiserver.TaskSchedule, error)
	RunScheduleReconcile(context.Context, string, string, time.Time) (map[string]any, error)
	// RegisterDAGDefinition validates and registers an immutable DAG definition for workflow-canvas publishing.
	RegisterDAGDefinition(context.Context, string, int, *iapiserver.DAGTaskGroupCreateRequest) (*DefinitionBinding, error)
}

type DefinitionBinding struct {
	Name     string
	Version  int
	Revision string
	Hash     string
}

type taskCenterService struct {
	store      store.TaskCenterStore
	runtime    workflowruntime.WorkflowRuntime
	functions  map[string]struct{}
	reconciles *ReconcileRegistry
}

func NewService(factory store.Factory, runtimes ...workflowruntime.WorkflowRuntime) TaskCenterSrv {
	runtime := workflowruntime.WorkflowRuntime(workflowruntime.UnavailableRuntime{})
	if len(runtimes) > 0 && runtimes[0] != nil {
		runtime = runtimes[0]
	}
	return &taskCenterService{store: factory.TaskCenters(), runtime: runtime, functions: make(map[string]struct{}), reconciles: NewReconcileRegistry()}
}

// NewServiceWithRegistries 注入 functionRef 与 ReconcileRegistry 两类受控后端注册表。
func NewServiceWithRegistries(factory store.Factory, runtime workflowruntime.WorkflowRuntime, reconciles *ReconcileRegistry, functionRefs ...string) TaskCenterSrv {
	service := NewServiceWithFunctions(factory, runtime, functionRefs...).(*taskCenterService)
	if reconciles != nil {
		service.reconciles = reconciles
	}
	return service
}

func NewServiceWithFunctions(factory store.Factory, runtime workflowruntime.WorkflowRuntime, functionRefs ...string) TaskCenterSrv {
	service := NewService(factory, runtime).(*taskCenterService)
	for _, ref := range functionRefs {
		if ref != "" {
			service.functions[ref] = struct{}{}
		}
	}
	return service
}

func (s *taskCenterService) ListAtomicTasks(ctx context.Context, req *iapiserver.AtomicTaskListRequest) (*iapiserver.AtomicTaskListResponse, error) {
	applyTaskScope(ctx, &req.ProjectID, &req.Namespace, &req.CreatedBy)
	req.IncludeSystem = req.CreatedBy == "system-admin"
	items, total, err := s.store.ListAtomicTasks(ctx, req)
	if err != nil {
		return nil, err
	}
	sources, err := s.store.ListScheduleSources(ctx, iapiserver.TaskScheduleTargetAtomic, atomicTaskIDs(items))
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		item.ScheduleSource = sources[item.ID]
	}
	if err := s.attachAtomicTaskRelations(ctx, items); err != nil {
		return nil, err
	}
	return &iapiserver.AtomicTaskListResponse{Total: total, Items: items}, nil
}
func (s *taskCenterService) GetAtomicTask(ctx context.Context, id string) (*iapiserver.AtomicTask, error) {
	item, err := s.store.GetAtomicTask(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canReadTaskCreatedBy(ctx, item.CreatedBy) {
		return nil, errors.NewStatus(code.ErrAtomicTaskNotFound, "atomic task not found")
	}
	sources, err := s.store.ListScheduleSources(ctx, iapiserver.TaskScheduleTargetAtomic, []string{item.ID})
	if err != nil {
		return nil, err
	}
	item.ScheduleSource = sources[item.ID]
	if err := s.attachAtomicTaskRelations(ctx, []*iapiserver.AtomicTask{item}); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *taskCenterService) GetAtomicTaskSummaries(ctx context.Context, ids []string) (map[string]*iapiserver.AtomicTaskSummary, error) {
	result := make(map[string]*iapiserver.AtomicTaskSummary)
	items, err := s.store.GetAtomicTasksByIDs(ctx, sliceutil.Unique(ids))
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item == nil || !canReadTaskCreatedBy(ctx, item.CreatedBy) {
			continue
		}
		result[item.ID] = &iapiserver.AtomicTaskSummary{ID: item.ID, Name: item.Name, Status: item.Status, Progress: item.Progress, FunctionRef: item.FunctionRef}
	}
	return result, nil
}
func (s *taskCenterService) ListAttempts(ctx context.Context, req *iapiserver.TaskAttemptListRequest) (*iapiserver.TaskAttemptListResponse, error) {
	task, err := s.GetAtomicTask(ctx, req.AtomicTaskID)
	if err != nil {
		return nil, err
	}
	items, total, err := s.store.ListAttempts(ctx, req)
	if err != nil {
		return nil, err
	}
	summary := atomicTaskSummary(task)
	for _, item := range items {
		item.AtomicTask = summary
	}
	return &iapiserver.TaskAttemptListResponse{Total: total, Items: items}, nil
}

func (s *taskCenterService) CreateAtomicTask(ctx context.Context, req *iapiserver.AtomicTaskCreateRequest) (*iapiserver.AtomicTask, error) {
	if err := s.validateFunctionRef(req.FunctionRef); err != nil {
		return nil, err
	}
	if (req.IdempotencyScope == "") != (req.IdempotencyKey == "") {
		return nil, errors.NewStatusF(code.ErrAtomicTaskIdempotencyConflict, "idempotency scope and key must be provided together")
	}
	createdBy := req.CreatedBy
	if createdBy == "" {
		createdBy = taskActor(ctx)
	}
	task := atomicTaskFromRequest(req, createdBy)
	task.ID = uuid.NewString()
	task.RootTaskID = task.ID
	task.Status = iapiserver.AtomicTaskStatusPending
	createdTask, created, err := s.store.AddAtomicTaskIdempotent(ctx, task)
	if err != nil || !created {
		return createdTask, err
	}
	definition := atomicDefinition(createdTask)
	binding, err := s.runtime.RegisterDefinition(ctx, definition)
	if err != nil {
		return createdTask, runtimeError(err)
	}
	execution, err := s.runtime.StartExecution(ctx, workflowruntime.StartRequest{DefinitionName: binding.DefinitionName, DefinitionVersion: binding.DefinitionVersion, CorrelationID: createdTask.ID, IdempotencyKey: stableRuntimeKey(createdTask.ProjectID, createdTask.Namespace, createdTask.IdempotencyScope, createdTask.IdempotencyKey, createdTask.ID), Input: map[string]any{"atomic_task_id": createdTask.ID, "arguments": createdTask.Arguments}})
	if err != nil {
		return createdTask, runtimeError(err)
	}
	createdTask.RuntimeExecutionID = execution.ID
	createdTask.RuntimeRevision = binding.Revision
	createdTask.Status = iapiserver.AtomicTaskStatusRunning
	return s.store.UpdateAtomicTask(ctx, createdTask)
}

func (s *taskCenterService) CancelAtomicTask(ctx context.Context, id string, req *iapiserver.ActionReasonRequest) (*iapiserver.AtomicTask, error) {
	task, err := s.GetAtomicTask(ctx, id)
	if err != nil {
		return nil, err
	}
	if iapiserver.IsAtomicTaskTerminal(task.Status) {
		return nil, errors.NewStatusF(code.ErrAtomicTaskStateBlocked, "terminal atomic task cannot be canceled")
	}
	if task.RuntimeExecutionID != "" {
		if err := s.runtime.CancelExecution(ctx, task.RuntimeExecutionID, req.Reason); err != nil {
			return nil, runtimeError(err)
		}
	}
	task.Status = iapiserver.AtomicTaskStatusCancelRequested
	return s.store.UpdateAtomicTask(ctx, task)
}

func (s *taskCenterService) RetryAtomicTask(ctx context.Context, id string, _ *iapiserver.ActionReasonRequest) (*iapiserver.AtomicTask, error) {
	source, err := s.GetAtomicTask(ctx, id)
	if err != nil {
		return nil, err
	}
	if source.Status != iapiserver.AtomicTaskStatusFailed && source.Status != iapiserver.AtomicTaskStatusTimeout && source.Status != iapiserver.AtomicTaskStatusCanceled {
		return nil, errors.NewStatusF(code.ErrAtomicTaskStateBlocked, "atomic task cannot be manually retried")
	}
	req := &iapiserver.AtomicTaskCreateRequest{Key: source.ChildKey, Name: source.Name, Description: source.Description, FunctionRef: source.FunctionRef, Arguments: source.Arguments, RequiredCapabilities: source.RequiredCapabilities, RetryPolicy: source.RetryPolicy, TimeoutPolicy: source.TimeoutPolicy, ProjectID: source.ProjectID, Namespace: source.Namespace}
	retried, err := s.CreateAtomicTask(ctx, req)
	if err != nil {
		return retried, err
	}
	retried.RetryOfTaskID = source.ID
	retried.RootTaskID = source.RootTaskID
	if retried.RootTaskID == "" {
		retried.RootTaskID = source.ID
	}
	return s.store.UpdateAtomicTask(ctx, retried)
}

func (s *taskCenterService) ListTaskGroups(ctx context.Context, req *iapiserver.TaskGroupListRequest) (*iapiserver.TaskGroupListResponse, error) {
	applyTaskScope(ctx, &req.ProjectID, &req.Namespace, &req.CreatedBy)
	req.IncludeSystem = req.CreatedBy == "system-admin"
	items, total, err := s.store.ListTaskGroups(ctx, req)
	if err != nil {
		return nil, err
	}
	sources, err := s.store.ListScheduleSources(ctx, iapiserver.TaskScheduleTargetGroup, taskGroupIDs(items))
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		item.ScheduleSource = sources[item.ID]
	}
	if err := s.attachTaskGroupRelations(ctx, items); err != nil {
		return nil, err
	}
	return &iapiserver.TaskGroupListResponse{Total: total, Items: items}, nil
}
func (s *taskCenterService) GetTaskGroup(ctx context.Context, id string) (*iapiserver.TaskGroup, error) {
	item, err := s.store.GetTaskGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canReadTaskCreatedBy(ctx, item.CreatedBy) {
		return nil, errors.NewStatus(code.ErrTaskGroupNotFound, "task group not found")
	}
	sources, err := s.store.ListScheduleSources(ctx, iapiserver.TaskScheduleTargetGroup, []string{item.ID})
	if err != nil {
		return nil, err
	}
	item.ScheduleSource = sources[item.ID]
	if err := s.attachTaskGroupRelations(ctx, []*iapiserver.TaskGroup{item}); err != nil {
		return nil, err
	}
	return item, nil
}
func (s *taskCenterService) ListTaskGroupTasks(ctx context.Context, id string, req *iapiserver.AtomicTaskListRequest) (*iapiserver.AtomicTaskListResponse, error) {
	if _, err := s.GetTaskGroup(ctx, id); err != nil {
		return nil, err
	}
	items, total, err := s.store.ListOwnedTasks(ctx, iapiserver.TaskOwnerTypeGroup, id, req)
	if err != nil {
		return nil, err
	}
	if err := s.attachAtomicTaskRelations(ctx, items); err != nil {
		return nil, err
	}
	return &iapiserver.AtomicTaskListResponse{Total: total, Items: items}, nil
}

func (s *taskCenterService) CreateTaskGroup(ctx context.Context, req *iapiserver.TaskGroupCreateRequest) (*iapiserver.TaskGroup, error) {
	if err := s.validateTemplates(req.Tasks); err != nil {
		return nil, err
	}
	if req.Strategy.MaxParallelism < 0 || req.Strategy.MaxParallelism > iapiserver.MaxTaskGraphNodes {
		return nil, errors.NewStatusF(code.ErrTaskGroupInvalid, "max parallelism is invalid")
	}
	if req.Mode == iapiserver.TaskGroupModeSerial && !req.Strategy.FailFast {
		req.Strategy.FailFast = true
	}
	createdBy := req.CreatedBy
	if createdBy == "" {
		createdBy = taskActor(ctx)
	}
	group := &iapiserver.TaskGroup{Mode: req.Mode, Tasks: req.Tasks, Strategy: req.Strategy, Status: iapiserver.TaskGroupStatusPending, Summary: iapiserver.TaskSummary{Total: len(req.Tasks), Pending: len(req.Tasks)}, IdempotencyScope: req.IdempotencyScope, IdempotencyKey: req.IdempotencyKey, ProjectID: req.ProjectID, Namespace: req.Namespace, CreatedBy: createdBy}
	group.ID = uuid.NewString()
	group.Name = req.Name
	group.Description = req.Description
	tasks := tasksFromTemplates(req.Tasks, group.ID, iapiserver.TaskOwnerTypeGroup, req.ProjectID, req.Namespace, group.CreatedBy)
	createdGroup, created, err := s.store.AddTaskGroupWithTasks(ctx, group, tasks)
	if err != nil || !created {
		return createdGroup, err
	}
	definition := groupDefinition(createdGroup, tasks)
	binding, err := s.runtime.RegisterDefinition(ctx, definition)
	if err != nil {
		return createdGroup, runtimeError(err)
	}
	execution, err := s.runtime.StartExecution(ctx, workflowruntime.StartRequest{DefinitionName: binding.DefinitionName, DefinitionVersion: binding.DefinitionVersion, CorrelationID: createdGroup.ID, IdempotencyKey: stableRuntimeKey(createdGroup.ProjectID, createdGroup.Namespace, createdGroup.IdempotencyScope, createdGroup.IdempotencyKey, createdGroup.ID), Input: map[string]any{"task_group_id": createdGroup.ID}})
	if err != nil {
		return createdGroup, runtimeError(err)
	}
	createdGroup.RuntimeExecutionID = execution.ID
	createdGroup.RuntimeDefinitionName = binding.DefinitionName
	createdGroup.RuntimeDefinitionVersion = binding.DefinitionVersion
	createdGroup.Status = iapiserver.TaskGroupStatusRunning
	return s.store.UpdateTaskGroup(ctx, createdGroup)
}

func (s *taskCenterService) CancelTaskGroup(ctx context.Context, id string) (*iapiserver.TaskGroup, error) {
	group, err := s.GetTaskGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	if iapiserver.IsTaskGroupTerminal(group.Status) {
		return nil, errors.NewStatusF(code.ErrAtomicTaskStateBlocked, "terminal task group cannot be canceled")
	}
	if group.RuntimeExecutionID != "" {
		if err := s.runtime.CancelExecution(ctx, group.RuntimeExecutionID, "task group canceled"); err != nil {
			return nil, runtimeError(err)
		}
	}
	group.Status = iapiserver.TaskGroupStatusCancelRequested
	return s.store.UpdateTaskGroup(ctx, group)
}
func (s *taskCenterService) RetryTaskGroup(ctx context.Context, id string) (*iapiserver.TaskGroup, error) {
	source, err := s.GetTaskGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	if !iapiserver.IsTaskGroupTerminal(source.Status) {
		return nil, errors.NewStatusF(code.ErrAtomicTaskStateBlocked, "task group cannot be rerun")
	}
	created, err := s.CreateTaskGroup(ctx, &iapiserver.TaskGroupCreateRequest{Name: source.Name, Description: source.Description, Mode: source.Mode, Tasks: source.Tasks, Strategy: source.Strategy, ProjectID: source.ProjectID, Namespace: source.Namespace})
	if err != nil {
		return created, err
	}
	created.RetryOfID = source.ID
	return s.store.UpdateTaskGroup(ctx, created)
}

func (s *taskCenterService) ListDAGTaskGroups(ctx context.Context, req *iapiserver.DAGTaskGroupListRequest) (*iapiserver.DAGTaskGroupListResponse, error) {
	applyTaskScope(ctx, &req.ProjectID, &req.Namespace, &req.CreatedBy)
	req.IncludeSystem = req.CreatedBy == "system-admin"
	items, total, err := s.store.ListDAGTaskGroups(ctx, req)
	if err != nil {
		return nil, err
	}
	sources, err := s.store.ListScheduleSources(ctx, iapiserver.TaskScheduleTargetDAG, dagTaskGroupIDs(items))
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		item.ScheduleSource = sources[item.ID]
	}
	if err := s.attachDAGTaskGroupRelations(ctx, items); err != nil {
		return nil, err
	}
	return &iapiserver.DAGTaskGroupListResponse{Total: total, Items: items}, nil
}
func (s *taskCenterService) GetDAGTaskGroup(ctx context.Context, id string) (*iapiserver.DAGTaskGroup, error) {
	item, err := s.store.GetDAGTaskGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canReadTaskCreatedBy(ctx, item.CreatedBy) {
		return nil, errors.NewStatus(code.ErrDAGTaskGroupNotFound, "dag task group not found")
	}
	sources, err := s.store.ListScheduleSources(ctx, iapiserver.TaskScheduleTargetDAG, []string{item.ID})
	if err != nil {
		return nil, err
	}
	item.ScheduleSource = sources[item.ID]
	if err := s.attachDAGTaskGroupRelations(ctx, []*iapiserver.DAGTaskGroup{item}); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *taskCenterService) GetDAGTaskGroupSummaries(ctx context.Context, ids []string) (map[string]*iapiserver.DAGTaskGroupSummary, error) {
	result := make(map[string]*iapiserver.DAGTaskGroupSummary)
	items, err := s.store.GetDAGTaskGroupsByIDs(ctx, sliceutil.Unique(ids))
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item == nil || !canReadTaskCreatedBy(ctx, item.CreatedBy) {
			continue
		}
		result[item.ID] = &iapiserver.DAGTaskGroupSummary{ID: item.ID, Name: item.Name, Status: item.Status, Progress: item.Progress}
	}
	return result, nil
}

func (s *taskCenterService) ListDAGTaskGroupTasks(ctx context.Context, id string, req *iapiserver.AtomicTaskListRequest) (*iapiserver.AtomicTaskListResponse, error) {
	if _, err := s.GetDAGTaskGroup(ctx, id); err != nil {
		return nil, err
	}
	items, total, err := s.store.ListOwnedTasks(ctx, iapiserver.TaskOwnerTypeDAGGroup, id, req)
	if err != nil {
		return nil, err
	}
	if err := s.attachAtomicTaskRelations(ctx, items); err != nil {
		return nil, err
	}
	return &iapiserver.AtomicTaskListResponse{Total: total, Items: items}, nil
}

func (s *taskCenterService) CreateDAGTaskGroup(ctx context.Context, req *iapiserver.DAGTaskGroupCreateRequest) (*iapiserver.DAGTaskGroup, error) {
	layers, err := s.validateDAG(req.Nodes, req.Edges)
	if err != nil {
		return nil, err
	}
	createdBy := req.CreatedBy
	if createdBy == "" {
		createdBy = taskActor(ctx)
	}
	group := &iapiserver.DAGTaskGroup{Nodes: req.Nodes, Edges: req.Edges, Input: req.Input, OutputMapping: req.OutputMapping, Status: iapiserver.TaskGroupStatusPending, Summary: iapiserver.TaskSummary{Total: len(req.Nodes), Pending: len(req.Nodes)}, CanvasVersionID: req.CanvasVersionID, IdempotencyScope: req.IdempotencyScope, IdempotencyKey: req.IdempotencyKey, ProjectID: req.ProjectID, Namespace: req.Namespace, CreatedBy: createdBy}
	group.ID = uuid.NewString()
	group.Name = req.Name
	group.Description = req.Description
	tasks := tasksFromDAG(req.Nodes, group.ID, req.ProjectID, req.Namespace, group.CreatedBy)
	definition := dagDefinition(group, tasks, layers)
	group.RuntimeDefinitionName = definition.Name
	group.RuntimeDefinitionVersion = definition.Version
	group.RuntimeDefinitionHash = definitionHash(definition)
	createdGroup, created, err := s.store.AddDAGTaskGroupWithTasks(ctx, group, tasks)
	if err != nil || !created {
		return createdGroup, err
	}
	binding, err := s.runtime.RegisterDefinition(ctx, definition)
	if err != nil {
		return createdGroup, runtimeError(err)
	}
	execution, err := s.runtime.StartExecution(ctx, workflowruntime.StartRequest{DefinitionName: binding.DefinitionName, DefinitionVersion: binding.DefinitionVersion, CorrelationID: createdGroup.ID, IdempotencyKey: stableRuntimeKey(createdGroup.ProjectID, createdGroup.Namespace, createdGroup.IdempotencyScope, createdGroup.IdempotencyKey, createdGroup.ID), Input: map[string]any{"dag_task_group_id": createdGroup.ID, "input": createdGroup.Input}})
	if err != nil {
		return createdGroup, runtimeError(err)
	}
	createdGroup.RuntimeExecutionID = execution.ID
	createdGroup.Status = iapiserver.TaskGroupStatusRunning
	return s.store.UpdateDAGTaskGroup(ctx, createdGroup)
}

func (s *taskCenterService) CancelDAGTaskGroup(ctx context.Context, id string) (*iapiserver.DAGTaskGroup, error) {
	group, err := s.GetDAGTaskGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	if iapiserver.IsTaskGroupTerminal(group.Status) {
		return nil, errors.NewStatusF(code.ErrAtomicTaskStateBlocked, "terminal dag task group cannot be canceled")
	}
	if group.RuntimeExecutionID != "" {
		if err := s.runtime.CancelExecution(ctx, group.RuntimeExecutionID, "dag task group canceled"); err != nil {
			return nil, runtimeError(err)
		}
	}
	group.Status = iapiserver.TaskGroupStatusCancelRequested
	return s.store.UpdateDAGTaskGroup(ctx, group)
}
func (s *taskCenterService) RetryDAGTaskGroup(ctx context.Context, id string) (*iapiserver.DAGTaskGroup, error) {
	source, err := s.GetDAGTaskGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	if !iapiserver.IsTaskGroupTerminal(source.Status) {
		return nil, errors.NewStatusF(code.ErrAtomicTaskStateBlocked, "dag task group cannot be rerun")
	}
	created, err := s.CreateDAGTaskGroup(ctx, &iapiserver.DAGTaskGroupCreateRequest{Name: source.Name, Description: source.Description, Nodes: source.Nodes, Edges: source.Edges, Input: source.Input, OutputMapping: source.OutputMapping, CanvasVersionID: source.CanvasVersionID, ProjectID: source.ProjectID, Namespace: source.Namespace})
	if err != nil {
		return created, err
	}
	created.RetryOfID = source.ID
	return s.store.UpdateDAGTaskGroup(ctx, created)
}

func (s *taskCenterService) ListTaskSchedules(ctx context.Context, req *iapiserver.TaskScheduleListRequest) (*iapiserver.TaskScheduleListResponse, error) {
	applyTaskScheduleScope(ctx, req)
	items, total, err := s.store.ListTaskSchedules(ctx, req)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		s.decorateReconcileSchedule(item)
		item.TargetSummary = scheduleTemplateSummary(item)
	}
	if err := s.attachLatestScheduleExecutions(ctx, items); err != nil {
		return nil, err
	}
	return &iapiserver.TaskScheduleListResponse{Total: total, Items: items}, nil
}
func (s *taskCenterService) GetTaskSchedule(ctx context.Context, id string) (*iapiserver.TaskSchedule, error) {
	item, err := s.store.GetTaskSchedule(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canReadTaskSchedule(ctx, item) {
		return nil, errors.NewStatus(code.ErrTaskScheduleNotFound, "task schedule not found")
	}
	item.TargetSummary = scheduleTemplateSummary(item)
	if err := s.attachLatestScheduleExecutions(ctx, []*iapiserver.TaskSchedule{item}); err != nil {
		return nil, err
	}
	return s.decorateReconcileSchedule(item), nil
}
func (s *taskCenterService) ListScheduleExecutions(ctx context.Context, req *iapiserver.ScheduleExecutionListRequest) (*iapiserver.ScheduleExecutionListResponse, error) {
	schedule, err := s.GetTaskSchedule(ctx, req.ScheduleID)
	if err != nil {
		return nil, err
	}
	items, total, err := s.store.ListScheduleExecutions(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := s.attachExecutionTargets(ctx, schedule, items); err != nil {
		return nil, err
	}
	summary := taskScheduleSummary(schedule)
	for _, item := range items {
		item.Schedule = summary
	}
	return &iapiserver.ScheduleExecutionListResponse{Total: total, Items: items}, nil
}

func (s *taskCenterService) RegisterDAGDefinition(ctx context.Context, name string, version int, req *iapiserver.DAGTaskGroupCreateRequest) (*DefinitionBinding, error) {
	if req == nil || name == "" || version <= 0 {
		return nil, errors.NewStatusF(code.ErrDAGTaskGroupInvalid, "runtime definition identity is invalid")
	}
	layers, err := s.validateDAG(req.Nodes, req.Edges)
	if err != nil {
		return nil, err
	}
	tasks := tasksFromDAG(req.Nodes, name, req.ProjectID, req.Namespace, iapiserver.DefaultTaskCenterCreatedBy)
	definition := dagDefinition(&iapiserver.DAGTaskGroup{Nodes: req.Nodes, Edges: req.Edges, OutputMapping: req.OutputMapping}, tasks, layers)
	definition.Name = name
	definition.Version = version
	hash := definitionHash(definition)
	binding, err := s.runtime.RegisterDefinition(ctx, definition)
	if err != nil {
		return nil, runtimeError(err)
	}
	return &DefinitionBinding{Name: binding.DefinitionName, Version: binding.DefinitionVersion, Revision: binding.Revision, Hash: hash}, nil
}

func (s *taskCenterService) CreateTaskSchedule(ctx context.Context, req *iapiserver.TaskScheduleCreateRequest) (*iapiserver.TaskSchedule, error) {
	if err := validateScheduleRequest(req); err != nil {
		return nil, err
	}
	schedule := &iapiserver.TaskSchedule{ExecutionMode: iapiserver.TaskScheduleModeMaterialized, ManagementMode: iapiserver.TaskScheduleManagementUser, TriggerType: req.TriggerType, CronExpression: req.CronExpression, RunAt: req.RunAt, TimeZone: req.TimeZone, Target: req.Target, HistoryRetention: defaultHistoryRetention(), Status: iapiserver.TaskScheduleStatusActive, MisfirePolicy: iapiserver.TaskSchedulePolicySkip, OverlapPolicy: iapiserver.TaskSchedulePolicySkip, ProjectID: req.ProjectID, Namespace: req.Namespace, CreatedBy: taskActor(ctx)}
	schedule.ID = uuid.NewString()
	schedule.Name = req.Name
	schedule.Description = req.Description
	schedule.RuntimeScheduleName = "task_schedule_" + schedule.ID
	definition := scheduleLauncherDefinition(schedule)
	binding, err := s.runtime.RegisterDefinition(ctx, definition)
	if err != nil {
		return nil, runtimeError(err)
	}
	created, err := s.store.AddTaskSchedule(ctx, schedule)
	if err != nil {
		return nil, err
	}
	start := workflowruntime.StartRequest{DefinitionName: binding.DefinitionName, DefinitionVersion: binding.DefinitionVersion, CorrelationID: schedule.ID + "-${scheduledTime}", Input: map[string]any{"task_schedule_id": schedule.ID, "run_at": schedule.RunAt.Time}}
	if schedule.TriggerType == iapiserver.TaskScheduleTriggerCron {
		err = s.runtime.SaveSchedule(ctx, runtimeSchedule(schedule, start))
	} else {
		start.CorrelationID = schedule.ID
		start.IdempotencyKey = schedule.ID
		_, err = s.runtime.StartExecution(ctx, start)
	}
	if err != nil {
		// 外部运行时注册失败时隐藏尚未生效的本地记录，避免暴露假的 ACTIVE Schedule。
		if schedule.TriggerType == iapiserver.TaskScheduleTriggerCron {
			_ = s.runtime.DeleteSchedule(ctx, schedule.RuntimeScheduleName)
		}
		schedule.Status = iapiserver.TaskScheduleStatusDeleted
		schedule.DeletedAt = imachinery.Now()
		_, _ = s.store.UpdateTaskSchedule(ctx, schedule)
		return created, runtimeError(err)
	}
	created.TargetSummary = scheduleTemplateSummary(created)
	return created, nil
}

func (s *taskCenterService) UpdateTaskSchedule(ctx context.Context, req *iapiserver.TaskScheduleUpdateRequest) (*iapiserver.TaskSchedule, error) {
	schedule, err := s.GetTaskSchedule(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if schedule.Status == iapiserver.TaskScheduleStatusDeleted || schedule.Status == iapiserver.TaskScheduleStatusCompleted {
		return nil, errors.NewStatusF(code.ErrTaskScheduleStateBlocked, "task schedule cannot be updated")
	}
	if schedule.ManagementMode == iapiserver.TaskScheduleManagementSystem {
		if taskActor(ctx) != "system-admin" || req.Name != nil || req.Description != nil || req.RunAt != nil || req.Target != nil {
			return nil, errors.NewStatus(code.ErrTaskSystemScheduleOperationRestricted, "system schedule protected fields cannot be changed")
		}
		if req.ReconcileSpec == nil && req.CronExpression == nil && req.TimeZone == nil {
			return nil, errors.NewStatus(code.ErrTaskSystemScheduleOperationRestricted, "system schedule update has no allowed field")
		}
	}
	if req.Name != nil {
		schedule.Name = *req.Name
	}
	if req.Description != nil {
		schedule.Description = *req.Description
	}
	if req.CronExpression != nil {
		schedule.CronExpression = *req.CronExpression
	}
	if req.RunAt != nil {
		schedule.RunAt = *req.RunAt
	}
	if req.TimeZone != nil {
		schedule.TimeZone = *req.TimeZone
	}
	if req.Target != nil {
		schedule.Target = *req.Target
	}
	if req.ReconcileSpec != nil {
		if schedule.ReconcileSpec == nil {
			return nil, errors.NewStatus(code.ErrTaskSystemScheduleOperationRestricted, "materialized schedule cannot accept reconcile spec")
		}
		if req.ReconcileSpec.Config != nil {
			schedule.ReconcileSpec.Config = *req.ReconcileSpec.Config
		}
		if req.ReconcileSpec.MaxParallelism != nil {
			schedule.ReconcileSpec.MaxParallelism = *req.ReconcileSpec.MaxParallelism
		}
		if req.ReconcileSpec.MaxItemsPerRun != nil {
			schedule.ReconcileSpec.MaxItemsPerRun = *req.ReconcileSpec.MaxItemsPerRun
		}
		if req.ReconcileSpec.PerItemTimeoutSeconds != nil {
			schedule.ReconcileSpec.PerItemTimeoutSeconds = *req.ReconcileSpec.PerItemTimeoutSeconds
		}
		if req.ReconcileSpec.OverallTimeoutSeconds != nil {
			schedule.ReconcileSpec.OverallTimeoutSeconds = *req.ReconcileSpec.OverallTimeoutSeconds
		}
	}
	if schedule.ExecutionMode == iapiserver.TaskScheduleModeReconcile {
		handler, ok := s.reconciles.Get(schedule.ReconcileSpec.ReconcileRef)
		if !ok {
			return nil, errors.NewStatus(code.ErrTaskReconcileRefUnregistered, "reconcile handler is not registered")
		}
		if schedule.TimeZone == "" || len(strings.Fields(schedule.CronExpression)) != 6 {
			return nil, errors.NewStatus(code.ErrTaskScheduleInvalid, "schedule cron or timezone is invalid")
		}
		if _, err := time.LoadLocation(schedule.TimeZone); err != nil {
			return nil, errors.NewStatus(code.ErrTaskScheduleInvalid, "schedule time zone is invalid")
		}
		if err := validateReconcileSpec(schedule.ReconcileSpec, handler); err != nil {
			return nil, err
		}
	} else {
		validation := &iapiserver.TaskScheduleCreateRequest{ExecutionMode: iapiserver.TaskScheduleModeMaterialized, TriggerType: schedule.TriggerType, CronExpression: schedule.CronExpression, RunAt: schedule.RunAt, TimeZone: schedule.TimeZone, Target: schedule.Target}
		if err := validateScheduleRequest(validation); err != nil {
			return nil, err
		}
	}
	if schedule.TriggerType == iapiserver.TaskScheduleTriggerCron {
		definition := scheduleLauncherDefinition(schedule)
		if schedule.ExecutionMode == iapiserver.TaskScheduleModeReconcile {
			definition = reconcileControllerDefinition()
		}
		binding, err := s.runtime.RegisterDefinition(ctx, definition)
		if err != nil {
			return nil, runtimeError(err)
		}
		start := workflowruntime.StartRequest{DefinitionName: binding.DefinitionName, DefinitionVersion: binding.DefinitionVersion, CorrelationID: schedule.ID + "-${scheduledTime}", Input: map[string]any{"task_schedule_id": schedule.ID}}
		if err := s.runtime.SaveSchedule(ctx, runtimeSchedule(schedule, start)); err != nil {
			return nil, runtimeError(err)
		}
	}
	schedule.TargetSummary = scheduleTemplateSummary(schedule)
	return s.store.UpdateTaskSchedule(ctx, schedule)
}
func (s *taskCenterService) DeleteTaskSchedule(ctx context.Context, id string) error {
	schedule, err := s.GetTaskSchedule(ctx, id)
	if err != nil {
		return err
	}
	if schedule.ManagementMode == iapiserver.TaskScheduleManagementSystem {
		return errors.NewStatus(code.ErrTaskSystemScheduleOperationRestricted, "system schedule cannot be deleted")
	}
	if schedule.RuntimeScheduleName != "" && schedule.TriggerType == iapiserver.TaskScheduleTriggerCron {
		if err := s.runtime.DeleteSchedule(ctx, schedule.RuntimeScheduleName); err != nil {
			return runtimeError(err)
		}
	}
	schedule.Status = iapiserver.TaskScheduleStatusDeleted
	schedule.DeletedAt = imachinery.NewTime(time.Now())
	_, err = s.store.UpdateTaskSchedule(ctx, schedule)
	return err
}
func (s *taskCenterService) PauseTaskSchedule(ctx context.Context, id string) (*iapiserver.TaskSchedule, error) {
	schedule, err := s.GetTaskSchedule(ctx, id)
	if err != nil {
		return nil, err
	}
	if schedule.Status != iapiserver.TaskScheduleStatusActive {
		return nil, errors.NewStatusF(code.ErrTaskScheduleStateBlocked, "task schedule is not active")
	}
	if schedule.TriggerType == iapiserver.TaskScheduleTriggerCron {
		if err := s.runtime.PauseSchedule(ctx, schedule.RuntimeScheduleName); err != nil {
			return nil, runtimeError(err)
		}
	}
	schedule.Status = iapiserver.TaskScheduleStatusPaused
	schedule.TargetSummary = scheduleTemplateSummary(schedule)
	return s.store.UpdateTaskSchedule(ctx, schedule)
}
func (s *taskCenterService) ResumeTaskSchedule(ctx context.Context, id string) (*iapiserver.TaskSchedule, error) {
	schedule, err := s.GetTaskSchedule(ctx, id)
	if err != nil {
		return nil, err
	}
	if schedule.Status != iapiserver.TaskScheduleStatusPaused {
		return nil, errors.NewStatusF(code.ErrTaskScheduleStateBlocked, "task schedule is not paused")
	}
	if schedule.TriggerType == iapiserver.TaskScheduleTriggerCron {
		if err := s.runtime.ResumeSchedule(ctx, schedule.RuntimeScheduleName); err != nil {
			return nil, runtimeError(err)
		}
	}
	schedule.Status = iapiserver.TaskScheduleStatusActive
	schedule.TargetSummary = scheduleTemplateSummary(schedule)
	return s.store.UpdateTaskSchedule(ctx, schedule)
}

func (s *taskCenterService) validateFunctionRef(ref string) error {
	if ref == "" {
		return errors.NewStatusF(code.ErrTaskFunctionRefNotRegistered, "function ref is required")
	}
	if len(s.functions) == 0 {
		return nil
	}
	if _, ok := s.functions[ref]; !ok {
		return errors.NewStatusF(code.ErrTaskFunctionRefNotRegistered, "function ref is not registered")
	}
	return nil
}
func (s *taskCenterService) validateTemplates(templates []iapiserver.AtomicTaskTemplate) error {
	if len(templates) == 0 || len(templates) > iapiserver.MaxTaskGraphNodes {
		return errors.NewStatusF(code.ErrTaskGroupInvalid, "task group size is invalid")
	}
	keys := make(map[string]struct{}, len(templates))
	for _, template := range templates {
		if template.Key == "" {
			return errors.NewStatusF(code.ErrTaskGroupInvalid, "child key is required")
		}
		if _, exists := keys[template.Key]; exists {
			return errors.NewStatusF(code.ErrTaskGroupInvalid, "child key must be unique")
		}
		keys[template.Key] = struct{}{}
		if err := s.validateFunctionRef(template.FunctionRef); err != nil {
			return err
		}
	}
	return nil
}

func (s *taskCenterService) validateDAG(nodes []iapiserver.DAGNode, edges []iapiserver.DAGEdge) ([][]string, error) {
	if len(nodes) == 0 || len(nodes) > iapiserver.MaxTaskGraphNodes || len(edges) > iapiserver.MaxTaskGraphEdges {
		return nil, errors.NewStatusF(code.ErrDAGTaskGroupInvalid, "dag graph size is invalid")
	}
	keys := make(map[string]iapiserver.DAGNode, len(nodes))
	indegree := make(map[string]int, len(nodes))
	adjacency := make(map[string][]string, len(nodes))
	for _, node := range nodes {
		if node.Key == "" {
			return nil, errors.NewStatusF(code.ErrDAGTaskGroupInvalid, "dag node key is required")
		}
		if _, exists := keys[node.Key]; exists {
			return nil, errors.NewStatusF(code.ErrDAGTaskGroupInvalid, "dag node key must be unique")
		}
		if err := s.validateFunctionRef(node.Task.FunctionRef); err != nil {
			return nil, err
		}
		if node.DynamicFork && (node.MaxDynamicTasks < 1 || node.MaxDynamicTasks > iapiserver.MaxDynamicForkTasks) {
			return nil, errors.NewStatusF(code.ErrDAGTaskGroupInvalid, "dynamic fork limit is invalid")
		}
		keys[node.Key] = node
		indegree[node.Key] = 0
	}
	for _, edge := range edges {
		if _, ok := keys[edge.FromNode]; !ok {
			return nil, errors.NewStatusF(code.ErrDAGTaskGroupInvalid, "dag edge source is missing")
		}
		if _, ok := keys[edge.ToNode]; !ok {
			return nil, errors.NewStatusF(code.ErrDAGTaskGroupInvalid, "dag edge target is missing")
		}
		adjacency[edge.FromNode] = append(adjacency[edge.FromNode], edge.ToNode)
		indegree[edge.ToNode]++
	}
	current := make([]string, 0)
	for key, degree := range indegree {
		if degree == 0 {
			current = append(current, key)
		}
	}
	layers := make([][]string, 0)
	visited := 0
	for len(current) > 0 {
		sort.Strings(current)
		layer := append([]string(nil), current...)
		layers = append(layers, layer)
		next := make([]string, 0)
		for _, key := range current {
			visited++
			for _, child := range adjacency[key] {
				indegree[child]--
				if indegree[child] == 0 {
					next = append(next, child)
				}
			}
		}
		current = next
	}
	if visited != len(nodes) {
		return nil, errors.NewStatusF(code.ErrTaskDAGCycleDetected, "dag contains a cycle")
	}
	return layers, nil
}

func atomicTaskFromRequest(req *iapiserver.AtomicTaskCreateRequest, createdBy string) *iapiserver.AtomicTask {
	task := &iapiserver.AtomicTask{FunctionRef: req.FunctionRef, Arguments: req.Arguments, RequiredCapabilities: req.RequiredCapabilities, RetryPolicy: req.RetryPolicy, TimeoutPolicy: req.TimeoutPolicy, OwnerType: req.OwnerType, OwnerID: req.OwnerID, ChildKey: req.Key, ApplicationRunID: req.ApplicationRunID, CanvasRunID: req.CanvasRunID, CanvasNodeRunID: req.CanvasNodeRunID, IdempotencyScope: req.IdempotencyScope, IdempotencyKey: req.IdempotencyKey, ProjectID: req.ProjectID, Namespace: req.Namespace, CreatedBy: createdBy}
	task.Name = req.Name
	if task.Name == "" {
		task.Name = req.Key
	}
	task.Description = req.Description
	if task.Arguments == nil {
		task.Arguments = map[string]any{}
	}
	return task
}
func tasksFromTemplates(templates []iapiserver.AtomicTaskTemplate, ownerID, ownerType, projectID, namespace, createdBy string) []*iapiserver.AtomicTask {
	tasks := make([]*iapiserver.AtomicTask, 0, len(templates))
	for index, template := range templates {
		task := &iapiserver.AtomicTask{FunctionRef: template.FunctionRef, Arguments: template.Arguments, RequiredCapabilities: template.RequiredCapabilities, RetryPolicy: template.RetryPolicy, TimeoutPolicy: template.TimeoutPolicy, Status: iapiserver.AtomicTaskStatusBlocked, OwnerType: ownerType, OwnerID: ownerID, ChildKey: template.Key, ChildOrder: index, ProjectID: projectID, Namespace: namespace, CreatedBy: createdBy}
		task.ID = uuid.NewString()
		task.RootTaskID = task.ID
		task.Name = template.Name
		if task.Name == "" {
			task.Name = template.Key
		}
		tasks = append(tasks, task)
	}
	return tasks
}
func tasksFromDAG(nodes []iapiserver.DAGNode, ownerID, projectID, namespace, createdBy string) []*iapiserver.AtomicTask {
	templates := make([]iapiserver.AtomicTaskTemplate, len(nodes))
	for i, node := range nodes {
		templates[i] = node.Task
		templates[i].Key = node.Key
	}
	return tasksFromTemplates(templates, ownerID, iapiserver.TaskOwnerTypeDAGGroup, projectID, namespace, createdBy)
}

func atomicDefinition(task *iapiserver.AtomicTask) workflowruntime.Definition {
	name := "atomic_" + safeName(task.FunctionRef) + "_" + shortID(task.ID)
	return workflowruntime.Definition{Name: name, Version: 1, Description: task.Description, TimeoutSeconds: max(task.TimeoutPolicy.OverallTimeoutSeconds, 1), Tasks: []workflowruntime.Task{simpleRuntimeTask(task)}}
}
func groupDefinition(group *iapiserver.TaskGroup, tasks []*iapiserver.AtomicTask) workflowruntime.Definition {
	definition := workflowruntime.Definition{Name: "task_group_" + shortID(group.ID), Version: 1, Description: group.Description, TimeoutSeconds: 86400}
	if group.Mode == iapiserver.TaskGroupModeSerial {
		for _, task := range tasks {
			definition.Tasks = append(definition.Tasks, simpleRuntimeTask(task))
		}
		return definition
	}
	branches := make([][]workflowruntime.Task, 0, len(tasks))
	joins := make([]string, 0, len(tasks))
	for _, task := range tasks {
		runtimeTask := simpleRuntimeTask(task)
		branches = append(branches, []workflowruntime.Task{runtimeTask})
		joins = append(joins, runtimeTask.ReferenceName)
	}
	definition.Tasks = []workflowruntime.Task{{Name: "fork", ReferenceName: "fork", Type: "FORK_JOIN", ForkTasks: branches}, {Name: "join", ReferenceName: "join", Type: "JOIN", JoinOn: joins}}
	return definition
}
func dagDefinition(group *iapiserver.DAGTaskGroup, tasks []*iapiserver.AtomicTask, layers [][]string) workflowruntime.Definition {
	byKey := make(map[string]*iapiserver.AtomicTask, len(tasks))
	for _, task := range tasks {
		byKey[task.ChildKey] = task
	}
	definition := workflowruntime.Definition{Name: "dag_" + shortID(group.ID), Version: 1, Description: group.Description, TimeoutSeconds: 86400, Output: group.OutputMapping}
	nodesByKey := make(map[string]iapiserver.DAGNode, len(group.Nodes))
	for _, node := range group.Nodes {
		nodesByKey[node.Key] = node
	}
	for layerIndex, layer := range layers {
		if len(layer) == 1 {
			definition.Tasks = append(definition.Tasks, runtimeTasksForDAGNode(nodesByKey[layer[0]], byKey[layer[0]])...)
			continue
		}
		branches := make([][]workflowruntime.Task, 0, len(layer))
		joins := make([]string, 0, len(layer))
		for _, key := range layer {
			tasks := runtimeTasksForDAGNode(nodesByKey[key], byKey[key])
			branches = append(branches, tasks)
			joins = append(joins, tasks[len(tasks)-1].ReferenceName)
		}
		forkRef := fmt.Sprintf("layer_%d_fork", layerIndex)
		definition.Tasks = append(definition.Tasks, workflowruntime.Task{Name: forkRef, ReferenceName: forkRef, Type: "FORK_JOIN", ForkTasks: branches}, workflowruntime.Task{Name: fmt.Sprintf("layer_%d_join", layerIndex), ReferenceName: fmt.Sprintf("layer_%d_join", layerIndex), Type: "JOIN", JoinOn: joins})
	}
	return definition
}
func runtimeTasksForDAGNode(node iapiserver.DAGNode, task *iapiserver.AtomicTask) []workflowruntime.Task {
	planner := simpleRuntimeTask(task)
	if !node.DynamicFork {
		return []workflowruntime.Task{planner}
	}
	dynamicRef := safeName(node.Key + "_dynamic_fork")
	joinRef := safeName(node.Key + "_dynamic_join")
	planner.Input["max_dynamic_tasks"] = node.MaxDynamicTasks
	dynamicTasksParam := "dynamic_tasks"
	dynamicInputParam := "dynamic_inputs"
	return []workflowruntime.Task{planner, {
		Name: dynamicRef, ReferenceName: dynamicRef, Type: "FORK_JOIN_DYNAMIC",
		Input:             map[string]any{dynamicTasksParam: "${" + planner.ReferenceName + ".output." + dynamicTasksParam + "}", dynamicInputParam: "${" + planner.ReferenceName + ".output." + dynamicInputParam + "}"},
		DynamicTasksParam: dynamicTasksParam, DynamicInputParam: dynamicInputParam,
	}, {Name: joinRef, ReferenceName: joinRef, Type: "JOIN", JoinOn: []string{dynamicRef}}}
}
func scheduleLauncherDefinition(schedule *iapiserver.TaskSchedule) workflowruntime.Definition {
	tasks := make([]workflowruntime.Task, 0, 2)
	if schedule.TriggerType == iapiserver.TaskScheduleTriggerRunAt {
		tasks = append(tasks, workflowruntime.Task{Name: "wait", ReferenceName: "wait_until", Type: "WAIT", Input: map[string]any{"until": "${workflow.input.run_at}"}})
	}
	tasks = append(tasks, workflowruntime.Task{Name: "task.schedule.acquire", ReferenceName: "schedule_acquire", Type: "SIMPLE", Input: map[string]any{"arguments": map[string]any{"task_schedule_id": "${workflow.input.task_schedule_id}", "scheduled_at": "${workflow.input._scheduledTime}"}}})
	return workflowruntime.Definition{Name: "task_schedule_launcher", Version: 1, Description: "Task Center schedule launcher", TimeoutSeconds: 31536000, Tasks: tasks}
}

func runtimeSchedule(schedule *iapiserver.TaskSchedule, start workflowruntime.StartRequest) workflowruntime.Schedule {
	return workflowruntime.Schedule{
		Name: schedule.RuntimeScheduleName, CronExpression: schedule.CronExpression,
		TimeZone: schedule.TimeZone,
		Paused:   schedule.Status == iapiserver.TaskScheduleStatusPaused, RunCatchup: false,
		StartAt: time.Now(), StartRequest: start,
	}
}
func simpleRuntimeTask(task *iapiserver.AtomicTask) workflowruntime.Task {
	return workflowruntime.Task{Name: task.FunctionRef, ReferenceName: safeName(task.ChildKey + "_" + shortID(task.ID)), Type: "SIMPLE", Input: map[string]any{"atomic_task_id": task.ID, "arguments": task.Arguments}}
}

func validateScheduleRequest(req *iapiserver.TaskScheduleCreateRequest) error {
	if req.ExecutionMode != "" && req.ExecutionMode != iapiserver.TaskScheduleModeMaterialized {
		return errors.NewStatusF(code.ErrTaskSystemScheduleOperationRestricted, "public schedule creation only accepts materialized mode")
	}
	if req.Target.Type != iapiserver.TaskScheduleTargetAtomic && req.Target.Type != iapiserver.TaskScheduleTargetGroup && req.Target.Type != iapiserver.TaskScheduleTargetDAG {
		return errors.NewStatusF(code.ErrTaskScheduleInvalid, "schedule target is invalid")
	}
	if req.TimeZone == "" {
		return errors.NewStatusF(code.ErrTaskScheduleInvalid, "schedule time zone is required")
	}
	if _, err := time.LoadLocation(req.TimeZone); err != nil {
		return errors.NewStatusF(code.ErrTaskScheduleInvalid, "schedule time zone is invalid")
	}
	switch req.TriggerType {
	case iapiserver.TaskScheduleTriggerCron:
		if len(strings.Fields(req.CronExpression)) != 6 || !req.RunAt.IsZero() {
			return errors.NewStatusF(code.ErrTaskScheduleInvalid, "schedule cron must use six fields")
		}
	case iapiserver.TaskScheduleTriggerRunAt:
		if req.RunAt.IsZero() || req.CronExpression != "" {
			return errors.NewStatusF(code.ErrTaskScheduleInvalid, "schedule run at is invalid")
		}
	default:
		return errors.NewStatusF(code.ErrTaskScheduleInvalid, "schedule trigger type is invalid")
	}
	return nil
}
func runtimeError(err error) error {
	if stderrors.Is(err, workflowruntime.ErrUnavailable) {
		return errors.NewStatus(code.ErrWorkflowRuntimeUnavailable, err.Error())
	}
	return errors.NewStatus(code.ErrWorkflowRuntimeRejected, err.Error())
}
func stableRuntimeKey(projectID, namespace, scope, key, fallback string) string {
	if scope == "" || key == "" {
		return fallback
	}
	return strings.Join([]string{projectID, namespace, scope, key}, ":")
}

var unsafeName = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func safeName(value string) string {
	value = unsafeName.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return "task"
	}
	return value
}
func shortID(value string) string {
	value = strings.ReplaceAll(value, "-", "")
	if len(value) > 12 {
		return value[:12]
	}
	return value
}
func definitionHash(value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func atomicTaskIDs(items []*iapiserver.AtomicTask) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func taskGroupIDs(items []*iapiserver.TaskGroup) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func dagTaskGroupIDs(items []*iapiserver.DAGTaskGroup) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func scheduleTemplateSummary(schedule *iapiserver.TaskSchedule) *iapiserver.TaskTargetSummary {
	if schedule == nil || schedule.ExecutionMode == iapiserver.TaskScheduleModeReconcile {
		return nil
	}
	summary := &iapiserver.TaskTargetSummary{Type: schedule.Target.Type, Name: schedule.Name}
	raw, err := json.Marshal(schedule.Target.Template)
	if err != nil {
		return summary
	}
	switch schedule.Target.Type {
	case iapiserver.TaskScheduleTargetAtomic:
		var target iapiserver.AtomicTaskCreateRequest
		if json.Unmarshal(raw, &target) == nil {
			summary.Name = target.Name
			if summary.Name == "" {
				summary.Name = target.Key
			}
			summary.FunctionRef = target.FunctionRef
			summary.ApplicationRunID = target.ApplicationRunID
			summary.CanvasRunID = target.CanvasRunID
			summary.CanvasNodeRunID = target.CanvasNodeRunID
		}
	case iapiserver.TaskScheduleTargetGroup:
		var target iapiserver.TaskGroupCreateRequest
		if json.Unmarshal(raw, &target) == nil {
			summary.Name = target.Name
			summary.TaskCount = len(target.Tasks)
		}
	case iapiserver.TaskScheduleTargetDAG:
		var target iapiserver.DAGTaskGroupCreateRequest
		if json.Unmarshal(raw, &target) == nil {
			summary.Name = target.Name
			summary.TaskCount = len(target.Nodes)
		}
	}
	if summary.Name == "" {
		summary.Name = schedule.Name
	}
	return summary
}

func atomicTargetSummary(item *iapiserver.AtomicTask) *iapiserver.TaskTargetSummary {
	return &iapiserver.TaskTargetSummary{Type: iapiserver.TaskScheduleTargetAtomic, ID: item.ID, Name: item.Name, Status: item.Status, Progress: item.Progress, FunctionRef: item.FunctionRef, ApplicationRunID: item.ApplicationRunID, CanvasRunID: item.CanvasRunID, CanvasNodeRunID: item.CanvasNodeRunID}
}

func groupTargetSummary(item *iapiserver.TaskGroup) *iapiserver.TaskTargetSummary {
	return &iapiserver.TaskTargetSummary{Type: iapiserver.TaskScheduleTargetGroup, ID: item.ID, Name: item.Name, Status: item.Status, Progress: item.Progress, TaskCount: item.Summary.Total}
}

func dagTargetSummary(item *iapiserver.DAGTaskGroup) *iapiserver.TaskTargetSummary {
	return &iapiserver.TaskTargetSummary{Type: iapiserver.TaskScheduleTargetDAG, ID: item.ID, Name: item.Name, Status: item.Status, Progress: item.Progress, TaskCount: item.Summary.Total}
}

func (s *taskCenterService) attachExecutionTargets(ctx context.Context, schedule *iapiserver.TaskSchedule, executions []*iapiserver.TaskScheduleExecution) error {
	schedules := map[string]*iapiserver.TaskSchedule{}
	if schedule != nil {
		schedules[schedule.ID] = schedule
	}
	return s.attachExecutionTargetsForSchedules(ctx, schedules, executions)
}

func (s *taskCenterService) attachExecutionTargetsForSchedules(ctx context.Context, schedules map[string]*iapiserver.TaskSchedule, executions []*iapiserver.TaskScheduleExecution) error {
	if len(executions) == 0 {
		return nil
	}
	idsByType := map[string][]string{
		iapiserver.TaskScheduleTargetAtomic: {},
		iapiserver.TaskScheduleTargetGroup:  {},
		iapiserver.TaskScheduleTargetDAG:    {},
	}
	for _, execution := range executions {
		if execution.TargetID != "" {
			idsByType[execution.TargetType] = append(idsByType[execution.TargetType], execution.TargetID)
		}
	}

	targets := make(map[string]*iapiserver.TaskTargetSummary)
	atomicTasks, err := s.store.GetAtomicTasksByIDs(ctx, idsByType[iapiserver.TaskScheduleTargetAtomic])
	if err != nil {
		return err
	}
	for _, item := range atomicTasks {
		targets[item.ID] = atomicTargetSummary(item)
	}
	groups, err := s.store.GetTaskGroupsByIDs(ctx, idsByType[iapiserver.TaskScheduleTargetGroup])
	if err != nil {
		return err
	}
	for _, item := range groups {
		targets[item.ID] = groupTargetSummary(item)
	}
	dags, err := s.store.GetDAGTaskGroupsByIDs(ctx, idsByType[iapiserver.TaskScheduleTargetDAG])
	if err != nil {
		return err
	}
	for _, item := range dags {
		targets[item.ID] = dagTargetSummary(item)
	}

	for _, execution := range executions {
		if execution.ExecutionMode == iapiserver.TaskScheduleModeReconcile {
			continue
		}
		if target := targets[execution.TargetID]; target != nil {
			execution.TargetSummary = target
			continue
		}
		fallback := scheduleTemplateSummary(schedules[execution.ScheduleID])
		if fallback == nil {
			continue
		}
		fallback.ID = execution.TargetID
		fallback.Type = execution.TargetType
		execution.TargetSummary = fallback
	}
	return nil
}

func (s *taskCenterService) attachLatestScheduleExecutions(ctx context.Context, schedules []*iapiserver.TaskSchedule) error {
	ids := make([]string, 0, len(schedules))
	byID := make(map[string]*iapiserver.TaskSchedule, len(schedules))
	for _, schedule := range schedules {
		ids = append(ids, schedule.ID)
		byID[schedule.ID] = schedule
	}
	latest, err := s.store.ListLatestScheduleExecutions(ctx, ids)
	if err != nil {
		return err
	}
	executions := make([]*iapiserver.TaskScheduleExecution, 0, len(latest))
	for _, execution := range latest {
		executions = append(executions, execution)
	}
	if err := s.attachExecutionTargetsForSchedules(ctx, byID, executions); err != nil {
		return err
	}
	for scheduleID, execution := range latest {
		byID[scheduleID].LastExecution = execution
	}
	return nil
}

var _ TaskCenterSrv = (*taskCenterService)(nil)

func taskActor(ctx context.Context) string {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err == nil && user != nil && user.ID != "" {
		return user.ID
	}
	return iapiserver.DefaultTaskCenterCreatedBy
}
func applyTaskScope(ctx context.Context, projectID, namespace, createdBy *string) {
	if *projectID == "" {
		*projectID = iapiserver.DefaultTaskCenterProjectID
	}
	if *namespace == "" {
		*namespace = iapiserver.DefaultTaskCenterNamespace
	}
	*createdBy = taskActor(ctx)
}

func applyTaskScheduleScope(ctx context.Context, req *iapiserver.TaskScheduleListRequest) {
	applyTaskScope(ctx, &req.ProjectID, &req.Namespace, &req.CreatedBy)
	// 系统健康计划归 TaskWorker 所有；系统管理员需要在同一计划列表中查看和排障。
	req.IncludeSystem = req.CreatedBy == "system-admin"
}

func canReadTaskSchedule(ctx context.Context, schedule *iapiserver.TaskSchedule) bool {
	return canReadTaskCreatedBy(ctx, schedule.CreatedBy)
}

func canReadTaskCreatedBy(ctx context.Context, createdBy string) bool {
	actor := taskActor(ctx)
	return createdBy == actor || (actor == "system-admin" && createdBy == iapiserver.DefaultTaskCenterCreatedBy)
}
