package platform

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"k8s.io/gengo/examples/set-gen/sets"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

func TestDefaultPermissionsIncludeComfyUIWorkflowContract(t *testing.T) {
	permissions := sets.NewString(defaultPermissions()...)
	for _, key := range []string{iapiserver.AIAppEngineInstanceRead, iapiserver.AIAppComfyUIWorkflowRead, iapiserver.AIAppComfyUIWorkflowManage, iapiserver.AIAppComfyUIWorkflowValidate, iapiserver.AIAppComfyUIWorkflowConvert, iapiserver.AIAppComfyUIWorkflowTest} {
		if !permissions.Has(key) {
			t.Fatalf("default permissions missing %s", key)
		}
	}
}

func TestGetProviderModelRefSummariesBatchesAndScopesByOwner(t *testing.T) {
	providerStore := &testProviderStore{items: map[string]*iapiserver.Provider{
		"provider-a": {ObjectMeta: imachinery.ObjectMeta{ID: "provider-a", Name: "OpenAI A"}, OwnerUserID: "user-a", Enabled: true},
		"provider-b": {ObjectMeta: imachinery.ObjectMeta{ID: "provider-b", Name: "OpenAI B"}, OwnerUserID: "user-b", Enabled: true},
	}}
	modelStore := &testProviderModelStore{items: map[string]*iapiserver.ProviderModel{
		"model-a": {ObjectMeta: imachinery.ObjectMeta{ID: "model-a"}, OwnerUserID: "user-a", ProviderID: "provider-a", Model: "gpt-a", DisplayName: "GPT A", HealthStatus: iapiserver.ProviderModelHealthHealthy, Enabled: true},
		"model-b": {ObjectMeta: imachinery.ObjectMeta{ID: "model-b"}, OwnerUserID: "user-b", ProviderID: "provider-b", Model: "gpt-b", DisplayName: "GPT B", HealthStatus: iapiserver.ProviderModelHealthHealthy, Enabled: true},
	}}
	service := NewService(&testFactory{providers: providerStore, models: modelStore})

	summaries, err := service.GetProviderModelRefSummaries(context.Background(), "user-a", []string{"model-a", "model-a", "model-b", "missing"})
	if err != nil {
		t.Fatalf("get summaries: %v", err)
	}
	if len(summaries) != 1 || summaries["model-a"] == nil {
		t.Fatalf("summaries = %#v", summaries)
	}
	if summaries["model-a"].ProviderName != "OpenAI A" || summaries["model-a"].DisplayName != "GPT A" {
		t.Fatalf("summary = %#v", summaries["model-a"])
	}
	if modelStore.getByIDsCalls != 1 || providerStore.getByIDsCalls != 1 {
		t.Fatalf("batch calls models=%d providers=%d", modelStore.getByIDsCalls, providerStore.getByIDsCalls)
	}
	if modelStore.lastGetByIDsUID != "user-a" || providerStore.lastGetByIDsUID != "user-a" {
		t.Fatalf("owner scope models=%q providers=%q", modelStore.lastGetByIDsUID, providerStore.lastGetByIDsUID)
	}
}

func TestProviderModelListAddsProviderNamesInOneBatch(t *testing.T) {
	providerStore := &testProviderStore{items: map[string]*iapiserver.Provider{
		"provider-a": {ObjectMeta: imachinery.ObjectMeta{ID: "provider-a", Name: "OpenAI A"}, OwnerUserID: "system-admin", Enabled: true},
	}}
	modelStore := &testProviderModelStore{items: map[string]*iapiserver.ProviderModel{}}
	for i := 0; i < 50; i++ {
		id := fmt.Sprintf("model-%02d", i)
		modelStore.items[id] = &iapiserver.ProviderModel{ObjectMeta: imachinery.ObjectMeta{ID: id}, OwnerUserID: "system-admin", ProviderID: "provider-a", Model: id, DisplayName: id, Enabled: true}
	}
	service := NewService(&testFactory{providers: providerStore, models: modelStore})

	response, err := service.ProviderModelList(context.Background(), &iapiserver.ProviderModelListRequest{})
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	if len(response.Items) != 50 || response.Items[0].ProviderName != "OpenAI A" {
		t.Fatalf("models = %#v", response.Items)
	}
	if providerStore.getByIDsCalls != 1 || providerStore.lastGetByIDsUID != "system-admin" {
		t.Fatalf("provider batches=%d owner=%q", providerStore.getByIDsCalls, providerStore.lastGetByIDsUID)
	}
}

func TestParseNaturalAssetQueryImageSize(t *testing.T) {
	query := parseNaturalAssetQuery("搜索 1920x1680 的赛博朋克图片")
	if query.MediaType != iapiserver.AssetMediaTypeImage {
		t.Fatalf("media type = %s", query.MediaType)
	}
	if query.Width != 1920 || query.Height != 1680 {
		t.Fatalf("size = %dx%d", query.Width, query.Height)
	}
}

func TestParseNaturalAssetQueryPromptTemplate(t *testing.T) {
	query := parseNaturalAssetQuery("ideogram4 提示词模板")
	if query.MediaType != iapiserver.AssetMediaTypePromptTemplate {
		t.Fatalf("media type = %s", query.MediaType)
	}
}

func TestParseNaturalAssetQueryDeletedStatus(t *testing.T) {
	query := parseNaturalAssetQuery("搜索回收站里已删除的图片")
	if query.MediaType != iapiserver.AssetMediaTypeImage {
		t.Fatalf("media type = %s", query.MediaType)
	}
	if query.Status != "deleted" {
		t.Fatalf("status = %q", query.Status)
	}
}

func TestAssetListRequestPostBindSplitsTags(t *testing.T) {
	query := &iapiserver.AssetListRequest{
		BasicQueryParam: imachinery.BasicQueryParam{SearchFields: []string{"name,description"}},
		Tags:            []string{"portrait, training；lora"},
		Status:          " Deleted ",
	}
	if err := query.PostBind(); err != nil {
		t.Fatalf("post bind: %v", err)
	}
	if len(query.Tags) != 3 || query.Tags[0] != "portrait" || query.Tags[2] != "lora" {
		t.Fatalf("tags = %#v", query.Tags)
	}
	if len(query.SearchFields) != 2 || query.SearchFields[1] != "description" {
		t.Fatalf("search fields = %#v", query.SearchFields)
	}
	if query.Status != "deleted" {
		t.Fatalf("status = %q", query.Status)
	}
}

func TestLocalObjectPathRejectsEscape(t *testing.T) {
	backend := &iapiserver.StorageBackend{Type: iapiserver.StorageBackendTypeLocal, Root: t.TempDir()}
	if _, err := localObjectPath(backend, "../secret.txt"); err == nil {
		t.Fatal("expected path escape to be rejected")
	}
	if _, err := localObjectPath(backend, "assets/file.txt"); err != nil {
		t.Fatalf("expected safe path: %v", err)
	}
}

func TestCreateAssetFromReaderRemovesObjectWhenPersistenceFails(t *testing.T) {
	root := t.TempDir()
	factory := &testFactory{
		storage: &testStorageBackendStore{item: &iapiserver.StorageBackend{
			Type: iapiserver.StorageBackendTypeLocal,
			Root: root,
		}},
		assets: &testFailingAssetStore{},
	}
	svc := &platformService{store: factory}

	_, err := svc.createAssetFromReader(
		context.Background(),
		bytes.NewBufferString("complete upload"),
		"image.png",
		nil,
		iapiserver.AssetSourceUserUpload,
	)
	if err == nil {
		t.Fatal("expected persistence failure")
	}

	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			t.Fatalf("orphaned upload object: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk storage root: %v", err)
	}
}

func TestFetchOpenAICompatibleModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"z-model"},{"id":"a-model"}]}`))
	}))
	defer server.Close()

	models, err := fetchOpenAICompatibleModels(context.Background(), &iapiserver.Provider{
		Type:          iapiserver.ProviderTypeOpenAICompatible,
		BaseURL:       server.URL,
		CredentialRef: "secret,backup",
	})
	if err != nil {
		t.Fatalf("fetch models: %v", err)
	}
	if len(models) != 2 || models[0] != "a-model" || models[1] != "z-model" {
		t.Fatalf("models = %#v", models)
	}
}

func TestFetchOpenAICompatibleModelsUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := fetchOpenAICompatibleModels(context.Background(), &iapiserver.Provider{
		Type:    iapiserver.ProviderTypeOpenAICompatible,
		BaseURL: server.URL,
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
	if status := errors.ToStatus(err); status.Code != code.ErrProviderUnauthorized ||
		status.HTTPStatus != http.StatusUnauthorized {
		t.Fatalf("status = %#v", status)
	}
}

func TestApplyPresetModelDefaults(t *testing.T) {
	provider := &iapiserver.Provider{PresetKey: "qwen"}
	provider.Name = "通义千问"

	tests := []struct {
		name         string
		model        string
		modelType    string
		endpointType string
	}{
		{name: "vision model", model: "qwen-vl-max", modelType: "vision", endpointType: "chat"},
		{name: "reasoning model", model: "qwq-plus", modelType: "reasoning", endpointType: "chat"},
		{name: "embedding model", model: "text-embedding-v3", modelType: "embedding", endpointType: "embeddings"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &iapiserver.ProviderModel{Model: tt.model}
			if !applyPresetModelDefaults(provider, model) {
				t.Fatal("expected defaults to change model")
			}
			if model.GroupName != "qwen" {
				t.Fatalf("group name = %q", model.GroupName)
			}
			if model.EndpointType != tt.endpointType {
				t.Fatalf("endpoint type = %q", model.EndpointType)
			}
			if !sets.NewString(model.ModelTypes...).Has(tt.modelType) {
				t.Fatalf("model types = %#v", model.ModelTypes)
			}
		})
	}
}

func TestProviderCreateRejectsDuplicateName(t *testing.T) {
	svc := newTestPlatformService()
	ctx := context.Background()

	_, err := svc.ProviderCreate(ctx, &iapiserver.ProviderCreateRequest{Name: "alpha", Type: iapiserver.ProviderTypeOpenAICompatible})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	_, err = svc.ProviderCreate(ctx, &iapiserver.ProviderCreateRequest{Name: "alpha", Type: iapiserver.ProviderTypeOpenAICompatible})
	if err == nil {
		t.Fatal("expected duplicate provider name to fail")
	}
	status := errors.ToStatus(err)
	if status.Code != code.ErrModelProviderNameDuplicated {
		t.Fatalf("status code = %d", status.Code)
	}
}

func TestProviderCreateHonorsDisabledRequest(t *testing.T) {
	svc := newTestPlatformService()
	ctx := context.Background()
	enabled := false

	provider, err := svc.ProviderCreate(ctx, &iapiserver.ProviderCreateRequest{
		Name:    "alpha",
		Type:    iapiserver.ProviderTypeOpenAICompatible,
		Enabled: &enabled,
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	if provider.Enabled {
		t.Fatal("expected provider to be disabled")
	}
}

func TestProviderUpdateAllowsSameNameButRejectsOtherProviderName(t *testing.T) {
	svc := newTestPlatformService()
	ctx := context.Background()

	first, err := svc.ProviderCreate(ctx, &iapiserver.ProviderCreateRequest{Name: "alpha", Type: iapiserver.ProviderTypeOpenAICompatible})
	if err != nil {
		t.Fatalf("create first provider: %v", err)
	}
	second, err := svc.ProviderCreate(ctx, &iapiserver.ProviderCreateRequest{Name: "beta", Type: iapiserver.ProviderTypeOpenAICompatible})
	if err != nil {
		t.Fatalf("create second provider: %v", err)
	}
	if _, err := svc.ProviderUpdate(ctx, &iapiserver.ProviderUpdateRequest{ID: first.ID, Name: &first.Name}); err != nil {
		t.Fatalf("update provider with same name: %v", err)
	}
	_, err = svc.ProviderUpdate(ctx, &iapiserver.ProviderUpdateRequest{ID: second.ID, Name: &first.Name})
	if err == nil {
		t.Fatal("expected duplicate provider name to fail")
	}
}

func TestProviderModelCreateRejectsDuplicateNameAndModelWithinProvider(t *testing.T) {
	svc := newTestPlatformService()
	ctx := context.Background()

	firstProvider, err := svc.ProviderCreate(ctx, &iapiserver.ProviderCreateRequest{Name: "alpha", Type: iapiserver.ProviderTypeOpenAICompatible})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	secondProvider, err := svc.ProviderCreate(ctx, &iapiserver.ProviderCreateRequest{Name: "beta", Type: iapiserver.ProviderTypeOpenAICompatible})
	if err != nil {
		t.Fatalf("create second provider: %v", err)
	}

	_, err = svc.ProviderModelCreate(ctx, &iapiserver.ProviderModelCreateRequest{
		ProviderID: firstProvider.ID,
		Name:       "chat-main",
		Model:      "gpt-main",
	})
	if err != nil {
		t.Fatalf("create provider model: %v", err)
	}
	_, err = svc.ProviderModelCreate(ctx, &iapiserver.ProviderModelCreateRequest{
		ProviderID: firstProvider.ID,
		Name:       "chat-main",
		Model:      "gpt-alt",
	})
	if err == nil {
		t.Fatal("expected duplicate model name to fail")
	}
	_, err = svc.ProviderModelCreate(ctx, &iapiserver.ProviderModelCreateRequest{
		ProviderID: firstProvider.ID,
		Name:       "chat-alt",
		Model:      "gpt-main",
	})
	if err == nil {
		t.Fatal("expected duplicate model identifier to fail")
	}
	if _, err := svc.ProviderModelCreate(ctx, &iapiserver.ProviderModelCreateRequest{
		ProviderID: secondProvider.ID,
		Name:       "chat-main",
		Model:      "gpt-main",
	}); err != nil {
		t.Fatalf("expected duplicate values in another provider to pass: %v", err)
	}
}

func TestProviderModelUpdateAllowsSameValuesButRejectsConflicts(t *testing.T) {
	svc := newTestPlatformService()
	ctx := context.Background()

	provider, err := svc.ProviderCreate(ctx, &iapiserver.ProviderCreateRequest{Name: "alpha", Type: iapiserver.ProviderTypeOpenAICompatible})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	first, err := svc.ProviderModelCreate(ctx, &iapiserver.ProviderModelCreateRequest{
		ProviderID: provider.ID,
		Name:       "chat-main",
		Model:      "gpt-main",
	})
	if err != nil {
		t.Fatalf("create first model: %v", err)
	}
	second, err := svc.ProviderModelCreate(ctx, &iapiserver.ProviderModelCreateRequest{
		ProviderID: provider.ID,
		Name:       "chat-alt",
		Model:      "gpt-alt",
	})
	if err != nil {
		t.Fatalf("create second model: %v", err)
	}
	if _, err := svc.ProviderModelUpdate(ctx, &iapiserver.ProviderModelUpdateRequest{
		ID:         first.ID,
		ProviderID: provider.ID,
		Name:       &first.Name,
		Model:      &first.Model,
	}); err != nil {
		t.Fatalf("update model with same values: %v", err)
	}
	_, err = svc.ProviderModelUpdate(ctx, &iapiserver.ProviderModelUpdateRequest{
		ID:         second.ID,
		ProviderID: provider.ID,
		Name:       &first.Name,
	})
	if err == nil {
		t.Fatal("expected duplicate model name to fail")
	}
	_, err = svc.ProviderModelUpdate(ctx, &iapiserver.ProviderModelUpdateRequest{
		ID:         second.ID,
		ProviderID: provider.ID,
		Model:      &first.Model,
	})
	if err == nil {
		t.Fatal("expected duplicate model identifier to fail")
	}
}

func TestProviderModelDeleteClearsDefaultModelBinding(t *testing.T) {
	svc := newTestPlatformService()
	ctx := context.Background()

	provider, err := svc.ProviderCreate(ctx, &iapiserver.ProviderCreateRequest{Name: "alpha", Type: iapiserver.ProviderTypeOpenAICompatible})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	model, err := svc.ProviderModelCreate(ctx, &iapiserver.ProviderModelCreateRequest{
		ProviderID: provider.ID,
		Name:       "chat-main",
		Model:      "gpt-main",
	})
	if err != nil {
		t.Fatalf("create model: %v", err)
	}
	if _, err := svc.SystemLLMConfigUpsert(ctx, &iapiserver.SystemLLMConfigUpsertRequest{Configs: []*iapiserver.SystemLLMConfigSpec{
		{
			Purpose:    "assistant.default",
			ProviderID: provider.ID,
			ModelID:    model.ID,
			Model:      model.Model,
		},
	}}); err != nil {
		t.Fatalf("upsert default model: %v", err)
	}

	deleted, err := svc.ProviderModelDelete(ctx, provider.ID, model.ID)
	if err != nil {
		t.Fatalf("delete model: %v", err)
	}
	if deleted.ID != model.ID {
		t.Fatalf("deleted model id = %s, want %s", deleted.ID, model.ID)
	}
	models, total, err := svc.store.ProviderModels().List(ctx, &iapiserver.ProviderModelListRequest{ProviderID: provider.ID})
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	if total != 0 || len(models) != 0 {
		t.Fatalf("models after delete = %d/%d, want 0/0", len(models), total)
	}
	configs, err := svc.SystemLLMConfigList(ctx)
	if err != nil {
		t.Fatalf("list configs: %v", err)
	}
	if len(configs.Configs) != 0 {
		t.Fatalf("configs after delete = %d, want 0", len(configs.Configs))
	}
}

func TestProviderModelDeleteRejectsWrongProvider(t *testing.T) {
	svc := newTestPlatformService()
	ctx := context.Background()

	firstProvider, err := svc.ProviderCreate(ctx, &iapiserver.ProviderCreateRequest{Name: "alpha", Type: iapiserver.ProviderTypeOpenAICompatible})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	secondProvider, err := svc.ProviderCreate(ctx, &iapiserver.ProviderCreateRequest{Name: "beta", Type: iapiserver.ProviderTypeOpenAICompatible})
	if err != nil {
		t.Fatalf("create second provider: %v", err)
	}
	model, err := svc.ProviderModelCreate(ctx, &iapiserver.ProviderModelCreateRequest{
		ProviderID: firstProvider.ID,
		Name:       "chat-main",
		Model:      "gpt-main",
	})
	if err != nil {
		t.Fatalf("create model: %v", err)
	}

	if _, err := svc.ProviderModelDelete(ctx, secondProvider.ID, model.ID); err == nil {
		t.Fatal("expected wrong provider delete to fail")
	}
	models, total, err := svc.store.ProviderModels().List(ctx, &iapiserver.ProviderModelListRequest{ProviderID: firstProvider.ID})
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	if total != 1 || len(models) != 1 {
		t.Fatalf("models after rejected delete = %d/%d, want 1/1", len(models), total)
	}
}

func TestProviderDeleteClearsModelsAndSystemConfig(t *testing.T) {
	svc := newTestPlatformService()
	ctx := context.Background()

	provider, err := svc.ProviderCreate(ctx, &iapiserver.ProviderCreateRequest{Name: "alpha", Type: iapiserver.ProviderTypeOpenAICompatible})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	model, err := svc.ProviderModelCreate(ctx, &iapiserver.ProviderModelCreateRequest{
		ProviderID: provider.ID,
		Name:       "chat-main",
		Model:      "gpt-main",
	})
	if err != nil {
		t.Fatalf("create model: %v", err)
	}
	if _, err := svc.SystemLLMConfigUpsert(ctx, &iapiserver.SystemLLMConfigUpsertRequest{
		Configs: []*iapiserver.SystemLLMConfigSpec{{
			Purpose:    "assistant.default",
			ProviderID: provider.ID,
			ModelID:    model.ID,
			Model:      model.Model,
		}},
	}); err != nil {
		t.Fatalf("upsert system config: %v", err)
	}

	if _, err := svc.ProviderDelete(ctx, provider.ID); err != nil {
		t.Fatalf("delete provider: %v", err)
	}

	if _, err := svc.store.Providers().Get(ctx, provider.ID); err == nil {
		t.Fatal("expected provider to be deleted")
	}
	models, _, err := svc.store.ProviderModels().List(ctx, &iapiserver.ProviderModelListRequest{ProviderID: provider.ID})
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("models not cleaned up: %#v", models)
	}
	configs, err := svc.store.SystemLLMConfigs().List(ctx)
	if err != nil {
		t.Fatalf("list configs: %v", err)
	}
	if len(configs) != 0 {
		t.Fatalf("configs not cleaned up: %#v", configs)
	}
}

func TestMeIncludesTaskCenterDefaultPermissions(t *testing.T) {
	svc := newTestPlatformService()
	resp, err := svc.Me(context.Background())
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	for _, permission := range []string{
		"task.definition.manage",
		"task.run.operate",
		"task.worker.protocol",
		"task.operation.admin",
	} {
		if !hasPermission(resp.Permissions, permission) {
			t.Fatalf("permissions missing %s: %#v", permission, resp.Permissions)
		}
	}
}

func TestMeIncludesReleasedFrontendPermissions(t *testing.T) {
	svc := newTestPlatformService()
	resp, err := svc.Me(context.Background())
	if err != nil {
		t.Fatalf("me: %v", err)
	}

	for _, permission := range []string{
		"asset.upload",
		"asset.label.manage",
		"asset.collection.read",
		"asset.collection.manage",
		"asset.artifact.read",
		"asset.artifact.register",
		"asset.content.read",
		"asset.representation.read",
		"asset.reference.read",
		"asset.storage.read",
		"asset.storage.manage",
		"task.atomic.operate",
		"task.group.operate",
		"task.schedule.manage",
		"sse.stream.read",
		"sse.history.read",
	} {
		if !hasPermission(resp.Permissions, permission) {
			t.Errorf("permissions missing %s", permission)
		}
	}
}

func TestMeDeduplicatesDatabasePermissions(t *testing.T) {
	svc := newTestPlatformService()
	factory := svc.store.(*testFactory)
	factory.permissions.items = []*iapiserver.Permission{
		{Key: "task.run.operate"},
		{Key: "custom.permission"},
	}

	resp, err := svc.Me(context.Background())
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	if countPermission(resp.Permissions, "task.run.operate") != 1 {
		t.Fatalf("task.run.operate should appear once: %#v", resp.Permissions)
	}
	if !hasPermission(resp.Permissions, "custom.permission") {
		t.Fatalf("custom permission missing: %#v", resp.Permissions)
	}
}

func newTestPlatformService() *platformService {
	return &platformService{
		store: &testFactory{
			providers:   &testProviderStore{items: map[string]*iapiserver.Provider{}},
			models:      &testProviderModelStore{items: map[string]*iapiserver.ProviderModel{}},
			configs:     &testSystemLLMConfigStore{items: map[string]*iapiserver.SystemLLMConfig{}},
			flags:       &testFeatureFlagStore{},
			permissions: &testPermissionStore{},
		},
	}
}

type testFactory struct {
	providers   *testProviderStore
	models      *testProviderModelStore
	configs     *testSystemLLMConfigStore
	flags       *testFeatureFlagStore
	permissions *testPermissionStore
	storage     store.StorageBackendStore
	assets      store.AssetStore
}

func (f *testFactory) IdentityProviders() store.IdentityProviderStore { return nil }
func (f *testFactory) ServiceProviders() store.ServiceProviderStore   { return nil }
func (f *testFactory) Settings() store.SettingStore                   { return nil }
func (f *testFactory) Users() store.UserStore                         { return nil }
func (f *testFactory) OneTimeTokens() store.OneTimeTokenStore         { return nil }
func (f *testFactory) UserOTPs() store.UserOTPStore                   { return nil }
func (f *testFactory) AssetLibraries() store.AssetLibraryStore        { return nil }
func (f *testFactory) AssetCategories() store.AssetCategoryStore      { return nil }
func (f *testFactory) AssetItems() store.AssetItemStore               { return nil }
func (f *testFactory) PromptLibraries() store.PromptLibraryStore      { return nil }
func (f *testFactory) PromptCategories() store.PromptCategoryStore    { return nil }
func (f *testFactory) PromptItems() store.PromptItemStore             { return nil }
func (f *testFactory) Projects() store.ProjectStore                   { return nil }
func (f *testFactory) Canvases() store.CanvasStore                    { return nil }
func (f *testFactory) WorkflowCanvases() store.WorkflowCanvasStore    { return nil }
func (f *testFactory) Providers() store.ProviderStore                 { return f.providers }
func (f *testFactory) ProviderModels() store.ProviderModelStore       { return f.models }
func (f *testFactory) ProviderCapabilities() store.ProviderCapabilityStore {
	return nil
}
func (f *testFactory) SystemLLMConfigs() store.SystemLLMConfigStore   { return f.configs }
func (f *testFactory) StorageBackends() store.StorageBackendStore     { return f.storage }
func (f *testFactory) AssetsV2() store.AssetStore                     { return f.assets }
func (f *testFactory) AssetsV1() store.AssetV1Store                   { return nil }
func (f *testFactory) AssetThumbnails() store.AssetThumbnailStore     { return nil }
func (f *testFactory) Tags() store.TagStore                           { return nil }
func (f *testFactory) AssetTags() store.AssetTagStore                 { return nil }
func (f *testFactory) AssetGroups() store.AssetGroupStore             { return nil }
func (f *testFactory) AssetGroupMembers() store.AssetGroupMemberStore { return nil }
func (f *testFactory) AssetRelations() store.AssetRelationStore       { return nil }
func (f *testFactory) TaskCenters() store.TaskCenterStore             { return nil }
func (f *testFactory) UserEvents() store.UserEventStore               { return nil }
func (f *testFactory) ApplicationPlatforms() store.ApplicationPlatformStore {
	return nil
}
func (f *testFactory) FeatureFlags() store.FeatureFlagStore { return f.flags }
func (f *testFactory) Roles() store.RoleStore               { return nil }
func (f *testFactory) Permissions() store.PermissionStore   { return f.permissions }
func (f *testFactory) UserRoles() store.UserRoleStore       { return nil }
func (f *testFactory) AIChat() store.AIChatStore            { return nil }
func (f *testFactory) EnsureScheme(metaTypes ...any) error  { return nil }
func (f *testFactory) Close() error                         { return nil }

type testStorageBackendStore struct {
	item *iapiserver.StorageBackend
}

func (s *testStorageBackendStore) GetBlob(context.Context, string) (*iapiserver.AssetBlob, error) {
	return nil, nil
}

func (s *testStorageBackendStore) List(context.Context, *iapiserver.StorageBackendListRequest) ([]*iapiserver.StorageBackend, int64, error) {
	return []*iapiserver.StorageBackend{s.item}, 1, nil
}

func (s *testStorageBackendStore) Get(context.Context, string) (*iapiserver.StorageBackend, error) {
	return s.item, nil
}

func (s *testStorageBackendStore) Add(_ context.Context, data *iapiserver.StorageBackend) (*iapiserver.StorageBackend, error) {
	s.item = data
	return data, nil
}

func (s *testStorageBackendStore) Update(_ context.Context, data *iapiserver.StorageBackend) (*iapiserver.StorageBackend, error) {
	s.item = data
	return data, nil
}

func (s *testStorageBackendStore) GetDefaultLocal(context.Context) (*iapiserver.StorageBackend, error) {
	return s.item, nil
}

type testFailingAssetStore struct{}

func (*testFailingAssetStore) List(context.Context, *iapiserver.AssetListRequest) ([]*iapiserver.Asset, int64, error) {
	return nil, 0, nil
}

func (*testFailingAssetStore) Get(context.Context, string) (*iapiserver.Asset, error) {
	return nil, errors.Errorf("not found")
}

func (*testFailingAssetStore) Add(context.Context, *iapiserver.Asset) (*iapiserver.Asset, error) {
	return nil, errors.Errorf("persistence failed")
}

func (*testFailingAssetStore) AddWithUploadEvent(context.Context, *iapiserver.Asset, *iapiserver.AssetThumbnail, map[string]any) (*iapiserver.Asset, *iapiserver.AssetThumbnail, error) {
	return nil, nil, errors.Errorf("outbox persistence failed")
}

func (*testFailingAssetStore) Update(context.Context, *iapiserver.Asset) (*iapiserver.Asset, error) {
	return nil, errors.Errorf("persistence failed")
}

func (*testFailingAssetStore) Delete(context.Context, string) error { return nil }

type testFeatureFlagStore struct {
	items []*iapiserver.FeatureFlag
}

func (s *testFeatureFlagStore) List(_ context.Context) ([]*iapiserver.FeatureFlag, error) {
	return s.items, nil
}

func (s *testFeatureFlagStore) Upsert(
	_ context.Context,
	data *iapiserver.FeatureFlag,
) (*iapiserver.FeatureFlag, error) {
	s.items = append(s.items, data)
	return data, nil
}

type testPermissionStore struct {
	items []*iapiserver.Permission
}

func (s *testPermissionStore) List(_ context.Context) ([]*iapiserver.Permission, error) {
	return s.items, nil
}

func hasPermission(items []string, permission string) bool {
	return countPermission(items, permission) > 0
}

func countPermission(items []string, permission string) int {
	count := 0
	for _, item := range items {
		if item == permission {
			count++
		}
	}
	return count
}

type testProviderStore struct {
	items           map[string]*iapiserver.Provider
	getByIDsCalls   int
	lastGetByIDsUID string
}

func (s *testProviderStore) List(
	_ context.Context,
	_ *iapiserver.ProviderListRequest,
) ([]*iapiserver.Provider, int64, error) {
	items := make([]*iapiserver.Provider, 0, len(s.items))
	for _, item := range s.items {
		cloned := *item
		items = append(items, &cloned)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, int64(len(items)), nil
}

func (s *testProviderStore) Get(_ context.Context, id string) (*iapiserver.Provider, error) {
	item, ok := s.items[id]
	if !ok {
		return nil, errors.NewStatusF(code.ErrPageNotFound, "provider not found")
	}
	cloned := *item
	return &cloned, nil
}

func (s *testProviderStore) GetByIDs(
	_ context.Context,
	ownerUserID string,
	ids []string,
) ([]*iapiserver.Provider, error) {
	s.getByIDsCalls++
	s.lastGetByIDsUID = ownerUserID
	items := make([]*iapiserver.Provider, 0, len(ids))
	for _, id := range ids {
		item, ok := s.items[id]
		if !ok || item.OwnerUserID != ownerUserID {
			continue
		}
		cloned := *item
		items = append(items, &cloned)
	}
	return items, nil
}

func (s *testProviderStore) Add(_ context.Context, data *iapiserver.Provider) (*iapiserver.Provider, error) {
	if data.ID == "" {
		data.ID = data.Name + "-id"
	}
	cloned := *data
	s.items[data.ID] = &cloned
	ret := cloned
	return &ret, nil
}

func (s *testProviderStore) Update(_ context.Context, data *iapiserver.Provider) (*iapiserver.Provider, error) {
	cloned := *data
	s.items[data.ID] = &cloned
	ret := cloned
	return &ret, nil
}

func (s *testProviderStore) Delete(_ context.Context, id string) error {
	delete(s.items, id)
	return nil
}

type testProviderModelStore struct {
	items           map[string]*iapiserver.ProviderModel
	getByIDsCalls   int
	lastGetByIDsUID string
}

func (s *testProviderModelStore) List(
	_ context.Context,
	req *iapiserver.ProviderModelListRequest,
) ([]*iapiserver.ProviderModel, int64, error) {
	items := make([]*iapiserver.ProviderModel, 0, len(s.items))
	for _, item := range s.items {
		if req.ProviderID != "" && item.ProviderID != req.ProviderID {
			continue
		}
		cloned := *item
		items = append(items, &cloned)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, int64(len(items)), nil
}

func (s *testProviderModelStore) Get(_ context.Context, id string) (*iapiserver.ProviderModel, error) {
	item, ok := s.items[id]
	if !ok {
		return nil, errors.NewStatusF(code.ErrPageNotFound, "provider model not found")
	}
	cloned := *item
	return &cloned, nil
}

func (s *testProviderModelStore) GetByIDs(
	_ context.Context,
	ownerUserID string,
	ids []string,
) ([]*iapiserver.ProviderModel, error) {
	s.getByIDsCalls++
	s.lastGetByIDsUID = ownerUserID
	items := make([]*iapiserver.ProviderModel, 0, len(ids))
	for _, id := range ids {
		item, ok := s.items[id]
		if !ok || item.OwnerUserID != ownerUserID {
			continue
		}
		cloned := *item
		items = append(items, &cloned)
	}
	return items, nil
}

func (s *testProviderModelStore) Add(_ context.Context, data *iapiserver.ProviderModel) (*iapiserver.ProviderModel, error) {
	if data.ID == "" {
		data.ID = data.ProviderID + ":" + data.Model
	}
	cloned := *data
	s.items[data.ID] = &cloned
	ret := cloned
	return &ret, nil
}

func (s *testProviderModelStore) Update(_ context.Context, data *iapiserver.ProviderModel) (*iapiserver.ProviderModel, error) {
	cloned := *data
	s.items[data.ID] = &cloned
	ret := cloned
	return &ret, nil
}

func (s *testProviderModelStore) Delete(_ context.Context, providerID, id string) error {
	item, ok := s.items[id]
	if ok && item.ProviderID == providerID {
		delete(s.items, id)
	}
	return nil
}

func (s *testProviderModelStore) DeleteByProviderID(_ context.Context, providerID string) error {
	for id, item := range s.items {
		if item.ProviderID == providerID {
			delete(s.items, id)
		}
	}
	return nil
}

type testSystemLLMConfigStore struct {
	items map[string]*iapiserver.SystemLLMConfig
}

func (s *testSystemLLMConfigStore) List(_ context.Context) ([]*iapiserver.SystemLLMConfig, error) {
	items := make([]*iapiserver.SystemLLMConfig, 0, len(s.items))
	for _, item := range s.items {
		cloned := *item
		items = append(items, &cloned)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Purpose < items[j].Purpose })
	return items, nil
}

func (s *testSystemLLMConfigStore) Upsert(
	_ context.Context,
	data *iapiserver.SystemLLMConfig,
) (*iapiserver.SystemLLMConfig, error) {
	if data.ID == "" {
		data.ID = data.Purpose + "-id"
	}
	cloned := *data
	s.items[data.Purpose] = &cloned
	ret := cloned
	return &ret, nil
}

func (s *testSystemLLMConfigStore) DeleteByProviderModelID(_ context.Context, providerID, modelID string) error {
	for purpose, item := range s.items {
		if item.ProviderID == providerID && item.ModelID == modelID {
			delete(s.items, purpose)
		}
	}
	return nil
}

func (s *testSystemLLMConfigStore) DeleteByProviderID(_ context.Context, providerID string) error {
	for purpose, item := range s.items {
		if item.ProviderID == providerID {
			delete(s.items, purpose)
		}
	}
	return nil
}
