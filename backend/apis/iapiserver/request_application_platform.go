package iapiserver

import (
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

type AppTemplateListRequest struct {
	imachinery.BasicQueryParam
	// OwnerUserID 由服务端按当前用户权限注入，客户端不能直接指定。
	OwnerUserID string `json:"-"    form:"-"`
	// Kind 按模板类型过滤，取值来自 application-platform S2。
	Kind string `json:"kind" form:"kind" binding:"omitempty,oneof=comfyui saas_api"`
	// IncludeAll 控制管理员或超级管理员是否查看全部用户资源。
	IncludeAll bool `json:"-"    form:"-"`
}

type AppTemplateCreateRequest struct {
	// Name 是模板名称，同一 owner_user_id 下必须唯一。
	Name string `json:"name"        binding:"required,max=100"`
	// Description 是模板描述，创建后仍可修改。
	Description string `json:"description" binding:"omitempty,max=500"`
	// Kind 是模板类型，当前仅允许 comfyui 或 saas_api。
	Kind string `json:"kind"        binding:"required,oneof=comfyui saas_api"`
	// Config 保存模板原始配置或请求配置，创建后不可修改。
	Config map[string]any `json:"config"      binding:"required"`
}

func (r *AppTemplateCreateRequest) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	r.Description = strings.TrimSpace(r.Description)
	if r.Name == "" {
		return errors.Errorf("template name is required")
	}
	if len([]rune(r.Name)) > 100 {
		return errors.Errorf("template name length must be <= 100")
	}
	if len([]rune(r.Description)) > 500 {
		return errors.Errorf("template description length must be <= 500")
	}
	return nil
}

type AppTemplateUpdateRequest struct {
	// ID 指定要更新的模板，由路径参数写入。
	ID string `json:"id"`
	// Name 更新模板名称，空指针表示不修改。
	Name *string `json:"name"        binding:"omitempty,max=100"`
	// Description 更新模板描述，空指针表示不修改。
	Description *string `json:"description" binding:"omitempty,max=500"`
}

func (r *AppTemplateUpdateRequest) Validate() error {
	if r.Name != nil {
		name := strings.TrimSpace(*r.Name)
		if name == "" {
			return errors.Errorf("template name is required")
		}
		if len([]rune(name)) > 100 {
			return errors.Errorf("template name length must be <= 100")
		}
		*r.Name = name
	}
	if r.Description != nil {
		desc := strings.TrimSpace(*r.Description)
		if len([]rune(desc)) > 500 {
			return errors.Errorf("template description length must be <= 500")
		}
		*r.Description = desc
	}
	return nil
}

type AppTemplateListResponse struct {
	// Total 返回当前查询条件下的模板总数。
	Total int64 `json:"total"`
	// Items 返回当前页模板，包含解析变量但不包含任何运行结果。
	Items []*AppTemplate `json:"items"`
}

type ApplicationListRequest struct {
	imachinery.BasicQueryParam
	// OwnerUserID 由服务端按当前用户权限注入，客户端不能直接指定。
	OwnerUserID string `json:"-"    form:"-"`
	// TemplateID 用于模板引用关系列表筛选。
	TemplateID string `json:"-"    form:"-"`
	// Kind 按模板类型过滤，取值来自 application-platform S2。
	Kind string `json:"kind" form:"kind" binding:"omitempty,oneof=comfyui saas_api"`
	// IncludeAll 控制管理员或超级管理员是否查看全部用户资源。
	IncludeAll bool `json:"-"    form:"-"`
}

type FieldMappingInput struct {
	// FieldKey 是同一应用内唯一的表单字段标识。
	FieldKey string `json:"field_key"   binding:"required"`
	// FieldLabel 是表单展示名称。
	FieldLabel string `json:"field_label" binding:"required"`
	// FieldType 必须来自模板解析变量中的 field_type。
	FieldType string `json:"field_type"  binding:"required"`
	// SourcePath 必须来自模板解析变量中的 source_path。
	SourcePath string `json:"source_path" binding:"required"`
	// DefaultValue 是表单默认值，可为任意 JSON 值。
	DefaultValue any `json:"default_value"`
	// SortOrder 是表单展示顺序。
	SortOrder int `json:"sort_order"`
}

type FieldMappingSaveRequest struct {
	// Items 是本次保存的完整字段映射列表，采用整体替换语义。
	Items []FieldMappingInput `json:"items" binding:"required,min=1"`
}

func (r *FieldMappingSaveRequest) Validate() error {
	if len(r.Items) == 0 {
		return errors.Errorf("field mappings are required")
	}
	for i := range r.Items {
		r.Items[i].FieldKey = strings.TrimSpace(r.Items[i].FieldKey)
		r.Items[i].FieldLabel = strings.TrimSpace(r.Items[i].FieldLabel)
		r.Items[i].FieldType = strings.TrimSpace(r.Items[i].FieldType)
		r.Items[i].SourcePath = strings.TrimSpace(r.Items[i].SourcePath)
		if r.Items[i].FieldKey == "" || r.Items[i].FieldLabel == "" ||
			r.Items[i].FieldType == "" || r.Items[i].SourcePath == "" {
			return errors.Errorf("field mapping fields are required")
		}
	}
	return nil
}

type ApplicationCreateRequest struct {
	// TemplateID 指定应用基于哪个模板创建。
	TemplateID string `json:"template_id"    binding:"required"`
	// Name 是应用名称。
	Name string `json:"name"           binding:"required,max=100"`
	// Description 是应用描述。
	Description string `json:"description"    binding:"omitempty,max=500"`
	// FieldMappings 是创建应用时一次性提交的完整字段映射。
	FieldMappings FieldMappingSaveRequest `json:"field_mappings" binding:"required"`
}

func (r *ApplicationCreateRequest) Validate() error {
	r.TemplateID = strings.TrimSpace(r.TemplateID)
	r.Name = strings.TrimSpace(r.Name)
	r.Description = strings.TrimSpace(r.Description)
	if r.TemplateID == "" {
		return errors.Errorf("template_id is required")
	}
	if r.Name == "" {
		return errors.Errorf("application name is required")
	}
	if len([]rune(r.Name)) > 100 {
		return errors.Errorf("application name length must be <= 100")
	}
	if len([]rune(r.Description)) > 500 {
		return errors.Errorf("application description length must be <= 500")
	}
	return r.FieldMappings.Validate()
}

type ApplicationUpdateRequest struct {
	// ID 指定要更新的应用，由路径参数写入。
	ID string `json:"id"`
	// Name 更新应用名称，空指针表示不修改。
	Name *string `json:"name"        binding:"omitempty,max=100"`
	// Description 更新应用描述，空指针表示不修改。
	Description *string `json:"description" binding:"omitempty,max=500"`
}

func (r *ApplicationUpdateRequest) Validate() error {
	if r.Name != nil {
		name := strings.TrimSpace(*r.Name)
		if name == "" {
			return errors.Errorf("application name is required")
		}
		if len([]rune(name)) > 100 {
			return errors.Errorf("application name length must be <= 100")
		}
		*r.Name = name
	}
	if r.Description != nil {
		desc := strings.TrimSpace(*r.Description)
		if len([]rune(desc)) > 500 {
			return errors.Errorf("application description length must be <= 500")
		}
		*r.Description = desc
	}
	return nil
}

type ApplicationListResponse struct {
	// Total 返回当前查询条件下的应用总数。
	Total int64 `json:"total"`
	// Items 返回当前页应用，应用详情场景可带字段映射。
	Items []*Application `json:"items"`
}

type FieldMappingListResponse struct {
	// Items 返回应用当前完整字段映射。
	Items []*FieldMapping `json:"items"`
}
