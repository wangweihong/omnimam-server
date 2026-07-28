package applicationplatform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

// ApplicationArtifactProjector 消费 Asset Library 领域事件并重建 ApplicationRun Artifact 引用。
type ApplicationArtifactProjector struct {
	store store.ApplicationPlatformStore
}

func NewApplicationArtifactProjector(applicationStore store.ApplicationPlatformStore) *ApplicationArtifactProjector {
	return &ApplicationArtifactProjector{store: applicationStore}
}

// Project 将 Artifact 事实的较新版本写入只读引用投影；正文、metadata 与存储地址不会跨域复制。
func (p *ApplicationArtifactProjector) Project(ctx context.Context, payload []byte) error {
	if p == nil || p.store == nil {
		return fmt.Errorf("application artifact projector store is required")
	}
	var event struct {
		ArtifactID              string  `json:"artifact_id"`
		ApplicationRunID        *string `json:"application_run_id"`
		OutputKey               string  `json:"output_key"`
		Sequence                int     `json:"sequence"`
		MediaType               string  `json:"media_type"`
		ProcessingStatus        string  `json:"processing_status"`
		RegistrationStatus      string  `json:"registration_status"`
		AssetID                 *string `json:"asset_id"`
		AssetVersionID          *string `json:"asset_version_id"`
		ProcessingErrorCode     *string `json:"processing_error_code"`
		RegistrationErrorCode   *string `json:"registration_error_code"`
		LegacyErrorCode         *string `json:"error_code"`
		ArtifactResourceVersion int64   `json:"resource_version"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("decode asset-library artifact event: %w", err)
	}
	if event.ApplicationRunID == nil || *event.ApplicationRunID == "" {
		return nil
	}
	if event.ArtifactID == "" || event.OutputKey == "" || event.MediaType == "" || event.ArtifactResourceVersion < 1 {
		return fmt.Errorf("asset-library artifact event is incomplete")
	}
	lastErrorCode := event.ProcessingErrorCode
	if lastErrorCode == nil || *lastErrorCode == "" {
		lastErrorCode = event.RegistrationErrorCode
	}
	if lastErrorCode == nil || *lastErrorCode == "" {
		lastErrorCode = event.LegacyErrorCode
	}
	_, _, err := p.store.ProjectApplicationArtifactRef(ctx, &iapiserver.ApplicationArtifactRef{
		ApplicationRunID: *event.ApplicationRunID, ArtifactID: event.ArtifactID,
		OutputKey: event.OutputKey, Sequence: event.Sequence, MediaType: event.MediaType,
		ArtifactProcessingStatus: event.ProcessingStatus, ArtifactRegistrationStatus: event.RegistrationStatus,
		AssetID: event.AssetID, AssetVersionID: event.AssetVersionID,
		ArtifactResourceVersion: event.ArtifactResourceVersion, LastErrorCode: lastErrorCode,
	})
	if err != nil {
		return fmt.Errorf("project application artifact reference: %w", err)
	}
	return nil
}
