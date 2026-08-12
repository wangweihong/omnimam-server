package gitlab

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type sourceProviderStoreStub struct {
	store.GitLabStore
	project *iapiserver.GitLabProject
}

func (s *sourceProviderStoreStub) GetGitLabProjectByExternalID(context.Context, int64) (*iapiserver.GitLabProject, error) {
	return s.project, nil
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

func sha256Digest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "sha256:" + hex.EncodeToString(sum[:])
}
