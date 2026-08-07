package store

var client Factory

// Factory defines the iam platform storage interface.
type Factory interface {
	// legacy users retained as compatibility projections for services not yet migrated to PrincipalContext.
	Users() UserStore

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
	WorkflowCanvases() WorkflowCanvasStore

	// platform contracts
	Providers() ProviderStore
	ProviderModels() ProviderModelStore
	ModelHealthChecks() ModelHealthCheckStore
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
	UserEvents() UserEventStore
	ApplicationPlatforms() ApplicationPlatformStore
	FeatureFlags() FeatureFlagStore
	Permissions() PermissionStore

	// Identity 认证、会话和授权事实。
	Identities() IdentityStore
	// Platform Management 系统认证配置与审计事实。
	PlatformManagement() PlatformManagementStore
	// ai chat
	AIChat() AIChatStore
	// Agents 返回 released Agent domain 的持久化边界。
	Agents() AgentStore
	AppStudio() AppStudioStore
	Infrastructure() InfrastructureStore

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
