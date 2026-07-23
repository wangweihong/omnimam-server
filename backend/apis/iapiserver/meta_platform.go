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
	StorageBackendTypeCOS   = "cos"
	StorageBackendTypeAzure = "azure_blob"

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
)

type Provider struct {
	imachinery.ObjectMeta
	// OwnerUserID 标识 provider 所属用户，model-management S2 当前按用户隔离配置。
	OwnerUserID string `json:"owner_user_id"           gorm:"column:owner_user_id;type:text;not null;index"`
	// Type 表示 provider 协议类型，当前 S2 使用 openai-compatible。
	Type string `json:"provider_type"           gorm:"column:provider_type;type:text;not null;index"`
	// Enabled 控制该 provider 是否参与默认模型和模型选项选择。
	Enabled bool `json:"enabled"                 gorm:"column:enabled;type:boolean;not null;default:true"`
	// BaseURL 是 provider API 入口地址，创建和连接检测都会使用。
	BaseURL string `json:"api_base_url"            gorm:"column:api_base_url;type:text;not null"`
	// AuthType 表示鉴权方式，当前 S2 仅定义 API key 引用模式。
	AuthType string `json:"auth_type"               gorm:"column:auth_type;type:text;not null"`
	// CredentialRef 引用凭据存储中的密钥，不保存或返回明文。
	CredentialRef string `json:"api_key_ref,omitempty"   gorm:"column:api_key_ref;type:text;default:''"`
	// PresetKey 是导入 provider preset 时的临时参数，不属于 S2 持久化字段。
	PresetKey string `json:"-"                       gorm:"-"`
	// Config 保存 provider 的额外连接配置，对外以 extra_config 暴露。
	Config map[string]any `json:"extra_config,omitempty"  gorm:"-"`
	// ConfigShadow 是 Config 的 JSON 存储影子字段，业务层不直接读写。
	ConfigShadow string `json:"-"                       gorm:"column:extra_config_json;type:text;not null;default:'{}'"`
	// DeletedAt 用于草稿阶段软删除，避免破坏已有 provider/model/default 关联。
	DeletedAt string `json:"-"                       gorm:"column:deleted_at;type:text;default:'';index"`
}

func (Provider) TableName() string { return "user_model_providers" }

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
	// OwnerUserID 标识 provider model 所属用户，与 Provider 保持相同隔离边界。
	OwnerUserID string `json:"owner_user_id"            gorm:"column:owner_user_id;type:text;not null;index"`
	// ProviderID 指向 user_model_providers.id，用于聚合某个 provider 下的模型。
	ProviderID string `json:"provider_id"              gorm:"column:provider_id;type:text;not null;index"`
	// ProviderName 是列表响应中的只读展示字段，不落库。
	ProviderName string `json:"provider_name,omitempty"  gorm:"-"`
	// Model 是上游 provider 的真实模型标识，调用模型时使用该值。
	Model string `json:"model"                    gorm:"column:model;type:text;not null;index"`
	// DisplayName 是控制台展示名，允许用户用更友好的名称管理模型。
	DisplayName string `json:"display_name"             gorm:"column:display_name;type:text;not null"`
	// GroupName 用于模型选项分组展示，对外字段名按 S2 使用 group。
	GroupName string `json:"group,omitempty"          gorm:"column:model_group;type:text;default:'';index"`
	// Capabilities 声明模型支持的业务能力，如 llm.chat、query.parse。
	Capabilities []string `json:"capabilities"             gorm:"-"`
	// CapabilitiesShadow 是 Capabilities 的 JSON 存储影子字段。
	CapabilitiesShadow string `json:"-"                        gorm:"column:capabilities_json;type:text;not null;default:'[]'"`
	// StreamSupported 表示模型是否支持流式输出，供 ai-chat generation 选择策略使用。
	StreamSupported bool `json:"stream_supported"         gorm:"column:stream_supported;type:boolean;not null;default:true"`
	// HealthStatus 保存最近一次健康检测结果，用于过滤不可用模型。
	HealthStatus string `json:"health_status"            gorm:"column:health_status;type:text;not null;default:'unknown';index"`
	// HealthReason 保存健康检测失败原因，对外按 S2 暴露为 unhealthy_reason。
	HealthReason string `json:"unhealthy_reason,omitempty" gorm:"column:unhealthy_reason;type:text;default:''"`
	// HealthCheckedAt 是旧草稿健康检查的运行态临时字段，S2 响应不直接暴露。
	HealthCheckedAt *time.Time `json:"-"                      gorm:"-"`
	// Enabled 控制该模型是否出现在可选模型列表中。
	Enabled bool `json:"enabled"                  gorm:"column:enabled;type:boolean;not null;default:true"`
	// EndpointType 是 provider 同步时的内部分类结果，当前不进入 S2 表结构。
	EndpointType string `json:"-"                         gorm:"-"`
	// ModelTypes 是 provider 同步时推导的内部类型集合，当前不进入 S2 表结构。
	ModelTypes []string `json:"-"                         gorm:"-"`
	// ModelTypesShadow 保留旧草稿字段名，当前不落库。
	ModelTypesShadow string `json:"-"                         gorm:"-"`
	// DefaultParams 是旧草稿默认参数，当前 S2 未定义为 provider_model 字段。
	DefaultParams map[string]any `json:"-"                   gorm:"-"`
	// DefaultParamsShadow 保留旧草稿字段名，当前不落库。
	DefaultParamsShadow string `json:"-"                   gorm:"-"`
	// Pricing 是旧草稿价格信息，当前 S2 未定义为 provider_model 字段。
	Pricing map[string]any `json:"-"                   gorm:"-"`
	// PricingShadow 保留旧草稿字段名，当前不落库。
	PricingShadow string `json:"-"                   gorm:"-"`
	// DeletedAt 用于软删除 provider model，避免影响已引用的默认模型配置。
	DeletedAt string `json:"-"                   gorm:"column:deleted_at;type:text;default:'';index"`
}

func (ProviderModel) TableName() string { return "user_provider_models" }

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
	return nil
}

func (m *ProviderModel) marshalShadows() error {
	capabilities, err := json.Marshal(m.Capabilities)
	if err != nil {
		return err
	}
	m.CapabilitiesShadow = string(capabilities)
	return nil
}

type ProviderCapability struct {
	imachinery.ObjectMeta
}

func (ProviderCapability) TableName() string { return "provider_capabilities" }

type SystemLLMConfig struct {
	imachinery.ObjectMeta
	// OwnerUserID 标识默认模型配置所属用户，usage 在同一用户内唯一。
	OwnerUserID string `json:"owner_user_id"     gorm:"column:owner_user_id;type:text;not null;uniqueIndex:idx_user_default_model_configs_owner_usage,priority:1"`
	// Purpose 对应 S2 的 usage，表示 chat、translation 等业务用途。
	Purpose string `json:"usage"             gorm:"column:usage;type:text;not null;uniqueIndex:idx_user_default_model_configs_owner_usage,priority:2"`
	// ProviderID 指向默认模型所属 provider。
	ProviderID string `json:"provider_id"       gorm:"column:provider_id;type:text;not null;index"`
	// ModelID 指向默认模型配置实际选中的 provider model。
	ModelID string `json:"model_id"          gorm:"column:model_id;type:text;not null;index"`
	// Model 是旧草稿传入的 provider 模型名，S2 默认模型配置不直接暴露。
	Model string `json:"-"                 gorm:"-"`
	// Enabled 是旧草稿开关字段，S2 默认模型配置当前以记录存在表示启用。
	Enabled bool `json:"-"                 gorm:"-"`
	// ModelDetail 是详情响应中的只读模型对象，来自 provider model 聚合查询。
	ModelDetail *ProviderModel `json:"model,omitempty"   gorm:"-"`
}

func (SystemLLMConfig) TableName() string { return "user_default_model_configs" }

type StorageBackend struct {
	imachinery.ObjectMeta
	// Type 标识存储协议；第一阶段运行时只提供 local adapter，其余枚举为后续适配预留。
	Type string `json:"type" gorm:"column:type;type:varchar(64);not null;index"`
	// Root 保存管理员可见的物理根位置，只能通过 storage-inspection 管理员接口返回。
	Root string `json:"-" gorm:"column:root;type:varchar(1024);not null;default:''"`
	// Config 保存完整后端配置，可能包含凭证，不得进入普通素材、任务输出或跨域摘要。
	Config map[string]any `json:"-" gorm:"-"`
	// ConfigShadow 是 Config 的数据库 JSON 影子字段，业务代码不得直接修改。
	ConfigShadow string `json:"-"                gorm:"column:config;type:text;not null;default:'{}'"`
	// Enabled 表示该后端是否可被运行时选择。
	Enabled bool `json:"enabled" gorm:"column:enabled;type:boolean;not null;default:true"`
	// Readonly 表示后端只能读取，上传与派生写入不得选择该后端。
	Readonly bool `json:"readonly" gorm:"column:readonly;type:boolean;not null;default:false"`
	// Quota 是后端配置的字节配额；零表示未设置配额。
	Quota int64 `json:"quota" gorm:"column:quota;not null;default:0"`
}

// StorageBackendDetail 是仅管理员可见的完整物理存储配置投影。
// Root 与 Config 不做脱敏，因此不得嵌入普通素材、Representation、Artifact 或任务响应。
type StorageBackendDetail struct {
	// ID 是全局 StorageBackend 标识。
	ID string `json:"id"`
	// Name 是管理员维护的后端显示名称。
	Name string `json:"name"`
	// Description 是管理员维护的后端说明。
	Description string `json:"description"`
	// Extend 返回受控扩展字段；无扩展时返回空对象。
	Extend map[string]any `json:"extend"`
	// Type 标识存储协议。
	Type string `json:"type"`
	// Root 是完整物理根位置，仅管理员可见。
	Root string `json:"root"`
	// Config 是完整后端配置，可能包含凭证，仅管理员可见。
	Config map[string]any `json:"config"`
	// Enabled 表示运行时是否可选择该后端。
	Enabled bool `json:"enabled"`
	// Readonly 表示该后端是否禁止写入。
	Readonly bool `json:"readonly"`
	// Quota 是字节配额，零表示未设置。
	Quota int64 `json:"quota"`
	// ResourceVersion 用于配置更新审计与并发识别。
	ResourceVersion int64 `json:"resource_version"`
	// CreatedAt 是后端配置创建时间。
	CreatedAt imachinery.Time `json:"created_at"`
	// UpdatedAt 是后端配置最后更新时间。
	UpdatedAt imachinery.Time `json:"updated_at"`
}

func (StorageBackend) TableName() string { return "storage_backends" }

func (b *StorageBackend) BeforeCreate(tx *gorm.DB) error {
	if err := b.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return b.marshalShadows()
}

func (*StorageBackend) AfterCreate(*gorm.DB) error { return nil }

func (b *StorageBackend) BeforeUpdate(tx *gorm.DB) error {
	if err := b.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return b.marshalShadows()
}

func (*StorageBackend) AfterUpdate(*gorm.DB) error { return nil }

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
	StorageBackendID string         `json:"-"                  gorm:"column:storage_backend_id;type:varchar(64);not null;index"`
	ObjectKey        string         `json:"-"                  gorm:"column:object_key;type:varchar(1024);not null"`
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
	// AssetID 指向原始素材，缩略图任务完成后仍通过该字段回写归属。
	AssetID string `json:"asset_id"           gorm:"column:asset_id;type:varchar(64);not null;index"`
	// StorageBackendID 标识缩略图对象所在存储后端，通常沿用原素材后端。
	StorageBackendID string `json:"-"                  gorm:"column:storage_backend_id;type:varchar(64);not null;index"`
	// ObjectKey 是缩略图对象在存储后端中的 key，pending/unsupported 时允许为空。
	ObjectKey string `json:"-"                  gorm:"column:object_key;type:varchar(1024)"`
	// Width 保存生成后缩略图宽度，任务未完成时为零值。
	Width int `json:"width"              gorm:"column:width"`
	// Height 保存生成后缩略图高度，任务未完成时为零值。
	Height int `json:"height"             gorm:"column:height"`
	// MimeType 保存缩略图对象 MIME 类型，当前图片缩略图默认生成 PNG。
	MimeType string `json:"mime_type"          gorm:"column:mime_type;type:varchar(128)"`
	// Size 保存缩略图对象大小，便于列表响应避免访问对象存储。
	Size int64 `json:"size"               gorm:"column:size"`
	// Status 表示缩略图任务状态，pending/processing 由 AtomicTask 推进，unsupported/failed/ready 为可展示结果。
	Status string `json:"status"             gorm:"column:status;type:varchar(32);not null;default:pending;index"`
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
