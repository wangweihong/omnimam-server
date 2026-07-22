package assetlibrary

import (
	"bytes"
	"context"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type fakeFFmpegRuntime struct {
	request VideoFrameRequest
	result  VideoFrameResult
	err     error
}

func (r *fakeFFmpegRuntime) ExtractFrame(_ context.Context, req VideoFrameRequest) (VideoFrameResult, error) {
	r.request = req
	return r.result, r.err
}

func TestDefaultRepresentationPolicyIncludesVideoThumbnail(t *testing.T) {
	policy := DefaultRepresentationPolicy{}
	tests := []struct {
		name          string
		mediaType     string
		expectedCount int
		requested     int
	}{
		{name: "image", mediaType: iapiserver.AssetMediaTypeImage, expectedCount: 2, requested: 1},
		{name: "video", mediaType: iapiserver.AssetMediaTypeVideo, expectedCount: 2, requested: 1},
		{name: "audio", mediaType: iapiserver.AssetMediaTypeAudio, expectedCount: 1, requested: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := policy.Plan(tt.mediaType, "default-v1")
			if plan.ExpectedCount != tt.expectedCount || len(plan.Requested) != tt.requested || plan.MediaType != tt.mediaType || plan.ProfileVersion != "default-v1" {
				t.Fatalf("plan = %#v", plan)
			}
			if tt.requested == 1 && (plan.Requested[0].Type != "thumbnail" || plan.Requested[0].Profile != thumbnailListProfile || plan.Requested[0].Required) {
				t.Fatalf("requested representation = %#v", plan.Requested[0])
			}
		})
	}
}

func TestVideoThumbnailGeneratorUsesFFmpegRuntime(t *testing.T) {
	runtime := &fakeFFmpegRuntime{result: VideoFrameResult{Content: []byte("png"), MIMEType: "image/png", Format: "png", Width: 320, Height: 180}}
	generator := NewVideoThumbnailGenerator(runtime)
	result, err := generator.Generate(context.Background(), ThumbnailRequest{Source: bytes.NewReader([]byte("video")), MediaType: "video", MIMEType: "video/mp4", SizeBytes: 5, MaxSide: 320})
	if err != nil {
		t.Fatal(err)
	}
	if runtime.request.Selection != VideoFrameSelectionRepresentative || runtime.request.MaxSide != 320 || runtime.request.MIMEType != "video/mp4" || runtime.request.OutputFormat != "png" {
		t.Fatalf("runtime request = %#v", runtime.request)
	}
	if result.Width != 320 || result.Height != 180 || result.MIMEType != "image/png" {
		t.Fatalf("result = %#v", result)
	}
}
