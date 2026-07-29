package workflowcanvas

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	applicationsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type applicationCatalogStore struct {
	store.WorkflowCanvasStore
	definition *iapiserver.WorkflowNodeDefinition
	calls      int
	err        error
}

type applicationDirectoryStore struct {
	store.WorkflowCanvasStore
	items []*iapiserver.WorkflowNodeDefinition
}

func (s *applicationDirectoryStore) ListWorkflowNodeDefinitions(
	context.Context,
	*iapiserver.WorkflowNodeDefinitionListRequest,
	string,
	string,
) ([]*iapiserver.WorkflowNodeDefinition, int64, error) {
	return s.items, int64(len(s.items)), nil
}

type batchApplicationResolver struct {
	results  []applicationsvc.CanvasApplicationVersionResult
	requests []applicationsvc.CanvasApplicationVersionRequest
	calls    int
}

func (r *batchApplicationResolver) ResolveCanvasApplicationVersions(
	_ context.Context,
	requests []applicationsvc.CanvasApplicationVersionRequest,
) []applicationsvc.CanvasApplicationVersionResult {
	r.calls++
	r.requests = requests
	return r.results
}

func (s *applicationCatalogStore) AddWorkflowNodeDefinitionIdempotent(
	_ context.Context,
	definition *iapiserver.WorkflowNodeDefinition,
) (*iapiserver.WorkflowNodeDefinition, bool, error) {
	s.calls++
	s.definition = definition
	return definition, s.calls == 1, s.err
}

func TestApplicationNodeDefinitionPreservesExecutionBindingAndPorts(t *testing.T) {
	definition, err := applicationNodeDefinition(applicationVersionPublicationFixture())
	if err != nil {
		t.Fatal(err)
	}
	if definition.ExecutionBinding.Mode != iapiserver.CanvasExecutionAtomic ||
		definition.ExecutionBinding.ApplicationVersionID == nil ||
		*definition.ExecutionBinding.ApplicationVersionID != "version-1" ||
		definition.ExecutionBinding.BindingVersion != "1.2.3" {
		t.Fatalf("execution binding = %#v", definition.ExecutionBinding)
	}
	if len(definition.Ports) != 2 || definition.Ports[0].Key != "prompt" ||
		definition.Ports[0].Direction != "input" ||
		definition.Ports[1].Key != "image" ||
		definition.Ports[1].Direction != "output" ||
		definition.Ports[1].DataType != "image" ||
		!definition.Ports[1].RequiredForCompletion {
		t.Fatalf("ports = %#v", definition.Ports)
	}
}

func TestApplicationCatalogRejectsLossySchemaAsDiagnostic(t *testing.T) {
	event := applicationVersionPublicationFixture()
	event.InputSchema["properties"] = map[string]any{
		"choice": map[string]any{"oneOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "number"}}},
	}
	target := &applicationCatalogStore{}
	err := NewApplicationCatalogProjector(target).ProjectPublication(t.Context(), event)
	var diagnostic *ApplicationCatalogDiagnosticError
	if !stderrors.As(err, &diagnostic) {
		t.Fatalf("error = %v, want catalog diagnostic", err)
	}
	if target.calls != 0 {
		t.Fatalf("lossy schema was registered: calls=%d", target.calls)
	}
}

func TestListNodeDefinitionsUsesOneBatchRealtimeApplicationValidation(t *testing.T) {
	event := applicationVersionPublicationFixture()
	definition, err := applicationNodeDefinition(event)
	if err != nil {
		t.Fatal(err)
	}
	definition.ID = "definition-1"
	definition.Name = "Application"
	definition.ApplicationVersionID = definition.ExecutionBinding.ApplicationVersionID
	resolver := &batchApplicationResolver{results: []applicationsvc.CanvasApplicationVersionResult{{
		Key: definition.ID,
		Resolved: &applicationsvc.CanvasApplicationVersion{
			Application: &iapiserver.Application{
				OwnerUserID: event.OwnerUserID,
			},
			Version: &iapiserver.ApplicationVersion{
				ApplicationID:   event.ApplicationID,
				SemanticVersion: event.SemanticVersion,
				InputSchema:     event.InputSchema,
				OutputSchema:    event.OutputSchema,
			},
		},
	}}}
	resolver.results[0].Resolved.Application.ID = event.ApplicationID
	resolver.results[0].Resolved.Version.ID = event.ApplicationVersionID
	service := &service{
		store:        &applicationDirectoryStore{items: []*iapiserver.WorkflowNodeDefinition{definition}},
		applications: resolver,
	}
	response, err := service.ListNodeDefinitions(t.Context(), &iapiserver.WorkflowNodeDefinitionListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resolver.calls != 1 || len(resolver.requests) != 1 ||
		resolver.requests[0].ApplicationVersionID != event.ApplicationVersionID ||
		len(response.Items) != 1 {
		t.Fatalf("calls=%d requests=%#v response=%#v", resolver.calls, resolver.requests, response)
	}
}

func applicationVersionPublicationFixture() ApplicationVersionPublication {
	return ApplicationVersionPublication{
		ApplicationID:                "application-1",
		ApplicationVersionID:         "version-1",
		ApplicationTemplateVersionID: "template-version-1",
		SemanticVersion:              "1.2.3",
		ApplicationName:              "Image generator",
		OwnerUserID:                  "user-1",
		Visibility:                   iapiserver.ApplicationVisibilityPrivate,
		CanvasEnabled:                true,
		RunEnabled:                   true,
		InputSchema: map[string]any{
			"type":       "object",
			"required":   []any{"prompt"},
			"properties": map[string]any{"prompt": map[string]any{"type": "string"}},
		},
		OutputSchema: map[string]any{
			"type":       "object",
			"required":   []any{"image"},
			"properties": map[string]any{"image": map[string]any{"type": "string", "format": "asset_image"}},
		},
	}
}
