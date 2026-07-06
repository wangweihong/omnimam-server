package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	stderrors "errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type Processor struct {
	store store.Factory
}

type RunOptions struct {
	Queue        string
	WorkerID     string
	Limit        int
	Lease        time.Duration
	PollInterval time.Duration
}

func NewProcessor() *Processor {
	return &Processor{store: store.Client()}
}

func (p *Processor) Run(ctx context.Context, opts RunOptions) error {
	if p.store == nil {
		return errors.Errorf("store client is not initialized")
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = 3 * time.Second
	}
	if opts.Lease <= 0 {
		opts.Lease = 5 * time.Minute
	}
	if opts.Limit <= 0 {
		opts.Limit = 1
	}
	ticker := time.NewTicker(opts.PollInterval)
	defer ticker.Stop()
	for {
		if err := p.runOnce(ctx, opts); err != nil {
			log.Errorf("worker run failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (p *Processor) runOnce(ctx context.Context, opts RunOptions) error {
	tasks, err := p.store.Tasks().Claim(ctx, opts.Queue, opts.WorkerID, opts.Limit, opts.Lease)
	if err != nil {
		return errors.WithStack(err)
	}
	for _, task := range tasks {
		if err := p.process(ctx, task); err != nil {
			if task.Attempts < task.MaxAttempts {
				task.Status = iapiserver.TaskStatusPending
				task.Progress = 0
			} else {
				task.Status = iapiserver.TaskStatusFailed
				task.Progress = 100
			}
			task.Error = err.Error()
			task.LockOwner = ""
			task.LockedUntil = time.Time{}
			_, _ = p.store.Tasks().Update(ctx, task)
			continue
		}
		task.Status = iapiserver.TaskStatusSucceeded
		task.Progress = 100
		task.Error = ""
		task.LockOwner = ""
		task.LockedUntil = time.Time{}
		if task.Output == nil {
			task.Output = map[string]any{"ok": true}
		}
		_, _ = p.store.Tasks().Update(ctx, task)
	}
	return nil
}

func (p *Processor) process(ctx context.Context, task *iapiserver.Task) error {
	switch task.Type {
	case iapiserver.TaskTypeAssetThumbnail:
		return p.processAssetThumbnail(ctx, task)
	case iapiserver.TaskTypeAssetProbe:
		return p.processAssetProbe(ctx, task)
	case iapiserver.TaskTypeQueryParse:
		task.Output = map[string]any{"message": "query parse fallback completed", "input": task.Input}
		return nil
	case iapiserver.TaskTypeLLMInvoke, iapiserver.TaskTypeAssetTagging:
		task.Output = map[string]any{"message": "task contract accepted; provider execution is adapter-driven"}
		return nil
	default:
		task.Output = map[string]any{"message": "task type accepted", "type": task.Type}
		return nil
	}
}

func (p *Processor) processAssetProbe(ctx context.Context, task *iapiserver.Task) error {
	assetID, _ := task.Input["asset_id"].(string)
	if assetID == "" {
		return errors.Errorf("asset_id is required")
	}
	asset, err := p.store.AssetsV2().Get(ctx, assetID)
	if err != nil {
		return errors.WithStack(err)
	}
	backend, err := p.store.StorageBackends().Get(ctx, asset.StorageBackendID)
	if err != nil {
		return errors.WithStack(err)
	}
	path, err := localObjectPath(backend, asset.ObjectKey)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return errors.WithStack(err)
	}
	mimeType, checksum, err := probeFile(path)
	if err != nil {
		return err
	}
	mediaType, format := mediaTypeFromFile(mimeType, asset.ObjectKey)
	asset.Size = info.Size()
	asset.Checksum = checksum
	asset.MimeType = mimeType
	asset.MediaType = mediaType
	asset.Format = format
	if mediaType == iapiserver.AssetMediaTypeImage {
		asset.Width, asset.Height = imageDimensions(path)
	}
	if asset.Metadata == nil {
		asset.Metadata = map[string]any{}
	}
	asset.Metadata["filename"] = filepath.Base(asset.ObjectKey)
	asset.Metadata["probed_at"] = time.Now().UTC().Format(time.RFC3339)
	if _, err := p.store.AssetsV2().Update(ctx, asset); err != nil {
		return errors.WithStack(err)
	}
	task.Output = map[string]any{
		"asset_id":   asset.ID,
		"media_type": asset.MediaType,
		"mime_type":  asset.MimeType,
		"format":     asset.Format,
		"width":      asset.Width,
		"height":     asset.Height,
		"size":       asset.Size,
	}
	return nil
}

func (p *Processor) processAssetThumbnail(ctx context.Context, task *iapiserver.Task) error {
	assetID, _ := task.Input["asset_id"].(string)
	if assetID == "" {
		return errors.Errorf("asset_id is required")
	}
	asset, err := p.store.AssetsV2().Get(ctx, assetID)
	if err != nil {
		return errors.WithStack(err)
	}
	thumbnail, err := p.store.AssetThumbnails().GetByAsset(ctx, assetID)
	if err != nil {
		return errors.WithStack(err)
	}
	if asset.MediaType != iapiserver.AssetMediaTypeImage && asset.MediaType != iapiserver.AssetMediaTypeVideo {
		thumbnail.Status = iapiserver.ThumbnailStatusUnsupported
		_, _ = p.store.AssetThumbnails().Update(ctx, thumbnail)
		task.Output = map[string]any{"thumbnail_status": thumbnail.Status}
		return nil
	}
	thumbnail.Status = iapiserver.ThumbnailStatusProcessing
	if _, err := p.store.AssetThumbnails().Update(ctx, thumbnail); err != nil {
		return errors.WithStack(err)
	}
	backend, err := p.store.StorageBackends().Get(ctx, asset.StorageBackendID)
	if err != nil {
		return errors.WithStack(err)
	}
	srcPath, err := localObjectPath(backend, asset.ObjectKey)
	if err != nil {
		return err
	}
	thumbKey := filepath.ToSlash(filepath.Join("thumbnails", asset.ID, "thumb.png"))
	dstPath, err := localObjectPath(backend, thumbKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0750); err != nil {
		return errors.WithStack(err)
	}
	var width, height int
	if asset.MediaType == iapiserver.AssetMediaTypeVideo {
		width, height, err = writeVideoThumbnail(srcPath, dstPath)
		if err != nil && stderrors.Is(err, exec.ErrNotFound) {
			thumbnail.Status = iapiserver.ThumbnailStatusUnsupported
			_, _ = p.store.AssetThumbnails().Update(ctx, thumbnail)
			task.Output = map[string]any{"thumbnail_status": thumbnail.Status, "reason": "ffmpeg not found"}
			return nil
		}
	} else {
		width, height, err = writeImageThumbnail(srcPath, dstPath, 320)
	}
	if err != nil {
		thumbnail.Status = iapiserver.ThumbnailStatusFailed
		_, _ = p.store.AssetThumbnails().Update(ctx, thumbnail)
		return err
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
	if _, err := p.store.AssetThumbnails().Update(ctx, thumbnail); err != nil {
		return errors.WithStack(err)
	}
	task.Output = map[string]any{
		"thumbnail_id":     thumbnail.ID,
		"thumbnail_status": thumbnail.Status,
	}
	return nil
}

func probeFile(path string) (string, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", "", errors.WithStack(err)
	}
	defer file.Close()
	head := make([]byte, 512)
	n, err := file.Read(head)
	if err != nil && err != io.EOF {
		return "", "", errors.WithStack(err)
	}
	mimeType := http.DetectContentType(head[:n])
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", "", errors.WithStack(err)
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", "", errors.WithStack(err)
	}
	return mimeType, hex.EncodeToString(hasher.Sum(nil)), nil
}

func mediaTypeFromFile(mimeType string, filename string) (string, string) {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	if parsedExts, err := mime.ExtensionsByType(mimeType); err == nil && ext == "" && len(parsedExts) > 0 {
		ext = strings.TrimPrefix(parsedExts[0], ".")
	}
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return iapiserver.AssetMediaTypeImage, ext
	case strings.HasPrefix(mimeType, "video/"):
		return iapiserver.AssetMediaTypeVideo, ext
	case ext == "mp4" || ext == "webm" || ext == "mov" || ext == "mkv" || ext == "avi":
		return iapiserver.AssetMediaTypeVideo, ext
	case strings.HasPrefix(mimeType, "audio/"):
		return iapiserver.AssetMediaTypeAudio, ext
	case mimeType == "application/pdf" || ext == "pdf":
		return iapiserver.AssetMediaTypePDF, ext
	case ext == "json":
		return iapiserver.AssetMediaTypeJSON, ext
	case ext == "md" || ext == "markdown":
		return iapiserver.AssetMediaTypeMarkdown, ext
	case strings.HasPrefix(mimeType, "text/"):
		return iapiserver.AssetMediaTypeText, ext
	default:
		return iapiserver.AssetMediaTypeOther, ext
	}
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
		if stderrors.Is(err, exec.ErrNotFound) {
			return 0, 0, err
		}
		return 0, 0, errors.Errorf("ffmpeg thumbnail failed: %s", strings.TrimSpace(string(output)))
	}
	width, height := imageDimensions(dstPath)
	if width == 0 || height == 0 {
		return 0, 0, errors.Errorf("video thumbnail has invalid dimensions")
	}
	return width, height, nil
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

func copyFile(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return errors.WithStack(err)
	}
	defer src.Close()
	dst, err := os.Create(dstPath)
	if err != nil {
		return errors.WithStack(err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return errors.WithStack(err)
	}
	return nil
}
