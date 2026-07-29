package notification

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	toolerrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

func TestNotificationErrorResponseWireShape(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	writeResponse(ctx, toolerrors.NewStatus(code.ErrNotificationQueryInvalid, "invalid topic"), nil)
	if recorder.Code != 200 {
		t.Fatalf("status=%d", recorder.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != "ERR_NOTIFICATION_QUERY_INVALID" || body["value"] != float64(code.ErrNotificationQueryInvalid) ||
		body["message"] == "" || body["retryable"] != false {
		t.Fatalf("body=%v", body)
	}
	messages, ok := body["messages"].(map[string]any)
	if !ok || messages["zh-CN"] == "" || messages["en-US"] == "" {
		t.Fatalf("messages=%v", body["messages"])
	}
}
