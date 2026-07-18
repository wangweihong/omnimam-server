package apiserver

import (
	"testing"
	"time"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func TestHealthCron(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		want     string
	}{
		{name: "seconds", interval: 30 * time.Second, want: "*/30 * * * * *"},
		{name: "minutes", interval: 5 * time.Minute, want: "0 */5 * * * *"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := healthCron(tt.interval); got != tt.want {
				t.Fatalf("healthCron(%s) = %q, want %q", tt.interval, got, tt.want)
			}
		})
	}
}

func TestScheduleTargetOwnership(t *testing.T) {
	schedule := &iapiserver.TaskSchedule{ProjectID: "project", Namespace: "namespace", CreatedBy: "user-1"}
	schedule.ID = "schedule-1"

	atomic := &iapiserver.AtomicTaskCreateRequest{}
	applyAtomicScheduleOwnership(atomic, schedule)
	if atomic.ProjectID != "project" || atomic.Namespace != "namespace" || atomic.CreatedBy != "user-1" || atomic.OwnerType != iapiserver.TaskOwnerTypeSchedule || atomic.OwnerID != "schedule-1" {
		t.Fatalf("atomic ownership = %#v", atomic)
	}
	group := &iapiserver.TaskGroupCreateRequest{}
	applyGroupScheduleOwnership(group, schedule)
	if group.ProjectID != "project" || group.Namespace != "namespace" || group.CreatedBy != "user-1" {
		t.Fatalf("group created_by = %q", group.CreatedBy)
	}
	dag := &iapiserver.DAGTaskGroupCreateRequest{}
	applyDAGScheduleOwnership(dag, schedule)
	if dag.ProjectID != "project" || dag.Namespace != "namespace" || dag.CreatedBy != "user-1" {
		t.Fatalf("dag created_by = %q", dag.CreatedBy)
	}
}

func TestScheduleTimeReadsConductorMilliseconds(t *testing.T) {
	want := time.Date(2026, time.July, 18, 1, 20, 30, 0, time.UTC)
	if got := scheduleTime(float64(want.UnixMilli())); !got.Equal(want) {
		t.Fatalf("scheduleTime = %s, want %s", got, want)
	}
}
