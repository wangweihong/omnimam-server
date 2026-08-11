package code

const (
	// @HTTP 200
	// @CN MCP Binding 无效。
	// @EN The MCP binding is invalid.
	ErrAgentMCPBindingInvalid int = 200206

	// @HTTP 200
	// @CN 当前 Agent 下存在同名的活动 MCP Binding。
	// @EN An active MCP binding with the same name already exists for the Agent.
	ErrAgentMCPBindingNameConflict int = 200207

	// @HTTP 200
	// @CN MCP Binding 已被更新，请刷新后重试。
	// @EN The MCP binding was modified; refresh it and retry.
	ErrAgentMCPBindingVersionConflict int = 200208

	// @HTTP 200
	// @CN MCP Binding revision 不可用。
	// @EN The MCP binding revision is unavailable.
	ErrAgentMCPBindingRevisionUnavailable int = 200209
)
