package store

import (
	"context"

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

// WorkflowCanvasStore owns spec-v1.0.0 canvas drafts, immutable versions, and run projections.
type WorkflowCanvasStore interface {
	ListWorkflowCanvases(context.Context, *iapiserver.WorkflowCanvasListRequest, string, string, string) ([]*iapiserver.WorkflowCanvas, int64, error)
	GetWorkflowCanvas(context.Context, string) (*iapiserver.WorkflowCanvas, error)
	AddWorkflowCanvas(context.Context, *iapiserver.WorkflowCanvas) (*iapiserver.WorkflowCanvas, error)
	UpdateWorkflowCanvas(context.Context, *iapiserver.WorkflowCanvas, int64) (*iapiserver.WorkflowCanvas, error)
	DeleteWorkflowCanvas(context.Context, string) error
	PublishWorkflowCanvas(context.Context, *iapiserver.WorkflowCanvas, *iapiserver.CanvasVersion, int64) (*iapiserver.CanvasVersion, error)
	ListCanvasVersions(context.Context, *iapiserver.CanvasVersionListRequest) ([]*iapiserver.CanvasVersion, int64, error)
	GetCanvasVersion(context.Context, string) (*iapiserver.CanvasVersion, error)
	ListWorkflowCanvasRuns(context.Context, *iapiserver.WorkflowCanvasRunListRequest, string, string, string) ([]*iapiserver.WorkflowCanvasRun, int64, error)
	GetWorkflowCanvasRun(context.Context, string) (*iapiserver.WorkflowCanvasRun, error)
	AddWorkflowCanvasRunIdempotent(context.Context, *iapiserver.WorkflowCanvasRun) (*iapiserver.WorkflowCanvasRun, bool, error)
	BindWorkflowCanvasRun(context.Context, string, string, []*iapiserver.CanvasNodeRun) (*iapiserver.WorkflowCanvasRun, error)
	UpdateWorkflowCanvasRun(context.Context, *iapiserver.WorkflowCanvasRun) (*iapiserver.WorkflowCanvasRun, error)
	ListCanvasNodeRuns(context.Context, *iapiserver.CanvasNodeRunListRequest) ([]*iapiserver.CanvasNodeRun, int64, error)
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
	AddWithUploadEvent(ctx context.Context, data *iapiserver.Asset, thumbnail *iapiserver.AssetThumbnail, event map[string]any) (*iapiserver.Asset, *iapiserver.AssetThumbnail, error)
	Update(ctx context.Context, data *iapiserver.Asset) (*iapiserver.Asset, error)
	// Delete marks the asset as deleted. It does not remove asset objects, thumbnails, or relation rows.
	Delete(ctx context.Context, id string) error
}

type AssetV1Store interface {
	RegisterArtifact(context.Context, *iapiserver.ArtifactRegistrationRequest) (*iapiserver.UserAsset, bool, error)
	ApplyLabels(context.Context, string, string, map[string]string, []string, []string) (*iapiserver.BatchLabelData, error)
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

type TaskCenterStore interface {
	ListAtomicTasks(context.Context, *iapiserver.AtomicTaskListRequest) ([]*iapiserver.AtomicTask, int64, error)
	// GetAtomicTasksByIDs 批量读取调度历史引用的 AtomicTask，不用于绕过 service 权限返回完整资源。
	GetAtomicTasksByIDs(context.Context, []string) ([]*iapiserver.AtomicTask, error)
	GetAtomicTask(context.Context, string) (*iapiserver.AtomicTask, error)
	AddAtomicTaskIdempotent(context.Context, *iapiserver.AtomicTask) (*iapiserver.AtomicTask, bool, error)
	UpdateAtomicTask(context.Context, *iapiserver.AtomicTask) (*iapiserver.AtomicTask, error)
	ListAttempts(ctx context.Context, req *iapiserver.TaskAttemptListRequest) ([]*iapiserver.TaskAttempt, int64, error)
	ListTaskGroups(context.Context, *iapiserver.TaskGroupListRequest) ([]*iapiserver.TaskGroup, int64, error)
	// GetTaskGroupsByIDs 批量读取调度历史引用的 TaskGroup。
	GetTaskGroupsByIDs(context.Context, []string) ([]*iapiserver.TaskGroup, error)
	GetTaskGroup(context.Context, string) (*iapiserver.TaskGroup, error)
	AddTaskGroupWithTasks(context.Context, *iapiserver.TaskGroup, []*iapiserver.AtomicTask) (*iapiserver.TaskGroup, bool, error)
	UpdateTaskGroup(context.Context, *iapiserver.TaskGroup) (*iapiserver.TaskGroup, error)
	ListDAGTaskGroups(context.Context, *iapiserver.DAGTaskGroupListRequest) ([]*iapiserver.DAGTaskGroup, int64, error)
	// GetDAGTaskGroupsByIDs 批量读取调度历史引用的 DAGTaskGroup。
	GetDAGTaskGroupsByIDs(context.Context, []string) ([]*iapiserver.DAGTaskGroup, error)
	GetDAGTaskGroup(context.Context, string) (*iapiserver.DAGTaskGroup, error)
	AddDAGTaskGroupWithTasks(context.Context, *iapiserver.DAGTaskGroup, []*iapiserver.AtomicTask) (*iapiserver.DAGTaskGroup, bool, error)
	UpdateDAGTaskGroup(context.Context, *iapiserver.DAGTaskGroup) (*iapiserver.DAGTaskGroup, error)
	ListOwnedTasks(context.Context, string, string, *iapiserver.AtomicTaskListRequest) ([]*iapiserver.AtomicTask, int64, error)
	AddOwnedAtomicTasks(context.Context, string, string, []*iapiserver.AtomicTask) error
	ListTaskSchedules(context.Context, *iapiserver.TaskScheduleListRequest) ([]*iapiserver.TaskSchedule, int64, error)
	GetTaskSchedule(context.Context, string) (*iapiserver.TaskSchedule, error)
	AddTaskSchedule(context.Context, *iapiserver.TaskSchedule) (*iapiserver.TaskSchedule, error)
	UpdateTaskSchedule(context.Context, *iapiserver.TaskSchedule) (*iapiserver.TaskSchedule, error)
	ListScheduleExecutions(context.Context, *iapiserver.ScheduleExecutionListRequest) ([]*iapiserver.TaskScheduleExecution, int64, error)
	// ListScheduleSources 按目标类型与 ID 批量返回最新的来源调度轮次。
	ListScheduleSources(context.Context, string, []string) (map[string]*iapiserver.ScheduleSourceSummary, error)
	AddProjectionEventIdempotent(context.Context, *iapiserver.RuntimeProjectionEvent) (*iapiserver.RuntimeProjectionEvent, bool, error)
	ListNonTerminalAtomicTasks(context.Context, int) ([]*iapiserver.AtomicTask, error)
	ListNonTerminalTaskGroups(context.Context, int) ([]*iapiserver.TaskGroup, error)
	ListNonTerminalDAGTaskGroups(context.Context, int) ([]*iapiserver.DAGTaskGroup, error)
	ListActiveScheduleExecutions(context.Context, int) ([]*iapiserver.TaskScheduleExecution, error)
	ApplyRuntimeProjection(context.Context, *iapiserver.AtomicTask, []*iapiserver.TaskAttempt, *iapiserver.RuntimeProjectionEvent) (bool, error)
	AcquireScheduleExecution(context.Context, *iapiserver.TaskScheduleExecution) (*iapiserver.TaskScheduleExecution, bool, error)
	UpdateScheduleExecution(context.Context, *iapiserver.TaskScheduleExecution) (*iapiserver.TaskScheduleExecution, error)
}

type ApplicationPlatformStore interface {
	ListEngineInstances(ctx context.Context, req *iapiserver.EngineInstanceListRequest) ([]*iapiserver.EngineInstance, int64, error)
	GetEngineInstance(ctx context.Context, id string) (*iapiserver.EngineInstance, error)
	AddEngineInstance(ctx context.Context, data *iapiserver.EngineInstance) (*iapiserver.EngineInstance, error)
	UpdateEngineInstance(ctx context.Context, data *iapiserver.EngineInstance, expectedVersion int64) (*iapiserver.EngineInstance, error)
	DeleteEngineInstance(ctx context.Context, id string) error
	CountRunsByEngineInstance(ctx context.Context, id string) (int64, error)
	ListComfyUIWorkflows(ctx context.Context, req *iapiserver.ComfyUIWorkflowListRequest) ([]*iapiserver.ComfyUIWorkflow, int64, error)
	GetComfyUIWorkflow(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflow, error)
	AddComfyUIWorkflow(ctx context.Context, data *iapiserver.ComfyUIWorkflow) (*iapiserver.ComfyUIWorkflow, error)
	UpdateComfyUIWorkflow(ctx context.Context, data *iapiserver.ComfyUIWorkflow, expectedVersion int64) (*iapiserver.ComfyUIWorkflow, error)
	ListComfyUIWorkflowDuplicateIDs(ctx context.Context, ownerUserID, checksum string) ([]string, error)
	ListComfyUIWorkflowValidations(ctx context.Context, req *iapiserver.ComfyUIWorkflowValidationListRequest) ([]*iapiserver.ComfyUIWorkflowValidation, int64, error)
	GetComfyUIWorkflowValidation(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflowValidation, error)
	AddComfyUIWorkflowValidation(ctx context.Context, data *iapiserver.ComfyUIWorkflowValidation) (*iapiserver.ComfyUIWorkflowValidation, error)
	ListComfyUIWorkflowTestRuns(ctx context.Context, req *iapiserver.ComfyUIWorkflowTestRunListRequest) ([]*iapiserver.ComfyUIWorkflowTestRun, int64, error)
	GetComfyUIWorkflowTestRun(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflowTestRun, error)
	GetComfyUIWorkflowTestRunByIdempotency(ctx context.Context, ownerUserID, key string) (*iapiserver.ComfyUIWorkflowTestRun, error)
	AddComfyUIWorkflowTestRun(ctx context.Context, data *iapiserver.ComfyUIWorkflowTestRun) (*iapiserver.ComfyUIWorkflowTestRun, error)
	UpdateComfyUIWorkflowTestRun(ctx context.Context, data *iapiserver.ComfyUIWorkflowTestRun) (*iapiserver.ComfyUIWorkflowTestRun, error)
	ConvertComfyUIWorkflow(ctx context.Context, workflowID, ownerUserID, actorUserID, idempotencyKey string, template *iapiserver.ApplicationTemplate, version *iapiserver.ApplicationTemplateVersion) (*iapiserver.ComfyUIWorkflowConvertResult, error)
	ListEngineBindings(ctx context.Context, req *iapiserver.EngineCapabilityBindingListRequest) ([]*iapiserver.EngineCapabilityBinding, int64, error)
	GetEngineBinding(ctx context.Context, id string) (*iapiserver.EngineCapabilityBinding, error)
	AddEngineBinding(ctx context.Context, data *iapiserver.EngineCapabilityBinding) (*iapiserver.EngineCapabilityBinding, error)
	UpdateEngineBinding(ctx context.Context, data *iapiserver.EngineCapabilityBinding, expectedVersion int64) (*iapiserver.EngineCapabilityBinding, error)
	DeleteEngineBinding(ctx context.Context, id string) error
	ListTemplates(ctx context.Context, req *iapiserver.ApplicationTemplateListRequest) ([]*iapiserver.ApplicationTemplate, int64, error)
	GetTemplate(ctx context.Context, id string) (*iapiserver.ApplicationTemplate, error)
	AddTemplateWithVersion(ctx context.Context, data *iapiserver.ApplicationTemplate, version *iapiserver.ApplicationTemplateVersion) (*iapiserver.ApplicationTemplate, error)
	ListTemplateVersions(ctx context.Context, req *iapiserver.ApplicationTemplateVersionListRequest) ([]*iapiserver.ApplicationTemplateVersion, int64, error)
	GetTemplateVersion(ctx context.Context, id string) (*iapiserver.ApplicationTemplateVersion, error)
	AddTemplateVersion(ctx context.Context, data *iapiserver.ApplicationTemplateVersion) (*iapiserver.ApplicationTemplateVersion, error)
	PublishTemplateVersion(ctx context.Context, id string) (*iapiserver.ApplicationTemplateVersion, error)
	ListApplications(ctx context.Context, req *iapiserver.ApplicationListRequest) ([]*iapiserver.Application, int64, error)
	GetApplication(ctx context.Context, id string) (*iapiserver.Application, error)
	AddApplication(ctx context.Context, data *iapiserver.Application) (*iapiserver.Application, error)
	UpdateApplication(ctx context.Context, data *iapiserver.Application, expectedVersion int64) (*iapiserver.Application, error)
	ListApplicationVersions(ctx context.Context, req *iapiserver.ApplicationVersionListRequest) ([]*iapiserver.ApplicationVersion, int64, error)
	GetApplicationVersion(ctx context.Context, id string) (*iapiserver.ApplicationVersion, error)
	AddApplicationVersion(ctx context.Context, data *iapiserver.ApplicationVersion) (*iapiserver.ApplicationVersion, error)
	PublishApplicationVersion(ctx context.Context, id string) (*iapiserver.ApplicationVersion, error)
	GetApplicationRun(ctx context.Context, id string) (*iapiserver.ApplicationRun, error)
	GetApplicationRunByIdempotency(ctx context.Context, ownerUserID, key string) (*iapiserver.ApplicationRun, error)
	AddApplicationRun(ctx context.Context, data *iapiserver.ApplicationRun) (*iapiserver.ApplicationRun, error)
	BindApplicationRunTask(ctx context.Context, id, atomicTaskID, status string, taskVersion int64, failure string) (*iapiserver.ApplicationRun, error)
	ProjectApplicationRun(ctx context.Context, id string, taskVersion int64, status, failure string, outputs []map[string]any) (*iapiserver.ApplicationRun, error)
	ListArtifactsByRun(ctx context.Context, runID string) ([]*iapiserver.ApplicationArtifact, error)
	UpsertArtifact(ctx context.Context, data *iapiserver.ApplicationArtifact) (*iapiserver.ApplicationArtifact, error)
	UpdateArtifactRegistration(ctx context.Context, id, status, assetID, errorCode, failureDetail string, expectedVersion int64) (*iapiserver.ApplicationArtifact, error)
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
