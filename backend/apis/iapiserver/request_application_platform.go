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
	// SaaSPlatformType 是 SaaS API 模板的第三方平台类型。
	SaaSPlatformType string `json:"saas_platform_type" binding:"omitempty,oneof=modelscope custom_http"`
	// CapabilityType 是 SaaS API 模板的能力类型。
	CapabilityType string `json:"capability_type" binding:"omitempty,oneof=image_generation image_editing video_generation"`
	// OperationKey 是 SaaS 平台内具体接口能力标识。
	OperationKey string `json:"operation_key"`
	// OperationContract 保存 SaaS API 第三方调用契约。
	OperationContract OperationContract `json:"operation_contract"`
	// Config 保存模板原始配置或请求配置，创建后不可修改。
	Config map[string]any `json:"config"      binding:"required"`
}

func (r *AppTemplateCreateRequest) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	r.Description = strings.TrimSpace(r.Description)
	r.SaaSPlatformType = strings.TrimSpace(r.SaaSPlatformType)
	r.CapabilityType = strings.TrimSpace(r.CapabilityType)
	r.OperationKey = strings.TrimSpace(r.OperationKey)
	if r.Name == "" {
		return errors.Errorf("template name is required")
	}
	if len([]rune(r.Name)) > 100 {
		return errors.Errorf("template name length must be <= 100")
	}
	if len([]rune(r.Description)) > 500 {
		return errors.Errorf("template description length must be <= 500")
	}
	if r.Kind == AppTemplateKindSaaSAPI &&
		(r.SaaSPlatformType == "" || r.CapabilityType == "" || r.OperationKey == "") {
		return errors.Errorf("saas platform type, capability type and operation key are required")
	}
	if r.Kind == AppTemplateKindComfyUI {
		r.SaaSPlatformType = ""
		r.CapabilityType = ""
		r.OperationKey = ""
		r.OperationContract = OperationContract{}
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

type AppEngineListRequest struct {
	imachinery.BasicQueryParam
	// OwnerUserID 由服务端按当前用户权限注入，客户端不能直接指定。
	OwnerUserID string `json:"-"             form:"-"`
	// EngineType 按引擎类型过滤，取值来自 application-platform S2。
	EngineType string `json:"engine_type"    form:"engine_type"    binding:"omitempty,oneof=comfyui saas_api"`
	// Status 按启用状态过滤。
	Status string `json:"status"         form:"status"         binding:"omitempty,oneof=active disabled"`
	// HealthStatus 按健康状态过滤。
	HealthStatus string `json:"health_status"  form:"health_status"  binding:"omitempty,oneof=unknown healthy unhealthy"`
	// IncludeAll 控制管理员或超级管理员是否查看全部用户资源。
	IncludeAll bool `json:"-"             form:"-"`
}

type AppEngineCreateRequest struct {
	// Name 是引擎名称，同一 owner_user_id 下必须唯一。
	Name string `json:"name"            binding:"required,max=100"`
	// Description 是引擎描述。
	Description string `json:"description"     binding:"omitempty,max=500"`
	// EngineType 表示引擎类型，当前仅允许 comfyui 或 saas_api。
	EngineType string `json:"engine_type"     binding:"required,oneof=comfyui saas_api"`
	// SaaSPlatformType 是 SaaS API 引擎的第三方平台类型。
	SaaSPlatformType string `json:"saas_platform_type" binding:"omitempty,oneof=modelscope custom_http"`
	// Endpoint 是引擎访问地址。
	Endpoint string `json:"endpoint"        binding:"required"`
	// AuthType 表示认证方式。
	AuthType string `json:"auth_type"       binding:"required,oneof=bearer_token api_key ak_sk none"`
	// AuthConfig 保存明文认证配置。
	AuthConfig AppEngineAuthConfig `json:"auth_config"`
	// SupportedCapabilityTypes 声明 SaaS API 引擎支持的能力类型。
	SupportedCapabilityTypes []string `json:"supported_capability_types"`
	// HealthCheckConfig 指定引擎健康检测方式和参数。
	HealthCheckConfig HealthCheckConfig `json:"health_check_config"`
	// CapabilityTags 保存引擎能力标签。
	CapabilityTags []string `json:"capability_tags"`
}

func (r *AppEngineCreateRequest) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	r.Description = strings.TrimSpace(r.Description)
	r.SaaSPlatformType = strings.TrimSpace(r.SaaSPlatformType)
	r.Endpoint = strings.TrimSpace(r.Endpoint)
	r.CapabilityTags = normalizeAppEngineTags(r.CapabilityTags)
	r.SupportedCapabilityTypes = normalizeCapabilityTypes(r.SupportedCapabilityTypes)
	if r.Name == "" {
		return errors.Errorf("app engine name is required")
	}
	if len([]rune(r.Name)) > 100 {
		return errors.Errorf("app engine name length must be <= 100")
	}
	if len([]rune(r.Description)) > 500 {
		return errors.Errorf("app engine description length must be <= 500")
	}
	if r.Endpoint == "" {
		return errors.Errorf("app engine endpoint is required")
	}
	if r.EngineType == AppEngineTypeSaaSAPI && r.SaaSPlatformType == "" {
		return errors.Errorf("saas platform type is required")
	}
	if r.EngineType == AppEngineTypeComfyUI {
		r.SaaSPlatformType = ""
		r.SupportedCapabilityTypes = nil
	}
	return nil
}

type AppEngineUpdateRequest struct {
	// ID 指定要更新的应用引擎，由路径参数写入。
	ID string `json:"id"`
	// Name 更新引擎名称，空指针表示不修改。
	Name *string `json:"name"        binding:"omitempty,max=100"`
	// Description 更新引擎描述，空指针表示不修改。
	Description *string `json:"description" binding:"omitempty,max=500"`
	// Endpoint 更新引擎访问地址，空指针表示不修改。
	Endpoint *string `json:"endpoint"`
	// SaaSPlatformType 更新 SaaS API 引擎第三方平台类型。
	SaaSPlatformType *string `json:"saas_platform_type" binding:"omitempty,oneof=modelscope custom_http"`
	// AuthType 更新认证方式，空指针表示不修改。
	AuthType *string `json:"auth_type"   binding:"omitempty,oneof=bearer_token api_key ak_sk none"`
	// AuthConfig 更新明文认证配置，空指针表示不修改。
	AuthConfig *AppEngineAuthConfig `json:"auth_config"`
	// Status 更新引擎状态，空指针表示不修改。
	Status *string `json:"status"      binding:"omitempty,oneof=active disabled"`
	// SupportedCapabilityTypes 更新 SaaS API 引擎支持的能力类型。
	SupportedCapabilityTypes *[]string `json:"supported_capability_types"`
	// HealthCheckConfig 更新健康检测配置。
	HealthCheckConfig *HealthCheckConfig `json:"health_check_config"`
	// CapabilityTags 更新能力标签，空指针表示不修改。
	CapabilityTags *[]string `json:"capability_tags"`
}

func (r *AppEngineUpdateRequest) Validate() error {
	if r.Name != nil {
		name := strings.TrimSpace(*r.Name)
		if name == "" {
			return errors.Errorf("app engine name is required")
		}
		if len([]rune(name)) > 100 {
			return errors.Errorf("app engine name length must be <= 100")
		}
		*r.Name = name
	}
	if r.Description != nil {
		desc := strings.TrimSpace(*r.Description)
		if len([]rune(desc)) > 500 {
			return errors.Errorf("app engine description length must be <= 500")
		}
		*r.Description = desc
	}
	if r.Endpoint != nil {
		endpoint := strings.TrimSpace(*r.Endpoint)
		if endpoint == "" {
			return errors.Errorf("app engine endpoint is required")
		}
		*r.Endpoint = endpoint
	}
	if r.SaaSPlatformType != nil {
		platformType := strings.TrimSpace(*r.SaaSPlatformType)
		*r.SaaSPlatformType = platformType
	}
	if r.AuthType != nil {
		authType := strings.TrimSpace(*r.AuthType)
		*r.AuthType = authType
	}
	if r.Status != nil {
		status := strings.TrimSpace(*r.Status)
		*r.Status = status
	}
	if r.SupportedCapabilityTypes != nil {
		capabilities := normalizeCapabilityTypes(*r.SupportedCapabilityTypes)
		r.SupportedCapabilityTypes = &capabilities
	}
	if r.CapabilityTags != nil {
		tags := normalizeAppEngineTags(*r.CapabilityTags)
		r.CapabilityTags = &tags
	}
	return nil
}

type AppEngineListResponse struct {
	// Total 返回当前查询条件下的 AppEngine 总数。
	Total int64 `json:"total"`
	// Items 返回当前页 AppEngine，包含明文 auth_config。
	Items []*AppEngine `json:"items"`
}

type AppEngineHealthCheckRequest struct {
	// EngineType 指定本次临时检测的引擎类型。
	EngineType string `json:"engine_type" binding:"required,oneof=comfyui saas_api"`
	// SaaSPlatformType 指定 SaaS API 平台类型，用于选择检测方式。
	SaaSPlatformType string `json:"saas_platform_type" binding:"omitempty,oneof=modelscope custom_http"`
	// Endpoint 是本次临时检测连接地址。
	Endpoint string `json:"endpoint" binding:"required"`
	// AuthType 指定本次临时检测的认证方式。
	AuthType string `json:"auth_type" binding:"required,oneof=bearer_token api_key ak_sk none"`
	// AuthConfig 保存本次检测携带的明文认证配置，不落库。
	AuthConfig AppEngineAuthConfig `json:"auth_config"`
	// HealthCheckConfig 指定本次临时检测方式和参数，不落库。
	HealthCheckConfig HealthCheckConfig `json:"health_check_config"`
}

func (r *AppEngineHealthCheckRequest) Validate() error {
	r.SaaSPlatformType = strings.TrimSpace(r.SaaSPlatformType)
	r.Endpoint = strings.TrimSpace(r.Endpoint)
	if r.Endpoint == "" {
		return errors.Errorf("app engine endpoint is required")
	}
	if r.EngineType == AppEngineTypeSaaSAPI && r.SaaSPlatformType == "" {
		return errors.Errorf("saas platform type is required")
	}
	if r.EngineType == AppEngineTypeComfyUI {
		r.SaaSPlatformType = ""
	}
	return nil
}

func normalizeAppEngineTags(tags []string) []string {
	ret := make([]string, 0, len(tags))
	seen := map[string]struct{}{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		ret = append(ret, tag)
	}
	return ret
}

func normalizeCapabilityTypes(items []string) []string {
	ret := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		ret = append(ret, item)
	}
	return ret
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
	// FixedParameters 保存应用从模板参数中固化下来的固定参数。
	FixedParameters map[string]any `json:"fixed_parameters"`
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
	// FixedParameters 更新应用固化参数，空指针表示不修改。
	FixedParameters *map[string]any `json:"fixed_parameters"`
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

type ApplicationRunCreateRequest struct {
	// AppEngineID 指定本次运行选择的 AppEngine，只绑定到本次 AppRun。
	AppEngineID string `json:"app_engine_id" binding:"required"`
	// Input 保存用户提交的运行输入快照。
	Input map[string]any `json:"input" binding:"required"`
	// ScheduleAt 指定最早调度时间，空值表示立即进入 task-center。
	ScheduleAt imachinery.Time `json:"schedule_at"`
}

func (r *ApplicationRunCreateRequest) Validate() error {
	r.AppEngineID = strings.TrimSpace(r.AppEngineID)
	if r.AppEngineID == "" {
		return errors.Errorf("app_engine_id is required")
	}
	if r.Input == nil {
		return errors.Errorf("input is required")
	}
	return nil
}

type ApplicationRunListRequest struct {
	imachinery.BasicQueryParam
	// OwnerUserID 由服务端按当前用户权限注入，客户端不能直接指定。
	OwnerUserID string `json:"-" form:"-"`
	// IncludeAll 控制管理员或超级管理员是否查看全部用户运行记录。
	IncludeAll bool `json:"-" form:"-"`
	// Status 按 AppRun 用户可见状态过滤。
	Status string `json:"status" form:"status" binding:"omitempty,oneof=pending running success failed canceled timeout"`
	// ApplicationID 按 Application 过滤运行记录。
	ApplicationID string `json:"application_id" form:"application_id"`
}

type ApplicationRunListResponse struct {
	// Total 返回当前查询条件下的 AppRun 总数。
	Total int64 `json:"total"`
	// Items 返回当前页 AppRun。
	Items []*ApplicationRun `json:"items"`
}
