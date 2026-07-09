package applicationplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
)

type ApplicationPlatformSrv interface {
	ListTemplates(ctx context.Context, req *iapiserver.AppTemplateListRequest) (*iapiserver.AppTemplateListResponse, error)
	CreateTemplate(ctx context.Context, req *iapiserver.AppTemplateCreateRequest) (*iapiserver.AppTemplate, error)
	GetTemplate(ctx context.Context, id string) (*iapiserver.AppTemplate, error)
	UpdateTemplate(ctx context.Context, req *iapiserver.AppTemplateUpdateRequest) (*iapiserver.AppTemplate, error)
	DeleteTemplate(ctx context.Context, id string) (*iapiserver.SuccessResponse, error)
	ListTemplateReferences(ctx context.Context, templateID string, req *iapiserver.ApplicationListRequest) (*iapiserver.ApplicationListResponse, error)
	ListApplications(ctx context.Context, req *iapiserver.ApplicationListRequest) (*iapiserver.ApplicationListResponse, error)
	CreateApplication(ctx context.Context, req *iapiserver.ApplicationCreateRequest) (*iapiserver.Application, error)
	GetApplication(ctx context.Context, id string) (*iapiserver.Application, error)
	UpdateApplication(ctx context.Context, req *iapiserver.ApplicationUpdateRequest) (*iapiserver.Application, error)
	DeleteApplication(ctx context.Context, id string) (*iapiserver.SuccessResponse, error)
	ListFieldMappings(ctx context.Context, applicationID string) (*iapiserver.FieldMappingListResponse, error)
	SaveFieldMappings(ctx context.Context, applicationID string, req *iapiserver.FieldMappingSaveRequest) (*iapiserver.FieldMappingListResponse, error)
}

type applicationPlatformService struct {
	store store.Factory
}

func NewService(str store.Factory) *applicationPlatformService {
	return &applicationPlatformService{store: str}
}

func (s *applicationPlatformService) ListTemplates(
	ctx context.Context,
	req *iapiserver.AppTemplateListRequest,
) (*iapiserver.AppTemplateListResponse, error) {
	principal, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	req.OwnerUserID = principal.userID
	req.IncludeAll = principal.admin
	items, total, err := s.store.ApplicationPlatforms().ListTemplates(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.AppTemplateListResponse{Total: total, Items: items}, nil
}

func (s *applicationPlatformService) CreateTemplate(
	ctx context.Context,
	req *iapiserver.AppTemplateCreateRequest,
) (*iapiserver.AppTemplate, error) {
	principal, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	parsedFields, err := parseTemplateFields(req.Kind, req.Config)
	if err != nil {
		return nil, err
	}
	if err := s.ensureTemplateNameUnique(ctx, principal.userID, req.Name, ""); err != nil {
		return nil, err
	}
	tpl := &iapiserver.AppTemplate{
		OwnerUserID:  principal.userID,
		Kind:         req.Kind,
		Config:       req.Config,
		ParsedFields: parsedFields,
	}
	tpl.Name = req.Name
	tpl.Description = req.Description
	created, err := s.store.ApplicationPlatforms().AddTemplate(ctx, tpl)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return created, nil
}

func (s *applicationPlatformService) GetTemplate(ctx context.Context, id string) (*iapiserver.AppTemplate, error) {
	principal, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	tpl, err := s.store.ApplicationPlatforms().GetTemplate(ctx, id)
	if err != nil {
		return nil, errors.NewStatusF(code.ErrAIAppPermissionDenied, "template not found or not visible")
	}
	if !principal.canAccess(tpl.OwnerUserID) {
		return nil, errors.NewStatusF(code.ErrAIAppPermissionDenied, "template not found or not visible")
	}
	return tpl, nil
}

func (s *applicationPlatformService) UpdateTemplate(
	ctx context.Context,
	req *iapiserver.AppTemplateUpdateRequest,
) (*iapiserver.AppTemplate, error) {
	tpl, err := s.GetTemplate(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		if err := s.ensureTemplateNameUnique(ctx, tpl.OwnerUserID, *req.Name, tpl.ID); err != nil {
			return nil, err
		}
		tpl.Name = *req.Name
	}
	if req.Description != nil {
		tpl.Description = *req.Description
	}
	updated, err := s.store.ApplicationPlatforms().UpdateTemplate(ctx, tpl)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return updated, nil
}

func (s *applicationPlatformService) DeleteTemplate(ctx context.Context, id string) (*iapiserver.SuccessResponse, error) {
	if _, err := s.GetTemplate(ctx, id); err != nil {
		return nil, err
	}
	if err := s.store.ApplicationPlatforms().DeleteTemplate(ctx, id); err != nil {
		return nil, err
	}
	return &iapiserver.SuccessResponse{Success: true}, nil
}

func (s *applicationPlatformService) ListTemplateReferences(
	ctx context.Context,
	templateID string,
	req *iapiserver.ApplicationListRequest,
) (*iapiserver.ApplicationListResponse, error) {
	principal, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.GetTemplate(ctx, templateID); err != nil {
		return nil, err
	}
	req.TemplateID = templateID
	req.OwnerUserID = principal.userID
	req.IncludeAll = principal.admin
	return s.listApplications(ctx, req)
}

func (s *applicationPlatformService) ListApplications(
	ctx context.Context,
	req *iapiserver.ApplicationListRequest,
) (*iapiserver.ApplicationListResponse, error) {
	principal, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	req.OwnerUserID = principal.userID
	req.IncludeAll = principal.admin
	return s.listApplications(ctx, req)
}

func (s *applicationPlatformService) listApplications(
	ctx context.Context,
	req *iapiserver.ApplicationListRequest,
) (*iapiserver.ApplicationListResponse, error) {
	items, total, err := s.store.ApplicationPlatforms().ListApplications(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.ApplicationListResponse{Total: total, Items: items}, nil
}

func (s *applicationPlatformService) CreateApplication(
	ctx context.Context,
	req *iapiserver.ApplicationCreateRequest,
) (*iapiserver.Application, error) {
	principal, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	tpl, err := s.GetTemplate(ctx, req.TemplateID)
	if err != nil {
		return nil, err
	}
	mappings, err := buildFieldMappings("", tpl.ID, tpl.ParsedFields, req.FieldMappings.Items)
	if err != nil {
		return nil, err
	}
	app := &iapiserver.Application{
		OwnerUserID: principal.userID,
		TemplateID:  tpl.ID,
		Kind:        tpl.Kind,
	}
	app.Name = req.Name
	app.Description = req.Description
	created, err := s.store.ApplicationPlatforms().AddApplication(ctx, app, mappings)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	for _, mapping := range created.FieldMappings {
		mapping.ApplicationID = created.ID
	}
	return s.GetApplication(ctx, created.ID)
}

func (s *applicationPlatformService) GetApplication(ctx context.Context, id string) (*iapiserver.Application, error) {
	principal, err := s.currentPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	app, err := s.store.ApplicationPlatforms().GetApplication(ctx, id)
	if err != nil {
		return nil, errors.NewStatusF(code.ErrAIAppPermissionDenied, "application not found or not visible")
	}
	if !principal.canAccess(app.OwnerUserID) {
		return nil, errors.NewStatusF(code.ErrAIAppPermissionDenied, "application not found or not visible")
	}
	mappings, err := s.store.ApplicationPlatforms().ListFieldMappings(ctx, app.ID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	app.FieldMappings = mappings
	return app, nil
}

func (s *applicationPlatformService) UpdateApplication(
	ctx context.Context,
	req *iapiserver.ApplicationUpdateRequest,
) (*iapiserver.Application, error) {
	app, err := s.GetApplication(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		app.Name = *req.Name
	}
	if req.Description != nil {
		app.Description = *req.Description
	}
	app.FieldMappings = nil
	updated, err := s.store.ApplicationPlatforms().UpdateApplication(ctx, app)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return s.GetApplication(ctx, updated.ID)
}

func (s *applicationPlatformService) DeleteApplication(ctx context.Context, id string) (*iapiserver.SuccessResponse, error) {
	if _, err := s.GetApplication(ctx, id); err != nil {
		return nil, err
	}
	if err := s.store.ApplicationPlatforms().DeleteApplication(ctx, id); err != nil {
		return nil, err
	}
	return &iapiserver.SuccessResponse{Success: true}, nil
}

func (s *applicationPlatformService) ListFieldMappings(
	ctx context.Context,
	applicationID string,
) (*iapiserver.FieldMappingListResponse, error) {
	app, err := s.GetApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	return &iapiserver.FieldMappingListResponse{Items: app.FieldMappings}, nil
}

func (s *applicationPlatformService) SaveFieldMappings(
	ctx context.Context,
	applicationID string,
	req *iapiserver.FieldMappingSaveRequest,
) (*iapiserver.FieldMappingListResponse, error) {
	app, err := s.GetApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	tpl, err := s.GetTemplate(ctx, app.TemplateID)
	if err != nil {
		return nil, err
	}
	mappings, err := buildFieldMappings(app.ID, tpl.ID, tpl.ParsedFields, req.Items)
	if err != nil {
		return nil, err
	}
	items, err := s.store.ApplicationPlatforms().ReplaceFieldMappings(ctx, app.ID, mappings)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.FieldMappingListResponse{Items: items}, nil
}

func (s *applicationPlatformService) ensureTemplateNameUnique(
	ctx context.Context,
	ownerUserID, name, excludeID string,
) error {
	item, err := s.store.ApplicationPlatforms().GetTemplateByOwnerName(ctx, ownerUserID, name)
	if err != nil {
		return nil
	}
	if item.ID != excludeID {
		return errors.NewStatusF(code.ErrTemplateNameDuplicated, "template name %s already exists", name)
	}
	return nil
}

type principal struct {
	userID string
	admin  bool
}

func (p principal) canAccess(ownerUserID string) bool {
	return p.admin || ownerUserID == "" || ownerUserID == p.userID
}

func (s *applicationPlatformService) currentPrincipal(ctx context.Context) (principal, error) {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err != nil || user == nil || user.ID == "" {
		return principal{userID: "system-admin", admin: true}, nil
	}
	isAdmin, err := s.isAdminUser(ctx, user.ID)
	if err != nil {
		return principal{}, err
	}
	return principal{userID: user.ID, admin: isAdmin}, nil
}

func (s *applicationPlatformService) isAdminUser(ctx context.Context, userID string) (bool, error) {
	userRoleStore := s.store.UserRoles()
	roleStore := s.store.Roles()
	if userRoleStore == nil || roleStore == nil {
		return false, nil
	}
	userRoles, err := userRoleStore.ListByUser(ctx, userID)
	if err != nil {
		return false, nil
	}
	roles, err := roleStore.List(ctx)
	if err != nil {
		return false, nil
	}
	roleNames := map[string]string{}
	for _, role := range roles {
		roleNames[role.ID] = role.Name
	}
	for _, userRole := range userRoles {
		name := strings.ToLower(strings.TrimSpace(roleNames[userRole.RoleID]))
		if name == "admin" || name == "super_admin" || name == "super-admin" {
			return true, nil
		}
	}
	return false, nil
}

func parseTemplateFields(kind string, config map[string]any) ([]iapiserver.ParsedField, error) {
	if len(config) == 0 {
		return nil, errors.NewStatusF(code.ErrTemplateParseFailed, "template config is required")
	}
	var root any
	switch kind {
	case iapiserver.AppTemplateKindComfyUI:
		if raw, ok := config["raw"].(string); ok {
			if strings.TrimSpace(raw) == "" {
				return nil, errors.NewStatusF(code.ErrTemplateParseFailed, "comfyui raw json is required")
			}
			if err := json.Unmarshal([]byte(raw), &root); err != nil {
				return nil, errors.NewStatusF(code.ErrTemplateParseFailed, "comfyui raw json parse failed")
			}
		} else {
			root = map[string]any(config)
		}
	case iapiserver.AppTemplateKindSaaSAPI:
		var ok bool
		root, ok = config["requestTemplate"]
		if !ok || root == nil {
			return nil, errors.NewStatusF(code.ErrTemplateParseFailed, "saas api requestTemplate is required")
		}
	default:
		return nil, errors.NewStatusF(code.ErrTemplateParseFailed, "template kind is unsupported")
	}
	fields := make([]iapiserver.ParsedField, 0)
	if kind == iapiserver.AppTemplateKindComfyUI {
		if err := collectComfyUIAPIFields(root, &fields); err != nil {
			return nil, err
		}
	} else {
		collectPrimitiveFields("", root, &fields)
	}
	if len(fields) == 0 {
		return nil, errors.NewStatusF(code.ErrTemplateParseFailed, "template has no primitive leaf fields")
	}
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].SourcePath < fields[j].SourcePath
	})
	return fields, nil
}

func collectComfyUIAPIFields(root any, out *[]iapiserver.ParsedField) error {
	workflow, ok := root.(map[string]any)
	if !ok || len(workflow) == 0 {
		return errors.NewStatusF(code.ErrTemplateParseFailed, "comfyui api workflow must be a json object")
	}
	if looksLikeComfyUIWorkflowSaveFormat(workflow) {
		return errors.NewStatusF(
			code.ErrTemplateParseFailed,
			"comfyui workflow must be exported in api format",
		)
	}

	nodeIDs := make([]string, 0, len(workflow))
	for nodeID := range workflow {
		nodeIDs = append(nodeIDs, nodeID)
	}
	sort.Strings(nodeIDs)
	for _, nodeID := range nodeIDs {
		if strings.TrimSpace(nodeID) == "" {
			return errors.NewStatusF(code.ErrTemplateParseFailed, "comfyui node id is empty")
		}
		node, ok := workflow[nodeID].(map[string]any)
		if !ok {
			return errors.NewStatusF(code.ErrTemplateParseFailed, "comfyui node %s must be an object", nodeID)
		}
		classType, ok := node["class_type"].(string)
		if !ok || strings.TrimSpace(classType) == "" {
			return errors.NewStatusF(code.ErrTemplateParseFailed, "comfyui node %s misses class_type", nodeID)
		}
		inputs, ok := node["inputs"].(map[string]any)
		if !ok {
			return errors.NewStatusF(code.ErrTemplateParseFailed, "comfyui node %s misses inputs", nodeID)
		}
		labelPrefix := classType
		if meta, ok := node["_meta"].(map[string]any); ok {
			if title, ok := meta["title"].(string); ok && strings.TrimSpace(title) != "" {
				labelPrefix = strings.TrimSpace(title)
			}
		}
		collectComfyUIInputFields(nodeID, "", labelPrefix, inputs, out)
	}
	return nil
}

func looksLikeComfyUIWorkflowSaveFormat(root map[string]any) bool {
	nodes, hasNodes := root["nodes"].([]any)
	_, hasLinks := root["links"]
	_, hasGroups := root["groups"]
	_, hasExtra := root["extra"]
	return hasNodes && len(nodes) > 0 && (hasLinks || hasGroups || hasExtra)
}

func collectComfyUIInputFields(
	nodeID, inputPath, labelPrefix string,
	value any,
	out *[]iapiserver.ParsedField,
) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			nextPath := joinSourcePath(inputPath, key)
			collectComfyUIInputFields(nodeID, nextPath, labelPrefix, typed[key], out)
		}
	case []any:
		if isComfyUILink(typed) {
			return
		}
		for i, item := range typed {
			collectComfyUIInputFields(nodeID, fmt.Sprintf("%s[%d]", inputPath, i), labelPrefix, item, out)
		}
	case string:
		appendComfyUIParsedField(nodeID, inputPath, labelPrefix, "string", out)
	case bool:
		appendComfyUIParsedField(nodeID, inputPath, labelPrefix, "boolean", out)
	case float64, float32, int, int64, int32, uint, uint64, uint32:
		appendComfyUIParsedField(nodeID, inputPath, labelPrefix, "number", out)
	case nil:
		appendComfyUIParsedField(nodeID, inputPath, labelPrefix, "null", out)
	}
}

func isComfyUILink(value []any) bool {
	if len(value) != 2 {
		return false
	}
	if _, ok := value[0].(string); !ok {
		return false
	}
	return isJSONNumber(value[1])
}

func isJSONNumber(value any) bool {
	switch value.(type) {
	case float64, float32, int, int64, int32, uint, uint64, uint32:
		return true
	default:
		return false
	}
}

func appendComfyUIParsedField(nodeID, inputPath, labelPrefix, fieldType string, out *[]iapiserver.ParsedField) {
	if inputPath == "" {
		return
	}
	sourcePath := nodeID + ".inputs." + inputPath
	*out = append(*out, iapiserver.ParsedField{
		SourcePath: sourcePath,
		FieldType:  fieldType,
		Required:   true,
		LabelHint:  labelPrefix + "." + labelHint(inputPath),
	})
}

func collectPrimitiveFields(prefix string, value any, out *[]iapiserver.ParsedField) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			collectPrimitiveFields(joinSourcePath(prefix, key), typed[key], out)
		}
	case []any:
		for i, item := range typed {
			collectPrimitiveFields(fmt.Sprintf("%s[%d]", prefix, i), item, out)
		}
	case string:
		appendParsedField(prefix, "string", out)
	case bool:
		appendParsedField(prefix, "boolean", out)
	case float64, float32, int, int64, int32, uint, uint64, uint32:
		appendParsedField(prefix, "number", out)
	case nil:
		appendParsedField(prefix, "null", out)
	}
}

func appendParsedField(sourcePath, fieldType string, out *[]iapiserver.ParsedField) {
	if sourcePath == "" {
		return
	}
	*out = append(*out, iapiserver.ParsedField{
		SourcePath: sourcePath,
		FieldType:  fieldType,
		Required:   true,
		LabelHint:  labelHint(sourcePath),
	})
}

func joinSourcePath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func labelHint(sourcePath string) string {
	base := path.Base(strings.ReplaceAll(sourcePath, ".", "/"))
	if idx := strings.LastIndex(base, "["); idx > 0 {
		base = base[:idx]
	}
	return strings.Trim(base, "[]")
}

func buildFieldMappings(
	applicationID, templateID string,
	parsedFields []iapiserver.ParsedField,
	inputs []iapiserver.FieldMappingInput,
) ([]*iapiserver.FieldMapping, error) {
	if len(inputs) == 0 {
		return nil, errors.NewStatusF(code.ErrFieldMappingIncomplete, "field mappings are required")
	}
	parsedByPath := map[string]iapiserver.ParsedField{}
	for _, field := range parsedFields {
		parsedByPath[field.SourcePath] = field
	}
	seenKeys := map[string]struct{}{}
	mappings := make([]*iapiserver.FieldMapping, 0, len(inputs))
	for _, input := range inputs {
		if input.FieldKey == "" || input.FieldLabel == "" || input.FieldType == "" || input.SourcePath == "" {
			return nil, errors.NewStatusF(code.ErrFieldMappingIncomplete, "field mapping is incomplete")
		}
		if _, ok := seenKeys[input.FieldKey]; ok {
			return nil, errors.NewStatusF(code.ErrFieldKeyDuplicated, "field key %s is duplicated", input.FieldKey)
		}
		seenKeys[input.FieldKey] = struct{}{}
		parsed, ok := parsedByPath[input.SourcePath]
		if !ok {
			return nil, errors.NewStatusF(code.ErrMappingPathInvalid, "source path %s is invalid", input.SourcePath)
		}
		if parsed.FieldType != input.FieldType {
			return nil, errors.NewStatusF(code.ErrFieldTypeInvalid, "field type %s is invalid", input.FieldType)
		}
		mapping := &iapiserver.FieldMapping{
			ApplicationID: applicationID,
			TemplateID:    templateID,
			FieldKey:      input.FieldKey,
			FieldLabel:    input.FieldLabel,
			FieldType:     input.FieldType,
			SourcePath:    input.SourcePath,
			DefaultValue:  input.DefaultValue,
			Required:      parsed.Required,
			SortOrder:     input.SortOrder,
		}
		mapping.Name = input.FieldKey
		mappings = append(mappings, mapping)
	}
	return mappings, nil
}
