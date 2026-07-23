package assetlibrary

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	assetlibrarysvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/assetlibrary"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

// Controller 将 spec-v1.7.4 asset-library HTTP 契约映射到领域 service。
type Controller struct{ service assetlibrarysvc.Service }

func New(service assetlibrarysvc.Service) *Controller { return &Controller{service: service} }

func (c *Controller) ListAssets(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.UserAssetListRequest{}, func(req *iapiserver.UserAssetListRequest) (any, error) { return c.service.ListAssets(ctx, req) })
}
func (c *Controller) CreateAsset(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.CreateCanonicalAssetRequest{}, func(req *iapiserver.CreateCanonicalAssetRequest) (any, error) { return c.service.CreateAsset(ctx, req) })
}
func (c *Controller) GetAsset(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) { return c.service.GetAsset(ctx, ctx.Param("asset_id")) })
}
func (c *Controller) UpdateAsset(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.UpdateUserAssetRequest{}, func(req *iapiserver.UpdateUserAssetRequest) (any, error) {
		return c.service.UpdateAsset(ctx, ctx.Param("asset_id"), req)
	})
}
func (c *Controller) DeleteAsset(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) { return c.service.DeleteAsset(ctx, ctx.Param("asset_id")) })
}
func (c *Controller) RestoreAsset(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) { return c.service.RestoreAsset(ctx, ctx.Param("asset_id")) })
}
func (c *Controller) PermanentlyDeleteAsset(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) { return c.service.PermanentlyDeleteAsset(ctx, ctx.Param("asset_id")) })
}
func (c *Controller) BatchLabels(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.BatchLabelRequest{}, func(req *iapiserver.BatchLabelRequest) (any, error) { return c.service.BatchLabels(ctx, req) })
}

func (c *Controller) CreateUploads(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.CreateAssetUploadsRequest{}, func(req *iapiserver.CreateAssetUploadsRequest) (any, error) { return c.service.CreateUploads(ctx, req) })
}
func (c *Controller) UploadContent(ctx *gin.Context) {
	part, _ := strconv.Atoi(ctx.Query("part_number"))
	result, err := c.service.UploadContent(ctx, ctx.Param("upload_id"), part, ctx.GetHeader("part_sha256"), ctx.GetHeader("content_range"), ctx.Request.Body)
	core.WriteResponse(ctx, err, result)
}
func (c *Controller) CompleteUpload(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.CompleteAssetUploadRequest{}, func(req *iapiserver.CompleteAssetUploadRequest) (any, error) {
		return c.service.CompleteUpload(ctx, ctx.Param("upload_id"), req)
	})
}
func (c *Controller) CancelUpload(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) { return c.service.CancelUpload(ctx, ctx.Param("upload_id")) })
}

// GetBlob 返回管理员可见的完整 object key，不递归嵌入 StorageBackend 配置。
func (c *Controller) GetBlob(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) { return c.service.GetBlob(ctx, ctx.Param("blob_id")) })
}

// ListStorageBackends 返回一次查询生成的 items 及兼容 backends 别名，包含完整 root 和 config。
func (c *Controller) ListStorageBackends(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StorageBackendListRequest{}, func(req *iapiserver.StorageBackendListRequest) (any, error) {
		return c.service.ListStorageBackends(ctx, req)
	})
}

// CreateStorageBackend 创建全局存储后端配置；service 在任何 store 写入前执行管理员鉴权。
func (c *Controller) CreateStorageBackend(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StorageBackendCreateRequest{}, func(req *iapiserver.StorageBackendCreateRequest) (any, error) {
		return c.service.CreateStorageBackend(ctx, req)
	})
}

// GetStorageBackend 返回管理员可见的完整 root 和 config。
func (c *Controller) GetStorageBackend(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) { return c.service.GetStorageBackend(ctx, ctx.Param("backend_id")) })
}

// UpdateStorageBackend 更新全局存储后端配置；service 在任何 store 读取前执行管理员鉴权。
func (c *Controller) UpdateStorageBackend(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StorageBackendUpdateRequest{}, func(req *iapiserver.StorageBackendUpdateRequest) (any, error) {
		return c.service.UpdateStorageBackend(ctx, ctx.Param("backend_id"), req)
	})
}

func (c *Controller) ListCollections(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.CollectionListRequest{}, func(req *iapiserver.CollectionListRequest) (any, error) { return c.service.ListCollections(ctx, req) })
}
func (c *Controller) CreateCollection(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.CreateCollectionRequest{}, func(req *iapiserver.CreateCollectionRequest) (any, error) {
		return c.service.CreateCollection(ctx, req)
	})
}
func (c *Controller) GetCollection(ctx *gin.Context) {
	core.Run(ctx, &imachinery.PagingParams{}, func(req *imachinery.PagingParams) (any, error) {
		return c.service.GetCollection(ctx, ctx.Param("collection_id"), *req)
	})
}
func (c *Controller) UpdateCollection(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.UpdateCollectionRequest{}, func(req *iapiserver.UpdateCollectionRequest) (any, error) {
		return c.service.UpdateCollection(ctx, ctx.Param("collection_id"), req)
	})
}
func (c *Controller) DeleteCollection(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) { return c.service.DeleteCollection(ctx, ctx.Param("collection_id")) })
}
func (c *Controller) AddCollectionItems(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AddCollectionItemsRequest{}, func(req *iapiserver.AddCollectionItemsRequest) (any, error) {
		return c.service.AddCollectionItems(ctx, ctx.Param("collection_id"), req)
	})
}
func (c *Controller) UpdateCollectionItem(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.UpdateCollectionItemRequest{}, func(req *iapiserver.UpdateCollectionItemRequest) (any, error) {
		return c.service.UpdateCollectionItem(ctx, ctx.Param("collection_id"), ctx.Param("item_id"), req)
	})
}
func (c *Controller) DeleteCollectionItem(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) {
		return c.service.DeleteCollectionItem(ctx, ctx.Param("collection_id"), ctx.Param("item_id"))
	})
}

func (c *Controller) ReplaceLabels(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.ReplaceLabelsRequest{}, func(req *iapiserver.ReplaceLabelsRequest) (any, error) {
		return c.service.ReplaceLabels(ctx, ctx.Param("asset_id"), req)
	})
}
func (c *Controller) DeleteLabel(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) {
		return c.service.DeleteLabel(ctx, ctx.Param("asset_id"), ctx.Param("label_id"))
	})
}
func (c *Controller) AddTags(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AddTagsRequest{}, func(req *iapiserver.AddTagsRequest) (any, error) {
		return c.service.AddTags(ctx, ctx.Param("asset_id"), req)
	})
}
func (c *Controller) DeleteTag(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) { return c.service.DeleteTag(ctx, ctx.Param("asset_id"), ctx.Param("tag_id")) })
}

func (c *Controller) ListArtifacts(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.ArtifactListRequest{}, func(req *iapiserver.ArtifactListRequest) (any, error) { return c.service.ListArtifacts(ctx, req) })
}

// BatchArtifactSummaries 返回当前主体可见的有界 Artifact 一跳摘要，不区分缺失与不可见目标。
func (c *Controller) BatchArtifactSummaries(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.BatchArtifactSummaryRequest{}, func(req *iapiserver.BatchArtifactSummaryRequest) (any, error) {
		return c.service.BatchArtifactSummaries(ctx, req)
	})
}
func (c *Controller) CreateArtifact(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.CreateArtifactRequest{}, func(req *iapiserver.CreateArtifactRequest) (any, error) { return c.service.CreateArtifact(ctx, req) })
}
func (c *Controller) GetArtifact(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) { return c.service.GetArtifact(ctx, ctx.Param("artifact_id")) })
}
func (c *Controller) DeleteArtifact(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) { return c.service.DeleteArtifact(ctx, ctx.Param("artifact_id")) })
}
func (c *Controller) UploadArtifactContent(ctx *gin.Context) {
	result, err := c.service.UploadArtifactContent(ctx, ctx.Param("artifact_id"), ctx.GetHeader("Content-Type"), ctx.Request.Body)
	core.WriteResponse(ctx, err, result)
}
func (c *Controller) CompleteArtifact(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.CompleteArtifactRequest{}, func(req *iapiserver.CompleteArtifactRequest) (any, error) {
		return c.service.CompleteArtifact(ctx, ctx.Param("artifact_id"), req)
	})
}
func (c *Controller) RegisterArtifact(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.RegisterArtifactRequest{}, func(req *iapiserver.RegisterArtifactRequest) (any, error) {
		return c.service.RegisterArtifact(ctx, ctx.Param("artifact_id"), req)
	})
}
func (c *Controller) RegisterArtifactCompat(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.ArtifactRegistrationRequest{}, func(req *iapiserver.ArtifactRegistrationRequest) (any, error) {
		mode, name := req.Mode, req.Name
		if mode == "" {
			mode = "create_asset"
		}
		if name == "" {
			name = req.OutputName
		}
		return c.service.RegisterArtifact(ctx, req.ArtifactID, &iapiserver.RegisterArtifactRequest{Mode: mode, AssetID: req.AssetID, Name: name, VersionNote: req.VersionNote, ProfileVersion: req.ProfileVersion})
	})
}

func (c *Controller) ListVersions(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) { return c.service.ListVersions(ctx, ctx.Param("asset_id")) })
}
func (c *Controller) CreateVersion(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.CreateCanonicalVersionRequest{}, func(req *iapiserver.CreateCanonicalVersionRequest) (any, error) {
		return c.service.CreateVersion(ctx, ctx.Param("asset_id"), req)
	})
}
func (c *Controller) GetVersion(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) { return c.service.GetVersion(ctx, ctx.Param("version_id")) })
}
func (c *Controller) SetCurrentVersion(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) {
		return c.service.SetCurrentVersion(ctx, ctx.Param("asset_id"), ctx.Param("version_id"))
	})
}
func (c *Controller) ListRepresentations(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) { return c.service.ListRepresentations(ctx, ctx.Param("version_id")) })
}
func (c *Controller) RegisterRepresentation(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.RegisterRepresentationRequest{}, func(req *iapiserver.RegisterRepresentationRequest) (any, error) {
		return c.service.RegisterRepresentation(ctx, ctx.Param("version_id"), req)
	})
}
func (c *Controller) GetRepresentation(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) { return c.service.GetRepresentation(ctx, ctx.Param("representation_id")) })
}
func (c *Controller) ReadRepresentation(ctx *gin.Context) {
	content, err := c.service.ReadRepresentation(ctx, ctx.Param("representation_id"))
	if err != nil {
		core.WriteResponse(ctx, err, nil)
		return
	}
	disposition := contentDisposition(ctx.Query("disposition"))
	filename := strings.ReplaceAll(filepath.Base(content.Filename), `"`, `_`)
	ctx.Header("Content-Type", content.MIMEType)
	ctx.Header("Content-Length", strconv.FormatInt(content.SizeBytes, 10))
	ctx.Header("ETag", `"`+content.SHA256+`"`)
	ctx.Header("Content-Disposition", fmt.Sprintf(`%s; filename=%s`, disposition, mime.QEncoding.Encode("UTF-8", filename)))
	ctx.Status(http.StatusOK)
	_, copyErr := io.Copy(ctx.Writer, content.Reader)
	closeErr := content.Reader.Close()
	if copyErr != nil {
		ctx.Error(copyErr)
	}
	if closeErr != nil {
		ctx.Error(closeErr)
	}
}
func (c *Controller) RepresentationAccess(ctx *gin.Context) {
	disposition := contentDisposition(ctx.Query("disposition"))
	core.Run(ctx, nil, func(_ any) (any, error) {
		return c.service.RepresentationAccess(ctx, ctx.Param("representation_id"), disposition)
	})
}

func contentDisposition(value string) string {
	if value == "attachment" {
		return value
	}
	return "inline"
}

// ListRelations 返回当前用户可见素材关系的分页列表，响应固定为 total/items，不返回 Lineage 图。
func (c *Controller) ListRelations(ctx *gin.Context) {
	core.Run(ctx, &imachinery.PagingParams{}, func(req *imachinery.PagingParams) (any, error) {
		return c.service.ListRelations(ctx, ctx.Param("asset_id"), *req)
	})
}

// Lineage 返回当前用户可见素材的来源图，响应固定为 asset_id/nodes/edges，不使用分页列表结构。
func (c *Controller) Lineage(ctx *gin.Context) {
	core.Run(ctx, nil, func(_ any) (any, error) { return c.service.Lineage(ctx, ctx.Param("asset_id")) })
}
func (c *Controller) ListReferences(ctx *gin.Context) {
	core.Run(ctx, &imachinery.PagingParams{}, func(req *imachinery.PagingParams) (any, error) {
		return c.service.ListReferences(ctx, ctx.Param("asset_id"), *req)
	})
}
func (c *Controller) ListUsages(ctx *gin.Context) {
	core.Run(ctx, &imachinery.PagingParams{}, func(req *imachinery.PagingParams) (any, error) {
		return c.service.ListUsages(ctx, ctx.Param("asset_id"), *req)
	})
}
