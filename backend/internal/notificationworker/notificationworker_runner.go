package notificationworker

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/internal/apiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/config"
	ssesvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/sse"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store/postgresql"
)

const (
	notificationRetentionInterval = time.Hour
	notificationRetentionBatch    = 500
)

// RunNotificationWorker 启动独立通知源消费、规则处理、保留清理和统一 SSE UserEvent 投影。
func RunNotificationWorker(cfg *config.Config) error {
	if err := apiserver.InitializeStore(cfg); err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	return runNotificationWorker(
		ctx,
		store.Client(),
		cfg.SSEOptions.Retention,
		postgresql.SubscribeOutbox,
	)
}

func runNotificationWorker(
	ctx context.Context,
	storeIns store.Factory,
	userEventRetention time.Duration,
	subscribe SubscribeFunc,
) error {
	if storeIns == nil {
		return fmt.Errorf("notification worker store is not configured")
	}
	candidateFactory, ok := storeIns.(store.NotificationCandidateStoreFactory)
	if !ok || candidateFactory.NotificationCandidates() == nil {
		return fmt.Errorf("notification candidate store is not configured")
	}
	outboxFactory, ok := storeIns.(store.NotificationOutboxStoreFactory)
	if !ok || outboxFactory.NotificationOutbox() == nil {
		return fmt.Errorf("notification outbox store is not configured")
	}
	retentionFactory, ok := storeIns.(store.NotificationRetentionStoreFactory)
	if !ok || retentionFactory.NotificationRetention() == nil {
		return fmt.Errorf("notification retention store is not configured")
	}
	if storeIns.UserEvents() == nil {
		return fmt.Errorf("notification user event store is not configured")
	}
	if subscribe == nil {
		return fmt.Errorf("notification worker subscriber is not configured")
	}

	registry, err := BuildRegistry()
	if err != nil {
		return errors.Wrap(err, "build notification worker registry")
	}
	projector := ssesvc.NewNotificationProjector(
		storeIns.UserEvents(),
		outboxFactory.NotificationOutbox(),
		userEventRetention,
		ssesvc.SubscribeFunc(subscribe),
	)
	if err := projector.Start(ctx); err != nil {
		return errors.Wrap(err, "start notification SSE projector")
	}
	defer projector.Close()

	ruleWorker := NewRuleWorker(
		candidateFactory.NotificationCandidates(),
		registry,
		DefaultRuleWorkerConfig(),
	)
	if err := ruleWorker.Start(ctx); err != nil {
		return errors.Wrap(err, "start notification rule worker")
	}
	defer ruleWorker.Close()

	sourceWorker := NewSourceWorker(
		candidateFactory.NotificationCandidates(),
		registry,
		subscribe,
	)
	if err := sourceWorker.Start(ctx); err != nil {
		return errors.Wrap(err, "start notification source worker")
	}
	defer sourceWorker.Close()

	retention := NewRetentionRunner(
		retentionFactory.NotificationRetention(),
		notificationRetentionInterval,
		notificationRetentionBatch,
	)
	if err := retention.Start(ctx); err != nil {
		return errors.Wrap(err, "start notification retention runner")
	}
	defer retention.Close()

	<-ctx.Done()
	return nil
}
