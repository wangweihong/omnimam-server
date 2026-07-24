package applicationplatform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/gowebpki/jcs"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

func (s *applicationPlatformService) ListComfyUIWorkflows(ctx context.Context, req *iapiserver.ComfyUIWorkflowListRequest) (*iapiserver.ComfyUIWorkflowListResponse, error) {
	p, err := s.principal(ctx, false)
	if err != nil {
		return nil, err
	}
	if !p.Admin {
		req.OwnerUserID = p.UserID
	}
	items, total, err := s.Store.ApplicationPlatforms().ListComfyUIWorkflows(ctx, req)
	if err != nil {
		return nil, err
	}
	summaries := make([]*iapiserver.ComfyUIWorkflowSummary, 0, len(items))
	for _, item := range items {
		if err := s.auditManagedWorkflow(ctx, p, item, "list"); err != nil {
			return nil, err
		}
		summaries = append(summaries, item.Summary())
	}
	return &iapiserver.ComfyUIWorkflowListResponse{Total: total, Items: summaries}, nil
}

func (s *applicationPlatformService) ImportComfyUIWorkflow(ctx context.Context, req *iapiserver.ComfyUIWorkflowImportRequest) (*iapiserver.ComfyUIWorkflowImportResult, error) {
	p, err := s.principal(ctx, false)
	if err != nil {
		return nil, err
	}
	source := req.SourceWorkflow
	sourceRaw := req.SourceWorkflowRaw
	if source == nil {
		source, sourceRaw = req.APIWorkflow, req.APIWorkflowRaw
	}
	sourceType := detectComfyWorkflowSource(source)
	if sourceType == "" {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIWorkflowSourceInvalid, "JSON is neither a ComfyUI visual workflow nor API workflow")
	}
	apiWorkflow, visualWorkflow := source, req.VisualWorkflow
	conversionStatus := iapiserver.ComfyUIAPIConversionReady
	if sourceType == iapiserver.ComfyUIWorkflowSourceVisual {
		visualWorkflow = source
		apiWorkflow = nil
		conversionStatus = iapiserver.ComfyUIAPIConversionPending
	}
	sourceChecksum, err := canonicalJSONRawDigest(sourceRaw, source)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIWorkflowFileInvalid, err.Error())
	}
	duplicates, err := s.Store.ApplicationPlatforms().ListComfyUIWorkflowDuplicateIDs(ctx, p.UserID, sourceChecksum)
	if err != nil {
		return nil, err
	}
	var apiChecksum *string
	if conversionStatus == iapiserver.ComfyUIAPIConversionReady {
		digest, digestErr := canonicalJSONDigest(apiWorkflow)
		if digestErr != nil {
			return nil, errors.NewStatus(code.ErrAIAppComfyUIWorkflowFileInvalid, digestErr.Error())
		}
		apiChecksum = &digest
	}
	workflow := &iapiserver.ComfyUIWorkflow{OwnerUserID: p.UserID, CreatedByUserID: p.UserID, UpdatedByUserID: p.UserID, SourceType: sourceType, APIConversionStatus: conversionStatus, SourceChecksum: sourceChecksum, APIWorkflowChecksum: apiChecksum, APIWorkflow: apiWorkflow, VisualWorkflow: visualWorkflow}
	workflow.Name, workflow.Description = req.Name, req.Description
	created, err := s.Store.ApplicationPlatforms().AddComfyUIWorkflow(ctx, workflow)
	if err != nil {
		return nil, err
	}
	return &iapiserver.ComfyUIWorkflowImportResult{Workflow: created.Summary(), DuplicateContent: len(duplicates) > 0, DuplicateWorkflowIDs: duplicates}, nil
}

func (s *applicationPlatformService) GetComfyUIWorkflow(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflowDetail, error) {
	workflow, _, err := s.visibleComfyUIWorkflow(ctx, id, "read")
	if err != nil {
		return nil, err
	}
	return workflow.Detail(), nil
}

func (s *applicationPlatformService) ConvertComfyUIWorkflowToAPI(ctx context.Context, id string, req *iapiserver.ComfyUIWorkflowAPIConversionRequest) (*iapiserver.ComfyUIWorkflowDetail, error) {
	workflow, principal, err := s.visibleComfyUIWorkflow(ctx, id, "convert_to_api")
	if err != nil {
		return nil, err
	}
	if workflow.APIConversionStatus == iapiserver.ComfyUIAPIConversionReady {
		return workflow.Detail(), nil
	}
	if workflow.SourceType != iapiserver.ComfyUIWorkflowSourceVisual || len(workflow.VisualWorkflow) == 0 {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIWorkflowFileInvalid, "visual workflow source is unavailable")
	}
	_, catalog, err := s.usableComfyUIObjectInfo(ctx, req.EngineInstanceID)
	if err != nil {
		return nil, err
	}
	parser := s.WorkflowParser
	if parser == nil {
		parser = comfy2GoWorkflowParser{}
	}
	api, err := parser.VisualToAPI(workflow.VisualWorkflow, catalog.ObjectInfo)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIAPIConversionBlocked, err.Error())
	}
	if _, err := parseComfyUIWorkflow(api, workflow.VisualWorkflow, catalog.ObjectInfo); err != nil {
		return nil, err
	}
	digest, err := canonicalJSONDigest(api)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIWorkflowFileInvalid, err.Error())
	}
	workflow.APIWorkflow, workflow.APIWorkflowChecksum = api, &digest
	workflow.APIConversionStatus = iapiserver.ComfyUIAPIConversionReady
	workflow.UpdatedByUserID = principal.UserID
	updated, err := s.Store.ApplicationPlatforms().UpdateComfyUIWorkflow(ctx, workflow, req.ResourceVersion)
	if err != nil {
		return nil, err
	}
	return updated.Detail(), nil
}

func (s *applicationPlatformService) UpdateComfyUIWorkflow(ctx context.Context, req *iapiserver.ComfyUIWorkflowUpdateRequest) (*iapiserver.ComfyUIWorkflowSummary, error) {
	workflow, p, err := s.visibleComfyUIWorkflow(ctx, req.ID, "update_metadata")
	if err != nil {
		return nil, err
	}
	applyString(&workflow.Name, req.Name)
	applyString(&workflow.Description, req.Description)
	workflow.UpdatedByUserID = p.UserID
	updated, err := s.Store.ApplicationPlatforms().UpdateComfyUIWorkflow(ctx, workflow, req.ResourceVersion)
	if err != nil {
		return nil, err
	}
	return updated.Summary(), nil
}
func (s *applicationPlatformService) ListComfyUIWorkflowNodes(ctx context.Context, id string, req *iapiserver.ComfyUIWorkflowDeriveRequest) (*iapiserver.ComfyUIWorkflowNodeListResponse, error) {
	parsed, err := s.deriveComfyUIWorkflow(ctx, id, req.EngineInstanceID, "read_nodes")
	if err != nil {
		return nil, err
	}
	window, err := req.PagingParams.Normalize()
	if err != nil {
		return nil, err
	}
	return &iapiserver.ComfyUIWorkflowNodeListResponse{Total: len(parsed.nodes), Items: imachinery.PaginateSlice(parsed.nodes, window)}, nil
}
func (s *applicationPlatformService) ListComfyUIWorkflowInputCandidates(ctx context.Context, id, engineID string) (*iapiserver.ComfyUIWorkflowInputCandidateListResponse, error) {
	parsed, err := s.deriveComfyUIWorkflow(ctx, id, engineID, "read_input_candidates")
	if err != nil {
		return nil, err
	}
	return &iapiserver.ComfyUIWorkflowInputCandidateListResponse{Total: len(parsed.inputs), Items: parsed.inputs}, nil
}
func (s *applicationPlatformService) ListComfyUIWorkflowOutputCandidates(ctx context.Context, id, engineID string) (*iapiserver.ComfyUIWorkflowOutputCandidateListResponse, error) {
	parsed, err := s.deriveComfyUIWorkflow(ctx, id, engineID, "read_output_candidates")
	if err != nil {
		return nil, err
	}
	return &iapiserver.ComfyUIWorkflowOutputCandidateListResponse{Total: len(parsed.outputs), Items: parsed.outputs}, nil
}
func (s *applicationPlatformService) ListComfyUIWorkflowDependencies(ctx context.Context, id, engineID string) (*iapiserver.ComfyUIWorkflowDependencyListResponse, error) {
	parsed, err := s.deriveComfyUIWorkflow(ctx, id, engineID, "read_dependencies")
	if err != nil {
		return nil, err
	}
	return &iapiserver.ComfyUIWorkflowDependencyListResponse{Total: len(parsed.dependencies), Items: parsed.dependencies}, nil
}

func (s *applicationPlatformService) deriveComfyUIWorkflow(ctx context.Context, id, engineID, action string) (*parsedComfyUIWorkflow, error) {
	workflow, _, err := s.visibleComfyUIWorkflow(ctx, id, action)
	if err != nil {
		return nil, err
	}
	if workflow.APIConversionStatus != iapiserver.ComfyUIAPIConversionReady || len(workflow.APIWorkflow) == 0 {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIAPINotReady, "API workflow is not ready")
	}
	_, catalog, err := s.usableComfyUIObjectInfo(ctx, engineID)
	if err != nil {
		return nil, err
	}
	return parseComfyUIWorkflow(workflow.APIWorkflow, workflow.VisualWorkflow, catalog.ObjectInfo)
}

func (s *applicationPlatformService) ListComfyUIWorkflowValidations(ctx context.Context, req *iapiserver.ComfyUIWorkflowValidationListRequest) (*iapiserver.ComfyUIWorkflowValidationListResponse, error) {
	if _, _, err := s.visibleComfyUIWorkflow(ctx, req.WorkflowID, "read_validations"); err != nil {
		return nil, err
	}
	items, total, err := s.Store.ApplicationPlatforms().ListComfyUIWorkflowValidations(ctx, req)
	if err != nil {
		return nil, err
	}
	return &iapiserver.ComfyUIWorkflowValidationListResponse{Total: total, Items: items}, nil
}
func (s *applicationPlatformService) GetComfyUIWorkflowValidation(ctx context.Context, id string) (*iapiserver.ComfyUIWorkflowValidation, error) {
	validation, err := s.Store.ApplicationPlatforms().GetComfyUIWorkflowValidation(ctx, id)
	if err != nil {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIValidationNotFound, "validation not found")
	}
	if _, _, err := s.visibleComfyUIWorkflow(ctx, validation.WorkflowID, "read_validation"); err != nil {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIValidationNotFound, "validation not found")
	}
	return validation, nil
}

func (s *applicationPlatformService) ValidateComfyUIWorkflow(ctx context.Context, id string, req *iapiserver.ComfyUIWorkflowValidationCreateRequest) (*iapiserver.ComfyUIWorkflowValidation, error) {
	workflow, p, err := s.visibleComfyUIWorkflow(ctx, id, "validate")
	if err != nil {
		return nil, err
	}
	now := imachinery.Now()
	validation := &iapiserver.ComfyUIWorkflowValidation{WorkflowID: id, OwnerUserID: workflow.OwnerUserID, RequestedByUserID: p.UserID, EngineInstanceID: req.EngineInstanceID, ValidatedAt: now, NodeSummary: map[string]any{}, DependencySummary: map[string]any{}, Errors: []iapiserver.ComfyUIWorkflowDiagnostic{}, Warnings: []iapiserver.ComfyUIWorkflowDiagnostic{}}
	validation.Name = "Compatibility check for " + workflow.Name
	_, catalog, catalogErr := s.usableComfyUIObjectInfo(ctx, req.EngineInstanceID)
	if catalogErr != nil {
		if errors.ToStatus(catalogErr).Code != code.ErrAIAppComfyUIObjectInfoUnavailable {
			return nil, catalogErr
		}
		validation.Status = iapiserver.ComfyUIValidationFailed
		validation.Errors = []iapiserver.ComfyUIWorkflowDiagnostic{{Code: "OBJECT_INFO_UNAVAILABLE", Message: "current object_info is unavailable"}}
	} else {
		parsed, parseErr := parseComfyUIWorkflow(workflow.APIWorkflow, workflow.VisualWorkflow, catalog.ObjectInfo)
		if parseErr != nil {
			return nil, parseErr
		}
		workflow.ParsedNodes, workflow.Dependencies = parsed.nodes, parsed.dependencies
		validation.ComfyUIVersion = catalog.ComfyUIVersion
		validation.NodeSummary["total_nodes"] = len(parsed.nodes)
		validation.DependencySummary["total_dependencies"] = len(parsed.dependencies)
		validation.Errors = compatibilityDiagnostics(workflow, catalog.ObjectInfo)
		validation.NodeSummary["blocking_errors"] = len(validation.Errors)
		validation.NodeSummary["warnings"] = len(validation.Warnings)
		validation.DependencySummary["blocking_errors"] = dependencyDiagnosticCount(validation.Errors)
		if len(validation.Errors) > 0 {
			validation.Status = iapiserver.ComfyUIValidationIncompatible
		} else {
			validation.Status = iapiserver.ComfyUIValidationCompatible
		}
	}
	return s.Store.ApplicationPlatforms().AddComfyUIWorkflowValidation(ctx, validation)
}

func (s *applicationPlatformService) ConvertComfyUIWorkflow(ctx context.Context, id string, req *iapiserver.ComfyUIWorkflowConvertRequest) (*iapiserver.ComfyUIWorkflowConvertResult, error) {
	workflow, p, err := s.visibleComfyUIWorkflow(ctx, id, "convert")
	if err != nil {
		return nil, err
	}
	if workflow.Converted() {
		if workflow.ConversionIdempotencyKey == nil || *workflow.ConversionIdempotencyKey != req.IdempotencyKey {
			return nil, errors.NewStatus(code.ErrAIAppComfyUIWorkflowAlreadyConverted, "workflow was already converted")
		}
		return s.Store.ApplicationPlatforms().ConvertComfyUIWorkflow(ctx, id, workflow.OwnerUserID, p.UserID, req.IdempotencyKey, &iapiserver.ApplicationTemplate{}, &iapiserver.ApplicationTemplateVersion{})
	}
	validation, err := s.Store.ApplicationPlatforms().GetComfyUIWorkflowValidation(ctx, req.WorkflowValidationID)
	if err != nil || validation.WorkflowID != id || validation.OwnerUserID != workflow.OwnerUserID {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIValidationNotFound, "validation not found")
	}
	if validation.Status != iapiserver.ComfyUIValidationCompatible {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIValidationNotCompatible, "validation is not compatible")
	}
	if _, ok := s.Runtime.Capability(req.CapabilityDefinitionID); !ok {
		return nil, errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "capability definition is not registered")
	}
	engineType, registered := s.Runtime.EngineType("comfyui")
	if !registered {
		return nil, errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "ComfyUI engine type is not registered")
	}
	if _, ok := engineType.OperationExecutors[req.CapabilityDefinitionID]; !ok {
		return nil, errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "ComfyUI executor is not registered for capability")
	}
	var result *iapiserver.ComfyUIWorkflowConvertResult
	var revision string
	err = s.Store.ApplicationPlatforms().WithEngineInstanceLock(ctx, validation.EngineInstanceID, func() error {
		engine, catalog, lockErr := s.usableComfyUIObjectInfo(ctx, validation.EngineInstanceID)
		if lockErr != nil {
			return lockErr
		}
		if !engineMatchesComfyUITemplateRestrictions(engine, req.TemplateContract) {
			return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "validation engine does not satisfy template restrictions")
		}
		parsed, lockErr := parseComfyUIWorkflow(workflow.APIWorkflow, workflow.VisualWorkflow, catalog.ObjectInfo)
		if lockErr != nil {
			return lockErr
		}
		workflow.ParsedNodes, workflow.InputCandidates, workflow.OutputCandidates, workflow.Dependencies = parsed.nodes, parsed.inputs, parsed.outputs, parsed.dependencies
		if diagnostics := compatibilityDiagnostics(workflow, catalog.ObjectInfo); len(diagnostics) != 0 {
			return errors.NewStatus(code.ErrAIAppComfyUIWorkflowIncompatible, "workflow is incompatible with the current object_info")
		}
		if lockErr := validateImportedComfyUITemplateContract(workflow, req.TemplateContract); lockErr != nil {
			return lockErr
		}
		revision, lockErr = canonicalJSONDigest(map[string]any{"api_workflow": workflow.APIWorkflow, "template_contract": req.TemplateContract})
		if lockErr != nil {
			return lockErr
		}
		template := &iapiserver.ApplicationTemplate{OwnerUserID: workflow.OwnerUserID, CapabilitySourceType: iapiserver.CapabilitySourceComfyUIWorkflow, CapabilityDefinitionID: req.CapabilityDefinitionID}
		template.Name, template.Description = req.Name, req.Description
		version := &iapiserver.ApplicationTemplateVersion{Status: iapiserver.VersionStatusDraft, CapabilitySourceType: iapiserver.CapabilitySourceComfyUIWorkflow, SourceRevision: revision, WorkflowContractRevision: &revision, SourceComfyUIWorkflowID: stringPtr(id), SourceWorkflowValidationID: stringPtr(validation.ID), TemplateContract: req.TemplateContract, ComfyUIAPIWorkflow: workflow.APIWorkflow}
		result, lockErr = s.Store.ApplicationPlatforms().ConvertComfyUIWorkflow(ctx, id, workflow.OwnerUserID, p.UserID, req.IdempotencyKey, template, version)
		return lockErr
	})
	if err != nil {
		return nil, err
	}
	s.publish(ctx, "comfyui_workflow_converted", id+":"+result.ApplicationTemplate.ID, map[string]any{"workflow_id": id, "owner_user_id": workflow.OwnerUserID, "actor_user_id": p.UserID, "workflow_validation_id": validation.ID, "application_template_id": result.ApplicationTemplate.ID, "application_template_version_id": result.ApplicationTemplateVersion.ID, "workflow_contract_revision": revision, "converted_at": imachinery.Now()})
	return result, nil
}

func (s *applicationPlatformService) visibleComfyUIWorkflow(ctx context.Context, id, action string) (*iapiserver.ComfyUIWorkflow, Principal, error) {
	p, err := s.principal(ctx, false)
	if err != nil {
		return nil, p, err
	}
	workflow, err := s.Store.ApplicationPlatforms().GetComfyUIWorkflow(ctx, id)
	if err != nil {
		return nil, p, errors.NewStatus(code.ErrAIAppComfyUIWorkflowNotFound, "workflow not found")
	}
	if !p.Admin && workflow.OwnerUserID != p.UserID {
		return nil, p, errors.NewStatus(code.ErrAIAppComfyUIWorkflowNotFound, "workflow not found")
	}
	if err := s.auditManagedWorkflow(ctx, p, workflow, action); err != nil {
		return nil, p, errors.NewStatus(code.ErrAIAppComfyUIWorkflowAccessDenied, "managed workflow access could not be audited")
	}
	return workflow, p, nil
}
func (s *applicationPlatformService) auditManagedWorkflow(ctx context.Context, p Principal, workflow *iapiserver.ComfyUIWorkflow, action string) error {
	if !p.Admin || workflow.OwnerUserID == p.UserID {
		return nil
	}
	auditor := s.WorkflowAudit
	if auditor == nil {
		auditor = StructuredWorkflowAuditor{}
	}
	return auditor.Record(ctx, WorkflowAuditRecord{Action: action, ActorUserID: p.UserID, OwnerUserID: workflow.OwnerUserID, WorkflowID: workflow.ID, Result: "success", OccurredAt: imachinery.Now()})
}

type parsedComfyUIWorkflow struct {
	status       string
	summary      iapiserver.ComfyUIWorkflowParseSummary
	nodes        []iapiserver.ComfyUIWorkflowNode
	inputs       []iapiserver.ComfyUIWorkflowInputCandidate
	outputs      []iapiserver.ComfyUIWorkflowOutputCandidate
	dependencies []iapiserver.ComfyUIWorkflowDependency
}

func parseComfyUIWorkflow(workflow, visual, objectInfo map[string]any) (*parsedComfyUIWorkflow, error) {
	if len(workflow) == 0 {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIWorkflowFileInvalid, "API workflow is empty")
	}
	ids := make([]string, 0, len(workflow))
	for id := range workflow {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := &parsedComfyUIWorkflow{status: iapiserver.ComfyUIParseFullySupported, nodes: []iapiserver.ComfyUIWorkflowNode{}, inputs: []iapiserver.ComfyUIWorkflowInputCandidate{}, outputs: []iapiserver.ComfyUIWorkflowOutputCandidate{}, dependencies: []iapiserver.ComfyUIWorkflowDependency{}}
	dependencyKeys := map[string]bool{}
	known := map[string]struct{}{}
	visualNodes := map[string]map[string]any{}
	for _, raw := range anySlice(visual["nodes"]) {
		item := mapValue(raw)
		visualNodes[fmt.Sprint(item["id"])] = item
	}
	for _, id := range ids {
		known[id] = struct{}{}
	}
	for _, id := range ids {
		raw, ok := workflow[id].(map[string]any)
		if !ok {
			return nil, errors.NewStatus(code.ErrAIAppComfyUIWorkflowFileInvalid, "workflow node is not an object")
		}
		class := stringValue(raw["class_type"])
		inputs, ok := raw["inputs"].(map[string]any)
		if class == "" || !ok {
			return nil, errors.NewStatus(code.ErrAIAppComfyUIWorkflowFileInvalid, "workflow node requires class_type and inputs")
		}
		node := iapiserver.ComfyUIWorkflowNode{NodeID: id, ClassType: class, Inputs: []map[string]any{}, Outputs: []map[string]any{}, ParseStatus: iapiserver.ComfyUIParseFullySupported, Errors: []iapiserver.ComfyUIWorkflowDiagnostic{}, Warnings: []iapiserver.ComfyUIWorkflowDiagnostic{}}
		if visualNode := visualNodes[id]; visualNode != nil {
			node.Title = stringValue(visualNode["title"])
			node.DisplayName = stringValue(visualNode["type"])
			if position := mapValue(visualNode["pos"]); position != nil {
				node.Position = position
			} else if visualNode["pos"] != nil {
				node.Position = map[string]any{"value": visualNode["pos"]}
			}
		}
		info, exists := objectInfo[class].(map[string]any)
		if !exists {
			node.ParseStatus = iapiserver.ComfyUIParseUnsupported
			result.status = iapiserver.ComfyUIParseUnsupported
			node.Errors = append(node.Errors, iapiserver.ComfyUIWorkflowDiagnostic{Code: "NODE_TYPE_MISSING", Message: "class_type is absent from object_info"})
		} else if module := stringValue(info["python_module"]); module != "" && module != "nodes" {
			node.ParseStatus = iapiserver.ComfyUIParsePartiallySupported
			node.Warnings = append(node.Warnings, iapiserver.ComfyUIWorkflowDiagnostic{Code: "CUSTOM_NODE_DEPENDENCY", Message: "custom node availability must be validated on each target instance", NodeID: stringPtr(id)})
			key := "custom_node:" + module
			if !dependencyKeys[key] {
				dependencyKeys[key] = true
				result.dependencies = append(result.dependencies, iapiserver.ComfyUIWorkflowDependency{DependencyType: "custom_node", Identifier: module, DisplayName: module, Required: true, SourceNodeIDs: []string{id}})
			}
		}
		inputNames := make([]string, 0, len(inputs))
		for name := range inputs {
			inputNames = append(inputNames, name)
		}
		sort.Strings(inputNames)
		for _, name := range inputNames {
			value := inputs[name]
			definition := comfyUIInputDefinition(info, name)
			candidate := iapiserver.ComfyUIWorkflowInputCandidate{NodeID: id, InputName: name, Classification: definition.classification, DataType: definition.dataType, CurrentValue: value, Required: definition.required, Options: definition.options, Minimum: definition.minimum, Maximum: definition.maximum, Step: definition.step}
			if candidate.Classification == "exposable" {
				if _, ok := value.(map[string]any); ok {
					candidate.Classification = "fixed_only"
				} else if _, ok := value.([]any); ok {
					candidate.Classification = "fixed_only"
				}
			}
			if candidate.Classification == "unsupported" {
				node.ParseStatus = iapiserver.ComfyUIParseManualConfigurationRequired
				node.Warnings = append(node.Warnings, iapiserver.ComfyUIWorkflowDiagnostic{Code: "INPUT_DEFINITION_UNKNOWN", Message: "input requires manual configuration", NodeID: stringPtr(id), FieldName: stringPtr(name)})
				result.status = mergeComfyUIParseStatus(result.status, iapiserver.ComfyUIParseManualConfigurationRequired)
			}
			if ref, ok := workflowReference(value); ok {
				if _, exists := known[ref.nodeID]; !exists {
					return nil, errors.NewStatus(code.ErrAIAppComfyUIWorkflowReferenceInvalid, "input references missing node "+ref.nodeID)
				}
				source := mapValue(workflow[ref.nodeID])
				sourceInfo := mapValue(objectInfo[stringValue(source["class_type"])])
				if outputs := anySlice(sourceInfo["output"]); ref.outputIndex >= len(outputs) {
					return nil, errors.NewStatus(code.ErrAIAppComfyUIWorkflowReferenceInvalid, "input references missing source output")
				}
				candidate.Classification = "connection"
				candidate.SourceNodeID = ref.nodeID
				candidate.SourceOutputIndex = &ref.outputIndex
			}
			node.Inputs = append(node.Inputs, map[string]any{"name": name, "value": value, "classification": candidate.Classification, "data_type": candidate.DataType})
			result.inputs = append(result.inputs, candidate)
			if dep := dependencyFromInput(id, name, value); dep != nil {
				key := dep.DependencyType + ":" + dep.Identifier
				if !dependencyKeys[key] {
					dependencyKeys[key] = true
					result.dependencies = append(result.dependencies, *dep)
				}
			}
		}
		outputTypes := anySlice(info["output"])
		outputNames := anySlice(info["output_name"])
		outputNode, _ := info["output_node"].(bool)
		for index, rawType := range outputTypes {
			name := ""
			if index < len(outputNames) {
				name = stringValue(outputNames[index])
			}
			dataType := stringValue(rawType)
			candidate := iapiserver.ComfyUIWorkflowOutputCandidate{NodeID: id, OutputIndex: index, OutputName: name, DataType: dataType, Extractable: outputNode, MediaType: comfyUIMediaType(dataType)}
			result.outputs = append(result.outputs, candidate)
			node.Outputs = append(node.Outputs, map[string]any{"output_index": index, "output_name": name, "data_type": dataType})
		}
		result.nodes = append(result.nodes, node)
		result.status = mergeComfyUIParseStatus(result.status, node.ParseStatus)
	}
	result.summary.TotalNodes = len(result.nodes)
	for _, node := range result.nodes {
		if node.ParseStatus == iapiserver.ComfyUIParseUnsupported {
			result.summary.UnsupportedNodes++
		} else if len(node.Warnings) > 0 {
			result.summary.WarningNodes++
		} else {
			result.summary.SupportedNodes++
		}
	}
	return result, nil
}

func mergeComfyUIParseStatus(current, next string) string {
	rank := map[string]int{iapiserver.ComfyUIParseFullySupported: 0, iapiserver.ComfyUIParsePartiallySupported: 1, iapiserver.ComfyUIParseManualConfigurationRequired: 2, iapiserver.ComfyUIParseUnsupported: 3}
	if rank[next] > rank[current] {
		return next
	}
	return current
}
func dependencyDiagnosticCount(items []iapiserver.ComfyUIWorkflowDiagnostic) int {
	count := 0
	for _, item := range items {
		if strings.HasPrefix(item.Code, "DEPENDENCY_") || item.Code == "ENUM_VALUE_UNAVAILABLE" {
			count++
		}
	}
	return count
}

type workflowRef struct {
	nodeID      string
	outputIndex int
}

func workflowReference(value any) (workflowRef, bool) {
	items, ok := value.([]any)
	if !ok || len(items) != 2 {
		return workflowRef{}, false
	}
	node := fmt.Sprint(items[0])
	index, ok := numberAsInt(items[1])
	return workflowRef{nodeID: node, outputIndex: index}, ok && index >= 0
}
func numberAsInt(value any) (int, bool) {
	switch number := value.(type) {
	case float64:
		return int(number), number == float64(int(number))
	case json.Number:
		value, err := number.Int64()
		return int(value), err == nil
	case int:
		return number, true
	default:
		return 0, false
	}
}

func numericValue(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	case json.Number:
		value, err := number.Float64()
		return value, err == nil
	default:
		return 0, false
	}
}

type comfyInputDefinition struct {
	classification, dataType string
	required                 bool
	options                  []any
	minimum, maximum, step   *float64
}

func comfyUIInputDefinition(info map[string]any, name string) comfyInputDefinition {
	input := mapValue(info["input"])
	for _, section := range []string{"required", "optional", "hidden"} {
		definition := mapValue(input[section])
		raw, ok := definition[name]
		if !ok {
			continue
		}
		result := comfyInputDefinition{classification: "exposable", dataType: "unknown", required: section == "required"}
		if section == "hidden" {
			result.classification = "hidden"
		}
		items := anySlice(raw)
		if len(items) == 0 {
			return result
		}
		if enum := anySlice(items[0]); len(enum) > 0 {
			result.dataType = "enum"
			result.options = enum
		} else if value := stringValue(items[0]); value != "" {
			result.dataType = strings.ToLower(value)
		}
		if len(items) > 1 {
			options := mapValue(items[1])
			result.minimum = floatPointer(options["min"])
			result.maximum = floatPointer(options["max"])
			result.step = floatPointer(options["step"])
		}
		return result
	}
	return comfyInputDefinition{classification: "unsupported", dataType: "unknown"}
}
func floatPointer(value any) *float64 {
	number, ok := value.(float64)
	if !ok {
		return nil
	}
	return &number
}
func dependencyFromInput(nodeID, name string, value any) *iapiserver.ComfyUIWorkflowDependency {
	text, ok := value.(string)
	if !ok || text == "" {
		return nil
	}
	lower := strings.ToLower(name)
	kind := ""
	switch {
	case strings.Contains(lower, "lora"):
		kind = "lora"
	case strings.Contains(lower, "model") || strings.Contains(lower, "ckpt"):
		kind = "model"
	case strings.Contains(lower, "file"):
		kind = "file"
	}
	if kind == "" {
		return nil
	}
	return &iapiserver.ComfyUIWorkflowDependency{DependencyType: kind, Identifier: text, DisplayName: text, Required: true, SourceNodeIDs: []string{nodeID}}
}
func comfyUIMediaType(value string) string {
	lower := strings.ToLower(value)
	switch {
	case strings.Contains(lower, "image"):
		return "image"
	case strings.Contains(lower, "video"):
		return "video"
	case strings.Contains(lower, "audio"):
		return "audio"
	case strings.Contains(lower, "text") || strings.Contains(lower, "string"):
		return "text"
	default:
		return "other"
	}
}
func compatibilityDiagnostics(workflow *iapiserver.ComfyUIWorkflow, objectInfo map[string]any) []iapiserver.ComfyUIWorkflowDiagnostic {
	diagnostics := []iapiserver.ComfyUIWorkflowDiagnostic{}
	for _, node := range workflow.ParsedNodes {
		info, ok := objectInfo[node.ClassType].(map[string]any)
		if !ok {
			id := node.NodeID
			diagnostics = append(diagnostics, iapiserver.ComfyUIWorkflowDiagnostic{Code: "NODE_TYPE_MISSING", Message: "target instance does not provide " + node.ClassType, NodeID: &id})
			continue
		}
		inputDefinitions := mapValue(info["input"])
		required := mapValue(inputDefinitions["required"])
		seenInputs := map[string]bool{}
		for _, input := range node.Inputs {
			name := stringValue(input["name"])
			seenInputs[name] = true
			definition, defined := required[name]
			if !defined {
				definition, defined = mapValue(inputDefinitions["optional"])[name]
			}
			if !defined {
				definition, defined = mapValue(inputDefinitions["hidden"])[name]
			}
			if !defined {
				id, field := node.NodeID, name
				diagnostics = append(diagnostics, iapiserver.ComfyUIWorkflowDiagnostic{Code: "INPUT_MISSING", Message: "target instance does not define workflow input", NodeID: &id, FieldName: &field})
				continue
			}
			items := anySlice(definition)
			if len(items) > 0 {
				targetType := ""
				if enum := anySlice(items[0]); len(enum) > 0 {
					targetType = "enum"
				} else {
					targetType = strings.ToLower(stringValue(items[0]))
				}
				sourceType := stringValue(input["data_type"])
				if targetType != "" && sourceType != "" && sourceType != "unknown" && targetType != sourceType {
					id, field := node.NodeID, name
					diagnostics = append(diagnostics, iapiserver.ComfyUIWorkflowDiagnostic{Code: "INPUT_TYPE_CHANGED", Message: "target input type differs from import snapshot", NodeID: &id, FieldName: &field})
				}
				if ref, ok := workflowReference(input["value"]); ok {
					sourceClass := ""
					for _, sourceNode := range workflow.ParsedNodes {
						if sourceNode.NodeID == ref.nodeID {
							sourceClass = sourceNode.ClassType
							break
						}
					}
					sourceInfo := mapValue(objectInfo[sourceClass])
					sourceOutputs := anySlice(sourceInfo["output"])
					if ref.outputIndex >= len(sourceOutputs) {
						id, field := node.NodeID, name
						diagnostics = append(diagnostics, iapiserver.ComfyUIWorkflowDiagnostic{Code: "CONNECTION_OUTPUT_MISSING", Message: "target source output no longer exists", NodeID: &id, FieldName: &field})
					} else if outputType := strings.ToLower(stringValue(sourceOutputs[ref.outputIndex])); targetType != "" && outputType != "" && targetType != "enum" && outputType != targetType {
						id, field := node.NodeID, name
						diagnostics = append(diagnostics, iapiserver.ComfyUIWorkflowDiagnostic{Code: "CONNECTION_TYPE_MISMATCH", Message: "target connection types are incompatible", NodeID: &id, FieldName: &field})
					}
				}
				if options := anySlice(items[0]); len(options) > 0 && stringValue(input["classification"]) != "connection" {
					current := fmt.Sprint(input["value"])
					found := false
					for _, option := range options {
						if fmt.Sprint(option) == current {
							found = true
							break
						}
					}
					if !found {
						id, field := node.NodeID, name
						diagnostics = append(diagnostics, iapiserver.ComfyUIWorkflowDiagnostic{Code: "ENUM_VALUE_UNAVAILABLE", Message: "target instance does not allow the selected value", NodeID: &id, FieldName: &field})
					}
				}
				if len(items) > 1 && stringValue(input["classification"]) != "connection" {
					constraints := mapValue(items[1])
					if number, ok := numericValue(input["value"]); ok {
						if minimum, ok := numericValue(constraints["min"]); ok && number < minimum {
							id, field := node.NodeID, name
							diagnostics = append(diagnostics, iapiserver.ComfyUIWorkflowDiagnostic{Code: "VALUE_BELOW_MINIMUM", Message: "workflow value is below target minimum", NodeID: &id, FieldName: &field})
						}
						if maximum, ok := numericValue(constraints["max"]); ok && number > maximum {
							id, field := node.NodeID, name
							diagnostics = append(diagnostics, iapiserver.ComfyUIWorkflowDiagnostic{Code: "VALUE_ABOVE_MAXIMUM", Message: "workflow value is above target maximum", NodeID: &id, FieldName: &field})
						}
					}
				}
			}
		}
		for name := range required {
			if !seenInputs[name] {
				id, field := node.NodeID, name
				diagnostics = append(diagnostics, iapiserver.ComfyUIWorkflowDiagnostic{Code: "REQUIRED_INPUT_ADDED", Message: "target instance requires an input absent from the workflow", NodeID: &id, FieldName: &field})
			}
		}
		outputTypes := anySlice(info["output"])
		for _, output := range node.Outputs {
			index, valid := numberAsInt(output["output_index"])
			if !valid || index >= len(outputTypes) {
				id := node.NodeID
				diagnostics = append(diagnostics, iapiserver.ComfyUIWorkflowDiagnostic{Code: "OUTPUT_MISSING", Message: "target instance does not provide workflow output", NodeID: &id})
			}
		}
	}
	return diagnostics
}
func validateImportedComfyUITemplateContract(workflow *iapiserver.ComfyUIWorkflow, contract map[string]any) error {
	allowedTop := map[string]bool{"inputs": true, "fixed_parameters": true, "parameter_mappings": true, "outputs": true, "engine_restrictions": true}
	for key := range contract {
		if !allowedTop[key] {
			return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "unknown template contract field "+key)
		}
	}
	inputs, inputsOK := contract["inputs"].([]any)
	fixed, fixedOK := contract["fixed_parameters"].([]any)
	mappings, mappingsOK := contract["parameter_mappings"].([]any)
	outputs, outputsOK := contract["outputs"].([]any)
	restrictions, restrictionsOK := contract["engine_restrictions"].(map[string]any)
	if !inputsOK || !fixedOK || !mappingsOK || !outputsOK || !restrictionsOK {
		return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "template contract fields have invalid types")
	}
	if len(outputs) == 0 {
		return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "at least one output is required")
	}
	for key, value := range restrictions {
		switch key {
		case "allowed_engine_instance_ids", "allowed_regions":
			if _, ok := value.(string); !ok {
				return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "engine restriction list must be a comma-separated string")
			}
		case "require_enabled":
			if _, ok := value.(bool); !ok {
				return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "require_enabled must be boolean")
			}
		case "required_health_status":
			status, ok := value.(string)
			if !ok || (status != iapiserver.EngineHealthOnline && status != iapiserver.EngineHealthDegraded) {
				return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "required_health_status is invalid")
			}
		default:
			return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "unknown engine restriction "+key)
		}
	}
	nodes := map[string]map[string]any{}
	for id, raw := range workflow.APIWorkflow {
		nodes[id] = mapValue(raw)
	}
	inputKeys := map[string]struct{}{}
	for _, raw := range inputs {
		item := mapValue(raw)
		key := stringValue(item["key"])
		_, requiredOK := item["required"].(bool)
		_, connectableOK := item["connectable"].(bool)
		if key == "" || stringValue(item["data_type"]) == "" || !requiredOK || !connectableOK {
			return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "template input key, data_type, required, and connectable are required")
		}
		if _, exists := inputKeys[key]; exists {
			return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "template input keys must be unique")
		}
		inputKeys[key] = struct{}{}
	}
	validateTarget := func(nodeID, inputName string) error {
		node := nodes[nodeID]
		if node == nil {
			return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "mapping target node does not exist")
		}
		if _, ok := mapValue(node["inputs"])[inputName]; !ok {
			return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "mapping target input does not exist")
		}
		return nil
	}
	configuredTargets := map[string]bool{}
	for _, raw := range fixed {
		item := mapValue(raw)
		if _, ok := item["value"]; !ok {
			return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "fixed parameter value is required")
		}
		nodeID, inputName := stringValue(item["node_id"]), stringValue(item["input_name"])
		if err := validateTarget(nodeID, inputName); err != nil {
			return err
		}
		configuredTargets[nodeID+":"+inputName] = true
	}
	allowedConversions := map[string]bool{"DIRECT": true, "FIXED_VALUE": true, "ENUM_MAP": true, "MULTI_TARGET_MAP": true, "ASPECT_RATIO_TO_SIZE": true, "BOOLEAN_SWITCH": true, "CONDITIONAL": true, "RANGE_SCALE": true, "CONCAT": true, "TEMPLATE_STRING": true}
	for _, raw := range mappings {
		item := mapValue(raw)
		if !allowedConversions[stringValue(item["conversion_type"])] {
			return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "parameter mapping conversion_type is invalid")
		}
		if err := validateComfyUIConversionConfig(stringValue(item["conversion_type"]), mapValue(item["config"])); err != nil {
			return err
		}
		if _, ok := inputKeys[stringValue(item["input_key"])]; !ok {
			return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "parameter mapping references unknown template input")
		}
		targets := anySlice(item["targets"])
		if len(targets) == 0 {
			return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "parameter mapping requires a target")
		}
		for _, targetRaw := range targets {
			target := mapValue(targetRaw)
			nodeID, inputName := stringValue(target["node_id"]), stringValue(target["input_name"])
			if err := validateTarget(nodeID, inputName); err != nil {
				return err
			}
			configuredTargets[nodeID+":"+inputName] = true
		}
	}
	for _, candidate := range workflow.InputCandidates {
		if !candidate.Required || candidate.Classification == "connection" || candidate.Classification == "hidden" {
			continue
		}
		if candidate.Classification == "unsupported" {
			return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "unsupported required workflow input cannot be converted")
		}
		if !configuredTargets[candidate.NodeID+":"+candidate.InputName] {
			return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "required workflow input is neither mapped nor fixed")
		}
	}
	candidates := map[string]struct{}{}
	for _, item := range workflow.OutputCandidates {
		if item.Extractable {
			candidates[fmt.Sprintf("%s:%d", item.NodeID, item.OutputIndex)] = struct{}{}
		}
	}
	outputKeys := map[string]bool{}
	for _, raw := range outputs {
		item := mapValue(raw)
		key := stringValue(item["key"])
		if key == "" || stringValue(item["data_type"]) == "" || outputKeys[key] {
			return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "template output keys and data_type are required and keys must be unique")
		}
		outputKeys[key] = true
		mediaType := stringValue(item["media_type"])
		if mediaType != "" && !map[string]bool{"image": true, "video": true, "audio": true, "text": true, "pdf": true, "other": true}[mediaType] {
			return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "template output media_type is invalid")
		}
		index, ok := numberAsInt(item["output_index"])
		if !ok {
			return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "output index is invalid")
		}
		if _, ok := candidates[fmt.Sprintf("%s:%d", stringValue(item["node_id"]), index)]; !ok {
			return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "output does not reference an extractable candidate")
		}
	}
	return nil
}

func validateComfyUIConversionConfig(conversion string, config map[string]any) error {
	invalid := func() error {
		return errors.NewStatus(code.ErrAIAppComfyUITemplateContractInvalid, "parameter mapping config is incomplete for "+conversion)
	}
	switch conversion {
	case "DIRECT", "MULTI_TARGET_MAP":
		return nil
	case "FIXED_VALUE":
		if _, ok := config["value"]; !ok {
			return invalid()
		}
	case "ENUM_MAP", "ASPECT_RATIO_TO_SIZE":
		if mapValue(config["values"]) == nil {
			return invalid()
		}
	case "BOOLEAN_SWITCH":
		if _, ok := config["true_value"]; !ok {
			return invalid()
		}
		if _, ok := config["false_value"]; !ok {
			return invalid()
		}
	case "RANGE_SCALE":
		if _, ok := numericValue(config["scale"]); !ok {
			return invalid()
		}
	case "CONCAT":
		if _, ok := config["parts"].([]any); !ok {
			return invalid()
		}
	case "TEMPLATE_STRING":
		if stringValue(config["template"]) == "" {
			return invalid()
		}
	case "CONDITIONAL":
		if mapValue(config["cases"]) == nil {
			return invalid()
		}
	}
	return nil
}
func validateComfyUITemplateSnapshot(workflow, objectInfo, contract map[string]any) error {
	if err := validateComfyUIWorkflow(workflow, objectInfo); err != nil {
		return err
	}
	if outputs := anySlice(contract["outputs"]); len(outputs) > 0 {
		parsed, err := parseComfyUIWorkflow(workflow, nil, objectInfo)
		if err != nil {
			return err
		}
		resource := &iapiserver.ComfyUIWorkflow{APIWorkflow: workflow, InputCandidates: parsed.inputs, OutputCandidates: parsed.outputs}
		return validateImportedComfyUITemplateContract(resource, contract)
	}
	return validateComfyUIContract(workflow, objectInfo, contract)
}

// canonicalJSONDigest uses RFC 8785 JCS bytes so revisions are stable across JSON encoders and platforms.
func canonicalJSONDigest(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return canonicalJSONRawDigest(raw, nil)
}

func canonicalJSONRawDigest(raw []byte, fallback any) (string, error) {
	if len(raw) == 0 {
		var err error
		raw, err = json.Marshal(fallback)
		if err != nil {
			return "", err
		}
	}
	if err := validateUniqueJSONProperties(raw); err != nil {
		return "", err
	}
	canonical, err := jcs.Transform(raw)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func validateUniqueJSONProperties(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := consumeUniqueJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("JSON contains trailing content")
	}
	return nil
}
func consumeUniqueJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		keys := map[string]bool{}
		for decoder.More() {
			rawKey, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := rawKey.(string)
			if !ok {
				return errors.New("JSON object key is invalid")
			}
			if keys[key] {
				return fmt.Errorf("JSON object contains duplicate key %q", key)
			}
			keys[key] = true
			if err := consumeUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errors.New("JSON object is not closed")
		}
	case '[':
		for decoder.More() {
			if err := consumeUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errors.New("JSON array is not closed")
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	return nil
}
