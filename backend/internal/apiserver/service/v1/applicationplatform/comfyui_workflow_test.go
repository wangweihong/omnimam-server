package applicationplatform

import (
	"context"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
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
	engine           *iapiserver.EngineInstance
	catalog          *iapiserver.ComfyUIEngineObjectInfo
	duplicates       []string
	addedWorkflow    *iapiserver.ComfyUIWorkflow
	addedValidation  *iapiserver.ComfyUIWorkflowValidation
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
	return s.engine, nil
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
func (s *workflowStore) AddComfyUIWorkflowValidation(_ context.Context, validation *iapiserver.ComfyUIWorkflowValidation) (*iapiserver.ComfyUIWorkflowValidation, error) {
	validation.ID = "validation-new"
	s.addedValidation = validation
	return validation, nil
}
func (s *workflowStore) ConvertComfyUIWorkflow(_ context.Context, workflowID, _, _, _ string, template *iapiserver.ApplicationTemplate, version *iapiserver.ApplicationTemplateVersion) (*iapiserver.ComfyUIWorkflowConvertResult, error) {
	template.ID = "template-1"
	version.ID = "version-1"
	version.ApplicationTemplateID = template.ID
	version.Version = 1
	s.convertedVersion = version
	return &iapiserver.ComfyUIWorkflowConvertResult{WorkflowID: workflowID, ApplicationTemplate: template, ApplicationTemplateVersion: version, WorkflowContractRevision: *version.WorkflowContractRevision}, nil
}

func TestImportComfyUIWorkflowUsesServerObjectInfoAndOwnerDuplicates(t *testing.T) {
	runtime, err := appregistry.LoadRuntimeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	applicationStore := &workflowStore{engine: &iapiserver.EngineInstance{ApplicationEngineTypeID: "comfyui", Enabled: true, HealthStatus: iapiserver.EngineHealthOnline}, catalog: currentTestCatalog(), duplicates: []string{"workflow-old"}}
	applicationStore.engine.ID = "engine-1"
	service := &applicationPlatformService{Dependencies: Dependencies{Store: &executorFactory{applications: applicationStore}, Runtime: runtime, Principals: staticPrincipal{principal: Principal{UserID: "user-1"}}}}
	request := &iapiserver.ComfyUIWorkflowImportRequest{Name: "Workflow", SourceEngineInstanceID: "engine-1", APIWorkflow: map[string]any{"1": map[string]any{"class_type": "KSampler", "inputs": map[string]any{}}}, APIWorkflowRaw: []byte(`{"1":{"class_type":"KSampler","inputs":{}}}`)}
	result, err := service.ImportComfyUIWorkflow(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !result.DuplicateContent || len(result.DuplicateWorkflowIDs) != 1 || applicationStore.addedWorkflow == nil || applicationStore.addedWorkflow.OwnerUserID != "user-1" {
		t.Fatalf("unexpected import result=%#v workflow=%#v", result, applicationStore.addedWorkflow)
	}
}

func TestImportComfyUIWorkflowDoesNotPersistWhenObjectInfoFails(t *testing.T) {
	runtime, err := appregistry.LoadRuntimeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	applicationStore := &workflowStore{engine: &iapiserver.EngineInstance{ApplicationEngineTypeID: "comfyui", Enabled: true, HealthStatus: iapiserver.EngineHealthOnline}}
	applicationStore.engine.ID = "engine-1"
	service := &applicationPlatformService{Dependencies: Dependencies{Store: &executorFactory{applications: applicationStore}, Runtime: runtime, Principals: staticPrincipal{principal: Principal{UserID: "user-1"}}}}
	_, err = service.ImportComfyUIWorkflow(context.Background(), &iapiserver.ComfyUIWorkflowImportRequest{Name: "Workflow", SourceEngineInstanceID: "engine-1", APIWorkflow: map[string]any{"1": map[string]any{"class_type": "KSampler", "inputs": map[string]any{}}}})
	if err == nil {
		t.Fatal("object_info failure was accepted")
	}
	if applicationStore.addedWorkflow != nil {
		t.Fatal("workflow was persisted after object_info failure")
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
	validation := &iapiserver.ComfyUIWorkflowValidation{WorkflowID: workflow.ID, OwnerUserID: "user-1", EngineInstanceID: "engine-1", Status: iapiserver.ComfyUIValidationCompatible}
	validation.ID = "validation-1"
	engine := &iapiserver.EngineInstance{ApplicationEngineTypeID: "comfyui", Enabled: true, HealthStatus: iapiserver.EngineHealthOnline}
	engine.ID = "engine-1"
	applicationStore := &workflowStore{workflow: workflow, validation: validation, engine: engine, catalog: saveImageTestCatalog()}
	service := &applicationPlatformService{Dependencies: Dependencies{Store: &executorFactory{applications: applicationStore}, Runtime: runtime, Principals: staticPrincipal{principal: Principal{UserID: "user-1"}}, Events: NoopEventPublisher{}}}
	request := &iapiserver.ComfyUIWorkflowConvertRequest{Name: "Template", CapabilityDefinitionID: "image.text_to_image", WorkflowValidationID: validation.ID, IdempotencyKey: "convert-1", TemplateContract: map[string]any{"inputs": []any{}, "fixed_parameters": []any{}, "parameter_mappings": []any{}, "outputs": []any{map[string]any{"key": "image", "node_id": "1", "output_index": float64(0), "data_type": "IMAGE", "media_type": "image"}}, "engine_restrictions": map[string]any{}}}
	result, err := service.ConvertComfyUIWorkflow(context.Background(), workflow.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	if result.ApplicationTemplateVersion.Version != 1 {
		t.Fatalf("unexpected version: %d", result.ApplicationTemplateVersion.Version)
	}
	version := applicationStore.convertedVersion
	if version.SourceComfyUIWorkflowID == nil || *version.SourceComfyUIWorkflowID != workflow.ID || version.SourceWorkflowValidationID == nil || *version.SourceWorkflowValidationID != validation.ID {
		t.Fatalf("source relationship missing: %#v", version)
	}
	if version.WorkflowContractRevision == nil {
		t.Fatalf("immutable snapshot incomplete: %#v", version)
	}
}

func currentTestCatalog() *iapiserver.ComfyUIEngineObjectInfo {
	return &iapiserver.ComfyUIEngineObjectInfo{ObjectInfo: map[string]any{"KSampler": map[string]any{"input": map[string]any{}, "output": []any{}}}, RefreshedAt: imachinery.Now()}
}

func saveImageTestCatalog() *iapiserver.ComfyUIEngineObjectInfo {
	return &iapiserver.ComfyUIEngineObjectInfo{ObjectInfo: map[string]any{"SaveImage": map[string]any{"input": map[string]any{"optional": map[string]any{"images": []any{"IMAGE"}}}, "output": []any{"IMAGE"}, "output_node": true}}, RefreshedAt: imachinery.Now()}
}
