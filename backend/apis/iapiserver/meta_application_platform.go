package iapiserver

import (
	"encoding/json"
	"strings"

	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	TemplateSourceKindComfyUIWorkflow  = "comfyui_workflow"
	TemplateSourceKindProviderWorkflow = "provider_workflow"

	ApplicationSourceTypeTemplate          = "template"
	ApplicationSourceTypeProviderOperation = "provider_operation"

	RunModeTest   = "test"
	RunModeNormal = "normal"

	SourceModeDirect   = "direct"
	SourceModeWorkflow = "workflow"

	ExecutionModeSynchronous  = "synchronous"
	ExecutionModeAsynchronous = "asynchronous"

	PortDirectionInput  = "input"
	PortDirectionOutput = "output"

	PortDataTypeText     = "text"
	PortDataTypeNumber   = "number"
	PortDataTypeBoolean  = "boolean"
	PortDataTypeEnum     = "enum"
	PortDataTypeJSON     = "json"
	PortDataTypeImage    = "image"
	PortDataTypeVideo    = "video"
	PortDataTypeAudio    = "audio"
	PortDataTypeFile     = "file"
	PortDataTypeModelRef = "model_ref"
	PortDataTypeArray    = "array"

	CardinalitySingle   = "single"
	CardinalityMultiple = "multiple"

	MaterializationInline    = "inline"
	MaterializationReference = "reference"
	MaterializationAsset     = "asset"

	AppEngineStatusActive   = "active"
	AppEngineStatusDisabled = "disabled"

	AppEngineHealthUnknown   = "unknown"
	AppEngineHealthHealthy   = "healthy"
	AppEngineHealthUnhealthy = "unhealthy"

	AppEngineAuthBearerToken = "bearer_token"
	AppEngineAuthAPIKey      = "api_key"
	AppEngineAuthAKSK        = "ak_sk"
	AppEngineAuthNone        = "none"

	LatestTestStatusUntested = "untested"
	LatestTestStatusRunning  = "running"
	LatestTestStatusPassed   = "passed"
	LatestTestStatusFailed   = "failed"

	AIAppAdapterRead          = "aiapp.adapter.read"
	AIAppTemplateManageOwn    = "aiapp.template.manage_own"
	AIAppApplicationManageOwn = "aiapp.application.manage_own"
	AIAppEngineManageOwn      = "aiapp.engine.manage_own"
	AIAppEngineUse            = "aiapp.engine.use"
	AIAppRunOperateOwn        = "aiapp.run.operate_own"
	AIAppAdminManageAll       = "aiapp.admin.manage_all"
	AIAppAppEngineManageOwn   = AIAppEngineManageOwn
	AIAppApplicationRunOwn    = AIAppRunOperateOwn
	AIAppSuperAdminManageAll  = AIAppAdminManageAll
)

const (
	ProviderAdapterKeyComfyUI        = "comfyui"
	ProviderAdapterKeyModelScope     = "modelscope"
	ProviderAdapterKeyRunningHub     = "runninghub"
	ProviderAdapterKeyByteDance      = "bytedance_seedance"
	ProviderAdapterKeyOpenAI         = "openai"
	ProviderOperationComfyUIRun      = "comfyui.workflow.run"
	ProviderOperationSeedanceText2V  = "seedance.text_to_video"
	ProviderOperationOpenAIImageGen  = "openai.image.generate"
	ProviderOperationOpenAIImageEdit = "openai.image.edit"
)

const (
	AppEngineTypeSaaSAPI      = "saas_api"
	SaaSPlatformModelScope    = "modelscope"
	SaaSPlatformCustomHTTP    = "custom_http"
	CapabilityImageGeneration = "image_generation"
	CapabilityImageEditing    = "image_editing"
	CapabilityVideoGeneration = "video_generation"
	AppTemplateKindComfyUI    = TemplateSourceKindComfyUIWorkflow
	AppTemplateKindSaaSAPI    = "saas_api"
)

type ParsedField struct {
	SourcePath string `json:"-"`
	FieldType  string `json:"-"`
	Required   bool   `json:"-"`
	LabelHint  string `json:"-"`
}

type FieldMappingInput struct {
	FieldKey     string `json:"-"`
	FieldLabel   string `json:"-"`
	FieldType    string `json:"-"`
	SourcePath   string `json:"-"`
	DefaultValue any    `json:"-"`
	SortOrder    int    `json:"-"`
}

type FieldMapping struct {
	imachinery.ObjectMeta
	ApplicationID string `json:"-" gorm:"-"`
	TemplateID    string `json:"-" gorm:"-"`
	FieldKey      string `json:"-" gorm:"-"`
	FieldLabel    string `json:"-" gorm:"-"`
	FieldType     string `json:"-" gorm:"-"`
	SourcePath    string `json:"-" gorm:"-"`
	DefaultValue  any    `json:"-" gorm:"-"`
	Required      bool   `json:"-" gorm:"-"`
	SortOrder     int    `json:"-" gorm:"-"`
}

// ProviderAdapter 是系统代码注册的只读平台适配器目录项。
type ProviderAdapter struct {
	// AdapterKey 是平台适配器稳定标识。
	AdapterKey string `json:"adapter_key"`
	// Name 是适配器展示名称。
	Name string `json:"name"`
	// PlatformType 是平台类别，用于列表过滤和展示。
	PlatformType string `json:"platform_type"`
	// Version 是适配器目录版本。
	Version string `json:"version"`
	// OperationKeys 列出该适配器提供的操作标识。
	OperationKeys []string `json:"operation_keys"`
	// Enabled 表示该适配器是否可被新模板、应用和引擎引用。
	Enabled bool `json:"enabled"`
}

// ProviderOperation 是系统代码注册的只读平台操作能力。
type ProviderOperation struct {
	// AdapterKey 指向所属 ProviderAdapter。
	AdapterKey string `json:"adapter_key"`
	// OperationKey 是平台操作稳定标识。
	OperationKey string `json:"operation_key"`
	// OperationVersion 是平台操作版本，参与应用和引擎匹配。
	OperationVersion string `json:"operation_version"`
	// Name 是操作展示名称。
	Name string `json:"name"`
	// CapabilityType 是操作能力类型，如 image_generation 或 video_generation。
	CapabilityType string `json:"capability_type"`
	// SourceMode 表示操作是 direct 还是 workflow 来源。
	SourceMode string `json:"source_mode"`
	// ExecutionMode 表示操作是同步还是异步执行。
	ExecutionMode string `json:"execution_mode"`
	// InputSchema 描述操作输入端口和参数 schema。
	InputSchema map[string]any `json:"input_schema"`
	// OutputSchema 描述操作输出端口和结果 schema。
	OutputSchema map[string]any `json:"output_schema"`
	// ProgressSupported 表示操作是否支持进度查询。
	ProgressSupported bool `json:"progress_supported"`
	// CancelSupported 表示操作是否支持取消。
	CancelSupported bool `json:"cancel_supported"`
	// IdempotencySupported 表示操作是否支持幂等提交或恢复。
	IdempotencySupported bool `json:"idempotency_supported"`
}

// PortDefinition 描述模板或操作能力图中的输入/输出端口。
type PortDefinition struct {
	// PortKey 是端口在节点内的稳定标识。
	PortKey string `json:"port_key"`
	// NodeKey 是端口所属节点标识。
	NodeKey string `json:"node_key"`
	// Direction 表示端口方向：input 或 output。
	Direction string `json:"direction"`
	// DataType 表示端口数据类型。
	DataType string `json:"data_type"`
	// Required 表示运行输入是否必填。
	Required bool `json:"required"`
	// Cardinality 表示端口接收单值或多值。
	Cardinality string `json:"cardinality"`
	// MediaType 保存媒体类端口的 MIME 类型提示。
	MediaType string `json:"media_type,omitempty"`
	// SourcePath 是底层模板或请求 payload 中的参数路径。
	SourcePath string `json:"source_path"`
	// SemanticRole 保存主图、参考图、缩略图等语义角色。
	SemanticRole string `json:"semantic_role,omitempty"`
	// Candidate 表示该端口是否仍需要用户确认。
	Candidate bool `json:"candidate"`
}

// CapabilityNode 是能力图节点，表达模板节点或 direct operation。
type CapabilityNode struct {
	// NodeKey 是能力图节点稳定标识。
	NodeKey string `json:"node_key"`
	// NodeType 是节点类型，如 ComfyUI class_type 或 provider operation。
	NodeType string `json:"node_type"`
	// Name 是节点展示名称。
	Name string `json:"name"`
	// InputPorts 保存该节点可映射输入端口。
	InputPorts []PortDefinition `json:"input_ports"`
	// OutputPorts 保存该节点可映射输出端口。
	OutputPorts []PortDefinition `json:"output_ports"`
	// ResolutionStatus 表示节点解析状态。
	ResolutionStatus string `json:"resolution_status"`
	// RawReference 保存小型原始引用信息，避免存大型原始内容。
	RawReference map[string]any `json:"raw_reference,omitempty"`
}

// CapabilityEdge 表达能力图中节点端口之间的数据依赖。
type CapabilityEdge struct {
	// SourceNodeKey 是来源节点标识。
	SourceNodeKey string `json:"source_node_key"`
	// SourcePortKey 是来源端口标识。
	SourcePortKey string `json:"source_port_key"`
	// TargetNodeKey 是目标节点标识。
	TargetNodeKey string `json:"target_node_key"`
	// TargetPortKey 是目标端口标识。
	TargetPortKey string `json:"target_port_key"`
}

// CapabilityGraph 保存模板解析后的能力图。
type CapabilityGraph struct {
	// Nodes 保存能力图节点。
	Nodes []CapabilityNode `json:"nodes"`
	// Edges 保存能力图依赖边。
	Edges []CapabilityEdge `json:"edges"`
	// GraphVersion 是解析器生成的能力图版本。
	GraphVersion string `json:"graph_version"`
	// UnresolvedNodeCount 记录未解析节点数量。
	UnresolvedNodeCount int `json:"unresolved_node_count"`
}

// AppTemplate 保存应用模板元数据和解析出的能力图。
type AppTemplate struct {
	imachinery.ObjectMeta
	// OwnerUserID 标识模板归属用户。
	OwnerUserID string `json:"owner_user_id" gorm:"column:owner_user_id;type:text;not null;index"`
	// SourceKind 表示模板来源类型：comfyui_workflow 或 provider_workflow。
	SourceKind string `json:"source_kind" gorm:"column:source_kind;type:text;not null;index"`
	// AdapterKey 保存模板依赖的平台适配器。
	AdapterKey string `json:"adapter_key" gorm:"column:adapter_key;type:text;not null;index"`
	// OperationKey 保存模板依赖的平台操作。
	OperationKey string `json:"operation_key" gorm:"column:operation_key;type:text;not null;index"`
	// OperationVersion 保存模板创建时选择的平台操作版本。
	OperationVersion string `json:"operation_version" gorm:"column:operation_version;type:text;not null;index"`
	// RawConfig 保存 ComfyUI 工作流或 provider workflow 原始小型配置。
	RawConfig map[string]any `json:"raw_config" gorm:"-"`
	// RawConfigShadow 是 RawConfig 的数据库 JSON 字符串。
	RawConfigShadow string `json:"-" gorm:"column:raw_config_json;type:text;not null;default:'{}'"`
	// CapabilityGraph 保存模板解析后的能力图。
	CapabilityGraph CapabilityGraph `json:"capability_graph" gorm:"-"`
	// CapabilityGraphShadow 是 CapabilityGraph 的数据库 JSON 字符串。
	CapabilityGraphShadow string `json:"-" gorm:"column:capability_graph_json;type:text;not null;default:'{}'"`
	// RequiredNodeTypes 保存运行该模板需要的节点类型集合。
	RequiredNodeTypes []string `json:"required_node_types" gorm:"-"`
	// RequiredNodeTypesShadow 是 RequiredNodeTypes 的数据库 JSON 字符串。
	RequiredNodeTypesShadow string `json:"-" gorm:"column:required_node_types_json;type:text;not null;default:'[]'"`
	// RequiredModelRefs 保存运行该模板需要的模型引用集合。
	RequiredModelRefs []string `json:"required_model_refs" gorm:"-"`
	// RequiredModelRefsShadow 是 RequiredModelRefs 的数据库 JSON 字符串。
	RequiredModelRefsShadow string `json:"-" gorm:"column:required_model_refs_json;type:text;not null;default:'[]'"`
	// ReferenceApplicationCount 记录引用该模板的应用数量。
	ReferenceApplicationCount int           `json:"reference_application_count" gorm:"column:reference_application_count;type:integer;not null;default:0"`
	Kind                      string        `json:"-" gorm:"-"`
	SaaSPlatformType          string        `json:"-" gorm:"-"`
	CapabilityType            string        `json:"-" gorm:"-"`
	ParsedFields              []ParsedField `json:"-" gorm:"-"`
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
	unmarshalJSON(t.RawConfigShadow, &t.RawConfig)
	unmarshalJSON(t.CapabilityGraphShadow, &t.CapabilityGraph)
	unmarshalJSON(t.RequiredNodeTypesShadow, &t.RequiredNodeTypes)
	unmarshalJSON(t.RequiredModelRefsShadow, &t.RequiredModelRefs)
	return nil
}
func (t *AppTemplate) marshalShadows() error {
	return marshalAppJSONShadows([]appJSONShadow{
		{value: &t.RawConfig, target: &t.RawConfigShadow, fallback: "{}"},
		{value: &t.CapabilityGraph, target: &t.CapabilityGraphShadow, fallback: "{}"},
		{value: &t.RequiredNodeTypes, target: &t.RequiredNodeTypesShadow, fallback: "[]"},
		{value: &t.RequiredModelRefs, target: &t.RequiredModelRefsShadow, fallback: "[]"},
	})
}

// InputMapping 保存应用对外输入字段到模板或 Operation 输入端口的映射。
type InputMapping struct {
	imachinery.ObjectMeta
	// ApplicationID 指定映射所属应用。
	ApplicationID string `json:"application_id" gorm:"column:application_id;type:text;not null;index"`
	// InputKey 是应用对外输入字段标识。
	InputKey string `json:"input_key" gorm:"column:input_key;type:text;not null"`
	// InputLabel 是输入字段展示名。
	InputLabel string `json:"input_label" gorm:"column:input_label;type:text;not null"`
	// SourcePortKey 指向能力图输入端口。
	SourcePortKey string `json:"source_port_key" gorm:"column:source_port_key;type:text;not null"`
	// SourcePath 指向底层 payload 参数路径。
	SourcePath string `json:"source_path" gorm:"column:source_path;type:text;not null"`
	// DataType 保存输入数据类型。
	DataType string `json:"data_type" gorm:"column:data_type;type:text;not null"`
	// Required 表示运行时该输入是否必填，来源于端口定义。
	Required bool `json:"required" gorm:"column:required;type:boolean;not null;default:false"`
	// DefaultValue 保存默认值。
	DefaultValue any `json:"default_value,omitempty" gorm:"-"`
	// DefaultValueShadow 是 DefaultValue 的数据库 JSON 字符串。
	DefaultValueShadow string `json:"-" gorm:"column:default_value_json;type:text;default:''"`
	// SortOrder 保存表单展示顺序。
	SortOrder int `json:"sort_order" gorm:"column:sort_order;type:integer;not null;default:0"`
}

func (InputMapping) TableName() string { return "aiapp_input_mappings" }
func (m *InputMapping) BeforeCreate(tx *gorm.DB) error {
	if err := m.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	m.Name = m.InputKey
	return m.marshalShadow()
}
func (m *InputMapping) AfterCreate(tx *gorm.DB) error { return nil }
func (m *InputMapping) BeforeUpdate(tx *gorm.DB) error {
	if err := m.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	m.Name = m.InputKey
	return m.marshalShadow()
}
func (m *InputMapping) AfterUpdate(tx *gorm.DB) error { return nil }
func (m *InputMapping) AfterFind(tx *gorm.DB) error {
	if err := m.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	if strings.TrimSpace(m.DefaultValueShadow) != "" {
		_ = json.Unmarshal([]byte(m.DefaultValueShadow), &m.DefaultValue)
	}
	return nil
}
func (m *InputMapping) marshalShadow() error {
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

// OutputMapping 保存应用输出字段到模板或 Operation 输出端口的映射。
type OutputMapping struct {
	imachinery.ObjectMeta
	// ApplicationID 指定映射所属应用。
	ApplicationID string `json:"application_id" gorm:"column:application_id;type:text;not null;index"`
	// OutputKey 是应用对外输出字段标识。
	OutputKey string `json:"output_key" gorm:"column:output_key;type:text;not null"`
	// OutputLabel 是输出字段展示名。
	OutputLabel string `json:"output_label" gorm:"column:output_label;type:text;not null"`
	// SourcePortKey 指向能力图输出端口。
	SourcePortKey string `json:"source_port_key" gorm:"column:source_port_key;type:text;not null"`
	// SourcePath 指向底层输出路径。
	SourcePath string `json:"source_path" gorm:"column:source_path;type:text;not null"`
	// DataType 保存输出数据类型。
	DataType string `json:"data_type" gorm:"column:data_type;type:text;not null"`
	// Cardinality 表示输出单值或多值。
	Cardinality string `json:"cardinality" gorm:"column:cardinality;type:text;not null"`
	// Primary 标识主输出，一个应用最多一个。
	Primary bool `json:"primary" gorm:"column:is_primary;type:boolean;not null;default:false"`
	// Materialization 表示输出保存方式。
	Materialization string `json:"materialization" gorm:"column:materialization;type:text;not null"`
	// SortOrder 保存展示顺序。
	SortOrder int `json:"sort_order" gorm:"column:sort_order;type:integer;not null;default:0"`
}

func (OutputMapping) TableName() string { return "aiapp_output_mappings" }
func (m *OutputMapping) BeforeCreate(tx *gorm.DB) error {
	if err := m.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	m.Name = m.OutputKey
	return nil
}
func (m *OutputMapping) AfterCreate(tx *gorm.DB) error { return nil }
func (m *OutputMapping) BeforeUpdate(tx *gorm.DB) error {
	if err := m.ObjectMeta.BeforeUpdate(tx); err != nil {
		return err
	}
	m.Name = m.OutputKey
	return nil
}
func (m *OutputMapping) AfterUpdate(tx *gorm.DB) error { return nil }

// Application 保存对用户开放的应用配置、映射和固化参数。
type Application struct {
	imachinery.ObjectMeta
	// OwnerUserID 标识应用归属用户。
	OwnerUserID string `json:"owner_user_id" gorm:"column:owner_user_id;type:text;not null;index"`
	// SourceType 表示应用来源：template 或 provider_operation。
	SourceType string `json:"source_type" gorm:"column:source_type;type:text;not null;index"`
	// TemplateID 保存模板来源应用的模板引用。
	TemplateID string `json:"template_id,omitempty" gorm:"column:template_id;type:text;default:'';index"`
	// AdapterKey 保存应用依赖的平台适配器。
	AdapterKey string `json:"adapter_key" gorm:"column:adapter_key;type:text;not null;index"`
	// OperationKey 保存应用依赖的平台操作。
	OperationKey string `json:"operation_key" gorm:"column:operation_key;type:text;not null;index"`
	// OperationVersion 保存应用依赖的平台操作版本。
	OperationVersion string `json:"operation_version" gorm:"column:operation_version;type:text;not null;index"`
	// CapabilityType 保存应用能力类型。
	CapabilityType string `json:"capability_type" gorm:"column:capability_type;type:text;not null"`
	// InputMappings 是应用详情聚合返回的输入映射。
	InputMappings []*InputMapping `json:"input_mappings,omitempty" gorm:"-"`
	// OutputMappings 是应用详情聚合返回的输出映射。
	OutputMappings []*OutputMapping `json:"output_mappings,omitempty" gorm:"-"`
	// FixedParameters 保存应用固化参数。
	FixedParameters map[string]any `json:"fixed_parameters" gorm:"-"`
	// FixedParametersShadow 是 FixedParameters 的数据库 JSON 字符串。
	FixedParametersShadow string `json:"-" gorm:"column:fixed_parameters_json;type:text;not null;default:'{}'"`
	// LatestTestStatus 保存最近一次测试运行状态。
	LatestTestStatus string `json:"latest_test_status" gorm:"column:latest_test_status;type:text;not null;default:'untested'"`
	// LastTestedAt 保存最近一次测试运行时间。
	LastTestedAt *imachinery.Time `json:"last_tested_at,omitempty" gorm:"column:last_tested_at;type:timestamptz"`
	// LatestTestFailureSummary 保存最近测试失败摘要。
	LatestTestFailureSummary string `json:"latest_test_failure_summary,omitempty" gorm:"column:latest_test_failure_summary;type:text;default:''"`
	// ReferenceRunCount 记录引用该应用的运行数量。
	ReferenceRunCount int             `json:"reference_run_count" gorm:"column:reference_run_count;type:integer;not null;default:0;index"`
	Kind              string          `json:"-" gorm:"-"`
	SaaSPlatformType  string          `json:"-" gorm:"-"`
	FieldMappings     []*FieldMapping `json:"-" gorm:"-"`
}

func (Application) TableName() string { return "aiapp_applications" }
func (a *Application) BeforeCreate(tx *gorm.DB) error {
	if err := a.ObjectMeta.BeforeCreate(tx); err != nil {
		return err
	}
	if a.LatestTestStatus == "" {
		a.LatestTestStatus = LatestTestStatusUntested
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
	unmarshalJSON(a.FixedParametersShadow, &a.FixedParameters)
	return nil
}
func (a *Application) marshalShadows() error {
	return marshalAppJSONShadows([]appJSONShadow{{value: &a.FixedParameters, target: &a.FixedParametersShadow, fallback: "{}"}})
}

// SupportedOperation 描述 AppEngine 支持的平台操作版本范围。
type SupportedOperation struct {
	// OperationKey 是支持的平台操作标识。
	OperationKey string `json:"operation_key"`
	// MinVersion 是支持的最小操作版本。
	MinVersion string `json:"min_version"`
	// MaxVersion 是支持的最大操作版本。
	MaxVersion string `json:"max_version"`
}

// AppEngineAuthConfig 保存 AppEngine 明文认证配置。
type AppEngineAuthConfig struct {
	// Token 是旧草稿 bearer token 字段，写入时会兼容映射到 BearerToken。
	Token string `json:"-" gorm:"-"`
	// BearerToken 保存 bearer token 明文。
	BearerToken string `json:"bearer_token,omitempty"`
	// APIKey 保存 API key 明文。
	APIKey string `json:"api_key,omitempty"`
	// AccessKey 保存 AK/SK 认证的 access key。
	AccessKey string `json:"access_key,omitempty"`
	// SecretKey 保存 AK/SK 认证的 secret key。
	SecretKey string `json:"secret_key,omitempty"`
}

type HealthCheckConfig struct {
	Path           string         `json:"-" gorm:"-"`
	Method         string         `json:"-" gorm:"-"`
	ExpectedStatus int            `json:"-" gorm:"-"`
	Payload        map[string]any `json:"-" gorm:"-"`
}

// AppEngine 保存用户自维护应用引擎连接配置和容量状态。
type AppEngine struct {
	imachinery.ObjectMeta
	// OwnerUserID 标识引擎归属用户。
	OwnerUserID string `json:"owner_user_id" gorm:"column:owner_user_id;type:text;not null;index"`
	// AdapterKey 保存引擎对应的平台适配器。
	AdapterKey string `json:"adapter_key" gorm:"column:adapter_key;type:text;not null;index"`
	// Endpoint 是应用引擎访问地址。
	Endpoint string `json:"endpoint" gorm:"column:endpoint;type:text;not null"`
	// AuthType 表示认证方式。
	AuthType string `json:"auth_type" gorm:"column:auth_type;type:text;not null"`
	// AuthConfig 保存明文认证配置。
	AuthConfig AppEngineAuthConfig `json:"auth_config" gorm:"-"`
	// AuthConfigShadow 是 AuthConfig 的数据库 JSON 字符串。
	AuthConfigShadow string `json:"-" gorm:"column:auth_config_json;type:text;not null;default:'{}'"`
	// Status 表示引擎启停状态。
	Status string `json:"status" gorm:"column:status;type:text;not null;default:'active';index"`
	// HealthStatus 表示最近健康检测状态。
	HealthStatus string `json:"health_status" gorm:"column:health_status;type:text;not null;default:'unknown';index"`
	// RuntimeVersion 保存引擎运行时版本。
	RuntimeVersion string `json:"runtime_version,omitempty" gorm:"column:runtime_version;type:text;default:''"`
	// SupportedOperations 保存引擎支持的操作版本范围。
	SupportedOperations []SupportedOperation `json:"supported_operations" gorm:"-"`
	// SupportedOperationsShadow 是 SupportedOperations 的数据库 JSON 字符串。
	SupportedOperationsShadow string `json:"-" gorm:"column:supported_operations_json;type:text;not null;default:'[]'"`
	// NodeTypes 保存 ComfyUI 引擎支持的节点类型。
	NodeTypes []string `json:"node_types,omitempty" gorm:"-"`
	// NodeTypesShadow 是 NodeTypes 的数据库 JSON 字符串。
	NodeTypesShadow string `json:"-" gorm:"column:node_types_json;type:text;not null;default:'[]'"`
	// ModelRefs 保存引擎可访问的模型引用。
	ModelRefs []string `json:"model_refs,omitempty" gorm:"-"`
	// ModelRefsShadow 是 ModelRefs 的数据库 JSON 字符串。
	ModelRefsShadow string `json:"-" gorm:"column:model_refs_json;type:text;not null;default:'[]'"`
	// Priority 保存自动路由排序优先级，数值越小越优先。
	Priority int `json:"priority" gorm:"column:priority;type:integer;not null;default:100"`
	// MaxConcurrency 保存引擎最大并发槽位。
	MaxConcurrency int `json:"max_concurrency" gorm:"column:max_concurrency;type:integer;not null;default:1"`
	// CurrentInflight 保存当前已占用并发槽位。
	CurrentInflight int `json:"current_inflight" gorm:"column:current_inflight;type:integer;not null;default:0"`
	// ReferenceRunCount 记录引用该引擎的运行数量，存在引用时禁止物理删除。
	ReferenceRunCount int `json:"reference_run_count" gorm:"column:reference_run_count;type:integer;not null;default:0;index"`
	// LastHealthCheckAt 是最近健康检查时间。
	LastHealthCheckAt *imachinery.Time `json:"last_health_check_at,omitempty" gorm:"column:last_health_check_at;type:timestamptz"`
	// UnhealthyReason 保存最近一次不健康原因。
	UnhealthyReason string `json:"unhealthy_reason,omitempty" gorm:"column:unhealthy_reason;type:text;default:''"`
	// HealthCheckConfig 是旧草稿健康检测配置，仅供兼容测试和本地检测，不持久化。
	HealthCheckConfig        HealthCheckConfig `json:"-" gorm:"-"`
	EngineType               string            `json:"-" gorm:"-"`
	SaaSPlatformType         string            `json:"-" gorm:"-"`
	SupportedCapabilityTypes []string          `json:"-" gorm:"-"`
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
	unmarshalJSON(e.AuthConfigShadow, &e.AuthConfig)
	unmarshalJSON(e.SupportedOperationsShadow, &e.SupportedOperations)
	unmarshalJSON(e.NodeTypesShadow, &e.NodeTypes)
	unmarshalJSON(e.ModelRefsShadow, &e.ModelRefs)
	return nil
}
func (e *AppEngine) marshalShadows() error {
	return marshalAppJSONShadows([]appJSONShadow{
		{value: &e.AuthConfig, target: &e.AuthConfigShadow, fallback: "{}"},
		{value: &e.SupportedOperations, target: &e.SupportedOperationsShadow, fallback: "[]"},
		{value: &e.NodeTypes, target: &e.NodeTypesShadow, fallback: "[]"},
		{value: &e.ModelRefs, target: &e.ModelRefsShadow, fallback: "[]"},
	})
}

func applyAppEngineDefaults(e *AppEngine) {
	if e.Status == "" {
		e.Status = AppEngineStatusActive
	}
	if e.HealthStatus == "" {
		e.HealthStatus = AppEngineHealthUnknown
	}
	if e.MaxConcurrency <= 0 {
		e.MaxConcurrency = 1
	}
	if e.Priority == 0 {
		e.Priority = 100
	}
}

// AvailableEngine 是运行应用时可选的引擎摘要，不包含 auth_config。
type AvailableEngine struct {
	// ID 是应用引擎 ID。
	ID string `json:"id"`
	// Name 是应用引擎名称。
	Name string `json:"name"`
	// AdapterKey 是引擎适配器。
	AdapterKey string `json:"adapter_key"`
	// SupportedOperations 返回引擎支持的操作版本范围。
	SupportedOperations []SupportedOperation `json:"supported_operations"`
	// HealthStatus 是当前健康状态。
	HealthStatus string `json:"health_status"`
	// RuntimeVersion 是运行时版本。
	RuntimeVersion string `json:"runtime_version,omitempty"`
	// MaxConcurrency 是最大并发槽位。
	MaxConcurrency int `json:"max_concurrency"`
	// CurrentInflight 是当前占用槽位。
	CurrentInflight int `json:"current_inflight"`
	// CapabilitySummary 保存小型能力摘要。
	CapabilitySummary map[string]any `json:"capability_summary,omitempty"`
}

// ApplicationOutputValue 保存应用运行标准化输出。
type ApplicationOutputValue struct {
	// OutputKey 是输出映射字段标识。
	OutputKey string `json:"output_key"`
	// DataType 是输出数据类型。
	DataType string `json:"data_type"`
	// InlineValue 保存小型内联结果。
	InlineValue any `json:"inline_value,omitempty"`
	// AssetID 保存素材库结果引用。
	AssetID string `json:"asset_id,omitempty"`
	// StorageURI 保存对象存储引用。
	StorageURI string `json:"storage_uri,omitempty"`
	// ExternalURL 保存外部结果 URL。
	ExternalURL string `json:"external_url,omitempty"`
	// MediaType 保存媒体类型。
	MediaType string `json:"media_type,omitempty"`
	// Metadata 保存小型输出元数据。
	Metadata map[string]any `json:"metadata"`
}

// ApplicationRun 保存一次应用运行与 TaskRun 投影。
type ApplicationRun struct {
	imachinery.ObjectMeta
	// OwnerUserID 标识运行发起用户。
	OwnerUserID string `json:"owner_user_id" gorm:"column:owner_user_id;type:text;not null;index"`
	// ApplicationID 指向被运行的应用。
	ApplicationID string `json:"application_id" gorm:"column:application_id;type:text;not null;index"`
	// TaskRunID 指向 task-center 中对应 TaskRun。
	TaskRunID string `json:"task_run_id" gorm:"column:task_run_id;type:text;not null;index"`
	// RunMode 区分 test 或 normal 运行。
	RunMode string `json:"run_mode" gorm:"column:run_mode;type:text;not null;index"`
	// RequestedEngineID 保存用户请求的引擎 ID，可为空。
	RequestedEngineID string `json:"requested_engine_id,omitempty" gorm:"column:requested_engine_id;type:text;default:''"`
	// ResolvedEngineID 保存实际占用的引擎 ID。
	ResolvedEngineID string `json:"resolved_engine_id,omitempty" gorm:"column:resolved_engine_id;type:text;default:'';index"`
	// AdapterKey 保存运行适配器快照。
	AdapterKey string `json:"adapter_key" gorm:"column:adapter_key;type:text;not null"`
	// OperationKey 保存运行操作快照。
	OperationKey string `json:"operation_key" gorm:"column:operation_key;type:text;not null"`
	// OperationVersion 保存运行操作版本快照。
	OperationVersion string `json:"operation_version" gorm:"column:operation_version;type:text;not null"`
	// InputSnapshot 保存用户输入快照。
	InputSnapshot map[string]any `json:"input_snapshot" gorm:"-"`
	// InputSnapshotShadow 是 InputSnapshot 的数据库 JSON 字符串。
	InputSnapshotShadow string `json:"-" gorm:"column:input_snapshot_json;type:text;not null;default:'{}'"`
	// RenderedPayloadSnapshot 保存渲染后的调用 payload。
	RenderedPayloadSnapshot map[string]any `json:"rendered_payload_snapshot" gorm:"-"`
	// RenderedPayloadSnapshotShadow 是 RenderedPayloadSnapshot 的数据库 JSON 字符串。
	RenderedPayloadSnapshotShadow string `json:"-" gorm:"column:rendered_payload_snapshot_json;type:text;not null;default:'{}'"`
	// OutputMappingSnapshot 保存运行时输出映射快照。
	OutputMappingSnapshot []OutputMapping `json:"output_mapping_snapshot" gorm:"-"`
	// OutputMappingSnapshotShadow 是 OutputMappingSnapshot 的数据库 JSON 字符串。
	OutputMappingSnapshotShadow string `json:"-" gorm:"column:output_mapping_snapshot_json;type:text;not null;default:'[]'"`
	// TaskStatusProjection 保存 TaskRun 状态投影。
	TaskStatusProjection string `json:"task_status_projection" gorm:"column:task_status_projection;type:text;not null;default:'PENDING'"`
	// TaskProgressProjection 保存 TaskRun 进度投影。
	TaskProgressProjection map[string]any `json:"task_progress_projection,omitempty" gorm:"-"`
	// TaskProgressProjectionShadow 是 TaskProgressProjection 的数据库 JSON 字符串。
	TaskProgressProjectionShadow string `json:"-" gorm:"column:task_progress_projection_json;type:text;not null;default:'{}'"`
	// TaskResourceVersion 保存已投影的 TaskRun resource_version。
	TaskResourceVersion int64 `json:"task_resource_version" gorm:"column:task_resource_version;type:integer;not null;default:0"`
	// OutputValues 保存标准化输出值。
	OutputValues []ApplicationOutputValue `json:"output_values" gorm:"-"`
	// OutputValuesShadow 是 OutputValues 的数据库 JSON 字符串。
	OutputValuesShadow string `json:"-" gorm:"column:output_values_json;type:text;not null;default:'[]'"`
	// FailureSummary 保存失败摘要。
	FailureSummary string `json:"failure_summary,omitempty" gorm:"column:failure_summary;type:text;default:''"`
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
	unmarshalJSON(r.InputSnapshotShadow, &r.InputSnapshot)
	unmarshalJSON(r.RenderedPayloadSnapshotShadow, &r.RenderedPayloadSnapshot)
	unmarshalJSON(r.OutputMappingSnapshotShadow, &r.OutputMappingSnapshot)
	unmarshalJSON(r.TaskProgressProjectionShadow, &r.TaskProgressProjection)
	unmarshalJSON(r.OutputValuesShadow, &r.OutputValues)
	return nil
}
func (r *ApplicationRun) marshalShadows() error {
	return marshalAppJSONShadows([]appJSONShadow{
		{value: &r.InputSnapshot, target: &r.InputSnapshotShadow, fallback: "{}"},
		{value: &r.RenderedPayloadSnapshot, target: &r.RenderedPayloadSnapshotShadow, fallback: "{}"},
		{value: &r.OutputMappingSnapshot, target: &r.OutputMappingSnapshotShadow, fallback: "[]"},
		{value: &r.TaskProgressProjection, target: &r.TaskProgressProjectionShadow, fallback: "{}"},
		{value: &r.OutputValues, target: &r.OutputValuesShadow, fallback: "[]"},
	})
}

type appJSONShadow struct {
	value    any
	target   *string
	fallback string
}

func marshalAppJSONShadows(shadows []appJSONShadow) error {
	for _, shadow := range shadows {
		data, err := json.Marshal(shadow.value)
		if err != nil {
			return err
		}
		if string(data) == "null" && shadow.fallback != "" {
			*shadow.target = shadow.fallback
			continue
		}
		*shadow.target = string(data)
	}
	return nil
}

func unmarshalJSON(raw string, target any) {
	if strings.TrimSpace(raw) == "" {
		return
	}
	_ = json.Unmarshal([]byte(raw), target)
}
