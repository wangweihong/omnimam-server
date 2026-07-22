package apiserver

import (
	"context"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/shutdown"
	"github.com/wangweihong/gotoolbox/pkg/shutdown/managers/posixsignal"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/config"
	ssectrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/sse"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/options"
	appsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	assetlibrarysvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/assetlibrary"
	platformsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/platform"
	taskcentersvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store/database"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store/postgresql"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/workflowruntime"
	"github.com/wangweihong/omnimam/backend/pkg/httpsvr"
	"github.com/wangweihong/omnimam/backend/pkg/httpsvr/genericoptions"
)

type server struct {
	// api服务,提供http和tls
	httpServer *httpsvr.GenericHTTPServer
	// 控制服务关闭时处理动作, 如捕捉到信号后如何处理
	gracefulShutdown       *shutdown.GracefulShutdown
	assetUpload            *options.AssetUploadOptions
	applicationPlatform    appsvc.ApplicationPlatformSrv
	taskCenter             taskcentersvc.TaskCenterSrv
	workflowRuntime        workflowruntime.WorkflowRuntime
	authOptions            *options.AuthOptions
	sseOptions             *options.SSEOptions
	serverMode             string
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
	runtimeRegistry, err := appregistry.LoadRuntimeRegistry()
	if err != nil {
		return nil, errors.Wrap(err, "load application platform runtime registry")
	}
	capabilityRegistry, err := appregistry.LoadProviderCapabilityRegistry(cfg.ApplicationPlatformOptions.ProviderCapabilityDirectory, runtimeRegistry)
	if err != nil {
		return nil, errors.Wrap(err, "load application platform provider capabilities")
	}
	adapters := appsvc.NewEngineAdapters()
	executors := appsvc.NewOperationExecutors()
	assets := appsvc.NoopAssetRegistrar{}
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
	taskCenterService := taskcentersvc.NewServiceWithDependencies(storeIns, workflowRuntime, reconcileRegistry, assetlibrarysvc.NewArtifactSummaryReader(storeIns.AssetsV1()),
		platformsvc.FunctionAssetThumbnailGenerate, "application-platform.run", "task.schedule.acquire",
		"comfyui.submit", "comfyui.poll", "comfyui.collect_preview",
		assetlibrarysvc.FunctionArtifactProcess, assetlibrarysvc.FunctionRepresentationFinalize)
	applicationPlatformService, err := appsvc.NewService(appsvc.Dependencies{
		Store: storeIns, Runtime: runtimeRegistry, Capabilities: capabilityRegistry,
		Adapters: adapters, Executors: executors, Tasks: taskCenterService, Assets: assets, Events: events,
	})
	if err != nil {
		return nil, errors.Wrap(err, "construct application platform service")
	}
	if err := reconcileRegistry.Register(appsvc.NewEngineHealthReconcileHandler(storeIns, applicationPlatformService)); err != nil {
		return nil, errors.Wrap(err, "register engine health reconciler")
	}
	if err := reconcileRegistry.Register(appsvc.NewComfyUIObjectInfoReconcileHandler(storeIns, applicationPlatformService)); err != nil {
		return nil, errors.Wrap(err, "register ComfyUI object_info reconciler")
	}

	server := &server{
		httpServer:          genericServer,
		gracefulShutdown:    gs,
		assetUpload:         cfg.AssetUploadOptions,
		applicationPlatform: applicationPlatformService,
		taskCenter:          taskCenterService,
		workflowRuntime:     workflowRuntime,
		authOptions:         cfg.AuthOptions,
		sseOptions:          cfg.SSEOptions,
		serverMode:          cfg.GenericServerRunOptions.Mode,
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
		//setting
		&iapiserver.Setting{},
		&iapiserver.ServiceProvider{},
		&iapiserver.IdentityProvider{},

		// identity
		&iapiserver.User{},
		&iapiserver.OneTimeToken{},
		&iapiserver.UserOTP{},

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
		&iapiserver.Provider{},
		&iapiserver.ProviderModel{},
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
		&iapiserver.Role{},
		&iapiserver.Permission{},
		&iapiserver.UserRole{},

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
		&iapiserver.ApplicationArtifact{},

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
	store.SetClient(storeIns)
	return nil
}

// InitializeStore initializes the shared PostgreSQL store and spec-v1.0.0 schema for non-HTTP processes.
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
	initRouter(s.httpServer.Engine, s.applicationPlatform, s.taskCenter, s.authOptions, s.sseOptions, s.serverMode)
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
	if s.assetUpload != nil {
		platformsvc.SetChunkUploadTempDir(s.assetUpload.ChunkTempDir)
		platformsvc.StartChunkUploadCleanup(stopCh, time.Duration(s.assetUpload.ChunkCleanupHours)*time.Hour)
	}
	platformsvc.StartProviderModelHealthCheck(stopCh, store.Client(), 30*time.Second)
	// start shutdown managers
	if err := s.gracefulShutdown.Start(); err != nil {
		log.Fatalf("start shutdown manager failed: %s", err.Error())
	}
	return s.httpServer.Run()
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
