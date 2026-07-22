package iapiserver

import (
	"fmt"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

type AtomicTaskListRequest struct {
	imachinery.BasicQueryParam
	Status     string `form:"status" binding:"omitempty,oneof=PENDING BLOCKED READY RUNNING RETRYING CANCEL_REQUESTED SUCCESS FAILED CANCELED TIMEOUT SKIPPED"`
	RootTaskID string `form:"root_task_id"`
	OwnerID    string `form:"owner_id"`
	// NodeKey 仅用于 DAG 子任务列表，精确匹配声明节点及其动态实际任务。
	NodeKey       string `form:"node_key" binding:"omitempty,max=128"`
	ProjectID     string `form:"-" json:"-"`
	Namespace     string `form:"-" json:"-"`
	CreatedBy     string `form:"-" json:"-"`
	IncludeSystem bool   `form:"-" json:"-"` // 系统管理员列表是否包含系统调度创建的任务。
}

type AtomicTaskCreateRequest struct {
	Key                  string         `json:"key" binding:"required,max=128"`
	Name                 string         `json:"name" binding:"omitempty,max=256"`
	Description          string         `json:"description" binding:"omitempty,max=2048"`
	FunctionRef          string         `json:"function_ref" binding:"required,max=256"`
	Arguments            map[string]any `json:"arguments"`
	RequiredCapabilities string         `json:"required_capabilities" binding:"omitempty,max=2048"`
	RetryPolicy          RetryPolicy    `json:"retry_policy"`
	TimeoutPolicy        TimeoutPolicy  `json:"timeout_policy"`
	ProjectID            string         `json:"project_id" binding:"required,max=128"`
	Namespace            string         `json:"namespace" binding:"required,max=128"`
	IdempotencyScope     string         `json:"idempotency_scope" binding:"omitempty,max=256"`
	IdempotencyKey       string         `json:"idempotency_key" binding:"omitempty,max=256"`
	ApplicationRunID     string         `json:"application_run_id" binding:"omitempty,max=64"`
	CanvasRunID          string         `json:"canvas_run_id" binding:"omitempty,max=64"`
	CanvasNodeRunID      string         `json:"canvas_node_run_id" binding:"omitempty,max=64"`
	CreatedBy            string         `json:"-"` // 内部调度触发时显式传递的计划创建者。
	OwnerType            string         `json:"-"` // 内部调度目标的归属类型，HTTP 客户端不可设置。
	OwnerID              string         `json:"-"` // 内部调度目标的来源计划 ID。
}

type TaskAttemptListRequest struct {
	imachinery.BasicQueryParam
	AtomicTaskID string `form:"-"`
}

// TaskAttemptLogListRequest 查询单个 Attempt 的运行时日志，不接受客户端提供 runtime task ID。
type TaskAttemptLogListRequest struct {
	imachinery.BasicQueryParam
	// AtomicTaskID 来自受权父资源路径，用于先执行任务可见性校验。
	AtomicTaskID string `form:"-" json:"-"`
	// TaskAttemptID 来自子资源路径，用于校验 Attempt 确实属于父 AtomicTask。
	TaskAttemptID string `form:"-" json:"-"`
	// Levels 是 INFO、WARN、ERROR 逗号分隔过滤器。
	Levels string `form:"levels" binding:"omitempty,max=64"`
	// Sources 是 LIFECYCLE、WORKER 逗号分隔过滤器。
	Sources string `form:"sources" binding:"omitempty,max=64"`
	// Cursor 是绑定当前排序方向与日志 sequence 的不透明游标。
	Cursor string `form:"cursor" binding:"omitempty,max=512"`
	// Direction 控制从游标向前或向后读取。
	Direction string `form:"direction" binding:"omitempty,oneof=forward backward"`
	// SortOrder 控制 occurred_at 与原始 sequence 的稳定顺序。
	SortOrder string `form:"sort_order" binding:"omitempty,oneof=asc desc"`
}

func (r *TaskAttemptLogListRequest) SetDefaults() {
	if r.PageSize == 0 {
		r.PageSize = 100
	}
	if r.Direction == "" {
		r.Direction = "forward"
	}
	if r.SortOrder == "" {
		r.SortOrder = "asc"
	}
}

// TaskAttemptLogDownloadRequest 使用与在线日志查询相同的授权、筛选、排序和脱敏管线。
type TaskAttemptLogDownloadRequest struct {
	// AtomicTaskID 来自父资源路径，用于任务可见性授权。
	AtomicTaskID string `form:"-" json:"-"`
	// TaskAttemptID 来自子资源路径，用于归属校验。
	TaskAttemptID string `form:"-" json:"-"`
	// Keyword 对已脱敏单行消息执行大小写不敏感匹配。
	Keyword string `form:"keyword" binding:"omitempty,max=256"`
	// Levels 与在线列表使用相同级别过滤器。
	Levels string `form:"levels" binding:"omitempty,max=64"`
	// Sources 与在线列表使用相同来源过滤器。
	Sources string `form:"sources" binding:"omitempty,max=64"`
	// SortOrder 与在线列表使用相同稳定排序。
	SortOrder string `form:"sort_order" binding:"omitempty,oneof=asc desc"`
}

func (r *TaskAttemptLogDownloadRequest) SetDefaults() {
	if r.SortOrder == "" {
		r.SortOrder = "asc"
	}
}

func (r *TaskAttemptLogListRequest) Validate() error {
	if r.PageNum < 0 || r.PageSize < 1 || r.PageSize > 200 {
		return fmt.Errorf("page_num or page_size is invalid")
	}
	if len(r.Keyword) > 256 {
		return fmt.Errorf("keyword is too long")
	}
	return nil
}

type ActionReasonRequest struct {
	Reason string `json:"reason" binding:"omitempty,max=1024"`
}

type TaskGroupListRequest struct {
	imachinery.BasicQueryParam
	Status        string `form:"status" binding:"omitempty,oneof=PENDING RUNNING CANCEL_REQUESTED SUCCESS FAILED CANCELED TIMEOUT"`
	ProjectID     string `form:"-" json:"-"`
	Namespace     string `form:"-" json:"-"`
	CreatedBy     string `form:"-" json:"-"`
	IncludeSystem bool   `form:"-" json:"-"` // 系统管理员列表是否包含系统调度创建的组合。
}

type TaskGroupCreateRequest struct {
	Name             string               `json:"name" binding:"required,max=256"`
	Description      string               `json:"description" binding:"omitempty,max=2048"`
	Mode             string               `json:"mode" binding:"required,oneof=SERIAL PARALLEL"`
	Tasks            []AtomicTaskTemplate `json:"tasks" binding:"required,min=1,max=1000,dive"`
	Strategy         GroupStrategy        `json:"strategy"`
	ProjectID        string               `json:"project_id" binding:"required,max=128"`
	Namespace        string               `json:"namespace" binding:"required,max=128"`
	IdempotencyScope string               `json:"idempotency_scope" binding:"omitempty,max=256"`
	IdempotencyKey   string               `json:"idempotency_key" binding:"omitempty,max=256"`
	CreatedBy        string               `json:"-"` // 内部调度触发时显式传递的计划创建者。
}

type DAGTaskGroupListRequest struct {
	imachinery.BasicQueryParam
	Status        string `form:"status" binding:"omitempty,oneof=PENDING RUNNING CANCEL_REQUESTED SUCCESS FAILED CANCELED TIMEOUT"`
	ProjectID     string `form:"-" json:"-"`
	Namespace     string `form:"-" json:"-"`
	CreatedBy     string `form:"-" json:"-"`
	IncludeSystem bool   `form:"-" json:"-"` // 系统管理员列表是否包含系统调度创建的 DAG。
}

type DAGTaskGroupCreateRequest struct {
	Name              string          `json:"name" binding:"required,max=256"`
	Description       string          `json:"description" binding:"omitempty,max=2048"`
	Nodes             []DAGNode       `json:"nodes" binding:"required,min=1,max=1000,dive"`
	Edges             []DAGEdge       `json:"edges" binding:"max=5000,dive"`
	Input             map[string]any  `json:"input"`
	OutputMapping     map[string]any  `json:"output_mapping"`
	CanvasVersionID   string          `json:"canvas_version_id" binding:"omitempty,max=64"`
	ProjectID         string          `json:"project_id" binding:"required,max=128"`
	Namespace         string          `json:"namespace" binding:"required,max=128"`
	IdempotencyScope  string          `json:"idempotency_scope" binding:"omitempty,max=256"`
	IdempotencyKey    string          `json:"idempotency_key" binding:"omitempty,max=256"`
	CreatedBy         string          `json:"-"` // 内部调度触发时显式传递的计划创建者。
	TriggerType       string          `json:"-"` // 内部创建链路声明 API/SCHEDULE/CANVAS/DOMAIN_EVENT/RETRY。
	TriggerSourceID   string          `json:"-"` // 内部触发来源 ID 快照，HTTP 客户端不可设置。
	TriggerSourceName string          `json:"-"` // 内部触发来源名称快照，HTTP 客户端不可设置。
	TriggeredAt       imachinery.Time `json:"-"` // 内部触发时刻快照，HTTP 客户端不可设置。
}

// DAGExecutionEventListRequest 过滤规范化运行投影事件，不接受原始运行时字段名。
type DAGExecutionEventListRequest struct {
	imachinery.BasicQueryParam
	// NodeKey 精确筛选声明节点及动态实际任务事件。
	NodeKey string `form:"node_key" binding:"omitempty,max=128"`
	// AtomicTaskID 精确筛选实际任务事件。
	AtomicTaskID string `form:"atomic_task_id" binding:"omitempty,max=128"`
	// TaskAttemptID 精确筛选自动尝试事件。
	TaskAttemptID string `form:"task_attempt_id" binding:"omitempty,max=128"`
	// AttemptNo 按从 1 开始的自动尝试序号筛选。
	AttemptNo int `form:"attempt_no" binding:"omitempty,min=1"`
	// EventTypes 是公开 DAGExecutionEventType 白名单的逗号分隔集合。
	EventTypes string `form:"event_types" binding:"omitempty,max=512"`
	// OccurredAfter 是 RFC3339Nano 事件时间下界。
	OccurredAfter string `form:"occurred_after" binding:"omitempty,max=64"`
	// OccurredBefore 是 RFC3339Nano 事件时间上界。
	OccurredBefore string `form:"occurred_before" binding:"omitempty,max=64"`
}

func (r *DAGExecutionEventListRequest) SetDefaults() {
	if r.SortOrder == "" {
		r.SortOrder = "asc"
	}
}

func (r *DAGExecutionEventListRequest) Validate() error {
	if err := r.BasicQueryParam.Validate(); err != nil {
		return err
	}
	if r.SortOrder != "asc" && r.SortOrder != "desc" {
		return fmt.Errorf("sort_order is invalid")
	}
	return nil
}

// DAGTimelineListRequest 按声明节点精确筛选实际 AtomicTask 时间线。
type DAGTimelineListRequest struct {
	imachinery.BasicQueryParam
	// NodeKey 精确筛选声明节点下的实际 AtomicTask 时间线。
	NodeKey string `form:"node_key" binding:"omitempty,max=128"`
}

type TaskScheduleListRequest struct {
	imachinery.BasicQueryParam
	Status        string `form:"status" binding:"omitempty,oneof=ACTIVE PAUSED COMPLETED DELETED"`
	ExecutionMode string `form:"execution_mode" binding:"omitempty,oneof=MATERIALIZED RECONCILE"` // 按完整作业历史或轻量巡检模式筛选。
	ProjectID     string `form:"-" json:"-"`
	Namespace     string `form:"-" json:"-"`
	CreatedBy     string `form:"-" json:"-"`
	IncludeSystem bool   `form:"-" json:"-"`
}

type TaskScheduleCreateRequest struct {
	Name           string          `json:"name" binding:"required,max=256"`
	Description    string          `json:"description" binding:"omitempty,max=2048"`
	ExecutionMode  string          `json:"execution_mode" binding:"omitempty,oneof=MATERIALIZED"` // 公开创建固定为 MATERIALIZED，省略时使用该默认值。
	TriggerType    string          `json:"trigger_type" binding:"required,oneof=CRON RUN_AT"`
	CronExpression string          `json:"cron_expression" binding:"omitempty,max=256"`
	RunAt          imachinery.Time `json:"run_at"`
	TimeZone       string          `json:"time_zone" binding:"required,max=128"`
	Target         ScheduleTarget  `json:"target" binding:"required"`
	ProjectID      string          `json:"project_id" binding:"required,max=128"`
	Namespace      string          `json:"namespace" binding:"required,max=128"`
}

type TaskScheduleUpdateRequest struct {
	ID             string               `json:"-"`
	Name           *string              `json:"name" binding:"omitempty,max=256"`
	Description    *string              `json:"description" binding:"omitempty,max=2048"`
	CronExpression *string              `json:"cron_expression" binding:"omitempty,max=256"`
	RunAt          *imachinery.Time     `json:"run_at"`
	TimeZone       *string              `json:"time_zone" binding:"omitempty,max=128"`
	Target         *ScheduleTarget      `json:"target"`
	ReconcileSpec  *ReconcileSpecUpdate `json:"reconcile_spec"` // 仅 SYSTEM RECONCILE 允许调整的安全运行参数。
}

// ReconcileSpecUpdate 只允许管理员调整 SYSTEM RECONCILE 的安全参数，不接受 reconcile_ref。
type ReconcileSpecUpdate struct {
	Config                *map[string]any `json:"config"`                                                    // 巡检器声明并校验的受控配置，不接受运行时任务定义。
	MaxParallelism        *int            `json:"max_parallelism" binding:"omitempty,min=1,max=64"`          // 单轮最大实际并发，范围 1..64。
	MaxItemsPerRun        *int            `json:"max_items_per_run" binding:"omitempty,min=1,max=1000"`      // 单轮最多扫描资源数，范围 1..1000。
	PerItemTimeoutSeconds *int            `json:"per_item_timeout_seconds" binding:"omitempty,min=1,max=30"` // 单资源探测超时，范围 1..30 秒。
	OverallTimeoutSeconds *int            `json:"overall_timeout_seconds" binding:"omitempty,min=1,max=300"` // 整轮超时，范围 1..300 秒且不得小于单项超时。
}

type ScheduleExecutionListRequest struct {
	imachinery.BasicQueryParam
	ScheduleID string `form:"-"`
	Status     string `form:"status" binding:"omitempty,oneof=TRIGGERED RUNNING SUCCESS FAILED CANCELED SKIPPED_OVERLAP TRIGGER_FAILED"`
}

type AtomicTaskListResponse struct {
	Total int64         `json:"total"`
	Items []*AtomicTask `json:"items"`
}

type TaskAttemptListResponse struct {
	Total int64          `json:"total"`
	Items []*TaskAttempt `json:"items"`
}

type TaskAttemptLogListResponse struct {
	Total          int64             `json:"total"`
	Items          []*TaskAttemptLog `json:"items"`
	NextCursor     *string           `json:"next_cursor"`
	PreviousCursor *string           `json:"previous_cursor"`
}

type TaskGroupListResponse struct {
	Total int64        `json:"total"`
	Items []*TaskGroup `json:"items"`
}

type DAGTaskGroupListResponse struct {
	Total int64           `json:"total"`
	Items []*DAGTaskGroup `json:"items"`
}

type TaskScheduleListResponse struct {
	Total int64           `json:"total"`
	Items []*TaskSchedule `json:"items"`
}

type ScheduleExecutionListResponse struct {
	Total int64                    `json:"total"`
	Items []*TaskScheduleExecution `json:"items"`
}
