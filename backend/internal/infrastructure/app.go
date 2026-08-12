package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/internal/apiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/config"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/options"
	agentsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/agent"
	appstudiosvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/appstudio"
	gitlabsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/gitlab"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/infrastructure/providers/dockerruntime"
	"github.com/wangweihong/omnimam/backend/internal/pkg/agentgrant"
	"github.com/wangweihong/omnimam/backend/pkg/app"
)

func NewApp(basename string) *app.App {
	opts := options.NewOptions()
	opts.Name = "infra-server"
	return app.NewApp("infra-server", basename, app.WithOptions(opts), app.WithDescription("infra-server"), app.WithDefaultValidArgs(), app.WithRunFunc(func(string) error {
		log.Init(opts.Log)
		defer log.Flush()
		cfg, err := config.CreateConfigFromOptions(opts)
		if err != nil {
			return err
		}
		return Run(cfg)
	}))
}
func Run(cfg *config.Config) error {
	if err := apiserver.InitializeStore(cfg); err != nil {
		return err
	}
	// 加载工作镜像
	var images dockerruntime.MapProfileImages
	if err := json.Unmarshal([]byte(os.Getenv("OMNIMAM_INFRA_PROFILE_IMAGES")), &images); err != nil || len(images) == 0 {
		return fmt.Errorf("OMNIMAM_INFRA_PROFILE_IMAGES must be a non-empty JSON object")
	}
	provider, err := dockerruntime.NewDockerProvider(
		os.Getenv("OMNIMAM_DOCKER_SOCKET"),
		os.Getenv("OMNIMAM_DOCKER_API_VERSION"),
		images,
		os.Getenv("OMNIMAM_MCP_RUNTIME_CA_FILE"),
	)
	if err != nil {
		return err
	}

	service, err := NewService(store.Client().Infrastructure(), provider)
	if err != nil {
		return err
	}
	gitLabSourceProvider, err := gitlabsvc.NewSourceProvider(store.Client().GitLab(), gitlabsvc.NewHTTPClientFactory())
	if err != nil {
		return err
	}
	sourceResolver, err := appstudiosvc.New(appstudiosvc.Dependencies{
		Store: store.Client().AppStudio(), SourceProvider: gitLabSourceProvider, ProjectInitializer: gitLabSourceProvider,
	})
	if err != nil {
		return err
	}
	service.SetSourceArchiveResolver(sourceResolver)
	if cfg.InfrastructureClientOptions == nil {
		return fmt.Errorf("infrastructure client options are required")
	}
	grantCodec, err := agentgrant.NewCodec(cfg.InfrastructureClientOptions.Token, time.Hour)
	if err != nil {
		return err
	}
	runtimeGitResolver, err := appstudiosvc.New(appstudiosvc.Dependencies{
		Store: store.Client().AppStudio(), SourceProvider: gitLabSourceProvider, ProjectInitializer: gitLabSourceProvider, Grants: grantCodec,
	})
	if err != nil {
		return err
	}
	service.SetRuntimeGitAccessResolver(runtimeGitResolver)
	agentResolver, err := agentsvc.NewMCPResolver(agentsvc.MCPResolverDependencies{
		Store: store.Client().Agents(), JWTSecret: []byte(cfg.AuthOptions.JWTSecret),
		PlatformMCPBaseURL: cfg.MCPOptions.PublicBaseURL,
	})
	if err != nil {
		return err
	}
	service.SetMCPBindingResolver(agentResolver)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := service.ReconcileCatalog(ctx); err != nil {
		return err
	}
	server, err := NewServer(service, os.Getenv("OMNIMAM_INFRA_SERVICE_TOKEN"))
	if err != nil {
		return err
	}
	address := os.Getenv("OMNIMAM_INFRA_LISTEN_ADDRESS")
	if address == "" {
		address = "127.0.0.1:8082"
	}
	httpServer := &http.Server{Addr: address, Handler: server.Handler(), ReadHeaderTimeout: 10 * time.Second}
	errCh := make(chan error, 1)
	go func() { errCh <- httpServer.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
