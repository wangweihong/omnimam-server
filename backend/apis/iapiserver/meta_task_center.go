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

	TaskScheduleTriggerCron  = "CRON"
	TaskScheduleTriggerRunAt = "RUN_AT"
	TaskScheduleTargetAtomic = "ATOMIC_TASK"
	TaskScheduleTargetGroup  = "TASK_GROUP"
	TaskScheduleTargetDAG    = "DAG_TASK_GROUP"

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
type TaskError struct {
	Code       string          `json:"code,omitempty"`
	Message    string          `json:"message,omitempty"`
	Detail     string          `json:"detail,omitempty"`
	Retryable  bool            `json:"retryable,omitempty"`
	OccurredAt imachinery.Time `json:"occurred_at,omitempty"`
}

// AtomicTask is the only business resource executed by a Worker handler.
type AtomicTask struct {
	imachinery.ObjectMeta
	FunctionRef          string          `json:"function_ref" gorm:"column:function_ref;type:varchar(256);not null;index"`
	Arguments            map[string]any  `json:"arguments,omitempty" gorm:"-"`
	ArgumentsShadow      string          `json:"-" gorm:"column:arguments_json;type:text;not null;default:'{}'"`
	RequiredCapabilities string          `json:"required_capabilities,omitempty" gorm:"column:required_capabilities;type:text"`
	RetryPolicy          RetryPolicy     `json:"retry_policy,omitempty" gorm:"-"`
	RetryPolicyShadow    string          `json:"-" gorm:"column:retry_policy_json;type:text;not null;default:'{}'"`
	TimeoutPolicy        TimeoutPolicy   `json:"timeout_policy,omitempty" gorm:"-"`
	TimeoutPolicyShadow  string          `json:"-" gorm:"column:timeout_policy_json;type:text;not null;default:'{}'"`
	CancelPolicy         map[string]any  `json:"cancel_policy,omitempty" gorm:"-"`
	CancelPolicyShadow   string          `json:"-" gorm:"column:cancel_policy_json;type:text;not null;default:'{}'"`
	Status               string          `json:"status" gorm:"column:status;type:varchar(32);not null;index:idx_atomic_tasks_status_schedule,priority:1"`
	Progress             float64         `json:"progress" gorm:"column:progress;not null;default:0"`
	CurrentAttempt       int             `json:"current_attempt" gorm:"column:current_attempt;not null;default:0"`
	Output               map[string]any  `json:"output,omitempty" gorm:"-"`
	OutputShadow         string          `json:"-" gorm:"column:output_json;type:text;not null;default:'{}'"`
	LastError            TaskError       `json:"last_error,omitempty" gorm:"-"`
	LastErrorShadow      string          `json:"-" gorm:"column:last_error_json;type:text;not null;default:'{}'"`
	RetryOfTaskID        string          `json:"retry_of_task_id,omitempty" gorm:"column:retry_of_task_id;type:varchar(64);index"`
	RootTaskID           string          `json:"root_task_id,omitempty" gorm:"column:root_task_id;type:varchar(64);index"`
	OwnerType            string          `json:"owner_type,omitempty" gorm:"column:owner_type;type:varchar(32)"`
	OwnerID              string          `json:"owner_id,omitempty" gorm:"column:owner_id;type:varchar(64);index"`
	ChildKey             string          `json:"child_key,omitempty" gorm:"column:child_key;type:varchar(128)"`
	ChildOrder           int             `json:"child_order,omitempty" gorm:"column:child_order;not null;default:0"`
	ApplicationRunID     string          `json:"application_run_id,omitempty" gorm:"column:application_run_id;type:varchar(64);index"`
	CanvasRunID          string          `json:"canvas_run_id,omitempty" gorm:"column:canvas_run_id;type:varchar(64);index"`
	CanvasNodeRunID      string          `json:"canvas_node_run_id,omitempty" gorm:"column:canvas_node_run_id;type:varchar(64);index"`
	IdempotencyScope     string          `json:"idempotency_scope,omitempty" gorm:"column:idempotency_scope;type:varchar(256)"`
	IdempotencyKey       string          `json:"idempotency_key,omitempty" gorm:"column:idempotency_key;type:varchar(256)"`
	RuntimeExecutionID   string          `json:"runtime_execution_id,omitempty" gorm:"column:runtime_execution_id;type:varchar(128);index"`
	RuntimeTaskID        string          `json:"runtime_task_id,omitempty" gorm:"column:runtime_task_id;type:varchar(128)"`
	RuntimeRevision      string          `json:"runtime_revision,omitempty" gorm:"column:runtime_revision;type:varchar(128)"`
	ScheduleAt           imachinery.Time `json:"schedule_at,omitempty" gorm:"column:schedule_at;index:idx_atomic_tasks_status_schedule,priority:2"`
	StartedAt            imachinery.Time `json:"started_at,omitempty" gorm:"column:started_at"`
	CompletedAt          imachinery.Time `json:"completed_at,omitempty" gorm:"column:completed_at"`
	CanceledAt           imachinery.Time `json:"canceled_at,omitempty" gorm:"column:canceled_at"`
	ProjectID            string          `json:"project_id" gorm:"column:project_id;type:varchar(128);not null;index:idx_atomic_tasks_scope,priority:1"`
	Namespace            string          `json:"namespace" gorm:"column:namespace;type:varchar(128);not null;index:idx_atomic_tasks_scope,priority:2"`
	CreatedBy            string          `json:"created_by" gorm:"column:created_by;type:varchar(128);not null"`
	Tags                 string          `json:"tags,omitempty" gorm:"column:tags;type:text"`
	DeletedAt            imachinery.Time `json:"-" gorm:"column:deleted_at;index"`
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
	return nil
}
func (t *AtomicTask) marshalShadows() error {
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
type TaskAttempt struct {
	imachinery.ObjectMeta
	AtomicTaskID   string          `json:"atomic_task_id" gorm:"column:atomic_task_id;type:varchar(64);not null;uniqueIndex:idx_task_attempts_task_no,priority:1;index"`
	AttemptNo      int             `json:"attempt_no" gorm:"column:attempt_no;not null;uniqueIndex:idx_task_attempts_task_no,priority:2"`
	RuntimeTaskID  string          `json:"runtime_task_id" gorm:"column:runtime_task_id;type:varchar(128);not null;uniqueIndex"`
	Status         string          `json:"status" gorm:"column:status;type:varchar(32);not null;index"`
	InputSnapshot  map[string]any  `json:"input_snapshot,omitempty" gorm:"-"`
	InputShadow    string          `json:"-" gorm:"column:input_snapshot_json;type:text;not null;default:'{}'"`
	OutputSnapshot map[string]any  `json:"output_snapshot,omitempty" gorm:"-"`
	OutputShadow   string          `json:"-" gorm:"column:output_snapshot_json;type:text;not null;default:'{}'"`
	Error          TaskError       `json:"error,omitempty" gorm:"-"`
	ErrorShadow    string          `json:"-" gorm:"column:error_json;type:text;not null;default:'{}'"`
	ExternalJobID  string          `json:"external_job_id,omitempty" gorm:"column:external_job_id;type:varchar(256);index"`
	LogsRef        string          `json:"logs_ref,omitempty" gorm:"column:logs_ref;type:text"`
	StartedAt      imachinery.Time `json:"started_at,omitempty" gorm:"column:started_at"`
	CompletedAt    imachinery.Time `json:"completed_at,omitempty" gorm:"column:completed_at"`
	DurationMS     int64           `json:"duration_ms" gorm:"column:duration_ms;not null;default:0"`
	Retryable      bool            `json:"retryable" gorm:"column:retryable;not null;default:false"`
}

func (TaskAttempt) TableName() string { return "task_attempts" }
func (a *TaskAttempt) BeforeCreate(tx *gorm.DB) error {
	if err := a.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return a.marshalShadows()
}
func (a *TaskAttempt) AfterCreate(*gorm.DB) error { return nil }
func (a *TaskAttempt) BeforeUpdate(tx *gorm.DB) error {
	if err := a.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
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
	return nil
}
func (a *TaskAttempt) marshalShadows() error {
	return marshalJSONFields(
		jsonField{a.InputSnapshot, &a.InputShadow, "{}"},
		jsonField{a.OutputSnapshot, &a.OutputShadow, "{}"},
		jsonField{a.Error, &a.ErrorShadow, "{}"},
	)
}

// AtomicTaskTemplate is embedded in Group, DAG, and Schedule immutable snapshots.
type AtomicTaskTemplate struct {
	Key                  string         `json:"key"`
	Name                 string         `json:"name,omitempty"`
	FunctionRef          string         `json:"function_ref"`
	Arguments            map[string]any `json:"arguments,omitempty"`
	RequiredCapabilities string         `json:"required_capabilities,omitempty"`
	RetryPolicy          RetryPolicy    `json:"retry_policy,omitempty"`
	TimeoutPolicy        TimeoutPolicy  `json:"timeout_policy,omitempty"`
}

type GroupStrategy struct {
	FailFast        bool `json:"fail_fast"`
	CancelOnFailure bool `json:"cancel_on_failure"`
	MaxParallelism  int  `json:"max_parallelism"`
}

type TaskSummary struct {
	Total    int `json:"total"`
	Pending  int `json:"pending"`
	Blocked  int `json:"blocked"`
	Running  int `json:"running"`
	Success  int `json:"success"`
	Failed   int `json:"failed"`
	Canceled int `json:"canceled"`
	Skipped  int `json:"skipped"`
}

// TaskGroup is a SERIAL or PARALLEL composition of AtomicTask templates.
type TaskGroup struct {
	imachinery.ObjectMeta
	Mode                     string               `json:"mode" gorm:"column:mode;type:varchar(16);not null"`
	Tasks                    []AtomicTaskTemplate `json:"tasks" gorm:"-"`
	TasksShadow              string               `json:"-" gorm:"column:task_templates_json;type:text;not null"`
	Strategy                 GroupStrategy        `json:"strategy" gorm:"-"`
	StrategyShadow           string               `json:"-" gorm:"column:strategy_json;type:text;not null;default:'{}'"`
	Status                   string               `json:"status" gorm:"column:status;type:varchar(32);not null;index"`
	Progress                 float64              `json:"progress" gorm:"column:progress;not null;default:0"`
	Summary                  TaskSummary          `json:"summary" gorm:"-"`
	SummaryShadow            string               `json:"-" gorm:"column:summary_json;type:text;not null;default:'{}'"`
	Result                   map[string]any       `json:"result,omitempty" gorm:"-"`
	ResultShadow             string               `json:"-" gorm:"column:result_json;type:text;not null;default:'{}'"`
	RetryOfID                string               `json:"retry_of_id,omitempty" gorm:"column:retry_of_id;type:varchar(64);index"`
	RuntimeExecutionID       string               `json:"runtime_execution_id,omitempty" gorm:"column:runtime_execution_id;type:varchar(128);index"`
	RuntimeDefinitionName    string               `json:"runtime_definition_name,omitempty" gorm:"column:runtime_definition_name;type:varchar(256)"`
	RuntimeDefinitionVersion int                  `json:"runtime_definition_version,omitempty" gorm:"column:runtime_definition_version;not null;default:0"`
	IdempotencyScope         string               `json:"idempotency_scope,omitempty" gorm:"column:idempotency_scope;type:varchar(256)"`
	IdempotencyKey           string               `json:"idempotency_key,omitempty" gorm:"column:idempotency_key;type:varchar(256)"`
	ProjectID                string               `json:"project_id" gorm:"column:project_id;type:varchar(128);not null"`
	Namespace                string               `json:"namespace" gorm:"column:namespace;type:varchar(128);not null"`
	CreatedBy                string               `json:"created_by" gorm:"column:created_by;type:varchar(128);not null"`
	DeletedAt                imachinery.Time      `json:"-" gorm:"column:deleted_at;index"`
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
	return nil
}
func (g *TaskGroup) marshalShadows() error {
	return marshalJSONFields(jsonField{g.Tasks, &g.TasksShadow, "[]"}, jsonField{g.Strategy, &g.StrategyShadow, "{}"}, jsonField{g.Summary, &g.SummaryShadow, "{}"}, jsonField{g.Result, &g.ResultShadow, "{}"})
}

type DAGNode struct {
	Key             string             `json:"key"`
	Task            AtomicTaskTemplate `json:"task"`
	InputMapping    map[string]any     `json:"input_mapping,omitempty"`
	DynamicFork     bool               `json:"dynamic_fork,omitempty"`
	MaxDynamicTasks int                `json:"max_dynamic_tasks,omitempty"`
}

type DAGEdge struct {
	FromNode    string         `json:"from_node"`
	ToNode      string         `json:"to_node"`
	DataMapping map[string]any `json:"data_mapping,omitempty"`
}

// DAGTaskGroup stores an immutable validated AtomicTask DAG execution.
type DAGTaskGroup struct {
	imachinery.ObjectMeta
	Nodes                    []DAGNode       `json:"nodes" gorm:"-"`
	NodesShadow              string          `json:"-" gorm:"column:nodes_json;type:text;not null"`
	Edges                    []DAGEdge       `json:"edges" gorm:"-"`
	EdgesShadow              string          `json:"-" gorm:"column:edges_json;type:text;not null"`
	Input                    map[string]any  `json:"input,omitempty" gorm:"-"`
	InputShadow              string          `json:"-" gorm:"column:input_mapping_json;type:text;not null;default:'{}'"`
	OutputMapping            map[string]any  `json:"output_mapping,omitempty" gorm:"-"`
	OutputMappingShadow      string          `json:"-" gorm:"column:output_mapping_json;type:text;not null;default:'{}'"`
	Status                   string          `json:"status" gorm:"column:status;type:varchar(32);not null;index"`
	Progress                 float64         `json:"progress" gorm:"column:progress;not null;default:0"`
	Summary                  TaskSummary     `json:"summary" gorm:"-"`
	SummaryShadow            string          `json:"-" gorm:"column:summary_json;type:text;not null;default:'{}'"`
	Result                   map[string]any  `json:"result,omitempty" gorm:"-"`
	ResultShadow             string          `json:"-" gorm:"column:result_json;type:text;not null;default:'{}'"`
	RetryOfID                string          `json:"retry_of_id,omitempty" gorm:"column:retry_of_id;type:varchar(64);index"`
	CanvasVersionID          string          `json:"canvas_version_id,omitempty" gorm:"column:canvas_version_id;type:varchar(64);index"`
	RuntimeExecutionID       string          `json:"runtime_execution_id,omitempty" gorm:"column:runtime_execution_id;type:varchar(128);index"`
	RuntimeDefinitionName    string          `json:"runtime_definition_name" gorm:"column:runtime_definition_name;type:varchar(256);not null;index:idx_dag_groups_definition,priority:1"`
	RuntimeDefinitionVersion int             `json:"runtime_definition_version" gorm:"column:runtime_definition_version;not null;index:idx_dag_groups_definition,priority:2"`
	RuntimeDefinitionHash    string          `json:"runtime_definition_hash" gorm:"column:runtime_definition_hash;type:varchar(128);not null"`
	IdempotencyScope         string          `json:"idempotency_scope,omitempty" gorm:"column:idempotency_scope;type:varchar(256)"`
	IdempotencyKey           string          `json:"idempotency_key,omitempty" gorm:"column:idempotency_key;type:varchar(256)"`
	ProjectID                string          `json:"project_id" gorm:"column:project_id;type:varchar(128);not null"`
	Namespace                string          `json:"namespace" gorm:"column:namespace;type:varchar(128);not null"`
	CreatedBy                string          `json:"created_by" gorm:"column:created_by;type:varchar(128);not null"`
	DeletedAt                imachinery.Time `json:"-" gorm:"column:deleted_at;index"`
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
	return nil
}
func (g *DAGTaskGroup) marshalShadows() error {
	return marshalJSONFields(jsonField{g.Nodes, &g.NodesShadow, "[]"}, jsonField{g.Edges, &g.EdgesShadow, "[]"}, jsonField{g.Input, &g.InputShadow, "{}"}, jsonField{g.OutputMapping, &g.OutputMappingShadow, "{}"}, jsonField{g.Summary, &g.SummaryShadow, "{}"}, jsonField{g.Result, &g.ResultShadow, "{}"})
}

type ScheduleTarget struct {
	Type     string         `json:"type"`
	Template map[string]any `json:"template"`
}

type ScheduleSummary struct {
	TotalTriggered int `json:"total_triggered"`
	Running        int `json:"running"`
	Success        int `json:"success"`
	Failed         int `json:"failed"`
	Canceled       int `json:"canceled"`
	SkippedOverlap int `json:"skipped_overlap"`
}

// TaskSchedule persistently triggers an AtomicTask, TaskGroup, or DAGTaskGroup template.
type TaskSchedule struct {
	imachinery.ObjectMeta
	TriggerType          string          `json:"trigger_type" gorm:"column:trigger_type;type:varchar(16);not null"`
	CronExpression       string          `json:"cron_expression,omitempty" gorm:"column:cron_expression;type:varchar(256)"`
	RunAt                imachinery.Time `json:"run_at,omitempty" gorm:"column:run_at"`
	TimeZone             string          `json:"time_zone" gorm:"column:time_zone;type:varchar(128);not null;default:'UTC'"`
	Target               ScheduleTarget  `json:"target" gorm:"-"`
	TargetType           string          `json:"-" gorm:"column:target_type;type:varchar(32);not null"`
	TargetTemplateShadow string          `json:"-" gorm:"column:target_template_json;type:text;not null"`
	Status               string          `json:"status" gorm:"column:status;type:varchar(32);not null;index:idx_task_schedules_status_next,priority:1"`
	MisfirePolicy        string          `json:"misfire_policy" gorm:"column:misfire_policy;type:varchar(16);not null;default:'SKIP'"`
	OverlapPolicy        string          `json:"overlap_policy" gorm:"column:overlap_policy;type:varchar(16);not null;default:'SKIP'"`
	RuntimeScheduleName  string          `json:"runtime_schedule_name,omitempty" gorm:"column:runtime_schedule_name;type:varchar(256);uniqueIndex"`
	LastTriggerAt        imachinery.Time `json:"last_trigger_at,omitempty" gorm:"column:last_trigger_at"`
	NextTriggerAt        imachinery.Time `json:"next_trigger_at,omitempty" gorm:"column:next_trigger_at;index:idx_task_schedules_status_next,priority:2"`
	Summary              ScheduleSummary `json:"summary" gorm:"-"`
	SummaryShadow        string          `json:"-" gorm:"column:summary_json;type:text;not null;default:'{}'"`
	ProjectID            string          `json:"project_id" gorm:"column:project_id;type:varchar(128);not null;index"`
	Namespace            string          `json:"namespace" gorm:"column:namespace;type:varchar(128);not null;index"`
	CreatedBy            string          `json:"created_by" gorm:"column:created_by;type:varchar(128);not null"`
	DeletedAt            imachinery.Time `json:"-" gorm:"column:deleted_at;index"`
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
	unmarshalJSON(s.SummaryShadow, &s.Summary, "{}")
	return nil
}
func (s *TaskSchedule) marshalShadows() error {
	s.TargetType = s.Target.Type
	return marshalJSONFields(jsonField{s.Target.Template, &s.TargetTemplateShadow, "{}"}, jsonField{s.Summary, &s.SummaryShadow, "{}"})
}

// TaskScheduleExecution records every scheduled time, including overlap skips.
type TaskScheduleExecution struct {
	imachinery.ObjectMeta
	ScheduleID         string          `json:"schedule_id" gorm:"column:schedule_id;type:varchar(64);not null;uniqueIndex:idx_schedule_execution_time,priority:1;index"`
	ScheduledAt        imachinery.Time `json:"scheduled_at" gorm:"column:scheduled_at;not null;uniqueIndex:idx_schedule_execution_time,priority:2"`
	TriggeredAt        imachinery.Time `json:"triggered_at,omitempty" gorm:"column:triggered_at"`
	TargetType         string          `json:"target_type" gorm:"column:target_type;type:varchar(32);not null"`
	TargetID           string          `json:"target_id,omitempty" gorm:"column:target_id;type:varchar(64);index"`
	RuntimeExecutionID string          `json:"runtime_execution_id,omitempty" gorm:"column:runtime_execution_id;type:varchar(128);index"`
	Status             string          `json:"status" gorm:"column:status;type:varchar(32);not null;index:idx_schedule_executions_status,priority:1"`
	Reason             string          `json:"reason,omitempty" gorm:"column:reason;type:text"`
	CompletedAt        imachinery.Time `json:"completed_at,omitempty" gorm:"column:completed_at"`
}

func (TaskScheduleExecution) TableName() string                 { return "task_schedule_executions" }
func (e *TaskScheduleExecution) BeforeCreate(tx *gorm.DB) error { return e.ObjectMeta.BeforeCreate(tx) }
func (e *TaskScheduleExecution) AfterCreate(*gorm.DB) error     { return nil }
func (e *TaskScheduleExecution) BeforeUpdate(tx *gorm.DB) error { return e.ObjectMeta.BeforeUpdate(tx) }
func (e *TaskScheduleExecution) AfterUpdate(*gorm.DB) error     { return nil }
func (e *TaskScheduleExecution) AfterFind(tx *gorm.DB) error    { return e.ObjectMeta.AfterFind(tx) }

// RuntimeProjectionEvent makes Conductor event projection idempotent and replayable.
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
