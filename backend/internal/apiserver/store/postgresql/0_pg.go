package postgresql

import (
	"fmt"
	"sync"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/pkg/httpsvr/genericoptions"

	"gorm.io/gorm"
)

var (
	postgresqlFactory store.Factory
	once              sync.Once
)

const taskCenterActiveScheduleIndexSQL = `
CREATE UNIQUE INDEX IF NOT EXISTS idx_schedule_executions_active
ON task_schedule_executions(schedule_id)
WHERE status IN ('TRIGGERED', 'RUNNING')
;
CREATE UNIQUE INDEX IF NOT EXISTS idx_schedule_execution_manual_key
ON task_schedule_executions(schedule_id, idempotency_key)
WHERE trigger_source = 'MANUAL';
CREATE UNIQUE INDEX IF NOT EXISTS idx_task_schedules_system_key
ON task_schedules(system_key)
WHERE system_key <> '';
CREATE INDEX IF NOT EXISTS idx_task_schedules_mode_status
ON task_schedules(execution_mode, status, next_trigger_at);
CREATE INDEX IF NOT EXISTS idx_schedule_executions_reconcile_retention
ON task_schedule_executions(schedule_id, execution_mode, status, completed_at DESC, scheduled_at DESC);
CREATE INDEX IF NOT EXISTS idx_schedule_reconcile_states_checkpoint
ON task_schedule_reconcile_states(last_completed_at, updated_at)
;
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_task_schedule_mode_target') THEN
    ALTER TABLE task_schedules ADD CONSTRAINT ck_task_schedule_mode_target CHECK (
      (execution_mode='MATERIALIZED' AND target_type<>'' AND target_template_json<>'' AND reconcile_ref='') OR
      (execution_mode='RECONCILE' AND trigger_type='CRON' AND target_type='' AND target_template_json='' AND reconcile_ref<>'')
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_task_schedule_management') THEN
    ALTER TABLE task_schedules ADD CONSTRAINT ck_task_schedule_management CHECK (
      (management_mode='USER' AND system_key='') OR (management_mode='SYSTEM' AND system_key<>'')
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_task_schedule_reconcile_timeout') THEN
    ALTER TABLE task_schedules ADD CONSTRAINT ck_task_schedule_reconcile_timeout CHECK (reconcile_overall_timeout_seconds >= reconcile_per_item_timeout_seconds);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_task_schedule_reconcile_limits') THEN
    ALTER TABLE task_schedules ADD CONSTRAINT ck_task_schedule_reconcile_limits CHECK (
      reconcile_max_parallelism BETWEEN 1 AND 64 AND
      reconcile_max_items_per_run BETWEEN 1 AND 1000 AND
      reconcile_per_item_timeout_seconds BETWEEN 1 AND 30 AND
      reconcile_overall_timeout_seconds BETWEEN 1 AND 300
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_task_schedule_modes') THEN
    ALTER TABLE task_schedules ADD CONSTRAINT ck_task_schedule_modes CHECK (
      execution_mode IN ('MATERIALIZED','RECONCILE') AND management_mode IN ('USER','SYSTEM')
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_schedule_execution_mode_target') THEN
    ALTER TABLE task_schedule_executions ADD CONSTRAINT ck_schedule_execution_mode_target CHECK (
      (execution_mode='MATERIALIZED' AND target_type<>'') OR (execution_mode='RECONCILE' AND target_type='' AND target_id='')
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_schedule_reconcile_state_schedule') THEN
    ALTER TABLE task_schedule_reconcile_states ADD CONSTRAINT fk_schedule_reconcile_state_schedule FOREIGN KEY (schedule_id) REFERENCES task_schedules(id) ON DELETE CASCADE;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_schedule_execution_schedule') THEN
    ALTER TABLE task_schedule_executions ADD CONSTRAINT fk_schedule_execution_schedule FOREIGN KEY (schedule_id) REFERENCES task_schedules(id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_schedule_reconcile_state_totals') THEN
    ALTER TABLE task_schedule_reconcile_states ADD CONSTRAINT ck_schedule_reconcile_state_totals CHECK (
      consecutive_failures >= 0 AND total_runs >= 0 AND total_scanned >= 0 AND total_findings >= 0 AND total_actions_created >= 0
    );
  END IF;
END $$
`

const taskCenterAttemptLogsRefBackfillSQL = `
UPDATE task_attempts
SET logs_ref = 'task-attempt-log:' || id
WHERE COALESCE(logs_ref, '') = '' AND id <> '';
`

const mcpTaskBindingConstraintsSQL = `
CREATE UNIQUE INDEX IF NOT EXISTS idx_mcp_binding_principal_run
ON mcp_task_bindings(principal_id, application_run_id);
CREATE INDEX IF NOT EXISTS idx_mcp_task_bindings_expiry
ON mcp_task_bindings(expires_at);
CREATE INDEX IF NOT EXISTS idx_mcp_task_bindings_principal_access
ON mcp_task_bindings(principal_id, last_accessed_at DESC);
CREATE INDEX IF NOT EXISTS idx_mcp_task_bindings_atomic_task
ON mcp_task_bindings(atomic_task_id);
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='chk_mcp_task_binding_ids') THEN
    ALTER TABLE mcp_task_bindings ADD CONSTRAINT chk_mcp_task_binding_ids CHECK (
      btrim(mcp_task_id) <> '' AND btrim(principal_id) <> '' AND
      btrim(application_run_id) <> '' AND btrim(atomic_task_id) <> ''
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='chk_mcp_task_binding_extension') THEN
    ALTER TABLE mcp_task_bindings ADD CONSTRAINT chk_mcp_task_binding_extension
      CHECK (extension_id = 'io.modelcontextprotocol/tasks');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='chk_mcp_task_binding_expiry') THEN
    ALTER TABLE mcp_task_bindings ADD CONSTRAINT chk_mcp_task_binding_expiry
      CHECK (expires_at > created_at);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='chk_mcp_task_binding_resource_version') THEN
    ALTER TABLE mcp_task_bindings ADD CONSTRAINT chk_mcp_task_binding_resource_version
      CHECK (resource_version >= 0);
  END IF;
END $$;
`

const taskCenterDAGObservabilityMigrationSQL = `
UPDATE atomic_tasks
SET dag_node_key = child_key
WHERE owner_type = 'DAG_TASK_GROUP' AND COALESCE(dag_node_key, '') = '' AND COALESCE(child_key, '') <> '';

UPDATE dag_task_groups
SET triggered_at = created_at
WHERE triggered_at IS NULL;

UPDATE dag_task_groups
SET trigger_type = CASE
  WHEN COALESCE(retry_of_id, '') <> '' THEN 'RETRY'
  WHEN COALESCE(canvas_version_id, '') <> '' THEN 'CANVAS'
  WHEN EXISTS (
    SELECT 1 FROM task_schedule_executions e
    WHERE e.target_type = 'DAG_TASK_GROUP' AND e.target_id = dag_task_groups.id
  ) THEN 'SCHEDULE'
  ELSE 'API'
END
WHERE COALESCE(trigger_type, '') = '' OR trigger_type = 'API';

UPDATE dag_task_groups AS dag
SET trigger_source_id = CASE
      WHEN dag.trigger_type = 'RETRY' THEN dag.retry_of_id
      WHEN dag.trigger_type = 'CANVAS' THEN dag.canvas_version_id
      WHEN dag.trigger_type = 'SCHEDULE' THEN COALESCE((
        SELECT e.schedule_id FROM task_schedule_executions e
        WHERE e.target_type = 'DAG_TASK_GROUP' AND e.target_id = dag.id
        ORDER BY e.scheduled_at DESC LIMIT 1
      ), '')
      ELSE dag.trigger_source_id
    END,
    trigger_source_name = CASE
      WHEN dag.trigger_type = 'SCHEDULE' THEN COALESCE((
        SELECT s.name FROM task_schedule_executions e
        JOIN task_schedules s ON s.id = e.schedule_id
        WHERE e.target_type = 'DAG_TASK_GROUP' AND e.target_id = dag.id
        ORDER BY e.scheduled_at DESC LIMIT 1
      ), '')
      ELSE dag.trigger_source_name
    END
WHERE COALESCE(dag.trigger_source_id, '') = '';

CREATE INDEX IF NOT EXISTS idx_atomic_tasks_dag_node
ON atomic_tasks(owner_id, dag_node_key, child_order)
WHERE owner_type = 'DAG_TASK_GROUP' AND dag_node_key <> '';
CREATE INDEX IF NOT EXISTS idx_dag_groups_status_time
ON dag_task_groups(status, started_at, completed_at);
CREATE INDEX IF NOT EXISTS idx_runtime_projection_execution_time
ON runtime_projection_events(runtime_execution_id, occurred_at, id);
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_dag_groups_trigger_type') THEN
    ALTER TABLE dag_task_groups ADD CONSTRAINT ck_dag_groups_trigger_type
      CHECK (trigger_type IN ('API','SCHEDULE','CANVAS','DOMAIN_EVENT','RETRY'));
  END IF;
END $$;
`

const taskCenterApplicationRunIndexesSQL = `
CREATE INDEX IF NOT EXISTS idx_atomic_tasks_application
ON atomic_tasks(application_run_id)
WHERE application_run_id <> '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_atomic_tasks_idempotency
ON atomic_tasks(project_id, namespace, idempotency_scope, idempotency_key)
WHERE idempotency_scope <> '';
DROP INDEX IF EXISTS idx_atomic_tasks_owner_child;
CREATE UNIQUE INDEX idx_atomic_tasks_owner_child
ON atomic_tasks(owner_type, owner_id, child_key)
WHERE owner_type IN ('TASK_GROUP','DAG_TASK_GROUP') AND child_key <> '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_atomic_tasks_runtime_task
ON atomic_tasks(runtime_task_id)
WHERE runtime_task_id <> '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_task_groups_idempotency
ON task_groups(project_id, namespace, idempotency_scope, idempotency_key)
WHERE idempotency_scope <> '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_dag_groups_idempotency
ON dag_task_groups(project_id, namespace, idempotency_scope, idempotency_key)
WHERE idempotency_scope <> '';
`

const sseUserEventConstraintsSQL = `
CREATE INDEX IF NOT EXISTS idx_sse_user_events_expires ON sse_user_events(expires_at);
CREATE INDEX IF NOT EXISTS idx_sse_user_events_recipient_sequence ON sse_user_events(recipient_user_id, event_sequence);
CREATE INDEX IF NOT EXISTS idx_sse_user_events_recipient_occurred ON sse_user_events(recipient_user_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_sse_user_events_aggregate ON sse_user_events(aggregate_type, aggregate_id, aggregate_version);
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_sse_user_events_version') THEN
    ALTER TABLE sse_user_events ADD CONSTRAINT ck_sse_user_events_version CHECK (event_version >= 1 AND aggregate_version >= 0);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_sse_user_events_expiry') THEN
    ALTER TABLE sse_user_events ADD CONSTRAINT ck_sse_user_events_expiry CHECK (expires_at > occurred_at);
  END IF;
END $$;
`

const taskCenterScheduleOwnershipBackfillSQL = `
UPDATE atomic_tasks AS target
SET created_by = schedule.created_by,
    project_id = schedule.project_id,
    namespace = schedule.namespace,
    owner_type = 'TASK_SCHEDULE',
    owner_id = schedule.id
FROM task_schedule_executions AS execution
JOIN task_schedules AS schedule ON schedule.id = execution.schedule_id
WHERE execution.target_type = 'ATOMIC_TASK'
  AND execution.target_id <> ''
  AND target.id = execution.target_id
  AND (target.created_by, target.project_id, target.namespace, target.owner_type, target.owner_id)
      IS DISTINCT FROM (schedule.created_by, schedule.project_id, schedule.namespace, 'TASK_SCHEDULE', schedule.id);

UPDATE task_groups AS target
SET created_by = schedule.created_by,
    project_id = schedule.project_id,
    namespace = schedule.namespace
FROM task_schedule_executions AS execution
JOIN task_schedules AS schedule ON schedule.id = execution.schedule_id
WHERE execution.target_type = 'TASK_GROUP'
  AND execution.target_id <> ''
  AND target.id = execution.target_id
  AND (target.created_by, target.project_id, target.namespace)
      IS DISTINCT FROM (schedule.created_by, schedule.project_id, schedule.namespace);

UPDATE atomic_tasks AS child
SET created_by = parent.created_by,
    project_id = parent.project_id,
    namespace = parent.namespace
FROM task_groups AS parent
WHERE child.owner_type = 'TASK_GROUP'
  AND child.owner_id = parent.id
  AND EXISTS (
    SELECT 1 FROM task_schedule_executions AS execution
    WHERE execution.target_type = 'TASK_GROUP' AND execution.target_id = parent.id
  )
  AND (child.created_by, child.project_id, child.namespace)
      IS DISTINCT FROM (parent.created_by, parent.project_id, parent.namespace);

UPDATE dag_task_groups AS target
SET created_by = schedule.created_by,
    project_id = schedule.project_id,
    namespace = schedule.namespace
FROM task_schedule_executions AS execution
JOIN task_schedules AS schedule ON schedule.id = execution.schedule_id
WHERE execution.target_type = 'DAG_TASK_GROUP'
  AND execution.target_id <> ''
  AND target.id = execution.target_id
  AND (target.created_by, target.project_id, target.namespace)
      IS DISTINCT FROM (schedule.created_by, schedule.project_id, schedule.namespace);

UPDATE atomic_tasks AS child
SET created_by = parent.created_by,
    project_id = parent.project_id,
    namespace = parent.namespace
FROM dag_task_groups AS parent
WHERE child.owner_type = 'DAG_TASK_GROUP'
  AND child.owner_id = parent.id
  AND EXISTS (
    SELECT 1 FROM task_schedule_executions AS execution
    WHERE execution.target_type = 'DAG_TASK_GROUP' AND execution.target_id = parent.id
  )
  AND (child.created_by, child.project_id, child.namespace)
      IS DISTINCT FROM (parent.created_by, parent.project_id, parent.namespace);
`

const applicationPlatformLegacySchemaSQL = `
DO $$
BEGIN
  PERFORM pg_advisory_xact_lock(hashtext('omnimam:application-platform-schema-reset'));
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = current_schema()
      AND table_name = 'aiapp_comfyui_workflows'
      AND column_name = 'converted_application_template_id'
  ) THEN
    DROP TABLE IF EXISTS
      aiapp_application_artifact_refs,
      aiapp_artifacts,
      aiapp_application_runs,
      aiapp_application_versions,
      aiapp_applications,
      aiapp_application_template_versions,
      aiapp_application_templates,
      aiapp_comfyui_workflow_test_runs,
      aiapp_comfyui_workflow_validations,
      aiapp_comfyui_workflows,
      aiapp_engine_capability_bindings,
      aiapp_comfyui_engine_object_info,
      aiapp_engine_instances
      CASCADE;
  END IF;
END $$;
`

const applicationPlatformConstraintsSQL = `
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_engine_instances_name ON aiapp_engine_instances(name);
CREATE INDEX IF NOT EXISTS idx_aiapp_engine_instances_type_health ON aiapp_engine_instances(application_engine_type_id, enabled, health_status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_binding_engine_capability ON aiapp_engine_capability_bindings(engine_instance_id, provider_capability_id);
CREATE INDEX IF NOT EXISTS idx_aiapp_binding_capability ON aiapp_engine_capability_bindings(provider_capability_id, enabled);
CREATE INDEX IF NOT EXISTS idx_aiapp_comfyui_workflows_owner_created ON aiapp_comfyui_workflows(owner_user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_aiapp_comfyui_workflows_filters ON aiapp_comfyui_workflows(owner_user_id, source_type, api_conversion_status);
CREATE INDEX IF NOT EXISTS idx_aiapp_comfyui_workflows_checksum ON aiapp_comfyui_workflows(owner_user_id, source_type, source_checksum);
CREATE INDEX IF NOT EXISTS idx_aiapp_comfyui_validations_workflow_created ON aiapp_comfyui_workflow_validations(workflow_id, created_at);
CREATE INDEX IF NOT EXISTS idx_aiapp_comfyui_validations_engine_status ON aiapp_comfyui_workflow_validations(engine_instance_id, status, validated_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_templates_owner_name ON aiapp_application_templates(owner_user_id, name);
CREATE INDEX IF NOT EXISTS idx_aiapp_templates_capability ON aiapp_application_templates(capability_definition_id, capability_source_type);
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_templates_conversion_key ON aiapp_application_templates(owner_user_id, comfyui_conversion_idempotency_key) WHERE comfyui_conversion_idempotency_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_template_versions_number ON aiapp_application_template_versions(application_template_id, version);
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_applications_owner_name ON aiapp_applications(owner_user_id, name);
CREATE INDEX IF NOT EXISTS idx_aiapp_applications_owner_visibility ON aiapp_applications(owner_user_id, visibility);
CREATE INDEX IF NOT EXISTS idx_aiapp_applications_capability_run ON aiapp_applications(capability_definition_id, run_enabled);
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_application_versions_semver ON aiapp_application_versions(application_id, semantic_version);
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_runs_owner_idempotency ON aiapp_application_runs(owner_user_id, idempotency_key);
CREATE INDEX IF NOT EXISTS idx_aiapp_runs_application_created ON aiapp_application_runs(application_id, created_at);
CREATE INDEX IF NOT EXISTS idx_aiapp_runs_engine_created ON aiapp_application_runs(engine_instance_id, created_at);
CREATE INDEX IF NOT EXISTS idx_aiapp_runs_capability_revision ON aiapp_application_runs(provider_capability_id, provider_capability_revision);
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_artifact_refs_artifact ON aiapp_application_artifact_refs(artifact_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_artifact_refs_run_output ON aiapp_application_artifact_refs(application_run_id, output_key, sequence);
CREATE INDEX IF NOT EXISTS idx_aiapp_artifact_refs_run ON aiapp_application_artifact_refs(application_run_id, output_key, sequence);
CREATE INDEX IF NOT EXISTS idx_aiapp_artifact_refs_processing ON aiapp_application_artifact_refs(artifact_processing_status, updated_at);
CREATE INDEX IF NOT EXISTS idx_aiapp_artifact_refs_registration ON aiapp_application_artifact_refs(artifact_registration_status, updated_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_artifacts_run_output ON aiapp_artifacts(application_run_id, output_key);
CREATE INDEX IF NOT EXISTS idx_aiapp_artifacts_registration ON aiapp_artifacts(registration_status, updated_at);

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_engine_auth_type') THEN ALTER TABLE aiapp_engine_instances ADD CONSTRAINT ck_aiapp_engine_auth_type CHECK (auth_type IN ('none','api_key','bearer_token','ak_sk')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_engine_health') THEN ALTER TABLE aiapp_engine_instances ADD CONSTRAINT ck_aiapp_engine_health CHECK (health_status IN ('unknown','online','offline','degraded')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_engine_limits') THEN ALTER TABLE aiapp_engine_instances ADD CONSTRAINT ck_aiapp_engine_limits CHECK (max_concurrency > 0 AND request_timeout_seconds > 0 AND task_timeout_seconds > 0); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_comfyui_object_info_engine') THEN ALTER TABLE aiapp_comfyui_engine_object_info ADD CONSTRAINT fk_aiapp_comfyui_object_info_engine FOREIGN KEY (engine_instance_id) REFERENCES aiapp_engine_instances(id) ON DELETE CASCADE; END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_comfyui_workflow_checksums') THEN ALTER TABLE aiapp_comfyui_workflows ADD CONSTRAINT ck_aiapp_comfyui_workflow_checksums CHECK (source_checksum ~ '^sha256:[0-9a-f]{64}$' AND (api_workflow_checksum IS NULL OR api_workflow_checksum ~ '^sha256:[0-9a-f]{64}$')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_comfyui_workflow_source') THEN ALTER TABLE aiapp_comfyui_workflows ADD CONSTRAINT ck_aiapp_comfyui_workflow_source CHECK ((source_type='api_workflow' AND api_conversion_status='ready' AND api_workflow_json IS NOT NULL AND api_workflow_checksum IS NOT NULL) OR (source_type='visual_workflow' AND visual_workflow_json IS NOT NULL AND ((api_conversion_status='pending' AND api_workflow_json IS NULL AND api_workflow_checksum IS NULL) OR (api_conversion_status='ready' AND api_workflow_json IS NOT NULL AND api_workflow_checksum IS NOT NULL)))); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_comfyui_validation_workflow') THEN ALTER TABLE aiapp_comfyui_workflow_validations ADD CONSTRAINT fk_aiapp_comfyui_validation_workflow FOREIGN KEY (workflow_id) REFERENCES aiapp_comfyui_workflows(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_comfyui_validation_engine') THEN ALTER TABLE aiapp_comfyui_workflow_validations ADD CONSTRAINT fk_aiapp_comfyui_validation_engine FOREIGN KEY (engine_instance_id) REFERENCES aiapp_engine_instances(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_comfyui_validation_status') THEN ALTER TABLE aiapp_comfyui_workflow_validations ADD CONSTRAINT ck_aiapp_comfyui_validation_status CHECK (status IN ('compatible','incompatible','failed')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_template_source') THEN ALTER TABLE aiapp_application_templates ADD CONSTRAINT ck_aiapp_template_source CHECK (capability_source_type IN ('comfyui_workflow','provider_capability')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_template_version_template') THEN ALTER TABLE aiapp_application_template_versions ADD CONSTRAINT fk_aiapp_template_version_template FOREIGN KEY (application_template_id) REFERENCES aiapp_application_templates(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_template_version_number') THEN ALTER TABLE aiapp_application_template_versions ADD CONSTRAINT ck_aiapp_template_version_number CHECK (version > 0); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_template_version_status') THEN ALTER TABLE aiapp_application_template_versions ADD CONSTRAINT ck_aiapp_template_version_status CHECK (status IN ('draft','published','retired')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_template_version_published_at') THEN ALTER TABLE aiapp_application_template_versions ADD CONSTRAINT ck_aiapp_template_version_published_at CHECK ((status='published' AND published_at IS NOT NULL) OR status<>'published'); END IF;
  ALTER TABLE aiapp_application_template_versions DROP CONSTRAINT IF EXISTS ck_aiapp_template_version_source;
  ALTER TABLE aiapp_application_template_versions ADD CONSTRAINT ck_aiapp_template_version_source CHECK ((capability_source_type='provider_capability' AND provider_capability_id IS NOT NULL AND provider_capability_revision IS NOT NULL AND provider_operation_id IS NOT NULL AND workflow_contract_revision IS NULL AND comfyui_api_workflow_json IS NULL) OR (capability_source_type='comfyui_workflow' AND provider_capability_id IS NULL AND provider_capability_revision IS NULL AND provider_operation_id IS NULL AND workflow_contract_revision IS NOT NULL AND comfyui_api_workflow_json IS NOT NULL));
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_template_version_source_workflow') THEN ALTER TABLE aiapp_application_template_versions ADD CONSTRAINT fk_aiapp_template_version_source_workflow FOREIGN KEY (source_comfyui_workflow_id) REFERENCES aiapp_comfyui_workflows(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_template_current_version') THEN ALTER TABLE aiapp_application_templates ADD CONSTRAINT fk_aiapp_template_current_version FOREIGN KEY (current_version_id) REFERENCES aiapp_application_template_versions(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_application_visibility') THEN ALTER TABLE aiapp_applications ADD CONSTRAINT ck_aiapp_application_visibility CHECK (visibility IN ('private','global')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_application_version_application') THEN ALTER TABLE aiapp_application_versions ADD CONSTRAINT fk_aiapp_application_version_application FOREIGN KEY (application_id) REFERENCES aiapp_applications(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_application_version_template') THEN ALTER TABLE aiapp_application_versions ADD CONSTRAINT fk_aiapp_application_version_template FOREIGN KEY (application_template_version_id) REFERENCES aiapp_application_template_versions(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_application_version_status') THEN ALTER TABLE aiapp_application_versions ADD CONSTRAINT ck_aiapp_application_version_status CHECK (status IN ('draft','published','retired')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_application_version_published_at') THEN ALTER TABLE aiapp_application_versions ADD CONSTRAINT ck_aiapp_application_version_published_at CHECK ((status='published' AND published_at IS NOT NULL) OR status<>'published'); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_application_semver') THEN ALTER TABLE aiapp_application_versions ADD CONSTRAINT ck_aiapp_application_semver CHECK (semantic_version ~ '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_application_current_version') THEN ALTER TABLE aiapp_applications ADD CONSTRAINT fk_aiapp_application_current_version FOREIGN KEY (current_version_id) REFERENCES aiapp_application_versions(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_run_application') THEN ALTER TABLE aiapp_application_runs ADD CONSTRAINT fk_aiapp_run_application FOREIGN KEY (application_id) REFERENCES aiapp_applications(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_run_application_version') THEN ALTER TABLE aiapp_application_runs ADD CONSTRAINT fk_aiapp_run_application_version FOREIGN KEY (application_version_id) REFERENCES aiapp_application_versions(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_run_template_version') THEN ALTER TABLE aiapp_application_runs ADD CONSTRAINT fk_aiapp_run_template_version FOREIGN KEY (application_template_version_id) REFERENCES aiapp_application_template_versions(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_run_engine') THEN ALTER TABLE aiapp_application_runs ADD CONSTRAINT fk_aiapp_run_engine FOREIGN KEY (engine_instance_id) REFERENCES aiapp_engine_instances(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_run_source') THEN ALTER TABLE aiapp_application_runs ADD CONSTRAINT ck_aiapp_run_source CHECK ((capability_source_type='provider_capability' AND provider_capability_id IS NOT NULL AND provider_capability_revision IS NOT NULL AND provider_operation_id IS NOT NULL AND workflow_contract_revision IS NULL) OR (capability_source_type='comfyui_workflow' AND provider_capability_id IS NULL AND provider_capability_revision IS NULL AND provider_operation_id IS NULL AND workflow_contract_revision IS NOT NULL)); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_run_task_creation') THEN ALTER TABLE aiapp_application_runs ADD CONSTRAINT ck_aiapp_run_task_creation CHECK ((task_creation_status='created' AND atomic_task_id IS NOT NULL AND task_status_projection IS NOT NULL) OR (task_creation_status IN ('pending','failed') AND atomic_task_id IS NULL AND task_status_projection IS NULL)); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_artifact_ref_run') THEN ALTER TABLE aiapp_application_artifact_refs ADD CONSTRAINT fk_aiapp_artifact_ref_run FOREIGN KEY (application_run_id) REFERENCES aiapp_application_runs(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_artifact_ref_sequence') THEN ALTER TABLE aiapp_application_artifact_refs ADD CONSTRAINT ck_aiapp_artifact_ref_sequence CHECK (sequence >= 0); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_artifact_ref_media') THEN ALTER TABLE aiapp_application_artifact_refs ADD CONSTRAINT ck_aiapp_artifact_ref_media CHECK (media_type IN ('image','video','audio','text','document','model_3d','prompt','prompt_template','pdf','other')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_artifact_ref_processing') THEN ALTER TABLE aiapp_application_artifact_refs ADD CONSTRAINT ck_aiapp_artifact_ref_processing CHECK (artifact_processing_status IN ('created','transferring','processing','ready','failed','deleted')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_artifact_ref_registration') THEN ALTER TABLE aiapp_application_artifact_refs ADD CONSTRAINT ck_aiapp_artifact_ref_registration CHECK (artifact_registration_status IN ('pending','registered','failed')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_artifact_run') THEN ALTER TABLE aiapp_artifacts ADD CONSTRAINT fk_aiapp_artifact_run FOREIGN KEY (application_run_id) REFERENCES aiapp_application_runs(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_artifact_media') THEN ALTER TABLE aiapp_artifacts ADD CONSTRAINT ck_aiapp_artifact_media CHECK (media_type IN ('image','video','audio','text','pdf','other')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_artifact_registration') THEN ALTER TABLE aiapp_artifacts ADD CONSTRAINT ck_aiapp_artifact_registration CHECK ((registration_status='registered' AND asset_id IS NOT NULL) OR (registration_status IN ('pending','failed') AND asset_id IS NULL)); END IF;
END $$;
`

const applicationPlatformBindingCascadeSQL = `
DO $$ BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='fk_aiapp_binding_engine'
      AND conrelid='aiapp_engine_capability_bindings'::regclass
      AND confdeltype <> 'c'
  ) THEN
    ALTER TABLE aiapp_engine_capability_bindings DROP CONSTRAINT fk_aiapp_binding_engine;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='fk_aiapp_binding_engine'
      AND conrelid='aiapp_engine_capability_bindings'::regclass
  ) THEN
    ALTER TABLE aiapp_engine_capability_bindings ADD CONSTRAINT fk_aiapp_binding_engine
      FOREIGN KEY (engine_instance_id) REFERENCES aiapp_engine_instances(id) ON DELETE CASCADE;
  END IF;
END $$;
`

// GetPostgresSQLFactoryOr create postgresql factory with the given config.
func GetPostgresSQLFactoryOr(opts *genericoptions.PostgresSQLOptions) (store.Factory, error) {
	if opts == nil && postgresqlFactory == nil {
		return nil, errors.Errorf("failed to get postgresql store factory")
	}

	var err error
	var dbIns *gorm.DB
	once.Do(func() {
		dbIns, err = opts.NewClient()
		postgresqlFactory = &datastore{dbIns}
	})

	if postgresqlFactory == nil || err != nil {
		return nil, fmt.Errorf(
			"failed to get postgresql store factory, postgresqlFactory: %+v, error: %w",
			postgresqlFactory,
			err,
		)
	}

	return postgresqlFactory, nil
}

type datastore struct {
	db *gorm.DB
	// redis ?
}

func (ds *datastore) Close() error {
	db, err := ds.db.DB()
	if err != nil {
		return errors.Wrap(err, "get gorm db instance failed")
	}

	return db.Close()
}

func (ds *datastore) EnsureScheme(metaTypes ...any) error {
	if err := ds.prepareApplicationPlatformScheme(); err != nil {
		return err
	}
	if err := ds.db.AutoMigrate(metaTypes...); err != nil {
		return err
	}
	if err := ds.ensureAssetLibraryScheme(); err != nil {
		return err
	}
	if err := ds.ensureTaskCenterScheme(); err != nil {
		return err
	}
	if err := ds.ensureApplicationPlatformScheme(); err != nil {
		return err
	}
	if err := ds.ensureWorkflowCanvasScheme(); err != nil {
		return err
	}
	if err := ds.ensureOutboxScheme(); err != nil {
		return err
	}
	if err := ds.ensureNotificationCenterScheme(); err != nil {
		return err
	}
	if err := ds.ensureMCPScheme(); err != nil {
		return err
	}
	return nil
}

func (ds *datastore) ensureMCPScheme() error {
	return ds.db.Exec(mcpTaskBindingConstraintsSQL).Error
}

func (ds *datastore) ensureAssetLibraryScheme() error {
	return ds.db.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_asset_groups_owner_name_unique
ON user_asset_groups(owner_user_id, lower(trim(name)))
WHERE deleted_at IS NULL;
`).Error
}

func (ds *datastore) ensureTaskCenterScheme() error {
	if err := ds.db.Exec(taskCenterActiveScheduleIndexSQL).Error; err != nil {
		return err
	}
	if err := ds.db.Exec(taskCenterApplicationRunIndexesSQL).Error; err != nil {
		return err
	}
	if err := ds.db.Exec(sseUserEventConstraintsSQL).Error; err != nil {
		return err
	}
	if err := ds.db.Exec(taskCenterScheduleOwnershipBackfillSQL).Error; err != nil {
		return err
	}
	if err := ds.db.Exec(taskCenterAttemptLogsRefBackfillSQL).Error; err != nil {
		return err
	}
	return ds.db.Exec(taskCenterDAGObservabilityMigrationSQL).Error
}

func (ds *datastore) ensureApplicationPlatformScheme() error {
	if err := ds.db.Exec(applicationPlatformConstraintsSQL).Error; err != nil {
		return err
	}
	return ds.db.Exec(applicationPlatformBindingCascadeSQL).Error
}

func (ds *datastore) prepareApplicationPlatformScheme() error {
	return ds.db.Exec(applicationPlatformLegacySchemaSQL).Error
}

func (ds *datastore) Users() store.UserStore {
	return newUser(ds)
}

func (ds *datastore) Identities() store.IdentityStore {
	return newIdentityStore(ds)
}

func (ds *datastore) PlatformManagement() store.PlatformManagementStore {
	return newPlatformManagementStore(ds)
}

/* ------ setting ------- */
/* ------ asset ------- */
func (ds *datastore) AssetLibraries() store.AssetLibraryStore {
	return newAssetLibrary(ds)
}

func (ds *datastore) AssetCategories() store.AssetCategoryStore {
	return newAssetCategory(ds)
}

func (ds *datastore) AssetItems() store.AssetItemStore {
	return newAssetItem(ds)
}

/* ------ prompt ------- */
func (ds *datastore) PromptLibraries() store.PromptLibraryStore {
	return newPromptLibrary(ds)
}

func (ds *datastore) PromptCategories() store.PromptCategoryStore {
	return newPromptCategory(ds)
}

func (ds *datastore) PromptItems() store.PromptItemStore {
	return newPromptItem(ds)
}

/* ------ canvas ------- */
func (ds *datastore) Projects() store.ProjectStore {
	return newProject(ds)
}

func (ds *datastore) Canvases() store.CanvasStore {
	return newCanvas(ds)
}

func (ds *datastore) WorkflowCanvases() store.WorkflowCanvasStore {
	return newWorkflowCanvasStore(ds)
}

/* ------ platform contracts ------- */
func (ds *datastore) Providers() store.ProviderStore {
	return newProvider(ds)
}

func (ds *datastore) ProviderModels() store.ProviderModelStore {
	return newProviderModel(ds)
}

func (ds *datastore) ProviderCapabilities() store.ProviderCapabilityStore {
	return newProviderCapability(ds)
}

func (ds *datastore) SystemLLMConfigs() store.SystemLLMConfigStore {
	return newSystemLLMConfig(ds)
}

func (ds *datastore) StorageBackends() store.StorageBackendStore {
	return newStorageBackend(ds)
}

func (ds *datastore) AssetsV2() store.AssetStore {
	return newPlatformAsset(ds)
}

func (ds *datastore) AssetThumbnails() store.AssetThumbnailStore {
	return newAssetThumbnail(ds)
}

func (ds *datastore) Tags() store.TagStore {
	return newTag(ds)
}

func (ds *datastore) AssetTags() store.AssetTagStore {
	return newAssetTag(ds)
}

func (ds *datastore) AssetGroups() store.AssetGroupStore {
	return newAssetGroup(ds)
}

func (ds *datastore) AssetGroupMembers() store.AssetGroupMemberStore {
	return newAssetGroupMember(ds)
}

func (ds *datastore) AssetRelations() store.AssetRelationStore {
	return newAssetRelation(ds)
}

func (ds *datastore) AssetsV1() store.AssetV1Store { return newAssetV1Store(ds) }

func (ds *datastore) TaskCenters() store.TaskCenterStore {
	return newTaskCenterStore(ds)
}

func (ds *datastore) UserEvents() store.UserEventStore { return newUserEventStore(ds) }

func (ds *datastore) Notifications() store.NotificationStore { return newNotificationStore(ds) }
func (ds *datastore) NotificationCandidates() store.NotificationCandidateStore {
	return newNotificationStore(ds)
}
func (ds *datastore) NotificationOutbox() store.NotificationOutboxStore {
	return newNotificationStore(ds)
}
func (ds *datastore) NotificationRetention() store.NotificationRetentionStore {
	return newNotificationStore(ds)
}

func (ds *datastore) ApplicationPlatforms() store.ApplicationPlatformStore {
	return newApplicationPlatform(ds)
}

func (ds *datastore) MCPTaskBindings() store.MCPTaskBindingStore {
	return newMCPTaskBindingStore(ds)
}

func (ds *datastore) FeatureFlags() store.FeatureFlagStore {
	return newFeatureFlag(ds)
}

func (ds *datastore) Roles() store.RoleStore {
	return newRole(ds)
}

func (ds *datastore) Permissions() store.PermissionStore {
	return newPermission(ds)
}

func (ds *datastore) UserRoles() store.UserRoleStore {
	return newUserRole(ds)
}

func (ds *datastore) AIChat() store.AIChatStore {
	return newAIChat(ds)
}
