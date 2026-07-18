package platform

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	for _, key := range []string{iapiserver.AIAppEngineInstanceRead, iapiserver.AIAppComfyUIWorkflowRead, iapiserver.AIAppComfyUIWorkflowManage, iapiserver.AIAppComfyUIWorkflowValidate, iapiserver.AIAppComfyUIWorkflowConvert} {
		if !permissions.Has(key) {
			t.Fatalf("default permissions missing %s", key)
		}
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
func (f *testFactory) StorageBackends() store.StorageBackendStore     { return nil }
func (f *testFactory) AssetsV2() store.AssetStore                     { return nil }
func (f *testFactory) AssetsV1() store.AssetV1Store                   { return nil }
func (f *testFactory) AssetThumbnails() store.AssetThumbnailStore     { return nil }
func (f *testFactory) Tags() store.TagStore                           { return nil }
func (f *testFactory) AssetTags() store.AssetTagStore                 { return nil }
func (f *testFactory) AssetGroups() store.AssetGroupStore             { return nil }
func (f *testFactory) AssetGroupMembers() store.AssetGroupMemberStore { return nil }
func (f *testFactory) AssetRelations() store.AssetRelationStore       { return nil }
func (f *testFactory) TaskCenters() store.TaskCenterStore             { return nil }
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
	items map[string]*iapiserver.Provider
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
	items map[string]*iapiserver.ProviderModel
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
