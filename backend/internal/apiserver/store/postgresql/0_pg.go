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
	if err := ds.db.AutoMigrate(metaTypes...); err != nil {
		return err
	}
	if err := ds.ensureTaskCenterScheme(); err != nil {
		return err
	}
	return nil
}

func (ds *datastore) ensureTaskCenterScheme() error {
	return ds.db.Exec(taskCenterActiveLeaseIndexSQL).Error
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
