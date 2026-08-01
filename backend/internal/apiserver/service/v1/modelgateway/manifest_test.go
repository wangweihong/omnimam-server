package modelgateway

import (
	"strings"
	"testing"
)

const testCapabilityManifest = `
schema_version: "1.0"
id: first-capability
name_i18n: {zh-CN: 第一个, en-US: First}
description_i18n: {zh-CN: 第一个能力。, en-US: First capability.}
kind: engine_binding
binding_policy: required_immutable
application_engine_type_id: test
revision: "1"
enabled: true
provider: {official_website: "https://example.com/"}
sources: [{url: "https://example.com/docs"}]
models: []
operations: []
variants: []
---
schema_version: "1.0"
id: second-capability
name_i18n: {zh-CN: 第二个, en-US: Second}
description_i18n: {zh-CN: 第二个能力。, en-US: Second capability.}
kind: engine_binding
binding_policy: required_immutable
application_engine_type_id: test
revision: "1"
enabled: true
provider: {official_website: "https://example.com/"}
sources: [{url: "https://example.com/docs"}]
models: []
operations: []
variants: []
`

func TestParseProviderCapabilityManifest(t *testing.T) {
	items, err := ParseProviderCapabilityManifest("test", testCapabilityManifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != "first-capability" || items[1].ID != "second-capability" {
		t.Fatalf("unexpected manifest items: %#v", items)
	}
	if items[0].Origin != "static" {
		t.Fatalf("origin = %q, want static", items[0].Origin)
	}
}

func TestParseProviderCapabilityManifestRejectsInvalidDocuments(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		want     string
	}{
		{name: "unknown field", manifest: strings.Replace(testCapabilityManifest, "revision: \"1\"", "unknown_field: true\nrevision: \"1\"", 1), want: "field unknown_field not found"},
		{name: "duplicate key", manifest: strings.Replace(testCapabilityManifest, "revision: \"1\"", "revision: \"1\"\nrevision: \"2\"", 1), want: "mapping key \"revision\" already defined"},
		{name: "empty document", manifest: "---\n", want: "document 1 is empty"},
		{name: "wrong schema", manifest: strings.Replace(testCapabilityManifest, "schema_version: \"1.0\"", "schema_version: \"2.0\"", 1), want: "unsupported schema_version"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseProviderCapabilityManifest("provider-test", test.manifest)
			if err == nil || !strings.Contains(err.Error(), test.want) || !strings.Contains(err.Error(), "provider-test") {
				t.Fatalf("err = %v, want source and %q", err, test.want)
			}
		})
	}
}
