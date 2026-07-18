package iapiserver

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEngineInstanceSummaryIncludesBaseURLWithoutAuthConfig(t *testing.T) {
	instance := &EngineInstance{
		ApplicationEngineTypeID: "comfyui",
		BaseURL:                 "http://127.0.0.1:8188",
		AuthConfig:              map[string]any{"api_key": "secret"},
	}
	instance.ID = "engine-1"
	instance.Name = "ComfyUI"

	summary := instance.Summary()
	if summary.BaseURL != instance.BaseURL {
		t.Fatalf("base URL = %q, want %q", summary.BaseURL, instance.BaseURL)
	}
	raw, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}
	if !strings.Contains(string(raw), `"base_url":"http://127.0.0.1:8188"`) {
		t.Fatalf("summary does not expose base_url: %s", raw)
	}
	if strings.Contains(string(raw), "auth_config") || strings.Contains(string(raw), "secret") {
		t.Fatalf("summary exposes authentication config: %s", raw)
	}
}
