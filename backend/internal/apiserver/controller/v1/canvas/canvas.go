package canvas

import (
	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	srvv1 "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

type CanvasController struct {
	srv srvv1.Service
}

func NewController(storeIns store.Factory) *CanvasController {
	return &CanvasController{
		srv: srvv1.NewService(storeIns),
	}
}

func (cc *CanvasController) ListCanvases(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return cc.srv.Canvases().CanvasList(c)
	})
}

func (cc *CanvasController) ListTrash(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return cc.srv.Canvases().CanvasTrashList(c)
	})
}

func (cc *CanvasController) CreateCanvas(c *gin.Context) {
	core.Run(c, &iapiserver.CanvasCreateRequest{}, func(r *iapiserver.CanvasCreateRequest) (any, error) {
		return cc.srv.Canvases().CanvasCreate(c, r)
	})
}

func (cc *CanvasController) GetCanvas(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return cc.srv.Canvases().CanvasGet(c, c.Param("canvas_id"))
	})
}

func (cc *CanvasController) GetCanvasMeta(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return cc.srv.Canvases().CanvasGetMeta(c, c.Param("canvas_id"))
	})
}

func (cc *CanvasController) UpdateCanvasMeta(c *gin.Context) {
	req := &iapiserver.CanvasMetaUpdateRequest{}
	req.ID = c.Param("canvas_id")
	core.Run(c, req, func(r *iapiserver.CanvasMetaUpdateRequest) (any, error) {
		return cc.srv.Canvases().CanvasUpdateMeta(c, r)
	})
}

func (cc *CanvasController) SaveCanvas(c *gin.Context) {
	req := &iapiserver.CanvasSaveRequest{}
	req.ID = c.Param("canvas_id")
	core.Run(c, req, func(r *iapiserver.CanvasSaveRequest) (any, error) {
		return cc.srv.Canvases().CanvasSave(c, r)
	})
}

// ExportCanvas returns a JSON canvas document for frontend import/export.
// It is controlled by the canvas API route and returns graph metadata only,
// not raw asset content and not async task state.
func (cc *CanvasController) ExportCanvas(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return cc.srv.Canvases().CanvasExport(c, c.Param("canvas_id"))
	})
}

// ImportCanvas creates a new canvas from JSON export data.
// It never overwrites an existing canvas and does not create async tasks.
func (cc *CanvasController) ImportCanvas(c *gin.Context) {
	core.Run(c, &iapiserver.CanvasImportRequest{}, func(r *iapiserver.CanvasImportRequest) (any, error) {
		return cc.srv.Canvases().CanvasImport(c, r)
	})
}

// ExportWorkflow returns a JSON workflow fragment from the canvas graph.
// It accepts selected nodes/connections from the frontend or falls back to
// the full canvas graph; no asset binaries are embedded.
func (cc *CanvasController) ExportWorkflow(c *gin.Context) {
	req := &iapiserver.CanvasWorkflowExportRequest{}
	core.Run(c, req, func(r *iapiserver.CanvasWorkflowExportRequest) (any, error) {
		return cc.srv.Canvases().CanvasWorkflowExport(c, c.Param("canvas_id"), r)
	})
}

// ImportWorkflow merges a JSON workflow fragment into an existing canvas.
// It updates graph metadata synchronously and does not run generation tasks.
func (cc *CanvasController) ImportWorkflow(c *gin.Context) {
	req := &iapiserver.CanvasWorkflowImportRequest{}
	core.Run(c, req, func(r *iapiserver.CanvasWorkflowImportRequest) (any, error) {
		return cc.srv.Canvases().CanvasWorkflowImport(c, c.Param("canvas_id"), r)
	})
}

// ExportWorkflowPackage returns selected workflow JSON with referenced asset metadata.
// It does not return raw asset content; binary download must use the canvas-assets endpoint.
func (cc *CanvasController) ExportWorkflowPackage(c *gin.Context) {
	req := &iapiserver.CanvasWorkflowPackageExportRequest{}
	core.Run(c, req, func(r *iapiserver.CanvasWorkflowPackageExportRequest) (any, error) {
		return cc.srv.Canvases().CanvasWorkflowPackageExport(c, c.Param("canvas_id"), r)
	})
}

// ImportWorkflowPackage merges a workflow package into the current canvas.
// It updates graph metadata and creates an audit task, but does not run generation.
func (cc *CanvasController) ImportWorkflowPackage(c *gin.Context) {
	req := &iapiserver.CanvasWorkflowPackageImportRequest{}
	core.Run(c, req, func(r *iapiserver.CanvasWorkflowPackageImportRequest) (any, error) {
		return cc.srv.Canvases().CanvasWorkflowPackageImport(c, c.Param("canvas_id"), r)
	})
}

func (cc *CanvasController) TouchCanvas(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return cc.srv.Canvases().CanvasTouch(c, c.Param("canvas_id"))
	})
}

func (cc *CanvasController) DeleteCanvas(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		if err := cc.srv.Canvases().CanvasSoftDelete(c, c.Param("canvas_id")); err != nil {
			return nil, err
		}
		return gin.H{"ok": true}, nil
	})
}

func (cc *CanvasController) RestoreCanvas(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return cc.srv.Canvases().CanvasRestore(c, c.Param("canvas_id"))
	})
}

func (cc *CanvasController) PurgeCanvas(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		if err := cc.srv.Canvases().CanvasPurge(c, c.Param("canvas_id")); err != nil {
			return nil, err
		}
		return gin.H{"ok": true}, nil
	})
}

func (cc *CanvasController) ListProjects(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return cc.srv.Canvases().ProjectList(c)
	})
}

func (cc *CanvasController) CreateProject(c *gin.Context) {
	core.Run(c, &iapiserver.ProjectCreateRequest{}, func(r *iapiserver.ProjectCreateRequest) (any, error) {
		return cc.srv.Canvases().ProjectCreate(c, r)
	})
}

func (cc *CanvasController) UpdateProject(c *gin.Context) {
	req := &iapiserver.ProjectUpdateRequest{}
	req.ID = c.Param("project_id")
	core.Run(c, req, func(r *iapiserver.ProjectUpdateRequest) (any, error) {
		return cc.srv.Canvases().ProjectUpdate(c, r)
	})
}

func (cc *CanvasController) DeleteProject(c *gin.Context) {
	req := &iapiserver.ProjectDeleteRequest{}
	req.ID = c.Param("project_id")
	core.Run(c, req, func(r *iapiserver.ProjectDeleteRequest) (any, error) {
		moved, err := cc.srv.Canvases().ProjectDelete(c, r)
		if err != nil {
			return nil, err
		}
		return gin.H{"ok": true, "moved": moved}, nil
	})
}
