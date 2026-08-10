package appstudio

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	appstudiosvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/appstudio"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/invocationsse"
	"github.com/wangweihong/omnimam/backend/pkg/core"
	mcpprotocol "github.com/wangweihong/omnimam/backend/pkg/mcp"
)

const (
	workspaceToolMaxRequestBytes = 1 << 20
	workspaceToolHeartbeat       = 15 * time.Second
)

type Controller struct{ service *appstudiosvc.Service }

func NewController(service *appstudiosvc.Service) *Controller { return &Controller{service: service} }

// HandleWorkspaceTool 处理 Runtime 使用 Invocation 短期 grant 发起的内部 MCP 请求，不接受 Identity JWT。
func (c *Controller) HandleWorkspaceTool(ctx *gin.Context) {
	if ctx.Request.Method == http.MethodGet {
		// OpenCode keeps this SSE transport open after the initialize handshake.
		// Closing it after the acknowledgement makes the client reconnect without
		// ever publishing the Workspace Tool IDs to the invocation runtime.
		if err := c.service.ValidateWorkspaceToolGrant(ctx, ctx.GetHeader("Authorization")); err != nil {
			ctx.Status(http.StatusUnauthorized)
			return
		}
		ctx.Header("Content-Type", "text/event-stream")
		ctx.Header("Cache-Control", "no-cache")
		ctx.Header("Connection", "keep-alive")
		ctx.Header("X-Accel-Buffering", "no")
		ctx.Status(http.StatusOK)
		_, _ = ctx.Writer.WriteString(": workspace tool connected\n\n")
		ctx.Writer.Flush()
		heartbeat := time.NewTicker(workspaceToolHeartbeat)
		defer heartbeat.Stop()
		for {
			select {
			case <-ctx.Request.Context().Done():
				return
			case <-heartbeat.C:
				if _, err := ctx.Writer.WriteString(": workspace tool heartbeat\n\n"); err != nil {
					return
				}
				ctx.Writer.Flush()
			}
		}
	}
	ctx.Header("MCP-Protocol-Version", mcpprotocol.ProtocolVersion)
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, workspaceToolMaxRequestBytes)
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		ctx.JSON(http.StatusRequestEntityTooLarge, mcpprotocol.Response{
			JSONRPC: "2.0", ID: mcpprotocol.RequestIDOrNull(nil),
			Error: &mcpprotocol.RPCError{Code: mcpprotocol.JSONRPCInvalidRequest, Message: "workspace tool request is too large"},
		})
		return
	}
	response, status := c.service.ProcessWorkspaceToolRequest(ctx, ctx.GetHeader("Authorization"), mcpprotocol.Headers{
		ProtocolVersion: ctx.GetHeader("MCP-Protocol-Version"),
		Method:          ctx.GetHeader("Mcp-Method"),
		Name:            ctx.GetHeader("Mcp-Name"),
		Accept:          ctx.GetHeader("Accept"),
		ContentType:     ctx.GetHeader("Content-Type"),
	}, body)
	if response == nil {
		ctx.Status(status)
		return
	}
	ctx.JSON(status, response)
}

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

// ListAgentMessages 返回应用当前 generation/session 的 Coding Agent 消息历史，不暴露 Workspace 或内部绑定。
func (c *Controller) ListAgentMessages(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.StudioAgentMessageListRequest{}, func(req *iapiserver.StudioAgentMessageListRequest) (any, error) {
		return c.service.ListAgentMessages(ctx, ctx.Param("studio_application_id"), req)
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
	afterSequence, err := invocationsse.ParseLastEventID(ctx.GetHeader("Last-Event-ID"))
	if err != nil {
		core.WriteResponse(ctx, errors.NewStatus(code.ErrValidation, "Last-Event-ID must be a canonical non-negative decimal integer"), nil)
		return
	}
	appID, invocationID := ctx.Param("studio_application_id"), ctx.Param("agent_invocation_id")
	invocation, err := c.service.GetAgentInvocation(ctx, appID, invocationID)
	if err != nil {
		core.WriteResponse(ctx, err, nil)
		return
	}
	terminal := invocationsse.IsTerminalInvocationStatus(invocation.Status)
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
		advanced := false
		for _, event := range events {
			if event.SequenceNo <= afterSequence {
				continue
			}
			if err := invocationsse.WriteEvent(ctx.Writer, event); err != nil {
				return
			}
			afterSequence = event.SequenceNo
			advanced = true
			ctx.Writer.Flush()
			if invocationsse.IsTerminalEventType(event.EventType) {
				return
			}
		}
		if advanced {
			continue
		}
		if terminal {
			return
		}
		invocation, err = c.service.GetAgentInvocation(ctx, appID, invocationID)
		if err != nil {
			return
		}
		terminal = invocationsse.IsTerminalInvocationStatus(invocation.Status)
		if terminal {
			// Re-list once so a terminal event committed with the status is flushed first.
			continue
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
