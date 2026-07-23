package assetlibrary

import (
	"bytes"
	"context"
	"fmt"
	_ "image/gif"
	_ "image/jpeg"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
)

const (
	FunctionArtifactProcess        = "asset-library.artifact.process"
	FunctionRepresentationInspect  = "asset-library.representation.inspect"
	FunctionRepresentationGenerate = "asset-library.representation.generate"
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
		task.Log(ctx, workflowruntime.WorkerLog("artifact.process.ready", workflowruntime.TaskLogLevelInfo, "Artifact was already ready."))
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
	task.Log(ctx, workflowruntime.WorkerLog("artifact.process.completed", workflowruntime.TaskLogLevelInfo, "Artifact processing completed."))
	return map[string]any{"artifact_id": updated.ID}, nil
}

// RepresentationFinalizeExecutor 汇总当前 Representation 事实，不从 AtomicTask 终态推断素材状态。
type RepresentationFinalizeExecutor struct{ store store.AssetV1Store }

// RepresentationInspectExecutor 校验版本事实，并从 original 内容探测和持久化媒体元数据。
type RepresentationInspectExecutor struct {
	store     store.AssetV1Store
	storage   ContentStorage
	inspector MediaMetadataInspector
}

func NewRepresentationInspectExecutor(factory store.Factory, storage ContentStorage, inspector MediaMetadataInspector) *RepresentationInspectExecutor {
	return &RepresentationInspectExecutor{store: factory.AssetsV1(), storage: storage, inspector: inspector}
}

func (e *RepresentationInspectExecutor) Execute(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
	versionID, _ := task.Arguments["asset_version_id"].(string)
	owner, _ := task.Arguments["owner_user_id"].(string)
	if versionID == "" || owner == "" {
		return nil, errors.Errorf("representation inspect task requires asset_version_id and owner_user_id")
	}
	detail, err := e.store.GetAssetVersionDetail(ctx, owner, versionID)
	if err != nil {
		return nil, err
	}
	mediaType, _ := task.Arguments["media_type"].(string)
	if mediaType == iapiserver.AssetMediaTypeImage || mediaType == iapiserver.AssetMediaTypeVideo || mediaType == iapiserver.AssetMediaTypeAudio {
		if err := e.inspectOriginalMedia(ctx, owner, mediaType, detail); err != nil {
			return nil, err
		}
	}
	task.Log(ctx, workflowruntime.WorkerLog("representation.inspect.completed", workflowruntime.TaskLogLevelInfo, "AssetVersion representation requirements were inspected."))
	return map[string]any{"asset_version_id": versionID, "media_type": mediaType}, nil
}

func (e *RepresentationInspectExecutor) inspectOriginalMedia(ctx context.Context, owner, mediaType string, detail *iapiserver.AssetVersionDetail) error {
	if e == nil || e.storage == nil || e.inspector == nil || detail == nil || detail.Version == nil {
		return errors.Errorf("representation media inspection dependencies are unavailable")
	}
	var original *iapiserver.AssetRepresentation
	for _, item := range detail.Representations {
		if item != nil && item.RepresentationType == iapiserver.AssetRepresentationOriginal && item.Status == "ready" {
			original = item
			break
		}
	}
	if original == nil {
		return errors.Errorf("original representation is unavailable for media inspection")
	}
	_, content, err := e.store.GetRepresentation(ctx, owner, original.ID)
	if err != nil {
		return err
	}
	if content == nil {
		return errors.Errorf("original representation content is unavailable for media inspection")
	}
	reader, err := e.storage.Open(ctx, *content)
	if err != nil {
		return err
	}
	metadata, inspectErr := e.inspector.Inspect(ctx, MediaMetadataRequest{
		Source: reader, MediaType: mediaType, MIMEType: content.MIMEType, SizeBytes: content.SizeBytes,
	})
	closeErr := reader.Close()
	if inspectErr != nil {
		return inspectErr
	}
	if closeErr != nil {
		return errors.WithStack(closeErr)
	}
	return e.store.ApplyAssetMediaMetadata(ctx, owner, detail.Version.ID, store.AssetMediaMetadataMutation{
		AssetID:                  detail.Version.AssetID,
		OriginalRepresentationID: original.ID,
		MIMEType:                 content.MIMEType,
		SizeBytes:                content.SizeBytes,
		Width:                    metadata.Width,
		Height:                   metadata.Height,
		DurationSeconds:          metadata.DurationSeconds,
	})
}

// RepresentationGenerateExecutor 从 original Blob 生成图片缩略图并幂等登记 Representation。
type RepresentationGenerateExecutor struct {
	store      store.AssetV1Store
	storage    ContentStorage
	generators *ThumbnailGenerators
}

func NewRepresentationGenerateExecutor(factory store.Factory, storage ContentStorage, generators *ThumbnailGenerators) *RepresentationGenerateExecutor {
	return &RepresentationGenerateExecutor{store: factory.AssetsV1(), storage: storage, generators: generators}
}

func (e *RepresentationGenerateExecutor) Execute(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
	result, err := e.generate(ctx, task)
	if err == nil {
		task.Log(ctx, workflowruntime.WorkerLog("representation.generate.completed", workflowruntime.TaskLogLevelInfo, "Derived representation was generated and registered."))
		return result, nil
	}
	required, _ := task.Arguments["required"].(bool)
	maxAttempts := intArgument(task.Arguments["max_attempts"], 3)
	if task.RetryCount+1 < maxAttempts {
		return nil, err
	}
	task.Log(ctx, workflowruntime.WorkerLog("representation.generate.optional_failed", workflowruntime.TaskLogLevelWarn, "Optional representation generation failed and was recorded."))
	versionID, _ := task.Arguments["asset_version_id"].(string)
	owner, _ := task.Arguments["owner_user_id"].(string)
	typeName, _ := task.Arguments["representation_type"].(string)
	profile, _ := task.Arguments["profile"].(string)
	profileVersion, _ := task.Arguments["profile_version"].(string)
	failed, registerErr := e.store.CompleteRepresentationGeneration(ctx, owner, versionID, store.RepresentationGenerationMutation{
		Type: typeName, Profile: profile, ProfileVersion: profileVersion,
		Status: "failed", Required: required, RetryCount: task.RetryCount + 1, ErrorDetail: "thumbnail generation failed",
	})
	if registerErr != nil {
		return nil, registerErr
	}
	if required {
		return nil, err
	}
	return map[string]any{"asset_version_id": versionID, "representation_id": failed.ID, "status": "failed"}, nil
}

func (e *RepresentationGenerateExecutor) generate(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
	versionID, _ := task.Arguments["asset_version_id"].(string)
	owner, _ := task.Arguments["owner_user_id"].(string)
	typeName, _ := task.Arguments["representation_type"].(string)
	profile, _ := task.Arguments["profile"].(string)
	profileVersion, _ := task.Arguments["profile_version"].(string)
	mediaType, _ := task.Arguments["media_type"].(string)
	if versionID == "" || owner == "" || typeName != "thumbnail" || profile == "" || profileVersion == "" || mediaType == "" {
		return nil, errors.Errorf("representation generate task arguments are invalid")
	}
	detail, err := e.store.GetAssetVersionDetail(ctx, owner, versionID)
	if err != nil {
		return nil, err
	}
	var original *iapiserver.AssetRepresentation
	for _, item := range detail.Representations {
		if item.RepresentationType == iapiserver.AssetRepresentationOriginal && item.Status == "ready" {
			original = item
			break
		}
	}
	if original == nil {
		return nil, errors.Errorf("original representation is unavailable")
	}
	_, content, err := e.store.GetRepresentation(ctx, owner, original.ID)
	if err != nil || content == nil {
		return nil, errors.Errorf("original representation content is unavailable")
	}
	reader, err := e.storage.Open(ctx, *content)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	thumbnail, err := e.generators.Generate(ctx, ThumbnailRequest{Source: reader, MediaType: mediaType, MIMEType: content.MIMEType, SizeBytes: content.SizeBytes, MaxSide: 320})
	if err != nil {
		return nil, err
	}
	generated, err := e.storage.WriteDerived(ctx, thumbnail.MIMEType, bytes.NewReader(thumbnail.Content))
	if err != nil {
		return nil, err
	}
	blobID, err := e.store.CreateRepresentationBlob(ctx, generated)
	if err != nil {
		return nil, err
	}
	registered, err := e.store.CompleteRepresentationGeneration(ctx, owner, versionID, store.RepresentationGenerationMutation{
		Type: typeName, Profile: profile, ProfileVersion: profileVersion, BlobID: blobID,
		Metadata: map[string]any{"width": thumbnail.Width, "height": thumbnail.Height, "mime_type": thumbnail.MIMEType, "format": thumbnail.Format, "size_bytes": generated.SizeBytes},
		Status:   "ready", Required: false, RetryCount: task.RetryCount,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"asset_version_id": versionID, "representation_id": registered.ID, "blob_id": blobID}, nil
}

func intArgument(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return typed
		}
	case int64:
		if typed > 0 {
			return int(typed)
		}
	case float64:
		if typed > 0 {
			return int(typed)
		}
	}
	return fallback
}

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
	task.Log(ctx, workflowruntime.WorkerLog("representation.finalize.completed", workflowruntime.TaskLogLevelInfo, fmt.Sprintf("Representation build finalized with %d completed and %d failed outputs.", completed, failed)))
	return map[string]any{"asset_version_id": updated.ID, "status": updated.Status, "summary": fmt.Sprintf("%d/%d", completed, expected)}, nil
}
