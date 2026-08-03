package iapiserver

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
