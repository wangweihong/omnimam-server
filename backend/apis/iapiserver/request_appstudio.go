package iapiserver

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	// AppStudio 各资源状态独立定义，避免不同生命周期因字面值相同而混用。
	AppStudioApplicationStatusCreating = "CREATING"
	AppStudioApplicationStatusReady    = "READY"
	AppStudioApplicationStatusArchived = "ARCHIVED"
	AppStudioApplicationStatusError    = "ERROR"
	AppStudioSourceStatusInitializing  = "INITIALIZING"
	AppStudioRepositoryStatusReady     = "READY"
	AppStudioWorkspaceStatusReady      = "READY"
	AppStudioChangeSetStatusApplied    = "APPLIED"
	AppStudioSnapshotStatusReady       = "READY"
	AppStudioVersionStatusDraft        = "DRAFT"
	AppStudioBuildStatusPending        = "PENDING"
	AppStudioBuildStatusRunning        = "RUNNING"
	AppStudioBuildStatusSucceeded      = "SUCCEEDED"
	AppStudioBuildStatusFailed         = "FAILED"
	AppStudioBuildStatusCanceled       = "CANCELED"
	AppStudioPreviewStatusPending      = "PENDING"
	AppStudioPreviewStatusRunning      = "RUNNING"
	AppStudioPreviewStatusStopped      = "STOPPED"
	AppStudioPreviewStatusExpired      = "EXPIRED"
	AppStudioPreviewStatusFailed       = "FAILED"
	AppStudioRuntimeConfigStatusValid  = "VALID"
	AppStudioReleaseStatusPending      = "PENDING"
	AppStudioReleaseStatusDeploying    = "DEPLOYING"
	AppStudioReleaseStatusReady        = "READY"
	AppStudioReleaseStatusFailed       = "FAILED"
	AppStudioRuntimeStatusCreating     = "CREATING"
	AppStudioRuntimeStatusReady        = "READY"
	AppStudioRuntimeStatusStopped      = "STOPPED"
	AppStudioRuntimeStatusFailed       = "FAILED"
	AppStudioRuntimeHealthUnknown      = "UNKNOWN"
	AppStudioRuntimeHealthHealthy      = "HEALTHY"
	AppStudioRuntimeHealthUnhealthy    = "UNHEALTHY"
	AppStudioEndpointVisibilityUser    = "USER_ACCESSIBLE"
	AppStudioEndpointStatusReady       = "READY"
	AppStudioOutboxDeliveryPending     = "PENDING"

	AppStudioSourceProviderBuiltIn    = "BUILT_IN"
	AppStudioApplicationTypeStaticWeb = "STATIC_WEB"
	AppStudioApplicationTypeLightWeb  = "WEB_WITH_LIGHT_BACKEND"
	AppStudioChangeOperationCreate    = "create"
	AppStudioChangeOperationUpdate    = "update"
	AppStudioChangeOperationDelete    = "delete"
	AppStudioChangeOperationMove      = "move"
	AppStudioEnvironmentPreview       = "preview"
	AppStudioEnvironmentProduction    = "production"
	AppStudioTaskActionStop           = "STOP"
	AppStudioDeploymentReasonRelease  = "RELEASE"
	AppStudioDeploymentReasonRollback = "ROLLBACK"
	AppStudioArtifactProcessingReady  = "ready"

	AppStudioFunctionPreviewEnsure      = "appstudio.preview.ensure"
	AppStudioFunctionPreviewStop        = "appstudio.preview.stop"
	AppStudioFunctionBuildExecute       = "appstudio.build.execute"
	AppStudioFunctionProductionEnsure   = "appstudio.production.reconcile"
	AppStudioFunctionProductionStop     = "appstudio.production.stop"
	AppStudioTaskDomain                 = "appstudio"
	AppStudioRuntimeProfileRevision     = "1.0"
	AppStudioPreviewProfileStaticWeb    = "appstudio.preview.static-web"
	AppStudioBuildProfileStaticWeb      = "appstudio.build.static-web"
	AppStudioProductionProfileStaticWeb = "appstudio.production.static-web"
	AppStudioDefaultWorkspaceName       = "main"

	AppStudioRefPrefixSecret            = "secret://"
	AppStudioRefPrefixIntegration       = "integration://"
	AppStudioRefPrefixStudioSnapshot    = "studio-snapshot://"
	AppStudioRefPrefixWorkspaceRevision = "studio-workspace-revision://"
	AppStudioRefPrefixBuildConfig       = "appstudio-build-config://"
	AppStudioRefPrefixBuildGrant        = "appstudio-build-grant://"
	AppStudioRefPrefixPreviewGrant      = "appstudio-preview-grant://"
	AppStudioRefPrefixArtifact          = "artifact://"
	AppStudioRefPrefixHealthCheck       = "appstudio-health-check://"
	AppStudioRefPrefixProductionGrant   = "appstudio-production-grant://"

	AppStudioAggregateTypeApplication         = "StudioApplication"
	AppStudioAggregateTypeSourceSnapshot      = "StudioSourceSnapshot"
	AppStudioAggregateTypeBuild               = "StudioBuild"
	AppStudioAggregateTypePreviewRuntime      = "StudioPreviewRuntime"
	AppStudioAggregateTypeRelease             = "StudioRelease"
	AppStudioAggregateTypeRuntime             = "StudioRuntimeInstance"
	AppStudioEventApplicationLifecycleChanged = "studio_application_lifecycle_changed"
	AppStudioEventSourceRevisionChanged       = "studio_source_revision_changed"
	AppStudioEventSourceSnapshotCreated       = "studio_source_snapshot_created"
	AppStudioEventBuildProjectionChanged      = "studio_build_projection_changed"
	AppStudioEventPreviewRuntimeChanged       = "studio_preview_runtime_status_changed"
	AppStudioEventReleaseStatusChanged        = "studio_release_status_changed"
	AppStudioEventRuntimeInstanceChanged      = "studio_runtime_instance_status_changed"
)

// AppStudioBuildTaskArguments 是 AppStudio 构建任务的结构化参数。
type AppStudioBuildTaskArguments struct {
	StudioApplicationID        string  `json:"studio_application_id"`
	StudioBuildID              string  `json:"studio_build_id"`
	SourceSnapshotID           string  `json:"source_snapshot_id"`
	SourceSnapshotDigest       string  `json:"source_snapshot_digest"`
	SourceSnapshotSourceRef    string  `json:"source_snapshot_source_ref"`
	StudioApplicationVersionID *string `json:"studio_application_version_id"`
	RuntimeProfileID           string  `json:"runtime_profile_id"`
	RuntimeProfileRevision     string  `json:"runtime_profile_revision"`
	BuildConfigRef             string  `json:"build_config_ref"`
	DependencyLockDigest       string  `json:"dependency_lock_digest"`
	AuthorizationRef           string  `json:"authorization_ref"`
	ExpectedResourceVersion    int64   `json:"expected_resource_version"`
}

func (a AppStudioBuildTaskArguments) AtomicTaskArguments() map[string]any {
	return map[string]any{
		"studio_application_id":         a.StudioApplicationID,
		"studio_build_id":               a.StudioBuildID,
		"source_snapshot_id":            a.SourceSnapshotID,
		"source_snapshot_digest":        a.SourceSnapshotDigest,
		"source_snapshot_source_ref":    a.SourceSnapshotSourceRef,
		"studio_application_version_id": a.StudioApplicationVersionID,
		"runtime_profile_id":            a.RuntimeProfileID,
		"runtime_profile_revision":      a.RuntimeProfileRevision,
		"build_config_ref":              a.BuildConfigRef,
		"dependency_lock_digest":        a.DependencyLockDigest,
		"authorization_ref":             a.AuthorizationRef,
		"expected_resource_version":     a.ExpectedResourceVersion,
	}
}

// AppStudioPreviewTaskArguments 是 AppStudio 预览环境启动任务的结构化参数。
type AppStudioPreviewTaskArguments struct {
	StudioApplicationID        string  `json:"studio_application_id"`
	PreviewRuntimeID           string  `json:"preview_runtime_id"`
	ExistingInfraRuntimeID     *string `json:"existing_infra_runtime_id"`
	WorkspaceID                string  `json:"workspace_id"`
	WorkspaceRevision          int64   `json:"workspace_revision"`
	WorkspaceRevisionSourceRef string  `json:"workspace_revision_source_ref"`
	RuntimeProfileID           string  `json:"runtime_profile_id"`
	RuntimeProfileRevision     string  `json:"runtime_profile_revision"`
	EndpointVisibility         string  `json:"endpoint_visibility"`
	AuthorizationRef           string  `json:"authorization_ref"`
	ExpectedResourceVersion    int64   `json:"expected_resource_version"`
}

func (a AppStudioPreviewTaskArguments) AtomicTaskArguments() map[string]any {
	return map[string]any{
		"studio_application_id":         a.StudioApplicationID,
		"preview_runtime_id":            a.PreviewRuntimeID,
		"existing_infra_runtime_id":     a.ExistingInfraRuntimeID,
		"workspace_id":                  a.WorkspaceID,
		"workspace_revision":            a.WorkspaceRevision,
		"workspace_revision_source_ref": a.WorkspaceRevisionSourceRef,
		"runtime_profile_id":            a.RuntimeProfileID,
		"runtime_profile_revision":      a.RuntimeProfileRevision,
		"endpoint_visibility":           a.EndpointVisibility,
		"authorization_ref":             a.AuthorizationRef,
		"expected_resource_version":     a.ExpectedResourceVersion,
	}
}

// AppStudioProductionTaskArguments 是 AppStudio 生产环境调和任务的结构化参数。
type AppStudioProductionTaskArguments struct {
	StudioApplicationID        string  `json:"studio_application_id"`
	StudioReleaseID            string  `json:"studio_release_id"`
	StudioRuntimeInstanceID    string  `json:"studio_runtime_instance_id"`
	ExistingInfraRuntimeID     *string `json:"existing_infra_runtime_id"`
	StudioApplicationVersionID string  `json:"studio_application_version_id"`
	RuntimeConfigID            string  `json:"runtime_config_id"`
	ArtifactID                 string  `json:"artifact_id"`
	ArtifactDigest             string  `json:"artifact_digest"`
	ArtifactSourceRef          string  `json:"artifact_source_ref"`
	Environment                string  `json:"environment"`
	DeploymentReason           string  `json:"deployment_reason"`
	RuntimeProfileID           string  `json:"runtime_profile_id"`
	RuntimeProfileRevision     string  `json:"runtime_profile_revision"`
	HealthCheckRef             string  `json:"health_check_ref"`
	EndpointVisibility         string  `json:"endpoint_visibility"`
	AuthorizationRef           string  `json:"authorization_ref"`
	ExpectedResourceVersion    int64   `json:"expected_resource_version"`
}

func (a AppStudioProductionTaskArguments) AtomicTaskArguments() map[string]any {
	return map[string]any{
		"studio_application_id":         a.StudioApplicationID,
		"studio_release_id":             a.StudioReleaseID,
		"studio_runtime_instance_id":    a.StudioRuntimeInstanceID,
		"existing_infra_runtime_id":     a.ExistingInfraRuntimeID,
		"studio_application_version_id": a.StudioApplicationVersionID,
		"runtime_config_id":             a.RuntimeConfigID,
		"artifact_id":                   a.ArtifactID,
		"artifact_digest":               a.ArtifactDigest,
		"artifact_source_ref":           a.ArtifactSourceRef,
		"environment":                   a.Environment,
		"deployment_reason":             a.DeploymentReason,
		"runtime_profile_id":            a.RuntimeProfileID,
		"runtime_profile_revision":      a.RuntimeProfileRevision,
		"health_check_ref":              a.HealthCheckRef,
		"endpoint_visibility":           a.EndpointVisibility,
		"authorization_ref":             a.AuthorizationRef,
		"expected_resource_version":     a.ExpectedResourceVersion,
	}
}

// AppStudioStopTaskArguments 是 AppStudio 预览或生产 Runtime 停止任务的结构化参数。
type AppStudioStopTaskArguments struct {
	StudioApplicationID     string `json:"studio_application_id"`
	PreviewRuntimeID        string `json:"preview_runtime_id,omitempty"`
	StudioReleaseID         string `json:"studio_release_id,omitempty"`
	StudioRuntimeInstanceID string `json:"studio_runtime_instance_id,omitempty"`
	InfraRuntimeID          string `json:"infra_runtime_id"`
	Action                  string `json:"action"`
	Reason                  string `json:"reason,omitempty"`
	AuthorizationRef        string `json:"authorization_ref"`
	ExpectedResourceVersion int64  `json:"expected_resource_version"`
}

func (a AppStudioStopTaskArguments) AtomicTaskArguments() map[string]any {
	arguments := map[string]any{
		"studio_application_id":     a.StudioApplicationID,
		"infra_runtime_id":          a.InfraRuntimeID,
		"action":                    a.Action,
		"authorization_ref":         a.AuthorizationRef,
		"expected_resource_version": a.ExpectedResourceVersion,
	}
	if a.PreviewRuntimeID != "" {
		arguments["preview_runtime_id"] = a.PreviewRuntimeID
	}
	if a.StudioReleaseID != "" {
		arguments["studio_release_id"] = a.StudioReleaseID
	}
	if a.StudioRuntimeInstanceID != "" {
		arguments["studio_runtime_instance_id"] = a.StudioRuntimeInstanceID
	}
	if a.Reason != "" {
		arguments["reason"] = a.Reason
	}
	return arguments
}

// AppStudioTaskOutput 是 AppStudio Task Worker 返回给投影器的结构化输出。
type AppStudioTaskOutput struct {
	InfraRuntimeID string `json:"infra_runtime_id"`
	EndpointRef    string `json:"endpoint_ref"`
	HealthStatus   string `json:"health_status"`
	ArtifactID     string `json:"artifact_id"`
	ArtifactDigest string `json:"artifact_digest"`
}

// AppStudioTaskProjectionArguments 是终态投影定位 AppStudio 资源所需的参数子集。
type AppStudioTaskProjectionArguments struct {
	PreviewRuntimeID        string `json:"preview_runtime_id"`
	StudioBuildID           string `json:"studio_build_id"`
	StudioRuntimeInstanceID string `json:"studio_runtime_instance_id"`
}

// +k8s:deepcopy-gen=true
type StudioApplicationListRequest struct {
	imachinery.BasicQueryParam
	Status      string `form:"status" binding:"omitempty,oneof=CREATING READY ARCHIVED ERROR"`
	OwnerUserID string `form:"-" json:"-"`
}

// +k8s:deepcopy-gen=true
type StudioApplicationCreateRequest struct {
	Name                 string                     `json:"name" binding:"required,min=1,max=200"`
	Description          string                     `json:"description,omitempty" binding:"omitempty,max=2000"`
	InitialRequirement   string                     `json:"initial_requirement" binding:"required,min=1,max=20000"`
	ApplicationType      string                     `json:"application_type,omitempty" binding:"omitempty,oneof=STATIC_WEB WEB_WITH_LIGHT_BACKEND"`
	BackendRequired      bool                       `json:"backend_required,omitempty"`
	CodingAgentProfile   string                     `json:"coding_agent_profile,omitempty" binding:"omitempty,max=200"`
	CodingModelSelection StudioCodingModelSelection `json:"coding_model_selection" binding:"required"`
	Attachments          []StudioAgentAttachment    `json:"attachments,omitempty" binding:"omitempty,max=50,dive"`
	IdempotencyKey       string                     `json:"idempotency_key" binding:"required,min=1,max=200"`
}

// +k8s:deepcopy-gen=true
type StudioCodingModelSelection struct {
	SourceType string `json:"source_type" binding:"required,oneof=USER_DEFAULT_MODEL USER_PROVIDER_MODEL PLATFORM_MODEL"`
	SourceRef  string `json:"source_ref" binding:"required,min=1,max=500"`
}

// +k8s:deepcopy-gen=true
type StudioAgentAttachment struct {
	Type        string `json:"type" binding:"required,oneof=ASSET ARTIFACT SOURCE_FILE BUILD_LOG PREVIEW_LOG"`
	ReferenceID string `json:"reference_id" binding:"required,min=1"`
}

// StudioAgentMessageRequest 通过应用级 facade 向当前 generation 的 Coding Agent 发送开发指令。
// +k8s:deepcopy-gen=true
type StudioAgentMessageRequest struct {
	// Instruction 是本次应用开发指令，不得包含模型凭证。
	Instruction string `json:"instruction" binding:"required,min=1,max=20000"`
	// IdempotencyKey 在当前 Coding Agent 范围内防止重复创建 Invocation。
	IdempotencyKey string `json:"idempotency_key" binding:"required,min=1,max=200"`
	// Attachments 只接受契约允许的稳定资源引用。
	Attachments []StudioAgentAttachment `json:"attachments,omitempty" binding:"omitempty,max=50,dive"`
}

// StudioAgentMessageListRequest 查询应用当前 Coding Agent generation/session 的消息历史。
// +k8s:deepcopy-gen=true
type StudioAgentMessageListRequest struct {
	imachinery.BasicQueryParam
}

// SetDefaults 对齐 Studio Agent 消息历史契约的默认分页大小 50。
func (r *StudioAgentMessageListRequest) SetDefaults() {
	if r.PageSize == 0 {
		r.PageSize = 50
	}
}

// Validate 限制消息历史每页最多返回 200 项。
func (r *StudioAgentMessageListRequest) Validate() error {
	if r.PageSize > 200 {
		return fmt.Errorf("page_size must be less than or equal to 200")
	}
	_, err := r.PagingParams.Normalize()
	return err
}

// StudioAgentReplaceRequest 原子切换应用当前 Coding Agent generation，旧历史保持可审计。
// +k8s:deepcopy-gen=true
type StudioAgentReplaceRequest struct {
	// IdempotencyKey 确保同一次替换不会创建多个 Agent generation。
	IdempotencyKey string `json:"idempotency_key" binding:"required,min=1,max=200"`
	// CodingAgentProfile 可选替换 Runtime profile；为空时沿用当前 profile。
	CodingAgentProfile string `json:"coding_agent_profile,omitempty" binding:"omitempty,max=200"`
	// CodingModelSelection 可选替换模型引用；为空时沿用当前 ModelBinding 来源。
	CodingModelSelection string `json:"coding_model_selection,omitempty" binding:"omitempty,max=500"`
}

// +k8s:deepcopy-gen=true
type StudioApplicationUpdateRequest struct {
	Name            *string `json:"name,omitempty" binding:"omitempty,max=200"`
	Description     *string `json:"description,omitempty" binding:"omitempty,max=2000"`
	ResourceVersion int64   `json:"resource_version" binding:"required,min=0"`
}

func (r *StudioApplicationUpdateRequest) Validate() error {
	if r.Name == nil && r.Description == nil {
		return fmt.Errorf("studio application update has no fields")
	}
	return nil
}

// +k8s:deepcopy-gen=true
type StudioSourceFileListRequest struct {
	imachinery.BasicQueryParam
	SourceRevision int64  `form:"source_revision" binding:"min=0"`
	Prefix         string `form:"prefix" binding:"omitempty,max=1024"`
}

// +k8s:deepcopy-gen=true
type StudioFileContentRequest struct {
	Path           string `form:"path" binding:"required,max=1024"`
	SourceRevision int64  `form:"source_revision" binding:"min=0"`
}

// +k8s:deepcopy-gen=true
type StudioChangeSetRequest struct {
	BaseRevision      int64                   `json:"base_revision" binding:"min=0"`
	IdempotencyKey    string                  `json:"idempotency_key" binding:"required,max=200"`
	Operations        []StudioChangeOperation `json:"operations" binding:"required,min=1,max=200,dive"`
	Summary           string                  `json:"summary,omitempty" binding:"omitempty,max=2000"`
	AgentID           string                  `json:"-"`
	AgentSessionID    string                  `json:"-"`
	AgentInvocationID string                  `json:"-"`
}

func (r *StudioChangeSetRequest) Validate() error {
	for _, op := range r.Operations {
		if op.Operation != AppStudioChangeOperationCreate && op.Operation != AppStudioChangeOperationUpdate && op.Operation != AppStudioChangeOperationDelete && op.Operation != AppStudioChangeOperationMove {
			return fmt.Errorf("unsupported change operation")
		}
		if op.Path == "" || strings.HasPrefix(op.Path, "/") || strings.Contains(op.Path, "..") {
			return fmt.Errorf("invalid source path")
		}
		if (op.Operation == AppStudioChangeOperationCreate || op.Operation == AppStudioChangeOperationUpdate) && op.Content == nil {
			return fmt.Errorf("source content is required")
		}
		if op.Operation == AppStudioChangeOperationMove && (op.TargetPath == nil || *op.TargetPath == "") {
			return fmt.Errorf("move target path is required")
		}
	}
	return nil
}

// +k8s:deepcopy-gen=true
type StudioSnapshotRequest struct {
	SourceRevision int64  `json:"source_revision" binding:"min=0"`
	IdempotencyKey string `json:"idempotency_key,omitempty" binding:"omitempty,max=200"`
}

// +k8s:deepcopy-gen=true
type StudioApplicationVersionCreateRequest struct {
	SourceSnapshotID string `json:"source_snapshot_id" binding:"required,max=128"`
	Version          string `json:"version" binding:"required,min=1,max=100"`
	IdempotencyKey   string `json:"idempotency_key" binding:"required,max=200"`
}

// +k8s:deepcopy-gen=true
type StudioBuildRequest struct {
	SourceSnapshotID           string `json:"source_snapshot_id" binding:"required,max=128"`
	IdempotencyKey             string `json:"idempotency_key" binding:"required,max=200"`
	StudioApplicationVersionID string `json:"studio_application_version_id,omitempty" binding:"omitempty,max=128"`
}

// StudioBuildBatchSummaryRequestItem identifies one producer without granting visibility.
type StudioBuildBatchSummaryRequestItem struct {
	ID string `json:"id" binding:"required,min=1,max=128"`
}

// StudioBuildBatchSummaryRequest resolves 1..200 Build producer summaries in request order.
type StudioBuildBatchSummaryRequest struct {
	Items []StudioBuildBatchSummaryRequestItem `json:"items" binding:"required,min=1,max=200,dive"`
}

// +k8s:deepcopy-gen=true
type StudioPreviewRequest struct {
	SourceRevision int64  `json:"source_revision" binding:"min=0"`
	IdempotencyKey string `json:"idempotency_key,omitempty" binding:"omitempty,max=200"`
}

// +k8s:deepcopy-gen=true
type StudioRestoreRevisionRequest struct {
	SourceRevision int64  `json:"source_revision" binding:"min=0"`
	BaseRevision   int64  `json:"base_revision" binding:"min=0"`
	IdempotencyKey string `json:"idempotency_key" binding:"required,max=200"`
}

// +k8s:deepcopy-gen=true
type StudioSourceSearchRequest struct {
	imachinery.BasicQueryParam
	Query          string `form:"query" binding:"required,min=1,max=500"`
	SourceRevision int64  `form:"source_revision" binding:"min=0"`
}

// +k8s:deepcopy-gen=true
type StudioRuntimeConfigRequest struct {
	PublicConfig          json.RawMessage `json:"public_config,omitempty"`
	SecretReferences      []string        `json:"secret_references,omitempty" binding:"omitempty,max=100,dive,max=1024"`
	IntegrationReferences []string        `json:"integration_references,omitempty" binding:"omitempty,max=100,dive,max=1024"`
	ResourceVersion       int64           `json:"resource_version" binding:"min=0"`
}

func (r *StudioRuntimeConfigRequest) Validate() error {
	if len(r.PublicConfig) > 64*1024 {
		return fmt.Errorf("runtime public config exceeds 64 KiB")
	}
	for _, ref := range r.SecretReferences {
		if !strings.HasPrefix(ref, AppStudioRefPrefixSecret) {
			return fmt.Errorf("invalid secret reference")
		}
	}
	for _, ref := range r.IntegrationReferences {
		if !strings.HasPrefix(ref, AppStudioRefPrefixIntegration) {
			return fmt.Errorf("invalid integration reference")
		}
	}
	return nil
}

// +k8s:deepcopy-gen=true
type StudioReleaseRequest struct {
	StudioBuildID              string `json:"studio_build_id" binding:"required,max=128"`
	StudioApplicationVersionID string `json:"studio_application_version_id" binding:"required,max=128"`
	Environment                string `json:"environment" binding:"required,oneof=preview production"`
	RuntimeConfigID            string `json:"runtime_config_id" binding:"required,max=128"`
	IdempotencyKey             string `json:"idempotency_key" binding:"required,max=200"`
}

// +k8s:deepcopy-gen=true
type StudioActionRequest struct {
	Reason          string `json:"reason,omitempty" binding:"omitempty,max=1000"`
	RequestID       string `json:"request_id,omitempty" binding:"omitempty,max=200"`
	ResourceVersion int64  `json:"resource_version" binding:"min=0"`
}

// +k8s:deepcopy-gen=true
type StudioBuildListRequest struct {
	imachinery.BasicQueryParam
	StudioApplicationID string `form:"-" json:"-"`
}

// +k8s:deepcopy-gen=true
type StudioReleaseListRequest struct {
	imachinery.BasicQueryParam
	StudioApplicationID string `form:"-" json:"-"`
}

// +k8s:deepcopy-gen=true
type StudioApplicationVersionListRequest struct {
	imachinery.BasicQueryParam
	StudioApplicationID string `form:"-" json:"-"`
}

// +k8s:deepcopy-gen=true
type StudioRuntimeInstanceListRequest struct {
	imachinery.BasicQueryParam
	StudioApplicationID string `form:"-" json:"-"`
	Environment         string `form:"environment" binding:"omitempty,oneof=preview production"`
	Status              string `form:"status" binding:"omitempty,oneof=CREATING READY DEGRADED STOPPED FAILED"`
}
