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
WHERE owner_type <> '' AND child_key <> '';
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

const applicationPlatformLegacySchemaSQL = `
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='aiapp_applications' AND column_name='template_id')
     OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='aiapp_application_runs' AND column_name='run_mode') THEN
    DROP TABLE IF EXISTS aiapp_input_mappings, aiapp_output_mappings, aiapp_app_templates,
      aiapp_app_engines, aiapp_application_runs, aiapp_applications CASCADE;
  END IF;
END $$;
DROP TABLE IF EXISTS aiapp_input_mappings, aiapp_output_mappings, aiapp_app_templates, aiapp_app_engines CASCADE;
`

const applicationPlatformConstraintsSQL = `
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_engine_instances_name ON aiapp_engine_instances(name);
CREATE INDEX IF NOT EXISTS idx_aiapp_engine_instances_type_health ON aiapp_engine_instances(application_engine_type_id, enabled, health_status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_binding_engine_capability ON aiapp_engine_capability_bindings(engine_instance_id, provider_capability_id);
CREATE INDEX IF NOT EXISTS idx_aiapp_binding_capability ON aiapp_engine_capability_bindings(provider_capability_id, enabled);
CREATE INDEX IF NOT EXISTS idx_aiapp_comfyui_workflows_owner_created ON aiapp_comfyui_workflows(owner_user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_aiapp_comfyui_workflows_filters ON aiapp_comfyui_workflows(owner_user_id, lifecycle_status, parse_status, latest_validation_status);
CREATE INDEX IF NOT EXISTS idx_aiapp_comfyui_workflows_checksum ON aiapp_comfyui_workflows(owner_user_id, workflow_checksum);
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_comfyui_workflows_conversion_key ON aiapp_comfyui_workflows(owner_user_id, conversion_idempotency_key) WHERE conversion_idempotency_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_comfyui_workflows_converted_template ON aiapp_comfyui_workflows(converted_application_template_id) WHERE converted_application_template_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_aiapp_comfyui_validations_workflow_created ON aiapp_comfyui_workflow_validations(workflow_id, created_at);
CREATE INDEX IF NOT EXISTS idx_aiapp_comfyui_validations_engine_status ON aiapp_comfyui_workflow_validations(engine_instance_id, status, validated_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_templates_owner_name ON aiapp_application_templates(owner_user_id, name);
CREATE INDEX IF NOT EXISTS idx_aiapp_templates_capability ON aiapp_application_templates(capability_definition_id, capability_source_type);
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_template_versions_number ON aiapp_application_template_versions(application_template_id, version);
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_applications_owner_name ON aiapp_applications(owner_user_id, name);
CREATE INDEX IF NOT EXISTS idx_aiapp_applications_owner_visibility ON aiapp_applications(owner_user_id, visibility);
CREATE INDEX IF NOT EXISTS idx_aiapp_applications_capability_run ON aiapp_applications(capability_definition_id, run_enabled);
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_application_versions_semver ON aiapp_application_versions(application_id, semantic_version);
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_runs_owner_idempotency ON aiapp_application_runs(owner_user_id, idempotency_key);
CREATE INDEX IF NOT EXISTS idx_aiapp_runs_application_created ON aiapp_application_runs(application_id, created_at);
CREATE INDEX IF NOT EXISTS idx_aiapp_runs_engine_created ON aiapp_application_runs(engine_instance_id, created_at);
CREATE INDEX IF NOT EXISTS idx_aiapp_runs_capability_revision ON aiapp_application_runs(provider_capability_id, provider_capability_revision);
CREATE UNIQUE INDEX IF NOT EXISTS idx_aiapp_artifacts_run_output ON aiapp_artifacts(application_run_id, output_key);
CREATE INDEX IF NOT EXISTS idx_aiapp_artifacts_registration ON aiapp_artifacts(registration_status, updated_at);

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_engine_auth_type') THEN ALTER TABLE aiapp_engine_instances ADD CONSTRAINT ck_aiapp_engine_auth_type CHECK (auth_type IN ('none','api_key','bearer_token','ak_sk')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_engine_health') THEN ALTER TABLE aiapp_engine_instances ADD CONSTRAINT ck_aiapp_engine_health CHECK (health_status IN ('unknown','online','offline','degraded')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_engine_limits') THEN ALTER TABLE aiapp_engine_instances ADD CONSTRAINT ck_aiapp_engine_limits CHECK (max_concurrency > 0 AND request_timeout_seconds > 0 AND task_timeout_seconds > 0); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_binding_engine') THEN ALTER TABLE aiapp_engine_capability_bindings ADD CONSTRAINT fk_aiapp_binding_engine FOREIGN KEY (engine_instance_id) REFERENCES aiapp_engine_instances(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_comfyui_workflow_source_engine') THEN ALTER TABLE aiapp_comfyui_workflows ADD CONSTRAINT fk_aiapp_comfyui_workflow_source_engine FOREIGN KEY (source_engine_instance_id) REFERENCES aiapp_engine_instances(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_comfyui_workflow_parse') THEN ALTER TABLE aiapp_comfyui_workflows ADD CONSTRAINT ck_aiapp_comfyui_workflow_parse CHECK (parse_status IN ('fully_supported','partially_supported','manual_configuration_required','unsupported')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_comfyui_workflow_checksums') THEN ALTER TABLE aiapp_comfyui_workflows ADD CONSTRAINT ck_aiapp_comfyui_workflow_checksums CHECK (workflow_checksum ~ '^sha256:[0-9a-f]{64}$' AND import_object_info_checksum ~ '^sha256:[0-9a-f]{64}$'); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_comfyui_workflow_latest_validation') THEN ALTER TABLE aiapp_comfyui_workflows ADD CONSTRAINT ck_aiapp_comfyui_workflow_latest_validation CHECK (latest_validation_status IN ('not_validated','compatible','incompatible','failed')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_comfyui_workflow_lifecycle') THEN ALTER TABLE aiapp_comfyui_workflows ADD CONSTRAINT ck_aiapp_comfyui_workflow_lifecycle CHECK ((lifecycle_status='active' AND archived_at IS NULL AND archived_by_user_id IS NULL) OR (lifecycle_status='archived' AND archived_at IS NOT NULL AND archived_by_user_id IS NOT NULL)); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_comfyui_workflow_conversion') THEN ALTER TABLE aiapp_comfyui_workflows ADD CONSTRAINT ck_aiapp_comfyui_workflow_conversion CHECK ((converted_application_template_id IS NULL AND converted_template_version_id IS NULL AND conversion_idempotency_key IS NULL AND converted_at IS NULL AND converted_by_user_id IS NULL) OR (converted_application_template_id IS NOT NULL AND converted_template_version_id IS NOT NULL AND conversion_idempotency_key IS NOT NULL AND converted_at IS NOT NULL AND converted_by_user_id IS NOT NULL)); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_comfyui_validation_workflow') THEN ALTER TABLE aiapp_comfyui_workflow_validations ADD CONSTRAINT fk_aiapp_comfyui_validation_workflow FOREIGN KEY (workflow_id) REFERENCES aiapp_comfyui_workflows(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_comfyui_validation_engine') THEN ALTER TABLE aiapp_comfyui_workflow_validations ADD CONSTRAINT fk_aiapp_comfyui_validation_engine FOREIGN KEY (engine_instance_id) REFERENCES aiapp_engine_instances(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_comfyui_validation_status') THEN ALTER TABLE aiapp_comfyui_workflow_validations ADD CONSTRAINT ck_aiapp_comfyui_validation_status CHECK ((status IN ('compatible','incompatible') AND object_info_json IS NOT NULL AND object_info_checksum IS NOT NULL) OR status='failed'); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_comfyui_validation_checksum') THEN ALTER TABLE aiapp_comfyui_workflow_validations ADD CONSTRAINT ck_aiapp_comfyui_validation_checksum CHECK (object_info_checksum IS NULL OR object_info_checksum ~ '^sha256:[0-9a-f]{64}$'); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_template_source') THEN ALTER TABLE aiapp_application_templates ADD CONSTRAINT ck_aiapp_template_source CHECK (capability_source_type IN ('comfyui_workflow','provider_capability')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_template_version_template') THEN ALTER TABLE aiapp_application_template_versions ADD CONSTRAINT fk_aiapp_template_version_template FOREIGN KEY (application_template_id) REFERENCES aiapp_application_templates(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_template_version_number') THEN ALTER TABLE aiapp_application_template_versions ADD CONSTRAINT ck_aiapp_template_version_number CHECK (version > 0); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_template_version_status') THEN ALTER TABLE aiapp_application_template_versions ADD CONSTRAINT ck_aiapp_template_version_status CHECK (status IN ('draft','published','retired')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_template_version_published_at') THEN ALTER TABLE aiapp_application_template_versions ADD CONSTRAINT ck_aiapp_template_version_published_at CHECK ((status='published' AND published_at IS NOT NULL) OR status<>'published'); END IF;
  ALTER TABLE aiapp_application_template_versions DROP CONSTRAINT IF EXISTS ck_aiapp_template_version_source;
  ALTER TABLE aiapp_application_template_versions ADD CONSTRAINT ck_aiapp_template_version_source CHECK ((capability_source_type='provider_capability' AND provider_capability_id IS NOT NULL AND provider_capability_revision IS NOT NULL AND provider_operation_id IS NOT NULL AND workflow_contract_revision IS NULL AND comfyui_api_workflow_json IS NULL AND comfyui_object_info_json IS NULL AND comfyui_dependencies_json IS NULL) OR (capability_source_type='comfyui_workflow' AND provider_capability_id IS NULL AND provider_capability_revision IS NULL AND provider_operation_id IS NULL AND workflow_contract_revision IS NOT NULL AND comfyui_api_workflow_json IS NOT NULL AND comfyui_object_info_json IS NOT NULL AND comfyui_dependencies_json IS NOT NULL));
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_template_version_source_workflow') THEN ALTER TABLE aiapp_application_template_versions ADD CONSTRAINT fk_aiapp_template_version_source_workflow FOREIGN KEY (source_comfyui_workflow_id) REFERENCES aiapp_comfyui_workflows(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_template_version_source_validation') THEN ALTER TABLE aiapp_application_template_versions ADD CONSTRAINT fk_aiapp_template_version_source_validation FOREIGN KEY (source_workflow_validation_id) REFERENCES aiapp_comfyui_workflow_validations(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_template_current_version') THEN ALTER TABLE aiapp_application_templates ADD CONSTRAINT fk_aiapp_template_current_version FOREIGN KEY (current_version_id) REFERENCES aiapp_application_template_versions(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_comfyui_workflow_converted_template') THEN ALTER TABLE aiapp_comfyui_workflows ADD CONSTRAINT fk_aiapp_comfyui_workflow_converted_template FOREIGN KEY (converted_application_template_id) REFERENCES aiapp_application_templates(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_comfyui_workflow_converted_version') THEN ALTER TABLE aiapp_comfyui_workflows ADD CONSTRAINT fk_aiapp_comfyui_workflow_converted_version FOREIGN KEY (converted_template_version_id) REFERENCES aiapp_application_template_versions(id); END IF;
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
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_artifact_run') THEN ALTER TABLE aiapp_artifacts ADD CONSTRAINT fk_aiapp_artifact_run FOREIGN KEY (application_run_id) REFERENCES aiapp_application_runs(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_artifact_media') THEN ALTER TABLE aiapp_artifacts ADD CONSTRAINT ck_aiapp_artifact_media CHECK (media_type IN ('image','video','audio','text','pdf','other')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_artifact_registration') THEN ALTER TABLE aiapp_artifacts ADD CONSTRAINT ck_aiapp_artifact_registration CHECK ((registration_status='registered' AND asset_id IS NOT NULL) OR (registration_status IN ('pending','failed') AND asset_id IS NULL)); END IF;
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
	if err := ds.ensureTaskCenterScheme(); err != nil {
		return err
	}
	if err := ds.ensureApplicationPlatformScheme(); err != nil {
		return err
	}
	if err := ds.ensureOutboxScheme(); err != nil {
		return err
	}
	return nil
}

func (ds *datastore) ensureTaskCenterScheme() error {
	if err := ds.db.Exec(taskCenterActiveScheduleIndexSQL).Error; err != nil {
		return err
	}
	return ds.db.Exec(taskCenterApplicationRunIndexesSQL).Error
}

func (ds *datastore) ensureApplicationPlatformScheme() error {
	return ds.db.Exec(applicationPlatformConstraintsSQL).Error
}

func (ds *datastore) prepareApplicationPlatformScheme() error {
	return ds.db.Exec(applicationPlatformLegacySchemaSQL).Error
}

func (ds *datastore) Users() store.UserStore {
	return newUser(ds)
}

/* ------ setting ------- */
func (ds *datastore) IdentityProviders() store.IdentityProviderStore {
	return newIdentityProvider(ds)
}

func (ds *datastore) ServiceProviders() store.ServiceProviderStore {
	return newServiceProvider(ds)
}

func (ds *datastore) Settings() store.SettingStore {
	return newSetting(ds)
}

func (ds *datastore) OneTimeTokens() store.OneTimeTokenStore {
	return newOneTimeToken(ds)
}

func (ds *datastore) UserOTPs() store.UserOTPStore {
	return newUserOTP(ds)
}

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

func (ds *datastore) ApplicationPlatforms() store.ApplicationPlatformStore {
	return newApplicationPlatform(ds)
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
