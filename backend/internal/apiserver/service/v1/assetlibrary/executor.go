package assetlibrary

import (
	"context"
	"fmt"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
)

const (
	FunctionArtifactProcess        = "asset-library.artifact.process"
	FunctionRepresentationFinalize = "asset-library.representation.finalize"
)

// ArtifactProcessExecutor 校验已完成上传的受控 Blob 摘要，并把 Artifact 推进为 ready。
type ArtifactProcessExecutor struct{ store store.AssetV1Store }

func NewArtifactProcessExecutor(factory store.Factory) *ArtifactProcessExecutor {
	return &ArtifactProcessExecutor{store: factory.AssetsV1()}
}

func (e *ArtifactProcessExecutor) Execute(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
	artifactID, _ := task.Arguments["artifact_id"].(string)
	owner, _ := task.Arguments["owner_user_id"].(string)
	if artifactID == "" || owner == "" {
		return nil, errors.Errorf("artifact process task requires artifact_id and owner_user_id")
	}
	artifact, err := e.store.GetArtifact(ctx, owner, artifactID)
	if err != nil {
		return nil, err
	}
	if artifact.ProcessingStatus == iapiserver.ArtifactProcessingReady {
		return map[string]any{"artifact_id": artifact.ID}, nil
	}
	if artifact.ProcessingStatus != iapiserver.ArtifactProcessingProcessing {
		return nil, errors.Errorf("artifact %s is not processing", artifact.ID)
	}
	readyAt := imachinery.Now()
	updated, err := e.store.UpdateArtifactProcessing(ctx, artifact.ID, owner, artifact.ResourceVersion, store.ArtifactProcessingMutation{ChangeType: "ready", ProcessingStatus: iapiserver.ArtifactProcessingReady, ReadyAt: &readyAt})
	if err != nil {
		return nil, err
	}
	return map[string]any{"artifact_id": updated.ID}, nil
}

// RepresentationFinalizeExecutor 汇总当前 Representation 事实，不从 AtomicTask 终态推断素材状态。
type RepresentationFinalizeExecutor struct{ store store.AssetV1Store }

func NewRepresentationFinalizeExecutor(factory store.Factory) *RepresentationFinalizeExecutor {
	return &RepresentationFinalizeExecutor{store: factory.AssetsV1()}
}

func (e *RepresentationFinalizeExecutor) Execute(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
	versionID, _ := task.Arguments["asset_version_id"].(string)
	owner, _ := task.Arguments["owner_user_id"].(string)
	if versionID == "" || owner == "" {
		return nil, errors.Errorf("representation finalize task requires asset_version_id and owner_user_id")
	}
	detail, err := e.store.GetAssetVersionDetail(ctx, owner, versionID)
	if err != nil {
		return nil, err
	}
	completed, failed, requiredFailed := 0, 0, false
	for _, representation := range detail.Representations {
		switch representation.Status {
		case "ready":
			completed++
		case "failed", "irreparable":
			failed++
			requiredFailed = requiredFailed || representation.Required
		}
	}
	status := iapiserver.AssetVersionStatusReady
	if requiredFailed {
		status = iapiserver.AssetVersionStatusFailed
	} else if failed > 0 {
		status = iapiserver.AssetVersionStatusReadyWithWarnings
	}
	expected := detail.Version.ExpectedCount
	if expected < completed+failed {
		expected = completed + failed
	}
	updated, err := e.store.UpdateAssetVersionProcessing(ctx, versionID, owner, detail.Version.ResourceVersion, store.AssetVersionProcessingMutation{Status: status, ExpectedCount: expected, CompletedCount: completed, FailedCount: failed, AtomicTaskID: task.AtomicTaskID})
	if err != nil {
		return nil, err
	}
	return map[string]any{"asset_version_id": updated.ID, "status": updated.Status, "summary": fmt.Sprintf("%d/%d", completed, expected)}, nil
}
