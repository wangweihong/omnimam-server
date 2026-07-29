package notificationworker

import (
	"context"
	"strings"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func TestRegistryRejectsDuplicateRegistration(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterSource("task-center", "atomic_task_status_changed", AtomicTaskSourceAdapter{}); err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterSource("task-center", "atomic_task_status_changed", AtomicTaskSourceAdapter{}); err == nil {
		t.Fatal("duplicate source registration succeeded")
	}
}

func TestAtomicTaskAdapterAndRuleBoundaries(t *testing.T) {
	registry, err := BuildRegistry()
	if err != nil {
		t.Fatal(err)
	}
	adapter, _ := registry.Source("task-center", "atomic_task_status_changed")
	tests := []struct {
		name, status, ownerType, applicationRunID, errorMessage string
		retryable, wantCandidate, wantNotify                    bool
		wantTopic                                               string
	}{
		{name: "standalone failed", status: "FAILED", errorMessage: "render failed", wantCandidate: true, wantNotify: true, wantTopic: "task.atomic_task.failed"},
		{name: "timeout", status: "TIMEOUT", wantCandidate: true, wantNotify: true, wantTopic: "task.atomic_task.timed_out"},
		{name: "success ignored", status: "SUCCESS", wantCandidate: true, wantTopic: "task.atomic_task.succeeded"},
		{name: "group child ignored", status: "FAILED", ownerType: "TASK_GROUP", wantCandidate: true, wantTopic: "task.atomic_task.failed"},
		{name: "application child ignored", status: "FAILED", applicationRunID: "run-1", wantCandidate: true, wantTopic: "task.atomic_task.failed"},
		{name: "retryable blocked ignored", status: "BLOCKED", errorMessage: "waiting", retryable: true, wantCandidate: true, wantTopic: "task.atomic_task.action_required"},
		{name: "action required", status: "BLOCKED", errorMessage: "missing input", wantCandidate: true, wantNotify: true, wantTopic: "task.atomic_task.action_required"},
		{name: "running omitted", status: "RUNNING"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := `{"source_domain":"task-center","source_event_id":"event-1","atomic_task_id":"task-1","owner_type":` + nullableJSON(test.ownerType) + `,"owner_id":null,"application_run_id":` + nullableJSON(test.applicationRunID) + `,"canvas_run_id":null,"to_status":"` + test.status + `","resource_version":2,"project_id":"project","namespace":"default","created_by":"user-1","occurred_at":"2026-07-29T01:00:00Z","last_error":{"code":"ERR_X","message":"` + test.errorMessage + `","retryable":` + boolJSON(test.retryable) + `},"authorization":"secret","private_url":"http://127.0.0.1"}`
			items, err := adapter.Normalize(context.Background(), []byte(raw))
			if err != nil {
				t.Fatal(err)
			}
			if (len(items) == 1) != test.wantCandidate {
				t.Fatalf("candidates=%d", len(items))
			}
			if len(items) == 0 {
				return
			}
			item := items[0]
			if item.NotificationTopic != test.wantTopic {
				t.Fatalf("topic=%q", item.NotificationTopic)
			}
			if strings.Contains(item.PayloadSnapshotShadow, "secret") || strings.Contains(item.PayloadSnapshotShadow, "127.0.0.1") {
				t.Fatalf("unsafe snapshot=%s", item.PayloadSnapshotShadow)
			}
			rule, ok := registry.Rule(item.NotificationTopic)
			if !ok {
				t.Fatalf("rule not registered")
			}
			decision, err := rule.Evaluate(context.Background(), item)
			if err != nil {
				t.Fatal(err)
			}
			if decision.Notify != test.wantNotify {
				t.Fatalf("notify=%v", decision.Notify)
			}
			if decision.Notify && (!strings.Contains(decision.Title, "任务") || decision.ActionPath == nil || !strings.HasPrefix(*decision.ActionPath, "/task-center/atomic/")) {
				t.Fatalf("decision=%+v", decision)
			}
		})
	}
}

func TestCanvasAdapterMapsReleasedTerminalStatuses(t *testing.T) {
	adapter := CanvasRunSourceAdapter{}
	tests := map[string]string{"SUCCESS": "canvas.run.succeeded", "PARTIAL_SUCCESS": "canvas.run.partially_succeeded", "FAILED": "canvas.run.failed", "TIMEOUT": "canvas.run.failed", "CANCELED": ""}
	for status, want := range tests {
		t.Run(status, func(t *testing.T) {
			raw := `{"source_domain":"workflow-canvas","source_event_id":"event-1","canvas_run_id":"run-1","canvas_id":"canvas-1","status":"` + status + `","aggregate_version":3,"project_id":"project","namespace":"default","created_by":"user-1","occurred_at":"2026-07-29T01:00:00Z","last_error":{"message":"canvas failed"},"warnings":[{"code":"WARN_NODE"}]}`
			items, err := adapter.Normalize(context.Background(), []byte(raw))
			if err != nil {
				t.Fatal(err)
			}
			if want == "" {
				if len(items) != 0 {
					t.Fatalf("items=%+v", items)
				}
				return
			}
			if len(items) != 1 || items[0].NotificationTopic != want || items[0].PayloadSnapshot.CanvasID != "canvas-1" {
				t.Fatalf("items=%+v", items)
			}
			registry, _ := BuildRegistry()
			rule, _ := registry.Rule(want)
			decision, err := rule.Evaluate(context.Background(), items[0])
			if err != nil {
				t.Fatal(err)
			}
			if !decision.Notify || decision.ActionPath == nil || *decision.ActionPath != "/canvases/canvas-1?canvas_run_id=run-1" || !strings.Contains(decision.Title, "画布") {
				t.Fatalf("decision=%+v", decision)
			}
		})
	}
}

func TestSourceAdaptersPersistUnresolvedRecipientCandidates(t *testing.T) {
	registry, err := BuildRegistry()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		adapter SourceAdapter
		raw     string
	}{
		{
			name:    "atomic task",
			adapter: AtomicTaskSourceAdapter{},
			raw:     `{"source_domain":"task-center","source_event_id":"event-atomic","atomic_task_id":"task-1","to_status":"FAILED","resource_version":2,"occurred_at":"2026-07-29T01:00:00Z","last_error":{"message":"failed"}}`,
		},
		{
			name:    "canvas run",
			adapter: CanvasRunSourceAdapter{},
			raw:     `{"source_domain":"workflow-canvas","source_event_id":"event-canvas","canvas_run_id":"run-1","canvas_id":"canvas-1","status":"FAILED","aggregate_version":3,"occurred_at":"2026-07-29T01:00:00Z","last_error":{"message":"failed"}}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items, err := test.adapter.Normalize(context.Background(), []byte(test.raw))
			if err != nil {
				t.Fatal(err)
			}
			if len(items) != 1 {
				t.Fatalf("candidates=%d, want 1", len(items))
			}
			rule, ok := registry.Rule(items[0].NotificationTopic)
			if !ok {
				t.Fatalf("rule %q is not registered", items[0].NotificationTopic)
			}
			if _, err := rule.Evaluate(context.Background(), items[0]); err == nil {
				t.Fatal("candidate without recipient basis was accepted by the rule")
			}
		})
	}
}

func nullableJSON(value string) string {
	if value == "" {
		return "null"
	}
	return `"` + value + `"`
}
func boolJSON(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

var _ SourceAdapter = AtomicTaskSourceAdapter{}
var _ SourceAdapter = CanvasRunSourceAdapter{}
var _ RuleEvaluator = PresetRule{}
var _ RecipientResolver = CreatedByRecipientResolver{}
var _ TemplateRenderer = SimplifiedChineseRenderer{}
var _ NavigationResolver = AtomicTaskNavigationResolver{}
var _ = iapiserver.NotificationTopicActive
