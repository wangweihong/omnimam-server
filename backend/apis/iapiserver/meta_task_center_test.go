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
