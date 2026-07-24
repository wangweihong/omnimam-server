package applicationplatform

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func TestRuntimeRegistryDerivesLocalizedCapabilityNames(t *testing.T) {
	runtime, err := LoadRuntimeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	engine, ok := runtime.EngineType("comfyui")
	if !ok {
		t.Fatal("comfyui engine type is missing")
	}
	wantCN := []string{"图像编辑", "文生图", "图像放大", "图生视频", "文生视频", "视频编辑"}
	wantEN := []string{"Image Editing", "Text to Image", "Image Upscaling", "Image to Video", "Text to Video", "Video Editing"}
	if !reflect.DeepEqual(engine.CapabilityDefinitions["zh-CN"], wantCN) || !reflect.DeepEqual(engine.CapabilityDefinitions["en-US"], wantEN) {
		t.Fatalf("unexpected localized capability names: %#v", engine.CapabilityDefinitions)
	}
	engine.OperationExecutors["unexpected"] = "mutated"
	engine.CapabilityDefinitions["zh-CN"][0] = "mutated"
	again, _ := runtime.EngineType("comfyui")
	if _, exists := again.OperationExecutors["unexpected"]; exists || again.CapabilityDefinitions["zh-CN"][0] != wantCN[0] {
		t.Fatalf("engine type snapshot mutated registry: %#v", again)
	}
}

const validCapabilityManifest = `schema_version: "1.0"
id: test-provider
name: Test Provider
kind: catalog
binding_policy: manual
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
		if registry.Status() != iapiserver.ProviderRegistryReady || len(registry.Capabilities()) != 3 {
			t.Fatalf("unexpected registry state: status=%s capabilities=%d", registry.Status(), len(registry.Capabilities()))
		}
		builtin, ok := registry.Get("comfyui-workflow-runtime")
		if !ok || builtin.Kind != iapiserver.ProviderCapabilityKindEngineBinding || builtin.Origin != iapiserver.ProviderCapabilityOriginBuiltin || builtin.BindingPolicy != iapiserver.ProviderBindingPolicyRequiredImmutable {
			t.Fatalf("unexpected builtin capability: %#v", builtin)
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
		if len(registry.Capabilities()) != 1 {
			t.Fatalf("schema-invalid files must not be registered, got %d entries including builtin", len(registry.Capabilities()))
		}
		if len(registry.Results()) != 3 {
			t.Fatalf("expected builtin plus two file diagnostics, got %d", len(registry.Results()))
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
		if _, ok := registry.Get("test-provider"); ok || len(registry.Capabilities()) != 1 {
			t.Fatal("duplicate capability id was registered")
		}
		for _, result := range registry.Results() {
			if result.ProviderCapabilityID == nil || *result.ProviderCapabilityID != "test-provider" {
				continue
			}
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
		if registry.Status() != iapiserver.ProviderRegistryDegraded || len(registry.Capabilities()) != 1 || len(registry.Results()) != 2 {
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
		if len(registry.Capabilities()) != 1 || len(registry.Results()) != 1 {
			t.Fatal("registry changed after startup snapshot")
		}
		items := registry.Capabilities()
		items[0].Kind = "mutated"
		stored, ok := registry.Get("comfyui-workflow-runtime")
		if !ok || stored.Kind != iapiserver.ProviderCapabilityKindEngineBinding {
			t.Fatal("registry snapshot was mutated through returned capability")
		}
	})

	t.Run("external manifests cannot override builtin ids", func(t *testing.T) {
		directory := t.TempDir()
		reserved := strings.Replace(validCapabilityManifest, "id: test-provider", "id: comfyui-workflow-runtime", 1)
		writeManifest(t, directory, "reserved.yaml", reserved)
		registry, loadErr := LoadProviderCapabilityRegistry(directory, runtime)
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		builtin, ok := registry.Get("comfyui-workflow-runtime")
		if !ok || builtin.Origin != iapiserver.ProviderCapabilityOriginBuiltin {
			t.Fatalf("builtin capability was replaced: %#v", builtin)
		}
		results := registry.Results()
		if len(results) != 2 || results[1].ErrorCode != "ERR_AIAPP_PROVIDER_CAPABILITY_ID_RESERVED" {
			t.Fatalf("reserved id diagnostic missing: %#v", results)
		}
	})
}

func writeManifest(t *testing.T, directory, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
