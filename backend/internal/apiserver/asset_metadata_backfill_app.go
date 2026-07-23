package apiserver

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/internal/apiserver/config"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/options"
	assetlibrarysvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/assetlibrary"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/pkg/app"
)

// NewAssetMetadataBackfillApp 创建只修复历史媒体元数据零值的单用途运维命令。
func NewAssetMetadataBackfillApp(basename string) *app.App {
	opts := options.NewOptions()
	opts.Name = "asset-metadata-backfill"
	return app.NewApp(
		"asset-metadata-backfill",
		basename,
		app.WithOptions(opts),
		app.WithDescription("backfill missing Asset media dimensions and duration"),
		app.WithDefaultValidArgs(),
		app.WithRunFunc(func(string) error {
			log.Init(opts.Log)
			defer log.Flush()
			cfg, err := config.CreateConfigFromOptions(opts)
			if err != nil {
				return err
			}
			return RunAssetMetadataBackfill(cfg)
		}),
	)
}

// RunAssetMetadataBackfill 扫描 current AssetVersion，复用 original 探测逻辑并输出有限汇总。
func RunAssetMetadataBackfill(cfg *config.Config) error {
	if err := InitializeStore(cfg); err != nil {
		return err
	}
	factory := store.Client()
	storage := assetlibrarysvc.NewLocalContentStorage(factory)
	ffprobeInspector, err := assetlibrarysvc.NewLocalFFprobeMediaMetadataInspector()
	if err != nil {
		return errors.Wrap(err, "construct ffprobe metadata inspector")
	}
	inspectors := assetlibrarysvc.NewMediaMetadataInspectors(assetlibrarysvc.ImageMediaMetadataInspector{}, ffprobeInspector)
	backfiller := assetlibrarysvc.NewAssetMediaMetadataBackfiller(factory, storage, inspectors)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	summary, err := backfiller.Run(ctx, 100)
	if err != nil {
		return err
	}
	log.Infof("asset media metadata backfill completed: scanned=%d updated=%d failed=%d", summary.Scanned, summary.Updated, summary.Failed)
	return nil
}
