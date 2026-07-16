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
	"sort"
	"strings"
	"sync"
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

	internalThumbnailWorkerID      = "api-internal-worker-v3"
	internalWorkerMaxConcurrency   = 64
	internalDispatcherPollInterval = 200 * time.Millisecond
)

type TaskFunctionExecutor interface {
	Execute(ctx context.Context, run *iapiserver.TaskRun) (map[string]any, error)
}

type taskCompletionObserver interface {
	Completed(ctx context.Context, task *iapiserver.TaskRun) error
}

type Dispatcher struct {
	store        store.Factory
	executors    map[string]TaskFunctionExecutor
	capabilities map[string]struct{}
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	lifecycleMu  sync.Mutex
	closing      bool
}

func NewDispatcher(store store.Factory) *Dispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	dispatcher := &Dispatcher{store: store, executors: map[string]TaskFunctionExecutor{}, capabilities: map[string]struct{}{CapabilityAssetThumbnail: {}}, ctx: ctx, cancel: cancel}
	dispatcher.Register(FunctionAssetThumbnailGenerate, NewThumbnailExecutor(store))
	return dispatcher
}

// RegisterCapability registers an executor together with the worker capability required to claim it.
func (d *Dispatcher) RegisterCapability(functionRef, capability string, executor TaskFunctionExecutor) {
	d.Register(functionRef, executor)
	if capability != "" {
		d.capabilities[capability] = struct{}{}
	}
}

func (d *Dispatcher) Register(functionRef string, executor TaskFunctionExecutor) {
	if d.executors == nil {
		d.executors = map[string]TaskFunctionExecutor{}
	}
	d.executors[functionRef] = executor
}

// DispatchAsync schedules one TaskRun for API-local execution while keeping Task Center as the state machine owner.
func (d *Dispatcher) DispatchAsync(_ context.Context, runID string) {
	if d == nil || d.store == nil || runID == "" {
		return
	}
	d.lifecycleMu.Lock()
	if d.closing {
		d.lifecycleMu.Unlock()
		return
	}
	d.wg.Add(1)
	d.lifecycleMu.Unlock()
	go func() {
		defer d.wg.Done()
		if err := d.Dispatch(d.ctx, runID); err != nil && !stderrors.Is(err, context.Canceled) {
			log.Errorf("task run dispatch failed: run_id=%s error=%v", runID, err)
		}
	}()
}

// Close cancels API-local executions and waits for every dispatcher goroutine to exit.
func (d *Dispatcher) Close() {
	if d == nil {
		return
	}
	d.lifecycleMu.Lock()
	if d.closing {
		d.lifecycleMu.Unlock()
		return
	}
	d.closing = true
	d.cancel()
	d.lifecycleMu.Unlock()
	d.wg.Wait()
}

// Dispatch claims and executes TaskRuns through the Task Center worker protocol.
func (d *Dispatcher) Dispatch(ctx context.Context, runID string) error {
	if err := d.ensureWorker(ctx); err != nil {
		return err
	}
	for {
		claim, err := d.store.TaskCenters().ClaimRun(ctx, &iapiserver.ClaimTaskRunRequest{
			WorkerID:     internalThumbnailWorkerID,
			Capabilities: d.workerCapabilities(),
			MaxCount:     1,
		})
		if err != nil {
			return errors.WithStack(err)
		}
		if claim.TaskRun == nil {
			target, getErr := d.store.TaskCenters().GetRun(ctx, runID)
			if getErr != nil {
				return errors.WithStack(getErr)
			}
			if terminalTaskStatus(target.Status) {
				return nil
			}
			timer := time.NewTimer(internalDispatcherPollInterval)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return ctx.Err()
			case <-timer.C:
				continue
			}
		}
		if err := d.executeClaim(ctx, claim); err != nil {
			return err
		}
		if claim.TaskRun.ID == runID {
			return nil
		}
	}
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
		WorkerType:     "api-internal",
		Status:         iapiserver.WorkerStatusOnline,
		Capabilities:   d.workerCapabilities(),
		MaxConcurrency: internalWorkerMaxConcurrency,
	}
	worker.ID = internalThumbnailWorkerID
	worker.Name = "api-internal"
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

func terminalTaskStatus(status string) bool {
	switch status {
	case iapiserver.TaskRunStatusSuccess, iapiserver.TaskRunStatusFailed, iapiserver.TaskRunStatusCanceled,
		iapiserver.TaskRunStatusTimeout, iapiserver.TaskRunStatusLost:
		return true
	default:
		return false
	}
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
		if _, failErr := d.failRun(ctx, claim, err); failErr != nil {
			return failErr
		}
		return err
	}
	_, _ = d.store.TaskCenters().UpdateProgress(ctx, &iapiserver.ProgressUpdateRequest{
		RunID:         run.ID,
		AttemptID:     claim.Attempt.ID,
		LeaseID:       claim.Lease.ID,
		WorkerID:      internalThumbnailWorkerID,
		Progress:      0.1,
		ExternalJobID: "api-internal",
	})
	executeCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go d.renewLeaseAndWatchCancellation(executeCtx, cancel, claim, done)
	output, err := executor.Execute(executeCtx, run)
	cancel()
	<-done
	if err != nil {
		failed, failErr := d.failRun(ctx, claim, err)
		if failErr != nil {
			return failErr
		}
		if observer, ok := executor.(taskCompletionObserver); ok {
			if observerErr := observer.Completed(ctx, failed); observerErr != nil {
				return errors.WithStack(observerErr)
			}
		}
		return err
	}
	completed, err := d.store.TaskCenters().CompleteRun(ctx, &iapiserver.TaskRunCompleteRequest{
		RunID:         run.ID,
		AttemptID:     claim.Attempt.ID,
		LeaseID:       claim.Lease.ID,
		WorkerID:      internalThumbnailWorkerID,
		Output:        output,
		ExternalJobID: "api-internal",
	})
	if err != nil {
		return errors.WithStack(err)
	}
	if observer, ok := executor.(taskCompletionObserver); ok {
		return errors.WithStack(observer.Completed(ctx, completed))
	}
	return nil
}

func (d *Dispatcher) workerCapabilities() string {
	items := make([]string, 0, len(d.capabilities))
	for capability := range d.capabilities {
		items = append(items, capability)
	}
	sort.Strings(items)
	return strings.Join(items, ",")
}

func (d *Dispatcher) renewLeaseAndWatchCancellation(ctx context.Context, cancel context.CancelFunc, claim *iapiserver.ClaimTaskRunResponse, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(iapiserver.DefaultTaskCenterLeaseDuration / 3)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run, err := d.store.TaskCenters().GetRun(ctx, claim.TaskRun.ID)
			if err != nil || run.Status == iapiserver.TaskRunStatusCancelRequested || run.Status == iapiserver.TaskRunStatusCanceled {
				cancel()
				return
			}
			if _, err := d.store.TaskCenters().RenewLease(ctx, &iapiserver.LeaseRenewRequest{LeaseID: claim.Lease.ID, WorkerID: internalThumbnailWorkerID, AttemptID: claim.Attempt.ID, RunID: claim.TaskRun.ID}); err != nil {
				cancel()
				return
			}
		}
	}
}

func (d *Dispatcher) failRun(ctx context.Context, claim *iapiserver.ClaimTaskRunResponse, cause error) (*iapiserver.TaskRun, error) {
	failureType := iapiserver.FailureTypeFunctionError
	errorCode := "asset_thumbnail_generate_failed"
	switch {
	case stderrors.Is(cause, context.Canceled):
		failureType, errorCode = iapiserver.FailureTypeCanceled, "task_execution_canceled"
	case stderrors.Is(cause, context.DeadlineExceeded):
		failureType, errorCode = iapiserver.FailureTypeTimeout, "task_execution_timeout"
	}
	failed, err := d.store.TaskCenters().FailRun(ctx, &iapiserver.TaskRunFailRequest{
		RunID:     claim.TaskRun.ID,
		AttemptID: claim.Attempt.ID,
		LeaseID:   claim.Lease.ID,
		WorkerID:  internalThumbnailWorkerID,
		Error: iapiserver.TaskError{
			Code:        errorCode,
			Message:     cause.Error(),
			FailureType: failureType,
			Retryable:   false,
			OccurredAt:  imachinery.NewTime(time.Now()),
		},
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return failed, nil
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
