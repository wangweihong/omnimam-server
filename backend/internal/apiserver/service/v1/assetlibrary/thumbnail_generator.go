package assetlibrary

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

// ThumbnailRequest 是 generator 接收的受控源内容和固定 profile 参数。
type ThumbnailRequest struct {
	Source    io.Reader
	MediaType string
	MIMEType  string
	SizeBytes int64
	MaxSide   int
}

// ThumbnailResult 是可安全写入 Blob 的小型派生图像。
type ThumbnailResult struct {
	Content  []byte
	MIMEType string
	Format   string
	Width    int
	Height   int
}

// ThumbnailGenerator 隔离不同媒体类型的缩略图生成策略。
type ThumbnailGenerator interface {
	Generate(context.Context, ThumbnailRequest) (ThumbnailResult, error)
}

// ThumbnailGenerators 是只读媒体类型路由表；注册在 TaskWorker composition root 完成。
type ThumbnailGenerators struct {
	items map[string]ThumbnailGenerator
}

func NewThumbnailGenerators(imageGenerator, videoGenerator ThumbnailGenerator) *ThumbnailGenerators {
	items := make(map[string]ThumbnailGenerator, 2)
	if imageGenerator != nil {
		items[iapiserver.AssetMediaTypeImage] = imageGenerator
	}
	if videoGenerator != nil {
		items[iapiserver.AssetMediaTypeVideo] = videoGenerator
	}
	return &ThumbnailGenerators{items: items}
}

func (g *ThumbnailGenerators) Generate(ctx context.Context, req ThumbnailRequest) (ThumbnailResult, error) {
	if g == nil {
		return ThumbnailResult{}, errors.Errorf("thumbnail generator registry is unavailable")
	}
	generator := g.items[req.MediaType]
	if generator == nil {
		return ThumbnailResult{}, errors.Errorf("thumbnail generation is unsupported for media type %s", req.MediaType)
	}
	return generator.Generate(ctx, req)
}

// ImageThumbnailGenerator 使用 Go 图像解码器生成 PNG，不依赖外部 runtime。
type ImageThumbnailGenerator struct{}

func (ImageThumbnailGenerator) Generate(ctx context.Context, req ThumbnailRequest) (ThumbnailResult, error) {
	if err := ctx.Err(); err != nil {
		return ThumbnailResult{}, err
	}
	src, _, err := image.Decode(req.Source)
	if err != nil {
		return ThumbnailResult{}, errors.WithStack(err)
	}
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return ThumbnailResult{}, errors.Errorf("invalid image dimensions")
	}
	maxSide := req.MaxSide
	if maxSide <= 0 {
		maxSide = 320
	}
	if width > maxSide || height > maxSide {
		if width >= height {
			height = max(1, maxSide*height/width)
			width = maxSide
		} else {
			width = max(1, maxSide*width/height)
			height = maxSide
		}
	}
	thumbnail := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		if err := ctx.Err(); err != nil {
			return ThumbnailResult{}, err
		}
		for x := 0; x < width; x++ {
			thumbnail.Set(x, y, src.At(bounds.Min.X+x*bounds.Dx()/width, bounds.Min.Y+y*bounds.Dy()/height))
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, thumbnail); err != nil {
		return ThumbnailResult{}, errors.WithStack(err)
	}
	return ThumbnailResult{Content: encoded.Bytes(), MIMEType: "image/png", Format: "png", Width: width, Height: height}, nil
}

// VideoThumbnailGenerator 将视频取帧委托给可替换 FFmpegRuntime。
type VideoThumbnailGenerator struct {
	runtime FFmpegRuntime
}

func NewVideoThumbnailGenerator(runtime FFmpegRuntime) *VideoThumbnailGenerator {
	return &VideoThumbnailGenerator{runtime: runtime}
}

func (g *VideoThumbnailGenerator) Generate(ctx context.Context, req ThumbnailRequest) (ThumbnailResult, error) {
	if g == nil || g.runtime == nil {
		return ThumbnailResult{}, errors.Errorf("ffmpeg runtime is unavailable")
	}
	result, err := g.runtime.ExtractFrame(ctx, VideoFrameRequest{
		Source: req.Source, MIMEType: req.MIMEType, SizeBytes: req.SizeBytes,
		Selection: VideoFrameSelectionRepresentative, MaxSide: req.MaxSide, OutputFormat: "png",
	})
	if err != nil {
		return ThumbnailResult{}, err
	}
	if result.MIMEType != "image/png" || result.Format != "png" || result.Width <= 0 || result.Height <= 0 || len(result.Content) == 0 {
		return ThumbnailResult{}, errors.Errorf("ffmpeg runtime returned an invalid thumbnail")
	}
	return ThumbnailResult{Content: result.Content, MIMEType: result.MIMEType, Format: result.Format, Width: result.Width, Height: result.Height}, nil
}
