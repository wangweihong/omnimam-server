package apiserver

import (
	"context"
	"os"
	"runtime"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/internal/apiserver/config"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/options"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/worker"
	"github.com/wangweihong/omnimam/backend/pkg/app"
)

const workerCommandDesc = `omnimam task worker`

func NewWorkerApp(basename string) *app.App {
	opts := options.NewOptions()
	return app.NewApp(workerCommandDesc,
		basename,
		app.WithOptions(opts),
		app.WithDescription(workerCommandDesc),
		app.WithDefaultValidArgs(),
		app.WithRunFunc(runWorker(opts)),
	)
}

func runWorker(opts *options.Options) app.RunFunc {
	return func(basename string) error {
		if len(os.Getenv("GOMAXPROCS")) == 0 {
			runtime.GOMAXPROCS(runtime.NumCPU())
		}
		log.Init(opts.Log)
		defer log.Flush()

		cfg, err := config.CreateConfigFromOptions(opts)
		if err != nil {
			return err
		}
		if err := BuildWorker(cfg); err != nil {
			return err
		}
		return nil
	}
}

func BuildWorker(cfg *config.Config) error {
	extraConfig, err := buildExtraConfig(cfg)
	if err != nil {
		return err
	}
	if err := extraConfig.Complete().New(); err != nil {
		return err
	}
	processor := worker.NewProcessor()
	ctx := context.Background()
	queue := envOrDefault("OMNIMAM_WORKER_QUEUE", "default")
	pollInterval := 3 * time.Second
	if raw := os.Getenv("OMNIMAM_WORKER_POLL_INTERVAL"); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			pollInterval = parsed
		}
	}
	return processor.Run(ctx, worker.RunOptions{
		Queue:        queue,
		WorkerID:     envOrDefault("OMNIMAM_WORKER_ID", "worker-"+time.Now().Format("20060102150405")),
		Limit:        4,
		Lease:        5 * time.Minute,
		PollInterval: pollInterval,
	})
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
