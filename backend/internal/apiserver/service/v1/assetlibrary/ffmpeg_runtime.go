package assetlibrary

import (
	"bytes"
	"context"
	"image/png"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
)

const (
	VideoFrameSelectionRepresentative = "representative"
	defaultFFmpegTimeout              = 30 * time.Second
	defaultFFmpegConcurrency          = 2
	maxFFmpegOutputBytes              = 4 << 20
	maxFFmpegStderrBytes              = 64 << 10
)

// VideoFrameRequest 描述受控视频输入和固定取帧约束，不暴露命令行参数。
type VideoFrameRequest struct {
	Source       io.Reader
	MIMEType     string
	SizeBytes    int64
	Selection    string
	MaxSide      int
	OutputFormat string
}

// VideoFrameResult 只返回派生帧和有限媒体事实。
type VideoFrameResult struct {
	Content  []byte
	MIMEType string
	Format   string
	Width    int
	Height   int
}

// FFmpegRuntime 是视频 generator 消费的可替换执行边界。
type FFmpegRuntime interface {
	ExtractFrame(context.Context, VideoFrameRequest) (VideoFrameResult, error)
}

// LocalFFmpegRuntime 使用本地 CLI 实现 FFmpegRuntime；未来可替换为 remote/sidecar adapter。
type LocalFFmpegRuntime struct {
	binary    string
	timeout   time.Duration
	semaphore chan struct{}
}

func NewLocalFFmpegRuntime() (*LocalFFmpegRuntime, error) {
	binary, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, errors.Wrap(err, "locate ffmpeg runtime")
	}
	return newLocalFFmpegRuntime(binary, defaultFFmpegTimeout, defaultFFmpegConcurrency), nil
}

func newLocalFFmpegRuntime(binary string, timeout time.Duration, concurrency int) *LocalFFmpegRuntime {
	if timeout <= 0 {
		timeout = defaultFFmpegTimeout
	}
	if concurrency <= 0 {
		concurrency = defaultFFmpegConcurrency
	}
	return &LocalFFmpegRuntime{binary: binary, timeout: timeout, semaphore: make(chan struct{}, concurrency)}
}

func (r *LocalFFmpegRuntime) ExtractFrame(ctx context.Context, req VideoFrameRequest) (VideoFrameResult, error) {
	if r == nil || r.binary == "" || req.Source == nil || req.Selection != VideoFrameSelectionRepresentative || req.OutputFormat != "png" {
		return VideoFrameResult{}, errors.Errorf("ffmpeg frame request is invalid")
	}
	select {
	case r.semaphore <- struct{}{}:
		defer func() { <-r.semaphore }()
	case <-ctx.Done():
		return VideoFrameResult{}, ctx.Err()
	}

	input, err := os.CreateTemp("", "omnimam-video-*")
	if err != nil {
		return VideoFrameResult{}, errors.WithStack(err)
	}
	inputPath := input.Name()
	defer func() { _ = os.Remove(inputPath) }()
	limit := req.SizeBytes
	if limit <= 0 {
		limit = 1 << 30
	}
	written, copyErr := copyWithContext(ctx, input, io.LimitReader(req.Source, limit+1))
	closeErr := input.Close()
	if copyErr != nil {
		return VideoFrameResult{}, copyErr
	}
	if closeErr != nil {
		return VideoFrameResult{}, errors.WithStack(closeErr)
	}
	if written > limit || (req.SizeBytes > 0 && written != req.SizeBytes) {
		return VideoFrameResult{}, errors.Errorf("video source size does not match the verified blob")
	}

	maxSide := req.MaxSide
	if maxSide <= 0 {
		maxSide = 320
	}
	runCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	filter := "thumbnail=100,scale='min(" + strconv.Itoa(maxSide) + ",iw)':'min(" + strconv.Itoa(maxSide) + ",ih)':force_original_aspect_ratio=decrease"
	cmd := exec.CommandContext(runCtx, r.binary,
		"-hide_banner", "-loglevel", "error", "-nostdin", "-i", inputPath,
		"-frames:v", "1", "-vf", filter, "-f", "image2pipe", "-vcodec", "png", "pipe:1",
	)
	stdout := &limitedBuffer{limit: maxFFmpegOutputBytes}
	stderr := &truncatingBuffer{limit: maxFFmpegStderrBytes}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		if runCtx.Err() != nil {
			return VideoFrameResult{}, runCtx.Err()
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = "ffmpeg frame extraction failed"
		}
		return VideoFrameResult{}, errors.Errorf("ffmpeg frame extraction failed: %s", detail)
	}
	config, err := png.DecodeConfig(bytes.NewReader(stdout.Bytes()))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > maxSide || config.Height > maxSide {
		return VideoFrameResult{}, errors.Errorf("ffmpeg returned an invalid png frame")
	}
	return VideoFrameResult{Content: stdout.Bytes(), MIMEType: "image/png", Format: "png", Width: config.Width, Height: config.Height}, nil
}

type limitedBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.buf.Len()+len(p) > b.limit {
		return 0, errors.Errorf("ffmpeg output exceeds limit")
	}
	return b.buf.Write(p)
}
func (b *limitedBuffer) Bytes() []byte { return b.buf.Bytes() }

type truncatingBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (b *truncatingBuffer) Write(p []byte) (int, error) {
	original := len(p)
	remaining := b.limit - b.buf.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.buf.Write(p)
	}
	return original, nil
}
func (b *truncatingBuffer) String() string { return b.buf.String() }

var _ FFmpegRuntime = (*LocalFFmpegRuntime)(nil)
