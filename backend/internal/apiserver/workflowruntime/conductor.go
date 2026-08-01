package workflowruntime

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/conductor-sdk/conductor-go/sdk/client"
	"github.com/conductor-sdk/conductor-go/sdk/model"
	"github.com/conductor-sdk/conductor-go/sdk/settings"
	"github.com/conductor-sdk/conductor-go/sdk/worker"
	"github.com/conductor-sdk/conductor-go/sdk/workflow/executor"
	"github.com/google/uuid"
)

type ConductorConfig struct {
	BaseURL      string
	AuthKey      string
	AuthSecret   string
	HTTPTimeout  time.Duration
	PollInterval time.Duration
}

type ConductorRuntime struct {
	executor   *executor.WorkflowExecutor
	apiClient  *client.APIClient
	taskClient client.TaskClient
	runner     *worker.TaskRunner

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
	registerTaskLogMetrics()
	return &ConductorRuntime{
		executor: executor.NewWorkflowExecutor(apiClient), apiClient: apiClient,
		taskClient: client.NewTaskClient(apiClient),
		runner:     worker.NewTaskRunnerWithApiClient(apiClient), registeredTasks: make(map[string]struct{}),
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

func (r *ConductorRuntime) ListTerminalExecutions(ctx context.Context, definition string, completedBefore time.Time, limit int) ([]Execution, error) {
	if limit <= 0 {
		limit = 100
	}
	items := make([]Execution, 0, limit)
	for _, status := range []string{"COMPLETED", "FAILED", "TERMINATED", "TIMED_OUT"} {
		const pageSize int32 = 200
		for start := int32(0); len(items) < limit; start += pageSize {
			summaries, err := r.executor.SearchWithContext(ctx, start, pageSize, fmt.Sprintf("status = %s AND workflowType = %s", status, definition), "")
			if err != nil {
				return nil, fmt.Errorf("search terminal conductor workflows: %w", err)
			}
			for _, summary := range summaries {
				completedAt := fromSummaryTime(summary.EndTime)
				if summary.WorkflowType != definition || completedAt.IsZero() || !completedAt.Before(completedBefore) {
					continue
				}
				items = append(items, Execution{ID: summary.WorkflowId, DefinitionName: summary.WorkflowType, Status: string(summary.Status), StartedAt: fromSummaryTime(summary.StartTime), CompletedAt: completedAt})
				if len(items) >= limit {
					return items, nil
				}
			}
			if len(summaries) < int(pageSize) {
				break
			}
		}
	}
	return items, nil
}

// DeleteTerminalExecution 先读取状态再调用 Conductor 官方 remove API，防止误删运行中 execution。
func (r *ConductorRuntime) DeleteTerminalExecution(ctx context.Context, id string) error {
	execution, err := r.GetExecution(ctx, id)
	if err != nil {
		return err
	}
	if !isTerminalExecutionStatus(execution.Status) {
		return fmt.Errorf("remove conductor workflow: execution is not terminal")
	}
	if err := r.executor.RemoveWorkflow(id); err != nil {
		return fmt.Errorf("remove conductor workflow: %w", err)
	}
	return nil
}

// AppendTaskLog 将经过 Task Center 约束的结构化日志写入对应 Conductor runtime task。
func (r *ConductorRuntime) AppendTaskLog(ctx context.Context, runtimeTaskID string, entry TaskLogEntry) error {
	encoded, normalized, err := encodeTaskLog(entry)
	if err != nil {
		taskLogWriteFailures.WithLabelValues("conductor").Inc()
		return fmt.Errorf("encode conductor task log: %w", err)
	}
	response, err := r.taskClient.Log(ctx, encoded, runtimeTaskID)
	if err != nil {
		taskLogWriteFailures.WithLabelValues("conductor").Inc()
		if taskLogResponseNotFound(response, err) {
			return fmt.Errorf("%w: %v", ErrTaskLogNotFound, err)
		}
		return fmt.Errorf("append conductor task log: %w", err)
	}
	taskLogEntriesWritten.WithLabelValues("conductor", normalized.Source, normalized.Level).Inc()
	return nil
}

// ListTaskLogs 读取、兼容解码、脱敏并稳定排序 Conductor runtime task 日志。
func (r *ConductorRuntime) ListTaskLogs(ctx context.Context, runtimeTaskID string) ([]TaskLogEntry, error) {
	items, response, err := r.taskClient.GetTaskLogs(ctx, runtimeTaskID)
	if err != nil {
		if taskLogResponseNotFound(response, err) {
			taskLogReadFailures.WithLabelValues("conductor", "not_found").Inc()
			return nil, fmt.Errorf("%w: %v", ErrTaskLogNotFound, err)
		}
		taskLogReadFailures.WithLabelValues("conductor", "unavailable").Inc()
		return nil, fmt.Errorf("get conductor task logs: %w", err)
	}
	logs := make([]TaskLogEntry, 0, len(items))
	for _, item := range items {
		logs = append(logs, decodeTaskLog(item.Log, fromMillis(item.CreatedTime)))
	}
	return normalizeTaskLogs(logs), nil
}

func taskLogResponseNotFound(response *http.Response, err error) bool {
	if response != nil && response.StatusCode == http.StatusNotFound {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such task") || strings.Contains(message, "task not found")
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
	typed := worker.NewTypedWorker[workerInput, any](functionRef, func(ctx worker.TaskContext, input workerInput) (any, error) {
		logger := newBoundTaskLogger("conductor", ctx.TaskID(), r)
		logger.Log(ctx, LifecycleLog("attempt.started", TaskLogLevelInfo, "Execution attempt started."))
		output, err := handler(ctx, WorkerTask{
			AtomicTaskID: input.AtomicTaskID, WorkflowID: ctx.WorkflowInstanceID(), RuntimeTaskID: ctx.TaskID(), FunctionRef: functionRef,
			RetryCount: ctx.RetryCount(), RetriedTaskID: ctx.RetriedTaskID(), Arguments: input.Arguments, Logger: logger,
			checkpointLoader: func(loadCtx context.Context) (map[string]any, error) {
				return loadTaskCheckpoint(loadCtx, ctx.TaskID(), ctx.RetriedTaskID(), func(loadCtx context.Context, taskID string) (map[string]any, error) {
					task, _, loadErr := r.taskClient.GetTask(loadCtx, taskID)
					return task.OutputData, loadErr
				})
			},
		})
		if err != nil {
			logger.Log(ctx, LifecycleLog("attempt.failed", TaskLogLevelError, "Execution attempt failed."))
			return nil, err
		}
		if err := prepareDynamicForkOutput(output, input, r.isRegisteredTask); err != nil {
			logger.Log(ctx, LifecycleLog("attempt.failed", TaskLogLevelError, "Dynamic Fork output validation failed."))
			return nil, err
		}
		if inProgress, _ := output["in_progress"].(bool); inProgress {
			logger.Log(ctx, LifecycleLog("attempt.waiting", TaskLogLevelInfo, "Execution is waiting for the next runtime callback."))
			seconds := int64(1)
			switch value := output["callback_after_seconds"].(type) {
			case int:
				seconds = int64(value)
			case int64:
				seconds = value
			case float64:
				seconds = int64(value)
			}
			delete(output, "in_progress")
			delete(output, "callback_after_seconds")
			return &model.TaskResult{WorkflowInstanceId: ctx.WorkflowInstanceID(), TaskId: ctx.TaskID(), Status: model.InProgressTask, CallbackAfterSeconds: seconds, OutputData: output}, nil
		}
		logger.Log(ctx, LifecycleLog("attempt.succeeded", TaskLogLevelInfo, "Execution attempt succeeded."))
		return output, nil
	}, worker.WithBatchSize(concurrency), worker.WithPollInterval(r.pollInterval))
	if err := r.runner.RegisterWorker(typed); err != nil {
		return fmt.Errorf("register conductor worker: %w", err)
	}
	r.registeredTasks[functionRef] = struct{}{}
	return nil
}

func loadTaskCheckpoint(ctx context.Context, taskID, retriedTaskID string, load func(context.Context, string) (map[string]any, error)) (map[string]any, error) {
	checkpoint, err := load(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("load workflow runtime task checkpoint: %w", err)
	}
	if len(checkpoint) != 0 || retriedTaskID == "" {
		return checkpoint, nil
	}
	checkpoint, err = load(ctx, retriedTaskID)
	if err != nil {
		return nil, fmt.Errorf("load retried workflow runtime task checkpoint: %w", err)
	}
	return checkpoint, nil
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
	AtomicTaskID    string         `json:"atomic_task_id"`
	DAGNodeKey      string         `json:"dag_node_key"`
	FunctionRef     string         `json:"function_ref"`
	MaxDynamicTasks int            `json:"max_dynamic_tasks"`
	Arguments       map[string]any `json:"arguments"`
}

func (r *ConductorRuntime) isRegisteredTask(functionRef string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, exists := r.registeredTasks[functionRef]
	return exists
}

// prepareDynamicForkOutput validates planner output and adds the deterministic identity used to project actual children.
func prepareDynamicForkOutput(output map[string]any, input workerInput, registered func(string) bool) error {
	if output == nil || input.AtomicTaskID == "" || input.DAGNodeKey == "" {
		return nil
	}
	tasksValue, hasTasks := output["dynamic_tasks"]
	inputsValue, hasInputs := output["dynamic_inputs"]
	if !hasTasks && !hasInputs {
		return nil
	}
	if !hasTasks || !hasInputs {
		return fmt.Errorf("dynamic_tasks and dynamic_inputs must be returned together")
	}
	tasks, ok := dynamicTaskMaps(tasksValue)
	if !ok {
		return fmt.Errorf("dynamic_tasks must be an array of task objects")
	}
	if input.MaxDynamicTasks < 1 || len(tasks) > input.MaxDynamicTasks {
		return fmt.Errorf("dynamic task count %d exceeds configured maximum %d", len(tasks), input.MaxDynamicTasks)
	}
	inputs, ok := dynamicInputMaps(inputsValue)
	if !ok {
		return fmt.Errorf("dynamic_inputs must be an object keyed by task reference")
	}
	output["dynamic_inputs"] = inputs
	seenReferences := make(map[string]struct{}, len(tasks))
	for index, task := range tasks {
		functionRef, _ := task["name"].(string)
		reference := dynamicTaskReference(task)
		if functionRef == "" || reference == "" {
			return fmt.Errorf("dynamic task at index %d requires name and task reference", index)
		}
		if _, duplicate := seenReferences[reference]; duplicate {
			return fmt.Errorf("dynamic task reference %q is duplicated", reference)
		}
		seenReferences[reference] = struct{}{}
		if registered == nil || !registered(functionRef) {
			return fmt.Errorf("dynamic task function %q is not registered", functionRef)
		}
		childInput, ok := inputs[reference].(map[string]any)
		if !ok || childInput == nil {
			return fmt.Errorf("dynamic input for reference %q must be an object", reference)
		}
		arguments, argumentsProvided := childInput["arguments"].(map[string]any)
		if _, exists := childInput["arguments"]; exists && !argumentsProvided {
			return fmt.Errorf("arguments for dynamic task reference %q must be an object", reference)
		}
		if arguments == nil {
			arguments = make(map[string]any, len(childInput))
			for key, value := range childInput {
				if key != "atomic_task_id" && key != "dag_node_key" && key != "function_ref" {
					arguments[key] = value
				}
			}
		}
		identity := fmt.Sprintf("%s:%s:%s:%d", input.AtomicTaskID, input.DAGNodeKey, reference, index)
		childInput["atomic_task_id"] = uuid.NewSHA1(uuid.NameSpaceOID, []byte(identity)).String()
		childInput["dag_node_key"] = input.DAGNodeKey
		childInput["function_ref"] = functionRef
		childInput["child_key"] = reference
		childInput["child_order"] = index
		childInput["arguments"] = arguments
		inputs[reference] = childInput
	}
	return nil
}

func dynamicInputMaps(value any) (map[string]any, bool) {
	switch inputs := value.(type) {
	case map[string]any:
		return inputs, true
	case map[string]map[string]any:
		result := make(map[string]any, len(inputs))
		for key, input := range inputs {
			result[key] = input
		}
		return result, true
	default:
		return nil, false
	}
}

func dynamicTaskMaps(value any) ([]map[string]any, bool) {
	switch tasks := value.(type) {
	case []map[string]any:
		return tasks, true
	case []any:
		result := make([]map[string]any, 0, len(tasks))
		for _, task := range tasks {
			mapped, ok := task.(map[string]any)
			if !ok {
				return nil, false
			}
			result = append(result, mapped)
		}
		return result, true
	default:
		return nil, false
	}
}

func dynamicTaskReference(task map[string]any) string {
	for _, key := range []string{"taskReferenceName", "task_reference_name", "reference_name"} {
		if value, _ := task[key].(string); value != "" {
			return value
		}
	}
	return ""
}

func conductorTask(task Task) model.WorkflowTask {
	ret := model.WorkflowTask{Name: task.Name, TaskReferenceName: task.ReferenceName, Type_: task.Type, InputParameters: task.Input, JoinOn: task.JoinOn, DynamicForkTasksParam: task.DynamicTasksParam, DynamicForkTasksInputParamName: task.DynamicInputParam}
	if task.Retry.MaxAttempts > 1 {
		backoff := task.Retry.BackoffType
		if backoff == "" {
			backoff = "FIXED"
		}
		definition := &model.TaskDef{Name: task.Name, RetryCount: int32(task.Retry.MaxAttempts - 1), RetryDelaySeconds: int32(task.Retry.RetryDelaySeconds), RetryLogic: backoff}
		if backoff == "EXPONENTIAL_BACKOFF" {
			definition.BackoffScaleFactor = 2
		}
		ret.TaskDefinition = definition
	}
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
	ret := Execution{ID: wf.WorkflowId, DefinitionName: wf.WorkflowName, Status: string(wf.Status), Output: wf.Output, StartedAt: fromMillis(wf.StartTime), CompletedAt: fromMillis(wf.EndTime), FailureReason: wf.ReasonForIncompletion}
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
