package assetlibrary

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func TestImageMediaMetadataInspectorReadsDimensions(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 321, 123))
	source.Set(1, 1, color.RGBA{R: 255, A: 255})
	encoders := []struct {
		name     string
		mimeType string
		encode   func(*bytes.Buffer) error
	}{
		{name: "png", mimeType: "image/png", encode: func(output *bytes.Buffer) error {
			return png.Encode(output, source)
		}},
		{name: "jpeg", mimeType: "image/jpeg", encode: func(output *bytes.Buffer) error {
			return jpeg.Encode(output, source, nil)
		}},
		{name: "gif", mimeType: "image/gif", encode: func(output *bytes.Buffer) error {
			return gif.Encode(output, source, nil)
		}},
	}
	for _, tt := range encoders {
		t.Run(tt.name, func(t *testing.T) {
			var encoded bytes.Buffer
			if err := tt.encode(&encoded); err != nil {
				t.Fatal(err)
			}
			inspector := NewMediaMetadataInspectors(ImageMediaMetadataInspector{}, nil)
			metadata, err := inspector.Inspect(context.Background(), MediaMetadataRequest{
				Source: bytes.NewReader(encoded.Bytes()), MediaType: iapiserver.AssetMediaTypeImage,
				MIMEType: tt.mimeType, SizeBytes: int64(encoded.Len()),
			})
			if err != nil {
				t.Fatal(err)
			}
			if metadata.Width != 321 || metadata.Height != 123 {
				t.Fatalf("metadata = %#v", metadata)
			}
		})
	}
}

func TestDecodeFFprobeMetadataUsesVideoDimensionsAndContainerDuration(t *testing.T) {
	metadata, err := decodeFFprobeMetadata([]byte(`{
		"streams": [
			{"codec_type": "video", "width": 1920, "height": 1080, "duration": "12.0"},
			{"codec_type": "audio", "duration": "12.4"}
		],
		"format": {"duration": "12.5"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Width != 1920 || metadata.Height != 1080 || metadata.DurationSeconds != 12.5 {
		t.Fatalf("metadata = %#v", metadata)
	}
}
