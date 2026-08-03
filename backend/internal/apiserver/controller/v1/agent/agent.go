package agent

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	agentsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/agent"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

type Controller struct{ service *agentsvc.Service }

func NewController(service *agentsvc.Service) *Controller { return &Controller{service: service} }

func (c *Controller) ListProfiles(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.ListProfiles(ctx) })
}
func (c *Controller) ListAgents(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentListRequest{}, func(req *iapiserver.AgentListRequest) (any, error) { return c.service.ListAgents(ctx, req) })
}
func (c *Controller) CreateAgent(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentCreateRequest{}, func(req *iapiserver.AgentCreateRequest) (any, error) { return c.service.CreateAgent(ctx, req) })
}
func (c *Controller) GetAgent(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetAgent(ctx, ctx.Param("agent_id")) })
}
func (c *Controller) UpdateAgent(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentUpdateRequest{}, func(req *iapiserver.AgentUpdateRequest) (any, error) {
		return c.service.UpdateAgent(ctx, ctx.Param("agent_id"), req)
	})
}
func (c *Controller) DeleteAgent(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.DeleteAgent(ctx, ctx.Param("agent_id")) })
}
func (c *Controller) EnableAgent(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.EnableAgent(ctx, ctx.Param("agent_id")) })
}
func (c *Controller) DisableAgent(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentActionRequest{}, func(req *iapiserver.AgentActionRequest) (any, error) {
		return c.service.DisableAgent(ctx, ctx.Param("agent_id"), req)
	})
}
func (c *Controller) StartRuntime(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentRuntimeActionRequest{}, func(req *iapiserver.AgentRuntimeActionRequest) (any, error) {
		return c.service.StartRuntime(ctx, ctx.Param("agent_id"), req)
	})
}
func (c *Controller) SuspendRuntime(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentActionRequest{}, func(req *iapiserver.AgentActionRequest) (any, error) {
		return c.service.SuspendRuntime(ctx, ctx.Param("agent_id"), req)
	})
}
func (c *Controller) RecoverRuntime(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentRuntimeActionRequest{}, func(req *iapiserver.AgentRuntimeActionRequest) (any, error) {
		return c.service.RecoverRuntime(ctx, ctx.Param("agent_id"), req)
	})
}
func (c *Controller) StopRuntime(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentActionRequest{}, func(req *iapiserver.AgentActionRequest) (any, error) {
		return c.service.StopRuntime(ctx, ctx.Param("agent_id"), req)
	})
}

func (c *Controller) ListSessions(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentSessionListRequest{}, func(req *iapiserver.AgentSessionListRequest) (any, error) {
		return c.service.ListSessions(ctx, ctx.Param("agent_id"), req)
	})
}
func (c *Controller) CreateSession(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentSessionCreateRequest{}, func(req *iapiserver.AgentSessionCreateRequest) (any, error) {
		return c.service.CreateSession(ctx, ctx.Param("agent_id"), req)
	})
}
func (c *Controller) GetSession(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetSession(ctx, ctx.Param("session_id")) })
}
func (c *Controller) UpdateSession(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentSessionUpdateRequest{}, func(req *iapiserver.AgentSessionUpdateRequest) (any, error) {
		return c.service.UpdateSession(ctx, ctx.Param("session_id"), req)
	})
}
func (c *Controller) CloseSession(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.CloseSession(ctx, ctx.Param("session_id")) })
}
func (c *Controller) ArchiveSession(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.ArchiveSession(ctx, ctx.Param("session_id")) })
}
func (c *Controller) SendMessage(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentMessageRequest{}, func(req *iapiserver.AgentMessageRequest) (any, error) {
		return c.service.SendMessage(ctx, ctx.Param("session_id"), req)
	})
}
func (c *Controller) ListMessages(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentMessageListRequest{}, func(req *iapiserver.AgentMessageListRequest) (any, error) {
		return c.service.ListMessages(ctx, ctx.Param("session_id"), req)
	})
}
func (c *Controller) ListInvocations(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentInvocationListRequest{}, func(req *iapiserver.AgentInvocationListRequest) (any, error) {
		return c.service.ListInvocations(ctx, ctx.Param("session_id"), req)
	})
}
func (c *Controller) GetInvocation(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetInvocation(ctx, ctx.Param("invocation_id")) })
}
func (c *Controller) CancelInvocation(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentActionRequest{}, func(req *iapiserver.AgentActionRequest) (any, error) {
		return c.service.CancelInvocation(ctx, ctx.Param("invocation_id"), req)
	})
}

// StreamInvocationEvents replays persisted events after Last-Event-ID and then polls for new events.
func (c *Controller) StreamInvocationEvents(ctx *gin.Context) {
	afterSequence := 0
	if value := ctx.GetHeader("Last-Event-ID"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			core.WriteResponse(ctx, errors.NewStatus(code.ErrValidation, "Last-Event-ID must be a non-negative integer"), nil)
			return
		}
		afterSequence = parsed
	}
	if _, err := c.service.GetInvocation(ctx, ctx.Param("invocation_id")); err != nil {
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
		events, err := c.service.ListInvocationEvents(ctx, ctx.Param("invocation_id"), afterSequence)
		if err != nil {
			return
		}
		for _, event := range events {
			if strings.ContainsAny(event.EventType, "\r\n") {
				return
			}
			data, marshalErr := json.Marshal(event)
			if marshalErr != nil {
				return
			}
			if _, writeErr := fmt.Fprintf(ctx.Writer, "id: %d\nevent: %s\ndata: %s\n\n", event.SequenceNo, event.EventType, data); writeErr != nil {
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

func (c *Controller) ListMemories(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentMemoryListRequest{}, func(req *iapiserver.AgentMemoryListRequest) (any, error) {
		return c.service.ListMemories(ctx, ctx.Param("agent_id"), req)
	})
}
func (c *Controller) CreateMemory(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentMemoryCreateRequest{}, func(req *iapiserver.AgentMemoryCreateRequest) (any, error) {
		return c.service.CreateMemory(ctx, ctx.Param("agent_id"), req)
	})
}
func (c *Controller) GetMemory(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetMemory(ctx, ctx.Param("memory_id")) })
}
func (c *Controller) UpdateMemory(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentMemoryUpdateRequest{}, func(req *iapiserver.AgentMemoryUpdateRequest) (any, error) {
		return c.service.UpdateMemory(ctx, ctx.Param("memory_id"), req)
	})
}
func (c *Controller) DeleteMemory(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		if err := c.service.DeleteMemory(ctx, ctx.Param("memory_id")); err != nil {
			return nil, err
		}
		return &iapiserver.AgentOperationResult{Success: true}, nil
	})
}
func (c *Controller) GetWorkspaceBinding(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetWorkspaceBinding(ctx, ctx.Param("agent_id")) })
}
func (c *Controller) ListModelBindings(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.ListModelBindings(ctx, ctx.Param("agent_id")) })
}
func (c *Controller) ReplaceModelBinding(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentModelBindingInput{}, func(req *iapiserver.AgentModelBindingInput) (any, error) {
		return c.service.ReplaceModelBinding(ctx, ctx.Param("agent_id"), req)
	})
}
func (c *Controller) ListSkillBindings(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.ListSkillBindings(ctx, ctx.Param("agent_id")) })
}
func (c *Controller) ListMCPBindings(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.ListMCPBindings(ctx, ctx.Param("agent_id")) })
}
func (c *Controller) CreateMCPBinding(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.AgentMCPBindingRequest{}, func(req *iapiserver.AgentMCPBindingRequest) (any, error) {
		return c.service.CreateMCPBinding(ctx, ctx.Param("agent_id"), req)
	})
}
