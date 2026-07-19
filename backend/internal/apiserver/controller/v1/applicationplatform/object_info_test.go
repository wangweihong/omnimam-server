package applicationplatform

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	appservice "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
)

type objectInfoControllerService struct {
	appservice.ApplicationPlatformSrv
}

func (objectInfoControllerService) GetComfyUIEngineObjectInfo(context.Context, string) (*iapiserver.ComfyUIEngineObjectInfoResponse, error) {
	return &iapiserver.ComfyUIEngineObjectInfoResponse{EngineInstanceID: "engine-1", Available: true, RefreshedAt: imachinery.Now(), ObjectInfo: map[string]any{"KSampler": map[string]any{}}}, nil
}

func TestGetComfyUIEngineObjectInfoSupportsGzip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/engine-instances/engine-1/object-info", nil)
	ctx.Request.Header.Set("Accept-Encoding", "gzip")
	ctx.Params = gin.Params{{Key: "engine_instance_id", Value: "engine-1"}}
	NewController(objectInfoControllerService{}).GetComfyUIEngineObjectInfo(ctx)

	if recorder.Header().Get("Content-Encoding") != "gzip" || recorder.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatalf("missing gzip negotiation headers: %#v", recorder.Header())
	}
	reader, err := gzip.NewReader(recorder.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var response iapiserver.ComfyUIEngineObjectInfoResponse
	if err := json.NewDecoder(reader).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if !response.Available || response.ObjectInfo["KSampler"] == nil {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestAcceptsGzipHonorsQualityZero(t *testing.T) {
	if acceptsGzip("br, gzip;q=0") {
		t.Fatal("gzip with q=0 was accepted")
	}
	if !acceptsGzip("br, gzip;q=0.8") {
		t.Fatal("gzip with positive quality was rejected")
	}
}
