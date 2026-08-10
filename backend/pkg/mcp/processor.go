package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"slices"
	"strconv"
	"strings"
)

const (
	base64SentinelPrefix                 = "=?base64?"
	base64SentinelSuffix                 = "?="
	initializeProtocolVersion20241105    = "2024-11-05"
	initializeProtocolVersion20250326    = "2025-03-26"
	initializeProtocolVersion20250618    = "2025-06-18"
	initializeProtocolVersion20251125    = "2025-11-25"
	initializeProtocolFallbackVersion    = initializeProtocolVersion20251125
	methodInitialize                     = "initialize"
	methodInitializedNotification        = "notifications/initialized"
	methodPing                           = "ping"
	initializeProtocolServerName         = "omnimam-workspace"
	initializeProtocolServerVersion      = "1.0.0"
	initializeProtocolServerInstructions = "Use only application-relative paths. Apply all writes atomically with base_revision and an idempotency key."
)

var initializeProtocolVersions = map[string]struct{}{
	initializeProtocolVersion20241105: {},
	initializeProtocolVersion20250326: {},
	initializeProtocolVersion20250618: {},
	initializeProtocolVersion20251125: {},
}

// Headers contains the MCP transport metadata validated independently for each request.
type Headers struct {
	ProtocolVersion string
	Method          string
	Name            string
	ContentType     string
	Accept          string
}

// TransportError rejects a request before JSON-RPC dispatch using an MCP-mandated HTTP status.
type TransportError struct {
	HTTPStatus int
	RPC        *RPCError
}

func (e *TransportError) Error() string {
	if e == nil || e.RPC == nil {
		return "mcp transport error"
	}
	return e.RPC.Message
}

type Processor struct {
	dispatcher Dispatcher
}

func NewProcessor(dispatcher Dispatcher) (*Processor, error) {
	if dispatcher == nil {
		return nil, fmt.Errorf("mcp dispatcher is required")
	}
	return &Processor{dispatcher: dispatcher}, nil
}

// ValidateHeaders checks transport-only requirements before authentication or dispatch.
func ValidateHeaders(headers Headers) *TransportError {
	if strings.TrimSpace(headers.ProtocolVersion) == "" || strings.TrimSpace(headers.Method) == "" {
		return transportFailure(400, JSONRPCInvalidRequest, "required MCP transport header is missing")
	}
	if headers.ProtocolVersion != ProtocolVersion {
		return transportFailure(400, JSONRPCInvalidRequest, "MCP protocol version is unsupported")
	}
	mediaType, _, err := mime.ParseMediaType(headers.ContentType)
	if err != nil || mediaType != "application/json" {
		return transportFailure(400, JSONRPCInvalidRequest, "Content-Type must be application/json")
	}
	if !accepts(headers.Accept, "application/json") && !accepts(headers.Accept, "text/event-stream") {
		return transportFailure(400, JSONRPCInvalidRequest, "MCP response media type is unsupported")
	}
	return nil
}

// Process validates the JSON-RPC body and dispatches one released MCP method.
func (p *Processor) Process(ctx context.Context, headers Headers, body []byte) (Response, int) {
	request, rpcErr := decodeRequest(body)
	if rpcErr != nil {
		return errorResponse(nil, rpcErr), 400
	}
	if rpcErr = validateHeaderBody(headers, request); rpcErr != nil {
		return errorResponse(request.ID, rpcErr), 400
	}
	invocation, rpcErr := invocationFromRequest(request)
	if rpcErr != nil {
		return errorResponse(request.ID, rpcErr), 200
	}
	result, err := p.dispatcher.Dispatch(ctx, invocation)
	if err != nil {
		var dispatched *RPCError
		if errors.As(err, &dispatched) {
			return errorResponse(request.ID, dispatched), 200
		}
		return errorResponse(request.ID, &RPCError{Code: JSONRPCInternalError, Message: "internal MCP dispatch error"}), 500
	}
	return Response{JSONRPC: "2.0", ID: request.ID, Result: result}, 200
}

// ProcessInitializeProtocol adapts the initialize-handshake MCP versions used by
// OpenCode to the same dispatcher as the released per-request metadata protocol.
// The caller must keep this adapter on an internal, independently authenticated route.
func (p *Processor) ProcessInitializeProtocol(ctx context.Context, body []byte) (*Response, int, bool) {
	var request struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if !json.Valid(body) || json.Unmarshal(body, &request) != nil {
		return nil, 0, false
	}
	switch request.Method {
	case methodInitialize, methodInitializedNotification, methodPing, MethodToolsList, MethodToolsCall:
	default:
		return nil, 0, false
	}
	if request.JSONRPC != "2.0" {
		return initializeProtocolError(request.ID, JSONRPCInvalidRequest, "invalid JSON-RPC request", 400)
	}
	if request.Method == methodInitializedNotification {
		if len(request.ID) != 0 {
			return initializeProtocolError(request.ID, JSONRPCInvalidRequest, "initialized must be a notification", 400)
		}
		return nil, 202, true
	}
	if err := validateRequestID(request.ID); err != nil {
		return initializeProtocolError(json.RawMessage("null"), JSONRPCInvalidRequest, err.Error(), 400)
	}
	if p == nil || p.dispatcher == nil {
		return initializeProtocolError(request.ID, JSONRPCInternalError, "internal MCP dispatch error", 500)
	}
	switch request.Method {
	case methodInitialize:
		var params struct {
			ProtocolVersion string                     `json:"protocolVersion"`
			Capabilities    map[string]json.RawMessage `json:"capabilities"`
			ClientInfo      Implementation             `json:"clientInfo"`
		}
		if err := decodeStrict(request.Params, &params); err != nil || params.ProtocolVersion == "" || params.Capabilities == nil || validateImplementation(params.ClientInfo) != nil {
			return initializeProtocolError(request.ID, JSONRPCInvalidParams, "invalid initialize params", 200)
		}
		version := params.ProtocolVersion
		if _, supported := initializeProtocolVersions[version]; !supported {
			version = initializeProtocolFallbackVersion
		}
		return &Response{JSONRPC: "2.0", ID: request.ID, Result: map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      Implementation{Name: initializeProtocolServerName, Version: initializeProtocolServerVersion},
			"instructions":    initializeProtocolServerInstructions,
		}}, 200, true
	case methodPing:
		return &Response{JSONRPC: "2.0", ID: request.ID, Result: map[string]any{}}, 200, true
	case MethodToolsList:
		result, err := p.dispatcher.Dispatch(ctx, Invocation{Method: MethodToolsList})
		return initializeProtocolDispatchResult(request.ID, result, err)
	case MethodToolsCall:
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments,omitempty"`
		}
		if err := decodeStrict(request.Params, &params); err != nil || params.Name == "" {
			return initializeProtocolError(request.ID, JSONRPCInvalidParams, "invalid tool call params", 200)
		}
		if len(params.Arguments) == 0 {
			params.Arguments = json.RawMessage("{}")
		}
		if !isJSONObject(params.Arguments) {
			return initializeProtocolError(request.ID, JSONRPCInvalidParams, "invalid tool call params", 200)
		}
		result, err := p.dispatcher.Dispatch(ctx, Invocation{Method: MethodToolsCall, Name: params.Name, Arguments: params.Arguments})
		return initializeProtocolDispatchResult(request.ID, result, err)
	default:
		panic("unreachable initialize protocol method")
	}
}

func initializeProtocolDispatchResult(id json.RawMessage, result any, err error) (*Response, int, bool) {
	if err == nil {
		return &Response{JSONRPC: "2.0", ID: id, Result: result}, 200, true
	}
	var dispatched *RPCError
	if errors.As(err, &dispatched) {
		return &Response{JSONRPC: "2.0", ID: id, Error: dispatched}, 200, true
	}
	return initializeProtocolError(id, JSONRPCInternalError, "internal MCP dispatch error", 500)
}

func initializeProtocolError(id json.RawMessage, code int, message string, status int) (*Response, int, bool) {
	return &Response{JSONRPC: "2.0", ID: id, Error: &RPCError{Code: code, Message: message}}, status, true
}

func decodeRequest(body []byte) (Request, *RPCError) {
	var request Request
	if err := decodeStrict(body, &request); err != nil {
		return Request{}, &RPCError{Code: JSONRPCInvalidRequest, Message: "invalid JSON-RPC request"}
	}
	if request.JSONRPC != "2.0" || strings.TrimSpace(request.Method) == "" || len(request.Params) == 0 {
		return Request{}, &RPCError{Code: JSONRPCInvalidRequest, Message: "invalid JSON-RPC request"}
	}
	if err := validateRequestID(request.ID); err != nil {
		return Request{}, &RPCError{Code: JSONRPCInvalidRequest, Message: err.Error()}
	}
	return request, nil
}

func validateRequestID(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("JSON-RPC request id is required")
	}
	return validateStringOrInteger(raw, 128)
}

// RequestIDOrNull extracts only a valid JSON-RPC string/integer ID for pre-dispatch transport/authentication errors.
func RequestIDOrNull(body []byte) json.RawMessage {
	var request struct {
		ID json.RawMessage `json:"id"`
	}
	if !json.Valid(body) || json.Unmarshal(body, &request) != nil || validateRequestID(request.ID) != nil {
		return json.RawMessage("null")
	}
	return request.ID
}

func validateStringOrInteger(raw json.RawMessage, maxStringLength int) error {
	if string(raw) == "null" {
		return fmt.Errorf("value must not be null")
	}
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("value is invalid")
	}
	switch typed := value.(type) {
	case string:
		if len(typed) > maxStringLength {
			return fmt.Errorf("string value is too long")
		}
	case json.Number:
		if _, err := typed.Int64(); err != nil {
			return fmt.Errorf("numeric value must be an integer")
		}
	default:
		return fmt.Errorf("value must be a string or integer")
	}
	return nil
}

func validateHeaderBody(headers Headers, request Request) *RPCError {
	if headers.ProtocolVersion != ProtocolVersion || headers.Method != request.Method {
		return &RPCError{Code: JSONRPCHeaderMismatch, Message: "MCP transport headers do not match the request body"}
	}
	expectedName, nameRequired, err := requestName(request)
	if err != nil {
		return &RPCError{Code: JSONRPCInvalidParams, Message: "invalid MCP request params"}
	}
	if !nameRequired {
		if strings.TrimSpace(headers.Name) != "" {
			return &RPCError{Code: JSONRPCHeaderMismatch, Message: "Mcp-Name must be omitted for this method"}
		}
		return nil
	}
	if strings.TrimSpace(headers.Name) == "" {
		return &RPCError{Code: JSONRPCHeaderMismatch, Message: "Mcp-Name is required for this method"}
	}
	decodedName, err := DecodeHeaderValue(headers.Name)
	if err != nil || decodedName != expectedName {
		return &RPCError{Code: JSONRPCHeaderMismatch, Message: "Mcp-Name does not match the request body"}
	}
	return nil
}

func requestName(request Request) (string, bool, error) {
	switch request.Method {
	case MethodToolsCall:
		var params struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(request.Params, &params); err != nil || params.Name == "" {
			return "", true, fmt.Errorf("tool name is required")
		}
		return params.Name, true, nil
	case MethodResourcesRead:
		var params struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(request.Params, &params); err != nil || params.URI == "" {
			return "", true, fmt.Errorf("resource uri is required")
		}
		return params.URI, true, nil
	default:
		return "", false, nil
	}
}

func invocationFromRequest(request Request) (Invocation, *RPCError) {
	if !slices.Contains(Methods, request.Method) {
		return Invocation{}, &RPCError{Code: JSONRPCMethodNotFound, Message: "MCP method is unsupported"}
	}
	switch request.Method {
	case MethodDiscover:
		var params standardParams
		if err := decodeStrict(request.Params, &params); err != nil {
			return invalidParams(err)
		}
		return invocationWithMeta(request.Method, params.Meta)
	case MethodToolsList, MethodResourcesList, MethodResourceTemplatesList:
		var params paginatedParams
		if err := decodeStrict(request.Params, &params); err != nil {
			return invalidParams(err)
		}
		if len(params.Cursor) > 512 {
			return invalidParams(fmt.Errorf("cursor exceeds 512 characters"))
		}
		invocation, rpcErr := invocationWithMeta(request.Method, params.Meta)
		invocation.Cursor = params.Cursor
		return invocation, rpcErr
	case MethodToolsCall:
		var params toolCallParams
		if err := decodeStrict(request.Params, &params); err != nil || params.Name == "" || !isJSONObject(params.Arguments) {
			return invalidParams(err)
		}
		invocation, rpcErr := invocationWithMeta(request.Method, params.Meta)
		invocation.Name, invocation.Arguments = params.Name, params.Arguments
		return invocation, rpcErr
	case MethodResourcesRead:
		var params resourceReadParams
		if err := decodeStrict(request.Params, &params); err != nil || params.URI == "" || len(params.URI) > 2048 {
			return invalidParams(err)
		}
		invocation, rpcErr := invocationWithMeta(request.Method, params.Meta)
		invocation.URI = params.URI
		return invocation, rpcErr
	case MethodTasksGet, MethodTasksCancel:
		var params taskParams
		if err := decodeStrict(request.Params, &params); err != nil || params.TaskID == "" || len(params.TaskID) > 255 {
			return invalidParams(err)
		}
		invocation, rpcErr := invocationWithMeta(request.Method, params.Meta)
		invocation.TaskID = params.TaskID
		return invocation, rpcErr
	default:
		return Invocation{}, &RPCError{Code: JSONRPCMethodNotFound, Message: "MCP method is unsupported"}
	}
}

type standardParams struct {
	Meta Meta `json:"_meta"`
}

type paginatedParams struct {
	Meta   Meta   `json:"_meta"`
	Cursor string `json:"cursor,omitempty"`
}

type toolCallParams struct {
	Meta      Meta            `json:"_meta"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type resourceReadParams struct {
	Meta Meta   `json:"_meta"`
	URI  string `json:"uri"`
}

type taskParams struct {
	Meta   Meta   `json:"_meta"`
	TaskID string `json:"taskId"`
}

func invocationWithMeta(method string, meta Meta) (Invocation, *RPCError) {
	if meta.ProtocolVersion != ProtocolVersion || meta.ClientCapabilities == nil {
		return Invocation{}, &RPCError{Code: JSONRPCInvalidParams, Message: "required MCP request metadata is invalid"}
	}
	return Invocation{Method: method, Meta: meta}, nil
}

func invalidParams(_ error) (Invocation, *RPCError) {
	return Invocation{}, &RPCError{Code: JSONRPCInvalidParams, Message: "invalid MCP request params"}
}

func isJSONObject(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")
}

// DecodeHeaderValue implements the official MCP 2026-07-28 Base64 sentinel.
func DecodeHeaderValue(value string) (string, error) {
	if !strings.HasPrefix(value, base64SentinelPrefix) || !strings.HasSuffix(value, base64SentinelSuffix) {
		return value, nil
	}
	encoded := strings.TrimSuffix(strings.TrimPrefix(value, base64SentinelPrefix), base64SentinelSuffix)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("invalid MCP Base64 header value: %w", err)
	}
	return string(decoded), nil
}

// EncodeHeaderValue applies the official sentinel when a value is not safe plain ASCII or is sentinel-shaped.
func EncodeHeaderValue(value string) string {
	isSafe := value != "" && strings.TrimSpace(value) == value &&
		!strings.HasPrefix(value, base64SentinelPrefix) && !strings.HasSuffix(value, base64SentinelSuffix)
	if isSafe {
		for _, character := range []byte(value) {
			if character < 0x20 || character > 0x7e {
				isSafe = false
				break
			}
		}
	}
	if isSafe {
		return value
	}
	return base64SentinelPrefix + base64.StdEncoding.EncodeToString([]byte(value)) + base64SentinelSuffix
}

func accepts(header, expected string) bool {
	if strings.TrimSpace(header) == "" {
		return false
	}
	for _, item := range strings.Split(header, ",") {
		mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(item))
		if err != nil || !acceptableQuality(params["q"]) {
			continue
		}
		if mediaType == expected || mediaType == "*/*" {
			return true
		}
	}
	return false
}

func acceptableQuality(raw string) bool {
	if raw == "" {
		return true
	}
	quality, err := strconv.ParseFloat(raw, 64)
	return err == nil && quality > 0 && quality <= 1
}

func WantsJSON(accept string) bool { return accepts(accept, "application/json") }

func errorResponse(id json.RawMessage, rpcErr *RPCError) Response {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	return Response{JSONRPC: "2.0", ID: id, Error: rpcErr}
}

func transportFailure(status, rpcCode int, message string) *TransportError {
	return &TransportError{HTTPStatus: status, RPC: &RPCError{Code: rpcCode, Message: message}}
}
