package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	ssesvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/sse"
)

type fakeSSEService struct {
	cursorState ssesvc.CursorState
	syncState   *iapiserver.EventSyncState
	listAfter   func(context.Context, int64, int) ([]*iapiserver.UserEvent, error)
}

func (s *fakeSSEService) List(context.Context, *iapiserver.UserEventListRequest) (*iapiserver.UserEventListResponse, error) {
	return &iapiserver.UserEventListResponse{}, nil
}
func (s *fakeSSEService) SyncState(context.Context) (*iapiserver.EventSyncState, error) {
	return s.syncState, nil
}
func (s *fakeSSEService) ResolveCursor(context.Context, int64) (ssesvc.CursorState, *iapiserver.EventSyncState, error) {
	return s.cursorState, s.syncState, nil
}
func (s *fakeSSEService) ListAfter(ctx context.Context, after int64, limit int) ([]*iapiserver.UserEvent, error) {
	return s.listAfter(ctx, after, limit)
}
func (s *fakeSSEService) CurrentUserID(context.Context) (string, error) { return "user-1", nil }

func newStreamContext(request *http.Request) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request
	return ctx, recorder
}

func TestStreamEventsEmitsResyncForExpiredCursor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := &Controller{
		service:      &fakeSSEService{cursorState: ssesvc.CursorExpired, syncState: &iapiserver.EventSyncState{EarliestAvailableEventID: 10}},
		pollInterval: time.Second, heartbeatInterval: time.Second, connections: newConnectionLimiter(2),
	}
	ctx, recorder := newStreamContext(httptest.NewRequest(http.MethodGet, "/api/v1/events/stream?after_event_id=4", nil))

	controller.StreamEvents(ctx)

	if got := recorder.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "event: connection.resync_required") || !strings.Contains(body, "event_retention_expired") || strings.Contains(body, "id:") {
		t.Fatalf("unexpected resync frame: %q", body)
	}
}

func TestStreamEventsReplaysBusinessEventAfterCursor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	requestContext, cancel := context.WithCancel(context.Background())
	event := &iapiserver.UserEvent{
		EventSequence: 6, EventType: iapiserver.UserEventAtomicTaskProgressed, EventVersion: 1,
		AggregateType: "atomic_task", AggregateID: "task-1", AggregateVersion: 3,
		AtomicTaskID: "task-1", Payload: map[string]any{"status": "RUNNING", "progress": 0.5},
		OccurredAt: imachinery.Now(),
	}
	service := &fakeSSEService{cursorState: ssesvc.CursorValid, syncState: &iapiserver.EventSyncState{LatestEventID: 5}}
	service.listAfter = func(context.Context, int64, int) ([]*iapiserver.UserEvent, error) {
		cancel()
		return []*iapiserver.UserEvent{event}, nil
	}
	controller := &Controller{service: service, pollInterval: time.Millisecond, heartbeatInterval: time.Hour, connections: newConnectionLimiter(2)}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/events/stream?after_event_id=5&client_instance_id=tab-1", nil).WithContext(requestContext)
	ctx, recorder := newStreamContext(request)

	controller.StreamEvents(ctx)

	body := recorder.Body.String()
	for _, expected := range []string{"event: connection.ready", "id: 6", "event: atomic_task.progressed", `"aggregate_version":3`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("stream body %q misses %q", body, expected)
		}
	}
	if strings.Contains(body, `"recipient_user_id"`) || strings.Contains(body, `"source_event_id"`) || strings.Contains(body, `"expires_at"`) {
		t.Fatalf("stream exposed internal fields: %q", body)
	}
}

func TestConnectionLimiterEnforcesUserAndClientBounds(t *testing.T) {
	limiter := newConnectionLimiter(2)
	if !limiter.acquire("user-1", "tab-1") || limiter.acquire("user-1", "tab-1") {
		t.Fatal("same client instance must have only one connection")
	}
	if !limiter.acquire("user-1", "tab-2") || limiter.acquire("user-1", "tab-3") {
		t.Fatal("per-user connection limit was not enforced")
	}
	limiter.release("user-1", "tab-1")
	if !limiter.acquire("user-1", "tab-3") {
		t.Fatal("released connection slot was not reusable")
	}
}
