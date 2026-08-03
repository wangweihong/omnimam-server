package iapiserver

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

// +k8s:deepcopy-gen=true
type InfraRuntimeMountInput struct {
	SourceRef        string `json:"source_ref" binding:"required,max=2048"`
	TargetPath       string `json:"target_path" binding:"required,max=512"`
	ReadOnly         bool   `json:"read_only"`
	MountKind        string `json:"mount_kind" binding:"required,oneof=AGENT_WORKSPACE STUDIO_WORKSPACE_REVISION STUDIO_SNAPSHOT ARTIFACT TEMPORARY"`
	AuthorizationRef string `json:"-"`
}

// +k8s:deepcopy-gen=true
type InfraRuntimeConfigBindingInput struct {
	Name        string `json:"name" binding:"required,max=200"`
	BindingType string `json:"binding_type" binding:"required,oneof=PLAIN_CONFIG MODEL_ACCESS SECRET_REF INTEGRATION_REF"`
	Reference   string `json:"reference" binding:"required,max=2048"`
}

// +k8s:deepcopy-gen=true
type InfraResourceRequirement struct {
	CPUCores    float64 `json:"cpu_cores,omitempty" binding:"min=0"`
	MemoryMB    int64   `json:"memory_mb,omitempty" binding:"min=0"`
	DiskMB      int64   `json:"disk_mb,omitempty" binding:"min=0"`
	GPUCount    int     `json:"gpu_count,omitempty" binding:"min=0"`
	GPUMemoryMB int64   `json:"gpu_memory_mb,omitempty" binding:"min=0"`
}

// +k8s:deepcopy-gen=true
type InfraTimeoutPolicy struct {
	StartupTimeoutSeconds   int              `json:"startup_timeout_seconds,omitempty" binding:"omitempty,min=1"`
	ExecutionTimeoutSeconds int              `json:"execution_timeout_seconds,omitempty" binding:"omitempty,min=1"`
	IdleTimeoutSeconds      int              `json:"idle_timeout_seconds,omitempty" binding:"omitempty,min=1"`
	MaximumLifetimeSeconds  int              `json:"maximum_lifetime_seconds,omitempty" binding:"omitempty,min=1"`
	ExpiresAt               *imachinery.Time `json:"expires_at,omitempty"`
}

// +k8s:deepcopy-gen=true
type InfraCreateRuntimeRequest struct {
	RequestID              string                           `json:"request_id" binding:"required,max=200"`
	RequestingService      string                           `json:"requesting_service" binding:"required,oneof=task-center"`
	OwnerDomain            string                           `json:"owner_domain" binding:"required,oneof=agent appstudio task-center asset-library"`
	OwnerReference         string                           `json:"owner_reference" binding:"required,max=512"`
	RequestUserID          string                           `json:"request_user_id,omitempty" binding:"omitempty,max=128"`
	RuntimeMode            string                           `json:"runtime_mode" binding:"required,oneof=JOB SERVICE"`
	RuntimeProfileID       string                           `json:"runtime_profile_id" binding:"required,max=200"`
	RuntimeProfileRevision string                           `json:"runtime_profile_revision" binding:"required,max=64"`
	SourceRef              string                           `json:"source_ref,omitempty" binding:"omitempty,max=2048"`
	Mounts                 []InfraRuntimeMountInput         `json:"mounts,omitempty" binding:"omitempty,max=20,dive"`
	ConfigurationBindings  []InfraRuntimeConfigBindingInput `json:"configuration_bindings,omitempty" binding:"omitempty,max=50,dive"`
	ResourceRequirement    InfraResourceRequirement         `json:"resource_requirement,omitempty"`
	TimeoutPolicy          InfraTimeoutPolicy               `json:"timeout_policy,omitempty"`
	AuthorizationRef       string                           `json:"-"`
	EndpointVisibility     string                           `json:"-"`
	FunctionRef            string                           `json:"-"`
	FunctionArguments      json.RawMessage                  `json:"-"`
}

func (r *InfraCreateRuntimeRequest) Validate() error {
	for _, mount := range r.Mounts {
		if !strings.HasPrefix(mount.SourceRef, "agent-workspace://") && !strings.HasPrefix(mount.SourceRef, "studio-workspace-revision://") && !strings.HasPrefix(mount.SourceRef, "studio-snapshot://") && !strings.HasPrefix(mount.SourceRef, "artifact://") && !strings.HasPrefix(mount.SourceRef, "temporary://") {
			return fmt.Errorf("unsupported mount source reference")
		}
		if !strings.HasPrefix(mount.TargetPath, "/") || strings.Contains(mount.TargetPath, "..") {
			return fmt.Errorf("invalid mount target")
		}
	}
	return nil
}

// +k8s:deepcopy-gen=true
type InfraRuntimeListRequest struct {
	imachinery.BasicQueryParam
	Status      string `form:"status" binding:"omitempty,max=256"`
	OwnerDomain string `form:"owner_domain" binding:"omitempty,oneof=agent appstudio task-center asset-library"`
}

// +k8s:deepcopy-gen=true
type InfraActionRequest struct {
	Reason          string `json:"reason,omitempty" binding:"omitempty,max=1000"`
	RequestID       string `json:"request_id,omitempty" binding:"omitempty,max=200"`
	ResourceVersion int64  `json:"resource_version" binding:"min=0"`
}

// +k8s:deepcopy-gen=true
type InfraBasicListRequest struct{ imachinery.BasicQueryParam }
