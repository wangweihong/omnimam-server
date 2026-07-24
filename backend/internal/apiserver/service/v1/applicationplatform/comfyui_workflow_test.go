package applicationplatform

import (
	"context"
	stderrors "errors"
	"fmt"
	"testing"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"gorm.io/gorm"
)

type staticPrincipal struct{ principal Principal }

func (r staticPrincipal) Resolve(context.Context) (Principal, error) { return r.principal, nil }

type recordingWorkflowAuditor struct{ records []WorkflowAuditRecord }

func (a *recordingWorkflowAuditor) Record(_ context.Context, record WorkflowAuditRecord) error {
	a.records = append(a.records, record)
	return nil
}

type workflowObjectInfoAdapter struct {
	objectInfo map[string]any
	err        error
}

func (workflowObjectInfoAdapter) ID() string { return "comfyui" }
func (workflowObjectInfoAdapter) Check(context.Context, *iapiserver.EngineInstance) (*iapiserver.EngineHealthCheckResult, error) {
	return nil, nil
}
func (a workflowObjectInfoAdapter) ReadObjectInfo(context.Context, *iapiserver.EngineInstance) (map[string]any, error) {
	return a.objectInfo, a.err
}

type workflowStore struct {
	store.ApplicationPlatformStore
	workflow         *iapiserver.ComfyUIWorkflow
	validation       *iapiserver.ComfyUIWorkflowValidation
	convertedVersion *iapiserver.ApplicationTemplateVersion
	convertCalls     int
	engine           *iapiserver.EngineInstance
	engineErr        error
	catalog          *iapiserver.ComfyUIEngineObjectInfo
	duplicates       []string
	addedWorkflow    *iapiserver.ComfyUIWorkflow
	addedValidation  *iapiserver.ComfyUIWorkflowValidation
	updatedWorkflow  *iapiserver.ComfyUIWorkflow
	updatedVersion   int64
	updateErr        error
	conversionResult *iapiserver.ComfyUIWorkflowConvertResult
}

type recordingWorkflowParser struct {
	api   map[string]any
	err   error
	calls int
}

func (p *recordingWorkflowParser) VisualToAPI(map[string]any, map[string]any) (map[string]any, error) {
	p.calls++
	return p.api, p.err
}

func TestManagedWorkflowReadRecordsActorAndOwner(t *testing.T) {
	workflow := &iapiserver.ComfyUIWorkflow{OwnerUserID: "owner-1"}
	workflow.ID = "workflow-1"
	auditor := &recordingWorkflowAuditor{}
	service := &applicationPlatformService{Dependencies: Dependencies{Store: &executorFactory{applications: &workflowStore{workflow: workflow}}, Principals: staticPrincipal{principal: Principal{UserID: "admin-1", Admin: true}}, WorkflowAudit: auditor}}
	if _, err := service.GetComfyUIWorkflow(context.Background(), workflow.ID); err != nil {
		t.Fatal(err)
	}
	if len(auditor.records) != 1 || auditor.records[0].Action != "read" || auditor.records[0].ActorUserID != "admin-1" || auditor.records[0].OwnerUserID != "owner-1" {
		t.Fatalf("unexpected audit records: %#v", auditor.records)
	}
}

func (s *workflowStore) GetComfyUIWorkflow(context.Context, string) (*iapiserver.ComfyUIWorkflow, error) {
	return s.workflow, nil
}
func (s *workflowStore) GetComfyUIWorkflowValidation(context.Context, string) (*iapiserver.ComfyUIWorkflowValidation, error) {
	return s.validation, nil
}
func (s *workflowStore) GetEngineInstance(context.Context, string) (*iapiserver.EngineInstance, error) {
	return s.engine, s.engineErr
}
func (s *workflowStore) GetComfyUIEngineObjectInfo(context.Context, string) (*iapiserver.ComfyUIEngineObjectInfo, error) {
	if s.catalog == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.catalog, nil
}
func (s *workflowStore) WithEngineInstanceLock(_ context.Context, _ string, action func() error) error {
	return action()
}
func (s *workflowStore) ListComfyUIWorkflowDuplicateIDs(context.Context, string, string) ([]string, error) {
	return s.duplicates, nil
}
func (s *workflowStore) AddComfyUIWorkflow(_ context.Context, workflow *iapiserver.ComfyUIWorkflow) (*iapiserver.ComfyUIWorkflow, error) {
	workflow.ID = "workflow-new"
	s.addedWorkflow = workflow
	return workflow, nil
}
func (s *workflowStore) UpdateComfyUIWorkflow(_ context.Context, workflow *iapiserver.ComfyUIWorkflow, expected int64) (*iapiserver.ComfyUIWorkflow, error) {
	s.updatedWorkflow = workflow
	s.updatedVersion = expected
	return workflow, s.updateErr
}
func (s *workflowStore) AddComfyUIWorkflowValidation(_ context.Context, validation *iapiserver.ComfyUIWorkflowValidation) (*iapiserver.ComfyUIWorkflowValidation, error) {
	validation.ID = "validation-new"
	s.addedValidation = validation
	return validation, nil
}
func (s *workflowStore) GetComfyUIWorkflowConversion(context.Context, string, string, string) (*iapiserver.ComfyUIWorkflowConvertResult, error) {
	if s.conversionResult == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.conversionResult, nil
}
func (s *workflowStore) ConvertComfyUIWorkflow(_ context.Context, workflowID, _, _ string, template *iapiserver.ApplicationTemplate, version *iapiserver.ApplicationTemplateVersion) (*iapiserver.ComfyUIWorkflowConvertResult, error) {
	s.convertCalls++
	template.ID = fmt.Sprintf("template-%d", s.convertCalls)
	version.ID = fmt.Sprintf("version-%d", s.convertCalls)
	version.ApplicationTemplateID = template.ID
	version.Version = 1
	s.convertedVersion = version
	return &iapiserver.ComfyUIWorkflowConvertResult{WorkflowID: workflowID, ApplicationTemplate: template, ApplicationTemplateVersion: version, WorkflowContractRevision: *version.WorkflowContractRevision}, nil
}

func TestImportComfyUIAPIWorkflowDoesNotRequireEngineAndReportsOwnerDuplicates(t *testing.T) {
	runtime, err := appregistry.LoadRuntimeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	applicationStore := &workflowStore{duplicates: []string{"workflow-old"}}
	service := &applicationPlatformService{Dependencies: Dependencies{Store: &executorFactory{applications: applicationStore}, Runtime: runtime, Principals: staticPrincipal{principal: Principal{UserID: "user-1"}}}}
	request := &iapiserver.ComfyUIWorkflowImportRequest{Name: "Workflow", APIWorkflow: map[string]any{"1": map[string]any{"class_type": "KSampler", "inputs": map[string]any{}}}, APIWorkflowRaw: []byte(`{"1":{"class_type":"KSampler","inputs":{}}}`)}
	result, err := service.ImportComfyUIWorkflow(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !result.DuplicateContent || len(result.DuplicateWorkflowIDs) != 1 || applicationStore.addedWorkflow == nil || applicationStore.addedWorkflow.OwnerUserID != "user-1" || applicationStore.addedWorkflow.APIConversionStatus != iapiserver.ComfyUIAPIConversionReady {
		t.Fatalf("unexpected import result=%#v workflow=%#v", result, applicationStore.addedWorkflow)
	}
}

func TestImportComfyUIVisualWorkflowStaysPendingWithoutParserOrEngine(t *testing.T) {
	runtime, err := appregistry.LoadRuntimeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	applicationStore := &workflowStore{}
	parser := &recordingWorkflowParser{err: stderrors.New("parser must not be called during import")}
	service := &applicationPlatformService{Dependencies: Dependencies{Store: &executorFactory{applications: applicationStore}, Runtime: runtime, Principals: staticPrincipal{principal: Principal{UserID: "user-1"}}, WorkflowParser: parser}}
	visual := map[string]any{"nodes": []any{}, "links": []any{}}
	_, err = service.ImportComfyUIWorkflow(context.Background(), &iapiserver.ComfyUIWorkflowImportRequest{Name: "Workflow", SourceWorkflow: visual, SourceWorkflowRaw: []byte(`{"nodes":[],"links":[]}`)})
	if err != nil {
		t.Fatal(err)
	}
	if parser.calls != 0 || applicationStore.addedWorkflow == nil || applicationStore.addedWorkflow.APIConversionStatus != iapiserver.ComfyUIAPIConversionPending || applicationStore.addedWorkflow.APIWorkflow != nil || applicationStore.addedWorkflow.APIWorkflowChecksum != nil {
		t.Fatalf("visual import crossed the conversion boundary: calls=%d workflow=%#v", parser.calls, applicationStore.addedWorkflow)
	}
}

func TestImportComfyUIWorkflowRejectsInvalidAPIStructure(t *testing.T) {
	applicationStore := &workflowStore{}
	service := &applicationPlatformService{Dependencies: Dependencies{Store: &executorFactory{applications: applicationStore}, Principals: staticPrincipal{principal: Principal{UserID: "user-1"}}}}
	_, err := service.ImportComfyUIWorkflow(context.Background(), &iapiserver.ComfyUIWorkflowImportRequest{Name: "Workflow", APIWorkflow: map[string]any{"1": map[string]any{"class_type": "KSampler"}}})
	if errors.ToStatus(err).Code != code.ErrAIAppComfyUIWorkflowSourceInvalid || applicationStore.addedWorkflow != nil {
		t.Fatalf("invalid API workflow was accepted: err=%v workflow=%#v", err, applicationStore.addedWorkflow)
	}
}

func TestConvertComfyUIWorkflowToAPIUsesRequestedEngine(t *testing.T) {
	workflow := &iapiserver.ComfyUIWorkflow{OwnerUserID: "user-1", SourceType: iapiserver.ComfyUIWorkflowSourceVisual, APIConversionStatus: iapiserver.ComfyUIAPIConversionPending, VisualWorkflow: map[string]any{"nodes": []any{}, "links": []any{}}}
	workflow.ID = "workflow-1"
	engine := &iapiserver.EngineInstance{ApplicationEngineTypeID: "comfyui", Enabled: true, HealthStatus: iapiserver.EngineHealthOnline}
	engine.ID = "engine-requested"
	applicationStore := &workflowStore{workflow: workflow, engine: engine, catalog: currentTestCatalog()}
	parser := &recordingWorkflowParser{api: map[string]any{"1": map[string]any{"class_type": "KSampler", "inputs": map[string]any{}}}}
	service := &applicationPlatformService{Dependencies: Dependencies{Store: &executorFactory{applications: applicationStore}, Principals: staticPrincipal{principal: Principal{UserID: "user-1"}}, WorkflowParser: parser}}
	result, err := service.ConvertComfyUIWorkflowToAPI(context.Background(), workflow.ID, &iapiserver.ComfyUIWorkflowAPIConversionRequest{EngineInstanceID: engine.ID, ResourceVersion: 7})
	if err != nil {
		t.Fatal(err)
	}
	if parser.calls != 1 || result.APIConversionStatus != iapiserver.ComfyUIAPIConversionReady || applicationStore.updatedWorkflow == nil || applicationStore.updatedVersion != 7 || applicationStore.updatedWorkflow.APIWorkflowChecksum == nil {
		t.Fatalf("unexpected conversion: calls=%d result=%#v store=%#v", parser.calls, result, applicationStore)
	}
}

func TestConvertComfyUIWorkflowToAPIPropagatesResourceVersionConflict(t *testing.T) {
	workflow := &iapiserver.ComfyUIWorkflow{OwnerUserID: "user-1", SourceType: iapiserver.ComfyUIWorkflowSourceVisual, APIConversionStatus: iapiserver.ComfyUIAPIConversionPending, VisualWorkflow: map[string]any{"nodes": []any{}, "links": []any{}}}
	workflow.ID = "workflow-1"
	engine := &iapiserver.EngineInstance{ApplicationEngineTypeID: "comfyui", Enabled: true, HealthStatus: iapiserver.EngineHealthOnline}
	engine.ID = "engine-1"
	applicationStore := &workflowStore{workflow: workflow, engine: engine, catalog: currentTestCatalog(), updateErr: errors.NewStatus(code.ErrAIAppComfyUIResourceVersionConflict, "version changed")}
	parser := &recordingWorkflowParser{api: map[string]any{"1": map[string]any{"class_type": "KSampler", "inputs": map[string]any{}}}}
	service := &applicationPlatformService{Dependencies: Dependencies{Store: &executorFactory{applications: applicationStore}, Principals: staticPrincipal{principal: Principal{UserID: "user-1"}}, WorkflowParser: parser}}
	result, err := service.ConvertComfyUIWorkflowToAPI(context.Background(), workflow.ID, &iapiserver.ComfyUIWorkflowAPIConversionRequest{EngineInstanceID: engine.ID, ResourceVersion: 9})
	if result != nil || errors.ToStatus(err).Code != code.ErrAIAppComfyUIResourceVersionConflict || applicationStore.updatedVersion != 9 {
		t.Fatalf("conflicting conversion result=%#v err=%v store=%#v", result, err, applicationStore)
	}
}

func TestConvertComfyUIWorkflowToAPIRejectsUnavailableEngineAndParserFailure(t *testing.T) {
	tests := []struct {
		name      string
		engine    *iapiserver.EngineInstance
		engineErr error
		catalog   *iapiserver.ComfyUIEngineObjectInfo
		parserAPI map[string]any
		parserErr error
		expected  int
	}{
		{name: "missing engine", engine: &iapiserver.EngineInstance{}, engineErr: gorm.ErrRecordNotFound, expected: code.ErrAIAppEngineInstanceNotFound},
		{name: "wrong engine type", engine: &iapiserver.EngineInstance{ApplicationEngineTypeID: "deepseek_official", Enabled: true, HealthStatus: iapiserver.EngineHealthOnline}, catalog: currentTestCatalog(), expected: code.ErrAIAppComfyUIEngineTypeInvalid},
		{name: "disabled engine", engine: &iapiserver.EngineInstance{ApplicationEngineTypeID: "comfyui", HealthStatus: iapiserver.EngineHealthOnline}, catalog: currentTestCatalog(), expected: code.ErrAIAppComfyUIObjectInfoUnavailable},
		{name: "offline engine", engine: &iapiserver.EngineInstance{ApplicationEngineTypeID: "comfyui", Enabled: true, HealthStatus: iapiserver.EngineHealthOffline}, catalog: currentTestCatalog(), expected: code.ErrAIAppComfyUIObjectInfoUnavailable},
		{name: "missing catalog", engine: &iapiserver.EngineInstance{ApplicationEngineTypeID: "comfyui", Enabled: true, HealthStatus: iapiserver.EngineHealthOnline}, expected: code.ErrAIAppComfyUIObjectInfoUnavailable},
		{name: "stale catalog", engine: &iapiserver.EngineInstance{ApplicationEngineTypeID: "comfyui", Enabled: true, HealthStatus: iapiserver.EngineHealthOnline}, catalog: &iapiserver.ComfyUIEngineObjectInfo{ObjectInfo: currentTestCatalog().ObjectInfo, RefreshedAt: imachinery.NewTime(time.Now().Add(-49 * time.Hour))}, expected: code.ErrAIAppComfyUIObjectInfoUnavailable},
		{name: "parser failure", engine: &iapiserver.EngineInstance{ApplicationEngineTypeID: "comfyui", Enabled: true, HealthStatus: iapiserver.EngineHealthOnline}, catalog: currentTestCatalog(), parserErr: stderrors.New("cannot convert"), expected: code.ErrAIAppComfyUIAPIConversionBlocked},
		{name: "generated API invalid", engine: &iapiserver.EngineInstance{ApplicationEngineTypeID: "comfyui", Enabled: true, HealthStatus: iapiserver.EngineHealthOnline}, catalog: currentTestCatalog(), parserAPI: map[string]any{"1": map[string]any{"class_type": "KSampler"}}, expected: code.ErrAIAppComfyUIWorkflowFileInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.engine.ID = "engine-1"
			workflow := &iapiserver.ComfyUIWorkflow{OwnerUserID: "user-1", SourceType: iapiserver.ComfyUIWorkflowSourceVisual, APIConversionStatus: iapiserver.ComfyUIAPIConversionPending, VisualWorkflow: map[string]any{"nodes": []any{}, "links": []any{}}}
			workflow.ID = "workflow-1"
			applicationStore := &workflowStore{workflow: workflow, engine: tt.engine, engineErr: tt.engineErr, catalog: tt.catalog}
			parserAPI := tt.parserAPI
			if parserAPI == nil {
				parserAPI = map[string]any{"1": map[string]any{"class_type": "KSampler", "inputs": map[string]any{}}}
			}
			parser := &recordingWorkflowParser{api: parserAPI, err: tt.parserErr}
			service := &applicationPlatformService{Dependencies: Dependencies{Store: &executorFactory{applications: applicationStore}, Principals: staticPrincipal{principal: Principal{UserID: "user-1"}}, WorkflowParser: parser}}
			_, err := service.ConvertComfyUIWorkflowToAPI(context.Background(), workflow.ID, &iapiserver.ComfyUIWorkflowAPIConversionRequest{EngineInstanceID: tt.engine.ID, ResourceVersion: 1})
			if errors.ToStatus(err).Code != tt.expected || applicationStore.updatedWorkflow != nil {
				t.Fatalf("conversion error=%v, want code=%d; update=%#v", err, tt.expected, applicationStore.updatedWorkflow)
			}
		})
	}
}

func TestValidateComfyUIWorkflowPersistsFailedHistory(t *testing.T) {
	runtime, err := appregistry.LoadRuntimeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	workflow := &iapiserver.ComfyUIWorkflow{OwnerUserID: "user-1", APIConversionStatus: iapiserver.ComfyUIAPIConversionReady, APIWorkflow: map[string]any{"1": map[string]any{"class_type": "KSampler", "inputs": map[string]any{}}}}
	workflow.ID = "workflow-1"
	applicationStore := &workflowStore{workflow: workflow, engine: &iapiserver.EngineInstance{ApplicationEngineTypeID: "comfyui", Enabled: true, HealthStatus: iapiserver.EngineHealthOnline}}
	applicationStore.engine.ID = "engine-1"
	service := &applicationPlatformService{Dependencies: Dependencies{Store: &executorFactory{applications: applicationStore}, Runtime: runtime, Principals: staticPrincipal{principal: Principal{UserID: "user-1"}}}}
	result, err := service.ValidateComfyUIWorkflow(context.Background(), workflow.ID, &iapiserver.ComfyUIWorkflowValidationCreateRequest{EngineInstanceID: "engine-1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != iapiserver.ComfyUIValidationFailed || applicationStore.addedValidation == nil || len(applicationStore.addedValidation.Errors) == 0 {
		t.Fatalf("failed validation was not persisted correctly: %#v", applicationStore.addedValidation)
	}
}

func TestConvertComfyUIWorkflowCreatesImmutableSourceSnapshot(t *testing.T) {
	runtime, err := appregistry.LoadRuntimeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	workflow := &iapiserver.ComfyUIWorkflow{OwnerUserID: "user-1", APIConversionStatus: iapiserver.ComfyUIAPIConversionReady, APIWorkflow: map[string]any{"1": map[string]any{"class_type": "SaveImage", "inputs": map[string]any{"images": "value"}}}}
	workflow.ID = "workflow-1"
	engine := &iapiserver.EngineInstance{ApplicationEngineTypeID: "comfyui", Enabled: true, HealthStatus: iapiserver.EngineHealthOnline}
	engine.ID = "engine-1"
	applicationStore := &workflowStore{workflow: workflow, engine: engine, catalog: saveImageTestCatalog()}
	service := &applicationPlatformService{Dependencies: Dependencies{Store: &executorFactory{applications: applicationStore}, Runtime: runtime, Principals: staticPrincipal{principal: Principal{UserID: "user-1"}}, Events: NoopEventPublisher{}}}
	request := &iapiserver.ComfyUIWorkflowConvertRequest{Name: "Template", EngineInstanceID: engine.ID, CapabilityDefinitionID: "image.text_to_image", IdempotencyKey: "convert-1", TemplateContract: map[string]any{"inputs": []any{}, "fixed_parameters": []any{}, "parameter_mappings": []any{}, "outputs": []any{map[string]any{"key": "image", "node_id": "1", "output_index": float64(0), "data_type": "IMAGE", "media_type": "image"}}, "engine_restrictions": map[string]any{}}}
	result, err := service.ConvertComfyUIWorkflow(context.Background(), workflow.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	if result.ApplicationTemplateVersion.Version != 1 {
		t.Fatalf("unexpected version: %d", result.ApplicationTemplateVersion.Version)
	}
	version := applicationStore.convertedVersion
	if version.SourceComfyUIWorkflowID == nil || *version.SourceComfyUIWorkflowID != workflow.ID {
		t.Fatalf("source relationship missing: %#v", version)
	}
	if version.WorkflowContractRevision == nil {
		t.Fatalf("immutable snapshot incomplete: %#v", version)
	}
	applicationStore.conversionResult = result
	engine.HealthStatus = iapiserver.EngineHealthOffline
	replayed, err := service.ConvertComfyUIWorkflow(context.Background(), workflow.ID, request)
	if err != nil || replayed.ApplicationTemplate.ID != result.ApplicationTemplate.ID || applicationStore.convertCalls != 1 {
		t.Fatalf("idempotent replay should bypass current engine state: result=%#v err=%v calls=%d", replayed, err, applicationStore.convertCalls)
	}
	applicationStore.conversionResult = nil
	engine.HealthStatus = iapiserver.EngineHealthOnline
	request.IdempotencyKey = "convert-2"
	if _, err := service.ConvertComfyUIWorkflow(context.Background(), workflow.ID, request); err != nil {
		t.Fatal(err)
	}
	if applicationStore.convertCalls != 2 {
		t.Fatalf("ready workflow should allow repeated conversion, calls=%d", applicationStore.convertCalls)
	}
}

func currentTestCatalog() *iapiserver.ComfyUIEngineObjectInfo {
	return &iapiserver.ComfyUIEngineObjectInfo{ObjectInfo: map[string]any{"KSampler": map[string]any{"input": map[string]any{}, "output": []any{}}}, RefreshedAt: imachinery.Now()}
}

func saveImageTestCatalog() *iapiserver.ComfyUIEngineObjectInfo {
	return &iapiserver.ComfyUIEngineObjectInfo{ObjectInfo: map[string]any{"SaveImage": map[string]any{"input": map[string]any{"optional": map[string]any{"images": []any{"IMAGE"}}}, "output": []any{"IMAGE"}, "output_node": true}}, RefreshedAt: imachinery.Now()}
}
