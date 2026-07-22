package workflowruntime

import (
	"context"
	"errors"
	"time"
)

var (
	ErrUnavailable       = errors.New("workflow runtime unavailable")
	ErrExecutionNotFound = errors.New("workflow execution not found")
	ErrTaskLogNotFound   = errors.New("workflow task log not found")
)

type Task struct {
	Name              string
	ReferenceName     string
	Type              string
	Input             map[string]any
	ForkTasks         [][]Task
	JoinOn            []string
	DynamicTasksParam string
	DynamicInputParam string
}

type Definition struct {
	Name           string
	Version        int
	Description    string
	Tasks          []Task
	Output         map[string]any
	TimeoutSeconds int
}

type Binding struct {
	DefinitionName    string
	DefinitionVersion int
	Revision          string
}

type StartRequest struct {
	DefinitionName    string
	DefinitionVersion int
	CorrelationID     string
	IdempotencyKey    string
	Input             map[string]any
}

type Execution struct {
	ID             string
	DefinitionName string
	Status         string
	Output         map[string]any
	StartedAt      time.Time
	CompletedAt    time.Time
	FailureReason  string
	Tasks          []ExecutionTask
}

func isTerminalExecutionStatus(status string) bool {
	switch status {
	case "COMPLETED", "FAILED", "TERMINATED", "TIMED_OUT", "CANCELED":
		return true
	default:
		return false
	}
}

type ExecutionTask struct {
	ID            string
	ReferenceName string
	TaskType      string
	Status        string
	RetryCount    int
	Input         map[string]any
	Output        map[string]any
	FailureReason string
	StartedAt     time.Time
	CompletedAt   time.Time
}

type Schedule struct {
	Name           string
	CronExpression string
	TimeZone       string
	Paused         bool
	RunCatchup     bool
	StartAt        time.Time
	EndAt          time.Time
	StartRequest   StartRequest
}

type WorkerTask struct {
	AtomicTaskID  string
	WorkflowID    string
	RuntimeTaskID string
	FunctionRef   string
	RetryCount    int
	RetriedTaskID string
	Arguments     map[string]any
	Logger        TaskLogger
}

// Log 通过运行时绑定的 best-effort logger 记录当前 Attempt 日志；缺少 logger 时安全忽略。
func (t WorkerTask) Log(ctx context.Context, entry TaskLogEntry) {
	if t.Logger != nil {
		t.Logger.Log(ctx, entry)
	}
}

type Handler func(context.Context, WorkerTask) (map[string]any, error)

type DefinitionRegistrar interface {
	RegisterDefinition(context.Context, Definition) (Binding, error)
}

type ExecutionManager interface {
	StartExecution(context.Context, StartRequest) (Execution, error)
	GetExecution(context.Context, string) (Execution, error)
	CancelExecution(context.Context, string, string) error
	RetryExecution(context.Context, string) error
	ListNonTerminalExecutions(context.Context, int) ([]Execution, error)
	ListTerminalExecutions(context.Context, string, time.Time, int) ([]Execution, error)
	// DeleteTerminalExecution 仅删除指定终态运行历史，由 RECONCILE retention 调用。
	DeleteTerminalExecution(context.Context, string) error
}

type ScheduleManager interface {
	SaveSchedule(context.Context, Schedule) error
	PauseSchedule(context.Context, string) error
	ResumeSchedule(context.Context, string) error
	DeleteSchedule(context.Context, string) error
}

// TaskLogManager 追加并读取 runtime task 隔离的执行日志正文。
type TaskLogManager interface {
	AppendTaskLog(context.Context, string, TaskLogEntry) error
	ListTaskLogs(context.Context, string) ([]TaskLogEntry, error)
}

type WorkerRegistrar interface {
	RegisterHandler(string, int, Handler) error
	Close() error
}

type WorkflowRuntime interface {
	DefinitionRegistrar
	ExecutionManager
	ScheduleManager
	TaskLogManager
	WorkerRegistrar
}

type UnavailableRuntime struct{}

func (UnavailableRuntime) RegisterDefinition(context.Context, Definition) (Binding, error) {
	return Binding{}, ErrUnavailable
}
func (UnavailableRuntime) StartExecution(context.Context, StartRequest) (Execution, error) {
	return Execution{}, ErrUnavailable
}
func (UnavailableRuntime) GetExecution(context.Context, string) (Execution, error) {
	return Execution{}, ErrUnavailable
}
func (UnavailableRuntime) CancelExecution(context.Context, string, string) error {
	return ErrUnavailable
}
func (UnavailableRuntime) RetryExecution(context.Context, string) error { return ErrUnavailable }
func (UnavailableRuntime) ListNonTerminalExecutions(context.Context, int) ([]Execution, error) {
	return nil, ErrUnavailable
}
func (UnavailableRuntime) ListTerminalExecutions(context.Context, string, time.Time, int) ([]Execution, error) {
	return nil, ErrUnavailable
}
func (UnavailableRuntime) DeleteTerminalExecution(context.Context, string) error {
	return ErrUnavailable
}
func (UnavailableRuntime) AppendTaskLog(context.Context, string, TaskLogEntry) error {
	return ErrUnavailable
}
func (UnavailableRuntime) ListTaskLogs(context.Context, string) ([]TaskLogEntry, error) {
	return nil, ErrUnavailable
}
func (UnavailableRuntime) SaveSchedule(context.Context, Schedule) error { return ErrUnavailable }
func (UnavailableRuntime) PauseSchedule(context.Context, string) error  { return ErrUnavailable }
func (UnavailableRuntime) ResumeSchedule(context.Context, string) error { return ErrUnavailable }
func (UnavailableRuntime) DeleteSchedule(context.Context, string) error { return ErrUnavailable }
func (UnavailableRuntime) RegisterHandler(string, int, Handler) error   { return ErrUnavailable }
func (UnavailableRuntime) Close() error                                 { return nil }

var _ WorkflowRuntime = UnavailableRuntime{}
