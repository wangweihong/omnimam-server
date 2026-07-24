package iapiserver

import (
	//"encoding/json"

	"gorm.io/gorm"
	"github.com/wangweihong/gotoolbox/pkg/json"
	"github.com/wangweihong/gotoolbox/pkg/maputil"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	ArtifactProcessingCreated      = "created"
	ArtifactProcessingTransferring = "transferring"
	ArtifactProcessingProcessing   = "processing"
	ArtifactProcessingReady        = "ready"
	ArtifactProcessingFailed       = "failed"
	ArtifactProcessingDeleted      = "deleted"

	ArtifactSaveTransient = "transient"
	ArtifactSaveManual    = "manual_save"
	ArtifactSaveAutomatic = "auto_save"

	AssetVersionStatusUploading         = "uploading"
	AssetVersionStatusProcessing        = "processing"
	AssetVersionStatusReady             = "ready"
	AssetVersionStatusReadyWithWarnings = "ready_with_warnings"
	AssetVersionStatusFailed            = "failed"
)

// AssetVersionSummary 是不可变素材版本的一跳可读摘要，不包含内容和 Representation。
type AssetVersionSummary struct {
	ID         string `json:"id"`
	AssetID    string `json:"asset_id"`
	VersionNo  int    `json:"version_no"`
	Status     string `json:"status"`
	SourceType string `json:"source_type"`
}

// UserAssetSummary 是当前用户素材的一跳可读摘要，不包含标签、版本列表和内容地址。
type UserAssetSummary struct {
	ID               string `json:"id"`
	DisplayName      string `json:"display_name"`
	MediaType        string `json:"media_type"`
	Status           string `json:"status"`
	ThumbnailStatus  string `json:"thumbnail_status"`
	CurrentVersionID string `json:"current_version_id,omitempty"`
}

// ArtifactReadableSummary 是供受控跨域批量识别的 owner 裁剪投影，不包含内容、metadata 或存储引用。
type ArtifactReadableSummary struct {
	ID                 string            `json:"id"`
	OutputKey          string            `json:"output_key"`
	ArtifactType       string            `json:"artifact_type"`
	MediaType          string            `json:"media_type"`
	ProcessingStatus   string            `json:"processing_status"`
	RegistrationStatus string            `json:"registration_status"`
	PreviewAvailable   bool              `json:"preview_available"`
	AssetID            *string           `json:"asset_id"`
	Asset              *UserAssetSummary `json:"asset"`
}

// RelatedResourceSummary 是 Artifact 来源任务或运行的非敏感一跳投影。
type RelatedResourceSummary struct {
	Type   string `json:"type"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// Artifact 是 asset-library 拥有的受控制品事实；内容正文和 Provider 响应不得进入事件 payload。
// +k8s:deepcopy-gen=true
type Artifact struct {
	imachinery.ObjectMeta
	// OwnerUserID 固定为受信 producer 对应的当前用户，是事件投影的 recipient。
	OwnerUserID string `json:"owner_user_id" gorm:"column:owner_user_id;type:text;not null;uniqueIndex:idx_artifacts_producer_key,priority:1;index"`
	// ArtifactType 表达制品业务类型，不替代 media_type。
	ArtifactType string `json:"artifact_type" gorm:"column:artifact_type;type:text;not null"`
	// MediaType 是 SSOT 支持的素材媒体类型。
	MediaType string `json:"media_type" gorm:"column:media_type;type:text;not null"`
	// ProducerType 仅允许 application_run、canvas_run 或 atomic_task。
	ProducerType string `json:"producer_type" gorm:"column:producer_type;type:text;not null;uniqueIndex:idx_artifacts_producer_key,priority:2"`
	// ProducerID 标识产生该制品的所属运行或任务。
	ProducerID string `json:"producer_id" gorm:"column:producer_id;type:text;not null"`
	// Producer 是来源事实源权限裁剪后的可读摘要，不包含运行输入输出。
	Producer *RelatedResourceSummary `json:"producer,omitempty" gorm:"-"`
	// ProducerIdempotencyKey 在 owner 和 producer 类型内稳定标识一个逻辑输出。
	ProducerIdempotencyKey string `json:"producer_idempotency_key" gorm:"column:producer_idempotency_key;type:text;not null;uniqueIndex:idx_artifacts_producer_key,priority:3"`
	// AtomicTaskID 关联 Task Center 事实，不用于推断素材状态。
	AtomicTaskID string `json:"atomic_task_id,omitempty" gorm:"column:atomic_task_id;type:text;index"`
	// AtomicTask 是 Task Center 返回的当前任务摘要。
	AtomicTask *RelatedResourceSummary `json:"atomic_task,omitempty" gorm:"-"`
	// TaskAttemptID 记录实际产生内容的自动执行尝试。
	TaskAttemptID string `json:"task_attempt_id,omitempty" gorm:"column:task_attempt_id;type:text"`
	// ApplicationRunID 关联应用运行的只读投影。
	ApplicationRunID string `json:"application_run_id,omitempty" gorm:"column:application_run_id;type:text;index"`
	// ApplicationRun 是 application-platform 返回的运行摘要。
	ApplicationRun *RelatedResourceSummary `json:"application_run,omitempty" gorm:"-"`
	// CanvasRunID 关联画布运行的只读投影。
	CanvasRunID string `json:"canvas_run_id,omitempty" gorm:"column:canvas_run_id;type:text"`
	// CanvasRun 是 workflow-canvas 返回的运行摘要。
	CanvasRun *RelatedResourceSummary `json:"canvas_run,omitempty" gorm:"-"`
	// NodeRunID 标识产生制品的画布节点运行。
	NodeRunID string `json:"node_run_id,omitempty" gorm:"column:node_run_id;type:text"`
	// NodeID 标识产生制品的画布节点定义。
	NodeID string `json:"node_id,omitempty" gorm:"column:node_id;type:text"`
	// OutputKey 是 producer 输出端口的稳定名称。
	OutputKey string `json:"output_key" gorm:"column:output_key;type:text;not null"`
	// Sequence 区分同一输出端口的多个制品。
	Sequence int `json:"sequence" gorm:"column:sequence;not null;default:0"`
	// BlobID 引用 asset-library 控制的内容对象，不暴露物理路径。
	BlobID string `json:"-" gorm:"column:blob_id;type:text"`
	// Content 只保存受控结构化内容，禁止凭证、任意 URL 和 Provider 原始响应。
	Content       map[string]any `json:"-" gorm:"-"`
	ContentShadow string         `json:"-" gorm:"column:content_json;type:text;not null;default:'{}'"`
	// Metadata 保存媒体信息和有限处理摘要。
	Metadata       map[string]any `json:"-" gorm:"-"`
	MetadataShadow string         `json:"-" gorm:"column:metadata_json;type:text;not null;default:'{}'"`
	// ProcessingStatus 独立表达内容处理生命周期。
	ProcessingStatus string `json:"processing_status" gorm:"column:processing_status;type:text;not null;index"`
	// RegistrationStatus 独立表达登记生命周期。
	RegistrationStatus string `json:"registration_status" gorm:"column:registration_status;type:text;not null;index"`
	// SavePolicy 表达 transient、manual_save 或 auto_save 登记意图。
	SavePolicy string `json:"save_policy" gorm:"column:save_policy;type:text;not null"`
	// ProcessingProfileVersion 固定本次处理策略版本。
	ProcessingProfileVersion string `json:"processing_profile_version" gorm:"column:processing_profile_version;type:text;not null"`
	// PreviewAvailable 表示受保护预览是否已就绪，不代表处理已 ready。
	PreviewAvailable bool `json:"preview_available" gorm:"column:preview_available;not null;default:false"`
	// PreviewRef 是受保护短期引用，不得是长期公开 URL。
	PreviewRef string `json:"preview_ref,omitempty" gorm:"column:preview_ref;type:text"`
	// ThumbnailRef 是受保护短期缩略图引用。
	ThumbnailRef string `json:"thumbnail_ref,omitempty" gorm:"column:thumbnail_ref;type:text"`
	// ProcessingErrorCode 是稳定业务错误码。
	ProcessingErrorCode string `json:"processing_error_code,omitempty" gorm:"column:processing_error_code;type:text"`
	// ProcessingErrorDetail 仅用于服务端诊断，不进入 SSE payload。
	ProcessingErrorDetail string `json:"-" gorm:"column:processing_error_detail;type:text"`
	// RegistrationErrorCode 是登记失败的稳定业务错误码。
	RegistrationErrorCode string `json:"registration_error_code,omitempty" gorm:"column:registration_error_code;type:text"`
	// RegistrationErrorDetail 仅用于服务端诊断，不进入 SSE payload。
	RegistrationErrorDetail string `json:"-" gorm:"column:registration_error_detail;type:text"`
	// AssetID 在登记成功后指向当前用户素材。
	AssetID string `json:"asset_id,omitempty" gorm:"column:asset_id;type:text"`
	// Asset 是登记成功后素材的一跳摘要。
	Asset *UserAssetSummary `json:"asset,omitempty" gorm:"-"`
	// AssetVersionID 在登记成功后指向不可变素材版本。
	AssetVersionID string `json:"asset_version_id,omitempty" gorm:"column:asset_version_id;type:text"`
	// AssetVersion 是登记成功后不可变版本的一跳摘要。
	AssetVersion *AssetVersionSummary `json:"asset_version,omitempty" gorm:"-"`
	// ExpiresAt 仅控制未登记临时制品保留时间。
	ExpiresAt *imachinery.Time `json:"expires_at,omitempty" gorm:"column:expires_at;type:timestamptz;index"`
	// ReadyAt 记录内容首次进入 ready 的时间，用于客户端增量展示。
	ReadyAt *imachinery.Time `json:"-" gorm:"-"`
	// DeletedAt 是逻辑删除时间，不级联删除已登记素材。
	DeletedAt *imachinery.Time `json:"-" gorm:"column:deleted_at;type:timestamptz;index"`
}

func (Artifact) TableName() string { return "artifacts" }
func (a *Artifact) BeforeCreate(tx *gorm.DB) error {
	if err := a.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return a.marshal()
}
func (*Artifact) AfterCreate(*gorm.DB) error { return nil }
func (a *Artifact) BeforeUpdate(tx *gorm.DB) error {
	if err := a.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return a.marshal()
}
func (*Artifact) AfterUpdate(*gorm.DB) error { return nil }
func (a *Artifact) AfterFind(tx *gorm.DB) error {
	if err := a.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	return unmarshalAssetLifecycleJSON(a.ContentShadow, &a.Content, a.MetadataShadow, &a.Metadata)
}
func (a *Artifact) marshal() error {
	return marshalAssetLifecycleJSON(a.Content, &a.ContentShadow, a.Metadata, &a.MetadataShadow)
}

// AssetVersion 是 user asset 的不可变内容版本及 Representation 处理汇总事实。
// +k8s:deepcopy-gen=true
type AssetVersion struct {
	imachinery.ObjectMeta
	// AssetID 指向所属 UserAsset。
	AssetID string `json:"asset_id" gorm:"column:asset_id;type:text;not null;uniqueIndex:idx_asset_versions_number,priority:1;index"`
	// OwnerUserID 固定为素材 owner，并作为 SSE recipient。
	OwnerUserID string `json:"owner_user_id" gorm:"column:owner_user_id;type:text;not null;index"`
	// VersionNo 在同一 Asset 内单调递增且不可覆盖。
	VersionNo int `json:"version_no" gorm:"column:version_no;not null;uniqueIndex:idx_asset_versions_number,priority:2"`
	// Status 由 Representation 必需项和可选项汇总，不能从 Task 状态推断。
	Status string `json:"status" gorm:"column:status;type:text;not null;index"`
	// SourceType 标识 upload、artifact、asset_edit、asset_conversion 或 external_import。
	SourceType string `json:"source_type" gorm:"column:source_type;type:text;not null"`
	// SourceRefID 关联来源 Artifact 或导入记录。
	SourceRefID string `json:"source_ref_id,omitempty" gorm:"column:source_ref_id;type:text"`
	// Content 保存 canonical 等受控结构化内容。
	Content       map[string]any `json:"content" gorm:"-"`
	ContentShadow string         `json:"-" gorm:"column:content_json;type:text;not null;default:'{}'"`
	// Metadata 保存有限媒体信息，不保存正文或 Provider 响应。
	Metadata       map[string]any `json:"metadata" gorm:"-"`
	MetadataShadow string         `json:"-" gorm:"column:metadata_json;type:text;not null;default:'{}'"`
	// VersionNote 是用户可读的版本说明。
	VersionNote string `json:"version_note" gorm:"column:version_note;type:text"`
	// ProcessingError 保存有限错误摘要，SSE 只暴露稳定 error_code。
	ProcessingError       map[string]any `json:"-" gorm:"-"`
	ProcessingErrorShadow string         `json:"-" gorm:"column:processing_error_json;type:text;not null;default:'{}'"`
	// ProfileVersion 固定 expected Representation policy 版本。
	ProfileVersion string `json:"profile_version" gorm:"column:profile_version;type:text;not null"`
	// ExpectedCount 是当前 policy 期望的 Representation 数量。
	ExpectedCount int `json:"expected_count" gorm:"column:expected_count;not null;default:0"`
	// CompletedCount 是已完成的 Representation 数量。
	CompletedCount int `json:"completed_count" gorm:"column:completed_count;not null;default:0"`
	// FailedCount 是已失败或不可恢复的 Representation 数量。
	FailedCount int `json:"failed_count" gorm:"column:failed_count;not null;default:0"`
	// DeletedAt 保留逻辑删除时间，历史版本不物理覆盖。
	DeletedAt *imachinery.Time `json:"-" gorm:"column:deleted_at;type:timestamptz;index"`
}

func (AssetVersion) TableName() string { return "asset_versions" }
func (v *AssetVersion) BeforeCreate(tx *gorm.DB) error {
	if err := v.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return v.marshal()
}
func (*AssetVersion) AfterCreate(*gorm.DB) error { return nil }
func (v *AssetVersion) BeforeUpdate(tx *gorm.DB) error {
	if err := v.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return v.marshal()
}
func (*AssetVersion) AfterUpdate(*gorm.DB) error { return nil }
func (v *AssetVersion) AfterFind(tx *gorm.DB) error {
	if err := v.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	if err := unmarshalAssetLifecycleJSON(v.ContentShadow, &v.Content, v.MetadataShadow, &v.Metadata); err != nil {
		return err
	}
	if v.ProcessingErrorShadow == "" {
		v.ProcessingError = map[string]any{}
		return nil
	}
	return json.Unmarshal([]byte(v.ProcessingErrorShadow), &v.ProcessingError)
}
func (v *AssetVersion) marshal() error {
	if err := marshalAssetLifecycleJSON(v.Content, &v.ContentShadow, v.Metadata, &v.MetadataShadow); err != nil {
		return err
	}
	raw, err := json.Marshal(maputil.NonNilMap(v.ProcessingError))
	if err != nil {
		return err
	}
	v.ProcessingErrorShadow = string(raw)
	return nil
}

func marshalAssetLifecycleJSON(content map[string]any, contentShadow *string, metadata map[string]any, metadataShadow *string) error {
	for _, item := range []struct {
		value  map[string]any
		shadow *string
	}{{content, contentShadow}, {metadata, metadataShadow}} {
		raw, err := json.Marshal(maputil.NonNilMap(item.value))
		if err != nil {
			return err
		}
		*item.shadow = string(raw)
	}
	return nil
}

func unmarshalAssetLifecycleJSON(contentShadow string, content *map[string]any, metadataShadow string, metadata *map[string]any) error {
	for _, item := range []struct {
		raw    string
		target *map[string]any
	}{{contentShadow, content}, {metadataShadow, metadata}} {
		if item.raw == "" {
			*item.target = map[string]any{}
			continue
		}
		if err := json.Unmarshal([]byte(item.raw), item.target); err != nil {
			return err
		}
	}
	return nil
}
