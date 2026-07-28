package apiserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/options"
	appsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	tasksvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	workflowcanvassvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/workflowcanvas"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
)

type routeContractService struct{ appsvc.ApplicationPlatformSrv }
type routeTaskCenterService struct{ tasksvc.TaskCenterSrv }
type routeWorkflowCanvasService struct{ workflowcanvassvc.Service }

type routeAuthenticationFactory struct {
	store.Factory
	users      store.UserStore
	userEvents store.UserEventStore
}

func (f routeAuthenticationFactory) Users() store.UserStore           { return f.users }
func (f routeAuthenticationFactory) UserEvents() store.UserEventStore { return f.userEvents }

type routeUserEventStore struct{ store.UserEventStore }

type routeApplicationService struct {
	appsvc.ApplicationPlatformSrv
	userID string
}

func (s *routeApplicationService) ListApplications(ctx context.Context, _ *iapiserver.ApplicationListRequest) (*iapiserver.ApplicationListResponse, error) {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err != nil {
		return nil, err
	}
	s.userID = user.ID
	return &iapiserver.ApplicationListResponse{Items: []*iapiserver.Application{}}, nil
}

func TestApplicationPlatformRoutesMatchSSOTOpenAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	installApplicationPlatformApis(router.Group("/api/v1"), routeContractService{})

	actual := map[string]struct{}{}
	for _, route := range router.Routes() {
		actual[route.Method+" "+route.Path] = struct{}{}
	}
	expected := readApplicationPlatformOperations(t)
	if difference := operationDifference(expected, actual); len(difference) != 0 {
		t.Fatalf("Application Platform route contract differs from SSOT OpenAPI:\n%s", strings.Join(difference, "\n"))
	}
	if len(actual) != 53 {
		t.Fatalf("expected exactly 53 S2 operations, got %d", len(actual))
	}
}

func TestTaskCenterRoutesMatchSSOTOpenAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	installTaskCenterApis(router.Group("/api/v1"), routeTaskCenterService{})
	assertRoutesMatchOpenAPI(t, router, filepath.Join("..", "..", "..", "ssot", "01_contracts", "domains", "task-center", "openapi.yaml"))
}

func TestSSERoutesMatchSSOTOpenAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	installSSEApis(router.Group("/api/v1"), routeAuthenticationFactory{userEvents: routeUserEventStore{}}, options.NewSSEOptions())
	assertRoutesMatchOpenAPI(t, router, filepath.Join("..", "..", "..", "ssot", "01_contracts", "domains", "sse", "openapi.yaml"))
}
func TestWorkflowCanvasRoutesMatchSSOTOpenAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	installCanvasApis(router.Group("/api/v1"), routeWorkflowCanvasService{})
	assertRoutesMatchOpenAPI(t, router, filepath.Join("..", "..", "..", "ssot", "01_contracts", "domains", "workflow-canvas", "openapi.yaml"))
}

func assertRoutesMatchOpenAPI(t *testing.T, router *gin.Engine, source string) {
	t.Helper()
	actual := map[string]struct{}{}
	for _, route := range router.Routes() {
		actual[route.Method+" "+route.Path] = struct{}{}
	}
	expected := readOperations(t, source)
	if difference := operationDifference(expected, actual); len(difference) != 0 {
		t.Fatalf("route contract differs from SSOT OpenAPI:\n%s", strings.Join(difference, "\n"))
	}
}

func TestApplicationPlatformAllowsAnonymousDevelopmentPrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousStore := store.Client()
	store.SetClient(routeAuthenticationFactory{})
	t.Cleanup(func() { store.SetClient(previousStore) })

	service := &routeApplicationService{}
	router := gin.New()
	installApis(router, service, nil, &options.AuthOptions{AllowAnonymousDevelopment: true}, options.NewSSEOptions(), "debug")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/applications?page_num=0&page_size=20", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if service.userID != "system-admin" {
		t.Fatalf("service principal = %q, want system-admin; body=%s", service.userID, recorder.Body.String())
	}
}

func readApplicationPlatformOperations(t *testing.T) map[string]struct{} {
	t.Helper()
	source := filepath.Join("..", "..", "..", "ssot", "01_contracts", "domains", "application-platform", "openapi.yaml")
	return readOperations(t, source)
}

func readOperations(t *testing.T, source string) map[string]struct{} {
	t.Helper()
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	operations := map[string]struct{}{}
	for openAPIPath, pathItem := range document.Paths {
		ginPath := openAPIPath
		for {
			start := strings.Index(ginPath, "{")
			if start < 0 {
				break
			}
			end := strings.Index(ginPath[start:], "}")
			if end < 0 {
				t.Fatalf("invalid OpenAPI path parameter: %s", openAPIPath)
			}
			end += start
			ginPath = ginPath[:start] + ":" + ginPath[start+1:end] + ginPath[end+1:]
		}
		for method := range pathItem {
			switch strings.ToUpper(method) {
			case "GET", "POST", "PATCH", "PUT", "DELETE":
				operations[strings.ToUpper(method)+" "+ginPath] = struct{}{}
			}
		}
	}
	return operations
}

func operationDifference(expected, actual map[string]struct{}) []string {
	difference := []string{}
	for operation := range expected {
		if _, ok := actual[operation]; !ok {
			difference = append(difference, "missing: "+operation)
		}
	}
	for operation := range actual {
		if _, ok := expected[operation]; !ok {
			difference = append(difference, "extra: "+operation)
		}
	}
	sort.Strings(difference)
	return difference
}
