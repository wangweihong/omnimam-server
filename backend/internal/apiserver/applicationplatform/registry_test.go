package applicationplatform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

const validCapabilityManifest = `schema_version: "1.0"
id: test-provider
name: Test Provider
application_engine_type_id: deepseek_official
revision: "1"
enabled: true
provider:
  code: test
  name: Test
  model_owner: Test
  serving_platform: Test API
  official_website: https://example.com
sources:
  - {type: api_reference, title: API, url: https://example.com/api, checked_at: "2026-07-16", scope: test}
models:
  - id: test-model
    provider_model_id: test-model-v1
    display_name: Test Model
    family: test
    variant: default
    lifecycle: {status: active}
operations:
  - id: chat
    capability_definition_id: text.chat_completion
    execution_mode: synchronous
    input_media_types: [text]
    output_media_types: [text]
variants:
  - id: test-chat
    model_id: test-model
    operation_id: chat
    lifecycle: {status: active}
    input_schema: {type: object, additionalProperties: false, properties: {prompt: {type: string}}}
    output_schema: {type: object, additionalProperties: false, properties: {text: {type: string}}}
`

func TestLoadProviderCapabilityRegistry(t *testing.T) {
	runtime, err := LoadRuntimeRegistry()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("loads valid and disabled manifests", func(t *testing.T) {
		directory := t.TempDir()
		writeManifest(t, directory, "enabled.yaml", validCapabilityManifest)
		disabled := strings.Replace(validCapabilityManifest, "id: test-provider", "id: disabled-provider", 1)
		disabled = strings.Replace(disabled, "enabled: true", "enabled: false", 1)
		writeManifest(t, directory, "disabled.yml", disabled)
		registry, loadErr := LoadProviderCapabilityRegistry(directory, runtime)
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		if registry.Status() != iapiserver.ProviderRegistryReady || len(registry.Capabilities()) != 2 {
			t.Fatalf("unexpected registry state: status=%s capabilities=%d", registry.Status(), len(registry.Capabilities()))
		}
		item, ok := registry.Get("disabled-provider")
		if !ok || item.Availability != iapiserver.ProviderCapabilityDisabled {
			t.Fatalf("disabled capability not preserved: %#v", item)
		}
	})

	t.Run("invalid files fail atomically", func(t *testing.T) {
		directory := t.TempDir()
		writeManifest(t, directory, "invalid.yaml", "schema_version: [")
		unsupported := strings.Replace(validCapabilityManifest, `schema_version: "1.0"`, `schema_version: "2.0"`, 1)
		writeManifest(t, directory, "unsupported.yaml", unsupported)
		registry, loadErr := LoadProviderCapabilityRegistry(directory, runtime)
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		if len(registry.Capabilities()) != 0 {
			t.Fatalf("schema-invalid files must not be registered, got %d entries", len(registry.Capabilities()))
		}
		if len(registry.Results()) != 2 {
			t.Fatalf("expected two file diagnostics, got %d", len(registry.Results()))
		}
	})

	t.Run("duplicate ids are all unregistered", func(t *testing.T) {
		directory := t.TempDir()
		writeManifest(t, directory, "one.yaml", validCapabilityManifest)
		writeManifest(t, directory, "two.yaml", validCapabilityManifest)
		registry, loadErr := LoadProviderCapabilityRegistry(directory, runtime)
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		if _, ok := registry.Get("test-provider"); ok || len(registry.Capabilities()) != 0 {
			t.Fatal("duplicate capability id was registered")
		}
		for _, result := range registry.Results() {
			if result.ErrorCode != "ERR_AIAPP_PROVIDER_CAPABILITY_ID_DUPLICATED" {
				t.Fatalf("unexpected duplicate diagnostic: %#v", result)
			}
		}
	})

	t.Run("directory failure degrades without startup error", func(t *testing.T) {
		registry, loadErr := LoadProviderCapabilityRegistry(filepath.Join(t.TempDir(), "missing"), runtime)
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		if registry.Status() != iapiserver.ProviderRegistryDegraded || len(registry.Results()) != 1 {
			t.Fatalf("unexpected degraded registry: %#v", registry)
		}
	})

	t.Run("scan is non-recursive and snapshot is immutable", func(t *testing.T) {
		directory := t.TempDir()
		if err := os.Mkdir(filepath.Join(directory, "nested"), 0o750); err != nil {
			t.Fatal(err)
		}
		writeManifest(t, filepath.Join(directory, "nested"), "nested.yaml", validCapabilityManifest)
		registry, loadErr := LoadProviderCapabilityRegistry(directory, runtime)
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		writeManifest(t, directory, "later.yaml", validCapabilityManifest)
		if len(registry.Capabilities()) != 0 || len(registry.Results()) != 0 {
			t.Fatal("registry changed after startup snapshot")
		}
	})
}

func writeManifest(t *testing.T, directory, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
