package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAICompatibleAdapterInvoke(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	adapter := NewOpenAICompatibleAdapter(OpenAICompatibleConfig{
		ID:      "test",
		BaseURL: server.URL,
		APIKey:  "test-key",
	})
	result, err := adapter.Invoke(context.Background(), InvokeRequest{
		Model: "test-model",
		Messages: []ChatMessage{
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}
	if result.Content != "ok" {
		t.Fatalf("unexpected content: %s", result.Content)
	}
}
