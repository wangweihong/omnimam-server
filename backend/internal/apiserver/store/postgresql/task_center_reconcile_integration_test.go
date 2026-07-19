//go:build integration

package postgresql

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

func TestPostgresReconcileOverlapAndRetention(t *testing.T) {
	dsn := os.Getenv("OMNIMAM_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("OMNIMAM_TEST_POSTGRES_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	storage := newTaskCenterStore(&datastore{db: tx})
	ctx := context.Background()

	t.Run("overlap is recorded without acquisition", func(t *testing.T) {
		schedule := integrationReconcileSchedule("overlap")
		if _, err := storage.AddTaskSchedule(ctx, schedule); err != nil {
			t.Fatal(err)
		}
		first := integrationReconcileExecution(schedule.ID, time.Now().Add(-time.Minute), iapiserver.ScheduleExecutionStatusTriggered)
		missing, err := storage.GetScheduleExecutionAt(ctx, schedule.ID, first.ScheduledAt.Time)
		if err != nil || missing != nil {
			t.Fatalf("missing execution = %#v, err = %v", missing, err)
		}
		persisted, acquired, err := storage.AcquireScheduleExecution(ctx, first)
		if err != nil || !acquired {
			t.Fatalf("first acquired = %t, err = %v", acquired, err)
		}
		found, err := storage.GetScheduleExecutionAt(ctx, schedule.ID, first.ScheduledAt.Time)
		if err != nil || found == nil || found.ID != persisted.ID {
			t.Fatalf("found execution = %#v, err = %v", found, err)
		}
		second := integrationReconcileExecution(schedule.ID, time.Now(), iapiserver.ScheduleExecutionStatusTriggered)
		skipped, acquired, err := storage.AcquireScheduleExecution(ctx, second)
		if err != nil || acquired || skipped.Status != iapiserver.ScheduleExecutionStatusSkippedOverlap {
			t.Fatalf("second = %#v, acquired = %t, err = %v", skipped, acquired, err)
		}
		persisted.Status, persisted.CompletedAt = iapiserver.ScheduleExecutionStatusSuccess, imachinery.Now()
		if _, err := storage.UpdateScheduleExecution(ctx, persisted); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("retention keeps bounded terminal history and all active rows", func(t *testing.T) {
		schedule := integrationReconcileSchedule("retention")
		if _, err := storage.AddTaskSchedule(ctx, schedule); err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC()
		sequence := 0
		add := func(status string, count int, age time.Duration) {
			t.Helper()
			for range count {
				sequence++
				completedAt := imachinery.NewTime(now.Add(-age - time.Duration(sequence)*time.Second))
				execution := integrationReconcileExecution(schedule.ID, completedAt.Time.Add(-time.Second), status)
				execution.CompletedAt = completedAt
				if err := tx.Create(execution).Error; err != nil {
					t.Fatal(err)
				}
			}
		}
		add(iapiserver.ScheduleExecutionStatusSuccess, 6, time.Hour)
		add(iapiserver.ScheduleExecutionStatusSkippedOverlap, 6, time.Hour)
		add(iapiserver.ScheduleExecutionStatusFailed, 22, time.Hour)
		add(iapiserver.ScheduleExecutionStatusFailed, 3, 8*24*time.Hour)
		active := integrationReconcileExecution(schedule.ID, now, iapiserver.ScheduleExecutionStatusRunning)
		if err := tx.Create(active).Error; err != nil {
			t.Fatal(err)
		}
		retention := iapiserver.HistoryRetention{SuccessCount: 4, SkippedCount: 4, FailureCount: 20, FailureDurationSeconds: 7 * 24 * 60 * 60}
		if _, err := storage.PruneReconcileExecutions(ctx, schedule.ID, retention, now); err != nil {
			t.Fatal(err)
		}
		var statuses []struct {
			Status string
			Count  int64
		}
		if err := tx.Model(&iapiserver.TaskScheduleExecution{}).Select("status, count(*) AS count").Where("schedule_id = ?", schedule.ID).Group("status").Scan(&statuses).Error; err != nil {
			t.Fatal(err)
		}
		got := make(map[string]int64, len(statuses))
		for _, item := range statuses {
			got[item.Status] = item.Count
		}
		if got[iapiserver.ScheduleExecutionStatusSuccess] != 4 || got[iapiserver.ScheduleExecutionStatusSkippedOverlap] != 4 || got[iapiserver.ScheduleExecutionStatusFailed] != 20 || got[iapiserver.ScheduleExecutionStatusRunning] != 1 {
			t.Fatalf("retained statuses = %#v", got)
		}
	})
}

func integrationReconcileSchedule(suffix string) *iapiserver.TaskSchedule {
	id := uuid.NewString()
	return &iapiserver.TaskSchedule{
		ObjectMeta:                     imachinery.ObjectMeta{ID: id, Name: "reconcile integration " + suffix},
		ExecutionMode:                  iapiserver.TaskScheduleModeReconcile,
		ManagementMode:                 iapiserver.TaskScheduleManagementSystem,
		SystemKey:                      fmt.Sprintf("integration.%s.%s", suffix, id),
		TriggerType:                    iapiserver.TaskScheduleTriggerCron,
		CronExpression:                 "*/30 * * * * *",
		TimeZone:                       "UTC",
		ReconcileSpec:                  &iapiserver.ReconcileSpec{ReconcileRef: "integration.reconcile", Config: map[string]any{}, MaxParallelism: 1, MaxItemsPerRun: 1, PerItemTimeoutSeconds: 1, OverallTimeoutSeconds: 1},
		HistoryRetention:               iapiserver.HistoryRetention{SuccessCount: 4, SkippedCount: 4, FailureCount: 20, FailureDurationSeconds: 7 * 24 * 60 * 60, RuntimeRetentionSeconds: 24 * 60 * 60},
		Status:                         iapiserver.TaskScheduleStatusActive,
		MisfirePolicy:                  iapiserver.TaskSchedulePolicySkip,
		OverlapPolicy:                  iapiserver.TaskSchedulePolicySkip,
		RuntimeScheduleName:            "integration_" + id,
		ProjectID:                      iapiserver.DefaultTaskCenterProjectID,
		Namespace:                      iapiserver.DefaultTaskCenterNamespace,
		CreatedBy:                      iapiserver.DefaultTaskCenterCreatedBy,
		ReconcileMaxParallelism:        1,
		ReconcileMaxItemsPerRun:        1,
		ReconcilePerItemTimeoutSeconds: 1,
		ReconcileOverallTimeoutSeconds: 1,
	}
}

func integrationReconcileExecution(scheduleID string, scheduledAt time.Time, status string) *iapiserver.TaskScheduleExecution {
	return &iapiserver.TaskScheduleExecution{
		ObjectMeta:       imachinery.ObjectMeta{ID: uuid.NewString(), Name: "reconcile integration execution"},
		ScheduleID:       scheduleID,
		ExecutionMode:    iapiserver.TaskScheduleModeReconcile,
		ScheduledAt:      imachinery.NewTime(scheduledAt.UTC()),
		TriggeredAt:      imachinery.Now(),
		Status:           status,
		ReconcileSummary: iapiserver.ReconcileSummary{},
	}
}
