package iapiserver

import (
	"encoding/json"
	"testing"
)

func TestApplicationRunTransformProjectsArtifactReferenceContract(t *testing.T) {
	assetID := "asset-1"
	run := &ApplicationRun{Artifacts: []*ApplicationArtifact{{
		ApplicationRunID: "run-1", OutputKey: "image", MediaType: "image",
		RegistrationStatus: ArtifactRegistrationRegistered, AssetID: &assetID,
	}}}
	run.Artifacts[0].ID = "artifact-1"
	run.Artifacts[0].ResourceVersion = 3

	response, ok := run.Transform().(*ApplicationRunResponse)
	if !ok || len(response.Artifacts) != 1 {
		t.Fatalf("unexpected transformed response: %#v", response)
	}
	artifact := response.Artifacts[0]
	if artifact.ArtifactID != "artifact-1" || artifact.Sequence != 0 || artifact.ArtifactProcessingStatus != ArtifactProcessingReady {
		t.Fatalf("unexpected artifact projection: %#v", artifact)
	}
	if artifact.AssetID == nil || *artifact.AssetID != assetID || artifact.AssetVersionID == nil || *artifact.AssetVersionID != assetID {
		t.Fatalf("asset navigation projection is incomplete: %#v", artifact)
	}
	if artifact.ArtifactResourceVersion != 3 || artifact.ResourceVersion != 3 {
		t.Fatalf("resource versions are not preserved: %#v", artifact)
	}
}

func TestApplicationRunResponseJSONUsesPublicArtifactProjection(t *testing.T) {
	run := &ApplicationRun{Artifacts: []*ApplicationArtifact{{ApplicationRunID: "run-1", OutputKey: "image", MediaType: "image"}}}
	run.Artifacts[0].ID = "artifact-1"
	data, err := json.Marshal(run.Transform())
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	var artifacts []map[string]any
	if err := json.Unmarshal(payload["artifacts"], &artifacts); err != nil {
		t.Fatalf("unmarshal artifacts: %v", err)
	}
	if len(artifacts) != 1 || artifacts[0]["artifact_id"] != "artifact-1" || artifacts[0]["output_key"] != "image" {
		t.Fatalf("public artifacts = %#v", artifacts)
	}
	if _, leaked := artifacts[0]["registration_status"]; leaked {
		t.Fatalf("persistence artifact shape leaked: %#v", artifacts[0])
	}
}

func TestApplicationRunListResponseTransformsEmptyArtifactsToArray(t *testing.T) {
	data, err := json.Marshal((&ApplicationRunListResponse{Total: 1, Items: []*ApplicationRun{{}}}).Transform())
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Items []struct {
			Artifacts []ApplicationArtifactRefResponse `json:"artifacts"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Items) != 1 || payload.Items[0].Artifacts == nil {
		t.Fatalf("list artifacts must be a JSON array: %s", data)
	}
}
