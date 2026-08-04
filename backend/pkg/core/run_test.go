package core_test

import (
	"encoding/json"
	"errors"
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

type postBindErrorRequest struct{}

func (postBindErrorRequest) PostBind() error {
	return errors.New("post bind failed")
}

func TestRunValidationErrorUsesValidationStatus(t *testing.T) {
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

	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusOK)
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

func TestRunPostBindErrorUsesBindStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	actionCalled := false
	router := gin.New()
	router.POST("/test", func(c *gin.Context) {
		core.Run(c, &postBindErrorRequest{}, func(*postBindErrorRequest) (any, error) {
			actionCalled = true
			return nil, nil
		})
	})

	request := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusOK)
	}
	if actionCalled {
		t.Fatal("action was called after post-bind failed")
	}

	var body core.ErrResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != code.ErrBind {
		t.Errorf("business code = %d, want %d", body.Code, code.ErrBind)
	}
	if body.Message != "Error occurred while binding the request body to the struct." {
		t.Errorf("message = %q, want %q", body.Message, "Error occurred while binding the request body to the struct.")
	}
}
