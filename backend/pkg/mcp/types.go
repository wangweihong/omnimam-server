package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
)

const (
	ProtocolVersion = "2026-07-28"
	// AppStudioWorkspaceToolPath 是仅接受 Invocation 短期 grant 的内部 Workspace Tool 传输路径。
	AppStudioWorkspaceToolPath = "/internal/appstudio/workspace-tool"
	TasksExtension             = "io.modelcontextprotocol/tasks"

	MethodDiscover              = "server/discover"
	MethodToolsList             = "tools/list"
	MethodToolsCall             = "tools/call"
	MethodResourcesList         = "resources/list"
	MethodResourceTemplatesList = "resources/templates/list"
	MethodResourcesRead         = "resources/read"
	MethodTasksGet              = "tasks/get"
	MethodTasksCancel           = "tasks/cancel"

	ToolCapabilitiesList      = "omnimam.capabilities.list"
	ToolCapabilitiesGet       = "omnimam.capabilities.get"
	ToolApplicationsList      = "omnimam.applications.list"
	ToolApplicationsGet       = "omnimam.applications.get"
	ToolApplicationsRun       = "omnimam.applications.run"
	ToolApplicationRunsGet    = "omnimam.application_runs.get"
	ToolApplicationRunsCancel = "omnimam.application_runs.cancel"
	ToolAssetsSearch          = "omnimam.assets.search"
	ToolAssetsGet             = "omnimam.assets.get"
	ToolAssetsPrepareUpload   = "omnimam.assets.prepare_upload"
	ToolAssetsCompleteUpload  = "omnimam.assets.complete_upload"

	// AppStudio Workspace Tool 仅由当前 Coding Invocation 的短期授权发现和调用。
	ToolAppStudioSourceStatus   = "omnimam.appstudio.source.status"
	ToolAppStudioSourceList     = "omnimam.appstudio.source.list"
	ToolAppStudioSourceRead     = "omnimam.appstudio.source.read"
	ToolAppStudioChangeSetApply = "omnimam.appstudio.changeset.apply"

	JSONRPCInvalidRequest = -32600
	JSONRPCMethodNotFound = -32601
	JSONRPCInvalidParams  = -32602
	JSONRPCInternalError  = -32603
	JSONRPCHeaderMismatch = -32020
	JSONRPCBusinessError  = -32000
)

var Methods = []string{
	MethodDiscover,
	MethodToolsList,
	MethodToolsCall,
	MethodResourcesList,
	MethodResourceTemplatesList,
	MethodResourcesRead,
	MethodTasksGet,
	MethodTasksCancel,
}

var ToolNames = []string{
	ToolCapabilitiesList,
	ToolCapabilitiesGet,
	ToolApplicationsList,
	ToolApplicationsGet,
	ToolApplicationsRun,
	ToolApplicationRunsGet,
	ToolApplicationRunsCancel,
	ToolAssetsSearch,
	ToolAssetsGet,
	ToolAssetsPrepareUpload,
	ToolAssetsCompleteUpload,
}

// Request is the strict JSON-RPC envelope accepted by the MCP endpoint.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// Response is a JSON-RPC success or error response for exactly one request.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError carries a protocol error and, for business failures, a stable business payload.
type RPCError struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    *BusinessError `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// BusinessError is the released MCP error projection shared by tool and task failures.
type BusinessError struct {
	Code         string            `json:"code"`
	Value        int               `json:"value"`
	Message      string            `json:"message"`
	Messages     map[string]string `json:"messages"`
	Detail       string            `json:"detail,omitempty"`
	Retryable    bool              `json:"retryable"`
	SourceDomain string            `json:"source_domain"`
	Causes       []ErrorCause      `json:"causes,omitempty"`
}

type ErrorCause struct {
	Path     string `json:"path,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Expected any    `json:"expected,omitempty"`
	Actual   any    `json:"actual,omitempty"`
}

// Meta is required on every MCP 2026-07-28 request.
type Meta struct {
	ProtocolVersion    string              `json:"io.modelcontextprotocol/protocolVersion"`
	ClientCapabilities *ClientCapabilities `json:"io.modelcontextprotocol/clientCapabilities"`
	ClientInfo         *Implementation     `json:"io.modelcontextprotocol/clientInfo,omitempty"`
	ProgressToken      json.RawMessage     `json:"progressToken,omitempty"`
}

// UnmarshalJSON preserves MCP's extensible _meta object while strictly validating its released known fields.
func (m *Meta) UnmarshalJSON(raw []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	protocolVersion, ok := fields["io.modelcontextprotocol/protocolVersion"]
	if !ok || json.Unmarshal(protocolVersion, &m.ProtocolVersion) != nil {
		return fmt.Errorf("MCP protocolVersion is required")
	}
	capabilities, ok := fields["io.modelcontextprotocol/clientCapabilities"]
	if !ok || !isJSONObject(capabilities) {
		return fmt.Errorf("MCP clientCapabilities is required")
	}
	var capabilityFields map[string]json.RawMessage
	if err := json.Unmarshal(capabilities, &capabilityFields); err != nil {
		return fmt.Errorf("decode MCP clientCapabilities: %w", err)
	}
	if extensions, exists := capabilityFields["extensions"]; exists && !isJSONObject(extensions) {
		return fmt.Errorf("MCP clientCapabilities extensions must be an object")
	}
	var clientCapabilities struct {
		Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
	}
	if err := json.Unmarshal(capabilities, &clientCapabilities); err != nil {
		return fmt.Errorf("decode MCP clientCapabilities: %w", err)
	}
	for name, extension := range clientCapabilities.Extensions {
		if !isJSONObject(extension) {
			return fmt.Errorf("MCP client extension %q must be an object", name)
		}
		if name == TasksExtension {
			var empty struct{}
			if err := decodeStrict(extension, &empty); err != nil {
				return fmt.Errorf("MCP Tasks extension must be empty: %w", err)
			}
		}
	}
	m.ClientCapabilities = &ClientCapabilities{Extensions: clientCapabilities.Extensions}
	if clientInfo, exists := fields["io.modelcontextprotocol/clientInfo"]; exists {
		var implementation Implementation
		if err := decodeStrict(clientInfo, &implementation); err != nil {
			return fmt.Errorf("decode MCP clientInfo: %w", err)
		}
		if err := validateImplementation(implementation); err != nil {
			return err
		}
		m.ClientInfo = &implementation
	}
	if progressToken, exists := fields["progressToken"]; exists {
		if err := validateStringOrInteger(progressToken, 128); err != nil {
			return fmt.Errorf("invalid MCP progressToken: %w", err)
		}
		m.ProgressToken = append(m.ProgressToken[:0], progressToken...)
	}
	return nil
}

type ClientCapabilities struct {
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
}

func (c *ClientCapabilities) HasTasks() bool {
	if c == nil || c.Extensions == nil {
		return false
	}
	_, ok := c.Extensions[TasksExtension]
	return ok
}

type Implementation struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
	WebsiteURL  string `json:"websiteUrl,omitempty"`
}

func validateImplementation(value Implementation) error {
	if len(value.Name) < 1 || len(value.Name) > 128 || len(value.Version) < 1 || len(value.Version) > 64 ||
		len(value.Title) > 255 || len(value.Description) > 1000 {
		return fmt.Errorf("MCP clientInfo violates released length limits")
	}
	if value.WebsiteURL == "" {
		return nil
	}
	parsed, err := url.Parse(value.WebsiteURL)
	if err != nil || strings.TrimSpace(parsed.Scheme) == "" {
		return fmt.Errorf("MCP clientInfo websiteUrl must be an absolute URI")
	}
	return nil
}

// Invocation is a validated method call passed to the application dispatcher.
type Invocation struct {
	Method    string
	Meta      Meta
	Cursor    string
	Name      string
	URI       string
	TaskID    string
	Arguments json.RawMessage
}

type Dispatcher interface {
	Dispatch(ctx context.Context, invocation Invocation) (any, error)
}

type DiscoverResult struct {
	ResultType        string             `json:"resultType"`
	SupportedVersions []string           `json:"supportedVersions"`
	Capabilities      ServerCapabilities `json:"capabilities"`
	Instructions      string             `json:"instructions,omitempty"`
	CacheScope        string             `json:"cacheScope"`
	TTLMS             int64              `json:"ttlMs"`
}

type ServerCapabilities struct {
	Tools      ToolsCapability            `json:"tools"`
	Resources  ResourcesCapability        `json:"resources"`
	Extensions map[string]json.RawMessage `json:"extensions"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type ResourcesCapability struct {
	ListChanged bool `json:"listChanged"`
	Subscribe   bool `json:"subscribe"`
}

type ToolDefinition struct {
	Name         string         `json:"name"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	InputSchema  map[string]any `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema"`
}

type ToolsListResult struct {
	ResultType string           `json:"resultType"`
	Tools      []ToolDefinition `json:"tools"`
	NextCursor string           `json:"nextCursor,omitempty"`
}

type ContentBlock struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	URI         string `json:"uri,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mimeType,omitempty"`
}

type ToolCompleteResult struct {
	ResultType        string         `json:"resultType"`
	Content           []ContentBlock `json:"content"`
	StructuredContent any            `json:"structuredContent"`
	IsError           bool           `json:"isError"`
}

type ToolTaskResult struct {
	ResultType        string         `json:"resultType"`
	Task              Task           `json:"task"`
	StructuredContent any            `json:"structuredContent"`
	Content           []ContentBlock `json:"content,omitempty"`
}

type Task struct {
	TaskID          string `json:"taskId"`
	Status          string `json:"status"`
	TTLMS           int64  `json:"ttlMs"`
	PollIntervalMS  int64  `json:"pollIntervalMs"`
	CancelRequested bool   `json:"cancelRequested,omitempty"`
	Result          any    `json:"result,omitempty"`
}

type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mimeType"`
}

type ResourcesListResult struct {
	ResultType string     `json:"resultType"`
	Resources  []Resource `json:"resources"`
	NextCursor string     `json:"nextCursor,omitempty"`
}

type ResourceTemplate struct {
	URITemplate string `json:"uriTemplate"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mimeType"`
}

type ResourceTemplatesListResult struct {
	ResultType        string             `json:"resultType"`
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
	NextCursor        string             `json:"nextCursor,omitempty"`
}

type TextResourceContents struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType"`
	Text     string `json:"text"`
}

type ResourceReadResult struct {
	ResultType string                 `json:"resultType"`
	Contents   []TextResourceContents `json:"contents"`
	CacheScope string                 `json:"cacheScope"`
	TTLMS      int64                  `json:"ttlMs"`
}

func decodeStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return fmt.Errorf("multiple json values are not allowed")
	} else if err != io.EOF {
		return err
	}
	return nil
}
