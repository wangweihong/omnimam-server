package agentmcp

import (
	"encoding/json"
	"testing"
)

func TestParseConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "empty", raw: ""},
		{name: "null", raw: "null", wantErr: true},
		{name: "non-sensitive object", raw: `{"timeout":30,"retry":{"count":2},"tags":["studio"]}`},
		{name: "non-object", raw: `[]`, wantErr: true},
		{name: "trailing object", raw: `{} {}`, wantErr: true},
		{name: "nested authorization", raw: `{"transport":{"Authorization":"Bearer plaintext"}}`, wantErr: true},
		{name: "normalized api key", raw: `{"api_key":"plaintext"}`, wantErr: true},
		{name: "nested secret in array", raw: `{"options":[{"client-secret":"plaintext"}]}`, wantErr: true},
		{name: "resolver url", raw: `{"base_url":"https://untrusted.invalid"}`, wantErr: true},
		{name: "server type", raw: `{"type":"local"}`, wantErr: true},
		{name: "server enabled", raw: `{"enabled":false}`, wantErr: true},
		{name: "host command", raw: `{"command":["sh","-c","id"]}`, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configuration, err := ParseConfiguration(json.RawMessage(test.raw))
			if (err != nil) != test.wantErr {
				t.Fatalf("ParseConfiguration() error = %v, wantErr %v", err, test.wantErr)
			}
			if !test.wantErr && configuration == nil {
				t.Fatal("ParseConfiguration() returned nil configuration")
			}
		})
	}
}

func TestValidateConfigurationRejectsProviderOverrides(t *testing.T) {
	configuration := map[string]any{
		"options": map[string]string{"headers": "plaintext"},
	}
	raw, err := json.Marshal(configuration)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if _, err := ParseConfiguration(raw); err == nil {
		t.Fatal("ParseConfiguration() accepted nested headers from a typed map")
	}
}
