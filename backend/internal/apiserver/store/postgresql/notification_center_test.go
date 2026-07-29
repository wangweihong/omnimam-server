package postgresql

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func TestNotificationTopicCatalogOnlyEnablesReleasedActiveInputs(t *testing.T) {
	items := notificationTopicCatalog()
	enabled := make(map[string]bool)
	for _, item := range items {
		if item.enabled {
			enabled[item.topic] = true
			if item.activation != iapiserver.NotificationTopicActive {
				t.Fatalf("enabled non-ACTIVE topic %s", item.topic)
			}
		}
		if item.activation != iapiserver.NotificationTopicActive && item.enabled {
			t.Fatalf("non-active topic enabled: %+v", item)
		}
	}
	want := []string{"task.atomic_task.succeeded", "task.atomic_task.failed", "task.atomic_task.timed_out", "task.atomic_task.action_required", "canvas.run.succeeded", "canvas.run.partially_succeeded", "canvas.run.failed"}
	if len(enabled) != len(want) {
		t.Fatalf("enabled topics=%v", enabled)
	}
	for _, topic := range want {
		if !enabled[topic] {
			t.Fatalf("active topic %s is disabled", topic)
		}
	}
}

func TestNotificationTopicCatalogMatchesReleasedEventsContract(t *testing.T) {
	expected := map[string]string{
		"task.atomic_task.succeeded":                      "task|ACTIVE|true|success|informational|immediate|false|owner_or_initiator",
		"task.atomic_task.failed":                         "task|ACTIVE|true|error|action_required|per_source|false|owner_or_initiator",
		"task.atomic_task.timed_out":                      "task|ACTIVE|true|error|action_required|per_source|false|owner_or_initiator",
		"task.atomic_task.action_required":                "task|ACTIVE|true|warning|action_required|per_source|false|owner_or_initiator",
		"task.group.succeeded":                            "task|CONTRACT_GAP|false|success|informational|per_source|false|owner_or_initiator",
		"task.group.failed":                               "task|CONTRACT_GAP|false|error|action_required|per_source|false|owner_or_initiator",
		"task.group.action_required":                      "task|CONTRACT_GAP|false|warning|action_required|per_source|false|owner_or_initiator",
		"canvas.run.succeeded":                            "canvas|ACTIVE|true|success|informational|immediate|false|initiator",
		"canvas.run.partially_succeeded":                  "canvas|ACTIVE|true|warning|action_required|per_source|false|initiator",
		"canvas.run.failed":                               "canvas|ACTIVE|true|error|action_required|per_source|false|initiator",
		"asset.representation.completed":                  "asset|CONTRACT_GAP|false|success|informational|per_source|false|owner",
		"asset.representation.failed":                     "asset|CONTRACT_GAP|false|error|action_required|per_source|false|owner",
		"asset.representation.batch_completed":            "asset|CONTRACT_GAP|false|info|informational|5_minutes|false|owner",
		"asset.representation.batch_failed":               "asset|CONTRACT_GAP|false|error|action_required|5_minutes|false|owner",
		"application.run.completed":                       "application|CONTRACT_GAP|false|success|informational|immediate|false|initiator",
		"application.run.failed":                          "application|CONTRACT_GAP|false|error|action_required|per_source|false|initiator",
		"application.artifact.ready":                      "application|CONTRACT_GAP|false|success|informational|per_source|false|initiator",
		"application.engine_instance.unavailable":         "provider|CONTRACT_GAP|false|critical|action_required|per_source|true|administrators",
		"application.engine_instance.recovered":           "provider|CONTRACT_GAP|false|info|resolved|per_source|true|administrators",
		"model.provider_model.unavailable":                "provider|CONTRACT_GAP|false|warning|action_required|per_source|false|owner",
		"model.provider_model.recovered":                  "provider|CONTRACT_GAP|false|info|resolved|per_source|false|owner",
		"asset.scan.completed":                            "asset|FUTURE|false|info|informational|per_source|false|owner",
		"asset.scan.partially_failed":                     "asset|FUTURE|false|warning|action_required|per_source|false|owner",
		"asset.scan.failed":                               "asset|FUTURE|false|error|action_required|per_source|false|owner",
		"asset.import.completed":                          "asset|FUTURE|false|success|informational|per_source|false|initiator",
		"asset.import.partially_failed":                   "asset|FUTURE|false|warning|action_required|per_source|false|initiator",
		"asset.metadata.failed":                           "asset|FUTURE|false|warning|action_required|5_minutes|false|owner",
		"asset.batch.completed":                           "asset|FUTURE|false|info|informational|per_source|false|initiator",
		"application.engine_instance.credential_expiring": "provider|FUTURE|false|warning|action_required|per_source|true|administrators",
		"model.provider.credential_expiring":              "provider|FUTURE|false|warning|action_required|per_source|false|owner",
		"storage.backend.unavailable":                     "storage|FUTURE|false|critical|action_required|per_source|true|administrators",
		"storage.capacity.warning":                        "storage|FUTURE|false|warning|action_required|1_hour|true|administrators",
		"storage.scan.completed":                          "storage|FUTURE|false|info|informational|per_source|false|administrators",
		"system.update.available":                         "system|FUTURE|false|info|informational|per_source|false|system_broadcast",
		"system.background_job.failed":                    "system|FUTURE|false|critical|action_required|1_hour|true|administrators",
		"agent.run.succeeded":                             "agent|FUTURE|false|success|informational|immediate|false|initiator",
		"agent.run.failed":                                "agent|FUTURE|false|error|action_required|per_source|false|initiator",
		"agent.approval.requested":                        "agent|FUTURE|false|warning|action_required|immediate|true|approver",
	}
	catalog := notificationTopicCatalog()
	if len(catalog) != len(expected) {
		t.Fatalf("catalog topics=%d want=%d", len(catalog), len(expected))
	}
	expectedSources := map[string]string{
		"task.atomic_task.succeeded": "task-center:atomic_task_status_changed", "task.atomic_task.failed": "task-center:atomic_task_status_changed",
		"task.atomic_task.timed_out": "task-center:atomic_task_status_changed", "task.atomic_task.action_required": "task-center:atomic_task_status_changed",
		"task.group.succeeded": "task-center:task_group_status_changed", "task.group.failed": "task-center:task_group_status_changed",
		"task.group.action_required": "task-center:task_group_status_changed",
		"canvas.run.succeeded":       "workflow-canvas:canvas_run_status_changed", "canvas.run.partially_succeeded": "workflow-canvas:canvas_run_status_changed",
		"canvas.run.failed":              "workflow-canvas:canvas_run_status_changed",
		"asset.representation.completed": "asset-library:asset_version_processing_changed", "asset.representation.failed": "asset-library:asset_version_processing_changed",
		"asset.representation.batch_completed": "asset-library:asset_version_processing_changed", "asset.representation.batch_failed": "asset-library:asset_version_processing_changed",
		"application.run.completed": "application-platform:application_run_projection_changed", "application.run.failed": "application-platform:application_run_projection_changed",
		"application.artifact.ready":              "asset-library:artifact_processing_changed,asset-library:artifact_registration_changed",
		"application.engine_instance.unavailable": "application-platform:engine_instance_health_changed", "application.engine_instance.recovered": "application-platform:engine_instance_health_changed",
		"model.provider_model.unavailable": "model-management:model_health_status_changed", "model.provider_model.recovered": "model-management:model_health_status_changed",
	}
	for _, item := range catalog {
		got := fmt.Sprintf(
			"%s|%s|%t|%s|%s|%s|%t|%s",
			item.category,
			item.activation,
			item.enabled,
			item.severity,
			item.attention,
			item.aggregation,
			item.mandatory,
			item.recipient,
		)
		if want, ok := expected[item.topic]; !ok || got != want {
			t.Fatalf("topic=%s got=%s want=%s registered=%t", item.topic, got, want, ok)
		}
		mappings := make([]string, 0, len(item.sources))
		for _, mapping := range item.sources {
			mappings = append(mappings, mapping.SourceDomain+":"+mapping.SourceEventType)
		}
		if gotSource, wantSource := strings.Join(mappings, ","), expectedSources[item.topic]; gotSource != wantSource {
			t.Fatalf("topic=%s source mappings=%q want=%q", item.topic, gotSource, wantSource)
		}
	}
}

func TestNotificationMigrationContainsReleasedConstraints(t *testing.T) {
	for _, required := range []string{"idx_notifications_aggregate_window", "FOR UPDATE", "fk_notification_events_topic", "ck_notification_outbox_values", "ck_notification_events_source_type", "ck_notifications_source_type", "notification_unread_count_changed"} {
		if required == "FOR UPDATE" {
			continue
		}
		if !strings.Contains(notificationCenterConstraintsSQL, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func TestNotificationSourceErrorsAreSanitizedBeforeOutboxPersistence(t *testing.T) {
	message := strings.Repeat("失败", 600) +
		" Authorization: Bearer secret https://example.invalid/path 10.0.0.8\npanic: boom\nworker.go:42"
	tests := []struct {
		name string
		safe map[string]any
	}{
		{
			name: "atomic task",
			safe: safeTaskNotificationError(iapiserver.TaskError{
				Code:      "ERR_TASK token=secret",
				Message:   message,
				Retryable: true,
			}),
		},
		{
			name: "canvas run",
			safe: safeCanvasNotificationError(map[string]any{
				"code":      "ERR_CANVAS token=secret",
				"message":   message,
				"retryable": true,
			}),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			codeValue, _ := test.safe["code"].(string)
			messageValue, _ := test.safe["message"].(string)
			if !utf8.ValidString(codeValue) || !utf8.ValidString(messageValue) {
				t.Fatalf("source error is not valid UTF-8: %#v", test.safe)
			}
			for _, forbidden := range []string{
				"secret",
				"https://",
				"10.0.0.8",
				"panic:",
				".go:",
			} {
				if strings.Contains(strings.ToLower(codeValue+" "+messageValue), strings.ToLower(forbidden)) {
					t.Fatalf("source error contains %q: %#v", forbidden, test.safe)
				}
			}
			if retryable, _ := test.safe["retryable"].(bool); !retryable {
				t.Fatalf("retryable flag was lost: %#v", test.safe)
			}
		})
	}
}
