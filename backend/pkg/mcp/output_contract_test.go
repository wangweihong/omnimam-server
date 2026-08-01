package mcp_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	service "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/mcp"
	protocol "github.com/wangweihong/omnimam/backend/pkg/mcp"
)

func TestReleasedToolOutputProjections(t *testing.T) {
	t.Parallel()
	now := imachinery.NewTime(time.Now())
	application := service.ApplicationSummary{
		ApplicationID: "app-1", Name: "Application", Description: "Runnable application",
		PublishedVersionID: "version-1", RunEnabled: true, Availability: "available",
		SchemaURI: "omnimam://applications/app-1",
	}
	version := service.ApplicationVersionSummary{ApplicationVersionID: "version-1", Version: 1}
	asset := service.AssetSummary{
		AssetID: "asset-1", DisplayName: "Asset", MediaType: "image", Status: "active",
		ThumbnailStatus: "none", URI: "omnimam://assets/asset-1", CreatedAt: now,
	}
	jsonSchema := map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
	}
	capability := service.CapabilitySummary{
		CapabilityID: "image.generate", Name: "Image generation", Description: "Generate images.",
		Status: "available", SupportsDirectInvoke: false,
		SchemaURI: "omnimam://capabilities/image.generate", Applications: []service.ApplicationNavigationSummary{{
			ApplicationID: "app-1", Name: "Application", PublishedVersionID: "version-1", RunEnabled: true,
		}},
	}
	outputs := map[string]any{
		protocol.ToolCapabilitiesList: service.CapabilityListResult{Total: 1, Items: []service.CapabilitySummary{capability}},
		protocol.ToolCapabilitiesGet:  service.CapabilityDetail{CapabilitySummary: capability, InputSchema: jsonSchema, OutputSchema: jsonSchema},
		protocol.ToolApplicationsList: service.ApplicationListResult{Total: 1, Items: []service.ApplicationSummary{application}},
		protocol.ToolApplicationsGet:  service.ApplicationDetail{ApplicationSummary: application, PublishedVersion: version, InputSchema: jsonSchema, OutputSchema: jsonSchema},
		protocol.ToolApplicationsRun: service.ApplicationRunAccepted{
			ApplicationRunID: "run-1", ApplicationID: "app-1", ApplicationVersionID: "version-1",
			Status: "queued", StatusURI: "omnimam://application-runs/run-1",
		},
		protocol.ToolApplicationRunsGet: service.ApplicationRunProjection{
			ApplicationRunID: "run-1", Application: application, ApplicationVersion: version,
			AtomicTask: &service.AtomicTaskProjection{
				AtomicTaskID: "task-1", Status: "PENDING", Progress: 0, ResourceVersion: 1,
			},
			Status: "queued", Progress: 0, Outputs: []service.ArtifactProjection{}, CreatedAt: now, ResourceVersion: 1,
		},
		protocol.ToolApplicationRunsCancel: service.ApplicationRunCancelResult{ApplicationRunID: "run-1", Accepted: true, Status: "cancel_requested"},
		protocol.ToolAssetsSearch:          service.AssetListResult{Total: 1, Items: []service.AssetSummary{asset}},
		protocol.ToolAssetsGet: service.AssetDetailProjection{
			AssetSummary:   asset,
			CurrentVersion: service.AssetVersionProjection{AssetVersionID: "asset-version-1", Version: 1, Status: "processing"},
			Representations: []service.RepresentationProjection{{
				RepresentationID: "representation-1", Type: "original", MediaType: "image/png",
				URI: "omnimam://assets/asset-1/representations/representation-1",
			}}, Tags: []string{},
		},
		protocol.ToolAssetsPrepareUpload: service.AssetUploadPrepared{
			UploadID: "upload-1", Method: "POST", ContentUploadURL: "https://example.test/api/v1/asset-uploads/upload-1/content",
			RequiredHeaders: map[string]string{"Content-Type": "image/png"}, AuthorizationRequired: true, ExpiresAt: now,
		},
		protocol.ToolAssetsCompleteUpload: service.AssetUploadCompleted{
			UploadID: "upload-1", Asset: asset, Status: "processing", URI: "omnimam://assets/asset-1",
		},
	}
	validator, err := protocol.NewResultValidator()
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range protocol.ToolNames {
		tool := tool
		t.Run(tool, func(t *testing.T) {
			t.Parallel()
			if err := validator.Validate(tool, outputs[tool]); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("reject nested projection drift", func(t *testing.T) {
		raw, err := json.Marshal(outputs[protocol.ToolApplicationRunsGet])
		if err != nil {
			t.Fatal(err)
		}
		var drift map[string]any
		if err := json.Unmarshal(raw, &drift); err != nil {
			t.Fatal(err)
		}
		atomicTask, ok := drift["atomic_task"].(map[string]any)
		if !ok {
			t.Fatal("atomic_task projection is missing")
		}
		atomicTask["private_state"] = true
		if err := validator.Validate(protocol.ToolApplicationRunsGet, drift); err == nil {
			t.Fatal("expected an undeclared nested projection field to be rejected")
		}
	})
}
