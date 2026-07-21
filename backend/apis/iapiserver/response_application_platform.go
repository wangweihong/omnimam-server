package iapiserver

import "github.com/wangweihong/omnimam/backend/apis/imachinery"

// ApplicationArtifactRefResponse 是 ApplicationRun 内嵌的 Artifact 只读投影。
// 它提供运行详情展示和 Asset 导航所需字段，不包含 Artifact 正文或受保护内容。
type ApplicationArtifactRefResponse struct {
	ID                         string          `json:"id"`
	ApplicationRunID           string          `json:"application_run_id"`
	ArtifactID                 string          `json:"artifact_id"`
	OutputKey                  string          `json:"output_key"`
	Sequence                   int             `json:"sequence"`
	MediaType                  string          `json:"media_type"`
	ArtifactProcessingStatus   string          `json:"artifact_processing_status"`
	ArtifactRegistrationStatus string          `json:"artifact_registration_status"`
	AssetID                    *string         `json:"asset_id,omitempty"`
	AssetVersionID             *string         `json:"asset_version_id,omitempty"`
	ArtifactResourceVersion    int64           `json:"artifact_resource_version"`
	LastErrorCode              *string         `json:"last_error_code,omitempty"`
	CreatedAt                  imachinery.Time `json:"created_at"`
	UpdatedAt                  imachinery.Time `json:"updated_at"`
	ResourceVersion            int64           `json:"resource_version"`
}

// ApplicationRunResponse 严格投影 SSOT ApplicationRun，并替换旧持久化 Artifact 行。
type ApplicationRunResponse struct {
	*ApplicationRun
	Artifacts []*ApplicationArtifactRefResponse `json:"artifacts"`
}

// Transform 在公共 Controller 写响应前转换为已发布契约，内部 Worker 仍使用持久化模型。
func (r *ApplicationRun) Transform() any {
	response := &ApplicationRunResponse{ApplicationRun: r, Artifacts: make([]*ApplicationArtifactRefResponse, 0, len(r.Artifacts))}
	for _, artifact := range r.Artifacts {
		if artifact == nil {
			continue
		}
		processingStatus := ArtifactProcessingCreated
		switch artifact.RegistrationStatus {
		case ArtifactRegistrationRegistered:
			processingStatus = ArtifactProcessingReady
		case ArtifactRegistrationFailed:
			processingStatus = ArtifactProcessingFailed
		}
		var lastErrorCode *string
		if artifact.RegistrationErrorCode != "" {
			lastErrorCode = &artifact.RegistrationErrorCode
		}
		response.Artifacts = append(response.Artifacts, &ApplicationArtifactRefResponse{
			ID: artifact.ID, ApplicationRunID: artifact.ApplicationRunID, ArtifactID: artifact.ID,
			OutputKey: artifact.OutputKey, Sequence: 0, MediaType: artifact.MediaType,
			ArtifactProcessingStatus: processingStatus, ArtifactRegistrationStatus: artifact.RegistrationStatus,
			AssetID: artifact.AssetID, AssetVersionID: artifact.AssetID,
			ArtifactResourceVersion: artifact.ResourceVersion, LastErrorCode: lastErrorCode,
			CreatedAt: artifact.CreatedAt, UpdatedAt: artifact.UpdatedAt, ResourceVersion: artifact.ResourceVersion,
		})
	}
	return response
}
