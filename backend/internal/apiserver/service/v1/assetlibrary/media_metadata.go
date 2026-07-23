package assetlibrary

import (
	"context"
	"encoding/json"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

const (
	defaultMediaProbeTimeout     = 30 * time.Second
	defaultMediaProbeConcurrency = 2
	maxMediaProbeSourceBytes     = int64(1 << 30)
	maxMediaProbeOutputBytes     = 1 << 20
)

// MediaMetadataRequest 描述 original Representation 的受控内容及已验证 Blob 摘要。
type MediaMetadataRequest struct {
	Source    io.Reader
	MediaType string
	MIMEType  string
	SizeBytes int64
}

// MediaMetadata 是可持久化到 Asset、AssetVersion 和 original Representation 的有限媒体事实。
type MediaMetadata struct {
	Width           int
	Height          int
	DurationSeconds float64
}

// MediaMetadataInspector 是 representation.inspect 消费的可替换媒体探测边界。
type MediaMetadataInspector interface {
	Inspect(context.Context, MediaMetadataRequest) (MediaMetadata, error)
}

// MediaMetadataInspectors 按媒体类型路由标准图像解码器和外部音视频探测 adapter。
type MediaMetadataInspectors struct {
	image       MediaMetadataInspector
	audiovisual MediaMetadataInspector
}

func NewMediaMetadataInspectors(imageInspector, audiovisualInspector MediaMetadataInspector) *MediaMetadataInspectors {
	return &MediaMetadataInspectors{image: imageInspector, audiovisual: audiovisualInspector}
}

func (i *MediaMetadataInspectors) Inspect(ctx context.Context, req MediaMetadataRequest) (MediaMetadata, error) {
	if i == nil || req.Source == nil {
		return MediaMetadata{}, errors.Errorf("media metadata inspector is unavailable")
	}
	var inspector MediaMetadataInspector
	switch req.MediaType {
	case iapiserver.AssetMediaTypeImage:
		inspector = i.image
		if req.MIMEType != "" && req.MIMEType != "image/gif" && req.MIMEType != "image/jpeg" && req.MIMEType != "image/png" {
			inspector = i.audiovisual
		}
	case iapiserver.AssetMediaTypeVideo, iapiserver.AssetMediaTypeAudio:
		inspector = i.audiovisual
	default:
		return MediaMetadata{}, nil
	}
	if inspector == nil {
		return MediaMetadata{}, errors.Errorf("media metadata inspection is unsupported for media type %s", req.MediaType)
	}
	metadata, err := inspector.Inspect(ctx, req)
	if err != nil {
		return MediaMetadata{}, err
	}
	if err := validateMediaMetadata(req.MediaType, metadata); err != nil {
		return MediaMetadata{}, err
	}
	return metadata, nil
}

// ImageMediaMetadataInspector 使用 Go DecodeConfig 读取图片头，不解码完整像素内容。
type ImageMediaMetadataInspector struct{}

func (ImageMediaMetadataInspector) Inspect(ctx context.Context, req MediaMetadataRequest) (MediaMetadata, error) {
	if err := ctx.Err(); err != nil {
		return MediaMetadata{}, err
	}
	config, _, err := image.DecodeConfig(req.Source)
	if err != nil {
		return MediaMetadata{}, errors.Wrap(err, "decode image metadata")
	}
	return MediaMetadata{Width: config.Width, Height: config.Height}, nil
}

// FFprobeMediaMetadataInspector 使用本地 ffprobe adapter 读取视频和音频元数据。
type FFprobeMediaMetadataInspector struct {
	binary    string
	timeout   time.Duration
	semaphore chan struct{}
}

func NewLocalFFprobeMediaMetadataInspector() (*FFprobeMediaMetadataInspector, error) {
	binary, err := exec.LookPath("ffprobe")
	if err != nil {
		return nil, errors.Wrap(err, "locate ffprobe runtime")
	}
	return newFFprobeMediaMetadataInspector(binary, defaultMediaProbeTimeout, defaultMediaProbeConcurrency), nil
}

func newFFprobeMediaMetadataInspector(binary string, timeout time.Duration, concurrency int) *FFprobeMediaMetadataInspector {
	if timeout <= 0 {
		timeout = defaultMediaProbeTimeout
	}
	if concurrency <= 0 {
		concurrency = defaultMediaProbeConcurrency
	}
	return &FFprobeMediaMetadataInspector{
		binary:    binary,
		timeout:   timeout,
		semaphore: make(chan struct{}, concurrency),
	}
}

func (i *FFprobeMediaMetadataInspector) Inspect(ctx context.Context, req MediaMetadataRequest) (MediaMetadata, error) {
	if i == nil || i.binary == "" || req.Source == nil {
		return MediaMetadata{}, errors.Errorf("ffprobe request is invalid")
	}
	select {
	case i.semaphore <- struct{}{}:
		defer func() { <-i.semaphore }()
	case <-ctx.Done():
		return MediaMetadata{}, ctx.Err()
	}

	inputPath, err := writeMediaProbeSource(ctx, req.Source, req.SizeBytes)
	if err != nil {
		return MediaMetadata{}, err
	}
	defer removeTemporary(inputPath)

	runCtx, cancel := context.WithTimeout(ctx, i.timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, i.binary,
		"-v", "error",
		"-show_entries", "format=duration:stream=codec_type,width,height,duration",
		"-of", "json",
		inputPath,
	)
	stdout := &limitedBuffer{limit: maxMediaProbeOutputBytes}
	stderr := &truncatingBuffer{limit: maxFFmpegStderrBytes}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		if runCtx.Err() != nil {
			return MediaMetadata{}, runCtx.Err()
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = "ffprobe metadata inspection failed"
		}
		return MediaMetadata{}, errors.Errorf("ffprobe metadata inspection failed: %s", detail)
	}
	metadata, err := decodeFFprobeMetadata(stdout.Bytes())
	if err != nil {
		return MediaMetadata{}, err
	}
	return metadata, nil
}

type ffprobeOutput struct {
	Streams []struct {
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		Duration  string `json:"duration"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

func decodeFFprobeMetadata(raw []byte) (MediaMetadata, error) {
	var output ffprobeOutput
	if err := json.Unmarshal(raw, &output); err != nil {
		return MediaMetadata{}, errors.Wrap(err, "decode ffprobe metadata")
	}
	result := MediaMetadata{}
	for _, stream := range output.Streams {
		if stream.CodecType == "video" && result.Width == 0 && result.Height == 0 {
			result.Width, result.Height = stream.Width, stream.Height
		}
		if result.DurationSeconds == 0 {
			result.DurationSeconds = parseProbeDuration(stream.Duration)
		}
	}
	if duration := parseProbeDuration(output.Format.Duration); duration > 0 {
		result.DurationSeconds = duration
	}
	return result, nil
}

func parseProbeDuration(value string) float64 {
	if value == "" || value == "N/A" {
		return 0
	}
	duration, err := strconv.ParseFloat(value, 64)
	if err != nil || duration < 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return 0
	}
	return duration
}

func validateMediaMetadata(mediaType string, metadata MediaMetadata) error {
	if metadata.Width < 0 || metadata.Height < 0 || metadata.DurationSeconds < 0 ||
		math.IsNaN(metadata.DurationSeconds) || math.IsInf(metadata.DurationSeconds, 0) {
		return errors.Errorf("media metadata contains invalid numeric values")
	}
	switch mediaType {
	case iapiserver.AssetMediaTypeImage:
		if metadata.Width == 0 || metadata.Height == 0 {
			return errors.Errorf("image metadata does not contain dimensions")
		}
	case iapiserver.AssetMediaTypeVideo:
		if metadata.Width == 0 || metadata.Height == 0 || metadata.DurationSeconds == 0 {
			return errors.Errorf("video metadata does not contain dimensions and duration")
		}
	case iapiserver.AssetMediaTypeAudio:
		if metadata.DurationSeconds == 0 {
			return errors.Errorf("audio metadata does not contain duration")
		}
	}
	return nil
}

func writeMediaProbeSource(ctx context.Context, source io.Reader, sizeBytes int64) (string, error) {
	input, err := os.CreateTemp("", "omnimam-media-probe-*")
	if err != nil {
		return "", errors.WithStack(err)
	}
	inputPath := input.Name()
	limit := sizeBytes
	if limit <= 0 {
		limit = maxMediaProbeSourceBytes
	}
	written, copyErr := copyWithContext(ctx, input, io.LimitReader(source, limit+1))
	closeErr := input.Close()
	if copyErr != nil {
		removeTemporary(inputPath)
		return "", copyErr
	}
	if closeErr != nil {
		removeTemporary(inputPath)
		return "", errors.WithStack(closeErr)
	}
	if written > limit || (sizeBytes > 0 && written != sizeBytes) {
		removeTemporary(inputPath)
		return "", errors.Errorf("media source size does not match the verified blob")
	}
	return inputPath, nil
}

var (
	_ MediaMetadataInspector = (*MediaMetadataInspectors)(nil)
	_ MediaMetadataInspector = ImageMediaMetadataInspector{}
	_ MediaMetadataInspector = (*FFprobeMediaMetadataInspector)(nil)
)
