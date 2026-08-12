package postgresql

import (
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
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

const identityRegistrationConstraintsSQL = `
CREATE UNIQUE INDEX IF NOT EXISTS uq_identity_registration_pending_user
ON identity_registration_applications(user_id)
WHERE status = 'PENDING';
CREATE INDEX IF NOT EXISTS idx_identity_registration_applications_status
ON identity_registration_applications(status, submitted_at DESC);
`

const taskCenterFunctionContractSQL = `
CREATE INDEX IF NOT EXISTS idx_atomic_tasks_function_contract
ON atomic_tasks(function_ref, function_contract_version);
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_atomic_tasks_function_contract') THEN
    ALTER TABLE atomic_tasks ADD CONSTRAINT ck_atomic_tasks_function_contract CHECK (
      function_ref NOT IN (
        'agent.runtime.ensure',
        'agent.runtime.stop',
        'agent.invocation.execute',
        'appstudio.preview.ensure',
        'appstudio.preview.stop',
        'appstudio.build.execute',
        'appstudio.production.reconcile',
        'appstudio.production.stop'
      ) OR (
        function_contract_version <> '' AND
        function_contract_digest ~ '^sha256:[0-9a-f]{64}$'
      )
    );
  END IF;
END $$;
`

const gitLabConstraintsSQL = `
ALTER TABLE gitlab_projects ALTER COLUMN external_project_id DROP NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_gitlab_servers_name ON gitlab_servers(name);
CREATE INDEX IF NOT EXISTS idx_gitlab_servers_status_updated_at ON gitlab_servers(status, updated_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_gitlab_servers_appstudio_default ON gitlab_servers(is_appstudio_default) WHERE is_appstudio_default = TRUE;
CREATE INDEX IF NOT EXISTS idx_gitlab_projects_server_created_at ON gitlab_projects(gitlab_server_id, created_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_studio_source_repositories_gitlab_project ON studio_source_repositories(gitlab_project_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_studio_workspace_revisions_commit_sha ON studio_workspace_revisions(workspace_id, commit_sha);
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_gitlab_servers_status') THEN
    ALTER TABLE gitlab_servers ADD CONSTRAINT ck_gitlab_servers_status
      CHECK (status IN ('UNKNOWN', 'READY', 'ERROR'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_gitlab_servers_appstudio_default') THEN
    ALTER TABLE gitlab_servers ADD CONSTRAINT ck_gitlab_servers_appstudio_default
      CHECK (is_appstudio_default = FALSE OR status = 'READY');
  END IF;
	IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_gitlab_projects_status') THEN
		ALTER TABLE gitlab_projects ADD CONSTRAINT ck_gitlab_projects_status
			CHECK (status IN ('CREATING', 'READY', 'ERROR'));
	END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_studio_source_repositories_provider_type') THEN
    ALTER TABLE studio_source_repositories ADD CONSTRAINT ck_studio_source_repositories_provider_type
      CHECK (provider_type = 'GITLAB');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_gitlab_projects_server') THEN
    ALTER TABLE gitlab_projects ADD CONSTRAINT fk_gitlab_projects_server
      FOREIGN KEY (gitlab_server_id) REFERENCES gitlab_servers(id) ON DELETE RESTRICT;
  END IF;
END $$;
`

const agentConstraintsSQL = `
CREATE INDEX IF NOT EXISTS idx_agents_owner_status ON agents(owner_user_id, status, created_at);
CREATE INDEX IF NOT EXISTS idx_agents_workspace ON agents(workspace_type, workspace_id);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_agent_status ON agent_sessions(agent_id, status, updated_at);
CREATE INDEX IF NOT EXISTS idx_agent_messages_session_created ON agent_messages(session_id, created_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_invocations_idempotency ON agent_invocations(agent_id, idempotency_key);
CREATE INDEX IF NOT EXISTS idx_agent_invocations_session_status ON agent_invocations(session_id, status, created_at);
CREATE INDEX IF NOT EXISTS idx_agent_invocations_task ON agent_invocations(atomic_task_id);
CREATE INDEX IF NOT EXISTS idx_agent_memories_scope ON agent_memories(agent_id, scope, updated_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_model_bindings_identity ON agent_model_bindings(agent_id, purpose, name);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_workspace_binding_agent ON agent_workspace_bindings(agent_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_skill_binding_agent_skill ON agent_skill_bindings(agent_id, skill_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_runtime_current ON agent_runtime_bindings(agent_id)
WHERE state NOT IN ('DELETED', 'STOPPED', 'FAILED');
CREATE INDEX IF NOT EXISTS idx_agent_runtime_current_task ON agent_runtime_bindings(current_task_id) WHERE current_task_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_operation_event_sequence ON agent_operation_events(invocation_id, sequence_no);
CREATE INDEX IF NOT EXISTS idx_agent_outbox_delivery ON agent_outbox(delivery_status, next_attempt_at);
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agents_kind') THEN
    ALTER TABLE agents ADD CONSTRAINT ck_agents_kind CHECK (kind IN ('platform','coding'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agents_workspace_kind') THEN
    ALTER TABLE agents ADD CONSTRAINT ck_agents_workspace_kind CHECK ((kind='platform' AND workspace_type='agent') OR (kind='coding' AND workspace_type='studio'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agents_status') THEN
    ALTER TABLE agents ADD CONSTRAINT ck_agents_status CHECK (status IN ('CREATING','READY','STARTING','RUNNING','IDLE','SUSPENDED','ERROR','DISABLED','DELETING'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_sessions_status') THEN
    ALTER TABLE agent_sessions ADD CONSTRAINT ck_agent_sessions_status CHECK (status IN ('OPEN','CLOSED','ARCHIVED'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_messages_role') THEN
    ALTER TABLE agent_messages ADD CONSTRAINT ck_agent_messages_role CHECK (role IN ('USER','ASSISTANT','SYSTEM','TOOL'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_invocations_type') THEN
    ALTER TABLE agent_invocations ADD CONSTRAINT ck_agent_invocations_type CHECK (type IN ('CHAT','CODING','TOOL_OPERATION','BACKGROUND_OPERATION'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_invocations_status') THEN
    ALTER TABLE agent_invocations ADD CONSTRAINT ck_agent_invocations_status CHECK (status IN ('QUEUED','STARTING','RUNNING','WAITING_FOR_TOOL','WAITING_FOR_USER','SUCCEEDED','FAILED','CANCELING','CANCELED'));
  END IF;
  ALTER TABLE agent_invocations ADD COLUMN IF NOT EXISTS runtime_binding_id TEXT;
  ALTER TABLE agent_invocations ADD COLUMN IF NOT EXISTS runtime_session_ref TEXT;
  ALTER TABLE agent_invocations ADD COLUMN IF NOT EXISTS runtime_invocation_ref TEXT;
  ALTER TABLE agent_invocations ADD COLUMN IF NOT EXISTS last_event_sequence INTEGER NOT NULL DEFAULT 0;
  ALTER TABLE agent_invocations ADD COLUMN IF NOT EXISTS submission_generation INTEGER NOT NULL DEFAULT 0;
  ALTER TABLE agent_invocations ADD COLUMN IF NOT EXISTS task_expected_resource_version INTEGER;
  ALTER TABLE agent_invocations ADD COLUMN IF NOT EXISTS terminal_projected_task_id TEXT;
  ALTER TABLE agent_invocations ADD COLUMN IF NOT EXISTS terminal_projected_at TIMESTAMPTZ;
  CREATE INDEX IF NOT EXISTS idx_agent_invocations_runtime_binding ON agent_invocations(runtime_binding_id) WHERE runtime_binding_id IS NOT NULL;
  IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_invocations_task') THEN
    ALTER TABLE agent_invocations DROP CONSTRAINT ck_agent_invocations_task;
  END IF;
  UPDATE agent_invocations
  SET status = 'FAILED',
      failure_code = 'ERR_AGENT_INVOCATION_TASK_UNAVAILABLE',
      failure_message = COALESCE(NULLIF(failure_message, ''), 'invocation task unavailable during contract migration')
  WHERE atomic_task_id IS NULL
    AND status <> 'QUEUED'
    AND NOT (status = 'FAILED' AND failure_code = 'ERR_AGENT_INVOCATION_TASK_UNAVAILABLE');
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_invocations_task_binding') THEN
    ALTER TABLE agent_invocations ADD CONSTRAINT ck_agent_invocations_task_binding CHECK (atomic_task_id IS NOT NULL OR status = 'QUEUED' OR (status = 'FAILED' AND failure_code = 'ERR_AGENT_INVOCATION_TASK_UNAVAILABLE'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_invocations_task_resource_version') THEN
    ALTER TABLE agent_invocations ADD CONSTRAINT ck_agent_invocations_task_resource_version CHECK (atomic_task_id IS NULL OR task_expected_resource_version IS NOT NULL);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_invocations_terminal_task') THEN
    ALTER TABLE agent_invocations ADD CONSTRAINT ck_agent_invocations_terminal_task CHECK (terminal_projected_task_id IS NULL OR terminal_projected_task_id = atomic_task_id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_invocations_sequences') THEN
    ALTER TABLE agent_invocations ADD CONSTRAINT ck_agent_invocations_sequences CHECK (last_event_sequence >= 0 AND submission_generation >= 0);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_memories_scope') THEN
    ALTER TABLE agent_memories ADD CONSTRAINT ck_agent_memories_scope CHECK (scope IN ('AGENT','SESSION'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_memories_type') THEN
    ALTER TABLE agent_memories ADD CONSTRAINT ck_agent_memories_type CHECK (type IN ('FACT','PREFERENCE','SUMMARY','INSTRUCTION','CONTEXT'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_memories_session') THEN
    ALTER TABLE agent_memories ADD CONSTRAINT ck_agent_memories_session CHECK ((scope='AGENT' AND session_id IS NULL) OR (scope='SESSION' AND session_id IS NOT NULL));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_model_bindings_source') THEN
    ALTER TABLE agent_model_bindings ADD CONSTRAINT ck_agent_model_bindings_source CHECK (source_type IN ('USER_DEFAULT_MODEL','USER_PROVIDER_MODEL','PLATFORM_MODEL'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_model_bindings_purpose') THEN
    ALTER TABLE agent_model_bindings ADD CONSTRAINT ck_agent_model_bindings_purpose CHECK (purpose IN ('CHAT','CODING','VISION','EMBEDDING'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_model_bindings_status') THEN
    ALTER TABLE agent_model_bindings ADD CONSTRAINT ck_agent_model_bindings_status CHECK (status IN ('ACTIVE','INVALID','DISABLED'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_workspace_bindings_type') THEN
    ALTER TABLE agent_workspace_bindings ADD CONSTRAINT ck_agent_workspace_bindings_type CHECK (workspace_type IN ('agent','studio'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_workspace_bindings_access') THEN
    ALTER TABLE agent_workspace_bindings ADD CONSTRAINT ck_agent_workspace_bindings_access CHECK (access_mode IN ('READ_ONLY','READ_WRITE'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_mcp_bindings_type') THEN
    ALTER TABLE agent_mcp_bindings ADD CONSTRAINT ck_agent_mcp_bindings_type CHECK (server_type IN ('PLATFORM','REMOTE','RUNTIME_LOCAL'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_runtime_bindings_state') THEN
    ALTER TABLE agent_runtime_bindings ADD CONSTRAINT ck_agent_runtime_bindings_state CHECK (state IN ('CREATING','STARTING','READY','RUNNING','STOPPING','STOPPED','FAILED','DELETED'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_runtime_bindings_activity') THEN
    ALTER TABLE agent_runtime_bindings ADD CONSTRAINT ck_agent_runtime_bindings_activity CHECK (activity_state IN ('IDLE','ACTIVE','SUSPENDED'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_runtime_bindings_health') THEN
    ALTER TABLE agent_runtime_bindings ADD CONSTRAINT ck_agent_runtime_bindings_health CHECK (health_status IN ('UNKNOWN','HEALTHY','UNHEALTHY'));
  END IF;
  ALTER TABLE agent_runtime_bindings ADD COLUMN IF NOT EXISTS current_task_id TEXT;
  ALTER TABLE agent_runtime_bindings ADD COLUMN IF NOT EXISTS current_operation TEXT;
  CREATE INDEX IF NOT EXISTS idx_agent_runtime_current_task ON agent_runtime_bindings(current_task_id) WHERE current_task_id IS NOT NULL;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_runtime_bindings_current_operation') THEN
    ALTER TABLE agent_runtime_bindings ADD CONSTRAINT ck_agent_runtime_bindings_current_operation CHECK (current_operation IS NULL OR current_operation IN ('START','RECOVER','SUSPEND','STOP','DELETE'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_agent_outbox_delivery') THEN
    ALTER TABLE agent_outbox ADD CONSTRAINT ck_agent_outbox_delivery CHECK (delivery_status IN ('PENDING','DELIVERED','FAILED'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_agent_sessions_agent') THEN
    ALTER TABLE agent_sessions ADD CONSTRAINT fk_agent_sessions_agent FOREIGN KEY (agent_id) REFERENCES agents(id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_agent_messages_session') THEN
    ALTER TABLE agent_messages ADD CONSTRAINT fk_agent_messages_session FOREIGN KEY (session_id) REFERENCES agent_sessions(id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_agent_messages_agent') THEN
    ALTER TABLE agent_messages ADD CONSTRAINT fk_agent_messages_agent FOREIGN KEY (agent_id) REFERENCES agents(id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_agent_invocations_agent') THEN
    ALTER TABLE agent_invocations ADD CONSTRAINT fk_agent_invocations_agent FOREIGN KEY (agent_id) REFERENCES agents(id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_agent_invocations_session') THEN
    ALTER TABLE agent_invocations ADD CONSTRAINT fk_agent_invocations_session FOREIGN KEY (session_id) REFERENCES agent_sessions(id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_agent_memories_agent') THEN
    ALTER TABLE agent_memories ADD CONSTRAINT fk_agent_memories_agent FOREIGN KEY (agent_id) REFERENCES agents(id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_agent_memories_session') THEN
    ALTER TABLE agent_memories ADD CONSTRAINT fk_agent_memories_session FOREIGN KEY (session_id) REFERENCES agent_sessions(id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_agent_model_bindings_agent') THEN
    ALTER TABLE agent_model_bindings ADD CONSTRAINT fk_agent_model_bindings_agent FOREIGN KEY (agent_id) REFERENCES agents(id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_agent_workspace_bindings_agent') THEN
    ALTER TABLE agent_workspace_bindings ADD CONSTRAINT fk_agent_workspace_bindings_agent FOREIGN KEY (agent_id) REFERENCES agents(id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_agent_skill_bindings_agent') THEN
    ALTER TABLE agent_skill_bindings ADD CONSTRAINT fk_agent_skill_bindings_agent FOREIGN KEY (agent_id) REFERENCES agents(id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_agent_mcp_bindings_agent') THEN
    ALTER TABLE agent_mcp_bindings ADD CONSTRAINT fk_agent_mcp_bindings_agent FOREIGN KEY (agent_id) REFERENCES agents(id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_agent_runtime_bindings_agent') THEN
    ALTER TABLE agent_runtime_bindings ADD CONSTRAINT fk_agent_runtime_bindings_agent FOREIGN KEY (agent_id) REFERENCES agents(id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_agent_operation_events_invocation') THEN
    ALTER TABLE agent_operation_events ADD CONSTRAINT fk_agent_operation_events_invocation FOREIGN KEY (invocation_id) REFERENCES agent_invocations(id);
  END IF;
END $$;
`

const appStudioConstraintsSQL = `
ALTER TABLE studio_applications ADD COLUMN IF NOT EXISTS coding_agent_id TEXT;
ALTER TABLE studio_applications ADD COLUMN IF NOT EXISTS coding_session_id TEXT;
ALTER TABLE studio_applications ADD COLUMN IF NOT EXISTS coding_agent_generation INTEGER NOT NULL DEFAULT 0;
ALTER TABLE studio_applications ADD COLUMN IF NOT EXISTS create_idempotency_key TEXT;
UPDATE studio_applications
SET create_idempotency_key = 'legacy:' || id
WHERE create_idempotency_key IS NULL OR create_idempotency_key = '';
ALTER TABLE studio_applications ALTER COLUMN create_idempotency_key SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_studio_applications_owner_create_key
ON studio_applications(owner_user_id, create_idempotency_key);
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_studio_applications_coding_agent_generation') THEN
    ALTER TABLE studio_applications ADD CONSTRAINT ck_studio_applications_coding_agent_generation
      CHECK (coding_agent_generation >= 0);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_studio_applications_coding_agent_binding') THEN
    ALTER TABLE studio_applications ADD CONSTRAINT ck_studio_applications_coding_agent_binding CHECK (
      (coding_agent_id IS NULL AND coding_session_id IS NULL) OR
      (coding_agent_id IS NOT NULL AND coding_session_id IS NOT NULL AND coding_agent_generation > 0)
    );
  END IF;
END $$;
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

const taskCenterDAGObservabilityConstraintsSQL = `
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
CREATE UNIQUE INDEX IF NOT EXISTS idx_atomic_tasks_owner_child
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
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_template_version_source') THEN ALTER TABLE aiapp_application_template_versions ADD CONSTRAINT ck_aiapp_template_version_source CHECK ((capability_source_type='provider_capability' AND provider_capability_id IS NOT NULL AND provider_capability_revision IS NOT NULL AND provider_operation_id IS NOT NULL AND workflow_contract_revision IS NULL AND comfyui_api_workflow_json IS NULL) OR (capability_source_type='comfyui_workflow' AND provider_capability_id IS NULL AND provider_capability_revision IS NULL AND provider_operation_id IS NULL AND workflow_contract_revision IS NOT NULL AND comfyui_api_workflow_json IS NOT NULL)); END IF;
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

const applicationPlatformBindingConstraintSQL = `
DO $$ BEGIN
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
	if err := ds.preflightAgentMCPBindingNames(); err != nil {
		return err
	}
	if err := ds.db.AutoMigrate(metaTypes...); err != nil {
		return err
	}
	if err := ds.db.Exec(gitLabConstraintsSQL).Error; err != nil {
		return err
	}
	if err := ds.db.Exec(identityRegistrationConstraintsSQL).Error; err != nil {
		return err
	}
	if err := ds.ensureAssetLibraryScheme(); err != nil {
		return err
	}
	if err := ds.ensureTaskCenterScheme(); err != nil {
		return err
	}
	if err := ds.ensureAgentScheme(); err != nil {
		return err
	}
	if err := ds.ensureAppStudioScheme(); err != nil {
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

func (ds *datastore) preflightAgentMCPBindingNames() error {
	migrator := ds.db.Migrator()
	if !migrator.HasTable(&iapiserver.AgentMCPBinding{}) {
		return nil
	}
	activePredicate := ""
	if migrator.HasColumn(&iapiserver.AgentMCPBinding{}, "deleted_at") {
		activePredicate = " WHERE deleted_at IS NULL"
	}
	var conflict struct {
		AgentID string
		Name    string
	}
	result := ds.db.Raw(`SELECT agent_id, name FROM agent_mcp_bindings` + activePredicate + ` GROUP BY agent_id, name HAVING COUNT(*) > 1 LIMIT 1`).Scan(&conflict)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return fmt.Errorf("agent MCP binding migration blocked: agent %q has duplicate active binding name %q", conflict.AgentID, conflict.Name)
	}
	return nil
}

func (ds *datastore) ensureAgentScheme() error {
	if err := ds.db.Exec(agentConstraintsSQL).Error; err != nil {
		return err
	}
	if err := ds.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_mcp_bindings_active_name ON agent_mcp_bindings(agent_id, name) WHERE deleted_at IS NULL;`).Error; err != nil {
		return err
	}
	if err := ds.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_mcp_binding_revision_unique ON agent_mcp_binding_revisions(binding_id, binding_revision);`).Error; err != nil {
		return err
	}
	if err := ds.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_runtime_grants_request ON agent_runtime_grants(runtime_binding_id, request_id);`).Error; err != nil {
		return err
	}
	var bindings []iapiserver.AgentMCPBinding
	if err := ds.db.Where("NOT EXISTS (SELECT 1 FROM agent_mcp_binding_revisions r WHERE r.binding_id = agent_mcp_bindings.id AND r.binding_revision = agent_mcp_bindings.resource_version)").Find(&bindings).Error; err != nil {
		return err
	}
	for i := range bindings {
		b := &bindings[i]
		if err := ds.db.Create(&iapiserver.AgentMCPBindingRevision{ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString(), Name: b.Name}, BindingID: b.ID, BindingRevision: b.ResourceVersion, AgentID: b.AgentID, ServerType: b.ServerType, EndpointRef: b.EndpointRef, CredentialRef: b.CredentialRef, AllowedTools: b.AllowedTools, Configuration: b.Configuration, Enabled: b.Enabled}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (ds *datastore) ensureAppStudioScheme() error {
	return ds.db.Exec(appStudioConstraintsSQL).Error
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
	if err := ds.db.Exec(taskCenterFunctionContractSQL).Error; err != nil {
		return err
	}
	if err := ds.db.Exec(taskCenterActiveScheduleIndexSQL).Error; err != nil {
		return err
	}
	if err := ds.db.Exec(taskCenterApplicationRunIndexesSQL).Error; err != nil {
		return err
	}
	if err := ds.db.Exec(sseUserEventConstraintsSQL).Error; err != nil {
		return err
	}
	return ds.db.Exec(taskCenterDAGObservabilityConstraintsSQL).Error
}

func (ds *datastore) ensureApplicationPlatformScheme() error {
	if err := ds.db.Exec(applicationPlatformConstraintsSQL).Error; err != nil {
		return err
	}
	return ds.db.Exec(applicationPlatformBindingConstraintSQL).Error
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

func (ds *datastore) ModelHealthChecks() store.ModelHealthCheckStore {
	return newModelHealthCheck(ds)
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

func (ds *datastore) Agents() store.AgentStore                  { return newAgentStore(ds) }
func (ds *datastore) AppStudio() store.AppStudioStore           { return newAppStudioStore(ds) }
func (ds *datastore) Infrastructure() store.InfrastructureStore { return newInfrastructureStore(ds) }
func (ds *datastore) GitLab() store.GitLabStore                 { return newGitLabStore(ds) }

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

func (ds *datastore) Permissions() store.PermissionStore {
	return newPermission(ds)
}

func (ds *datastore) AIChat() store.AIChatStore {
	return newAIChat(ds)
}
