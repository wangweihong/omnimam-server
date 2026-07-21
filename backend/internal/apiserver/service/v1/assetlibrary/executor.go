package assetlibrary

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"

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

// RepresentationInspectExecutor 校验版本事实并返回由 asset-library policy 形成的派生计划。
type RepresentationInspectExecutor struct{ store store.AssetV1Store }

func NewRepresentationInspectExecutor(factory store.Factory) *RepresentationInspectExecutor {
	return &RepresentationInspectExecutor{store: factory.AssetsV1()}
}

func (e *RepresentationInspectExecutor) Execute(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
	versionID, _ := task.Arguments["asset_version_id"].(string)
	owner, _ := task.Arguments["owner_user_id"].(string)
	if versionID == "" || owner == "" {
		return nil, errors.Errorf("representation inspect task requires asset_version_id and owner_user_id")
	}
	if _, err := e.store.GetAssetVersionDetail(ctx, owner, versionID); err != nil {
		return nil, err
	}
	mediaType, _ := task.Arguments["media_type"].(string)
	return map[string]any{"asset_version_id": versionID, "media_type": mediaType}, nil
}

// RepresentationGenerateExecutor 从 original Blob 生成图片缩略图并幂等登记 Representation。
type RepresentationGenerateExecutor struct {
	store   store.AssetV1Store
	storage ContentStorage
}

func NewRepresentationGenerateExecutor(factory store.Factory, storage ContentStorage) *RepresentationGenerateExecutor {
	return &RepresentationGenerateExecutor{store: factory.AssetsV1(), storage: storage}
}

func (e *RepresentationGenerateExecutor) Execute(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
	result, err := e.generate(ctx, task)
	if err == nil {
		return result, nil
	}
	required, _ := task.Arguments["required"].(bool)
	if required {
		return nil, err
	}
	versionID, _ := task.Arguments["asset_version_id"].(string)
	owner, _ := task.Arguments["owner_user_id"].(string)
	typeName, _ := task.Arguments["representation_type"].(string)
	profile, _ := task.Arguments["profile"].(string)
	profileVersion, _ := task.Arguments["profile_version"].(string)
	failed, registerErr := e.store.RegisterRepresentation(ctx, owner, versionID, &iapiserver.RegisterRepresentationRequest{
		RepresentationType: typeName, Profile: profile, ProfileVersion: profileVersion,
		Status: "failed", Required: false, ErrorDetail: "thumbnail generation failed",
	})
	if registerErr != nil {
		return nil, registerErr
	}
	return map[string]any{"asset_version_id": versionID, "representation_id": failed.ID, "status": "failed"}, nil
}

func (e *RepresentationGenerateExecutor) generate(ctx context.Context, task workflowruntime.WorkerTask) (map[string]any, error) {
	versionID, _ := task.Arguments["asset_version_id"].(string)
	owner, _ := task.Arguments["owner_user_id"].(string)
	typeName, _ := task.Arguments["representation_type"].(string)
	profile, _ := task.Arguments["profile"].(string)
	profileVersion, _ := task.Arguments["profile_version"].(string)
	if versionID == "" || owner == "" || typeName != "thumbnail" || profile == "" || profileVersion == "" {
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
	src, _, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, errors.Errorf("invalid image dimensions")
	}
	if width > 320 || height > 320 {
		if width >= height {
			height = max(1, 320*height/width)
			width = 320
		} else {
			width = max(1, 320*width/height)
			height = 320
		}
	}
	thumb := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			thumb.Set(x, y, src.At(bounds.Min.X+x*bounds.Dx()/width, bounds.Min.Y+y*bounds.Dy()/height))
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, thumb); err != nil {
		return nil, err
	}
	generated, err := e.storage.WriteDerived(ctx, "image/png", bytes.NewReader(encoded.Bytes()))
	if err != nil {
		return nil, err
	}
	blobID, err := e.store.CreateRepresentationBlob(ctx, generated)
	if err != nil {
		return nil, err
	}
	registered, err := e.store.RegisterRepresentation(ctx, owner, versionID, &iapiserver.RegisterRepresentationRequest{
		RepresentationType: typeName, Profile: profile, ProfileVersion: profileVersion, BlobID: blobID,
		Metadata: map[string]any{"width": width, "height": height, "mime_type": "image/png", "size_bytes": generated.SizeBytes},
		Status:   "ready", Required: false,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"asset_version_id": versionID, "representation_id": registered.ID, "blob_id": blobID}, nil
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
	return map[string]any{"asset_version_id": updated.ID, "status": updated.Status, "summary": fmt.Sprintf("%d/%d", completed, expected)}, nil
}
