package iapiserver

import (
	"encoding/json"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"gorm.io/gorm"
)

// +k8s:deepcopy-gen=true
type InfraRuntimeProfile struct {
	imachinery.ObjectMeta
	Revision           string          `json:"revision" gorm:"column:revision;type:text;not null"`
	RuntimeMode        string          `json:"runtime_mode" gorm:"column:runtime_mode;type:text;not null"`
	ProviderType       string          `json:"provider_type" gorm:"column:provider_type;type:text;not null"`
	Capabilities       []string        `json:"capabilities" gorm:"-"`
	CapabilitiesShadow string          `json:"-" gorm:"column:capabilities_json;type:text;not null;default:'[]'"`
	Policy             json.RawMessage `json:"-" gorm:"-"`
	PolicyShadow       string          `json:"-" gorm:"column:policy_json;type:text;not null;default:'{}'"`
	Status             string          `json:"status" gorm:"column:status;type:text;not null"`
}

func (InfraRuntimeProfile) TableName() string { return "infra_runtime_profiles" }
func (p *InfraRuntimeProfile) BeforeCreate(tx *gorm.DB) error {
	return beforeAgentJSONCreate(&p.ObjectMeta, tx, p.marshalJSON)
}
func (p *InfraRuntimeProfile) BeforeUpdate(tx *gorm.DB) error {
	return beforeAgentJSONUpdate(&p.ObjectMeta, tx, p.marshalJSON)
}
func (p *InfraRuntimeProfile) AfterFind(tx *gorm.DB) error {
	if err := p.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(p.CapabilitiesShadow, &p.Capabilities, "[]")
	unmarshalJSON(p.PolicyShadow, &p.Policy, "{}")
	return nil
}
func (p *InfraRuntimeProfile) marshalJSON() error {
	return marshalJSONFields(jsonField{p.Capabilities, &p.CapabilitiesShadow, "[]"}, jsonField{p.Policy, &p.PolicyShadow, "{}"})
}

// +k8s:deepcopy-gen=true
type InfraNode struct {
	imachinery.ObjectMeta
	ProviderType    string          `json:"provider_type" gorm:"column:provider_type;type:text;not null"`
	Status          string          `json:"status" gorm:"column:status;type:text;not null"`
	CPUCores        float64         `json:"cpu_cores" gorm:"column:cpu_cores;not null;default:0"`
	MemoryMB        int64           `json:"memory_mb" gorm:"column:memory_mb;not null;default:0"`
	DiskMB          int64           `json:"disk_mb,omitempty" gorm:"column:disk_mb;not null;default:0"`
	GPUCount        int             `json:"gpu_count" gorm:"column:gpu_count;not null;default:0"`
	GPUMemoryMB     int64           `json:"gpu_memory_mb,omitempty" gorm:"column:gpu_memory_mb;not null;default:0"`
	LastHeartbeatAt imachinery.Time `json:"last_heartbeat_at,omitempty" gorm:"column:last_heartbeat_at"`
}

func (InfraNode) TableName() string { return "infra_nodes" }

// +k8s:deepcopy-gen=true
type InfraRuntime struct {
	imachinery.ObjectMeta
	RuntimeMode            string          `json:"runtime_mode" gorm:"column:runtime_mode;type:text;not null"`
	Status                 string          `json:"status" gorm:"column:status;type:text;not null"`
	RequestingService      string          `json:"requesting_service" gorm:"column:requesting_service;type:text;not null"`
	OwnerDomain            string          `json:"owner_domain" gorm:"column:owner_domain;type:text;not null"`
	OwnerReference         string          `json:"owner_reference" gorm:"column:owner_reference;type:text;not null"`
	RequestUserID          string          `json:"request_user_id,omitempty" gorm:"column:request_user_id;type:text"`
	RequestID              string          `json:"request_id" gorm:"column:request_id;type:text;not null"`
	RequestFingerprint     string          `json:"-" gorm:"column:request_fingerprint;type:text;not null"`
	RuntimeProfileID       string          `json:"runtime_profile_id" gorm:"column:runtime_profile_id;type:text;not null"`
	RuntimeProfileRevision string          `json:"runtime_profile_revision" gorm:"column:runtime_profile_revision;type:text;not null"`
	ProviderType           string          `json:"provider_type" gorm:"column:provider_type;type:text;not null"`
	ProviderRuntimeRef     string          `json:"-" gorm:"column:provider_runtime_ref;type:text"`
	SelectedNodeID         string          `json:"-" gorm:"column:selected_node_id;type:text"`
	EndpointRef            string          `json:"endpoint_ref,omitempty" gorm:"column:endpoint_ref;type:text"`
	SourceRef              string          `json:"-" gorm:"column:source_ref;type:text"`
	FailureCode            string          `json:"failure_code,omitempty" gorm:"column:failure_code;type:text"`
	TimeoutPolicy          json.RawMessage `json:"-" gorm:"-"`
	TimeoutPolicyShadow    string          `json:"-" gorm:"column:timeout_policy_json;type:text;not null;default:'{}'"`
}

func (InfraRuntime) TableName() string { return "infra_runtimes" }
func (r *InfraRuntime) BeforeCreate(tx *gorm.DB) error {
	return beforeAgentJSONCreate(&r.ObjectMeta, tx, r.marshalJSON)
}
func (r *InfraRuntime) BeforeUpdate(tx *gorm.DB) error {
	return beforeAgentJSONUpdate(&r.ObjectMeta, tx, r.marshalJSON)
}
func (r *InfraRuntime) AfterFind(tx *gorm.DB) error {
	if err := r.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(r.TimeoutPolicyShadow, &r.TimeoutPolicy, "{}")
	return nil
}
func (r *InfraRuntime) marshalJSON() error {
	return marshalJSONFields(jsonField{r.TimeoutPolicy, &r.TimeoutPolicyShadow, "{}"})
}

// +k8s:deepcopy-gen=true
type InfraRuntimeEndpoint struct {
	imachinery.ObjectMeta
	RuntimeID  string          `json:"runtime_id" gorm:"column:runtime_id;type:text;not null"`
	Visibility string          `json:"visibility" gorm:"column:visibility;type:text;not null"`
	Status     string          `json:"status" gorm:"column:status;type:text;not null"`
	DisplayRef string          `json:"display_ref,omitempty" gorm:"column:display_ref;type:text"`
	ExpiresAt  imachinery.Time `json:"expires_at,omitempty" gorm:"column:expires_at"`
	RevokedAt  imachinery.Time `json:"-" gorm:"column:revoked_at"`
}

func (InfraRuntimeEndpoint) TableName() string { return "infra_runtime_endpoints" }

// +k8s:deepcopy-gen=true
type InfraRuntimeMount struct {
	imachinery.ObjectMeta
	RuntimeID        string `json:"runtime_id" gorm:"column:runtime_id;type:text;not null"`
	SourceRef        string `json:"source_ref" gorm:"column:source_ref;type:text;not null"`
	MountKind        string `json:"mount_kind" gorm:"column:mount_kind;type:text;not null"`
	TargetPath       string `json:"-" gorm:"column:target_path;type:text;not null"`
	ReadOnly         bool   `json:"read_only" gorm:"column:read_only;not null"`
	AuthorizationRef string `json:"-" gorm:"column:authorization_ref;type:text"`
}

func (InfraRuntimeMount) TableName() string { return "infra_runtime_mounts" }

// +k8s:deepcopy-gen=true
type InfraRuntimeConfigBinding struct {
	imachinery.ObjectMeta
	RuntimeID       string `json:"runtime_id" gorm:"column:runtime_id;type:text;not null"`
	BindingType     string `json:"binding_type" gorm:"column:binding_type;type:text;not null"`
	Reference       string `json:"-" gorm:"column:reference;type:text;not null"`
	InjectionStatus string `json:"injection_status" gorm:"column:injection_status;type:text;not null"`
	FailureCode     string `json:"failure_code,omitempty" gorm:"column:failure_code;type:text"`
}

func (InfraRuntimeConfigBinding) TableName() string { return "infra_runtime_config_bindings" }

// +k8s:deepcopy-gen=true
type InfraRuntimeOutput struct {
	imachinery.ObjectMeta
	RuntimeID   string `json:"runtime_id" gorm:"column:runtime_id;type:text;not null"`
	OutputKey   string `json:"output_key" gorm:"column:output_key;type:text;not null"`
	Status      string `json:"status" gorm:"column:status;type:text;not null"`
	ArtifactID  string `json:"artifact_id,omitempty" gorm:"column:artifact_id;type:text"`
	MediaType   string `json:"media_type,omitempty" gorm:"column:media_type;type:text"`
	FailureCode string `json:"-" gorm:"column:failure_code;type:text"`
}

func (InfraRuntimeOutput) TableName() string { return "infra_runtime_outputs" }

// +k8s:deepcopy-gen=true
type InfraRuntimeEvent struct {
	imachinery.ObjectMeta
	RuntimeID         string          `json:"runtime_id" gorm:"column:runtime_id;type:text;not null"`
	EventType         string          `json:"event_type" gorm:"column:event_type;type:text;not null"`
	ProviderSequence  string          `json:"-" gorm:"column:provider_sequence;type:text"`
	ProviderStatus    string          `json:"provider_status,omitempty" gorm:"column:provider_status;type:text"`
	SafeSummary       json.RawMessage `json:"safe_summary,omitempty" gorm:"-"`
	SafeSummaryShadow string          `json:"-" gorm:"column:safe_summary_json;type:text;not null;default:'{}'"`
	IdempotencyKey    string          `json:"-" gorm:"column:idempotency_key;type:text;not null;uniqueIndex"`
}

func (InfraRuntimeEvent) TableName() string { return "infra_runtime_events" }
func (e *InfraRuntimeEvent) BeforeCreate(tx *gorm.DB) error {
	return beforeAgentJSONCreate(&e.ObjectMeta, tx, e.marshalJSON)
}
func (e *InfraRuntimeEvent) BeforeUpdate(tx *gorm.DB) error {
	return beforeAgentJSONUpdate(&e.ObjectMeta, tx, e.marshalJSON)
}
func (e *InfraRuntimeEvent) AfterFind(tx *gorm.DB) error {
	if err := e.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(e.SafeSummaryShadow, &e.SafeSummary, "{}")
	return nil
}
func (e *InfraRuntimeEvent) marshalJSON() error {
	return marshalJSONFields(jsonField{e.SafeSummary, &e.SafeSummaryShadow, "{}"})
}

// +k8s:deepcopy-gen=true
type InfraRuntimeLogEntry struct {
	OccurredAt imachinery.Time `json:"occurred_at"`
	Level      string          `json:"level"`
	Message    string          `json:"message"`
	Source     string          `json:"source,omitempty"`
}
