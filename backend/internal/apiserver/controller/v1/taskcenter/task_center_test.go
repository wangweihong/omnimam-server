package taskcenter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	taskcentersvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
)

type atomicTaskResponseService struct{ taskcentersvc.TaskCenterSrv }

func (atomicTaskResponseService) ListAtomicTasks(context.Context, *iapiserver.AtomicTaskListRequest) (*iapiserver.AtomicTaskListResponse, error) {
	now := imachinery.NewTime(time.Date(2026, time.July, 20, 16, 0, 0, 0, time.UTC))
	return &iapiserver.AtomicTaskListResponse{
		Total: 1,
		Items: []*iapiserver.AtomicTask{{
			ObjectMeta: imachinery.ObjectMeta{
				ID:              "task-1",
				Name:            "Task one",
				Description:     "internal description",
				CreatedAt:       now,
				UpdatedAt:       now,
				ResourceVersion: 3,
			},
			ChildKey:             "task-key",
			FunctionRef:          "asset-library.representation.finalize",
			Status:               iapiserver.AtomicTaskStatusSuccess,
			Progress:             1,
			ProjectID:            "default",
			Namespace:            "default",
			CreatedBy:            "system-admin",
			IdempotencyScope:     "internal-scope",
			IdempotencyKey:       "internal-key",
			RuntimeRevision:      "internal-revision",
			ScheduleAt:           imachinery.Time{},
			CanceledAt:           imachinery.Time{},
			ApplicationRunID:     "application-run-1",
			CanvasRunID:          "canvas-run-1",
			CanvasNodeRunID:      "canvas-node-run-1",
			RequiredCapabilities: "asset-library.representation.finalize",
		}},
	}, nil
}

func TestListAtomicTasksReturnsReleasedContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	controller := NewController(atomicTaskResponseService{})
	router.GET("/api/v1/atomic-tasks", controller.ListAtomicTasks)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/atomic-tasks?page_num=0&page_size=1", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Items) != 1 {
		t.Fatalf("items = %#v, want one item", response.Items)
	}
	item := response.Items[0]
	if item["key"] != "task-key" {
		t.Fatalf("key = %#v, want task-key; item=%#v", item["key"], item)
	}
	for _, field := range []string{
		"key",
		"function_ref",
		"id",
		"status",
		"progress",
		"resource_version",
		"project_id",
		"namespace",
		"created_at",
		"updated_at",
	} {
		if _, exists := item[field]; !exists {
			t.Errorf("required contract field %q missing from response: %#v", field, item)
		}
	}
	for _, field := range []string{
		"child_key",
		"description",
		"idempotency_scope",
		"idempotency_key",
		"runtime_revision",
		"schedule_at",
		"started_at",
		"completed_at",
		"canceled_at",
		"last_error",
		"application_run_id",
		"canvas_run_id",
		"canvas_node_run_id",
	} {
		if _, exists := item[field]; exists {
			t.Errorf("non-contract field %q leaked in response: %#v", field, item)
		}
	}
}
