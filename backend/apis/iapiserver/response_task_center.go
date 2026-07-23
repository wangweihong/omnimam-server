package iapiserver

import "github.com/wangweihong/omnimam/backend/apis/imachinery"

// TaskErrorResponse 是公开 Task Center API 返回的安全错误摘要，不包含运行时内部错误对象。
type TaskErrorResponse struct {
	// Code 是可稳定识别的任务失败码。
	Code string `json:"code,omitempty"`
	// Message 是可向当前用户展示的失败摘要。
	Message string `json:"message,omitempty"`
	// Detail 是经过清理的补充诊断信息，不得包含凭证或原始 Provider 响应。
	Detail string `json:"detail,omitempty"`
	// Retryable 表示运行时是否将该失败判定为可自动重试。
	Retryable bool `json:"retryable,omitempty"`
	// OccurredAt 是错误发生时间；未知时不返回该字段。
	OccurredAt *imachinery.Time `json:"occurred_at,omitempty"`
}

// AtomicTaskResponse 严格投影 SSOT AtomicTask，不暴露持久化、幂等或运行时修订内部字段。
type AtomicTaskResponse struct {
	AtomicTaskTemplate
	// ID 是 AtomicTask 的稳定资源标识。
	ID string `json:"id"`
	// Status 是 Task Center 投影的当前任务状态。
	Status string `json:"status"`
	// Progress 是 0 到 1 的任务进度。
	Progress float64 `json:"progress"`
	// ResourceVersion 用于客户端拒绝乱序状态投影。
	ResourceVersion int64 `json:"resource_version"`
	// CurrentAttempt 是当前或最近一次自动尝试序号。
	CurrentAttempt int `json:"current_attempt,omitempty"`
	// Output 仅包含结构化摘要和小型 Artifact/Representation 引用。
	Output map[string]any `json:"output,omitempty"`
	// LastError 是当前任务最近一次用户安全失败摘要。
	LastError *TaskErrorResponse `json:"last_error,omitempty"`
	// RetryOfTaskID 指向手动重试所基于的 AtomicTask。
	RetryOfTaskID string `json:"retry_of_task_id,omitempty"`
	// RetryOfTask 是手动重试直接来源的一跳可读摘要。
	RetryOfTask *AtomicTaskSummary `json:"retry_of_task,omitempty"`
	// RootTaskID 标识手动重试链的根 AtomicTask。
	RootTaskID string `json:"root_task_id,omitempty"`
	// RootTask 是手动重试链根任务的一跳可读摘要。
	RootTask *AtomicTaskSummary `json:"root_task,omitempty"`
	// OwnerType 表示任务由 TaskGroup、DAGTaskGroup 或 TaskSchedule 拥有。
	OwnerType string `json:"owner_type,omitempty"`
	// OwnerID 是组合任务或计划的资源标识。
	OwnerID string `json:"owner_id,omitempty"`
	// Owner 是所属 Group、DAG 或 Schedule 的一跳可读摘要。
	Owner *TaskOwnerSummary `json:"owner,omitempty"`
	// NodeKey 是所属 DAG 声明节点 key；动态子任务共享该值。
	NodeKey string `json:"node_key,omitempty"`
	// RuntimeExecutionID 是受控运行时执行标识，仅用于任务诊断和关联。
	RuntimeExecutionID string `json:"runtime_execution_id,omitempty"`
	// RuntimeTaskID 是受控运行时任务标识，仅用于 Attempt 关联。
	RuntimeTaskID string `json:"runtime_task_id,omitempty"`
	// ProjectID 是任务所属项目边界。
	ProjectID string `json:"project_id"`
	// Namespace 是任务所属命名空间边界。
	Namespace string `json:"namespace"`
	// CreatedBy 是任务访问控制使用的创建主体。
	CreatedBy string `json:"created_by,omitempty"`
	// CreatedAt 是任务创建时间。
	CreatedAt imachinery.Time `json:"created_at"`
	// UpdatedAt 是任务投影最后更新时间。
	UpdatedAt imachinery.Time `json:"updated_at"`
	// StartedAt 是任务首次进入运行状态的时间。
	StartedAt *imachinery.Time `json:"started_at,omitempty"`
	// CompletedAt 是任务进入终态的时间。
	CompletedAt *imachinery.Time `json:"completed_at,omitempty"`
	// ScheduleSource 描述调度创建任务的计划与执行轮次。
	ScheduleSource *ScheduleSourceSummary `json:"schedule_source,omitempty"`
}

// AtomicTaskListAPIResponse 是公开 AtomicTask 列表响应，items 均使用发布契约投影。
type AtomicTaskListAPIResponse struct {
	// Total 是当前过滤条件下可见 AtomicTask 的总数。
	Total int64 `json:"total"`
	// Items 是当前分页内的 AtomicTask 契约投影。
	Items []*AtomicTaskResponse `json:"items"`
}

// TaskAttemptLog 是公开 Task Center API 返回的单条已脱敏执行日志。
type TaskAttemptLog struct {
	// Sequence 是当前 runtime task 日志历史内稳定升序的序号。
	Sequence int `json:"sequence"`
	// Source 区分框架生命周期与受控业务 Worker 日志。
	Source string `json:"source"`
	// Level 是 INFO、WARN 或 ERROR。
	Level string `json:"level"`
	// Message 是双重脱敏且不超过 4096 字节的单行消息。
	Message string `json:"message"`
	// OccurredAt 使用运行时持久化的日志创建时间。
	OccurredAt imachinery.Time `json:"occurred_at"`
}

// TaskExecutorSummary 是仅向 task.operation.admin 返回的非敏感执行器快照。
type TaskExecutorSummary struct {
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
}

// DAGTriggerSummary 固化 DAG 创建时的触发来源，不回查可变或已删除资源。
type DAGTriggerSummary struct {
	Type        string          `json:"type"`
	SourceID    *string         `json:"source_id"`
	SourceName  *string         `json:"source_name"`
	TriggeredAt imachinery.Time `json:"triggered_at"`
}

// DAGNodeExecutionSummary 将声明节点与实际 AtomicTask/Attempt 确定性聚合。
type DAGNodeExecutionSummary struct {
	NodeKey             string             `json:"node_key"`
	Dynamic             bool               `json:"dynamic"`
	Status              string             `json:"status"`
	Progress            float64            `json:"progress"`
	TaskSummary         TaskSummary        `json:"task_summary"`
	PrimaryAtomicTaskID *string            `json:"primary_atomic_task_id"`
	PrimaryAtomicTask   *AtomicTaskSummary `json:"primary_atomic_task"`
	AttemptCount        int                `json:"attempt_count"`
	RetryCount          int                `json:"retry_count"`
	StartedAt           *imachinery.Time   `json:"started_at"`
	CompletedAt         *imachinery.Time   `json:"completed_at"`
	DurationMS          int64              `json:"duration_ms"`
	LatestError         *TaskError         `json:"latest_error"`
	ArtifactCount       int                `json:"artifact_count"`
	RepresentationCount int                `json:"representation_count"`
}

// DAGTaskGroupDetail 是 DAG 运行工作台的受控详情投影。
type DAGTaskGroupDetail struct {
	*DAGTaskGroup
	StartedAt      *imachinery.Time           `json:"started_at"`
	CompletedAt    *imachinery.Time           `json:"completed_at"`
	TriggerSummary DAGTriggerSummary          `json:"trigger_summary"`
	ExecutionNodes []*DAGNodeExecutionSummary `json:"execution_nodes"`
}

type DAGExecutionEvent struct {
	ID            string          `json:"id"`
	EventType     string          `json:"event_type"`
	NodeKey       string          `json:"node_key"`
	AtomicTaskID  *string         `json:"atomic_task_id"`
	TaskAttemptID *string         `json:"task_attempt_id"`
	AttemptNo     *int            `json:"attempt_no"`
	Status        *string         `json:"status"`
	Progress      *float64        `json:"progress"`
	Error         *TaskError      `json:"error"`
	OutputCount   *int            `json:"output_count"`
	Message       *string         `json:"message"`
	OccurredAt    imachinery.Time `json:"occurred_at"`
}

type DAGExecutionEventListResponse struct {
	Total int64                `json:"total"`
	Items []*DAGExecutionEvent `json:"items"`
}

type DAGTimelineSegment struct {
	Phase       string           `json:"phase"`
	AttemptNo   *int             `json:"attempt_no"`
	StartedAt   imachinery.Time  `json:"started_at"`
	CompletedAt *imachinery.Time `json:"completed_at"`
	Complete    bool             `json:"complete"`
}

type DAGTimelineRow struct {
	NodeKey            string                `json:"node_key"`
	AtomicTaskID       string                `json:"atomic_task_id"`
	AtomicTaskName     string                `json:"atomic_task_name"`
	AtomicTaskNameI18n map[string]string     `json:"atomic_task_name_i18n,omitempty"`
	Status             string                `json:"status"`
	Complete           bool                  `json:"complete"`
	Segments           []*DAGTimelineSegment `json:"segments"`
}

type DAGTimelineListResponse struct {
	Total int64             `json:"total"`
	Items []*DAGTimelineRow `json:"items"`
}
