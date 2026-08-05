package apiserver

import (
	"testing"

	"github.com/gin-gonic/gin"
	usermodelsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/usermodel"
)

func TestInstallUserModelApisUsesCanonicalRoutesOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	installUserModelApis(engine.Group("/api/v1"), (*usermodelsvc.Service)(nil))

	want := map[string]struct{}{
		"GET /api/v1/user-model/provider-types": {},
		"GET /api/v1/user-model/providers":      {}, "POST /api/v1/user-model/providers": {},
		"POST /api/v1/user-model/providers/test": {}, "GET /api/v1/user-model/providers/:provider_id": {},
		"PATCH /api/v1/user-model/providers/:provider_id": {}, "DELETE /api/v1/user-model/providers/:provider_id": {},
		"POST /api/v1/user-model/providers/:provider_id/test":  {},
		"GET /api/v1/user-model/providers/:provider_id/models": {}, "POST /api/v1/user-model/providers/:provider_id/models": {},
		"POST /api/v1/user-model/providers/:provider_id/models/sync": {},
		"PATCH /api/v1/user-model/models/:model_id":                  {}, "DELETE /api/v1/user-model/models/:model_id": {},
		"POST /api/v1/user-model/models/:model_id/test": {},
		"GET /api/v1/user-model/defaults/:usage":        {}, "PUT /api/v1/user-model/defaults/:usage": {},
		"GET /api/v1/user-model/options": {},
	}
	for _, route := range engine.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; !ok {
			t.Fatalf("unexpected route %s", key)
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatalf("missing canonical routes: %#v", want)
	}
}
