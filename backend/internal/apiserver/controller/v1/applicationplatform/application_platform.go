package applicationplatform

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	appservice "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type Controller struct {
	service appservice.ApplicationPlatformSrv
}

func NewController(service appservice.ApplicationPlatformSrv) *Controller {
	return &Controller{service: service}
}

func (c *Controller) ListProviderCapabilities(ctx *gin.Context) {
	run(ctx, &iapiserver.ProviderCapabilityListRequest{}, func(r *iapiserver.ProviderCapabilityListRequest) (any, error) {
		return c.service.ListProviderCapabilities(ctx, r)
	})
}
func (c *Controller) GetProviderCapability(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) {
		return c.service.GetProviderCapability(ctx, ctx.Param("provider_capability_id"))
	})
}
func (c *Controller) ListProviderCapabilityLoadResults(ctx *gin.Context) {
	run(ctx, &iapiserver.ProviderCapabilityLoadResultListRequest{}, func(r *iapiserver.ProviderCapabilityLoadResultListRequest) (any, error) {
		return c.service.ListProviderCapabilityLoadResults(ctx, r)
	})
}
func (c *Controller) ListApplicationEngineTypes(ctx *gin.Context) {
	run(ctx, &iapiserver.ApplicationEngineTypeListRequest{}, func(r *iapiserver.ApplicationEngineTypeListRequest) (any, error) {
		return c.service.ListApplicationEngineTypes(ctx, r)
	})
}

func (c *Controller) ListEngineInstances(ctx *gin.Context) {
	run(ctx, &iapiserver.EngineInstanceListRequest{}, func(r *iapiserver.EngineInstanceListRequest) (any, error) {
		return c.service.ListEngineInstances(ctx, r)
	})
}
func (c *Controller) CreateEngineInstance(ctx *gin.Context) {
	run(ctx, &iapiserver.EngineInstanceCreateRequest{}, func(r *iapiserver.EngineInstanceCreateRequest) (any, error) {
		return c.service.CreateEngineInstance(ctx, r)
	})
}
func (c *Controller) GetEngineInstance(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) { return c.service.GetEngineInstance(ctx, ctx.Param("engine_instance_id")) })
}
func (c *Controller) UpdateEngineInstance(ctx *gin.Context) {
	req := &iapiserver.EngineInstanceUpdateRequest{ID: ctx.Param("engine_instance_id")}
	run(ctx, req, func(r *iapiserver.EngineInstanceUpdateRequest) (any, error) {
		return c.service.UpdateEngineInstance(ctx, r)
	})
}
func (c *Controller) DeleteEngineInstance(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) { return c.service.DeleteEngineInstance(ctx, ctx.Param("engine_instance_id")) })
}
func (c *Controller) CheckEngineInstanceHealth(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) {
		return c.service.CheckEngineInstanceHealth(ctx, ctx.Param("engine_instance_id"))
	})
}

// GetComfyUIEngineObjectInfo 返回原始目录，并按 Accept-Encoding 协商 gzip。
func (c *Controller) GetComfyUIEngineObjectInfo(ctx *gin.Context) {
	result, err := c.service.GetComfyUIEngineObjectInfo(ctx, ctx.Param("engine_instance_id"))
	if err != nil {
		writeResponse(ctx, err, nil)
		return
	}
	ctx.Header("Vary", "Accept-Encoding")
	if !acceptsGzip(ctx.GetHeader("Accept-Encoding")) {
		ctx.JSON(http.StatusOK, result)
		return
	}
	ctx.Header("Content-Encoding", "gzip")
	ctx.Header("Content-Type", "application/json; charset=utf-8")
	ctx.Status(http.StatusOK)
	writer := gzip.NewWriter(ctx.Writer)
	if err := json.NewEncoder(writer).Encode(result); err != nil {
		_ = writer.Close()
		return
	}
	_ = writer.Close()
}

func acceptsGzip(header string) bool {
	for _, encoding := range strings.Split(header, ",") {
		parts := strings.Split(encoding, ";")
		name := strings.TrimSpace(strings.ToLower(parts[0]))
		if name != "gzip" && name != "*" {
			continue
		}
		quality := 1.0
		for _, parameter := range parts[1:] {
			key, value, ok := strings.Cut(strings.TrimSpace(parameter), "=")
			if ok && strings.EqualFold(key, "q") {
				if parsed, err := strconv.ParseFloat(value, 64); err == nil {
					quality = parsed
				}
			}
		}
		return quality > 0
	}
	return false
}

// RefreshComfyUIEngineObjectInfo 刷新单个实例的当前目录并返回轻量状态。
func (c *Controller) RefreshComfyUIEngineObjectInfo(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) {
		return c.service.RefreshComfyUIEngineObjectInfo(ctx, ctx.Param("engine_instance_id"))
	})
}

func (c *Controller) ListComfyUIWorkflows(ctx *gin.Context) {
	run(ctx, &iapiserver.ComfyUIWorkflowListRequest{}, func(r *iapiserver.ComfyUIWorkflowListRequest) (any, error) {
		return c.service.ListComfyUIWorkflows(ctx, r)
	})
}
func (c *Controller) ImportComfyUIWorkflow(ctx *gin.Context) {
	req := &iapiserver.ComfyUIWorkflowImportRequest{Name: ctx.PostForm("name"), Description: ctx.PostForm("description")}
	if sourceHeader, sourceErr := ctx.FormFile("workflow_file"); sourceErr == nil {
		req.WorkflowFile = sourceHeader
		req.SourceWorkflow, req.SourceWorkflowRaw, sourceErr = decodeWorkflowFile(sourceHeader)
		if sourceErr != nil {
			writeResponse(ctx, errors.NewStatus(code.ErrAIAppComfyUIWorkflowFileInvalid, sourceErr.Error()), nil)
			return
		}
	}
	if req.WorkflowFile != nil {
		if validationErr := req.Validate(); validationErr != nil {
			writeResponse(ctx, errors.NewStatus(code.ErrAIAppComfyUIWorkflowFileInvalid, validationErr.Error()), nil)
			return
		}
		result, importErr := c.service.ImportComfyUIWorkflow(ctx, req)
		writeResponse(ctx, importErr, result)
		return
	}
	apiHeader, err := ctx.FormFile("api_workflow_file")
	if err != nil {
		writeResponse(ctx, errors.NewStatus(code.ErrAIAppComfyUIWorkflowFileInvalid, "api_workflow_file is required"), nil)
		return
	}
	req.APIWorkflowFile = apiHeader
	req.APIWorkflow, req.APIWorkflowRaw, err = decodeWorkflowFile(apiHeader)
	if err != nil {
		writeResponse(ctx, errors.NewStatus(code.ErrAIAppComfyUIWorkflowFileInvalid, err.Error()), nil)
		return
	}
	if visualHeader, visualErr := ctx.FormFile("visual_workflow_file"); visualErr == nil {
		req.VisualWorkflowFile = visualHeader
		req.VisualWorkflow, _, err = decodeWorkflowFile(visualHeader)
		if err != nil {
			writeResponse(ctx, errors.NewStatus(code.ErrAIAppComfyUIWorkflowFileInvalid, "visual workflow: "+err.Error()), nil)
			return
		}
	}
	if validationErr := req.Validate(); validationErr != nil {
		writeResponse(ctx, errors.NewStatus(code.ErrAIAppComfyUIWorkflowFileInvalid, validationErr.Error()), nil)
		return
	}
	result, err := c.service.ImportComfyUIWorkflow(ctx, req)
	writeResponse(ctx, err, result)
}
func (c *Controller) GetComfyUIWorkflow(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) { return c.service.GetComfyUIWorkflow(ctx, ctx.Param("workflow_id")) })
}
func (c *Controller) UpdateComfyUIWorkflow(ctx *gin.Context) {
	req := &iapiserver.ComfyUIWorkflowUpdateRequest{ID: ctx.Param("workflow_id")}
	run(ctx, req, func(r *iapiserver.ComfyUIWorkflowUpdateRequest) (any, error) {
		return c.service.UpdateComfyUIWorkflow(ctx, r)
	})
}
func (c *Controller) ListComfyUIWorkflowNodes(ctx *gin.Context) {
	run(ctx, &iapiserver.ComfyUIWorkflowDeriveRequest{}, func(r *iapiserver.ComfyUIWorkflowDeriveRequest) (any, error) {
		return c.service.ListComfyUIWorkflowNodes(ctx, ctx.Param("workflow_id"), r)
	})
}
func (c *Controller) ListComfyUIWorkflowInputCandidates(ctx *gin.Context) {
	run(ctx, &iapiserver.ComfyUIWorkflowDeriveRequest{}, func(r *iapiserver.ComfyUIWorkflowDeriveRequest) (any, error) {
		return c.service.ListComfyUIWorkflowInputCandidates(ctx, ctx.Param("workflow_id"), r.EngineInstanceID)
	})
}
func (c *Controller) ListComfyUIWorkflowOutputCandidates(ctx *gin.Context) {
	run(ctx, &iapiserver.ComfyUIWorkflowDeriveRequest{}, func(r *iapiserver.ComfyUIWorkflowDeriveRequest) (any, error) {
		return c.service.ListComfyUIWorkflowOutputCandidates(ctx, ctx.Param("workflow_id"), r.EngineInstanceID)
	})
}
func (c *Controller) ListComfyUIWorkflowDependencies(ctx *gin.Context) {
	run(ctx, &iapiserver.ComfyUIWorkflowDeriveRequest{}, func(r *iapiserver.ComfyUIWorkflowDeriveRequest) (any, error) {
		return c.service.ListComfyUIWorkflowDependencies(ctx, ctx.Param("workflow_id"), r.EngineInstanceID)
	})
}
func (c *Controller) ListComfyUIWorkflowValidations(ctx *gin.Context) {
	req := &iapiserver.ComfyUIWorkflowValidationListRequest{WorkflowID: ctx.Param("workflow_id")}
	run(ctx, req, func(r *iapiserver.ComfyUIWorkflowValidationListRequest) (any, error) {
		return c.service.ListComfyUIWorkflowValidations(ctx, r)
	})
}
func (c *Controller) ValidateComfyUIWorkflow(ctx *gin.Context) {
	run(ctx, &iapiserver.ComfyUIWorkflowValidationCreateRequest{}, func(r *iapiserver.ComfyUIWorkflowValidationCreateRequest) (any, error) {
		return c.service.ValidateComfyUIWorkflow(ctx, ctx.Param("workflow_id"), r)
	})
}
func (c *Controller) GetComfyUIWorkflowValidation(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) {
		return c.service.GetComfyUIWorkflowValidation(ctx, ctx.Param("workflow_validation_id"))
	})
}
func (c *Controller) ConvertComfyUIWorkflow(ctx *gin.Context) {
	run(ctx, &iapiserver.ComfyUIWorkflowConvertRequest{}, func(r *iapiserver.ComfyUIWorkflowConvertRequest) (any, error) {
		return c.service.ConvertComfyUIWorkflow(ctx, ctx.Param("workflow_id"), r)
	})
}
func (c *Controller) ConvertComfyUIWorkflowToAPI(ctx *gin.Context) {
	run(ctx, &iapiserver.ComfyUIWorkflowAPIConversionRequest{}, func(r *iapiserver.ComfyUIWorkflowAPIConversionRequest) (any, error) {
		return c.service.ConvertComfyUIWorkflowToAPI(ctx, ctx.Param("workflow_id"), r)
	})
}
func (c *Controller) ListComfyUIWorkflowTestRuns(ctx *gin.Context) {
	req := &iapiserver.ComfyUIWorkflowTestRunListRequest{WorkflowID: ctx.Param("workflow_id")}
	run(ctx, req, func(r *iapiserver.ComfyUIWorkflowTestRunListRequest) (any, error) {
		return c.service.ListComfyUIWorkflowTestRuns(ctx, r)
	})
}
func (c *Controller) CreateComfyUIWorkflowTestRun(ctx *gin.Context) {
	run(ctx, &iapiserver.ComfyUIWorkflowTestRunCreateRequest{}, func(r *iapiserver.ComfyUIWorkflowTestRunCreateRequest) (any, error) {
		return c.service.CreateComfyUIWorkflowTestRun(ctx, ctx.Param("workflow_id"), r)
	})
}
func (c *Controller) GetComfyUIWorkflowTestRun(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) { return c.service.GetComfyUIWorkflowTestRun(ctx, ctx.Param("test_run_id")) })
}
func (c *Controller) CancelComfyUIWorkflowTestRun(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) { return c.service.CancelComfyUIWorkflowTestRun(ctx, ctx.Param("test_run_id")) })
}
func (c *Controller) GetComfyUIWorkflowTestOutputContent(ctx *gin.Context) {
	content, contentType, err := c.service.GetComfyUIWorkflowTestOutputContent(ctx, ctx.Param("test_run_id"), ctx.Param("output_id"))
	if err != nil {
		writeResponse(ctx, err, nil)
		return
	}
	ctx.Data(http.StatusOK, contentType, content)
}

func decodeWorkflowFile(header *multipart.FileHeader) (map[string]any, []byte, error) {
	file, err := header.Open()
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	const maximumWorkflowFileSize = 32 << 20
	raw, err := io.ReadAll(io.LimitReader(file, maximumWorkflowFileSize+1))
	if err != nil {
		return nil, nil, err
	}
	if len(raw) > maximumWorkflowFileSize {
		return nil, nil, errors.New("workflow file exceeds 32 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		return nil, nil, err
	}
	if len(value) == 0 {
		return nil, nil, errors.New("workflow JSON object is empty")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, nil, errors.New("workflow file contains trailing JSON content")
	}
	return value, raw, nil
}

func (c *Controller) ListEngineBindings(ctx *gin.Context) {
	run(ctx, &iapiserver.EngineCapabilityBindingListRequest{}, func(r *iapiserver.EngineCapabilityBindingListRequest) (any, error) {
		return c.service.ListEngineBindings(ctx, r)
	})
}
func (c *Controller) CreateEngineBinding(ctx *gin.Context) {
	run(ctx, &iapiserver.EngineCapabilityBindingCreateRequest{}, func(r *iapiserver.EngineCapabilityBindingCreateRequest) (any, error) {
		return c.service.CreateEngineBinding(ctx, r)
	})
}
func (c *Controller) UpdateEngineBinding(ctx *gin.Context) {
	req := &iapiserver.EngineCapabilityBindingUpdateRequest{ID: ctx.Param("binding_id")}
	run(ctx, req, func(r *iapiserver.EngineCapabilityBindingUpdateRequest) (any, error) {
		return c.service.UpdateEngineBinding(ctx, r)
	})
}
func (c *Controller) DeleteEngineBinding(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) { return c.service.DeleteEngineBinding(ctx, ctx.Param("binding_id")) })
}

func (c *Controller) ListTemplates(ctx *gin.Context) {
	run(ctx, &iapiserver.ApplicationTemplateListRequest{}, func(r *iapiserver.ApplicationTemplateListRequest) (any, error) {
		return c.service.ListTemplates(ctx, r)
	})
}
func (c *Controller) CreateTemplate(ctx *gin.Context) {
	run(ctx, &iapiserver.ApplicationTemplateCreateRequest{}, func(r *iapiserver.ApplicationTemplateCreateRequest) (any, error) {
		return c.service.CreateTemplate(ctx, r)
	})
}
func (c *Controller) GetTemplate(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) { return c.service.GetTemplate(ctx, ctx.Param("application_template_id")) })
}
func (c *Controller) ListTemplateVersions(ctx *gin.Context) {
	req := &iapiserver.ApplicationTemplateVersionListRequest{ApplicationTemplateID: ctx.Param("application_template_id")}
	run(ctx, req, func(r *iapiserver.ApplicationTemplateVersionListRequest) (any, error) {
		return c.service.ListTemplateVersions(ctx, r)
	})
}
func (c *Controller) CreateTemplateVersion(ctx *gin.Context) {
	run(ctx, &iapiserver.ApplicationTemplateVersionCreateRequest{}, func(r *iapiserver.ApplicationTemplateVersionCreateRequest) (any, error) {
		return c.service.CreateTemplateVersion(ctx, ctx.Param("application_template_id"), r)
	})
}
func (c *Controller) GetTemplateVersion(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) {
		return c.service.GetTemplateVersion(ctx, ctx.Param("application_template_version_id"))
	})
}
func (c *Controller) PublishTemplateVersion(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) {
		return c.service.PublishTemplateVersion(ctx, ctx.Param("application_template_version_id"))
	})
}

func (c *Controller) ListApplications(ctx *gin.Context) {
	run(ctx, &iapiserver.ApplicationListRequest{}, func(r *iapiserver.ApplicationListRequest) (any, error) { return c.service.ListApplications(ctx, r) })
}
func (c *Controller) CreateApplication(ctx *gin.Context) {
	run(ctx, &iapiserver.ApplicationCreateRequest{}, func(r *iapiserver.ApplicationCreateRequest) (any, error) { return c.service.CreateApplication(ctx, r) })
}
func (c *Controller) GetApplication(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) { return c.service.GetApplication(ctx, ctx.Param("application_id")) })
}
func (c *Controller) UpdateApplication(ctx *gin.Context) {
	req := &iapiserver.ApplicationUpdateRequest{ID: ctx.Param("application_id")}
	run(ctx, req, func(r *iapiserver.ApplicationUpdateRequest) (any, error) { return c.service.UpdateApplication(ctx, r) })
}
func (c *Controller) ListApplicationVersions(ctx *gin.Context) {
	req := &iapiserver.ApplicationVersionListRequest{ApplicationID: ctx.Param("application_id")}
	run(ctx, req, func(r *iapiserver.ApplicationVersionListRequest) (any, error) {
		return c.service.ListApplicationVersions(ctx, r)
	})
}
func (c *Controller) CreateApplicationVersion(ctx *gin.Context) {
	run(ctx, &iapiserver.ApplicationVersionCreateRequest{}, func(r *iapiserver.ApplicationVersionCreateRequest) (any, error) {
		return c.service.CreateApplicationVersion(ctx, ctx.Param("application_id"), r)
	})
}
func (c *Controller) GetApplicationVersion(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) {
		return c.service.GetApplicationVersion(ctx, ctx.Param("application_version_id"))
	})
}
func (c *Controller) PublishApplicationVersion(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) {
		return c.service.PublishApplicationVersion(ctx, ctx.Param("application_version_id"))
	})
}
func (c *Controller) ResolveRuntimeForm(ctx *gin.Context) {
	run(ctx, &iapiserver.RuntimeFormResolveRequest{}, func(r *iapiserver.RuntimeFormResolveRequest) (any, error) {
		return c.service.ResolveRuntimeForm(ctx, ctx.Param("application_id"), r)
	})
}
func (c *Controller) CreateApplicationRun(ctx *gin.Context) {
	run(ctx, &iapiserver.ApplicationRunCreateRequest{}, func(r *iapiserver.ApplicationRunCreateRequest) (any, error) {
		return c.service.CreateApplicationRun(ctx, ctx.Param("application_id"), r)
	})
}
func (c *Controller) ListApplicationRuns(ctx *gin.Context) {
	req := &iapiserver.ApplicationRunListRequest{ApplicationID: ctx.Param("application_id")}
	run(ctx, req, func(r *iapiserver.ApplicationRunListRequest) (any, error) {
		return c.service.ListApplicationRuns(ctx, r)
	})
}
func (c *Controller) GetApplicationRun(ctx *gin.Context) {
	run(ctx, nil, func(any) (any, error) { return c.service.GetApplicationRun(ctx, ctx.Param("application_run_id")) })
}
