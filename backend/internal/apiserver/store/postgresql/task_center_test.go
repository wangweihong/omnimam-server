package postgresql

import (
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
