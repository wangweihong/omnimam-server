package assetlibrary

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalFFmpegRuntimeExtractsRepresentativeFrame(t *testing.T) {
	binary, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	temporary := t.TempDir()
	t.Setenv("TMPDIR", temporary)
	videoPath := filepath.Join(temporary, "fixture.mp4")
	create := exec.Command(binary, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "color=c=black:s=640x360:r=25:d=1",
		"-f", "lavfi", "-i", "testsrc=s=640x360:r=25:d=2",
		"-filter_complex", "[0:v][1:v]concat=n=2:v=1:a=0,format=yuv420p",
		"-c:v", "libx264", videoPath)
	if output, err := create.CombinedOutput(); err != nil {
		t.Fatalf("create video fixture: %v: %s", err, output)
	}
	content, err := os.ReadFile(videoPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(videoPath); err != nil {
		t.Fatal(err)
	}
	runtime := newLocalFFmpegRuntime(binary, 30*time.Second, 1)
	result, err := runtime.ExtractFrame(context.Background(), VideoFrameRequest{Source: bytes.NewReader(content), MIMEType: "video/mp4", SizeBytes: int64(len(content)), Selection: VideoFrameSelectionRepresentative, MaxSide: 320, OutputFormat: "png"})
	if err != nil {
		t.Fatal(err)
	}
	image, err := png.Decode(bytes.NewReader(result.Content))
	if err != nil {
		t.Fatal(err)
	}
	if bounds := image.Bounds(); bounds.Dx() != 320 || bounds.Dy() != 180 {
		t.Fatalf("thumbnail bounds = %v", bounds)
	}
	center := image.At(160, 90)
	r, g, b, _ := center.RGBA()
	if r+g+b < 1000 {
		t.Fatalf("representative frame remained black: rgb=%d/%d/%d", r, g, b)
	}
	entries, err := os.ReadDir(temporary)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary files were not removed: %#v", entries)
	}
}

func TestNewLocalFFmpegRuntimeRequiresBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := NewLocalFFmpegRuntime(); err == nil {
		t.Fatal("expected missing ffmpeg error")
	}
}
