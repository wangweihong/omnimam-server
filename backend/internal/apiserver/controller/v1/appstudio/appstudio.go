package appstudio

import (
	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	appstudiosvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/appstudio"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

type Controller struct{ service *appstudiosvc.Service }

func NewController(service *appstudiosvc.Service) *Controller { return &Controller{service: service} }

func (c *Controller) ListApplications(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioApplicationListRequest{}, func(req *iapiserver.StudioApplicationListRequest) (any, error) {
		return c.service.ListApplications(ctx, req)
	})
}
func (c *Controller) CreateApplication(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioApplicationCreateRequest{}, func(req *iapiserver.StudioApplicationCreateRequest) (any, error) {
		return c.service.CreateApplication(ctx, req)
	})
}
func (c *Controller) GetApplication(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetApplication(ctx, ctx.Param("studio_application_id")) })
}
func (c *Controller) UpdateApplication(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioApplicationUpdateRequest{}, func(req *iapiserver.StudioApplicationUpdateRequest) (any, error) {
		return c.service.UpdateApplication(ctx, ctx.Param("studio_application_id"), req)
	})
}
func (c *Controller) ArchiveApplication(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.ArchiveApplication(ctx, ctx.Param("studio_application_id")) })
}
func (c *Controller) GetSource(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		return c.service.GetSource(ctx, ctx.Param("studio_application_id"))
	})
}
func (c *Controller) ListFiles(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioSourceFileListRequest{}, func(req *iapiserver.StudioSourceFileListRequest) (any, error) {
		return c.service.ListFiles(ctx, ctx.Param("studio_application_id"), req)
	})
}
func (c *Controller) GetFileContent(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioFileContentRequest{}, func(req *iapiserver.StudioFileContentRequest) (any, error) {
		return c.service.GetFileContent(ctx, ctx.Param("studio_application_id"), req)
	})
}
func (c *Controller) ApplyChangeSet(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioChangeSetRequest{}, func(req *iapiserver.StudioChangeSetRequest) (any, error) {
		return c.service.ApplyChangeSet(ctx, ctx.Param("studio_application_id"), req)
	})
}
func (c *Controller) RestoreRevision(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioRestoreRevisionRequest{}, func(req *iapiserver.StudioRestoreRevisionRequest) (any, error) {
		return c.service.RestoreRevision(ctx, ctx.Param("studio_application_id"), req)
	})
}
func (c *Controller) SearchSource(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioSourceSearchRequest{}, func(req *iapiserver.StudioSourceSearchRequest) (any, error) {
		return c.service.SearchSource(ctx, ctx.Param("studio_application_id"), req)
	})
}
func (c *Controller) CreateSnapshot(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioSnapshotRequest{}, func(req *iapiserver.StudioSnapshotRequest) (any, error) {
		return c.service.CreateSnapshot(ctx, ctx.Param("studio_application_id"), req)
	})
}
func (c *Controller) GetSnapshot(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		return c.service.GetSnapshot(ctx, ctx.Param("studio_application_id"), ctx.Param("source_snapshot_id"))
	})
}
func (c *Controller) CreateVersion(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioApplicationVersionCreateRequest{}, func(req *iapiserver.StudioApplicationVersionCreateRequest) (any, error) {
		return c.service.CreateVersion(ctx, ctx.Param("studio_application_id"), req)
	})
}
func (c *Controller) ListVersions(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioApplicationVersionListRequest{}, func(req *iapiserver.StudioApplicationVersionListRequest) (any, error) {
		return c.service.ListVersions(ctx, ctx.Param("studio_application_id"), req)
	})
}
func (c *Controller) ListBuilds(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioBuildListRequest{}, func(req *iapiserver.StudioBuildListRequest) (any, error) {
		return c.service.ListBuilds(ctx, ctx.Param("studio_application_id"), req)
	})
}
func (c *Controller) CreateBuild(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioBuildRequest{}, func(req *iapiserver.StudioBuildRequest) (any, error) {
		return c.service.CreateBuild(ctx, ctx.Param("studio_application_id"), req)
	})
}
func (c *Controller) GetBuild(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetBuild(ctx, ctx.Param("studio_build_id")) })
}
func (c *Controller) CancelBuild(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioActionRequest{}, func(req *iapiserver.StudioActionRequest) (any, error) {
		return c.service.CancelBuild(ctx, ctx.Param("studio_build_id"), req)
	})
}
func (c *Controller) BuildLogs(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.BuildLogs(ctx, ctx.Param("studio_build_id")) })
}
func (c *Controller) GetPreview(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetPreview(ctx, ctx.Param("studio_application_id")) })
}
func (c *Controller) RefreshPreview(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioPreviewRequest{}, func(req *iapiserver.StudioPreviewRequest) (any, error) {
		return c.service.RefreshPreview(ctx, ctx.Param("studio_application_id"), req)
	})
}
func (c *Controller) StopPreview(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioActionRequest{}, func(req *iapiserver.StudioActionRequest) (any, error) {
		return c.service.StopPreview(ctx, ctx.Param("studio_application_id"), req)
	})
}
func (c *Controller) GetRuntimeConfig(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		return c.service.GetRuntimeConfig(ctx, ctx.Param("studio_application_version_id"), ctx.Param("environment"))
	})
}
func (c *Controller) ReplaceRuntimeConfig(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioRuntimeConfigRequest{}, func(req *iapiserver.StudioRuntimeConfigRequest) (any, error) {
		return c.service.ReplaceRuntimeConfig(ctx, ctx.Param("studio_application_version_id"), ctx.Param("environment"), req)
	})
}
func (c *Controller) ListReleases(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioReleaseListRequest{}, func(req *iapiserver.StudioReleaseListRequest) (any, error) {
		return c.service.ListReleases(ctx, ctx.Param("studio_application_id"), req)
	})
}
func (c *Controller) CreateRelease(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioReleaseRequest{}, func(req *iapiserver.StudioReleaseRequest) (any, error) {
		return c.service.CreateRelease(ctx, ctx.Param("studio_application_id"), req)
	})
}
func (c *Controller) GetRelease(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetRelease(ctx, ctx.Param("studio_release_id")) })
}
func (c *Controller) RollbackRelease(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioActionRequest{}, func(req *iapiserver.StudioActionRequest) (any, error) {
		return c.service.RollbackRelease(ctx, ctx.Param("studio_release_id"), req)
	})
}
func (c *Controller) ListRuntimeInstances(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioRuntimeInstanceListRequest{}, func(req *iapiserver.StudioRuntimeInstanceListRequest) (any, error) {
		return c.service.ListRuntimeInstances(ctx, ctx.Param("studio_application_id"), req)
	})
}
func (c *Controller) GetRuntimeInstance(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		return c.service.GetRuntimeInstance(ctx, ctx.Param("studio_runtime_instance_id"))
	})
}
func (c *Controller) StopRuntimeInstance(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioActionRequest{}, func(req *iapiserver.StudioActionRequest) (any, error) {
		return c.service.StopRuntimeInstance(ctx, ctx.Param("studio_runtime_instance_id"), req)
	})
}
func (c *Controller) RuntimeLogs(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.RuntimeLogs(ctx, ctx.Param("studio_runtime_instance_id")) })
}
