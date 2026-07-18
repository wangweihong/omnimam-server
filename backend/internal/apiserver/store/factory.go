package store

var client Factory

// Factory defines the iam platform storage interface.
type Factory interface {
	// settings
	IdentityProviders() IdentityProviderStore
	ServiceProviders() ServiceProviderStore
	Settings() SettingStore

	// identitys
	Users() UserStore
	OneTimeTokens() OneTimeTokenStore
	UserOTPs() UserOTPStore

	// assets
	AssetLibraries() AssetLibraryStore
	AssetCategories() AssetCategoryStore
	AssetItems() AssetItemStore

	// prompts
	PromptLibraries() PromptLibraryStore
	PromptCategories() PromptCategoryStore
	PromptItems() PromptItemStore

	// canvases
	Projects() ProjectStore
	Canvases() CanvasStore
	WorkflowCanvases() WorkflowCanvasStore

	// platform contracts
	Providers() ProviderStore
	ProviderModels() ProviderModelStore
	ProviderCapabilities() ProviderCapabilityStore
	SystemLLMConfigs() SystemLLMConfigStore
	StorageBackends() StorageBackendStore
	AssetsV2() AssetStore
	AssetThumbnails() AssetThumbnailStore
	Tags() TagStore
	AssetTags() AssetTagStore
	AssetGroups() AssetGroupStore
	AssetGroupMembers() AssetGroupMemberStore
	AssetRelations() AssetRelationStore
	AssetsV1() AssetV1Store
	TaskCenters() TaskCenterStore
	ApplicationPlatforms() ApplicationPlatformStore
	FeatureFlags() FeatureFlagStore
	Roles() RoleStore
	Permissions() PermissionStore
	UserRoles() UserRoleStore

	// ai chat
	AIChat() AIChatStore

	EnsureScheme(metaTypes ...any) error
	Close() error
}

// Client return the store client instance.
func Client() Factory {
	return client
}

// SetClient set the iam store client.
func SetClient(factory Factory) {
	client = factory
}
