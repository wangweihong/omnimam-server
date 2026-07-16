package apiserver

import (
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/shutdown"
	"github.com/wangweihong/gotoolbox/pkg/shutdown/managers/posixsignal"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/config"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/options"
	appsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	platformsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/platform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store/database"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store/postgresql"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/taskexecutor"
	"github.com/wangweihong/omnimam/backend/pkg/httpsvr"
	"github.com/wangweihong/omnimam/backend/pkg/httpsvr/genericoptions"
)

type server struct {
	// api服务,提供http和tls
	httpServer *httpsvr.GenericHTTPServer
	// 控制服务关闭时处理动作, 如捕捉到信号后如何处理
	gracefulShutdown    *shutdown.GracefulShutdown
	assetUpload         *options.AssetUploadOptions
	applicationPlatform appsvc.ApplicationPlatformSrv
	dispatcher          *taskexecutor.Dispatcher
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
	dispatcher := taskexecutor.NewDispatcher(storeIns)
	applicationExecutor, err := appsvc.NewApplicationRunExecutor(storeIns, runtimeRegistry, capabilityRegistry, adapters, executors, assets, events)
	if err != nil {
		return nil, errors.Wrap(err, "construct application platform task executor")
	}
	dispatcher.RegisterCapability("application-platform.run", "application-platform", applicationExecutor)
	applicationPlatformService, err := appsvc.NewService(appsvc.Dependencies{
		Store: storeIns, Runtime: runtimeRegistry, Capabilities: capabilityRegistry,
		Adapters: adapters, Executors: executors, Dispatcher: dispatcher, Assets: assets, Events: events,
	})
	if err != nil {
		return nil, errors.Wrap(err, "construct application platform service")
	}

	server := &server{
		httpServer:          genericServer,
		gracefulShutdown:    gs,
		assetUpload:         cfg.AssetUploadOptions,
		applicationPlatform: applicationPlatformService,
		dispatcher:          dispatcher,
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
		&iapiserver.Canvas{},

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
		&iapiserver.TaskDefinition{},
		&iapiserver.TaskRun{},
		&iapiserver.TaskAttempt{},
		&iapiserver.Worker{},
		&iapiserver.ExecutionLease{},
		&iapiserver.TaskRunEvent{},
		&iapiserver.WatchdogRecord{},
		&iapiserver.FeatureFlag{},
		&iapiserver.Role{},
		&iapiserver.Permission{},
		&iapiserver.UserRole{},

		// application platform
		&iapiserver.EngineInstance{},
		&iapiserver.EngineCapabilityBinding{},
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
	initRouter(s.httpServer.Engine, s.applicationPlatform)
	// 设置服务优雅退出回调处理
	s.gracefulShutdown.AddShutdownCallback(shutdown.ShutdownFunc(func(string) error {
		if s.dispatcher != nil {
			s.dispatcher.Close()
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
