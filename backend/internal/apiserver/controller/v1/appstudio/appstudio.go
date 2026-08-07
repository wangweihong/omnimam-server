package appstudio

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	appstudiosvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/appstudio"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
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

// GetAgentStatus 返回应用当前 generation 的 Coding Agent/Session 状态。
func (c *Controller) GetAgentStatus(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetAgentStatus(ctx, ctx.Param("studio_application_id")) })
}

// SendAgentMessage 向应用当前 Coding Agent 发送开发指令并创建 CODING Invocation。
func (c *Controller) SendAgentMessage(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioAgentMessageRequest{}, func(req *iapiserver.StudioAgentMessageRequest) (any, error) {
		return c.service.SendAgentMessage(ctx, ctx.Param("studio_application_id"), req)
	})
}

// ListAgentInvocations 返回应用当前 generation 的 Invocation 列表。
func (c *Controller) ListAgentInvocations(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentInvocationListRequest{}, func(req *iapiserver.AgentInvocationListRequest) (any, error) {
		return c.service.ListAgentInvocations(ctx, ctx.Param("studio_application_id"), req)
	})
}

// GetAgentInvocation 返回应用当前 generation 的单个 Invocation。
func (c *Controller) GetAgentInvocation(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		return c.service.GetAgentInvocation(ctx, ctx.Param("studio_application_id"), ctx.Param("agent_invocation_id"))
	})
}

// CancelAgentInvocation 请求取消应用当前 generation 的 Invocation。
func (c *Controller) CancelAgentInvocation(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentActionRequest{}, func(req *iapiserver.AgentActionRequest) (any, error) {
		return c.service.CancelAgentInvocation(ctx, ctx.Param("studio_application_id"), ctx.Param("agent_invocation_id"), req)
	})
}

// StreamAgentInvocationEvents 在应用绑定校验后重放并持续输出持久化 Invocation 事件。
func (c *Controller) StreamAgentInvocationEvents(ctx *gin.Context) {
	afterSequence := 0
	if value := ctx.GetHeader("Last-Event-ID"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			core.WriteResponse(ctx, errors.NewStatus(code.ErrValidation, "Last-Event-ID must be a non-negative integer"), nil)
			return
		}
		afterSequence = parsed
	}
	appID, invocationID := ctx.Param("studio_application_id"), ctx.Param("agent_invocation_id")
	if _, err := c.service.GetAgentInvocation(ctx, appID, invocationID); err != nil {
		core.WriteResponse(ctx, err, nil)
		return
	}
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Header("X-Accel-Buffering", "no")
	ctx.Status(200)
	ctx.Writer.Flush()
	poll := time.NewTicker(time.Second)
	heartbeat := time.NewTicker(15 * time.Second)
	defer poll.Stop()
	defer heartbeat.Stop()
	for {
		events, err := c.service.ListAgentInvocationEvents(ctx, appID, invocationID, afterSequence)
		if err != nil {
			return
		}
		for _, event := range events {
			if strings.ContainsAny(event.EventType, "\r\n") {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				return
			}
			if _, err := fmt.Fprintf(ctx.Writer, "id: %d\nevent: %s\ndata: %s\n\n", event.SequenceNo, event.EventType, data); err != nil {
				return
			}
			afterSequence = event.SequenceNo
			ctx.Writer.Flush()
		}
		select {
		case <-ctx.Request.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprintf(ctx.Writer, ": heartbeat %d\n\n", afterSequence); err != nil {
				return
			}
			ctx.Writer.Flush()
		case <-poll.C:
		}
	}
}

// SuspendAgent 挂起应用当前 Coding Agent Runtime。
func (c *Controller) SuspendAgent(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentActionRequest{}, func(req *iapiserver.AgentActionRequest) (any, error) {
		return c.service.SuspendAgent(ctx, ctx.Param("studio_application_id"), req)
	})
}

// ResumeAgent 恢复应用当前 Coding Agent Runtime。
func (c *Controller) ResumeAgent(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentActionRequest{}, func(req *iapiserver.AgentActionRequest) (any, error) {
		return c.service.ResumeAgent(ctx, ctx.Param("studio_application_id"), req)
	})
}

// ReplaceAgent 原子替换应用 Coding Agent generation 并保留旧历史。
func (c *Controller) ReplaceAgent(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioAgentReplaceRequest{}, func(req *iapiserver.StudioAgentReplaceRequest) (any, error) {
		return c.service.ReplaceAgent(ctx, ctx.Param("studio_application_id"), req)
	})
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
func (c *Controller) BatchBuildSummaries(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioBuildBatchSummaryRequest{}, func(req *iapiserver.StudioBuildBatchSummaryRequest) (any, error) {
		return c.service.BatchBuildSummaries(ctx, req)
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
