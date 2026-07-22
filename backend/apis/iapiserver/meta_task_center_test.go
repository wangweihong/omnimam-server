package iapiserver

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestTaskCenterReconcileCompositeIndexesMatchSSOT(t *testing.T) {
	tests := []struct {
		name       string
		model      any
		index      string
		wantFields []string
	}{
		{name: "schedule mode status", model: &TaskSchedule{}, index: "idx_task_schedules_mode_status", wantFields: []string{"execution_mode", "status", "next_trigger_at"}},
		{name: "checkpoint age", model: &ScheduleReconcileState{}, index: "idx_schedule_reconcile_states_checkpoint", wantFields: []string{"last_completed_at", "updated_at"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := schema.Parse(tt.model, &sync.Map{}, schema.NamingStrategy{})
			if err != nil {
				t.Fatal(err)
			}
			index, ok := parsed.ParseIndexes()[tt.index]
			if !ok {
				t.Fatalf("index %s is missing", tt.index)
			}
			if len(index.Fields) != len(tt.wantFields) {
				t.Fatalf("index fields = %d, want %d", len(index.Fields), len(tt.wantFields))
			}
			for i, field := range index.Fields {
				if field.DBName != tt.wantFields[i] {
					t.Fatalf("index field %d = %s, want %s", i, field.DBName, tt.wantFields[i])
				}
			}
		})
	}
}

func TestTaskAttemptLogsRefIsStableAcrossPersistenceHooks(t *testing.T) {
	attempt := &TaskAttempt{}
	attempt.ID = "attempt-1"
	if err := attempt.BeforeCreate(nil); err != nil {
		t.Fatal(err)
	}
	if attempt.LogsRef != "task-attempt-log:attempt-1" {
		t.Fatalf("logs_ref = %q", attempt.LogsRef)
	}

	attempt.LogsRef = ""
	if err := attempt.AfterFind(nil); err != nil {
		t.Fatal(err)
	}
	if attempt.LogsRef != "task-attempt-log:attempt-1" {
		t.Fatalf("synthesized logs_ref = %q", attempt.LogsRef)
	}
	if got := TaskAttemptLogsRef(""); got != "" {
		t.Fatalf("empty attempt logs_ref = %q", got)
	}
}

func TestTaskCenterV172PersistenceFieldsUseReleasedColumns(t *testing.T) {
	taskSchema, err := schema.Parse(&AtomicTask{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"dag_node_key"} {
		if taskSchema.LookUpField(column) == nil {
			t.Fatalf("AtomicTask column %s is missing", column)
		}
	}
	attemptSchema, err := schema.Parse(&TaskAttempt{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"executor_type", "executor_display_name"} {
		if attemptSchema.LookUpField(column) == nil {
			t.Fatalf("TaskAttempt column %s is missing", column)
		}
	}
	dagSchema, err := schema.Parse(&DAGTaskGroup{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"started_at", "completed_at", "trigger_type", "trigger_source_id", "trigger_source_name", "triggered_at"} {
		if dagSchema.LookUpField(column) == nil {
			t.Fatalf("DAGTaskGroup column %s is missing", column)
		}
	}
}
