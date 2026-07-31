package engine

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type objectInfoStore struct {
	store.ApplicationPlatformStore
	engine  *iapiserver.EngineInstance
	catalog *iapiserver.ComfyUIEngineObjectInfo
}

func (s *objectInfoStore) GetEngineInstance(context.Context, string) (*iapiserver.EngineInstance, error) {
	if s.engine == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.engine, nil
}

func (s *objectInfoStore) GetComfyUIEngineObjectInfo(context.Context, string) (*iapiserver.ComfyUIEngineObjectInfo, error) {
	if s.catalog == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.catalog, nil
}

func (s *objectInfoStore) RefreshComfyUIEngineObjectInfo(_ context.Context, _ string, fetch func(*iapiserver.EngineInstance) (*iapiserver.ComfyUIEngineObjectInfo, error)) (*iapiserver.ComfyUIEngineObjectInfo, error) {
	catalog, err := fetch(s.engine)
	if err != nil {
		return nil, err
	}
	s.catalog = catalog
	return catalog, nil
}

type objectInfoAdapter struct {
	objectInfo map[string]any
	err        error
}

func (objectInfoAdapter) ID() string { return "comfyui" }
func (a objectInfoAdapter) Check(context.Context, *iapiserver.EngineInstance) (*iapiserver.EngineHealthCheckResult, error) {
	return nil, a.err
}
func (a objectInfoAdapter) ReadObjectInfo(context.Context, *iapiserver.EngineInstance) (map[string]any, error) {
	return a.objectInfo, a.err
}

type engineTestPrincipal struct{ principal modelgateway.Principal }

func (p engineTestPrincipal) Resolve(context.Context) (modelgateway.Principal, error) {
	return p.principal, nil
}

func newObjectInfoTestService(t *testing.T, storage *objectInfoStore, adapter Adapter, admin bool) *Service {
	t.Helper()
	runtime, err := appregistry.LoadRuntimeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	adapters := map[string]Adapter{}
	if adapter != nil {
		adapters["comfyui"] = adapter
	}
	return &Service{store: &engineTestFactory{applications: storage}, runtime: runtime, adapters: adapters, principals: engineTestPrincipal{principal: modelgateway.Principal{UserID: "user-1", Admin: admin}}}
}

func onlineComfyEngine() *iapiserver.EngineInstance {
	engine := &iapiserver.EngineInstance{ApplicationEngineTypeID: "comfyui", Enabled: true, HealthStatus: iapiserver.EngineHealthOnline}
	engine.ID = "engine-1"
	return engine
}

func validObjectInfo() map[string]any {
	return map[string]any{"KSampler": map[string]any{"input": map[string]any{}, "output": []any{}}}
}

func TestRefreshComfyUIObjectInfoReplacesOnlyAfterSuccessfulValidation(t *testing.T) {
	old := &iapiserver.ComfyUIEngineObjectInfo{EngineInstanceID: "engine-1", ObjectInfo: map[string]any{"Old": map[string]any{}}, RefreshedAt: imachinery.NewTime(time.Now().Add(-time.Hour))}
	storage := &objectInfoStore{engine: onlineComfyEngine(), catalog: old}
	service := newObjectInfoTestService(t, storage, objectInfoAdapter{objectInfo: validObjectInfo()}, true)

	status, err := service.RefreshComfyUIEngineObjectInfo(context.Background(), "engine-1")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Available || status.Stale || storage.catalog == old || storage.catalog.ObjectInfo["KSampler"] == nil {
		t.Fatalf("unexpected refresh result: status=%#v catalog=%#v", status, storage.catalog)
	}

	service.adapters["comfyui"] = objectInfoAdapter{err: stderrors.New("offline")}
	current := storage.catalog
	_, err = service.RefreshComfyUIEngineObjectInfo(context.Background(), "engine-1")
	if errors.ToStatus(err).Code != code.ErrAIAppComfyUIObjectInfoRefreshFailed {
		t.Fatalf("unexpected error: %v", err)
	}
	if storage.catalog != current {
		t.Fatal("failed refresh replaced the last successful catalog")
	}
}

func TestComfyUIObjectInfoEligibilityAndFreshness(t *testing.T) {
	engine := onlineComfyEngine()
	engine.Enabled = false
	storage := &objectInfoStore{engine: engine}
	service := newObjectInfoTestService(t, storage, objectInfoAdapter{objectInfo: validObjectInfo()}, true)
	_, err := service.RefreshComfyUIEngineObjectInfo(context.Background(), engine.ID)
	if errors.ToStatus(err).Code != code.ErrAIAppComfyUIObjectInfoRefreshNotAllowed {
		t.Fatalf("unexpected refresh eligibility error: %v", err)
	}

	engine.Enabled = true
	storage.catalog = &iapiserver.ComfyUIEngineObjectInfo{EngineInstanceID: engine.ID, ObjectInfo: validObjectInfo(), RefreshedAt: imachinery.NewTime(time.Now().Add(-49 * time.Hour))}
	response, err := service.GetComfyUIEngineObjectInfo(context.Background(), engine.ID)
	if err != nil || !response.Stale {
		t.Fatalf("stale catalog should remain readable: response=%#v err=%v", response, err)
	}
	if _, _, err := service.ResolveUsableComfyUIObjectInfo(context.Background(), engine.ID); errors.ToStatus(err).Code != code.ErrAIAppComfyUIObjectInfoUnavailable {
		t.Fatalf("stale catalog was accepted for execution: %v", err)
	}
}

func TestValidateCurrentObjectInfoRejectsPartialCatalog(t *testing.T) {
	if err := validateCurrentObjectInfo(map[string]any{"KSampler": map[string]any{"input": map[string]any{}}}); err == nil {
		t.Fatal("partial object_info was accepted")
	}
}
