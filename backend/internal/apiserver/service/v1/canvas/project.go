package canvas

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/pkg/general"
)

type CanvasSrv interface {
	ProjectList(ctx context.Context) (*iapiserver.ProjectListResponse, error)
	ProjectCreate(ctx context.Context, req *iapiserver.ProjectCreateRequest) (*iapiserver.ProjectCreateResponse, error)
	ProjectUpdate(ctx context.Context, req *iapiserver.ProjectUpdateRequest) (*iapiserver.ProjectRecord, error)
	ProjectDelete(ctx context.Context, req *iapiserver.ProjectDeleteRequest) (int, error)

	// CanvasList returns active canvas metadata only; it does not return raw asset content.
	CanvasList(ctx context.Context) (*iapiserver.CanvasListResponse, error)
	// CanvasTrashList returns soft-deleted canvas metadata for restore or purge flows.
	CanvasTrashList(ctx context.Context) (*iapiserver.CanvasTrashResponse, error)
	// CanvasCreate creates a classic or smart canvas in the selected project.
	CanvasCreate(ctx context.Context, req *iapiserver.CanvasCreateRequest) (*iapiserver.CanvasCreateResponse, error)
	// CanvasGet returns canvas metadata and editable graph JSON.
	CanvasGet(ctx context.Context, id string) (*iapiserver.CanvasGetResponse, error)
	// CanvasGetMeta returns lightweight metadata used for conflict checks.
	CanvasGetMeta(ctx context.Context, id string) (*iapiserver.CanvasMetaResponse, error)
	// CanvasUpdateMeta updates title, icon, project, and board placement metadata only.
	CanvasUpdateMeta(ctx context.Context, req *iapiserver.CanvasMetaUpdateRequest) (*iapiserver.CanvasRecord, error)
	// CanvasSave persists editable graph JSON and returns the updated canvas document.
	CanvasSave(ctx context.Context, req *iapiserver.CanvasSaveRequest) (*iapiserver.Canvas, error)
	// CanvasExport returns the JSON canvas document for import/export workflows.
	// It returns canvas metadata and graph data only, never embedded asset binaries.
	CanvasExport(ctx context.Context, id string) (*iapiserver.CanvasExportResponse, error)
	// CanvasImport creates a new canvas from a JSON export document.
	// It never overwrites an existing canvas and does not create async tasks.
	CanvasImport(ctx context.Context, req *iapiserver.CanvasImportRequest) (*iapiserver.CanvasImportResponse, error)
	// CanvasWorkflowExport returns a selected workflow fragment or the full canvas graph.
	// The response contains JSON nodes/connections only and no raw asset content.
	CanvasWorkflowExport(ctx context.Context, id string, req *iapiserver.CanvasWorkflowExportRequest) (
		*iapiserver.CanvasWorkflowExportResponse,
		error,
	)
	// CanvasWorkflowImport merges a workflow JSON fragment into an existing canvas.
	// It updates canvas graph metadata synchronously and does not run the workflow.
	CanvasWorkflowImport(ctx context.Context, id string, req *iapiserver.CanvasWorkflowImportRequest) (
		*iapiserver.CanvasWorkflowImportResponse,
		error,
	)
	// CanvasWorkflowPackageExport returns workflow JSON plus referenced asset metadata.
	// It creates an async audit task and never embeds raw asset content.
	CanvasWorkflowPackageExport(ctx context.Context, id string, req *iapiserver.CanvasWorkflowPackageExportRequest) (
		*iapiserver.CanvasWorkflowPackageExportResponse,
		error,
	)
	// CanvasWorkflowPackageImport merges workflow package JSON into an existing canvas.
	// It creates an async audit task and does not run generation tasks.
	CanvasWorkflowPackageImport(ctx context.Context, id string, req *iapiserver.CanvasWorkflowPackageImportRequest) (
		*iapiserver.CanvasWorkflowPackageImportResponse,
		error,
	)
	CanvasTouch(ctx context.Context, id string) (*iapiserver.CanvasTouchResponse, error)
	CanvasSoftDelete(ctx context.Context, id string) error
	CanvasRestore(ctx context.Context, id string) (*iapiserver.CanvasRestoreResponse, error)
	CanvasPurge(ctx context.Context, id string) error
}

type canvasService struct {
	store store.Factory
}

func NewService(str store.Factory) *canvasService {
	return &canvasService{store: str}
}

func (s *canvasService) ensureDefaultProject(ctx context.Context) error {
	_, err := s.store.Projects().Get(ctx, iapiserver.DefaultProjectID)
	if err == nil {
		return nil
	}
	def := &iapiserver.Project{}
	def.ID = iapiserver.DefaultProjectID
	def.Name = "默认项目"
	def.SortOrder = 0
	_, err = s.store.Projects().Add(ctx, def)
	return err
}

func (s *canvasService) toCanvasRecord(c *iapiserver.Canvas) *iapiserver.CanvasRecord {
	nodeCount := 0
	if nodes, ok := c.Extend["nodes"]; ok {
		if arr, ok := nodes.([]any); ok {
			nodeCount = len(arr)
		}
	}
	return &iapiserver.CanvasRecord{Canvas: c, NodeCount: nodeCount}
}

func hydrateCanvas(c *iapiserver.Canvas) {
	if c == nil {
		return
	}
	c.Nodes = c.Extend["nodes"]
	c.Connections = c.Extend["connections"]
	c.Viewport = c.Extend["viewport"]
	c.Logs = c.Extend["logs"]
	c.Settings = c.Extend["settings"]
}

func canvasPayload(c *iapiserver.Canvas) iapiserver.CanvasExportPayload {
	return iapiserver.CanvasExportPayload{
		Title:       c.Title,
		Icon:        c.Icon,
		Kind:        c.Kind,
		Nodes:       c.Extend["nodes"],
		Connections: c.Extend["connections"],
		Viewport:    c.Extend["viewport"],
		Logs:        c.Extend["logs"],
		Settings:    c.Extend["settings"],
	}
}

func mergeGraphValue(current, incoming any) any {
	if incoming == nil {
		return current
	}
	currentSlice, currentIsSlice := current.([]any)
	incomingSlice, incomingIsSlice := incoming.([]any)
	if currentIsSlice && incomingIsSlice {
		return append(currentSlice, incomingSlice...)
	}

	currentMap, currentIsMap := current.(map[string]any)
	incomingMap, incomingIsMap := incoming.(map[string]any)
	if currentIsMap && incomingIsMap {
		merged := make(map[string]any, len(currentMap)+len(incomingMap))
		for key, value := range currentMap {
			merged[key] = value
		}
		for key, value := range incomingMap {
			merged[key] = value
		}
		return merged
	}
	return incoming
}

func (s *canvasService) ProjectList(ctx context.Context) (*iapiserver.ProjectListResponse, error) {
	if err := s.ensureDefaultProject(ctx); err != nil {
		return nil, errors.WithStack(err)
	}

	projects, err := s.store.Projects().List(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	records := make([]*iapiserver.ProjectRecord, 0, len(projects))
	for _, p := range projects {
		count, _ := s.store.Canvases().CountByProject(ctx, p.ID)
		records = append(records, &iapiserver.ProjectRecord{Project: p, CanvasCount: count})
	}
	return &iapiserver.ProjectListResponse{Projects: records}, nil
}

func (s *canvasService) ProjectCreate(
	ctx context.Context,
	req *iapiserver.ProjectCreateRequest,
) (*iapiserver.ProjectCreateResponse, error) {
	if err := s.ensureDefaultProject(ctx); err != nil {
		return nil, errors.WithStack(err)
	}

	projects, err := s.store.Projects().List(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	maxOrder := 0
	for _, p := range projects {
		if p.SortOrder > maxOrder {
			maxOrder = p.SortOrder
		}
	}

	p := &iapiserver.Project{}
	p.Name = req.Name
	p.SortOrder = maxOrder + 1

	created, err := s.store.Projects().Add(ctx, p)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.ProjectCreateResponse{
		Project: &iapiserver.ProjectRecord{Project: created, CanvasCount: 0},
	}, nil
}

func (s *canvasService) ProjectUpdate(
	ctx context.Context,
	req *iapiserver.ProjectUpdateRequest,
) (*iapiserver.ProjectRecord, error) {
	p := &iapiserver.Project{}
	p.ID = req.ID
	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.SortOrder != nil {
		p.SortOrder = *req.SortOrder
	}

	updated, err := s.store.Projects().Update(ctx, p)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	count, _ := s.store.Canvases().CountByProject(ctx, p.ID)
	return &iapiserver.ProjectRecord{Project: updated, CanvasCount: count}, nil
}

func (s *canvasService) ProjectDelete(ctx context.Context, req *iapiserver.ProjectDeleteRequest) (int, error) {
	if req.ID == iapiserver.DefaultProjectID {
		return 0, errors.Errorf("default project cannot be deleted")
	}
	_, err := s.store.Projects().Get(ctx, req.ID)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	moved, err := s.store.Canvases().ReassignProject(ctx, req.ID, iapiserver.DefaultProjectID)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	if err := s.store.Projects().Delete(ctx, req.ID); err != nil {
		return 0, errors.WithStack(err)
	}
	return moved, nil
}

func (s *canvasService) CanvasList(ctx context.Context) (*iapiserver.CanvasListResponse, error) {
	_ = s.store.Canvases().CleanupExpiredTrash(ctx, 30)

	canvases, err := s.store.Canvases().List(ctx, false)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	records := make([]*iapiserver.CanvasRecord, 0, len(canvases))
	for _, c := range canvases {
		records = append(records, s.toCanvasRecord(c))
	}
	return &iapiserver.CanvasListResponse{Canvases: records}, nil
}

func (s *canvasService) CanvasTrashList(ctx context.Context) (*iapiserver.CanvasTrashResponse, error) {
	canvases, err := s.store.Canvases().List(ctx, true)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	records := make([]*iapiserver.CanvasRecord, 0, len(canvases))
	for _, c := range canvases {
		records = append(records, s.toCanvasRecord(c))
	}
	return &iapiserver.CanvasTrashResponse{Canvases: records, RetentionDays: 30}, nil
}

func (s *canvasService) CanvasCreate(
	ctx context.Context,
	req *iapiserver.CanvasCreateRequest,
) (*iapiserver.CanvasCreateResponse, error) {
	if err := s.ensureDefaultProject(ctx); err != nil {
		return nil, errors.WithStack(err)
	}

	kind := iapiserver.CanvasKindClassic
	if req.Kind == iapiserver.CanvasKindSmart {
		kind = iapiserver.CanvasKindSmart
	}

	projectID := req.Project
	if projectID == "" {
		projectID = iapiserver.DefaultProjectID
	}

	title := req.Title
	if title == "" {
		if kind == iapiserver.CanvasKindSmart {
			title = "智能画布"
		} else {
			title = "未命名画布"
		}
	}

	icon := req.Icon
	if icon == "" {
		if kind == iapiserver.CanvasKindSmart {
			icon = "sparkles"
		} else {
			icon = "layers"
		}
	}

	c := &iapiserver.Canvas{}
	c.Title = title
	c.Icon = icon
	c.Kind = kind
	c.ProjectID = projectID
	c.BoardX = req.BoardX
	c.BoardY = req.BoardY
	c.Extend = map[string]any{
		"nodes":       []any{},
		"connections": []any{},
		"viewport":    map[string]any{"x": 0, "y": 0, "scale": 1},
		"logs":        []any{},
		"settings":    map[string]any{},
	}

	created, err := s.store.Canvases().Add(ctx, c)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.CanvasCreateResponse{Canvas: s.toCanvasRecord(created)}, nil
}

func (s *canvasService) CanvasGet(ctx context.Context, id string) (*iapiserver.CanvasGetResponse, error) {
	c, err := s.store.Canvases().Get(ctx, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	hydrateCanvas(c)
	return &iapiserver.CanvasGetResponse{Canvas: c}, nil
}

func (s *canvasService) CanvasGetMeta(ctx context.Context, id string) (*iapiserver.CanvasMetaResponse, error) {
	c, err := s.store.Canvases().Get(ctx, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.CanvasMetaResponse{
		ID:        c.ID,
		UpdatedAt: c.UpdatedAt.UnixMilli(),
		Title:     c.Title,
		Icon:      c.Icon,
		Kind:      c.Kind,
	}, nil
}

func (s *canvasService) CanvasUpdateMeta(
	ctx context.Context,
	req *iapiserver.CanvasMetaUpdateRequest,
) (*iapiserver.CanvasRecord, error) {
	c, err := s.store.Canvases().Get(ctx, req.ID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	c.Title = general.FallbackIfNil(req.Title, c.Title)
	c.Icon = general.FallbackIfNil(req.Icon, c.Icon)
	c.Owner = general.FallbackIfNil(req.Owner, c.Owner)
	c.Color = general.FallbackIfNil(req.Color, c.Color)
	c.Pinned = general.FallbackIfNil(req.Pinned, c.Pinned)
	c.ProjectID = general.FallbackIfNil(req.Project, c.ProjectID)
	c.BoardX = general.FallbackIfNil(req.BoardX, c.BoardX)
	c.BoardY = general.FallbackIfNil(req.BoardY, c.BoardY)

	updated, err := s.store.Canvases().Update(ctx, c)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return s.toCanvasRecord(updated), nil
}

func (s *canvasService) CanvasSave(ctx context.Context, req *iapiserver.CanvasSaveRequest) (*iapiserver.Canvas, error) {
	c, err := s.store.Canvases().Get(ctx, req.ID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if req.BaseUpdatedAt > 0 && c.UpdatedAt.UnixMilli() > 0 && req.BaseUpdatedAt < c.UpdatedAt.UnixMilli() {
		return nil, errors.Errorf("canvas has been updated by another session, conflict detected")
	}

	c.Title = req.Title
	c.Icon = req.Icon
	if req.Kind != "" {
		c.Kind = req.Kind
	}
	c.Extend["nodes"] = req.Nodes
	c.Extend["connections"] = req.Connections
	c.Extend["viewport"] = req.Viewport
	c.Extend["logs"] = req.Logs
	c.Extend["settings"] = req.Settings

	updated, err := s.store.Canvases().Update(ctx, c)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	hydrateCanvas(updated)
	return updated, nil
}

func (s *canvasService) CanvasExport(ctx context.Context, id string) (*iapiserver.CanvasExportResponse, error) {
	c, err := s.store.Canvases().Get(ctx, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.CanvasExportResponse{CanvasID: c.ID, Canvas: canvasPayload(c)}, nil
}

func (s *canvasService) CanvasImport(
	ctx context.Context,
	req *iapiserver.CanvasImportRequest,
) (*iapiserver.CanvasImportResponse, error) {
	if err := s.ensureDefaultProject(ctx); err != nil {
		return nil, errors.WithStack(err)
	}

	projectID := req.Project
	if projectID == "" {
		projectID = iapiserver.DefaultProjectID
	}
	kind := req.Canvas.Kind
	if kind == "" {
		kind = iapiserver.CanvasKindClassic
	}
	title := req.Canvas.Title
	if title == "" {
		title = "导入画布"
	}

	c := &iapiserver.Canvas{}
	c.Title = title
	c.Icon = req.Canvas.Icon
	c.Kind = kind
	c.ProjectID = projectID
	c.Extend = map[string]any{
		"nodes":       req.Canvas.Nodes,
		"connections": req.Canvas.Connections,
		"viewport":    req.Canvas.Viewport,
		"logs":        req.Canvas.Logs,
		"settings":    req.Canvas.Settings,
	}

	created, err := s.store.Canvases().Add(ctx, c)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.CanvasImportResponse{Canvas: s.toCanvasRecord(created)}, nil
}

func (s *canvasService) CanvasWorkflowExport(
	ctx context.Context,
	id string,
	req *iapiserver.CanvasWorkflowExportRequest,
) (*iapiserver.CanvasWorkflowExportResponse, error) {
	c, err := s.store.Canvases().Get(ctx, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	nodes := req.Nodes
	if nodes == nil {
		nodes = c.Extend.Get("nodes")
	}
	connections := req.Connections
	if connections == nil {
		connections = c.Extend.Get("connections")
	}
	return &iapiserver.CanvasWorkflowExportResponse{
		Workflow: iapiserver.CanvasWorkflowPayload{
			CanvasID:    c.ID,
			Nodes:       nodes,
			Connections: connections,
			Metadata:    req.Metadata,
		},
	}, nil
}

func (s *canvasService) CanvasWorkflowImport(
	ctx context.Context,
	id string,
	req *iapiserver.CanvasWorkflowImportRequest,
) (*iapiserver.CanvasWorkflowImportResponse, error) {
	c, err := s.store.Canvases().Get(ctx, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	c.Extend["nodes"] = mergeGraphValue(c.Extend["nodes"], req.Workflow.Nodes)
	c.Extend["connections"] = mergeGraphValue(c.Extend["connections"], req.Workflow.Connections)

	updated, err := s.store.Canvases().Update(ctx, c)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	hydrateCanvas(updated)
	return &iapiserver.CanvasWorkflowImportResponse{Canvas: updated}, nil
}

func (s *canvasService) CanvasWorkflowPackageExport(
	ctx context.Context,
	id string,
	req *iapiserver.CanvasWorkflowPackageExportRequest,
) (*iapiserver.CanvasWorkflowPackageExportResponse, error) {
	workflow, err := s.CanvasWorkflowExport(ctx, id, &req.CanvasWorkflowExportRequest)
	if err != nil {
		return nil, err
	}
	assets := make([]*iapiserver.AssetRecord, 0, len(req.AssetIDs))
	for _, assetID := range req.AssetIDs {
		asset, err := s.store.AssetsV2().Get(ctx, assetID)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		assets = append(assets, &iapiserver.AssetRecord{Asset: asset})
	}
	task := &iapiserver.Task{
		Type:        iapiserver.TaskTypeCanvasWorkflowPackageExport,
		Status:      iapiserver.TaskStatusPending,
		Queue:       "default",
		Input:       map[string]any{"canvas_id": id, "asset_ids": req.AssetIDs, "filename": req.Filename},
		MaxAttempts: 1,
	}
	task.Name = "canvas-workflow-package-export"
	createdTask, err := s.store.Tasks().Add(ctx, task)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.CanvasWorkflowPackageExportResponse{
		Package: iapiserver.CanvasWorkflowPackage{Workflow: workflow.Workflow, Assets: assets, Metadata: req.Metadata},
		Task:    createdTask,
	}, nil
}

func (s *canvasService) CanvasWorkflowPackageImport(
	ctx context.Context,
	id string,
	req *iapiserver.CanvasWorkflowPackageImportRequest,
) (*iapiserver.CanvasWorkflowPackageImportResponse, error) {
	imported, err := s.CanvasWorkflowImport(
		ctx,
		id,
		&iapiserver.CanvasWorkflowImportRequest{Workflow: req.Package.Workflow},
	)
	if err != nil {
		return nil, err
	}
	task := &iapiserver.Task{
		Type:        iapiserver.TaskTypeCanvasWorkflowPackageImport,
		Status:      iapiserver.TaskStatusPending,
		Queue:       "default",
		Input:       map[string]any{"canvas_id": id, "metadata": req.Package.Metadata},
		MaxAttempts: 1,
	}
	task.Name = "canvas-workflow-package-import"
	createdTask, err := s.store.Tasks().Add(ctx, task)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.CanvasWorkflowPackageImportResponse{Canvas: imported.Canvas, Task: createdTask}, nil
}

func (s *canvasService) CanvasTouch(ctx context.Context, id string) (*iapiserver.CanvasTouchResponse, error) {
	c, err := s.store.Canvases().Get(ctx, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	updated, err := s.store.Canvases().Update(ctx, c)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.CanvasTouchResponse{
		Canvas:    s.toCanvasRecord(updated),
		UpdatedAt: updated.UpdatedAt.UnixMilli(),
	}, nil
}

func (s *canvasService) CanvasSoftDelete(ctx context.Context, id string) error {
	return s.store.Canvases().SoftDelete(ctx, id)
}

func (s *canvasService) CanvasRestore(ctx context.Context, id string) (*iapiserver.CanvasRestoreResponse, error) {
	if err := s.store.Canvases().Restore(ctx, id); err != nil {
		return nil, errors.WithStack(err)
	}
	c, err := s.store.Canvases().Get(ctx, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.CanvasRestoreResponse{Canvas: c}, nil
}

func (s *canvasService) CanvasPurge(ctx context.Context, id string) error {
	return s.store.Canvases().Purge(ctx, id)
}
