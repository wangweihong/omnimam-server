package postgresql

import (
	"strings"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

func TestTaskCenterOrderByUsesContractTimestampColumn(t *testing.T) {
	order := taskCenterOrderBy(imachinery.BasicQueryParam{})
	if order.Column.Name != taskCenterCreatedAtColumn {
		t.Fatalf("column = %s, want %s", order.Column.Name, taskCenterCreatedAtColumn)
	}
	if !order.Desc {
		t.Fatal("default order should be descending")
	}

	order = taskCenterOrderBy(imachinery.BasicQueryParam{
		SortField: "updated_at",
		SortOrder: "asc",
	})
	if order.Column.Name != taskCenterUpdatedAtColumn {
		t.Fatalf("column = %s, want %s", order.Column.Name, taskCenterUpdatedAtColumn)
	}
	if order.Desc {
		t.Fatal("asc sort should not be descending")
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
