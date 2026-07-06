package iapiserver

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	ProviderTypeOpenAICompatible = "openai-compatible"

	ProviderAuthTypeAPIKey = "api_key"

	// ProviderModelHealthUnknown 表示模型尚未完成健康检测。
	ProviderModelHealthUnknown = "unknown"
	// ProviderModelHealthHealthy 表示最近一次检测确认模型可连接。
	ProviderModelHealthHealthy = "healthy"
	// ProviderModelHealthUnhealthy 表示最近一次检测确认模型不可用，需要阻止下游选择。
	ProviderModelHealthUnhealthy = "unhealthy"

	CapabilityLLMChat        = "llm.chat"
	CapabilityQueryParse     = "query.parse"
	CapabilityPromptGenerate = "prompt.generate"
	CapabilityAssetTagging   = "asset.tagging"
	CapabilityOCRExtract     = "ocr.extract"

	StorageBackendTypeLocal = "local"
	StorageBackendTypeS3    = "s3"
	StorageBackendTypeOSS   = "oss"
	StorageBackendTypeMinIO = "minio"

	AssetMediaTypeImage          = "image"
	AssetMediaTypeVideo          = "video"
	AssetMediaTypeAudio          = "audio"
	AssetMediaTypePDF            = "pdf"
	AssetMediaTypeText           = "text"
	AssetMediaTypeJSON           = "json"
	AssetMediaTypeMarkdown       = "markdown"
	AssetMediaTypePrompt         = "prompt"
	AssetMediaTypePromptTemplate = "prompt_template"
	AssetMediaTypeWorkflow       = "workflow"
	AssetMediaTypeOther          = "other"

	AssetSourceUserUpload = "user_upload"
	AssetSourceSystem     = "system"
	AssetSourceGenerated  = "generated"
	AssetSourceImported   = "imported"

	ThumbnailStatusPending     = "pending"
	ThumbnailStatusProcessing  = "processing"
	ThumbnailStatusReady       = "ready"
	ThumbnailStatusUnsupported = "unsupported"
	ThumbnailStatusFailed      = "failed"

	TagSourceUser     = "user"
	TagSourceSystem   = "system"
	TagSourceAI       = "ai"
	TagSourceBusiness = "business"

	AssetGroupTypeCollection = "collection"
	AssetGroupTypeDataset    = "dataset"
	AssetGroupTypeDynamic    = "dynamic"

	TaskStatusPending   = "pending"
	TaskStatusRunning   = "running"
	TaskStatusSucceeded = "succeeded"
	TaskStatusFailed    = "failed"
	TaskStatusCanceled  = "canceled"

	TaskTypeAssetProbe     = "asset.probe"
	TaskTypeAssetThumbnail = "asset.thumbnail"
	TaskTypeLLMInvoke      = "llm.invoke"
	TaskTypeAssetTagging   = "asset.tagging"
	TaskTypeQueryParse     = "query.parse"

	TaskTypeCanvasRun                   = "canvas.run"
	TaskTypeCanvasNodeRun               = "canvas.node.run"
	TaskTypeCanvasWorkflowPackageExport = "canvas.workflow.package.export"
	TaskTypeCanvasWorkflowPackageImport = "canvas.workflow.package.import"
	TaskTypeCanvasOutputRegister        = "canvas.output.register"
	TaskTypeProviderInvoke              = "provider.invoke"
	TaskTypeProviderQuery               = "provider.query"
)

type Provider struct {
	imachinery.ObjectMeta
	Type          string         `json:"type"                     gorm:"column:type;type:varchar(64);not null;index"`
	Enabled       bool           `json:"enabled"                  gorm:"column:enabled;type:boolean;not null"`
	BaseURL       string         `json:"base_url"                 gorm:"column:base_url;type:varchar(512)"`
	AuthType      string         `json:"auth_type"                gorm:"column:auth_type;type:varchar(64)"`
	CredentialRef string         `json:"credential_ref,omitempty" gorm:"column:credential_ref;type:varchar(512)"`
	PresetKey     string         `json:"preset_key,omitempty"     gorm:"column:preset_key;type:varchar(64);index"`
	Config        map[string]any `json:"config,omitempty"         gorm:"-"`
	ConfigShadow  string         `json:"-"                        gorm:"column:config;type:text"`
}

func (Provider) TableName() string { return "providers" }

func (p *Provider) BeforeCreate(tx *gorm.DB) error {
	if err := p.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return p.marshalShadows()
}

func (p *Provider) BeforeUpdate(tx *gorm.DB) error {
	if err := p.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return p.marshalShadows()
}

func (p *Provider) AfterFind(tx *gorm.DB) error {
	if err := p.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	_ = json.Unmarshal([]byte(p.ConfigShadow), &p.Config)
	return nil
}

func (p *Provider) marshalShadows() error {
	data, err := json.Marshal(p.Config)
	if err != nil {
		return err
	}
	p.ConfigShadow = string(data)
	return nil
}

type ProviderModel struct {
	imachinery.ObjectMeta
	ProviderID          string         `json:"provider_id"              gorm:"column:provider_id;type:varchar(64);not null;index"`
	Model               string         `json:"model"                    gorm:"column:model;type:varchar(128);not null;index"`
	EndpointType        string         `json:"endpoint_type,omitempty"   gorm:"column:endpoint_type;type:varchar(64);index"`
	GroupName           string         `json:"group_name,omitempty"      gorm:"column:group_name;type:varchar(128);index"`
	HealthStatus        string         `json:"health_status"            gorm:"column:health_status;type:varchar(32);not null;default:unknown;index"`
	HealthReason        string         `json:"health_reason,omitempty"   gorm:"column:health_reason;type:varchar(512)"`
	HealthCheckedAt     *time.Time     `json:"health_checked_at"        gorm:"column:health_checked_at"`
	Capabilities        []string       `json:"capabilities"             gorm:"-"`
	CapabilitiesShadow  string         `json:"-"                        gorm:"column:capabilities;type:text"`
	ModelTypes          []string       `json:"model_types,omitempty"     gorm:"-"`
	ModelTypesShadow    string         `json:"-"                        gorm:"column:model_types;type:text"`
	Enabled             bool           `json:"enabled"                  gorm:"column:enabled;type:boolean;not null;default:true"`
	DefaultParams       map[string]any `json:"default_params,omitempty" gorm:"-"`
	DefaultParamsShadow string         `json:"-"                        gorm:"column:default_params;type:text"`
	Pricing             map[string]any `json:"pricing,omitempty"        gorm:"-"`
	PricingShadow       string         `json:"-"                        gorm:"column:pricing;type:text"`
}

func (ProviderModel) TableName() string { return "provider_models" }

func (m *ProviderModel) BeforeCreate(tx *gorm.DB) error {
	if err := m.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return m.marshalShadows()
}

func (m *ProviderModel) BeforeUpdate(tx *gorm.DB) error {
	if err := m.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return m.marshalShadows()
}

func (m *ProviderModel) AfterFind(tx *gorm.DB) error {
	if err := m.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	_ = json.Unmarshal([]byte(m.CapabilitiesShadow), &m.Capabilities)
	_ = json.Unmarshal([]byte(m.ModelTypesShadow), &m.ModelTypes)
	_ = json.Unmarshal([]byte(m.DefaultParamsShadow), &m.DefaultParams)
	_ = json.Unmarshal([]byte(m.PricingShadow), &m.Pricing)
	return nil
}

func (m *ProviderModel) marshalShadows() error {
	capabilities, err := json.Marshal(m.Capabilities)
	if err != nil {
		return err
	}
	modelTypes, err := json.Marshal(m.ModelTypes)
	if err != nil {
		return err
	}
	params, err := json.Marshal(m.DefaultParams)
	if err != nil {
		return err
	}
	pricing, err := json.Marshal(m.Pricing)
	if err != nil {
		return err
	}
	m.CapabilitiesShadow = string(capabilities)
	m.ModelTypesShadow = string(modelTypes)
	m.DefaultParamsShadow = string(params)
	m.PricingShadow = string(pricing)
	return nil
}

type ProviderCapability struct {
	imachinery.ObjectMeta
}

func (ProviderCapability) TableName() string { return "provider_capabilities" }

type SystemLLMConfig struct {
	imachinery.ObjectMeta
	Purpose    string `json:"purpose"     gorm:"column:purpose;type:varchar(64);not null;uniqueIndex"`
	ProviderID string `json:"provider_id" gorm:"column:provider_id;type:varchar(64);not null;index"`
	ModelID    string `json:"model_id"    gorm:"column:model_id;type:varchar(64);index"`
	Model      string `json:"model"       gorm:"column:model;type:varchar(128)"`
	Enabled    bool   `json:"enabled"     gorm:"column:enabled;type:boolean;not null;default:true"`
}

func (SystemLLMConfig) TableName() string { return "system_llm_configs" }

type StorageBackend struct {
	imachinery.ObjectMeta
	Type         string         `json:"type"             gorm:"column:type;type:varchar(64);not null;index"`
	Root         string         `json:"root"             gorm:"column:root;type:varchar(1024)"`
	Config       map[string]any `json:"config,omitempty" gorm:"-"`
	ConfigShadow string         `json:"-"                gorm:"column:config;type:text"`
	Enabled      bool           `json:"enabled"          gorm:"column:enabled;type:boolean;not null;default:true"`
	Readonly     bool           `json:"readonly"         gorm:"column:readonly;type:boolean;not null;default:false"`
	Quota        int64          `json:"quota"            gorm:"column:quota"`
}

func (StorageBackend) TableName() string { return "storage_backends" }

func (b *StorageBackend) BeforeCreate(tx *gorm.DB) error {
	if err := b.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return b.marshalShadows()
}

func (b *StorageBackend) BeforeUpdate(tx *gorm.DB) error {
	if err := b.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return b.marshalShadows()
}

func (b *StorageBackend) AfterFind(tx *gorm.DB) error {
	if err := b.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	_ = json.Unmarshal([]byte(b.ConfigShadow), &b.Config)
	return nil
}

func (b *StorageBackend) marshalShadows() error {
	data, err := json.Marshal(b.Config)
	if err != nil {
		return err
	}
	b.ConfigShadow = string(data)
	return nil
}

type Asset struct {
	imachinery.ObjectMeta
	MediaType        string         `json:"media_type"         gorm:"column:media_type;type:varchar(64);not null;index"`
	MimeType         string         `json:"mime_type"          gorm:"column:mime_type;type:varchar(128);index"`
	StorageBackendID string         `json:"storage_backend_id" gorm:"column:storage_backend_id;type:varchar(64);not null;index"`
	ObjectKey        string         `json:"object_key"         gorm:"column:object_key;type:varchar(1024);not null"`
	Size             int64          `json:"size"               gorm:"column:size;index"`
	Checksum         string         `json:"checksum"           gorm:"column:checksum;type:varchar(128);index"`
	Width            int            `json:"width"              gorm:"column:width;index"`
	Height           int            `json:"height"             gorm:"column:height;index"`
	Duration         int64          `json:"duration"           gorm:"column:duration;index"`
	Format           string         `json:"format"             gorm:"column:format;type:varchar(32);index"`
	SourceType       string         `json:"source_type"        gorm:"column:source_type;type:varchar(64);index"`
	SourceRef        string         `json:"source_ref"         gorm:"column:source_ref;type:varchar(256);index"`
	DeletedAt        int64          `json:"deleted_at"         gorm:"column:deleted_at;not null;default:0;index"`
	Metadata         map[string]any `json:"metadata,omitempty" gorm:"-"`
	MetadataShadow   string         `json:"-"                  gorm:"column:metadata;type:text"`
}

func (Asset) TableName() string { return "assets" }

func (a *Asset) BeforeCreate(tx *gorm.DB) error {
	if err := a.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return a.marshalShadows()
}

func (a *Asset) BeforeUpdate(tx *gorm.DB) error {
	if err := a.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return a.marshalShadows()
}

func (a *Asset) AfterFind(tx *gorm.DB) error {
	if err := a.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	_ = json.Unmarshal([]byte(a.MetadataShadow), &a.Metadata)
	return nil
}

func (a *Asset) marshalShadows() error {
	data, err := json.Marshal(a.Metadata)
	if err != nil {
		return err
	}
	a.MetadataShadow = string(data)
	return nil
}

type AssetThumbnail struct {
	imachinery.ObjectMeta
	AssetID          string `json:"asset_id"           gorm:"column:asset_id;type:varchar(64);not null;index"`
	StorageBackendID string `json:"storage_backend_id" gorm:"column:storage_backend_id;type:varchar(64);not null;index"`
	ObjectKey        string `json:"object_key"         gorm:"column:object_key;type:varchar(1024)"`
	Width            int    `json:"width"              gorm:"column:width"`
	Height           int    `json:"height"             gorm:"column:height"`
	MimeType         string `json:"mime_type"          gorm:"column:mime_type;type:varchar(128)"`
	Size             int64  `json:"size"               gorm:"column:size"`
	Status           string `json:"status"             gorm:"column:status;type:varchar(32);not null;default:pending;index"`
}

func (AssetThumbnail) TableName() string { return "asset_thumbnails" }

type Tag struct {
	imachinery.ObjectMeta
	Source string `json:"source" gorm:"column:source;type:varchar(32);not null;default:user;index"`
}

func (Tag) TableName() string { return "tags" }

type AssetTag struct {
	imachinery.ObjectMeta
	AssetID string `json:"asset_id" gorm:"column:asset_id;type:varchar(64);not null;index"`
	TagID   string `json:"tag_id"   gorm:"column:tag_id;type:varchar(64);not null;index"`
	Source  string `json:"source"   gorm:"column:source;type:varchar(32);not null;default:user;index"`
}

func (AssetTag) TableName() string { return "asset_tags" }

type AssetGroup struct {
	imachinery.ObjectMeta
	Type              string         `json:"type"                   gorm:"column:type;type:varchar(64);not null;default:collection;index"`
	DynamicRule       map[string]any `json:"dynamic_rule,omitempty" gorm:"-"`
	DynamicRuleShadow string         `json:"-"                      gorm:"column:dynamic_rule;type:text"`
}

func (AssetGroup) TableName() string { return "asset_groups" }

func (g *AssetGroup) BeforeCreate(tx *gorm.DB) error {
	if err := g.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return g.marshalShadows()
}

func (g *AssetGroup) BeforeUpdate(tx *gorm.DB) error {
	if err := g.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return g.marshalShadows()
}

func (g *AssetGroup) AfterFind(tx *gorm.DB) error {
	if err := g.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	_ = json.Unmarshal([]byte(g.DynamicRuleShadow), &g.DynamicRule)
	return nil
}

func (g *AssetGroup) marshalShadows() error {
	data, err := json.Marshal(g.DynamicRule)
	if err != nil {
		return err
	}
	g.DynamicRuleShadow = string(data)
	return nil
}

type AssetGroupMember struct {
	imachinery.ObjectMeta
	GroupID string `json:"group_id" gorm:"column:group_id;type:varchar(64);not null;index"`
	AssetID string `json:"asset_id" gorm:"column:asset_id;type:varchar(64);not null;index"`
	Role    string `json:"role"     gorm:"column:role;type:varchar(64)"`
}

func (AssetGroupMember) TableName() string { return "asset_group_members" }

type AssetRelation struct {
	imachinery.ObjectMeta
	SourceAssetID string         `json:"source_asset_id"  gorm:"column:source_asset_id;type:varchar(64);not null;index"`
	TargetAssetID string         `json:"target_asset_id"  gorm:"column:target_asset_id;type:varchar(64);not null;index"`
	TaskID        string         `json:"task_id"          gorm:"column:task_id;type:varchar(64);index"`
	RelationType  string         `json:"relation_type"    gorm:"column:relation_type;type:varchar(64);not null;index"`
	Params        map[string]any `json:"params,omitempty" gorm:"-"`
	ParamsShadow  string         `json:"-"                gorm:"column:params;type:text"`
}

func (AssetRelation) TableName() string { return "asset_relations" }

func (r *AssetRelation) BeforeCreate(tx *gorm.DB) error {
	if err := r.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return r.marshalShadows()
}

func (r *AssetRelation) BeforeUpdate(tx *gorm.DB) error {
	if err := r.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return r.marshalShadows()
}

func (r *AssetRelation) AfterFind(tx *gorm.DB) error {
	if err := r.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	_ = json.Unmarshal([]byte(r.ParamsShadow), &r.Params)
	return nil
}

func (r *AssetRelation) marshalShadows() error {
	data, err := json.Marshal(r.Params)
	if err != nil {
		return err
	}
	r.ParamsShadow = string(data)
	return nil
}

type Task struct {
	imachinery.ObjectMeta
	Type           string         `json:"type"                   gorm:"column:type;type:varchar(64);not null;index"`
	Status         string         `json:"status"                 gorm:"column:status;type:varchar(32);not null;default:pending;index"`
	Priority       int            `json:"priority"               gorm:"column:priority;not null;default:0;index"`
	Queue          string         `json:"queue"                  gorm:"column:queue;type:varchar(64);not null;default:default;index"`
	Input          map[string]any `json:"input,omitempty"        gorm:"-"`
	InputShadow    string         `json:"-"                      gorm:"column:input;type:text"`
	Output         map[string]any `json:"output,omitempty"       gorm:"-"`
	OutputShadow   string         `json:"-"                      gorm:"column:output;type:text"`
	Progress       int            `json:"progress"               gorm:"column:progress;not null;default:0"`
	Error          string         `json:"error"                  gorm:"column:error;type:text"`
	Attempts       int            `json:"attempts"               gorm:"column:attempts;not null;default:0"`
	MaxAttempts    int            `json:"max_attempts"           gorm:"column:max_attempts;not null;default:3"`
	LockOwner      string         `json:"lock_owner"             gorm:"column:lock_owner;type:varchar(128);index"`
	LockedUntil    time.Time      `json:"locked_until,omitempty" gorm:"column:locked_until;index"`
	IdempotencyKey string         `json:"idempotency_key"        gorm:"column:idempotency_key;type:varchar(128);index"`
}

func (Task) TableName() string { return "tasks" }

func (t *Task) BeforeCreate(tx *gorm.DB) error {
	if err := t.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return t.marshalShadows()
}

func (t *Task) BeforeUpdate(tx *gorm.DB) error {
	if err := t.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return t.marshalShadows()
}

func (t *Task) AfterFind(tx *gorm.DB) error {
	if err := t.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	_ = json.Unmarshal([]byte(t.InputShadow), &t.Input)
	_ = json.Unmarshal([]byte(t.OutputShadow), &t.Output)
	return nil
}

func (t *Task) marshalShadows() error {
	input, err := json.Marshal(t.Input)
	if err != nil {
		return err
	}
	output, err := json.Marshal(t.Output)
	if err != nil {
		return err
	}
	t.InputShadow = string(input)
	t.OutputShadow = string(output)
	return nil
}

type FeatureFlag struct {
	imachinery.ObjectMeta
	Key     string `json:"key"     gorm:"column:key;type:varchar(128);not null;uniqueIndex"`
	Enabled bool   `json:"enabled" gorm:"column:enabled;type:boolean;not null;default:true"`
}

func (FeatureFlag) TableName() string { return "feature_flags" }

type Role struct {
	imachinery.ObjectMeta
	System bool `json:"system" gorm:"column:system;type:boolean;not null;default:false"`
}

func (Role) TableName() string { return "roles" }

type Permission struct {
	imachinery.ObjectMeta
	Key string `json:"key" gorm:"column:key;type:varchar(128);not null;uniqueIndex"`
}

func (Permission) TableName() string { return "permissions" }

type UserRole struct {
	imachinery.ObjectMeta
	UserID string `json:"user_id" gorm:"column:user_id;type:varchar(64);not null;index"`
	RoleID string `json:"role_id" gorm:"column:role_id;type:varchar(64);not null;index"`
}

func (UserRole) TableName() string { return "user_roles" }
