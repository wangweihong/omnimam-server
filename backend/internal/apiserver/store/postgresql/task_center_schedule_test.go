package postgresql

import (
	"strings"
	"testing"
)

func TestTaskCenterScheduleOwnershipBackfillIsIdempotentAndScoped(t *testing.T) {
	markers := []string{
		"execution.target_id <> ''",
		"owner_type = 'TASK_SCHEDULE'",
		"owner_id = schedule.id",
		"FROM task_groups AS parent",
		"FROM dag_task_groups AS parent",
		"IS DISTINCT FROM",
	}
	for _, marker := range markers {
		if !strings.Contains(taskCenterScheduleOwnershipBackfillSQL, marker) {
			t.Fatalf("schedule ownership backfill missing %q", marker)
		}
	}
}

func TestTaskCenterOwnerChildIndexAllowsRecurringScheduleTargets(t *testing.T) {
	if !strings.Contains(taskCenterApplicationRunIndexesSQL, "owner_type IN ('TASK_GROUP','DAG_TASK_GROUP')") {
		t.Fatal("owner child uniqueness must only cover group and DAG children")
	}
	if !strings.Contains(taskCenterApplicationRunIndexesSQL, "DROP INDEX IF EXISTS idx_atomic_tasks_owner_child") {
		t.Fatal("legacy owner child index is not replaced")
	}
}
