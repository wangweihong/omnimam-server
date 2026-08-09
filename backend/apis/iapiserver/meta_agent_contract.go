package iapiserver

const (
	// AgentProfileIDHermes 是 Platform Agent 使用的内置 Hermes Profile 标识。
	AgentProfileIDHermes = "agent.hermes"
	// AgentProfileIDCoding 是 Studio Coding Agent 使用的内置 Profile 标识。
	AgentProfileIDCoding = "agent.coding"
	// AgentProfileRevisionInitial 是内置 Agent Profile 的初始固定修订。
	AgentProfileRevisionInitial = "1.0"
	// AgentProfileStatusActive 表示 Profile 可用于创建 Agent。
	AgentProfileStatusActive = "ACTIVE"
)

const (
	// AgentStatusReady 表示 Agent 已完成初始化并可接受操作。
	AgentStatusReady = "READY"
	// AgentStatusStarting 表示 Agent Runtime 正在启动或恢复。
	AgentStatusStarting = "STARTING"
	// AgentStatusIdle 表示 Agent Runtime 已就绪且当前空闲。
	AgentStatusIdle = "IDLE"
	// AgentStatusSuspended 表示 Agent Runtime 已挂起。
	AgentStatusSuspended = "SUSPENDED"
	// AgentStatusDisabled 表示 Agent 已禁用并拒绝新 Invocation。
	AgentStatusDisabled = "DISABLED"
	// AgentStatusDeleting 表示 Agent 正在停止 Runtime 并进入删除流程。
	AgentStatusDeleting = "DELETING"
	// AgentStatusError 表示 Agent Runtime 投影进入错误状态。
	AgentStatusError = "ERROR"
)

const (
	// AgentSessionStatusOpen 表示 Session 可更新并接受消息。
	AgentSessionStatusOpen = "OPEN"
	// AgentSessionStatusClosed 表示 Session 已关闭但仍保留历史。
	AgentSessionStatusClosed = "CLOSED"
	// AgentSessionStatusArchived 表示 Session 已归档。
	AgentSessionStatusArchived = "ARCHIVED"
)

const (
	// AgentWorkspaceAccessModeReadWrite 允许 Agent 读写其固定 Workspace。
	AgentWorkspaceAccessModeReadWrite = "READ_WRITE"
	// AgentAuthorizationSourceAgent 表示授权摘要由 Agent 固定绑定流程产生。
	AgentAuthorizationSourceAgent = "agent"
)

const (
	// AgentModelBindingSourceTypeUserDefault 选择当前用户的默认模型。
	AgentModelBindingSourceTypeUserDefault = "USER_DEFAULT_MODEL"
	// AgentModelBindingSourceRefUserDefault 是用户默认模型的稳定来源引用。
	AgentModelBindingSourceRefUserDefault = "user-default"
	// AgentModelBindingSourceTypeUserProvider 选择当前用户显式绑定的 Provider Model。
	AgentModelBindingSourceTypeUserProvider = "USER_PROVIDER_MODEL"
	// AgentModelBindingSourceTypePlatform 选择平台托管模型；无 released resolver 时必须 fail closed。
	AgentModelBindingSourceTypePlatform = "PLATFORM_MODEL"
	// AgentModelBindingPurposeChat 表示模型用于 Platform Agent 对话。
	AgentModelBindingPurposeChat = "CHAT"
	// AgentModelBindingPurposeCoding 表示模型用于 Coding Agent 操作。
	AgentModelBindingPurposeCoding = "CODING"
	// AgentModelBindingStatusActive 表示模型绑定可被 Runtime 使用。
	AgentModelBindingStatusActive = "ACTIVE"
)

const (
	// AgentOperationEventTypeInvocationStarted 表示 Runtime 已开始处理 Invocation。
	AgentOperationEventTypeInvocationStarted = "invocation.started"
	// AgentOperationEventTypeMessageCompleted 表示 assistant Message 已持久化。
	AgentOperationEventTypeMessageCompleted = "message.completed"
	// AgentOperationEventTypeInvocationCompleted 表示 Invocation 已产生可投影的成功结果。
	AgentOperationEventTypeInvocationCompleted = "invocation.completed"
	// AgentOperationEventTypeInvocationCanceled 表示 Invocation 已响应取消。
	AgentOperationEventTypeInvocationCanceled = "invocation.canceled"
)

const (
	// AgentMessageRoleUser 表示消息由当前用户提交。
	AgentMessageRoleUser = "USER"
	// AgentMessageRoleAssistant 表示消息由 Agent Invocation 成功生成。
	AgentMessageRoleAssistant = "ASSISTANT"
	// AgentInvocationTypeChat 表示普通对话 Invocation。
	AgentInvocationTypeChat = "CHAT"
	// AgentInvocationTypeCoding 表示 Coding Agent Invocation。
	AgentInvocationTypeCoding = "CODING"
	// AgentInvocationStatusQueued 表示 Invocation 已持久化并等待执行。
	AgentInvocationStatusQueued = "QUEUED"
	// AgentInvocationStatusStarting 表示 Worker 正在建立或恢复 Runtime 会话。
	AgentInvocationStatusStarting = "STARTING"
	// AgentInvocationStatusRunning 表示 Runtime 正在执行 Invocation。
	AgentInvocationStatusRunning = "RUNNING"
	// AgentInvocationStatusWaitingForTool 表示 Invocation 正在等待工具结果。
	AgentInvocationStatusWaitingForTool = "WAITING_FOR_TOOL"
	// AgentInvocationStatusWaitingForUser 表示 Invocation 正在等待用户输入。
	AgentInvocationStatusWaitingForUser = "WAITING_FOR_USER"
	// AgentInvocationStatusCanceling 表示 Invocation 正在请求取消关联任务。
	AgentInvocationStatusCanceling = "CANCELING"
	// AgentInvocationStatusSucceeded 表示 Invocation 已成功完成。
	AgentInvocationStatusSucceeded = "SUCCEEDED"
	// AgentInvocationStatusFailed 表示 Invocation 已失败终止。
	AgentInvocationStatusFailed = "FAILED"
	// AgentInvocationStatusCanceled 表示 Invocation 已取消终止。
	AgentInvocationStatusCanceled = "CANCELED"
	// AgentInvocationFailureCodeTaskUnavailable 表示缺少可执行 Invocation 的任务适配器。
	AgentInvocationFailureCodeTaskUnavailable = "ERR_AGENT_INVOCATION_TASK_UNAVAILABLE"
)

const (
	// AgentRuntimeOperationStart 请求首次启动 Agent Runtime。
	AgentRuntimeOperationStart = "START"
	// AgentRuntimeOperationRecover 请求恢复已有 Agent Runtime。
	AgentRuntimeOperationRecover = "RECOVER"
	// AgentRuntimeActionSuspend 请求停止 Runtime 并保留挂起语义。
	AgentRuntimeActionSuspend = "SUSPEND"
	// AgentRuntimeActionStop 请求停止当前 Runtime。
	AgentRuntimeActionStop = "STOP"
	// AgentRuntimeActionDelete 请求为删除 Agent 停止 Runtime。
	AgentRuntimeActionDelete = "DELETE"
	// AgentRuntimeStateStarting 表示 Runtime 正在启动或恢复。
	AgentRuntimeStateStarting = "STARTING"
	// AgentRuntimeStateReady 表示 Runtime 已成功建立。
	AgentRuntimeStateReady = "READY"
	// AgentRuntimeStateStopping 表示 Runtime 正在停止。
	AgentRuntimeStateStopping = "STOPPING"
	// AgentRuntimeStateStopped 表示 Runtime 已停止。
	AgentRuntimeStateStopped = "STOPPED"
	// AgentRuntimeStateFailed 表示 Runtime 操作失败。
	AgentRuntimeStateFailed = "FAILED"
	// AgentRuntimeActivityIdle 表示 Runtime 已就绪且无活动 Invocation。
	AgentRuntimeActivityIdle = "IDLE"
	// AgentRuntimeActivitySuspended 表示 Runtime 活动已挂起。
	AgentRuntimeActivitySuspended = "SUSPENDED"
	// AgentRuntimeHealthUnknown 表示尚无可用 Runtime 健康结论。
	AgentRuntimeHealthUnknown = "UNKNOWN"
	// AgentRuntimeHealthHealthy 表示 Runtime 健康可用。
	AgentRuntimeHealthHealthy = "HEALTHY"
	// AgentRuntimeHealthUnhealthy 表示 Runtime 健康检查失败。
	AgentRuntimeHealthUnhealthy = "UNHEALTHY"
)

const (
	// AgentRuntimeFunctionEnsure 是 Agent 启动与恢复任务的 functionRef。
	AgentRuntimeFunctionEnsure = "agent.runtime.ensure"
	// AgentRuntimeFunctionStop 是 Agent 停止任务的 functionRef。
	AgentRuntimeFunctionStop = "agent.runtime.stop"
	// AgentInvocationFunctionExecute 是 CHAT/CODING Invocation 共用的执行 functionRef。
	AgentInvocationFunctionExecute = "agent.invocation.execute"
	// AgentTaskDomain 是 Agent 向 Task Center 创建任务时使用的领域标识。
	AgentTaskDomain = "agent"
)

const (
	// AgentRuntimeGrantStatusActive 表示 Grant 仍可解析其固定 Binding revisions。
	AgentRuntimeGrantStatusActive = "ACTIVE"
	// AgentRuntimeGrantStatusRevoked 表示 Grant 已被生命周期操作主动撤销。
	AgentRuntimeGrantStatusRevoked = "REVOKED"
	// AgentRuntimeGrantStatusExpired 表示 Grant 已超过 Runtime 授权窗口。
	AgentRuntimeGrantStatusExpired = "EXPIRED"
)

const (
	// AgentMCPServerTypePlatform 表示 OmniMAM 平台提供的受信 MCP Server。
	AgentMCPServerTypePlatform = "PLATFORM"
	// AgentMCPServerTypeRemote 表示由受信远端目标注册表解析的 MCP Server。
	AgentMCPServerTypeRemote = "REMOTE"
	// AgentMCPServerTypeRuntimeLocal 表示 Runtime Profile 声明的本地 MCP Server。
	AgentMCPServerTypeRuntimeLocal = "RUNTIME_LOCAL"
	// AgentMCPBindingCredentialRefModeKeep 保留 Binding 当前 Secret 引用。
	AgentMCPBindingCredentialRefModeKeep = "KEEP"
	// AgentMCPBindingCredentialRefModeSet 使用请求中的新 Secret 引用。
	AgentMCPBindingCredentialRefModeSet = "SET"
	// AgentMCPBindingCredentialRefModeClear 清除 Binding 当前 Secret 引用。
	AgentMCPBindingCredentialRefModeClear = "CLEAR"
	// AgentMCPPlatformEndpointRefDefault 是平台默认 MCP Server 的受控引用。
	AgentMCPPlatformEndpointRefDefault = "platform-mcp://default"
	// AgentRuntimeMaxMCPBindings 是一次 Runtime Grant 允许固定的 Binding 上限。
	AgentRuntimeMaxMCPBindings = 50
)

const (
	AgentInvocationTaskKeyAgentID                    = "agent_id"
	AgentInvocationTaskKeySessionID                  = "session_id"
	AgentInvocationTaskKeyInvocationID               = "invocation_id"
	AgentInvocationTaskKeyRuntimeBindingID           = "runtime_binding_id"
	AgentInvocationTaskKeyInvocationType             = "invocation_type"
	AgentInvocationTaskKeyAuthorizationRef           = "authorization_ref"
	AgentInvocationTaskKeyExpectedResourceVersion    = "expected_resource_version"
	AgentInvocationTaskKeyResumeRuntimeSessionRef    = "resume_runtime_session_ref"
	AgentInvocationTaskKeyResumeRuntimeInvocationRef = "resume_runtime_invocation_ref"
	AgentInvocationTaskKeyEventSequenceAfter         = "event_sequence_after"
)
