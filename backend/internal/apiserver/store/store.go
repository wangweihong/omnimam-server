package store

import (
	"context"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

type IdentityProviderStore interface {
	List(
		ctx context.Context,
		req *iapiserver.IdentityProviderListRequest,
	) ([]*iapiserver.IdentityProvider, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.IdentityProvider, error)
	GetByName(ctx context.Context, name string) (*iapiserver.IdentityProvider, error)
	Delete(ctx context.Context, id string) error
	Update(ctx context.Context, data *iapiserver.IdentityProvider) (*iapiserver.IdentityProvider, error)
	Sync(ctx context.Context, datas []*iapiserver.IdentityProvider) error
	Add(ctx context.Context, data *iapiserver.IdentityProvider) (*iapiserver.IdentityProvider, error)
}

type ServiceProviderStore interface {
	List(ctx context.Context, req *iapiserver.ServiceProviderListRequest) ([]*iapiserver.ServiceProvider, int64, error)
	Add(ctx context.Context, data *iapiserver.ServiceProvider) (*iapiserver.ServiceProvider, error)
	Delete(ctx context.Context, id string) error
	Get(ctx context.Context, id string) (*iapiserver.ServiceProvider, error)
	GetByKey(ctx context.Context, protocol, key string) (*iapiserver.ServiceProvider, error)
	GetByName(ctx context.Context, name string) (*iapiserver.ServiceProvider, error)
	Update(ctx context.Context, data *iapiserver.ServiceProvider) (*iapiserver.ServiceProvider, error)
	Sync(ctx context.Context, datas []*iapiserver.ServiceProvider) error
}

type SettingStore interface {
	List(ctx context.Context) ([]*iapiserver.Setting, error)
	Delete(ctx context.Context, id string) error
	Get(ctx context.Context, id string) (*iapiserver.Setting, error)
	GetByName(ctx context.Context, name string) (*iapiserver.Setting, error)
	GetMultiByNames(ctx context.Context, names ...string) ([]*iapiserver.Setting, error)
	Upsert(ctx context.Context, data *iapiserver.Setting) (*iapiserver.Setting, error)
	FirstOrCreate(ctx context.Context, data *iapiserver.Setting) (*iapiserver.Setting, error)
}

type UserStore interface {
	List(ctx context.Context, req *iapiserver.UserListRequest) ([]*iapiserver.User, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.User, error)
	GetByName(ctx context.Context, name string) (*iapiserver.User, error)
	Delete(ctx context.Context, id string) error
	Update(ctx context.Context, data *iapiserver.User) (*iapiserver.User, error)
	Sync(ctx context.Context, datas []*iapiserver.User) error
	Add(ctx context.Context, data *iapiserver.User) (*iapiserver.User, error)
}

type OneTimeTokenStore interface {
	GetByHash(ctx context.Context, hash string) (*iapiserver.OneTimeToken, error)
	Delete(ctx context.Context, id string) error
	Add(ctx context.Context, data *iapiserver.OneTimeToken) (*iapiserver.OneTimeToken, error)
	CleanupExpiredTokens(ctx context.Context) error
}

type UserOTPStore interface {
	List(ctx context.Context) ([]*iapiserver.UserOTP, error)
	Delete(ctx context.Context, id string) error
	GetByUser(ctx context.Context, uid string) (*iapiserver.UserOTP, error)
	Upsert(ctx context.Context, data *iapiserver.UserOTP) (*iapiserver.UserOTP, error)
	FirstOrCreate(ctx context.Context, data *iapiserver.UserOTP) (*iapiserver.UserOTP, error)
	Add(ctx context.Context, data *iapiserver.UserOTP) (*iapiserver.UserOTP, error)
}

type AssetLibraryStore interface {
	List(ctx context.Context, req *iapiserver.AssetLibraryListRequest) ([]*iapiserver.AssetLibrary, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.AssetLibrary, error)
	Add(ctx context.Context, data *iapiserver.AssetLibrary) (*iapiserver.AssetLibrary, error)
	Update(ctx context.Context, data *iapiserver.AssetLibrary) (*iapiserver.AssetLibrary, error)
	Delete(ctx context.Context, id string) error
}

type AssetCategoryStore interface {
	List(ctx context.Context, req *iapiserver.AssetCategoryListRequest) ([]*iapiserver.AssetCategory, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.AssetCategory, error)
	Add(ctx context.Context, data *iapiserver.AssetCategory) (*iapiserver.AssetCategory, error)
	Update(ctx context.Context, data *iapiserver.AssetCategory) (*iapiserver.AssetCategory, error)
	Delete(ctx context.Context, id string, libraryID string) error
	DeleteByLibraryID(ctx context.Context, libraryID string) error
}

type AssetItemStore interface {
	List(ctx context.Context, req *iapiserver.AssetItemListRequest) ([]*iapiserver.AssetItem, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.AssetItem, error)
	Add(ctx context.Context, data *iapiserver.AssetItem) (*iapiserver.AssetItem, error)
	BatchAdd(ctx context.Context, items []*iapiserver.AssetItem) ([]*iapiserver.AssetItem, error)
	Update(ctx context.Context, data *iapiserver.AssetItem) (*iapiserver.AssetItem, error)
	Delete(ctx context.Context, id string) error
	BatchDelete(ctx context.Context, ids []string, libraryID string) (int, error)
	BatchMove(ctx context.Context, ids []string, targetLibraryID, targetCategoryID string) (int, error)
	FindByIDs(ctx context.Context, ids []string, libraryID string) ([]*iapiserver.AssetItem, error)
}

type PromptLibraryStore interface {
	List(ctx context.Context) ([]*iapiserver.PromptLibrary, error)
	Get(ctx context.Context, id string) (*iapiserver.PromptLibrary, error)
	Add(ctx context.Context, data *iapiserver.PromptLibrary) (*iapiserver.PromptLibrary, error)
	Update(ctx context.Context, data *iapiserver.PromptLibrary) (*iapiserver.PromptLibrary, error)
	Delete(ctx context.Context, id string) error
	SetActive(ctx context.Context, id string) error
	GetActive(ctx context.Context) (*iapiserver.PromptLibrary, error)
}

type PromptCategoryStore interface {
	ListByLibrary(ctx context.Context, libraryID string) ([]*iapiserver.PromptCategory, error)
	Get(ctx context.Context, id string) (*iapiserver.PromptCategory, error)
	Add(ctx context.Context, data *iapiserver.PromptCategory) (*iapiserver.PromptCategory, error)
	Update(ctx context.Context, data *iapiserver.PromptCategory) (*iapiserver.PromptCategory, error)
	Delete(ctx context.Context, id string, libraryID string) error
	DeleteByLibraryID(ctx context.Context, libraryID string) error
}

type PromptItemStore interface {
	ListByLibrary(ctx context.Context, libraryID string) ([]*iapiserver.PromptItem, error)
	Get(ctx context.Context, id string) (*iapiserver.PromptItem, error)
	Add(ctx context.Context, data *iapiserver.PromptItem) (*iapiserver.PromptItem, error)
	Update(ctx context.Context, data *iapiserver.PromptItem) (*iapiserver.PromptItem, error)
	Delete(ctx context.Context, id string) error
	BatchDelete(ctx context.Context, ids []string) (int, error)
	ReassignCategory(ctx context.Context, oldCategoryID, newCategoryID string) error
}

type ProjectStore interface {
	List(ctx context.Context) ([]*iapiserver.Project, error)
	Get(ctx context.Context, id string) (*iapiserver.Project, error)
	Add(ctx context.Context, data *iapiserver.Project) (*iapiserver.Project, error)
	Update(ctx context.Context, data *iapiserver.Project) (*iapiserver.Project, error)
	Delete(ctx context.Context, id string) error
}

type CanvasStore interface {
	List(ctx context.Context, includeDeleted bool) ([]*iapiserver.Canvas, error)
	ListByProject(ctx context.Context, projectID string) ([]*iapiserver.Canvas, error)
	Get(ctx context.Context, id string) (*iapiserver.Canvas, error)
	GetAny(ctx context.Context, id string) (*iapiserver.Canvas, error)
	Add(ctx context.Context, data *iapiserver.Canvas) (*iapiserver.Canvas, error)
	Update(ctx context.Context, data *iapiserver.Canvas) (*iapiserver.Canvas, error)
	SoftDelete(ctx context.Context, id string) error
	Restore(ctx context.Context, id string) error
	Purge(ctx context.Context, id string) error
	CountByProject(ctx context.Context, projectID string) (int, error)
	ReassignProject(ctx context.Context, oldProjectID, newProjectID string) (int, error)
	CleanupExpiredTrash(ctx context.Context, retentionDays int) error
}

// WorkflowCanvasStore owns spec-v1.7.0 node definitions, drafts, immutable versions, and run projections.
type WorkflowCanvasStore interface {
	ListWorkflowNodeDefinitions(
		context.Context,
		*iapiserver.WorkflowNodeDefinitionListRequest,
		string,
		string,
	) ([]*iapiserver.WorkflowNodeDefinition, int64, error)
	GetWorkflowNodeDefinition(context.Context, string, string, string, string, bool) (*iapiserver.WorkflowNodeDefinition, error)
	AddWorkflowNodeDefinitionIdempotent(context.Context, *iapiserver.WorkflowNodeDefinition) (*iapiserver.WorkflowNodeDefinition, bool, error)
	DeprecateWorkflowNodeDefinition(context.Context, string, string, string) (*iapiserver.WorkflowNodeDefinition, error)
	ListWorkflowCanvases(context.Context, *iapiserver.WorkflowCanvasListRequest, string, string, string) ([]*iapiserver.WorkflowCanvas, int64, error)
	GetWorkflowCanvas(context.Context, string) (*iapiserver.WorkflowCanvas, error)
	GetWorkflowCanvasesByIDs(context.Context, []string) ([]*iapiserver.WorkflowCanvas, error)
	AddWorkflowCanvas(context.Context, *iapiserver.WorkflowCanvas) (*iapiserver.WorkflowCanvas, error)
	UpdateWorkflowCanvas(context.Context, *iapiserver.WorkflowCanvas, int64) (*iapiserver.WorkflowCanvas, error)
	DeleteWorkflowCanvas(context.Context, string) error
	PublishWorkflowCanvas(context.Context, *iapiserver.WorkflowCanvas, *iapiserver.CanvasVersion, int64) (*iapiserver.CanvasVersion, error)
	ListCanvasVersions(context.Context, *iapiserver.CanvasVersionListRequest) ([]*iapiserver.CanvasVersion, int64, error)
	GetCanvasVersion(context.Context, string) (*iapiserver.CanvasVersion, error)
	GetCanvasVersionsByIDs(context.Context, []string) ([]*iapiserver.CanvasVersion, error)
	ListWorkflowCanvasRuns(context.Context, *iapiserver.WorkflowCanvasRunListRequest, string, string, string) ([]*iapiserver.WorkflowCanvasRun, int64, error)
	GetWorkflowCanvasRun(context.Context, string) (*iapiserver.WorkflowCanvasRun, error)
	GetWorkflowCanvasRunsByIDs(context.Context, []string) ([]*iapiserver.WorkflowCanvasRun, error)
	AddWorkflowCanvasRunIdempotent(context.Context, *iapiserver.WorkflowCanvasRun) (*iapiserver.WorkflowCanvasRun, bool, error)
	BindWorkflowCanvasRun(
		context.Context,
		string,
		string,
		[]*iapiserver.CanvasFlowRun,
		[]*iapiserver.CanvasNodeRun,
		[]*iapiserver.CanvasNodeRunTaskBinding,
		[]*iapiserver.CanvasNodeRunOutputBinding,
	) (*iapiserver.WorkflowCanvasRun, error)
	UpdateWorkflowCanvasRun(context.Context, *iapiserver.WorkflowCanvasRun) (*iapiserver.WorkflowCanvasRun, error)
	ListCanvasFlowRuns(context.Context, *iapiserver.CanvasFlowRunListRequest) ([]*iapiserver.CanvasFlowRun, int64, error)
	ListCanvasNodeRuns(context.Context, *iapiserver.CanvasNodeRunListRequest) ([]*iapiserver.CanvasNodeRun, int64, error)
	GetCanvasNodeRun(context.Context, string) (*iapiserver.CanvasNodeRun, error)
	GetCanvasNodeRunDetail(context.Context, string) ([]*iapiserver.CanvasNodeRunTaskBinding, []*iapiserver.CanvasNodeRunOutputBinding, error)
}

type ProviderStore interface {
	List(ctx context.Context, req *iapiserver.ProviderListRequest) ([]*iapiserver.Provider, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.Provider, error)
	// GetByIDs 按当前所有者边界批量读取 provider，供跨领域一跳投影使用。
	GetByIDs(ctx context.Context, ownerUserID string, ids []string) ([]*iapiserver.Provider, error)
	Add(ctx context.Context, data *iapiserver.Provider) (*iapiserver.Provider, error)
	Update(ctx context.Context, data *iapiserver.Provider) (*iapiserver.Provider, error)
	// Delete removes one provider record by id.
	Delete(ctx context.Context, id string) error
}

type ProviderModelStore interface {
	List(ctx context.Context, req *iapiserver.ProviderModelListRequest) ([]*iapiserver.ProviderModel, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.ProviderModel, error)
	// GetByIDs 按当前所有者边界批量读取模型，禁止消费方逐 ID 查询。
	GetByIDs(ctx context.Context, ownerUserID string, ids []string) ([]*iapiserver.ProviderModel, error)
	Add(ctx context.Context, data *iapiserver.ProviderModel) (*iapiserver.ProviderModel, error)
	Update(ctx context.Context, data *iapiserver.ProviderModel) (*iapiserver.ProviderModel, error)
	// Delete 删除指定模型提供商下的一个模型元数据。
	Delete(ctx context.Context, providerID, id string) error
	// DeleteByProviderID removes all models under one provider.
	DeleteByProviderID(ctx context.Context, providerID string) error
}

type ProviderCapabilityStore interface {
	List(ctx context.Context) ([]*iapiserver.ProviderCapability, error)
	Add(ctx context.Context, data *iapiserver.ProviderCapability) (*iapiserver.ProviderCapability, error)
}

type SystemLLMConfigStore interface {
	List(ctx context.Context) ([]*iapiserver.SystemLLMConfig, error)
	Upsert(ctx context.Context, data *iapiserver.SystemLLMConfig) (*iapiserver.SystemLLMConfig, error)
	// DeleteByProviderModelID 删除引用指定模型的默认模型绑定。
	DeleteByProviderModelID(ctx context.Context, providerID, modelID string) error
	// DeleteByProviderID removes all default model bindings under one provider.
	DeleteByProviderID(ctx context.Context, providerID string) error
}

type StorageBackendStore interface {
	// GetBlob 读取全局 Blob 物理元数据，仅供完成管理员鉴权后的 storage-inspection 服务调用。
	GetBlob(ctx context.Context, id string) (*iapiserver.AssetBlob, error)
	List(ctx context.Context, req *iapiserver.StorageBackendListRequest) ([]*iapiserver.StorageBackend, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.StorageBackend, error)
	Add(ctx context.Context, data *iapiserver.StorageBackend) (*iapiserver.StorageBackend, error)
	Update(ctx context.Context, data *iapiserver.StorageBackend) (*iapiserver.StorageBackend, error)
	GetDefaultLocal(ctx context.Context) (*iapiserver.StorageBackend, error)
}

type AssetStore interface {
	List(ctx context.Context, req *iapiserver.AssetListRequest) ([]*iapiserver.Asset, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.Asset, error)
	Add(ctx context.Context, data *iapiserver.Asset) (*iapiserver.Asset, error)
	AddWithUploadEvent(
		ctx context.Context,
		data *iapiserver.Asset,
		thumbnail *iapiserver.AssetThumbnail,
		event map[string]any,
	) (*iapiserver.Asset, *iapiserver.AssetThumbnail, error)
	Update(ctx context.Context, data *iapiserver.Asset) (*iapiserver.Asset, error)
	// Delete marks the asset as deleted. It does not remove asset objects, thumbnails, or relation rows.
	Delete(ctx context.Context, id string) error
}

type AssetV1Store interface {
	RegisterArtifact(context.Context, *iapiserver.ArtifactRegistrationRequest) (*iapiserver.UserAsset, bool, error)
	ApplyLabels(context.Context, string, string, map[string]string, []string, []string) (*iapiserver.BatchLabelData, error)
	// CreateArtifact 幂等创建 asset-library Artifact，并在同一事务写 artifact_created outbox。
	CreateArtifact(context.Context, *iapiserver.Artifact) (*iapiserver.Artifact, bool, error)
	// UpdateArtifactProcessing 以乐观版本推进处理事实，并在同一事务写 artifact_processing_changed outbox。
	UpdateArtifactProcessing(context.Context, string, string, int64, ArtifactProcessingMutation) (*iapiserver.Artifact, error)
	// UpdateArtifactRegistration 以乐观版本推进登记事实，并在同一事务写 artifact_registration_changed outbox。
	UpdateArtifactRegistration(context.Context, string, string, int64, ArtifactRegistrationMutation) (*iapiserver.Artifact, error)
	// CreateAssetVersion 幂等创建 processing 状态版本，并在同一事务写 asset_version_processing_changed outbox。
	CreateAssetVersion(context.Context, *iapiserver.AssetVersion, string, string) (*iapiserver.AssetVersion, bool, error)
	// UpdateAssetVersionProcessing 以乐观版本推进 Representation 汇总，并可靠发布素材版本事件。
	UpdateAssetVersionProcessing(context.Context, string, string, int64, AssetVersionProcessingMutation) (*iapiserver.AssetVersion, error)

	ListUserAssets(context.Context, string, *iapiserver.UserAssetListRequest) ([]*iapiserver.UserAsset, int64, error)
	GetUserAsset(context.Context, string, string, bool) (*iapiserver.UserAsset, error)
	GetAssetDetail(context.Context, string, string) (*iapiserver.AssetDetail, error)
	CreateCanonicalAsset(context.Context, string, *iapiserver.CreateCanonicalAssetRequest) (*iapiserver.AssetDetail, error)
	UpdateUserAsset(context.Context, string, string, *iapiserver.UpdateUserAssetRequest) (*iapiserver.UserAsset, error)
	SetUserAssetDeleted(context.Context, string, string, bool) (*iapiserver.UserAsset, error)
	PermanentlyDeleteUserAsset(context.Context, string, string) (*iapiserver.PermanentDeleteResult, []StoredAssetContent, error)

	CreateAssetUploads(context.Context, string, []iapiserver.AssetUploadItemRequest, int64) ([]iapiserver.AssetUploadInitResult, error)
	GetAssetUpload(context.Context, string, string) (*iapiserver.AssetUploadSession, error)
	RecordAssetUploadPart(context.Context, string, string, iapiserver.UploadedPart) (*iapiserver.AssetUploadSession, error)
	CompleteAssetUpload(
		context.Context,
		string,
		string,
		*iapiserver.CompleteAssetUploadRequest,
		StoredAssetContent,
	) (*iapiserver.CompleteAssetUploadResponse, error)
	// CompleteAssetUploadWithPlan 在上传事务内校验并发布由 service 计算的 expected Representation 计划。
	CompleteAssetUploadWithPlan(context.Context, string, string, *iapiserver.CompleteAssetUploadRequest, StoredAssetContent, RepresentationPlan) (*iapiserver.CompleteAssetUploadResponse, error)
	CancelAssetUpload(context.Context, string, string) (*iapiserver.AssetUploadSession, error)

	ListCollections(context.Context, string, *iapiserver.CollectionListRequest) ([]*iapiserver.AssetCollection, int64, error)
	GetCollection(context.Context, string, string, imachinery.PagingParams) (*iapiserver.CollectionDetail, error)
	CreateCollection(context.Context, string, *iapiserver.CreateCollectionRequest) (*iapiserver.AssetCollection, error)
	UpdateCollection(context.Context, string, string, *iapiserver.UpdateCollectionRequest) (*iapiserver.AssetCollection, error)
	DeleteCollection(context.Context, string, string) error
	AddCollectionItems(context.Context, string, string, []iapiserver.AddCollectionItem) (*iapiserver.CollectionItemBatchResponse, error)
	UpdateCollectionItem(context.Context, string, string, string, *iapiserver.UpdateCollectionItemRequest) (*iapiserver.AssetCollectionItem, error)
	DeleteCollectionItem(context.Context, string, string, string) error

	ReplaceLabels(context.Context, string, string, map[string]string) (*iapiserver.BatchLabelData, error)
	DeleteLabel(context.Context, string, string, string) (*iapiserver.BatchLabelData, error)
	AddTags(context.Context, string, string, []string) (*iapiserver.BatchLabelData, error)
	DeleteTag(context.Context, string, string, string) (*iapiserver.BatchLabelData, error)

	ListArtifacts(context.Context, string, *iapiserver.ArtifactListRequest) ([]*iapiserver.Artifact, int64, error)
	// ResolveArtifactSummaries 批量读取 owner 可见且未删除的 Artifact 一跳摘要，不返回缺失目标差异。
	ResolveArtifactSummaries(context.Context, string, []string) (map[string]*iapiserver.ArtifactReadableSummary, error)
	// DecorateArtifacts 批量组合登记素材与版本摘要，不读取跨领域 producer 私有表。
	DecorateArtifacts(context.Context, string, []*iapiserver.Artifact) error
	GetArtifact(context.Context, string, string) (*iapiserver.Artifact, error)
	StoreArtifactContent(context.Context, string, string, StoredAssetContent) (*iapiserver.Artifact, error)
	CompleteArtifact(context.Context, string, string, *iapiserver.CompleteArtifactRequest) (*iapiserver.Artifact, error)
	DeleteArtifact(context.Context, string, string) (*iapiserver.Artifact, error)
	RegisterArtifactLifecycle(context.Context, string, string, *iapiserver.RegisterArtifactRequest) (*iapiserver.ArtifactRegistrationResponse, error)
	// RegisterArtifactLifecycleWithPlan 在 Artifact 登记事务内应用受控 Representation 计划。
	RegisterArtifactLifecycleWithPlan(context.Context, string, string, *iapiserver.RegisterArtifactRequest, RepresentationPlan) (*iapiserver.ArtifactRegistrationResponse, error)

	ListAssetVersions(context.Context, string, string) ([]*iapiserver.AssetVersion, error)
	CreateCanonicalVersion(context.Context, string, string, *iapiserver.CreateCanonicalVersionRequest) (*iapiserver.AssetVersion, error)
	GetAssetVersionDetail(context.Context, string, string) (*iapiserver.AssetVersionDetail, error)
	SetCurrentAssetVersion(context.Context, string, string, string) (*iapiserver.UserAsset, error)
	ListRepresentations(context.Context, string, string) ([]*iapiserver.AssetRepresentation, error)
	RegisterRepresentation(context.Context, string, string, *iapiserver.RegisterRepresentationRequest) (*iapiserver.AssetRepresentation, error)
	// ApplyAssetMediaMetadata 原子写回 original 探测事实，并仅在版本仍为 current 时刷新 UserAsset 投影。
	ApplyAssetMediaMetadata(context.Context, string, string, AssetMediaMetadataMutation) error
	// ListAssetMediaMetadataBackfillCandidatesAfter 按稳定 Asset ID 扫描当前版本缺失媒体元数据的素材。
	ListAssetMediaMetadataBackfillCandidatesAfter(context.Context, string, int) ([]AssetMediaMetadataBackfillCandidate, error)
	// CompleteRepresentationGeneration 允许 Worker 幂等推进 pending/failed Representation，并原子刷新版本投影。
	CompleteRepresentationGeneration(context.Context, string, string, RepresentationGenerationMutation) (*iapiserver.AssetRepresentation, error)
	// ListRepresentationBackfillCandidatesAfter 按稳定 AssetVersion ID 扫描当前可见素材的 expected set 事实。
	ListRepresentationBackfillCandidatesAfter(context.Context, string, int) ([]RepresentationBackfillCandidate, error)
	// PrepareRepresentationBackfill 在创建修复动作前推进 expected count 和缩略图投影。
	PrepareRepresentationBackfill(context.Context, string, string, int) error
	// CreateRepresentationBlob 幂等登记 Worker 生成的受控派生内容，返回 Blob ID。
	CreateRepresentationBlob(context.Context, StoredAssetContent) (string, error)
	GetRepresentation(context.Context, string, string) (*iapiserver.AssetRepresentation, *StoredAssetContent, error)

	ListAssetRelations(context.Context, string, string, imachinery.PagingParams) (*iapiserver.AssetRelationListResponse, error)
	GetAssetLineage(context.Context, string, string) (*iapiserver.AssetLineage, error)
	ListAssetReferences(context.Context, string, string, imachinery.PagingParams) (*iapiserver.AssetReferenceListResponse, error)
	ListAssetUsages(context.Context, string, string, imachinery.PagingParams) (*iapiserver.AssetUsageListResponse, error)
}

// StoredAssetContent 是 StorageAdapter 与持久化事务之间的受控对象引用。
type StoredAssetContent struct {
	StorageBackendID string
	ObjectKey        string
	SHA256           string
	SizeBytes        int64
	MIMEType         string
	BlobID           string
}

// ExpectedRepresentation 是 asset-library policy 交给事务层的受控派生计划项。
type ExpectedRepresentation struct {
	Type     string
	Profile  string
	Required bool
}

// RepresentationPlan 固定一个 AssetVersion 的媒体策略和 expected set。
type RepresentationPlan struct {
	MediaType      string
	ProfileVersion string
	ExpectedCount  int
	Requested      []ExpectedRepresentation
}

// RepresentationGenerationMutation 是 Worker 对单个 expected Representation 的有限状态写入。
type RepresentationGenerationMutation struct {
	Type           string
	Profile        string
	ProfileVersion string
	BlobID         string
	Metadata       map[string]any
	Status         string
	Required       bool
	RetryCount     int
	RetryAfter     *imachinery.Time
	ErrorCode      string
	ErrorDetail    string
}

// AssetMediaMetadataMutation 是 representation.inspect 对原始媒体元数据的有限写回。
type AssetMediaMetadataMutation struct {
	AssetID                  string
	OriginalRepresentationID string
	MIMEType                 string
	SizeBytes                int64
	Width                    int
	Height                   int
	DurationSeconds          float64
}

// AssetMediaMetadataBackfillCandidate 是一次性运维回填所需的当前版本最小投影。
type AssetMediaMetadataBackfillCandidate struct {
	AssetID        string
	AssetVersionID string
	OwnerUserID    string
	MediaType      string
}

// RepresentationBackfillCandidate 是 backfill handler 所需的 owner 裁剪最小投影。
type RepresentationBackfillCandidate struct {
	AssetID              string
	AssetVersionID       string
	OwnerUserID          string
	MediaType            string
	ProfileVersion       string
	RepresentationStatus string
	RepresentationBlobOK bool
	RetryCount           int
	RetryAfter           *time.Time
	SourceAvailable      bool
}

// ArtifactProcessingMutation 只允许处理模块修改 Artifact 的处理维度和受保护预览摘要。
type ArtifactProcessingMutation struct {
	ChangeType            string
	ProcessingStatus      string
	Progress              *float64
	ProcessingPhase       string
	PreviewAvailable      bool
	PreviewRef            string
	ThumbnailRef          string
	ProcessingErrorCode   string
	ProcessingErrorDetail string
	Retryable             bool
	ReadyAt               *imachinery.Time
}

// ArtifactRegistrationMutation 只允许登记模块修改 Artifact 的登记维度和目标引用。
type ArtifactRegistrationMutation struct {
	RegistrationStatus      string
	RegistrationResult      string
	AssetID                 string
	AssetVersionID          string
	RegistrationErrorCode   string
	RegistrationErrorDetail string
	Retryable               bool
}

// AssetVersionProcessingMutation 由 Representation 汇总器提交有限计数、状态和任务引用。
type AssetVersionProcessingMutation struct {
	Status         string
	ExpectedCount  int
	CompletedCount int
	FailedCount    int
	TaskGroupID    string
	AtomicTaskID   string
	ErrorCode      string
}

type AssetThumbnailStore interface {
	GetByAsset(ctx context.Context, assetID string) (*iapiserver.AssetThumbnail, error)
	ListByAssetIDs(ctx context.Context, assetIDs []string) ([]*iapiserver.AssetThumbnail, error)
	Add(ctx context.Context, data *iapiserver.AssetThumbnail) (*iapiserver.AssetThumbnail, error)
	Update(ctx context.Context, data *iapiserver.AssetThumbnail) (*iapiserver.AssetThumbnail, error)
	// DeleteByAsset removes thumbnail metadata for one asset after the preview object is removed.
	DeleteByAsset(ctx context.Context, assetID string) error
}

type TagStore interface {
	ListByAssetIDs(ctx context.Context, assetIDs []string) (map[string][]*iapiserver.Tag, error)
	GetByName(ctx context.Context, name string, source string) (*iapiserver.Tag, error)
	FirstOrCreate(ctx context.Context, data *iapiserver.Tag) (*iapiserver.Tag, error)
}

type AssetTagStore interface {
	Replace(ctx context.Context, assetID string, tags []*iapiserver.Tag, source string) error
	ListTagNames(ctx context.Context, assetID string) ([]string, error)
	// DeleteByAsset removes tag links for one asset without deleting reusable tag records.
	DeleteByAsset(ctx context.Context, assetID string) error
}

type AIChatGenerationBundle struct {
	UserMessage      *iapiserver.AIChatMessage
	AssistantMessage *iapiserver.AIChatMessage
	Generation       *iapiserver.AIChatGeneration
}

type AIChatStore interface {
	ListAssistants(ctx context.Context, ownerUserID string) ([]*iapiserver.AIChatAssistant, error)
	GetAssistant(ctx context.Context, ownerUserID, id string) (*iapiserver.AIChatAssistant, error)
	// GetAssistantsByIDs 批量读取当前用户可见的系统助手和用户助手。
	GetAssistantsByIDs(ctx context.Context, ownerUserID string, ids []string) ([]*iapiserver.AIChatAssistant, error)
	CreateAssistant(ctx context.Context, ownerUserID string, data *iapiserver.AIChatAssistant) (*iapiserver.AIChatAssistant, error)
	UpdateAssistant(ctx context.Context, ownerUserID string, data *iapiserver.AIChatAssistant) (*iapiserver.AIChatAssistant, error)
	DeleteAssistant(ctx context.Context, ownerUserID, id string) error

	ListTopics(ctx context.Context, ownerUserID string, req *iapiserver.AIChatTopicListRequest) ([]*iapiserver.AIChatTopic, int64, error)
	GetTopic(ctx context.Context, ownerUserID, id string) (*iapiserver.AIChatTopic, error)
	CreateTopic(ctx context.Context, ownerUserID string, data *iapiserver.AIChatTopic) (*iapiserver.AIChatTopic, error)
	UpdateTopic(ctx context.Context, ownerUserID string, data *iapiserver.AIChatTopic) (*iapiserver.AIChatTopic, error)
	DeleteTopic(ctx context.Context, ownerUserID, id string) error
	BranchTopic(ctx context.Context, ownerUserID, messageID string) (*iapiserver.AIChatTopic, error)

	ListMessages(ctx context.Context, ownerUserID, topicID string) ([]*iapiserver.AIChatMessage, error)
	GetMessage(ctx context.Context, ownerUserID, id string) (*iapiserver.AIChatMessage, error)
	CreateMessageGeneration(
		ctx context.Context,
		ownerUserID string,
		topic *iapiserver.AIChatTopic,
		req *iapiserver.AIChatMessageCreateRequest,
		model *iapiserver.AIChatModel,
		assistant *iapiserver.AIChatAssistant,
	) (*AIChatGenerationBundle, error)
	CreateEditRegenerateGeneration(
		ctx context.Context,
		ownerUserID string,
		source *iapiserver.AIChatMessage,
		req *iapiserver.AIChatEditRegenerateRequest,
		model *iapiserver.AIChatModel,
		assistant *iapiserver.AIChatAssistant,
	) (*AIChatGenerationBundle, error)
	CompleteGeneration(ctx context.Context, ownerUserID, generationID, content string) (*iapiserver.AIChatGeneration, error)
	FailGeneration(ctx context.Context, ownerUserID, generationID, errorCode, errorMessage string) error
	StopGeneration(ctx context.Context, ownerUserID, generationID string) (*iapiserver.AIChatGeneration, error)

	ListQuickPhrases(ctx context.Context, ownerUserID string, req *iapiserver.AIChatQuickPhraseListRequest) ([]*iapiserver.AIChatQuickPhrase, error)
	GetQuickPhrase(ctx context.Context, ownerUserID, id string) (*iapiserver.AIChatQuickPhrase, error)
	CreateQuickPhrase(ctx context.Context, ownerUserID string, data *iapiserver.AIChatQuickPhrase) (*iapiserver.AIChatQuickPhrase, error)
	UpdateQuickPhrase(ctx context.Context, ownerUserID string, data *iapiserver.AIChatQuickPhrase) (*iapiserver.AIChatQuickPhrase, error)
	DeleteQuickPhrase(ctx context.Context, ownerUserID, id string) error
	CreateTranslation(ctx context.Context, translation *iapiserver.AIChatMessageTranslation) (*iapiserver.AIChatMessageTranslation, error)
}

type AssetGroupStore interface {
	Add(ctx context.Context, data *iapiserver.AssetGroup) (*iapiserver.AssetGroup, error)
}

type AssetGroupMemberStore interface {
	BatchAdd(ctx context.Context, members []*iapiserver.AssetGroupMember) ([]*iapiserver.AssetGroupMember, error)
	// DeleteByAsset removes an asset from all groups before the asset metadata is deleted.
	DeleteByAsset(ctx context.Context, assetID string) error
}

type AssetRelationStore interface {
	Add(ctx context.Context, data *iapiserver.AssetRelation) (*iapiserver.AssetRelation, error)
	// DeleteByAsset removes derivation relations where the asset is either source or target.
	DeleteByAsset(ctx context.Context, assetID string) error
}

type TaskCenterStore interface {
	ListAtomicTasks(context.Context, *iapiserver.AtomicTaskListRequest) ([]*iapiserver.AtomicTask, int64, error)
	// GetAtomicTasksByIDs 批量读取调度历史引用的 AtomicTask，不用于绕过 service 权限返回完整资源。
	GetAtomicTasksByIDs(context.Context, []string) ([]*iapiserver.AtomicTask, error)
	GetAtomicTask(context.Context, string) (*iapiserver.AtomicTask, error)
	AddAtomicTaskIdempotent(context.Context, *iapiserver.AtomicTask) (*iapiserver.AtomicTask, bool, error)
	UpdateAtomicTask(context.Context, *iapiserver.AtomicTask) (*iapiserver.AtomicTask, error)
	ListAttempts(ctx context.Context, req *iapiserver.TaskAttemptListRequest) ([]*iapiserver.TaskAttempt, int64, error)
	GetAttempt(context.Context, string, string) (*iapiserver.TaskAttempt, error)
	ListAttemptsByTaskIDs(context.Context, []string) ([]*iapiserver.TaskAttempt, error)
	ListTaskGroups(context.Context, *iapiserver.TaskGroupListRequest) ([]*iapiserver.TaskGroup, int64, error)
	// GetTaskGroupsByIDs 批量读取调度历史引用的 TaskGroup。
	GetTaskGroupsByIDs(context.Context, []string) ([]*iapiserver.TaskGroup, error)
	GetTaskGroup(context.Context, string) (*iapiserver.TaskGroup, error)
	AddTaskGroupWithTasks(context.Context, *iapiserver.TaskGroup, []*iapiserver.AtomicTask) (*iapiserver.TaskGroup, bool, error)
	UpdateTaskGroup(context.Context, *iapiserver.TaskGroup) (*iapiserver.TaskGroup, error)
	ListDAGTaskGroups(context.Context, *iapiserver.DAGTaskGroupListRequest) ([]*iapiserver.DAGTaskGroup, int64, error)
	// GetDAGTaskGroupsByIDs 批量读取调度历史引用的 DAGTaskGroup。
	GetDAGTaskGroupsByIDs(context.Context, []string) ([]*iapiserver.DAGTaskGroup, error)
	GetDAGTaskGroup(context.Context, string) (*iapiserver.DAGTaskGroup, error)
	AddDAGTaskGroupWithTasks(context.Context, *iapiserver.DAGTaskGroup, []*iapiserver.AtomicTask) (*iapiserver.DAGTaskGroup, bool, error)
	UpdateDAGTaskGroup(context.Context, *iapiserver.DAGTaskGroup) (*iapiserver.DAGTaskGroup, error)
	ListOwnedTasks(context.Context, string, string, *iapiserver.AtomicTaskListRequest) ([]*iapiserver.AtomicTask, int64, error)
	// ListDAGObservationTasks 批量读取一个已授权 DAG 的实际任务投影，供详情、事件和时间线聚合。
	ListDAGObservationTasks(context.Context, string) ([]*iapiserver.AtomicTask, error)
	AddOwnedAtomicTasks(context.Context, string, string, []*iapiserver.AtomicTask) error
	ListTaskSchedules(context.Context, *iapiserver.TaskScheduleListRequest) ([]*iapiserver.TaskSchedule, int64, error)
	// GetTaskSchedulesByIDs 批量读取关联摘要使用的 TaskSchedule，调用方仍需执行主体可见性过滤。
	GetTaskSchedulesByIDs(context.Context, []string) ([]*iapiserver.TaskSchedule, error)
	GetTaskSchedule(context.Context, string) (*iapiserver.TaskSchedule, error)
	GetTaskScheduleBySystemKey(context.Context, string) (*iapiserver.TaskSchedule, error)
	AddTaskSchedule(context.Context, *iapiserver.TaskSchedule) (*iapiserver.TaskSchedule, error)
	EnsureSystemTaskSchedule(context.Context, *iapiserver.TaskSchedule, *iapiserver.ScheduleReconcileState) (*iapiserver.TaskSchedule, bool, error)
	UpdateTaskSchedule(context.Context, *iapiserver.TaskSchedule) (*iapiserver.TaskSchedule, error)
	ListScheduleExecutions(context.Context, *iapiserver.ScheduleExecutionListRequest) ([]*iapiserver.TaskScheduleExecution, int64, error)
	GetScheduleExecution(context.Context, string) (*iapiserver.TaskScheduleExecution, error)
	// GetScheduleExecutionAt 按计划与计划时间读取幂等轮次；不存在时返回 nil，供 misfire 守卫区分首次迟到触发与已有轮次恢复。
	GetScheduleExecutionAt(context.Context, string, time.Time) (*iapiserver.TaskScheduleExecution, error)
	ListLatestScheduleExecutions(context.Context, []string) (map[string]*iapiserver.TaskScheduleExecution, error)
	// ListScheduleSources 按目标类型与 ID 批量返回最新的来源调度轮次。
	ListScheduleSources(context.Context, string, []string) (map[string]*iapiserver.ScheduleSourceSummary, error)
	AddProjectionEventIdempotent(context.Context, *iapiserver.RuntimeProjectionEvent) (*iapiserver.RuntimeProjectionEvent, bool, error)
	ListRuntimeProjectionEvents(context.Context, string) ([]*iapiserver.RuntimeProjectionEvent, error)
	ListNonTerminalAtomicTasks(context.Context, int) ([]*iapiserver.AtomicTask, error)
	ListNonTerminalTaskGroups(context.Context, int) ([]*iapiserver.TaskGroup, error)
	ListNonTerminalDAGTaskGroups(context.Context, int) ([]*iapiserver.DAGTaskGroup, error)
	ListActiveScheduleExecutions(context.Context, int) ([]*iapiserver.TaskScheduleExecution, error)
	ApplyRuntimeProjection(context.Context, *iapiserver.AtomicTask, []*iapiserver.TaskAttempt, *iapiserver.RuntimeProjectionEvent) (bool, error)
	AcquireScheduleExecution(context.Context, *iapiserver.TaskScheduleExecution) (*iapiserver.TaskScheduleExecution, bool, error)
	// WithScheduleReconcileLock 在 PostgreSQL 事务级 advisory lock 下串行执行同一计划的 controller，进程退出时锁自动释放。
	WithScheduleReconcileLock(context.Context, string, func() error) (bool, error)
	UpdateScheduleExecution(context.Context, *iapiserver.TaskScheduleExecution) (*iapiserver.TaskScheduleExecution, error)
	GetScheduleReconcileState(context.Context, string) (*iapiserver.ScheduleReconcileState, error)
	CompleteScheduleReconcile(context.Context, *iapiserver.TaskScheduleExecution, *iapiserver.ScheduleReconcileState) error
	PruneReconcileExecutions(context.Context, string, iapiserver.HistoryRetention, time.Time) (int64, error)
}

// UserEventStore 提供当前用户短期事件历史与 SSE 增量读取，不暴露跨用户查询。
type UserEventStore interface {
	AddIdempotent(context.Context, *iapiserver.UserEvent) (*iapiserver.UserEvent, bool, error)
	List(context.Context, *iapiserver.UserEventListRequest, time.Time) ([]*iapiserver.UserEvent, int64, error)
	ListAfter(context.Context, string, int64, int, time.Time) ([]*iapiserver.UserEvent, error)
	CursorVisible(context.Context, string, int64) (bool, bool, error)
	SyncState(context.Context, string, time.Time) (int64, int64, error)
	PruneExpired(context.Context, time.Time) (int64, error)
}

type ApplicationPlatformStore interface {
	ListEngineInstances(ctx context.Context, req *iapiserver.EngineInstanceListRequest) ([]*iapiserver.EngineInstance, int64, error)
	ListEnabledEngineInstancesAfter(context.Context, string, int) ([]*iapiserver.EngineInstance, error)
	ListRefreshableComfyUIEngineInstancesAfter(context.Context, string, int) ([]*iapiserver.EngineInstance, error)
	GetEngineInstance(ctx context.Context, id string) (*iapiserver.EngineInstance, error)
	AddEngineInstance(ctx context.Context, data *iapiserver.EngineInstance) (*iapiserver.EngineInstance, error)
	UpdateEngineInstance(ctx context.Context, data *iapiserver.EngineInstance, expectedVersion int64) (*iapiserver.EngineInstance, error)
	UpdateEngineInstanceHealth(
		ctx context.Context,
		data *iapiserver.EngineInstance,
		expectedVersion int64,
		event *iapiserver.ApplicationPlatformEvent,
	) (*iapiserver.EngineInstance, error)
	GetComfyUIEngineObjectInfo(context.Context, string) (*iapiserver.ComfyUIEngineObjectInfo, error)
	RefreshComfyUIEngineObjectInfo(
		context.Context,
		string,
		func(*iapiserver.EngineInstance) (*iapiserver.ComfyUIEngineObjectInfo, error),
	) (*iapiserver.ComfyUIEngineObjectInfo, error)
	WithEngineInstanceLock(context.Context, string, func() error) error
	DeleteEngineInstance(ctx context.Context, id string) error
	CountRunsByEngineInstance(ctx context.Context, id string) (int64, error)
	ListComfyUIWorkflows(ctx context.Context, req *iapiserver.ComfyUIWorkflowListRequest) ([]*iapiserver.ComfyUIWorkflow, int64, error)
	GetComfyUIWorkflow(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflow, error)
	AddComfyUIWorkflow(ctx context.Context, data *iapiserver.ComfyUIWorkflow) (*iapiserver.ComfyUIWorkflow, error)
	UpdateComfyUIWorkflow(ctx context.Context, data *iapiserver.ComfyUIWorkflow, expectedVersion int64) (*iapiserver.ComfyUIWorkflow, error)
	ListComfyUIWorkflowDuplicateIDs(ctx context.Context, ownerUserID, checksum string) ([]string, error)
	ListComfyUIWorkflowValidations(
		ctx context.Context,
		req *iapiserver.ComfyUIWorkflowValidationListRequest,
	) ([]*iapiserver.ComfyUIWorkflowValidation, int64, error)
	GetComfyUIWorkflowValidation(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflowValidation, error)
	AddComfyUIWorkflowValidation(ctx context.Context, data *iapiserver.ComfyUIWorkflowValidation) (*iapiserver.ComfyUIWorkflowValidation, error)
	ListComfyUIWorkflowTestRuns(ctx context.Context, req *iapiserver.ComfyUIWorkflowTestRunListRequest) ([]*iapiserver.ComfyUIWorkflowTestRun, int64, error)
	GetComfyUIWorkflowTestRun(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflowTestRun, error)
	GetComfyUIWorkflowTestRunByIdempotency(ctx context.Context, ownerUserID, key string) (*iapiserver.ComfyUIWorkflowTestRun, error)
	AddComfyUIWorkflowTestRun(ctx context.Context, data *iapiserver.ComfyUIWorkflowTestRun) (*iapiserver.ComfyUIWorkflowTestRun, error)
	UpdateComfyUIWorkflowTestRun(ctx context.Context, data *iapiserver.ComfyUIWorkflowTestRun) (*iapiserver.ComfyUIWorkflowTestRun, error)
	ConvertComfyUIWorkflow(
		ctx context.Context,
		workflowID, ownerUserID, actorUserID, idempotencyKey string,
		template *iapiserver.ApplicationTemplate,
		version *iapiserver.ApplicationTemplateVersion,
	) (*iapiserver.ComfyUIWorkflowConvertResult, error)
	ListEngineBindings(ctx context.Context, req *iapiserver.EngineCapabilityBindingListRequest) ([]*iapiserver.EngineCapabilityBinding, int64, error)
	GetEngineBinding(ctx context.Context, id string) (*iapiserver.EngineCapabilityBinding, error)
	AddEngineBinding(ctx context.Context, data *iapiserver.EngineCapabilityBinding) (*iapiserver.EngineCapabilityBinding, error)
	UpdateEngineBinding(ctx context.Context, data *iapiserver.EngineCapabilityBinding, expectedVersion int64) (*iapiserver.EngineCapabilityBinding, error)
	DeleteEngineBinding(ctx context.Context, id string) error
	ListTemplates(ctx context.Context, req *iapiserver.ApplicationTemplateListRequest) ([]*iapiserver.ApplicationTemplate, int64, error)
	GetTemplate(ctx context.Context, id string) (*iapiserver.ApplicationTemplate, error)
	AddTemplateWithVersion(
		ctx context.Context,
		data *iapiserver.ApplicationTemplate,
		version *iapiserver.ApplicationTemplateVersion,
	) (*iapiserver.ApplicationTemplate, error)
	ListTemplateVersions(ctx context.Context, req *iapiserver.ApplicationTemplateVersionListRequest) ([]*iapiserver.ApplicationTemplateVersion, int64, error)
	GetTemplateVersion(ctx context.Context, id string) (*iapiserver.ApplicationTemplateVersion, error)
	AddTemplateVersion(ctx context.Context, data *iapiserver.ApplicationTemplateVersion) (*iapiserver.ApplicationTemplateVersion, error)
	PublishTemplateVersion(ctx context.Context, id string) (*iapiserver.ApplicationTemplateVersion, error)
	ListApplications(ctx context.Context, req *iapiserver.ApplicationListRequest) ([]*iapiserver.Application, int64, error)
	GetApplication(ctx context.Context, id string) (*iapiserver.Application, error)
	AddApplication(ctx context.Context, data *iapiserver.Application) (*iapiserver.Application, error)
	UpdateApplication(ctx context.Context, data *iapiserver.Application, expectedVersion int64) (*iapiserver.Application, error)
	ListApplicationVersions(ctx context.Context, req *iapiserver.ApplicationVersionListRequest) ([]*iapiserver.ApplicationVersion, int64, error)
	GetApplicationVersion(ctx context.Context, id string) (*iapiserver.ApplicationVersion, error)
	AddApplicationVersion(ctx context.Context, data *iapiserver.ApplicationVersion) (*iapiserver.ApplicationVersion, error)
	PublishApplicationVersion(ctx context.Context, id string) (*iapiserver.ApplicationVersion, error)
	GetApplicationRun(ctx context.Context, id string) (*iapiserver.ApplicationRun, error)
	GetApplicationRunsByIDs(ctx context.Context, ownerUserID string, ids []string) ([]*iapiserver.ApplicationRun, error)
	GetApplicationRunByIdempotency(ctx context.Context, ownerUserID, key string) (*iapiserver.ApplicationRun, error)
	AddApplicationRun(ctx context.Context, data *iapiserver.ApplicationRun) (*iapiserver.ApplicationRun, error)
	BindApplicationRunTask(ctx context.Context, id, atomicTaskID, status string, taskVersion int64, failure string) (*iapiserver.ApplicationRun, error)
	ProjectApplicationRun(
		ctx context.Context,
		id string,
		taskVersion int64,
		status, failure string,
		outputs []map[string]any,
	) (*iapiserver.ApplicationRun, error)
	ListArtifactsByRun(ctx context.Context, runID string) ([]*iapiserver.ApplicationArtifact, error)
	UpsertArtifact(ctx context.Context, data *iapiserver.ApplicationArtifact) (*iapiserver.ApplicationArtifact, error)
	UpdateArtifactRegistration(
		ctx context.Context,
		id, status, assetID, errorCode, failureDetail string,
		expectedVersion int64,
	) (*iapiserver.ApplicationArtifact, error)
}

type FeatureFlagStore interface {
	List(ctx context.Context) ([]*iapiserver.FeatureFlag, error)
	Upsert(ctx context.Context, data *iapiserver.FeatureFlag) (*iapiserver.FeatureFlag, error)
}

type RoleStore interface {
	List(ctx context.Context) ([]*iapiserver.Role, error)
}

type PermissionStore interface {
	List(ctx context.Context) ([]*iapiserver.Permission, error)
}

type UserRoleStore interface {
	ListByUser(ctx context.Context, userID string) ([]*iapiserver.UserRole, error)
}
