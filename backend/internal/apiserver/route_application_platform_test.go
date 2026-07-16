package apiserver

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	appsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
)

type routeContractService struct{ appsvc.ApplicationPlatformSrv }

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
	if len(actual) != 32 {
		t.Fatalf("expected exactly 32 S2 operations, got %d", len(actual))
	}
}

func readApplicationPlatformOperations(t *testing.T) map[string]struct{} {
	t.Helper()
	source := filepath.Join("..", "..", "..", "ssot", "01_contracts", "domains", "application-platform", "openapi.yaml")
	raw, err := os.ReadFile(source)
	if err != nil {
		 t.Fatal(err)
	}
	serverCopy, err := os.ReadFile(filepath.Join("..", "..", "..", "api", "swagger", "application-platform.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, serverCopy) {
		t.Fatal("api/swagger/application-platform.yaml is not synchronized with SSOT OpenAPI")
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
