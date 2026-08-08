package store

import (
	"context"
	"errors"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

var (
	// ErrRequiredEngineBindingFailed 标识 EngineInstance 已写入但同事务内的系统必需绑定写入失败。
	ErrRequiredEngineBindingFailed = errors.New("required engine binding failed")
	// ErrAssetDeleteBlocked 标识素材仍有强引用，永久删除事务不得产生任何副作用。
	ErrAssetDeleteBlocked = errors.New("asset delete blocked")
	// ErrNotificationNotVisible 统一隐藏通知不存在和接收者不匹配。
	ErrNotificationNotVisible = errors.New("notification not visible")
	// ErrNotificationStateConflict 表示收件箱状态机拒绝当前操作。
	ErrNotificationStateConflict = errors.New("notification state conflict")
)

// IdentityRegistrationApplicationView 是注册申请及对应用户的最小管理视图。
type IdentityRegistrationApplicationView struct {
	Application *iapiserver.IdentityRegistrationApplication
	User        *iapiserver.IdentityUser
}

// AgentInvocationTerminalProjection 是 Task Center 终态观察者允许写回的 Invocation 小型投影。
// Store 必须使用 AtomicTask ID 与预期资源版本做并发栅栏，旧任务和重复通知不得覆盖当前事实。
type AgentInvocationTerminalProjection struct {
	TaskID                  string
	ExpectedResourceVersion int64
	Status                  string
	RuntimeSessionRef       string
	RuntimeInvocationRef    string
	AssistantMessageID      string
	LastEventSequence       int
	FailureCode             string
	FailureMessage          string
}

// AgentRuntimeQueueCandidate 是 READY Runtime 与待提交 Invocation 的内部恢复索引，不对外暴露 Agent 内容。
type AgentRuntimeQueueCandidate struct {
	RuntimeID   string
	AgentID     string
	OwnerUserID string
}

// AgentRuntimeTerminalProjection 是 Runtime 生命周期 Task 允许写回的受栅栏投影。
// Store 仅在当前 Task、操作和资源版本全部匹配时应用，并在终态清空当前操作。
type AgentRuntimeTerminalProjection struct {
	TaskID                  string
	Operation               string
	ExpectedResourceVersion int64
	Runtime                 *iapiserver.AgentRuntimeBinding
	AgentStatus             string
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

// IdentityStore 是认证和授权的消费方持久化边界，维护密码、会话和 Token 状态。
type IdentityStore interface {
	GetUser(ctx context.Context, id string) (*iapiserver.IdentityUser, error)
	GetUserByLogin(ctx context.Context, login string) (*iapiserver.IdentityUser, error)
	ListUsers(ctx context.Context, req *iapiserver.IdentityUserListRequest) ([]*iapiserver.IdentityUser, int64, error)
	ListPermissionDefinitions(ctx context.Context, req *iapiserver.IdentityPermissionListRequest) ([]*iapiserver.IdentityPermissionDefinition, int64, error)
	CreateUser(ctx context.Context, user *iapiserver.IdentityUser) (*iapiserver.IdentityUser, error)
	CreateOpenRegistration(ctx context.Context, user *iapiserver.IdentityUser) (*iapiserver.IdentityUser, error)
	CreatePendingRegistration(ctx context.Context, user *iapiserver.IdentityUser) (*IdentityRegistrationApplicationView, error)
	UpdateUser(ctx context.Context, user *iapiserver.IdentityUser) (*iapiserver.IdentityUser, error)
	ListRegistrationApplications(ctx context.Context, req *iapiserver.IdentityRegistrationApplicationListRequest) ([]*IdentityRegistrationApplicationView, int64, error)
	GetRegistrationApplication(ctx context.Context, id string) (*IdentityRegistrationApplicationView, error)
	ReviewRegistrationApplication(ctx context.Context, id, decision, reason, actorPrincipalType, actorPrincipalID, actorUserID string) (*IdentityRegistrationApplicationView, error)
	CreateOpaqueExchange(ctx context.Context, exchange *iapiserver.IdentityOpaqueExchange) error
	GetOpaqueExchange(ctx context.Context, id string) (*iapiserver.IdentityOpaqueExchange, error)
	ConsumeOpaqueExchange(ctx context.Context, id string) (*iapiserver.IdentityOpaqueExchange, error)
	FinalizeOpaqueRegistration(ctx context.Context, user *iapiserver.IdentityUser, active bool) (*iapiserver.IdentityUser, error)
	GetSession(ctx context.Context, id string) (*iapiserver.IdentityAuthSession, error)
	ListSessions(ctx context.Context, userID string, req *iapiserver.IdentitySessionListRequest) ([]*iapiserver.IdentityAuthSession, int64, error)
	CreateSession(ctx context.Context, session *iapiserver.IdentityAuthSession) (*iapiserver.IdentityAuthSession, error)
	TouchSession(ctx context.Context, id string, at imachinery.Time) (*iapiserver.IdentityAuthSession, error)
	RevokeSession(ctx context.Context, id, reason string) error
	RevokeUserSessions(ctx context.Context, userID, reason string) error
	CreateTokenCredential(ctx context.Context, token *iapiserver.IdentityTokenCredential) error
	GetTokenCredentialByJTI(ctx context.Context, jti string) (*iapiserver.IdentityTokenCredential, error)
	RevokeTokenCredential(ctx context.Context, jti string) error
	GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*iapiserver.IdentityRefreshToken, error)
	CreateRefreshToken(ctx context.Context, token *iapiserver.IdentityRefreshToken) error
	MarkRefreshTokenUsed(ctx context.Context, id string) error
	RevokeSessionRefreshTokens(ctx context.Context, sessionID, reason string) error
	PermissionCodes(ctx context.Context, principalType, principalID string) ([]string, int64, error)
	// EffectiveRoles 返回当前主体的有效角色及其授权来源，供授权投影展示使用。
	EffectiveRoles(ctx context.Context, principalType, principalID string) ([]iapiserver.IdentityEffectiveRole, error)
	UserHasAnyRole(ctx context.Context, userID string, roleCodes []string) (bool, error)
	EnsureDefaultPermissions(ctx context.Context, permissions []*iapiserver.IdentityPermissionDefinition, rolePermissions map[string][]string) error
}

// PlatformManagementStore is the persistence boundary for SystemAuthConfig and append-only AuditLog.
type PlatformManagementStore interface {
	GetSystemAuthConfig(ctx context.Context) (*iapiserver.PlatformSystemAuthConfig, error)
	ReplaceSystemAuthConfig(ctx context.Context, config *iapiserver.PlatformSystemAuthConfig, expectedVersion int64) (*iapiserver.PlatformSystemAuthConfig, error)
	AppendAuditLog(ctx context.Context, record *iapiserver.PlatformAuditLog) (*iapiserver.PlatformAuditLog, error)
	GetAuditLog(ctx context.Context, id string) (*iapiserver.PlatformAuditLog, error)
	ListAuditLogs(ctx context.Context, req *iapiserver.PlatformAuditLogListRequest) ([]*iapiserver.PlatformAuditLog, int64, error)
}

// AgentStore 是 Agent、Session、Invocation、Memory、Binding、Runtime 和事件的持久化边界。
type AgentStore interface {
	CreateAgentAggregate(context.Context, *iapiserver.Agent, *iapiserver.AgentSession, *iapiserver.AgentWorkspaceBinding, *iapiserver.AgentModelBinding) error
	ListAgents(context.Context, *iapiserver.AgentListRequest) ([]*iapiserver.Agent, int64, error)
	GetAgent(context.Context, string, string) (*iapiserver.Agent, error)
	UpdateAgent(context.Context, *iapiserver.Agent, int64) (*iapiserver.Agent, error)
	ListAgentSessions(context.Context, *iapiserver.AgentSessionListRequest) ([]*iapiserver.AgentSession, int64, error)
	CreateAgentSession(context.Context, *iapiserver.AgentSession) (*iapiserver.AgentSession, error)
	GetAgentSession(context.Context, string, string) (*iapiserver.AgentSession, error)
	UpdateAgentSession(context.Context, *iapiserver.AgentSession, int64) (*iapiserver.AgentSession, error)
	CreateAgentInvocation(context.Context, *iapiserver.AgentMessage, *iapiserver.AgentInvocation) (*iapiserver.AgentInvocation, error)
	GetAgentMessage(context.Context, string, string) (*iapiserver.AgentMessage, error)
	CreateAgentAssistantMessage(context.Context, *iapiserver.AgentMessage) (*iapiserver.AgentMessage, error)
	ListAgentMessages(context.Context, *iapiserver.AgentMessageListRequest, string) ([]*iapiserver.AgentMessage, int64, error)
	ListAgentInvocations(context.Context, *iapiserver.AgentInvocationListRequest, string) ([]*iapiserver.AgentInvocation, int64, error)
	GetAgentInvocation(context.Context, string, string) (*iapiserver.AgentInvocation, error)
	ListQueuedAgentInvocationsByAgent(context.Context, string) ([]*iapiserver.AgentInvocation, error)
	ListPendingAgentTerminalTaskIDs(context.Context, int) ([]string, error)
	ListAgentRuntimeQueueCandidates(context.Context, int) ([]AgentRuntimeQueueCandidate, error)
	UpdateAgentInvocation(context.Context, *iapiserver.AgentInvocation) (*iapiserver.AgentInvocation, error)
	BindAgentInvocationTask(context.Context, string, int64, int, string, string, int64) (*iapiserver.AgentInvocation, bool, error)
	ProjectAgentInvocationTerminal(context.Context, string, AgentInvocationTerminalProjection) (*iapiserver.AgentInvocation, bool, error)
	ListAgentMemories(context.Context, *iapiserver.AgentMemoryListRequest, string) ([]*iapiserver.AgentMemory, int64, error)
	CreateAgentMemory(context.Context, *iapiserver.AgentMemory) (*iapiserver.AgentMemory, error)
	GetAgentMemory(context.Context, string, string) (*iapiserver.AgentMemory, error)
	UpdateAgentMemory(context.Context, *iapiserver.AgentMemory, int64) (*iapiserver.AgentMemory, error)
	DeleteAgentMemory(context.Context, string, string) error
	GetAgentWorkspaceBinding(context.Context, string, string) (*iapiserver.AgentWorkspaceBinding, error)
	ListAgentModelBindings(context.Context, string, string) ([]*iapiserver.AgentModelBinding, error)
	ReplaceAgentModelBinding(context.Context, string, string, *iapiserver.AgentModelBinding) (*iapiserver.AgentModelBinding, error)
	ListAgentSkillBindings(context.Context, string, string) ([]*iapiserver.AgentSkillBinding, error)
	ListAgentMCPBindings(context.Context, string, string) ([]*iapiserver.AgentMCPBinding, error)
	CreateAgentMCPBinding(context.Context, string, *iapiserver.AgentMCPBinding) (*iapiserver.AgentMCPBinding, error)
	GetCurrentAgentRuntime(context.Context, string, string) (*iapiserver.AgentRuntimeBinding, error)
	GetAgentRuntimeByID(context.Context, string) (*iapiserver.AgentRuntimeBinding, error)
	CreateAgentRuntime(context.Context, string, *iapiserver.AgentRuntimeBinding) (*iapiserver.AgentRuntimeBinding, error)
	UpdateAgentRuntime(context.Context, *iapiserver.AgentRuntimeBinding) (*iapiserver.AgentRuntimeBinding, error)
	BindAgentRuntimeTask(context.Context, string, int64, string, string, string) (*iapiserver.AgentRuntimeBinding, bool, error)
	ProjectAgentRuntimeTerminal(context.Context, AgentRuntimeTerminalProjection) (*iapiserver.AgentRuntimeBinding, bool, error)
	AppendAgentOperationEvent(context.Context, *iapiserver.AgentOperationEvent) (*iapiserver.AgentOperationEvent, error)
	ListAgentOperationEvents(context.Context, string, int) ([]*iapiserver.AgentOperationEvent, error)
}

type StudioApplicationInitialization struct {
	Application       *iapiserver.StudioApplication
	Repository        *iapiserver.StudioSourceRepository
	Workspace         *iapiserver.StudioWorkspace
	Revision          *iapiserver.StudioWorkspaceRevision
	Agent             *iapiserver.Agent
	Session           *iapiserver.AgentSession
	WorkspaceBinding  *iapiserver.AgentWorkspaceBinding
	ModelBinding      *iapiserver.AgentModelBinding
	UserMessage       *iapiserver.AgentMessage
	InitialInvocation *iapiserver.AgentInvocation
}

// StudioCodingAgentReplacement 是 AppStudio 切换当前 Coding Agent generation 的原子写入集合。
type StudioCodingAgentReplacement struct {
	Agent            *iapiserver.Agent
	Session          *iapiserver.AgentSession
	WorkspaceBinding *iapiserver.AgentWorkspaceBinding
	ModelBinding     *iapiserver.AgentModelBinding
}

// AppStudioStore 是 StudioApplication 源码谱系、构建、发布和 Runtime 投影的事实边界。
type AppStudioStore interface {
	CreateStudioApplicationInitialization(context.Context, *StudioApplicationInitialization) (bool, error)
	GetStudioApplicationInitialization(context.Context, string, string) (*StudioApplicationInitialization, error)
	ListStudioApplications(context.Context, *iapiserver.StudioApplicationListRequest) ([]*iapiserver.StudioApplication, int64, error)
	GetStudioApplication(context.Context, string, string) (*iapiserver.StudioApplication, error)
	UpdateStudioApplication(context.Context, *iapiserver.StudioApplication, int64) (*iapiserver.StudioApplication, error)
	ReplaceStudioCodingAgent(context.Context, string, string, *StudioCodingAgentReplacement) (*iapiserver.StudioApplication, error)
	GetStudioWorkspaceByApplication(context.Context, string, string) (*iapiserver.StudioWorkspace, error)
	GetStudioWorkspace(context.Context, string, string) (*iapiserver.StudioWorkspace, error)
	ListStudioSourceFiles(context.Context, string, int64, string, string) ([]*iapiserver.StudioSourceFile, error)
	GetStudioWorkspaceRevision(context.Context, string, int64, string) (*iapiserver.StudioWorkspaceRevision, error)
	ApplyStudioChangeSet(context.Context, string, *iapiserver.StudioChangeSet, *iapiserver.StudioWorkspaceRevision, []*iapiserver.StudioSourceFile) (*iapiserver.StudioChangeSet, error)
	ResolveStudioInvocationChangeSets(context.Context, string, string, []string) (map[string]*iapiserver.StudioChangeSet, error)
	CreateStudioSourceSnapshot(context.Context, string, *iapiserver.StudioSourceSnapshot) (*iapiserver.StudioSourceSnapshot, error)
	GetStudioSourceSnapshot(context.Context, string, string) (*iapiserver.StudioSourceSnapshot, error)
	CreateStudioApplicationVersion(context.Context, string, *iapiserver.StudioApplicationVersion) (*iapiserver.StudioApplicationVersion, error)
	ListStudioApplicationVersions(context.Context, string, string, *iapiserver.StudioApplicationVersionListRequest) ([]*iapiserver.StudioApplicationVersion, int64, error)
	GetStudioApplicationVersion(context.Context, string, string) (*iapiserver.StudioApplicationVersion, error)
	CreateStudioBuild(context.Context, string, *iapiserver.StudioBuild) (*iapiserver.StudioBuild, error)
	ListStudioBuilds(context.Context, string, string, *iapiserver.StudioBuildListRequest) ([]*iapiserver.StudioBuild, int64, error)
	GetStudioBuild(context.Context, string, string) (*iapiserver.StudioBuild, error)
	ResolveStudioBuildSummaries(context.Context, string, []string) (map[string]*iapiserver.StudioBuildProducerProjection, error)
	UpdateStudioBuild(context.Context, *iapiserver.StudioBuild) (*iapiserver.StudioBuild, error)
	GetStudioPreviewRuntime(context.Context, string, string) (*iapiserver.StudioPreviewRuntime, error)
	CreateStudioPreviewRuntime(context.Context, string, *iapiserver.StudioPreviewRuntime) (*iapiserver.StudioPreviewRuntime, error)
	UpdateStudioPreviewRuntime(context.Context, *iapiserver.StudioPreviewRuntime) (*iapiserver.StudioPreviewRuntime, error)
	GetStudioRuntimeConfig(context.Context, string, string, string) (*iapiserver.StudioRuntimeConfig, error)
	ReplaceStudioRuntimeConfig(context.Context, string, *iapiserver.StudioRuntimeConfig, int64) (*iapiserver.StudioRuntimeConfig, error)
	CreateStudioReleaseAggregate(context.Context, string, *iapiserver.StudioRelease, *iapiserver.StudioRuntimeInstance) (*iapiserver.StudioRelease, error)
	ListStudioReleases(context.Context, string, string, *iapiserver.StudioReleaseListRequest) ([]*iapiserver.StudioRelease, int64, error)
	GetStudioRelease(context.Context, string, string) (*iapiserver.StudioRelease, error)
	UpdateStudioRelease(context.Context, *iapiserver.StudioRelease) (*iapiserver.StudioRelease, error)
	ListStudioRuntimeInstances(context.Context, string, string, *iapiserver.StudioRuntimeInstanceListRequest) ([]*iapiserver.StudioRuntimeInstance, int64, error)
	GetStudioRuntimeInstance(context.Context, string, string) (*iapiserver.StudioRuntimeInstance, error)
	UpdateStudioRuntimeInstance(context.Context, *iapiserver.StudioRuntimeInstance) (*iapiserver.StudioRuntimeInstance, error)
	ProjectStudioTaskTerminal(context.Context, *iapiserver.AtomicTask) error
}

type InfrastructureStore interface {
	ReconcileInfraCatalog(context.Context, []*iapiserver.InfraRuntimeProfile, *iapiserver.InfraNode) error
	ListInfraRuntimeProfiles(context.Context, *iapiserver.InfraBasicListRequest) ([]*iapiserver.InfraRuntimeProfile, int64, error)
	GetInfraRuntimeProfile(context.Context, string) (*iapiserver.InfraRuntimeProfile, error)
	ListInfraNodes(context.Context, *iapiserver.InfraBasicListRequest) ([]*iapiserver.InfraNode, int64, error)
	GetInfraNode(context.Context, string) (*iapiserver.InfraNode, error)
	CreateInfraRuntimeAggregate(context.Context, *iapiserver.InfraRuntime, []*iapiserver.InfraRuntimeMount, []*iapiserver.InfraRuntimeConfigBinding) (*iapiserver.InfraRuntime, error)
	ListInfraRuntimes(context.Context, *iapiserver.InfraRuntimeListRequest) ([]*iapiserver.InfraRuntime, int64, error)
	GetInfraRuntime(context.Context, string) (*iapiserver.InfraRuntime, error)
	UpdateInfraRuntime(context.Context, *iapiserver.InfraRuntime, *iapiserver.InfraRuntimeEndpoint, []*iapiserver.InfraRuntimeOutput, string) (*iapiserver.InfraRuntime, error)
	GetInfraRuntimeEndpoint(context.Context, string) (*iapiserver.InfraRuntimeEndpoint, error)
	GetInfraRuntimeEndpointByID(context.Context, string) (*iapiserver.InfraRuntimeEndpoint, error)
	ListInfraRuntimeOutputs(context.Context, string) ([]*iapiserver.InfraRuntimeOutput, error)
	GetInfraRuntimeOutput(context.Context, string) (*iapiserver.InfraRuntimeOutput, error)
	AttachInfraRuntimeOutputArtifact(context.Context, string, string) (*iapiserver.InfraRuntimeOutput, error)
}

// IdentityAdminStore 是 Identity RBAC、资源授权和服务账号的管理能力边界。
type IdentityAdminStore interface {
	ListRoles(ctx context.Context, req *iapiserver.IdentityRoleListRequest) ([]*iapiserver.IdentityRole, int64, error)
	GetRole(ctx context.Context, id string) (*iapiserver.IdentityRole, error)
	CreateRole(ctx context.Context, role *iapiserver.IdentityRole) (*iapiserver.IdentityRole, error)
	UpdateRole(ctx context.Context, role *iapiserver.IdentityRole) (*iapiserver.IdentityRole, error)
	ReplaceRolePermissions(ctx context.Context, id string, permissionCodes []string) error
	ListGroups(ctx context.Context, req *iapiserver.IdentityGroupListRequest) ([]*iapiserver.IdentityGroup, int64, error)
	GetGroup(ctx context.Context, id string) (*iapiserver.IdentityGroup, error)
	CreateGroup(ctx context.Context, group *iapiserver.IdentityGroup) (*iapiserver.IdentityGroup, error)
	UpdateGroup(ctx context.Context, group *iapiserver.IdentityGroup) (*iapiserver.IdentityGroup, error)
	ReplaceGroupMembers(ctx context.Context, id string, userIDs []string) error
	ReplaceGroupRoles(ctx context.Context, id string, roleIDs []string) error
	ListResourceGrants(ctx context.Context, resourceType, resourceID string, req *iapiserver.IdentityResourceGrantListRequest) ([]*iapiserver.IdentityResourceAccessGrant, int64, error)
	CreateResourceGrant(ctx context.Context, grant *iapiserver.IdentityResourceAccessGrant) (*iapiserver.IdentityResourceAccessGrant, error)
	UpdateResourceGrant(ctx context.Context, grant *iapiserver.IdentityResourceAccessGrant) (*iapiserver.IdentityResourceAccessGrant, error)
	RevokeResourceGrant(ctx context.Context, id string) error
	ListServiceAccounts(ctx context.Context, req *iapiserver.IdentityServiceAccountListRequest) ([]*iapiserver.IdentityServiceAccount, int64, error)
	GetServiceAccount(ctx context.Context, id string) (*iapiserver.IdentityServiceAccount, error)
	CreateServiceAccount(ctx context.Context, account *iapiserver.IdentityServiceAccount, permissionCodes []string) (*iapiserver.IdentityServiceAccount, error)
	UpdateServiceAccount(ctx context.Context, account *iapiserver.IdentityServiceAccount, permissionCodes []string) (*iapiserver.IdentityServiceAccount, error)
	SetServiceAccountStatus(ctx context.Context, id, status string) (*iapiserver.IdentityServiceAccount, error)
	RotateServiceAccountCredential(ctx context.Context, id string) (*iapiserver.IdentityServiceAccountCredentialResponse, error)
}

// PlatformManagementFactory is an optional capability implemented by stores that have the released Platform schema.
type PlatformManagementFactory interface {
	PlatformManagement() PlatformManagementStore
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

// WorkflowCanvasStore owns spec-v1.7.0 node definitions, drafts, immutable versions, and run projections.
type WorkflowCanvasStore interface {
	ListWorkflowNodeDefinitions(
		context.Context,
		*iapiserver.WorkflowNodeDefinitionListRequest,
		string,
		string,
	) ([]*iapiserver.WorkflowNodeDefinition, int64, error)
	GetWorkflowNodeDefinition(context.Context, string, string, string, string, bool) (*iapiserver.WorkflowNodeDefinition, error)
	AddWorkflowNodeDefinitionIdempotent(context.Context, *iapiserver.WorkflowNodeDefinition) (*iapiserver.WorkflowNodeDefinition, bool, error)
	DeprecateWorkflowNodeDefinition(context.Context, string, string, string) (*iapiserver.WorkflowNodeDefinition, error)
	ListWorkflowCanvases(context.Context, *iapiserver.WorkflowCanvasListRequest, string, string, string) ([]*iapiserver.WorkflowCanvas, int64, error)
	GetWorkflowCanvas(context.Context, string) (*iapiserver.WorkflowCanvas, error)
	GetWorkflowCanvasesByIDs(context.Context, []string) ([]*iapiserver.WorkflowCanvas, error)
	AddWorkflowCanvas(context.Context, *iapiserver.WorkflowCanvas) (*iapiserver.WorkflowCanvas, error)
	UpdateWorkflowCanvas(context.Context, *iapiserver.WorkflowCanvas, int64) (*iapiserver.WorkflowCanvas, error)
	DeleteWorkflowCanvas(context.Context, string) error
	PublishWorkflowCanvas(context.Context, *iapiserver.WorkflowCanvas, *iapiserver.CanvasVersion, int64) (*iapiserver.CanvasVersion, error)
	ListCanvasVersions(context.Context, *iapiserver.CanvasVersionListRequest) ([]*iapiserver.CanvasVersion, int64, error)
	GetCanvasVersion(context.Context, string) (*iapiserver.CanvasVersion, error)
	GetCanvasVersionsByIDs(context.Context, []string) ([]*iapiserver.CanvasVersion, error)
	ListWorkflowCanvasRuns(context.Context, *iapiserver.WorkflowCanvasRunListRequest, string, string, string) ([]*iapiserver.WorkflowCanvasRun, int64, error)
	GetWorkflowCanvasRun(context.Context, string) (*iapiserver.WorkflowCanvasRun, error)
	GetWorkflowCanvasRunsByIDs(context.Context, []string) ([]*iapiserver.WorkflowCanvasRun, error)
	AddWorkflowCanvasRunIdempotent(context.Context, *iapiserver.WorkflowCanvasRun) (*iapiserver.WorkflowCanvasRun, bool, error)
	BindWorkflowCanvasRun(
		context.Context,
		string,
		string,
		[]*iapiserver.CanvasFlowRun,
		[]*iapiserver.CanvasNodeRun,
		[]*iapiserver.CanvasNodeRunTaskBinding,
		[]*iapiserver.CanvasNodeRunOutputBinding,
	) (*iapiserver.WorkflowCanvasRun, error)
	UpdateWorkflowCanvasRun(context.Context, *iapiserver.WorkflowCanvasRun) (*iapiserver.WorkflowCanvasRun, error)
	ListCanvasFlowRuns(context.Context, *iapiserver.CanvasFlowRunListRequest) ([]*iapiserver.CanvasFlowRun, int64, error)
	ListCanvasNodeRuns(context.Context, *iapiserver.CanvasNodeRunListRequest) ([]*iapiserver.CanvasNodeRun, int64, error)
	GetCanvasNodeRun(context.Context, string) (*iapiserver.CanvasNodeRun, error)
	GetCanvasNodeRunDetail(context.Context, string) ([]*iapiserver.CanvasNodeRunTaskBinding, []*iapiserver.CanvasNodeRunOutputBinding, error)
	ProjectCanvasApplicationArtifact(context.Context, *CanvasApplicationArtifactProjection) (bool, error)
}

// CanvasApplicationArtifactProjection 是 Application Artifact 事实到 Canvas 输出槽位的内部投影命令。
type CanvasApplicationArtifactProjection struct {
	AtomicTaskID             string
	OutputKey                string
	Sequence                 int
	ArtifactID               string
	MediaType                string
	ArtifactProcessingStatus string
	ArtifactResourceVersion  int64
}

type ProviderStore interface {
	List(ctx context.Context, req *iapiserver.ProviderListRequest) ([]*iapiserver.Provider, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.Provider, error)
	// GetOwned 只返回指定用户拥有的 Provider，不扩大 MODEL_* 权限的数据范围。
	GetOwned(ctx context.Context, ownerUserID, id string) (*iapiserver.Provider, error)
	// GetByIDs 按当前所有者边界批量读取 provider，供跨领域一跳投影使用。
	GetByIDs(ctx context.Context, ownerUserID string, ids []string) ([]*iapiserver.Provider, error)
	Add(ctx context.Context, data *iapiserver.Provider) (*iapiserver.Provider, error)
	Update(ctx context.Context, data *iapiserver.Provider) (*iapiserver.Provider, error)
	// Delete removes one provider record by id.
	Delete(ctx context.Context, id string) error
	// DeleteOwnedCascade 在同一事务内清理当前用户的 Provider、模型和默认绑定。
	DeleteOwnedCascade(ctx context.Context, ownerUserID, id string) error
}

type ProviderModelStore interface {
	List(ctx context.Context, req *iapiserver.ProviderModelListRequest) ([]*iapiserver.ProviderModel, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.ProviderModel, error)
	// GetOwned 只返回指定用户拥有的 ProviderModel。
	GetOwned(ctx context.Context, ownerUserID, id string) (*iapiserver.ProviderModel, error)
	// GetByIDs 按当前所有者边界批量读取模型，禁止消费方逐 ID 查询。
	GetByIDs(ctx context.Context, ownerUserID string, ids []string) ([]*iapiserver.ProviderModel, error)
	Add(ctx context.Context, data *iapiserver.ProviderModel) (*iapiserver.ProviderModel, error)
	Update(ctx context.Context, data *iapiserver.ProviderModel) (*iapiserver.ProviderModel, error)
	// ProjectHealth 以探测开始时的配置版本和检测时间为栅栏投影健康事实；仅健康资格变化递增配置版本。
	ProjectHealth(ctx context.Context, ownerUserID, id string, expectedConfigVersion int64, healthStatus, healthReason string, checkedAt imachinery.Time) (*iapiserver.ProviderModel, error)
	// Delete 删除指定模型提供商下的一个模型元数据。
	Delete(ctx context.Context, providerID, id string) error
	// DeleteByProviderID removes all models under one provider.
	DeleteByProviderID(ctx context.Context, providerID string) error
	// DeleteOwnedCascade 在同一事务内清理当前用户的模型及其默认绑定。
	DeleteOwnedCascade(ctx context.Context, ownerUserID, id string) error
}

type ModelHealthCheckStore interface {
	// Add 保存一次已持久化 Provider 或模型的检测结果；未保存表单测试不得调用。
	Add(ctx context.Context, data *iapiserver.ModelHealthCheck) (*iapiserver.ModelHealthCheck, error)
}

type ProviderCapabilityStore interface {
	List(ctx context.Context) ([]*iapiserver.ProviderCapability, error)
	Add(ctx context.Context, data *iapiserver.ProviderCapability) (*iapiserver.ProviderCapability, error)
}

type SystemLLMConfigStore interface {
	List(ctx context.Context) ([]*iapiserver.SystemLLMConfig, error)
	// ListOwned 只读取指定用户的默认模型配置。
	ListOwned(ctx context.Context, ownerUserID string) ([]*iapiserver.SystemLLMConfig, error)
	// GetOwned 读取指定用户和用途的唯一默认模型配置。
	GetOwned(ctx context.Context, ownerUserID, usage string) (*iapiserver.SystemLLMConfig, error)
	Upsert(ctx context.Context, data *iapiserver.SystemLLMConfig) (*iapiserver.SystemLLMConfig, error)
	// DeleteByProviderModelID 删除引用指定模型的默认模型绑定。
	DeleteByProviderModelID(ctx context.Context, providerID, modelID string) error
	// DeleteByProviderID removes all default model bindings under one provider.
	DeleteByProviderID(ctx context.Context, providerID string) error
}

type StorageBackendStore interface {
	// GetBlob 读取全局 Blob 物理元数据，仅供完成管理员鉴权后的 storage-inspection 服务调用。
	GetBlob(ctx context.Context, id string) (*iapiserver.AssetBlob, error)
	List(ctx context.Context, req *iapiserver.StorageBackendListRequest) ([]*iapiserver.StorageBackend, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.StorageBackend, error)
	Add(ctx context.Context, data *iapiserver.StorageBackend) (*iapiserver.StorageBackend, error)
	Update(ctx context.Context, data *iapiserver.StorageBackend) (*iapiserver.StorageBackend, error)
	GetDefaultLocal(ctx context.Context) (*iapiserver.StorageBackend, error)
	// EnsureDefaultLocal 幂等返回已有可写 local 后端，缺失时创建 bootstrap 提供的默认配置。
	EnsureDefaultLocal(ctx context.Context, desired *iapiserver.StorageBackend) (*iapiserver.StorageBackend, error)
}

type AssetStore interface {
	List(ctx context.Context, req *iapiserver.AssetListRequest) ([]*iapiserver.Asset, int64, error)
	Get(ctx context.Context, id string) (*iapiserver.Asset, error)
	Add(ctx context.Context, data *iapiserver.Asset) (*iapiserver.Asset, error)
	AddWithUploadEvent(
		ctx context.Context,
		data *iapiserver.Asset,
		thumbnail *iapiserver.AssetThumbnail,
		event map[string]any,
	) (*iapiserver.Asset, *iapiserver.AssetThumbnail, error)
	Update(ctx context.Context, data *iapiserver.Asset) (*iapiserver.Asset, error)
	// Delete marks the asset as deleted. It does not remove asset objects, thumbnails, or relation rows.
	Delete(ctx context.Context, id string) error
}

type AssetV1Store interface {
	RegisterArtifact(context.Context, *iapiserver.ArtifactRegistrationRequest) (*iapiserver.UserAsset, bool, error)
	ApplyLabels(context.Context, string, string, map[string]string, []string, []string) (*iapiserver.BatchLabelData, error)
	// CreateArtifact 幂等创建 asset-library Artifact，并在同一事务写 artifact_created outbox。
	CreateArtifact(context.Context, *iapiserver.Artifact) (*iapiserver.Artifact, bool, error)
	// UpdateArtifactProcessing 以乐观版本推进处理事实，并在同一事务写 artifact_processing_changed outbox。
	UpdateArtifactProcessing(context.Context, string, string, int64, ArtifactProcessingMutation) (*iapiserver.Artifact, error)
	// UpdateArtifactRegistration 以乐观版本推进登记事实，并在同一事务写 artifact_registration_changed outbox。
	UpdateArtifactRegistration(context.Context, string, string, int64, ArtifactRegistrationMutation) (*iapiserver.Artifact, error)
	// CreateAssetVersion 幂等创建 processing 状态版本，并在同一事务写 asset_version_processing_changed outbox。
	CreateAssetVersion(context.Context, *iapiserver.AssetVersion, string, string) (*iapiserver.AssetVersion, bool, error)
	// UpdateAssetVersionProcessing 以乐观版本推进 Representation 汇总，并可靠发布素材版本事件。
	UpdateAssetVersionProcessing(context.Context, string, string, int64, AssetVersionProcessingMutation) (*iapiserver.AssetVersion, error)

	ListUserAssets(context.Context, string, *iapiserver.UserAssetListRequest) ([]*iapiserver.UserAsset, int64, error)
	GetUserAsset(context.Context, string, string, bool) (*iapiserver.UserAsset, error)
	GetAssetDetail(context.Context, string, string) (*iapiserver.AssetDetail, error)
	CreateCanonicalAsset(context.Context, string, *iapiserver.CreateCanonicalAssetRequest) (*iapiserver.AssetDetail, error)
	UpdateUserAsset(context.Context, string, string, *iapiserver.UpdateUserAssetRequest) (*iapiserver.UserAsset, error)
	SetUserAssetDeleted(context.Context, string, string, bool) (*iapiserver.UserAsset, error)
	// ListDeletedUserAssetIDs 按稳定顺序返回当前 owner 回收站中的全部素材 ID。
	ListDeletedUserAssetIDs(context.Context, string) ([]string, error)
	// HardDeleteUserAsset 从任意素材状态执行强引用检查并永久删除，不要求先进入回收站。
	HardDeleteUserAsset(context.Context, string, string) (*iapiserver.PermanentDeleteResult, []StoredAssetContent, error)
	PermanentlyDeleteUserAsset(context.Context, string, string) (*iapiserver.PermanentDeleteResult, []StoredAssetContent, error)

	CreateAssetUploads(context.Context, string, []iapiserver.AssetUploadItemRequest, int64) ([]iapiserver.AssetUploadInitResult, error)
	GetAssetUpload(context.Context, string, string) (*iapiserver.AssetUploadSession, error)
	RecordAssetUploadPart(context.Context, string, string, iapiserver.UploadedPart) (*iapiserver.AssetUploadSession, error)
	CompleteAssetUpload(
		context.Context,
		string,
		string,
		*iapiserver.CompleteAssetUploadRequest,
		StoredAssetContent,
	) (*iapiserver.CompleteAssetUploadResponse, error)
	// CompleteAssetUploadWithPlan 在上传事务内校验并发布由 service 计算的 expected Representation 计划。
	CompleteAssetUploadWithPlan(context.Context, string, string, *iapiserver.CompleteAssetUploadRequest, StoredAssetContent, RepresentationPlan) (*iapiserver.CompleteAssetUploadResponse, error)
	CancelAssetUpload(context.Context, string, string) (*iapiserver.AssetUploadSession, error)

	ListCollections(context.Context, string, *iapiserver.CollectionListRequest) ([]*iapiserver.AssetCollection, int64, error)
	GetCollection(context.Context, string, string, imachinery.PagingParams) (*iapiserver.CollectionDetail, error)
	CreateCollection(context.Context, string, *iapiserver.CreateCollectionRequest) (*iapiserver.AssetCollection, error)
	UpdateCollection(context.Context, string, string, *iapiserver.UpdateCollectionRequest) (*iapiserver.AssetCollection, error)
	DeleteCollection(context.Context, string, string) error
	AddCollectionItems(context.Context, string, string, []iapiserver.AddCollectionItem) (*iapiserver.CollectionItemBatchResponse, error)
	UpdateCollectionItem(context.Context, string, string, string, *iapiserver.UpdateCollectionItemRequest) (*iapiserver.AssetCollectionItem, error)
	DeleteCollectionItem(context.Context, string, string, string) error

	ReplaceLabels(context.Context, string, string, map[string]string) (*iapiserver.BatchLabelData, error)
	DeleteLabel(context.Context, string, string, string) (*iapiserver.BatchLabelData, error)
	AddTags(context.Context, string, string, []string) (*iapiserver.BatchLabelData, error)
	DeleteTag(context.Context, string, string, string) (*iapiserver.BatchLabelData, error)

	ListArtifacts(context.Context, string, *iapiserver.ArtifactListRequest) ([]*iapiserver.Artifact, int64, error)
	// ResolveArtifactSummaries 批量读取 owner 可见且未删除的 Artifact 一跳摘要，不返回缺失目标差异。
	ResolveArtifactSummaries(context.Context, string, []string) (map[string]*iapiserver.ArtifactReadableSummary, error)
	// DecorateArtifacts 批量组合登记素材与版本摘要，不读取跨领域 producer 私有表。
	DecorateArtifacts(context.Context, string, []*iapiserver.Artifact) error
	GetArtifact(context.Context, string, string) (*iapiserver.Artifact, error)
	StoreArtifactContent(context.Context, string, string, StoredAssetContent) (*iapiserver.Artifact, error)
	CompleteArtifact(context.Context, string, string, *iapiserver.CompleteArtifactRequest) (*iapiserver.Artifact, error)
	DeleteArtifact(context.Context, string, string) (*iapiserver.Artifact, error)
	RegisterArtifactLifecycle(context.Context, string, string, *iapiserver.RegisterArtifactRequest) (*iapiserver.ArtifactRegistrationResponse, error)
	// RegisterArtifactLifecycleWithPlan 在 Artifact 登记事务内应用受控 Representation 计划。
	RegisterArtifactLifecycleWithPlan(context.Context, string, string, *iapiserver.RegisterArtifactRequest, RepresentationPlan) (*iapiserver.ArtifactRegistrationResponse, error)

	ListAssetVersions(context.Context, string, string) ([]*iapiserver.AssetVersion, error)
	CreateCanonicalVersion(context.Context, string, string, *iapiserver.CreateCanonicalVersionRequest) (*iapiserver.AssetVersion, error)
	GetAssetVersionDetail(context.Context, string, string) (*iapiserver.AssetVersionDetail, error)
	SetCurrentAssetVersion(context.Context, string, string, string) (*iapiserver.UserAsset, error)
	ListRepresentations(context.Context, string, string) ([]*iapiserver.AssetRepresentation, error)
	RegisterRepresentation(context.Context, string, string, *iapiserver.RegisterRepresentationRequest) (*iapiserver.AssetRepresentation, error)
	// ApplyAssetMediaMetadata 原子写回 original 探测事实，并仅在版本仍为 current 时刷新 UserAsset 投影。
	ApplyAssetMediaMetadata(context.Context, string, string, AssetMediaMetadataMutation) error
	// ListAssetMediaMetadataBackfillCandidatesAfter 按稳定 Asset ID 扫描当前版本缺失媒体元数据的素材。
	ListAssetMediaMetadataBackfillCandidatesAfter(context.Context, string, int) ([]AssetMediaMetadataBackfillCandidate, error)
	// CompleteRepresentationGeneration 允许 Worker 幂等推进 pending/failed Representation，并原子刷新版本投影。
	CompleteRepresentationGeneration(context.Context, string, string, RepresentationGenerationMutation) (*iapiserver.AssetRepresentation, error)
	// ListRepresentationBackfillCandidatesAfter 按稳定 AssetVersion ID 扫描当前可见素材的 expected set 事实。
	ListRepresentationBackfillCandidatesAfter(context.Context, string, int) ([]RepresentationBackfillCandidate, error)
	// PrepareRepresentationBackfill 在创建修复动作前推进 expected count 和缩略图投影。
	PrepareRepresentationBackfill(context.Context, string, string, int) error
	// CreateRepresentationBlob 幂等登记 Worker 生成的受控派生内容，返回 Blob ID。
	CreateRepresentationBlob(context.Context, StoredAssetContent) (string, error)
	GetRepresentation(context.Context, string, string) (*iapiserver.AssetRepresentation, *StoredAssetContent, error)

	ListAssetRelations(context.Context, string, string, imachinery.PagingParams) (*iapiserver.AssetRelationListResponse, error)
	GetAssetLineage(context.Context, string, string) (*iapiserver.AssetLineage, error)
	ListAssetReferences(context.Context, string, string, imachinery.PagingParams) (*iapiserver.AssetReferenceListResponse, error)
	ListAssetUsages(context.Context, string, string, imachinery.PagingParams) (*iapiserver.AssetUsageListResponse, error)
}

// StoredAssetContent 是 StorageAdapter 与持久化事务之间的受控对象引用。
type StoredAssetContent struct {
	StorageBackendID string
	ObjectKey        string
	SHA256           string
	SizeBytes        int64
	MIMEType         string
	BlobID           string
}

// ExpectedRepresentation 是 asset-library policy 交给事务层的受控派生计划项。
type ExpectedRepresentation struct {
	Type     string
	Profile  string
	Required bool
}

// RepresentationPlan 固定一个 AssetVersion 的媒体策略和 expected set。
type RepresentationPlan struct {
	MediaType      string
	ProfileVersion string
	ExpectedCount  int
	Requested      []ExpectedRepresentation
}

// RepresentationGenerationMutation 是 Worker 对单个 expected Representation 的有限状态写入。
type RepresentationGenerationMutation struct {
	Type           string
	Profile        string
	ProfileVersion string
	BlobID         string
	Metadata       map[string]any
	Status         string
	Required       bool
	RetryCount     int
	RetryAfter     *imachinery.Time
	ErrorCode      string
	ErrorDetail    string
}

// AssetMediaMetadataMutation 是 representation.inspect 对原始媒体元数据的有限写回。
type AssetMediaMetadataMutation struct {
	AssetID                  string
	OriginalRepresentationID string
	MIMEType                 string
	SizeBytes                int64
	Width                    int
	Height                   int
	DurationSeconds          float64
}

// AssetMediaMetadataBackfillCandidate 是一次性运维回填所需的当前版本最小投影。
type AssetMediaMetadataBackfillCandidate struct {
	AssetID        string
	AssetVersionID string
	OwnerUserID    string
	MediaType      string
}

// RepresentationBackfillCandidate 是 backfill handler 所需的 owner 裁剪最小投影。
type RepresentationBackfillCandidate struct {
	AssetID              string
	AssetVersionID       string
	OwnerUserID          string
	MediaType            string
	ProfileVersion       string
	RepresentationStatus string
	RepresentationBlobOK bool
	RetryCount           int
	RetryAfter           *time.Time
	SourceAvailable      bool
}

// ArtifactProcessingMutation 只允许处理模块修改 Artifact 的处理维度和受保护预览摘要。
type ArtifactProcessingMutation struct {
	ChangeType            string
	ProcessingStatus      string
	Progress              *float64
	ProcessingPhase       string
	PreviewAvailable      bool
	PreviewRef            string
	ThumbnailRef          string
	ProcessingErrorCode   string
	ProcessingErrorDetail string
	Retryable             bool
	ReadyAt               *imachinery.Time
}

// ArtifactRegistrationMutation 只允许登记模块修改 Artifact 的登记维度和目标引用。
type ArtifactRegistrationMutation struct {
	RegistrationStatus      string
	RegistrationResult      string
	AssetID                 string
	AssetVersionID          string
	RegistrationErrorCode   string
	RegistrationErrorDetail string
	Retryable               bool
}

// AssetVersionProcessingMutation 由 Representation 汇总器提交有限计数、状态和任务引用。
type AssetVersionProcessingMutation struct {
	Status         string
	ExpectedCount  int
	CompletedCount int
	FailedCount    int
	TaskGroupID    string
	AtomicTaskID   string
	ErrorCode      string
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

type AIChatGenerationRoute struct {
	ModelID                string
	CapabilityDefinitionID string
	ModelConfigVersion     int64
	ModelSnapshot          map[string]any
}

type AIChatStore interface {
	ListAssistants(ctx context.Context, ownerUserID string) ([]*iapiserver.AIChatAssistant, error)
	GetAssistant(ctx context.Context, ownerUserID, id string) (*iapiserver.AIChatAssistant, error)
	// GetAssistantsByIDs 批量读取当前用户可见的系统助手和用户助手。
	GetAssistantsByIDs(ctx context.Context, ownerUserID string, ids []string) ([]*iapiserver.AIChatAssistant, error)
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
		route AIChatGenerationRoute,
		assistant *iapiserver.AIChatAssistant,
	) (*AIChatGenerationBundle, error)
	CreateEditRegenerateGeneration(
		ctx context.Context,
		ownerUserID string,
		source *iapiserver.AIChatMessage,
		req *iapiserver.AIChatEditRegenerateRequest,
		route AIChatGenerationRoute,
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
	BindApplicationRunToAtomicTask(context.Context, string, string, string, string, string, map[string]any) (*iapiserver.AtomicTask, error)
	AddAtomicTaskIdempotent(context.Context, *iapiserver.AtomicTask) (*iapiserver.AtomicTask, bool, error)
	UpdateAtomicTask(context.Context, *iapiserver.AtomicTask) (*iapiserver.AtomicTask, error)
	ListAttempts(ctx context.Context, req *iapiserver.TaskAttemptListRequest) ([]*iapiserver.TaskAttempt, int64, error)
	GetAttempt(context.Context, string, string) (*iapiserver.TaskAttempt, error)
	ListAttemptsByTaskIDs(context.Context, []string) ([]*iapiserver.TaskAttempt, error)
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
	// ListDAGObservationTasks 批量读取一个已授权 DAG 的实际任务投影，供详情、事件和时间线聚合。
	ListDAGObservationTasks(context.Context, string) ([]*iapiserver.AtomicTask, error)
	AddOwnedAtomicTasks(context.Context, string, string, []*iapiserver.AtomicTask) error
	RepairTerminalTaskOwner(context.Context, string, string) error
	ListTaskSchedules(context.Context, *iapiserver.TaskScheduleListRequest) ([]*iapiserver.TaskSchedule, int64, error)
	// GetTaskSchedulesByIDs 批量读取关联摘要使用的 TaskSchedule，调用方仍需执行主体可见性过滤。
	GetTaskSchedulesByIDs(context.Context, []string) ([]*iapiserver.TaskSchedule, error)
	GetTaskSchedule(context.Context, string) (*iapiserver.TaskSchedule, error)
	GetTaskScheduleBySystemKey(context.Context, string) (*iapiserver.TaskSchedule, error)
	AddTaskSchedule(context.Context, *iapiserver.TaskSchedule) (*iapiserver.TaskSchedule, error)
	EnsureSystemTaskSchedule(context.Context, *iapiserver.TaskSchedule, *iapiserver.ScheduleReconcileState) (*iapiserver.TaskSchedule, bool, error)
	UpdateTaskSchedule(context.Context, *iapiserver.TaskSchedule) (*iapiserver.TaskSchedule, error)
	ListScheduleExecutions(context.Context, *iapiserver.ScheduleExecutionListRequest) ([]*iapiserver.TaskScheduleExecution, int64, error)
	GetScheduleExecution(context.Context, string) (*iapiserver.TaskScheduleExecution, error)
	// GetScheduleExecutionAt 按计划与计划时间读取幂等轮次；不存在时返回 nil，供 misfire 守卫区分首次迟到触发与已有轮次恢复。
	GetScheduleExecutionAt(context.Context, string, time.Time) (*iapiserver.TaskScheduleExecution, error)
	ListLatestScheduleExecutions(context.Context, []string) (map[string]*iapiserver.TaskScheduleExecution, error)
	// ListScheduleSources 按目标类型与 ID 批量返回最新的来源调度轮次。
	ListScheduleSources(context.Context, string, []string) (map[string]*iapiserver.ScheduleSourceSummary, error)
	AddProjectionEventIdempotent(context.Context, *iapiserver.RuntimeProjectionEvent) (*iapiserver.RuntimeProjectionEvent, bool, error)
	ListRuntimeProjectionEvents(context.Context, string) ([]*iapiserver.RuntimeProjectionEvent, error)
	ListNonTerminalAtomicTasks(context.Context, int) ([]*iapiserver.AtomicTask, error)
	ListNonTerminalTaskGroups(context.Context, int) ([]*iapiserver.TaskGroup, error)
	ListNonTerminalDAGTaskGroups(context.Context, int) ([]*iapiserver.DAGTaskGroup, error)
	ListActiveScheduleExecutions(context.Context, int) ([]*iapiserver.TaskScheduleExecution, error)
	ApplyRuntimeProjection(context.Context, *iapiserver.AtomicTask, []*iapiserver.TaskAttempt, *iapiserver.RuntimeProjectionEvent) (bool, error)
	AcquireScheduleExecution(context.Context, *iapiserver.TaskScheduleExecution) (*iapiserver.TaskScheduleExecution, bool, error)
	// WithScheduleReconcileLock 在 PostgreSQL 事务级 advisory lock 下串行执行同一计划的 controller，进程退出时锁自动释放。
	WithScheduleReconcileLock(context.Context, string, func() error) (bool, error)
	UpdateScheduleExecution(context.Context, *iapiserver.TaskScheduleExecution) (*iapiserver.TaskScheduleExecution, error)
	GetScheduleReconcileState(context.Context, string) (*iapiserver.ScheduleReconcileState, error)
	CompleteScheduleReconcile(context.Context, *iapiserver.TaskScheduleExecution, *iapiserver.ScheduleReconcileState) error
	PruneReconcileExecutions(context.Context, string, iapiserver.HistoryRetention, time.Time) (int64, error)
}

// UserEventStore 提供当前用户短期事件历史与 SSE 增量读取，不暴露跨用户查询。
type UserEventStore interface {
	AddIdempotent(context.Context, *iapiserver.UserEvent) (*iapiserver.UserEvent, bool, error)
	List(context.Context, *iapiserver.UserEventListRequest, time.Time) ([]*iapiserver.UserEvent, int64, error)
	ListAfter(context.Context, string, int64, int, time.Time) ([]*iapiserver.UserEvent, error)
	CursorVisible(context.Context, string, int64) (bool, bool, error)
	SyncState(context.Context, string, time.Time) (int64, int64, error)
	PruneExpired(context.Context, time.Time) (int64, error)
}

// NotificationStore 组合通知收件箱 API 所需的小型持久化边界；worker 的候选 claim 边界另行扩展。
type NotificationStore interface {
	ListNotifications(context.Context, *iapiserver.NotificationListRequest) ([]*iapiserver.Notification, int64, error)
	GetNotificationCounter(context.Context, string) (*iapiserver.NotificationRecipientCounter, error)
	MutateNotificationInbox(context.Context, string, string, string) (*iapiserver.Notification, *iapiserver.NotificationRecipientCounter, error)
	ReadAllNotifications(context.Context, string, string) (int64, *iapiserver.NotificationRecipientCounter, error)
	ListNotificationTopics(context.Context, bool) ([]*iapiserver.NotificationTopic, error)
	ListNotificationPreferences(context.Context, string) ([]*iapiserver.NotificationPreference, error)
	ReplaceNotificationPreferences(context.Context, string, []*iapiserver.NotificationPreference) ([]*iapiserver.NotificationPreference, error)
}

// NotificationMaterialization 是规则 Worker 向持久化层提交的已校验通知草案。
type NotificationMaterialization struct {
	RecipientUserID   string
	Title             string
	Content           string
	Severity          string
	AttentionStatus   string
	NavigationTarget  *iapiserver.NotificationNavigationTarget
	ActionPath        *string
	AggregateKey      string
	AggregationWindow string
	ExpiresAt         imachinery.Time
}

// NotificationCandidateStore 提供 Worker 候选落库、有限租约 claim、物化和失败重试。
type NotificationCandidateStore interface {
	AddNotificationCandidates(context.Context, []*iapiserver.NotificationEvent) error
	ClaimNotificationCandidates(context.Context, time.Time, int, time.Duration) ([]*iapiserver.NotificationEvent, error)
	FinishNotificationCandidate(context.Context, string, int, string, time.Time, string, string) error
	MaterializeNotification(context.Context, *iapiserver.NotificationEvent, NotificationMaterialization) (*iapiserver.Notification, bool, error)
}

// NotificationOutboxStore 由统一 SSE projector 使用，只负责通知出站投影的确认与失败退避。
type NotificationOutboxStore interface {
	MarkNotificationOutboxPublished(context.Context, string, time.Time) error
	MarkNotificationOutboxFailed(context.Context, string, time.Time, string, string) error
}

// NotificationRetentionResult 汇总一次受控保留清理，不包含任何源业务事实。
type NotificationRetentionResult struct {
	Notifications int64
	Candidates    int64
	Outbox        int64
}

// NotificationRetentionStore 负责通知、无引用候选和已投递出站记录的有限保留。
type NotificationRetentionStore interface {
	CleanupNotifications(context.Context, time.Time, int) (NotificationRetentionResult, error)
}

// NotificationStoreFactory 由支持 Notification Center 的 Factory 额外实现，避免其他消费方依赖具体数据库。
type NotificationStoreFactory interface {
	Notifications() NotificationStore
}

type NotificationCandidateStoreFactory interface {
	NotificationCandidates() NotificationCandidateStore
}

type NotificationOutboxStoreFactory interface {
	NotificationOutbox() NotificationOutboxStore
}

type NotificationRetentionStoreFactory interface {
	NotificationRetention() NotificationRetentionStore
}

type ApplicationPlatformStore interface {
	ListEngineInstances(ctx context.Context, req *iapiserver.EngineInstanceListRequest) ([]*iapiserver.EngineInstance, int64, error)
	ListEnabledEngineInstancesAfter(context.Context, string, int) ([]*iapiserver.EngineInstance, error)
	ListRefreshableComfyUIEngineInstancesAfter(context.Context, string, int) ([]*iapiserver.EngineInstance, error)
	GetEngineInstance(ctx context.Context, id string) (*iapiserver.EngineInstance, error)
	AddEngineInstance(ctx context.Context, data *iapiserver.EngineInstance) (*iapiserver.EngineInstance, error)
	// AddEngineInstanceWithBindings 原子创建 EngineInstance 及其全部系统必需能力绑定。
	AddEngineInstanceWithBindings(context.Context, *iapiserver.EngineInstance, []*iapiserver.EngineCapabilityBinding) (*iapiserver.EngineInstance, error)
	// EnsureRequiredEngineBindings 为匹配 EngineType 的全部实例幂等修复系统必需的不可变绑定。
	EnsureRequiredEngineBindings(context.Context, string, string, string, string, string) error
	UpdateEngineInstance(ctx context.Context, data *iapiserver.EngineInstance, expectedVersion int64) (*iapiserver.EngineInstance, error)
	UpdateEngineInstanceHealth(
		ctx context.Context,
		data *iapiserver.EngineInstance,
		expectedVersion int64,
		event *iapiserver.ApplicationPlatformEvent,
	) (*iapiserver.EngineInstance, error)
	GetComfyUIEngineObjectInfo(context.Context, string) (*iapiserver.ComfyUIEngineObjectInfo, error)
	RefreshComfyUIEngineObjectInfo(
		context.Context,
		string,
		func(*iapiserver.EngineInstance) (*iapiserver.ComfyUIEngineObjectInfo, error),
	) (*iapiserver.ComfyUIEngineObjectInfo, error)
	WithEngineInstanceLock(context.Context, string, func() error) error
	DeleteEngineInstance(ctx context.Context, id string) error
	CountRunsByEngineInstance(ctx context.Context, id string) (int64, error)
	ListComfyUIWorkflows(ctx context.Context, req *iapiserver.ComfyUIWorkflowListRequest) ([]*iapiserver.ComfyUIWorkflow, int64, error)
	GetComfyUIWorkflow(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflow, error)
	AddComfyUIWorkflow(ctx context.Context, data *iapiserver.ComfyUIWorkflow) (*iapiserver.ComfyUIWorkflow, error)
	UpdateComfyUIWorkflow(ctx context.Context, data *iapiserver.ComfyUIWorkflow, expectedVersion int64) (*iapiserver.ComfyUIWorkflow, error)
	ListComfyUIWorkflowDuplicateIDs(ctx context.Context, ownerUserID, checksum string) ([]string, error)
	ListComfyUIWorkflowValidations(
		ctx context.Context,
		req *iapiserver.ComfyUIWorkflowValidationListRequest,
	) ([]*iapiserver.ComfyUIWorkflowValidation, int64, error)
	GetComfyUIWorkflowValidation(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflowValidation, error)
	AddComfyUIWorkflowValidation(ctx context.Context, data *iapiserver.ComfyUIWorkflowValidation) (*iapiserver.ComfyUIWorkflowValidation, error)
	ListComfyUIWorkflowTestRuns(ctx context.Context, req *iapiserver.ComfyUIWorkflowTestRunListRequest) ([]*iapiserver.ComfyUIWorkflowTestRun, int64, error)
	GetComfyUIWorkflowTestRun(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflowTestRun, error)
	GetComfyUIWorkflowTestRunByIdempotency(ctx context.Context, ownerUserID, key string) (*iapiserver.ComfyUIWorkflowTestRun, error)
	AddComfyUIWorkflowTestRun(ctx context.Context, data *iapiserver.ComfyUIWorkflowTestRun) (*iapiserver.ComfyUIWorkflowTestRun, error)
	// BindComfyUIWorkflowTestRunDAG records the canonical Task Center owner after DAG creation succeeds.
	BindComfyUIWorkflowTestRunDAG(ctx context.Context, testRunID, dagTaskGroupID string) (*iapiserver.ComfyUIWorkflowTestRun, error)
	// SetComfyUIWorkflowTestRunExternalJob records the provider job identifier without copying execution state.
	SetComfyUIWorkflowTestRunExternalJob(ctx context.Context, testRunID, externalJobID string) (*iapiserver.ComfyUIWorkflowTestRun, error)
	// SetComfyUIWorkflowTestRunOutputs stores only the selected temporary preview descriptors.
	SetComfyUIWorkflowTestRunOutputs(ctx context.Context, testRunID string, outputs []iapiserver.ComfyUIWorkflowTestOutput) (*iapiserver.ComfyUIWorkflowTestRun, error)
	// FailComfyUIWorkflowTestRunCreation records a failure that occurs before a DAG can be bound.
	FailComfyUIWorkflowTestRunCreation(ctx context.Context, testRunID, failure string) (*iapiserver.ComfyUIWorkflowTestRun, error)
	GetComfyUIWorkflowConversion(ctx context.Context, workflowID, ownerUserID, idempotencyKey string) (*iapiserver.ComfyUIWorkflowConvertResult, error)
	ConvertComfyUIWorkflow(
		ctx context.Context,
		workflowID, ownerUserID, idempotencyKey string,
		template *iapiserver.ApplicationTemplate,
		version *iapiserver.ApplicationTemplateVersion,
	) (*iapiserver.ComfyUIWorkflowConvertResult, error)
	ListEngineBindings(ctx context.Context, req *iapiserver.EngineCapabilityBindingListRequest) ([]*iapiserver.EngineCapabilityBinding, int64, error)
	GetEngineBinding(ctx context.Context, id string) (*iapiserver.EngineCapabilityBinding, error)
	AddEngineBinding(ctx context.Context, data *iapiserver.EngineCapabilityBinding) (*iapiserver.EngineCapabilityBinding, error)
	UpdateEngineBinding(ctx context.Context, data *iapiserver.EngineCapabilityBinding, expectedVersion int64) (*iapiserver.EngineCapabilityBinding, error)
	DeleteEngineBinding(ctx context.Context, id string) error
	ListTemplates(ctx context.Context, req *iapiserver.ApplicationTemplateListRequest) ([]*iapiserver.ApplicationTemplate, int64, error)
	GetTemplate(ctx context.Context, id string) (*iapiserver.ApplicationTemplate, error)
	AddTemplateWithVersion(
		ctx context.Context,
		data *iapiserver.ApplicationTemplate,
		version *iapiserver.ApplicationTemplateVersion,
	) (*iapiserver.ApplicationTemplate, error)
	ListTemplateVersions(ctx context.Context, req *iapiserver.ApplicationTemplateVersionListRequest) ([]*iapiserver.ApplicationTemplateVersion, int64, error)
	GetTemplateVersion(ctx context.Context, id string) (*iapiserver.ApplicationTemplateVersion, error)
	AddTemplateVersion(ctx context.Context, data *iapiserver.ApplicationTemplateVersion) (*iapiserver.ApplicationTemplateVersion, error)
	PublishTemplateVersion(ctx context.Context, id string) (*iapiserver.ApplicationTemplateVersion, error)
	ListApplications(ctx context.Context, req *iapiserver.ApplicationListRequest) ([]*iapiserver.Application, int64, error)
	GetApplication(ctx context.Context, id string) (*iapiserver.Application, error)
	GetApplicationsByIDs(ctx context.Context, ids []string) ([]*iapiserver.Application, error)
	AddApplication(ctx context.Context, data *iapiserver.Application) (*iapiserver.Application, error)
	UpdateApplication(ctx context.Context, data *iapiserver.Application, expectedVersion int64) (*iapiserver.Application, error)
	ListApplicationVersions(ctx context.Context, req *iapiserver.ApplicationVersionListRequest) ([]*iapiserver.ApplicationVersion, int64, error)
	ListPublishedApplicationVersions(ctx context.Context) ([]*iapiserver.ApplicationVersion, error)
	GetApplicationVersion(ctx context.Context, id string) (*iapiserver.ApplicationVersion, error)
	GetApplicationVersionsByIDs(ctx context.Context, ids []string) ([]*iapiserver.ApplicationVersion, error)
	AddApplicationVersion(ctx context.Context, data *iapiserver.ApplicationVersion) (*iapiserver.ApplicationVersion, error)
	PublishApplicationVersion(ctx context.Context, id string) (*iapiserver.ApplicationVersion, error)
	GetApplicationRun(ctx context.Context, id string) (*iapiserver.ApplicationRun, error)
	ListApplicationRuns(ctx context.Context, req *iapiserver.ApplicationRunListRequest) ([]*iapiserver.ApplicationRun, int64, error)
	GetApplicationRunsByIDs(ctx context.Context, ownerUserID string, ids []string) ([]*iapiserver.ApplicationRun, error)
	GetApplicationRunByIdempotency(ctx context.Context, ownerUserID, key string) (*iapiserver.ApplicationRun, error)
	AddApplicationRun(ctx context.Context, data *iapiserver.ApplicationRun) (*iapiserver.ApplicationRun, error)
	BindApplicationRunTask(ctx context.Context, id, atomicTaskID, status, taskStatus string, taskVersion int64, failure string) (*iapiserver.ApplicationRun, error)
	ProjectApplicationRun(
		ctx context.Context,
		id string,
		taskVersion int64,
		status, failure string,
		outputs []map[string]any,
	) (*iapiserver.ApplicationRun, error)
	// ListApplicationArtifactRefsByRun 读取 ApplicationRun 的新 Artifact 引用投影。
	ListApplicationArtifactRefsByRun(ctx context.Context, runID string) ([]*iapiserver.ApplicationArtifactRef, error)
	// ProjectApplicationArtifactRef 仅接受更高 Artifact resource version，并按运行输出键幂等投影。
	ProjectApplicationArtifactRef(ctx context.Context, data *iapiserver.ApplicationArtifactRef) (*iapiserver.ApplicationArtifactRef, bool, error)
	// ListArtifactsByRun 仅供切换期间读取旧 aiapp_artifacts 回填目标。
	// Deprecated: 新运行不得写入旧投影。
	ListArtifactsByRun(ctx context.Context, runID string) ([]*iapiserver.ApplicationArtifact, error)
	// Deprecated: 新运行不得写入旧 aiapp_artifacts。
	UpsertArtifact(ctx context.Context, data *iapiserver.ApplicationArtifact) (*iapiserver.ApplicationArtifact, error)
	// Deprecated: 新运行不得更新旧 aiapp_artifacts。
	UpdateArtifactRegistration(
		ctx context.Context,
		id, status, assetID, errorCode, failureDetail string,
		expectedVersion int64,
	) (*iapiserver.ApplicationArtifact, error)
}

type FeatureFlagStore interface {
	List(ctx context.Context) ([]*iapiserver.FeatureFlag, error)
	Upsert(ctx context.Context, data *iapiserver.FeatureFlag) (*iapiserver.FeatureFlag, error)
}

type PermissionStore interface {
	List(ctx context.Context) ([]*iapiserver.Permission, error)
}
