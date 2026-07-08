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

	AppTemplateChangedEvent   = "app_template_changed"
	ApplicationChangedEvent   = "application_changed"
	FieldMappingChangedEvent  = "field_mapping_changed"
	AIAppTemplateManageOwn    = "aiapp.template.manage_own"
	AIAppApplicationManageOwn = "aiapp.application.manage_own"
	AIAppAdminManageAll       = "aiapp.admin.manage_all"
	AIAppSuperAdminManageAll  = "aiapp.super_admin.manage_all"
)

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
	OwnerUserID string `json:"owner_user_id" gorm:"column:owner_user_id;type:text;not null;uniqueIndex:idx_aiapp_app_templates_owner_name,priority:1;index"`
	// Kind 表示模板类型，当前 S2 仅允许 comfyui 和 saas_api。
	Kind string `json:"kind"          gorm:"column:kind;type:text;not null;index"`
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
	if t.ParsedFieldsShadow != "" {
		_ = json.Unmarshal([]byte(t.ParsedFieldsShadow), &t.ParsedFields)
	}
	return nil
}

func (t *AppTemplate) marshalShadows() error {
	if t.Config == nil {
		t.Config = map[string]any{}
	}
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
	// FieldMappings 是应用详情聚合返回的字段映射，不直接落在应用表。
	FieldMappings []*FieldMapping `json:"field_mappings,omitempty" gorm:"-"`
}

func (Application) TableName() string { return "aiapp_applications" }

func (a *Application) BeforeCreate(tx *gorm.DB) error { return a.ObjectMeta.BeforeCreate(tx) }
func (a *Application) AfterCreate(tx *gorm.DB) error  { return nil }
func (a *Application) BeforeUpdate(tx *gorm.DB) error { return a.ObjectMeta.BeforeUpdate(tx) }
func (a *Application) AfterUpdate(tx *gorm.DB) error  { return nil }

func (a *Application) AfterFind(tx *gorm.DB) error {
	return a.ObjectMeta.AfterFind(tx)
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
