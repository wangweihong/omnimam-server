package worker

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWriteImageThumbnailResizesImage(t *testing.T) {
	srcPath := writeTestPNG(t, 640, 320)
	dstPath := filepath.Join(t.TempDir(), "thumb.png")
	width, height, err := writeImageThumbnail(srcPath, dstPath, 320)
	if err != nil {
		t.Fatalf("write thumbnail: %v", err)
	}
	if width != 320 || height != 160 {
		t.Fatalf("thumbnail size = %dx%d", width, height)
	}
	file, err := os.Open(dstPath)
	if err != nil {
		t.Fatalf("open thumbnail: %v", err)
	}
	defer file.Close()
	cfg, err := png.DecodeConfig(file)
	if err != nil {
		t.Fatalf("decode thumbnail: %v", err)
	}
	if cfg.Width != width || cfg.Height != height {
		t.Fatalf("encoded size = %dx%d", cfg.Width, cfg.Height)
	}
}

func TestProbeFileDetectsImageMetadata(t *testing.T) {
	srcPath := writeTestPNG(t, 32, 24)
	mimeType, checksum, err := probeFile(srcPath)
	if err != nil {
		t.Fatalf("probe file: %v", err)
	}
	if mimeType != "image/png" {
		t.Fatalf("mime type = %s", mimeType)
	}
	if checksum == "" {
		t.Fatal("checksum is empty")
	}
	width, height := imageDimensions(srcPath)
	if width != 32 || height != 24 {
		t.Fatalf("image dimensions = %dx%d", width, height)
	}
}

func TestWriteVideoThumbnailExtractsFirstFrame(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	tmp := t.TempDir()
	srcPath := filepath.Join(tmp, "source.mp4")
	dstPath := filepath.Join(tmp, "thumb.png")
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-y",
		"-f", "lavfi",
		"-i", "color=c=blue:s=320x180:d=1",
		"-frames:v", "10",
		srcPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create test video: %v: %s", err, string(output))
	}
	width, height, err := writeVideoThumbnail(srcPath, dstPath)
	if err != nil {
		t.Fatalf("write video thumbnail: %v", err)
	}
	if width <= 0 || height <= 0 {
		t.Fatalf("thumbnail size = %dx%d", width, height)
	}
	file, err := os.Open(dstPath)
	if err != nil {
		t.Fatalf("open thumbnail: %v", err)
	}
	defer file.Close()
	if _, err := png.DecodeConfig(file); err != nil {
		t.Fatalf("decode thumbnail: %v", err)
	}
}

func writeTestPNG(t *testing.T, width, height int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 120, A: 255})
		}
	}
	path := filepath.Join(t.TempDir(), "source.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create source image: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode source image: %v", err)
	}
	return path
}
