package applicationplatform

import (
	"strings"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func TestStaticRegistries(t *testing.T) {
	registration := testRegistration()
	runtime, err := NewRuntimeRegistry([]Registration{registration})
	if err != nil {
		t.Fatal(err)
	}
	capabilities, err := NewProviderCapabilityRegistry([]Registration{registration}, runtime)
	if err != nil {
		t.Fatal(err)
	}
	engineTypes := runtime.EngineTypes()
	if len(engineTypes) != 1 || engineTypes[0].ID != "test" || engineTypes[0].NameI18n["zh-CN"] != "测试" || engineTypes[0].DefaultAPIBaseURL != "https://api.example.com/v1" {
		t.Fatalf("unexpected engine types: %#v", engineTypes)
	}
	authSchema := engineTypes[0].AuthenticationConfigSchema[iapiserver.EngineAuthAPIKey]
	if authSchema["additionalProperties"] != false {
		t.Fatalf("authentication schema is not strict JSON Schema: %#v", authSchema)
	}
	items := capabilities.Capabilities()
	if len(items) != 1 || items[0].ID != "test-capability" || items[0].Origin != iapiserver.ProviderCapabilityOriginStatic || items[0].Availability != iapiserver.ProviderCapabilityAvailable {
		t.Fatalf("unexpected provider capabilities: %#v", items)
	}
	items[0].NameI18n["zh-CN"] = "changed"
	engineTypes[0].NameI18n["zh-CN"] = "changed"
	storedCapability, _ := capabilities.Get("test-capability")
	storedEngine, _ := runtime.EngineType("test")
	if storedCapability.NameI18n["zh-CN"] != "测试能力" || storedEngine.NameI18n["zh-CN"] != "测试" {
		t.Fatal("registry returned mutable internal state")
	}
}

func TestStaticRegistryRejectsInvalidRegistrations(t *testing.T) {
	tests := []struct {
		name          string
		registrations func() []Registration
		want          string
	}{
		{
			name: "duplicate adapter",
			registrations: func() []Registration {
				first, second := testRegistration(), testRegistration()
				second.EngineType.ID = "test-two"
				return []Registration{first, second}
			},
			want: "duplicate static engine adapter",
		},
		{
			name: "missing localization",
			registrations: func() []Registration {
				item := testRegistration()
				delete(item.EngineType.DescriptionI18n, "en-US")
				return []Registration{item}
			},
			want: "missing zh-CN or en-US",
		},
		{
			name: "invalid executor reference",
			registrations: func() []Registration {
				item := testRegistration()
				item.EngineType.OperationExecutors["text.responses"] = "missing"
				return []Registration{item}
			},
			want: "invalid executor mapping",
		},
		{
			name: "missing authentication schema",
			registrations: func() []Registration {
				item := testRegistration()
				item.EngineType.AuthenticationConfigSchema = nil
				return []Registration{item}
			},
			want: "invalid authentication schema",
		},
		{
			name: "non-strict authentication schema",
			registrations: func() []Registration {
				item := testRegistration()
				item.EngineType.AuthenticationConfigSchema[iapiserver.EngineAuthAPIKey]["additionalProperties"] = true
				return []Registration{item}
			},
			want: "invalid authentication schema",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewRuntimeRegistry(test.registrations())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestStaticCapabilityRejectsInvalidReferences(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*iapiserver.AIAppProviderCapability)
		want   string
	}{
		{name: "unknown model", mutate: func(item *iapiserver.AIAppProviderCapability) { item.Variants[0].ModelID = "missing" }, want: "unknown model"},
		{name: "invalid model lifecycle", mutate: func(item *iapiserver.AIAppProviderCapability) { item.Models[0].Lifecycle.Status = "unknown" }, want: "model"},
		{name: "invalid execution mode", mutate: func(item *iapiserver.AIAppProviderCapability) { item.Operations[0].ExecutionMode = "streaming" }, want: "operation"},
		{name: "missing variant schema", mutate: func(item *iapiserver.AIAppProviderCapability) { item.Variants[0].InputSchema = nil }, want: "variant"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registration := testRegistration()
			runtime, err := NewRuntimeRegistry([]Registration{registration})
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(&registration.ProviderCapabilities[0])
			_, err = NewProviderCapabilityRegistry([]Registration{registration}, runtime)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestStaticCapabilityRejectsInvalidSourceURL(t *testing.T) {
	registration := testRegistration()
	runtime, err := NewRuntimeRegistry([]Registration{registration})
	if err != nil {
		t.Fatal(err)
	}
	registration.ProviderCapabilities[0].Sources[0]["url"] = "relative"
	_, err = NewProviderCapabilityRegistry([]Registration{registration}, runtime)
	if err == nil || !strings.Contains(err.Error(), "invalid url") {
		t.Fatalf("err = %v", err)
	}
}

func testRegistration() Registration {
	definition := iapiserver.CapabilityDefinition{ID: "text.responses", NameI18n: map[string]string{"zh-CN": "统一响应", "en-US": "Responses"}, InputMediaTypes: []string{"text"}, OutputMediaTypes: []string{"text"}}
	return Registration{
		CapabilityDefinitions: []iapiserver.CapabilityDefinition{definition},
		EngineAdapter:         iapiserver.EngineAdapterDefinition{ID: "test", Responsibilities: []string{"authentication"}},
		OperationExecutors:    []iapiserver.OperationExecutorDefinition{{ID: "test-responses", EngineAdapterID: "test", CapabilityDefinitionIDs: []string{definition.ID}}},
		EngineType:            iapiserver.ApplicationEngineType{ID: "test", NameI18n: map[string]string{"zh-CN": "测试", "en-US": "Test"}, DescriptionI18n: map[string]string{"zh-CN": "测试服务。", "en-US": "Test service."}, OfficialWebsiteURL: "https://example.com/", OfficialDocumentationURL: "https://example.com/docs", DefaultAPIBaseURL: "https://api.example.com/v1", Enabled: true, EngineAdapterID: "test", AuthenticationTypes: []string{iapiserver.EngineAuthAPIKey}, AuthenticationConfigSchema: AuthenticationConfigSchema(iapiserver.EngineAuthAPIKey), OperationExecutors: map[string]string{definition.ID: "test-responses"}},
		ProviderCapabilities:  []iapiserver.AIAppProviderCapability{{SchemaVersion: "1.0", ID: "test-capability", NameI18n: map[string]string{"zh-CN": "测试能力", "en-US": "Test Capability"}, DescriptionI18n: map[string]string{"zh-CN": "测试能力说明。", "en-US": "Test capability description."}, Kind: iapiserver.ProviderCapabilityKindCatalog, Origin: iapiserver.ProviderCapabilityOriginStatic, BindingPolicy: iapiserver.ProviderBindingPolicyManual, ApplicationEngineTypeID: "test", Revision: "1", Enabled: true, Provider: map[string]any{"code": "test", "official_website": "https://example.com/"}, Sources: []map[string]any{{"url": "https://example.com/docs"}}, Models: []iapiserver.ProviderCapabilityModel{{ID: "test-model", ProviderModelID: "test-model", DisplayNameI18n: map[string]string{"zh-CN": "测试模型", "en-US": "Test Model"}, DescriptionI18n: map[string]string{"zh-CN": "测试模型说明。", "en-US": "Test model description."}, Family: "test", Variant: "default", Lifecycle: iapiserver.ProviderLifecycle{Status: "active"}}}, Operations: []iapiserver.ProviderCapabilityOperation{{ID: "responses", CapabilityDefinitionID: definition.ID, NameI18n: map[string]string{"zh-CN": "统一响应", "en-US": "Responses"}, DescriptionI18n: map[string]string{"zh-CN": "响应说明。", "en-US": "Response description."}, ExecutionMode: "synchronous", InputMediaTypes: []string{"text"}, OutputMediaTypes: []string{"text"}}}, Variants: []iapiserver.ProviderCapabilityVariant{{ID: "test-model-responses", ModelID: "test-model", OperationID: "responses", Lifecycle: iapiserver.ProviderLifecycle{Status: "active"}, InputSchema: map[string]any{"type": "object"}, OutputSchema: map[string]any{"type": "object"}}}}},
	}
}
