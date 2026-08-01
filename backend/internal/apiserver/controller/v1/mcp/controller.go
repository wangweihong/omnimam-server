package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	authmiddleware "github.com/wangweihong/omnimam/backend/internal/apiserver/middleware"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/options"
	mcpsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/mcp"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	protocol "github.com/wangweihong/omnimam/backend/pkg/mcp"
)

// Controller implements the released MCP 2026-07-28 Streamable HTTP endpoint.
type Controller struct {
	processor *protocol.Processor
	users     store.UserStore
	options   *options.MCPOptions
}

func New(processor *protocol.Processor, users store.UserStore, opts *options.MCPOptions) *Controller {
	return &Controller{processor: processor, users: users, options: opts}
}

// Handle validates transport security, independently authenticates the Bearer JWT, and dispatches one JSON-RPC request.
func (c *Controller) Handle(ctx *gin.Context) {
	if c == nil || c.processor == nil || c.users == nil || c.options == nil || !c.options.Enabled {
		ctx.Status(http.StatusNotFound)
		return
	}
	headers := protocol.Headers{
		ProtocolVersion: ctx.GetHeader("MCP-Protocol-Version"), Method: ctx.GetHeader("Mcp-Method"),
		Name: ctx.GetHeader("Mcp-Name"), ContentType: ctx.GetHeader("Content-Type"), Accept: ctx.GetHeader("Accept"),
	}
	if ctx.GetHeader("Mcp-Session-Id") != "" {
		c.write(ctx, transportResponse(nil, protocol.JSONRPCInvalidRequest, "MCP sessions are not supported", protocolFailure(190201, "ERR_MCP_REQUEST_INVALID", "MCP JSON-RPC 请求结构无效。", "The MCP JSON-RPC request is invalid.")), http.StatusBadRequest, headers.Accept)
		return
	}
	if headersTooLarge(ctx) {
		c.write(ctx, transportResponse(nil, protocol.JSONRPCInvalidRequest, "MCP headers exceed the configured limit", protocolFailure(190803, "ERR_MCP_REQUEST_TOO_LARGE", "MCP 请求体或 Header 超过允许上限。", "The MCP request body or headers exceed the configured limit.")), http.StatusBadRequest, headers.Accept)
		return
	}
	if !c.originAllowed(ctx) {
		c.write(ctx, transportResponse(nil, protocol.JSONRPCBusinessError, "MCP Origin is not allowed", protocolFailure(190802, "ERR_MCP_ORIGIN_REJECTED", "请求 Origin 不在允许范围内。", "The request Origin is not allowed.")), http.StatusForbidden, headers.Accept)
		return
	}
	if transportErr := protocol.ValidateHeaders(headers); transportErr != nil {
		transportErr.RPC.Data = c.headerFailure(headers, transportErr.RPC.Message)
		c.write(ctx, protocol.Response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: transportErr.RPC}, transportErr.HTTPStatus, headers.Accept)
		return
	}
	body, err := io.ReadAll(io.LimitReader(ctx.Request.Body, c.options.MaxRequestBytes+1))
	if err != nil || int64(len(body)) > c.options.MaxRequestBytes {
		c.write(ctx, transportResponse(nil, protocol.JSONRPCInvalidRequest, "MCP request body exceeds the configured limit", protocolFailure(190803, "ERR_MCP_REQUEST_TOO_LARGE", "MCP 请求体或 Header 超过允许上限。", "The MCP request body or headers exceed the configured limit.")), http.StatusBadRequest, headers.Accept)
		return
	}
	id := protocol.RequestIDOrNull(body)
	user, err := authmiddleware.ResolveBearerUser(ctx.Request.Context(), ctx.GetHeader("Authorization"), c.users)
	if err != nil {
		failure := protocolFailure(190800, "ERR_MCP_AUTHENTICATION_REQUIRED", "当前请求缺少有效 Identity JWT。", "The request does not contain a valid Identity JWT.")
		c.write(ctx, transportResponse(id, protocol.JSONRPCBusinessError, failure.Message, failure), http.StatusOK, headers.Accept)
		return
	}
	authmiddleware.SetUserContext(ctx, user)
	requestID := ctx.GetString("X-Request-ID")
	ctx.Request = ctx.Request.WithContext(mcpsvc.WithRequestMetadata(ctx.Request.Context(), mcpsvc.RequestMetadata{
		RequestID: requestID,
		TraceID:   traceID(ctx.GetHeader("traceparent"), requestID),
		Transport: "streamable-http",
	}))
	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), c.options.RequestTimeout)
	defer cancel()
	response, status := c.processor.Process(requestCtx, headers, body)
	c.decorateProtocolError(&response)
	c.write(ctx, response, status, headers.Accept)
}

func (c *Controller) originAllowed(ctx *gin.Context) bool {
	origin := strings.TrimSpace(ctx.GetHeader("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	requestHost := strings.TrimSpace(ctx.Request.Host)
	if strings.EqualFold(parsed.Host, requestHost) {
		return true
	}
	normalizedOrigin := strings.TrimRight(parsed.String(), "/")
	for _, allowed := range c.options.AllowedOrigins {
		if strings.EqualFold(strings.TrimRight(strings.TrimSpace(allowed), "/"), normalizedOrigin) {
			return true
		}
	}
	return false
}

func headersTooLarge(ctx *gin.Context) bool {
	limits := map[string]int{
		"MCP-Protocol-Version": 64,
		"Mcp-Method":           128,
		"Mcp-Name":             2048,
		"Origin":               2048,
		"traceparent":          128,
		"tracestate":           512,
		"baggage":              8192,
		"Authorization":        8192,
	}
	for name, maximum := range limits {
		if len(ctx.GetHeader(name)) > maximum {
			return true
		}
	}
	return false
}

func traceID(traceparent, fallback string) string {
	parts := strings.Split(traceparent, "-")
	if len(parts) == 4 && len(parts[1]) == 32 && isLowerHex(parts[1]) && parts[1] != strings.Repeat("0", 32) {
		return parts[1]
	}
	return fallback
}

func isLowerHex(value string) bool {
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func (c *Controller) write(ctx *gin.Context, response protocol.Response, status int, accept string) {
	if status != http.StatusOK || protocol.WantsJSON(accept) {
		ctx.JSON(status, response)
		return
	}
	raw, err := json.Marshal(response)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, transportResponse(response.ID, protocol.JSONRPCInternalError, "MCP response encoding failed", nil))
		return
	}
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-store")
	ctx.Status(status)
	_, _ = fmt.Fprintf(ctx.Writer, "event: message\ndata: %s\n\n", raw)
	if flusher, ok := ctx.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (c *Controller) headerFailure(headers protocol.Headers, message string) *protocol.BusinessError {
	switch {
	case strings.TrimSpace(headers.ProtocolVersion) == "" || strings.TrimSpace(headers.Method) == "":
		return protocolFailure(190202, "ERR_MCP_REQUIRED_HEADER_MISSING", "MCP 请求缺少必需的传输 Header。", "A required MCP transport header is missing.")
	case headers.ProtocolVersion != protocol.ProtocolVersion:
		return protocolFailure(190200, "ERR_MCP_PROTOCOL_VERSION_UNSUPPORTED", "MCP 协议版本不受支持。", "The requested MCP protocol version is not supported.")
	case strings.Contains(message, "media type"):
		return protocolFailure(190206, "ERR_MCP_RESPONSE_MEDIA_UNSUPPORTED", "客户端未声明可接受的 MCP 响应媒体类型。", "The client did not advertise an acceptable MCP response media type.")
	default:
		return protocolFailure(190201, "ERR_MCP_REQUEST_INVALID", "MCP JSON-RPC 请求结构无效。", "The MCP JSON-RPC request is invalid.")
	}
}

func (c *Controller) decorateProtocolError(response *protocol.Response) {
	if response == nil || response.Error == nil || response.Error.Data != nil {
		return
	}
	switch response.Error.Code {
	case protocol.JSONRPCHeaderMismatch:
		response.Error.Data = protocolFailure(190203, "ERR_MCP_HEADER_BODY_MISMATCH", "MCP Header 与 JSON-RPC Body 不一致。", "MCP transport headers do not match the JSON-RPC body.")
	case protocol.JSONRPCMethodNotFound:
		response.Error.Data = protocolFailure(190204, "ERR_MCP_METHOD_UNSUPPORTED", "当前 MCP 方法不受支持。", "The MCP method is not supported.")
	case protocol.JSONRPCInvalidParams:
		response.Error.Data = protocolFailure(190201, "ERR_MCP_REQUEST_INVALID", "MCP JSON-RPC 请求结构无效。", "The MCP JSON-RPC request is invalid.")
	case protocol.JSONRPCInvalidRequest:
		response.Error.Data = protocolFailure(190201, "ERR_MCP_REQUEST_INVALID", "MCP JSON-RPC 请求结构无效。", "The MCP JSON-RPC request is invalid.")
	case protocol.JSONRPCInternalError:
		response.Error.Data = protocolFailure(190402, "ERR_MCP_TOOL_RESULT_INVALID", "下游结果无法转换为 Tool 输出 Schema。", "A downstream result cannot be converted to the tool output schema.")
	}
}

func protocolFailure(value int, name, chinese, english string) *protocol.BusinessError {
	return &protocol.BusinessError{
		Code: name, Value: value, Message: english,
		Messages:     map[string]string{"zh-CN": chinese, "en-US": english},
		Retryable:    value == 190402 || value == 190804 || value == 190805 || value == 190806,
		SourceDomain: "mcp",
	}
}

func transportResponse(id json.RawMessage, rpcCode int, message string, data *protocol.BusinessError) protocol.Response {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	return protocol.Response{JSONRPC: "2.0", ID: id, Error: &protocol.RPCError{Code: rpcCode, Message: message, Data: data}}
}
