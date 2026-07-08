package taskexecutor

import (
	"context"
	stderrors "errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

const (
	FunctionAssetThumbnailGenerate = "asset.thumbnail.generate"
	CapabilityAssetThumbnail       = "asset.thumbnail"
	AssetThumbnailDefinitionID     = "asset-thumbnail-generate"

	internalThumbnailWorkerID = "api-internal-thumbnail-worker"
)

type TaskFunctionExecutor interface {
	Execute(ctx context.Context, run *iapiserver.TaskRun) (map[string]any, error)
}

type Dispatcher struct {
	store     store.Factory
	executors map[string]TaskFunctionExecutor
}

func NewDispatcher(store store.Factory) *Dispatcher {
	dispatcher := &Dispatcher{store: store, executors: map[string]TaskFunctionExecutor{}}
	dispatcher.Register(FunctionAssetThumbnailGenerate, NewThumbnailExecutor(store))
	return dispatcher
}

func (d *Dispatcher) Register(functionRef string, executor TaskFunctionExecutor) {
	if d.executors == nil {
		d.executors = map[string]TaskFunctionExecutor{}
	}
	d.executors[functionRef] = executor
}

// DispatchAsync schedules one TaskRun for API-local execution while keeping Task Center as the state machine owner.
func (d *Dispatcher) DispatchAsync(ctx context.Context, runID string) {
	if d == nil || d.store == nil || runID == "" {
		return
	}
	runCtx := context.WithoutCancel(ctx)
	go func() {
		if err := d.Dispatch(runCtx, runID); err != nil {
			log.Errorf("task run dispatch failed: run_id=%s error=%v", runID, err)
		}
	}()
}

// Dispatch claims and executes TaskRuns through the Task Center worker protocol.
func (d *Dispatcher) Dispatch(ctx context.Context, runID string) error {
	if err := d.ensureWorker(ctx); err != nil {
		return err
	}
	for i := 0; i < 10; i++ {
		claim, err := d.store.TaskCenters().ClaimRun(ctx, &iapiserver.ClaimTaskRunRequest{
			WorkerID:     internalThumbnailWorkerID,
			Capabilities: CapabilityAssetThumbnail,
			MaxCount:     1,
		})
		if err != nil {
			return errors.WithStack(err)
		}
		if claim.TaskRun == nil {
			return nil
		}
		if err := d.executeClaim(ctx, claim); err != nil {
			return err
		}
		if claim.TaskRun.ID == runID {
			return nil
		}
	}
	return errors.Errorf("task run %s was not claimed by internal dispatcher", runID)
}

func (d *Dispatcher) ensureWorker(ctx context.Context) error {
	_, err := d.store.TaskCenters().HeartbeatWorker(ctx, &iapiserver.WorkerHeartbeatRequest{
		WorkerID:     internalThumbnailWorkerID,
		Status:       iapiserver.WorkerStatusOnline,
		RunningCount: 0,
	})
	if err == nil {
		return nil
	}
	worker := &iapiserver.Worker{
		WorkerType:     "api-internal-thumbnail",
		Status:         iapiserver.WorkerStatusOnline,
		Capabilities:   CapabilityAssetThumbnail,
		MaxConcurrency: 1,
	}
	worker.ID = internalThumbnailWorkerID
	worker.Name = "api-internal-thumbnail"
	if _, registerErr := d.store.TaskCenters().RegisterWorker(ctx, worker); registerErr != nil {
		_, heartbeatErr := d.store.TaskCenters().HeartbeatWorker(ctx, &iapiserver.WorkerHeartbeatRequest{
			WorkerID:     internalThumbnailWorkerID,
			Status:       iapiserver.WorkerStatusOnline,
			RunningCount: 0,
		})
		return errors.WithStack(heartbeatErr)
	}
	return nil
}

func (d *Dispatcher) executeClaim(ctx context.Context, claim *iapiserver.ClaimTaskRunResponse) error {
	run := claim.TaskRun
	definition, err := d.store.TaskCenters().GetDefinition(ctx, run.DefinitionType, run.DefinitionID)
	if err != nil {
		return errors.WithStack(err)
	}
	executor := d.executors[definition.FunctionRef]
	if executor == nil {
		err = errors.Errorf("task function %s has no internal executor", definition.FunctionRef)
		return d.failRun(ctx, claim, err)
	}
	_, _ = d.store.TaskCenters().UpdateProgress(ctx, &iapiserver.ProgressUpdateRequest{
		RunID:         run.ID,
		AttemptID:     claim.Attempt.ID,
		LeaseID:       claim.Lease.ID,
		WorkerID:      internalThumbnailWorkerID,
		Progress:      0.1,
		ExternalJobID: "api-internal",
	})
	output, err := executor.Execute(ctx, run)
	if err != nil {
		return d.failRun(ctx, claim, err)
	}
	_, err = d.store.TaskCenters().CompleteRun(ctx, &iapiserver.TaskRunCompleteRequest{
		RunID:         run.ID,
		AttemptID:     claim.Attempt.ID,
		LeaseID:       claim.Lease.ID,
		WorkerID:      internalThumbnailWorkerID,
		Output:        output,
		ExternalJobID: "api-internal",
	})
	return errors.WithStack(err)
}

func (d *Dispatcher) failRun(ctx context.Context, claim *iapiserver.ClaimTaskRunResponse, cause error) error {
	_, err := d.store.TaskCenters().FailRun(ctx, &iapiserver.TaskRunFailRequest{
		RunID:     claim.TaskRun.ID,
		AttemptID: claim.Attempt.ID,
		LeaseID:   claim.Lease.ID,
		WorkerID:  internalThumbnailWorkerID,
		Error: iapiserver.TaskError{
			Code:        "asset_thumbnail_generate_failed",
			Message:     cause.Error(),
			FailureType: iapiserver.FailureTypeFunctionError,
			Retryable:   false,
			OccurredAt:  imachinery.NewTime(time.Now()),
		},
	})
	if err != nil {
		return errors.WithStack(err)
	}
	return cause
}

type ThumbnailExecutor struct {
	store store.Factory
}

func NewThumbnailExecutor(store store.Factory) *ThumbnailExecutor {
	return &ThumbnailExecutor{store: store}
}

// Execute generates image/video thumbnails and stores only thumbnail object references in TaskRun output.
func (e *ThumbnailExecutor) Execute(ctx context.Context, run *iapiserver.TaskRun) (map[string]any, error) {
	assetID, _ := run.Input["asset_id"].(string)
	if assetID == "" {
		return nil, errors.Errorf("asset_id is required")
	}
	asset, err := e.store.AssetsV2().Get(ctx, assetID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	thumbnail, err := e.thumbnail(ctx, run, assetID)
	if err != nil {
		return nil, err
	}
	if asset.MediaType != iapiserver.AssetMediaTypeImage && asset.MediaType != iapiserver.AssetMediaTypeVideo {
		thumbnail.Status = iapiserver.ThumbnailStatusUnsupported
		if _, err := e.store.AssetThumbnails().Update(ctx, thumbnail); err != nil {
			return nil, errors.WithStack(err)
		}
		return map[string]any{"thumbnail_id": thumbnail.ID, "thumbnail_status": thumbnail.Status}, nil
	}
	thumbnail.Status = iapiserver.ThumbnailStatusProcessing
	if _, err := e.store.AssetThumbnails().Update(ctx, thumbnail); err != nil {
		return nil, errors.WithStack(err)
	}
	backend, err := e.store.StorageBackends().Get(ctx, asset.StorageBackendID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	srcPath, err := localObjectPath(backend, asset.ObjectKey)
	if err != nil {
		return nil, err
	}
	thumbKey := filepath.ToSlash(filepath.Join("thumbnails", asset.ID, "thumb.png"))
	dstPath, err := localObjectPath(backend, thumbKey)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0750); err != nil {
		return nil, errors.WithStack(err)
	}
	var width, height int
	if asset.MediaType == iapiserver.AssetMediaTypeVideo {
		width, height, err = writeVideoThumbnail(srcPath, dstPath)
		if err != nil && stderrors.Is(err, exec.ErrNotFound) {
			thumbnail.Status = iapiserver.ThumbnailStatusUnsupported
			if _, updateErr := e.store.AssetThumbnails().Update(ctx, thumbnail); updateErr != nil {
				return nil, errors.WithStack(updateErr)
			}
			return map[string]any{
				"thumbnail_id":     thumbnail.ID,
				"thumbnail_status": thumbnail.Status,
				"reason":           "ffmpeg not found",
			}, nil
		}
	} else {
		width, height, err = writeImageThumbnail(srcPath, dstPath, 320)
	}
	if err != nil {
		thumbnail.Status = iapiserver.ThumbnailStatusFailed
		_, _ = e.store.AssetThumbnails().Update(ctx, thumbnail)
		return nil, err
	}
	stat, _ := os.Stat(dstPath)
	thumbnail.ObjectKey = thumbKey
	thumbnail.MimeType = "image/png"
	thumbnail.Width = width
	thumbnail.Height = height
	thumbnail.Status = iapiserver.ThumbnailStatusReady
	if stat != nil {
		thumbnail.Size = stat.Size()
	}
	if _, err := e.store.AssetThumbnails().Update(ctx, thumbnail); err != nil {
		return nil, errors.WithStack(err)
	}
	return map[string]any{
		"thumbnail_id":     thumbnail.ID,
		"thumbnail_status": thumbnail.Status,
		"object_key":       thumbnail.ObjectKey,
		"width":            thumbnail.Width,
		"height":           thumbnail.Height,
		"mime_type":        thumbnail.MimeType,
		"size":             thumbnail.Size,
	}, nil
}

func (e *ThumbnailExecutor) thumbnail(
	ctx context.Context,
	run *iapiserver.TaskRun,
	assetID string,
) (*iapiserver.AssetThumbnail, error) {
	thumbnailID, _ := run.Input["thumbnail_id"].(string)
	thumbnail, err := e.store.AssetThumbnails().GetByAsset(ctx, assetID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if thumbnailID != "" && thumbnail.ID != thumbnailID {
		return nil, errors.Errorf("thumbnail_id does not match asset")
	}
	return thumbnail, nil
}

func localObjectPath(backend *iapiserver.StorageBackend, objectKey string) (string, error) {
	if backend.Type != iapiserver.StorageBackendTypeLocal {
		return "", errors.Errorf("storage backend %s is not local", backend.ID)
	}
	root := backend.Root
	if root == "" {
		root = os.Getenv("OMNIMAM_STORAGE_ROOT")
	}
	if root == "" {
		root = filepath.Join("data", "assets")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", errors.WithStack(err)
	}
	cleanKey := filepath.Clean(filepath.FromSlash(objectKey))
	if filepath.IsAbs(cleanKey) || cleanKey == ".." || strings.HasPrefix(cleanKey, ".."+string(filepath.Separator)) {
		return "", errors.Errorf("invalid object key")
	}
	path := filepath.Join(root, cleanKey)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", errors.WithStack(err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.Errorf("object key escapes storage root")
	}
	return path, nil
}

func imageDimensions(path string) (int, int) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer file.Close()
	cfg, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

func writeImageThumbnail(srcPath, dstPath string, maxSide int) (int, int, error) {
	src, err := os.Open(srcPath)
	if err != nil {
		return 0, 0, errors.WithStack(err)
	}
	defer src.Close()
	img, _, err := image.Decode(src)
	if err != nil {
		return 0, 0, errors.WithStack(err)
	}
	bounds := img.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return 0, 0, errors.Errorf("invalid image dimensions")
	}
	dstW, dstH := srcW, srcH
	if maxSide > 0 && (srcW > maxSide || srcH > maxSide) {
		if srcW >= srcH {
			dstW = maxSide
			dstH = maxSide * srcH / srcW
		} else {
			dstH = maxSide
			dstW = maxSide * srcW / srcH
		}
		if dstW == 0 {
			dstW = 1
		}
		if dstH == 0 {
			dstH = 1
		}
	}
	thumb := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		for x := 0; x < dstW; x++ {
			srcX := bounds.Min.X + x*srcW/dstW
			srcY := bounds.Min.Y + y*srcH/dstH
			thumb.Set(x, y, img.At(srcX, srcY))
		}
	}
	dst, err := os.Create(dstPath)
	if err != nil {
		return 0, 0, errors.WithStack(err)
	}
	defer dst.Close()
	if err := png.Encode(dst, thumb); err != nil {
		return 0, 0, errors.WithStack(err)
	}
	return dstW, dstH, nil
}

func writeVideoThumbnail(srcPath, dstPath string) (int, int, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return 0, 0, err
	}
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-y",
		"-ss", "0.1",
		"-i", srcPath,
		"-frames:v", "1",
		"-vf", "scale='min(320,iw)':-2",
		dstPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return 0, 0, errors.Errorf("ffmpeg thumbnail failed: %s", strings.TrimSpace(string(output)))
	}
	width, height := imageDimensions(dstPath)
	if width == 0 || height == 0 {
		return 0, 0, errors.Errorf("video thumbnail has invalid dimensions")
	}
	return width, height, nil
}
