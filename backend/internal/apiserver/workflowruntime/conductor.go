package workflowruntime

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/conductor-sdk/conductor-go/sdk/client"
	"github.com/conductor-sdk/conductor-go/sdk/model"
	"github.com/conductor-sdk/conductor-go/sdk/settings"
	"github.com/conductor-sdk/conductor-go/sdk/worker"
	"github.com/conductor-sdk/conductor-go/sdk/workflow/executor"
)

type ConductorConfig struct {
	BaseURL      string
	AuthKey      string
	AuthSecret   string
	HTTPTimeout  time.Duration
	PollInterval time.Duration
}

type ConductorRuntime struct {
	executor  *executor.WorkflowExecutor
	apiClient *client.APIClient
	runner    *worker.TaskRunner

	mu              sync.Mutex
	registeredTasks map[string]struct{}
	pollInterval    time.Duration
}

func NewConductor(config ConductorConfig) (*ConductorRuntime, error) {
	if config.BaseURL == "" {
		return nil, fmt.Errorf("conductor base url is required")
	}
	if config.HTTPTimeout <= 0 {
		config.HTTPTimeout = 30 * time.Second
	}
	if config.PollInterval <= 0 {
		config.PollInterval = 250 * time.Millisecond
	}
	httpSettings := settings.NewHttpSettings(config.BaseURL)
	httpSettings.Timeout = config.HTTPTimeout
	authSettings := settings.NewAuthenticationSettings(config.AuthKey, config.AuthSecret)
	apiClient := client.NewAPIClient(authSettings, httpSettings)
	return &ConductorRuntime{
		executor: executor.NewWorkflowExecutor(apiClient), apiClient: apiClient,
		runner: worker.NewTaskRunnerWithApiClient(apiClient), registeredTasks: make(map[string]struct{}),
		pollInterval: config.PollInterval,
	}, nil
}

func (r *ConductorRuntime) RegisterDefinition(ctx context.Context, definition Definition) (Binding, error) {
	wf := &model.WorkflowDef{
		Name: definition.Name, Version: int32(definition.Version), Description: definition.Description,
		SchemaVersion: 2, Restartable: true, TimeoutPolicy: "TIME_OUT_WF",
		TimeoutSeconds: int64(definition.TimeoutSeconds), OutputParameters: definition.Output,
	}
	wf.Tasks = make([]model.WorkflowTask, 0, len(definition.Tasks))
	for _, task := range definition.Tasks {
		wf.Tasks = append(wf.Tasks, conductorTask(task))
	}
	var response any
	if _, err := r.apiClient.Put(ctx, "/metadata/workflow", []model.WorkflowDef{*wf}, &response); err != nil {
		return Binding{}, fmt.Errorf("register conductor workflow: %w", err)
	}
	return Binding{DefinitionName: definition.Name, DefinitionVersion: definition.Version, Revision: fmt.Sprintf("%s:%d", definition.Name, definition.Version)}, nil
}

func (r *ConductorRuntime) StartExecution(ctx context.Context, request StartRequest) (Execution, error) {
	id, err := r.executor.StartWorkflowWithContext(ctx, &model.StartWorkflowRequest{
		Name: request.DefinitionName, Version: int32(request.DefinitionVersion), CorrelationId: request.CorrelationID,
		Input: request.Input, IdempotencyKey: request.IdempotencyKey, IdempotencyStrategy: model.ReturnExisting,
	})
	if err != nil {
		return Execution{}, fmt.Errorf("start conductor workflow: %w", err)
	}
	return Execution{ID: id, Status: string(model.RunningWorkflow), StartedAt: time.Now()}, nil
}

func (r *ConductorRuntime) GetExecution(ctx context.Context, id string) (Execution, error) {
	wf, err := r.executor.GetWorkflowWithContext(ctx, id, true)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such workflow") {
			return Execution{}, fmt.Errorf("%w: %v", ErrExecutionNotFound, err)
		}
		return Execution{}, fmt.Errorf("get conductor workflow: %w", err)
	}
	return fromConductorWorkflow(wf), nil
}

func (r *ConductorRuntime) CancelExecution(ctx context.Context, id, reason string) error {
	if err := r.executor.TerminateWithContext(ctx, id, reason); err != nil {
		return fmt.Errorf("terminate conductor workflow: %w", err)
	}
	return nil
}

func (r *ConductorRuntime) RetryExecution(ctx context.Context, id string) error {
	if err := r.executor.RetryWithContext(ctx, id, true); err != nil {
		return fmt.Errorf("retry conductor workflow: %w", err)
	}
	return nil
}

func (r *ConductorRuntime) ListNonTerminalExecutions(ctx context.Context, limit int) ([]Execution, error) {
	if limit <= 0 {
		limit = 100
	}
	items := make([]Execution, 0, limit)
	for _, status := range []string{"RUNNING", "PAUSED"} {
		summaries, err := r.executor.SearchWithContext(ctx, 0, int32(limit-len(items)), fmt.Sprintf("status=%s", status), "")
		if err != nil {
			return nil, fmt.Errorf("search conductor workflows: %w", err)
		}
		for _, summary := range summaries {
			items = append(items, Execution{ID: summary.WorkflowId, Status: string(summary.Status), StartedAt: fromSummaryTime(summary.StartTime), CompletedAt: fromSummaryTime(summary.EndTime)})
			if len(items) >= limit {
				return items, nil
			}
		}
	}
	return items, nil
}

func (r *ConductorRuntime) SaveSchedule(ctx context.Context, schedule Schedule) error {
	req := schedule.StartRequest
	body := struct {
		model.SaveScheduleRequest
		ZoneID string `json:"zoneId,omitempty"`
	}{SaveScheduleRequest: model.SaveScheduleRequest{
		Name: schedule.Name, CronExpression: schedule.CronExpression, Paused: schedule.Paused,
		RunCatchupScheduleInstances: schedule.RunCatchup, ScheduleStartTime: schedule.StartAt.UnixMilli(),
		StartWorkflowRequest: &model.StartWorkflowRequest{Name: req.DefinitionName, Version: int32(req.DefinitionVersion), CorrelationId: req.CorrelationID, Input: req.Input, IdempotencyKey: req.IdempotencyKey, IdempotencyStrategy: model.ReturnExisting},
	}, ZoneID: schedule.TimeZone}
	if !schedule.EndAt.IsZero() {
		body.ScheduleEndTime = schedule.EndAt.UnixMilli()
	}
	var response any
	if _, err := r.apiClient.Post(ctx, "/scheduler/schedules", body, &response); err != nil {
		return fmt.Errorf("save conductor schedule: %w", err)
	}
	return nil
}

func (r *ConductorRuntime) PauseSchedule(ctx context.Context, name string) error {
	_, err := r.apiClient.Put(ctx, "/scheduler/schedules/"+name+"/pause", nil, nil)
	if err != nil {
		return fmt.Errorf("pause conductor schedule: %w", err)
	}
	return nil
}
func (r *ConductorRuntime) ResumeSchedule(ctx context.Context, name string) error {
	_, err := r.apiClient.Put(ctx, "/scheduler/schedules/"+name+"/resume", nil, nil)
	if err != nil {
		return fmt.Errorf("resume conductor schedule: %w", err)
	}
	return nil
}
func (r *ConductorRuntime) DeleteSchedule(ctx context.Context, name string) error {
	_, err := r.apiClient.Delete(ctx, "/scheduler/schedules/"+name, nil, nil)
	if err != nil {
		return fmt.Errorf("delete conductor schedule: %w", err)
	}
	return nil
}

func (r *ConductorRuntime) RegisterHandler(functionRef string, concurrency int, handler Handler) error {
	if functionRef == "" || handler == nil {
		return fmt.Errorf("function ref and handler are required")
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.registeredTasks[functionRef]; exists {
		return fmt.Errorf("function ref %s is already registered", functionRef)
	}
	typed := worker.NewTypedWorker[workerInput, map[string]any](functionRef, func(ctx worker.TaskContext, input workerInput) (map[string]any, error) {
		return handler(ctx, WorkerTask{AtomicTaskID: input.AtomicTaskID, WorkflowID: ctx.WorkflowInstanceID(), RuntimeTaskID: ctx.TaskID(), FunctionRef: functionRef, RetryCount: ctx.RetryCount(), RetriedTaskID: ctx.RetriedTaskID(), Arguments: input.Arguments})
	}, worker.WithBatchSize(concurrency), worker.WithPollInterval(r.pollInterval))
	if err := r.runner.RegisterWorker(typed); err != nil {
		return fmt.Errorf("register conductor worker: %w", err)
	}
	r.registeredTasks[functionRef] = struct{}{}
	return nil
}

func (r *ConductorRuntime) Close() error {
	r.mu.Lock()
	names := make([]string, 0, len(r.registeredTasks))
	for name := range r.registeredTasks {
		names = append(names, name)
	}
	r.mu.Unlock()
	for _, name := range names {
		r.runner.Shutdown(name)
	}
	r.runner.WaitWorkers()
	return nil
}

type workerInput struct {
	AtomicTaskID string         `json:"atomic_task_id"`
	Arguments    map[string]any `json:"arguments"`
}

func conductorTask(task Task) model.WorkflowTask {
	ret := model.WorkflowTask{Name: task.Name, TaskReferenceName: task.ReferenceName, Type_: task.Type, InputParameters: task.Input, JoinOn: task.JoinOn, DynamicForkTasksParam: task.DynamicTasksParam, DynamicForkTasksInputParamName: task.DynamicInputParam}
	if len(task.ForkTasks) > 0 {
		ret.ForkTasks = make([][]model.WorkflowTask, len(task.ForkTasks))
		for i, branch := range task.ForkTasks {
			for _, child := range branch {
				ret.ForkTasks[i] = append(ret.ForkTasks[i], conductorTask(child))
			}
		}
	}
	return ret
}

func fromConductorWorkflow(wf *model.Workflow) Execution {
	if wf == nil {
		return Execution{}
	}
	ret := Execution{ID: wf.WorkflowId, Status: string(wf.Status), Output: wf.Output, StartedAt: fromMillis(wf.StartTime), CompletedAt: fromMillis(wf.EndTime), FailureReason: wf.ReasonForIncompletion}
	ret.Tasks = make([]ExecutionTask, 0, len(wf.Tasks))
	for _, task := range wf.Tasks {
		ret.Tasks = append(ret.Tasks, ExecutionTask{ID: task.TaskId, ReferenceName: task.ReferenceTaskName, TaskType: task.TaskDefName, Status: string(task.Status), RetryCount: int(task.RetryCount), Input: task.InputData, Output: task.OutputData, FailureReason: task.ReasonForIncompletion, StartedAt: fromMillis(task.StartTime), CompletedAt: fromMillis(task.EndTime)})
	}
	return ret
}

func fromMillis(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value)
}

func fromSummaryTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	if millis, err := strconv.ParseInt(value, 10, 64); err == nil {
		return fromMillis(millis)
	}
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}

var _ WorkflowRuntime = (*ConductorRuntime)(nil)
