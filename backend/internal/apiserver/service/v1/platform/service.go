package platform

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/httpcli"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/sets"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
	"github.com/wangweihong/omnimam/backend/pkg/general"
)

type PlatformSrv interface {
	Me(ctx context.Context) (*iapiserver.MeResponse, error)

	ProviderList(ctx context.Context, req *iapiserver.ProviderListRequest) (*iapiserver.ProviderListResponse, error)
	ProviderGet(ctx context.Context, id string) (*iapiserver.Provider, error)
	ProviderCreate(ctx context.Context, req *iapiserver.ProviderCreateRequest) (*iapiserver.Provider, error)
	ProviderUpdate(ctx context.Context, req *iapiserver.ProviderUpdateRequest) (*iapiserver.Provider, error)
	// ProviderDelete removes one provider together with its models and related default model bindings.
	ProviderDelete(ctx context.Context, id string) (*iapiserver.Provider, error)
	// ProviderTest checks OpenAI-compatible provider reachability using saved data plus optional overrides.
	ProviderTest(ctx context.Context, req *iapiserver.ProviderTestRequest) (*iapiserver.ProviderTestResponse, error)
	ProviderModelList(
		ctx context.Context,
		req *iapiserver.ProviderModelListRequest,
	) (*iapiserver.ProviderModelListResponse, error)
	ProviderModelCreate(
		ctx context.Context,
		req *iapiserver.ProviderModelCreateRequest,
	) (*iapiserver.ProviderModel, error)
	ProviderModelUpdate(
		ctx context.Context,
		req *iapiserver.ProviderModelUpdateRequest,
	) (*iapiserver.ProviderModel, error)
	// ProviderModelDelete 删除一个模型提供商下的模型，并清理相关默认模型绑定。
	ProviderModelDelete(ctx context.Context, providerID, id string) (*iapiserver.ProviderModel, error)
	// ProviderModelHealthCheck 立即检测单个模型的 provider metadata 连接状态并保存健康结果。
	ProviderModelHealthCheck(ctx context.Context, providerID, id string) (*iapiserver.ProviderModelHealthCheckResponse, error)
	// ProviderModelSync imports remote model metadata for one provider without invoking generation.
	ProviderModelSync(
		ctx context.Context,
		req *iapiserver.ProviderModelSyncRequest,
	) (*iapiserver.ProviderModelSyncResponse, error)
	SystemLLMConfigList(ctx context.Context) (*iapiserver.SystemLLMConfigListResponse, error)
	SystemLLMConfigUpsert(
		ctx context.Context,
		req *iapiserver.SystemLLMConfigUpsertRequest,
	) (*iapiserver.SystemLLMConfigListResponse, error)
	DefaultModelGet(ctx context.Context, usage string) (*iapiserver.SystemLLMConfig, error)
	DefaultModelSave(ctx context.Context, usage string, req *iapiserver.DefaultModelSaveRequest) (*iapiserver.SystemLLMConfig, error)
	ModelOptionList(ctx context.Context, req *iapiserver.ProviderModelListRequest) (*iapiserver.ModelOptionListResponse, error)

	AssetUpload(
		ctx context.Context,
		file *multipart.FileHeader,
		tagNames []string,
		sourceType string,
	) (*iapiserver.AssetUploadResponse, error)
	RegisterArtifact(context.Context, *iapiserver.ArtifactRegistrationRequest) (*iapiserver.ArtifactRegistrationResponse, error)
	BatchApplyAssetLabels(context.Context, *iapiserver.BatchLabelRequest) (*iapiserver.BatchLabelResponse, error)
	// AssetChunkUploadInit prepares a checksum-scoped resumable upload directory and reports uploaded chunks.
	AssetChunkUploadInit(
		ctx context.Context,
		req *iapiserver.AssetChunkUploadInitRequest,
	) (*iapiserver.AssetChunkUploadInitResponse, error)
	// AssetChunkUploadPart writes one resumable upload chunk. The final asset is not created until complete.
	AssetChunkUploadPart(
		ctx context.Context,
		checksum string,
		index int,
		body io.Reader,
	) (*iapiserver.AssetChunkUploadPartResponse, error)
	// AssetChunkUploadComplete merges chunks, validates final checksum, creates the asset, and removes temp files.
	AssetChunkUploadComplete(
		ctx context.Context,
		req *iapiserver.AssetChunkUploadCompleteRequest,
	) (*iapiserver.AssetUploadResponse, error)
	// AssetChunkUploadCancel removes a checksum-scoped resumable upload directory.
	AssetChunkUploadCancel(ctx context.Context, checksum string) (*iapiserver.AssetChunkUploadCancelResponse, error)
	AssetList(ctx context.Context, req *iapiserver.AssetListRequest) (*iapiserver.AssetListResponse, error)
	AssetSearch(ctx context.Context, req *iapiserver.AssetSearchRequest) (*iapiserver.AssetListResponse, error)
	AssetSearchParse(
		ctx context.Context,
		req *iapiserver.AssetSearchParseRequest,
	) (*iapiserver.AssetSearchParseResponse, error)
	AssetGet(ctx context.Context, id string) (*iapiserver.AssetRecord, error)
	AssetUpdate(ctx context.Context, req *iapiserver.AssetUpdateRequest) (*iapiserver.AssetRecord, error)
	// AssetDelete marks one asset as deleted. It keeps stored objects and relations for recovery/audit.
	AssetDelete(ctx context.Context, id string) (*iapiserver.AssetRecord, error)
	AssetContentPath(ctx context.Context, id string) (string, string, error)
	AssetThumbnailPath(ctx context.Context, id string) (string, string, error)
	AssetGroupCreate(
		ctx context.Context,
		req *iapiserver.AssetGroupCreateRequest,
	) (*iapiserver.AssetGroupCreateResponse, error)

	// CanvasAssetRegisterOutput registers one generated asset as a canvas output reference.
	// It returns asset metadata and creates a Task Center audit run; raw content is not returned.
	CanvasAssetRegisterOutput(
		ctx context.Context,
		req *iapiserver.CanvasAssetRegisterOutputRequest,
	) (*iapiserver.CanvasAssetRegisterOutputResponse, error)
	// CanvasAssetDownloadZip writes selected asset contents into a zip stream.
	// It reads raw asset objects through StorageBackend and never exposes local paths.
	CanvasAssetDownloadZip(ctx context.Context, req *iapiserver.CanvasAssetDownloadRequest, dst io.Writer) error
	// CanvasNodeRun creates a Task Center run for one canvas node execution.
	// Provider-specific work is handled later by task-center executors through run input.
	CanvasNodeRun(
		ctx context.Context,
		canvasID, nodeID string,
		req *iapiserver.CanvasNodeRunRequest,
	) (*iapiserver.CanvasRunResponse, error)
}

func (s *platformService) RegisterArtifact(ctx context.Context, req *iapiserver.ArtifactRegistrationRequest) (*iapiserver.ArtifactRegistrationResponse, error) {
	asset, created, err := s.store.AssetsV1().RegisterArtifact(ctx, req)
	if err != nil {
		return nil, err
	}
	result := "already_registered"
	if created {
		result = "created"
	}
	return &iapiserver.ArtifactRegistrationResponse{ArtifactID: req.ArtifactID, ApplicationRunID: req.ApplicationRunID, RegistrationResult: result, Asset: asset}, nil
}

func (s *platformService) BatchApplyAssetLabels(ctx context.Context, req *iapiserver.BatchLabelRequest) (*iapiserver.BatchLabelResponse, error) {
	if err := validateBatchLabels(req); err != nil {
		return nil, err
	}
	owner := iapiserver.DefaultTaskCenterCreatedBy
	if user, userErr := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser); userErr == nil && user != nil && user.ID != "" {
		owner = user.ID
	}
	response := &iapiserver.BatchLabelResponse{Total: len(req.Items), Results: make([]iapiserver.BatchLabelResult, 0, len(req.Items))}
	for _, item := range req.Items {
		data, err := s.store.AssetsV1().ApplyLabels(ctx, owner, item.ID, req.LabelsToUpsert, req.TagsToAdd, req.TagsToRemove)
		result := iapiserver.BatchLabelResult{ID: item.ID, Success: err == nil, Data: data}
		if err != nil {
			response.Fail++
			result.Error = map[string]any{"code": "ERR_ASSET_NOT_FOUND", "message": err.Error(), "retryable": false}
		} else {
			response.Success++
		}
		response.Results = append(response.Results, result)
	}
	return response, nil
}

func validateBatchLabels(req *iapiserver.BatchLabelRequest) error {
	if len(req.LabelsToUpsert) == 0 && len(req.TagsToAdd) == 0 && len(req.TagsToRemove) == 0 {
		return errors.NewStatusF(code.ErrValidation, "at least one label change is required")
	}
	if len(req.LabelsToUpsert) > 20 {
		return errors.NewStatusF(code.ErrValidation, "labels exceed limit")
	}
	seen := map[string]struct{}{}
	for _, item := range req.Items {
		if _, ok := seen[item.ID]; ok {
			return errors.NewStatusF(code.ErrValidation, "asset ids must be unique")
		}
		seen[item.ID] = struct{}{}
	}
	add := map[string]struct{}{}
	for _, tag := range req.TagsToAdd {
		tag = strings.TrimSpace(tag)
		if tag == "" || len([]rune(tag)) > 64 {
			return errors.NewStatusF(code.ErrValidation, "tag is invalid")
		}
		add[tag] = struct{}{}
	}
	for _, tag := range req.TagsToRemove {
		tag = strings.TrimSpace(tag)
		if _, ok := add[tag]; ok {
			return errors.NewStatusF(code.ErrValidation, "tag cannot be added and removed together")
		}
	}
	for key, value := range req.LabelsToUpsert {
		key = strings.TrimSpace(key)
		if key == "" || len([]rune(key)) > 63 || strings.ContainsAny(key, ",;()=!\"#@ \t\r\n") || len([]rune(strings.TrimSpace(value))) > 63 {
			return errors.NewStatusF(code.ErrValidation, "label is invalid")
		}
	}
	return nil
}

type platformService struct {
	store      store.Factory
	dispatcher TaskDispatcher
}

// TaskDispatcher 是素材上传完成后唤醒共享任务执行器所需的最小接口。
type TaskDispatcher interface {
	DispatchAsync(ctx context.Context, runID string)
}

var (
	chunkUploadTempDir      = filepath.Join(os.TempDir(), "omnimam", "upload-parts")
	chunkUploadCleanupHours = 24
)

func SetChunkUploadTempDir(dir string) {
	if strings.TrimSpace(dir) != "" {
		chunkUploadTempDir = dir
	}
}

func StartChunkUploadCleanup(stopCh <-chan struct{}, ttl time.Duration) {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	chunkUploadCleanupHours = int(ttl / time.Hour)
	ticker := time.NewTicker(time.Hour)
	go func() {
		defer ticker.Stop()
		cleanupChunkUploadDirs(ttl)
		for {
			select {
			case <-ticker.C:
				cleanupChunkUploadDirs(ttl)
			case <-stopCh:
				return
			}
		}
	}()
}

func StartProviderModelHealthCheck(stopCh <-chan struct{}, storeIns store.Factory, interval time.Duration) {
	if storeIns == nil {
		return
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	svc := NewService(storeIns)
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		svc.checkAllProviderModels(context.Background())
		for {
			select {
			case <-ticker.C:
				svc.checkAllProviderModels(context.Background())
			case <-stopCh:
				return
			}
		}
	}()
}

func NewService(str store.Factory, dispatcher ...TaskDispatcher) *platformService {
	service := &platformService{store: str}
	if len(dispatcher) > 0 {
		service.dispatcher = dispatcher[0]
	}
	return service
}

// GetProviderModelRefSummaries 按用户边界批量返回非敏感模型摘要，供其他领域组合一跳关系。
func (s *platformService) GetProviderModelRefSummaries(
	ctx context.Context,
	ownerUserID string,
	ids []string,
) (map[string]*iapiserver.ProviderModelRefSummary, error) {
	result := make(map[string]*iapiserver.ProviderModelRefSummary)
	ids = uniqueStrings(ids)
	if len(ids) == 0 {
		return result, nil
	}
	models, err := s.store.ProviderModels().GetByIDs(ctx, ownerUserID, ids)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	providerIDs := make([]string, 0, len(models))
	for _, model := range models {
		if model != nil {
			providerIDs = append(providerIDs, model.ProviderID)
		}
	}
	providers, err := s.store.Providers().GetByIDs(ctx, ownerUserID, uniqueStrings(providerIDs))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	providerByID := make(map[string]*iapiserver.Provider, len(providers))
	for _, item := range providers {
		if item != nil {
			providerByID[item.ID] = item
		}
	}
	for _, model := range models {
		if model == nil {
			continue
		}
		provider := providerByID[model.ProviderID]
		if provider == nil {
			continue
		}
		displayName := model.DisplayName
		if displayName == "" {
			displayName = model.Model
		}
		result[model.ID] = &iapiserver.ProviderModelRefSummary{
			ID:           model.ID,
			DisplayName:  displayName,
			ProviderID:   provider.ID,
			ProviderName: provider.Name,
			Model:        model.Model,
			HealthStatus: model.HealthStatus,
			Enabled:      provider.Enabled && model.Enabled,
		}
	}
	return result, nil
}

func uniqueStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (s *platformService) Me(ctx context.Context) (*iapiserver.MeResponse, error) {
	flags, err := s.store.FeatureFlags().List(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	featureFlags := defaultFeatureFlags()
	for _, flag := range flags {
		if flag.Key != "" {
			featureFlags[flag.Key] = flag.Enabled
		}
	}
	permissions, err := s.store.Permissions().List(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	permissionKeys := defaultPermissions()
	for _, p := range permissions {
		if p.Key != "" {
			permissionKeys = appendUnique(permissionKeys, p.Key)
		}
	}
	return &iapiserver.MeResponse{
		User: iapiserver.MeUser{
			ID:   "system-admin",
			Name: "System Admin",
		},
		Roles:        []string{"admin"},
		Permissions:  permissionKeys,
		FeatureFlags: featureFlags,
	}, nil
}

func (s *platformService) ProviderList(
	ctx context.Context,
	req *iapiserver.ProviderListRequest,
) (*iapiserver.ProviderListResponse, error) {
	ownerUserID, err := currentPlatformUserID(ctx)
	if err != nil {
		return nil, err
	}
	req.OwnerUserID = ownerUserID
	items, total, err := s.store.Providers().List(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.ProviderListResponse{Total: total, Items: items}, nil
}

func (s *platformService) ProviderGet(ctx context.Context, id string) (*iapiserver.Provider, error) {
	ownerUserID, err := currentPlatformUserID(ctx)
	if err != nil {
		return nil, err
	}
	provider, err := s.store.Providers().Get(ctx, id)
	if err != nil {
		return nil, errors.NewStatusF(code.ErrModelProviderNotFound, "model provider not found")
	}
	if provider.OwnerUserID != "" && provider.OwnerUserID != ownerUserID {
		return nil, errors.NewStatusF(code.ErrModelProviderNotFound, "model provider not found")
	}
	return provider, nil
}

func (s *platformService) ProviderCreate(
	ctx context.Context,
	req *iapiserver.ProviderCreateRequest,
) (*iapiserver.Provider, error) {
	ownerUserID, err := currentPlatformUserID(ctx)
	if err != nil {
		return nil, err
	}
	provider := &iapiserver.Provider{
		OwnerUserID:   ownerUserID,
		Type:          req.Type,
		Enabled:       general.FallbackIfNil(req.Enabled, true),
		BaseURL:       req.BaseURL,
		AuthType:      req.AuthType,
		CredentialRef: req.CredentialRef,
		PresetKey:     req.PresetKey,
		Config:        req.Config,
	}
	provider.Name = req.Name
	provider.Description = req.Description
	if err := s.ensureProviderNameUnique(ctx, ownerUserID, provider.Name, ""); err != nil {
		return nil, err
	}
	applyProviderPresetDefaults(provider)
	if provider.AuthType == "" && provider.CredentialRef != "" {
		provider.AuthType = iapiserver.ProviderAuthTypeAPIKey
	}
	created, err := s.store.Providers().Add(ctx, provider)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return created, nil
}

func (s *platformService) ProviderUpdate(
	ctx context.Context,
	req *iapiserver.ProviderUpdateRequest,
) (*iapiserver.Provider, error) {
	provider, err := s.store.Providers().Get(ctx, req.ID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	provider.Name = general.FallbackIfNil(req.Name, provider.Name)
	provider.Type = general.FallbackIfNil(req.Type, provider.Type)
	provider.Enabled = general.FallbackIfNil(req.Enabled, provider.Enabled)
	provider.BaseURL = general.FallbackIfNil(req.BaseURL, provider.BaseURL)
	provider.AuthType = general.FallbackIfNil(req.AuthType, provider.AuthType)
	provider.CredentialRef = general.FallbackIfNil(req.CredentialRef, provider.CredentialRef)
	provider.Config = general.FallbackIfNil(req.Config, provider.Config)
	provider.Description = general.FallbackIfNil(req.Description, provider.Description)
	if err := s.ensureProviderNameUnique(ctx, provider.OwnerUserID, provider.Name, provider.ID); err != nil {
		return nil, err
	}
	updated, err := s.store.Providers().Update(ctx, provider)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return updated, nil
}

func (s *platformService) ProviderDelete(ctx context.Context, id string) (*iapiserver.Provider, error) {
	provider, err := s.store.Providers().Get(ctx, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if err := s.store.SystemLLMConfigs().DeleteByProviderID(ctx, id); err != nil {
		return nil, errors.WithStack(err)
	}
	if err := s.store.ProviderModels().DeleteByProviderID(ctx, id); err != nil {
		return nil, errors.WithStack(err)
	}
	if err := s.store.Providers().Delete(ctx, id); err != nil {
		return nil, errors.WithStack(err)
	}
	return provider, nil
}

func (s *platformService) ProviderTest(
	ctx context.Context,
	req *iapiserver.ProviderTestRequest,
) (*iapiserver.ProviderTestResponse, error) {
	provider, err := s.providerWithOverrides(ctx, req.ID, req.BaseURL, req.AuthType, req.CredentialRef)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if _, err := fetchOpenAICompatibleModels(ctx, provider); err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.ProviderTestResponse{
		TargetType:   "provider",
		ProviderID:   provider.ID,
		Success:      true,
		HealthStatus: iapiserver.ProviderModelHealthHealthy,
		Message:      "provider connection ok",
		CheckedAt:    imachinery.NewTime(time.Now()),
	}, nil
}

func (s *platformService) ProviderModelList(
	ctx context.Context,
	req *iapiserver.ProviderModelListRequest,
) (*iapiserver.ProviderModelListResponse, error) {
	ownerUserID, err := currentPlatformUserID(ctx)
	if err != nil {
		return nil, err
	}
	req.OwnerUserID = ownerUserID
	items, total, err := s.store.ProviderModels().List(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if err := s.attachProviderNames(ctx, ownerUserID, items); err != nil {
		return nil, err
	}
	return &iapiserver.ProviderModelListResponse{Total: total, Items: items}, nil
}

func (s *platformService) ProviderModelCreate(
	ctx context.Context,
	req *iapiserver.ProviderModelCreateRequest,
) (*iapiserver.ProviderModel, error) {
	ownerUserID, err := currentPlatformUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.ensureProviderModelUnique(ctx, req.ProviderID, req.Name, req.Model, ""); err != nil {
		return nil, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	streamSupported := true
	if req.StreamSupported != nil {
		streamSupported = *req.StreamSupported
	}
	model := &iapiserver.ProviderModel{
		ProviderID:      req.ProviderID,
		OwnerUserID:     ownerUserID,
		Model:           req.Model,
		GroupName:       req.GroupName,
		HealthStatus:    iapiserver.ProviderModelHealthUnknown,
		Capabilities:    req.Capabilities,
		StreamSupported: streamSupported,
		Enabled:         enabled,
	}
	model.Name = req.Name
	model.DisplayName = req.Name
	created, err := s.store.ProviderModels().Add(ctx, model)
	if err != nil {
		return nil, err
	}
	if err := s.attachProviderNames(ctx, ownerUserID, []*iapiserver.ProviderModel{created}); err != nil {
		return nil, err
	}
	return created, nil
}

func (s *platformService) ProviderModelUpdate(
	ctx context.Context,
	req *iapiserver.ProviderModelUpdateRequest,
) (*iapiserver.ProviderModel, error) {
	model, err := s.store.ProviderModels().Get(ctx, req.ID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	nextName := general.FallbackIfNil(req.Name, model.Name)
	nextModel := general.FallbackIfNil(req.Model, model.Model)
	if err := s.ensureProviderModelUnique(ctx, model.ProviderID, nextName, nextModel, model.ID); err != nil {
		return nil, err
	}
	model.Name = nextName
	model.DisplayName = nextName
	model.Model = nextModel
	model.GroupName = general.FallbackIfNil(req.GroupName, model.GroupName)
	model.Capabilities = general.FallbackIfNil(req.Capabilities, model.Capabilities)
	model.StreamSupported = general.FallbackIfNil(req.StreamSupported, model.StreamSupported)
	model.Enabled = general.FallbackIfNil(req.Enabled, model.Enabled)

	updated, err := s.store.ProviderModels().Update(ctx, model)
	if err != nil {
		return nil, err
	}
	if err := s.attachProviderNames(ctx, model.OwnerUserID, []*iapiserver.ProviderModel{updated}); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *platformService) ProviderModelHealthCheck(
	ctx context.Context,
	providerID string,
	id string,
) (*iapiserver.ProviderModelHealthCheckResponse, error) {
	model, err := s.store.ProviderModels().Get(ctx, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if providerID != "" && model.ProviderID != providerID {
		return nil, errors.NewStatusF(code.ErrValidation, "provider model %s does not belong to provider %s", id, providerID)
	}
	checked, err := s.checkProviderModel(ctx, model)
	if err != nil {
		return nil, err
	}
	success := checked.HealthStatus == iapiserver.ProviderModelHealthHealthy
	return &iapiserver.ProviderModelHealthCheckResponse{
		TargetType:   "model",
		ProviderID:   checked.ProviderID,
		ModelID:      checked.ID,
		Success:      success,
		HealthStatus: checked.HealthStatus,
		Message:      checked.HealthReason,
		CheckedAt:    imachinery.NewTime(time.Now()),
	}, nil
}

func (s *platformService) checkAllProviderModels(ctx context.Context) {
	models, _, err := s.store.ProviderModels().List(ctx, &iapiserver.ProviderModelListRequest{})
	if err != nil {
		log.Errorf("provider model health list failed: %v", err)
		return
	}
	for _, model := range models {
		checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if _, err := s.checkProviderModel(checkCtx, model); err != nil {
			log.Errorf("provider model health check failed: %v", err)
		}
		cancel()
	}
}

func (s *platformService) checkProviderModel(
	ctx context.Context,
	model *iapiserver.ProviderModel,
) (*iapiserver.ProviderModel, error) {
	provider, err := s.store.Providers().Get(ctx, model.ProviderID)
	now := time.Now()
	if err != nil {
		model.HealthStatus = iapiserver.ProviderModelHealthUnhealthy
		model.HealthReason = "模型提供商不存在或不可用"
		model.HealthCheckedAt = &now
		return s.store.ProviderModels().Update(context.WithoutCancel(ctx), model)
	}
	reason := ""
	healthy := true
	if !provider.Enabled {
		healthy = false
		reason = "模型提供商已禁用"
	} else if !model.Enabled {
		healthy = false
		reason = "模型已禁用"
	} else {
		remoteModels, err := fetchOpenAICompatibleModels(ctx, provider)
		if err != nil {
			healthy = false
			reason = providerModelHealthReason(err)
		} else if !slices.Contains(remoteModels, model.Model) {
			healthy = false
			reason = "远端模型列表中未找到该模型"
		}
	}
	if healthy {
		model.HealthStatus = iapiserver.ProviderModelHealthHealthy
		model.HealthReason = ""
	} else {
		model.HealthStatus = iapiserver.ProviderModelHealthUnhealthy
		model.HealthReason = reason
	}
	model.HealthCheckedAt = &now
	return s.store.ProviderModels().Update(context.WithoutCancel(ctx), model)
}

func providerModelHealthReason(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if strings.Contains(message, "ErrProviderUnauthorized") || strings.Contains(message, "authentication") {
		return "认证失败或 API 密钥无效"
	}
	if strings.Contains(message, "ErrProviderResponseParseError") || strings.Contains(message, "parse") {
		return "模型服务响应解析失败"
	}
	if strings.Contains(message, "ErrProviderUnsupported") || strings.Contains(message, "unsupported") {
		return "模型服务协议不支持"
	}
	if strings.Contains(message, "deadline exceeded") || strings.Contains(message, "timeout") {
		return "连接超时"
	}
	return "模型服务不可用"
}

func (s *platformService) ProviderModelDelete(
	ctx context.Context,
	providerID string,
	id string,
) (*iapiserver.ProviderModel, error) {
	model, err := s.store.ProviderModels().Get(ctx, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if providerID != "" && model.ProviderID != providerID {
		return nil, errors.NewStatusF(code.ErrValidation, "provider model %s does not belong to provider %s", id, providerID)
	}
	if providerID == "" {
		providerID = model.ProviderID
	}
	if err := s.attachProviderNames(ctx, model.OwnerUserID, []*iapiserver.ProviderModel{model}); err != nil {
		return nil, err
	}
	if err := s.store.SystemLLMConfigs().DeleteByProviderModelID(ctx, providerID, id); err != nil {
		return nil, errors.WithStack(err)
	}
	if err := s.store.ProviderModels().Delete(ctx, providerID, id); err != nil {
		return nil, errors.WithStack(err)
	}
	return model, nil
}

func (s *platformService) ProviderModelSync(
	ctx context.Context,
	req *iapiserver.ProviderModelSyncRequest,
) (*iapiserver.ProviderModelSyncResponse, error) {
	provider, err := s.store.Providers().Get(ctx, req.ProviderID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	remoteModels, err := fetchOpenAICompatibleModels(ctx, provider)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	existingResp, err := s.ProviderModelList(ctx, &iapiserver.ProviderModelListRequest{ProviderID: provider.ID})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	existingByModel := map[string]*iapiserver.ProviderModel{}
	for _, item := range existingResp.Items {
		existingByModel[item.Model] = item
	}
	created := 0
	updated := 0
	skipped := 0
	for _, remote := range remoteModels {
		if remote == "" {
			continue
		}
		if existing := existingByModel[remote]; existing != nil {
			nextCaps := appendCapability(existing.Capabilities, iapiserver.CapabilityLLMChat)
			changed := applyPresetModelDefaults(provider, existing)
			if existing.DisplayName == remote && sets.NewString(existing.Capabilities...).Equal(sets.NewString(nextCaps...)) && !changed {
				skipped++
				continue
			}
			existing.Name = remote
			existing.DisplayName = remote
			existing.Capabilities = nextCaps
			if _, err := s.store.ProviderModels().Update(ctx, existing); err != nil {
				return nil, errors.WithStack(err)
			}
			updated++
			continue
		}
		model := &iapiserver.ProviderModel{
			ProviderID:      provider.ID,
			OwnerUserID:     provider.OwnerUserID,
			Model:           remote,
			DisplayName:     remote,
			HealthStatus:    iapiserver.ProviderModelHealthUnknown,
			Capabilities:    []string{iapiserver.CapabilityLLMChat},
			StreamSupported: true,
			Enabled:         true,
		}
		model.Name = remote
		applyPresetModelDefaults(provider, model)
		if _, err := s.store.ProviderModels().Add(ctx, model); err != nil {
			return nil, errors.WithStack(err)
		}
		created++
	}
	models, err := s.ProviderModelList(ctx, &iapiserver.ProviderModelListRequest{ProviderID: provider.ID})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.ProviderModelSyncResponse{
		Total:   int(models.Total),
		Models:  models.Items,
		Created: created,
		Updated: updated,
		Skipped: skipped,
	}, nil
}

func (s *platformService) SystemLLMConfigList(ctx context.Context) (*iapiserver.SystemLLMConfigListResponse, error) {
	ownerUserID, err := currentPlatformUserID(ctx)
	if err != nil {
		return nil, err
	}
	configs, err := s.store.SystemLLMConfigs().List(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	filtered := configs[:0]
	for _, cfg := range configs {
		if cfg.OwnerUserID == "" || cfg.OwnerUserID == ownerUserID {
			filtered = append(filtered, cfg)
		}
	}
	return &iapiserver.SystemLLMConfigListResponse{Configs: filtered}, nil
}

func (s *platformService) SystemLLMConfigUpsert(
	ctx context.Context,
	req *iapiserver.SystemLLMConfigUpsertRequest,
) (*iapiserver.SystemLLMConfigListResponse, error) {
	ownerUserID, err := currentPlatformUserID(ctx)
	if err != nil {
		return nil, err
	}
	for _, spec := range req.Configs {
		if !isDefaultModelUsage(spec.Purpose) {
			return nil, errors.NewStatusF(code.ErrDefaultModelInvalid, "default model usage %s is invalid", spec.Purpose)
		}
		cfg := &iapiserver.SystemLLMConfig{
			OwnerUserID: ownerUserID,
			Purpose:     spec.Purpose,
			ProviderID:  spec.ProviderID,
			ModelID:     spec.ModelID,
		}
		cfg.Name = spec.Purpose
		if _, err := s.store.SystemLLMConfigs().Upsert(ctx, cfg); err != nil {
			return nil, errors.WithStack(err)
		}
	}
	return s.SystemLLMConfigList(ctx)
}

func (s *platformService) DefaultModelGet(ctx context.Context, usage string) (*iapiserver.SystemLLMConfig, error) {
	if !isDefaultModelUsage(usage) {
		return nil, errors.NewStatusF(code.ErrDefaultModelMissing, "default model usage %s is invalid", usage)
	}
	list, err := s.SystemLLMConfigList(ctx)
	if err != nil {
		return nil, err
	}
	for _, cfg := range list.Configs {
		if cfg.Purpose == usage {
			if model, err := s.store.ProviderModels().Get(ctx, cfg.ModelID); err == nil {
				if err := s.attachProviderNames(ctx, cfg.OwnerUserID, []*iapiserver.ProviderModel{model}); err != nil {
					return nil, err
				}
				cfg.ModelDetail = model
			}
			return cfg, nil
		}
	}
	return nil, errors.NewStatusF(code.ErrDefaultModelMissing, "default model %s is missing", usage)
}

func (s *platformService) DefaultModelSave(
	ctx context.Context,
	usage string,
	req *iapiserver.DefaultModelSaveRequest,
) (*iapiserver.SystemLLMConfig, error) {
	if !isDefaultModelUsage(usage) {
		return nil, errors.NewStatusF(code.ErrDefaultModelInvalid, "default model usage %s is invalid", usage)
	}
	ownerUserID, err := currentPlatformUserID(ctx)
	if err != nil {
		return nil, err
	}
	provider, err := s.ProviderGet(ctx, req.ProviderID)
	if err != nil {
		return nil, err
	}
	model, err := s.store.ProviderModels().Get(ctx, req.ModelID)
	if err != nil || model.ProviderID != provider.ID || model.OwnerUserID != ownerUserID || !provider.Enabled || !model.Enabled ||
		model.HealthStatus == iapiserver.ProviderModelHealthUnhealthy {
		return nil, errors.NewStatusF(code.ErrDefaultModelInvalid, "default model candidate is invalid")
	}
	model.ProviderName = provider.Name
	cfg := &iapiserver.SystemLLMConfig{
		OwnerUserID: ownerUserID,
		Purpose:     usage,
		ProviderID:  provider.ID,
		ModelID:     model.ID,
		ModelDetail: model,
	}
	cfg.Name = usage
	saved, err := s.store.SystemLLMConfigs().Upsert(ctx, cfg)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	saved.ModelDetail = model
	return saved, nil
}

func (s *platformService) attachProviderNames(ctx context.Context, ownerUserID string, items []*iapiserver.ProviderModel) error {
	providerIDs := make([]string, 0, len(items))
	for _, item := range items {
		if item != nil {
			item.ProviderName = ""
			providerIDs = append(providerIDs, item.ProviderID)
		}
	}
	providerIDs = uniqueStrings(providerIDs)
	if len(providerIDs) == 0 {
		return nil
	}
	providers, err := s.store.Providers().GetByIDs(ctx, ownerUserID, providerIDs)
	if err != nil {
		return errors.WithStack(err)
	}
	providerNames := make(map[string]string, len(providers))
	for _, provider := range providers {
		if provider != nil {
			providerNames[provider.ID] = provider.Name
		}
	}
	for _, item := range items {
		if item != nil {
			item.ProviderName = providerNames[item.ProviderID]
		}
	}
	return nil
}

func (s *platformService) ModelOptionList(
	ctx context.Context,
	req *iapiserver.ProviderModelListRequest,
) (*iapiserver.ModelOptionListResponse, error) {
	enabled := true
	req.Enabled = &enabled
	if req.Capability == "" && req.Usage != "" {
		req.Capability = capabilityForDefaultModelUsage(req.Usage)
	}
	resp, err := s.ProviderModelList(ctx, req)
	if err != nil {
		return nil, err
	}
	items := resp.Items[:0]
	for _, item := range resp.Items {
		if item.HealthStatus != iapiserver.ProviderModelHealthUnhealthy {
			items = append(items, item)
		}
	}
	return &iapiserver.ModelOptionListResponse{Total: int64(len(items)), Items: items}, nil
}

func capabilityForDefaultModelUsage(usage string) string {
	switch usage {
	case "assistant.default", "quick", "translation":
		return iapiserver.CapabilityLLMChat
	default:
		return ""
	}
}

type providerPreset struct {
	Key            string
	Name           string
	Type           string
	BaseURL        string
	AuthType       string
	ConfigDefaults map[string]any
	ModelTypeRules []iapiserver.ProviderModelTypeRule
}

func providerPresets() []*providerPreset {
	commonDefaults := map[string]any{
		"array_message_content": true,
		"developer_message":     false,
		"stream_options":        true,
		"service_tier":          false,
		"enable_thinking":       false,
		"verbosity":             false,
	}
	return []*providerPreset{
		{
			Key:            "deepseek",
			Name:           "DeepSeek",
			Type:           "",
			BaseURL:        "https://api.deepseek.com",
			AuthType:       iapiserver.ProviderAuthTypeAPIKey,
			ConfigDefaults: commonDefaults,
			ModelTypeRules: []iapiserver.ProviderModelTypeRule{
				{Contains: []string{"reasoner", "r1"}, ModelTypes: []string{"reasoning"}, GroupName: "deepseek", EndpointType: "chat"},
			},
		},
		{
			Key:            "qwen",
			Name:           "通义千问",
			Type:           iapiserver.ProviderTypeOpenAICompatible,
			BaseURL:        "https://dashscope.aliyuncs.com/compatible-mode",
			AuthType:       iapiserver.ProviderAuthTypeAPIKey,
			ConfigDefaults: commonDefaults,
			ModelTypeRules: []iapiserver.ProviderModelTypeRule{
				{Contains: []string{"vl", "vision"}, ModelTypes: []string{"vision"}, GroupName: "qwen", EndpointType: "chat"},
				{Contains: []string{"qwq", "reason", "thinking"}, ModelTypes: []string{"reasoning"}, GroupName: "qwen", EndpointType: "chat"},
				{Contains: []string{"embedding", "embed"}, ModelTypes: []string{"embedding"}, GroupName: "qwen", EndpointType: "embeddings"},
			},
		},
		{
			Key:            "openrouter",
			Name:           "OpenRouter",
			Type:           iapiserver.ProviderTypeOpenAICompatible,
			BaseURL:        "https://openrouter.ai/api",
			AuthType:       iapiserver.ProviderAuthTypeAPIKey,
			ConfigDefaults: commonDefaults,
			ModelTypeRules: []iapiserver.ProviderModelTypeRule{
				{Contains: []string{"vision", "vl"}, ModelTypes: []string{"vision"}, GroupName: "openrouter", EndpointType: "chat"},
				{Contains: []string{"web", "search"}, ModelTypes: []string{"web"}, GroupName: "openrouter", EndpointType: "chat"},
				{Contains: []string{"tool"}, ModelTypes: []string{"tool"}, GroupName: "openrouter", EndpointType: "chat"},
			},
		},
		{
			Key:            "siliconflow",
			Name:           "硅基流动",
			Type:           iapiserver.ProviderTypeOpenAICompatible,
			BaseURL:        "https://api.siliconflow.cn",
			AuthType:       iapiserver.ProviderAuthTypeAPIKey,
			ConfigDefaults: commonDefaults,
			ModelTypeRules: []iapiserver.ProviderModelTypeRule{
				{Contains: []string{"vl", "vision"}, ModelTypes: []string{"vision"}, GroupName: "siliconflow", EndpointType: "chat"},
				{Contains: []string{"rerank"}, ModelTypes: []string{"rerank"}, GroupName: "siliconflow", EndpointType: "rerank"},
				{Contains: []string{"embedding", "embed"}, ModelTypes: []string{"embedding"}, GroupName: "siliconflow", EndpointType: "embeddings"},
			},
		},
	}
}

func providerPresetByKey(key string) *providerPreset {
	for _, preset := range providerPresets() {
		if preset.Key == key {
			return preset
		}
	}
	return nil
}

func providerPresetConfigDefaults(preset *providerPreset) map[string]any {
	config := map[string]any{}
	if preset == nil {
		return config
	}
	for key, value := range preset.ConfigDefaults {
		config[key] = value
	}
	return config
}

func applyProviderPresetDefaults(provider *iapiserver.Provider) {
	if provider == nil || provider.PresetKey == "" {
		return
	}
	preset := providerPresetByKey(provider.PresetKey)
	if preset == nil {
		return
	}
	if provider.Name == "" {
		provider.Name = preset.Name
	}
	if provider.Type == "" {
		provider.Type = preset.Type
	}
	if provider.BaseURL == "" {
		provider.BaseURL = preset.BaseURL
	}
	if provider.AuthType == "" {
		provider.AuthType = preset.AuthType
	}
	if provider.Config == nil {
		provider.Config = providerPresetConfigDefaults(preset)
	}
}

func applyPresetModelDefaults(provider *iapiserver.Provider, model *iapiserver.ProviderModel) bool {
	if provider == nil || model == nil {
		return false
	}
	changed := false
	if model.EndpointType == "" {
		model.EndpointType = "chat"
		changed = true
	}
	if model.GroupName == "" {
		model.GroupName = provider.Name
		changed = true
	}
	preset := providerPresetByKey(provider.PresetKey)
	if preset == nil {
		return changed
	}
	lowerModel := strings.ToLower(model.Model)
	for _, rule := range preset.ModelTypeRules {
		if !ruleMatchesModel(rule, lowerModel) {
			continue
		}
		if rule.EndpointType != "" && model.EndpointType == "chat" {
			model.EndpointType = rule.EndpointType
			changed = true
		}
		if rule.GroupName != "" && model.GroupName == provider.Name {
			model.GroupName = rule.GroupName
			changed = true
		}
		for _, modelType := range rule.ModelTypes {
			// A previous reversed membership check silently dropped inferred model types from preset rules.
			if !sets.NewString(model.ModelTypes...).Has(modelType) {
				model.ModelTypes = append(model.ModelTypes, modelType)
				changed = true
			}
		}
	}
	return changed
}

func ruleMatchesModel(rule iapiserver.ProviderModelTypeRule, lowerModel string) bool {
	for _, token := range rule.Contains {
		if strings.Contains(lowerModel, strings.ToLower(token)) {
			return true
		}
	}
	return false
}

func (s *platformService) ensureProviderModelUnique(
	ctx context.Context,
	providerID, name, modelName, exceptID string,
) error {
	normalizedName := strings.TrimSpace(name)
	normalizedModel := strings.TrimSpace(modelName)
	if normalizedName == "" {
		return errors.NewStatusF(code.ErrValidation, "provider model name is required")
	}
	if normalizedModel == "" {
		return errors.NewStatusF(code.ErrValidation, "provider model is required")
	}
	models, _, err := s.store.ProviderModels().List(ctx, &iapiserver.ProviderModelListRequest{ProviderID: providerID})
	if err != nil {
		return errors.WithStack(err)
	}
	for _, item := range models {
		if item.ID == exceptID {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.Name), normalizedName) {
			return errors.NewStatusF(code.ErrValidation, "provider model name %q already exists", normalizedName)
		}
		if strings.EqualFold(strings.TrimSpace(item.Model), normalizedModel) {
			return errors.NewStatusF(code.ErrValidation, "provider model %q already exists", normalizedModel)
		}
	}
	return nil
}

func (s *platformService) providerWithOverrides(
	ctx context.Context,
	id string,
	baseURL string,
	authType string,
	credentialRef string,
) (*iapiserver.Provider, error) {
	provider, err := s.store.Providers().Get(ctx, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	ret := *provider
	ret.BaseURL = general.FallbackIfEmpty(strings.TrimSpace(baseURL), ret.BaseURL)
	ret.AuthType = general.FallbackIfEmpty(strings.TrimSpace(authType), ret.AuthType)
	ret.CredentialRef = general.FallbackIfMatch(credentialRef, ret.CredentialRef, func(credentialRef string) bool {
		return strings.TrimSpace(credentialRef) != "" && credentialRef != "configured"
	})
	return &ret, nil
}

func fetchOpenAICompatibleModels(ctx context.Context, provider *iapiserver.Provider) ([]string, error) {
	switch provider.Type {
	case iapiserver.ProviderTypeOpenAICompatible:
	default:
		return nil, errors.NewStatusF(code.ErrProviderUnsupported, "provider type %s is unsupported", provider.Type)
	}
	endpoint := openAICompatibleEndpoint(provider.BaseURL, "/models")
	builder := httpcli.NewHttpRequestBuilder().
		GET().
		WithTimeout(15 * time.Second).
		WithEndpoint(endpoint)
	if apiKey := firstProviderAPIKey(provider.CredentialRef); apiKey != "" {
		builder.AddHeaderParam("Authorization", "Bearer "+apiKey)
	}

	resp, err := builder.Build().Invoke()
	if err != nil {
		return nil, errors.WrapStatus(err, code.ErrProviderUnavailable)
	}

	if resp.GetStatusCode() == http.StatusUnauthorized || resp.GetStatusCode() == http.StatusForbidden {
		return nil, errors.NewStatusF(code.ErrProviderUnauthorized, "provider authentication failed")
	}
	if resp.GetStatusCode() < 200 || resp.GetStatusCode() >= 300 {
		return nil, errors.NewStatusF(
			code.ErrProviderUnavailable,
			"provider request failed with status %d",
			resp.GetStatusCode(),
		)
	}
	var decoded struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := resp.Decode(&decoded); err != nil {
		return nil, errors.WrapStatus(err, code.ErrProviderResponseParseError)
	}
	models := make([]string, 0, len(decoded.Data))
	for _, item := range decoded.Data {
		if strings.TrimSpace(item.ID) != "" {
			models = append(models, strings.TrimSpace(item.ID))
		}
	}
	if len(models) == 0 {
		return nil, errors.NewStatusF(code.ErrProviderResponseParseError, "provider returned no models")
	}
	sort.Strings(models)
	return models, nil
}

func openAICompatibleEndpoint(baseURL string, path string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "https://api.openai.com"
	}
	path = "/" + strings.TrimLeft(path, "/")
	if strings.HasSuffix(base, "/v1") {
		return base + path
	}
	return base + "/v1" + path
}

func firstProviderAPIKey(credentialRef string) string {
	ref := strings.TrimSpace(credentialRef)
	for _, item := range strings.Split(ref, ",") {
		if key := strings.TrimSpace(item); key != "" {
			return key
		}
	}
	return ""
}

func appendCapability(capabilities []string, capability string) []string {
	if slices.Contains(capabilities, capability) {
		return capabilities
	}
	return append(capabilities, capability)
}

func stringSliceEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy := append([]string(nil), left...)
	rightCopy := append([]string(nil), right...)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	for i := range leftCopy {
		if leftCopy[i] != rightCopy[i] {
			return false
		}
	}
	return true
}

func (s *platformService) AssetUpload(
	ctx context.Context,
	fileHeader *multipart.FileHeader,
	tagNames []string,
	sourceType string,
) (*iapiserver.AssetUploadResponse, error) {
	src, err := fileHeader.Open()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer src.Close()
	return s.createAssetFromReader(ctx, src, filepath.Base(fileHeader.Filename), tagNames, sourceType)
}

func (s *platformService) createAssetFromReader(
	ctx context.Context,
	src io.Reader,
	filename string,
	tagNames []string,
	sourceType string,
) (*iapiserver.AssetUploadResponse, error) {
	backend, err := s.ensureDefaultLocalBackend(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	assetID := uuid.New().String()
	filename = filepath.Base(filename)
	objectKey := filepath.ToSlash(filepath.Join("assets", time.Now().Format("2006/01"), assetID, filename))
	absPath, err := localObjectPath(backend, objectKey)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0750); err != nil {
		return nil, errors.WithStack(err)
	}

	dst, err := os.Create(absPath)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer dst.Close()
	assetPersisted := false
	// 本地对象存储无法加入 PostgreSQL 事务；素材或 outbox 回滚时需补偿删除完整文件。
	defer func() {
		if !assetPersisted {
			_ = os.Remove(absPath)
		}
	}()

	hasher := sha256.New()
	head := make([]byte, 512)
	n, readErr := src.Read(head)
	if readErr != nil && readErr != io.EOF {
		return nil, errors.WithStack(readErr)
	}
	mimeType := http.DetectContentType(head[:n])
	w := io.MultiWriter(dst, hasher)
	if n > 0 {
		if _, err := w.Write(head[:n]); err != nil {
			return nil, errors.WithStack(err)
		}
	}
	size, err := io.Copy(w, src)
	if err != nil {
		_ = dst.Close()
		return nil, errors.WithStack(err)
	}
	size += int64(n)
	if err := dst.Close(); err != nil {
		return nil, errors.WithStack(err)
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))

	mediaType, format := mediaTypeFromFile(mimeType, filename)
	width, height := imageDimensions(absPath)
	if sourceType == "" {
		sourceType = iapiserver.AssetSourceUserUpload
	}
	asset := &iapiserver.Asset{
		MediaType:        mediaType,
		MimeType:         mimeType,
		StorageBackendID: backend.ID,
		ObjectKey:        objectKey,
		Size:             size,
		Checksum:         checksum,
		Width:            width,
		Height:           height,
		Format:           format,
		SourceType:       sourceType,
		Metadata: map[string]any{
			"filename": filename,
		},
	}
	asset.ID = assetID
	asset.Name = safeObjectName(filename, "asset")
	thumbnail := &iapiserver.AssetThumbnail{
		AssetID:          asset.ID,
		StorageBackendID: backend.ID,
		Status:           thumbnailInitialStatus(mediaType),
	}
	thumbnail.Name = safeObjectName(asset.ID+"-thumbnail", "thumbnail")
	owner := iapiserver.DefaultTaskCenterCreatedBy
	if user, userErr := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser); userErr == nil && user != nil && user.ID != "" {
		owner = user.ID
	}
	created, thumb, err := s.store.AssetsV2().AddWithUploadEvent(ctx, asset, thumbnail, map[string]any{"asset_id": asset.ID, "owner_user_id": owner, "project_id": iapiserver.DefaultTaskCenterProjectID, "namespace": iapiserver.DefaultTaskCenterNamespace, "media_type": mediaType, "content_ref": objectKey, "profile_version": "v1", "occurred_at": time.Now().UTC().Format(time.RFC3339Nano)})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	assetPersisted = true
	if len(tagNames) > 0 {
		if err := s.replaceAssetTags(ctx, created.ID, tagNames, iapiserver.TagSourceUser); err != nil {
			return nil, err
		}
	}

	_ = thumb
	record, err := s.assetRecord(ctx, created)
	if err != nil {
		return nil, err
	}
	return &iapiserver.AssetUploadResponse{Asset: record, AtomicTasks: []*iapiserver.AtomicTask{}}, nil
}

func (s *platformService) AssetChunkUploadInit(
	_ context.Context,
	req *iapiserver.AssetChunkUploadInitRequest,
) (*iapiserver.AssetChunkUploadInitResponse, error) {
	if err := validateChunkUploadSpec(req.Checksum, req.ChunkSize, req.TotalChunks, req.Size); err != nil {
		return nil, err
	}
	dir, err := chunkUploadDir(req.Checksum)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, errors.WithStack(err)
	}
	if err := touchChunkUploadDir(dir); err != nil {
		return nil, err
	}
	uploaded, err := uploadedChunkIndexes(dir)
	if err != nil {
		return nil, err
	}
	return &iapiserver.AssetChunkUploadInitResponse{
		Checksum:       req.Checksum,
		UploadedChunks: uploaded,
		ChunkSize:      req.ChunkSize,
		TotalChunks:    req.TotalChunks,
		ExpiresHours:   chunkUploadCleanupHours,
	}, nil
}

func (s *platformService) AssetChunkUploadPart(
	_ context.Context,
	checksum string,
	index int,
	body io.Reader,
) (*iapiserver.AssetChunkUploadPartResponse, error) {
	if err := validateChecksum(checksum); err != nil {
		return nil, err
	}
	if index < 0 {
		return nil, errors.NewStatusF(code.ErrValidation, "chunk index must be greater than or equal to zero")
	}
	dir, err := chunkUploadDir(checksum)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, errors.WithStack(err)
	}
	tmpPath := filepath.Join(dir, fmt.Sprintf("%06d.part.tmp", index))
	partPath := filepath.Join(dir, chunkPartName(index))
	dst, err := os.Create(tmpPath)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	size, copyErr := io.Copy(dst, body)
	closeErr := dst.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return nil, errors.WithStack(copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return nil, errors.WithStack(closeErr)
	}
	if err := os.Rename(tmpPath, partPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, errors.WithStack(err)
	}
	if err := touchChunkUploadDir(dir); err != nil {
		return nil, err
	}
	return &iapiserver.AssetChunkUploadPartResponse{Checksum: checksum, Index: index, Size: size}, nil
}

func (s *platformService) AssetChunkUploadComplete(
	ctx context.Context,
	req *iapiserver.AssetChunkUploadCompleteRequest,
) (*iapiserver.AssetUploadResponse, error) {
	if err := validateChunkUploadSpec(req.Checksum, req.ChunkSize, req.TotalChunks, req.Size); err != nil {
		return nil, err
	}
	dir, err := chunkUploadDir(req.Checksum)
	if err != nil {
		return nil, err
	}
	mergedPath := filepath.Join(dir, "merged")
	merged, err := os.Create(mergedPath)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	hasher := sha256.New()
	writer := io.MultiWriter(merged, hasher)
	var total int64
	for i := 0; i < req.TotalChunks; i++ {
		partPath := filepath.Join(dir, chunkPartName(i))
		part, err := os.Open(partPath)
		if err != nil {
			_ = merged.Close()
			_ = os.Remove(mergedPath)
			return nil, errors.NewStatusF(code.ErrValidation, "missing chunk %d", i)
		}
		written, copyErr := io.Copy(writer, part)
		closeErr := part.Close()
		if copyErr != nil {
			_ = merged.Close()
			_ = os.Remove(mergedPath)
			return nil, errors.WithStack(copyErr)
		}
		if closeErr != nil {
			_ = merged.Close()
			_ = os.Remove(mergedPath)
			return nil, errors.WithStack(closeErr)
		}
		total += written
	}
	if err := merged.Close(); err != nil {
		_ = os.Remove(mergedPath)
		return nil, errors.WithStack(err)
	}
	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	if total != req.Size || actualChecksum != strings.ToLower(req.Checksum) {
		_ = os.Remove(mergedPath)
		return nil, errors.NewStatusF(code.ErrValidation, "merged file checksum or size mismatch")
	}
	src, err := os.Open(mergedPath)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer src.Close()
	resp, err := s.createAssetFromReader(ctx, src, req.Filename, req.TagNames, req.SourceType)
	if err != nil {
		return nil, err
	}
	_ = os.RemoveAll(dir)
	return resp, nil
}

func (s *platformService) AssetChunkUploadCancel(
	_ context.Context,
	checksum string,
) (*iapiserver.AssetChunkUploadCancelResponse, error) {
	if err := validateChecksum(checksum); err != nil {
		return nil, err
	}
	dir, err := chunkUploadDir(checksum)
	if err != nil {
		return nil, err
	}
	deleted := false
	if _, err := os.Stat(dir); err == nil {
		deleted = true
		if err := os.RemoveAll(dir); err != nil {
			return nil, errors.WithStack(err)
		}
	} else if !os.IsNotExist(err) {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.AssetChunkUploadCancelResponse{Checksum: checksum, Deleted: deleted}, nil
}

func (s *platformService) AssetList(
	ctx context.Context,
	req *iapiserver.AssetListRequest,
) (*iapiserver.AssetListResponse, error) {
	items, total, err := s.store.AssetsV2().List(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	records, err := s.assetRecords(ctx, items)
	if err != nil {
		return nil, err
	}
	return &iapiserver.AssetListResponse{ListRet: imachinery.ListRet{Total: total}, Assets: records}, nil
}

func (s *platformService) AssetSearch(
	ctx context.Context,
	req *iapiserver.AssetSearchRequest,
) (*iapiserver.AssetListResponse, error) {
	return s.AssetList(ctx, &req.Query)
}

func (s *platformService) AssetSearchParse(
	ctx context.Context,
	req *iapiserver.AssetSearchParseRequest,
) (*iapiserver.AssetSearchParseResponse, error) {
	query := parseNaturalAssetQuery(req.Text)
	run, err := s.createAtomicTask(ctx, atomicTaskSpec{
		DefinitionID:         "asset-search-parse",
		Name:                 "asset-search-parse",
		Description:          "Parse a natural-language asset search query.",
		FunctionRef:          "asset.search.parse",
		RequiredCapabilities: "asset.search.parse",
		Input:                map[string]any{"text": req.Text, "fallback_query": query},
		Tags:                 "asset,search",
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.AssetSearchParseResponse{Query: query, AtomicTaskID: run.ID}, nil
}

func (s *platformService) AssetGet(ctx context.Context, id string) (*iapiserver.AssetRecord, error) {
	asset, err := s.store.AssetsV2().Get(ctx, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return s.assetRecord(ctx, asset)
}

func (s *platformService) AssetUpdate(
	ctx context.Context,
	req *iapiserver.AssetUpdateRequest,
) (*iapiserver.AssetRecord, error) {
	asset, err := s.store.AssetsV2().Get(ctx, req.ID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	asset.Name = general.FallbackIfNil(req.Name, asset.Name)
	asset.SourceType = general.FallbackIfNil(req.SourceType, asset.SourceType)
	asset.SourceRef = general.FallbackIfNil(req.SourceRef, asset.SourceRef)
	asset.Metadata = general.FallbackIfNil(req.Metadata, asset.Metadata)
	asset.Description = general.FallbackIfNil(req.Description, asset.Description)

	updated, err := s.store.AssetsV2().Update(ctx, asset)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if req.TagNames != nil {
		source := req.TagSource
		if source == "" {
			source = iapiserver.TagSourceUser
		}
		if err := s.replaceAssetTags(ctx, updated.ID, *req.TagNames, source); err != nil {
			return nil, err
		}
	}
	return s.assetRecord(ctx, updated)
}

func (s *platformService) AssetDelete(ctx context.Context, id string) (*iapiserver.AssetRecord, error) {
	if err := s.store.AssetsV2().Delete(ctx, id); err != nil {
		return nil, errors.WithStack(err)
	}
	return s.AssetGet(ctx, id)
}

func (s *platformService) AssetContentPath(ctx context.Context, id string) (string, string, error) {
	asset, err := s.store.AssetsV2().Get(ctx, id)
	if err != nil {
		return "", "", errors.WithStack(err)
	}
	backend, err := s.store.StorageBackends().Get(ctx, asset.StorageBackendID)
	if err != nil {
		return "", "", errors.WithStack(err)
	}
	path, err := localObjectPath(backend, asset.ObjectKey)
	if err != nil {
		return "", "", err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", "", errors.NewStatusF(code.ErrPageNotFound, "asset content not found")
		}
		return "", "", errors.WithStack(err)
	}
	return path, asset.MimeType, nil
}

func (s *platformService) AssetThumbnailPath(ctx context.Context, id string) (string, string, error) {
	thumb, err := s.store.AssetThumbnails().GetByAsset(ctx, id)
	if err != nil {
		return "", "", errors.WithStack(err)
	}
	if thumb.Status != iapiserver.ThumbnailStatusReady || thumb.ObjectKey == "" {
		return "", "", errors.NewStatusF(code.ErrValidation, "thumbnail is not ready")
	}
	backend, err := s.store.StorageBackends().Get(ctx, thumb.StorageBackendID)
	if err != nil {
		return "", "", errors.WithStack(err)
	}
	path, err := localObjectPath(backend, thumb.ObjectKey)
	if err != nil {
		return "", "", err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", "", errors.NewStatusF(code.ErrPageNotFound, "thumbnail content not found")
		}
		return "", "", errors.WithStack(err)
	}
	return path, thumb.MimeType, nil
}

func (s *platformService) AssetGroupCreate(
	ctx context.Context,
	req *iapiserver.AssetGroupCreateRequest,
) (*iapiserver.AssetGroupCreateResponse, error) {
	groupType := req.Type
	if groupType == "" {
		groupType = iapiserver.AssetGroupTypeCollection
	}
	group := &iapiserver.AssetGroup{Type: groupType, DynamicRule: req.DynamicRule}
	group.Name = req.Name
	group.Description = req.Description
	created, err := s.store.AssetGroups().Add(ctx, group)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	members := make([]*iapiserver.AssetGroupMember, 0, len(req.AssetIDs))
	for _, assetID := range req.AssetIDs {
		member := &iapiserver.AssetGroupMember{GroupID: created.ID, AssetID: assetID}
		member.Name = "group-member"
		members = append(members, member)
	}
	createdMembers, err := s.store.AssetGroupMembers().BatchAdd(ctx, members)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.AssetGroupCreateResponse{Group: created, Members: createdMembers}, nil
}

func (s *platformService) CanvasAssetRegisterOutput(
	ctx context.Context,
	req *iapiserver.CanvasAssetRegisterOutputRequest,
) (*iapiserver.CanvasAssetRegisterOutputResponse, error) {
	record, err := s.AssetGet(ctx, req.AssetID)
	if err != nil {
		return nil, err
	}
	run, err := s.createAtomicTask(ctx, atomicTaskSpec{
		DefinitionID:         "canvas-output-register",
		Name:                 "canvas-output-register",
		Description:          "Register a canvas output asset reference.",
		FunctionRef:          "canvas.output.register",
		RequiredCapabilities: "canvas.output.register",
		Input: map[string]any{
			"canvas_id": req.CanvasID,
			"node_id":   req.NodeID,
			"asset_id":  req.AssetID,
			"metadata":  req.Metadata,
		},
		Tags: "canvas,asset",
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.CanvasAssetRegisterOutputResponse{Asset: record, AtomicTask: run}, nil
}

func (s *platformService) CanvasAssetDownloadZip(
	ctx context.Context,
	req *iapiserver.CanvasAssetDownloadRequest,
	dst io.Writer,
) error {
	items := req.Items
	for _, assetID := range req.AssetIDs {
		items = append(items, iapiserver.CanvasAssetDownloadItem{AssetID: assetID})
	}
	if len(items) == 0 {
		return errors.NewStatusF(code.ErrValidation, "asset_ids or items is required")
	}
	zw := zip.NewWriter(dst)
	defer zw.Close()
	usedNames := map[string]int{}
	for index, item := range items {
		asset, err := s.store.AssetsV2().Get(ctx, item.AssetID)
		if err != nil {
			return errors.WithStack(err)
		}
		if asset.DeletedAt > 0 {
			return errors.NewStatusF(code.ErrValidation, "asset %s has been deleted", item.AssetID)
		}
		path, _, err := s.AssetContentPath(ctx, asset.ID)
		if err != nil {
			return err
		}
		name := item.Name
		if name == "" {
			name = asset.Name
		}
		if name == "" {
			name = fmt.Sprintf("asset-%02d", index+1)
		}
		name = uniqueZipName(usedNames, safeObjectName(name, fmt.Sprintf("asset-%02d", index+1)))
		if err := addFileToZip(zw, path, name); err != nil {
			return err
		}
	}
	return nil
}

func (s *platformService) CanvasNodeRun(
	ctx context.Context,
	canvasID, nodeID string,
	req *iapiserver.CanvasNodeRunRequest,
) (*iapiserver.CanvasRunResponse, error) {
	functionRef := canvasNodeFunctionRef(req.Node)
	run, err := s.createAtomicTask(ctx, atomicTaskSpec{
		DefinitionID:         safeTaskDefinitionID(functionRef),
		Name:                 "canvas-node-run",
		Description:          "Execute one canvas node through Task Center.",
		FunctionRef:          functionRef,
		RequiredCapabilities: functionRef,
		Input: map[string]any{
			"canvas_id": canvasID,
			"node_id":   nodeID,
			"node":      req.Node,
			"settings":  req.Settings,
		},
		Tags: "canvas,node",
	})
	if err != nil {
		return nil, err
	}
	return &iapiserver.CanvasRunResponse{AtomicTask: run}, nil
}

func (s *platformService) ensureDefaultLocalBackend(ctx context.Context) (*iapiserver.StorageBackend, error) {
	backend, err := s.store.StorageBackends().GetDefaultLocal(ctx)
	if err == nil {
		return backend, nil
	}
	root, err := normalizeLocalRoot("")
	if err != nil {
		return nil, err
	}
	backend = &iapiserver.StorageBackend{
		Type:    iapiserver.StorageBackendTypeLocal,
		Root:    root,
		Enabled: true,
	}
	backend.Name = "default-local"
	return s.store.StorageBackends().Add(ctx, backend)
}

func (s *platformService) assetRecord(ctx context.Context, asset *iapiserver.Asset) (*iapiserver.AssetRecord, error) {
	records, err := s.assetRecords(ctx, []*iapiserver.Asset{asset})
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, errors.Errorf("asset record not found")
	}
	return records[0], nil
}

func (s *platformService) assetRecords(
	ctx context.Context,
	assets []*iapiserver.Asset,
) ([]*iapiserver.AssetRecord, error) {
	ids := make([]string, 0, len(assets))
	for _, asset := range assets {
		ids = append(ids, asset.ID)
	}
	thumbnails, err := s.store.AssetThumbnails().ListByAssetIDs(ctx, ids)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	thumbnailMap := map[string]*iapiserver.AssetThumbnail{}
	for _, thumbnail := range thumbnails {
		thumbnailMap[thumbnail.AssetID] = thumbnail
	}
	tagMap, err := s.store.Tags().ListByAssetIDs(ctx, ids)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	records := make([]*iapiserver.AssetRecord, 0, len(assets))
	for _, asset := range assets {
		records = append(records, &iapiserver.AssetRecord{
			Asset:     asset,
			Thumbnail: thumbnailMap[asset.ID],
			Tags:      tagMap[asset.ID],
		})
	}
	return records, nil
}

func (s *platformService) replaceAssetTags(
	ctx context.Context,
	assetID string,
	tagNames []string,
	source string,
) error {
	tags := make([]*iapiserver.Tag, 0, len(tagNames))
	for _, name := range tagNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		tag := &iapiserver.Tag{Source: source}
		tag.Name = name
		created, err := s.store.Tags().FirstOrCreate(ctx, tag)
		if err != nil {
			return errors.WithStack(err)
		}
		tags = append(tags, created)
	}
	return s.store.AssetTags().Replace(ctx, assetID, tags, source)
}

type atomicTaskSpec struct {
	DefinitionID         string
	Name                 string
	Description          string
	FunctionRef          string
	RequiredCapabilities string
	Input                map[string]any
	Tags                 string
}

func (s *platformService) createAtomicTask(ctx context.Context, spec atomicTaskSpec) (*iapiserver.AtomicTask, error) {
	task := &iapiserver.AtomicTask{
		FunctionRef: spec.FunctionRef, Arguments: spec.Input, RequiredCapabilities: spec.RequiredCapabilities,
		Status: iapiserver.AtomicTaskStatusPending, ProjectID: iapiserver.DefaultTaskCenterProjectID,
		Namespace: iapiserver.DefaultTaskCenterNamespace, CreatedBy: iapiserver.DefaultTaskCenterCreatedBy,
		ChildKey: spec.DefinitionID, Tags: spec.Tags,
	}
	task.ID = uuid.NewString()
	task.RootTaskID = task.ID
	task.Name = spec.Name
	task.Description = spec.Description
	created, _, err := s.store.TaskCenters().AddAtomicTaskIdempotent(ctx, task)
	return created, errors.WithStack(err)
}

func defaultFeatureFlags() map[string]bool {
	return map[string]bool{
		"provider.deepseek":   true,
		"asset.upload":        true,
		"asset.smart_tagging": true,
		"asset.thumbnail":     true,
		"asset.search.parse":  true,
		"canvas.classic":      true,
		"canvas.smart":        true,
		"canvas.execute":      true,
		"ai_chat_system":      true,
		"task.async":          true,
		"storage.local":       true,
	}
}

func defaultPermissions() []string {
	return []string{
		"asset.create",
		"asset.read",
		"asset.update",
		"asset.delete",
		"asset.search",
		"asset.group.create",
		"asset.upload",
		"asset.label.manage",
		"asset.collection.read",
		"asset.collection.manage",
		"asset.artifact.read",
		"asset.artifact.delete",
		"asset.artifact.register",
		"asset.content.read",
		"asset.representation.read",
		"asset.reference.read",
		"asset.storage.read",
		"asset.storage.manage",
		"provider.manage",
		"storage.manage",
		"MODEL_CONFIG_READ",
		"MODEL_CONFIG_WRITE",
		"MODEL_HEALTH_TEST",
		"MODEL_DEFAULT_WRITE",
		"task.definition.manage",
		"task.run.operate",
		"task.worker.protocol",
		"task.atomic.operate",
		"task.group.operate",
		"task.schedule.manage",
		"task.operation.admin",
		"sse.stream.read",
		"sse.history.read",
		"workflow.node_definition.read",
		"workflow.node_definition.manage",
		"workflow.canvas.read",
		"workflow.canvas.edit",
		"workflow.canvas.publish",
		"workflow.canvas.delete",
		"workflow.run.create",
		"workflow.run.read",
		"workflow.run.cancel",
		"workflow.run.retry",
		"notification.inbox.read",
		"notification.inbox.manage",
		"notification.preference.read",
		"notification.preference.manage",
		"feature.manage",
		iapiserver.AIAppProviderCapabilityRead,
		iapiserver.AIAppProviderCapabilityReadDiagnostics,
		iapiserver.AIAppEngineInstanceRead,
		iapiserver.AIAppEngineInstanceManage,
		iapiserver.AIAppEngineBindingManage,
		iapiserver.AIAppComfyUIWorkflowRead,
		iapiserver.AIAppComfyUIWorkflowManage,
		iapiserver.AIAppComfyUIWorkflowValidate,
		iapiserver.AIAppComfyUIWorkflowConvert,
		iapiserver.AIAppComfyUIWorkflowTest,
		iapiserver.AIAppApplicationRead,
		iapiserver.AIAppApplicationManage,
		iapiserver.AIAppApplicationRun,
	}
}

func appendUnique(items []string, item string) []string {
	if sets.NewString(items...).Has(item) {
		return items
	}
	return append(items, item)
}

func (s *platformService) ensureProviderNameUnique(
	ctx context.Context,
	ownerUserID string,
	name string,
	excludeID string,
) error {
	items, _, err := s.store.Providers().List(ctx, &iapiserver.ProviderListRequest{OwnerUserID: ownerUserID})
	if err != nil {
		return errors.WithStack(err)
	}
	for _, item := range items {
		if item.Name == name && item.ID != excludeID {
			return errors.NewStatusF(code.ErrModelProviderNameDuplicated, "provider name %s already exists", name)
		}
	}
	return nil
}

func currentPlatformUserID(ctx context.Context) (string, error) {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err != nil || user == nil || user.ID == "" {
		return "system-admin", nil
	}
	return user.ID, nil
}

func isDefaultModelUsage(usage string) bool {
	switch usage {
	case "assistant.default", "quick", "translation":
		return true
	default:
		return false
	}
}

func normalizeLocalRoot(root string) (string, error) {
	if root == "" {
		root = os.Getenv("OMNIMAM_STORAGE_ROOT")
	}
	if root == "" {
		root = filepath.Join("data", "assets")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return filepath.Clean(abs), nil
}

func localObjectPath(backend *iapiserver.StorageBackend, objectKey string) (string, error) {
	if backend.Type != iapiserver.StorageBackendTypeLocal {
		return "", errors.Errorf("storage backend %s is not local", backend.ID)
	}
	root, err := normalizeLocalRoot(backend.Root)
	if err != nil {
		return "", err
	}
	cleanKey := filepath.Clean(filepath.FromSlash(objectKey))
	if filepath.IsAbs(cleanKey) || strings.HasPrefix(cleanKey, ".."+string(filepath.Separator)) || cleanKey == ".." {
		return "", errors.Errorf("invalid object key")
	}
	path := filepath.Join(root, cleanKey)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", errors.WithStack(err)
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", errors.Errorf("object key escapes storage root")
	}
	return path, nil
}

func validateChunkUploadSpec(checksum string, chunkSize int64, totalChunks int, size int64) error {
	if err := validateChecksum(checksum); err != nil {
		return err
	}
	if chunkSize <= 0 {
		return errors.NewStatusF(code.ErrValidation, "chunk_size must be greater than zero")
	}
	if totalChunks <= 0 {
		return errors.NewStatusF(code.ErrValidation, "total_chunks must be greater than zero")
	}
	if size <= 0 {
		return errors.NewStatusF(code.ErrValidation, "size must be greater than zero")
	}
	expectedChunks := int((size + chunkSize - 1) / chunkSize)
	if expectedChunks != totalChunks {
		return errors.NewStatusF(code.ErrValidation, "total_chunks does not match size and chunk_size")
	}
	return nil
}

func validateChecksum(checksum string) error {
	if len(checksum) != 64 {
		return errors.NewStatusF(code.ErrValidation, "checksum must be a sha256 hex string")
	}
	for _, r := range checksum {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return errors.NewStatusF(code.ErrValidation, "checksum must be a sha256 hex string")
		}
	}
	return nil
}

func chunkUploadDir(checksum string) (string, error) {
	if err := validateChecksum(checksum); err != nil {
		return "", err
	}
	root, err := filepath.Abs(chunkUploadTempDir)
	if err != nil {
		return "", errors.WithStack(err)
	}
	target := filepath.Join(root, strings.ToLower(checksum))
	if !strings.HasPrefix(target, root+string(os.PathSeparator)) && target != root {
		return "", errors.NewStatusF(code.ErrValidation, "invalid chunk upload path")
	}
	return target, nil
}

func chunkPartName(index int) string {
	return fmt.Sprintf("%06d.part", index)
}

func uploadedChunkIndexes(dir string) ([]int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []int{}, nil
		}
		return nil, errors.WithStack(err)
	}
	indexes := []int{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".part") {
			continue
		}
		raw := strings.TrimSuffix(entry.Name(), ".part")
		index, err := strconv.Atoi(raw)
		if err == nil {
			indexes = append(indexes, index)
		}
	}
	sort.Ints(indexes)
	return indexes, nil
}

func touchChunkUploadDir(dir string) error {
	now := time.Now()
	if err := os.Chtimes(dir, now, now); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func cleanupChunkUploadDirs(ttl time.Duration) {
	entries, err := os.ReadDir(chunkUploadTempDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-ttl)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(chunkUploadTempDir, entry.Name()))
		}
	}
}

func mediaTypeFromFile(mimeType string, filename string) (string, string) {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return iapiserver.AssetMediaTypeImage, ext
	case strings.HasPrefix(mimeType, "video/"):
		return iapiserver.AssetMediaTypeVideo, ext
	case ext == "mp4" || ext == "webm" || ext == "mov" || ext == "mkv" || ext == "avi":
		return iapiserver.AssetMediaTypeVideo, ext
	case strings.HasPrefix(mimeType, "audio/"):
		return iapiserver.AssetMediaTypeAudio, ext
	case ext == "mp3" || ext == "wav" || ext == "flac" || ext == "m4a" || ext == "ogg":
		return iapiserver.AssetMediaTypeAudio, ext
	case mimeType == "application/pdf" || ext == "pdf":
		return iapiserver.AssetMediaTypePDF, ext
	case ext == "json":
		return iapiserver.AssetMediaTypeJSON, ext
	case ext == "md" || ext == "markdown":
		return iapiserver.AssetMediaTypeMarkdown, ext
	case strings.HasPrefix(mimeType, "text/"):
		return iapiserver.AssetMediaTypeText, ext
	default:
		return iapiserver.AssetMediaTypeOther, ext
	}
}

func imageDimensions(path string) (int, int) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer file.Close()
	cfg, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

func thumbnailInitialStatus(mediaType string) string {
	switch mediaType {
	case iapiserver.AssetMediaTypeImage, iapiserver.AssetMediaTypeVideo:
		return iapiserver.ThumbnailStatusPending
	default:
		return iapiserver.ThumbnailStatusUnsupported
	}
}

func parseNaturalAssetQuery(text string) iapiserver.AssetListRequest {
	lower := strings.ToLower(text)
	query := iapiserver.AssetListRequest{}
	switch {
	case strings.Contains(text, "图片") || strings.Contains(lower, "image"):
		query.MediaType = iapiserver.AssetMediaTypeImage
	case strings.Contains(text, "视频") || strings.Contains(lower, "video"):
		query.MediaType = iapiserver.AssetMediaTypeVideo
	case strings.Contains(text, "音频") || strings.Contains(lower, "audio"):
		query.MediaType = iapiserver.AssetMediaTypeAudio
	case strings.Contains(lower, "pdf"):
		query.MediaType = iapiserver.AssetMediaTypePDF
	case strings.Contains(lower, "json"):
		query.MediaType = iapiserver.AssetMediaTypeJSON
	case strings.Contains(lower, "markdown") || strings.Contains(lower, ".md"):
		query.MediaType = iapiserver.AssetMediaTypeMarkdown
	}
	if strings.Contains(lower, "prompt") || strings.Contains(text, "提示词") {
		query.MediaType = iapiserver.AssetMediaTypePrompt
	}
	if strings.Contains(lower, "template") || strings.Contains(text, "模板") || strings.Contains(lower, "ideogram4") {
		query.MediaType = iapiserver.AssetMediaTypePromptTemplate
	}
	if strings.Contains(lower, "deleted") || strings.Contains(text, "已删除") || strings.Contains(text, "回收站") {
		query.Status = "deleted"
	}
	re := regexp.MustCompile(`(\d{2,5})\s*[x×*]\s*(\d{2,5})`)
	if match := re.FindStringSubmatch(text); len(match) == 3 {
		_, _ = fmt.Sscanf(match[1], "%d", &query.Width)
		_, _ = fmt.Sscanf(match[2], "%d", &query.Height)
	}
	resolutionRe := regexp.MustCompile(`(?i)(480|720|1080|1440|2160)p`)
	if match := resolutionRe.FindStringSubmatch(text); len(match) == 2 &&
		query.MediaType == iapiserver.AssetMediaTypeVideo {
		_, _ = fmt.Sscanf(match[1], "%d", &query.Height)
	}
	query.Keyword = strings.TrimSpace(text)
	return query
}

func safeObjectName(name, fallback string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = fallback
	}
	name = strings.ReplaceAll(name, "\x00", "")
	if len([]rune(name)) <= 60 {
		return name
	}
	runes := []rune(name)
	return string(runes[:60])
}

func uniqueZipName(used map[string]int, name string) string {
	name = strings.Trim(strings.ReplaceAll(name, "\\", "/"), "/")
	if name == "" {
		name = "asset"
	}
	used[name]++
	if used[name] == 1 {
		return name
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return fmt.Sprintf("%s-%d%s", base, used[name], ext)
}

func addFileToZip(zw *zip.Writer, path string, name string) error {
	src, err := os.Open(path)
	if err != nil {
		return errors.WithStack(err)
	}
	defer src.Close()
	info, err := src.Stat()
	if err != nil {
		return errors.WithStack(err)
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return errors.WithStack(err)
	}
	header.Name = name
	header.Method = zip.Deflate
	dst, err := zw.CreateHeader(header)
	if err != nil {
		return errors.WithStack(err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func canvasNodeFunctionRef(node map[string]any) string {
	nodeType, _ := node["type"].(string)
	switch nodeType {
	case "llm", "smart-prompt":
		return "canvas.node.llm"
	case "generator":
		return "canvas.node.image_generate"
	case "video":
		return "canvas.node.video_generate"
	case "comfy":
		return "canvas.node.comfy"
	case "rh":
		return "canvas.node.runninghub"
	case "msgen":
		return "canvas.node.modelscope"
	case "ltxDirector":
		return "canvas.node.ltx_director"
	default:
		return "canvas.node.run"
	}
}

func safeTaskDefinitionID(functionRef string) string {
	id := strings.ToLower(strings.ReplaceAll(functionRef, ".", "-"))
	id = strings.Trim(id, "-")
	if id == "" {
		return "task-run"
	}
	if len(id) <= 64 {
		return id
	}
	return id[:64]
}
