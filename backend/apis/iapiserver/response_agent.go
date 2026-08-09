package iapiserver

import (
	"encoding/json"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

// AgentOperationResult 是无资源正文动作的业务结果。
// +k8s:deepcopy-gen=true
type AgentOperationResult struct {
	Success bool `json:"success"`
}

// AgentProfileListResponse 返回只读 Profile 列表。
// +k8s:deepcopy-gen=true
type AgentProfileListResponse struct {
	Total int             `json:"total"`
	Items []*AgentProfile `json:"items"`
}

// AgentListResponse 返回当前主体可见 Agent。
// +k8s:deepcopy-gen=true
type AgentListResponse struct {
	Total int64    `json:"total"`
	Items []*Agent `json:"items"`
}

// AgentSessionListResponse 返回 Session 历史。
// +k8s:deepcopy-gen=true
type AgentSessionListResponse struct {
	Total int64           `json:"total"`
	Items []*AgentSession `json:"items"`
}

// AgentMessageListResponse 返回 Session 消息历史。
// +k8s:deepcopy-gen=true
type AgentMessageListResponse struct {
	Total int64           `json:"total"`
	Items []*AgentMessage `json:"items"`
}

// AgentInvocationListResponse 返回 Session Invocation 历史。
// +k8s:deepcopy-gen=true
type AgentInvocationListResponse struct {
	Total int64              `json:"total"`
	Items []*AgentInvocation `json:"items"`
}

// AgentInvocationEventEnvelope 是持久化 Invocation 事件的统一 SSE data 结构。
// +k8s:deepcopy-gen=true
type AgentInvocationEventEnvelope struct {
	// InvocationID 标识事件所属的单次 Agent 交互。
	InvocationID string `json:"invocation_id"`
	// SequenceNo 是 Invocation 内从 1 开始严格递增的恢复游标。
	SequenceNo int64 `json:"sequence_no"`
	// OccurredAt 是事件首次持久化的时间。
	OccurredAt imachinery.Time `json:"occurred_at"`
	// Event 是 12 类统一 Invocation 事件之一，并与 SSE event 字段一致。
	Event string `json:"event"`
	// Payload 是与 Event 对应的完整类型化 JSON payload。
	Payload json.RawMessage `json:"payload"`
}

// AgentInvocationStartedEventPayload 描述 Invocation 开始执行。
type AgentInvocationStartedEventPayload struct {
	Status string `json:"status"`
}

// AgentMessageDeltaEventPayload 描述稳定 assistant Message 的新增文本片段。
type AgentMessageDeltaEventPayload struct {
	MessageID string `json:"message_id"`
	Delta     string `json:"delta"`
}

// AgentMessageCompletedEventPayload 描述已持久化的完整 assistant Message。
type AgentMessageCompletedEventPayload struct {
	MessageID   string           `json:"message_id"`
	Role        string           `json:"role"`
	Content     string           `json:"content"`
	Attachments []AgentReference `json:"attachments"`
	CreatedAt   imachinery.Time  `json:"created_at"`
}

// AgentToolRequestedEventPayload 描述一次受控工具请求。
type AgentToolRequestedEventPayload struct {
	ToolCallID   string `json:"tool_call_id"`
	ToolName     string `json:"tool_name"`
	InputSummary string `json:"input_summary,omitempty"`
}

// AgentToolStartedEventPayload 描述一次受控工具开始执行。
type AgentToolStartedEventPayload struct {
	ToolCallID string `json:"tool_call_id"`
	ToolName   string `json:"tool_name"`
}

// AgentToolProgressEventPayload 描述一次受控工具的限长进度摘要。
type AgentToolProgressEventPayload struct {
	ToolCallID      string   `json:"tool_call_id"`
	ToolName        string   `json:"tool_name"`
	ProgressMessage string   `json:"progress_message"`
	ProgressPercent *float64 `json:"progress_percent,omitempty"`
}

// AgentToolCompletedEventPayload 描述一次受控工具成功完成。
type AgentToolCompletedEventPayload struct {
	ToolCallID    string `json:"tool_call_id"`
	ToolName      string `json:"tool_name"`
	OutputSummary string `json:"output_summary,omitempty"`
}

// AgentToolFailedEventPayload 描述一次受控工具失败。
type AgentToolFailedEventPayload struct {
	ToolCallID     string `json:"tool_call_id"`
	ToolName       string `json:"tool_name"`
	FailureCode    string `json:"failure_code"`
	FailureMessage string `json:"failure_message"`
}

// AgentUserInputRequiredEventPayload 描述 Invocation 等待用户补充输入。
type AgentUserInputRequiredEventPayload struct {
	RequestID string `json:"request_id"`
	Prompt    string `json:"prompt"`
}

// AgentInvocationCompletedEventPayload 描述 Invocation 成功终态。
type AgentInvocationCompletedEventPayload struct {
	Status             string `json:"status"`
	AssistantMessageID string `json:"assistant_message_id"`
}

// AgentInvocationFailedEventPayload 描述 Invocation 失败终态的脱敏原因。
type AgentInvocationFailedEventPayload struct {
	Status         string `json:"status"`
	FailureCode    string `json:"failure_code"`
	FailureMessage string `json:"failure_message"`
}

// AgentInvocationCanceledEventPayload 描述 Invocation 取消终态。
type AgentInvocationCanceledEventPayload struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// AgentMemoryListResponse 返回 Agent 可见记忆。
// +k8s:deepcopy-gen=true
type AgentMemoryListResponse struct {
	Total int64          `json:"total"`
	Items []*AgentMemory `json:"items"`
}

// AgentModelBindingListResponse 返回模型绑定列表。
// +k8s:deepcopy-gen=true
type AgentModelBindingListResponse struct {
	Total int64                `json:"total"`
	Items []*AgentModelBinding `json:"items"`
}

// AgentSkillBindingListResponse 返回 Skill 绑定列表。
// +k8s:deepcopy-gen=true
type AgentSkillBindingListResponse struct {
	Total int64                `json:"total"`
	Items []*AgentSkillBinding `json:"items"`
}

// AgentMCPBindingListResponse 返回不含 CredentialRef 的 MCP 绑定列表。
// +k8s:deepcopy-gen=true
type AgentMCPBindingListResponse struct {
	Total int64              `json:"total"`
	Items []*AgentMCPBinding `json:"items"`
}
