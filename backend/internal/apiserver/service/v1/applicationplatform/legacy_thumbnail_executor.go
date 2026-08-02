package applicationplatform

import (
	"context"
	stderrors "errors"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

const (
	FunctionAssetThumbnailGenerate = "asset.thumbnail.generate"
	CapabilityAssetThumbnail       = "asset.thumbnail"
	AssetThumbnailDefinitionID     = "asset-thumbnail-generate"
)

// ThumbnailExecutor 实现素材库缩略图处理语义，不负责 Task Center 状态流转。
type ThumbnailExecutor struct {
	store store.Factory
}

// NewThumbnailExecutor 创建素材缩略图执行器，具体任务生命周期由共享 Dispatcher 管理。
func NewThumbnailExecutor(store store.Factory) *ThumbnailExecutor {
	return &ThumbnailExecutor{store: store}
}

// Execute 生成图片或视频缩略图，AtomicTask 输出只保存派生对象引用和轻量 metadata。
func (e *ThumbnailExecutor) Execute(ctx context.Context, run *iapiserver.AtomicTask) (map[string]any, error) {
	assetID, _ := run.Arguments["asset_id"].(string)
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
				"thumbnail_id": thumbnail.ID, "thumbnail_status": thumbnail.Status, "reason": "ffmpeg not found",
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
		"thumbnail_id": thumbnail.ID, "thumbnail_status": thumbnail.Status, "object_key": thumbnail.ObjectKey,
		"width": thumbnail.Width, "height": thumbnail.Height, "mime_type": thumbnail.MimeType, "size": thumbnail.Size,
	}, nil
}

func (e *ThumbnailExecutor) thumbnail(ctx context.Context, run *iapiserver.AtomicTask, assetID string) (*iapiserver.AssetThumbnail, error) {
	thumbnailID, _ := run.Arguments["thumbnail_id"].(string)
	thumbnail, err := e.store.AssetThumbnails().GetByAsset(ctx, assetID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if thumbnailID != "" && thumbnail.ID != thumbnailID {
		return nil, errors.Errorf("thumbnail_id does not match asset")
	}
	return thumbnail, nil
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
			dstW, dstH = maxSide, maxSide*srcH/srcW
		} else {
			dstW, dstH = maxSide*srcW/srcH, maxSide
		}
		dstW = max(dstW, 1)
		dstH = max(dstH, 1)
	}
	thumb := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := range dstH {
		for x := range dstW {
			thumb.Set(x, y, img.At(bounds.Min.X+x*srcW/dstW, bounds.Min.Y+y*srcH/dstH))
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
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-ss", "0.1", "-i", srcPath,
		"-frames:v", "1", "-vf", "scale='min(320,iw)':-2", dstPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return 0, 0, errors.Errorf("ffmpeg thumbnail failed: %s", strings.TrimSpace(string(output)))
	}
	width, height := imageDimensions(dstPath)
	if width == 0 || height == 0 {
		return 0, 0, errors.Errorf("video thumbnail has invalid dimensions")
	}
	return width, height, nil
}
