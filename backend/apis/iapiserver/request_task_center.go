package iapiserver

import "github.com/wangweihong/omnimam/backend/apis/imachinery"

type AtomicTaskListRequest struct {
	imachinery.BasicQueryParam
	Status        string `form:"status" binding:"omitempty,oneof=PENDING BLOCKED READY RUNNING RETRYING CANCEL_REQUESTED SUCCESS FAILED CANCELED TIMEOUT SKIPPED"`
	RootTaskID    string `form:"root_task_id"`
	OwnerID       string `form:"owner_id"`
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
	Name             string         `json:"name" binding:"required,max=256"`
	Description      string         `json:"description" binding:"omitempty,max=2048"`
	Nodes            []DAGNode      `json:"nodes" binding:"required,min=1,max=1000,dive"`
	Edges            []DAGEdge      `json:"edges" binding:"max=5000,dive"`
	Input            map[string]any `json:"input"`
	OutputMapping    map[string]any `json:"output_mapping"`
	CanvasVersionID  string         `json:"canvas_version_id" binding:"omitempty,max=64"`
	ProjectID        string         `json:"project_id" binding:"required,max=128"`
	Namespace        string         `json:"namespace" binding:"required,max=128"`
	IdempotencyScope string         `json:"idempotency_scope" binding:"omitempty,max=256"`
	IdempotencyKey   string         `json:"idempotency_key" binding:"omitempty,max=256"`
	CreatedBy        string         `json:"-"` // 内部调度触发时显式传递的计划创建者。
}

type TaskScheduleListRequest struct {
	imachinery.BasicQueryParam
	Status        string `form:"status" binding:"omitempty,oneof=ACTIVE PAUSED COMPLETED DELETED"`
	ProjectID     string `form:"-" json:"-"`
	Namespace     string `form:"-" json:"-"`
	CreatedBy     string `form:"-" json:"-"`
	IncludeSystem bool   `form:"-" json:"-"`
}

type TaskScheduleCreateRequest struct {
	Name           string          `json:"name" binding:"required,max=256"`
	Description    string          `json:"description" binding:"omitempty,max=2048"`
	TriggerType    string          `json:"trigger_type" binding:"required,oneof=CRON RUN_AT"`
	CronExpression string          `json:"cron_expression" binding:"omitempty,max=256"`
	RunAt          imachinery.Time `json:"run_at"`
	TimeZone       string          `json:"time_zone" binding:"required,max=128"`
	Target         ScheduleTarget  `json:"target" binding:"required"`
	ProjectID      string          `json:"project_id" binding:"required,max=128"`
	Namespace      string          `json:"namespace" binding:"required,max=128"`
}

type TaskScheduleUpdateRequest struct {
	ID             string           `json:"-"`
	Name           *string          `json:"name" binding:"omitempty,max=256"`
	Description    *string          `json:"description" binding:"omitempty,max=2048"`
	CronExpression *string          `json:"cron_expression" binding:"omitempty,max=256"`
	RunAt          *imachinery.Time `json:"run_at"`
	TimeZone       *string          `json:"time_zone" binding:"omitempty,max=128"`
	Target         *ScheduleTarget  `json:"target"`
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
