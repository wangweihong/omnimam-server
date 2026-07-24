package postgresql

import (
	"fmt"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestApplicationPlatformResetRecreatesOnlyApplicationPlatformOnce(t *testing.T) {
	dsn := os.Getenv("OMNIMAM_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("OMNIMAM_TEST_POSTGRES_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })

	schema := fmt.Sprintf("test_aiapp_reset_%d", time.Now().UnixNano())
	if err := tx.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec("SET LOCAL search_path TO " + schema).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(applicationPlatformResetFixtureSQL).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(applicationPlatformLegacySchemaSQL).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(applicationPlatformLegacySchemaSQL).Error; err != nil {
		t.Fatalf("reset is not idempotent: %v", err)
	}

	for _, table := range []string{"aiapp_engine_instances", "aiapp_comfyui_workflows", "aiapp_application_runs"} {
		var exists bool
		if err := tx.Raw("SELECT to_regclass(?) IS NOT NULL", table).Scan(&exists).Error; err != nil {
			t.Fatal(err)
		}
		if exists {
			t.Fatalf("application platform table %s survived reset", table)
		}
	}
	for _, assertion := range []struct {
		table string
		want  int64
	}{
		{table: "atomic_tasks", want: 3},
		{table: "task_attempts", want: 2},
		{table: "dag_task_groups", want: 2},
		{table: "artifacts", want: 2},
		{table: "artifact_asset_registrations", want: 2},
		{table: "sse_user_events", want: 2},
		{table: "runtime_projection_events", want: 2},
		{table: "watermill_test_topic", want: 2},
		{table: "user_assets", want: 1},
		{table: "blobs", want: 1},
	} {
		var count int64
		if err := tx.Table(assertion.table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != assertion.want {
			t.Fatalf("%s rows=%d, want %d", assertion.table, count, assertion.want)
		}
	}
}

const applicationPlatformResetFixtureSQL = `
CREATE TABLE aiapp_engine_instances (id TEXT PRIMARY KEY);
CREATE TABLE aiapp_comfyui_workflows (id TEXT PRIMARY KEY, converted_application_template_id TEXT);
CREATE TABLE aiapp_comfyui_workflow_validations (id TEXT PRIMARY KEY);
CREATE TABLE aiapp_comfyui_workflow_test_runs (id TEXT PRIMARY KEY, dag_task_group_id TEXT);
CREATE TABLE aiapp_application_templates (id TEXT PRIMARY KEY);
CREATE TABLE aiapp_application_template_versions (id TEXT PRIMARY KEY);
CREATE TABLE aiapp_applications (id TEXT PRIMARY KEY);
CREATE TABLE aiapp_application_versions (id TEXT PRIMARY KEY);
CREATE TABLE aiapp_application_runs (id TEXT PRIMARY KEY);
CREATE TABLE aiapp_artifacts (id TEXT PRIMARY KEY);
CREATE TABLE aiapp_engine_capability_bindings (id TEXT PRIMARY KEY);
CREATE TABLE aiapp_comfyui_engine_object_info (id TEXT PRIMARY KEY);

CREATE TABLE dag_task_groups (id TEXT PRIMARY KEY, runtime_execution_id TEXT DEFAULT '');
CREATE TABLE atomic_tasks (
  id TEXT PRIMARY KEY,
  application_run_id TEXT DEFAULT '',
  owner_type TEXT DEFAULT '',
  owner_id TEXT DEFAULT '',
  runtime_task_id TEXT DEFAULT '',
  runtime_execution_id TEXT DEFAULT ''
);
CREATE TABLE task_attempts (id TEXT PRIMARY KEY, atomic_task_id TEXT NOT NULL);
CREATE TABLE runtime_projection_events (id TEXT PRIMARY KEY, runtime_task_id TEXT DEFAULT '', runtime_execution_id TEXT DEFAULT '');
CREATE TABLE artifacts (id TEXT PRIMARY KEY, application_run_id TEXT DEFAULT '', atomic_task_id TEXT DEFAULT '');
CREATE TABLE artifact_asset_registrations (id TEXT PRIMARY KEY, application_run_id TEXT DEFAULT '', artifact_id TEXT DEFAULT '');
CREATE TABLE sse_user_events (
  id TEXT PRIMARY KEY,
  application_run_id TEXT DEFAULT '',
  dag_task_group_id TEXT DEFAULT '',
  atomic_task_id TEXT DEFAULT '',
  task_attempt_id TEXT DEFAULT '',
  artifact_id TEXT DEFAULT '',
  aggregate_id TEXT DEFAULT ''
);
CREATE TABLE watermill_test_topic (id TEXT PRIMARY KEY, payload JSON);
CREATE TABLE user_assets (id TEXT PRIMARY KEY);
CREATE TABLE blobs (id TEXT PRIMARY KEY);

INSERT INTO aiapp_engine_instances VALUES ('engine-linked');
INSERT INTO aiapp_comfyui_workflows VALUES ('workflow-linked', 'template-linked');
INSERT INTO aiapp_comfyui_workflow_validations VALUES ('validation-linked');
INSERT INTO aiapp_comfyui_workflow_test_runs VALUES ('test-run-linked', 'dag-linked');
INSERT INTO aiapp_application_templates VALUES ('template-linked');
INSERT INTO aiapp_application_template_versions VALUES ('template-version-linked');
INSERT INTO aiapp_applications VALUES ('application-linked');
INSERT INTO aiapp_application_versions VALUES ('application-version-linked');
INSERT INTO aiapp_application_runs VALUES ('run-linked');
INSERT INTO aiapp_artifacts VALUES ('projection-linked');
INSERT INTO aiapp_engine_capability_bindings VALUES ('binding-linked');
INSERT INTO aiapp_comfyui_engine_object_info VALUES ('catalog-linked');

INSERT INTO dag_task_groups VALUES ('dag-linked', 'execution-linked'), ('dag-unrelated', 'execution-unrelated');
INSERT INTO atomic_tasks VALUES
  ('task-linked', 'run-linked', '', '', 'runtime-task-linked', 'execution-linked'),
  ('task-test-linked', '', 'DAG_TASK_GROUP', 'dag-linked', 'runtime-task-test-linked', 'execution-linked'),
  ('task-unrelated', '', '', '', 'runtime-task-unrelated', 'execution-unrelated');
INSERT INTO task_attempts VALUES ('attempt-linked', 'task-linked'), ('attempt-unrelated', 'task-unrelated');
INSERT INTO runtime_projection_events VALUES
  ('projection-event-linked', 'runtime-task-linked', 'execution-linked'),
  ('projection-event-unrelated', 'runtime-task-unrelated', 'execution-unrelated');
INSERT INTO artifacts VALUES ('artifact-linked', 'run-linked', 'task-linked'), ('artifact-unrelated', '', 'task-unrelated');
INSERT INTO artifact_asset_registrations VALUES
  ('registration-linked', 'run-linked', 'artifact-linked'),
  ('registration-unrelated', '', 'artifact-unrelated');
INSERT INTO sse_user_events VALUES
  ('event-linked', 'run-linked', '', 'task-linked', 'attempt-linked', 'artifact-linked', 'application-linked'),
  ('event-unrelated', '', 'dag-unrelated', 'task-unrelated', 'attempt-unrelated', 'artifact-unrelated', 'unrelated');
INSERT INTO watermill_test_topic VALUES
  ('outbox-linked', '{"application_run_id":"run-linked"}'),
  ('outbox-unrelated', '{"application_run_id":"run-unrelated"}');
INSERT INTO user_assets VALUES ('asset-preserved');
INSERT INTO blobs VALUES ('blob-preserved');
`
