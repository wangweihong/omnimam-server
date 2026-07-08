package iapiserver

import (
	"strings"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

type (
	TaskDefinitionListRequest struct {
		imachinery.BasicQueryParam
		// DefinitionType 按任务定义类型过滤列表。
		DefinitionType string `json:"definition_type" form:"definition_type" binding:"omitempty,oneof=ATOMIC TASK_GROUP DAG_FLOW"`
		// ProjectID 按项目过滤任务定义，空值由服务端默认范围处理。
		ProjectID string `json:"project_id"      form:"project_id"`
		// Namespace 按命名空间过滤任务定义，空值由服务端默认范围处理。
		Namespace string `json:"namespace"       form:"namespace"`
	}

	TaskDefinitionListResponse struct {
		// Total 返回当前查询条件下的任务定义总数。
		Total int64 `json:"total"`
		// Items 返回当前页任务定义，不展开运行实例。
		Items []*TaskDefinition `json:"items"`
	}

	AtomicTaskCreateRequest struct {
		// Name 是 AtomicTask 名称，创建时必填。
		Name string `json:"name"                  binding:"required"`
		// Description 保存任务定义说明，供列表和审计展示。
		Description string `json:"description"`
		// FunctionRef 指向执行器注册表中的函数入口。
		FunctionRef string `json:"function_ref"          binding:"required"`
		// AppID 预留给 AppEngine 关联应用，本轮不触发 AppEngine 执行。
		AppID string `json:"app_id"`
		// EngineRef 预留给具体执行引擎引用。
		EngineRef string `json:"engine_ref"`
		// DefaultArguments 保存创建 TaskRun 时可继承的默认输入。
		DefaultArguments map[string]any `json:"default_arguments"`
		// TimeoutPolicy 定义尝试级和整体超时约束。
		TimeoutPolicy TimeoutPolicy `json:"timeout_policy"`
		// RetryPolicy 定义失败重试策略，无限重试需有退出保护。
		RetryPolicy RetryPolicy `json:"retry_policy"`
		// CancelPolicy 保存取消策略配置。
		CancelPolicy map[string]any `json:"cancel_policy"`
		// RequiredCapabilities 声明可领取该任务的 worker 能力。
		RequiredCapabilities string `json:"required_capabilities"`
		// Tags 保存轻量标签文本。
		Tags string `json:"tags"`
		// ProjectID 指定任务定义所属项目，空值使用默认项目。
		ProjectID string `json:"project_id"`
		// Namespace 指定任务定义命名空间，空值使用默认命名空间。
		Namespace string `json:"namespace"`
		// CreatedBy 指定创建者，空值由服务层填充为系统默认。
		CreatedBy string `json:"created_by"`
	}

	TaskGroupCreateRequest struct {
		// Name 是任务组名称，创建时必填。
		Name string `json:"name"       binding:"required"`
		// Description 保存任务组说明。
		Description string `json:"description"`
		// GroupType 指定子任务串行或并行执行。
		GroupType string `json:"group_type" binding:"required,oneof=SERIAL PARALLEL"`
		// Children 保存子任务引用，创建时必填。
		Children []TaskDefinitionChild `json:"children"   binding:"required"`
		// StrategyConfig 保存任务组执行策略。
		StrategyConfig StrategyConfig `json:"strategy_config"`
		// TimeoutPolicy 定义任务组超时约束。
		TimeoutPolicy TimeoutPolicy `json:"timeout_policy"`
		// RetryPolicy 定义任务组失败重试策略。
		RetryPolicy RetryPolicy `json:"retry_policy"`
		// Tags 保存轻量标签文本。
		Tags string `json:"tags"`
		// ProjectID 指定任务组所属项目。
		ProjectID string `json:"project_id"`
		// Namespace 指定任务组命名空间。
		Namespace string `json:"namespace"`
		// CreatedBy 指定创建者，空值由服务层填充。
		CreatedBy string `json:"created_by"`
	}

	DAGFlowTaskCreateRequest struct {
		// Name 是 DAGFlowTask 名称，创建时必填。
		Name string `json:"name"  binding:"required"`
		// Description 保存 DAGFlowTask 说明。
		Description string `json:"description"`
		// Nodes 保存 DAG 节点，创建时必填并会做环检测。
		Nodes []DAGNode `json:"nodes" binding:"required"`
		// Edges 保存 DAG 边，用于表达节点依赖关系。
		Edges []DAGEdge `json:"edges"`
		// InputMapping 描述运行输入如何映射到 DAG 节点。
		InputMapping map[string]any `json:"input_mapping"`
		// OutputMapping 描述 DAG 节点输出如何汇总。
		OutputMapping map[string]any `json:"output_mapping"`
		// StrategyConfig 保存 DAG 执行策略。
		StrategyConfig StrategyConfig `json:"strategy_config"`
		// TimeoutPolicy 定义 DAG 超时约束。
		TimeoutPolicy TimeoutPolicy `json:"timeout_policy"`
		// RetryPolicy 定义 DAG 失败重试策略。
		RetryPolicy RetryPolicy `json:"retry_policy"`
		// Tags 保存轻量标签文本。
		Tags string `json:"tags"`
		// ProjectID 指定 DAG 所属项目。
		ProjectID string `json:"project_id"`
		// Namespace 指定 DAG 命名空间。
		Namespace string `json:"namespace"`
		// CreatedBy 指定创建者，空值由服务层填充。
		CreatedBy string `json:"created_by"`
	}

	TaskRunListRequest struct {
		imachinery.BasicQueryParam
		// Status 按 TaskRun 状态机状态过滤列表。
		Status string `json:"status"          form:"status"          binding:"omitempty,oneof=PENDING READY CLAIMED RUNNING RETRYING CANCEL_REQUESTED PAUSED SUCCESS FAILED CANCELED TIMEOUT LOST"`
		// DefinitionType 按任务定义类型过滤运行实例。
		DefinitionType string `json:"definition_type" form:"definition_type" binding:"omitempty,oneof=ATOMIC TASK_GROUP DAG_FLOW"`
		// RootRunID 按根运行实例过滤编排链路。
		RootRunID string `json:"root_run_id"     form:"root_run_id"`
		// ProjectID 按项目过滤运行实例。
		ProjectID string `json:"project_id"      form:"project_id"`
	}

	TaskRunCreateRequest struct {
		// DefinitionType 指定要运行的任务定义类型。
		DefinitionType string `json:"definition_type" binding:"required,oneof=ATOMIC TASK_GROUP DAG_FLOW"`
		// DefinitionID 指定要运行的任务定义 ID。
		DefinitionID string `json:"definition_id"   binding:"required"`
		// ParentRunID 指定父运行实例，通常由编排展开逻辑填充。
		ParentRunID string `json:"parent_run_id"`
		// RootRunID 指定根运行实例，空值时服务层会使用自身。
		RootRunID string `json:"root_run_id"`
		// ScheduleAt 指定最早可调度时间，空值表示立即进入队列。
		ScheduleAt imachinery.Time `json:"schedule_at"`
		// Input 保存本次运行输入。
		Input map[string]any `json:"input"`
		// TimeoutPolicy 覆盖定义上的超时策略。
		TimeoutPolicy TimeoutPolicy `json:"timeout_policy"`
		// RetryPolicy 覆盖定义上的重试策略。
		RetryPolicy RetryPolicy `json:"retry_policy"`
		// ProjectID 指定运行所属项目。
		ProjectID string `json:"project_id"`
		// Namespace 指定运行命名空间。
		Namespace string `json:"namespace"`
		// Tags 保存本次运行标签。
		Tags string `json:"tags"`
		// CreatedBy 指定创建者，空值由服务层填充。
		CreatedBy string `json:"created_by"`
	}

	TaskRunListResponse struct {
		// Total 返回当前查询条件下的 TaskRun 总数。
		Total int64 `json:"total"`
		// Items 返回当前页 TaskRun。
		Items []*TaskRun `json:"items"`
	}

	TaskAttemptListRequest struct {
		imachinery.BasicQueryParam
		// RunID 按 TaskRun 过滤 attempts。
		RunID string `json:"run_id" form:"run_id"`
	}

	TaskAttemptListResponse struct {
		// Total 返回当前查询条件下的 attempt 总数。
		Total int64 `json:"total"`
		// Items 返回当前页 attempt 历史。
		Items []*TaskAttempt `json:"items"`
	}

	CancelTaskRunRequest struct {
		// RunID 由路径参数写入，指定要取消的运行实例。
		RunID string `json:"-"      form:"-"`
		// Reason 记录取消原因，用于事件和审计。
		Reason string `json:"reason"`
	}

	RetryTaskRunRequest struct {
		// RunID 由路径参数写入，指定要重试的运行实例。
		RunID string `json:"-"          form:"-"`
		// Reason 记录人工重试原因。
		Reason string `json:"reason"`
		// RetryMode 预留重试模式，当前按服务层默认重试逻辑处理。
		RetryMode string `json:"retry_mode"`
	}

	WorkerRegisterRequest struct {
		// WorkerType 声明 worker 类型，便于调度和运维识别。
		WorkerType string `json:"worker_type"     binding:"required"`
		// Capabilities 声明 worker 可执行能力，claim 时用于匹配任务。
		Capabilities string `json:"capabilities"    binding:"required"`
		// Labels 保存 worker 自定义标签，后续可用于调度约束。
		Labels string `json:"labels"`
		// MaxConcurrency 限制 worker 最大并发执行数量。
		MaxConcurrency int `json:"max_concurrency" binding:"required,min=1"`
	}

	WorkerHeartbeatRequest struct {
		// WorkerID 由路径参数写入，指定上报心跳的 worker。
		WorkerID string `json:"-"             form:"-"`
		// Status 上报 worker 当前生命周期状态。
		Status string `json:"status"        binding:"omitempty,oneof=ONLINE BUSY DRAINING OFFLINE LOST DISABLED"`
		// RunningCount 上报 worker 当前运行任务数。
		RunningCount int `json:"running_count"`
		// ObservedAt 是 worker 侧观察时间，空值时服务端使用当前时间。
		ObservedAt imachinery.Time `json:"observed_at"`
	}

	ClaimTaskRunRequest struct {
		// WorkerID 由路径参数写入，指定领取任务的 worker。
		WorkerID string `json:"-"            form:"-"`
		// Capabilities 上报本次领取可用能力，用于匹配 ready run。
		Capabilities string `json:"capabilities" binding:"required"`
		// Labels 上报本次领取的 worker 标签。
		Labels string `json:"labels"`
		// MaxCount 限制本次最多领取的任务数量。
		MaxCount int `json:"max_count"    binding:"required,min=1"`
	}

	ClaimTaskRunResponse struct {
		// TaskRun 返回成功领取的运行实例。
		TaskRun *TaskRun `json:"task_run,omitempty"`
		// Attempt 返回本次领取创建的执行尝试。
		Attempt *TaskAttempt `json:"attempt,omitempty"`
		// Lease 返回 worker 后续上报必须携带的执行租约。
		Lease *ExecutionLease `json:"lease,omitempty"`
	}

	ProgressUpdateRequest struct {
		// RunID 由路径参数写入，指定进度所属 TaskRun。
		RunID string `json:"-"           form:"-"`
		// WorkerID 校验上报者必须是持有租约的 worker。
		WorkerID string `json:"worker_id"   binding:"required"`
		// AttemptID 校验进度所属尝试。
		AttemptID string `json:"attempt_id"  binding:"required"`
		// LeaseID 校验进度上报所持租约。
		LeaseID string `json:"lease_id"    binding:"required"`
		// Progress 上报 0 到 1 的运行进度。
		Progress float64 `json:"progress"    binding:"min=0,max=1"`
		// OutputSnapshot 保存阶段性输出快照。
		OutputSnapshot map[string]any `json:"output_snapshot"`
		// ExternalJobID 保存外部执行器任务 ID。
		ExternalJobID string `json:"external_job_id"`
	}

	TaskRunCompleteRequest struct {
		// RunID 由路径参数写入，指定完成的 TaskRun。
		RunID string `json:"-"          form:"-"`
		// WorkerID 校验完成上报者必须是持有租约的 worker。
		WorkerID string `json:"worker_id"  binding:"required"`
		// AttemptID 校验完成上报所属尝试。
		AttemptID string `json:"attempt_id" binding:"required"`
		// LeaseID 校验完成上报所持租约。
		LeaseID string `json:"lease_id"   binding:"required"`
		// Output 保存最终输出，通常是引用型结果而不是大对象内容。
		Output map[string]any `json:"output"     binding:"required"`
		// ExternalJobID 保存外部执行器任务 ID。
		ExternalJobID string `json:"external_job_id"`
	}

	TaskRunFailRequest struct {
		// RunID 由路径参数写入，指定失败的 TaskRun。
		RunID string `json:"-"          form:"-"`
		// WorkerID 校验失败上报者必须是持有租约的 worker。
		WorkerID string `json:"worker_id"  binding:"required"`
		// AttemptID 校验失败上报所属尝试。
		AttemptID string `json:"attempt_id" binding:"required"`
		// LeaseID 校验失败上报所持租约。
		LeaseID string `json:"lease_id"   binding:"required"`
		// Error 保存结构化失败信息，用于 TaskRun.last_error 和事件。
		Error TaskError `json:"error"      binding:"required"`
		// LogsRef 保存外部日志引用。
		LogsRef string `json:"logs_ref"`
		// ExternalJobID 保存外部执行器任务 ID。
		ExternalJobID string `json:"external_job_id"`
	}

	LeaseRenewRequest struct {
		// LeaseID 由路径参数写入，指定要续约的租约。
		LeaseID string `json:"-"          form:"-"`
		// WorkerID 校验续约者必须是租约持有者。
		WorkerID string `json:"worker_id"  binding:"required"`
		// AttemptID 校验租约所属尝试。
		AttemptID string `json:"attempt_id" binding:"required"`
		// RunID 校验租约所属运行实例。
		RunID string `json:"run_id"     binding:"required"`
	}

	TaskCenterHealth struct {
		// SchedulerStatus 返回调度器健康状态。
		SchedulerStatus string `json:"scheduler_status"`
		// WorkerOnlineTotal 返回在线 worker 数量。
		WorkerOnlineTotal int64 `json:"worker_online_total"`
		// WorkerLostTotal 返回丢失 worker 数量。
		WorkerLostTotal int64 `json:"worker_lost_total"`
		// QueueReadyTotal 返回 ready 队列中的运行实例数量。
		QueueReadyTotal int64 `json:"queue_ready_total"`
		// TaskRunningTotal 返回运行中任务数量。
		TaskRunningTotal int64 `json:"task_running_total"`
		// TaskRetryingTotal 返回等待重试任务数量。
		TaskRetryingTotal int64 `json:"task_retrying_total"`
		// LeaseExpiredTotal 返回已过期租约数量。
		LeaseExpiredTotal int64 `json:"lease_expired_total"`
		// WatchdogStatus 返回 watchdog 健康状态。
		WatchdogStatus string `json:"watchdog_status"`
	}

	SuccessResponse struct {
		// Success 表示无额外业务载荷的操作已完成。
		Success bool `json:"success"`
	}
)

func (r *TaskDefinitionListRequest) PostBind() error {
	r.DefinitionType = strings.ToUpper(strings.TrimSpace(r.DefinitionType))
	return nil
}

func (r *TaskRunListRequest) PostBind() error {
	r.Status = strings.ToUpper(strings.TrimSpace(r.Status))
	r.DefinitionType = strings.ToUpper(strings.TrimSpace(r.DefinitionType))
	return nil
}
