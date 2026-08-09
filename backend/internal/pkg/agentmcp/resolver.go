package agentmcp

import "context"

// Resolution is the non-persistent, non-logging result of resolving an
// authorized MCP binding revision for a runtime.
type Resolution struct {
	ServerKey     string
	ServerType    string
	Endpoint      string
	Credential    string
	AllowedTools  []string
	Configuration map[string]any
}

// Resolver is the internal authorization port owned by the Agent domain.
type Resolver interface {
	ResolveMCPBinding(context.Context, string, string, string) (*Resolution, error)
}
