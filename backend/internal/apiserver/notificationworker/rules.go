package notificationworker

import (
	"context"
	"fmt"
	"net/url"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type PresetRule struct {
	recipients RecipientResolver
	templates  TemplateRenderer
	navigation NavigationResolver
}

func (r PresetRule) Evaluate(ctx context.Context, event *iapiserver.NotificationEvent) (*RuleDecision, error) {
	if event.SourceType == "atomic_task" {
		p := event.PayloadSnapshot
		if p.OwnerType != "" || p.OwnerID != "" || p.ApplicationRunID != "" || p.CanvasRunID != "" {
			return &RuleDecision{Notify: false, IgnoredReason: "owned by a higher-level business fact"}, nil
		}
		if p.Status == iapiserver.AtomicTaskStatusSuccess {
			return &RuleDecision{Notify: false, IgnoredReason: "ordinary standalone success has no confirmed user-visible result"}, nil
		}
		if p.Status == iapiserver.AtomicTaskStatusBlocked && (p.ErrorSummary == "" || p.Retryable) {
			return &RuleDecision{Notify: false, IgnoredReason: "blocked task remains retryable or lacks a stable action reason"}, nil
		}
	}
	recipient, err := r.recipients.Resolve(ctx, event)
	if err != nil {
		return nil, err
	}
	title, content, err := r.templates.Render(ctx, event)
	if err != nil {
		return nil, err
	}
	target, path, err := r.navigation.Resolve(ctx, event)
	if err != nil {
		return nil, err
	}
	severity, attention, aggregation := topicPresentation(event.NotificationTopic)
	return &RuleDecision{Notify: true, RecipientUserID: recipient, Title: title, Content: content, Severity: severity, AttentionStatus: attention, NavigationTarget: target, ActionPath: path, AggregateKey: event.SourceID, AggregationWindow: aggregation}, nil
}

type CreatedByRecipientResolver struct{}

func (CreatedByRecipientResolver) Resolve(_ context.Context, event *iapiserver.NotificationEvent) (string, error) {
	if event.RecipientBasis.CreatedBy == "" {
		return "", fmt.Errorf("notification recipient is unresolved")
	}
	return event.RecipientBasis.CreatedBy, nil
}

type SimplifiedChineseRenderer struct{}

func (SimplifiedChineseRenderer) Render(_ context.Context, event *iapiserver.NotificationEvent) (string, string, error) {
	summary := event.PayloadSnapshot.ErrorSummary
	switch event.NotificationTopic {
	case "task.atomic_task.failed":
		if summary != "" {
			return "任务执行失败", "任务执行失败：" + summary, nil
		}
		return "任务执行失败", "任务执行失败，请查看任务详情。", nil
	case "task.atomic_task.timed_out":
		return "任务执行超时", "任务未在限定时间内完成，请查看任务详情。", nil
	case "task.atomic_task.action_required":
		if summary != "" {
			return "任务需要处理", "任务无法自动恢复：" + summary, nil
		}
		return "任务需要处理", "任务无法自动恢复，请检查任务详情。", nil
	case "canvas.run.succeeded":
		return "画布运行完成", "画布已成功完成运行。", nil
	case "canvas.run.partially_succeeded":
		return "画布运行部分完成", "画布运行已完成，但部分节点存在警告或失败。", nil
	case "canvas.run.failed":
		if summary != "" {
			return "画布运行失败", "画布运行失败：" + summary, nil
		}
		return "画布运行失败", "画布运行失败，请查看运行详情。", nil
	default:
		return "", "", fmt.Errorf("notification template for topic %s is not registered", event.NotificationTopic)
	}
}

type AtomicTaskNavigationResolver struct{}

func (AtomicTaskNavigationResolver) Resolve(_ context.Context, event *iapiserver.NotificationEvent) (*iapiserver.NotificationNavigationTarget, *string, error) {
	if event.SourceID == "" {
		return nil, nil, nil
	}
	path := "/task-center/atomic/" + url.PathEscape(event.SourceID)
	return &iapiserver.NotificationNavigationTarget{Type: "navigate", TargetType: "atomic_task", TargetID: event.SourceID, View: "detail", Params: map[string]string{}}, &path, nil
}

type CanvasRunNavigationResolver struct{}

func (CanvasRunNavigationResolver) Resolve(_ context.Context, event *iapiserver.NotificationEvent) (*iapiserver.NotificationNavigationTarget, *string, error) {
	canvasID := event.PayloadSnapshot.CanvasID
	if canvasID == "" || event.SourceID == "" {
		return nil, nil, nil
	}
	path := "/canvases/" + url.PathEscape(canvasID) + "?canvas_run_id=" + url.QueryEscape(event.SourceID)
	return &iapiserver.NotificationNavigationTarget{Type: "navigate", TargetType: "canvas_run", TargetID: event.SourceID, View: "detail", Params: map[string]string{"canvas_id": canvasID}}, &path, nil
}

func topicPresentation(topic string) (string, string, string) {
	values := map[string][3]string{
		"task.atomic_task.failed":          {"error", "action_required", "per_source"},
		"task.atomic_task.timed_out":       {"error", "action_required", "per_source"},
		"task.atomic_task.action_required": {"warning", "action_required", "per_source"},
		"canvas.run.succeeded":             {"success", "informational", "immediate"},
		"canvas.run.partially_succeeded":   {"warning", "action_required", "per_source"},
		"canvas.run.failed":                {"error", "action_required", "per_source"},
	}
	value := values[topic]
	return value[0], value[1], value[2]
}

func BuildRegistry() (*Registry, error) {
	registry := NewRegistry()
	if err := registry.RegisterSource(iapiserver.SSESourceDomainTaskCenter, "atomic_task_status_changed", AtomicTaskSourceAdapter{}); err != nil {
		return nil, err
	}
	if err := registry.RegisterSource(iapiserver.SSESourceDomainWorkflowCanvas, "canvas_run_status_changed", CanvasRunSourceAdapter{}); err != nil {
		return nil, err
	}
	recipient, renderer := CreatedByRecipientResolver{}, SimplifiedChineseRenderer{}
	for _, topic := range []string{"task.atomic_task.failed", "task.atomic_task.timed_out", "task.atomic_task.action_required", "task.atomic_task.succeeded"} {
		navigation := AtomicTaskNavigationResolver{}
		rule := PresetRule{recipients: recipient, templates: renderer, navigation: navigation}
		if err := registry.RegisterRule(topic, rule); err != nil {
			return nil, err
		}
		if err := registry.RegisterRecipient(topic, recipient); err != nil {
			return nil, err
		}
		if err := registry.RegisterTemplate(topic, renderer); err != nil {
			return nil, err
		}
	}
	for _, topic := range []string{"canvas.run.succeeded", "canvas.run.partially_succeeded", "canvas.run.failed"} {
		navigation := CanvasRunNavigationResolver{}
		rule := PresetRule{recipients: recipient, templates: renderer, navigation: navigation}
		if err := registry.RegisterRule(topic, rule); err != nil {
			return nil, err
		}
		if err := registry.RegisterRecipient(topic, recipient); err != nil {
			return nil, err
		}
		if err := registry.RegisterTemplate(topic, renderer); err != nil {
			return nil, err
		}
	}
	if err := registry.RegisterNavigation("atomic_task", AtomicTaskNavigationResolver{}); err != nil {
		return nil, err
	}
	if err := registry.RegisterNavigation("canvas_run", CanvasRunNavigationResolver{}); err != nil {
		return nil, err
	}
	return registry, nil
}
