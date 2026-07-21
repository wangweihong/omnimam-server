package iapiserver

import (
	"encoding/json"

	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

type UserAsset struct {
	imachinery.ObjectMeta
	OwnerUserID     string  `json:"owner_user_id" gorm:"column:owner_user_id;type:varchar(128);not null;index"`
	DisplayName     string  `json:"display_name" gorm:"column:display_name;type:varchar(255);not null"`
	OriginalName    string  `json:"original_name" gorm:"column:original_name;type:varchar(255)"`
	MediaType       string  `json:"media_type" gorm:"column:media_type;type:varchar(32);not null"`
	Format          string  `json:"format" gorm:"column:format;type:varchar(64)"`
	SizeBytes       int64   `json:"size_bytes" gorm:"column:size_bytes;not null"`
	Width           int     `json:"width" gorm:"column:width"`
	Height          int     `json:"height" gorm:"column:height"`
	DurationSeconds float64 `json:"duration_seconds" gorm:"column:duration_seconds"`
	SourceType      string  `json:"source_type" gorm:"column:source_type;type:varchar(32);not null"`
	// Status 表达 active、archived 或 deleted，删除只隐藏素材并保留版本与内容事实。
	Status                 string               `json:"status" gorm:"column:status;type:varchar(16);not null;default:active;index"`
	CurrentVersionID       string               `json:"current_version_id,omitempty" gorm:"column:current_version_id;type:text;index"`
	CurrentVersion         *AssetVersionSummary `json:"current_version,omitempty" gorm:"-"`
	ObjectPath             string               `json:"-" gorm:"column:object_path;type:text;not null;default:''"`
	ThumbnailStatus        string               `json:"thumbnail_status" gorm:"column:thumbnail_status;type:varchar(16);not null"`
	PreviewStatus          string               `json:"preview_status" gorm:"column:preview_status;type:varchar(16);not null;default:none"`
	SHA256                 string               `json:"sha256" gorm:"column:sha256;type:varchar(80)"`
	ReferenceCount         int                  `json:"reference_count" gorm:"column:reference_count;not null;default:0"`
	ReferenceSources       []AssetReference     `json:"-" gorm:"-"`
	ReferenceSourcesShadow string               `json:"-" gorm:"column:reference_sources_json;type:text;not null;default:'[]'"`
	DeletedAt              *imachinery.Time     `json:"deleted_at,omitempty" gorm:"column:deleted_at;type:timestamptz;index"`
	Labels                 map[string]string    `json:"labels" gorm:"-"`
	Tags                   []string             `json:"tags" gorm:"-"`
	LabelSources           map[string]string    `json:"label_sources" gorm:"-"`
	TagSources             map[string]string    `json:"tag_sources" gorm:"-"`
}

func (UserAsset) TableName() string { return "user_assets" }
func (a *UserAsset) BeforeCreate(tx *gorm.DB) error {
	if err := a.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	if a.Status == "" {
		a.Status = "active"
	}
	return a.marshalReferences()
}
func (a *UserAsset) AfterCreate(*gorm.DB) error { return nil }
func (a *UserAsset) BeforeUpdate(tx *gorm.DB) error {
	if err := a.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return a.marshalReferences()
}
func (a *UserAsset) AfterUpdate(*gorm.DB) error { return nil }
func (a *UserAsset) AfterFind(tx *gorm.DB) error {
	if err := a.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	if a.ReferenceSourcesShadow == "" {
		a.ReferenceSources = []AssetReference{}
		return nil
	}
	return json.Unmarshal([]byte(a.ReferenceSourcesShadow), &a.ReferenceSources)
}

func (a *UserAsset) marshalReferences() error {
	if a.ReferenceSources == nil {
		a.ReferenceSources = []AssetReference{}
	}
	raw, err := json.Marshal(a.ReferenceSources)
	if err != nil {
		return err
	}
	a.ReferenceSourcesShadow = string(raw)
	return nil
}

type ArtifactAssetRegistration struct {
	imachinery.ObjectMeta
	ArtifactID         string `json:"artifact_id" gorm:"column:artifact_id;type:varchar(64);not null;uniqueIndex"`
	ApplicationRunID   string `json:"application_run_id,omitempty" gorm:"column:application_run_id;type:varchar(64);index"`
	OwnerUserID        string `json:"owner_user_id" gorm:"column:owner_user_id;type:varchar(128);not null"`
	AssetID            string `json:"asset_id" gorm:"column:asset_id;type:varchar(64);not null;index"`
	AssetVersionID     string `json:"asset_version_id" gorm:"column:asset_version_id;type:varchar(64);not null;uniqueIndex"`
	RegistrationMode   string `json:"registration_mode" gorm:"column:registration_mode;type:varchar(32);not null"`
	RegistrationResult string `json:"registration_result" gorm:"column:registration_result;type:varchar(32);not null"`
	ContentRef         string `json:"-" gorm:"column:content_ref;type:text;not null;default:''"`
	MediaType          string `json:"media_type" gorm:"column:media_type;type:varchar(32);not null"`
}

func (ArtifactAssetRegistration) TableName() string { return "artifact_asset_registrations" }
func (r *ArtifactAssetRegistration) BeforeCreate(tx *gorm.DB) error {
	return r.ObjectMeta.BeforeCreate(tx)
}
func (r *ArtifactAssetRegistration) AfterCreate(*gorm.DB) error { return nil }
func (r *ArtifactAssetRegistration) BeforeUpdate(tx *gorm.DB) error {
	return r.ObjectMeta.BeforeUpdate(tx)
}
func (r *ArtifactAssetRegistration) AfterUpdate(*gorm.DB) error { return nil }

type UserAssetLabel struct {
	imachinery.ObjectMeta
	OwnerUserID string           `json:"-" gorm:"column:owner_user_id;type:varchar(128);not null;index"`
	AssetID     string           `gorm:"column:asset_id;type:varchar(64);not null;uniqueIndex:idx_user_asset_label,priority:1"`
	Key         string           `gorm:"column:label_key;type:varchar(63);not null;uniqueIndex:idx_user_asset_label,priority:2"`
	Value       string           `gorm:"column:label_value;type:varchar(63)"`
	Source      string           `gorm:"column:source;type:varchar(16);not null"`
	DeletedAt   *imachinery.Time `json:"-" gorm:"column:deleted_at;type:timestamptz;index"`
}

func (UserAssetLabel) TableName() string                 { return "user_asset_labels" }
func (l *UserAssetLabel) BeforeCreate(tx *gorm.DB) error { return l.ObjectMeta.BeforeCreate(tx) }
func (l *UserAssetLabel) AfterCreate(*gorm.DB) error     { return nil }
func (l *UserAssetLabel) BeforeUpdate(tx *gorm.DB) error { return l.ObjectMeta.BeforeUpdate(tx) }
func (l *UserAssetLabel) AfterUpdate(*gorm.DB) error     { return nil }

type UserAssetTag struct {
	imachinery.ObjectMeta
	OwnerUserID string           `json:"-" gorm:"column:owner_user_id;type:varchar(128);not null;index"`
	AssetID     string           `gorm:"column:asset_id;type:varchar(64);not null;uniqueIndex:idx_user_asset_tag,priority:1"`
	Tag         string           `gorm:"column:tag;type:varchar(64);not null;uniqueIndex:idx_user_asset_tag,priority:2"`
	Source      string           `gorm:"column:source;type:varchar(16);not null"`
	DeletedAt   *imachinery.Time `json:"-" gorm:"column:deleted_at;type:timestamptz;index"`
}

func (UserAssetTag) TableName() string                 { return "user_asset_tags" }
func (t *UserAssetTag) BeforeCreate(tx *gorm.DB) error { return t.ObjectMeta.BeforeCreate(tx) }
func (t *UserAssetTag) AfterCreate(*gorm.DB) error     { return nil }
func (t *UserAssetTag) BeforeUpdate(tx *gorm.DB) error { return t.ObjectMeta.BeforeUpdate(tx) }
func (t *UserAssetTag) AfterUpdate(*gorm.DB) error     { return nil }
