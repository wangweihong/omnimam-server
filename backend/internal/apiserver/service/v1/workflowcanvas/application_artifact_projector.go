package workflowcanvas

import (
	"context"
	"encoding/json"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

// ApplicationArtifactRefChanged 是 Application Platform 可靠输出引用事件的受控载荷。
type ApplicationArtifactRefChanged struct {
	AtomicTaskID             string `json:"atomic_task_id"`
	OutputKey                string `json:"output_key"`
	Sequence                 int    `json:"sequence"`
	ArtifactID               string `json:"artifact_id"`
	MediaType                string `json:"media_type"`
	ArtifactProcessingStatus string `json:"artifact_processing_status"`
	ArtifactResourceVersion  int64  `json:"artifact_resource_version"`
}

// ApplicationArtifactProjector 将 ApplicationRun Artifact 引用单调投影到既有 Canvas 输出槽位。
type ApplicationArtifactProjector struct {
	store store.WorkflowCanvasStore
}

func NewApplicationArtifactProjector(target store.WorkflowCanvasStore) *ApplicationArtifactProjector {
	return &ApplicationArtifactProjector{store: target}
}

// Project 校验事件身份，并由 Canvas store 在同一事务内更新输出绑定和可靠事件。
func (p *ApplicationArtifactProjector) Project(ctx context.Context, payload []byte) error {
	var event ApplicationArtifactRefChanged
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.Wrap(err, "decode application artifact reference event")
	}
	if event.AtomicTaskID == "" || event.OutputKey == "" || event.Sequence < 0 || event.ArtifactID == "" ||
		event.ArtifactResourceVersion < 1 {
		return errors.Errorf("application artifact reference event is incomplete")
	}
	_, err := p.store.ProjectCanvasApplicationArtifact(ctx, &store.CanvasApplicationArtifactProjection{
		AtomicTaskID:             event.AtomicTaskID,
		OutputKey:                event.OutputKey,
		Sequence:                 event.Sequence,
		ArtifactID:               event.ArtifactID,
		MediaType:                event.MediaType,
		ArtifactProcessingStatus: event.ArtifactProcessingStatus,
		ArtifactResourceVersion:  event.ArtifactResourceVersion,
	})
	return errors.Wrap(err, "project application artifact reference")
}
