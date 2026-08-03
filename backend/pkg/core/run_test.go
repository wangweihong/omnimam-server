package core_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

type requiredRequest struct {
	Name string `json:"name" binding:"required"`
}

func TestRunBindingErrorUsesValidationStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	actionCalled := false
	router := gin.New()
	router.POST("/test", func(c *gin.Context) {
		core.Run(c, &requiredRequest{}, func(*requiredRequest) (any, error) {
			actionCalled = true
			return nil, nil
		})
	})

	request := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if actionCalled {
		t.Fatal("action was called after request validation failed")
	}

	var body core.ErrResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != code.ErrValidation {
		t.Errorf("business code = %d, want %d", body.Code, code.ErrValidation)
	}
	if body.Message != "Validation failed." {
		t.Errorf("message = %q, want %q", body.Message, "Validation failed.")
	}
}
