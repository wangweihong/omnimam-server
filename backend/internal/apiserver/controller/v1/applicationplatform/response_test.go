package applicationplatform

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

func TestApplicationPlatformErrorResponseMatchesS2(t *testing.T) {
	if len(applicationPlatformErrors) != 61 {
		t.Fatalf("expected all 61 S2 error definitions, got %d", len(applicationPlatformErrors))
	}
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	writeResponse(ctx, errors.NewStatus(code.ErrAIAppEngineUnavailable, "provider timed out"), nil)

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || response["code"] != "ERR_AIAPP_ENGINE_UNAVAILABLE" || response["value"] != float64(code.ErrAIAppEngineUnavailable) || response["retryable"] != true {
		t.Fatalf("unexpected S2 error response: status=%d body=%#v", recorder.Code, response)
	}
	messages, _ := response["messages"].(map[string]any)
	if messages["zh-CN"] == "" || messages["en-US"] == "" {
		t.Fatalf("localized messages are missing: %#v", response)
	}
}

func TestApplicationPlatformBindingErrorUsesDomainCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/engine-instances", strings.NewReader(`{"name":"missing-fields"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	run(ctx, &iapiserver.EngineInstanceCreateRequest{}, func(*iapiserver.EngineInstanceCreateRequest) (any, error) {
		t.Fatal("action must not run for an invalid request")
		return nil, nil
	})
	var response applicationPlatformErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != "ERR_AIAPP_APPLICATION_INPUT_INVALID" || response.Value != code.ErrAIAppApplicationInputInvalid {
		t.Fatalf("unexpected validation response: %#v", response)
	}
}
