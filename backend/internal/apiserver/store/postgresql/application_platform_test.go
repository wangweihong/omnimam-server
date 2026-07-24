package postgresql

import (
	"strings"
	"testing"
)

func TestApplicationPlatformLegacyMigrationIsConditional(t *testing.T) {
	for _, marker := range []string{"pg_advisory_xact_lock", "information_schema.columns", "converted_application_template_id", "aiapp_engine_instances", "DROP TABLE IF EXISTS"} {
		if !strings.Contains(applicationPlatformLegacySchemaSQL, marker) {
			t.Fatalf("legacy migration missing %q", marker)
		}
	}
	for _, removed := range []string{"source_engine_instance_id", "converted_application_template_id", "source_workflow_validation_id", "reset_application_runs", "watermill_%"} {
		if strings.Contains(applicationPlatformConstraintsSQL, removed) || strings.Contains(applicationPlatformLegacySchemaSQL, removed) && removed != "converted_application_template_id" {
			t.Fatalf("application platform migration still contains removed compatibility marker %q", removed)
		}
	}
}

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
		"idx_aiapp_artifacts_run_output",
		"ck_aiapp_template_version_number",
		"ck_aiapp_template_version_published_at",
		"ck_aiapp_template_version_source",
		"ck_aiapp_comfyui_workflow_checksums",
		"ck_aiapp_comfyui_workflow_source",
		"ck_aiapp_comfyui_validation_status",
		"ck_aiapp_application_version_published_at",
		"ck_aiapp_run_task_creation",
		"ck_aiapp_artifact_registration",
	}
	for _, marker := range markers {
		if !strings.Contains(applicationPlatformConstraintsSQL, marker) {
			t.Fatalf("application platform constraints missing %q", marker)
		}
	}
	for _, marker := range []string{"confdeltype <> 'c'", "fk_aiapp_binding_engine", "ON DELETE CASCADE"} {
		if !strings.Contains(applicationPlatformBindingCascadeSQL, marker) {
			t.Fatalf("application platform binding cascade migration missing %q", marker)
		}
	}
}
