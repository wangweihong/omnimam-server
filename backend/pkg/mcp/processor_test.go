package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
)

type recordingDispatcher struct {
	invocation Invocation
	result     any
	err        error
}

func (d *recordingDispatcher) Dispatch(_ context.Context, invocation Invocation) (any, error) {
	d.invocation = invocation
	return d.result, d.err
}

func TestProcessor_Process(t *testing.T) {
	t.Parallel()

	meta := `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{"extensions":{}}}`
	tests := []struct {
		name       string
		headers    Headers
		body       string
		wantStatus int
		wantCode   int
		wantMethod string
	}{
		{
			name:       "discover",
			headers:    Headers{ProtocolVersion: ProtocolVersion, Method: MethodDiscover, ContentType: "application/json", Accept: "application/json"},
			body:       `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{` + meta + `}}`,
			wantStatus: 200,
			wantMethod: MethodDiscover,
		},
		{
			name:    "extensible metadata",
			headers: Headers{ProtocolVersion: ProtocolVersion, Method: MethodDiscover, ContentType: "application/json", Accept: "application/json;q=1.0"},
			body: `{"jsonrpc":"2.0","id":2,"method":"server/discover","params":{"_meta":{` +
				`"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
				`"io.modelcontextprotocol/clientCapabilities":{"vendorCapability":true,"extensions":{"vendor.extension":{"enabled":true}}},` +
				`"vendor.metadata":{"value":1}}}}`,
			wantStatus: 200,
			wantMethod: MethodDiscover,
		},
		{
			name:       "method header mismatch",
			headers:    Headers{ProtocolVersion: ProtocolVersion, Method: MethodToolsList, ContentType: "application/json", Accept: "application/json"},
			body:       `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{` + meta + `}}`,
			wantStatus: 400,
			wantCode:   JSONRPCHeaderMismatch,
		},
		{
			name:       "unknown body member",
			headers:    Headers{ProtocolVersion: ProtocolVersion, Method: MethodDiscover, ContentType: "application/json", Accept: "application/json"},
			body:       `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{` + meta + `},"session":"forbidden"}`,
			wantStatus: 400,
			wantCode:   JSONRPCInvalidRequest,
		},
		{
			name:    "null client capabilities",
			headers: Headers{ProtocolVersion: ProtocolVersion, Method: MethodDiscover, ContentType: "application/json", Accept: "application/json"},
			body: `{"jsonrpc":"2.0","id":3,"method":"server/discover","params":{"_meta":{` +
				`"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
				`"io.modelcontextprotocol/clientCapabilities":null}}}`,
			wantStatus: 200,
			wantCode:   JSONRPCInvalidParams,
		},
		{
			name:    "null client extensions",
			headers: Headers{ProtocolVersion: ProtocolVersion, Method: MethodDiscover, ContentType: "application/json", Accept: "application/json"},
			body: `{"jsonrpc":"2.0","id":4,"method":"server/discover","params":{"_meta":{` +
				`"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
				`"io.modelcontextprotocol/clientCapabilities":{"extensions":null}}}}`,
			wantStatus: 200,
			wantCode:   JSONRPCInvalidParams,
		},
		{
			name:       "legacy method rejected",
			headers:    Headers{ProtocolVersion: ProtocolVersion, Method: "initialize", ContentType: "application/json", Accept: "application/json"},
			body:       `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{` + meta + `}}`,
			wantStatus: 200,
			wantCode:   JSONRPCMethodNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dispatcher := &recordingDispatcher{result: map[string]any{"resultType": "complete"}}
			processor, err := NewProcessor(dispatcher)
			if err != nil {
				t.Fatal(err)
			}
			response, status := processor.Process(t.Context(), tt.headers, []byte(tt.body))
			if status != tt.wantStatus {
				t.Fatalf("status = %d, want %d", status, tt.wantStatus)
			}
			if tt.wantCode != 0 {
				if response.Error == nil || response.Error.Code != tt.wantCode {
					t.Fatalf("error = %#v, want code %d", response.Error, tt.wantCode)
				}
			}
			if dispatcher.invocation.Method != tt.wantMethod {
				t.Fatalf("method = %q, want %q", dispatcher.invocation.Method, tt.wantMethod)
			}
		})
	}
}

func TestValidateHeaders_RejectsZeroQuality(t *testing.T) {
	t.Parallel()
	failure := ValidateHeaders(Headers{
		ProtocolVersion: ProtocolVersion,
		Method:          MethodDiscover,
		ContentType:     "application/json",
		Accept:          "application/json;q=0.0, text/event-stream;q=0",
	})
	if failure == nil {
		t.Fatal("expected zero-quality media types to be rejected")
	}
}

func TestProcessor_DecodesMCPNameSentinel(t *testing.T) {
	t.Parallel()
	name := "omnimam://assets/素材"
	encoded := "=?base64?" + base64.StdEncoding.EncodeToString([]byte(name)) + "?="
	body := map[string]any{
		"jsonrpc": "2.0", "id": "read-1", "method": MethodResourcesRead,
		"params": map[string]any{
			"_meta": map[string]any{
				"io.modelcontextprotocol/protocolVersion":    ProtocolVersion,
				"io.modelcontextprotocol/clientCapabilities": map[string]any{"extensions": map[string]any{}},
			},
			"uri": name,
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := &recordingDispatcher{result: map[string]any{"resultType": "complete"}}
	processor, err := NewProcessor(dispatcher)
	if err != nil {
		t.Fatal(err)
	}
	response, status := processor.Process(t.Context(), Headers{
		ProtocolVersion: ProtocolVersion, Method: MethodResourcesRead, Name: encoded,
		ContentType: "application/json", Accept: "application/json",
	}, raw)
	if status != 200 || response.Error != nil {
		t.Fatalf("status=%d response=%#v", status, response)
	}
	if dispatcher.invocation.URI != name {
		t.Fatalf("uri = %q, want %q", dispatcher.invocation.URI, name)
	}
}

func TestEncodeHeaderValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value string
		plain bool
	}{
		{name: "plain tool", value: ToolApplicationsRun, plain: true},
		{name: "unicode uri", value: "omnimam://assets/素材"},
		{name: "leading whitespace", value: " padded "},
		{name: "sentinel shaped", value: "=?base64?literal?="},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			encoded := EncodeHeaderValue(tt.value)
			if tt.plain && encoded != tt.value {
				t.Fatalf("encoded = %q, want plain value", encoded)
			}
			decoded, err := DecodeHeaderValue(encoded)
			if err != nil || decoded != tt.value {
				t.Fatalf("decoded = %q, err=%v, want %q", decoded, err, tt.value)
			}
		})
	}
}

func TestCatalogMatchesReleasedSurface(t *testing.T) {
	t.Parallel()
	definitions := ToolDefinitions(nil)
	if len(definitions) != 11 {
		t.Fatalf("tool count = %d, want 11", len(definitions))
	}
	if len(ResourceTemplates()) != 6 {
		t.Fatalf("resource template count = %d, want 6", len(ResourceTemplates()))
	}
	validator, err := NewResultValidator()
	if err != nil {
		t.Fatal(err)
	}
	if validator == nil {
		t.Fatal("result validator is nil")
	}
}
