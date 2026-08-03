package postgresql

import (
	"strings"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func TestApplicationPlatformConstraintsCoverSSOTResources(t *testing.T) {
	markers := []string{
		"idx_aiapp_engine_instances_name",
		"fk_aiapp_comfyui_object_info_engine",
		"idx_aiapp_binding_engine_capability",
		"idx_aiapp_templates_conversion_key",
		"idx_aiapp_comfyui_validations_engine_status",
		"idx_aiapp_template_versions_number",
		"idx_aiapp_application_versions_semver",
		"idx_aiapp_runs_owner_idempotency",
		"idx_aiapp_artifact_refs_run_output",
		"idx_aiapp_artifact_refs_processing",
		"idx_aiapp_artifacts_run_output",
		"ck_aiapp_template_version_number",
		"ck_aiapp_template_version_published_at",
		"ck_aiapp_template_version_source",
		"ck_aiapp_comfyui_workflow_checksums",
		"ck_aiapp_comfyui_workflow_source",
		"ck_aiapp_comfyui_validation_status",
		"ck_aiapp_application_version_published_at",
		"ck_aiapp_run_task_creation",
		"ck_aiapp_artifact_ref_processing",
		"ck_aiapp_artifact_ref_registration",
		"ck_aiapp_artifact_registration",
	}
	for _, marker := range markers {
		if !strings.Contains(applicationPlatformConstraintsSQL, marker) {
			t.Fatalf("application platform constraints missing %q", marker)
		}
	}
	for _, marker := range []string{"IF NOT EXISTS", "fk_aiapp_binding_engine", "ON DELETE CASCADE"} {
		if !strings.Contains(applicationPlatformBindingConstraintSQL, marker) {
			t.Fatalf("application platform binding constraint missing %q", marker)
		}
	}
	if strings.Contains(applicationPlatformBindingConstraintSQL, "DROP CONSTRAINT") {
		t.Fatal("application platform binding constraint contains historical replacement logic")
	}
}

func TestApplicationArtifactRefChangedPayloadIncludesContractIdentity(t *testing.T) {
	ref := &iapiserver.ApplicationArtifactRef{
		ApplicationRunID:           "run-1",
		ArtifactID:                 "artifact-1",
		OutputKey:                  "images",
		Sequence:                   2,
		MediaType:                  "image",
		ArtifactProcessingStatus:   iapiserver.ArtifactProcessingReady,
		ArtifactRegistrationStatus: iapiserver.ArtifactRegistrationRegistered,
		ArtifactResourceVersion:    7,
	}
	payload := applicationArtifactRefChangedPayload(ref, "task-1", "run-1:artifact-1:7")
	for key, want := range map[string]any{
		"source_event_id":           "run-1:artifact-1:7",
		"application_run_id":        "run-1",
		"atomic_task_id":            "task-1",
		"artifact_id":               "artifact-1",
		"artifact_resource_version": int64(7),
	} {
		if payload[key] != want {
			t.Fatalf("payload[%q] = %#v, want %#v", key, payload[key], want)
		}
	}
}
