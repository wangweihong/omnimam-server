package store

import (
	"context"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type IdentityProviderStore interface {
	List(
		ctx context.Context,
		req *iapiserver.IdentityProviderListRequest,
	) ([]*iapiserver.IdentityProvider, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.IdentityProvider, error)
	GetByName(ctx context.Context, name string) (*iapiserver.IdentityProvider, error)
	Delete(ctx context.Context, id string) error
	Update(ctx context.Context, data *iapiserver.IdentityProvider) (*iapiserver.IdentityProvider, error)
	Sync(ctx context.Context, datas []*iapiserver.IdentityProvider) error
	Add(ctx context.Context, data *iapiserver.IdentityProvider) (*iapiserver.IdentityProvider, error)
}

type ServiceProviderStore interface {
	List(ctx context.Context, req *iapiserver.ServiceProviderListRequest) ([]*iapiserver.ServiceProvider, int64, error)
	Add(ctx context.Context, data *iapiserver.ServiceProvider) (*iapiserver.ServiceProvider, error)
	Delete(ctx context.Context, id string) error
	Get(ctx context.Context, id string) (*iapiserver.ServiceProvider, error)
	GetByKey(ctx context.Context, protocol, key string) (*iapiserver.ServiceProvider, error)
	GetByName(ctx context.Context, name string) (*iapiserver.ServiceProvider, error)
	Update(ctx context.Context, data *iapiserver.ServiceProvider) (*iapiserver.ServiceProvider, error)
	Sync(ctx context.Context, datas []*iapiserver.ServiceProvider) error
}

type SettingStore interface {
	List(ctx context.Context) ([]*iapiserver.Setting, error)
	Delete(ctx context.Context, id string) error
	Get(ctx context.Context, id string) (*iapiserver.Setting, error)
	GetByName(ctx context.Context, name string) (*iapiserver.Setting, error)
	GetMultiByNames(ctx context.Context, names ...string) ([]*iapiserver.Setting, error)
	Upsert(ctx context.Context, data *iapiserver.Setting) (*iapiserver.Setting, error)
	FirstOrCreate(ctx context.Context, data *iapiserver.Setting) (*iapiserver.Setting, error)
}

type UserStore interface {
	List(ctx context.Context, req *iapiserver.UserListRequest) ([]*iapiserver.User, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.User, error)
	GetByName(ctx context.Context, name string) (*iapiserver.User, error)
	Delete(ctx context.Context, id string) error
	Update(ctx context.Context, data *iapiserver.User) (*iapiserver.User, error)
	Sync(ctx context.Context, datas []*iapiserver.User) error
	Add(ctx context.Context, data *iapiserver.User) (*iapiserver.User, error)
}

type OneTimeTokenStore interface {
	GetByHash(ctx context.Context, hash string) (*iapiserver.OneTimeToken, error)
	Delete(ctx context.Context, id string) error
	Add(ctx context.Context, data *iapiserver.OneTimeToken) (*iapiserver.OneTimeToken, error)
	CleanupExpiredTokens(ctx context.Context) error
}

type UserOTPStore interface {
	List(ctx context.Context) ([]*iapiserver.UserOTP, error)
	Delete(ctx context.Context, id string) error
	GetByUser(ctx context.Context, uid string) (*iapiserver.UserOTP, error)
	Upsert(ctx context.Context, data *iapiserver.UserOTP) (*iapiserver.UserOTP, error)
	FirstOrCreate(ctx context.Context, data *iapiserver.UserOTP) (*iapiserver.UserOTP, error)
	Add(ctx context.Context, data *iapiserver.UserOTP) (*iapiserver.UserOTP, error)
}

type AssetLibraryStore interface {
	List(ctx context.Context, req *iapiserver.AssetLibraryListRequest) ([]*iapiserver.AssetLibrary, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.AssetLibrary, error)
	Add(ctx context.Context, data *iapiserver.AssetLibrary) (*iapiserver.AssetLibrary, error)
	Update(ctx context.Context, data *iapiserver.AssetLibrary) (*iapiserver.AssetLibrary, error)
	Delete(ctx context.Context, id string) error
}

type AssetCategoryStore interface {
	List(ctx context.Context, req *iapiserver.AssetCategoryListRequest) ([]*iapiserver.AssetCategory, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.AssetCategory, error)
	Add(ctx context.Context, data *iapiserver.AssetCategory) (*iapiserver.AssetCategory, error)
	Update(ctx context.Context, data *iapiserver.AssetCategory) (*iapiserver.AssetCategory, error)
	Delete(ctx context.Context, id string, libraryID string) error
	DeleteByLibraryID(ctx context.Context, libraryID string) error
}

type AssetItemStore interface {
	List(ctx context.Context, req *iapiserver.AssetItemListRequest) ([]*iapiserver.AssetItem, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.AssetItem, error)
	Add(ctx context.Context, data *iapiserver.AssetItem) (*iapiserver.AssetItem, error)
	BatchAdd(ctx context.Context, items []*iapiserver.AssetItem) ([]*iapiserver.AssetItem, error)
	Update(ctx context.Context, data *iapiserver.AssetItem) (*iapiserver.AssetItem, error)
	Delete(ctx context.Context, id string) error
	BatchDelete(ctx context.Context, ids []string, libraryID string) (int, error)
	BatchMove(ctx context.Context, ids []string, targetLibraryID, targetCategoryID string) (int, error)
	FindByIDs(ctx context.Context, ids []string, libraryID string) ([]*iapiserver.AssetItem, error)
}

type PromptLibraryStore interface {
	List(ctx context.Context) ([]*iapiserver.PromptLibrary, error)
	Get(ctx context.Context, id string) (*iapiserver.PromptLibrary, error)
	Add(ctx context.Context, data *iapiserver.PromptLibrary) (*iapiserver.PromptLibrary, error)
	Update(ctx context.Context, data *iapiserver.PromptLibrary) (*iapiserver.PromptLibrary, error)
	Delete(ctx context.Context, id string) error
	SetActive(ctx context.Context, id string) error
	GetActive(ctx context.Context) (*iapiserver.PromptLibrary, error)
}

type PromptCategoryStore interface {
	ListByLibrary(ctx context.Context, libraryID string) ([]*iapiserver.PromptCategory, error)
	Get(ctx context.Context, id string) (*iapiserver.PromptCategory, error)
	Add(ctx context.Context, data *iapiserver.PromptCategory) (*iapiserver.PromptCategory, error)
	Update(ctx context.Context, data *iapiserver.PromptCategory) (*iapiserver.PromptCategory, error)
	Delete(ctx context.Context, id string, libraryID string) error
	DeleteByLibraryID(ctx context.Context, libraryID string) error
}

type PromptItemStore interface {
	ListByLibrary(ctx context.Context, libraryID string) ([]*iapiserver.PromptItem, error)
	Get(ctx context.Context, id string) (*iapiserver.PromptItem, error)
	Add(ctx context.Context, data *iapiserver.PromptItem) (*iapiserver.PromptItem, error)
	Update(ctx context.Context, data *iapiserver.PromptItem) (*iapiserver.PromptItem, error)
	Delete(ctx context.Context, id string) error
	BatchDelete(ctx context.Context, ids []string) (int, error)
	ReassignCategory(ctx context.Context, oldCategoryID, newCategoryID string) error
}

type ProjectStore interface {
	List(ctx context.Context) ([]*iapiserver.Project, error)
	Get(ctx context.Context, id string) (*iapiserver.Project, error)
	Add(ctx context.Context, data *iapiserver.Project) (*iapiserver.Project, error)
	Update(ctx context.Context, data *iapiserver.Project) (*iapiserver.Project, error)
	Delete(ctx context.Context, id string) error
}

type CanvasStore interface {
	List(ctx context.Context, includeDeleted bool) ([]*iapiserver.Canvas, error)
	ListByProject(ctx context.Context, projectID string) ([]*iapiserver.Canvas, error)
	Get(ctx context.Context, id string) (*iapiserver.Canvas, error)
	GetAny(ctx context.Context, id string) (*iapiserver.Canvas, error)
	Add(ctx context.Context, data *iapiserver.Canvas) (*iapiserver.Canvas, error)
	Update(ctx context.Context, data *iapiserver.Canvas) (*iapiserver.Canvas, error)
	SoftDelete(ctx context.Context, id string) error
	Restore(ctx context.Context, id string) error
	Purge(ctx context.Context, id string) error
	CountByProject(ctx context.Context, projectID string) (int, error)
	ReassignProject(ctx context.Context, oldProjectID, newProjectID string) (int, error)
	CleanupExpiredTrash(ctx context.Context, retentionDays int) error
}

type ProviderStore interface {
	List(ctx context.Context, req *iapiserver.ProviderListRequest) ([]*iapiserver.Provider, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.Provider, error)
	Add(ctx context.Context, data *iapiserver.Provider) (*iapiserver.Provider, error)
	Update(ctx context.Context, data *iapiserver.Provider) (*iapiserver.Provider, error)
	// Delete removes one provider record by id.
	Delete(ctx context.Context, id string) error
}

type ProviderModelStore interface {
	List(ctx context.Context, req *iapiserver.ProviderModelListRequest) ([]*iapiserver.ProviderModel, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.ProviderModel, error)
	Add(ctx context.Context, data *iapiserver.ProviderModel) (*iapiserver.ProviderModel, error)
	Update(ctx context.Context, data *iapiserver.ProviderModel) (*iapiserver.ProviderModel, error)
	// Delete 删除指定模型提供商下的一个模型元数据。
	Delete(ctx context.Context, providerID, id string) error
	// DeleteByProviderID removes all models under one provider.
	DeleteByProviderID(ctx context.Context, providerID string) error
}

type ProviderCapabilityStore interface {
	List(ctx context.Context) ([]*iapiserver.ProviderCapability, error)
	Add(ctx context.Context, data *iapiserver.ProviderCapability) (*iapiserver.ProviderCapability, error)
}

type SystemLLMConfigStore interface {
	List(ctx context.Context) ([]*iapiserver.SystemLLMConfig, error)
	Upsert(ctx context.Context, data *iapiserver.SystemLLMConfig) (*iapiserver.SystemLLMConfig, error)
	// DeleteByProviderModelID 删除引用指定模型的默认模型绑定。
	DeleteByProviderModelID(ctx context.Context, providerID, modelID string) error
	// DeleteByProviderID removes all default model bindings under one provider.
	DeleteByProviderID(ctx context.Context, providerID string) error
}

type StorageBackendStore interface {
	List(ctx context.Context, req *iapiserver.StorageBackendListRequest) ([]*iapiserver.StorageBackend, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.StorageBackend, error)
	Add(ctx context.Context, data *iapiserver.StorageBackend) (*iapiserver.StorageBackend, error)
	Update(ctx context.Context, data *iapiserver.StorageBackend) (*iapiserver.StorageBackend, error)
	GetDefaultLocal(ctx context.Context) (*iapiserver.StorageBackend, error)
}

type AssetStore interface {
	List(ctx context.Context, req *iapiserver.AssetListRequest) ([]*iapiserver.Asset, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.Asset, error)
	Add(ctx context.Context, data *iapiserver.Asset) (*iapiserver.Asset, error)
	Update(ctx context.Context, data *iapiserver.Asset) (*iapiserver.Asset, error)
	// Delete marks the asset as deleted. It does not remove asset objects, thumbnails, or relation rows.
	Delete(ctx context.Context, id string) error
}

type AssetThumbnailStore interface {
	GetByAsset(ctx context.Context, assetID string) (*iapiserver.AssetThumbnail, error)
	ListByAssetIDs(ctx context.Context, assetIDs []string) ([]*iapiserver.AssetThumbnail, error)
	Add(ctx context.Context, data *iapiserver.AssetThumbnail) (*iapiserver.AssetThumbnail, error)
	Update(ctx context.Context, data *iapiserver.AssetThumbnail) (*iapiserver.AssetThumbnail, error)
	// DeleteByAsset removes thumbnail metadata for one asset after the preview object is removed.
	DeleteByAsset(ctx context.Context, assetID string) error
}

type TagStore interface {
	ListByAssetIDs(ctx context.Context, assetIDs []string) (map[string][]*iapiserver.Tag, error)
	GetByName(ctx context.Context, name string, source string) (*iapiserver.Tag, error)
	FirstOrCreate(ctx context.Context, data *iapiserver.Tag) (*iapiserver.Tag, error)
}

type AssetTagStore interface {
	Replace(ctx context.Context, assetID string, tags []*iapiserver.Tag, source string) error
	ListTagNames(ctx context.Context, assetID string) ([]string, error)
	// DeleteByAsset removes tag links for one asset without deleting reusable tag records.
	DeleteByAsset(ctx context.Context, assetID string) error
}

type AIChatGenerationBundle struct {
	UserMessage      *iapiserver.AIChatMessage
	AssistantMessage *iapiserver.AIChatMessage
	Generation       *iapiserver.AIChatGeneration
}

type AIChatStore interface {
	ListModels(ctx context.Context, ownerUserID string, req *iapiserver.AIChatModelListRequest) ([]*iapiserver.AIChatModel, int64, error)
	GetModel(ctx context.Context, ownerUserID, id string) (*iapiserver.AIChatModel, error)
	GetDefaultTranslationModel(ctx context.Context, ownerUserID string) (*iapiserver.AIChatModel, error)

	ListAssistants(ctx context.Context, ownerUserID string) ([]*iapiserver.AIChatAssistant, error)
	GetAssistant(ctx context.Context, ownerUserID, id string) (*iapiserver.AIChatAssistant, error)
	CreateAssistant(ctx context.Context, ownerUserID string, data *iapiserver.AIChatAssistant) (*iapiserver.AIChatAssistant, error)
	UpdateAssistant(ctx context.Context, ownerUserID string, data *iapiserver.AIChatAssistant) (*iapiserver.AIChatAssistant, error)
	DeleteAssistant(ctx context.Context, ownerUserID, id string) error

	ListTopics(ctx context.Context, ownerUserID string, req *iapiserver.AIChatTopicListRequest) ([]*iapiserver.AIChatTopic, int64, error)
	GetTopic(ctx context.Context, ownerUserID, id string) (*iapiserver.AIChatTopic, error)
	CreateTopic(ctx context.Context, ownerUserID string, data *iapiserver.AIChatTopic) (*iapiserver.AIChatTopic, error)
	UpdateTopic(ctx context.Context, ownerUserID string, data *iapiserver.AIChatTopic) (*iapiserver.AIChatTopic, error)
	DeleteTopic(ctx context.Context, ownerUserID, id string) error
	BranchTopic(ctx context.Context, ownerUserID, messageID string) (*iapiserver.AIChatTopic, error)

	ListMessages(ctx context.Context, ownerUserID, topicID string) ([]*iapiserver.AIChatMessage, error)
	GetMessage(ctx context.Context, ownerUserID, id string) (*iapiserver.AIChatMessage, error)
	CreateMessageGeneration(
		ctx context.Context,
		ownerUserID string,
		topic *iapiserver.AIChatTopic,
		req *iapiserver.AIChatMessageCreateRequest,
		model *iapiserver.AIChatModel,
		assistant *iapiserver.AIChatAssistant,
	) (*AIChatGenerationBundle, error)
	CreateEditRegenerateGeneration(
		ctx context.Context,
		ownerUserID string,
		source *iapiserver.AIChatMessage,
		req *iapiserver.AIChatEditRegenerateRequest,
		model *iapiserver.AIChatModel,
		assistant *iapiserver.AIChatAssistant,
	) (*AIChatGenerationBundle, error)
	CompleteGeneration(ctx context.Context, ownerUserID, generationID, content string) (*iapiserver.AIChatGeneration, error)
	FailGeneration(ctx context.Context, ownerUserID, generationID, errorCode, errorMessage string) error
	StopGeneration(ctx context.Context, ownerUserID, generationID string) (*iapiserver.AIChatGeneration, error)

	ListQuickPhrases(ctx context.Context, ownerUserID string, req *iapiserver.AIChatQuickPhraseListRequest) ([]*iapiserver.AIChatQuickPhrase, error)
	GetQuickPhrase(ctx context.Context, ownerUserID, id string) (*iapiserver.AIChatQuickPhrase, error)
	CreateQuickPhrase(ctx context.Context, ownerUserID string, data *iapiserver.AIChatQuickPhrase) (*iapiserver.AIChatQuickPhrase, error)
	UpdateQuickPhrase(ctx context.Context, ownerUserID string, data *iapiserver.AIChatQuickPhrase) (*iapiserver.AIChatQuickPhrase, error)
	DeleteQuickPhrase(ctx context.Context, ownerUserID, id string) error
	CreateTranslation(ctx context.Context, translation *iapiserver.AIChatMessageTranslation) (*iapiserver.AIChatMessageTranslation, error)
}

type AssetGroupStore interface {
	Add(ctx context.Context, data *iapiserver.AssetGroup) (*iapiserver.AssetGroup, error)
}

type AssetGroupMemberStore interface {
	BatchAdd(ctx context.Context, members []*iapiserver.AssetGroupMember) ([]*iapiserver.AssetGroupMember, error)
	// DeleteByAsset removes an asset from all groups before the asset metadata is deleted.
	DeleteByAsset(ctx context.Context, assetID string) error
}

type AssetRelationStore interface {
	Add(ctx context.Context, data *iapiserver.AssetRelation) (*iapiserver.AssetRelation, error)
	// DeleteByAsset removes derivation relations where the asset is either source or target.
	DeleteByAsset(ctx context.Context, assetID string) error
}

type TaskStore interface {
	List(ctx context.Context, req *iapiserver.TaskListRequest) ([]*iapiserver.Task, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.Task, error)
	Add(ctx context.Context, data *iapiserver.Task) (*iapiserver.Task, error)
	Update(ctx context.Context, data *iapiserver.Task) (*iapiserver.Task, error)
	Cancel(ctx context.Context, id string) (*iapiserver.Task, error)
	Claim(ctx context.Context, queue, worker string, limit int, lease time.Duration) ([]*iapiserver.Task, error)
}

type TaskCenterStore interface {
	ListDefinitions(ctx context.Context, req *iapiserver.TaskDefinitionListRequest) ([]*iapiserver.TaskDefinition, int64, error)
	GetDefinition(ctx context.Context, definitionType, id string) (*iapiserver.TaskDefinition, error)
	AddDefinition(ctx context.Context, data *iapiserver.TaskDefinition) (*iapiserver.TaskDefinition, error)
	ListRuns(ctx context.Context, req *iapiserver.TaskRunListRequest) ([]*iapiserver.TaskRun, int64, error)
	GetRun(ctx context.Context, id string) (*iapiserver.TaskRun, error)
	AddRun(ctx context.Context, data *iapiserver.TaskRun) (*iapiserver.TaskRun, error)
	UpdateRun(ctx context.Context, data *iapiserver.TaskRun) (*iapiserver.TaskRun, error)
	SoftDeleteRun(ctx context.Context, id string) error
	ListAttempts(ctx context.Context, req *iapiserver.TaskAttemptListRequest) ([]*iapiserver.TaskAttempt, int64, error)
	RegisterWorker(ctx context.Context, data *iapiserver.Worker) (*iapiserver.Worker, error)
	HeartbeatWorker(ctx context.Context, req *iapiserver.WorkerHeartbeatRequest) (*iapiserver.Worker, error)
	ClaimRun(ctx context.Context, req *iapiserver.ClaimTaskRunRequest) (*iapiserver.ClaimTaskRunResponse, error)
	UpdateProgress(ctx context.Context, req *iapiserver.ProgressUpdateRequest) (*iapiserver.TaskRun, error)
	CompleteRun(ctx context.Context, req *iapiserver.TaskRunCompleteRequest) (*iapiserver.TaskRun, error)
	FailRun(ctx context.Context, req *iapiserver.TaskRunFailRequest) (*iapiserver.TaskRun, error)
	RenewLease(ctx context.Context, req *iapiserver.LeaseRenewRequest) (*iapiserver.ExecutionLease, error)
	Health(ctx context.Context) (*iapiserver.TaskCenterHealth, error)
	AddEvent(ctx context.Context, data *iapiserver.TaskRunEvent) (*iapiserver.TaskRunEvent, error)
}

type ApplicationPlatformStore interface {
	ListTemplates(ctx context.Context, req *iapiserver.AppTemplateListRequest) ([]*iapiserver.AppTemplate, int64, error)
	GetTemplate(ctx context.Context, id string) (*iapiserver.AppTemplate, error)
	GetTemplateByOwnerName(ctx context.Context, ownerUserID, name string) (*iapiserver.AppTemplate, error)
	AddTemplate(ctx context.Context, data *iapiserver.AppTemplate) (*iapiserver.AppTemplate, error)
	UpdateTemplate(ctx context.Context, data *iapiserver.AppTemplate) (*iapiserver.AppTemplate, error)
	DeleteTemplate(ctx context.Context, id string) error
	ListApplications(ctx context.Context, req *iapiserver.ApplicationListRequest) ([]*iapiserver.Application, int64, error)
	GetApplication(ctx context.Context, id string) (*iapiserver.Application, error)
	AddApplication(
		ctx context.Context,
		data *iapiserver.Application,
		mappings []*iapiserver.FieldMapping,
	) (*iapiserver.Application, error)
	UpdateApplication(ctx context.Context, data *iapiserver.Application) (*iapiserver.Application, error)
	DeleteApplication(ctx context.Context, id string) error
	ListFieldMappings(ctx context.Context, applicationID string) ([]*iapiserver.FieldMapping, error)
	ReplaceFieldMappings(
		ctx context.Context,
		applicationID string,
		mappings []*iapiserver.FieldMapping,
	) ([]*iapiserver.FieldMapping, error)
}

type FeatureFlagStore interface {
	List(ctx context.Context) ([]*iapiserver.FeatureFlag, error)
	Upsert(ctx context.Context, data *iapiserver.FeatureFlag) (*iapiserver.FeatureFlag, error)
}

type RoleStore interface {
	List(ctx context.Context) ([]*iapiserver.Role, error)
}

type PermissionStore interface {
	List(ctx context.Context) ([]*iapiserver.Permission, error)
}

type UserRoleStore interface {
	ListByUser(ctx context.Context, userID string) ([]*iapiserver.UserRole, error)
}
