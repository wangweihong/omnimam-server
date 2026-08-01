package mcp

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	protocol "github.com/wangweihong/omnimam/backend/pkg/mcp"
)

func (s *Service) callTool(ctx context.Context, invocation protocol.Invocation, arguments any) (toolExecution, error) {
	switch value := arguments.(type) {
	case *protocol.CapabilitiesListArguments:
		return s.listCapabilities(ctx, value)
	case *protocol.CapabilitiesGetArguments:
		return s.getCapability(ctx, value)
	case *protocol.ApplicationsListArguments:
		return s.listApplications(ctx, value)
	case *protocol.ApplicationsGetArguments:
		return s.getApplication(ctx, value)
	case *protocol.ApplicationsRunArguments:
		return s.runApplication(ctx, invocation, value)
	case *protocol.ApplicationRunsGetArguments:
		return s.getApplicationRun(ctx, value)
	case *protocol.ApplicationRunsCancelArguments:
		return s.cancelApplicationRun(ctx, value)
	case *protocol.AssetsSearchArguments:
		return s.searchAssets(ctx, value)
	case *protocol.AssetsGetArguments:
		return s.getAsset(ctx, value)
	case *protocol.AssetsPrepareUploadArguments:
		return s.prepareAssetUpload(ctx, value)
	case *protocol.AssetsCompleteUploadArguments:
		return s.completeAssetUpload(ctx, value)
	default:
		return toolExecution{}, internalMCPError(code.ErrMCPToolArgumentInvalid, "ERR_MCP_TOOL_ARGUMENT_INVALID", false, "unsupported tool arguments")
	}
}

func (s *Service) listCapabilities(ctx context.Context, args *protocol.CapabilitiesListArguments) (toolExecution, error) {
	page, err := decodeCursor(args.Cursor, args.Limit, "capabilities|"+args.Domain+"|"+args.Query)
	if err != nil {
		return toolExecution{}, invalidCursorError(err)
	}
	applications, err := s.runnableApplications(ctx, "", "")
	if err != nil {
		return toolExecution{}, err
	}
	applicationsByCapability := make(map[string][]*iapiserver.Application)
	for _, application := range applications {
		applicationsByCapability[application.CapabilityDefinitionID] = append(applicationsByCapability[application.CapabilityDefinitionID], application)
	}
	definitions := s.capabilities.CapabilityDefinitions()
	filtered := make([]CapabilitySummary, 0, len(definitions))
	query := strings.ToLower(strings.TrimSpace(args.Query))
	for _, definition := range definitions {
		if args.Domain != "" && !strings.HasPrefix(definition.ID, args.Domain+".") {
			continue
		}
		name := localizedText(definition.NameI18n)
		if query != "" && !strings.Contains(strings.ToLower(definition.ID+" "+name), query) {
			continue
		}
		apps := applicationsByCapability[definition.ID]
		if len(apps) == 0 {
			continue
		}
		filtered = append(filtered, capabilitySummary(definition, apps))
	}
	offset := page * args.Limit
	items := paginate(filtered, offset, args.Limit)
	result := CapabilityListResult{
		Total: len(filtered), Items: items,
		NextCursor: nextCursor(offset, len(items), len(filtered), args.Limit, "capabilities|"+args.Domain+"|"+args.Query),
	}
	return completeExecution(result, fmt.Sprintf("Found %d OmniMAM capabilities.", len(items))), nil
}

func (s *Service) getCapability(ctx context.Context, args *protocol.CapabilitiesGetArguments) (toolExecution, error) {
	definition, ok := s.capabilities.Capability(args.CapabilityID)
	if !ok {
		return toolExecution{}, internalMCPError(code.ErrMCPResourceNotVisible, "ERR_MCP_RESOURCE_NOT_VISIBLE", false, "capability does not exist or is not visible")
	}
	applications, err := s.runnableApplications(ctx, "", definition.ID)
	if err != nil {
		return toolExecution{}, err
	}
	if len(applications) == 0 {
		return toolExecution{}, internalMCPError(code.ErrMCPResourceNotVisible, "ERR_MCP_RESOURCE_NOT_VISIBLE", false, "capability has no visible runnable Application")
	}
	inputSchema, outputSchema := emptyPublishedSchema(), emptyPublishedSchema()
	versionID := *applications[0].CurrentVersionID
	if version, versionErr := s.applications.GetApplicationVersion(ctx, versionID); versionErr == nil {
		inputSchema, outputSchema = publishedSchema(version.InputSchema), publishedSchema(version.OutputSchema)
	}
	result := CapabilityDetail{
		CapabilitySummary: capabilitySummary(definition, applications),
		InputSchema:       inputSchema, OutputSchema: outputSchema,
	}
	return completeExecution(result, "Capability "+definition.ID+" is read-only; use a published Application to run it."), nil
}

func (s *Service) listApplications(ctx context.Context, args *protocol.ApplicationsListArguments) (toolExecution, error) {
	page, err := decodeCursor(args.Cursor, args.Limit, "applications|"+args.Query+"|"+args.CapabilityID)
	if err != nil {
		return toolExecution{}, invalidCursorError(err)
	}
	runnable := true
	response, err := s.applications.ListApplications(ctx, &iapiserver.ApplicationListRequest{
		BasicQueryParam: imachinery.BasicQueryParam{
			PagingParams: imachinery.PagingParams{PageNum: page, PageSize: args.Limit},
			Keyword:      args.Query,
		},
		CapabilityDefinitionID: args.CapabilityID, RunEnabled: &runnable, PublishedOnly: true,
	})
	if err != nil {
		return toolExecution{}, err
	}
	items := make([]ApplicationSummary, 0, len(response.Items))
	for _, application := range response.Items {
		items = append(items, applicationSummary(application))
	}
	offset := page * args.Limit
	result := ApplicationListResult{
		Total: int(response.Total), Items: items,
		NextCursor: nextCursor(offset, len(items), int(response.Total), args.Limit, "applications|"+args.Query+"|"+args.CapabilityID),
	}
	return completeExecution(result, fmt.Sprintf("Found %d runnable OmniMAM applications.", len(items))), nil
}

func (s *Service) getApplication(ctx context.Context, args *protocol.ApplicationsGetArguments) (toolExecution, error) {
	detail, err := s.applicationDetail(ctx, args.ApplicationID)
	if err != nil {
		return toolExecution{}, err
	}
	return completeExecution(detail, "Application "+detail.ApplicationID+" is ready for structured execution."), nil
}

func (s *Service) runApplication(
	ctx context.Context,
	invocation protocol.Invocation,
	args *protocol.ApplicationsRunArguments,
) (toolExecution, error) {
	application, err := s.applications.GetApplication(ctx, args.ApplicationID)
	if err != nil {
		return toolExecution{}, err
	}
	if !application.RunEnabled || application.CurrentVersionID == nil || *application.CurrentVersionID == "" {
		return toolExecution{}, internalMCPError(code.ErrMCPToolArgumentInvalid, "ERR_MCP_TOOL_ARGUMENT_INVALID", false, "Application is not published and runnable")
	}
	versionID := args.ApplicationVersionID
	if versionID == "" {
		versionID = *application.CurrentVersionID
	}
	version, err := s.applications.GetApplicationVersion(ctx, versionID)
	if err != nil {
		return toolExecution{}, err
	}
	if version.ApplicationID != application.ID || version.Status != iapiserver.VersionStatusPublished {
		return toolExecution{}, internalMCPError(code.ErrMCPToolArgumentInvalid, "ERR_MCP_TOOL_ARGUMENT_INVALID", false, "Application version is not a published version of the requested Application")
	}
	run, err := s.applications.CreateApplicationRun(ctx, application.ID, &iapiserver.ApplicationRunCreateRequest{
		ApplicationVersionID: version.ID, Inputs: args.Input, IdempotencyKey: args.IdempotencyKey,
	})
	if err != nil {
		return toolExecution{}, err
	}
	correlateAudit(ctx, run.ID, "")
	accepted := ApplicationRunAccepted{
		ApplicationRunID: run.ID, ApplicationID: application.ID, ApplicationVersionID: version.ID,
		Status: acceptedRunStatus(run), StatusURI: "omnimam://application-runs/" + run.ID,
	}
	content := []protocol.ContentBlock{{Type: "text", Text: "ApplicationRun " + run.ID + " was created."}}
	if !invocation.Meta.ClientCapabilities.HasTasks() || run.AtomicTaskID == nil || *run.AtomicTaskID == "" {
		return toolExecution{Structured: accepted, Content: content}, nil
	}
	binding, bindingErr := s.createTaskBinding(ctx, invocation, run)
	if bindingErr != nil {
		return toolExecution{Structured: accepted, Content: content}, nil
	}
	accepted.MCPTaskID = binding.MCPTaskID
	task := s.taskFromBinding(binding, run.AtomicTask)
	return toolExecution{Structured: accepted, Content: content, Task: &task}, nil
}

func (s *Service) getApplicationRun(ctx context.Context, args *protocol.ApplicationRunsGetArguments) (toolExecution, error) {
	projection, err := s.applicationRunProjection(ctx, args.ApplicationRunID)
	if err != nil {
		return toolExecution{}, err
	}
	content := []protocol.ContentBlock{{Type: "text", Text: "ApplicationRun " + projection.ApplicationRunID + " is " + projection.Status + "."}}
	for _, output := range projection.Outputs {
		content = append(content, protocol.ContentBlock{Type: "resource_link", URI: output.URI, Name: output.OutputKey, MIMEType: output.MediaType})
	}
	return toolExecution{Structured: projection, Content: content}, nil
}

func (s *Service) cancelApplicationRun(ctx context.Context, args *protocol.ApplicationRunsCancelArguments) (toolExecution, error) {
	run, err := s.applications.GetApplicationRun(ctx, args.ApplicationRunID)
	if err != nil {
		return toolExecution{}, err
	}
	if run.AtomicTaskID == nil || *run.AtomicTaskID == "" {
		return toolExecution{}, internalMCPError(code.ErrMCPTaskSourceUnavailable, "ERR_MCP_TASK_SOURCE_UNAVAILABLE", true, "ApplicationRun has no readable AtomicTask")
	}
	currentStatus := ""
	if run.AtomicTask != nil {
		currentStatus = run.AtomicTask.Status
	}
	if iapiserver.IsAtomicTaskTerminal(currentStatus) {
		result := ApplicationRunCancelResult{ApplicationRunID: run.ID, Accepted: false, Status: runStatus(currentStatus)}
		return completeExecution(result, "ApplicationRun "+run.ID+" is already terminal."), nil
	}
	task, err := s.tasks.CancelAtomicTask(ctx, *run.AtomicTaskID, &iapiserver.ActionReasonRequest{Reason: args.Reason})
	if err != nil {
		return toolExecution{}, err
	}
	result := ApplicationRunCancelResult{ApplicationRunID: run.ID, Accepted: true, Status: runStatus(task.Status)}
	return completeExecution(result, "Cancellation was requested for ApplicationRun "+run.ID+"."), nil
}

func (s *Service) searchAssets(ctx context.Context, args *protocol.AssetsSearchArguments) (toolExecution, error) {
	page, err := decodeCursor(args.Cursor, args.Limit, "assets|"+args.Query+"|"+strings.Join(args.MediaTypes, ",")+"|"+strings.Join(args.Tags, ",")+"|"+args.Sort)
	if err != nil {
		return toolExecution{}, invalidCursorError(err)
	}
	sortField, sortOrder := strings.CutSuffix(args.Sort, "_desc")
	if !sortOrder {
		sortField, _ = strings.CutSuffix(args.Sort, "_asc")
	}
	order := "asc"
	if strings.HasSuffix(args.Sort, "_desc") {
		order = "desc"
	}
	request := &iapiserver.UserAssetListRequest{BasicQueryParam: imachinery.BasicQueryParam{
		PagingParams: imachinery.PagingParams{PageNum: 0, PageSize: imachinery.MaxPageSize},
		Keyword:      args.Query, SortField: sortField, SortOrder: order,
	}}
	if len(args.MediaTypes) == 1 {
		request.MediaType = args.MediaTypes[0]
	}
	response, err := s.assets.ListAssets(ctx, request)
	if err != nil {
		return toolExecution{}, err
	}
	filtered := make([]AssetSummary, 0, len(response.Items))
	for _, item := range response.Items {
		if item.Status != iapiserver.AssetStatusActive && item.Status != iapiserver.AssetStatusArchived {
			continue
		}
		if len(args.MediaTypes) > 1 && !slices.Contains(args.MediaTypes, item.MediaType) {
			continue
		}
		if !containsAll(item.Tags, args.Tags) {
			continue
		}
		filtered = append(filtered, assetSummary(item))
	}
	offset := page * args.Limit
	items := paginate(filtered, offset, args.Limit)
	result := AssetListResult{
		Total: len(filtered), Items: items,
		NextCursor: nextCursor(offset, len(items), len(filtered), args.Limit, "assets|"+args.Query+"|"+strings.Join(args.MediaTypes, ",")+"|"+strings.Join(args.Tags, ",")+"|"+args.Sort),
	}
	return completeExecution(result, fmt.Sprintf("Found %d visible OmniMAM assets.", len(items))), nil
}

func (s *Service) getAsset(ctx context.Context, args *protocol.AssetsGetArguments) (toolExecution, error) {
	includeRepresentations := args.IncludeRepresentations == nil || *args.IncludeRepresentations
	projection, err := s.assetDetail(ctx, args.AssetID, includeRepresentations)
	if err != nil {
		return toolExecution{}, err
	}
	return completeExecution(projection, "Asset "+projection.AssetID+" metadata is available."), nil
}

func (s *Service) prepareAssetUpload(ctx context.Context, args *protocol.AssetsPrepareUploadArguments) (toolExecution, error) {
	response, err := s.assets.CreateUploads(ctx, &iapiserver.CreateAssetUploadsRequest{Items: []iapiserver.AssetUploadItemRequest{{
		ClientUploadKey: args.IdempotencyKey, FileName: args.Filename, DisplayName: args.Filename,
		SizeBytes: args.SizeBytes, SHA256: strings.ToLower(args.Checksum.Value), MIMEType: args.MediaType,
	}}})
	if err != nil {
		return toolExecution{}, err
	}
	if response.Success != 1 || len(response.Results) != 1 || response.Results[0].Upload == nil {
		return toolExecution{}, internalMCPError(code.ErrMCPToolResultInvalid, "ERR_MCP_TOOL_RESULT_INVALID", true, "Asset Library did not return an UploadSession")
	}
	upload := response.Results[0].Upload
	expiresAt := imachinery.NewTime(upload.CreatedAt.Add(s.config.UploadTTL))
	result := AssetUploadPrepared{
		UploadID: upload.ID, Method: "POST",
		ContentUploadURL:      s.publicURL("/api/v1/asset-uploads/" + upload.ID + "/content"),
		RequiredHeaders:       map[string]string{"Content-Type": args.MediaType},
		AuthorizationRequired: true, ExpiresAt: expiresAt,
	}
	return completeExecution(result, "UploadSession "+upload.ID+" is ready for binary content."), nil
}

func (s *Service) completeAssetUpload(ctx context.Context, args *protocol.AssetsCompleteUploadArguments) (toolExecution, error) {
	response, err := s.assets.CompleteUploadSession(ctx, args.UploadID, args.IdempotencyKey)
	if err != nil {
		return toolExecution{}, err
	}
	if response.Asset == nil || response.AssetVersion == nil {
		return toolExecution{}, internalMCPError(code.ErrMCPToolResultInvalid, "ERR_MCP_TOOL_RESULT_INVALID", true, "Asset Library returned an incomplete upload result")
	}
	status := assetVersionProjection(response.AssetVersion).Status
	result := AssetUploadCompleted{
		UploadID: args.UploadID, Asset: assetSummary(response.Asset), Status: status,
		URI: "omnimam://assets/" + response.Asset.ID,
	}
	return completeExecution(result, "UploadSession "+args.UploadID+" completed with Asset status "+status+"."), nil
}

func (s *Service) runnableApplications(ctx context.Context, query, capabilityID string) ([]*iapiserver.Application, error) {
	runnable := true
	response, err := s.applications.ListApplications(ctx, &iapiserver.ApplicationListRequest{
		BasicQueryParam:        imachinery.BasicQueryParam{PagingParams: imachinery.PagingParams{PageSize: imachinery.MaxPageSize}, Keyword: query},
		CapabilityDefinitionID: capabilityID, RunEnabled: &runnable, PublishedOnly: true,
	})
	if err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (s *Service) applicationDetail(ctx context.Context, id string) (ApplicationDetail, error) {
	application, err := s.applications.GetApplication(ctx, id)
	if err != nil {
		return ApplicationDetail{}, err
	}
	if !application.RunEnabled || application.CurrentVersionID == nil || *application.CurrentVersionID == "" {
		return ApplicationDetail{}, internalMCPError(code.ErrMCPResourceNotVisible, "ERR_MCP_RESOURCE_NOT_VISIBLE", false, "Application is not published and runnable")
	}
	version, err := s.applications.GetApplicationVersion(ctx, *application.CurrentVersionID)
	if err != nil || version.Status != iapiserver.VersionStatusPublished {
		return ApplicationDetail{}, internalMCPError(code.ErrMCPResourceNotVisible, "ERR_MCP_RESOURCE_NOT_VISIBLE", false, "published ApplicationVersion is not visible")
	}
	return ApplicationDetail{
		ApplicationSummary: applicationSummary(application), PublishedVersion: applicationVersionSummary(version),
		InputSchema: publishedSchema(version.InputSchema), OutputSchema: publishedSchema(version.OutputSchema),
	}, nil
}

func capabilitySummary(definition *iapiserver.CapabilityDefinition, applications []*iapiserver.Application) CapabilitySummary {
	navigation := make([]ApplicationNavigationSummary, 0, len(applications))
	for _, application := range applications {
		navigation = append(navigation, applicationNavigation(application))
	}
	sort.Slice(navigation, func(i, j int) bool { return navigation[i].ApplicationID < navigation[j].ApplicationID })
	name := localizedText(definition.NameI18n)
	return CapabilitySummary{
		CapabilityID: definition.ID, Name: name, Description: "OmniMAM " + name + " capability.",
		Status: "available", SupportsDirectInvoke: false,
		SchemaURI: "omnimam://capabilities/" + definition.ID, Applications: navigation,
	}
}

func localizedText(values map[string]string) string {
	if value := strings.TrimSpace(values["en-US"]); value != "" {
		return value
	}
	if value := strings.TrimSpace(values["zh-CN"]); value != "" {
		return value
	}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "Unnamed"
}

func completeExecution(structured any, text string) toolExecution {
	return toolExecution{Structured: structured, Content: []protocol.ContentBlock{{Type: "text", Text: text}}}
}

func paginate[T any](items []T, offset, limit int) []T {
	if offset >= len(items) {
		return []T{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

func containsAll(items, required []string) bool {
	for _, value := range required {
		if !slices.Contains(items, value) {
			return false
		}
	}
	return true
}

func acceptedRunStatus(run *iapiserver.ApplicationRun) string {
	if run != nil && run.TaskStatusProjection != nil && *run.TaskStatusProjection == iapiserver.AtomicTaskStatusRunning {
		return "running"
	}
	return "queued"
}

func emptyPublishedSchema() map[string]any {
	return map[string]any{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "additionalProperties": false}
}

func newMCPTaskID() string {
	return "mcp_task_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}
