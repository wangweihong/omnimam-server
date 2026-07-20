package assetlibrary

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type storageFactory struct {
	store.Factory
	backend store.StorageBackendStore
}

func (f storageFactory) StorageBackends() store.StorageBackendStore { return f.backend }

type localBackendStore struct {
	store.StorageBackendStore
	backend *iapiserver.StorageBackend
}

func (s localBackendStore) GetDefaultLocal(context.Context) (*iapiserver.StorageBackend, error) {
	return s.backend, nil
}
func (s localBackendStore) Get(context.Context, string) (*iapiserver.StorageBackend, error) {
	return s.backend, nil
}

func TestLocalContentStorageFinalizesVerifiedUpload(t *testing.T) {
	root := t.TempDir()
	backend := &iapiserver.StorageBackend{Type: "local", Root: root, Enabled: true}
	backend.ID = "local-main"
	storage := NewLocalContentStorage(storageFactory{backend: localBackendStore{backend: backend}})
	upload := &iapiserver.AssetUploadSession{SHA256: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", SizeBytes: 5, MIMEType: "text/plain", UploadMode: "single"}
	upload.ID = "upload-1"
	part, err := storage.WriteUploadPart(context.Background(), upload, 1, strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	upload.UploadedParts = []iapiserver.UploadedPart{part}
	content, err := storage.FinalizeUpload(context.Background(), upload)
	if err != nil {
		t.Fatal(err)
	}
	if content.SHA256 != upload.SHA256 || content.SizeBytes != 5 || content.StorageBackendID != "local-main" {
		t.Fatalf("stored content = %#v", content)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(content.ObjectKey))); err != nil {
		t.Fatal(err)
	}
}

func TestSecureJoinRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	for _, key := range []string{"../secret", "/absolute", ""} {
		if _, err := secureJoin(root, key); err == nil {
			t.Fatalf("secureJoin accepted %q", key)
		}
	}
}
