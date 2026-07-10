package iapiserver

import (
	"encoding/json"
	"strings"

	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	AppTemplateKindComfyUI = "comfyui"
	AppTemplateKindSaaSAPI = "saas_api"

	SaaSPlatformModelScope = "modelscope"
	SaaSPlatformCustomHTTP = "custom_http"

	CapabilityImageGeneration = "image_generation"
	CapabilityImageEditing    = "image_editing"
	CapabilityVideoGeneration = "video_generation"

	AppEngineTypeComfyUI = "comfyui"
	AppEngineTypeSaaSAPI = "saas_api"

	AppEngineAuthBearerToken = "bearer_token"
	AppEngineAuthAPIKey      = "api_key"
	AppEngineAuthAKSK        = "ak_sk"
	AppEngineAuthNone        = "none"

	AppEngineStatusActive   = "active"
	AppEngineStatusDisabled = "disabled"

	AppEngineHealthUnknown   = "unknown"
	AppEngineHealthHealthy   = "healthy"
	AppEngineHealthUnhealthy = "unhealthy"

	AppRunStatusPending  = "pending"
	AppRunStatusRunning  = "running"
	AppRunStatusSuccess  = "success"
	AppRunStatusFailed   = "failed"
	AppRunStatusCanceled = "canceled"
	AppRunStatusTimeout  = "timeout"

	AppTemplateChangedEvent        = "app_template_changed"
	ApplicationChangedEvent        = "application_changed"
	FieldMappingChangedEvent       = "field_mapping_changed"
	AppEngineChangedEvent          = "app_engine_changed"
	AppEngineHealthChangedEvent    = "app_engine_health_changed"
	ApplicationRunCreatedEvent     = "application_run_created"
	ApplicationRunStatusChangedEvt = "application_run_status_changed"
	AIAppTemplateManageOwn         = "aiapp.template.manage_own"
	AIAppApplicationManageOwn      = "aiapp.application.manage_own"
	AIAppAppEngineManageOwn        = "aiapp.app_engine.manage_own"
	AIAppApplicationRunOwn         = "aiapp.application.run_own"
	AIAppAdminManageAll            = "aiapp.admin.manage_all"
	AIAppSuperAdminManageAll       = "aiapp.super_admin.manage_all"
)

// OperationContract 保存 SaaS API 模板的第三方调用契约。
type OperationContract struct {
	// Method 是第三方接口调用方法或动作标识。
	Method string `json:"method,omitempty"`
	// Path 是第三方接口路径或资源标识。
	Path string `json:"path,omitempty"`
	// RequestMapping 描述应用输入和固化参数如何渲染为第三方请求。
	RequestMapping map[string]any `json:"request_mapping,omitempty"`
	// ResultMapping 描述第三方响应如何映射为运行输出摘要。
	ResultMapping map[string]any `json:"result_mapping,omitempty"`
	// CancelSupported 表示第三方操作是否支持取消。
	CancelSupported bool `json:"cancel_supported"`
	// ProgressSupported 表示第三方操作是否支持进度查询。
	ProgressSupported bool `json:"progress_supported"`
}

// HealthCheckConfig 保存 AppEngine 健康检测方式与小型请求参数。
type HealthCheckConfig struct {
	// Mode 指定检测模式：none、http_ping 或 api_call。
	Mode string `json:"mode,omitempty"`
	// Path 是相对 endpoint 的健康检测路径。
	Path string `json:"path,omitempty"`
	// Method 是健康检测 HTTP 方法。
	Method string `json:"method,omitempty"`
	// ExpectedStatus 是期望 HTTP 状态码；为空时 2xx/3xx 视为健康。
	ExpectedStatus int `json:"expected_status,omitempty"`
	// Payload 是 api_call 检测使用的小型结构化请求体。
	Payload map[string]any `json:"payload,omitempty"`
}

// ParsedField 是 application-platform S2 中从模板配置解析出的可映射变量。
type ParsedField struct {
	// SourcePath 是底层模板参数路径，字段映射必须引用该路径。
	SourcePath string `json:"source_path"`
	// FieldType 是由模板 JSON 叶子值推导出的字段类型。
	FieldType string `json:"field_type"`
	// Required 表示模板变量是否必填，第一阶段叶子节点解析默认 true。
	Required bool `json:"required"`
	// LabelHint 是展示层可用的名称提示，默认取路径末段。
	LabelHint string `json:"label_hint,omitempty"`
}

// AppTemplate 保存应用模板元数据、原始配置和创建时解析出的变量。
type AppTemplate struct {
	imachinery.ObjectMeta
	// OwnerUserID 标识模板归属用户；同一 owner 下模板名称必须唯一。
	OwnerUserID string `json:"owner_user_id" gorm:"column:owner_user_id;type:text;not null;index"`
	// Kind 表示模板类型，当前 S2 仅允许 comfyui 和 saas_api。
	Kind string `json:"kind"          gorm:"column:kind;type:text;not null;index"`
	// SaaSPlatformType 是 SaaS API 模板所属第三方平台；comfyui 模板为空。
	SaaSPlatformType string `json:"saas_platform_type,omitempty" gorm:"column:saas_platform_type;type:text;default:'';index"`
	// CapabilityType 是 SaaS API 模板提供的能力类型；comfyui 模板为空。
	CapabilityType string `json:"capability_type,omitempty" gorm:"column:capability_type;type:text;default:'';index"`
	// OperationKey 是 SaaS 平台内具体接口能力标识。
	OperationKey string `json:"operation_key,omitempty" gorm:"column:operation_key;type:text;default:''"`
	// OperationContract 保存 SaaS API 操作契约，对外按结构化对象返回。
	OperationContract OperationContract `json:"operation_contract,omitempty" gorm:"-"`
	// OperationContractShadow 是 OperationContract 的数据库 JSON 存储字段。
	OperationContractShadow string `json:"-" gorm:"column:operation_contract_json;type:text;not null;default:'{}'"`
	// Config 保存 ComfyUI raw JSON 或 SaaS API requestTemplate 配置，创建后不可修改。
	Config map[string]any `json:"config"        gorm:"-"`
	// ConfigShadow 是 Config 的数据库 JSON 存储字段。
	ConfigShadow string `json:"-"             gorm:"column:config_json;type:text;not null"`
	// ParsedFields 保存创建模板时解析出的可映射变量，创建后不可修改。
	ParsedFields []ParsedField `json:"parsed_fields" gorm:"-"`
	// ParsedFieldsShadow 是 ParsedFields 的数据库 JSON 存储字段。
	ParsedFieldsShadow string `json:"-"             gorm:"column:parsed_fields_json;type:text;not null;default:'[]'"`
	// ReferenceApplicationCount 记录引用该模板的应用数量，用于删除预检和列表展示。
	ReferenceApplicationCount int `json:"reference_application_count" gorm:"column:reference_application_count;type:integer;not null;default:0"`
}

func (AppTemplate) TableName() string { return "aiapp_app_templates" }

func (t *AppTemplate) BeforeCreate(tx *gorm.DB) error {
	if err := t.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return t.marshalShadows()
}

func (t *AppTemplate) AfterCreate(tx *gorm.DB) error { return nil }

func (t *AppTemplate) BeforeUpdate(tx *gorm.DB) error {
	if err := t.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return t.marshalShadows()
}

func (t *AppTemplate) AfterUpdate(tx *gorm.DB) error { return nil }

func (t *AppTemplate) AfterFind(tx *gorm.DB) error {
	if err := t.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	if t.ConfigShadow != "" {
		_ = json.Unmarshal([]byte(t.ConfigShadow), &t.Config)
	}
	if strings.TrimSpace(t.OperationContractShadow) != "" {
		_ = json.Unmarshal([]byte(t.OperationContractShadow), &t.OperationContract)
	}
	if t.ParsedFieldsShadow != "" {
		_ = json.Unmarshal([]byte(t.ParsedFieldsShadow), &t.ParsedFields)
	}
	return nil
}

func (t *AppTemplate) marshalShadows() error {
	if t.Config == nil {
		t.Config = map[string]any{}
	}
	operationContract, err := json.Marshal(t.OperationContract)
	if err != nil {
		return err
	}
	t.OperationContractShadow = string(operationContract)
	configData, err := json.Marshal(t.Config)
	if err != nil {
		return err
	}
	t.ConfigShadow = string(configData)
	if t.ParsedFields == nil {
		t.ParsedFields = []ParsedField{}
	}
	fieldsData, err := json.Marshal(t.ParsedFields)
	if err != nil {
		return err
	}
	t.ParsedFieldsShadow = string(fieldsData)
	return nil
}

// Application 保存基于模板创建的正式应用配置；应用不存在状态流转。
type Application struct {
	imachinery.ObjectMeta
	// OwnerUserID 标识应用归属用户；管理员修改他人应用时不得改变该字段。
	OwnerUserID string `json:"owner_user_id" gorm:"column:owner_user_id;type:text;not null;index"`
	// TemplateID 指向应用所基于的模板。
	TemplateID string `json:"template_id"   gorm:"column:template_id;type:text;not null;index"`
	// Kind 继承自模板类型，客户端创建应用时不能自定义。
	Kind string `json:"kind"          gorm:"column:kind;type:text;not null;index"`
	// SaaSPlatformType 继承自 SaaS API 模板；comfyui 应用为空。
	SaaSPlatformType string `json:"saas_platform_type,omitempty" gorm:"column:saas_platform_type;type:text;default:'';index"`
	// CapabilityType 继承自 SaaS API 模板；运行时用于匹配 AppEngine 能力。
	CapabilityType string `json:"capability_type,omitempty" gorm:"column:capability_type;type:text;default:'';index"`
	// OperationKey 继承自 SaaS API 模板，用于运行上下文。
	OperationKey string `json:"operation_key,omitempty" gorm:"column:operation_key;type:text;default:''"`
	// FixedParameters 保存应用固化参数，运行时与用户输入共同渲染 payload。
	FixedParameters map[string]any `json:"fixed_parameters,omitempty" gorm:"-"`
	// FixedParametersShadow 是 FixedParameters 的数据库 JSON 存储字段。
	FixedParametersShadow string `json:"-" gorm:"column:fixed_parameters_json;type:text;not null;default:'{}'"`
	// ReferenceRunCount 记录引用该应用的 AppRun 数量，用于物理删除保护。
	ReferenceRunCount int `json:"reference_run_count" gorm:"column:reference_run_count;type:integer;not null;default:0;index"`
	// FieldMappings 是应用详情聚合返回的字段映射，不直接落在应用表。
	FieldMappings []*FieldMapping `json:"field_mappings,omitempty" gorm:"-"`
}

func (Application) TableName() string { return "aiapp_applications" }

func (a *Application) BeforeCreate(tx *gorm.DB) error {
	if err := a.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return a.marshalShadows()
}
func (a *Application) AfterCreate(tx *gorm.DB) error { return nil }
func (a *Application) BeforeUpdate(tx *gorm.DB) error {
	if err := a.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return a.marshalShadows()
}
func (a *Application) AfterUpdate(tx *gorm.DB) error { return nil }

func (a *Application) AfterFind(tx *gorm.DB) error {
	if err := a.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	if strings.TrimSpace(a.FixedParametersShadow) != "" {
		_ = json.Unmarshal([]byte(a.FixedParametersShadow), &a.FixedParameters)
	}
	return nil
}

func (a *Application) marshalShadows() error {
	if a.FixedParameters == nil {
		a.FixedParameters = map[string]any{}
	}
	data, err := json.Marshal(a.FixedParameters)
	if err != nil {
		return err
	}
	a.FixedParametersShadow = string(data)
	return nil
}

// AppEngine 保存用户自维护应用引擎连接配置和健康状态。
type AppEngine struct {
	imachinery.ObjectMeta
	// OwnerUserID 标识应用引擎归属用户；管理员跨用户更新时不得改变。
	OwnerUserID string `json:"owner_user_id" gorm:"column:owner_user_id;type:text;not null;index"`
	// EngineType 表示引擎类型，当前 S2 仅允许 comfyui 和 saas_api。
	EngineType string `json:"engine_type"  gorm:"column:engine_type;type:text;not null;index"`
	// SaaSPlatformType 是 SaaS API 引擎所属第三方平台；comfyui 引擎为空。
	SaaSPlatformType string `json:"saas_platform_type,omitempty" gorm:"column:saas_platform_type;type:text;default:'';index"`
	// Endpoint 是应用引擎访问地址，健康检测会连接该地址或其平台健康路径。
	Endpoint string `json:"endpoint"     gorm:"column:endpoint;type:text;not null"`
	// AuthType 表示健康检测和后续调用使用的认证方式。
	AuthType string `json:"auth_type"    gorm:"column:auth_type;type:text;not null"`
	// AuthConfig 保存明文认证配置；按 S1/S2 要求可返回给有权用户。
	AuthConfig AppEngineAuthConfig `json:"auth_config" gorm:"-"`
	// AuthConfigShadow 是 AuthConfig 的数据库 JSON 存储字段。
	AuthConfigShadow string `json:"-"           gorm:"column:auth_config_json;type:text;not null;default:'{}'"`
	// Status 表示引擎是否启用；disabled 引擎不应被视为可用。
	Status string `json:"status"       gorm:"column:status;type:text;not null;default:'active';index"`
	// HealthStatus 表示最近健康检测结果。
	HealthStatus string `json:"health_status" gorm:"column:health_status;type:text;not null;default:'unknown';index"`
	// SupportedCapabilityTypes 保存 SaaS 引擎声明支持的能力类型。
	SupportedCapabilityTypes []string `json:"supported_capability_types,omitempty" gorm:"-"`
	// SupportedCapabilityTypesShadow 是 SupportedCapabilityTypes 的数据库 JSON 存储字段。
	SupportedCapabilityTypesShadow string `json:"-" gorm:"column:supported_capability_types_json;type:text;not null;default:'[]'"`
	// HealthCheckConfig 保存不同平台/能力的健康检测方式。
	HealthCheckConfig HealthCheckConfig `json:"health_check_config,omitempty" gorm:"-"`
	// HealthCheckConfigShadow 是 HealthCheckConfig 的数据库 JSON 存储字段。
	HealthCheckConfigShadow string `json:"-" gorm:"column:health_check_config_json;type:text;not null;default:'{}'"`
	// CapabilityTags 保存引擎能力标签，如 GPU、image、video 或 api_call。
	CapabilityTags []string `json:"capability_tags,omitempty" gorm:"-"`
	// CapabilityTagsShadow 是 CapabilityTags 的数据库 JSON 存储字段。
	CapabilityTagsShadow string `json:"-"                         gorm:"column:capability_tags_json;type:text;not null;default:'[]'"`
	// ReferenceRunCount 记录引用该引擎的 AppRun 数量，用于物理删除保护。
	ReferenceRunCount int `json:"reference_run_count" gorm:"column:reference_run_count;type:integer;not null;default:0"`
	// LastHealthCheckAt 是最近一次健康检查时间。
	LastHealthCheckAt *imachinery.Time `json:"last_health_check_at,omitempty" gorm:"column:last_health_check_at;type:timestamptz"`
	// UnhealthyReason 保存最近一次不健康原因。
	UnhealthyReason string `json:"unhealthy_reason,omitempty" gorm:"column:unhealthy_reason;type:text;default:''"`
}

type AppEngineAuthConfig struct {
	Token     string `json:"token,omitempty"`
	APIKey    string `json:"api_key,omitempty"`
	AccessKey string `json:"access_key,omitempty"`
	SecretKey string `json:"secret_key,omitempty"`
}

func (AppEngine) TableName() string { return "aiapp_app_engines" }

func (e *AppEngine) BeforeCreate(tx *gorm.DB) error {
	if err := e.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	applyAppEngineDefaults(e)
	return e.marshalShadows()
}

func (e *AppEngine) AfterCreate(tx *gorm.DB) error { return nil }

func (e *AppEngine) BeforeUpdate(tx *gorm.DB) error {
	if err := e.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	applyAppEngineDefaults(e)
	return e.marshalShadows()
}

func (e *AppEngine) AfterUpdate(tx *gorm.DB) error { return nil }

func (e *AppEngine) AfterFind(tx *gorm.DB) error {
	if err := e.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	if strings.TrimSpace(e.AuthConfigShadow) != "" {
		_ = json.Unmarshal([]byte(e.AuthConfigShadow), &e.AuthConfig)
	}
	if strings.TrimSpace(e.SupportedCapabilityTypesShadow) != "" {
		_ = json.Unmarshal([]byte(e.SupportedCapabilityTypesShadow), &e.SupportedCapabilityTypes)
	}
	if strings.TrimSpace(e.HealthCheckConfigShadow) != "" {
		_ = json.Unmarshal([]byte(e.HealthCheckConfigShadow), &e.HealthCheckConfig)
	}
	if strings.TrimSpace(e.CapabilityTagsShadow) != "" {
		_ = json.Unmarshal([]byte(e.CapabilityTagsShadow), &e.CapabilityTags)
	}
	return nil
}

func (e *AppEngine) marshalShadows() error {
	authConfig, err := json.Marshal(e.AuthConfig)
	if err != nil {
		return err
	}
	e.AuthConfigShadow = string(authConfig)
	if e.SupportedCapabilityTypes == nil {
		e.SupportedCapabilityTypes = []string{}
	}
	supportedCapabilityTypes, err := json.Marshal(e.SupportedCapabilityTypes)
	if err != nil {
		return err
	}
	e.SupportedCapabilityTypesShadow = string(supportedCapabilityTypes)
	healthCheckConfig, err := json.Marshal(e.HealthCheckConfig)
	if err != nil {
		return err
	}
	e.HealthCheckConfigShadow = string(healthCheckConfig)
	if e.CapabilityTags == nil {
		e.CapabilityTags = []string{}
	}
	capabilityTags, err := json.Marshal(e.CapabilityTags)
	if err != nil {
		return err
	}
	e.CapabilityTagsShadow = string(capabilityTags)
	return nil
}

func applyAppEngineDefaults(e *AppEngine) {
	if e.Status == "" {
		e.Status = AppEngineStatusActive
	}
	if e.HealthStatus == "" {
		e.HealthStatus = AppEngineHealthUnknown
	}
}

// AppEngineHealthCheckResult 表达一次 AppEngine 连接检测结果；临时检测不落库。
type AppEngineHealthCheckResult struct {
	// HealthStatus 是本次检测结果。
	HealthStatus string `json:"health_status"`
	// CheckedAt 是检测完成时间。
	CheckedAt imachinery.Time `json:"checked_at"`
	// UnhealthyReason 保存不健康原因。
	UnhealthyReason string `json:"unhealthy_reason,omitempty"`
	// LatencyMs 保存健康检测耗时毫秒。
	LatencyMs int64 `json:"latency_ms,omitempty"`
	// RawSummary 保存小型响应摘要，不保存大型响应或敏感凭证。
	RawSummary map[string]any `json:"raw_summary,omitempty"`
}

// ApplicationRun 保存一次 Application 运行快照及其 TaskRun 关联。
type ApplicationRun struct {
	imachinery.ObjectMeta
	// OwnerUserID 标识运行发起用户；管理员查看时不得改变归属。
	OwnerUserID string `json:"owner_user_id" gorm:"column:owner_user_id;type:text;not null;index"`
	// ApplicationID 指向被运行的 Application。
	ApplicationID string `json:"application_id" gorm:"column:application_id;type:text;not null;index"`
	// AppTemplateID 保存运行时 Application 引用的模板 ID 快照。
	AppTemplateID string `json:"app_template_id" gorm:"column:app_template_id;type:text;not null"`
	// AppEngineID 保存本次运行选择的 AppEngine。
	AppEngineID string `json:"app_engine_id" gorm:"column:app_engine_id;type:text;not null;index"`
	// TaskRunID 指向 task-center 中对应 TaskRun。
	TaskRunID string `json:"task_run_id" gorm:"column:task_run_id;type:text;not null;index"`
	// Kind 保存运行时应用类型快照。
	Kind string `json:"kind" gorm:"column:kind;type:text;not null;index"`
	// SaaSPlatformType 保存 SaaS 平台类型快照。
	SaaSPlatformType string `json:"saas_platform_type,omitempty" gorm:"column:saas_platform_type;type:text;default:''"`
	// CapabilityType 保存 SaaS 能力类型快照。
	CapabilityType string `json:"capability_type,omitempty" gorm:"column:capability_type;type:text;default:''"`
	// OperationKey 保存 SaaS 操作标识快照。
	OperationKey string `json:"operation_key,omitempty" gorm:"column:operation_key;type:text;default:''"`
	// InputSnapshot 保存用户提交的运行输入快照。
	InputSnapshot map[string]any `json:"input_snapshot" gorm:"-"`
	// InputSnapshotShadow 是 InputSnapshot 的数据库 JSON 存储字段。
	InputSnapshotShadow string `json:"-" gorm:"column:input_snapshot_json;type:text;not null;default:'{}'"`
	// RenderedPayloadSnapshot 保存固化参数和用户输入渲染后的运行参数快照。
	RenderedPayloadSnapshot map[string]any `json:"rendered_payload_snapshot" gorm:"-"`
	// RenderedPayloadSnapshotShadow 是 RenderedPayloadSnapshot 的数据库 JSON 存储字段。
	RenderedPayloadSnapshotShadow string `json:"-" gorm:"column:rendered_payload_snapshot_json;type:text;not null;default:'{}'"`
	// Status 保存 AppRun 用户可见状态。
	Status string `json:"status" gorm:"column:status;type:text;not null;index"`
	// OutputSummary 保存小型运行输出摘要或结果引用。
	OutputSummary map[string]any `json:"output_summary,omitempty" gorm:"-"`
	// OutputSummaryShadow 是 OutputSummary 的数据库 JSON 存储字段。
	OutputSummaryShadow string `json:"-" gorm:"column:output_summary_json;type:text;not null;default:'{}'"`
	// FailureReason 保存运行失败、取消或超时原因。
	FailureReason string `json:"failure_reason,omitempty" gorm:"column:failure_reason;type:text;default:''"`
}

func (ApplicationRun) TableName() string { return "aiapp_application_runs" }

func (r *ApplicationRun) BeforeCreate(tx *gorm.DB) error {
	if err := r.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	return r.marshalShadows()
}

func (r *ApplicationRun) AfterCreate(tx *gorm.DB) error { return nil }

func (r *ApplicationRun) BeforeUpdate(tx *gorm.DB) error {
	if err := r.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	return r.marshalShadows()
}

func (r *ApplicationRun) AfterUpdate(tx *gorm.DB) error { return nil }

func (r *ApplicationRun) AfterFind(tx *gorm.DB) error {
	if err := r.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	if strings.TrimSpace(r.InputSnapshotShadow) != "" {
		_ = json.Unmarshal([]byte(r.InputSnapshotShadow), &r.InputSnapshot)
	}
	if strings.TrimSpace(r.RenderedPayloadSnapshotShadow) != "" {
		_ = json.Unmarshal([]byte(r.RenderedPayloadSnapshotShadow), &r.RenderedPayloadSnapshot)
	}
	if strings.TrimSpace(r.OutputSummaryShadow) != "" {
		_ = json.Unmarshal([]byte(r.OutputSummaryShadow), &r.OutputSummary)
	}
	return nil
}

func (r *ApplicationRun) marshalShadows() error {
	if r.InputSnapshot == nil {
		r.InputSnapshot = map[string]any{}
	}
	input, err := json.Marshal(r.InputSnapshot)
	if err != nil {
		return err
	}
	r.InputSnapshotShadow = string(input)
	if r.RenderedPayloadSnapshot == nil {
		r.RenderedPayloadSnapshot = map[string]any{}
	}
	payload, err := json.Marshal(r.RenderedPayloadSnapshot)
	if err != nil {
		return err
	}
	r.RenderedPayloadSnapshotShadow = string(payload)
	if r.OutputSummary == nil {
		r.OutputSummary = map[string]any{}
	}
	output, err := json.Marshal(r.OutputSummary)
	if err != nil {
		return err
	}
	r.OutputSummaryShadow = string(output)
	return nil
}

// FieldMapping 保存应用表单字段到模板解析变量的映射关系。
type FieldMapping struct {
	imachinery.ObjectMeta
	// ApplicationID 指定字段映射所属应用，应用删除时同步清理。
	ApplicationID string `json:"application_id" gorm:"column:application_id;type:text;not null;uniqueIndex:idx_aiapp_field_mappings_app_key,priority:1;index"`
	// TemplateID 冗余保存所属模板，便于校验和引用查询。
	TemplateID string `json:"template_id"    gorm:"column:template_id;type:text;not null;index:idx_aiapp_field_mappings_template"`
	// FieldKey 是同一应用内唯一的表单字段标识。
	FieldKey string `json:"field_key"      gorm:"column:field_key;type:text;not null;uniqueIndex:idx_aiapp_field_mappings_app_key,priority:2"`
	// FieldLabel 是表单展示名称。
	FieldLabel string `json:"field_label"    gorm:"column:field_label;type:text;not null"`
	// FieldType 必须来自模板 ParsedField.field_type。
	FieldType string `json:"field_type"     gorm:"column:field_type;type:text;not null"`
	// SourcePath 必须来自模板 ParsedField.source_path。
	SourcePath string `json:"source_path"    gorm:"column:source_path;type:text;not null;index:idx_aiapp_field_mappings_source_path,priority:2"`
	// DefaultValue 保存字段默认值，对外按任意 JSON 值返回。
	DefaultValue any `json:"default_value,omitempty" gorm:"-"`
	// DefaultValueShadow 是 DefaultValue 的数据库 JSON 存储字段。
	DefaultValueShadow string `json:"-"                       gorm:"column:default_value_json;type:text;default:''"`
	// Required 来自模板解析变量，服务端保存时覆盖客户端输入。
	Required bool `json:"required"                gorm:"column:required;type:boolean;not null;default:false"`
	// SortOrder 是表单展示顺序。
	SortOrder int `json:"sort_order"              gorm:"column:sort_order;type:integer;not null;default:0"`
}

func (FieldMapping) TableName() string { return "aiapp_field_mappings" }

func (m *FieldMapping) BeforeCreate(tx *gorm.DB) error {
	if err := m.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	m.Name = m.FieldKey
	return m.marshalShadow()
}

func (m *FieldMapping) AfterCreate(tx *gorm.DB) error { return nil }

func (m *FieldMapping) BeforeUpdate(tx *gorm.DB) error {
	if err := m.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	m.Name = m.FieldKey
	return m.marshalShadow()
}

func (m *FieldMapping) AfterUpdate(tx *gorm.DB) error { return nil }

func (m *FieldMapping) AfterFind(tx *gorm.DB) error {
	if err := m.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	if strings.TrimSpace(m.DefaultValueShadow) != "" {
		_ = json.Unmarshal([]byte(m.DefaultValueShadow), &m.DefaultValue)
	}
	return nil
}

func (m *FieldMapping) marshalShadow() error {
	if m.DefaultValue == nil {
		m.DefaultValueShadow = ""
		return nil
	}
	data, err := json.Marshal(m.DefaultValue)
	if err != nil {
		return err
	}
	m.DefaultValueShadow = string(data)
	return nil
}
