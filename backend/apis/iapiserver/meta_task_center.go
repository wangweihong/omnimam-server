package iapiserver

import (
	"encoding/json"
	"time"

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
	imachinery.ObjectMeta
	// DefinitionType 区分 ATOMIC、TASK_GROUP、DAG_FLOW 三类定义，决定后续校验和运行展开方式。
	DefinitionType string `json:"definition_type"                   gorm:"column:definition_type;type:varchar(32);not null;index"`
	// FunctionRef 标识可执行函数入口，AtomicTask 通过它映射到内部或外部执行器。
	FunctionRef string `json:"function_ref,omitempty"            gorm:"column:function_ref;type:varchar(256);index"`
	// AppID 预留给 AppEngine 关联应用，当前探索实现不直接执行 AppEngine。
	AppID string `json:"app_id,omitempty"                  gorm:"column:app_id;type:varchar(128)"`
	// EngineRef 预留给具体执行引擎引用，当前由 function_ref/能力匹配承担主要调度。
	EngineRef string `json:"engine_ref,omitempty"              gorm:"column:engine_ref;type:varchar(256)"`
	// DefaultArguments 保存创建 TaskRun 时可继承的默认输入参数。
	DefaultArguments map[string]any `json:"default_arguments,omitempty"       gorm:"-"`
	// DefaultArgumentsShadow 是 DefaultArguments 的 JSON 存储字段。
	DefaultArgumentsShadow string `json:"-"                                gorm:"column:default_arguments_json;type:text;not null;default:'{}'"`
	// RequiredCapabilities 声明 Worker 领取该任务必须具备的能力。
	RequiredCapabilities string `json:"required_capabilities,omitempty"   gorm:"column:required_capabilities;type:text"`
	// GroupType 仅 TaskGroup 使用，决定 children 串行或并行执行。
	GroupType string `json:"group_type,omitempty"              gorm:"column:group_type;type:varchar(32)"`
	// Children 保存 TaskGroup 的子任务引用，按 sort_order 参与执行排序。
	Children []TaskDefinitionChild `json:"children,omitempty"                gorm:"-"`
	// ChildrenShadow 是 Children 的 JSON 存储字段。
	ChildrenShadow string `json:"-"                                gorm:"column:children_json;type:text;not null;default:'[]'"`
	// DAGNodes 保存 DAGFlowTask 的节点定义，创建时会校验节点和边。
	DAGNodes []DAGNode `json:"nodes,omitempty"                   gorm:"-"`
	// DAGNodesShadow 是 DAGNodes 的 JSON 存储字段。
	DAGNodesShadow string `json:"-"                                gorm:"column:dag_nodes_json;type:text;not null;default:'[]'"`
	// DAGEdges 保存 DAGFlowTask 的边定义，用于依赖和环检测。
	DAGEdges []DAGEdge `json:"edges,omitempty"                   gorm:"-"`
	// DAGEdgesShadow 是 DAGEdges 的 JSON 存储字段。
	DAGEdgesShadow string `json:"-"                                gorm:"column:dag_edges_json;type:text;not null;default:'[]'"`
	// InputMapping 描述编排任务输入如何映射到子任务。
	InputMapping map[string]any `json:"input_mapping,omitempty"           gorm:"-"`
	// OutputMapping 描述子任务输出如何汇总到编排结果。
	OutputMapping map[string]any `json:"output_mapping,omitempty"          gorm:"-"`
	// StrategyConfig 保存 TaskGroup/DAGFlow 的执行策略配置。
	StrategyConfig StrategyConfig `json:"strategy_config,omitempty"         gorm:"-"`
	// StrategyConfigShadow 是 StrategyConfig 的 JSON 存储字段。
	StrategyConfigShadow string `json:"-"                                gorm:"column:strategy_config_json;type:text;not null;default:'{}'"`
	// TimeoutPolicy 保存单次尝试和整体超时约束。
	TimeoutPolicy TimeoutPolicy `json:"timeout_policy,omitempty"          gorm:"-"`
	// TimeoutPolicyShadow 是 TimeoutPolicy 的 JSON 存储字段。
	TimeoutPolicyShadow string `json:"-"                                gorm:"column:timeout_policy_json;type:text;not null;default:'{}'"`
	// RetryPolicy 保存失败重试策略，无限重试必须搭配退出保护。
	RetryPolicy RetryPolicy `json:"retry_policy,omitempty"            gorm:"-"`
	// RetryPolicyShadow 是 RetryPolicy 的 JSON 存储字段。
	RetryPolicyShadow string `json:"-"                                gorm:"column:retry_policy_json;type:text;not null;default:'{}'"`
	// CancelPolicy 保存取消策略，当前仅按 S2 结构持久化。
	CancelPolicy map[string]any `json:"cancel_policy,omitempty"           gorm:"-"`
	// CancelPolicyShadow 是 CancelPolicy 的 JSON 存储字段。
	CancelPolicyShadow string `json:"-"                                gorm:"column:cancel_policy_json;type:text;not null;default:'{}'"`
	// Tags 保存轻量标签文本，用于列表过滤或运维排查。
	Tags string `json:"tags,omitempty"                    gorm:"column:tags;type:text"`
	// ProjectID 标识任务所属项目，默认由服务层填充为 default。
	ProjectID string `json:"project_id,omitempty"              gorm:"column:project_id;type:varchar(128);not null;index:idx_task_definitions_project_namespace,priority:1"`
	// Namespace 标识任务命名空间，默认由服务层填充为 default。
	Namespace string `json:"namespace,omitempty"               gorm:"column:namespace;type:varchar(128);not null;index:idx_task_definitions_project_namespace,priority:2"`
	// CreatedBy 记录定义创建者，系统生成任务使用 system。
	CreatedBy string `json:"created_by,omitempty"              gorm:"column:created_by;type:varchar(128);not null"`
	// DeletedAt 用于软删除定义，避免破坏历史 TaskRun 引用。
	DeletedAt imachinery.Time `json:"-"                                gorm:"column:deleted_at"`
	// AdditionalProperties 保留 OpenAPI 生成兼容占位，不落库。
	AdditionalProperties map[string]interface{} `json:"-"                                gorm:"-"`
}

type AtomicTask = TaskDefinition
type TaskGroup = TaskDefinition
type DAGFlowTask = TaskDefinition

func (TaskDefinition) TableName() string { return "task_definitions" }

func (d *TaskDefinition) BeforeCreate(tx *gorm.DB) error {
	if err := d.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return d.marshalShadows()
}

func (d *TaskDefinition) AfterCreate(tx *gorm.DB) error { return nil }

func (d *TaskDefinition) BeforeUpdate(tx *gorm.DB) error {
	if err := d.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return d.marshalShadows()
}

func (d *TaskDefinition) AfterUpdate(tx *gorm.DB) error { return nil }

func (d *TaskDefinition) AfterFind(tx *gorm.DB) error {
	if err := d.ObjectMeta.AfterFind(tx); err != nil {
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
	// Code 保存业务错误码或执行器错误标识，便于客户端按 code 判断。
	Code string `json:"code,omitempty"`
	// Message 保存面向调用方的失败说明。
	Message string `json:"message,omitempty"`
	// FailureType 按 task-center S2 分类失败原因，用于重试和监控聚合。
	FailureType string `json:"failure_type,omitempty"`
	// Detail 保存执行器或系统侧补充诊断信息。
	Detail string `json:"detail,omitempty"`
	// Retryable 标识当前失败是否允许进入重试策略。
	Retryable bool `json:"retryable,omitempty"`
	// OccurredAt 记录错误发生时间，便于和事件流对齐。
	OccurredAt imachinery.Time `json:"occurred_at,omitempty"`
}

// TaskRun 表示某次任务定义的实际运行实例，状态机来源于 task-center S1/S2。
type TaskRun struct {
	imachinery.ObjectMeta
	// DefinitionType 冗余保存定义类型，避免运行时每次回查定义判断展开策略。
	DefinitionType string `json:"definition_type"          gorm:"column:definition_type;type:varchar(32);not null;index:idx_task_runs_definition,priority:1"`
	// DefinitionID 指向 task_definitions.id，是运行实例的定义来源。
	DefinitionID string `json:"definition_id"            gorm:"column:definition_id;type:varchar(64);not null;index:idx_task_runs_definition,priority:2"`
	// ParentRunID 指向直接父运行实例，编排任务展开子运行时使用。
	ParentRunID string `json:"parent_run_id,omitempty"  gorm:"column:parent_run_id;type:varchar(64);index"`
	// RootRunID 指向根运行实例，便于整条编排链路查询。
	RootRunID string `json:"root_run_id,omitempty"    gorm:"column:root_run_id;type:varchar(64);index"`
	// Status 表示 TaskRun 状态机当前状态，终态才允许软删除。
	Status string `json:"status"                   gorm:"column:status;type:varchar(32);not null;index"`
	// ScheduleAt 表示最早可调度时间，重试和延迟执行会设置该字段。
	ScheduleAt imachinery.Time `json:"schedule_at,omitempty"    gorm:"column:schedule_at;index"`
	// TimeoutAt 表示整体超时边界，watchdog 可据此转为 TIMEOUT。
	TimeoutAt imachinery.Time `json:"timeout_at,omitempty"     gorm:"column:timeout_at"`
	// CurrentAttempt 记录当前尝试序号，用于生成下一次 TaskAttempt。
	CurrentAttempt int `json:"current_attempt"          gorm:"column:current_attempt;not null;default:0"`
	// MaxAttempts 保存最大尝试次数，-1 表示无限重试但必须有退出保护。
	MaxAttempts int `json:"max_attempts"             gorm:"column:max_attempts;not null;default:1"`
	// Progress 保存 0 到 1 的运行进度，由持 lease 的 worker 上报。
	Progress float64 `json:"progress"                 gorm:"column:progress;not null;default:0"`
	// Input 保存运行输入，创建时来自请求和定义默认参数合并。
	Input map[string]any `json:"input,omitempty"          gorm:"-"`
	// InputShadow 是 Input 的 JSON 存储字段。
	InputShadow string `json:"-"                        gorm:"column:input_json;type:text;not null;default:'{}'"`
	// Output 保存运行成功后的输出引用或结构化结果。
	Output map[string]any `json:"output,omitempty"         gorm:"-"`
	// OutputShadow 是 Output 的 JSON 存储字段。
	OutputShadow string `json:"-"                        gorm:"column:output_json;type:text;not null;default:'{}'"`
	// LastError 保存最近一次失败信息，便于列表和详情直接展示。
	LastError TaskError `json:"last_error,omitempty"     gorm:"-"`
	// LastErrorShadow 是 LastError 的 JSON 存储字段。
	LastErrorShadow string `json:"-"                        gorm:"column:last_error_json;type:text;not null;default:'{}'"`
	// StartedAt 记录首次进入运行状态的时间。
	StartedAt imachinery.Time `json:"started_at,omitempty"     gorm:"column:started_at"`
	// CompletedAt 记录进入成功或失败等终态的时间。
	CompletedAt imachinery.Time `json:"completed_at,omitempty"   gorm:"column:completed_at"`
	// CanceledAt 记录取消请求被确认或最终取消的时间。
	CanceledAt imachinery.Time `json:"canceled_at,omitempty"    gorm:"column:canceled_at"`
	// CreatedBy 记录创建运行实例的用户或系统来源。
	CreatedBy string `json:"created_by,omitempty"     gorm:"column:created_by;type:varchar(128);not null"`
	// ProjectID 标识运行实例所属项目，继承自定义或创建请求。
	ProjectID string `json:"project_id,omitempty"     gorm:"column:project_id;type:varchar(128);not null;index:idx_task_runs_project_namespace,priority:1"`
	// Namespace 标识运行实例命名空间，继承自定义或创建请求。
	Namespace string `json:"namespace,omitempty"      gorm:"column:namespace;type:varchar(128);not null;index:idx_task_runs_project_namespace,priority:2"`
	// Tags 保存轻量标签文本，用于运行列表过滤和排查。
	Tags string `json:"tags,omitempty"           gorm:"column:tags;type:text"`
	// DeletedAt 用于终态运行实例软删除，保留审计数据。
	DeletedAt imachinery.Time `json:"-"                       gorm:"column:deleted_at"`
}

func (TaskRun) TableName() string { return "task_runs" }

func (r *TaskRun) BeforeCreate(tx *gorm.DB) error {
	if err := r.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return r.marshalShadows()
}

func (r *TaskRun) AfterCreate(tx *gorm.DB) error { return nil }

func (r *TaskRun) BeforeUpdate(tx *gorm.DB) error {
	if err := r.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return r.marshalShadows()
}

func (r *TaskRun) AfterUpdate(tx *gorm.DB) error { return nil }

func (r *TaskRun) AfterFind(tx *gorm.DB) error {
	if err := r.ObjectMeta.AfterFind(tx); err != nil {
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
	imachinery.ObjectMeta
	// RunID 指向所属 TaskRun，同一个 run 内 AttemptNo 唯一。
	RunID string `json:"run_id"                  gorm:"column:run_id;type:varchar(64);not null;uniqueIndex:idx_task_attempts_run_attempt_no,priority:1;index"`
	// AttemptNo 表示第几次尝试，从 1 开始随重试递增。
	AttemptNo int `json:"attempt_no"              gorm:"column:attempt_no;not null;uniqueIndex:idx_task_attempts_run_attempt_no,priority:2"`
	// WorkerID 记录领取本次尝试的 worker，用于 worker protocol 校验。
	WorkerID string `json:"worker_id,omitempty"     gorm:"column:worker_id;type:varchar(64);index"`
	// LeaseID 记录本次尝试当前关联的执行租约。
	LeaseID string `json:"lease_id,omitempty"      gorm:"column:lease_id;type:varchar(64);index"`
	// Status 表示尝试状态，尝试终态会推动 TaskRun 后续重试或终态。
	Status string `json:"status"                  gorm:"column:status;type:varchar(32);not null;index"`
	// InputSnapshot 固化本次尝试看到的输入，避免重试间输入漂移。
	InputSnapshot map[string]any `json:"input_snapshot,omitempty"  gorm:"-"`
	// InputSnapshotShadow 是 InputSnapshot 的 JSON 存储字段。
	InputSnapshotShadow string `json:"-"                       gorm:"column:input_snapshot_json;type:text;not null;default:'{}'"`
	// OutputSnapshot 固化本次尝试上报的阶段性或最终输出。
	OutputSnapshot map[string]any `json:"output_snapshot,omitempty" gorm:"-"`
	// OutputSnapshotShadow 是 OutputSnapshot 的 JSON 存储字段。
	OutputSnapshotShadow string `json:"-"                       gorm:"column:output_snapshot_json;type:text;not null;default:'{}'"`
	// Error 保存本次尝试失败详情。
	Error TaskError `json:"error,omitempty"         gorm:"-"`
	// ErrorShadow 是 Error 的 JSON 存储字段。
	ErrorShadow string `json:"-"                       gorm:"column:error_json;type:text;not null;default:'{}'"`
	// StartedAt 记录尝试创建或开始执行的时间。
	StartedAt imachinery.Time `json:"started_at"              gorm:"column:started_at;not null"`
	// HeartbeatAt 记录 worker 最近一次心跳或进度上报时间。
	HeartbeatAt imachinery.Time `json:"heartbeat_at,omitempty"  gorm:"column:heartbeat_at"`
	// ProgressAt 记录最近一次进度更新发生时间。
	ProgressAt imachinery.Time `json:"progress_at,omitempty"   gorm:"column:progress_at"`
	// CompletedAt 记录尝试进入终态时间。
	CompletedAt imachinery.Time `json:"completed_at,omitempty"  gorm:"column:completed_at"`
	// DurationMS 保存尝试耗时毫秒数，用于运行统计。
	DurationMS int64 `json:"duration_ms"             gorm:"column:duration_ms;not null;default:0"`
	// Retryable 标识该尝试失败是否可按 RetryPolicy 重试。
	Retryable bool `json:"retryable"               gorm:"column:retryable;not null;default:false"`
	// FailureType 记录失败分类，驱动重试策略和监控聚合。
	FailureType string `json:"failure_type,omitempty"  gorm:"column:failure_type;type:varchar(64)"`
	// LogsRef 保存外部日志引用，避免直接在任务中心存储大日志。
	LogsRef string `json:"logs_ref,omitempty"      gorm:"column:logs_ref;type:text"`
	// ExternalJobID 保存外部执行器任务 ID，用于排查和后续对账。
	ExternalJobID string `json:"external_job_id,omitempty" gorm:"column:external_job_id;type:varchar(128)"`
}

func (TaskAttempt) TableName() string { return "task_attempts" }

func (a *TaskAttempt) BeforeCreate(tx *gorm.DB) error {
	if err := a.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return a.marshalShadows()
}

func (a *TaskAttempt) AfterCreate(tx *gorm.DB) error { return nil }

func (a *TaskAttempt) BeforeUpdate(tx *gorm.DB) error {
	if err := a.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return a.marshalShadows()
}

func (a *TaskAttempt) AfterUpdate(tx *gorm.DB) error { return nil }

func (a *TaskAttempt) AfterFind(tx *gorm.DB) error {
	if err := a.ObjectMeta.AfterFind(tx); err != nil {
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
	imachinery.ObjectMeta
	// WorkerType 表示 worker 类型，通常对应执行器族或部署形态。
	WorkerType string `json:"worker_type"     gorm:"column:worker_type;type:varchar(128);not null;index"`
	// Status 表示 worker 生命周期状态，watchdog 会根据心跳转为 LOST。
	Status string `json:"status"          gorm:"column:status;type:varchar(32);not null;index"`
	// Capabilities 声明 worker 可执行的能力集合，用于领取任务匹配。
	Capabilities string `json:"capabilities"    gorm:"column:capabilities;type:text;not null"`
	// Labels 保存 worker 自定义标签，后续可用于调度约束。
	Labels string `json:"labels,omitempty" gorm:"column:labels;type:text"`
	// MaxConcurrency 限制 worker 同时运行的任务数量。
	MaxConcurrency int `json:"max_concurrency" gorm:"column:max_concurrency;not null;default:1"`
	// RunningCount 保存当前运行数，由 claim/complete/fail 流程维护。
	RunningCount int `json:"running_count"   gorm:"column:running_count;not null;default:0"`
	// HeartbeatAt 记录最近一次 worker heartbeat 时间。
	HeartbeatAt imachinery.Time `json:"heartbeat_at"    gorm:"column:heartbeat_at;not null;index"`
	// RegisteredAt 记录 worker 首次注册时间。
	RegisteredAt imachinery.Time `json:"registered_at"   gorm:"column:registered_at;not null"`
	// LastSeenAt 记录最近一次被 API 观察到的时间。
	LastSeenAt imachinery.Time `json:"last_seen_at"    gorm:"column:last_seen_at;not null"`
}

func (Worker) TableName() string { return "task_workers" }

func (w *Worker) BeforeCreate(tx *gorm.DB) error {
	if err := w.ObjectMeta.BeforeCreate(tx); err != nil {
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

func (w *Worker) BeforeUpdate(tx *gorm.DB) error { return w.ObjectMeta.BeforeUpdate(tx) }
func (w *Worker) AfterUpdate(tx *gorm.DB) error  { return nil }
func (w *Worker) AfterFind(tx *gorm.DB) error    { return w.ObjectMeta.AfterFind(tx) }

// ExecutionLease 限定同一 TaskRun 同一时间只能由一个 Worker 持有执行权。
type ExecutionLease struct {
	imachinery.ObjectMeta
	// RunID 指向被租约保护的 TaskRun，同一 run 同时只能有一个有效 lease。
	RunID string `json:"run_id"      gorm:"column:run_id;type:varchar(64);not null;index"`
	// AttemptID 指向 lease 对应的尝试，用于 progress/complete/fail 校验。
	AttemptID string `json:"attempt_id"  gorm:"column:attempt_id;type:varchar(64);not null;index"`
	// WorkerID 指向持有租约的 worker。
	WorkerID string `json:"worker_id"   gorm:"column:worker_id;type:varchar(64);not null;index"`
	// AcquiredAt 记录 worker 获取租约时间。
	AcquiredAt imachinery.Time `json:"acquired_at" gorm:"column:acquired_at;not null"`
	// ExpireAt 记录租约过期时间，renew 会延长该时间。
	ExpireAt imachinery.Time `json:"expire_at"   gorm:"column:expire_at;not null;index"`
	// RenewedAt 记录最近一次续约时间。
	RenewedAt imachinery.Time `json:"renewed_at,omitempty" gorm:"column:renewed_at"`
	// Status 表示租约状态，ACTIVE/RENEWED 才允许上报运行结果。
	Status string `json:"status"      gorm:"column:status;type:varchar(32);not null;index"`
}

func (ExecutionLease) TableName() string { return "task_execution_leases" }

func (l *ExecutionLease) BeforeCreate(tx *gorm.DB) error {
	if err := l.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	now := imachinery.NewTime(time.Now())
	if l.AcquiredAt.IsZero() {
		l.AcquiredAt = now
	}
	return nil
}

func (l *ExecutionLease) AfterCreate(tx *gorm.DB) error  { return nil }
func (l *ExecutionLease) BeforeUpdate(tx *gorm.DB) error { return l.ObjectMeta.BeforeUpdate(tx) }
func (l *ExecutionLease) AfterUpdate(tx *gorm.DB) error  { return nil }
func (l *ExecutionLease) AfterFind(tx *gorm.DB) error    { return l.ObjectMeta.AfterFind(tx) }

// TaskRunEvent 保存 task-center S2 events.yaml 中定义的内部事件事实。
type TaskRunEvent struct {
	imachinery.ObjectMeta
	// RunID 指向事件所属 TaskRun。
	RunID string `json:"run_id"                   gorm:"column:run_id;type:varchar(64);not null;index"`
	// AttemptID 记录事件关联的尝试，无尝试事件可为空。
	AttemptID string `json:"attempt_id,omitempty"     gorm:"column:attempt_id;type:varchar(64)"`
	// WorkerID 记录事件关联的 worker，用户操作事件可为空。
	WorkerID string `json:"worker_id,omitempty"      gorm:"column:worker_id;type:varchar(64)"`
	// EventType 来自 task-center S2 events.yaml，用于事件订阅和审计。
	EventType string `json:"event_type"               gorm:"column:event_type;type:varchar(128);not null;index"`
	// FromStatus 记录状态变更前状态，非状态事件可为空。
	FromStatus string `json:"from_status,omitempty"    gorm:"column:from_status;type:varchar(32)"`
	// ToStatus 记录状态变更后状态，非状态事件可为空。
	ToStatus string `json:"to_status,omitempty"      gorm:"column:to_status;type:varchar(32)"`
	// Payload 保存事件补充数据，事实写入成功后事件失败不会回滚主事实。
	Payload map[string]any `json:"payload,omitempty"        gorm:"-"`
	// PayloadShadow 是 Payload 的 JSON 存储字段。
	PayloadShadow string `json:"-"                        gorm:"column:payload_json;type:text;not null;default:'{}'"`
	// OccurredAt 记录事件发生时间，默认由服务端填充。
	OccurredAt imachinery.Time `json:"occurred_at"              gorm:"column:occurred_at;not null;index"`
}

func (TaskRunEvent) TableName() string { return "task_run_events" }

func (e *TaskRunEvent) BeforeCreate(tx *gorm.DB) error {
	if err := e.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = imachinery.NewTime(time.Now())
	}
	return marshalJSONShadows([]jsonShadow{{value: &e.Payload, target: &e.PayloadShadow, fallback: "{}"}})
}

func (e *TaskRunEvent) AfterCreate(tx *gorm.DB) error { return nil }

func (e *TaskRunEvent) BeforeUpdate(tx *gorm.DB) error {
	if err := e.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return marshalJSONShadows([]jsonShadow{{value: &e.Payload, target: &e.PayloadShadow, fallback: "{}"}})
}

func (e *TaskRunEvent) AfterUpdate(tx *gorm.DB) error { return nil }

func (e *TaskRunEvent) AfterFind(tx *gorm.DB) error {
	if err := e.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	_ = json.Unmarshal([]byte(defaultJSON(e.PayloadShadow, "{}")), &e.Payload)
	return nil
}

// WatchdogRecord 保存 watchdog 对 Worker、Lease、Attempt 和 Run 的异常扫描结果。
type WatchdogRecord struct {
	imachinery.ObjectMeta
	// ScanType 表示 watchdog 本次扫描的规则类型。
	ScanType string `json:"scan_type"     gorm:"column:scan_type;type:varchar(64);not null"`
	// TargetType 表示被处理对象类型，如 worker、lease、attempt 或 run。
	TargetType string `json:"target_type"   gorm:"column:target_type;type:varchar(64);not null;index:idx_task_watchdog_records_target,priority:1"`
	// TargetID 表示被处理对象 ID，用于追溯 watchdog 处理结果。
	TargetID string `json:"target_id"     gorm:"column:target_id;type:varchar(64);not null;index:idx_task_watchdog_records_target,priority:2"`
	// ActionTaken 记录 watchdog 实际采取的动作，如标记 LOST 或 EXPIRED。
	ActionTaken string `json:"action_taken"  gorm:"column:action_taken;type:varchar(128);not null"`
	// ResultStatus 记录动作执行后的目标状态或处理结果。
	ResultStatus string `json:"result_status" gorm:"column:result_status;type:varchar(64);not null"`
	// Detail 保存 watchdog 判定依据和补充诊断信息。
	Detail map[string]any `json:"detail,omitempty" gorm:"-"`
	// DetailShadow 是 Detail 的 JSON 存储字段。
	DetailShadow string `json:"-"            gorm:"column:detail_json;type:text;not null;default:'{}'"`
	// OccurredAt 记录 watchdog 处理发生时间。
	OccurredAt imachinery.Time `json:"occurred_at"  gorm:"column:occurred_at;not null;index"`
}

func (WatchdogRecord) TableName() string { return "task_watchdog_records" }

func (r *WatchdogRecord) BeforeCreate(tx *gorm.DB) error {
	if err := r.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	if r.OccurredAt.IsZero() {
		r.OccurredAt = imachinery.NewTime(time.Now())
	}
	return marshalJSONShadows([]jsonShadow{{value: &r.Detail, target: &r.DetailShadow, fallback: "{}"}})
}

func (r *WatchdogRecord) AfterCreate(tx *gorm.DB) error { return nil }

func (r *WatchdogRecord) BeforeUpdate(tx *gorm.DB) error {
	if err := r.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return marshalJSONShadows([]jsonShadow{{value: &r.Detail, target: &r.DetailShadow, fallback: "{}"}})
}

func (r *WatchdogRecord) AfterUpdate(tx *gorm.DB) error { return nil }

func (r *WatchdogRecord) AfterFind(tx *gorm.DB) error {
	if err := r.ObjectMeta.AfterFind(tx); err != nil {
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
