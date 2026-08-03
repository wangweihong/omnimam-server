// Code generated from released spec-v1.12.0 error catalogs; DO NOT EDIT.

package code

const (
	// @HTTP 200
	// @CN Agent 不存在或当前主体不可见。
	// @EN The Agent does not exist or is not visible to the current principal.
	ErrAgentNotVisible int = 200200

	// @HTTP 200
	// @CN AgentProfile 不存在、禁用或版本不可用。
	// @EN The AgentProfile is missing, disabled, or unavailable at the requested revision.
	ErrAgentProfileInvalid int = 200201

	// @HTTP 200
	// @CN Agent 当前状态不允许该操作。
	// @EN The Agent is not in a state that permits this operation.
	ErrAgentStateInvalid int = 200202

	// @HTTP 200
	// @CN Agent Workspace 绑定不符合类型或授权规则。
	// @EN The Agent workspace binding violates workspace type or authorization rules.
	ErrAgentWorkspaceBindingInvalid int = 200203

	// @HTTP 200
	// @CN 无权访问 Agent 资源。
	// @EN The current principal is not authorized to access the Agent resource.
	ErrAgentAccessDenied int = 201000

	// @HTTP 200
	// @CN Session 不存在或当前主体不可见。
	// @EN The Session does not exist or is not visible to the current principal.
	ErrAgentSessionNotVisible int = 200400

	// @HTTP 200
	// @CN Session 已关闭或归档，不能接收新消息。
	// @EN The Session is closed or archived and cannot accept new messages.
	ErrAgentSessionClosed int = 200401

	// @HTTP 200
	// @CN 当前 Session 存在不允许并发的 Invocation。
	// @EN The Session has a conflicting active Invocation.
	ErrAgentInvocationConflict int = 200402

	// @HTTP 200
	// @CN Invocation 关联的 AtomicTask 不可用。
	// @EN The AtomicTask bound to the Invocation is unavailable.
	ErrAgentInvocationTaskUnavailable int = 200403

	// @HTTP 200
	// @CN Agent 模型绑定无效或不支持当前用途。
	// @EN The Agent model binding is invalid or unsupported for the requested purpose.
	ErrAgentModelBindingInvalid int = 200204

	// @HTTP 200
	// @CN Memory scope、类型或来源引用无效。
	// @EN The memory scope, type, or source reference is invalid.
	ErrAgentMemoryInvalid int = 200205

	// @HTTP 200
	// @CN AgentRuntime 不存在或当前主体不可见。
	// @EN The AgentRuntime does not exist or is not visible to the current principal.
	ErrAgentRuntimeNotVisible int = 200800

	// @HTTP 200
	// @CN AgentRuntime 操作失败。
	// @EN The AgentRuntime operation failed.
	ErrAgentRuntimeOperationFailed int = 200801

	// @HTTP 200
	// @CN AgentRuntime 健康检查失败。
	// @EN The AgentRuntime health check failed.
	ErrAgentRuntimeUnhealthy int = 200802

	// @HTTP 200
	// @CN StudioApplication 不存在或当前主体不可见。
	// @EN The StudioApplication does not exist or is not visible to the current principal.
	ErrAppStudioApplicationNotVisible int = 210200

	// @HTTP 200
	// @CN StudioApplication 当前状态不允许该操作。
	// @EN The StudioApplication is not in a state that permits this operation.
	ErrAppStudioApplicationInvalidState int = 210201

	// @HTTP 200
	// @CN StudioWorkspace 不存在或当前主体不可见。
	// @EN The StudioWorkspace does not exist or is not visible to the current principal.
	ErrAppStudioWorkspaceNotVisible int = 210400

	// @HTTP 200
	// @CN Workspace 当前 Revision 与 base_revision 冲突。
	// @EN The Workspace current Revision conflicts with base_revision.
	ErrAppStudioWorkspaceRevisionConflict int = 210401

	// @HTTP 200
	// @CN ChangeSet 未通过路径、依赖或安全校验。
	// @EN The ChangeSet failed path, dependency, or security validation.
	ErrAppStudioWorkspaceChangeRejected int = 210402

	// @HTTP 200
	// @CN Workspace Tool 授权已过期、范围不匹配或不允许当前操作。
	// @EN The Workspace Tool grant is expired, scope-mismatched, or does not allow the operation.
	ErrAppStudioWorkspaceToolGrantInvalid int = 210403

	// @HTTP 200
	// @CN Source Snapshot 不存在或当前主体不可见。
	// @EN The Source Snapshot does not exist or is not visible to the current principal.
	ErrAppStudioSnapshotNotVisible int = 210600

	// @HTTP 200
	// @CN Source Snapshot 未完成或 digest 校验失败。
	// @EN The Source Snapshot is incomplete or its digest validation failed.
	ErrAppStudioSnapshotInvalid int = 210601

	// @HTTP 200
	// @CN StudioBuild 不存在或当前主体不可见。
	// @EN The StudioBuild does not exist or is not visible to the current principal.
	ErrAppStudioBuildNotVisible int = 210800

	// @HTTP 200
	// @CN Build 执行或 Artifact 交付失败。
	// @EN The Build execution or Artifact delivery failed.
	ErrAppStudioBuildFailed int = 210801

	// @HTTP 200
	// @CN Build 已取消。
	// @EN The Build was canceled.
	ErrAppStudioBuildCanceled int = 210802

	// @HTTP 200
	// @CN Build 任务已完成，但 Artifact 尚未 READY 或登记失败。
	// @EN The Build task completed, but the Artifact is not READY or registration failed.
	ErrAppStudioBuildArtifactNotReady int = 210803

	// @HTTP 200
	// @CN Asset Library Artifact digest 与 Build 输出不一致。
	// @EN The Asset Library Artifact digest does not match the Build output.
	ErrAppStudioBuildArtifactDigestMismatch int = 210804

	// @HTTP 200
	// @CN Release 的 Build、Artifact、配置或环境无效。
	// @EN The Release Build, Artifact, configuration, or environment is invalid.
	ErrAppStudioReleaseInvalid int = 211000

	// @HTTP 200
	// @CN StudioRuntimeInstance 部署或健康检查失败。
	// @EN StudioRuntimeInstance deployment or health checking failed.
	ErrAppStudioRuntimeDeployFailed int = 211001

	// @HTTP 200
	// @CN 不允许修改不可变 Release。
	// @EN An immutable Release cannot be modified.
	ErrAppStudioReleaseImmutable int = 211002

	// @HTTP 200
	// @CN 无权访问 AppStudio 资源。
	// @EN The current principal is not authorized to access the AppStudio resource.
	ErrAppStudioAccessDenied int = 211200

	// @HTTP 200
	// @CN Infra 请求缺少受控身份、归属或运行参数。
	// @EN The Infra request lacks a trusted identity, ownership, or runtime parameter.
	ErrInfraRequestInvalid int = 240200

	// @HTTP 200
	// @CN RuntimeProfile 不存在或版本不可用。
	// @EN The RuntimeProfile does not exist or its revision is unavailable.
	ErrInfraRuntimeProfileNotFound int = 240201

	// @HTTP 200
	// @CN 当前第一阶段不支持该运行模式。
	// @EN The requested runtime mode is not supported in the first phase.
	ErrInfraUnsupportedRuntimeMode int = 240202

	// @HTTP 200
	// @CN 相同 requestingService/requestId 对应的请求摘要不一致。
	// @EN The request fingerprint differs for the same requestingService/requestId scope.
	ErrInfraIdempotencyConflict int = 240203

	// @HTTP 200
	// @CN 没有满足资源和状态要求的节点。
	// @EN No node satisfies the requested resource and status requirements.
	ErrInfraNoEligibleNode int = 240400

	// @HTTP 200
	// @CN 可用 CPU、内存、磁盘或 GPU 资源不足。
	// @EN Available CPU, memory, disk, or GPU resources are insufficient.
	ErrInfraResourceInsufficient int = 240401

	// @HTTP 200
	// @CN InfraRuntime 不存在或不可见。
	// @EN The InfraRuntime does not exist or is not visible.
	ErrInfraRuntimeNotFound int = 240600

	// @HTTP 200
	// @CN Runtime Provider 操作失败。
	// @EN The Runtime Provider operation failed.
	ErrInfraRuntimeOperationFailed int = 240601

	// @HTTP 200
	// @CN InfraRuntime 当前状态与操作不一致。
	// @EN The InfraRuntime state conflicts with the requested operation.
	ErrInfraRuntimeStateConflict int = 240602

	// @HTTP 200
	// @CN 挂载引用、目标路径或只读策略不符合安全规则。
	// @EN The mount reference, target path, or read-only policy violates security rules.
	ErrInfraMountNotAllowed int = 240800

	// @HTTP 200
	// @CN SecretRef 或 ModelAccessSpec 解析失败。
	// @EN SecretRef or ModelAccessSpec resolution failed.
	ErrInfraSecretResolutionFailed int = 240801

	// @HTTP 200
	// @CN Endpoint 分配或刷新失败。
	// @EN Endpoint allocation or refresh failed.
	ErrInfraEndpointAllocationFailed int = 240802

	// @HTTP 200
	// @CN 当前主体、owner 或短期授权不允许解析该 Endpoint。
	// @EN The current principal, owner, or short-lived grant cannot resolve the endpoint.
	ErrInfraEndpointAccessDenied int = 240803
)
