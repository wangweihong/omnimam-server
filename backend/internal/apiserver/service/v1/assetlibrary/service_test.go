package assetlibrary

import (
	"context"
	"testing"

	toolerrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type captureAssetStore struct {
	store.AssetV1Store
	artifact     *iapiserver.Artifact
	summaryOwner string
	summaryIDs   []string
	summaries    map[string]*iapiserver.ArtifactReadableSummary
}

func (s *captureAssetStore) DecorateArtifacts(context.Context, string, []*iapiserver.Artifact) error {
	return nil
}

func (s *captureAssetStore) CreateArtifact(_ context.Context, artifact *iapiserver.Artifact) (*iapiserver.Artifact, bool, error) {
	s.artifact = artifact
	artifact.ID = "artifact-1"
	return artifact, true, nil
}

func (s *captureAssetStore) ResolveArtifactSummaries(_ context.Context, owner string, ids []string) (map[string]*iapiserver.ArtifactReadableSummary, error) {
	s.summaryOwner = owner
	s.summaryIDs = append([]string(nil), ids...)
	return s.summaries, nil
}

func TestCreateArtifactUsesAuthenticatedOwner(t *testing.T) {
	assetStore := &captureAssetStore{}
	service := NewStore(assetStore, nil)
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, &iapiserver.User{})
	ctx.Value(iapiserver.GinContextKeyUser).(*iapiserver.User).ID = "user-1"

	result, err := service.CreateArtifact(ctx, &iapiserver.CreateArtifactRequest{
		ProducerType: "atomic_task", ProducerID: "task-1", ProducerIdempotencyKey: "task-1:images:0",
		OutputKey: "images", ArtifactType: "image", MediaType: "image", SavePolicy: "manual_save",
		ProcessingProfileVersion: "v1", Metadata: map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != "artifact-1" || assetStore.artifact.OwnerUserID != "user-1" {
		t.Fatalf("artifact owner/result = %#v", result)
	}
}

func TestCreateArtifactRejectsForbiddenSourceMetadata(t *testing.T) {
	service := NewStore(&captureAssetStore{}, nil)
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, &iapiserver.User{})
	ctx.Value(iapiserver.GinContextKeyUser).(*iapiserver.User).ID = "user-1"
	_, err := service.CreateArtifact(ctx, &iapiserver.CreateArtifactRequest{
		ProducerType: "atomic_task", ProducerID: "task-1", ProducerIdempotencyKey: "key", OutputKey: "output",
		ArtifactType: "image", MediaType: "image", SavePolicy: "manual_save", ProcessingProfileVersion: "v1",
		Metadata: map[string]any{"provider_url": "http://127.0.0.1/private"},
	})
	if status := toolerrors.ToStatus(err); status.Code != code.ErrArtifactSourceForbidden {
		t.Fatalf("error status = %#v", status)
	}
}

func TestBatchArtifactSummariesPreservesOrderAndTrimsInvisibleItems(t *testing.T) {
	assetStore := &captureAssetStore{summaries: map[string]*iapiserver.ArtifactReadableSummary{
		"artifact-1": {ID: "artifact-1", OutputKey: "image", ArtifactType: "image", MediaType: "image", ProcessingStatus: "ready", RegistrationStatus: "registered", PreviewAvailable: true},
	}}
	service := NewStore(assetStore, nil)
	user := &iapiserver.User{}
	user.ID = "user-1"
	ctx := context.WithValue(context.Background(), iapiserver.GinContextKeyUser, user)

	response, err := service.BatchArtifactSummaries(ctx, &iapiserver.BatchArtifactSummaryRequest{Items: []iapiserver.BatchArtifactSummaryItem{{ID: "artifact-1"}, {ID: "missing"}, {ID: "artifact-1"}}})
	if err != nil {
		t.Fatal(err)
	}
	if assetStore.summaryOwner != "user-1" || len(assetStore.summaryIDs) != 3 {
		t.Fatalf("summary scope = owner:%q ids:%#v", assetStore.summaryOwner, assetStore.summaryIDs)
	}
	if response.Total != 3 || response.Items[0].Artifact == nil || response.Items[1].Artifact != nil || response.Items[2].Artifact == nil {
		t.Fatalf("response = %#v", response)
	}
}

func TestValidateLabelsAndTags(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		tags   []string
		code   int
	}{
		{name: "valid case-sensitive values", labels: map[string]string{"Character.Name": "Alice"}, tags: []string{"Hero"}},
		{name: "label whitespace", labels: map[string]string{"bad key": "value"}, code: code.ErrAssetLabelInvalid},
		{name: "duplicate tag", tags: []string{"Hero", "Hero"}, code: code.ErrAssetTagInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateLabelsAndTags(test.labels, test.tags)
			if test.code == 0 && err != nil {
				t.Fatal(err)
			}
			if test.code != 0 && toolerrors.ToStatus(err).Code != test.code {
				t.Fatalf("error = %#v", toolerrors.ToStatus(err))
			}
		})
	}
}

func TestCurrentUserIsRequired(t *testing.T) {
	_, err := currentUserID(context.Background())
	if status := toolerrors.ToStatus(err); status.Code != code.ErrTokenInvalid {
		t.Fatalf("error status = %#v", status)
	}
}

func TestValidateContentRangeMatchesPartNumber(t *testing.T) {
	if err := validateContentRange("bytes 8-15/20", 8, 20, 2, 8); err != nil {
		t.Fatal(err)
	}
	if err := validateContentRange("bytes 0-7/20", 8, 20, 2, 8); err == nil {
		t.Fatal("range for the wrong part number was accepted")
	}
}
