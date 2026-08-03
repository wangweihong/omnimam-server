package modelgateway

import (
	"context"
	stderrors "errors"
	"fmt"
	"testing"

	toolerrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	identitymiddleware "github.com/wangweihong/omnimam/backend/internal/apiserver/middleware"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type systemBindingFactory struct {
	store.Factory
	applications store.ApplicationPlatformStore
}

func (f *systemBindingFactory) ApplicationPlatforms() store.ApplicationPlatformStore {
	return f.applications
}

type principalResolverFactory struct {
	store.Factory
	identity store.IdentityStore
}

func (f *principalResolverFactory) Identities() store.IdentityStore { return f.identity }

type principalResolverIdentityStore struct {
	store.IdentityStore
	hasRole   bool
	roleCodes []string
}

func (s *principalResolverIdentityStore) UserHasAnyRole(_ context.Context, _ string, roleCodes []string) (bool, error) {
	s.roleCodes = append([]string(nil), roleCodes...)
	return s.hasRole, nil
}

type systemBindingStore struct {
	store.ApplicationPlatformStore
	engine        *iapiserver.EngineInstance
	bindings      []*iapiserver.EngineCapabilityBinding
	addErr        error
	ensureErr     error
	ensured       []requiredBindingCall
	storedBinding *iapiserver.EngineCapabilityBinding
}

type engineTestPrincipal struct{ principal Principal }

func (p engineTestPrincipal) Resolve(context.Context) (Principal, error) {
	return p.principal, nil
}

type requiredBindingCall struct {
	engineTypeID, capabilityID, revision, name, description string
}

func (s *systemBindingStore) AddEngineInstanceWithBindings(_ context.Context, engine *iapiserver.EngineInstance, bindings []*iapiserver.EngineCapabilityBinding) (*iapiserver.EngineInstance, error) {
	s.engine = engine
	s.bindings = bindings
	return engine, s.addErr
}

func (s *systemBindingStore) EnsureRequiredEngineBindings(_ context.Context, engineTypeID, capabilityID, revision, name, description string) error {
	s.ensured = append(s.ensured, requiredBindingCall{engineTypeID: engineTypeID, capabilityID: capabilityID, revision: revision, name: name, description: description})
	return s.ensureErr
}

func (s *systemBindingStore) GetEngineBinding(context.Context, string) (*iapiserver.EngineCapabilityBinding, error) {
	if s.storedBinding == nil {
		return nil, stderrors.New("binding not found")
	}
	return s.storedBinding, nil
}

func (s *systemBindingStore) GetEngineInstance(context.Context, string) (*iapiserver.EngineInstance, error) {
	return &iapiserver.EngineInstance{ApplicationEngineTypeID: "comfyui"}, nil
}

func newSystemBindingService(t *testing.T, storage *systemBindingStore) *EngineService {
	t.Helper()
	runtime, capabilities := newGatewayTestRegistries(t)
	service, err := NewEngineService(EngineDependencies{
		Store:        &systemBindingFactory{applications: storage},
		Runtime:      runtime,
		Capabilities: capabilities,
		Principals:   engineTestPrincipal{principal: Principal{UserID: "admin", Admin: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestStorePrincipalResolverUsesIdentityRoles(t *testing.T) {
	identityStore := &principalResolverIdentityStore{hasRole: true}
	resolver := NewStorePrincipalResolver(&principalResolverFactory{identity: identityStore})
	user := &iapiserver.User{}
	user.ID = "admin-1"
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, user)
	ctx = context.WithValue(ctx, identitymiddleware.IdentityPrincipalContextKey, identitymiddleware.IdentityPrincipal{
		PrincipalID: "admin-1",
		Permissions: map[string]struct{}{"aiapp.application.manage_global": {}},
	})
	principal, err := resolver.Resolve(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !principal.Admin || !principal.HasPermission("aiapp.application.manage_global") {
		t.Fatalf("unexpected principal: %#v", principal)
	}
	wantRoleCodes := []string{"ADMIN", "SUPER_ADMIN"}
	if fmt.Sprint(identityStore.roleCodes) != fmt.Sprint(wantRoleCodes) {
		t.Fatalf("role codes=%v, want %v", identityStore.roleCodes, wantRoleCodes)
	}
}

func TestCreateComfyUIEngineIncludesRequiredSystemBinding(t *testing.T) {
	storage := &systemBindingStore{}
	service := newSystemBindingService(t, storage)
	enabled := true
	engine, err := service.CreateEngineInstance(context.Background(), &iapiserver.EngineInstanceCreateRequest{
		Name: "comfy", ApplicationEngineTypeID: "comfyui", BaseURL: "http://127.0.0.1:8188", AuthType: "none", Enabled: &enabled, MaxConcurrency: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if engine != storage.engine || len(storage.bindings) != 1 {
		t.Fatalf("engine=%#v bindings=%#v", engine, storage.bindings)
	}
	binding := storage.bindings[0]
	if binding.ProviderCapabilityID != "comfyui-workflow-runtime" || !binding.Enabled || !binding.SystemManaged || len(binding.Restrictions) != 0 {
		t.Fatalf("unexpected required binding: %#v", binding)
	}
}

func TestCreateComfyUIEngineMapsAtomicBindingFailure(t *testing.T) {
	storage := &systemBindingStore{addErr: fmt.Errorf("%w: controlled failure", store.ErrRequiredEngineBindingFailed)}
	service := newSystemBindingService(t, storage)
	enabled := true
	_, err := service.CreateEngineInstance(context.Background(), &iapiserver.EngineInstanceCreateRequest{
		Name: "comfy", ApplicationEngineTypeID: "comfyui", BaseURL: "http://127.0.0.1:8188", AuthType: "none", Enabled: &enabled, MaxConcurrency: 1,
	})
	if status := toolerrors.ToStatus(err); status.Code != code.ErrAIAppRequiredEngineBindingFailed {
		t.Fatalf("status=%#v, want required binding failure", status)
	}
}

func TestCreateNonComfyUIEngineDoesNotAddRequiredBindings(t *testing.T) {
	storage := &systemBindingStore{}
	service := newSystemBindingService(t, storage)
	enabled := true
	_, err := service.CreateEngineInstance(context.Background(), &iapiserver.EngineInstanceCreateRequest{
		Name: "deepseek", ApplicationEngineTypeID: "deepseek_official", BaseURL: "https://api.deepseek.com", AuthType: "api_key", AuthConfig: map[string]any{"api_key": "secret"}, Enabled: &enabled, MaxConcurrency: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(storage.bindings) != 0 {
		t.Fatalf("non-ComfyUI engine received required bindings: %#v", storage.bindings)
	}
}

func TestCreateEngineDoesNotMisclassifyParentInsertFailure(t *testing.T) {
	parentErr := stderrors.New("create required engine binding: parent insert failure")
	storage := &systemBindingStore{addErr: parentErr}
	service := newSystemBindingService(t, storage)
	enabled := true
	_, err := service.CreateEngineInstance(context.Background(), &iapiserver.EngineInstanceCreateRequest{
		Name: "comfy", ApplicationEngineTypeID: "comfyui", BaseURL: "http://127.0.0.1:8188", AuthType: "none", Enabled: &enabled, MaxConcurrency: 1,
	})
	if !stderrors.Is(err, parentErr) {
		t.Fatalf("error=%v, want parent insert failure", err)
	}
}

func TestReconcileRequiredEngineBindingsUsesStaticCapability(t *testing.T) {
	storage := &systemBindingStore{}
	service := newSystemBindingService(t, storage)
	if err := service.ReconcileRequiredEngineBindings(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(storage.ensured) != 1 {
		t.Fatalf("ensure calls=%#v", storage.ensured)
	}
	call := storage.ensured[0]
	if call.engineTypeID != "comfyui" || call.capabilityID != "comfyui-workflow-runtime" || call.revision != "1" {
		t.Fatalf("unexpected ensure call: %#v", call)
	}
}

func TestReconcileRequiredEngineBindingsPropagatesStoreFailure(t *testing.T) {
	ensureErr := stderrors.New("controlled reconcile failure")
	storage := &systemBindingStore{ensureErr: ensureErr}
	service := newSystemBindingService(t, storage)
	err := service.ReconcileRequiredEngineBindings(context.Background())
	if !stderrors.Is(err, ensureErr) {
		t.Fatalf("error=%v, want reconcile store failure", err)
	}
}

func TestSystemEngineBindingIsImmutable(t *testing.T) {
	storage := &systemBindingStore{storedBinding: &iapiserver.EngineCapabilityBinding{ProviderCapabilityID: "comfyui-workflow-runtime"}}
	service := newSystemBindingService(t, storage)

	_, updateErr := service.UpdateEngineBinding(context.Background(), &iapiserver.EngineCapabilityBindingUpdateRequest{ID: "binding-1", ResourceVersion: 1})
	if status := toolerrors.ToStatus(updateErr); status.Code != code.ErrAIAppSystemEngineBindingImmutable {
		t.Fatalf("update status=%#v", status)
	}
	_, deleteErr := service.DeleteEngineBinding(context.Background(), "binding-1")
	if status := toolerrors.ToStatus(deleteErr); status.Code != code.ErrAIAppSystemEngineBindingImmutable {
		t.Fatalf("delete status=%#v", status)
	}

	enabled := true
	_, createErr := service.CreateEngineBinding(context.Background(), &iapiserver.EngineCapabilityBindingCreateRequest{
		Name: "manual", EngineInstanceID: "engine-1", ProviderCapabilityID: "comfyui-workflow-runtime", Enabled: &enabled, Restrictions: map[string]any{},
	})
	if status := toolerrors.ToStatus(createErr); status.Code != code.ErrAIAppSystemEngineBindingImmutable {
		t.Fatalf("create status=%#v", status)
	}

	service.ResolveBindingStatus(storage.storedBinding)
	if !storage.storedBinding.SystemManaged {
		t.Fatal("system binding was not marked system_managed")
	}
}
