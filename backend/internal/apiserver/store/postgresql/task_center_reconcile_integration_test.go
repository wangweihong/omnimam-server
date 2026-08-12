//go:build integration

package postgresql

import (
	"context"
	"encoding/json"
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

func TestPostgresProjectStudioPreviewEnsureTerminal(t *testing.T) {
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

	ownerID := uuid.NewString()
	applicationID := uuid.NewString()
	if err := tx.Create(&iapiserver.StudioApplication{
		ObjectMeta:  imachinery.ObjectMeta{ID: applicationID},
		OwnerUserID: ownerID,
		Status:      "ACTIVE",
	}).Error; err != nil {
		t.Fatal(err)
	}
	previewID := uuid.NewString()
	workspaceID := uuid.NewString()
	preview := &iapiserver.StudioPreviewRuntime{
		ObjectMeta:          imachinery.ObjectMeta{ID: previewID},
		StudioApplicationID: applicationID,
		WorkspaceID:         workspaceID,
		WorkspaceRevision:   3,
		Status:              "STARTING",
	}
	if err := tx.Create(preview).Error; err != nil {
		t.Fatal(err)
	}

	storage := newAppStudioStore(&datastore{db: tx})
	task := &iapiserver.AtomicTask{
		FunctionRef: "appstudio.preview.ensure",
		Arguments:   map[string]any{"preview_runtime_id": previewID},
		Status:      iapiserver.AtomicTaskStatusSuccess,
		Output: map[string]any{
			"infra_runtime_id": "infra-preview-1",
			"runtime_status":   "RUNNING",
			"health_status":    "HEALTHY",
			"endpoint_ref":     "infra-endpoint://endpoint-1",
		},
	}
	if err := storage.ProjectStudioTaskTerminal(t.Context(), task); err != nil {
		t.Fatal(err)
	}

	var persisted iapiserver.StudioPreviewRuntime
	if err := tx.Where("id = ?", previewID).First(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.InfraRuntimeID != "infra-preview-1" || persisted.EndpointRef != "infra-endpoint://endpoint-1" || persisted.Status != "RUNNING" {
		t.Fatalf("preview projection = %#v", persisted)
	}
	var diagnostics map[string]any
	if err := json.Unmarshal(persisted.DiagnosticsSummary, &diagnostics); err != nil {
		t.Fatalf("decode diagnostics summary: %v", err)
	}
	if diagnostics["health_status"] != "HEALTHY" {
		t.Fatalf("diagnostics summary = %#v, want health_status HEALTHY", diagnostics)
	}
	visible, err := storage.GetStudioPreviewRuntime(t.Context(), workspaceID, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if visible.EndpointSummary == nil || visible.EndpointSummary.DisplayRef != "infra-endpoint://endpoint-1" || visible.EndpointSummary.Visibility != "USER_ACCESSIBLE" || visible.EndpointSummary.Status != "READY" {
		t.Fatalf("endpoint summary = %#v", visible.EndpointSummary)
	}
}

func TestPostgresProjectStudioInitializationTerminalFencesOldDAG(t *testing.T) {
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

	ownerID, applicationID := uuid.NewString(), uuid.NewString()
	currentDAGID, oldDAGID := uuid.NewString(), uuid.NewString()
	app := &iapiserver.StudioApplication{ObjectMeta: imachinery.ObjectMeta{ID: applicationID}, OwnerUserID: ownerID, Status: iapiserver.AppStudioApplicationStatusCreating, InitializationDAGTaskGroupID: currentDAGID, CreateIdempotencyKey: uuid.NewString()}
	if err := tx.Create(app).Error; err != nil {
		t.Fatal(err)
	}
	storage := newAppStudioStore(&datastore{db: tx})
	task := &iapiserver.AtomicTask{FunctionRef: iapiserver.AppStudioFunctionInitializationInvocationStart, OwnerID: oldDAGID, Status: iapiserver.AtomicTaskStatusFailed, Arguments: iapiserver.AppStudioInitializationTaskArguments{StudioApplicationID: applicationID, OwnerUserID: ownerID, CreateIdempotencyKey: app.CreateIdempotencyKey}.AtomicTaskArguments()}
	if err := storage.ProjectStudioTaskTerminal(t.Context(), task); err != nil {
		t.Fatal(err)
	}
	var persisted iapiserver.StudioApplication
	if err := tx.Where("id = ?", applicationID).First(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.Status != iapiserver.AppStudioApplicationStatusCreating || persisted.InitializationDAGTaskGroupID != currentDAGID {
		t.Fatalf("old DAG overwrote current initialization: %#v", persisted)
	}
}

func TestPostgresAgentInvocationSubmissionFailureFence(t *testing.T) {
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

	ownerID := uuid.NewString()
	agent := &iapiserver.Agent{
		ObjectMeta:           imachinery.ObjectMeta{ID: uuid.NewString()},
		OwnerUserID:          ownerID,
		Kind:                 iapiserver.AgentKindCoding,
		AgentProfileID:       "integration-profile",
		AgentProfileRevision: "1",
		WorkspaceType:        iapiserver.AgentWorkspaceTypeStudio,
		WorkspaceID:          uuid.NewString(),
		Status:               iapiserver.AgentStatusReady,
	}
	if err := tx.Create(agent).Error; err != nil {
		t.Fatal(err)
	}
	session := &iapiserver.AgentSession{
		ObjectMeta:  imachinery.ObjectMeta{ID: uuid.NewString()},
		AgentID:     agent.ID,
		OwnerUserID: ownerID,
		Title:       "Invocation submission fence",
		Status:      iapiserver.AgentSessionStatusOpen,
	}
	if err := tx.Create(session).Error; err != nil {
		t.Fatal(err)
	}

	storage := newAgentStore(&datastore{db: tx})
	invocation := integrationQueuedAgentInvocation(agent.ID, session.ID, "failure")
	if err := tx.Create(invocation).Error; err != nil {
		t.Fatal(err)
	}
	originalVersion := invocation.ResourceVersion
	originalGeneration := invocation.SubmissionGeneration

	failed, applied, err := storage.FailAgentInvocationSubmission(
		t.Context(), invocation.ID, originalVersion, originalGeneration, "Invocation task adapter is unavailable.",
	)
	if err != nil || !applied {
		t.Fatalf("failure projection applied = %t, err = %v", applied, err)
	}
	if failed.Status != iapiserver.AgentInvocationStatusFailed ||
		failed.FailureCode != iapiserver.AgentInvocationFailureCodeTaskUnavailable || failed.FailureMessage == "" ||
		failed.AtomicTaskID != nil || failed.CompletedAt.Time.IsZero() || failed.ResourceVersion != originalVersion+1 {
		t.Fatalf("failed invocation = %#v", failed)
	}

	var failureEvent iapiserver.AgentOutbox
	if err := tx.Where("aggregate_id = ? AND idempotency_key = ?", invocation.ID, fmt.Sprintf("%s:%d", invocation.ID, failed.ResourceVersion)).First(&failureEvent).Error; err != nil {
		t.Fatal(err)
	}
	var failurePayload map[string]any
	if err := json.Unmarshal(failureEvent.Payload, &failurePayload); err != nil {
		t.Fatal(err)
	}
	if failureEvent.EventType != "agent_invocation_status_changed" ||
		failurePayload["from_status"] != iapiserver.AgentInvocationStatusQueued ||
		failurePayload["to_status"] != iapiserver.AgentInvocationStatusFailed ||
		failurePayload["error_code"] != iapiserver.AgentInvocationFailureCodeTaskUnavailable {
		t.Fatalf("failure outbox = %#v, payload = %#v", failureEvent, failurePayload)
	}

	retried, applied, err := storage.RetryAgentInvocationSubmission(
		t.Context(), failed.ID, failed.ResourceVersion, failed.SubmissionGeneration,
	)
	if err != nil || !applied {
		t.Fatalf("retry projection applied = %t, err = %v", applied, err)
	}
	if retried.Status != iapiserver.AgentInvocationStatusQueued || retried.AtomicTaskID != nil ||
		retried.FailureCode != "" || retried.FailureMessage != "" || !retried.CompletedAt.Time.IsZero() ||
		retried.SubmissionGeneration != originalGeneration+1 || retried.ResourceVersion != failed.ResourceVersion+1 {
		t.Fatalf("retried invocation = %#v", retried)
	}

	stale, applied, err := storage.FailAgentInvocationSubmission(
		t.Context(), invocation.ID, failed.ResourceVersion, originalGeneration, "stale failure",
	)
	if err != nil || applied {
		t.Fatalf("stale failure projection applied = %t, err = %v", applied, err)
	}
	if stale.Status != iapiserver.AgentInvocationStatusQueued || stale.SubmissionGeneration != originalGeneration+1 || stale.FailureMessage != "" {
		t.Fatalf("stale failure changed invocation = %#v", stale)
	}

	boundCandidate := integrationQueuedAgentInvocation(agent.ID, session.ID, "bound")
	if err := tx.Create(boundCandidate).Error; err != nil {
		t.Fatal(err)
	}
	taskID := uuid.NewString()
	bound, applied, err := storage.BindAgentInvocationTask(
		t.Context(), boundCandidate.ID, boundCandidate.ResourceVersion, boundCandidate.SubmissionGeneration,
		uuid.NewString(), taskID, boundCandidate.ResourceVersion+1,
	)
	if err != nil || !applied {
		t.Fatalf("task binding applied = %t, err = %v", applied, err)
	}
	unchanged, applied, err := storage.FailAgentInvocationSubmission(
		t.Context(), bound.ID, bound.ResourceVersion, bound.SubmissionGeneration, "late failure",
	)
	if err != nil || applied {
		t.Fatalf("bound failure projection applied = %t, err = %v", applied, err)
	}
	if unchanged.AtomicTaskID == nil || *unchanged.AtomicTaskID != taskID || unchanged.Status != iapiserver.AgentInvocationStatusQueued {
		t.Fatalf("late failure changed task binding = %#v", unchanged)
	}

	fenceCandidate := integrationQueuedAgentInvocation(agent.ID, session.ID, "fence-loss")
	if err := tx.Create(fenceCandidate).Error; err != nil {
		t.Fatal(err)
	}
	staleVersion := fenceCandidate.ResourceVersion
	fenceCandidate.RuntimeBindingID = uuid.NewString()
	current, err := storage.UpdateAgentInvocation(t.Context(), fenceCandidate)
	if err != nil {
		t.Fatal(err)
	}
	latest, applied, err := storage.BindAgentInvocationTask(
		t.Context(), current.ID, staleVersion, current.SubmissionGeneration,
		current.RuntimeBindingID, uuid.NewString(), staleVersion+1,
	)
	if err != nil || applied || latest.ResourceVersion != current.ResourceVersion || latest.AtomicTaskID != nil {
		t.Fatalf("stale task binding result = %#v, applied = %t, err = %v", latest, applied, err)
	}
	failedAfterFenceLoss, applied, err := storage.FailAgentInvocationSubmission(
		t.Context(), latest.ID, latest.ResourceVersion, latest.SubmissionGeneration, "task binding fence was lost",
	)
	if err != nil || !applied || failedAfterFenceLoss.Status != iapiserver.AgentInvocationStatusFailed {
		t.Fatalf("failure after binding fence loss = %#v, applied = %t, err = %v", failedAfterFenceLoss, applied, err)
	}
}

func TestPostgresAppStudioTerminalRecoverySource(t *testing.T) {
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

	terminalBuildTask := integrationAtomicTask(iapiserver.AppStudioFunctionBuildExecute, iapiserver.AtomicTaskStatusSuccess)
	nonTerminalBuildTask := integrationAtomicTask(iapiserver.AppStudioFunctionBuildExecute, iapiserver.AtomicTaskStatusRunning)
	terminalProductionTask := integrationAtomicTask(iapiserver.AppStudioFunctionProductionEnsure, iapiserver.AtomicTaskStatusFailed)
	projectedProductionTask := integrationAtomicTask(iapiserver.AppStudioFunctionProductionEnsure, iapiserver.AtomicTaskStatusSuccess)
	for _, task := range []*iapiserver.AtomicTask{terminalBuildTask, nonTerminalBuildTask, terminalProductionTask, projectedProductionTask} {
		if err := tx.Create(task).Error; err != nil {
			t.Fatal(err)
		}
	}

	for _, build := range []*iapiserver.StudioBuild{
		integrationStudioBuild(terminalBuildTask.ID, iapiserver.AppStudioBuildStatusRunning),
		integrationStudioBuild(nonTerminalBuildTask.ID, iapiserver.AppStudioBuildStatusRunning),
	} {
		if err := tx.Create(build).Error; err != nil {
			t.Fatal(err)
		}
	}

	pendingRelease := integrationStudioRelease(iapiserver.AppStudioReleaseStatusDeploying)
	projectedRelease := integrationStudioRelease(iapiserver.AppStudioReleaseStatusReady)
	for _, release := range []*iapiserver.StudioRelease{pendingRelease, projectedRelease} {
		if err := tx.Create(release).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, runtime := range []*iapiserver.StudioRuntimeInstance{
		integrationStudioRuntime(pendingRelease, terminalProductionTask.ID),
		integrationStudioRuntime(projectedRelease, projectedProductionTask.ID),
	} {
		if err := tx.Create(runtime).Error; err != nil {
			t.Fatal(err)
		}
	}

	storage := newAppStudioStore(&datastore{db: tx})
	ids, err := storage.ListPendingStudioTerminalTaskIDs(t.Context(), 20)
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]bool, len(ids))
	for _, id := range ids {
		got[id] = true
	}
	if !got[terminalBuildTask.ID] || !got[terminalProductionTask.ID] ||
		got[nonTerminalBuildTask.ID] || got[projectedProductionTask.ID] || len(got) != 2 {
		t.Fatalf("pending AppStudio terminal tasks = %#v", ids)
	}
}

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

func integrationQueuedAgentInvocation(agentID, sessionID, suffix string) *iapiserver.AgentInvocation {
	return &iapiserver.AgentInvocation{
		ObjectMeta:           imachinery.ObjectMeta{ID: uuid.NewString()},
		AgentID:              agentID,
		SessionID:            sessionID,
		Type:                 iapiserver.AgentInvocationTypeCoding,
		Status:               iapiserver.AgentInvocationStatusQueued,
		IdempotencyKey:       "integration:" + suffix + ":" + uuid.NewString(),
		SubmissionGeneration: 0,
	}
}

func integrationAtomicTask(functionRef, status string) *iapiserver.AtomicTask {
	id := uuid.NewString()
	return &iapiserver.AtomicTask{
		ObjectMeta:  imachinery.ObjectMeta{ID: id, Name: "Integration task " + id},
		FunctionRef: functionRef,
		Status:      status,
		ProjectID:   iapiserver.DefaultTaskCenterProjectID,
		Namespace:   iapiserver.DefaultTaskCenterNamespace,
		CreatedBy:   iapiserver.DefaultTaskCenterCreatedBy,
	}
}

func integrationStudioBuild(taskID, status string) *iapiserver.StudioBuild {
	id := uuid.NewString()
	return &iapiserver.StudioBuild{
		ObjectMeta:          imachinery.ObjectMeta{ID: id, Name: "Integration build " + id},
		OwnerUserID:         uuid.NewString(),
		StudioApplicationID: uuid.NewString(),
		SourceSnapshotID:    uuid.NewString(),
		AtomicTaskID:        taskID,
		Status:              status,
		IdempotencyKey:      "integration-build:" + id,
	}
}

func integrationStudioRelease(status string) *iapiserver.StudioRelease {
	id := uuid.NewString()
	return &iapiserver.StudioRelease{
		ObjectMeta:                 imachinery.ObjectMeta{ID: id, Name: "Integration release " + id},
		OwnerUserID:                uuid.NewString(),
		StudioApplicationID:        uuid.NewString(),
		StudioApplicationVersionID: uuid.NewString(),
		StudioBuildID:              uuid.NewString(),
		RuntimeConfigID:            uuid.NewString(),
		ArtifactID:                 uuid.NewString(),
		ArtifactDigest:             "sha256:integration",
		Environment:                iapiserver.AppStudioEnvironmentProduction,
		Status:                     status,
		IdempotencyKey:             "integration-release:" + id,
	}
}

func integrationStudioRuntime(release *iapiserver.StudioRelease, taskID string) *iapiserver.StudioRuntimeInstance {
	return &iapiserver.StudioRuntimeInstance{
		ObjectMeta:          imachinery.ObjectMeta{ID: uuid.NewString()},
		StudioApplicationID: release.StudioApplicationID,
		StudioReleaseID:     release.ID,
		Environment:         release.Environment,
		AtomicTaskID:        taskID,
		Status:              iapiserver.AppStudioRuntimeStatusCreating,
		HealthStatus:        iapiserver.AppStudioRuntimeHealthUnknown,
	}
}
