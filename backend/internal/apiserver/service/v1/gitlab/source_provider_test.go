package gitlab

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	toolboxerrors "github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type sourceProviderStoreStub struct {
	store.GitLabStore
	project *iapiserver.GitLabProject
}

func (s *sourceProviderStoreStub) GetGitLabProjectByExternalID(context.Context, int64) (*iapiserver.GitLabProject, error) {
	return s.project, nil
}

func (s *sourceProviderStoreStub) GetGitLabProject(context.Context, string) (*iapiserver.GitLabProject, error) {
	return nil, gorm.ErrRecordNotFound
}

func (s *sourceProviderStoreStub) GetDefaultGitLabServer(context.Context) (*iapiserver.GitLabServer, error) {
	return nil, gorm.ErrRecordNotFound
}

func TestEnsureProjectPreservesDefaultServerUnavailable(t *testing.T) {
	provider := &SourceProvider{store: &sourceProviderStoreStub{}}
	_, err := provider.EnsureProject(t.Context(), "project-1", "app", "app", "", map[string][]byte{"README.md": []byte("test")})
	if err == nil {
		t.Fatal("EnsureProject() error = nil")
	}
	if status := toolboxerrors.ToStatus(err); status.Code != code.ErrGitLabAppStudioDefaultServerUnavailable {
		t.Fatalf("EnsureProject() code = %d, want %d", status.Code, code.ErrGitLabAppStudioDefaultServerUnavailable)
	}
}

func TestHTTPClientPreservesGitLabAPIBasePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.Header.Get("PRIVATE-TOKEN"); token != "test-token" {
			t.Fatalf("PRIVATE-TOKEN = %q, want test-token", token)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v4/version":
			_, _ = w.Write([]byte(`{"version":"19.2.1"}`))
		case "/api/v4/users/42":
			_, _ = w.Write([]byte(`{"id":42,"username":"project_bot"}`))
		default:
			t.Fatalf("unexpected request path = %q", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	client, err := NewHTTPClientFactory().NewClient(&iapiserver.GitLabServer{
		APIURL:     server.URL + "/api/v4",
		Credential: "test-token",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	version, err := client.GetVersion(t.Context())
	if err != nil {
		t.Fatalf("GetVersion() error = %v", err)
	}
	if version.Version != "19.2.1" {
		t.Fatalf("GetVersion().Version = %q, want 19.2.1", version.Version)
	}
	user, err := client.(*httpClient).GetUser(t.Context(), 42)
	if err != nil || user.Username != "project_bot" {
		t.Fatalf("GetUser() = %#v, %v", user, err)
	}
}

func TestAuthenticateAppStudioWebhookUsesOnlySHA256Digest(t *testing.T) {
	token := "0123456789abcdefghijklmnopqrstuvwxyz-SECRET"
	digest := sha256Digest(token)
	storage := &sourceProviderStoreStub{project: &iapiserver.GitLabProject{Status: iapiserver.GitLabProjectStatusReady, AppStudioWebhookTokenDigest: digest}}
	storage.project.ID = "project-1"
	provider := &SourceProvider{store: storage}
	projectID, err := provider.AuthenticateAppStudioWebhook(t.Context(), 42, token)
	if err != nil || projectID != "project-1" {
		t.Fatalf("AuthenticateAppStudioWebhook() = %q, %v", projectID, err)
	}
	if strings.Contains(storage.project.AppStudioWebhookTokenDigest, token) {
		t.Fatal("persisted digest contains plaintext token")
	}
	if _, err := provider.AuthenticateAppStudioWebhook(t.Context(), 42, token+"x"); err == nil {
		t.Fatal("AuthenticateAppStudioWebhook() accepted invalid token")
	}
}

func TestValidateAppStudioBundleRejectsTraversal(t *testing.T) {
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	tarWriter := tar.NewWriter(gzipWriter)
	content := []byte("bad")
	if err := tarWriter.WriteHeader(&tar.Header{Name: "../outside", Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := validateAppStudioBundle(compressed.Bytes()); err == nil {
		t.Fatal("validateAppStudioBundle() accepted traversal path")
	}
}

func TestRepositoryArchiveMetadataClassification(t *testing.T) {
	for _, typeflag := range []byte{tar.TypeXHeader, tar.TypeXGlobalHeader, tar.TypeGNULongName, tar.TypeGNULongLink} {
		if !isRepositoryArchiveMetadata(typeflag) {
			t.Fatalf("isRepositoryArchiveMetadata(%q) = false", typeflag)
		}
	}
	if isRepositoryArchiveMetadata(tar.TypeSymlink) {
		t.Fatal("isRepositoryArchiveMetadata() accepted symlink")
	}
}

func TestRepositoryArchiveRootDirectoryClassification(t *testing.T) {
	if !isRepositoryArchiveRootDirectory("", tar.TypeDir) {
		t.Fatal("isRepositoryArchiveRootDirectory() rejected GitLab archive root directory")
	}
	if isRepositoryArchiveRootDirectory("", tar.TypeReg) {
		t.Fatal("isRepositoryArchiveRootDirectory() accepted root-level regular file")
	}
	if isRepositoryArchiveRootDirectory("src", tar.TypeDir) {
		t.Fatal("isRepositoryArchiveRootDirectory() accepted nested directory")
	}
}

func TestRepositoryArchiveDirectoryPathCleaning(t *testing.T) {
	directoryPath := strings.TrimSuffix("src/", "/")
	if clean := path.Clean(directoryPath); clean != directoryPath {
		t.Fatalf("clean directory path = %q, want %q", clean, directoryPath)
	}
	unsafeDirectoryPath := strings.TrimSuffix("src//", "/")
	if clean := path.Clean(unsafeDirectoryPath); clean == unsafeDirectoryPath {
		t.Fatal("directory path normalization accepted repeated separator")
	}
}

func sha256Digest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "sha256:" + hex.EncodeToString(sum[:])
}
