package modelgateway

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type testCapabilityValidator struct{ id string }

func (v testCapabilityValidator) ID() string { return v.id }
func (v testCapabilityValidator) Validate(_ context.Context, request CapabilityValidationRequest) error {
	if request.Value["blocked"] == true {
		return fmt.Errorf("blocked value")
	}
	return nil
}

type testCredentialResolver struct {
	request CredentialResolveRequest
	calls   int
}

func (r *testCredentialResolver) ResolveCredential(_ context.Context, request CredentialResolveRequest) (*ResolvedCredential, error) {
	r.request = request
	r.calls++
	return &ResolvedCredential{Authentication: map[string]any{"api_key": "secret"}}, nil
}

type testUserModelAdapter struct{}

func (testUserModelAdapter) ID() string { return "test" }
func (testUserModelAdapter) Check(context.Context, *iapiserver.EngineInstance) (*iapiserver.EngineHealthCheckResult, error) {
	return &iapiserver.EngineHealthCheckResult{HealthStatus: iapiserver.EngineHealthOnline}, nil
}
func (testUserModelAdapter) DiscoverProviderModels(context.Context, *iapiserver.EngineInstance) ([]DiscoveredModel, error) {
	return []DiscoveredModel{{RemoteModel: "z-model"}, {RemoteModel: "a-model"}, {RemoteModel: "a-model"}, {}}, nil
}
func (testUserModelAdapter) ProbeProviderModel(_ context.Context, _ *iapiserver.EngineInstance, remoteModel string) (*ModelProbeResult, error) {
	return &ModelProbeResult{RemoteModel: remoteModel, Available: true, StreamSupported: true}, nil
}

type testUserModelExecutor struct {
	input map[string]any
}

func (*testUserModelExecutor) ID() string { return "test-responses" }
func (e *testUserModelExecutor) Execute(_ context.Context, _ *iapiserver.EngineInstance, run *iapiserver.ApplicationRun) (map[string]any, error) {
	e.input = run.InputSnapshot
	return map[string]any{"content": "ok"}, nil
}

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
	providerTypes := runtime.ProviderTypes()
	if len(providerTypes) != 1 || providerTypes[0].ID != "test" || providerTypes[0].DisplayName != "Test Provider" || !providerTypes[0].SupportsModelDiscovery {
		t.Fatalf("unexpected provider types: %#v", providerTypes)
	}
	items := capabilities.Capabilities()
	if len(items) != 1 || items[0].ID != "test-capability" || items[0].Origin != iapiserver.ProviderCapabilityOriginStatic || items[0].Availability != iapiserver.ProviderCapabilityAvailable {
		t.Fatalf("unexpected provider capabilities: %#v", items)
	}
	items[0].NameI18n["zh-CN"] = "changed"
	engineTypes[0].NameI18n["zh-CN"] = "changed"
	providerTypes[0].AuthenticationTypes[0] = iapiserver.EngineAuthNone
	providerTypes[0].ConfigurationSchema["properties"].(map[string]any)["changed"] = map[string]any{"type": "string"}
	storedCapability, _ := capabilities.Get("test-capability")
	storedEngine, _ := runtime.EngineType("test")
	storedProvider := runtime.ProviderTypes()[0]
	if storedCapability.NameI18n["zh-CN"] != "测试能力" || storedEngine.NameI18n["zh-CN"] != "测试" || storedProvider.AuthenticationTypes[0] != iapiserver.EngineAuthAPIKey {
		t.Fatal("registry returned mutable internal state")
	}
	if _, changed := storedProvider.ConfigurationSchema["properties"].(map[string]any)["changed"]; changed {
		t.Fatal("provider type configuration schema shares mutable nested state")
	}
}

func TestProviderTypeRegistryRejectsInternalMappingErrors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Registration)
		want   string
	}{
		{name: "unknown adapter", mutate: func(item *Registration) { item.ProviderType.AdapterID = "missing" }, want: "unknown adapter"},
		{name: "unknown executor", mutate: func(item *Registration) { item.ProviderType.OperationExecutors["text.responses"] = "missing" }, want: "invalid executor mapping"},
		{name: "unsupported auth", mutate: func(item *Registration) { item.ProviderType.AuthenticationTypes = []string{"password"} }, want: "unsupported authentication type"},
		{name: "non-strict config", mutate: func(item *Registration) { item.ProviderType.ConfigurationSchema["additionalProperties"] = true }, want: "must reject additional properties"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registration := testRegistration()
			test.mutate(&registration)
			_, err := NewRuntimeRegistry([]Registration{registration})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestUserModelGatewayProviderOperations(t *testing.T) {
	runtime, err := NewRuntimeRegistry([]Registration{testRegistration()})
	if err != nil {
		t.Fatal(err)
	}
	credentials := &testCredentialResolver{}
	executor := &testUserModelExecutor{}
	service, err := NewUserModelGatewayService(UserModelGatewayDependencies{
		Runtime: runtime, Adapters: map[string]Adapter{"test": testUserModelAdapter{}},
		Executors: map[string]OperationExecutor{"test-responses": executor}, Credentials: credentials,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := ProviderConnectionRequest{
		ProviderType: "test", Endpoint: "https://api.example.com/v1", AuthType: iapiserver.EngineAuthAPIKey, CredentialHandle: "opaque-handle", Config: map[string]any{},
	}
	providerTypes, err := service.ListProviderTypes(context.Background())
	if err != nil || len(providerTypes) != 1 || providerTypes[0].ID != "test" {
		t.Fatalf("provider types = %#v, %v", providerTypes, err)
	}
	probe, err := service.TestProviderConnection(context.Background(), request)
	if err != nil || !probe.Success || probe.HealthStatus != iapiserver.ProviderModelHealthHealthy {
		t.Fatalf("provider probe = %#v, %v", probe, err)
	}
	if credentials.request.Handle != "opaque-handle" || credentials.request.ProviderType != "test" || credentials.request.AuthenticationType != iapiserver.EngineAuthAPIKey {
		t.Fatalf("credential scope = %#v", credentials.request)
	}
	models, err := service.DiscoverProviderModels(context.Background(), request)
	if err != nil || len(models) != 2 || models[0].RemoteModel != "a-model" || models[0].DisplayName != "a-model" || models[1].RemoteModel != "z-model" {
		t.Fatalf("models = %#v, %v", models, err)
	}
	modelProbe, err := service.ProbeProviderModel(context.Background(), request, "a-model")
	if err != nil || !modelProbe.Available || len(modelProbe.CapabilityDefinitionIDs) != 1 || modelProbe.CapabilityDefinitionIDs[0] != "text.responses" {
		t.Fatalf("model probe = %#v, %v", modelProbe, err)
	}
	resolution, err := service.ResolveUserModelCapabilities("test", modelProbe, nil)
	if err != nil || !resolution.Executable || resolution.Status != CapabilityResolutionResolved || !resolution.StreamSupported {
		t.Fatalf("capability resolution = %#v, %v", resolution, err)
	}
	resolution, err = service.ResolveUserModelCapabilities("test", modelProbe, []string{"text.responses"})
	if err != nil || resolution.Executable || resolution.Status != CapabilityResolutionUnavailable {
		t.Fatalf("disabled capability resolution = %#v, %v", resolution, err)
	}
	grant := UserModelExecutionContext{
		OwnerUserID: "user-1", ProviderID: "provider-1", ModelID: "model-1", RemoteModel: "remote-1",
		ProviderType: "test", CapabilityDefinitionID: "text.responses",
		CapabilityDefinitionIDs: []string{"text.responses"}, ConfigVersion: 3,
		CredentialHandle: "opaque-handle", IssuedAt: time.Now().Add(-time.Second), ExpiresAt: time.Now().Add(time.Minute),
		Issuer: UserModelExecutionContextIssuer, Endpoint: "https://api.example.com/v1",
		AuthenticationType: iapiserver.EngineAuthAPIKey, ProviderConfiguration: map[string]any{},
	}
	result, err := service.ExecuteOperation(context.Background(), OperationExecutionRequest{
		PrincipalUserID: "user-1", Target: UserModelTarget{ExecutionContext: grant},
		CapabilityDefinitionID: "text.responses", Input: map[string]any{"input": "hello"},
	})
	if err != nil || result.Output["content"] != "ok" || executor.input["model"] != "remote-1" {
		t.Fatalf("operation result = %#v, input = %#v, err = %v", result, executor.input, err)
	}
	requestWithWrongPrincipal := OperationExecutionRequest{
		PrincipalUserID: "user-2", Target: UserModelTarget{ExecutionContext: grant}, CapabilityDefinitionID: "text.responses",
	}
	if _, err := service.ExecuteOperation(context.Background(), requestWithWrongPrincipal); errors.ToStatus(err).Code != code.ErrAIAppPermissionDenied {
		t.Fatalf("wrong principal error = %v", err)
	}
	grant.ExpiresAt = time.Now().Add(-time.Second)
	if _, err := service.ExecuteOperation(context.Background(), OperationExecutionRequest{
		PrincipalUserID: "user-1", Target: UserModelTarget{ExecutionContext: grant}, CapabilityDefinitionID: "text.responses",
	}); errors.ToStatus(err).Code != code.ErrAIAppPermissionDenied {
		t.Fatalf("expired context error = %v", err)
	}
}

func TestUserModelGatewayRejectsUntrustedConnectionFields(t *testing.T) {
	runtime, err := NewRuntimeRegistry([]Registration{testRegistration()})
	if err != nil {
		t.Fatal(err)
	}
	credentials := &testCredentialResolver{}
	service, err := NewUserModelGatewayService(UserModelGatewayDependencies{
		Runtime: runtime, Adapters: map[string]Adapter{"test": testUserModelAdapter{}},
		Executors: map[string]OperationExecutor{"test-responses": &testUserModelExecutor{}}, Credentials: credentials,
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		request ProviderConnectionRequest
		code    int
	}{
		{name: "unknown provider type", request: ProviderConnectionRequest{ProviderType: "missing", Endpoint: "https://api.example.com", AuthType: iapiserver.EngineAuthAPIKey, CredentialHandle: "handle"}, code: code.ErrAIAppProviderRuntimeCapabilityMismatch},
		{name: "endpoint user info", request: ProviderConnectionRequest{ProviderType: "test", Endpoint: "https://user:secret@api.example.com", AuthType: iapiserver.EngineAuthAPIKey, CredentialHandle: "handle", Config: map[string]any{}}, code: code.ErrAIAppEngineUnavailable},
		{name: "unsupported config", request: ProviderConnectionRequest{ProviderType: "test", Endpoint: "https://api.example.com", AuthType: iapiserver.EngineAuthAPIKey, CredentialHandle: "handle", Config: map[string]any{"adapter_id": "test"}}, code: code.ErrAIAppEngineAuthConfigInvalid},
		{name: "missing handle", request: ProviderConnectionRequest{ProviderType: "test", Endpoint: "https://api.example.com", AuthType: iapiserver.EngineAuthAPIKey, Config: map[string]any{}}, code: code.ErrAIAppEngineAuthConfigInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.TestProviderConnection(context.Background(), test.request)
			if errors.ToStatus(err).Code != test.code {
				t.Fatalf("error = %v, code = %d", err, errors.ToStatus(err).Code)
			}
		})
	}
	if _, err := service.ResolveUserModelCapabilities("test", &ModelProbeResult{Available: true}, []string{"invented.capability"}); errors.ToStatus(err).Code != code.ErrAIAppProviderRuntimeCapabilityMismatch {
		t.Fatalf("unknown disabled capability error = %v", err)
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

func TestProviderCapabilitySchemaValidation(t *testing.T) {
	registration := testRegistration()
	registration.ProviderCapabilities[0].Operations[0].InputSchema = map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"model", "prompt"},
		"properties": map[string]any{
			"model":   map[string]any{"type": "string"},
			"prompt":  map[string]any{"type": "string", "minLength": 1},
			"blocked": map[string]any{"type": "boolean"},
		},
		validatorIDsExtension: []any{"test.input"},
	}
	registration.ProviderCapabilities[0].Operations[0].OutputSchema = map[string]any{
		"type": "object", "additionalProperties": true, "required": []string{"id"},
		"properties": map[string]any{"id": map[string]any{"type": "string"}},
	}
	registration.ProviderCapabilities[0].Variants[0].InputSchema = nil
	registration.ProviderCapabilities[0].Variants[0].OutputSchema = nil
	runtime, err := NewRuntimeRegistry([]Registration{registration})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewProviderCapabilityRegistry([]Registration{registration}, runtime, testCapabilityValidator{id: "test.input"})
	if err != nil {
		t.Fatal(err)
	}
	valid := map[string]any{"model": "test-model", "prompt": "hello"}
	if err := registry.ValidateInput(context.Background(), "test-capability", "responses", "test-model", nil, nil, valid); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	if err := registry.ValidateInput(context.Background(), "test-capability", "responses", "unknown-model", nil, nil, map[string]any{"model": "unknown-model", "prompt": "hello"}); err == nil {
		t.Fatal("unregistered catalog model was accepted through the operation schema fallback")
	}
	for name, input := range map[string]map[string]any{
		"unknown": {"model": "test-model", "prompt": "hello", "unknown": true},
		"empty":   {"model": "test-model", "prompt": ""},
		"custom":  {"model": "test-model", "prompt": "hello", "blocked": true},
	} {
		t.Run(name, func(t *testing.T) {
			if err := registry.ValidateInput(context.Background(), "test-capability", "responses", "test-model", nil, nil, input); err == nil {
				t.Fatal("invalid input was accepted")
			}
		})
	}
	if err := registry.ValidateOutput(context.Background(), "test-capability", "responses", "test-model", nil, nil, map[string]any{"id": "response-1", "new_field": true}); err != nil {
		t.Fatalf("compatible output rejected: %v", err)
	}
	if err := registry.ValidateOutput(context.Background(), "test-capability", "responses", "test-model", nil, nil, map[string]any{"new_field": true}); err == nil {
		t.Fatal("output without required id was accepted")
	}
}

func TestProviderCapabilitySchemaRejectsUnknownValidator(t *testing.T) {
	registration := testRegistration()
	registration.ProviderCapabilities[0].Operations[0].InputSchema = map[string]any{
		"type": "object", "additionalProperties": false, validatorIDsExtension: []any{"missing.validator"},
	}
	registration.ProviderCapabilities[0].Variants[0].InputSchema = nil
	runtime, err := NewRuntimeRegistry([]Registration{registration})
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewProviderCapabilityRegistry([]Registration{registration}, runtime)
	if err == nil || !strings.Contains(err.Error(), "missing.validator") {
		t.Fatalf("err = %v, want missing validator", err)
	}
}

func testRegistration() Registration {
	definition := iapiserver.CapabilityDefinition{ID: "text.responses", NameI18n: map[string]string{"zh-CN": "统一响应", "en-US": "Responses"}, InputMediaTypes: []string{"text"}, OutputMediaTypes: []string{"text"}}
	return Registration{
		CapabilityDefinitions: []iapiserver.CapabilityDefinition{definition},
		EngineAdapter:         iapiserver.EngineAdapterDefinition{ID: "test", Responsibilities: []string{"authentication"}},
		OperationExecutors:    []iapiserver.OperationExecutorDefinition{{ID: "test-responses", EngineAdapterID: "test", CapabilityDefinitionIDs: []string{definition.ID}}},
		ProviderType: &ProviderTypeRegistration{
			ID: "test", DisplayName: "Test Provider",
			AuthenticationTypes:    []string{iapiserver.EngineAuthAPIKey},
			ConfigurationSchema:    map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{}},
			SupportsModelDiscovery: true, SupportsModelProbe: true, AdapterID: "test",
			OperationExecutors: map[string]string{definition.ID: "test-responses"},
		},
		EngineType:           iapiserver.ApplicationEngineType{ID: "test", NameI18n: map[string]string{"zh-CN": "测试", "en-US": "Test"}, DescriptionI18n: map[string]string{"zh-CN": "测试服务。", "en-US": "Test service."}, OfficialWebsiteURL: "https://example.com/", OfficialDocumentationURL: "https://example.com/docs", DefaultAPIBaseURL: "https://api.example.com/v1", Enabled: true, EngineAdapterID: "test", AuthenticationTypes: []string{iapiserver.EngineAuthAPIKey}, AuthenticationConfigSchema: AuthenticationConfigSchema(iapiserver.EngineAuthAPIKey), OperationExecutors: map[string]string{definition.ID: "test-responses"}},
		ProviderCapabilities: []iapiserver.AIAppProviderCapability{{SchemaVersion: "1.0", ID: "test-capability", NameI18n: map[string]string{"zh-CN": "测试能力", "en-US": "Test Capability"}, DescriptionI18n: map[string]string{"zh-CN": "测试能力说明。", "en-US": "Test capability description."}, Kind: iapiserver.ProviderCapabilityKindCatalog, Origin: iapiserver.ProviderCapabilityOriginStatic, BindingPolicy: iapiserver.ProviderBindingPolicyManual, ApplicationEngineTypeID: "test", Revision: "1", Enabled: true, Provider: map[string]any{"code": "test", "official_website": "https://example.com/"}, Sources: []map[string]any{{"url": "https://example.com/docs"}}, Models: []iapiserver.ProviderCapabilityModel{{ID: "test-model", ProviderModelID: "test-model", DisplayNameI18n: map[string]string{"zh-CN": "测试模型", "en-US": "Test Model"}, DescriptionI18n: map[string]string{"zh-CN": "测试模型说明。", "en-US": "Test model description."}, Family: "test", Variant: "default", Lifecycle: iapiserver.ProviderLifecycle{Status: "active"}}}, Operations: []iapiserver.ProviderCapabilityOperation{{ID: "responses", CapabilityDefinitionID: definition.ID, NameI18n: map[string]string{"zh-CN": "统一响应", "en-US": "Responses"}, DescriptionI18n: map[string]string{"zh-CN": "响应说明。", "en-US": "Response description."}, ExecutionMode: "synchronous", InputMediaTypes: []string{"text"}, OutputMediaTypes: []string{"text"}}}, Variants: []iapiserver.ProviderCapabilityVariant{{ID: "test-model-responses", ModelID: "test-model", OperationID: "responses", Lifecycle: iapiserver.ProviderLifecycle{Status: "active"}, InputSchema: map[string]any{"type": "object"}, OutputSchema: map[string]any{"type": "object"}}}}},
	}
}
