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

func TestTaskCenterDAGObservabilityConstraintsMatchReleasedContract(t *testing.T) {
	for _, required := range []string{
		"idx_atomic_tasks_dag_node", "idx_dag_groups_status_time", "idx_runtime_projection_execution_time",
		"ck_dag_groups_trigger_type", "'RETRY'", "'CANVAS'", "'SCHEDULE'",
	} {
		if !strings.Contains(taskCenterDAGObservabilityConstraintsSQL, required) {
			t.Fatalf("DAG observability constraints do not contain %q: %s", required, taskCenterDAGObservabilityConstraintsSQL)
		}
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

func TestOwnerStatusTreatsBlockedDescendantsAsTerminalAfterFailure(t *testing.T) {
	tests := []struct {
		name         string
		statuses     []string
		wantStatus   string
		wantTerminal bool
	}{
		{name: "failed predecessor blocks descendant", statuses: []string{iapiserver.AtomicTaskStatusSuccess, iapiserver.AtomicTaskStatusFailed, iapiserver.AtomicTaskStatusBlocked}, wantStatus: iapiserver.TaskGroupStatusFailed, wantTerminal: true},
		{name: "timeout predecessor blocks descendant", statuses: []string{iapiserver.AtomicTaskStatusTimeout, iapiserver.AtomicTaskStatusBlocked}, wantStatus: iapiserver.TaskGroupStatusTimeout, wantTerminal: true},
		{name: "active independent branch remains running", statuses: []string{iapiserver.AtomicTaskStatusFailed, iapiserver.AtomicTaskStatusBlocked, iapiserver.AtomicTaskStatusRunning}, wantStatus: iapiserver.TaskGroupStatusRunning, wantTerminal: false},
		{name: "pending independent branch remains running", statuses: []string{iapiserver.AtomicTaskStatusFailed, iapiserver.AtomicTaskStatusBlocked, iapiserver.AtomicTaskStatusPending}, wantStatus: iapiserver.TaskGroupStatusRunning, wantTerminal: false},
		{name: "blocked without terminal cause remains running", statuses: []string{iapiserver.AtomicTaskStatusSuccess, iapiserver.AtomicTaskStatusBlocked}, wantStatus: iapiserver.TaskGroupStatusRunning, wantTerminal: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := make([]*iapiserver.AtomicTask, 0, len(tt.statuses))
			for _, status := range tt.statuses {
				tasks = append(tasks, &iapiserver.AtomicTask{Status: status})
			}
			status, terminal := ownerStatusAndTerminal(tasks)
			if status != tt.wantStatus || terminal != tt.wantTerminal {
				t.Fatalf("owner status=%s terminal=%t, want %s/%t", status, terminal, tt.wantStatus, tt.wantTerminal)
			}
		})
	}
}
