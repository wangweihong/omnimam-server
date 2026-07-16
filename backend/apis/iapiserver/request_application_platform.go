package iapiserver

import "github.com/wangweihong/omnimam/backend/apis/imachinery"

type (
	PageInfo struct {
		PageNum  int   `json:"page_num"`
		PageSize int   `json:"page_size"`
		Total    int64 `json:"total"`
	}

	ProviderAdapterListRequest struct {
		imachinery.BasicQueryParam
		PlatformType   string `json:"platform_type" form:"platform_type"`
		CapabilityType string `json:"capability_type" form:"capability_type"`
		Enabled        *bool  `json:"enabled" form:"enabled"`
	}

	ProviderAdapterListResponse struct {
		Items []*ProviderAdapter `json:"items"`
		Page  PageInfo           `json:"page"`
	}

	ProviderOperationListRequest struct {
		SourceMode     string `json:"source_mode" form:"source_mode"`
		CapabilityType string `json:"capability_type" form:"capability_type"`
	}

	ProviderOperationListResponse struct {
		Items []*ProviderOperation `json:"items"`
	}

	AppTemplateListRequest struct {
		imachinery.BasicQueryParam
		OwnerUserID string `json:"-" form:"-"`
		IncludeAll  bool   `json:"-" form:"-"`
		SourceKind  string `json:"source_kind" form:"source_kind"`
		AdapterKey  string `json:"adapter_key" form:"adapter_key"`
	}

	AppTemplateCreateRequest struct {
		Name             string         `json:"name" binding:"required"`
		Description      string         `json:"description"`
		SourceKind       string         `json:"source_kind" binding:"required,oneof=comfyui_workflow provider_workflow"`
		AdapterKey       string         `json:"adapter_key" binding:"required"`
		OperationKey     string         `json:"operation_key" binding:"required"`
		OperationVersion string         `json:"operation_version" binding:"required"`
		RawConfig        map[string]any `json:"raw_config" binding:"required"`
		Kind             string         `json:"-"`
		Config           map[string]any `json:"-"`
		SaaSPlatformType string         `json:"-"`
		CapabilityType   string         `json:"-"`
	}

	MetadataUpdateRequest struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}

	AppTemplateUpdateRequest struct {
		ID string `json:"id"`
		MetadataUpdateRequest
	}

	AppTemplateListResponse struct {
		Items []*AppTemplate `json:"items"`
		Page  PageInfo       `json:"page"`
	}

	CapabilityGraphResponse struct {
		Data CapabilityGraph `json:"data"`
	}

	InputMappingInput struct {
		InputKey      string `json:"input_key" binding:"required"`
		InputLabel    string `json:"input_label" binding:"required"`
		SourcePortKey string `json:"source_port_key" binding:"required"`
		SourcePath    string `json:"source_path" binding:"required"`
		DataType      string `json:"data_type" binding:"required"`
		DefaultValue  any    `json:"default_value"`
		SortOrder     int    `json:"sort_order"`
	}

	InputMappingSaveRequest struct {
		Items []InputMappingInput `json:"items" binding:"required"`
	}

	InputMappingListResponse struct {
		Items []*InputMapping `json:"items"`
	}

	OutputMappingInput struct {
		OutputKey       string `json:"output_key" binding:"required"`
		OutputLabel     string `json:"output_label" binding:"required"`
		SourcePortKey   string `json:"source_port_key" binding:"required"`
		SourcePath      string `json:"source_path" binding:"required"`
		DataType        string `json:"data_type" binding:"required"`
		Cardinality     string `json:"cardinality" binding:"required,oneof=single multiple"`
		Primary         bool   `json:"primary"`
		Materialization string `json:"materialization" binding:"required,oneof=inline reference asset"`
		SortOrder       int    `json:"sort_order"`
	}

	OutputMappingSaveRequest struct {
		Items []OutputMappingInput `json:"items" binding:"required"`
	}

	OutputMappingListResponse struct {
		Items []*OutputMapping `json:"items"`
	}

	ApplicationListRequest struct {
		imachinery.BasicQueryParam
		OwnerUserID    string `json:"-" form:"-"`
		IncludeAll     bool   `json:"-" form:"-"`
		TemplateID     string `json:"template_id" form:"template_id"`
		SourceType     string `json:"source_type" form:"source_type"`
		CapabilityType string `json:"capability_type" form:"capability_type"`
	}

	ApplicationFromTemplateRequest struct {
		Name            string                   `json:"name" binding:"required"`
		Description     string                   `json:"description"`
		InputMappings   InputMappingSaveRequest  `json:"input_mappings" binding:"required"`
		OutputMappings  OutputMappingSaveRequest `json:"output_mappings" binding:"required"`
		FixedParameters map[string]any           `json:"fixed_parameters" binding:"required"`
	}

	ApplicationFromOperationRequest struct {
		Name             string                   `json:"name" binding:"required"`
		Description      string                   `json:"description"`
		AdapterKey       string                   `json:"adapter_key" binding:"required"`
		OperationKey     string                   `json:"operation_key" binding:"required"`
		OperationVersion string                   `json:"operation_version" binding:"required"`
		InputMappings    InputMappingSaveRequest  `json:"input_mappings" binding:"required"`
		OutputMappings   OutputMappingSaveRequest `json:"output_mappings" binding:"required"`
		FixedParameters  map[string]any           `json:"fixed_parameters" binding:"required"`
	}

	ApplicationCreateRequest = ApplicationFromOperationRequest

	ApplicationUpdateRequest struct {
		ID              string          `json:"id"`
		Name            *string         `json:"name"`
		Description     *string         `json:"description"`
		FixedParameters *map[string]any `json:"fixed_parameters"`
	}

	ApplicationListResponse struct {
		Items []*Application `json:"items"`
		Page  PageInfo       `json:"page"`
	}

	AvailableEngineListResponse struct {
		Items []*AvailableEngine `json:"items"`
	}

	AppEngineListRequest struct {
		imachinery.BasicQueryParam
		OwnerUserID  string `json:"-" form:"-"`
		IncludeAll   bool   `json:"-" form:"-"`
		AdapterKey   string `json:"adapter_key" form:"adapter_key"`
		Status       string `json:"status" form:"status"`
		HealthStatus string `json:"health_status" form:"health_status"`
	}

	AppEngineCreateRequest struct {
		Name                string               `json:"name" binding:"required"`
		Description         string               `json:"description"`
		AdapterKey          string               `json:"adapter_key" binding:"required"`
		Endpoint            string               `json:"endpoint" binding:"required"`
		AuthType            string               `json:"auth_type" binding:"required,oneof=bearer_token api_key ak_sk none"`
		AuthConfig          AppEngineAuthConfig  `json:"auth_config"`
		SupportedOperations []SupportedOperation `json:"supported_operations" binding:"required"`
		NodeTypes           []string             `json:"node_types"`
		ModelRefs           []string             `json:"model_refs"`
		Priority            int                  `json:"priority"`
		MaxConcurrency      int                  `json:"max_concurrency" binding:"required,min=1"`
	}

	AppEngineUpdateRequest struct {
		ID                  string                `json:"id"`
		Name                *string               `json:"name"`
		Description         *string               `json:"description"`
		Endpoint            *string               `json:"endpoint"`
		AuthType            *string               `json:"auth_type"`
		AuthConfig          *AppEngineAuthConfig  `json:"auth_config"`
		Status              *string               `json:"status"`
		SupportedOperations *[]SupportedOperation `json:"supported_operations"`
		NodeTypes           *[]string             `json:"node_types"`
		ModelRefs           *[]string             `json:"model_refs"`
		Priority            *int                  `json:"priority"`
		MaxConcurrency      *int                  `json:"max_concurrency"`
	}

	AppEngineHealthCheckRequest struct {
		AdapterKey string              `json:"adapter_key" binding:"required"`
		Endpoint   string              `json:"endpoint" binding:"required"`
		AuthType   string              `json:"auth_type" binding:"required"`
		AuthConfig AppEngineAuthConfig `json:"auth_config"`
	}

	AppEngineHealthCheckResult struct {
		HealthStatus      string          `json:"health_status"`
		CheckedAt         imachinery.Time `json:"checked_at"`
		UnhealthyReason   string          `json:"unhealthy_reason,omitempty"`
		LatencyMS         int64           `json:"latency_ms,omitempty"`
		CapabilitySummary map[string]any  `json:"capability_summary,omitempty"`
	}

	AppEngineListResponse struct {
		Items []*AppEngine `json:"items"`
		Page  PageInfo     `json:"page"`
	}

	ApplicationRunCreateRequest struct {
		AppEngineID string         `json:"app_engine_id"`
		Input       map[string]any `json:"input" binding:"required"`
	}

	ApplicationRunListRequest struct {
		imachinery.BasicQueryParam
		OwnerUserID   string `json:"-" form:"-"`
		IncludeAll    bool   `json:"-" form:"-"`
		ApplicationID string `json:"application_id" form:"application_id"`
		RunMode       string `json:"run_mode" form:"run_mode"`
		TaskStatus    string `json:"task_status" form:"task_status"`
	}

	ApplicationRunListResponse struct {
		Items []*ApplicationRun `json:"items"`
		Page  PageInfo          `json:"page"`
	}
)
