package iapiserver

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	// TaskDefinitionTypeAtomic 表示单个可执行任务定义，来源于 task-center S2。
	TaskDefinitionTypeAtomic = "ATOMIC"
	// TaskDefinitionTypeGroup 表示只承载 SERIAL/PARALLEL 的任务组定义，来源于 task-center S2。
	TaskDefinitionTypeGroup = "TASK_GROUP"
	// TaskDefinitionTypeDAGFlow 表示 DAG 编排任务定义，来源于 task-center S2。
	TaskDefinitionTypeDAGFlow = "DAG_FLOW"

	// TaskGroupTypeSerial 表示子任务按顺序执行。
	TaskGroupTypeSerial = "SERIAL"
	// TaskGroupTypeParallel 表示子任务可并行执行。
	TaskGroupTypeParallel = "PARALLEL"
)

const (
	// TaskRunStatusPending 表示运行实例已创建但尚未进入可领取队列。
	TaskRunStatusPending = "PENDING"
	// TaskRunStatusReady 表示运行实例可被匹配能力的 Worker 领取。
	TaskRunStatusReady = "READY"
	// TaskRunStatusClaimed 表示运行实例已被 Worker 领取并持有 lease。
	TaskRunStatusClaimed = "CLAIMED"
	// TaskRunStatusRunning 表示 Worker 已开始执行并可上报进度。
	TaskRunStatusRunning = "RUNNING"
	// TaskRunStatusRetrying 表示运行实例正在等待下一次重试。
	TaskRunStatusRetrying = "RETRYING"
	// TaskRunStatusCancelRequested 表示用户已请求取消，最终状态仍取决于 Worker/外部执行器。
	TaskRunStatusCancelRequested = "CANCEL_REQUESTED"
	// TaskRunStatusPaused 为 SSOT 预留暂停状态，当前探索实现不主动产生。
	TaskRunStatusPaused = "PAUSED"
	// TaskRunStatusSuccess 表示运行实例成功终态。
	TaskRunStatusSuccess = "SUCCESS"
	// TaskRunStatusFailed 表示运行实例失败终态。
	TaskRunStatusFailed = "FAILED"
	// TaskRunStatusCanceled 表示运行实例取消终态。
	TaskRunStatusCanceled = "CANCELED"
	// TaskRunStatusTimeout 表示运行实例整体超时终态。
	TaskRunStatusTimeout = "TIMEOUT"
	// TaskRunStatusLost 表示运行实例因 Worker/lease 异常进入丢失状态。
	TaskRunStatusLost = "LOST"
)

const (
	// TaskAttemptStatusClaimed 表示一次执行尝试已创建但 Worker 尚未上报运行。
	TaskAttemptStatusClaimed = "CLAIMED"
	// TaskAttemptStatusRunning 表示一次执行尝试正在运行。
	TaskAttemptStatusRunning = "RUNNING"
	// TaskAttemptStatusSuccess 表示一次执行尝试成功结束。
	TaskAttemptStatusSuccess = "SUCCESS"
	// TaskAttemptStatusFailed 表示一次执行尝试业务失败。
	TaskAttemptStatusFailed = "FAILED"
	// TaskAttemptStatusTimeout 表示一次执行尝试超时。
	TaskAttemptStatusTimeout = "TIMEOUT"
	// TaskAttemptStatusCanceled 表示一次执行尝试响应取消。
	TaskAttemptStatusCanceled = "CANCELED"
	// TaskAttemptStatusWorkerLost 表示一次执行尝试因 Worker 失联结束。
	TaskAttemptStatusWorkerLost = "WORKER_LOST"
	// TaskAttemptStatusLeaseExpired 表示一次执行尝试因 lease 过期结束。
	TaskAttemptStatusLeaseExpired = "LEASE_EXPIRED"
	// TaskAttemptStatusStalled 表示一次执行尝试长时间无进度。
	TaskAttemptStatusStalled = "STALLED"
)

const (
	// WorkerStatusOnline 表示 Worker 可领取任务。
	WorkerStatusOnline = "ONLINE"
	// WorkerStatusBusy 表示 Worker 在线但已达到并发上限。
	WorkerStatusBusy = "BUSY"
	// WorkerStatusDraining 表示 Worker 正在下线，不再领取新任务。
	WorkerStatusDraining = "DRAINING"
	// WorkerStatusOffline 表示 Worker 主动离线。
	WorkerStatusOffline = "OFFLINE"
	// WorkerStatusLost 表示 watchdog 判定 Worker 心跳超时。
	WorkerStatusLost = "LOST"
	// WorkerStatusDisabled 表示 Worker 被运维禁用。
	WorkerStatusDisabled = "DISABLED"
)

const (
	// LeaseStatusActive 表示 lease 当前有效。
	LeaseStatusActive = "ACTIVE"
	// LeaseStatusRenewed 表示 lease 已续约且当前有效。
	LeaseStatusRenewed = "RENEWED"
	// LeaseStatusExpired 表示 lease 已过期。
	LeaseStatusExpired = "EXPIRED"
	// LeaseStatusReleased 表示 lease 已随成功/失败结果释放。
	LeaseStatusReleased = "RELEASED"
	// LeaseStatusRevoked 表示 lease 被任务中心主动撤销。
	LeaseStatusRevoked = "REVOKED"
)

const (
	FailureTypeFunctionError         = "FUNCTION_ERROR"
	FailureTypeTimeout               = "TIMEOUT"
	FailureTypeCanceled              = "CANCELED"
	FailureTypeWorkerLost            = "WORKER_LOST"
	FailureTypeLeaseExpired          = "LEASE_EXPIRED"
	FailureTypeExternalExecutorError = "EXTERNAL_EXECUTOR_ERROR"
	FailureTypeSystemError           = "SYSTEM_ERROR"
	FailureTypeStalled               = "STALLED"
	TaskCenterEventRunCreated        = "task_run_created"
	TaskCenterEventStatusChanged     = "task_run_status_changed"
	TaskCenterEventProgressUpdated   = "task_run_progress_updated"
	TaskCenterEventAttemptFailed     = "task_attempt_failed"
	TaskCenterEventWorkerLost        = "worker_lost"
	TaskCenterEventLeaseExpired      = "lease_expired"
	DefaultTaskCenterLeaseDuration   = 5 * time.Minute
	DefaultTaskCenterProjectID       = "default"
	DefaultTaskCenterNamespace       = "default"
	DefaultTaskCenterCreatedBy       = "system"
)

// TaskCenterMeta 使用 task-center OpenAPI 指定的 createdAt/updatedAt 字段；因此不嵌入 ObjectMeta。
type TaskCenterMeta struct {
	ID           string          `json:"id,omitempty"          gorm:"primary_key;column:id;type:varchar(64)"`
	Name         string          `json:"name,omitempty"        gorm:"column:name;type:varchar(128);not null"`
	CreatedAt    imachinery.Time `json:"createdAt,omitempty"   gorm:"column:createdAt;not null"`
	UpdatedAt    imachinery.Time `json:"updatedAt,omitempty"   gorm:"column:updatedAt;not null"`
	Description  string          `json:"description,omitempty" gorm:"column:description;type:text;default:''"`
	Extend       map[string]any  `json:"extend,omitempty"      gorm:"-"`
	ExtendShadow string          `json:"-"                     gorm:"column:extend_shadow;type:text;default:''"`
}

func (m *TaskCenterMeta) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	now := imachinery.NewTime(time.Now())
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	return m.marshalExtend()
}

func (m *TaskCenterMeta) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = imachinery.NewTime(time.Now())
	return m.marshalExtend()
}

func (m *TaskCenterMeta) AfterFind(tx *gorm.DB) error {
	if m.ExtendShadow == "" {
		m.Extend = map[string]any{}
		return nil
	}
	return json.Unmarshal([]byte(m.ExtendShadow), &m.Extend)
}

func (m *TaskCenterMeta) marshalExtend() error {
	if m.Extend == nil {
		m.Extend = map[string]any{}
	}
	data, err := json.Marshal(m.Extend)
	if err != nil {
		return err
	}
	m.ExtendShadow = string(data)
	return nil
}

type RetryPolicy struct {
	MaxRetries            int    `json:"max_retries,omitempty"`
	RetryDelay            string `json:"retry_delay,omitempty"`
	BackoffType           string `json:"backoff_type,omitempty"`
	MaxRetryDelay         string `json:"max_retry_delay,omitempty"`
	RetryableFailureTypes string `json:"retryable_failure_types,omitempty"`
	MaxRetryDuration      string `json:"max_retry_duration,omitempty"`
}

type TimeoutPolicy struct {
	PerAttemptTimeout string `json:"per_attempt_timeout,omitempty"`
	OverallTimeout    string `json:"overall_timeout,omitempty"`
}

type StrategyConfig struct {
	FailFast        bool   `json:"fail_fast,omitempty"`
	CancelOnFailure bool   `json:"cancel_on_failure,omitempty"`
	MaxParallelism  int    `json:"max_parallelism,omitempty"`
	ResultPolicy    string `json:"result_policy,omitempty"`
	GroupRetryMode  string `json:"group_retry_mode,omitempty"`
}

type TaskDefinitionChild struct {
	DefinitionType string `json:"definition_type"`
	DefinitionID   string `json:"definition_id"`
	SortOrder      int    `json:"sort_order,omitempty"`
}

type TaskRef struct {
	DefinitionType string `json:"definition_type"`
	DefinitionID   string `json:"definition_id"`
}

type DAGNode struct {
	NodeID        string         `json:"node_id"`
	Name          string         `json:"name"`
	TaskRef       TaskRef        `json:"task_ref"`
	InputMapping  map[string]any `json:"input_mapping,omitempty"`
	OutputMapping map[string]any `json:"output_mapping,omitempty"`
	TimeoutPolicy TimeoutPolicy  `json:"timeout_policy,omitempty"`
	RetryPolicy   RetryPolicy    `json:"retry_policy,omitempty"`
}

type DAGEdge struct {
	FromNodeID  string         `json:"from_node_id"`
	ToNodeID    string         `json:"to_node_id"`
	Condition   string         `json:"condition,omitempty"`
	DataMapping map[string]any `json:"data_mapping,omitempty"`
}

// TaskDefinition 保存 AtomicTask、TaskGroup 与 DAGFlowTask 的统一定义表，字段来自 task-center S2 schema.sql。
type TaskDefinition struct {
	TaskCenterMeta
	DefinitionType         string                 `json:"definition_type"                   gorm:"column:definition_type;type:varchar(32);not null;index"`
	FunctionRef            string                 `json:"function_ref,omitempty"            gorm:"column:function_ref;type:varchar(256);index"`
	AppID                  string                 `json:"app_id,omitempty"                  gorm:"column:app_id;type:varchar(128)"`
	EngineRef              string                 `json:"engine_ref,omitempty"              gorm:"column:engine_ref;type:varchar(256)"`
	DefaultArguments       map[string]any         `json:"default_arguments,omitempty"       gorm:"-"`
	DefaultArgumentsShadow string                 `json:"-"                                gorm:"column:default_arguments_json;type:text;not null;default:'{}'"`
	RequiredCapabilities   string                 `json:"required_capabilities,omitempty"   gorm:"column:required_capabilities;type:text"`
	GroupType              string                 `json:"group_type,omitempty"              gorm:"column:group_type;type:varchar(32)"`
	Children               []TaskDefinitionChild  `json:"children,omitempty"                gorm:"-"`
	ChildrenShadow         string                 `json:"-"                                gorm:"column:children_json;type:text;not null;default:'[]'"`
	DAGNodes               []DAGNode              `json:"nodes,omitempty"                   gorm:"-"`
	DAGNodesShadow         string                 `json:"-"                                gorm:"column:dag_nodes_json;type:text;not null;default:'[]'"`
	DAGEdges               []DAGEdge              `json:"edges,omitempty"                   gorm:"-"`
	DAGEdgesShadow         string                 `json:"-"                                gorm:"column:dag_edges_json;type:text;not null;default:'[]'"`
	InputMapping           map[string]any         `json:"input_mapping,omitempty"           gorm:"-"`
	OutputMapping          map[string]any         `json:"output_mapping,omitempty"          gorm:"-"`
	StrategyConfig         StrategyConfig         `json:"strategy_config,omitempty"         gorm:"-"`
	StrategyConfigShadow   string                 `json:"-"                                gorm:"column:strategy_config_json;type:text;not null;default:'{}'"`
	TimeoutPolicy          TimeoutPolicy          `json:"timeout_policy,omitempty"          gorm:"-"`
	TimeoutPolicyShadow    string                 `json:"-"                                gorm:"column:timeout_policy_json;type:text;not null;default:'{}'"`
	RetryPolicy            RetryPolicy            `json:"retry_policy,omitempty"            gorm:"-"`
	RetryPolicyShadow      string                 `json:"-"                                gorm:"column:retry_policy_json;type:text;not null;default:'{}'"`
	CancelPolicy           map[string]any         `json:"cancel_policy,omitempty"           gorm:"-"`
	CancelPolicyShadow     string                 `json:"-"                                gorm:"column:cancel_policy_json;type:text;not null;default:'{}'"`
	Tags                   string                 `json:"tags,omitempty"                    gorm:"column:tags;type:text"`
	ProjectID              string                 `json:"project_id,omitempty"              gorm:"column:project_id;type:varchar(128);not null;index:idx_task_definitions_project_namespace,priority:1"`
	Namespace              string                 `json:"namespace,omitempty"               gorm:"column:namespace;type:varchar(128);not null;index:idx_task_definitions_project_namespace,priority:2"`
	CreatedBy              string                 `json:"created_by,omitempty"              gorm:"column:created_by;type:varchar(128);not null"`
	DeletedAt              imachinery.Time        `json:"-"                                gorm:"column:deleted_at"`
	AdditionalProperties   map[string]interface{} `json:"-"                                gorm:"-"`
}

type AtomicTask = TaskDefinition
type TaskGroup = TaskDefinition
type DAGFlowTask = TaskDefinition

func (TaskDefinition) TableName() string { return "task_definitions" }

func (d *TaskDefinition) BeforeCreate(tx *gorm.DB) error {
	if err := d.TaskCenterMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return d.marshalShadows()
}

func (d *TaskDefinition) AfterCreate(tx *gorm.DB) error { return nil }

func (d *TaskDefinition) BeforeUpdate(tx *gorm.DB) error {
	if err := d.TaskCenterMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return d.marshalShadows()
}

func (d *TaskDefinition) AfterUpdate(tx *gorm.DB) error { return nil }

func (d *TaskDefinition) AfterFind(tx *gorm.DB) error {
	if err := d.TaskCenterMeta.AfterFind(tx); err != nil {
		return err
	}
	_ = json.Unmarshal([]byte(defaultJSON(d.DefaultArgumentsShadow, "{}")), &d.DefaultArguments)
	_ = json.Unmarshal([]byte(defaultJSON(d.ChildrenShadow, "[]")), &d.Children)
	_ = json.Unmarshal([]byte(defaultJSON(d.DAGNodesShadow, "[]")), &d.DAGNodes)
	_ = json.Unmarshal([]byte(defaultJSON(d.DAGEdgesShadow, "[]")), &d.DAGEdges)
	_ = json.Unmarshal([]byte(defaultJSON(d.StrategyConfigShadow, "{}")), &d.StrategyConfig)
	_ = json.Unmarshal([]byte(defaultJSON(d.TimeoutPolicyShadow, "{}")), &d.TimeoutPolicy)
	_ = json.Unmarshal([]byte(defaultJSON(d.RetryPolicyShadow, "{}")), &d.RetryPolicy)
	_ = json.Unmarshal([]byte(defaultJSON(d.CancelPolicyShadow, "{}")), &d.CancelPolicy)
	return nil
}

func (d *TaskDefinition) marshalShadows() error {
	return marshalJSONShadows([]jsonShadow{
		{value: &d.DefaultArguments, target: &d.DefaultArgumentsShadow, fallback: "{}"},
		{value: &d.Children, target: &d.ChildrenShadow, fallback: "[]"},
		{value: &d.DAGNodes, target: &d.DAGNodesShadow, fallback: "[]"},
		{value: &d.DAGEdges, target: &d.DAGEdgesShadow, fallback: "[]"},
		{value: &d.StrategyConfig, target: &d.StrategyConfigShadow, fallback: "{}"},
		{value: &d.TimeoutPolicy, target: &d.TimeoutPolicyShadow, fallback: "{}"},
		{value: &d.RetryPolicy, target: &d.RetryPolicyShadow, fallback: "{}"},
		{value: &d.CancelPolicy, target: &d.CancelPolicyShadow, fallback: "{}"},
	})
}

type TaskError struct {
	Code        string          `json:"code,omitempty"`
	Message     string          `json:"message,omitempty"`
	FailureType string          `json:"failure_type,omitempty"`
	Detail      string          `json:"detail,omitempty"`
	Retryable   bool            `json:"retryable,omitempty"`
	OccurredAt  imachinery.Time `json:"occurred_at,omitempty"`
}

// TaskRun 表示某次任务定义的实际运行实例，状态机来源于 task-center S1/S2。
type TaskRun struct {
	TaskCenterMeta
	DefinitionType  string          `json:"definition_type"          gorm:"column:definition_type;type:varchar(32);not null;index:idx_task_runs_definition,priority:1"`
	DefinitionID    string          `json:"definition_id"            gorm:"column:definition_id;type:varchar(64);not null;index:idx_task_runs_definition,priority:2"`
	ParentRunID     string          `json:"parent_run_id,omitempty"  gorm:"column:parent_run_id;type:varchar(64);index"`
	RootRunID       string          `json:"root_run_id,omitempty"    gorm:"column:root_run_id;type:varchar(64);index"`
	Status          string          `json:"status"                   gorm:"column:status;type:varchar(32);not null;index"`
	ScheduleAt      imachinery.Time `json:"schedule_at,omitempty"    gorm:"column:schedule_at;index"`
	TimeoutAt       imachinery.Time `json:"timeout_at,omitempty"     gorm:"column:timeout_at"`
	CurrentAttempt  int             `json:"current_attempt"          gorm:"column:current_attempt;not null;default:0"`
	MaxAttempts     int             `json:"max_attempts"             gorm:"column:max_attempts;not null;default:1"`
	Progress        float64         `json:"progress"                 gorm:"column:progress;not null;default:0"`
	Input           map[string]any  `json:"input,omitempty"          gorm:"-"`
	InputShadow     string          `json:"-"                        gorm:"column:input_json;type:text;not null;default:'{}'"`
	Output          map[string]any  `json:"output,omitempty"         gorm:"-"`
	OutputShadow    string          `json:"-"                        gorm:"column:output_json;type:text;not null;default:'{}'"`
	LastError       TaskError       `json:"last_error,omitempty"     gorm:"-"`
	LastErrorShadow string          `json:"-"                        gorm:"column:last_error_json;type:text;not null;default:'{}'"`
	StartedAt       imachinery.Time `json:"started_at,omitempty"     gorm:"column:started_at"`
	CompletedAt     imachinery.Time `json:"completed_at,omitempty"   gorm:"column:completed_at"`
	CanceledAt      imachinery.Time `json:"canceled_at,omitempty"    gorm:"column:canceled_at"`
	CreatedBy       string          `json:"created_by,omitempty"     gorm:"column:created_by;type:varchar(128);not null"`
	ProjectID       string          `json:"project_id,omitempty"     gorm:"column:project_id;type:varchar(128);not null;index:idx_task_runs_project_namespace,priority:1"`
	Namespace       string          `json:"namespace,omitempty"      gorm:"column:namespace;type:varchar(128);not null;index:idx_task_runs_project_namespace,priority:2"`
	Tags            string          `json:"tags,omitempty"           gorm:"column:tags;type:text"`
	DeletedAt       imachinery.Time `json:"-"                       gorm:"column:deleted_at"`
}

func (TaskRun) TableName() string { return "task_runs" }

func (r *TaskRun) BeforeCreate(tx *gorm.DB) error {
	if err := r.TaskCenterMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return r.marshalShadows()
}

func (r *TaskRun) AfterCreate(tx *gorm.DB) error { return nil }

func (r *TaskRun) BeforeUpdate(tx *gorm.DB) error {
	if err := r.TaskCenterMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return r.marshalShadows()
}

func (r *TaskRun) AfterUpdate(tx *gorm.DB) error { return nil }

func (r *TaskRun) AfterFind(tx *gorm.DB) error {
	if err := r.TaskCenterMeta.AfterFind(tx); err != nil {
		return err
	}
	_ = json.Unmarshal([]byte(defaultJSON(r.InputShadow, "{}")), &r.Input)
	_ = json.Unmarshal([]byte(defaultJSON(r.OutputShadow, "{}")), &r.Output)
	_ = json.Unmarshal([]byte(defaultJSON(r.LastErrorShadow, "{}")), &r.LastError)
	return nil
}

func (r *TaskRun) marshalShadows() error {
	return marshalJSONShadows([]jsonShadow{
		{value: &r.Input, target: &r.InputShadow, fallback: "{}"},
		{value: &r.Output, target: &r.OutputShadow, fallback: "{}"},
		{value: &r.LastError, target: &r.LastErrorShadow, fallback: "{}"},
	})
}

// TaskAttempt 记录 Worker 对 TaskRun 的一次执行尝试，历史不可覆盖。
type TaskAttempt struct {
	TaskCenterMeta
	RunID                string          `json:"run_id"                  gorm:"column:run_id;type:varchar(64);not null;uniqueIndex:idx_task_attempts_run_attempt_no,priority:1;index"`
	AttemptNo            int             `json:"attempt_no"              gorm:"column:attempt_no;not null;uniqueIndex:idx_task_attempts_run_attempt_no,priority:2"`
	WorkerID             string          `json:"worker_id,omitempty"     gorm:"column:worker_id;type:varchar(64);index"`
	LeaseID              string          `json:"lease_id,omitempty"      gorm:"column:lease_id;type:varchar(64);index"`
	Status               string          `json:"status"                  gorm:"column:status;type:varchar(32);not null;index"`
	InputSnapshot        map[string]any  `json:"input_snapshot,omitempty"  gorm:"-"`
	InputSnapshotShadow  string          `json:"-"                       gorm:"column:input_snapshot_json;type:text;not null;default:'{}'"`
	OutputSnapshot       map[string]any  `json:"output_snapshot,omitempty" gorm:"-"`
	OutputSnapshotShadow string          `json:"-"                       gorm:"column:output_snapshot_json;type:text;not null;default:'{}'"`
	Error                TaskError       `json:"error,omitempty"         gorm:"-"`
	ErrorShadow          string          `json:"-"                       gorm:"column:error_json;type:text;not null;default:'{}'"`
	StartedAt            imachinery.Time `json:"started_at"              gorm:"column:started_at;not null"`
	HeartbeatAt          imachinery.Time `json:"heartbeat_at,omitempty"  gorm:"column:heartbeat_at"`
	ProgressAt           imachinery.Time `json:"progress_at,omitempty"   gorm:"column:progress_at"`
	CompletedAt          imachinery.Time `json:"completed_at,omitempty"  gorm:"column:completed_at"`
	DurationMS           int64           `json:"duration_ms"             gorm:"column:duration_ms;not null;default:0"`
	Retryable            bool            `json:"retryable"               gorm:"column:retryable;not null;default:false"`
	FailureType          string          `json:"failure_type,omitempty"  gorm:"column:failure_type;type:varchar(64)"`
	LogsRef              string          `json:"logs_ref,omitempty"      gorm:"column:logs_ref;type:text"`
	ExternalJobID        string          `json:"external_job_id,omitempty" gorm:"column:external_job_id;type:varchar(128)"`
}

func (TaskAttempt) TableName() string { return "task_attempts" }

func (a *TaskAttempt) BeforeCreate(tx *gorm.DB) error {
	if err := a.TaskCenterMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return a.marshalShadows()
}

func (a *TaskAttempt) AfterCreate(tx *gorm.DB) error { return nil }

func (a *TaskAttempt) BeforeUpdate(tx *gorm.DB) error {
	if err := a.TaskCenterMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return a.marshalShadows()
}

func (a *TaskAttempt) AfterUpdate(tx *gorm.DB) error { return nil }

func (a *TaskAttempt) AfterFind(tx *gorm.DB) error {
	if err := a.TaskCenterMeta.AfterFind(tx); err != nil {
		return err
	}
	_ = json.Unmarshal([]byte(defaultJSON(a.InputSnapshotShadow, "{}")), &a.InputSnapshot)
	_ = json.Unmarshal([]byte(defaultJSON(a.OutputSnapshotShadow, "{}")), &a.OutputSnapshot)
	_ = json.Unmarshal([]byte(defaultJSON(a.ErrorShadow, "{}")), &a.Error)
	return nil
}

func (a *TaskAttempt) marshalShadows() error {
	return marshalJSONShadows([]jsonShadow{
		{value: &a.InputSnapshot, target: &a.InputSnapshotShadow, fallback: "{}"},
		{value: &a.OutputSnapshot, target: &a.OutputSnapshotShadow, fallback: "{}"},
		{value: &a.Error, target: &a.ErrorShadow, fallback: "{}"},
	})
}

// Worker 表示任务中心侧执行进程的生命周期与能力声明。
type Worker struct {
	TaskCenterMeta
	WorkerType     string          `json:"worker_type"     gorm:"column:worker_type;type:varchar(128);not null;index"`
	Status         string          `json:"status"          gorm:"column:status;type:varchar(32);not null;index"`
	Capabilities   string          `json:"capabilities"    gorm:"column:capabilities;type:text;not null"`
	Labels         string          `json:"labels,omitempty" gorm:"column:labels;type:text"`
	MaxConcurrency int             `json:"max_concurrency" gorm:"column:max_concurrency;not null;default:1"`
	RunningCount   int             `json:"running_count"   gorm:"column:running_count;not null;default:0"`
	HeartbeatAt    imachinery.Time `json:"heartbeat_at"    gorm:"column:heartbeat_at;not null;index"`
	RegisteredAt   imachinery.Time `json:"registered_at"   gorm:"column:registered_at;not null"`
	LastSeenAt     imachinery.Time `json:"last_seen_at"    gorm:"column:last_seen_at;not null"`
}

func (Worker) TableName() string { return "task_workers" }

func (w *Worker) BeforeCreate(tx *gorm.DB) error {
	if err := w.TaskCenterMeta.BeforeCreate(tx); err != nil {
		return err
	}
	now := imachinery.NewTime(time.Now())
	if w.RegisteredAt.IsZero() {
		w.RegisteredAt = now
	}
	if w.HeartbeatAt.IsZero() {
		w.HeartbeatAt = now
	}
	if w.LastSeenAt.IsZero() {
		w.LastSeenAt = now
	}
	return nil
}

func (w *Worker) AfterCreate(tx *gorm.DB) error { return nil }

func (w *Worker) BeforeUpdate(tx *gorm.DB) error { return w.TaskCenterMeta.BeforeUpdate(tx) }
func (w *Worker) AfterUpdate(tx *gorm.DB) error  { return nil }
func (w *Worker) AfterFind(tx *gorm.DB) error    { return w.TaskCenterMeta.AfterFind(tx) }

// ExecutionLease 限定同一 TaskRun 同一时间只能由一个 Worker 持有执行权。
type ExecutionLease struct {
	TaskCenterMeta
	RunID      string          `json:"run_id"      gorm:"column:run_id;type:varchar(64);not null;index"`
	AttemptID  string          `json:"attempt_id"  gorm:"column:attempt_id;type:varchar(64);not null;index"`
	WorkerID   string          `json:"worker_id"   gorm:"column:worker_id;type:varchar(64);not null;index"`
	AcquiredAt imachinery.Time `json:"acquired_at" gorm:"column:acquired_at;not null"`
	ExpireAt   imachinery.Time `json:"expire_at"   gorm:"column:expire_at;not null;index"`
	RenewedAt  imachinery.Time `json:"renewed_at,omitempty" gorm:"column:renewed_at"`
	Status     string          `json:"status"      gorm:"column:status;type:varchar(32);not null;index"`
}

func (ExecutionLease) TableName() string { return "task_execution_leases" }

func (l *ExecutionLease) BeforeCreate(tx *gorm.DB) error {
	if err := l.TaskCenterMeta.BeforeCreate(tx); err != nil {
		return err
	}
	now := imachinery.NewTime(time.Now())
	if l.AcquiredAt.IsZero() {
		l.AcquiredAt = now
	}
	return nil
}

func (l *ExecutionLease) AfterCreate(tx *gorm.DB) error  { return nil }
func (l *ExecutionLease) BeforeUpdate(tx *gorm.DB) error { return l.TaskCenterMeta.BeforeUpdate(tx) }
func (l *ExecutionLease) AfterUpdate(tx *gorm.DB) error  { return nil }
func (l *ExecutionLease) AfterFind(tx *gorm.DB) error    { return l.TaskCenterMeta.AfterFind(tx) }

// TaskRunEvent 保存 task-center S2 events.yaml 中定义的内部事件事实。
type TaskRunEvent struct {
	TaskCenterMeta
	RunID         string          `json:"run_id"                   gorm:"column:run_id;type:varchar(64);not null;index"`
	AttemptID     string          `json:"attempt_id,omitempty"     gorm:"column:attempt_id;type:varchar(64)"`
	WorkerID      string          `json:"worker_id,omitempty"      gorm:"column:worker_id;type:varchar(64)"`
	EventType     string          `json:"event_type"               gorm:"column:event_type;type:varchar(128);not null;index"`
	FromStatus    string          `json:"from_status,omitempty"    gorm:"column:from_status;type:varchar(32)"`
	ToStatus      string          `json:"to_status,omitempty"      gorm:"column:to_status;type:varchar(32)"`
	Payload       map[string]any  `json:"payload,omitempty"        gorm:"-"`
	PayloadShadow string          `json:"-"                        gorm:"column:payload_json;type:text;not null;default:'{}'"`
	OccurredAt    imachinery.Time `json:"occurred_at"              gorm:"column:occurred_at;not null;index"`
}

func (TaskRunEvent) TableName() string { return "task_run_events" }

func (e *TaskRunEvent) BeforeCreate(tx *gorm.DB) error {
	if err := e.TaskCenterMeta.BeforeCreate(tx); err != nil {
		return err
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = imachinery.NewTime(time.Now())
	}
	return marshalJSONShadows([]jsonShadow{{value: &e.Payload, target: &e.PayloadShadow, fallback: "{}"}})
}

func (e *TaskRunEvent) AfterCreate(tx *gorm.DB) error { return nil }

func (e *TaskRunEvent) BeforeUpdate(tx *gorm.DB) error {
	if err := e.TaskCenterMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return marshalJSONShadows([]jsonShadow{{value: &e.Payload, target: &e.PayloadShadow, fallback: "{}"}})
}

func (e *TaskRunEvent) AfterUpdate(tx *gorm.DB) error { return nil }

func (e *TaskRunEvent) AfterFind(tx *gorm.DB) error {
	if err := e.TaskCenterMeta.AfterFind(tx); err != nil {
		return err
	}
	_ = json.Unmarshal([]byte(defaultJSON(e.PayloadShadow, "{}")), &e.Payload)
	return nil
}

// WatchdogRecord 保存 watchdog 对 Worker、Lease、Attempt 和 Run 的异常扫描结果。
type WatchdogRecord struct {
	TaskCenterMeta
	ScanType     string          `json:"scan_type"     gorm:"column:scan_type;type:varchar(64);not null"`
	TargetType   string          `json:"target_type"   gorm:"column:target_type;type:varchar(64);not null;index:idx_task_watchdog_records_target,priority:1"`
	TargetID     string          `json:"target_id"     gorm:"column:target_id;type:varchar(64);not null;index:idx_task_watchdog_records_target,priority:2"`
	ActionTaken  string          `json:"action_taken"  gorm:"column:action_taken;type:varchar(128);not null"`
	ResultStatus string          `json:"result_status" gorm:"column:result_status;type:varchar(64);not null"`
	Detail       map[string]any  `json:"detail,omitempty" gorm:"-"`
	DetailShadow string          `json:"-"            gorm:"column:detail_json;type:text;not null;default:'{}'"`
	OccurredAt   imachinery.Time `json:"occurred_at"  gorm:"column:occurred_at;not null;index"`
}

func (WatchdogRecord) TableName() string { return "task_watchdog_records" }

func (r *WatchdogRecord) BeforeCreate(tx *gorm.DB) error {
	if err := r.TaskCenterMeta.BeforeCreate(tx); err != nil {
		return err
	}
	if r.OccurredAt.IsZero() {
		r.OccurredAt = imachinery.NewTime(time.Now())
	}
	return marshalJSONShadows([]jsonShadow{{value: &r.Detail, target: &r.DetailShadow, fallback: "{}"}})
}

func (r *WatchdogRecord) AfterCreate(tx *gorm.DB) error { return nil }

func (r *WatchdogRecord) BeforeUpdate(tx *gorm.DB) error {
	if err := r.TaskCenterMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return marshalJSONShadows([]jsonShadow{{value: &r.Detail, target: &r.DetailShadow, fallback: "{}"}})
}

func (r *WatchdogRecord) AfterUpdate(tx *gorm.DB) error { return nil }

func (r *WatchdogRecord) AfterFind(tx *gorm.DB) error {
	if err := r.TaskCenterMeta.AfterFind(tx); err != nil {
		return err
	}
	_ = json.Unmarshal([]byte(defaultJSON(r.DetailShadow, "{}")), &r.Detail)
	return nil
}

type jsonShadow struct {
	value    any
	target   *string
	fallback string
}

func marshalJSONShadows(shadows []jsonShadow) error {
	for _, shadow := range shadows {
		data, err := json.Marshal(shadow.value)
		if err != nil {
			return err
		}
		if string(data) == "null" {
			*shadow.target = shadow.fallback
			continue
		}
		*shadow.target = string(data)
	}
	return nil
}

func defaultJSON(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
