package notificationworker

import (
	"context"
	"fmt"
	"sync"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

// SourceAdapter 将一个可靠源事件转换为零或多个最小 NotificationEvent 候选。
type SourceAdapter interface {
	Normalize(context.Context, []byte) ([]*iapiserver.NotificationEvent, error)
}

// RuleDecision 是规则处理后的站内通知事实草案；Notify=false 表示候选应标记 IGNORED。
type RuleDecision struct {
	Notify            bool
	IgnoredReason     string
	RecipientUserID   string
	Title             string
	Content           string
	Severity          string
	AttentionStatus   string
	NavigationTarget  *iapiserver.NotificationNavigationTarget
	ActionPath        *string
	AggregateKey      string
	AggregationWindow string
}

// RuleEvaluator 对候选执行价值判断和 topic 规则，不读取源领域私表。
type RuleEvaluator interface {
	Evaluate(context.Context, *iapiserver.NotificationEvent) (*RuleDecision, error)
}

// RecipientResolver 从候选的受控依据中确定一个站内接收者。
type RecipientResolver interface {
	Resolve(context.Context, *iapiserver.NotificationEvent) (string, error)
}

// TemplateRenderer 将候选渲染成首期固定简体中文标题和正文。
type TemplateRenderer interface {
	Render(context.Context, *iapiserver.NotificationEvent) (string, string, error)
}

// NavigationResolver 从结构化同源目标派生白名单 action path。
type NavigationResolver interface {
	Resolve(context.Context, *iapiserver.NotificationEvent) (*iapiserver.NotificationNavigationTarget, *string, error)
}

type Registry struct {
	mu          sync.RWMutex
	sources     map[string]SourceAdapter
	rules       map[string]RuleEvaluator
	recipients  map[string]RecipientResolver
	templates   map[string]TemplateRenderer
	navigations map[string]NavigationResolver
}

func NewRegistry() *Registry {
	return &Registry{sources: map[string]SourceAdapter{}, rules: map[string]RuleEvaluator{}, recipients: map[string]RecipientResolver{}, templates: map[string]TemplateRenderer{}, navigations: map[string]NavigationResolver{}}
}

func sourceKey(domain, event string) string { return domain + "\x00" + event }

func register[T any](values map[string]T, key string, value T) error {
	if key == "" {
		return fmt.Errorf("notification registry key is empty")
	}
	if _, exists := values[key]; exists {
		return fmt.Errorf("notification registry key %q is duplicated", key)
	}
	values[key] = value
	return nil
}

func (r *Registry) RegisterSource(domain, event string, adapter SourceAdapter) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return register(r.sources, sourceKey(domain, event), adapter)
}
func (r *Registry) RegisterRule(topic string, evaluator RuleEvaluator) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return register(r.rules, topic, evaluator)
}
func (r *Registry) RegisterRecipient(topic string, resolver RecipientResolver) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return register(r.recipients, topic, resolver)
}
func (r *Registry) RegisterTemplate(topic string, renderer TemplateRenderer) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return register(r.templates, topic, renderer)
}
func (r *Registry) RegisterNavigation(sourceType string, resolver NavigationResolver) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return register(r.navigations, sourceType, resolver)
}

func (r *Registry) Source(domain, event string) (SourceAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, ok := r.sources[sourceKey(domain, event)]
	return value, ok
}
func (r *Registry) Rule(topic string) (RuleEvaluator, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, ok := r.rules[topic]
	return value, ok
}
func (r *Registry) Recipient(topic string) (RecipientResolver, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, ok := r.recipients[topic]
	return value, ok
}
func (r *Registry) Template(topic string) (TemplateRenderer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, ok := r.templates[topic]
	return value, ok
}
func (r *Registry) Navigation(sourceType string) (NavigationResolver, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, ok := r.navigations[sourceType]
	return value, ok
}
