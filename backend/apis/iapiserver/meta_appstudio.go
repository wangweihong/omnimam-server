package iapiserver

import (
	"encoding/json"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"gorm.io/gorm"
)

// +k8s:deepcopy-gen=true
type StudioApplication struct {
	imachinery.ObjectMeta
	OwnerUserID           string `json:"-" gorm:"column:owner_user_id;type:text;not null;uniqueIndex:idx_studio_applications_owner_create_key,priority:1"`
	Status                string `json:"status" gorm:"column:status;type:text;not null"`
	DefaultWorkspaceID    string `json:"-" gorm:"column:default_workspace_id;type:text"`
	CurrentVersionID      string `json:"current_version_id,omitempty" gorm:"column:current_version_id;type:text"`
	CodingAgentID         string `json:"-" gorm:"column:coding_agent_id;type:text"`
	CodingSessionID       string `json:"-" gorm:"column:coding_session_id;type:text"`
	CodingAgentGeneration int    `json:"-" gorm:"column:coding_agent_generation;not null;default:0"`
	CreateIdempotencyKey  string `json:"-" gorm:"column:create_idempotency_key;type:text;uniqueIndex:idx_studio_applications_owner_create_key,priority:2"`
}

func (StudioApplication) TableName() string { return "studio_applications" }

// +k8s:deepcopy-gen=true
type StudioSourceRepository struct {
	imachinery.ObjectMeta
	StudioApplicationID string `json:"studio_application_id" gorm:"column:studio_application_id;type:text;not null"`
	ProviderType        string `json:"provider_type" gorm:"column:provider_type;type:text;not null"`
	Status              string `json:"status" gorm:"column:status;type:text;not null"`
	CurrentRevision     int64  `json:"current_revision" gorm:"column:current_revision;not null;default:0"`
}

func (StudioSourceRepository) TableName() string { return "studio_source_repositories" }

// +k8s:deepcopy-gen=true
type StudioWorkspace struct {
	imachinery.ObjectMeta
	StudioApplicationID   string `json:"studio_application_id" gorm:"column:studio_application_id;type:text;not null"`
	RepositoryID          string `json:"-" gorm:"column:repository_id;type:text;not null"`
	Status                string `json:"status" gorm:"column:status;type:text;not null"`
	CurrentRevision       int64  `json:"current_revision" gorm:"column:current_revision;not null;default:0"`
	CurrentRevisionDigest string `json:"current_revision_digest,omitempty" gorm:"column:current_revision_digest;type:text"`
}

func (StudioWorkspace) TableName() string { return "studio_workspaces" }

// StudioSourceState 是按 StudioApplication 投影的公共源码状态。
// +k8s:deepcopy-gen=true
type StudioSourceState struct {
	StudioApplicationID string          `json:"studio_application_id"`
	CurrentRevision     int64           `json:"current_revision"`
	Status              string          `json:"status"`
	UpdatedAt           imachinery.Time `json:"updated_at"`
}

// +k8s:deepcopy-gen=true
type StudioSourceFile struct {
	imachinery.ObjectMeta
	WorkspaceID   string `json:"-" gorm:"column:workspace_id;type:text;not null"`
	Revision      int64  `json:"-" gorm:"column:revision;not null"`
	Path          string `json:"path" gorm:"column:path;type:text;not null"`
	ContentDigest string `json:"content_digest" gorm:"column:content_digest;type:text;not null"`
	SizeBytes     int64  `json:"size_bytes" gorm:"column:size_bytes;not null"`
	Deleted       bool   `json:"deleted" gorm:"column:deleted;not null;default:false"`
	Protected     bool   `json:"protected" gorm:"column:protected;not null;default:false"`
}

func (StudioSourceFile) TableName() string { return "studio_source_files" }

// +k8s:deepcopy-gen=true
type StudioWorkspaceRevision struct {
	imachinery.ObjectMeta
	WorkspaceID    string `json:"-" gorm:"column:workspace_id;type:text;not null"`
	Revision       int64  `json:"-" gorm:"column:revision;not null"`
	ContentDigest  string `json:"content_digest" gorm:"column:content_digest;type:text;not null"`
	ParentRevision *int64 `json:"parent_revision,omitempty" gorm:"column:parent_revision"`
	CreatedBy      string `json:"created_by" gorm:"column:created_by;type:text;not null"`
	ChangeSetID    string `json:"change_set_id,omitempty" gorm:"column:change_set_id;type:text"`
}

func (StudioWorkspaceRevision) TableName() string { return "studio_workspace_revisions" }

// +k8s:deepcopy-gen=true
type StudioChangeOperation struct {
	Operation  string  `json:"operation"`
	Path       string  `json:"path"`
	Content    *string `json:"content,omitempty"`
	TargetPath *string `json:"target_path,omitempty"`
}

// +k8s:deepcopy-gen=true
type StudioChangeSet struct {
	imachinery.ObjectMeta
	StudioApplicationID string                  `json:"studio_application_id" gorm:"-"`
	WorkspaceID         string                  `json:"-" gorm:"column:workspace_id;type:text;not null"`
	BaseRevision        int64                   `json:"base_revision" gorm:"column:base_revision;not null"`
	TargetRevision      *int64                  `json:"target_revision,omitempty" gorm:"column:target_revision"`
	ActorID             string                  `json:"-" gorm:"column:actor_id;type:text;not null"`
	AgentID             string                  `json:"agent_id,omitempty" gorm:"column:agent_id;type:text"`
	AgentSessionID      string                  `json:"agent_session_id,omitempty" gorm:"column:agent_session_id;type:text"`
	AgentInvocationID   string                  `json:"agent_invocation_id,omitempty" gorm:"column:agent_invocation_id;type:text"`
	Operations          []StudioChangeOperation `json:"operations,omitempty" gorm:"-"`
	OperationsShadow    string                  `json:"-" gorm:"column:operations_json;type:text;not null"`
	Status              string                  `json:"status" gorm:"column:status;type:text;not null"`
	FailureCode         string                  `json:"failure_code,omitempty" gorm:"column:failure_code;type:text"`
	IdempotencyKey      string                  `json:"idempotency_key" gorm:"column:idempotency_key;type:text;not null"`
}

func (StudioChangeSet) TableName() string { return "studio_change_sets" }
func (c *StudioChangeSet) BeforeCreate(tx *gorm.DB) error {
	return beforeAgentJSONCreate(&c.ObjectMeta, tx, c.marshalJSON)
}
func (c *StudioChangeSet) BeforeUpdate(tx *gorm.DB) error {
	return beforeAgentJSONUpdate(&c.ObjectMeta, tx, c.marshalJSON)
}
func (c *StudioChangeSet) AfterFind(tx *gorm.DB) error {
	if err := c.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(c.OperationsShadow, &c.Operations, "[]")
	return nil
}
func (c *StudioChangeSet) marshalJSON() error {
	return marshalJSONFields(jsonField{c.Operations, &c.OperationsShadow, "[]"})
}

// +k8s:deepcopy-gen=true
type StudioSourceSnapshot struct {
	imachinery.ObjectMeta
	StudioApplicationID string `json:"studio_application_id,omitempty" gorm:"column:studio_application_id;type:text;not null"`
	WorkspaceID         string `json:"-" gorm:"column:workspace_id;type:text;not null"`
	WorkspaceRevision   int64  `json:"source_revision" gorm:"column:workspace_revision;not null"`
	ContentDigest       string `json:"content_digest,omitempty" gorm:"column:content_digest;type:text"`
	ManifestDigest      string `json:"manifest_digest,omitempty" gorm:"column:manifest_digest;type:text"`
	Status              string `json:"status" gorm:"column:status;type:text;not null"`
	FailureCode         string `json:"failure_code,omitempty" gorm:"column:failure_code;type:text"`
	CreatedBy           string `json:"-" gorm:"column:created_by;type:text;not null"`
}

func (StudioSourceSnapshot) TableName() string { return "studio_source_snapshots" }

// +k8s:deepcopy-gen=true
type StudioApplicationVersion struct {
	imachinery.ObjectMeta
	StudioApplicationID string          `json:"studio_application_id" gorm:"column:studio_application_id;type:text;not null"`
	SourceSnapshotID    string          `json:"source_snapshot_id" gorm:"column:source_snapshot_id;type:text;not null"`
	Version             string          `json:"version" gorm:"column:version;type:text;not null"`
	IdempotencyKey      string          `json:"-" gorm:"column:idempotency_key;type:text;not null"`
	Status              string          `json:"status" gorm:"column:status;type:text;not null"`
	PublishedAt         imachinery.Time `json:"published_at,omitempty" gorm:"column:published_at"`
}

func (StudioApplicationVersion) TableName() string { return "studio_application_versions" }

// +k8s:deepcopy-gen=true
type StudioPreviewRuntime struct {
	imachinery.ObjectMeta
	StudioApplicationID      string                 `json:"studio_application_id" gorm:"column:studio_application_id;type:text;not null"`
	WorkspaceID              string                 `json:"-" gorm:"column:workspace_id;type:text;not null"`
	WorkspaceRevision        int64                  `json:"source_revision" gorm:"column:workspace_revision;not null"`
	InfraRuntimeID           string                 `json:"-" gorm:"column:infra_runtime_id;type:text"`
	EndpointRef              string                 `json:"-" gorm:"column:endpoint_ref;type:text"`
	Status                   string                 `json:"status" gorm:"column:status;type:text;not null"`
	DiagnosticsSummary       json.RawMessage        `json:"diagnostics_summary,omitempty" gorm:"-"`
	DiagnosticsSummaryShadow string                 `json:"-" gorm:"column:diagnostics_summary_json;type:text;not null;default:'{}'"`
	ExpiresAt                imachinery.Time        `json:"expires_at,omitempty" gorm:"column:expires_at"`
	EndpointSummary          *StudioEndpointSummary `json:"endpoint_summary,omitempty" gorm:"-"`
}

func (StudioPreviewRuntime) TableName() string { return "studio_preview_runtimes" }
func (r *StudioPreviewRuntime) BeforeCreate(tx *gorm.DB) error {
	return beforeAgentJSONCreate(&r.ObjectMeta, tx, r.marshalJSON)
}
func (r *StudioPreviewRuntime) BeforeUpdate(tx *gorm.DB) error {
	return beforeAgentJSONUpdate(&r.ObjectMeta, tx, r.marshalJSON)
}
func (r *StudioPreviewRuntime) AfterFind(tx *gorm.DB) error {
	if err := r.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(r.DiagnosticsSummaryShadow, &r.DiagnosticsSummary, "{}")
	return nil
}
func (r *StudioPreviewRuntime) marshalJSON() error {
	return marshalJSONFields(jsonField{r.DiagnosticsSummary, &r.DiagnosticsSummaryShadow, "{}"})
}

// +k8s:deepcopy-gen=true
type StudioRuntimeConfig struct {
	imachinery.ObjectMeta
	StudioApplicationVersionID  string          `json:"studio_application_version_id" gorm:"column:studio_application_version_id;type:text;not null"`
	Environment                 string          `json:"environment" gorm:"column:environment;type:text;not null"`
	PublicConfig                json.RawMessage `json:"public_config,omitempty" gorm:"-"`
	PublicConfigShadow          string          `json:"-" gorm:"column:public_config_json;type:text;not null;default:'{}'"`
	SecretReferences            []string        `json:"secret_references" gorm:"-"`
	SecretReferencesShadow      string          `json:"-" gorm:"column:secret_references_json;type:text;not null;default:'[]'"`
	IntegrationReferences       []string        `json:"integration_references" gorm:"-"`
	IntegrationReferencesShadow string          `json:"-" gorm:"column:integration_references_json;type:text;not null;default:'[]'"`
	ValidationStatus            string          `json:"validation_status" gorm:"column:validation_status;type:text;not null"`
}

func (StudioRuntimeConfig) TableName() string { return "studio_runtime_configs" }
func (c *StudioRuntimeConfig) BeforeCreate(tx *gorm.DB) error {
	return beforeAgentJSONCreate(&c.ObjectMeta, tx, c.marshalJSON)
}
func (c *StudioRuntimeConfig) BeforeUpdate(tx *gorm.DB) error {
	return beforeAgentJSONUpdate(&c.ObjectMeta, tx, c.marshalJSON)
}
func (c *StudioRuntimeConfig) AfterFind(tx *gorm.DB) error {
	if err := c.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(c.PublicConfigShadow, &c.PublicConfig, "{}")
	unmarshalJSON(c.SecretReferencesShadow, &c.SecretReferences, "[]")
	unmarshalJSON(c.IntegrationReferencesShadow, &c.IntegrationReferences, "[]")
	return nil
}
func (c *StudioRuntimeConfig) marshalJSON() error {
	return marshalJSONFields(jsonField{c.PublicConfig, &c.PublicConfigShadow, "{}"}, jsonField{c.SecretReferences, &c.SecretReferencesShadow, "[]"}, jsonField{c.IntegrationReferences, &c.IntegrationReferencesShadow, "[]"})
}

// +k8s:deepcopy-gen=true
type StudioBuild struct {
	imachinery.ObjectMeta
	OwnerUserID                string          `json:"owner_user_id" gorm:"column:owner_user_id;type:text;not null"`
	StudioApplicationID        string          `json:"studio_application_id" gorm:"column:studio_application_id;type:text;not null"`
	SourceSnapshotID           string          `json:"source_snapshot_id" gorm:"column:source_snapshot_id;type:text;not null"`
	StudioApplicationVersionID string          `json:"studio_application_version_id,omitempty" gorm:"column:studio_application_version_id;type:text"`
	AtomicTaskID               string          `json:"atomic_task_id,omitempty" gorm:"column:atomic_task_id;type:text"`
	ArtifactID                 string          `json:"artifact_id,omitempty" gorm:"column:artifact_id;type:text"`
	ArtifactDigest             string          `json:"artifact_digest,omitempty" gorm:"column:artifact_digest;type:text"`
	Status                     string          `json:"status" gorm:"column:status;type:text;not null"`
	DiagnosticsSummary         json.RawMessage `json:"diagnostics_summary,omitempty" gorm:"-"`
	DiagnosticsSummaryShadow   string          `json:"-" gorm:"column:diagnostics_summary_json;type:text;not null;default:'{}'"`
	IdempotencyKey             string          `json:"-" gorm:"column:idempotency_key;type:text;not null"`
}

func (StudioBuild) TableName() string { return "studio_builds" }
func (b *StudioBuild) BeforeCreate(tx *gorm.DB) error {
	return beforeAgentJSONCreate(&b.ObjectMeta, tx, b.marshalJSON)
}
func (b *StudioBuild) BeforeUpdate(tx *gorm.DB) error {
	return beforeAgentJSONUpdate(&b.ObjectMeta, tx, b.marshalJSON)
}
func (b *StudioBuild) AfterFind(tx *gorm.DB) error {
	if err := b.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(b.DiagnosticsSummaryShadow, &b.DiagnosticsSummary, "{}")
	return nil
}
func (b *StudioBuild) marshalJSON() error {
	return marshalJSONFields(jsonField{b.DiagnosticsSummary, &b.DiagnosticsSummaryShadow, "{}"})
}

// +k8s:deepcopy-gen=true
type StudioRelease struct {
	imachinery.ObjectMeta
	OwnerUserID                string `json:"-" gorm:"column:owner_user_id;type:text;not null"`
	StudioApplicationID        string `json:"studio_application_id" gorm:"column:studio_application_id;type:text;not null"`
	StudioApplicationVersionID string `json:"studio_application_version_id" gorm:"column:studio_application_version_id;type:text;not null"`
	StudioBuildID              string `json:"studio_build_id" gorm:"column:studio_build_id;type:text;not null"`
	RuntimeConfigID            string `json:"runtime_config_id" gorm:"column:runtime_config_id;type:text;not null"`
	ArtifactID                 string `json:"artifact_id" gorm:"column:artifact_id;type:text;not null"`
	ArtifactDigest             string `json:"artifact_digest" gorm:"column:artifact_digest;type:text;not null"`
	Environment                string `json:"environment" gorm:"column:environment;type:text;not null"`
	Status                     string `json:"status" gorm:"column:status;type:text;not null"`
	RollbackOfReleaseID        string `json:"rollback_of_release_id,omitempty" gorm:"column:rollback_of_release_id;type:text"`
	IdempotencyKey             string `json:"-" gorm:"column:idempotency_key;type:text;not null"`
	RuntimeInstanceID          string `json:"runtime_instance_id,omitempty" gorm:"-"`
}

func (StudioRelease) TableName() string { return "studio_releases" }

// +k8s:deepcopy-gen=true
type StudioRuntimeInstance struct {
	imachinery.ObjectMeta
	StudioApplicationID string                 `json:"-" gorm:"column:studio_application_id;type:text;not null"`
	StudioReleaseID     string                 `json:"studio_release_id" gorm:"column:studio_release_id;type:text;not null"`
	Environment         string                 `json:"environment" gorm:"column:environment;type:text;not null"`
	AtomicTaskID        string                 `json:"atomic_task_id,omitempty" gorm:"column:atomic_task_id;type:text"`
	InfraRuntimeID      string                 `json:"infra_runtime_id,omitempty" gorm:"column:infra_runtime_id;type:text"`
	EndpointRef         string                 `json:"-" gorm:"column:endpoint_ref;type:text"`
	Status              string                 `json:"status" gorm:"column:status;type:text;not null"`
	HealthStatus        string                 `json:"health_status" gorm:"column:health_status;type:text;not null"`
	ErrorCode           string                 `json:"error_code,omitempty" gorm:"column:error_code;type:text"`
	IsCurrent           bool                   `json:"is_current" gorm:"column:is_current;not null;default:false"`
	EndpointSummary     *StudioEndpointSummary `json:"endpoint_summary,omitempty" gorm:"-"`
}

func (StudioRuntimeInstance) TableName() string { return "studio_runtime_instances" }

// +k8s:deepcopy-gen=true
type AppStudioOutbox struct {
	imachinery.ObjectMeta
	AggregateType  string          `json:"aggregate_type" gorm:"column:aggregate_type;type:text;not null"`
	AggregateID    string          `json:"aggregate_id" gorm:"column:aggregate_id;type:text;not null"`
	EventType      string          `json:"event_type" gorm:"column:event_type;type:text;not null"`
	Payload        json.RawMessage `json:"payload" gorm:"-"`
	PayloadShadow  string          `json:"-" gorm:"column:payload_json;type:text;not null"`
	IdempotencyKey string          `json:"idempotency_key" gorm:"column:idempotency_key;type:text;not null;uniqueIndex"`
	DeliveryStatus string          `json:"delivery_status" gorm:"column:delivery_status;type:text;not null"`
	NextAttemptAt  imachinery.Time `json:"next_attempt_at,omitempty" gorm:"column:next_attempt_at"`
	DeliveredAt    imachinery.Time `json:"delivered_at,omitempty" gorm:"column:delivered_at"`
}

func (AppStudioOutbox) TableName() string { return "appstudio_outbox" }
func (e *AppStudioOutbox) BeforeCreate(tx *gorm.DB) error {
	return beforeAgentJSONCreate(&e.ObjectMeta, tx, e.marshalJSON)
}
func (e *AppStudioOutbox) BeforeUpdate(tx *gorm.DB) error {
	return beforeAgentJSONUpdate(&e.ObjectMeta, tx, e.marshalJSON)
}
func (e *AppStudioOutbox) AfterFind(tx *gorm.DB) error {
	if err := e.ObjectMeta.AfterFind(tx); err != nil {
		return err
	}
	unmarshalJSON(e.PayloadShadow, &e.Payload, "{}")
	return nil
}
func (e *AppStudioOutbox) marshalJSON() error {
	return marshalJSONFields(jsonField{e.Payload, &e.PayloadShadow, "{}"})
}

// +k8s:deepcopy-gen=true
type StudioEndpointSummary struct {
	DisplayRef string          `json:"display_ref,omitempty"`
	Visibility string          `json:"visibility"`
	Status     string          `json:"status"`
	ExpiresAt  imachinery.Time `json:"expires_at,omitempty"`
}

// +k8s:deepcopy-gen=true
type StudioRuntimeLogEntry struct {
	OccurredAt imachinery.Time `json:"occurred_at"`
	Level      string          `json:"level"`
	Message    string          `json:"message"`
	Source     string          `json:"source,omitempty"`
}
