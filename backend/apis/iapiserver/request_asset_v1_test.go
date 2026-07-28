package iapiserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/pkg/core"
)

func TestUpdateCollectionRequestValidate(t *testing.T) {
	validName := "  renamed collection  "
	emptyName := "   "
	description := "updated description"
	resourceVersion := int64(3)

	tests := []struct {
		name    string
		request *UpdateCollectionRequest
		wantErr bool
	}{
		{
			name:    "rejects empty update",
			request: &UpdateCollectionRequest{},
			wantErr: true,
		},
		{
			name:    "rejects trim-empty name",
			request: &UpdateCollectionRequest{Name: &emptyName},
			wantErr: true,
		},
		{
			name:    "accepts padded valid name",
			request: &UpdateCollectionRequest{Name: &validName},
		},
		{
			name:    "accepts description-only update",
			request: &UpdateCollectionRequest{Description: &description},
		},
		{
			name:    "accepts resource-version-only request",
			request: &UpdateCollectionRequest{ResourceVersion: &resourceVersion},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() accepted an invalid request")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() rejected a valid request: %v", err)
			}
		})
	}
}

func TestDecodeParameterRejectsInvalidCollectionUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "rejects empty object", body: `{}`, wantErr: true},
		{name: "rejects empty name", body: `{"name":""}`, wantErr: true},
		{name: "rejects whitespace name", body: `{"name":"   "}`, wantErr: true},
		{name: "accepts description update", body: `{"description":"updated"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/collections/collection-1", strings.NewReader(tt.body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			request := &UpdateCollectionRequest{}

			err := core.DecodeParameter(ctx, request)
			if tt.wantErr && err == nil {
				t.Fatal("DecodeParameter() accepted an invalid request")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("DecodeParameter() rejected a valid request: %v", err)
			}
		})
	}
}
