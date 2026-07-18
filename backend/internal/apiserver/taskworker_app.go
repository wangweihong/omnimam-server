package apiserver

import (
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/internal/apiserver/config"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/options"
	"github.com/wangweihong/omnimam/backend/pkg/app"
)

// NewTaskWorkerApp creates the standalone Task Center worker command.
func NewTaskWorkerApp(basename string) *app.App {
	opts := options.NewOptions()
	opts.Name = "task-worker"
	return app.NewApp("task-worker", basename, app.WithOptions(opts), app.WithDescription("task-worker"), app.WithDefaultValidArgs(), app.WithRunFunc(func(string) error {
		log.Init(opts.Log)
		defer log.Flush()
		cfg, err := config.CreateConfigFromOptions(opts)
		if err != nil {
			return err
		}
		return RunTaskWorker(cfg)
	}))
}
