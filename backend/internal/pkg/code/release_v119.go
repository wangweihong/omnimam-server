package code

const (
	ErrAgentMCPBindingInvalid             int = 200206
	ErrAgentMCPBindingNameConflict        int = 200207
	ErrAgentMCPBindingVersionConflict     int = 200208
	ErrAgentMCPBindingRevisionUnavailable int = 200209
)

func init() {
	register(ErrAgentMCPBindingInvalid, 200, map[string]string{"CN": "MCP Binding 无效。", "EN": "The MCP binding is invalid."})
	register(ErrAgentMCPBindingNameConflict, 200, map[string]string{"CN": "当前 Agent 下存在同名的活动 MCP Binding。", "EN": "An active MCP binding with the same name already exists for the Agent."})
	register(ErrAgentMCPBindingVersionConflict, 200, map[string]string{"CN": "MCP Binding 已被更新，请刷新后重试。", "EN": "The MCP binding was modified; refresh it and retry."})
	register(ErrAgentMCPBindingRevisionUnavailable, 200, map[string]string{"CN": "MCP Binding revision 不可用。", "EN": "The MCP binding revision is unavailable."})
}
