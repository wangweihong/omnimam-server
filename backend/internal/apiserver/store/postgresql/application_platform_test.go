package postgresql

import (
	"strings"
	"testing"
)

func TestApplicationPlatformLegacyMigrationIsConditional(t *testing.T) {
	for _, marker := range []string{"pg_advisory_xact_lock", "information_schema.columns", "source_engine_instance_id", "reset_application_runs", "reset_test_dags", "reset_atomic_tasks", "reset_artifacts", "watermill_%", "aiapp_engine_instances", "column_name='template_id'", "column_name='run_mode'", "DROP TABLE IF EXISTS"} {
		if !strings.Contains(applicationPlatformLegacySchemaSQL, marker) {
			t.Fatalf("legacy migration missing %q", marker)
		}
	}
	if strings.Contains(applicationPlatformConstraintsSQL, "fk_aiapp_comfyui_workflow_source_engine") || strings.Contains(applicationPlatformConstraintsSQL, "source_engine_instance_id") {
		t.Fatal("application platform constraints still recreate the removed workflow source engine")
	}
}

func TestApplicationPlatformConstraintsCoverSSOTResources(t *testing.T) {
	markers := []string{
		"idx_aiapp_engine_instances_name",
		"fk_aiapp_comfyui_object_info_engine",
		"idx_aiapp_binding_engine_capability",
		"idx_aiapp_comfyui_workflows_conversion_key",
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
		"ck_aiapp_comfyui_workflow_conversion",
		"ck_aiapp_comfyui_validation_status",
		"fk_aiapp_comfyui_workflow_converted_version",
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
