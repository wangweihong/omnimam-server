package iapiserver

import (
	"encoding/json"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"gorm.io/gorm"
)

const (
	ProviderCapabilityAvailable            = "available"
	ProviderCapabilityDisabled             = "disabled"
	ProviderCapabilityKindCatalog          = "catalog"
	ProviderCapabilityKindEngineBinding    = "engine_binding"
	ProviderCapabilityOriginStatic         = "static"
	ProviderBindingPolicyManual            = "manual"
	ProviderBindingPolicyRequiredImmutable = "required_immutable"

	CapabilitySourceProviderCapability = "provider_capability"
	CapabilitySourceComfyUIWorkflow    = "comfyui_workflow"

	VersionStatusDraft     = "draft"
	VersionStatusPublished = "published"
	VersionStatusRetired   = "retired"

	ApplicationVisibilityPrivate = "private"
	ApplicationVisibilityGlobal  = "global"

	EngineHealthUnknown  = "unknown"
	EngineHealthOnline   = "online"
	EngineHealthOffline  = "offline"
	EngineHealthDegraded = "degraded"

	EngineAuthNone        = "none"
	EngineAuthAPIKey      = "api_key"
	EngineAuthBearerToken = "bearer_token"
	EngineAuthAKSK        = "ak_sk"

	BindingEffectiveAvailable   = "available"
	BindingEffectiveDisabled    = "disabled"
	BindingEffectiveUnavailable = "unavailable"

	TaskCreationPending = "pending"
	TaskCreationCreated = "created"
	TaskCreationFailed  = "failed"

	ArtifactRegistrationPending    = "pending"
	ArtifactRegistrationRegistered = "registered"
	ArtifactRegistrationFailed     = "failed"

	RuntimeInvalidReset    = "reset"
	RuntimeInvalidFallback = "fallback"
	RuntimeInvalidClamp    = "clamp"
	RuntimeInvalidReject   = "reject"

	AIAppProviderCapabilityRead  = "aiapp.provider_capability.read"
	AIAppEngineInstanceManage    = "aiapp.engine_instance.manage"
	AIAppEngineInstanceRead      = "aiapp.engine_instance.read"
	AIAppEngineBindingManage     = "aiapp.engine_binding.manage"
	AIAppComfyUIWorkflowRead     = "aiapp.comfyui_workflow.read"
	AIAppComfyUIWorkflowManage   = "aiapp.comfyui_workflow.manage"
	AIAppComfyUIWorkflowValidate = "aiapp.comfyui_workflow.validate"
	AIAppComfyUIWorkflowConvert  = "aiapp.comfyui_workflow.convert"
	AIAppComfyUIWorkflowTest     = "aiapp.comfyui_workflow.test"
	AIAppApplicationRead         = "aiapp.application.read"
	AIAppApplicationManage       = "aiapp.application.manage"
	AIAppApplicationRun          = "aiapp.application.run"
)

// CapabilityDefinition 描述平台内置的统一业务能力分类。
type CapabilityDefinition struct {
	ID               string            `json:"id" yaml:"id"`
	NameI18n         map[string]string `json:"name_i18n" yaml:"name_i18n"`
	InputMediaTypes  []string          `json:"input_media_types" yaml:"input_media_types"`
	OutputMediaTypes []string          `json:"output_media_types" yaml:"output_media_types"`
}

// EngineAdapterDefinition 描述只读 Runtime Registry 中的平台协议适配器。
type EngineAdapterDefinition struct {
	ID               string   `json:"id" yaml:"id"`
	Responsibilities []string `json:"responsibilities" yaml:"responsibilities"`
}

// OperationExecutorDefinition 描述能力与运行执行器的只读映射。
type OperationExecutorDefinition struct {
	ID                      string   `json:"id" yaml:"id"`
	EngineAdapterID         string   `json:"engine_adapter_id" yaml:"engine_adapter_id"`
	CapabilityDefinitionIDs []string `json:"capability_definition_ids" yaml:"capability_definition_ids"`
}

// ApplicationEngineType 是系统启动时注册的不可写引擎类型。
// +k8s:deepcopy-gen=true
type ApplicationEngineType struct {
	ID                         string                    `json:"id"`
	NameI18n                   map[string]string         `json:"name_i18n"`
	DescriptionI18n            map[string]string         `json:"description_i18n"`
	OfficialWebsiteURL         string                    `json:"official_website_url"`
	OfficialDocumentationURL   string                    `json:"official_documentation_url"`
	DefaultAPIBaseURL          string                    `json:"default_api_base_url"`
	Enabled                    bool                      `json:"enabled" yaml:"enabled"`
	EngineAdapterID            string                    `json:"engine_adapter_id" yaml:"engine_adapter_id"`
	AuthenticationTypes        []string                  `json:"authentication_types" yaml:"authentication_types"`
	AuthenticationConfigSchema map[string]map[string]any `json:"authentication_config_schema" yaml:"authentication_config_schema"`
	OperationExecutors         map[string]string         `json:"operation_executors" yaml:"operation_executors"`
	// CapabilityDefinitions 按 OperationExecutors key 字典序返回中英文能力名称。
	CapabilityDefinitions map[string][]string `json:"capability_definitions" yaml:"-"`
}

// +k8s:deepcopy-gen=true
type ProviderLifecycle struct {
	Status             string `json:"status" yaml:"status"`
	AvailableSince     string `json:"available_since,omitempty" yaml:"available_since,omitempty"`
	DeprecatedAt       string `json:"deprecated_at,omitempty" yaml:"deprecated_at,omitempty"`
	RetiredAt          string `json:"retired_at,omitempty" yaml:"retired_at,omitempty"`
	ReplacementModelID string `json:"replacement_model_id,omitempty" yaml:"replacement_model_id,omitempty"`
}

// +k8s:deepcopy-gen=true
type ProviderCapabilityModel struct {
	ID                  string            `json:"id" yaml:"id"`
	ProviderModelID     string            `json:"provider_model_id" yaml:"provider_model_id"`
	DisplayNameI18n     map[string]string `json:"display_name_i18n" yaml:"display_name_i18n"`
	DescriptionI18n     map[string]string `json:"description_i18n" yaml:"description_i18n"`
	Family              string            `json:"family" yaml:"family"`
	Variant             string            `json:"variant" yaml:"variant"`
	Lifecycle           ProviderLifecycle `json:"lifecycle" yaml:"lifecycle"`
	ContextWindowTokens int               `json:"context_window_tokens,omitempty" yaml:"context_window_tokens,omitempty"`
	MaximumOutputTokens int               `json:"maximum_output_tokens,omitempty" yaml:"maximum_output_tokens,omitempty"`
	Limits              map[string]any    `json:"limits,omitempty" yaml:"limits,omitempty"`
}

// +k8s:deepcopy-gen=true
type ProviderCapabilityOperation struct {
	ID                     string            `json:"id" yaml:"id"`
	CapabilityDefinitionID string            `json:"capability_definition_id" yaml:"capability_definition_id"`
	NameI18n               map[string]string `json:"name_i18n" yaml:"name_i18n"`
	DescriptionI18n        map[string]string `json:"description_i18n" yaml:"description_i18n"`
	ExecutionMode          string            `json:"execution_mode" yaml:"execution_mode"`
	InputMediaTypes        []string          `json:"input_media_types" yaml:"input_media_types"`
	OutputMediaTypes       []string          `json:"output_media_types" yaml:"output_media_types"`
	// InputSchema 和 OutputSchema 描述不依赖固定模型的稳定非流式协议基础约束。
	InputSchema           map[string]any `json:"input_schema,omitempty" yaml:"input_schema,omitempty"`
	OutputSchema          map[string]any `json:"output_schema,omitempty" yaml:"output_schema,omitempty"`
	UnsupportedParameters []string       `json:"unsupported_parameters,omitempty" yaml:"unsupported_parameters,omitempty"`
}

// +k8s:deepcopy-gen=true
type ProviderCapabilityVariant struct {
	ID                    string            `json:"id" yaml:"id"`
	ModelID               string            `json:"model_id" yaml:"model_id"`
	OperationID           string            `json:"operation_id" yaml:"operation_id"`
	Lifecycle             ProviderLifecycle `json:"lifecycle" yaml:"lifecycle"`
	InputSchema           map[string]any    `json:"input_schema" yaml:"input_schema"`
	OutputSchema          map[string]any    `json:"output_schema" yaml:"output_schema"`
	UnsupportedParameters []string          `json:"unsupported_parameters,omitempty" yaml:"unsupported_parameters,omitempty"`
	Notes                 []string          `json:"notes,omitempty" yaml:"notes,omitempty"`
}

// AIAppProviderCapability 是由具体协议适配器静态注册的只读能力清单，不持久化到数据库。
// Go 名称带领域前缀，避免与 model-management 的同名资源混淆。
// +k8s:deepcopy-gen=true
type AIAppProviderCapability struct {
	SchemaVersion   string            `json:"schema_version" yaml:"schema_version"`
	ID              string            `json:"id" yaml:"id"`
	NameI18n        map[string]string `json:"name_i18n" yaml:"name_i18n"`
	DescriptionI18n map[string]string `json:"description_i18n" yaml:"description_i18n"`
	// Kind 区分完整模型目录与仅用于标识引擎运行时身份的绑定能力。
	Kind string `json:"kind" yaml:"kind"`
	// Origin 固定为 static，表示能力随协议适配器编译交付。
	Origin string `json:"origin" yaml:"-"`
	// BindingPolicy 决定绑定由管理员维护，还是由系统强制维护且不可变。
	BindingPolicy           string                        `json:"binding_policy" yaml:"binding_policy"`
	ApplicationEngineTypeID string                        `json:"application_engine_type_id" yaml:"application_engine_type_id"`
	Revision                string                        `json:"revision" yaml:"revision"`
	Enabled                 bool                          `json:"enabled" yaml:"enabled"`
	Provider                map[string]any                `json:"provider" yaml:"provider"`
	Sources                 []map[string]any              `json:"sources" yaml:"sources"`
	Models                  []ProviderCapabilityModel     `json:"models" yaml:"models"`
	Operations              []ProviderCapabilityOperation `json:"operations" yaml:"operations"`
	Variants                []ProviderCapabilityVariant   `json:"variants" yaml:"variants"`
	Labels                  map[string]string             `json:"labels,omitempty" yaml:"labels,omitempty"`
	Notes                   []string                      `json:"notes,omitempty" yaml:"notes,omitempty"`
	Extensions              map[string]any                `json:"extensions,omitempty" yaml:"extensions,omitempty"`
	Availability            string                        `json:"availability" yaml:"-"`
}

type ProviderCapabilityListResponse struct {
	Total int                        `json:"total"`
	Items []*AIAppProviderCapability `json:"items"`
}

type ApplicationEngineTypeListResponse struct {
	Total int                      `json:"total"`
	Items []*ApplicationEngineType `json:"items"`
}

// EngineInstance 保存管理员维护的真实执行平台连接；列表接口必须清除 AuthConfig。
type EngineInstance struct {
	imachinery.ObjectMeta
	ApplicationEngineTypeID string           `json:"application_engine_type_id" gorm:"column:application_engine_type_id;type:text;not null;index"`
	BaseURL                 string           `json:"base_url" gorm:"column:base_url;type:text;not null"`
	AuthType                string           `json:"auth_type" gorm:"column:auth_type;type:text;not null"`
	AuthConfig              map[string]any   `json:"auth_config,omitempty" gorm:"-"`
	AuthConfigShadow        string           `json:"-" gorm:"column:auth_config_json;type:text;not null;default:'{}'"`
	Enabled                 bool             `json:"enabled" gorm:"column:enabled;not null;default:true;index"`
	HealthStatus            string           `json:"health_status" gorm:"column:health_status;type:text;not null;default:'unknown';index"`
	LastHealthCheckAt       *imachinery.Time `json:"last_health_check_at" gorm:"column:last_health_check_at;type:timestamptz"`
	UnhealthyReason         string           `json:"unhealthy_reason" gorm:"column:unhealthy_reason;type:text;default:''"`
	Region                  string           `json:"region" gorm:"column:region;type:text;default:''"`
	MaxConcurrency          int              `json:"max_concurrency" gorm:"column:max_concurrency;not null;default:1"`
	RequestTimeoutSeconds   int              `json:"request_timeout_seconds" gorm:"column:request_timeout_seconds;not null;default:60"`
	TaskTimeoutSeconds      int              `json:"task_timeout_seconds" gorm:"column:task_timeout_seconds;not null;default:1800"`
	// ObjectInfo* 是列表查询批量补充的当前目录摘要，不属于 EngineInstance 表。
	ObjectInfoAvailable   bool             `json:"-" gorm:"-"`
	ObjectInfoRefreshedAt *imachinery.Time `json:"-" gorm:"-"`
}

func (EngineInstance) TableName() string { return "aiapp_engine_instances" }
func (e *EngineInstance) BeforeCreate(tx *gorm.DB) error {
	if err := e.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return marshalShadow(e.AuthConfig, &e.AuthConfigShadow, "{}")
}
func (e *EngineInstance) AfterCreate(*gorm.DB) error { return nil }
func (e *EngineInstance) BeforeUpdate(tx *gorm.DB) error {
	if err := e.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return marshalShadow(e.AuthConfig, &e.AuthConfigShadow, "{}")
}
func (e *EngineInstance) AfterUpdate(*gorm.DB) error { return nil }
func (e *EngineInstance) AfterFind(*gorm.DB) error {
	unmarshalShadow(e.AuthConfigShadow, &e.AuthConfig)
	return nil
}

type EngineInstanceListResponse struct {
	Total int64                    `json:"total"`
	Items []*EngineInstanceSummary `json:"items"`
}
type EngineInstanceSummary struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	Description             string `json:"description,omitempty"`
	ApplicationEngineTypeID string `json:"application_engine_type_id"`
	// BaseURL 是列表展示和实例选择使用的执行端点；摘要不得包含 AuthConfig。
	BaseURL           string           `json:"base_url"`
	Enabled           bool             `json:"enabled"`
	HealthStatus      string           `json:"health_status"`
	LastHealthCheckAt *imachinery.Time `json:"last_health_check_at"`
	UnhealthyReason   string           `json:"unhealthy_reason"`
	// ObjectInfoAvailable 表示 ComfyUI 实例是否已有最后成功目录；非 ComfyUI 固定为 false。
	ObjectInfoAvailable bool `json:"object_info_available"`
	// ObjectInfoRefreshedAt 是当前目录最近成功刷新时间，不存在目录时为 null。
	ObjectInfoRefreshedAt *imachinery.Time `json:"object_info_refreshed_at"`
	// ObjectInfoStale 根据 48 小时阈值即时派生，不单独持久化状态。
	ObjectInfoStale bool   `json:"object_info_stale"`
	Region          string `json:"region,omitempty"`
}

func (e *EngineInstance) Summary() *EngineInstanceSummary {
	stale := false
	if e.ApplicationEngineTypeID == "comfyui" {
		stale = !e.ObjectInfoAvailable || e.ObjectInfoRefreshedAt == nil || time.Since(e.ObjectInfoRefreshedAt.Time) > ComfyUIObjectInfoMaxAge
	}
	return &EngineInstanceSummary{ID: e.ID, Name: e.Name, Description: e.Description, ApplicationEngineTypeID: e.ApplicationEngineTypeID, BaseURL: e.BaseURL, Enabled: e.Enabled, HealthStatus: e.HealthStatus, LastHealthCheckAt: e.LastHealthCheckAt, UnhealthyReason: e.UnhealthyReason, ObjectInfoAvailable: e.ObjectInfoAvailable, ObjectInfoRefreshedAt: e.ObjectInfoRefreshedAt, ObjectInfoStale: stale, Region: e.Region}
}

const ComfyUIObjectInfoMaxAge = 48 * time.Hour

// ComfyUIEngineObjectInfo 是 EngineInstance 的一对一当前事实扩展，因此不使用 ObjectMeta、版本或历史字段。
type ComfyUIEngineObjectInfo struct {
	// EngineInstanceID 同时作为主键和级联外键，保证每个实例至多一份目录。
	EngineInstanceID string `json:"engine_instance_id" gorm:"column:engine_instance_id;type:text;primaryKey"`
	// ObjectInfo 保存最近一次完整校验通过的原始 ComfyUI 节点目录。
	ObjectInfo       map[string]any `json:"object_info" gorm:"-"`
	ObjectInfoShadow string         `json:"-" gorm:"column:object_info_json;type:text;not null"`
	// ComfyUIVersion 保存刷新时从 system_stats 取得的可选版本；上游未提供时为空。
	ComfyUIVersion string `json:"comfyui_version" gorm:"column:comfyui_version;type:text;not null;default:''"`
	// RefreshedAt 是最近成功刷新完成时间，也是 stale 的唯一计算依据。
	RefreshedAt imachinery.Time `json:"refreshed_at" gorm:"column:refreshed_at;type:timestamptz;not null"`
}

func (ComfyUIEngineObjectInfo) TableName() string { return "aiapp_comfyui_engine_object_info" }
func (c *ComfyUIEngineObjectInfo) BeforeCreate(*gorm.DB) error {
	return marshalShadow(c.ObjectInfo, &c.ObjectInfoShadow, "{}")
}
func (*ComfyUIEngineObjectInfo) AfterCreate(*gorm.DB) error { return nil }
func (c *ComfyUIEngineObjectInfo) BeforeUpdate(*gorm.DB) error {
	return marshalShadow(c.ObjectInfo, &c.ObjectInfoShadow, "{}")
}
func (*ComfyUIEngineObjectInfo) AfterUpdate(*gorm.DB) error { return nil }
func (c *ComfyUIEngineObjectInfo) AfterFind(*gorm.DB) error {
	unmarshalShadow(c.ObjectInfoShadow, &c.ObjectInfo)
	return nil
}
func (c *ComfyUIEngineObjectInfo) Stale(now time.Time) bool {
	return now.Sub(c.RefreshedAt.Time) > ComfyUIObjectInfoMaxAge
}

type ComfyUIEngineObjectInfoStatus struct {
	// EngineInstanceID 标识本次刷新或状态查询对应的实例。
	EngineInstanceID string `json:"engine_instance_id"`
	// Available 表示当前是否存在至少一次成功刷新的目录。
	Available bool `json:"available"`
	// Stale 表示目录是否缺失或超过 48 小时；成功刷新固定为 false。
	Stale bool `json:"stale"`
	// ComfyUIVersion 和 RefreshedAt 在目录不存在或上游未提供版本时允许为空。
	ComfyUIVersion *string          `json:"comfyui_version"`
	RefreshedAt    *imachinery.Time `json:"refreshed_at"`
}

type ComfyUIEngineObjectInfoResponse struct {
	// EngineInstanceID 标识原始目录所属实例。
	EngineInstanceID string `json:"engine_instance_id"`
	// Available 对成功响应固定为 true；不存在目录通过业务错误表达。
	Available bool `json:"available"`
	// Stale 允许管理员和应用创建者识别仅可诊断、不可执行的旧目录。
	Stale bool `json:"stale"`
	// ComfyUIVersion 是可选上游版本，ObjectInfo 保持第三方原始字段名。
	ComfyUIVersion string          `json:"comfyui_version,omitempty"`
	RefreshedAt    imachinery.Time `json:"refreshed_at"`
	ObjectInfo     map[string]any  `json:"object_info"`
}

type EngineHealthCheckResult struct {
	EngineInstanceID string          `json:"engine_instance_id"`
	HealthStatus     string          `json:"health_status"`
	CheckedAt        imachinery.Time `json:"checked_at"`
	FailureSummary   string          `json:"failure_summary,omitempty"`
}

type EngineCapabilityBinding struct {
	imachinery.ObjectMeta
	EngineInstanceID           string         `json:"engine_instance_id" gorm:"column:engine_instance_id;type:text;not null;index"`
	ProviderCapabilityID       string         `json:"provider_capability_id" gorm:"column:provider_capability_id;type:text;not null;index"`
	ProviderCapabilityRevision string         `json:"provider_capability_revision" gorm:"column:provider_capability_revision;type:text;not null"`
	Enabled                    bool           `json:"enabled" gorm:"column:enabled;not null;default:true"`
	Restrictions               map[string]any `json:"restrictions" gorm:"-"`
	RestrictionsShadow         string         `json:"-" gorm:"column:restrictions_json;type:text;not null;default:'{}'"`
	EffectiveStatus            string         `json:"effective_status" gorm:"-"`
	// SystemManaged 表示绑定由 required_immutable 内置能力维护，不允许通过绑定 API 写入。
	SystemManaged bool `json:"system_managed" gorm:"-"`
}

func (EngineCapabilityBinding) TableName() string { return "aiapp_engine_capability_bindings" }
func (b *EngineCapabilityBinding) BeforeCreate(tx *gorm.DB) error {
	if err := b.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return marshalShadow(b.Restrictions, &b.RestrictionsShadow, "{}")
}
func (b *EngineCapabilityBinding) AfterCreate(*gorm.DB) error { return nil }
func (b *EngineCapabilityBinding) BeforeUpdate(tx *gorm.DB) error {
	if err := b.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return marshalShadow(b.Restrictions, &b.RestrictionsShadow, "{}")
}
func (b *EngineCapabilityBinding) AfterUpdate(*gorm.DB) error { return nil }
func (b *EngineCapabilityBinding) AfterFind(*gorm.DB) error {
	unmarshalShadow(b.RestrictionsShadow, &b.Restrictions)
	return nil
}

type EngineCapabilityBindingListResponse struct {
	Total int64                      `json:"total"`
	Items []*EngineCapabilityBinding `json:"items"`
}

type ApplicationTemplate struct {
	imachinery.ObjectMeta
	OwnerUserID            string  `json:"-" gorm:"column:owner_user_id;type:text;not null;index"`
	CapabilitySourceType   string  `json:"capability_source_type" gorm:"column:capability_source_type;type:text;not null;index"`
	CapabilityDefinitionID string  `json:"capability_definition_id" gorm:"column:capability_definition_id;type:text;not null;index"`
	CurrentVersionID       *string `json:"current_version_id" gorm:"column:current_version_id;type:text"`
	// ComfyUIConversionIdempotencyKey 只用于 ComfyUI 工作流转换重试，不通过 API 返回。
	ComfyUIConversionIdempotencyKey *string `json:"-" gorm:"column:comfyui_conversion_idempotency_key;type:text"`
}

func (ApplicationTemplate) TableName() string                 { return "aiapp_application_templates" }
func (t *ApplicationTemplate) BeforeCreate(tx *gorm.DB) error { return t.ObjectMeta.BeforeCreate(tx) }
func (*ApplicationTemplate) AfterCreate(*gorm.DB) error       { return nil }
func (t *ApplicationTemplate) BeforeUpdate(tx *gorm.DB) error { return t.ObjectMeta.BeforeUpdate(tx) }
func (*ApplicationTemplate) AfterUpdate(*gorm.DB) error       { return nil }

type ApplicationTemplateVersion struct {
	imachinery.ObjectMeta
	ApplicationTemplateID      string           `json:"application_template_id" gorm:"column:application_template_id;type:text;not null;index"`
	Version                    int              `json:"version" gorm:"column:version;not null"`
	Status                     string           `json:"status" gorm:"column:status;type:text;not null;index"`
	CapabilitySourceType       string           `json:"capability_source_type" gorm:"column:capability_source_type;type:text;not null"`
	SourceRevision             string           `json:"source_revision" gorm:"column:source_revision;type:text;not null"`
	ProviderCapabilityID       *string          `json:"provider_capability_id" gorm:"column:provider_capability_id;type:text"`
	ProviderCapabilityRevision *string          `json:"provider_capability_revision" gorm:"column:provider_capability_revision;type:text"`
	ProviderOperationID        *string          `json:"provider_operation_id" gorm:"column:provider_operation_id;type:text"`
	WorkflowContractRevision   *string          `json:"workflow_contract_revision" gorm:"column:workflow_contract_revision;type:text"`
	SourceComfyUIWorkflowID    *string          `json:"source_comfyui_workflow_id" gorm:"column:source_comfyui_workflow_id;type:text"`
	TemplateContract           map[string]any   `json:"template_contract" gorm:"-"`
	TemplateContractShadow     string           `json:"-" gorm:"column:template_contract_json;type:text;not null"`
	ComfyUIAPIWorkflow         map[string]any   `json:"comfyui_api_workflow" gorm:"-"`
	ComfyUIAPIWorkflowShadow   *string          `json:"-" gorm:"column:comfyui_api_workflow_json;type:text"`
	PublishedAt                *imachinery.Time `json:"published_at" gorm:"column:published_at;type:timestamptz"`
}

func (ApplicationTemplateVersion) TableName() string { return "aiapp_application_template_versions" }
func (v *ApplicationTemplateVersion) BeforeCreate(tx *gorm.DB) error {
	if err := v.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return v.marshal()
}
func (*ApplicationTemplateVersion) AfterCreate(*gorm.DB) error { return nil }
func (v *ApplicationTemplateVersion) BeforeUpdate(tx *gorm.DB) error {
	if err := v.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return v.marshal()
}
func (*ApplicationTemplateVersion) AfterUpdate(*gorm.DB) error { return nil }
func (v *ApplicationTemplateVersion) AfterFind(*gorm.DB) error {
	unmarshalShadow(v.TemplateContractShadow, &v.TemplateContract)
	unmarshalOptional(v.ComfyUIAPIWorkflowShadow, &v.ComfyUIAPIWorkflow)
	return nil
}
func (v *ApplicationTemplateVersion) marshal() error {
	if err := marshalShadow(v.TemplateContract, &v.TemplateContractShadow, "{}"); err != nil {
		return err
	}
	if err := marshalOptional(v.ComfyUIAPIWorkflow, &v.ComfyUIAPIWorkflowShadow); err != nil {
		return err
	}
	return nil
}

type Application struct {
	imachinery.ObjectMeta
	OwnerUserID            string  `json:"owner_user_id" gorm:"column:owner_user_id;type:text;not null;index"`
	CapabilityDefinitionID string  `json:"capability_definition_id" gorm:"column:capability_definition_id;type:text;not null;index"`
	Visibility             string  `json:"visibility" gorm:"column:visibility;type:text;not null;default:'private';index"`
	RunEnabled             bool    `json:"run_enabled" gorm:"column:run_enabled;not null;default:true"`
	CanvasEnabled          bool    `json:"canvas_enabled" gorm:"column:canvas_enabled;not null;default:true"`
	CopyEnabled            bool    `json:"copy_enabled" gorm:"column:copy_enabled;not null;default:false"`
	PresetEnabled          bool    `json:"preset_enabled" gorm:"column:preset_enabled;not null;default:false"`
	CurrentVersionID       *string `json:"current_version_id" gorm:"column:current_version_id;type:text"`
}

func (Application) TableName() string                 { return "aiapp_applications" }
func (a *Application) BeforeCreate(tx *gorm.DB) error { return a.ObjectMeta.BeforeCreate(tx) }
func (*Application) AfterCreate(*gorm.DB) error       { return nil }
func (a *Application) BeforeUpdate(tx *gorm.DB) error { return a.ObjectMeta.BeforeUpdate(tx) }
func (*Application) AfterUpdate(*gorm.DB) error       { return nil }

type ApplicationVersion struct {
	imachinery.ObjectMeta
	ApplicationID                string           `json:"application_id" gorm:"column:application_id;type:text;not null;index"`
	SemanticVersion              string           `json:"semantic_version" gorm:"column:semantic_version;type:text;not null"`
	Status                       string           `json:"status" gorm:"column:status;type:text;not null;index"`
	ApplicationTemplateVersionID string           `json:"application_template_version_id" gorm:"column:application_template_version_id;type:text;not null;index"`
	InputSchema                  map[string]any   `json:"input_schema" gorm:"-"`
	InputSchemaShadow            string           `json:"-" gorm:"column:input_schema_json;type:text;not null"`
	OutputSchema                 map[string]any   `json:"output_schema" gorm:"-"`
	OutputSchemaShadow           string           `json:"-" gorm:"column:output_schema_json;type:text;not null"`
	ParameterPolicies            map[string]any   `json:"parameter_policies" gorm:"-"`
	ParameterPoliciesShadow      string           `json:"-" gorm:"column:parameter_policies_json;type:text;not null"`
	PublishedAt                  *imachinery.Time `json:"published_at" gorm:"column:published_at;type:timestamptz"`
}

// ApplicationSummary 是 ApplicationRun 返回的一跳应用摘要，不递归展开 owner 或当前版本。
type ApplicationSummary struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Visibility string `json:"visibility"`
}

// ApplicationVersionSummary 是 ApplicationRun 创建时固定的不可变应用版本摘要。
type ApplicationVersionSummary struct {
	ID              string `json:"id"`
	SemanticVersion string `json:"semantic_version"`
	Status          string `json:"status"`
}

// ApplicationTemplateVersionSummary 是 ApplicationRun 创建时固定的模板来源摘要。
type ApplicationTemplateVersionSummary struct {
	ID                   string `json:"id"`
	Version              int    `json:"version"`
	Status               string `json:"status"`
	CapabilitySourceType string `json:"capability_source_type"`
	SourceRevision       string `json:"source_revision"`
}

// ProviderCapabilityRefSummary 是运行快照中的非敏感 ProviderCapability 与 Operation 摘要。
// +k8s:deepcopy-gen=true
type ProviderCapabilityRefSummary struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Revision      string  `json:"revision"`
	Availability  string  `json:"availability"`
	OperationID   *string `json:"operation_id,omitempty"`
	OperationName *string `json:"operation_name,omitempty"`
}

// EngineInstanceRefSummary 是 ApplicationRun 可见的非敏感 EngineInstance 摘要。
type EngineInstanceRefSummary struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	ApplicationEngineTypeID string `json:"application_engine_type_id"`
	Enabled                 bool   `json:"enabled"`
	HealthStatus            string `json:"health_status"`
}

// AtomicTaskRefSummary 复用 Task Center 的一跳任务摘要语义。
// +k8s:deepcopy-gen=true
type AtomicTaskRefSummary struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	NameI18n    map[string]string `json:"name_i18n,omitempty"`
	Status      string            `json:"status"`
	Progress    float64           `json:"progress"`
	FunctionRef string            `json:"function_ref,omitempty"`
}

func (ApplicationVersion) TableName() string { return "aiapp_application_versions" }
func (v *ApplicationVersion) BeforeCreate(tx *gorm.DB) error {
	if err := v.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return v.marshal()
}
func (*ApplicationVersion) AfterCreate(*gorm.DB) error { return nil }
func (v *ApplicationVersion) BeforeUpdate(tx *gorm.DB) error {
	if err := v.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return v.marshal()
}
func (*ApplicationVersion) AfterUpdate(*gorm.DB) error { return nil }
func (v *ApplicationVersion) AfterFind(*gorm.DB) error {
	unmarshalShadow(v.InputSchemaShadow, &v.InputSchema)
	unmarshalShadow(v.OutputSchemaShadow, &v.OutputSchema)
	unmarshalShadow(v.ParameterPoliciesShadow, &v.ParameterPolicies)
	return nil
}
func (v *ApplicationVersion) marshal() error {
	if err := marshalShadow(v.InputSchema, &v.InputSchemaShadow, "{}"); err != nil {
		return err
	}
	if err := marshalShadow(v.OutputSchema, &v.OutputSchemaShadow, "{}"); err != nil {
		return err
	}
	return marshalShadow(v.ParameterPolicies, &v.ParameterPoliciesShadow, "{}")
}

// +k8s:deepcopy-gen=true
type ApplicationRun struct {
	imachinery.ObjectMeta
	OwnerUserID                    string                    `json:"-" gorm:"column:owner_user_id;type:text;not null;index"`
	ApplicationID                  string                    `json:"application_id" gorm:"column:application_id;type:text;not null;index"`
	ApplicationVersionID           string                    `json:"application_version_id" gorm:"column:application_version_id;type:text;not null;index"`
	ApplicationTemplateVersionID   string                    `json:"application_template_version_id" gorm:"column:application_template_version_id;type:text;not null;index"`
	AtomicTaskID                   *string                   `json:"atomic_task_id" gorm:"column:atomic_task_id;type:text;uniqueIndex"`
	EngineInstanceID               string                    `json:"engine_instance_id" gorm:"column:engine_instance_id;type:text;not null;index"`
	CapabilitySourceType           string                    `json:"capability_source_type" gorm:"column:capability_source_type;type:text;not null"`
	SourceRevision                 string                    `json:"source_revision" gorm:"column:source_revision;type:text;not null"`
	ProviderCapabilityID           *string                   `json:"provider_capability_id" gorm:"column:provider_capability_id;type:text;index"`
	ProviderCapabilityRevision     *string                   `json:"provider_capability_revision" gorm:"column:provider_capability_revision;type:text"`
	ProviderOperationID            *string                   `json:"provider_operation_id" gorm:"column:provider_operation_id;type:text"`
	WorkflowContractRevision       *string                   `json:"workflow_contract_revision" gorm:"column:workflow_contract_revision;type:text"`
	CapabilitySourceSnapshot       map[string]any            `json:"-" gorm:"-"`
	CapabilitySourceSnapshotShadow string                    `json:"-" gorm:"column:capability_source_snapshot_json;type:text;not null"`
	InputSnapshot                  map[string]any            `json:"input_snapshot" gorm:"-"`
	InputSnapshotShadow            string                    `json:"-" gorm:"column:input_snapshot_json;type:text;not null"`
	ExecutionSnapshot              map[string]any            `json:"execution_snapshot" gorm:"-"`
	ExecutionSnapshotShadow        string                    `json:"-" gorm:"column:execution_snapshot_json;type:text;not null"`
	OutputMappingSnapshot          map[string]any            `json:"output_mapping_snapshot" gorm:"-"`
	OutputMappingSnapshotShadow    string                    `json:"-" gorm:"column:output_mapping_snapshot_json;type:text;not null"`
	TaskCreationStatus             string                    `json:"task_creation_status" gorm:"column:task_creation_status;type:text;not null;default:'pending'"`
	TaskCreationFailure            string                    `json:"task_creation_failure" gorm:"column:task_creation_failure;type:text;default:''"`
	TaskStatusProjection           *string                   `json:"task_status_projection" gorm:"column:task_status_projection;type:text"`
	TaskResourceVersion            int64                     `json:"task_resource_version" gorm:"column:task_resource_version;not null;default:0"`
	OutputValues                   []map[string]any          `json:"output_values" gorm:"-"`
	OutputValuesShadow             string                    `json:"-" gorm:"column:output_values_json;type:text;not null;default:'[]'"`
	FailureSummary                 string                    `json:"failure_summary" gorm:"column:failure_summary;type:text;default:''"`
	IdempotencyKey                 string                    `json:"-" gorm:"column:idempotency_key;type:text;not null;uniqueIndex:idx_aiapp_runs_owner_idempotency,priority:2"`
	Artifacts                      []*ApplicationArtifactRef `json:"artifacts" gorm:"-"`
	// Application 是权限裁剪后的应用摘要；关联缺失时为空但保留 ApplicationID。
	Application *ApplicationSummary `json:"application,omitempty" gorm:"-"`
	// ApplicationVersion 是运行固定的应用版本摘要。
	ApplicationVersion *ApplicationVersionSummary `json:"application_version,omitempty" gorm:"-"`
	// ApplicationTemplateVersion 是运行固定的模板版本摘要。
	ApplicationTemplateVersion *ApplicationTemplateVersionSummary `json:"application_template_version,omitempty" gorm:"-"`
	// ProviderCapability 是目录平台运行固定的能力与 Operation 摘要，ComfyUI 运行为空。
	ProviderCapability *ProviderCapabilityRefSummary `json:"provider_capability,omitempty" gorm:"-"`
	// EngineInstance 是运行选择的非敏感引擎摘要，不包含连接地址或凭证。
	EngineInstance *EngineInstanceRefSummary `json:"engine_instance,omitempty" gorm:"-"`
	// AtomicTask 是 Task Center 权限校验后返回的当前任务摘要。
	AtomicTask *AtomicTaskRefSummary `json:"atomic_task,omitempty" gorm:"-"`
}

func (ApplicationRun) TableName() string { return "aiapp_application_runs" }
func (r *ApplicationRun) BeforeCreate(tx *gorm.DB) error {
	if err := r.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return r.marshal()
}
func (*ApplicationRun) AfterCreate(*gorm.DB) error { return nil }
func (r *ApplicationRun) BeforeUpdate(tx *gorm.DB) error {
	if err := r.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return r.marshal()
}
func (*ApplicationRun) AfterUpdate(*gorm.DB) error { return nil }
func (r *ApplicationRun) AfterFind(*gorm.DB) error {
	unmarshalShadow(r.CapabilitySourceSnapshotShadow, &r.CapabilitySourceSnapshot)
	unmarshalShadow(r.InputSnapshotShadow, &r.InputSnapshot)
	unmarshalShadow(r.ExecutionSnapshotShadow, &r.ExecutionSnapshot)
	unmarshalShadow(r.OutputMappingSnapshotShadow, &r.OutputMappingSnapshot)
	unmarshalShadow(r.OutputValuesShadow, &r.OutputValues)
	return nil
}
func (r *ApplicationRun) marshal() error {
	for _, item := range []struct {
		value    any
		shadow   *string
		fallback string
	}{{r.CapabilitySourceSnapshot, &r.CapabilitySourceSnapshotShadow, "{}"}, {r.InputSnapshot, &r.InputSnapshotShadow, "{}"}, {r.ExecutionSnapshot, &r.ExecutionSnapshotShadow, "{}"}, {r.OutputMappingSnapshot, &r.OutputMappingSnapshotShadow, "{}"}, {r.OutputValues, &r.OutputValuesShadow, "[]"}} {
		if err := marshalShadow(item.value, item.shadow, item.fallback); err != nil {
			return err
		}
	}
	return nil
}

// ApplicationArtifact 是切换到 Asset Library Artifact 前的旧应用运行制品投影。
// Deprecated: 新写入必须使用 ApplicationArtifactRef；该模型仅用于历史数据回填。
// +k8s:deepcopy-gen=true
type ApplicationArtifact struct {
	imachinery.ObjectMeta
	OwnerUserID               string  `json:"owner_user_id" gorm:"column:owner_user_id;type:text;not null;index"`
	ApplicationRunID          string  `json:"application_run_id" gorm:"column:application_run_id;type:text;not null;uniqueIndex:idx_aiapp_artifacts_run_output,priority:1"`
	OutputKey                 string  `json:"output_key" gorm:"column:output_key;type:text;not null;uniqueIndex:idx_aiapp_artifacts_run_output,priority:2"`
	MediaType                 string  `json:"media_type" gorm:"column:media_type;type:text;not null"`
	ContentRef                string  `json:"content_ref" gorm:"column:content_ref;type:text;not null"`
	RegistrationStatus        string  `json:"registration_status" gorm:"column:registration_status;type:text;not null;default:'pending';index"`
	AssetID                   *string `json:"asset_id" gorm:"column:asset_id;type:text"`
	RegistrationErrorCode     string  `json:"registration_error_code" gorm:"column:registration_error_code;type:text;default:''"`
	RegistrationFailureDetail string  `json:"registration_failure_detail" gorm:"column:registration_failure_detail;type:text;default:''"`
}

func (ApplicationArtifact) TableName() string                 { return "aiapp_artifacts" }
func (a *ApplicationArtifact) BeforeCreate(tx *gorm.DB) error { return a.ObjectMeta.BeforeCreate(tx) }
func (*ApplicationArtifact) AfterCreate(*gorm.DB) error       { return nil }
func (a *ApplicationArtifact) BeforeUpdate(tx *gorm.DB) error { return a.ObjectMeta.BeforeUpdate(tx) }
func (*ApplicationArtifact) AfterUpdate(*gorm.DB) error       { return nil }

// ApplicationArtifactRef 是 ApplicationRun 到 Asset Library Artifact 的可重建只读引用投影。
// +k8s:deepcopy-gen=true
type ApplicationArtifactRef struct {
	imachinery.ObjectMeta
	// ApplicationRunID 标识拥有该输出声明的应用运行。
	ApplicationRunID string `json:"application_run_id" gorm:"column:application_run_id;type:text;not null;uniqueIndex:idx_aiapp_artifact_refs_run_output,priority:1;index"`
	// ArtifactID 指向 Asset Library 持有的 Artifact 事实。
	ArtifactID string `json:"artifact_id" gorm:"column:artifact_id;type:text;not null;uniqueIndex:idx_aiapp_artifact_refs_artifact"`
	// OutputKey 是运行输出端口的稳定名称。
	OutputKey string `json:"output_key" gorm:"column:output_key;type:text;not null;uniqueIndex:idx_aiapp_artifact_refs_run_output,priority:2"`
	// Sequence 区分同一输出端口的多个制品。
	Sequence int `json:"sequence" gorm:"column:sequence;not null;default:0;uniqueIndex:idx_aiapp_artifact_refs_run_output,priority:3"`
	// MediaType 是列表展示所需的受控媒体分类。
	MediaType string `json:"media_type" gorm:"column:media_type;type:text;not null"`
	// ArtifactProcessingStatus 是 Asset Library 处理状态的只读投影。
	ArtifactProcessingStatus string `json:"artifact_processing_status" gorm:"column:artifact_processing_status;type:text;not null;index"`
	// ArtifactRegistrationStatus 是 Asset Library 登记状态的只读投影。
	ArtifactRegistrationStatus string `json:"artifact_registration_status" gorm:"column:artifact_registration_status;type:text;not null;index"`
	// AssetID 在登记成功后提供素材导航目标。
	AssetID *string `json:"asset_id,omitempty" gorm:"column:asset_id;type:text"`
	// AssetVersionID 在登记成功后提供不可变版本导航目标。
	AssetVersionID *string `json:"asset_version_id,omitempty" gorm:"column:asset_version_id;type:text"`
	// ArtifactResourceVersion 用于丢弃重复或乱序的 Asset Library 事件。
	ArtifactResourceVersion int64 `json:"artifact_resource_version" gorm:"column:artifact_resource_version;not null;default:0"`
	// LastErrorCode 是处理或登记失败的稳定业务错误码。
	LastErrorCode *string `json:"last_error_code,omitempty" gorm:"column:last_error_code;type:text"`
}

func (ApplicationArtifactRef) TableName() string { return "aiapp_application_artifact_refs" }
func (r *ApplicationArtifactRef) BeforeCreate(tx *gorm.DB) error {
	return r.ObjectMeta.BeforeCreate(tx)
}
func (*ApplicationArtifactRef) AfterCreate(*gorm.DB) error { return nil }
func (r *ApplicationArtifactRef) BeforeUpdate(tx *gorm.DB) error {
	return r.ObjectMeta.BeforeUpdate(tx)
}
func (*ApplicationArtifactRef) AfterUpdate(*gorm.DB) error { return nil }

type RuntimeFormSchema struct {
	ApplicationVersionID        string                 `json:"application_version_id"`
	CapabilitySourceType        string                 `json:"capability_source_type"`
	SourceRevision              string                 `json:"source_revision"`
	ProviderCapabilityID        *string                `json:"provider_capability_id"`
	ProviderCapabilityRevision  *string                `json:"provider_capability_revision"`
	WorkflowContractRevision    *string                `json:"workflow_contract_revision"`
	CompatibleEngineInstanceIDs []string               `json:"compatible_engine_instance_ids"`
	Fields                      []RuntimeFormField     `json:"fields"`
	Changes                     []RuntimeFormChange    `json:"changes"`
	Violations                  []RuntimeFormViolation `json:"violations"`
	ResolvedAt                  imachinery.Time        `json:"resolved_at"`
}

type RuntimeFormField struct {
	Name              string         `json:"name"`
	Type              string         `json:"type"`
	Required          bool           `json:"required"`
	Value             any            `json:"value"`
	Options           []any          `json:"options"`
	Connectable       bool           `json:"connectable"`
	Dynamic           bool           `json:"dynamic"`
	DependsOn         []string       `json:"depends_on,omitempty"`
	OnInvalid         string         `json:"on_invalid,omitempty"`
	UnavailableReason *string        `json:"unavailable_reason,omitempty"`
	UI                map[string]any `json:"ui,omitempty"`
}
type RuntimeFormChange struct {
	Field         string `json:"field"`
	Reason        string `json:"reason"`
	Strategy      string `json:"strategy"`
	PreviousValue any    `json:"previous_value,omitempty"`
	CurrentValue  any    `json:"current_value,omitempty"`
}
type RuntimeFormViolation struct {
	Field        string `json:"field"`
	Code         string `json:"code"`
	Message      string `json:"message"`
	CurrentValue any    `json:"current_value,omitempty"`
}

type ApplicationTemplateListResponse struct {
	Total int64                  `json:"total"`
	Items []*ApplicationTemplate `json:"items"`
}
type ApplicationTemplateVersionListResponse struct {
	Total int64                         `json:"total"`
	Items []*ApplicationTemplateVersion `json:"items"`
}
type ApplicationListResponse struct {
	Total int64          `json:"total"`
	Items []*Application `json:"items"`
}
type ApplicationVersionListResponse struct {
	Total int64                 `json:"total"`
	Items []*ApplicationVersion `json:"items"`
}
type ApplicationRunListResponse struct {
	Total int64             `json:"total"`
	Items []*ApplicationRun `json:"items"`
}
type DeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// ApplicationPlatformEvent 是通过注入发布器交付的领域事件。
type ApplicationPlatformEvent struct {
	Type           string          `json:"type"`
	IdempotencyKey string          `json:"idempotency_key"`
	Payload        map[string]any  `json:"payload"`
	OccurredAt     imachinery.Time `json:"occurred_at"`
}

func marshalShadow(value any, target *string, fallback string) error {
	if value == nil {
		*target = fallback
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	*target = string(data)
	return nil
}
func unmarshalShadow(raw string, target any) {
	if raw == "" {
		return
	}
	_ = json.Unmarshal([]byte(raw), target)
}
func marshalOptional(value map[string]any, target **string) error {
	if value == nil {
		*target = nil
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	raw := string(data)
	*target = &raw
	return nil
}
func unmarshalOptional(raw *string, target any) {
	if raw != nil {
		_ = json.Unmarshal([]byte(*raw), target)
	}
}
