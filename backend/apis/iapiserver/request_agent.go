package iapiserver

import (
	"encoding/json"
	"fmt"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

// AgentListRequest 查询当前主体可见 Agent，OwnerUserID 只由 service 注入。
// +k8s:deepcopy-gen=true
type AgentListRequest struct {
	imachinery.BasicQueryParam
	// Statuses 是逗号分隔的 Agent 状态过滤器。
	Statuses string `form:"statuses" binding:"omitempty,max=256"`
	// OwnerUserID 是认证后端注入的所有者范围，客户端不可设置。
	OwnerUserID string `form:"-" json:"-"`
}

// AgentCreateRequest 创建 Platform Agent，并由后端初始化内部 Workspace Binding。
// +k8s:deepcopy-gen=true
type AgentCreateRequest struct {
	// Name 是当前所有者可识别的 Agent 名称。
	Name string `json:"name" binding:"required,min=1,max=200"`
	// Description 是不包含凭证的用途说明。
	Description string `json:"description,omitempty" binding:"omitempty,max=2000"`
	// AgentProfileID 只允许平台启用的只读 Profile。
	AgentProfileID string `json:"agent_profile_id" binding:"required,oneof=agent.hermes"`
	// AgentProfileRevision 固定 Profile 修订；省略时使用当前 ACTIVE 修订。
	AgentProfileRevision string `json:"agent_profile_revision,omitempty" binding:"omitempty,max=64"`
	// ModelBinding 可选地创建主要模型绑定，不包含明文凭证。
	ModelBinding *AgentModelBindingInput `json:"model_binding,omitempty"`
	// RuntimePolicy 是 Profile 允许范围内的恢复和空闲策略。
	RuntimePolicy json.RawMessage `json:"runtime_policy,omitempty"`
}

func (r *AgentCreateRequest) Validate() error {
	if len(r.RuntimePolicy) > 16*1024 {
		return fmt.Errorf("agent runtime policy exceeds 16 KiB")
	}
	return nil
}

// AgentUpdateRequest 只修改非敏感元数据和受控策略，不允许切换 Workspace/Profile/Kind。
// +k8s:deepcopy-gen=true
type AgentUpdateRequest struct {
	// Name 是新的展示名称；空指针表示不修改。
	Name *string `json:"name,omitempty" binding:"omitempty,max=200"`
	// Description 是新的用途说明；空指针表示不修改。
	Description *string `json:"description,omitempty" binding:"omitempty,max=2000"`
	// RuntimePolicy 整体替换受控运行策略。
	RuntimePolicy json.RawMessage `json:"runtime_policy,omitempty"`
	// ResourceVersion 对更新执行乐观并发控制。
	ResourceVersion int64 `json:"resource_version" binding:"required,min=0"`
}

func (r *AgentUpdateRequest) Validate() error {
	if r.Name == nil && r.Description == nil && len(r.RuntimePolicy) == 0 {
		return fmt.Errorf("agent update has no fields")
	}
	if len(r.RuntimePolicy) > 16*1024 {
		return fmt.Errorf("agent runtime policy exceeds 16 KiB")
	}
	return nil
}

// AgentSessionListRequest 查询一个 Agent 的 Session 历史。
// +k8s:deepcopy-gen=true
type AgentSessionListRequest struct {
	imachinery.BasicQueryParam
	// AgentID 来自受权父资源路径。
	AgentID string `form:"-" json:"-"`
	// OwnerUserID 由认证上下文注入。
	OwnerUserID string `form:"-" json:"-"`
}

// AgentSessionCreateRequest 创建可跨 Runtime 恢复的 OPEN Session。
// +k8s:deepcopy-gen=true
type AgentSessionCreateRequest struct {
	// Title 是可选会话标题。
	Title string `json:"title,omitempty" binding:"omitempty,max=200"`
}

// AgentSessionUpdateRequest 更新 Session 元数据。
// +k8s:deepcopy-gen=true
type AgentSessionUpdateRequest struct {
	// Title 是新的会话标题。
	Title string `json:"title" binding:"max=200"`
	// ResourceVersion 对标题更新执行乐观并发控制。
	ResourceVersion int64 `json:"resource_version" binding:"required,min=0"`
}

// AgentMessageRequest 持久化用户消息并创建对应 Invocation。
// +k8s:deepcopy-gen=true
type AgentMessageRequest struct {
	// Content 是业务消息正文，不能作为服务端命令或凭证载体。
	Content string `json:"content" binding:"required,min=1,max=20000"`
	// Attachments 只接受稳定跨域引用。
	Attachments []AgentReference `json:"attachments,omitempty" binding:"omitempty,max=100,dive"`
	// IdempotencyKey 在 Agent 范围防止重复 Invocation。
	IdempotencyKey string `json:"idempotency_key,omitempty" binding:"omitempty,max=200"`
}

// AgentMessageListRequest 查询一个 Session 的消息历史。
// +k8s:deepcopy-gen=true
type AgentMessageListRequest struct {
	imachinery.BasicQueryParam
	// SessionID 来自已授权父资源路径。
	SessionID string `form:"-" json:"-"`
}

// AgentInvocationListRequest 查询一个 Session 的 Invocation 历史。
// +k8s:deepcopy-gen=true
type AgentInvocationListRequest struct {
	imachinery.BasicQueryParam
	// SessionID 来自已授权父资源路径。
	SessionID string `form:"-" json:"-"`
}

// AgentMemoryListRequest 查询 Agent/Session 范围记忆。
// +k8s:deepcopy-gen=true
type AgentMemoryListRequest struct {
	imachinery.BasicQueryParam
	// AgentID 来自已授权父资源路径。
	AgentID string `form:"-" json:"-"`
	// Scope 可选过滤 AGENT 或 SESSION 范围。
	Scope string `form:"scope" binding:"omitempty,oneof=AGENT SESSION"`
}

// AgentMemoryCreateRequest 创建显式记忆或 Session 摘要。
// +k8s:deepcopy-gen=true
type AgentMemoryCreateRequest struct {
	// SessionID 仅 SESSION scope 必填，且必须属于同一 Agent。
	SessionID string `json:"session_id,omitempty" binding:"omitempty,max=128"`
	// Scope 决定记忆属于 Agent 或一个 Session。
	Scope string `json:"scope" binding:"required,oneof=AGENT SESSION"`
	// Type 是受控记忆分类。
	Type string `json:"type" binding:"required,oneof=FACT PREFERENCE SUMMARY INSTRUCTION CONTEXT"`
	// Content 是记忆正文，不能跨用户共享。
	Content string `json:"content" binding:"required,max=20000"`
	// SourceMessageID 可追溯到同一 Agent 的来源消息。
	SourceMessageID string `json:"source_message_id,omitempty" binding:"omitempty,max=128"`
}

func (r *AgentMemoryCreateRequest) Validate() error {
	if (r.Scope == "AGENT" && r.SessionID != "") || (r.Scope == "SESSION" && r.SessionID == "") {
		return fmt.Errorf("agent memory scope and session do not match")
	}
	return nil
}

// AgentMemoryUpdateRequest 整体替换记忆正文。
// +k8s:deepcopy-gen=true
type AgentMemoryUpdateRequest struct {
	// Content 是新的记忆正文。
	Content string `json:"content" binding:"required,max=20000"`
	// ResourceVersion 对更新执行乐观并发控制。
	ResourceVersion int64 `json:"resource_version" binding:"required,min=0"`
}

// AgentRuntimeActionRequest 请求启动或对账恢复 Runtime。
// +k8s:deepcopy-gen=true
type AgentRuntimeActionRequest struct {
	// RequestID 是来源领域操作幂等引用；省略时由服务生成。
	RequestID string `json:"request_id,omitempty" binding:"omitempty,max=200"`
	// ResourceVersion 对 RuntimeBinding 期望状态执行乐观并发控制。
	ResourceVersion int64 `json:"resource_version" binding:"min=0"`
}

// AgentActionRequest 为禁用、挂起、停止、关闭和取消提供安全原因。
// +k8s:deepcopy-gen=true
type AgentActionRequest struct {
	// Reason 是脱敏操作原因，不得包含凭证。
	Reason string `json:"reason,omitempty" binding:"omitempty,max=1000"`
	// RequestID 是当前操作的幂等引用。
	RequestID string `json:"request_id,omitempty" binding:"omitempty,max=200"`
	// ResourceVersion 对目标资源执行乐观并发控制。
	ResourceVersion int64 `json:"resource_version" binding:"min=0"`
}

// AgentModelBindingInput 设置模型来源和用途，不接受 CredentialRef 或明文 Secret。
// +k8s:deepcopy-gen=true
type AgentModelBindingInput struct {
	// SourceType 选择用户默认、用户 Provider 模型或平台模型。
	SourceType string `json:"source_type" binding:"required,oneof=USER_DEFAULT_MODEL USER_PROVIDER_MODEL PLATFORM_MODEL"`
	// SourceRef 是 model-management/modelgateway 可解析的稳定引用。
	SourceRef string `json:"source_ref" binding:"required,max=512"`
	// Purpose 是模型用于 CHAT、CODING、VISION 或 EMBEDDING。
	Purpose string `json:"purpose" binding:"required,oneof=CHAT CODING VISION EMBEDDING"`
}

// AgentMCPBindingRequest 创建显式 MCP 工具绑定。
// +k8s:deepcopy-gen=true
type AgentMCPBindingRequest struct {
	// Name 是 Agent 范围内的 Binding 名称。
	Name string `json:"name" binding:"required,max=200"`
	// ServerType 区分平台、远端和 Runtime 本地 Server。
	ServerType string `json:"server_type" binding:"required,oneof=PLATFORM REMOTE RUNTIME_LOCAL"`
	// EndpointRef 是受控 Endpoint 引用，不接受任意内部地址。
	EndpointRef string `json:"endpoint_ref" binding:"required,max=1024"`
	// CredentialRef 只保存 Secret 引用，响应不会返回。
	CredentialRef string `json:"credential_ref,omitempty" binding:"omitempty,max=1024"`
	// AllowedTools 是显式工具白名单。
	AllowedTools []string `json:"allowed_tools,omitempty" binding:"omitempty,max=200,dive,max=200"`
	// Configuration 是 Server 类型允许的非敏感配置。
	Configuration json.RawMessage `json:"configuration,omitempty"`
}

func (r *AgentMCPBindingRequest) Validate() error {
	if len(r.Configuration) > 32*1024 {
		return fmt.Errorf("agent MCP configuration exceeds 32 KiB")
	}
	return nil
}
