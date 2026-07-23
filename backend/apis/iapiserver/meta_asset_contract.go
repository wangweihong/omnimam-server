package iapiserver

import (
	"encoding/json"

	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	AssetStatusActive   = "active"
	AssetStatusArchived = "archived"
	AssetStatusDeleted  = "deleted"

	AssetRepresentationOriginal  = "original"
	AssetRepresentationCanonical = "canonical"
)

// AssetBlob 记录受 StorageBackend 管理的物理内容；普通素材 API 不返回物理定位字段，管理员详情单独投影。
type AssetBlob struct {
	imachinery.ObjectMeta
	// StorageBackendID 指向承载该内容的全局存储后端，仅管理员物理检查接口可见。
	StorageBackendID string `json:"-" gorm:"column:storage_backend_id;type:text;not null;index;uniqueIndex:idx_blob_backend_object,priority:1"`
	// ObjectKey 是后端内完整对象键，仅管理员物理检查接口可见。
	ObjectKey string `json:"-" gorm:"column:object_key;type:text;not null;uniqueIndex:idx_blob_backend_object,priority:2"`
	// SHA256 是内容摘要，用于完整性检查与受控去重。
	SHA256 string `json:"sha256" gorm:"column:sha256;type:text;not null;index"`
	// SizeBytes 是 Blob 的实际字节数。
	SizeBytes int64 `json:"size_bytes" gorm:"column:size_bytes;not null"`
	// MIMEType 是已登记内容的媒体类型。
	MIMEType string `json:"mime_type" gorm:"column:mime_type;type:text;not null"`
	// Status 表示物理内容是否可用、损坏、缺失或处于删除流程。
	Status string `json:"status" gorm:"column:status;type:text;not null;index"`
}

// AssetBlobDetail 是仅管理员可见的 Blob 物理存储投影。
// 它只返回 StorageBackend ID，不递归嵌入可能包含凭证的后端配置。
type AssetBlobDetail struct {
	// ID 是全局 Blob 标识。
	ID string `json:"id"`
	// Name 是 Blob 的通用资源名称。
	Name string `json:"name"`
	// Description 是 Blob 的通用资源说明。
	Description string `json:"description"`
	// Extend 返回受控扩展字段；无扩展时返回空对象。
	Extend map[string]any `json:"extend"`
	// StorageBackendID 指向承载该内容的 StorageBackend。
	StorageBackendID string `json:"storage_backend_id"`
	// ObjectKey 是后端内完整对象键，仅管理员可见。
	ObjectKey string `json:"object_key"`
	// SHA256 是内容摘要。
	SHA256 string `json:"sha256"`
	// SizeBytes 是内容字节数。
	SizeBytes int64 `json:"size_bytes"`
	// MIMEType 是内容媒体类型。
	MIMEType string `json:"mime_type"`
	// Status 表示物理内容当前可用性与删除状态。
	Status string `json:"status"`
	// ResourceVersion 用于识别 Blob 元数据版本。
	ResourceVersion int64 `json:"resource_version"`
	// CreatedAt 是 Blob 登记时间。
	CreatedAt imachinery.Time `json:"created_at"`
	// UpdatedAt 是 Blob 元数据最后更新时间。
	UpdatedAt imachinery.Time `json:"updated_at"`
}

func (AssetBlob) TableName() string                 { return "blobs" }
func (b *AssetBlob) BeforeCreate(tx *gorm.DB) error { return b.ObjectMeta.BeforeCreate(tx) }
func (*AssetBlob) AfterCreate(*gorm.DB) error       { return nil }
func (b *AssetBlob) BeforeUpdate(tx *gorm.DB) error { return b.ObjectMeta.BeforeUpdate(tx) }
func (*AssetBlob) AfterUpdate(*gorm.DB) error       { return nil }
func (b *AssetBlob) AfterFind(tx *gorm.DB) error    { return b.ObjectMeta.AfterFind(tx) }

// AssetRepresentation 是同一 AssetVersion 的受控技术表现形式。
type AssetRepresentation struct {
	imachinery.ObjectMeta
	AssetVersionID     string           `json:"asset_version_id" gorm:"column:asset_version_id;type:text;not null;uniqueIndex:idx_representation_identity,priority:1;index"`
	OwnerUserID        string           `json:"-" gorm:"column:owner_user_id;type:text;not null;index"`
	RepresentationType string           `json:"representation_type" gorm:"column:representation_type;type:text;not null;uniqueIndex:idx_representation_identity,priority:2"`
	Profile            string           `json:"profile" gorm:"column:profile;type:text;not null;uniqueIndex:idx_representation_identity,priority:3"`
	ProfileVersion     string           `json:"profile_version" gorm:"column:profile_version;type:text;not null;uniqueIndex:idx_representation_identity,priority:4"`
	BlobID             string           `json:"blob_id,omitempty" gorm:"column:blob_id;type:text;index"`
	Content            map[string]any   `json:"-" gorm:"-"`
	ContentShadow      string           `json:"-" gorm:"column:content_json;type:text;not null;default:'{}'"`
	Metadata           map[string]any   `json:"metadata" gorm:"-"`
	MetadataShadow     string           `json:"-" gorm:"column:metadata_json;type:text;not null;default:'{}'"`
	Status             string           `json:"status" gorm:"column:status;type:text;not null;index"`
	Required           bool             `json:"required" gorm:"column:required;not null;default:false"`
	RetryCount         int              `json:"retry_count" gorm:"column:retry_count;not null;default:0"`
	RetryAfter         *imachinery.Time `json:"-" gorm:"column:retry_after;type:timestamptz"`
	ErrorCode          string           `json:"error_code,omitempty" gorm:"column:error_code;type:text"`
	ErrorDetail        string           `json:"error_detail,omitempty" gorm:"column:error_detail;type:text"`
	DeletedAt          *imachinery.Time `json:"-" gorm:"column:deleted_at;type:timestamptz;index"`
	Format             string           `json:"format" gorm:"-"`
	MIMEType           string           `json:"mime_type" gorm:"-"`
}

func (AssetRepresentation) TableName() string { return "asset_representations" }
func (r *AssetRepresentation) BeforeCreate(tx *gorm.DB) error {
	if err := r.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return r.marshal()
}
func (*AssetRepresentation) AfterCreate(*gorm.DB) error { return nil }
func (r *AssetRepresentation) BeforeUpdate(tx *gorm.DB) error {
	if err := r.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return r.marshal()
}
func (*AssetRepresentation) AfterUpdate(*gorm.DB) error { return nil }
func (r *AssetRepresentation) AfterFind(tx *gorm.DB) error {
	if err := r.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	return unmarshalJSONMaps(r.ContentShadow, &r.Content, r.MetadataShadow, &r.Metadata)
}
func (r *AssetRepresentation) marshal() error {
	return marshalJSONMaps(r.Content, &r.ContentShadow, r.Metadata, &r.MetadataShadow)
}

// AssetUploadSession 保存普通或分片上传的可恢复会话与临时标签输入。
type AssetUploadSession struct {
	imachinery.ObjectMeta
	OwnerUserID         string            `json:"-" gorm:"column:owner_user_id;type:text;not null;uniqueIndex:idx_upload_owner_key,priority:1;index"`
	ClientUploadKey     string            `json:"client_upload_key" gorm:"column:client_upload_key;type:text;not null;uniqueIndex:idx_upload_owner_key,priority:2"`
	SHA256              string            `json:"sha256" gorm:"column:sha256;type:text;not null;index"`
	FileName            string            `json:"file_name" gorm:"column:file_name;type:text;not null"`
	DisplayName         string            `json:"-" gorm:"column:display_name;type:text;not null"`
	MIMEType            string            `json:"mime_type" gorm:"column:mime_type;type:text;not null"`
	SizeBytes           int64             `json:"size_bytes" gorm:"column:size_bytes;not null"`
	TargetAssetID       string            `json:"target_asset_id,omitempty" gorm:"column:target_asset_id;type:text;index"`
	VersionNote         string            `json:"-" gorm:"column:version_note;type:text"`
	ProfileVersion      string            `json:"-" gorm:"column:profile_version;type:text;not null"`
	UploadMode          string            `json:"upload_mode" gorm:"column:upload_mode;type:text;not null"`
	ChunkSizeBytes      int64             `json:"chunk_size_bytes" gorm:"column:chunk_size_bytes;not null;default:0"`
	UploadedParts       []UploadedPart    `json:"uploaded_parts" gorm:"-"`
	UploadedPartsShadow string            `json:"-" gorm:"column:uploaded_parts_json;type:text;not null;default:'[]'"`
	Status              string            `json:"status" gorm:"column:status;type:text;not null;index"`
	PendingLabels       map[string]string `json:"-" gorm:"-"`
	PendingLabelsShadow string            `json:"-" gorm:"column:pending_labels_payload;type:text;not null;default:'{}'"`
	PendingTags         []string          `json:"-" gorm:"-"`
	PendingTagsShadow   string            `json:"-" gorm:"column:pending_tags_payload;type:text;not null;default:'[]'"`
}

func (AssetUploadSession) TableName() string { return "user_asset_upload_sessions" }
func (u *AssetUploadSession) BeforeCreate(tx *gorm.DB) error {
	if err := u.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return u.marshal()
}
func (*AssetUploadSession) AfterCreate(*gorm.DB) error { return nil }
func (u *AssetUploadSession) BeforeUpdate(tx *gorm.DB) error {
	if err := u.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return u.marshal()
}
func (*AssetUploadSession) AfterUpdate(*gorm.DB) error { return nil }
func (u *AssetUploadSession) AfterFind(tx *gorm.DB) error {
	if err := u.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(defaultJSON(u.UploadedPartsShadow, "[]")), &u.UploadedParts); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(defaultJSON(u.PendingLabelsShadow, "{}")), &u.PendingLabels); err != nil {
		return err
	}
	return json.Unmarshal([]byte(defaultJSON(u.PendingTagsShadow, "[]")), &u.PendingTags)
}
func (u *AssetUploadSession) marshal() error {
	return marshalValues(
		[]any{nonNilParts(u.UploadedParts), nonNilStringMap(u.PendingLabels), nonNilStrings(u.PendingTags)},
		[]*string{&u.UploadedPartsShadow, &u.PendingLabelsShadow, &u.PendingTagsShadow},
	)
}

// AssetCollection 是用户范围逻辑分组，对外使用 Collection 术语。
type AssetCollection struct {
	imachinery.ObjectMeta
	OwnerUserID        string `json:"-" gorm:"column:owner_user_id;type:text;not null;index"`
	ParentCollectionID string `json:"parent_collection_id,omitempty" gorm:"column:parent_group_id;type:text;index"`
	// ParentCollection 是直接父级的一跳摘要，不递归展开祖先。
	ParentCollection *CollectionSummary `json:"parent_collection,omitempty" gorm:"-"`
	Color            string             `json:"color" gorm:"column:color;type:text;not null;default:''"`
	SortOrder        int                `json:"sort_order" gorm:"column:sort_order;not null;default:0"`
	DeletedAt        *imachinery.Time   `json:"-" gorm:"column:deleted_at;type:timestamptz;index"`
	Depth            int                `json:"depth" gorm:"-"`
	ItemCount        int64              `json:"item_count" gorm:"-"`
}

// CollectionSummary 是 Collection 的一跳可读摘要，不包含父级和成员。
type CollectionSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	Depth     int    `json:"depth"`
	ItemCount int64  `json:"item_count"`
}

func (AssetCollection) TableName() string                 { return "user_asset_groups" }
func (c *AssetCollection) BeforeCreate(tx *gorm.DB) error { return c.ObjectMeta.BeforeCreate(tx) }
func (*AssetCollection) AfterCreate(*gorm.DB) error       { return nil }
func (c *AssetCollection) BeforeUpdate(tx *gorm.DB) error { return c.ObjectMeta.BeforeUpdate(tx) }
func (*AssetCollection) AfterUpdate(*gorm.DB) error       { return nil }
func (c *AssetCollection) AfterFind(tx *gorm.DB) error    { return c.ObjectMeta.AfterFind(tx) }

// AssetCollectionItem 保存 Collection 与 Asset 的多对多关系和可选固定版本。
type AssetCollectionItem struct {
	imachinery.ObjectMeta
	OwnerUserID     string `json:"-" gorm:"column:owner_user_id;type:text;not null;index;uniqueIndex:idx_collection_member,priority:1"`
	CollectionID    string `json:"collection_id" gorm:"column:group_id;type:text;not null;index;uniqueIndex:idx_collection_member,priority:2"`
	AssetID         string `json:"asset_id" gorm:"column:asset_id;type:text;not null;index;uniqueIndex:idx_collection_member,priority:3"`
	PinnedVersionID string `json:"pinned_version_id,omitempty" gorm:"column:pinned_version_id;type:text;index"`
	// PinnedVersion 是成员固定版本的一跳摘要；跟随当前版本时为空。
	PinnedVersion  *AssetVersionSummary `json:"pinned_version,omitempty" gorm:"-"`
	Role           string               `json:"role" gorm:"column:role;type:text;not null;default:''"`
	SortOrder      int                  `json:"sort_order" gorm:"column:sort_order;not null;default:0"`
	Metadata       map[string]any       `json:"metadata" gorm:"-"`
	MetadataShadow string               `json:"-" gorm:"column:metadata_json;type:text;not null;default:'{}'"`
	CreatedBy      string               `json:"-" gorm:"column:created_by;type:text;not null"`
	JoinedAt       imachinery.Time      `json:"-" gorm:"column:joined_at;type:timestamptz;not null"`
	DeletedAt      *imachinery.Time     `json:"-" gorm:"column:deleted_at;type:timestamptz;index"`
	Asset          *UserAsset           `json:"asset,omitempty" gorm:"-"`
}

func (AssetCollectionItem) TableName() string { return "user_asset_group_memberships" }
func (i *AssetCollectionItem) BeforeCreate(tx *gorm.DB) error {
	if err := i.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	if i.JoinedAt.IsZero() {
		i.JoinedAt = i.CreatedAt
	}
	return marshalValues([]any{nonNilAnyMap(i.Metadata)}, []*string{&i.MetadataShadow})
}
func (*AssetCollectionItem) AfterCreate(*gorm.DB) error { return nil }
func (i *AssetCollectionItem) BeforeUpdate(tx *gorm.DB) error {
	if err := i.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return marshalValues([]any{nonNilAnyMap(i.Metadata)}, []*string{&i.MetadataShadow})
}
func (*AssetCollectionItem) AfterUpdate(*gorm.DB) error { return nil }
func (i *AssetCollectionItem) AfterFind(tx *gorm.DB) error {
	if err := i.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	return json.Unmarshal([]byte(defaultJSON(i.MetadataShadow, "{}")), &i.Metadata)
}

func marshalJSONMaps(first map[string]any, firstShadow *string, second map[string]any, secondShadow *string) error {
	return marshalValues([]any{nonNilAnyMap(first), nonNilAnyMap(second)}, []*string{firstShadow, secondShadow})
}

func unmarshalJSONMaps(first string, firstValue *map[string]any, second string, secondValue *map[string]any) error {
	if err := json.Unmarshal([]byte(defaultJSON(first, "{}")), firstValue); err != nil {
		return err
	}
	return json.Unmarshal([]byte(defaultJSON(second, "{}")), secondValue)
}

func marshalValues(values []any, shadows []*string) error {
	for index, value := range values {
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		*shadows[index] = string(raw)
	}
	return nil
}

func defaultJSON(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func nonNilAnyMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}
func nonNilStringMap(value map[string]string) map[string]string {
	if value == nil {
		return map[string]string{}
	}
	return value
}
func nonNilStrings(value []string) []string {
	if value == nil {
		return []string{}
	}
	return value
}
func nonNilParts(value []UploadedPart) []UploadedPart {
	if value == nil {
		return []UploadedPart{}
	}
	return value
}
