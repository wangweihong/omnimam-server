package applicationplatform

import (
	"context"
	"testing"
)

func TestApplicationArtifactProjectorAppliesAssetLibraryFact(t *testing.T) {
	applicationStore := &executorApplicationStore{}
	projector := NewApplicationArtifactProjector(applicationStore)
	err := projector.Project(context.Background(), []byte(`{
		"artifact_id":"artifact-1",
		"application_run_id":"run-1",
		"output_key":"images",
		"sequence":2,
		"media_type":"image",
		"processing_status":"ready",
		"registration_status":"registered",
		"asset_id":"asset-1",
		"asset_version_id":"version-1",
		"resource_version":7
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if applicationStore.ref == nil || applicationStore.ref.ArtifactID != "artifact-1" ||
		applicationStore.ref.ApplicationRunID != "run-1" || applicationStore.ref.Sequence != 2 ||
		applicationStore.ref.ArtifactResourceVersion != 7 ||
		applicationStore.ref.AssetVersionID == nil || *applicationStore.ref.AssetVersionID != "version-1" {
		t.Fatalf("unexpected projected reference: %#v", applicationStore.ref)
	}
}

func TestApplicationArtifactProjectorIgnoresNonApplicationArtifact(t *testing.T) {
	applicationStore := &executorApplicationStore{}
	projector := NewApplicationArtifactProjector(applicationStore)
	if err := projector.Project(context.Background(), []byte(`{
		"artifact_id":"artifact-1",
		"application_run_id":null,
		"output_key":"images",
		"media_type":"image",
		"processing_status":"ready",
		"registration_status":"pending",
		"resource_version":1
	}`)); err != nil {
		t.Fatal(err)
	}
	if applicationStore.ref != nil {
		t.Fatalf("non-application artifact was projected: %#v", applicationStore.ref)
	}
}
