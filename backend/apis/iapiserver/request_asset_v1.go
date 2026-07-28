package iapiserver

import (
	"errors"
	"mime/multipart"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

type ArtifactRegistrationRequest struct {
	ArtifactID     string `json:"artifact_id" binding:"required,max=64"`
	Mode           string `json:"mode" binding:"omitempty,oneof=create_asset append_version"`
	AssetID        string `json:"asset_id" binding:"omitempty,max=64"`
	Name           string `json:"name" binding:"omitempty,max=255"`
	VersionNote    string `json:"version_note" binding:"omitempty,max=1000"`
	ProfileVersion string `json:"profile_version" binding:"omitempty,max=128"`
	// 以下字段仅保留旧 application-platform 兼容调用，不属于 spec-v1.5.1 公共请求。
	ApplicationRunID string  `json:"application_run_id,omitempty" binding:"omitempty,max=64"`
	OwnerUserID      string  `json:"owner_user_id,omitempty" binding:"omitempty,max=128"`
	OutputName       string  `json:"output_name,omitempty" binding:"omitempty,max=255"`
	MediaType        string  `json:"media_type,omitempty" binding:"omitempty,oneof=image video audio text pdf other"`
	ContentRef       string  `json:"content_ref,omitempty"`
	Format           string  `json:"format" binding:"omitempty,max=64"`
	SizeBytes        int64   `json:"size_bytes" binding:"min=0"`
	Width            int     `json:"width" binding:"min=0"`
	Height           int     `json:"height" binding:"min=0"`
	DurationSeconds  float64 `json:"duration_seconds" binding:"min=0"`
	SHA256           string  `json:"sha256"`
}
type ArtifactRegistrationResponse struct {
	ArtifactID         string        `json:"artifact_id"`
	ApplicationRunID   string        `json:"application_run_id,omitempty"`
	RegistrationResult string        `json:"registration_result"`
	Asset              *UserAsset    `json:"asset"`
	AssetVersion       *AssetVersion `json:"asset_version,omitempty"`
}
type BatchAssetItem struct {
	// ID 指定当前用户范围内要处理的素材；批量请求中不得重复。
	ID string `json:"id" binding:"required"`
}

// DeleteAssetRequest 控制单素材删除模式；asset.delete 权限下默认软删除，显式 hard_delete 才直接永久删除。
type DeleteAssetRequest struct {
	// HardDelete 为 true 时绕过回收站并执行强引用检查；不返回原始内容、metadata 或缩略图。
	HardDelete bool `form:"hard_delete"`
}

// Decode 从 DELETE query 读取删除模式，避免改变其他 DELETE endpoint 的参数绑定方式。
func (r *DeleteAssetRequest) Decode(c *gin.Context) error {
	return c.ShouldBindQuery(r)
}

// BatchDeleteAssetsRequest 批量删除当前用户素材；整批统一使用软删除或硬删除模式。
type BatchDeleteAssetsRequest struct {
	// Items 包含 1 至 200 个唯一素材 ID；每项独立提交并按请求顺序返回。
	Items []BatchAssetItem `json:"items" binding:"required,min=1,max=200,dive"`
	// HardDelete 为 true 时整批绕过回收站并逐项执行永久删除检查。
	HardDelete bool `json:"hard_delete"`
}

type BatchDeleteAssetError struct {
	// Code 是 SSOT 定义的稳定业务错误名，供客户端按错误类别处理单项失败。
	Code string `json:"code"`
	// Value 是业务错误的稳定数值编码。
	Value int `json:"value"`
	// Message 是面向当前默认语言的错误消息。
	Message string `json:"message"`
	// Messages 同时提供简体中文和英文错误消息。
	Messages map[string]string `json:"messages"`
	// Detail 提供本次失败的非敏感上下文，不泄露其他用户素材是否存在。
	Detail string `json:"detail"`
	// Retryable 指示客户端稍后重试是否可能成功。
	Retryable bool `json:"retryable"`
}

type BatchDeleteAssetResult struct {
	// ID 回显请求中的素材 ID，并保持原始请求顺序。
	ID string `json:"id"`
	// Success 表示该素材是否完成所选删除模式。
	Success bool `json:"success"`
	// HardDelete 表示该项是否按绕过回收站的永久删除模式处理。
	HardDelete bool `json:"hard_delete"`
	// Asset 仅在软删除成功时返回 deleted 状态素材。
	Asset *UserAsset `json:"asset,omitempty"`
	// PermanentDelete 仅在硬删除成功时返回 Blob 删除和保留结果。
	PermanentDelete *PermanentDeleteResult `json:"permanent_delete,omitempty"`
	// Error 仅在单项失败时返回对应业务错误。
	Error *BatchDeleteAssetError `json:"error,omitempty"`
}

type BatchDeleteAssetsResponse struct {
	// Total 是本次实际处理的素材总数；空回收站时为 0。
	Total int `json:"total"`
	// Success 是删除成功的素材数。
	Success int `json:"success"`
	// Fail 是删除失败并保留错误结果的素材数。
	Fail int `json:"fail"`
	// Results 按请求或回收站稳定枚举顺序返回逐项结果。
	Results []BatchDeleteAssetResult `json:"results"`
}

type BatchLabelRequest struct {
	Items          []BatchAssetItem  `json:"items" binding:"required,min=1,max=100,dive"`
	LabelsToUpsert map[string]string `json:"labels_to_upsert"`
	TagsToAdd      []string          `json:"tags_to_add" binding:"max=30"`
	TagsToRemove   []string          `json:"tags_to_remove" binding:"max=30"`
}
type BatchLabelData struct {
	ID              string            `json:"id"`
	Labels          map[string]string `json:"labels"`
	Tags            []string          `json:"tags"`
	LabelSources    map[string]string `json:"label_sources"`
	TagSources      map[string]string `json:"tag_sources"`
	ResourceVersion int64             `json:"resource_version"`
}
type BatchLabelResult struct {
	ID      string          `json:"id"`
	Success bool            `json:"success"`
	Data    *BatchLabelData `json:"data,omitempty"`
	Error   map[string]any  `json:"error,omitempty"`
}
type BatchLabelResponse struct {
	Total   int                `json:"total"`
	Success int                `json:"success"`
	Fail    int                `json:"fail"`
	Results []BatchLabelResult `json:"results"`
}

type QueryResolution struct {
	RequestedMode    string  `json:"requested_mode"`
	AppliedMode      string  `json:"applied_mode"`
	ResolvedSelector *string `json:"resolved_selector,omitempty"`
}

// UserAssetListRequest 查询当前用户素材，owner 仅由认证上下文注入。
type UserAssetListRequest struct {
	imachinery.BasicQueryParam
	Selector             string                   `form:"selector" binding:"omitempty,max=4096"`
	NaturalLanguageQuery string                   `form:"natural_language_query" binding:"omitempty,max=4096"`
	MediaType            string                   `form:"media_type" binding:"omitempty,oneof=image video audio text document model_3d prompt prompt_template pdf other"`
	Format               string                   `form:"format" binding:"omitempty,max=64"`
	SourceType           string                   `form:"source_type" binding:"omitempty,oneof=upload canvas_output application_output"`
	Status               string                   `form:"status" binding:"omitempty,oneof=active archived deleted"`
	Width                int                      `form:"width" binding:"omitempty,min=0"`
	Height               int                      `form:"height" binding:"omitempty,min=0"`
	SelectorExpression   *AssetSelectorExpression `form:"-" json:"-"`
}

type AssetSelectorExpression struct {
	Operator  string
	Predicate *AssetSelectorPredicate
	Children  []*AssetSelectorExpression
}

type AssetSelectorPredicate struct {
	Kind   string
	Key    string
	Action string
	Values []string
}

type UserAssetListResponse struct {
	Total           int64            `json:"total"`
	Items           []*UserAsset     `json:"items"`
	QueryResolution *QueryResolution `json:"query_resolution"`
}

type CreateCanonicalAssetRequest struct {
	DisplayName      string            `json:"display_name" binding:"required,max=255"`
	Description      string            `json:"description" binding:"omitempty,max=2000"`
	MediaType        string            `json:"media_type" binding:"required,oneof=text prompt prompt_template"`
	CanonicalContent map[string]any    `json:"canonical_content" binding:"required"`
	Labels           map[string]string `json:"labels" binding:"omitempty,max=20"`
	Tags             []string          `json:"tags" binding:"omitempty,max=30,unique"`
	ProfileVersion   string            `json:"profile_version" binding:"omitempty,max=128"`
}

type UpdateUserAssetRequest struct {
	DisplayName     *string `json:"display_name" binding:"omitempty,max=255"`
	Description     *string `json:"description" binding:"omitempty,max=2000"`
	Status          *string `json:"status" binding:"omitempty,oneof=active archived"`
	ResourceVersion *int64  `json:"resource_version" binding:"omitempty,min=0"`
}

type AssetDetail struct {
	Asset            *UserAsset             `json:"asset"`
	CurrentVersion   *AssetVersion          `json:"current_version,omitempty"`
	Versions         []*AssetVersion        `json:"versions"`
	Representations  []*AssetRepresentation `json:"representations"`
	Collections      []*AssetCollection     `json:"collections"`
	ReferenceSummary *ReferenceSummary      `json:"reference_summary"`
}

type PermanentDeleteResult struct {
	AssetID         string   `json:"asset_id"`
	Deleted         bool     `json:"deleted"`
	DeletedBlobIDs  []string `json:"deleted_blob_ids"`
	RetainedBlobIDs []string `json:"retained_blob_ids"`
}

type CreateCanonicalVersionRequest struct {
	CanonicalContent map[string]any `json:"canonical_content" binding:"required"`
	VersionNote      string         `json:"version_note" binding:"omitempty,max=1000"`
	ProfileVersion   string         `json:"profile_version" binding:"required,max=128"`
}

type AssetVersionListResponse struct {
	Total int64           `json:"total"`
	Items []*AssetVersion `json:"items"`
}

type AssetVersionDetail struct {
	Version         *AssetVersion          `json:"version"`
	Representations []*AssetRepresentation `json:"representations"`
}

type UploadedPart struct {
	PartNumber int    `json:"part_number" binding:"required,min=1"`
	SizeBytes  int64  `json:"size_bytes" binding:"required,min=1"`
	SHA256     string `json:"sha256" binding:"required,len=64,hexadecimal"`
}

type AssetUploadItemRequest struct {
	ClientUploadKey string            `json:"client_upload_key" binding:"required,max=128"`
	FileName        string            `json:"file_name" binding:"required,max=512"`
	DisplayName     string            `json:"display_name" binding:"omitempty,max=255"`
	SizeBytes       int64             `json:"size_bytes" binding:"min=0"`
	SHA256          string            `json:"sha256" binding:"required,len=64,hexadecimal"`
	MIMEType        string            `json:"mime_type" binding:"required,max=255"`
	TargetAssetID   string            `json:"target_asset_id" binding:"omitempty,max=64"`
	VersionNote     string            `json:"version_note" binding:"omitempty,max=1000"`
	Labels          map[string]string `json:"labels" binding:"omitempty,max=20"`
	Tags            []string          `json:"tags" binding:"omitempty,max=30,unique"`
	ProfileVersion  string            `json:"profile_version" binding:"omitempty,max=128"`
}

type CreateAssetUploadsRequest struct {
	Items []AssetUploadItemRequest `json:"items" binding:"required,min=1,max=100,dive"`
}

type AssetUploadInitResult struct {
	ClientUploadKey string              `json:"client_upload_key"`
	Deduplicated    bool                `json:"deduplicated"`
	Upload          *AssetUploadSession `json:"upload,omitempty"`
	ExistingAsset   *UserAsset          `json:"existing_asset,omitempty"`
	Error           map[string]any      `json:"error,omitempty"`
}

type CreateAssetUploadsResponse struct {
	Total   int                     `json:"total"`
	Success int                     `json:"success"`
	Fail    int                     `json:"fail"`
	Results []AssetUploadInitResult `json:"results"`
}

type UploadAssetContentRequest struct {
	UploadID     string `json:"-"`
	PartNumber   int    `form:"part_number" binding:"omitempty,min=1"`
	PartSHA256   string `header:"part_sha256"`
	ContentRange string `header:"content_range"`
}

type CompleteAssetUploadRequest struct {
	SHA256 string         `json:"sha256" binding:"required,len=64,hexadecimal"`
	Parts  []UploadedPart `json:"parts" binding:"omitempty,dive"`
}

type CompleteAssetUploadResponse struct {
	Upload                 *AssetUploadSession  `json:"upload"`
	Asset                  *UserAsset           `json:"asset"`
	AssetVersion           *AssetVersion        `json:"asset_version"`
	OriginalRepresentation *AssetRepresentation `json:"original_representation"`
	Deduplicated           bool                 `json:"deduplicated"`
}

// CollectionListRequest 查询 Collection；分页和排序遵循公共列表参数。
type CollectionListRequest struct {
	imachinery.BasicQueryParam
	ParentCollectionID string `form:"parent_collection_id" binding:"omitempty,max=64"`
}

type CollectionListResponse struct {
	Total int64              `json:"total"`
	Items []*AssetCollection `json:"items"`
}

type CollectionDetail struct {
	Collection *AssetCollection       `json:"collection"`
	Total      int64                  `json:"total"`
	Items      []*AssetCollectionItem `json:"items"`
}

type CreateCollectionRequest struct {
	Name               string `json:"name" binding:"required,max=255"`
	Description        string `json:"description" binding:"omitempty,max=2000"`
	ParentCollectionID string `json:"parent_collection_id" binding:"omitempty,max=64"`
	Color              string `json:"color" binding:"omitempty,max=64"`
	SortOrder          int    `json:"sort_order"`
}

type UpdateCollectionRequest struct {
	// Name 修改 Collection 展示名称；提供时 trim 后必须非空且不超过 255 个字符。
	Name *string `json:"name" binding:"omitempty,min=1,max=255"`
	// Description 修改 Collection 描述；nil 表示保持原值。
	Description *string `json:"description" binding:"omitempty,max=2000"`
	// ParentCollectionID 修改父级 Collection；服务端校验同用户、无环且最大深度为 8。
	ParentCollectionID *string `json:"parent_collection_id" binding:"omitempty,max=64"`
	// Color 修改前端展示色；nil 表示保持原值。
	Color *string `json:"color" binding:"omitempty,max=64"`
	// SortOrder 修改同级 Collection 的手动排序值。
	SortOrder *int `json:"sort_order"`
	// ResourceVersion 可选地执行乐观并发校验；版本不匹配时拒绝更新。
	ResourceVersion *int64 `json:"resource_version" binding:"omitempty,min=0"`
}

// Validate 保证 Collection 更新符合 OpenAPI 的至少一个字段约束，并拒绝 trim 后的空名称。
func (r *UpdateCollectionRequest) Validate() error {
	if r.Name == nil &&
		r.Description == nil &&
		r.ParentCollectionID == nil &&
		r.Color == nil &&
		r.SortOrder == nil &&
		r.ResourceVersion == nil {
		return errors.New("at least one collection field is required")
	}
	if r.Name != nil && strings.TrimSpace(*r.Name) == "" {
		return errors.New("collection name must not be empty")
	}
	return nil
}

type AddCollectionItem struct {
	AssetID         string         `json:"asset_id" binding:"required,max=64"`
	PinnedVersionID string         `json:"pinned_version_id" binding:"omitempty,max=64"`
	Role            string         `json:"role" binding:"omitempty,max=128"`
	SortOrder       int            `json:"sort_order"`
	Metadata        map[string]any `json:"metadata"`
}

type AddCollectionItemsRequest struct {
	Items []AddCollectionItem `json:"items" binding:"required,min=1,max=100,dive"`
}

type UpdateCollectionItemRequest struct {
	PinnedVersionID *string         `json:"pinned_version_id" binding:"omitempty,max=64"`
	Role            *string         `json:"role" binding:"omitempty,max=128"`
	SortOrder       *int            `json:"sort_order"`
	Metadata        *map[string]any `json:"metadata"`
}

type CollectionItemResult struct {
	AssetID string               `json:"asset_id"`
	Success bool                 `json:"success"`
	Data    *AssetCollectionItem `json:"data,omitempty"`
	Error   map[string]any       `json:"error,omitempty"`
}

type CollectionItemBatchResponse struct {
	Total   int                    `json:"total"`
	Success int                    `json:"success"`
	Fail    int                    `json:"fail"`
	Results []CollectionItemResult `json:"results"`
}

type ReplaceLabelsRequest struct {
	Labels map[string]string `json:"labels" binding:"required,max=20"`
}

type AddTagsRequest struct {
	Tags []string `json:"tags" binding:"required,max=30,unique"`
}

type CreateArtifactRequest struct {
	ProducerType             string           `json:"producer_type" binding:"required,oneof=application_run canvas_run atomic_task"`
	ProducerID               string           `json:"producer_id" binding:"required,max=128"`
	ProducerIdempotencyKey   string           `json:"producer_idempotency_key" binding:"required,max=512"`
	AtomicTaskID             string           `json:"atomic_task_id" binding:"omitempty,max=128"`
	TaskAttemptID            string           `json:"task_attempt_id" binding:"omitempty,max=128"`
	ApplicationRunID         string           `json:"application_run_id" binding:"omitempty,max=128"`
	CanvasRunID              string           `json:"canvas_run_id" binding:"omitempty,max=128"`
	NodeRunID                string           `json:"node_run_id" binding:"omitempty,max=128"`
	NodeID                   string           `json:"node_id" binding:"omitempty,max=128"`
	OutputKey                string           `json:"output_key" binding:"required,max=255"`
	Sequence                 int              `json:"sequence" binding:"min=0"`
	ArtifactType             string           `json:"artifact_type" binding:"required,max=128"`
	MediaType                string           `json:"media_type" binding:"required,oneof=image video audio text document model_3d prompt prompt_template pdf other"`
	SavePolicy               string           `json:"save_policy" binding:"required,oneof=transient manual_save auto_save"`
	ProcessingProfileVersion string           `json:"processing_profile_version" binding:"required,max=128"`
	ExpiresAt                *imachinery.Time `json:"expires_at"`
	Metadata                 map[string]any   `json:"metadata"`
}

// ArtifactListRequest 查询当前用户 Artifact。
type ArtifactListRequest struct {
	imachinery.BasicQueryParam
	ProcessingStatus   string `form:"processing_status" binding:"omitempty,oneof=created transferring processing ready failed deleted"`
	RegistrationStatus string `form:"registration_status" binding:"omitempty,oneof=pending registered failed"`
	ProducerType       string `form:"producer_type" binding:"omitempty,oneof=application_run canvas_run atomic_task"`
}

type ArtifactListResponse struct {
	Total int64       `json:"total"`
	Items []*Artifact `json:"items"`
}

// BatchArtifactSummaryItem 指定一个待解析 Artifact；每个 ID 独立按当前主体裁剪。
type BatchArtifactSummaryItem struct {
	// ID 是 Artifact 稳定标识，不授予额外可见性。
	ID string `json:"id" binding:"required,max=128"`
}

// BatchArtifactSummaryRequest 请求 1..200 个 Artifact 摘要并保持输入顺序。
type BatchArtifactSummaryRequest struct {
	// Items 是受固定批次上限约束的 Artifact 标识数组。
	Items []BatchArtifactSummaryItem `json:"items" binding:"required,min=1,max=200,dive"`
}

type BatchArtifactSummaryResult struct {
	ID       string                   `json:"id"`
	Artifact *ArtifactReadableSummary `json:"artifact"`
}

type BatchArtifactSummaryResponse struct {
	Total int                           `json:"total"`
	Items []*BatchArtifactSummaryResult `json:"items"`
}

type CompleteArtifactRequest struct {
	SHA256                   string         `json:"sha256" binding:"required,len=64,hexadecimal"`
	SizeBytes                int64          `json:"size_bytes" binding:"min=0"`
	MIMEType                 string         `json:"mime_type" binding:"required,max=255"`
	ProcessingProfileVersion string         `json:"processing_profile_version" binding:"required,max=128"`
	Metadata                 map[string]any `json:"metadata"`
}

type RegisterArtifactRequest struct {
	Mode           string `json:"mode" binding:"required,oneof=create_asset append_version"`
	AssetID        string `json:"asset_id" binding:"omitempty,max=64"`
	Name           string `json:"name" binding:"omitempty,max=255"`
	VersionNote    string `json:"version_note" binding:"omitempty,max=1000"`
	ProfileVersion string `json:"profile_version" binding:"omitempty,max=128"`
}

type RegisterRepresentationRequest struct {
	RepresentationType string         `json:"representation_type" binding:"required,oneof=original canonical thumbnail preview playback package manifest"`
	Profile            string         `json:"profile" binding:"required,max=128"`
	ProfileVersion     string         `json:"profile_version" binding:"required,max=128"`
	BlobID             string         `json:"blob_id" binding:"omitempty,max=64"`
	Content            map[string]any `json:"content"`
	Metadata           map[string]any `json:"metadata"`
	Status             string         `json:"status" binding:"required,oneof=ready failed irreparable"`
	Required           bool           `json:"required"`
	ErrorCode          string         `json:"error_code" binding:"omitempty,max=128"`
	ErrorDetail        string         `json:"error_detail" binding:"omitempty,max=2000"`
}

type AssetRepresentationListResponse struct {
	Total int64                  `json:"total"`
	Items []*AssetRepresentation `json:"items"`
}

type RepresentationAccess struct {
	RepresentationID string          `json:"representation_id"`
	AccessURL        string          `json:"access_url"`
	ExpiresAt        imachinery.Time `json:"expires_at"`
}

type AssetRelationView struct {
	ID               string            `json:"id"`
	SourceAssetID    string            `json:"source_asset_id"`
	SourceAsset      *UserAssetSummary `json:"source_asset,omitempty"`
	SourceVersionID  string            `json:"source_version_id,omitempty"`
	RelationType     string            `json:"relation_type"`
	TargetAssetID    string            `json:"target_asset_id"`
	TargetAsset      *UserAssetSummary `json:"target_asset,omitempty"`
	TargetVersionID  string            `json:"target_version_id,omitempty"`
	AtomicTaskID     string            `json:"atomic_task_id,omitempty"`
	TaskAttemptID    string            `json:"task_attempt_id,omitempty"`
	ApplicationRunID string            `json:"application_run_id,omitempty"`
	CanvasRunID      string            `json:"canvas_run_id,omitempty"`
	Metadata         map[string]any    `json:"metadata"`
	CreatedAt        imachinery.Time   `json:"created_at"`
}

type AssetReference struct {
	ID             string          `json:"id"`
	AssetID        string          `json:"asset_id"`
	AssetVersionID string          `json:"asset_version_id,omitempty"`
	SourceType     string          `json:"source_type"`
	SourceID       string          `json:"source_id"`
	SourceLabel    string          `json:"source_label"`
	VersionPolicy  string          `json:"version_policy"`
	CreatedAt      imachinery.Time `json:"created_at"`
}

type AssetUsage struct {
	SourceType  string         `json:"source_type"`
	SourceID    string         `json:"source_id"`
	SourceLabel string         `json:"source_label"`
	Location    map[string]any `json:"location"`
}

type ReferenceSummary struct {
	ReferenceCount int              `json:"reference_count"`
	Sources        []AssetReference `json:"sources"`
}

type AssetRelationListResponse struct {
	Total int64               `json:"total"`
	Items []AssetRelationView `json:"items"`
}
type AssetLineage struct {
	AssetID string              `json:"asset_id"`
	Nodes   []*UserAsset        `json:"nodes"`
	Edges   []AssetRelationView `json:"edges"`
}
type AssetReferenceListResponse struct {
	Total int64            `json:"total"`
	Items []AssetReference `json:"items"`
}
type AssetUsageListResponse struct {
	Total int64        `json:"total"`
	Items []AssetUsage `json:"items"`
}

// AssetContentUpload 是 controller 读取的受控二进制流，不参与 JSON 绑定。
type AssetContentUpload struct {
	FileHeader *multipart.FileHeader `json:"-"`
}
