package mcpproxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	protocol "github.com/wangweihong/omnimam/backend/pkg/mcp"
)

type Config struct {
	Endpoint        string
	Token           string
	Timeout         time.Duration
	MaxMessageBytes int
	HTTPClient      *http.Client
}

type Proxy struct {
	endpoint        string
	token           string
	timeout         time.Duration
	maxMessageBytes int
	client          *http.Client
}

func New(config Config) (*Proxy, error) {
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" || endpoint.User != nil {
		return nil, fmt.Errorf("MCP endpoint must be an absolute http or https URL without user info")
	}
	if endpoint.Scheme == "http" && !isLoopbackHost(endpoint.Hostname()) {
		return nil, fmt.Errorf("remote MCP endpoints must use https")
	}
	config.Token = strings.TrimSpace(config.Token)
	if config.Token == "" {
		return nil, fmt.Errorf("OMNIMAM_MCP_TOKEN is required")
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxMessageBytes <= 0 {
		config.MaxMessageBytes = 1 << 20
	}
	if config.MaxMessageBytes < 1024 || config.MaxMessageBytes > 16<<20 {
		return nil, fmt.Errorf("max MCP message bytes must be between 1024 and 16777216")
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: config.Timeout}
	}
	clientCopy := *client
	clientCopy.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &Proxy{
		endpoint: endpoint.String(), token: config.Token, timeout: config.Timeout,
		maxMessageBytes: config.MaxMessageBytes, client: &clientCopy,
	}, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Run forwards newline-delimited stdio JSON-RPC requests sequentially and preserves stdout as protocol-only output.
func (p *Proxy) Run(ctx context.Context, input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), p.maxMessageBytes)
	writer := bufio.NewWriter(output)
	defer writer.Flush()
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		response := p.forward(ctx, line)
		if _, err := writer.Write(response); err != nil {
			return fmt.Errorf("write MCP stdio response: %w", err)
		}
		if err := writer.WriteByte('\n'); err != nil {
			return fmt.Errorf("terminate MCP stdio response: %w", err)
		}
		if err := writer.Flush(); err != nil {
			return fmt.Errorf("flush MCP stdio response: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read MCP stdio request: %w", err)
	}
	return nil
}

func (p *Proxy) forward(ctx context.Context, raw []byte) []byte {
	request, err := proxyRequest(raw)
	if err != nil {
		return localError(nil, protocol.JSONRPCInvalidRequest, "invalid JSON-RPC request")
	}
	requestCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	httpRequest, err := http.NewRequestWithContext(requestCtx, http.MethodPost, p.endpoint, bytes.NewReader(raw))
	if err != nil {
		return localError(request.ID, protocol.JSONRPCInternalError, "MCP proxy request creation failed")
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json, text/event-stream")
	httpRequest.Header.Set("Authorization", "Bearer "+p.token)
	httpRequest.Header.Set("MCP-Protocol-Version", protocol.ProtocolVersion)
	httpRequest.Header.Set("Mcp-Method", request.Method)
	if request.Name != "" {
		httpRequest.Header.Set("Mcp-Name", protocol.EncodeHeaderValue(request.Name))
	}
	response, err := p.client.Do(httpRequest)
	if err != nil {
		return localError(request.ID, protocol.JSONRPCInternalError, "MCP endpoint is unavailable")
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, int64(p.maxMessageBytes)+1)
	body, err := io.ReadAll(limited)
	if err != nil || len(body) > p.maxMessageBytes {
		return localError(request.ID, protocol.JSONRPCInternalError, "MCP endpoint response is invalid")
	}
	mediaType, _, mediaErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if mediaErr != nil || (mediaType != "application/json" && mediaType != "text/event-stream") {
		return localError(request.ID, protocol.JSONRPCInternalError, "MCP endpoint response is invalid")
	}
	if mediaType == "text/event-stream" {
		body = firstSSEData(body)
	}
	if !json.Valid(body) {
		return localError(request.ID, protocol.JSONRPCInternalError, "MCP endpoint response is invalid")
	}
	responseID := protocol.RequestIDOrNull(body)
	if !bytes.Equal(bytes.TrimSpace(responseID), bytes.TrimSpace(request.ID)) {
		return localError(request.ID, protocol.JSONRPCInternalError, "MCP endpoint response id does not match the request")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, body); err != nil {
		return localError(request.ID, protocol.JSONRPCInternalError, "MCP endpoint response is invalid")
	}
	return compact.Bytes()
}

type requestMetadata struct {
	ID     json.RawMessage
	Method string
	Name   string
}

func proxyRequest(raw []byte) (requestMetadata, error) {
	var request struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || request.JSONRPC != "2.0" || len(request.ID) == 0 || request.Method == "" {
		return requestMetadata{}, fmt.Errorf("invalid JSON-RPC request")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return requestMetadata{}, fmt.Errorf("multiple JSON values are not allowed")
	}
	metadata := requestMetadata{ID: request.ID, Method: request.Method}
	switch request.Method {
	case protocol.MethodToolsCall:
		var params struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(request.Params, &params); err != nil || params.Name == "" {
			return requestMetadata{}, fmt.Errorf("tool name is required")
		}
		metadata.Name = params.Name
	case protocol.MethodResourcesRead:
		var params struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(request.Params, &params); err != nil || params.URI == "" {
			return requestMetadata{}, fmt.Errorf("resource URI is required")
		}
		metadata.Name = params.URI
	}
	return metadata, nil
}

func firstSSEData(body []byte) []byte {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			return bytes.TrimSpace([]byte(strings.TrimPrefix(line, "data:")))
		}
	}
	return nil
}

func localError(id json.RawMessage, code int, message string) []byte {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	raw, _ := json.Marshal(protocol.Response{
		JSONRPC: "2.0", ID: id,
		Error: &protocol.RPCError{Code: code, Message: message},
	})
	return raw
}
