// +k8s:deepcopy-gen=package

package iapiserver

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	AtomicTaskStatusPending         = "PENDING"
	AtomicTaskStatusBlocked         = "BLOCKED"
	AtomicTaskStatusReady           = "READY"
	AtomicTaskStatusRunning         = "RUNNING"
	AtomicTaskStatusRetrying        = "RETRYING"
	AtomicTaskStatusCancelRequested = "CANCEL_REQUESTED"
	AtomicTaskStatusSuccess         = "SUCCESS"
	AtomicTaskStatusFailed          = "FAILED"
	AtomicTaskStatusCanceled        = "CANCELED"
	AtomicTaskStatusTimeout         = "TIMEOUT"
	AtomicTaskStatusSkipped         = "SKIPPED"
)

const (
	TaskAttemptStatusScheduled = "SCHEDULED"
	TaskAttemptStatusRunning   = "RUNNING"
	TaskAttemptStatusSuccess   = "SUCCESS"
	TaskAttemptStatusFailed    = "FAILED"
	TaskAttemptStatusCanceled  = "CANCELED"
	TaskAttemptStatusTimeout   = "TIMEOUT"
)

const (
	TaskGroupModeSerial   = "SERIAL"
	TaskGroupModeParallel = "PARALLEL"

	TaskGroupStatusPending         = "PENDING"
	TaskGroupStatusRunning         = "RUNNING"
	TaskGroupStatusCancelRequested = "CANCEL_REQUESTED"
	TaskGroupStatusSuccess         = "SUCCESS"
	TaskGroupStatusFailed          = "FAILED"
	TaskGroupStatusCanceled        = "CANCELED"
	TaskGroupStatusTimeout         = "TIMEOUT"
)

const (
	TaskOwnerTypeGroup    = "TASK_GROUP"
	TaskOwnerTypeDAGGroup = "DAG_TASK_GROUP"
	TaskOwnerTypeSchedule = "TASK_SCHEDULE"

	TaskScheduleTriggerCron      = "CRON"
	TaskScheduleTriggerRunAt     = "RUN_AT"
	TaskScheduleTargetAtomic     = "ATOMIC_TASK"
	TaskScheduleTargetGroup      = "TASK_GROUP"
	TaskScheduleTargetDAG        = "DAG_TASK_GROUP"
	TaskScheduleModeMaterialized = "MATERIALIZED"
	TaskScheduleModeReconcile    = "RECONCILE"
	TaskScheduleManagementUser   = "USER"
	TaskScheduleManagementSystem = "SYSTEM"

	TaskScheduleStatusActive    = "ACTIVE"
	TaskScheduleStatusPaused    = "PAUSED"
	TaskScheduleStatusCompleted = "COMPLETED"
	TaskScheduleStatusDeleted   = "DELETED"

	ScheduleExecutionStatusTriggered      = "TRIGGERED"
	ScheduleExecutionStatusRunning        = "RUNNING"
	ScheduleExecutionStatusSuccess        = "SUCCESS"
	ScheduleExecutionStatusFailed         = "FAILED"
	ScheduleExecutionStatusCanceled       = "CANCELED"
	ScheduleExecutionStatusSkippedOverlap = "SKIPPED_OVERLAP"
	ScheduleExecutionStatusTriggerFailed  = "TRIGGER_FAILED"

	TaskSchedulePolicySkip = "SKIP"
)

const (
	RuntimeProjectionStatusPending = "PENDING"
	RuntimeProjectionStatusApplied = "APPLIED"
	RuntimeProjectionStatusIgnored = "IGNORED"
	RuntimeProjectionStatusFailed  = "FAILED"

	TaskCenterEventAtomicTaskCreated         = "atomic_task_created"
	TaskCenterEventAtomicTaskStatusChanged   = "atomic_task_status_changed"
	TaskCenterEventGroupStatusChanged        = "task_group_status_changed"
	TaskCenterEventScheduleExecutionRecorded = "task_schedule_execution_recorded"
	TaskCenterEventProjectionReconciled      = "atomic_tasktime_projection_reconciled"

	DefaultTaskCenterProjectID = "default"
	DefaultTaskCenterNamespace = "default"
	DefaultTaskCenterCreatedBy = "system"
	MaxTaskGraphNodes          = 1000
	MaxTaskGraphEdges          = 5000
	MaxDynamicForkTasks        = 1000
	TaskNameSourceUser         = "USER"
	TaskNameSourceSystem       = "SYSTEM"
)

const (
	DAGTriggerAPI         = "API"
	DAGTriggerSchedule    = "SCHEDULE"
	DAGTriggerCanvas      = "CANVAS"
	DAGTriggerDomainEvent = "DOMAIN_EVENT"
	DAGTriggerRetry       = "RETRY"

	TaskExecutorWorker      = "WORKER"
	TaskExecutorApplication = "APPLICATION_EXECUTOR"
	TaskExecutorSystem      = "SYSTEM"
)

// RetryPolicy controls Conductor retries for one AtomicTask.
type RetryPolicy struct {
	MaxAttempts          int    `json:"max_attempts,omitempty"`
	RetryDelaySeconds    int    `json:"retry_delay_seconds,omitempty"`
	BackoffType          string `json:"backoff_type,omitempty"`
	MaxRetryDelaySeconds int    `json:"max_retry_delay_seconds,omitempty"`
}

// TimeoutPolicy defines per-attempt and whole-execution limits in seconds.
type TimeoutPolicy struct {
	PerAttemptTimeoutSeconds int `json:"per_attempt_timeout_seconds,omitempty"`
	OverallTimeoutSeconds    int `json:"overall_timeout_seconds,omitempty"`
}

// TaskError is a stable, user-safe execution failure summary.
// +k8s:deepcopy-gen=true
type TaskError struct {
	Code       string          `json:"code,omitempty"`
	Message    string          `json:"message,omitempty"`
	Detail     string          `json:"detail,omitempty"`
	Retryable  bool            `json:"retryable,omitempty"`
	OccurredAt imachinery.Time `json:"occurred_at,omitempty"`
}

// SystemNameSpec 是只允许后端内部创建路径设置的稳定系统名称引用。
// +k8s:deepcopy-gen=true
type SystemNameSpec struct {
	Key    string            `json:"-"`
	Params map[string]string `json:"-"`
}

// TaskNameMeta 持久化名称来源和系统名称引用；多语言投影本身不入库。
// +k8s:deepcopy-gen=true
type TaskNameMeta struct {
	NameSource             string            `json:"-" gorm:"column:name_source;type:varchar(16);not null;default:'USER'"`
	SystemNameKey          string            `json:"-" gorm:"column:system_name_key;type:varchar(256);not null;default:''"`
	SystemNameParams       map[string]string `json:"-" gorm:"-"`
	SystemNameParamsShadow string            `json:"-" gorm:"column:system_name_params_json;type:text;not null;default:'{}'"`
	NameI18n               map[string]string `json:"name_i18n,omitempty" gorm:"-"`
}

func (m *TaskNameMeta) normalize() {
	if m.NameSource == "" {
		m.NameSource = TaskNameSourceUser
	}
	if m.SystemNameParams == nil {
		m.SystemNameParams = map[string]string{}
	}
}

func (m *TaskNameMeta) marshal() error {
	m.normalize()
	return marshalJSONFields(jsonField{m.SystemNameParams, &m.SystemNameParamsShadow, "{}"})
}

func (m *TaskNameMeta) unmarshal() {
	m.normalize()
	unmarshalJSON(m.SystemNameParamsShadow, &m.SystemNameParams, "{}")
}

// AtomicTask is the only business resource executed by a Worker handler.
// +k8s:deepcopy-gen=true
type AtomicTask struct {
	imachinery.ObjectMeta
	TaskNameMeta
	FunctionRef          string             `json:"function_ref" gorm:"column:function_ref;type:varchar(256);not null;index"`
	Arguments            map[string]any     `json:"arguments,omitempty" gorm:"-"`
	ArgumentsShadow      string             `json:"-" gorm:"column:arguments_json;type:text;not null;default:'{}'"`
	RequiredCapabilities string             `json:"required_capabilities,omitempty" gorm:"column:required_capabilities;type:text"`
	RetryPolicy          RetryPolicy        `json:"retry_policy,omitempty" gorm:"-"`
	RetryPolicyShadow    string             `json:"-" gorm:"column:retry_policy_json;type:text;not null;default:'{}'"`
	TimeoutPolicy        TimeoutPolicy      `json:"timeout_policy,omitempty" gorm:"-"`
	TimeoutPolicyShadow  string             `json:"-" gorm:"column:timeout_policy_json;type:text;not null;default:'{}'"`
	CancelPolicy         map[string]any     `json:"cancel_policy,omitempty" gorm:"-"`
	CancelPolicyShadow   string             `json:"-" gorm:"column:cancel_policy_json;type:text;not null;default:'{}'"`
	Status               string             `json:"status" gorm:"column:status;type:varchar(32);not null;index:idx_atomic_tasks_status_schedule,priority:1"`
	Progress             float64            `json:"progress" gorm:"column:progress;not null;default:0"`
	CurrentAttempt       int                `json:"current_attempt" gorm:"column:current_attempt;not null;default:0"`
	Output               map[string]any     `json:"output,omitempty" gorm:"-"`
	OutputShadow         string             `json:"-" gorm:"column:output_json;type:text;not null;default:'{}'"`
	LastError            TaskError          `json:"last_error,omitempty" gorm:"-"`
	LastErrorShadow      string             `json:"-" gorm:"column:last_error_json;type:text;not null;default:'{}'"`
	RetryOfTaskID        string             `json:"retry_of_task_id,omitempty" gorm:"column:retry_of_task_id;type:varchar(64);index"`
	RetryOfTask          *AtomicTaskSummary `json:"retry_of_task,omitempty" gorm:"-"`
	RootTaskID           string             `json:"root_task_id,omitempty" gorm:"column:root_task_id;type:varchar(64);index"`
	RootTask             *AtomicTaskSummary `json:"root_task,omitempty" gorm:"-"`
	OwnerType            string             `json:"owner_type,omitempty" gorm:"column:owner_type;type:varchar(32)"`
	OwnerID              string             `json:"owner_id,omitempty" gorm:"column:owner_id;type:varchar(64);index"`
	Owner                *TaskOwnerSummary  `json:"owner,omitempty" gorm:"-"`
	ChildKey             string             `json:"child_key,omitempty" gorm:"column:child_key;type:varchar(128)"`
	// DAGNodeKey 保存声明 DAG 节点 key；动态 fan-out 的实际任务共享该值。
	DAGNodeKey         string                 `json:"node_key,omitempty" gorm:"column:dag_node_key;type:varchar(128)"`
	ChildOrder         int                    `json:"child_order,omitempty" gorm:"column:child_order;not null;default:0"`
	ApplicationRunID   string                 `json:"application_run_id,omitempty" gorm:"column:application_run_id;type:varchar(64);index"`
	CanvasRunID        string                 `json:"canvas_run_id,omitempty" gorm:"column:canvas_run_id;type:varchar(64);index"`
	CanvasNodeRunID    string                 `json:"canvas_node_run_id,omitempty" gorm:"column:canvas_node_run_id;type:varchar(64);index"`
	IdempotencyScope   string                 `json:"idempotency_scope,omitempty" gorm:"column:idempotency_scope;type:varchar(256)"`
	IdempotencyKey     string                 `json:"idempotency_key,omitempty" gorm:"column:idempotency_key;type:varchar(256)"`
	RuntimeExecutionID string                 `json:"runtime_execution_id,omitempty" gorm:"column:runtime_execution_id;type:varchar(128);index"`
	RuntimeTaskID      string                 `json:"runtime_task_id,omitempty" gorm:"column:runtime_task_id;type:varchar(128)"`
	RuntimeRevision    string                 `json:"runtime_revision,omitempty" gorm:"column:runtime_revision;type:varchar(128)"`
	ScheduleAt         imachinery.Time        `json:"schedule_at,omitempty" gorm:"column:schedule_at;index:idx_atomic_tasks_status_schedule,priority:2"`
	StartedAt          imachinery.Time        `json:"started_at,omitempty" gorm:"column:started_at"`
	CompletedAt        imachinery.Time        `json:"completed_at,omitempty" gorm:"column:completed_at"`
	CanceledAt         imachinery.Time        `json:"canceled_at,omitempty" gorm:"column:canceled_at"`
	ProjectID          string                 `json:"project_id" gorm:"column:project_id;type:varchar(128);not null;index:idx_atomic_tasks_scope,priority:1"`
	Namespace          string                 `json:"namespace" gorm:"column:namespace;type:varchar(128);not null;index:idx_atomic_tasks_scope,priority:2"`
	CreatedBy          string                 `json:"created_by" gorm:"column:created_by;type:varchar(128);not null"`
	ScheduleSource     *ScheduleSourceSummary `json:"schedule_source,omitempty" gorm:"-"` // 调度创建的根任务来源，非调度任务为空。
	Tags               string                 `json:"tags,omitempty" gorm:"column:tags;type:text"`
	DeletedAt          imachinery.Time        `json:"-" gorm:"column:deleted_at;index"`
}

func (AtomicTask) TableName() string { return "atomic_tasks" }

func (t *AtomicTask) BeforeCreate(tx *gorm.DB) error {
	if err := t.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return t.marshalShadows()
}
func (t *AtomicTask) AfterCreate(*gorm.DB) error { return nil }
func (t *AtomicTask) BeforeUpdate(tx *gorm.DB) error {
	if err := t.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return t.marshalShadows()
}
func (t *AtomicTask) AfterUpdate(*gorm.DB) error { return nil }
func (t *AtomicTask) AfterFind(tx *gorm.DB) error {
	if err := t.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(t.ArgumentsShadow, &t.Arguments, "{}")
	unmarshalJSON(t.RetryPolicyShadow, &t.RetryPolicy, "{}")
	unmarshalJSON(t.TimeoutPolicyShadow, &t.TimeoutPolicy, "{}")
	unmarshalJSON(t.CancelPolicyShadow, &t.CancelPolicy, "{}")
	unmarshalJSON(t.OutputShadow, &t.Output, "{}")
	unmarshalJSON(t.LastErrorShadow, &t.LastError, "{}")
	t.TaskNameMeta.unmarshal()
	return nil
}
func (t *AtomicTask) marshalShadows() error {
	if err := t.TaskNameMeta.marshal(); err != nil {
		return err
	}
	return marshalJSONFields(
		jsonField{t.Arguments, &t.ArgumentsShadow, "{}"},
		jsonField{t.RetryPolicy, &t.RetryPolicyShadow, "{}"},
		jsonField{t.TimeoutPolicy, &t.TimeoutPolicyShadow, "{}"},
		jsonField{t.CancelPolicy, &t.CancelPolicyShadow, "{}"},
		jsonField{t.Output, &t.OutputShadow, "{}"},
		jsonField{t.LastError, &t.LastErrorShadow, "{}"},
	)
}

// TaskAttempt records one Conductor execution attempt for an AtomicTask.
// +k8s:deepcopy-gen=true
type TaskAttempt struct {
	imachinery.ObjectMeta
	AtomicTaskID   string             `json:"atomic_task_id" gorm:"column:atomic_task_id;type:varchar(64);not null;uniqueIndex:idx_task_attempts_task_no,priority:1;index"`
	AtomicTask     *AtomicTaskSummary `json:"atomic_task,omitempty" gorm:"-"`
	AttemptNo      int                `json:"attempt_no" gorm:"column:attempt_no;not null;uniqueIndex:idx_task_attempts_task_no,priority:2"`
	RuntimeTaskID  string             `json:"runtime_task_id" gorm:"column:runtime_task_id;type:varchar(128);not null;uniqueIndex"`
	Status         string             `json:"status" gorm:"column:status;type:varchar(32);not null;index"`
	InputSnapshot  map[string]any     `json:"input_snapshot,omitempty" gorm:"-"`
	InputShadow    string             `json:"-" gorm:"column:input_snapshot_json;type:text;not null;default:'{}'"`
	OutputSnapshot map[string]any     `json:"output_snapshot,omitempty" gorm:"-"`
	OutputShadow   string             `json:"-" gorm:"column:output_snapshot_json;type:text;not null;default:'{}'"`
	Error          TaskError          `json:"error,omitempty" gorm:"-"`
	ErrorShadow    string             `json:"-" gorm:"column:error_json;type:text;not null;default:'{}'"`
	ExternalJobID  string             `json:"external_job_id,omitempty" gorm:"column:external_job_id;type:varchar(256);index"`
	LogsRef        string             `json:"logs_ref,omitempty" gorm:"column:logs_ref;type:text"` // Task Center 稳定不透明日志引用，客户端不得解析为运行时地址。
	// ExecutorType 是受控执行器类别快照，不保存 Worker ID、队列、主机或地址。
	ExecutorType string `json:"-" gorm:"column:executor_type;type:varchar(32);not null;default:''"`
	// ExecutorDisplayName 是执行器的稳定可读名称，仅管理员响应可见。
	ExecutorDisplayName string `json:"-" gorm:"column:executor_display_name;type:varchar(256);not null;default:''"`
	// Executor 是权限裁剪后的公开摘要；普通用户响应保持为空。
	Executor    *TaskExecutorSummary `json:"executor,omitempty" gorm:"-"`
	StartedAt   imachinery.Time      `json:"started_at,omitempty" gorm:"column:started_at"`
	CompletedAt imachinery.Time      `json:"completed_at,omitempty" gorm:"column:completed_at"`
	DurationMS  int64                `json:"duration_ms" gorm:"column:duration_ms;not null;default:0"`
	Retryable   bool                 `json:"retryable" gorm:"column:retryable;not null;default:false"`
}

func (TaskAttempt) TableName() string { return "task_attempts" }
func (a *TaskAttempt) BeforeCreate(tx *gorm.DB) error {
	if err := a.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	a.ensureLogsRef()
	return a.marshalShadows()
}
func (a *TaskAttempt) AfterCreate(*gorm.DB) error { return nil }
func (a *TaskAttempt) BeforeUpdate(tx *gorm.DB) error {
	if err := a.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	a.ensureLogsRef()
	return a.marshalShadows()
}
func (a *TaskAttempt) AfterUpdate(*gorm.DB) error { return nil }
func (a *TaskAttempt) AfterFind(tx *gorm.DB) error {
	if err := a.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(a.InputShadow, &a.InputSnapshot, "{}")
	unmarshalJSON(a.OutputShadow, &a.OutputSnapshot, "{}")
	unmarshalJSON(a.ErrorShadow, &a.Error, "{}")
	a.ensureLogsRef()
	return nil
}
func (a *TaskAttempt) marshalShadows() error {
	return marshalJSONFields(
		jsonField{a.InputSnapshot, &a.InputShadow, "{}"},
		jsonField{a.OutputSnapshot, &a.OutputShadow, "{}"},
		jsonField{a.Error, &a.ErrorShadow, "{}"},
	)
}

func (a *TaskAttempt) ensureLogsRef() {
	if a.LogsRef == "" && a.ID != "" {
		a.LogsRef = TaskAttemptLogsRef(a.ID)
	}
}

// TaskAttemptLogsRef 返回不暴露 runtime backend 的稳定日志引用。
func TaskAttemptLogsRef(attemptID string) string {
	if attemptID == "" {
		return ""
	}
	return "task-attempt-log:" + attemptID
}

// AtomicTaskTemplate is embedded in Group, DAG, and Schedule immutable snapshots.
// +k8s:deepcopy-gen=true
type AtomicTaskTemplate struct {
	Key                  string            `json:"key"`
	Name                 string            `json:"name,omitempty"`
	NameI18n             map[string]string `json:"name_i18n,omitempty"`
	SystemName           SystemNameSpec    `json:"-"`
	FunctionRef          string            `json:"function_ref"`
	Arguments            map[string]any    `json:"arguments,omitempty"`
	RequiredCapabilities string            `json:"required_capabilities,omitempty"`
	RetryPolicy          RetryPolicy       `json:"retry_policy,omitempty"`
	TimeoutPolicy        TimeoutPolicy     `json:"timeout_policy,omitempty"`
}

type GroupStrategy struct {
	FailFast        bool `json:"fail_fast"`
	CancelOnFailure bool `json:"cancel_on_failure"`
	MaxParallelism  int  `json:"max_parallelism"`
}

// +k8s:deepcopy-gen=true
type TaskSummary struct {
	Total           int `json:"total"`
	Pending         int `json:"pending"`
	Blocked         int `json:"blocked"`
	Ready           int `json:"ready"`
	Running         int `json:"running"`
	Retrying        int `json:"retrying"`
	CancelRequested int `json:"cancel_requested"`
	Success         int `json:"success"`
	Failed          int `json:"failed"`
	Canceled        int `json:"canceled"`
	Timeout         int `json:"timeout"`
	Skipped         int `json:"skipped"`
}

// ScheduleSourceSummary 标识创建运行资源的调度计划与具体轮次。
// +k8s:deepcopy-gen=true
type ScheduleSourceSummary struct {
	ScheduleID          string            `json:"schedule_id"`                  // 来源 TaskSchedule 标识。
	ScheduleName        string            `json:"schedule_name"`                // 来源计划的可读名称。
	ScheduleNameI18n    map[string]string `json:"schedule_name_i18n,omitempty"` // 系统计划的多语言名称。
	ScheduleExecutionID string            `json:"schedule_execution_id"`        // 实际创建该目标的调度轮次。
	ScheduledAt         imachinery.Time   `json:"scheduled_at"`                 // 该轮次的计划触发时间。
}

// AtomicTaskSummary 是关联响应使用的一跳任务摘要，不携带参数、输出、错误或其他关联。
// +k8s:deepcopy-gen=true
type AtomicTaskSummary struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	NameI18n    map[string]string `json:"name_i18n,omitempty"`
	Status      string            `json:"status"`
	Progress    float64           `json:"progress"`
	FunctionRef string            `json:"function_ref,omitempty"`
}
func (v *AtomicTaskSummary) ToAtomicTaskSummary() AtomicTaskRefSummary {
	return AtomicTaskRefSummary{
		ID:          v.ID,
		Name:        v.Name,
		NameI18n:    v.NameI18n,
		Status:      v.Status,
		Progress:    v.Progress,
		FunctionRef: v.FunctionRef,
	}
}
// DAGTaskGroupSummary 是跨领域读取 DAG 运行状态的一跳摘要，不包含节点、边或运行时标识。
type DAGTaskGroupSummary struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	NameI18n map[string]string `json:"name_i18n,omitempty"`
	Status   string            `json:"status"`
	Progress float64           `json:"progress"`
}

// TaskOwnerSummary 是 AtomicTask 多态 owner 以及 Group/DAG 重试来源的一跳摘要。
// +k8s:deepcopy-gen=true
type TaskOwnerSummary struct {
	Type     string            `json:"type"`
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	NameI18n map[string]string `json:"name_i18n,omitempty"`
	Status   string            `json:"status"`
	Progress float64           `json:"progress,omitempty"`
}

// TaskScheduleSummary 是执行历史关联的计划摘要，不携带 target 模板或运行历史。
// +k8s:deepcopy-gen=true
type TaskScheduleSummary struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	NameI18n map[string]string `json:"name_i18n,omitempty"`
	Status   string            `json:"status"`
}

// TaskTargetSummary 是可展示且可导航的调度目标轻量投影，不包含大型输入输出。
// +k8s:deepcopy-gen=true
type TaskTargetSummary struct {
	Type             string            `json:"type"`                         // 目标为 AtomicTask、TaskGroup 或 DAGTaskGroup。
	ID               string            `json:"id,omitempty"`                 // 实际目标标识；未创建目标时为空。
	Name             string            `json:"name"`                         // 目标或模板的可读名称。
	NameI18n         map[string]string `json:"name_i18n,omitempty"`          // 系统目标的多语言名称。
	Status           string            `json:"status,omitempty"`             // 实际目标的当前状态。
	Progress         float64           `json:"progress,omitempty"`           // 实际目标的 0 到 1 进度。
	FunctionRef      string            `json:"function_ref,omitempty"`       // AtomicTask 注册执行函数。
	TaskCount        int               `json:"task_count,omitempty"`         // Group/DAG 包含的任务数量。
	ApplicationRunID string            `json:"application_run_id,omitempty"` // 可选应用运行业务引用。
	CanvasRunID      string            `json:"canvas_run_id,omitempty"`      // 可选画布运行业务引用。
	CanvasNodeRunID  string            `json:"canvas_node_run_id,omitempty"` // 可选画布节点运行引用。
}

// TaskGroup is a SERIAL or PARALLEL composition of AtomicTask templates.
// +k8s:deepcopy-gen=true
type TaskGroup struct {
	imachinery.ObjectMeta
	TaskNameMeta
	Mode                     string                 `json:"mode" gorm:"column:mode;type:varchar(16);not null"`
	Tasks                    []AtomicTaskTemplate   `json:"tasks" gorm:"-"`
	TasksShadow              string                 `json:"-" gorm:"column:task_templates_json;type:text;not null"`
	Strategy                 GroupStrategy          `json:"strategy" gorm:"-"`
	StrategyShadow           string                 `json:"-" gorm:"column:strategy_json;type:text;not null;default:'{}'"`
	Status                   string                 `json:"status" gorm:"column:status;type:varchar(32);not null;index"`
	Progress                 float64                `json:"progress" gorm:"column:progress;not null;default:0"`
	Summary                  TaskSummary            `json:"summary" gorm:"-"`
	SummaryShadow            string                 `json:"-" gorm:"column:summary_json;type:text;not null;default:'{}'"`
	Result                   map[string]any         `json:"result,omitempty" gorm:"-"`
	ResultShadow             string                 `json:"-" gorm:"column:result_json;type:text;not null;default:'{}'"`
	RetryOfID                string                 `json:"retry_of_id,omitempty" gorm:"column:retry_of_id;type:varchar(64);index"`
	RetryOf                  *TaskOwnerSummary      `json:"retry_of,omitempty" gorm:"-"`
	RuntimeExecutionID       string                 `json:"runtime_execution_id,omitempty" gorm:"column:runtime_execution_id;type:varchar(128);index"`
	RuntimeDefinitionName    string                 `json:"runtime_definition_name,omitempty" gorm:"column:runtime_definition_name;type:varchar(256)"`
	RuntimeDefinitionVersion int                    `json:"runtime_definition_version,omitempty" gorm:"column:runtime_definition_version;not null;default:0"`
	IdempotencyScope         string                 `json:"idempotency_scope,omitempty" gorm:"column:idempotency_scope;type:varchar(256)"`
	IdempotencyKey           string                 `json:"idempotency_key,omitempty" gorm:"column:idempotency_key;type:varchar(256)"`
	ProjectID                string                 `json:"project_id" gorm:"column:project_id;type:varchar(128);not null"`
	Namespace                string                 `json:"namespace" gorm:"column:namespace;type:varchar(128);not null"`
	CreatedBy                string                 `json:"created_by" gorm:"column:created_by;type:varchar(128);not null"`
	ScheduleSource           *ScheduleSourceSummary `json:"schedule_source,omitempty" gorm:"-"` // 调度创建的组合来源，非调度组合为空。
	DeletedAt                imachinery.Time        `json:"-" gorm:"column:deleted_at;index"`
}

func (TaskGroup) TableName() string { return "task_groups" }
func (g *TaskGroup) BeforeCreate(tx *gorm.DB) error {
	if err := g.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return g.marshalShadows()
}
func (g *TaskGroup) AfterCreate(*gorm.DB) error { return nil }
func (g *TaskGroup) BeforeUpdate(tx *gorm.DB) error {
	if err := g.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return g.marshalShadows()
}
func (g *TaskGroup) AfterUpdate(*gorm.DB) error { return nil }
func (g *TaskGroup) AfterFind(tx *gorm.DB) error {
	if err := g.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(g.TasksShadow, &g.Tasks, "[]")
	unmarshalJSON(g.StrategyShadow, &g.Strategy, "{}")
	unmarshalJSON(g.SummaryShadow, &g.Summary, "{}")
	unmarshalJSON(g.ResultShadow, &g.Result, "{}")
	g.TaskNameMeta.unmarshal()
	return nil
}
func (g *TaskGroup) marshalShadows() error {
	if err := g.TaskNameMeta.marshal(); err != nil {
		return err
	}
	return marshalJSONFields(jsonField{g.Tasks, &g.TasksShadow, "[]"}, jsonField{g.Strategy, &g.StrategyShadow, "{}"}, jsonField{g.Summary, &g.SummaryShadow, "{}"}, jsonField{g.Result, &g.ResultShadow, "{}"})
}

// DAGNode is the node in a DAG.
// +k8s:deepcopy-gen=true
type DAGNode struct {
	Key             string             `json:"key"`
	Task            AtomicTaskTemplate `json:"task"`
	InputMapping    map[string]any     `json:"input_mapping,omitempty"`
	DynamicFork     bool               `json:"dynamic_fork,omitempty"`
	MaxDynamicTasks int                `json:"max_dynamic_tasks,omitempty"`
}

// DAGEdge is the edge between two nodes in a DAG.
// +k8s:deepcopy-gen=true
type DAGEdge struct {
	FromNode    string         `json:"from_node"`
	ToNode      string         `json:"to_node"`
	DataMapping map[string]any `json:"data_mapping,omitempty"`
}

// DAGTaskGroup stores an immutable validated AtomicTask DAG execution.
// +k8s:deepcopy-gen=true
type DAGTaskGroup struct {
	imachinery.ObjectMeta
	TaskNameMeta
	Nodes               []DAGNode      `json:"nodes" gorm:"-"`
	NodesShadow         string         `json:"-" gorm:"column:nodes_json;type:text;not null"`
	Edges               []DAGEdge      `json:"edges" gorm:"-"`
	EdgesShadow         string         `json:"-" gorm:"column:edges_json;type:text;not null"`
	Input               map[string]any `json:"input,omitempty" gorm:"-"`
	InputShadow         string         `json:"-" gorm:"column:input_mapping_json;type:text;not null;default:'{}'"`
	OutputMapping       map[string]any `json:"output_mapping,omitempty" gorm:"-"`
	OutputMappingShadow string         `json:"-" gorm:"column:output_mapping_json;type:text;not null;default:'{}'"`
	Status              string         `json:"status" gorm:"column:status;type:varchar(32);not null;index"`
	Progress            float64        `json:"progress" gorm:"column:progress;not null;default:0"`
	Summary             TaskSummary    `json:"summary" gorm:"-"`
	SummaryShadow       string         `json:"-" gorm:"column:summary_json;type:text;not null;default:'{}'"`
	Result              map[string]any `json:"result,omitempty" gorm:"-"`
	ResultShadow        string         `json:"-" gorm:"column:result_json;type:text;not null;default:'{}'"`
	// StartedAt 是 DAG 首个实际任务开始执行的时间。
	StartedAt imachinery.Time `json:"started_at,omitempty" gorm:"column:started_at"`
	// CompletedAt 是 DAG 汇总进入终态的时间。
	CompletedAt imachinery.Time `json:"completed_at,omitempty" gorm:"column:completed_at"`
	// TriggerType 固定为 API、SCHEDULE、CANVAS、DOMAIN_EVENT 或 RETRY。
	TriggerType string `json:"-" gorm:"column:trigger_type;type:varchar(32);not null;default:'API'"`
	// TriggerSourceID 是触发时来源快照；来源删除不影响历史读取。
	TriggerSourceID string `json:"-" gorm:"column:trigger_source_id;type:varchar(128);not null;default:''"`
	// TriggerSourceName 是触发时的可读来源名称快照。
	TriggerSourceName string `json:"-" gorm:"column:trigger_source_name;type:varchar(256);not null;default:''"`
	// TriggeredAt 是服务接收 DAG 触发的时间。
	TriggeredAt              imachinery.Time        `json:"-" gorm:"column:triggered_at"`
	RetryOfID                string                 `json:"retry_of_id,omitempty" gorm:"column:retry_of_id;type:varchar(64);index"`
	RetryOf                  *TaskOwnerSummary      `json:"retry_of,omitempty" gorm:"-"`
	CanvasVersionID          string                 `json:"canvas_version_id,omitempty" gorm:"column:canvas_version_id;type:varchar(64);index"`
	RuntimeExecutionID       string                 `json:"runtime_execution_id,omitempty" gorm:"column:runtime_execution_id;type:varchar(128);index"`
	RuntimeDefinitionName    string                 `json:"runtime_definition_name" gorm:"column:runtime_definition_name;type:varchar(256);not null;index:idx_dag_groups_definition,priority:1"`
	RuntimeDefinitionVersion int                    `json:"runtime_definition_version" gorm:"column:runtime_definition_version;not null;index:idx_dag_groups_definition,priority:2"`
	RuntimeDefinitionHash    string                 `json:"runtime_definition_hash" gorm:"column:runtime_definition_hash;type:varchar(128);not null"`
	IdempotencyScope         string                 `json:"idempotency_scope,omitempty" gorm:"column:idempotency_scope;type:varchar(256)"`
	IdempotencyKey           string                 `json:"idempotency_key,omitempty" gorm:"column:idempotency_key;type:varchar(256)"`
	ProjectID                string                 `json:"project_id" gorm:"column:project_id;type:varchar(128);not null"`
	Namespace                string                 `json:"namespace" gorm:"column:namespace;type:varchar(128);not null"`
	CreatedBy                string                 `json:"created_by" gorm:"column:created_by;type:varchar(128);not null"`
	ScheduleSource           *ScheduleSourceSummary `json:"schedule_source,omitempty" gorm:"-"` // 调度创建的 DAG 来源，非调度 DAG 为空。
	DeletedAt                imachinery.Time        `json:"-" gorm:"column:deleted_at;index"`
}

func (DAGTaskGroup) TableName() string { return "dag_task_groups" }
func (g *DAGTaskGroup) BeforeCreate(tx *gorm.DB) error {
	if err := g.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return g.marshalShadows()
}
func (g *DAGTaskGroup) AfterCreate(*gorm.DB) error { return nil }
func (g *DAGTaskGroup) BeforeUpdate(tx *gorm.DB) error {
	if err := g.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return g.marshalShadows()
}
func (g *DAGTaskGroup) AfterUpdate(*gorm.DB) error { return nil }
func (g *DAGTaskGroup) AfterFind(tx *gorm.DB) error {
	if err := g.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(g.NodesShadow, &g.Nodes, "[]")
	unmarshalJSON(g.EdgesShadow, &g.Edges, "[]")
	unmarshalJSON(g.InputShadow, &g.Input, "{}")
	unmarshalJSON(g.OutputMappingShadow, &g.OutputMapping, "{}")
	unmarshalJSON(g.SummaryShadow, &g.Summary, "{}")
	unmarshalJSON(g.ResultShadow, &g.Result, "{}")
	g.TaskNameMeta.unmarshal()
	return nil
}
func (g *DAGTaskGroup) marshalShadows() error {
	if err := g.TaskNameMeta.marshal(); err != nil {
		return err
	}
	return marshalJSONFields(jsonField{g.Nodes, &g.NodesShadow, "[]"}, jsonField{g.Edges, &g.EdgesShadow, "[]"}, jsonField{g.Input, &g.InputShadow, "{}"}, jsonField{g.OutputMapping, &g.OutputMappingShadow, "{}"}, jsonField{g.Summary, &g.SummaryShadow, "{}"}, jsonField{g.Result, &g.ResultShadow, "{}"})
}

// ScheduleTarget 是调度任务的目标。
// +k8s:deepcopy-gen=true
type ScheduleTarget struct {
	Type     string         `json:"type"`
	Template map[string]any `json:"template"`
}

// ScheduleSummary 是调度任务的摘要。
type ScheduleSummary struct {
	TotalTriggered int `json:"total_triggered"`
	Running        int `json:"running"`
	Success        int `json:"success"`
	Failed         int `json:"failed"`
	Canceled       int `json:"canceled"`
	SkippedOverlap int `json:"skipped_overlap"`
}

// ReconcileSpec 描述后端注册巡检器的受控运行参数；reconcile_ref 不能由公开更新接口更换。
// +k8s:deepcopy-gen=true
type ReconcileSpec struct {
	ReconcileRef          string         `json:"reconcile_ref"`                   // 后端 ReconcileRegistry 中的稳定引用。
	DisplayName           string         `json:"display_name,omitempty" gorm:"-"` // 后端注册器提供的可读名称，不持久化。
	Config                map[string]any `json:"config"`                          // 巡检器拥有并校验的受控配置。
	MaxParallelism        int            `json:"max_parallelism"`                 // 单轮最大并发，契约范围 1..64。
	MaxItemsPerRun        int            `json:"max_items_per_run"`               // 单轮最大扫描量，契约范围 1..1000。
	PerItemTimeoutSeconds int            `json:"per_item_timeout_seconds"`        // 单资源探测超时秒数。
	OverallTimeoutSeconds int            `json:"overall_timeout_seconds"`         // 整轮超时秒数，不得小于单项超时。
}

// HistoryRetention 控制 RECONCILE 轻量业务历史与运行时历史的有限保留。
// +k8s:deepcopy-gen=true
type HistoryRetention struct {
	SuccessCount            int `json:"success_count"`
	FailureCount            int `json:"failure_count"`
	FailureDurationSeconds  int `json:"failure_duration_seconds"`
	SkippedCount            int `json:"skipped_count"`
	RuntimeRetentionSeconds int `json:"runtime_retention_seconds"`
}

// ReconcileSummary 是单轮巡检的低成本结果摘要；领域 summary 只能携带有界关联摘要，不能保存无界逐项详情。
// +k8s:deepcopy-gen=true
type ReconcileSummary struct {
	Scanned            int            `json:"scanned"`
	Findings           int            `json:"findings"`
	ActionsCreated     int            `json:"actions_created"`
	Deferred           int            `json:"deferred"`
	DurationMS         int64          `json:"duration_ms"`
	CheckpointAdvanced bool           `json:"checkpoint_advanced"`
	CycleCompleted     bool           `json:"cycle_completed"`
	Summary            map[string]any `json:"summary,omitempty"`
}

// TaskSchedule persistently triggers an AtomicTask, TaskGroup, or DAGTaskGroup template.
// +k8s:deepcopy-gen=true
type TaskSchedule struct {
	imachinery.ObjectMeta
	TaskNameMeta
	ExecutionMode                  string                 `json:"execution_mode" gorm:"column:execution_mode;type:varchar(16);not null;default:'MATERIALIZED';index:idx_task_schedules_mode_status,priority:1"` // MATERIALIZED 保留完整作业历史，RECONCILE 保存轻量巡检历史。
	ManagementMode                 string                 `json:"management_mode" gorm:"column:management_mode;type:varchar(16);not null;default:'USER'"`                                                       // USER 可常规管理，SYSTEM 仅管理员可调整安全参数。
	SystemKey                      string                 `json:"system_key" gorm:"column:system_key;type:varchar(256);not null;default:''"`                                                                    // SYSTEM 计划的全局幂等键，非空时由数据库保证唯一。
	TriggerType                    string                 `json:"trigger_type" gorm:"column:trigger_type;type:varchar(16);not null"`
	CronExpression                 string                 `json:"cron_expression,omitempty" gorm:"column:cron_expression;type:varchar(256)"`
	RunAt                          imachinery.Time        `json:"run_at,omitempty" gorm:"column:run_at"`
	TimeZone                       string                 `json:"time_zone" gorm:"column:time_zone;type:varchar(128);not null;default:'UTC'"`
	Target                         ScheduleTarget         `json:"target,omitzero" gorm:"-"`
	TargetSummary                  *TaskTargetSummary     `json:"target_summary,omitempty" gorm:"-"` // 由目标模板派生的可读摘要。
	TargetType                     string                 `json:"-" gorm:"column:target_type;type:varchar(32);not null"`
	TargetTemplateShadow           string                 `json:"-" gorm:"column:target_template_json;type:text;not null;default:''"`
	ReconcileSpec                  *ReconcileSpec         `json:"reconcile_spec,omitempty" gorm:"-"`
	ReconcileRef                   string                 `json:"-" gorm:"column:reconcile_ref;type:varchar(256);not null;default:''"` // 只允许引用后端注册的巡检器。
	ReconcileConfigShadow          string                 `json:"-" gorm:"column:reconcile_config_json;type:text;not null;default:'{}'"`
	ReconcileMaxParallelism        int                    `json:"-" gorm:"column:reconcile_max_parallelism;not null;default:16"`
	ReconcileMaxItemsPerRun        int                    `json:"-" gorm:"column:reconcile_max_items_per_run;not null;default:1000"`
	ReconcilePerItemTimeoutSeconds int                    `json:"-" gorm:"column:reconcile_per_item_timeout_seconds;not null;default:4"`
	ReconcileOverallTimeoutSeconds int                    `json:"-" gorm:"column:reconcile_overall_timeout_seconds;not null;default:5"`
	HistoryRetention               HistoryRetention       `json:"history_retention" gorm:"-"` // RECONCILE 业务历史和 runtime 历史的有限保留策略。
	HistoryRetentionShadow         string                 `json:"-" gorm:"column:history_retention_json;type:text;not null;default:'{}'"`
	Status                         string                 `json:"status" gorm:"column:status;type:varchar(32);not null;index:idx_task_schedules_status_next,priority:1;index:idx_task_schedules_mode_status,priority:2"`
	MisfirePolicy                  string                 `json:"misfire_policy" gorm:"column:misfire_policy;type:varchar(16);not null;default:'SKIP'"`
	OverlapPolicy                  string                 `json:"overlap_policy" gorm:"column:overlap_policy;type:varchar(16);not null;default:'SKIP'"`
	RuntimeScheduleName            string                 `json:"runtime_schedule_name,omitempty" gorm:"column:runtime_schedule_name;type:varchar(256);uniqueIndex"`
	LastTriggerAt                  imachinery.Time        `json:"last_trigger_at,omitempty" gorm:"column:last_trigger_at"`
	NextTriggerAt                  imachinery.Time        `json:"next_trigger_at,omitempty" gorm:"column:next_trigger_at;index:idx_task_schedules_status_next,priority:2;index:idx_task_schedules_mode_status,priority:3"`
	Summary                        ScheduleSummary        `json:"summary" gorm:"-"`
	SummaryShadow                  string                 `json:"-" gorm:"column:summary_json;type:text;not null;default:'{}'"`
	LastExecution                  *TaskScheduleExecution `json:"last_execution,omitempty" gorm:"-"`
	ProjectID                      string                 `json:"project_id" gorm:"column:project_id;type:varchar(128);not null;index"`
	Namespace                      string                 `json:"namespace" gorm:"column:namespace;type:varchar(128);not null;index"`
	CreatedBy                      string                 `json:"created_by" gorm:"column:created_by;type:varchar(128);not null"`
	DeletedAt                      imachinery.Time        `json:"-" gorm:"column:deleted_at;index"`
}

func (TaskSchedule) TableName() string { return "task_schedules" }
func (s *TaskSchedule) BeforeCreate(tx *gorm.DB) error {
	if err := s.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return s.marshalShadows()
}
func (s *TaskSchedule) AfterCreate(*gorm.DB) error { return nil }
func (s *TaskSchedule) BeforeUpdate(tx *gorm.DB) error {
	if err := s.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return s.marshalShadows()
}
func (s *TaskSchedule) AfterUpdate(*gorm.DB) error { return nil }
func (s *TaskSchedule) AfterFind(tx *gorm.DB) error {
	if err := s.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	s.Target.Type = s.TargetType
	unmarshalJSON(s.TargetTemplateShadow, &s.Target.Template, "{}")
	if s.ExecutionMode == TaskScheduleModeReconcile {
		config := map[string]any{}
		unmarshalJSON(s.ReconcileConfigShadow, &config, "{}")
		s.ReconcileSpec = &ReconcileSpec{ReconcileRef: s.ReconcileRef, Config: config, MaxParallelism: s.ReconcileMaxParallelism, MaxItemsPerRun: s.ReconcileMaxItemsPerRun, PerItemTimeoutSeconds: s.ReconcilePerItemTimeoutSeconds, OverallTimeoutSeconds: s.ReconcileOverallTimeoutSeconds}
	} else {
		s.ReconcileSpec = nil
	}
	unmarshalJSON(s.HistoryRetentionShadow, &s.HistoryRetention, "{}")
	unmarshalJSON(s.SummaryShadow, &s.Summary, "{}")
	s.TaskNameMeta.unmarshal()
	return nil
}
func (s *TaskSchedule) marshalShadows() error {
	if err := s.TaskNameMeta.marshal(); err != nil {
		return err
	}
	s.TargetType = s.Target.Type
	if s.ReconcileSpec != nil {
		s.ReconcileRef = s.ReconcileSpec.ReconcileRef
		s.ReconcileMaxParallelism = s.ReconcileSpec.MaxParallelism
		s.ReconcileMaxItemsPerRun = s.ReconcileSpec.MaxItemsPerRun
		s.ReconcilePerItemTimeoutSeconds = s.ReconcileSpec.PerItemTimeoutSeconds
		s.ReconcileOverallTimeoutSeconds = s.ReconcileSpec.OverallTimeoutSeconds
	}
	var config map[string]any
	if s.ReconcileSpec != nil {
		config = s.ReconcileSpec.Config
	}
	if s.ExecutionMode == TaskScheduleModeReconcile {
		s.TargetType, s.TargetTemplateShadow = "", ""
	} else if err := marshalJSONFields(jsonField{s.Target.Template, &s.TargetTemplateShadow, "{}"}); err != nil {
		return err
	}
	return marshalJSONFields(jsonField{config, &s.ReconcileConfigShadow, "{}"}, jsonField{s.HistoryRetention, &s.HistoryRetentionShadow, "{}"}, jsonField{s.Summary, &s.SummaryShadow, "{}"})
}

// TaskScheduleExecution records every scheduled time, including overlap skips.
// +k8s:deepcopy-gen=true
type TaskScheduleExecution struct {
	imachinery.ObjectMeta
	ScheduleID             string               `json:"schedule_id" gorm:"column:schedule_id;type:varchar(64);not null;uniqueIndex:idx_schedule_execution_time,priority:1;index"`
	Schedule               *TaskScheduleSummary `json:"schedule,omitempty" gorm:"-"`
	ExecutionMode          string               `json:"execution_mode" gorm:"column:execution_mode;type:varchar(16);not null;default:'MATERIALIZED'"` // 固化本轮语义，避免计划后续变化改写历史。
	ScheduledAt            imachinery.Time      `json:"scheduled_at" gorm:"column:scheduled_at;not null;uniqueIndex:idx_schedule_execution_time,priority:2"`
	TriggeredAt            imachinery.Time      `json:"triggered_at,omitempty" gorm:"column:triggered_at"`
	TargetType             string               `json:"target_type,omitempty" gorm:"column:target_type;type:varchar(32);not null"`
	TargetID               string               `json:"target_id,omitempty" gorm:"column:target_id;type:varchar(64);index"`
	TargetSummary          *TaskTargetSummary   `json:"target_summary,omitempty" gorm:"-"`   // 实际目标摘要，不可用时回退到计划模板摘要。
	ReconcileSummary       ReconcileSummary     `json:"reconcile_summary,omitzero" gorm:"-"` // RECONCILE 扫描、发现、动作与 checkpoint 推进摘要。
	ReconcileSummaryShadow string               `json:"-" gorm:"column:reconcile_summary_json;type:text;not null;default:'{}'"`
	RuntimeExecutionID     string               `json:"runtime_execution_id,omitempty" gorm:"column:runtime_execution_id;type:varchar(128);index"`
	Status                 string               `json:"status" gorm:"column:status;type:varchar(32);not null;index:idx_schedule_executions_status,priority:1"`
	Reason                 string               `json:"reason,omitempty" gorm:"column:reason;type:text"`
	CompletedAt            imachinery.Time      `json:"completed_at,omitempty" gorm:"column:completed_at"`
}

func (TaskScheduleExecution) TableName() string { return "task_schedule_executions" }
func (e *TaskScheduleExecution) BeforeCreate(tx *gorm.DB) error {
	if err := e.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return marshalJSONFields(jsonField{e.ReconcileSummary, &e.ReconcileSummaryShadow, "{}"})
}
func (e *TaskScheduleExecution) AfterCreate(*gorm.DB) error { return nil }
func (e *TaskScheduleExecution) BeforeUpdate(tx *gorm.DB) error {
	if err := e.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return marshalJSONFields(jsonField{e.ReconcileSummary, &e.ReconcileSummaryShadow, "{}"})
}
func (e *TaskScheduleExecution) AfterUpdate(*gorm.DB) error { return nil }
func (e *TaskScheduleExecution) AfterFind(tx *gorm.DB) error {
	if err := e.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(e.ReconcileSummaryShadow, &e.ReconcileSummary, "{}")
	return nil
}

// ScheduleReconcileState 是 TaskSchedule 的内部一对一投影，不嵌入 ObjectMeta，避免被误当作可独立命名、删除的普通资源。
// +k8s:deepcopy-gen=true
type ScheduleReconcileState struct {
	ScheduleID                string           `json:"schedule_id" gorm:"column:schedule_id;primaryKey;type:varchar(64)"`
	Checkpoint                map[string]any   `json:"-" gorm:"-"`
	CheckpointShadow          string           `json:"-" gorm:"column:checkpoint_json;type:text;not null;default:'{}'"`
	CurrentRuntimeExecutionID string           `json:"-" gorm:"column:current_runtime_execution_id;type:varchar(128);not null;default:''"`
	LastStartedAt             *imachinery.Time `json:"last_started_at,omitempty" gorm:"column:last_started_at"`
	LastCompletedAt           *imachinery.Time `json:"last_completed_at,omitempty" gorm:"column:last_completed_at;index:idx_schedule_reconcile_states_checkpoint,priority:1"`
	LastSummary               ReconcileSummary `json:"last_summary" gorm:"-"`
	LastSummaryShadow         string           `json:"-" gorm:"column:last_summary_json;type:text;not null;default:'{}'"`
	CheckpointAgeSeconds      int64            `json:"checkpoint_age_seconds" gorm:"-"`
	ConsecutiveFailures       int64            `json:"consecutive_failures" gorm:"column:consecutive_failures;not null;default:0"`
	TotalRuns                 int64            `json:"total_runs" gorm:"column:total_runs;not null;default:0"`
	TotalScanned              int64            `json:"total_scanned" gorm:"column:total_scanned;not null;default:0"`
	TotalFindings             int64            `json:"total_findings" gorm:"column:total_findings;not null;default:0"`
	TotalActionsCreated       int64            `json:"total_actions_created" gorm:"column:total_actions_created;not null;default:0"`
	ResourceVersion           int64            `json:"resource_version" gorm:"column:resource_version;not null;default:0"`
	UpdatedAt                 imachinery.Time  `json:"updated_at" gorm:"column:updated_at;not null;index:idx_schedule_reconcile_states_checkpoint,priority:2"`
}

func (ScheduleReconcileState) TableName() string { return "task_schedule_reconcile_states" }
func (s *ScheduleReconcileState) BeforeCreate(*gorm.DB) error {
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = imachinery.Now()
	}
	return s.marshalShadows()
}
func (*ScheduleReconcileState) AfterCreate(*gorm.DB) error { return nil }
func (s *ScheduleReconcileState) BeforeUpdate(*gorm.DB) error {
	s.UpdatedAt = imachinery.Now()
	return s.marshalShadows()
}
func (*ScheduleReconcileState) AfterUpdate(*gorm.DB) error { return nil }
func (s *ScheduleReconcileState) AfterFind(*gorm.DB) error {
	unmarshalJSON(s.CheckpointShadow, &s.Checkpoint, "{}")
	unmarshalJSON(s.LastSummaryShadow, &s.LastSummary, "{}")
	if !s.UpdatedAt.IsZero() {
		s.CheckpointAgeSeconds = max(int64(time.Since(s.UpdatedAt.Time).Seconds()), 0)
	}
	return nil
}
func (s *ScheduleReconcileState) marshalShadows() error {
	return marshalJSONFields(jsonField{s.Checkpoint, &s.CheckpointShadow, "{}"}, jsonField{s.LastSummary, &s.LastSummaryShadow, "{}"})
}

// RuntimeProjectionEvent makes Conductor event projection idempotent and replayable.
// +k8s:deepcopy-gen=true
type RuntimeProjectionEvent struct {
	imachinery.ObjectMeta
	RuntimeEventID     string          `json:"runtime_event_id" gorm:"column:runtime_event_id;type:varchar(256);not null;uniqueIndex"`
	RuntimeExecutionID string          `json:"runtime_execution_id" gorm:"column:runtime_execution_id;type:varchar(128);not null;index"`
	RuntimeTaskID      string          `json:"runtime_task_id,omitempty" gorm:"column:runtime_task_id;type:varchar(128);index"`
	EventType          string          `json:"event_type" gorm:"column:event_type;type:varchar(128);not null"`
	Payload            map[string]any  `json:"payload" gorm:"-"`
	PayloadShadow      string          `json:"-" gorm:"column:payload_json;type:text;not null"`
	OccurredAt         imachinery.Time `json:"occurred_at" gorm:"column:occurred_at;not null;index"`
	ProjectedAt        imachinery.Time `json:"projected_at,omitempty" gorm:"column:projected_at"`
	ProjectionStatus   string          `json:"projection_status" gorm:"column:projection_status;type:varchar(32);not null;index:idx_runtime_projection_pending,priority:1"`
	FailureDetail      string          `json:"failure_detail,omitempty" gorm:"column:failure_detail;type:text"`
}

func (RuntimeProjectionEvent) TableName() string { return "runtime_projection_events" }
func (e *RuntimeProjectionEvent) BeforeCreate(tx *gorm.DB) error {
	if err := e.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return marshalJSONFields(jsonField{e.Payload, &e.PayloadShadow, "{}"})
}
func (e *RuntimeProjectionEvent) AfterCreate(*gorm.DB) error { return nil }
func (e *RuntimeProjectionEvent) BeforeUpdate(tx *gorm.DB) error {
	if err := e.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return marshalJSONFields(jsonField{e.Payload, &e.PayloadShadow, "{}"})
}
func (e *RuntimeProjectionEvent) AfterUpdate(*gorm.DB) error { return nil }
func (e *RuntimeProjectionEvent) AfterFind(tx *gorm.DB) error {
	if err := e.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(e.PayloadShadow, &e.Payload, "{}")
	return nil
}

type jsonField struct {
	value    any
	target   *string
	fallback string
}

func marshalJSONFields(fields ...jsonField) error {
	for _, field := range fields {
		data, err := json.Marshal(field.value)
		if err != nil {
			return err
		}
		if string(data) == "null" {
			*field.target = field.fallback
		} else {
			*field.target = string(data)
		}
	}
	return nil
}

func unmarshalJSON(value string, target any, fallback string) {
	if value == "" {
		value = fallback
	}
	_ = json.Unmarshal([]byte(value), target)
}

func IsAtomicTaskTerminal(status string) bool {
	switch status {
	case AtomicTaskStatusSuccess, AtomicTaskStatusFailed, AtomicTaskStatusCanceled, AtomicTaskStatusTimeout, AtomicTaskStatusSkipped:
		return true
	default:
		return false
	}
}

func IsTaskGroupTerminal(status string) bool {
	switch status {
	case TaskGroupStatusSuccess, TaskGroupStatusFailed, TaskGroupStatusCanceled, TaskGroupStatusTimeout:
		return true
	default:
		return false
	}
}

func NewTaskTime(value time.Time) imachinery.Time { return imachinery.NewTime(value) }
