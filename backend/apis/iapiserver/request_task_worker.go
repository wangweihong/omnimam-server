package iapiserver

const (
	// TaskWorkerInfrastructureOperationCreate/Start/Stop 是 Task Worker 调用 Infrastructure 的操作类型。
	TaskWorkerInfrastructureOperationCreate = "create"
	TaskWorkerInfrastructureOperationStart  = "start"
	TaskWorkerInfrastructureOperationStop   = "stop"

	// TaskWorkerRuntimeModeJob/Service 是 Task Worker 合同使用的 Runtime 模式。
	TaskWorkerRuntimeModeJob     = "JOB"
	TaskWorkerRuntimeModeService = "SERVICE"

	// Task Worker Runtime 状态用于校验 Worker 执行结果，不复用 InfraRuntime 状态常量。
	TaskWorkerRuntimeStatusRunning   = "RUNNING"
	TaskWorkerRuntimeStatusSucceeded = "SUCCEEDED"
	TaskWorkerRuntimeStatusStopped   = "STOPPED"
	TaskWorkerRuntimeStatusDeleted   = "DELETED"

	TaskWorkerRuntimeEndpointStatusReady   = "READY"
	TaskWorkerRuntimeOutputStatusCollected = "COLLECTED"

	TaskWorkerEndpointVisibilityInternal       = "INTERNAL"
	TaskWorkerEndpointVisibilityUserAccessible = "USER_ACCESSIBLE"
	TaskWorkerRuntimeHealthStatusHealthy       = "HEALTHY"
	TaskWorkerBuildValidationStatusPassed      = "PASSED"

	TaskWorkerActionSuspend = "SUSPEND"
	TaskWorkerActionStop    = "STOP"
	TaskWorkerActionDelete  = "DELETE"

	TaskWorkerAgentRuntimeOperationStart   = "START"
	TaskWorkerAgentRuntimeOperationRecover = "RECOVER"

	TaskWorkerAgentRuntimeRestartPolicyNever     = "NEVER"
	TaskWorkerAgentRuntimeRestartPolicyOnFailure = "ON_FAILURE"
	TaskWorkerAgentRuntimeRestartPolicyAlways    = "ALWAYS"

	TaskWorkerDeploymentReasonDeploy   = "DEPLOY"
	TaskWorkerDeploymentReasonUpgrade  = "UPGRADE"
	TaskWorkerDeploymentReasonRollback = "ROLLBACK"

	TaskWorkerRetryBackoffExponential = "EXPONENTIAL_BACKOFF"
)
