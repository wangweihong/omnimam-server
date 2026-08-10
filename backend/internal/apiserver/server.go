package apiserver

import (
	"context"
	"os"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/shutdown"
	"github.com/wangweihong/gotoolbox/pkg/shutdown/managers/posixsignal"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/config"
	ssectrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/sse"
	infrastructureclient "github.com/wangweihong/omnimam/backend/internal/apiserver/infrastructureclient"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/options"
	agentsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/agent"
	aichatsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/aichat"
	appplatformsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	appsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	appstudiosvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/appstudio"
	assetlibrarysvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/assetlibrary"
	identitysvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/identity"
	mcpsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/mcp"
	engine "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	modeladapters "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters"
	comfyuiadapter "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/providers/comfyui"
	taskcentersvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	usermodelsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/usermodel"
	workflowcanvassvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/workflowcanvas"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store/database"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store/postgresql"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
	"github.com/wangweihong/omnimam/backend/internal/pkg/agentgrant"
	"github.com/wangweihong/omnimam/backend/internal/taskfunctionregistry"
	"github.com/wangweihong/omnimam/backend/pkg/httpsvr"
	"github.com/wangweihong/omnimam/backend/pkg/httpsvr/genericoptions"
	mcpprotocol "github.com/wangweihong/omnimam/backend/pkg/mcp"
)

type server struct {
	// api服务,提供http和tls
	httpServer *httpsvr.GenericHTTPServer
	// 控制服务关闭时处理动作, 如捕捉到信号后如何处理
	gracefulShutdown       *shutdown.GracefulShutdown
	assetUpload            *options.AssetUploadOptions
	applicationPlatform    appsvc.ApplicationPlatformSrv
	agent                  *agentsvc.Service
	appStudio              *appstudiosvc.Service
	taskCenter             taskcentersvc.TaskCenterSrv
	userModel              *usermodelsvc.Service
	aiChat                 aichatsvc.AIChatSrv
	workflowRuntime        workflowruntime.WorkflowRuntime
	authOptions            *options.AuthOptions
	sseOptions             *options.SSEOptions
	mcpOptions             *options.MCPOptions
	mcpProcessor           *mcpprotocol.Processor
	userEventCleanupCtx    context.Context
	userEventCleanupCancel context.CancelFunc
}

// preparedServer is a private wrapper that enforces a call of PrepareRun() before Run can be invoked.
type preparedServer struct {
	*server
}

type ExtraConfig struct {
	//postgresqlOptions *genericoptions.PostgresSQLOptions
	databaseOptions *genericoptions.DatabaseOptions
}

func (c *ExtraConfig) Complete() *CompletedExtraConfig {
	// if c.postgresqlOptions.Database == "" {
	// 	c.postgresqlOptions.Database = "apiserver"
	// }
	if c.databaseOptions.Type == "" {
		c.databaseOptions.Type = "postgresql"
	}

	return &CompletedExtraConfig{c}
}

// 创建服务器实例.
func createServer(cfg *config.Config) (*server, error) {
	gs := shutdown.New()
	gs.AddShutdownManager(posixsignal.NewPosixSignalManager())

	// 构建通用的http(s) server服务配置
	genericConfig, err := buildGenericHTTPServerConfig(cfg)
	if err != nil {
		return nil, err
	}

	extraConfig, err := buildExtraConfig(cfg)
	if err != nil {
		return nil, err
	}

	// 补全通用服务器配置, 并生成通用服务实例
	genericServer, err := genericConfig.Complete().New()
	if err != nil {
		return nil, err
	}

	if err := extraConfig.Complete().New(); err != nil {
		return nil, err
	}
	storeIns := store.Client()
	if _, err := assetlibrarysvc.ReconcileDefaultLocalStorageBackend(context.Background(), storeIns.StorageBackends()); err != nil {
		return nil, errors.Wrap(err, "reconcile default local storage backend")
	}
	registrations, err := modeladapters.NewRegistrations()
	if err != nil {
		return nil, errors.Wrap(err, "load application platform adapter registrations")
	}
	runtimeRegistry, err := engine.NewRuntimeRegistry(registrations)
	if err != nil {
		return nil, errors.Wrap(err, "build application platform runtime registry")
	}
	capabilityRegistry, err := engine.NewProviderCapabilityRegistry(registrations, runtimeRegistry, modeladapters.NewCapabilityValidators()...)
	if err != nil {
		return nil, errors.Wrap(err, "build application platform provider capabilities")
	}
	adapters := modeladapters.NewEngineAdapters()
	executors := modeladapters.NewOperationExecutors()
	if err := modeladapters.ValidateImplementations(runtimeRegistry, adapters, executors); err != nil {
		return nil, errors.Wrap(err, "validate application platform adapter implementations")
	}
	credentialBroker := usermodelsvc.NewCredentialBroker(0)
	if cfg.InfrastructureClientOptions == nil {
		return nil, errors.New("infrastructure client options are required for agent grants")
	}
	grantCodec, err := agentgrant.NewCodec(cfg.InfrastructureClientOptions.Token, time.Hour)
	if err != nil {
		return nil, errors.Wrap(err, "construct agent grant codec")
	}
	infrastructureClient, err := infrastructureclient.New(cfg.InfrastructureClientOptions.BaseURL, cfg.InfrastructureClientOptions.Token)
	if err != nil {
		return nil, errors.Wrap(err, "construct infrastructure client")
	}
	userModelGateway, err := engine.NewUserModelGatewayService(engine.UserModelGatewayDependencies{
		Runtime: runtimeRegistry, Adapters: adapters, Executors: executors, Credentials: credentialBroker,
	})
	if err != nil {
		return nil, errors.Wrap(err, "construct user model gateway")
	}
	userModelService, err := usermodelsvc.New(usermodelsvc.Dependencies{
		Store: storeIns, Gateway: userModelGateway, Credentials: credentialBroker, Grants: grantCodec,
	})
	if err != nil {
		return nil, errors.Wrap(err, "construct user model service")
	}
	aiChatService := aichatsvc.NewService(aichatsvc.Dependencies{
		Store: storeIns, ModelReader: appplatformsvc.NewLegacyService(storeIns),
		UserModels: userModelService, Gateway: userModelGateway,
	})
	assets := appsvc.NoopArtifactLifecycle{}
	events := appsvc.NoopEventPublisher{}
	workflowRuntime := workflowruntime.WorkflowRuntime(workflowruntime.UnavailableRuntime{})
	if cfg.WorkflowRuntimeOptions != nil && cfg.WorkflowRuntimeOptions.Enabled {
		workflowRuntime, err = workflowruntime.NewConductor(workflowruntime.ConductorConfig{
			BaseURL: cfg.WorkflowRuntimeOptions.BaseURL, AuthKey: cfg.WorkflowRuntimeOptions.AuthKey,
			AuthSecret: cfg.WorkflowRuntimeOptions.AuthSecret, HTTPTimeout: cfg.WorkflowRuntimeOptions.HTTPTimeout,
			PollInterval: cfg.WorkflowRuntimeOptions.PollInterval,
		})
		if err != nil {
			return nil, errors.Wrap(err, "construct workflow runtime")
		}
	}
	reconcileRegistry := taskcentersvc.NewReconcileRegistry()
	functionRegistry, err := taskfunctionregistry.New()
	if err != nil {
		return nil, errors.Wrap(err, "load task center function registry")
	}
	taskCenterService := taskcentersvc.NewServiceWithFunctionRegistry(storeIns, workflowRuntime, reconcileRegistry, functionRegistry, assetlibrarysvc.NewArtifactSummaryReader(storeIns.AssetsV1()),
		appplatformsvc.FunctionAssetThumbnailGenerate, "application-platform.run", "task.schedule.acquire",
		"comfyui.submit", "comfyui.poll", "comfyui.collect_preview",
		assetlibrarysvc.FunctionArtifactProcess, assetlibrarysvc.FunctionRepresentationFinalize)
	applicationPlatformService, err := appsvc.NewService(appsvc.Dependencies{
		Store: storeIns, Runtime: runtimeRegistry, Capabilities: capabilityRegistry,
		Adapters: adapters, Tasks: taskCenterService, Assets: assets, Events: events,
	})
	if err != nil {
		return nil, errors.Wrap(err, "construct application platform service")
	}
	if err := applicationPlatformService.ReconcileRequiredEngineBindings(context.Background()); err != nil {
		return nil, errors.Wrap(err, "reconcile required application platform bindings")
	}
	if err := reconcileRegistry.Register(engine.NewEngineHealthReconcileHandler(storeIns, applicationPlatformService)); err != nil {
		return nil, errors.Wrap(err, "register engine health reconciler")
	}
	if err := reconcileRegistry.Register(comfyuiadapter.NewComfyUIObjectInfoReconcileHandler(storeIns, applicationPlatformService)); err != nil {
		return nil, errors.Wrap(err, "register ComfyUI object_info reconciler")
	}
	sourceDir := os.Getenv("OMNIMAM_APPSTUDIO_SOURCE_DIR")
	if sourceDir == "" {
		sourceDir = "data/appstudio/source"
	}
	sourceStore, err := appstudiosvc.NewLocalSourceContentStore(sourceDir)
	if err != nil {
		return nil, errors.Wrap(err, "construct appstudio source content store")
	}
	appStudioService, err := appstudiosvc.New(appstudiosvc.Dependencies{Store: storeIns.AppStudio(), Tasks: taskCenterService, Sources: sourceStore, Artifacts: storeIns.AssetsV1(), Grants: grantCodec})
	if err != nil {
		return nil, errors.Wrap(err, "construct appstudio service")
	}
	agentService, err := agentsvc.New(agentsvc.Dependencies{Store: storeIns.Agents(), Tasks: taskCenterService, Workspaces: appStudioService, Models: userModelService, Scopes: appStudioService, Diagnostics: infrastructureClient, Grants: grantCodec})
	if err != nil {
		return nil, errors.Wrap(err, "construct agent service")
	}
	appStudioService.SetCodingAgentCreator(agentService)
	var mcpProcessor *mcpprotocol.Processor
	if cfg.MCPOptions != nil && cfg.MCPOptions.Enabled {
		mcpFactory, ok := storeIns.(store.MCPStoreFactory)
		if !ok || mcpFactory.MCPTaskBindings() == nil {
			return nil, errors.New("MCP Task Binding store is unavailable")
		}
		assetService := newAssetLibraryService(storeIns, taskCenterService, applicationPlatformService)
		if assetService == nil {
			return nil, errors.New("Asset Library service is unavailable for MCP")
		}
		mcpService, err := mcpsvc.New(mcpsvc.Dependencies{
			Capabilities: runtimeRegistry, Applications: applicationPlatformService,
			Tasks: taskCenterService, Assets: assetService, Bindings: mcpFactory.MCPTaskBindings(),
			AgentGrants: storeIns.Agents(), WorkloadScopes: appStudioService,
			Config: mcpsvc.Config{
				DiscoverTTL: cfg.MCPOptions.DiscoverTTL, ResourceTTL: cfg.MCPOptions.ResourceTTL,
				TaskTTL: cfg.MCPOptions.TaskTTL, TaskPollInterval: cfg.MCPOptions.TaskPollInterval,
				UploadTTL: cfg.MCPOptions.UploadTTL, PublicBaseURL: cfg.MCPOptions.PublicBaseURL,
				RequestRate: cfg.MCPOptions.RequestRate, RequestBurst: cfg.MCPOptions.RequestBurst,
				ToolRate: cfg.MCPOptions.ToolRate, ToolBurst: cfg.MCPOptions.ToolBurst,
				MaxUploadBytes: cfg.MCPOptions.MaxUploadBytes, MaxLimiterScopes: cfg.MCPOptions.MaxLimiterScopes,
			},
		})
		if err != nil {
			return nil, errors.Wrap(err, "construct MCP service")
		}
		mcpProcessor, err = mcpprotocol.NewProcessor(mcpService)
		if err != nil {
			return nil, errors.Wrap(err, "construct MCP protocol processor")
		}
	}

	server := &server{
		httpServer:          genericServer,
		gracefulShutdown:    gs,
		assetUpload:         cfg.AssetUploadOptions,
		applicationPlatform: applicationPlatformService,
		agent:               agentService,
		appStudio:           appStudioService,
		taskCenter:          taskCenterService,
		userModel:           userModelService,
		aiChat:              aiChatService,
		workflowRuntime:     workflowRuntime,
		authOptions:         cfg.AuthOptions,
		sseOptions:          cfg.SSEOptions,
		mcpOptions:          cfg.MCPOptions,
		mcpProcessor:        mcpProcessor,
	}

	return server, nil
}

type CompletedExtraConfig struct {
	*ExtraConfig
}

func (c *CompletedExtraConfig) New() error {
	// 连接数据库,检测连接
	// storeIns, err := postgresql.GetPostgresSQLFactoryOr(c.postgresqlOptions)
	// if err != nil {
	// 	return errors.Wrap(err, "completeExtra fail")
	// }
	storeIns, err := database.GetDatabaseFactoryOr(c.databaseOptions)
	if err != nil {
		return errors.Wrap(err, "completeExtra fail")
	}

	// 新建数据库表
	if err := storeIns.EnsureScheme(
		// identity
		&iapiserver.User{},
		&iapiserver.IdentityUser{},
		&iapiserver.IdentityOpaqueExchange{},
		&iapiserver.IdentityRegistrationApplication{},
		&iapiserver.IdentityRole{},
		&iapiserver.IdentityPermissionDefinition{},
		&iapiserver.IdentityGroup{},
		&iapiserver.IdentityResourceAccessGrant{},
		&iapiserver.IdentityAuthSession{},
		&iapiserver.IdentityTokenCredential{},
		&iapiserver.IdentityRefreshToken{},
		&iapiserver.IdentityServiceAccount{},
		&iapiserver.IdentityServiceAccountCredential{},
		&iapiserver.IdentityRolePermissionGrant{},
		&iapiserver.IdentityUserRoleGrant{},
		&iapiserver.IdentityServiceAccountRoleGrant{},
		&iapiserver.IdentityUserDeletionCheck{},
		&iapiserver.IdentityUserDeletionCheckItem{},
		&iapiserver.IdentityGroupMember{},
		&iapiserver.IdentityGroupRoleGrant{},
		&iapiserver.IdentityOutboxEvent{},
		// assets
		&iapiserver.AssetLibrary{},
		&iapiserver.AssetCategory{},
		&iapiserver.AssetItem{},

		// prompts
		&iapiserver.PromptLibrary{},
		&iapiserver.PromptCategory{},
		&iapiserver.PromptItem{},

		// canvases
		&iapiserver.Project{},
		&iapiserver.WorkflowNodeDefinition{},
		&iapiserver.WorkflowCanvas{},
		&iapiserver.CanvasVersion{},
		&iapiserver.WorkflowCanvasRun{},
		&iapiserver.CanvasFlowRun{},
		&iapiserver.CanvasNodeRun{},
		&iapiserver.CanvasNodeRunFlowRef{},
		&iapiserver.CanvasNodeRunTaskBinding{},
		&iapiserver.CanvasNodeRunOutputBinding{},
		&iapiserver.WorkflowCanvasOutbox{},
		&iapiserver.WorkflowCanvasReconcileCursor{},

		// platform contracts
		&iapiserver.PlatformSystemAuthConfig{},
		&iapiserver.PlatformAuditLog{},
		&iapiserver.PlatformOutboxEvent{},
		&iapiserver.Provider{},
		&iapiserver.ProviderModel{},
		&iapiserver.ModelHealthCheck{},
		&iapiserver.ProviderCapability{},
		&iapiserver.SystemLLMConfig{},
		&iapiserver.StorageBackend{},
		&iapiserver.Asset{},
		&iapiserver.AssetThumbnail{},
		&iapiserver.Tag{},
		&iapiserver.AssetTag{},
		&iapiserver.AssetGroup{},
		&iapiserver.AssetGroupMember{},
		&iapiserver.AssetRelation{},
		&iapiserver.UserAsset{},
		&iapiserver.AssetBlob{},
		&iapiserver.Artifact{},
		&iapiserver.AssetVersion{},
		&iapiserver.AssetRepresentation{},
		&iapiserver.AssetUploadSession{},
		&iapiserver.AssetCollection{},
		&iapiserver.AssetCollectionItem{},
		&iapiserver.ArtifactAssetRegistration{},
		&iapiserver.UserAssetLabel{},
		&iapiserver.UserAssetTag{},
		&iapiserver.AtomicTask{},
		&iapiserver.TaskAttempt{},
		&iapiserver.TaskGroup{},
		&iapiserver.DAGTaskGroup{},
		&iapiserver.TaskSchedule{},
		&iapiserver.ScheduleReconcileState{},
		&iapiserver.TaskScheduleExecution{},
		&iapiserver.RuntimeProjectionEvent{},
		&iapiserver.UserEvent{},
		&iapiserver.FeatureFlag{},
		&iapiserver.Permission{},

		// agent
		&iapiserver.Agent{},
		&iapiserver.AgentSession{},
		&iapiserver.AgentMessage{},
		&iapiserver.AgentInvocation{},
		&iapiserver.AgentMemory{},
		&iapiserver.AgentModelBinding{},
		&iapiserver.AgentWorkspaceBinding{},
		&iapiserver.AgentSkillBinding{},
		&iapiserver.AgentMCPBinding{},
		&iapiserver.AgentMCPBindingRevision{},
		&iapiserver.AgentRuntimeGrant{},
		&iapiserver.AgentRuntimeBinding{},
		&iapiserver.AgentOperationEvent{},
		&iapiserver.AgentOutbox{},

		// appstudio
		&iapiserver.StudioApplication{},
		&iapiserver.StudioSourceRepository{},
		&iapiserver.StudioWorkspace{},
		&iapiserver.StudioSourceFile{},
		&iapiserver.StudioWorkspaceRevision{},
		&iapiserver.StudioChangeSet{},
		&iapiserver.StudioSourceSnapshot{},
		&iapiserver.StudioApplicationVersion{},
		&iapiserver.StudioPreviewRuntime{},
		&iapiserver.StudioRuntimeConfig{},
		&iapiserver.StudioBuild{},
		&iapiserver.StudioRelease{},
		&iapiserver.StudioRuntimeInstance{},
		&iapiserver.AppStudioOutbox{},

		// infrastructure
		&iapiserver.InfraRuntimeProfile{},
		&iapiserver.InfraNode{},
		&iapiserver.InfraRuntime{},
		&iapiserver.InfraRuntimeEndpoint{},
		&iapiserver.InfraRuntimeMount{},
		&iapiserver.InfraRuntimeConfigBinding{},
		&iapiserver.InfraRuntimeOutput{},
		&iapiserver.InfraRuntimeEvent{},

		// application platform
		&iapiserver.EngineInstance{},
		&iapiserver.ComfyUIEngineObjectInfo{},
		&iapiserver.EngineCapabilityBinding{},
		&iapiserver.ComfyUIWorkflow{},
		&iapiserver.ComfyUIWorkflowValidation{},
		&iapiserver.ComfyUIWorkflowTestRun{},
		&iapiserver.ApplicationTemplate{},
		&iapiserver.ApplicationTemplateVersion{},
		&iapiserver.Application{},
		&iapiserver.ApplicationVersion{},
		&iapiserver.ApplicationRun{},
		&iapiserver.ApplicationArtifactRef{},
		&iapiserver.ApplicationArtifact{},

		// mcp
		&iapiserver.MCPTaskBinding{},

		// notification center
		&iapiserver.NotificationTopic{},
		&iapiserver.NotificationEvent{},
		&iapiserver.Notification{},
		&iapiserver.NotificationEventLink{},
		&iapiserver.NotificationRecipientCounter{},
		&iapiserver.NotificationPreference{},
		&iapiserver.NotificationDelivery{},
		&iapiserver.NotificationOutbox{},

		// ai chat
		&iapiserver.AIChatAssistant{},
		&iapiserver.AIChatTopic{},
		&iapiserver.AIChatMessage{},
		&iapiserver.AIChatGeneration{},
		&iapiserver.AIChatQuickPhrase{},
		&iapiserver.AIChatMessageTranslation{},
	); err != nil {
		return errors.Wrap(err, "EnsureScheme fail")
	}
	identityStore := storeIns.Identities()
	if identityStore == nil {
		return errors.New("Identity store is unavailable")
	}
	if err := identityStore.EnsureDefaultPermissions(context.Background(), identitysvc.DefaultPermissions(), identitysvc.DefaultRolePermissions()); err != nil {
		return errors.Wrap(err, "initialize Identity permissions")
	}
	if err := workflowcanvassvc.ReconcileBuiltInNodeDefinitions(context.Background(), storeIns.WorkflowCanvases()); err != nil {
		return errors.Wrap(err, "reconcile built-in workflow canvas node definitions")
	}
	store.SetClient(storeIns)
	return nil
}

// InitializeStore initializes the shared PostgreSQL store and reconciles shared system catalogs for non-HTTP processes.
func InitializeStore(cfg *config.Config) error {
	extraConfig, err := buildExtraConfig(cfg)
	if err != nil {
		return err
	}
	return extraConfig.Complete().New()
}

// 根据服务器配置应用到通用服务器配置上.
func buildGenericHTTPServerConfig(cfg *config.Config) (genericConfig *httpsvr.Config, lastErr error) {
	genericConfig = httpsvr.NewConfig()
	if lastErr = cfg.GenericServerRunOptions.ApplyTo(genericConfig); lastErr != nil {
		return
	}

	if lastErr = cfg.FeatureOptions.ApplyTo(genericConfig); lastErr != nil {
		return
	}

	if lastErr = cfg.InsecureServing.ApplyTo(genericConfig); lastErr != nil {
		return
	}

	if lastErr = cfg.SecureServing.ApplyTo(genericConfig); lastErr != nil {
		return
	}

	return
}

func BuildExtraConfig(cfg *config.Config) (*ExtraConfig, error) {
	return buildExtraConfig(cfg)
}

func buildExtraConfig(cfg *config.Config) (*ExtraConfig, error) {
	return &ExtraConfig{
		//postgresqlOptions: cfg.PostgresSQLOptions,
		databaseOptions: cfg.DatabaseOptions,
	}, nil
}

// PrepareRun prepares the server to run, by setting up the server instance.
func (s *server) PrepareRun() preparedServer {
	s.userEventCleanupCtx, s.userEventCleanupCancel = context.WithCancel(context.Background())
	initRouter(s.httpServer.Engine, s.applicationPlatform, s.taskCenter, s.userModel, s.aiChat, s.agent, s.appStudio, s.authOptions, s.sseOptions, s.mcpProcessor, s.mcpOptions)
	// 设置服务优雅退出回调处理
	s.gracefulShutdown.AddShutdownCallback(shutdown.ShutdownFunc(func(string) error {
		ssectrl.BeginDraining()
		s.userEventCleanupCancel()
		if s.workflowRuntime != nil {
			_ = s.workflowRuntime.Close()
		}
		dataStore, _ := postgresql.GetPostgresSQLFactoryOr(nil)
		if dataStore != nil {
			_ = dataStore.Close()
		}
		s.httpServer.Close()
		return nil
	}))

	return preparedServer{s}
}

func (s preparedServer) Run(stopCh <-chan struct{}) error {
	startUserEventCleanup(s.userEventCleanupCtx)
	startMCPTaskBindingCleanup(s.userEventCleanupCtx)
	if s.assetUpload != nil {
		appplatformsvc.SetChunkUploadTempDir(s.assetUpload.ChunkTempDir)
		appplatformsvc.StartChunkUploadCleanup(stopCh, time.Duration(s.assetUpload.ChunkCleanupHours)*time.Hour)
	}
	if s.userModel != nil {
		s.userModel.StartProviderModelHealthChecks(stopCh, 30*time.Second)
	}
	// start shutdown managers
	if err := s.gracefulShutdown.Start(); err != nil {
		log.Fatalf("start shutdown manager failed: %s", err.Error())
	}
	return s.httpServer.Run()
}

// startMCPTaskBindingCleanup 只物理删除过期协议映射，不修改 ApplicationRun、AtomicTask 或制品事实。
func startMCPTaskBindingCleanup(ctx context.Context) {
	prune := func(now time.Time) {
		dataStore := store.Client()
		factory, ok := dataStore.(store.MCPStoreFactory)
		if !ok || factory.MCPTaskBindings() == nil {
			return
		}
		if _, err := factory.MCPTaskBindings().DeleteExpired(ctx, now); err != nil {
			log.Warnf("prune expired MCP task bindings: %v", err)
		}
	}
	go func() {
		prune(time.Now())
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				prune(now)
			}
		}
	}()
}

// startUserEventCleanup 定期清理过期 SSE 投影，清理失败不影响任务执行和 API 服务。
func startUserEventCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				if dataStore := store.Client(); dataStore != nil && dataStore.UserEvents() != nil {
					if _, err := dataStore.UserEvents().PruneExpired(ctx, now); err != nil {
						log.Warnf("prune expired SSE user events: %v", err)
					}
				}
			}
		}
	}()
}
