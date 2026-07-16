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

const taskCenterActiveLeaseIndexSQL = `
CREATE UNIQUE INDEX IF NOT EXISTS idx_task_execution_leases_active_run
ON task_execution_leases(run_id)
WHERE status IN ('ACTIVE', 'RENEWED')
`

const taskCenterApplicationRunIndexesSQL = `
CREATE INDEX IF NOT EXISTS idx_task_runs_application
ON task_runs(application_run_id)
WHERE application_run_id <> '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_task_runs_application_idempotency
ON task_runs(application_run_id, idempotency_key)
WHERE application_run_id <> '' AND idempotency_key <> ''
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
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_template_source') THEN ALTER TABLE aiapp_application_templates ADD CONSTRAINT ck_aiapp_template_source CHECK (capability_source_type IN ('comfyui_workflow','provider_capability')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_aiapp_template_version_template') THEN ALTER TABLE aiapp_application_template_versions ADD CONSTRAINT fk_aiapp_template_version_template FOREIGN KEY (application_template_id) REFERENCES aiapp_application_templates(id); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_template_version_number') THEN ALTER TABLE aiapp_application_template_versions ADD CONSTRAINT ck_aiapp_template_version_number CHECK (version > 0); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_template_version_status') THEN ALTER TABLE aiapp_application_template_versions ADD CONSTRAINT ck_aiapp_template_version_status CHECK (status IN ('draft','published','retired')); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_template_version_published_at') THEN ALTER TABLE aiapp_application_template_versions ADD CONSTRAINT ck_aiapp_template_version_published_at CHECK ((status='published' AND published_at IS NOT NULL) OR status<>'published'); END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_template_version_source') THEN ALTER TABLE aiapp_application_template_versions ADD CONSTRAINT ck_aiapp_template_version_source CHECK ((capability_source_type='provider_capability' AND provider_capability_id IS NOT NULL AND provider_capability_revision IS NOT NULL AND provider_operation_id IS NOT NULL AND workflow_contract_revision IS NULL AND comfyui_api_workflow_json IS NULL AND comfyui_object_info_json IS NULL) OR (capability_source_type='comfyui_workflow' AND provider_capability_id IS NULL AND provider_capability_revision IS NULL AND provider_operation_id IS NULL AND workflow_contract_revision IS NOT NULL AND comfyui_api_workflow_json IS NOT NULL AND comfyui_object_info_json IS NOT NULL)); END IF;
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
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_aiapp_run_task_creation') THEN ALTER TABLE aiapp_application_runs ADD CONSTRAINT ck_aiapp_run_task_creation CHECK ((task_creation_status='created' AND task_run_id IS NOT NULL AND task_status_projection IS NOT NULL) OR (task_creation_status IN ('pending','failed') AND task_run_id IS NULL AND task_status_projection IS NULL)); END IF;
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
	return nil
}

func (ds *datastore) ensureTaskCenterScheme() error {
	if err := ds.db.Exec(taskCenterActiveLeaseIndexSQL).Error; err != nil {
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

func (ds *datastore) TaskCenters() store.TaskCenterStore {
	return newTaskCenter(ds)
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
