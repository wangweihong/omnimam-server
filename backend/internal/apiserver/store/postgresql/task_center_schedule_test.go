package postgresql

import (
	"strings"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func TestScheduleSummaryDoesNotRegressWhenLightweightHistoryIsPruned(t *testing.T) {
	schedule := &iapiserver.TaskSchedule{}
	applyScheduleSummaryTransition(schedule, "", iapiserver.ScheduleExecutionStatusTriggered)
	applyScheduleSummaryTransition(schedule, iapiserver.ScheduleExecutionStatusTriggered, iapiserver.ScheduleExecutionStatusSuccess)
	applyScheduleSummaryTransition(schedule, "", iapiserver.ScheduleExecutionStatusSkippedOverlap)
	if schedule.Summary.TotalTriggered != 2 || schedule.Summary.Running != 0 || schedule.Summary.Success != 1 || schedule.Summary.SkippedOverlap != 1 {
		t.Fatalf("summary = %#v", schedule.Summary)
	}
}

func TestTaskCenterOwnerChildIndexAllowsRecurringScheduleTargets(t *testing.T) {
	if !strings.Contains(taskCenterApplicationRunIndexesSQL, "owner_type IN ('TASK_GROUP','DAG_TASK_GROUP')") {
		t.Fatal("owner child uniqueness must only cover group and DAG children")
	}
	if !strings.Contains(taskCenterApplicationRunIndexesSQL, "CREATE UNIQUE INDEX IF NOT EXISTS idx_atomic_tasks_owner_child") {
		t.Fatal("owner child index is not created idempotently")
	}
	if strings.Contains(taskCenterApplicationRunIndexesSQL, "DROP INDEX") {
		t.Fatal("owner child index setup must not contain historical replacement logic")
	}
}
