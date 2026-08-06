package iapiserver

import "encoding/json"

// AppStudioEventMetadata 是所有 AppStudio outbox 事件共享的版本与时间元数据。
type AppStudioEventMetadata struct {
	ResourceVersion int64  `json:"resource_version"`
	OccurredAt      string `json:"occurred_at"`
}

// AppStudioApplicationLifecycleEventPayload 描述 StudioApplication 生命周期状态变化。
type AppStudioApplicationLifecycleEventPayload struct {
	AppStudioEventMetadata
	StudioApplicationID string  `json:"studio_application_id"`
	OwnerUserID         string  `json:"owner_user_id"`
	FromStatus          *string `json:"from_status"`
	ToStatus            string  `json:"to_status"`
}

// AppStudioSourceRevisionEventPayload 描述源码工作区版本推进。
type AppStudioSourceRevisionEventPayload struct {
	AppStudioEventMetadata
	StudioApplicationID string  `json:"studio_application_id"`
	PreviousRevision    *int64  `json:"previous_revision"`
	CurrentRevision     int64   `json:"current_revision"`
	ChangeSetID         *string `json:"change_set_id"`
	AgentID             *string `json:"agent_id"`
	AgentInvocationID   *string `json:"agent_invocation_id"`
}

// AppStudioSourceSnapshotEventPayload 描述可构建源码快照。
type AppStudioSourceSnapshotEventPayload struct {
	AppStudioEventMetadata
	SourceSnapshotID    string `json:"source_snapshot_id"`
	StudioApplicationID string `json:"studio_application_id"`
	SourceRevision      int64  `json:"source_revision"`
	ContentDigest       string `json:"content_digest"`
	ManifestDigest      string `json:"manifest_digest"`
	CreatedBy           string `json:"created_by"`
}

// AppStudioBuildEventPayload 描述构建投影状态及产物变化。
type AppStudioBuildEventPayload struct {
	AppStudioEventMetadata
	StudioBuildID       string  `json:"studio_build_id"`
	StudioApplicationID string  `json:"studio_application_id"`
	SourceSnapshotID    string  `json:"source_snapshot_id"`
	AtomicTaskID        *string `json:"atomic_task_id"`
	ArtifactID          *string `json:"artifact_id"`
	ArtifactDigest      *string `json:"artifact_digest"`
	FromStatus          *string `json:"from_status"`
	ToStatus            string  `json:"to_status"`
	ErrorCode           *string `json:"error_code"`
}

// AppStudioPreviewEventPayload 描述预览 Runtime 状态变化。
type AppStudioPreviewEventPayload struct {
	AppStudioEventMetadata
	PreviewRuntimeID    string          `json:"preview_runtime_id"`
	StudioApplicationID string          `json:"studio_application_id"`
	SourceRevision      int64           `json:"source_revision"`
	FromStatus          *string         `json:"from_status"`
	ToStatus            string          `json:"to_status"`
	DiagnosticsSummary  json.RawMessage `json:"diagnostics_summary"`
	ErrorCode           *string         `json:"error_code"`
}

// AppStudioReleaseEventPayload 描述发布状态及其部署依赖。
type AppStudioReleaseEventPayload struct {
	AppStudioEventMetadata
	StudioReleaseID            string  `json:"studio_release_id"`
	StudioApplicationID        string  `json:"studio_application_id"`
	StudioApplicationVersionID string  `json:"studio_application_version_id"`
	StudioBuildID              string  `json:"studio_build_id"`
	RuntimeConfigID            string  `json:"runtime_config_id"`
	ArtifactID                 string  `json:"artifact_id"`
	ArtifactDigest             string  `json:"artifact_digest"`
	Environment                string  `json:"environment"`
	RuntimeInstanceID          *string `json:"runtime_instance_id"`
	RollbackOfReleaseID        *string `json:"rollback_of_release_id"`
	FromStatus                 *string `json:"from_status"`
	ToStatus                   string  `json:"to_status"`
	ErrorCode                  *string `json:"error_code"`
}

// AppStudioRuntimeEventPayload 描述生产 Runtime 状态与健康度变化。
type AppStudioRuntimeEventPayload struct {
	AppStudioEventMetadata
	RuntimeInstanceID   string  `json:"runtime_instance_id"`
	StudioReleaseID     string  `json:"studio_release_id"`
	StudioApplicationID string  `json:"studio_application_id"`
	Environment         string  `json:"environment"`
	AtomicTaskID        *string `json:"atomic_task_id"`
	InfraRuntimeID      *string `json:"infra_runtime_id"`
	FromStatus          *string `json:"from_status"`
	ToStatus            string  `json:"to_status"`
	HealthStatus        string  `json:"health_status"`
	IsCurrent           bool    `json:"is_current"`
	ErrorCode           *string `json:"error_code"`
}

// +k8s:deepcopy-gen=true
type StudioApplicationListResponse struct {
	Total int64                `json:"total"`
	Items []*StudioApplication `json:"items"`
}

// +k8s:deepcopy-gen=true
type StudioSourceFileListResponse struct {
	Total int64               `json:"total"`
	Items []*StudioSourceFile `json:"items"`
}

// +k8s:deepcopy-gen=true
type StudioFileContent struct {
	Path           string `json:"path"`
	Content        string `json:"content"`
	Truncated      bool   `json:"truncated"`
	SourceRevision int64  `json:"source_revision"`
}

// +k8s:deepcopy-gen=true
type StudioSourceSearchHit struct {
	Path           string `json:"path"`
	LineNumber     int    `json:"line_number"`
	Snippet        string `json:"snippet"`
	SourceRevision int64  `json:"source_revision"`
}

// +k8s:deepcopy-gen=true
type StudioSourceSearchResponse struct {
	Total int64                    `json:"total"`
	Items []*StudioSourceSearchHit `json:"items"`
}

// +k8s:deepcopy-gen=true
type StudioBuildListResponse struct {
	Total int64          `json:"total"`
	Items []*StudioBuild `json:"items"`
}

// StudioBuildProducerProjection is the bounded cross-domain Build identity projection.
type StudioBuildProducerProjection struct {
	ID          string `json:"id"`
	OwnerUserID string `json:"owner_user_id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
}

type StudioBuildBatchSummaryItem struct {
	ID          string                         `json:"id"`
	StudioBuild *StudioBuildProducerProjection `json:"studio_build"`
}

type StudioBuildBatchSummaryResponse struct {
	Total int                            `json:"total"`
	Items []*StudioBuildBatchSummaryItem `json:"items"`
}

// +k8s:deepcopy-gen=true
type StudioReleaseListResponse struct {
	Total int64            `json:"total"`
	Items []*StudioRelease `json:"items"`
}

// +k8s:deepcopy-gen=true
type StudioApplicationVersionListResponse struct {
	Total int64                       `json:"total"`
	Items []*StudioApplicationVersion `json:"items"`
}

// +k8s:deepcopy-gen=true
type StudioRuntimeInstanceListResponse struct {
	Total int64                    `json:"total"`
	Items []*StudioRuntimeInstance `json:"items"`
}

// +k8s:deepcopy-gen=true
type StudioRuntimeLogListResponse struct {
	Total int64                    `json:"total"`
	Items []*StudioRuntimeLogEntry `json:"items"`
}

// +k8s:deepcopy-gen=true
type StudioOperationResult struct {
	Success bool `json:"success"`
}
