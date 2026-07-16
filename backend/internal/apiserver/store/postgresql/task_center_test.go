package postgresql

import (
	"strings"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

func TestTaskCenterOrderByUsesContractTimestampColumn(t *testing.T) {
	allowed := map[string]string{
		"created_at": taskCenterCreatedAtColumn,
		"updated_at": taskCenterUpdatedAtColumn,
	}
	order := taskCenterOrderBy(imachinery.BasicQueryParam{}, allowed, taskCenterCreatedAtColumn)
	if order.Column.Name != taskCenterCreatedAtColumn {
		t.Fatalf("column = %s, want %s", order.Column.Name, taskCenterCreatedAtColumn)
	}
	if order.Desc {
		t.Fatal("contract default order should be ascending")
	}

	order = taskCenterOrderBy(imachinery.BasicQueryParam{
		SortField: "updated_at",
		SortOrder: "asc",
	}, allowed, taskCenterCreatedAtColumn)
	if order.Column.Name != taskCenterUpdatedAtColumn {
		t.Fatalf("column = %s, want %s", order.Column.Name, taskCenterUpdatedAtColumn)
	}
	if order.Desc {
		t.Fatal("asc sort should not be descending")
	}
}

func TestTaskCenterOrderBySupportsRunAndAttemptContractFields(t *testing.T) {
	tests := []struct {
		name      string
		sortField string
	}{
		{name: "run started time", sortField: "started_at"},
		{name: "run completed time", sortField: "completed_at"},
		{name: "attempt number", sortField: "attempt_no"},
		{name: "attempt heartbeat", sortField: "heartbeat_at"},
		{name: "attempt progress", sortField: "progress_at"},
		{name: "attempt worker", sortField: "worker_id"},
	}
	allowed := make(map[string]string, len(tests))
	for _, tt := range tests {
		allowed[tt.sortField] = tt.sortField
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := taskCenterOrderBy(
				imachinery.BasicQueryParam{SortField: tt.sortField, SortOrder: "asc"},
				allowed,
				"attempt_no",
			)
			if order.Column.Name != tt.sortField {
				t.Fatalf("column = %s, want %s", order.Column.Name, tt.sortField)
			}
			if order.Desc {
				t.Fatal("asc sort should not be descending")
			}
		})
	}
}

func TestTaskCenterActiveLeaseIndexMatchesContract(t *testing.T) {
	if !strings.Contains(taskCenterActiveLeaseIndexSQL, "idx_task_execution_leases_active_run") {
		t.Fatalf("index SQL missing contract index name: %s", taskCenterActiveLeaseIndexSQL)
	}
	if !strings.Contains(taskCenterActiveLeaseIndexSQL, "WHERE status IN ('ACTIVE', 'RENEWED')") {
		t.Fatalf("index SQL missing active lease predicate: %s", taskCenterActiveLeaseIndexSQL)
	}
}

func TestTaskCenterApplicationRunIndexesMatchContract(t *testing.T) {
	if !strings.Contains(taskCenterApplicationRunIndexesSQL, "idx_task_runs_application_idempotency") {
		t.Fatalf("index SQL missing idempotency index: %s", taskCenterApplicationRunIndexesSQL)
	}
	if !strings.Contains(
		taskCenterApplicationRunIndexesSQL,
		"WHERE application_run_id <> '' AND idempotency_key <> ''",
	) {
		t.Fatalf("index SQL missing application idempotency predicate: %s", taskCenterApplicationRunIndexesSQL)
	}
}

func TestApplyTaskFailureStatus(t *testing.T) {
	tests := []struct {
		name        string
		failureType string
		retryable   bool
		wantRun     string
		wantAttempt string
	}{
		{name: "canceled", failureType: iapiserver.FailureTypeCanceled, wantRun: iapiserver.TaskRunStatusCanceled, wantAttempt: iapiserver.TaskAttemptStatusCanceled},
		{name: "timeout", failureType: iapiserver.FailureTypeTimeout, wantRun: iapiserver.TaskRunStatusTimeout, wantAttempt: iapiserver.TaskAttemptStatusTimeout},
		{name: "retryable function error", failureType: iapiserver.FailureTypeFunctionError, retryable: true, wantRun: iapiserver.TaskRunStatusRetrying, wantAttempt: iapiserver.TaskAttemptStatusFailed},
		{name: "terminal function error", failureType: iapiserver.FailureTypeFunctionError, wantRun: iapiserver.TaskRunStatusFailed, wantAttempt: iapiserver.TaskAttemptStatusFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			run := &iapiserver.TaskRun{CurrentAttempt: 1, MaxAttempts: 2, Progress: 1}
			attempt := &iapiserver.TaskAttempt{}
			applyTaskFailureStatus(run, attempt, iapiserver.TaskError{FailureType: test.failureType, Retryable: test.retryable}, imachinery.Now())
			if run.Status != test.wantRun || attempt.Status != test.wantAttempt {
				t.Fatalf("got run=%s attempt=%s, want run=%s attempt=%s", run.Status, attempt.Status, test.wantRun, test.wantAttempt)
			}
		})
	}
}

func TestSameTaskRunCreateRequest(t *testing.T) {
	base := &iapiserver.TaskRun{
		DefinitionType:   iapiserver.TaskDefinitionTypeAtomic,
		DefinitionID:     "application.execute",
		ApplicationRunID: "app-run-1",
		IdempotencyKey:   "submit-1",
		Input:            map[string]any{"prompt": "hello"},
		MaxAttempts:      1,
		ProjectID:        "default",
		Namespace:        "default",
		CreatedBy:        "user-1",
	}
	same := *base
	same.Input = map[string]any{"prompt": "hello"}
	if !sameTaskRunCreateRequest(base, &same) {
		t.Fatal("equivalent requests should match")
	}
	different := same
	different.Input = map[string]any{"prompt": "different"}
	if sameTaskRunCreateRequest(base, &different) {
		t.Fatal("different request payloads should conflict")
	}
}
