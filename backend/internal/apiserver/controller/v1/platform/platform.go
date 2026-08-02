package platform

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	srvv1 "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1"
	appplatformsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

type PlatformController struct {
	srv srvv1.Service
}

func NewController(storeIns store.Factory, dispatcher ...appplatformsvc.TaskDispatcher) *PlatformController {
	return &PlatformController{srv: srvv1.NewService(storeIns, dispatcher...)}
}

func (pc *PlatformController) Me(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return pc.srv.Platforms().Me(c)
	})
}

func (pc *PlatformController) ListProviders(c *gin.Context) {
	core.Run(c, &iapiserver.ProviderListRequest{}, func(r *iapiserver.ProviderListRequest) (any, error) {
		return pc.srv.Platforms().ProviderList(c, r)
	})
}

func (pc *PlatformController) CreateProvider(c *gin.Context) {
	core.Run(c, &iapiserver.ProviderCreateRequest{}, func(r *iapiserver.ProviderCreateRequest) (any, error) {
		return pc.srv.Platforms().ProviderCreate(c, r)
	})
}

func (pc *PlatformController) GetProvider(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return pc.srv.Platforms().ProviderGet(c, c.Param("provider_id"))
	})
}

func (pc *PlatformController) UpdateProvider(c *gin.Context) {
	req := &iapiserver.ProviderUpdateRequest{ID: c.Param("provider_id")}
	core.Run(c, req, func(r *iapiserver.ProviderUpdateRequest) (any, error) {
		return pc.srv.Platforms().ProviderUpdate(c, req)
	})

}

// DeleteProvider removes one provider and clears its related models and default model bindings.
// It returns sanitized provider metadata only and never returns credentials.
func (pc *PlatformController) DeleteProvider(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return pc.srv.Platforms().ProviderDelete(c, c.Param("provider_id"))
	})
}

// TestProvider checks a provider endpoint with optional unsaved API settings.
// It returns only connection metadata and never persists credentials or invokes async tasks.
func (pc *PlatformController) TestProvider(c *gin.Context) {
	req := &iapiserver.ProviderTestRequest{ID: c.Param("provider_id")}
	core.Run(c, req, func(r *iapiserver.ProviderTestRequest) (any, error) {
		return pc.srv.Platforms().ProviderTest(c, r)
	})
}

func (pc *PlatformController) TestUnsavedProvider(c *gin.Context) {
	core.Run(c, &iapiserver.ProviderTestRequest{}, func(r *iapiserver.ProviderTestRequest) (any, error) {
		return pc.srv.Platforms().ProviderTest(c, r)
	})
}

func (pc *PlatformController) ListProviderModels(c *gin.Context) {
	req := &iapiserver.ProviderModelListRequest{ProviderID: c.Param("provider_id")}
	core.Run(c, req, func(r *iapiserver.ProviderModelListRequest) (any, error) {
		return pc.srv.Platforms().ProviderModelList(c, r)
	})
}

func (pc *PlatformController) CreateProviderModel(c *gin.Context) {
	req := &iapiserver.ProviderModelCreateRequest{ProviderID: c.Param("provider_id")}
	core.Run(c, req, func(r *iapiserver.ProviderModelCreateRequest) (any, error) {
		return pc.srv.Platforms().ProviderModelCreate(c, r)
	})
}

func (pc *PlatformController) UpdateProviderModel(c *gin.Context) {
	req := &iapiserver.ProviderModelUpdateRequest{
		ID: c.Param("model_id"),
	}
	core.Run(c, req, func(r *iapiserver.ProviderModelUpdateRequest) (any, error) {
		return pc.srv.Platforms().ProviderModelUpdate(c, req)
	})
}

// DeleteProviderModel 删除一个模型提供商下的模型，并清理相关默认模型绑定。
// 该接口只删除模型元数据，不会返回或修改 provider credential，也不会返回 asset 原始内容。
func (pc *PlatformController) DeleteProviderModel(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return pc.srv.Platforms().ProviderModelDelete(c, "", c.Param("model_id"))
	})
}

// CheckProviderModelHealth 立即检测单个 provider model 的 metadata 连接状态。
// 该接口会保存健康状态和原因，但不会触发真实模型生成或返回 provider credential。
func (pc *PlatformController) CheckProviderModelHealth(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return pc.srv.Platforms().ProviderModelHealthCheck(c, "", c.Param("model_id"))
	})
}

func (pc *PlatformController) GetDefaultModel(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return pc.srv.Platforms().DefaultModelGet(c, c.Param("usage"))
	})
}

func (pc *PlatformController) PutDefaultModel(c *gin.Context) {
	req := &iapiserver.DefaultModelSaveRequest{}
	core.Run(c, req, func(r *iapiserver.DefaultModelSaveRequest) (any, error) {
		return pc.srv.Platforms().DefaultModelSave(c, c.Param("usage"), r)
	})
}

func (pc *PlatformController) ListModelOptions(c *gin.Context) {
	core.Run(c, &iapiserver.ProviderModelListRequest{}, func(r *iapiserver.ProviderModelListRequest) (any, error) {
		return pc.srv.Platforms().ModelOptionList(c, r)
	})
}

// SyncProviderModels imports remote OpenAI-compatible model metadata for one provider.
// It updates provider model metadata only and does not return raw provider credentials.
func (pc *PlatformController) SyncProviderModels(c *gin.Context) {
	req := &iapiserver.ProviderModelSyncRequest{ProviderID: c.Param("provider_id")}
	core.Run(c, req, func(r *iapiserver.ProviderModelSyncRequest) (any, error) {
		return pc.srv.Platforms().ProviderModelSync(c, r)
	})
}

func (pc *PlatformController) GetSystemLLMConfig(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return pc.srv.Platforms().SystemLLMConfigList(c)
	})
}

func (pc *PlatformController) PutSystemLLMConfig(c *gin.Context) {
	core.Run(
		c,
		&iapiserver.SystemLLMConfigUpsertRequest{},
		func(r *iapiserver.SystemLLMConfigUpsertRequest) (any, error) {
			return pc.srv.Platforms().SystemLLMConfigUpsert(c, r)
		},
	)
}

func (pc *PlatformController) UploadAsset(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		core.WriteResponse(c, err, nil)
		return
	}
	tags := splitTags(c.PostForm("tags"))
	sourceType := c.PostForm("source_type")
	ret, err := pc.srv.Platforms().AssetUpload(c, file, tags, sourceType)
	core.WriteResponse(c, err, ret)
}

// InitAssetChunkUpload prepares a resumable upload under a checksum-scoped temp directory.
func (pc *PlatformController) InitAssetChunkUpload(c *gin.Context) {
	core.Run(
		c,
		&iapiserver.AssetChunkUploadInitRequest{},
		func(r *iapiserver.AssetChunkUploadInitRequest) (any, error) {
			return pc.srv.Platforms().AssetChunkUploadInit(c, r)
		},
	)
}

// UploadAssetChunk writes one chunk. It does not create an asset until complete is called.
func (pc *PlatformController) UploadAssetChunk(c *gin.Context) {
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil {
		core.WriteResponse(c, errors.NewStatusF(code.ErrValidation, "invalid chunk index"), nil)
		return
	}
	ret, err := pc.srv.Platforms().AssetChunkUploadPart(c, c.Param("checksum"), index, c.Request.Body)
	core.WriteResponse(c, err, ret)
}

// CompleteAssetChunkUpload merges chunks, validates the final checksum, creates the asset, and removes temp chunks.
func (pc *PlatformController) CompleteAssetChunkUpload(c *gin.Context) {
	req := &iapiserver.AssetChunkUploadCompleteRequest{Checksum: c.Param("checksum")}
	core.Run(c, req, func(r *iapiserver.AssetChunkUploadCompleteRequest) (any, error) {
		return pc.srv.Platforms().AssetChunkUploadComplete(c, req)
	})
}

// CancelAssetChunkUpload removes a checksum-scoped temporary chunk directory.
func (pc *PlatformController) CancelAssetChunkUpload(c *gin.Context) {
	ret, err := pc.srv.Platforms().AssetChunkUploadCancel(c, c.Param("checksum"))
	core.WriteResponse(c, err, ret)
}

func (pc *PlatformController) ListAssets(c *gin.Context) {
	core.Run(c, &iapiserver.AssetListRequest{}, func(r *iapiserver.AssetListRequest) (any, error) {
		return pc.srv.Platforms().AssetList(c, r)
	})
}

func (pc *PlatformController) SearchAssets(c *gin.Context) {
	core.Run(c, &iapiserver.AssetSearchRequest{}, func(r *iapiserver.AssetSearchRequest) (any, error) {
		return pc.srv.Platforms().AssetSearch(c, r)
	})
}

func (pc *PlatformController) ParseAssetSearch(c *gin.Context) {
	core.Run(c, &iapiserver.AssetSearchParseRequest{}, func(r *iapiserver.AssetSearchParseRequest) (any, error) {
		return pc.srv.Platforms().AssetSearchParse(c, r)
	})
}

func (pc *PlatformController) GetAsset(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return pc.srv.Platforms().AssetGet(c, c.Param("asset_id"))
	})
}

func (pc *PlatformController) UpdateAsset(c *gin.Context) {
	req := &iapiserver.AssetUpdateRequest{ID: c.Param("asset_id")}
	core.Run(c, req, func(r *iapiserver.AssetUpdateRequest) (any, error) {
		return pc.srv.Platforms().AssetUpdate(c, req)
	})
}

// DeleteAsset marks one asset as deleted. It keeps raw content and thumbnail
// objects in storage, and the asset is hidden from default list/search results.
func (pc *PlatformController) DeleteAsset(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return pc.srv.Platforms().AssetDelete(c, c.Param("asset_id"))
	})
}

func (pc *PlatformController) GetAssetContent(c *gin.Context) {
	path, mimeType, err := pc.srv.Platforms().AssetContentPath(c, c.Param("asset_id"))
	if err != nil {
		core.WriteResponse(c, err, nil)
		return
	}
	if mimeType != "" {
		c.Header("Content-Type", mimeType)
	}
	c.File(path)
}

func (pc *PlatformController) GetAssetThumbnail(c *gin.Context) {
	path, mimeType, err := pc.srv.Platforms().AssetThumbnailPath(c, c.Param("asset_id"))
	if err != nil {
		core.WriteResponse(c, err, nil)
		return
	}
	if mimeType != "" {
		c.Header("Content-Type", mimeType)
	}
	c.File(path)
}

func (pc *PlatformController) CreateAssetGroup(c *gin.Context) {
	core.Run(c, &iapiserver.AssetGroupCreateRequest{}, func(r *iapiserver.AssetGroupCreateRequest) (any, error) {
		return pc.srv.Platforms().AssetGroupCreate(c, r)
	})
}

func (pc *PlatformController) RunCanvas(c *gin.Context) {
	core.Run(c, &iapiserver.CanvasNodeRunRequest{}, func(r *iapiserver.CanvasNodeRunRequest) (any, error) {
		return pc.srv.Platforms().CanvasNodeRun(c, c.Param("canvas_id"), "", r)
	})
}

// DownloadCanvasAssets streams selected asset contents as a zip archive.
// It accepts asset IDs only, does not expose local paths, and skips no security checks.
func (pc *PlatformController) DownloadCanvasAssets(c *gin.Context) {
	req := &iapiserver.CanvasAssetDownloadRequest{}
	if err := c.ShouldBindJSON(req); err != nil {
		core.WriteResponse(c, err, nil)
		return
	}
	filename := strings.TrimSpace(req.Filename)
	if filename == "" {
		filename = "canvas-assets.zip"
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".zip") {
		filename += ".zip"
	}
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(filename, `"`, `_`)+`"`)
	if err := pc.srv.Platforms().CanvasAssetDownloadZip(c, req, c.Writer); err != nil {
		core.WriteResponse(c, err, nil)
	}
}

// RegisterCanvasOutput records an existing asset as a canvas output reference.
// It returns metadata only and creates an async audit task.
func (pc *PlatformController) RegisterCanvasOutput(c *gin.Context) {
	core.Run(
		c,
		&iapiserver.CanvasAssetRegisterOutputRequest{},
		func(r *iapiserver.CanvasAssetRegisterOutputRequest) (any, error) {
			return pc.srv.Platforms().CanvasAssetRegisterOutput(c, r)
		},
	)
}

// RunCanvasNode creates a task for one canvas node execution.
// Provider-specific execution is handled by workers through the task input.
func (pc *PlatformController) RunCanvasNode(c *gin.Context) {
	req := &iapiserver.CanvasNodeRunRequest{}
	core.Run(c, req, func(r *iapiserver.CanvasNodeRunRequest) (any, error) {
		return pc.srv.Platforms().CanvasNodeRun(c, c.Param("canvas_id"), c.Param("node_id"), r)
	})
}

// BatchApplyAssetLabels applies manual labels independently per asset and returns partial-success results.
func (pc *PlatformController) BatchApplyAssetLabels(c *gin.Context) {
	core.Run(c, &iapiserver.BatchLabelRequest{}, func(req *iapiserver.BatchLabelRequest) (any, error) {
		return pc.srv.Platforms().BatchApplyAssetLabels(c, req)
	})
}

// RegisterArtifact idempotently registers one ApplicationRun artifact as a UserAsset metadata record.
func (pc *PlatformController) RegisterArtifact(c *gin.Context) {
	core.Run(c, &iapiserver.ArtifactRegistrationRequest{}, func(req *iapiserver.ArtifactRegistrationRequest) (any, error) {
		return pc.srv.Platforms().RegisterArtifact(c, req)
	})
}

func splitTags(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；' || r == '\n' || r == '\t'
	})
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			tags = append(tags, part)
		}
	}
	return tags
}
