package iapiserver

import (
	"errors"
	"regexp"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

type ProviderCapabilityListRequest struct {
	imachinery.BasicQueryParam
	ApplicationEngineTypeID string `form:"application_engine_type_id"`
	Availability            string `form:"availability" binding:"omitempty,oneof=available disabled unavailable"`
}
type ProviderCapabilityLoadResultListRequest struct {
	imachinery.BasicQueryParam
	Result string `form:"result" binding:"omitempty,oneof=loaded disabled failed"`
}
type ApplicationEngineTypeListRequest struct{ imachinery.BasicQueryParam }
type EngineInstanceListRequest struct {
	imachinery.BasicQueryParam
	ApplicationEngineTypeID string `form:"application_engine_type_id"`
	HealthStatus            string `form:"health_status" binding:"omitempty,oneof=unknown online offline degraded"`
	Enabled                 *bool  `form:"enabled"`
}
type EngineCapabilityBindingListRequest struct {
	imachinery.BasicQueryParam
	EngineInstanceID     string `form:"engine_instance_id"`
	ProviderCapabilityID string `form:"provider_capability_id"`
	Enabled              *bool  `form:"enabled"`
}
type ApplicationTemplateListRequest struct {
	imachinery.BasicQueryParam
	CapabilitySourceType   string `form:"capability_source_type" binding:"omitempty,oneof=provider_capability comfyui_workflow"`
	CapabilityDefinitionID string `form:"capability_definition_id"`
	OwnerUserID            string `form:"-"`
}
type ApplicationTemplateVersionListRequest struct {
	imachinery.BasicQueryParam
	ApplicationTemplateID string `form:"-"`
	Status                string `form:"status" binding:"omitempty,oneof=draft published retired"`
}
type ApplicationListRequest struct {
	imachinery.BasicQueryParam
	CapabilityDefinitionID string `form:"capability_definition_id"`
	Visibility             string `form:"visibility" binding:"omitempty,oneof=private global"`
	RunEnabled             *bool  `form:"run_enabled"`
	OwnerUserID            string `form:"-"`
	IncludeGlobal          bool   `form:"-"`
}
type ApplicationVersionListRequest struct {
	imachinery.BasicQueryParam
	ApplicationID string `form:"-"`
	Status        string `form:"status" binding:"omitempty,oneof=draft published retired"`
}

type EngineInstanceCreateRequest struct {
	Name                    string         `json:"name" binding:"required,max=255"`
	Description             string         `json:"description"`
	ApplicationEngineTypeID string         `json:"application_engine_type_id" binding:"required"`
	BaseURL                 string         `json:"base_url" binding:"required,url"`
	AuthType                string         `json:"auth_type" binding:"required,oneof=none api_key bearer_token ak_sk"`
	AuthConfig              map[string]any `json:"auth_config"`
	Enabled                 *bool          `json:"enabled" binding:"required"`
	Region                  string         `json:"region"`
	MaxConcurrency          int            `json:"max_concurrency" binding:"required,min=1"`
	RequestTimeoutSeconds   int            `json:"request_timeout_seconds" binding:"omitempty,min=1"`
	TaskTimeoutSeconds      int            `json:"task_timeout_seconds" binding:"omitempty,min=1"`
}
type EngineInstanceUpdateRequest struct {
	ID                    string         `json:"-"`
	Name                  *string        `json:"name"`
	Description           *string        `json:"description"`
	BaseURL               *string        `json:"base_url" binding:"omitempty,url"`
	AuthType              *string        `json:"auth_type" binding:"omitempty,oneof=none api_key bearer_token ak_sk"`
	AuthConfig            map[string]any `json:"auth_config"`
	Enabled               *bool          `json:"enabled"`
	Region                *string        `json:"region"`
	MaxConcurrency        *int           `json:"max_concurrency" binding:"omitempty,min=1"`
	RequestTimeoutSeconds *int           `json:"request_timeout_seconds" binding:"omitempty,min=1"`
	TaskTimeoutSeconds    *int           `json:"task_timeout_seconds" binding:"omitempty,min=1"`
	ResourceVersion       int64          `json:"resource_version" binding:"required"`
}
type EngineCapabilityBindingCreateRequest struct {
	Name                 string         `json:"name" binding:"required"`
	Description          string         `json:"description"`
	EngineInstanceID     string         `json:"engine_instance_id" binding:"required"`
	ProviderCapabilityID string         `json:"provider_capability_id" binding:"required"`
	Enabled              *bool          `json:"enabled" binding:"required"`
	Restrictions         map[string]any `json:"restrictions"`
}
type EngineCapabilityBindingUpdateRequest struct {
	ID              string         `json:"-"`
	Name            *string        `json:"name"`
	Description     *string        `json:"description"`
	Enabled         *bool          `json:"enabled"`
	Restrictions    map[string]any `json:"restrictions"`
	ResourceVersion int64          `json:"resource_version" binding:"required"`
}

type ApplicationTemplateCreateRequest struct {
	Name                   string         `json:"name" binding:"required"`
	Description            string         `json:"description"`
	CapabilityDefinitionID string         `json:"capability_definition_id" binding:"required"`
	CapabilitySourceType   string         `json:"capability_source_type" binding:"required,oneof=provider_capability"`
	ProviderCapabilityID   string         `json:"provider_capability_id" binding:"required"`
	ProviderOperationID    string         `json:"provider_operation_id" binding:"required"`
	TemplateContract       map[string]any `json:"template_contract"`
}
type ApplicationTemplateVersionCreateRequest struct {
	Name                 string         `json:"name"`
	Description          string         `json:"description"`
	CapabilitySourceType string         `json:"capability_source_type" binding:"required,oneof=provider_capability comfyui_workflow"`
	ProviderCapabilityID string         `json:"provider_capability_id"`
	ProviderOperationID  string         `json:"provider_operation_id"`
	ComfyUIAPIWorkflow   map[string]any `json:"comfyui_api_workflow"`
	TemplateContract     map[string]any `json:"template_contract"`
}
type ApplicationCreateRequest struct {
	Name                   string `json:"name" binding:"required"`
	Description            string `json:"description"`
	CapabilityDefinitionID string `json:"capability_definition_id" binding:"required"`
	Visibility             string `json:"visibility" binding:"omitempty,oneof=private global"`
	RunEnabled             *bool  `json:"run_enabled"`
	CanvasEnabled          *bool  `json:"canvas_enabled"`
	CopyEnabled            *bool  `json:"copy_enabled"`
	PresetEnabled          *bool  `json:"preset_enabled"`
}
type ApplicationUpdateRequest struct {
	ID              string  `json:"-"`
	Name            *string `json:"name"`
	Description     *string `json:"description"`
	Visibility      *string `json:"visibility" binding:"omitempty,oneof=private global"`
	RunEnabled      *bool   `json:"run_enabled"`
	CanvasEnabled   *bool   `json:"canvas_enabled"`
	CopyEnabled     *bool   `json:"copy_enabled"`
	PresetEnabled   *bool   `json:"preset_enabled"`
	ResourceVersion int64   `json:"resource_version" binding:"required"`
}
type ApplicationVersionCreateRequest struct {
	SemanticVersion              string         `json:"semantic_version" binding:"required"`
	ApplicationTemplateVersionID string         `json:"application_template_version_id" binding:"required"`
	InputSchema                  map[string]any `json:"input_schema"`
	OutputSchema                 map[string]any `json:"output_schema"`
	ParameterPolicies            map[string]any `json:"parameter_policies"`
}
type RuntimeFormResolveRequest struct {
	ApplicationVersionID string         `json:"application_version_id" binding:"required"`
	EngineInstanceID     string         `json:"engine_instance_id"`
	CurrentValues        map[string]any `json:"current_values"`
}
type ApplicationRunCreateRequest struct {
	ApplicationVersionID string         `json:"application_version_id" binding:"required"`
	EngineInstanceID     string         `json:"engine_instance_id"`
	Inputs               map[string]any `json:"inputs"`
	IdempotencyKey       string         `json:"idempotency_key" binding:"required"`
}

var applicationSemanticVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

// Validate distinguishes required JSON fields whose valid zero values cannot be expressed by binding tags.
func (r *EngineInstanceCreateRequest) Validate() error {
	if r.Enabled == nil {
		return errors.New("enabled is required")
	}
	if r.AuthType == EngineAuthNone && r.AuthConfig != nil {
		return errors.New("auth_config must be omitted when auth_type is none")
	}
	if r.AuthType != EngineAuthNone && r.AuthConfig == nil {
		return errors.New("auth_config is required for the selected auth_type")
	}
	return nil
}

func (r *EngineInstanceUpdateRequest) Validate() error {
	if r.AuthType == nil && r.AuthConfig != nil {
		return errors.New("auth_type and auth_config must be updated together")
	}
	if r.AuthType != nil && *r.AuthType == EngineAuthNone && r.AuthConfig != nil {
		return errors.New("auth_config must be omitted when auth_type is none")
	}
	if r.AuthType != nil && *r.AuthType != EngineAuthNone && r.AuthConfig == nil {
		return errors.New("auth_config is required when auth_type changes")
	}
	return nil
}

func (r *EngineCapabilityBindingCreateRequest) Validate() error {
	if r.Enabled == nil {
		return errors.New("enabled is required")
	}
	if r.Restrictions == nil {
		return errors.New("restrictions is required")
	}
	return nil
}

func (r *ApplicationTemplateCreateRequest) Validate() error {
	if r.TemplateContract == nil {
		return errors.New("template_contract is required")
	}
	return nil
}

func (r *ApplicationTemplateVersionCreateRequest) Validate() error {
	if r.TemplateContract == nil {
		return errors.New("template_contract is required")
	}
	switch r.CapabilitySourceType {
	case CapabilitySourceProviderCapability:
		if r.ProviderCapabilityID == "" || r.ProviderOperationID == "" {
			return errors.New("provider_capability_id and provider_operation_id are required")
		}
		if r.ComfyUIAPIWorkflow != nil {
			return errors.New("ComfyUI fields are not allowed for provider capability versions")
		}
	case CapabilitySourceComfyUIWorkflow:
		if r.ProviderCapabilityID != "" || r.ProviderOperationID != "" {
			return errors.New("provider fields are not allowed for ComfyUI versions")
		}
		if r.ComfyUIAPIWorkflow == nil {
			return errors.New("comfyui_api_workflow is required")
		}
	}
	return nil
}

func (r *ApplicationVersionCreateRequest) Validate() error {
	if !applicationSemanticVersionPattern.MatchString(r.SemanticVersion) {
		return errors.New("semantic_version is invalid")
	}
	if r.InputSchema == nil || r.OutputSchema == nil || r.ParameterPolicies == nil {
		return errors.New("input_schema, output_schema, and parameter_policies are required")
	}
	return nil
}

func (r *ApplicationRunCreateRequest) Validate() error {
	if r.Inputs == nil {
		return errors.New("inputs is required")
	}
	return nil
}
