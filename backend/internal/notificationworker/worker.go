package notificationworker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/notificationsanitize"
)

const (
	atomicTaskSourceTopic = "atomic_task_status_changed"
	canvasRunSourceTopic  = "canvas_run_status_changed"

	atomicTaskConsumerGroup = "notification-source-task-center"
	canvasRunConsumerGroup  = "notification-source-workflow-canvas"
)

// SubscribeFunc 创建指定通知 source topic 的可靠订阅。
type SubscribeFunc func(context.Context, string, string) (<-chan *message.Message, error)

// SourceWorker 使用独立 consumer group 将已发布 source event 规范化并持久化为候选。
type SourceWorker struct {
	store     store.NotificationCandidateStore
	registry  *Registry
	subscribe SubscribeFunc
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func NewSourceWorker(candidateStore store.NotificationCandidateStore, registry *Registry, subscribe SubscribeFunc) *SourceWorker {
	return &SourceWorker{store: candidateStore, registry: registry, subscribe: subscribe}
}

// Start 启动 task-center 与 workflow-canvas 两条独立订阅；候选持久化成功后才 ACK。
func (w *SourceWorker) Start(parent context.Context) error {
	if w.store == nil || w.registry == nil || w.subscribe == nil {
		return fmt.Errorf("notification source worker dependencies are not configured")
	}
	ctx, cancel := context.WithCancel(parent)
	w.cancel = cancel
	sources := []struct {
		domain, topic, group string
	}{
		{iapiserver.SSESourceDomainTaskCenter, atomicTaskSourceTopic, atomicTaskConsumerGroup},
		{iapiserver.SSESourceDomainWorkflowCanvas, canvasRunSourceTopic, canvasRunConsumerGroup},
	}
	for _, source := range sources {
		messages, err := w.subscribe(ctx, source.topic, source.group)
		if err != nil {
			cancel()
			w.wg.Wait()
			return fmt.Errorf("subscribe notification source topic %s: %w", source.topic, err)
		}
		w.wg.Add(1)
		go w.consume(ctx, source.domain, source.topic, messages)
	}
	return nil
}

func (w *SourceWorker) consume(ctx context.Context, domain, topic string, messages <-chan *message.Message) {
	defer w.wg.Done()
	adapter, ok := w.registry.Source(domain, topic)
	if !ok {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case msg, open := <-messages:
			if !open {
				return
			}
			candidates, err := adapter.Normalize(ctx, msg.Payload)
			if err == nil && len(candidates) > 0 {
				err = w.store.AddNotificationCandidates(ctx, candidates)
			}
			if err != nil {
				log.Errorf(
					"notification source processing failed: domain=%s topic=%s error=%s",
					domain,
					topic,
					notificationsanitize.Text(err.Error(), 500),
				)
				msg.Nack()
				continue
			}
			msg.Ack()
		}
	}
}

// Close 取消两条订阅并等待消费 goroutine 完整退出。
func (w *SourceWorker) Close() {
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
}

// RuleWorkerConfig 是通知规则轮询的内部固定运行参数，不是公共配置项。
type RuleWorkerConfig struct {
	PollInterval   time.Duration
	BatchSize      int
	ClaimLease     time.Duration
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

func DefaultRuleWorkerConfig() RuleWorkerConfig {
	return RuleWorkerConfig{
		PollInterval: time.Second, BatchSize: 100, ClaimLease: 30 * time.Second,
		MaxAttempts: 10, InitialBackoff: time.Second, MaxBackoff: 5 * time.Minute,
	}
}

// RuleWorker claim 已持久化候选并执行价值判断、偏好门禁与原子收件箱物化。
type RuleWorker struct {
	store    store.NotificationCandidateStore
	registry *Registry
	config   RuleWorkerConfig
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewRuleWorker(candidateStore store.NotificationCandidateStore, registry *Registry, config RuleWorkerConfig) *RuleWorker {
	defaults := DefaultRuleWorkerConfig()
	if config.PollInterval <= 0 {
		config.PollInterval = defaults.PollInterval
	}
	if config.BatchSize <= 0 {
		config.BatchSize = defaults.BatchSize
	}
	if config.ClaimLease <= 0 {
		config.ClaimLease = defaults.ClaimLease
	}
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = defaults.MaxAttempts
	}
	if config.InitialBackoff <= 0 {
		config.InitialBackoff = defaults.InitialBackoff
	}
	if config.MaxBackoff <= 0 {
		config.MaxBackoff = defaults.MaxBackoff
	}
	return &RuleWorker{store: candidateStore, registry: registry, config: config}
}

// Start 启动规则轮询；每个候选完成、忽略、失败或死信状态均在 Store 中可审计。
func (w *RuleWorker) Start(parent context.Context) error {
	if w.store == nil || w.registry == nil {
		return fmt.Errorf("notification rule worker dependencies are not configured")
	}
	ctx, cancel := context.WithCancel(parent)
	w.cancel = cancel
	w.wg.Add(1)
	go w.run(ctx)
	return nil
}

func (w *RuleWorker) run(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()
	for {
		if err := w.poll(ctx, time.Now().UTC()); err != nil && ctx.Err() == nil {
			log.Errorf("notification rule poll failed: error=%s", notificationsanitize.Text(err.Error(), 500))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *RuleWorker) poll(ctx context.Context, now time.Time) error {
	items, err := w.store.ClaimNotificationCandidates(ctx, now, w.config.BatchSize, w.config.ClaimLease)
	if err != nil {
		return err
	}
	processErrors := make([]error, 0)
	for _, item := range items {
		if ctx.Err() != nil {
			processErrors = append(processErrors, ctx.Err())
			break
		}
		if err := w.process(ctx, item, now); err != nil {
			processErrors = append(processErrors, err)
		}
	}
	return errors.Join(processErrors...)
}

func (w *RuleWorker) process(ctx context.Context, item *iapiserver.NotificationEvent, now time.Time) error {
	rule, ok := w.registry.Rule(item.NotificationTopic)
	if !ok {
		return w.finishFailure(ctx, item, now, "ERR_NOTIFICATION_SOURCE_EVENT_UNSUPPORTED", "notification topic rule is not enabled")
	}
	decision, err := rule.Evaluate(ctx, item)
	if err != nil {
		return w.finishFailure(ctx, item, now, "ERR_NOTIFICATION_RECIPIENT_UNRESOLVED", err.Error())
	}
	if !decision.Notify {
		return w.store.FinishNotificationCandidate(ctx, item.ID, item.ProcessingAttemptCount, iapiserver.NotificationEventIgnored, time.Time{}, "", decision.IgnoredReason)
	}
	expiresAt := imachinery.NewTime(now.Add(180 * 24 * time.Hour))
	if decision.Severity == iapiserver.NotificationSeverityError || decision.Severity == iapiserver.NotificationSeverityCritical {
		expiresAt = imachinery.NewTime(now.Add(365 * 24 * time.Hour))
	}
	notification, _, err := w.store.MaterializeNotification(ctx, item, store.NotificationMaterialization{
		RecipientUserID: decision.RecipientUserID, Title: decision.Title, Content: decision.Content,
		Severity: decision.Severity, AttentionStatus: decision.AttentionStatus,
		NavigationTarget: decision.NavigationTarget, ActionPath: decision.ActionPath,
		AggregateKey: decision.AggregateKey, AggregationWindow: decision.AggregationWindow, ExpiresAt: expiresAt,
	})
	if err != nil {
		return w.finishFailure(ctx, item, now, "ERR_NOTIFICATION_RULE_PROCESSING_FAILED", err.Error())
	}
	if notification == nil {
		return w.store.FinishNotificationCandidate(ctx, item.ID, item.ProcessingAttemptCount, iapiserver.NotificationEventIgnored, time.Time{}, "", "in-app preference disabled or below minimum severity")
	}
	return w.store.FinishNotificationCandidate(ctx, item.ID, item.ProcessingAttemptCount, iapiserver.NotificationEventProcessed, time.Time{}, "", "")
}

func (w *RuleWorker) finishFailure(ctx context.Context, item *iapiserver.NotificationEvent, now time.Time, errorCode, summary string) error {
	status := iapiserver.NotificationEventFailed
	nextAttempt := now.Add(w.backoff(item.ProcessingAttemptCount))
	if item.ProcessingAttemptCount >= w.config.MaxAttempts {
		status, nextAttempt = iapiserver.NotificationEventDeadLetter, time.Time{}
	}
	return w.store.FinishNotificationCandidate(
		ctx,
		item.ID,
		item.ProcessingAttemptCount,
		status,
		nextAttempt,
		errorCode,
		notificationsanitize.Text(summary, 500),
	)
}

func (w *RuleWorker) backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := w.config.InitialBackoff
	for n := 1; n < attempt && delay < w.config.MaxBackoff; n++ {
		if delay > w.config.MaxBackoff/2 {
			return w.config.MaxBackoff
		}
		delay *= 2
	}
	if delay > w.config.MaxBackoff {
		return w.config.MaxBackoff
	}
	return delay
}

// Close 停止新 claim 并等待当前轮询退出；未完成候选由租约恢复。
func (w *RuleWorker) Close() {
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
}

// RetentionRunner 每小时有限批次逻辑删除到期通知，并清理已确认投影的 tombstone。
type RetentionRunner struct {
	store    store.NotificationRetentionStore
	interval time.Duration
	batch    int
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewRetentionRunner(retentionStore store.NotificationRetentionStore, interval time.Duration, batch int) *RetentionRunner {
	if interval <= 0 {
		interval = time.Hour
	}
	if batch <= 0 {
		batch = 500
	}
	return &RetentionRunner{store: retentionStore, interval: interval, batch: batch}
}

func (r *RetentionRunner) Start(parent context.Context) error {
	if r.store == nil {
		return fmt.Errorf("notification retention store is not configured")
	}
	ctx, cancel := context.WithCancel(parent)
	r.cancel = cancel
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				if _, err := r.store.CleanupNotifications(ctx, now.UTC(), r.batch); err != nil && ctx.Err() == nil {
					log.Errorf("notification retention failed: error=%s", notificationsanitize.Text(err.Error(), 500))
				}
			}
		}
	}()
	return nil
}

func (r *RetentionRunner) Close() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
}
