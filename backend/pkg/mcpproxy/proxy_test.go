package mcpproxy

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	protocol "github.com/wangweihong/omnimam/backend/pkg/mcp"
)

func TestProxy_Run(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("Authorization = %q", request.Header.Get("Authorization"))
		}
		if request.Header.Get("Mcp-Method") != protocol.MethodDiscover {
			t.Errorf("Mcp-Method = %q", request.Header.Get("Mcp-Method"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete"}}`))
	}))
	defer server.Close()
	proxy, err := New(Config{Endpoint: server.URL, Token: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	input := `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{"extensions":{}}}}}` + "\n"
	var output bytes.Buffer
	if err := proxy.Run(t.Context(), strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(output.String()); got != `{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete"}}` {
		t.Fatalf("output = %s", got)
	}
}

func TestProxy_RejectsRedirectWithoutForwardingToken(t *testing.T) {
	t.Parallel()
	var redirected atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirected.Store(true)
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", target.URL)
		writer.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	proxy, err := New(Config{Endpoint: server.URL, Token: " secret\n"})
	if err != nil {
		t.Fatal(err)
	}
	input := `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{"extensions":{}}}}}` + "\n"
	var output bytes.Buffer
	if err := proxy.Run(t.Context(), strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	if redirected.Load() {
		t.Fatal("proxy followed a redirect and could have exposed the Bearer token")
	}
	if !strings.Contains(output.String(), `"error"`) {
		t.Fatalf("output = %s", output.String())
	}
}

func TestNew_RejectsEndpointUserInfo(t *testing.T) {
	t.Parallel()
	endpoint := &url.URL{Scheme: "https", Host: "example.test", User: url.UserPassword("user", "secret"), Path: "/mcp"}
	if _, err := New(Config{Endpoint: endpoint.String(), Token: "secret"}); err == nil {
		t.Fatal("expected endpoint user info to be rejected")
	}
}

func TestNew_RejectsRemotePlaintextEndpoint(t *testing.T) {
	t.Parallel()
	if _, err := New(Config{Endpoint: "http://example.test/mcp", Token: "secret"}); err == nil {
		t.Fatal("expected a remote plaintext endpoint to be rejected")
	}
}

func TestProxy_ForwardsSSEAsStdioJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n\n"))
	}))
	defer server.Close()
	proxy, err := New(Config{Endpoint: server.URL, Token: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{"extensions":{}}}}}` + "\n"
	var output bytes.Buffer
	if err := proxy.Run(t.Context(), strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(output.String()) != `{"jsonrpc":"2.0","id":1,"result":{}}` {
		t.Fatalf("output = %s", output.String())
	}
}
