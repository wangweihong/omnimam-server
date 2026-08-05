package usermodel

import (
	"context"
	"slices"
	"testing"
	"time"

	toolerrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type testFactory struct {
	store.Factory
	providers store.ProviderStore
	models    store.ProviderModelStore
	defaults  store.SystemLLMConfigStore
	health    store.ModelHealthCheckStore
}

func (f *testFactory) Providers() store.ProviderStore                 { return f.providers }
func (f *testFactory) ProviderModels() store.ProviderModelStore       { return f.models }
func (f *testFactory) SystemLLMConfigs() store.SystemLLMConfigStore   { return f.defaults }
func (f *testFactory) ModelHealthChecks() store.ModelHealthCheckStore { return f.health }

type testProviderStore struct {
	store.ProviderStore
	provider *iapiserver.Provider
}

func (s *testProviderStore) GetOwned(_ context.Context, owner, id string) (*iapiserver.Provider, error) {
	if s.provider == nil || s.provider.OwnerUserID != owner || s.provider.ID != id {
		return nil, toolerrors.New("not found")
	}
	return s.provider, nil
}

type testModelStore struct {
	store.ProviderModelStore
	items []*iapiserver.ProviderModel
	added []*iapiserver.ProviderModel
}

func (s *testModelStore) List(_ context.Context, req *iapiserver.ProviderModelListRequest) ([]*iapiserver.ProviderModel, int64, error) {
	items := make([]*iapiserver.ProviderModel, 0, len(s.items))
	for _, item := range s.items {
		if (req.OwnerUserID == "" || item.OwnerUserID == req.OwnerUserID) && (req.ProviderID == "" || item.ProviderID == req.ProviderID) {
			items = append(items, item)
		}
	}
	return items, int64(len(items)), nil
}

func (s *testModelStore) GetOwned(_ context.Context, owner, id string) (*iapiserver.ProviderModel, error) {
	for _, item := range s.items {
		if item.OwnerUserID == owner && item.ID == id {
			return item, nil
		}
	}
	return nil, toolerrors.New("not found")
}

func (s *testModelStore) Add(_ context.Context, model *iapiserver.ProviderModel) (*iapiserver.ProviderModel, error) {
	model.ID = "created-" + model.Model
	s.added = append(s.added, model)
	s.items = append(s.items, model)
	return model, nil
}

type testGateway struct {
	discovered []modelgateway.DiscoveredModel
	request    modelgateway.ProviderConnectionRequest
}

func (*testGateway) ListProviderTypes(context.Context) ([]*iapiserver.ProviderType, error) {
	return []*iapiserver.ProviderType{{ID: iapiserver.ProviderTypeOpenAICompatible, AuthenticationTypes: []string{iapiserver.EngineAuthAPIKey}}}, nil
}

func (*testGateway) TestProviderConnection(context.Context, modelgateway.ProviderConnectionRequest) (*modelgateway.ProviderProbeResult, error) {
	return &modelgateway.ProviderProbeResult{Success: true}, nil
}

func (g *testGateway) DiscoverProviderModels(_ context.Context, request modelgateway.ProviderConnectionRequest) ([]modelgateway.DiscoveredModel, error) {
	g.request = request
	return g.discovered, nil
}

func (*testGateway) ProbeProviderModel(context.Context, modelgateway.ProviderConnectionRequest, string) (*modelgateway.ModelProbeResult, error) {
	return &modelgateway.ModelProbeResult{Available: true}, nil
}

func (*testGateway) ResolveUserModelCapabilities(_ string, probe *modelgateway.ModelProbeResult, disabled []string) (*modelgateway.CapabilityResolution, error) {
	capabilities := []string{CapabilityTextChatCompletion, CapabilityTextTranslate}
	for _, id := range disabled {
		capabilities = slices.DeleteFunc(capabilities, func(candidate string) bool { return candidate == id })
	}
	return &modelgateway.CapabilityResolution{
		CapabilityDefinitionIDs: capabilities, StreamSupported: probe.StreamSupported,
		Executable: len(capabilities) > 0, Status: modelgateway.CapabilityResolutionResolved,
	}, nil
}

func (*testGateway) ExecuteOperation(context.Context, modelgateway.OperationExecutionRequest) (*modelgateway.OperationExecutionResult, error) {
	return &modelgateway.OperationExecutionResult{}, nil
}

func TestCredentialBrokerScopesOpaqueHandles(t *testing.T) {
	broker := NewCredentialBroker(time.Minute)
	handle, err := broker.Issue(iapiserver.ProviderTypeOpenAICompatible, iapiserver.EngineAuthAPIKey, "secret-value")
	if err != nil {
		t.Fatalf("issue credential: %v", err)
	}
	if handle == "" || handle == "secret-value" {
		t.Fatalf("handle is not opaque: %q", handle)
	}
	if _, err := broker.ResolveCredential(context.Background(), modelgateway.CredentialResolveRequest{
		Handle: handle, ProviderType: iapiserver.ProviderTypeDeepSeekOfficial, AuthenticationType: iapiserver.EngineAuthAPIKey,
	}); toolerrors.ToStatus(err).Code != code.ErrAIAppEngineAuthConfigInvalid {
		t.Fatalf("scope mismatch code = %d", toolerrors.ToStatus(err).Code)
	}
	credential, err := broker.ResolveCredential(context.Background(), modelgateway.CredentialResolveRequest{
		Handle: handle, ProviderType: iapiserver.ProviderTypeOpenAICompatible, AuthenticationType: iapiserver.EngineAuthAPIKey,
	})
	if err != nil || credential.Authentication["api_key"] != "secret-value" {
		t.Fatalf("resolve credential = %#v, %v", credential, err)
	}
}

func TestSyncProviderModelsOnlyCreatesMissingModels(t *testing.T) {
	provider := &iapiserver.Provider{
		ObjectMeta:  imachinery.ObjectMeta{ID: "provider-1", Name: "Provider"},
		OwnerUserID: "user-1", Type: iapiserver.ProviderTypeOpenAICompatible,
		BaseURL: "https://example.test/v1", AuthType: iapiserver.EngineAuthAPIKey, CredentialRef: "secret-value", Enabled: true,
	}
	existing := &iapiserver.ProviderModel{
		ObjectMeta: imachinery.ObjectMeta{ID: "model-existing"}, OwnerUserID: "user-1", ProviderID: provider.ID,
		Model: "remote-existing", DisplayName: "User display name", GroupName: "Pinned group",
		FeatureLabels: []string{"favorite"}, DisabledCapabilityDefinitionIDs: []string{CapabilityTextTranslate}, Enabled: false,
	}
	models := &testModelStore{items: []*iapiserver.ProviderModel{existing}}
	gateway := &testGateway{discovered: []modelgateway.DiscoveredModel{
		{RemoteModel: "remote-existing", DisplayName: "Remote overwrite"},
		{RemoteModel: "remote-new", DisplayName: "Remote new"},
	}}
	service, err := New(Dependencies{
		Store:   &testFactory{providers: &testProviderStore{provider: provider}, models: models},
		Gateway: gateway, Credentials: NewCredentialBroker(time.Minute),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, &iapiserver.User{ObjectMeta: imachinery.ObjectMeta{ID: "user-1"}})
	result, err := service.SyncProviderModels(ctx, provider.ID)
	if err != nil {
		t.Fatalf("sync models: %v", err)
	}
	if result.Total != 2 || result.Created != 1 || result.Updated != 0 || result.Skipped != 1 || len(models.added) != 1 {
		t.Fatalf("unexpected sync result: %#v", result)
	}
	if existing.DisplayName != "User display name" || existing.GroupName != "Pinned group" || existing.Enabled || !slices.Equal(existing.FeatureLabels, []string{"favorite"}) {
		t.Fatalf("existing user fields were overwritten: %#v", existing)
	}
	if gateway.request.CredentialHandle == "" || gateway.request.CredentialHandle == provider.CredentialRef {
		t.Fatalf("gateway received non-opaque credential handle: %#v", gateway.request)
	}
}

func TestResolveUserModelExecutionContextIssuesEligibleNonSensitiveGrant(t *testing.T) {
	now := time.Now().UTC()
	checkedAt := imachinery.NewTime(now)
	provider := &iapiserver.Provider{
		ObjectMeta: imachinery.ObjectMeta{
			ID: "provider-1", Name: "Provider", UpdatedAt: imachinery.NewTime(now.Add(-time.Minute)), ResourceVersion: 4,
		},
		OwnerUserID: "user-1", Type: iapiserver.ProviderTypeOpenAICompatible,
		BaseURL: "https://example.test/v1", AuthType: iapiserver.EngineAuthAPIKey,
		CredentialRef: "secret-value", Config: map[string]any{}, Enabled: true,
	}
	model := &iapiserver.ProviderModel{
		ObjectMeta:  imachinery.ObjectMeta{ID: "model-1", ResourceVersion: 7},
		OwnerUserID: "user-1", ProviderID: provider.ID, Model: "remote-model", DisplayName: "Display model",
		Enabled: true, HealthStatus: iapiserver.ProviderModelHealthHealthy, HealthCheckedAt: &checkedAt,
	}
	service, err := New(Dependencies{
		Store:   &testFactory{providers: &testProviderStore{provider: provider}, models: &testModelStore{items: []*iapiserver.ProviderModel{model}}},
		Gateway: &testGateway{}, Credentials: NewCredentialBroker(time.Minute),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, &iapiserver.User{ObjectMeta: imachinery.ObjectMeta{ID: "user-1"}})
	grant, err := service.ResolveUserModelExecutionContext(ctx, ExecutionContextRequest{
		ModelID: model.ID, CapabilityDefinitionID: CapabilityTextChatCompletion,
		RequiredCapabilityDefinitionIDs: []string{CapabilityTextTranslate},
	})
	if err != nil {
		t.Fatalf("resolve execution context: %v", err)
	}
	if grant.OwnerUserID != "user-1" || grant.ModelID != model.ID || grant.RemoteModel != model.Model ||
		grant.ConfigVersion != 7 || grant.Issuer != modelgateway.UserModelExecutionContextIssuer || !grant.ExpiresAt.After(grant.IssuedAt) {
		t.Fatalf("unexpected execution context: %#v", grant)
	}
	if grant.CredentialHandle == "" || grant.CredentialHandle == provider.CredentialRef {
		t.Fatalf("credential handle is not opaque: %q", grant.CredentialHandle)
	}
	if _, exposed := grant.ModelSnapshot["endpoint"]; exposed {
		t.Fatalf("model snapshot exposed endpoint: %#v", grant.ModelSnapshot)
	}
	if _, exposed := grant.ModelSnapshot["credential_ref"]; exposed {
		t.Fatalf("model snapshot exposed credential: %#v", grant.ModelSnapshot)
	}
}

var _ modelgateway.UserModelGateway = (*testGateway)(nil)
