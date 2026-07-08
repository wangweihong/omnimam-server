package taskexecutor

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

func TestThumbnailExecutorImageSuccess(t *testing.T) {
	root := t.TempDir()
	srcKey := filepath.ToSlash(filepath.Join("assets", "image.png"))
	srcPath := filepath.Join(root, filepath.FromSlash(srcKey))
	if err := os.MkdirAll(filepath.Dir(srcPath), 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeTestPNG(t, srcPath, 640, 320)

	thumbnail := &iapiserver.AssetThumbnail{AssetID: "asset-1", StorageBackendID: "local", Status: iapiserver.ThumbnailStatusPending}
	factory := &fakeFactory{
		assets: &fakeAssetStore{item: &iapiserver.Asset{
			MediaType:        iapiserver.AssetMediaTypeImage,
			StorageBackendID: "local",
			ObjectKey:        srcKey,
		}},
		thumbnails: &fakeThumbnailStore{item: thumbnail},
		storage:    &fakeStorageBackendStore{item: &iapiserver.StorageBackend{Type: iapiserver.StorageBackendTypeLocal, Root: root}},
	}
	factory.assets.item.ID = "asset-1"
	factory.thumbnails.item.ID = "thumbnail-1"

	output, err := NewThumbnailExecutor(factory).Execute(context.Background(), &iapiserver.TaskRun{
		Input: map[string]any{"asset_id": "asset-1", "thumbnail_id": "thumbnail-1"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if thumbnail.Status != iapiserver.ThumbnailStatusReady {
		t.Fatalf("thumbnail status = %s", thumbnail.Status)
	}
	if thumbnail.Width != 320 || thumbnail.Height != 160 {
		t.Fatalf("thumbnail size = %dx%d", thumbnail.Width, thumbnail.Height)
	}
	if output["thumbnail_status"] != iapiserver.ThumbnailStatusReady {
		t.Fatalf("output = %#v", output)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(thumbnail.ObjectKey))); err != nil {
		t.Fatalf("thumbnail file: %v", err)
	}
}

func TestThumbnailExecutorUnsupportedMedia(t *testing.T) {
	thumbnail := &iapiserver.AssetThumbnail{AssetID: "asset-1", StorageBackendID: "local", Status: iapiserver.ThumbnailStatusPending}
	factory := &fakeFactory{
		assets: &fakeAssetStore{item: &iapiserver.Asset{
			MediaType:        iapiserver.AssetMediaTypeAudio,
			StorageBackendID: "local",
			ObjectKey:        "assets/audio.mp3",
		}},
		thumbnails: &fakeThumbnailStore{item: thumbnail},
	}
	factory.assets.item.ID = "asset-1"
	factory.thumbnails.item.ID = "thumbnail-1"

	output, err := NewThumbnailExecutor(factory).Execute(context.Background(), &iapiserver.TaskRun{
		Input: map[string]any{"asset_id": "asset-1", "thumbnail_id": "thumbnail-1"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if thumbnail.Status != iapiserver.ThumbnailStatusUnsupported {
		t.Fatalf("thumbnail status = %s", thumbnail.Status)
	}
	if output["thumbnail_status"] != iapiserver.ThumbnailStatusUnsupported {
		t.Fatalf("output = %#v", output)
	}
}

type fakeFactory struct {
	store.Factory
	assets     *fakeAssetStore
	thumbnails *fakeThumbnailStore
	storage    *fakeStorageBackendStore
}

func (f *fakeFactory) AssetsV2() store.AssetStore                 { return f.assets }
func (f *fakeFactory) AssetThumbnails() store.AssetThumbnailStore { return f.thumbnails }
func (f *fakeFactory) StorageBackends() store.StorageBackendStore { return f.storage }

type fakeAssetStore struct {
	store.AssetStore
	item *iapiserver.Asset
}

func (s *fakeAssetStore) Get(context.Context, string) (*iapiserver.Asset, error) { return s.item, nil }

type fakeThumbnailStore struct {
	store.AssetThumbnailStore
	item *iapiserver.AssetThumbnail
}

func (s *fakeThumbnailStore) GetByAsset(context.Context, string) (*iapiserver.AssetThumbnail, error) {
	return s.item, nil
}

func (s *fakeThumbnailStore) Update(
	_ context.Context,
	data *iapiserver.AssetThumbnail,
) (*iapiserver.AssetThumbnail, error) {
	s.item = data
	return data, nil
}

type fakeStorageBackendStore struct {
	store.StorageBackendStore
	item *iapiserver.StorageBackend
}

func (s *fakeStorageBackendStore) Get(context.Context, string) (*iapiserver.StorageBackend, error) {
	return s.item, nil
}

func writeTestPNG(t *testing.T, path string, width int, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 80, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
}
