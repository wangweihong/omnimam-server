package postgresql

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func TestTaskCenterSchemaIndexesMatchV1Contract(t *testing.T) {
	if !strings.Contains(taskCenterActiveScheduleIndexSQL, "idx_schedule_executions_active") || !strings.Contains(taskCenterActiveScheduleIndexSQL, "TRIGGERED") || !strings.Contains(taskCenterActiveScheduleIndexSQL, "RUNNING") {
		t.Fatalf("active schedule SQL is not v1: %s", taskCenterActiveScheduleIndexSQL)
	}
	if strings.Contains(taskCenterActiveScheduleIndexSQL, "lease") {
		t.Fatalf("legacy lease index remains: %s", taskCenterActiveScheduleIndexSQL)
	}
}

func TestAtomicTaskUserEventMappingAndOutputRedaction(t *testing.T) {
	tests := []struct{ status, want string }{
		{iapiserver.AtomicTaskStatusRunning, iapiserver.UserEventAtomicTaskStarted},
		{iapiserver.AtomicTaskStatusRetrying, iapiserver.UserEventAtomicTaskRetrying},
		{iapiserver.AtomicTaskStatusSuccess, iapiserver.UserEventAtomicTaskSucceeded},
		{iapiserver.AtomicTaskStatusTimeout, iapiserver.UserEventAtomicTaskTimedOut},
	}
	for _, test := range tests {
		t.Run(test.status, func(t *testing.T) {
			if got := atomicTaskEventType(test.status); got != test.want {
				t.Fatalf("atomicTaskEventType(%q) = %q, want %q", test.status, got, test.want)
			}
		})
	}
	output := map[string]any{
		"artifact_refs": []any{map[string]any{"artifact_id": "artifact-1"}},
		"result":        map[string]any{"authorization": "secret", "body": strings.Repeat("x", 1024)},
	}
	summary := summarizeTaskOutput(output)
	if _, ok := summary["artifact_refs"]; !ok {
		t.Fatal("artifact_refs were removed from the user-safe output summary")
	}
	if _, ok := summary["result"]; ok {
		t.Fatalf("arbitrary task result leaked into SSE output summary: %+v", summary)
	}
}

func TestUserEventJSONOmitsPersistenceAndRoutingFields(t *testing.T) {
	event := &iapiserver.UserEvent{
		EventSequence: 3, RecipientUserID: "user-1", EventType: iapiserver.UserEventAtomicTaskStarted,
		EventVersion: 1, AggregateType: "atomic_task", AggregateID: "task-1", AggregateVersion: 2,
		SourceDomain: "task-center", SourceEventID: "source-1", Payload: map[string]any{"status": "RUNNING"},
	}
	event.ID = "row-1"
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	text := string(raw)
	for _, forbidden := range []string{"recipient_user_id", "source_domain", "source_event_id", "expires_at", `"id":"row-1"`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("UserEvent JSON %s contains internal field %q", text, forbidden)
		}
	}
}
func TestFingerprintChangesWithAtomicArguments(t *testing.T) {
	base := iapiserver.AtomicTask{FunctionRef: "test.run", Arguments: map[string]any{"value": "one"}}
	other := base
	other.Arguments = map[string]any{"value": "two"}
	if fingerprint(base) == fingerprint(other) {
		t.Fatal("different immutable arguments must produce different fingerprints")
	}
}

func TestDAGTaskGroupFingerprintIgnoresRuntimeState(t *testing.T) {
	base := &iapiserver.DAGTaskGroup{Nodes: []iapiserver.DAGNode{{Key: "plan", Task: iapiserver.AtomicTaskTemplate{FunctionRef: "engine.plan"}}}, IdempotencyScope: "startup", IdempotencyKey: "slot", ProjectID: "default", Namespace: "default", CreatedBy: "system"}
	base.ID = "first"
	other := *base
	other.ID = "second"
	other.Status = iapiserver.TaskGroupStatusRunning
	other.RuntimeExecutionID = "runtime-id"
	if dagTaskGroupFingerprint(base) != dagTaskGroupFingerprint(&other) {
		t.Fatal("runtime state must not change an idempotent request fingerprint")
	}
}
