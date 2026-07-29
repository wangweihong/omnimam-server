package apiserver

import (
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/internal/apiserver/config"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/options"
	"github.com/wangweihong/omnimam/backend/pkg/app"
)

// NewNotificationWorkerApp 创建独立 Notification Center Worker 命令。
func NewNotificationWorkerApp(basename string) *app.App {
	opts := options.NewOptions()
	opts.Name = "notification-worker"
	return app.NewApp(
		"notification-worker",
		basename,
		app.WithOptions(opts),
		app.WithDescription("notification-worker"),
		app.WithDefaultValidArgs(),
		app.WithRunFunc(func(string) error {
			log.Init(opts.Log)
			defer log.Flush()
			cfg, err := config.CreateConfigFromOptions(opts)
			if err != nil {
				return err
			}
			return RunNotificationWorker(cfg)
		}),
	)
}
