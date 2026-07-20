package apiserver

import (
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type assetContractRouteFactory struct {
	store.Factory
	assetStore store.AssetV1Store
}

func (f assetContractRouteFactory) AssetsV1() store.AssetV1Store { return f.assetStore }

func TestAssetLibraryRoutesCoverReleasedOpenAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api/v1")
	factory := assetContractRouteFactory{assetStore: routeAssetStore{}}
	installPlatformApis(group, factory, nil)
	installAssetLibraryContractApis(group, factory)

	actual := make(map[string]struct{})
	for _, route := range router.Routes() {
		actual[route.Method+" "+route.Path] = struct{}{}
	}
	expected := []string{
		"GET /api/v1/assets", "POST /api/v1/assets", "GET /api/v1/assets/:asset_id", "PATCH /api/v1/assets/:asset_id", "DELETE /api/v1/assets/:asset_id",
		"POST /api/v1/assets/:asset_id/restore", "DELETE /api/v1/assets/:asset_id/permanent", "POST /api/v1/assets/batch-labels",
		"POST /api/v1/asset-uploads", "POST /api/v1/asset-uploads/:upload_id/content", "POST /api/v1/asset-uploads/:upload_id/complete", "DELETE /api/v1/asset-uploads/:upload_id",
		"GET /api/v1/collections", "POST /api/v1/collections", "GET /api/v1/collections/:collection_id", "PATCH /api/v1/collections/:collection_id", "DELETE /api/v1/collections/:collection_id",
		"POST /api/v1/collections/:collection_id/items", "PATCH /api/v1/collections/:collection_id/items/:item_id", "DELETE /api/v1/collections/:collection_id/items/:item_id",
		"PUT /api/v1/assets/:asset_id/labels", "DELETE /api/v1/assets/:asset_id/labels/:label_id", "POST /api/v1/assets/:asset_id/tags", "DELETE /api/v1/assets/:asset_id/tags/:tag_id",
		"GET /api/v1/artifacts", "POST /api/v1/artifacts", "GET /api/v1/artifacts/:artifact_id", "DELETE /api/v1/artifacts/:artifact_id", "POST /api/v1/artifacts/:artifact_id/content", "POST /api/v1/artifacts/:artifact_id/complete", "POST /api/v1/artifacts/:artifact_id/register", "POST /api/v1/artifact-registrations",
		"GET /api/v1/assets/:asset_id/versions", "POST /api/v1/assets/:asset_id/versions", "GET /api/v1/asset-versions/:version_id", "POST /api/v1/assets/:asset_id/versions/:version_id/set-current",
		"GET /api/v1/asset-versions/:version_id/representations", "POST /api/v1/asset-versions/:version_id/representations",
		"GET /api/v1/asset-representations/:representation_id", "GET /api/v1/asset-representations/:representation_id/content", "GET /api/v1/asset-representations/:representation_id/access-url",
		"GET /api/v1/assets/:asset_id/relations", "GET /api/v1/assets/:asset_id/lineage", "GET /api/v1/assets/:asset_id/references", "GET /api/v1/assets/:asset_id/usages",
	}
	if len(expected) != 45 {
		t.Fatalf("test fixture has %d operations, want 45", len(expected))
	}
	for _, route := range expected {
		if _, ok := actual[route]; !ok {
			t.Errorf("released route is not installed: %s", route)
		}
	}
}

type routeAssetStore struct{ store.AssetV1Store }
