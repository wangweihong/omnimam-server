package platform

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

type dispatcherStub struct{}

func (dispatcherStub) DispatchAsync(context.Context, string) {}

func TestNewServiceUsesInjectedDispatcher(t *testing.T) {
	dispatcher := dispatcherStub{}
	service := NewService(&thumbnailExecutorFactory{}, dispatcher)
	if service.dispatcher != dispatcher {
		t.Fatal("platform service did not retain the bootstrap dispatcher")
	}

	serviceWithoutDispatcher := NewService(&thumbnailExecutorFactory{})
	if serviceWithoutDispatcher.dispatcher != nil {
		t.Fatal("platform service constructed an implicit dispatcher")
	}
}

func TestThumbnailExecutorImageSuccess(t *testing.T) {
	root := t.TempDir()
	srcKey := filepath.ToSlash(filepath.Join("assets", "image.png"))
	srcPath := filepath.Join(root, filepath.FromSlash(srcKey))
	if err := os.MkdirAll(filepath.Dir(srcPath), 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeTestPNG(t, srcPath, 640, 320)
	thumbnail := &iapiserver.AssetThumbnail{AssetID: "asset-1", StorageBackendID: "local", Status: iapiserver.ThumbnailStatusPending}
	factory := &thumbnailExecutorFactory{
		assets:     &thumbnailAssetStore{item: &iapiserver.Asset{MediaType: iapiserver.AssetMediaTypeImage, StorageBackendID: "local", ObjectKey: srcKey}},
		thumbnails: &thumbnailStore{item: thumbnail},
		storage:    &thumbnailStorageStore{item: &iapiserver.StorageBackend{Type: iapiserver.StorageBackendTypeLocal, Root: root}},
	}
	factory.assets.item.ID = "asset-1"
	factory.thumbnails.item.ID = "thumbnail-1"
	output, err := NewThumbnailExecutor(factory).Execute(context.Background(), &iapiserver.TaskRun{Input: map[string]any{"asset_id": "asset-1", "thumbnail_id": "thumbnail-1"}})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if thumbnail.Status != iapiserver.ThumbnailStatusReady || thumbnail.Width != 320 || thumbnail.Height != 160 {
		t.Fatalf("thumbnail = status %s, size %dx%d", thumbnail.Status, thumbnail.Width, thumbnail.Height)
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
	factory := &thumbnailExecutorFactory{
		assets:     &thumbnailAssetStore{item: &iapiserver.Asset{MediaType: iapiserver.AssetMediaTypeAudio, StorageBackendID: "local", ObjectKey: "assets/audio.mp3"}},
		thumbnails: &thumbnailStore{item: thumbnail},
	}
	factory.assets.item.ID = "asset-1"
	factory.thumbnails.item.ID = "thumbnail-1"
	output, err := NewThumbnailExecutor(factory).Execute(context.Background(), &iapiserver.TaskRun{Input: map[string]any{"asset_id": "asset-1", "thumbnail_id": "thumbnail-1"}})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if thumbnail.Status != iapiserver.ThumbnailStatusUnsupported || output["thumbnail_status"] != iapiserver.ThumbnailStatusUnsupported {
		t.Fatalf("thumbnail status = %s, output = %#v", thumbnail.Status, output)
	}
}

type thumbnailExecutorFactory struct {
	store.Factory
	assets     *thumbnailAssetStore
	thumbnails *thumbnailStore
	storage    *thumbnailStorageStore
}

func (f *thumbnailExecutorFactory) AssetsV2() store.AssetStore                 { return f.assets }
func (f *thumbnailExecutorFactory) AssetThumbnails() store.AssetThumbnailStore { return f.thumbnails }
func (f *thumbnailExecutorFactory) StorageBackends() store.StorageBackendStore { return f.storage }

type thumbnailAssetStore struct {
	store.AssetStore
	item *iapiserver.Asset
}

func (s *thumbnailAssetStore) Get(context.Context, string) (*iapiserver.Asset, error) {
	return s.item, nil
}

type thumbnailStore struct {
	store.AssetThumbnailStore
	item *iapiserver.AssetThumbnail
}

func (s *thumbnailStore) GetByAsset(context.Context, string) (*iapiserver.AssetThumbnail, error) {
	return s.item, nil
}
func (s *thumbnailStore) Update(_ context.Context, data *iapiserver.AssetThumbnail) (*iapiserver.AssetThumbnail, error) {
	s.item = data
	return data, nil
}

type thumbnailStorageStore struct {
	store.StorageBackendStore
	item *iapiserver.StorageBackend
}

func (s *thumbnailStorageStore) Get(context.Context, string) (*iapiserver.StorageBackend, error) {
	return s.item, nil
}

func writeTestPNG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
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
