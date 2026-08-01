package mcp

import (
	"context"
	stderrors "errors"
	"time"

	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	protocol "github.com/wangweihong/omnimam/backend/pkg/mcp"
)

func (s *Service) createTaskBinding(
	ctx context.Context,
	invocation protocol.Invocation,
	run *iapiserver.ApplicationRun,
) (*iapiserver.MCPTaskBinding, error) {
	if run == nil || run.AtomicTaskID == nil || *run.AtomicTaskID == "" {
		return nil, internalMCPError(code.ErrMCPTaskBindingUnavailable, "ERR_MCP_TASK_BINDING_UNAVAILABLE", true, "ApplicationRun is not bound to an AtomicTask")
	}
	principalID, err := currentPrincipalID(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	_, _ = s.bindings.DeleteExpired(ctx, now)
	clientName := ""
	if invocation.Meta.ClientInfo != nil {
		clientName = invocation.Meta.ClientInfo.Name
	}
	binding := &iapiserver.MCPTaskBinding{
		MCPTaskID: newMCPTaskID(), PrincipalID: principalID, ClientName: clientName,
		ApplicationRunID: run.ID, AtomicTaskID: *run.AtomicTaskID,
		ExtensionID: iapiserver.MCPTasksExtensionID,
		ExpiresAt:   imachinery.NewTime(now.Add(s.config.TaskTTL)), LastAccessedAt: imachinery.NewTime(now),
	}
	binding.Name = binding.MCPTaskID
	created, _, err := s.bindings.CreateOrGet(ctx, binding)
	if err != nil {
		return nil, internalMCPError(code.ErrMCPTaskBindingUnavailable, "ERR_MCP_TASK_BINDING_UNAVAILABLE", true, "MCP Task Binding could not be persisted")
	}
	if !created.ExpiresAt.After(now) || created.AtomicTaskID == "" {
		return nil, internalMCPError(code.ErrMCPTaskBindingUnavailable, "ERR_MCP_TASK_BINDING_UNAVAILABLE", true, "MCP Task Binding is expired or incomplete")
	}
	return created, nil
}

func (s *Service) getTask(ctx context.Context, invocation protocol.Invocation) (any, error) {
	if !invocation.Meta.ClientCapabilities.HasTasks() {
		return nil, rpcBusiness(mcpError(code.ErrMCPTaskExtensionNotNegotiated, "ERR_MCP_TASK_EXTENSION_NOT_NEGOTIATED", false, "mcp"))
	}
	if err := s.require(ctx, permissionTaskRead); err != nil {
		return nil, err
	}
	binding, run, task, err := s.resolveTask(ctx, invocation.TaskID)
	if err != nil {
		return nil, err
	}
	result := s.taskFromBinding(binding, task)
	//nolint:misspell // MCP 2026-07-28 fixes this status to the British spelling.
	if result.Status == "completed" || result.Status == "failed" || result.Status == "cancelled" {
		projection, projectionErr := s.applicationRunProjection(ctx, run.ID)
		if projectionErr != nil {
			return nil, rpcBusiness(mcpError(code.ErrMCPTaskSourceUnavailable, "ERR_MCP_TASK_SOURCE_UNAVAILABLE", true, "mcp"))
		}
		result.Result = projection
	}
	return result, nil
}

func (s *Service) cancelTask(ctx context.Context, invocation protocol.Invocation) (any, error) {
	if !invocation.Meta.ClientCapabilities.HasTasks() {
		return nil, rpcBusiness(mcpError(code.ErrMCPTaskExtensionNotNegotiated, "ERR_MCP_TASK_EXTENSION_NOT_NEGOTIATED", false, "mcp"))
	}
	for _, permission := range []string{permissionTaskRead, permissionTaskCancel, "aiapp.application.run", "task.atomic.operate"} {
		if err := s.require(ctx, permission); err != nil {
			return nil, err
		}
	}
	binding, _, task, err := s.resolveTask(ctx, invocation.TaskID)
	if err != nil {
		return nil, err
	}
	if iapiserver.IsAtomicTaskTerminal(task.Status) {
		return nil, rpcBusiness(mcpError(code.ErrMCPTaskNotCancellable, "ERR_MCP_TASK_NOT_CANCELLABLE", false, "mcp"))
	}
	canceled, err := s.tasks.CancelAtomicTask(ctx, task.ID, &iapiserver.ActionReasonRequest{Reason: "MCP client requested cancellation"})
	if err != nil {
		return nil, rpcBusiness(sourceBusinessError(err))
	}
	return s.taskFromBinding(binding, canceled), nil
}

func (s *Service) resolveTask(
	ctx context.Context,
	taskID string,
) (*iapiserver.MCPTaskBinding, *iapiserver.ApplicationRun, *iapiserver.AtomicTask, error) {
	principalID, err := currentPrincipalID(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	binding, err := s.bindings.GetByTaskID(ctx, taskID)
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, rpcBusiness(mcpError(code.ErrMCPTaskNotVisible, "ERR_MCP_TASK_NOT_VISIBLE", false, "mcp"))
		}
		return nil, nil, nil, rpcBusiness(mcpError(code.ErrMCPTaskBindingUnavailable, "ERR_MCP_TASK_BINDING_UNAVAILABLE", true, "mcp"))
	}
	if binding.PrincipalID != principalID {
		return nil, nil, nil, rpcBusiness(mcpError(code.ErrMCPTaskNotVisible, "ERR_MCP_TASK_NOT_VISIBLE", false, "mcp"))
	}
	if !binding.ExpiresAt.After(time.Now()) {
		return nil, nil, nil, rpcBusiness(mcpError(code.ErrMCPTaskBindingExpired, "ERR_MCP_TASK_BINDING_EXPIRED", false, "mcp"))
	}
	correlateAudit(ctx, binding.ApplicationRunID, binding.MCPTaskID)
	run, err := s.applications.GetApplicationRun(ctx, binding.ApplicationRunID)
	if err != nil || run.AtomicTaskID == nil || *run.AtomicTaskID != binding.AtomicTaskID {
		return nil, nil, nil, rpcBusiness(mcpError(code.ErrMCPTaskSourceUnavailable, "ERR_MCP_TASK_SOURCE_UNAVAILABLE", true, "mcp"))
	}
	task, err := s.tasks.GetAtomicTask(ctx, binding.AtomicTaskID)
	if err != nil {
		return nil, nil, nil, rpcBusiness(mcpError(code.ErrMCPTaskSourceUnavailable, "ERR_MCP_TASK_SOURCE_UNAVAILABLE", true, "mcp"))
	}
	if err := s.bindings.Touch(ctx, binding.ID, time.Now()); err != nil {
		return nil, nil, nil, rpcBusiness(mcpError(code.ErrMCPTaskBindingUnavailable, "ERR_MCP_TASK_BINDING_UNAVAILABLE", true, "mcp"))
	}
	return binding, run, task, nil
}

func (s *Service) taskFromBinding(binding *iapiserver.MCPTaskBinding, task any) protocol.Task {
	status := "working"
	cancelRequested := false
	switch value := task.(type) {
	case *iapiserver.AtomicTask:
		status = mcpTaskStatus(value.Status)
		cancelRequested = value.Status == iapiserver.AtomicTaskStatusCancelRequested
	case *iapiserver.AtomicTaskRefSummary:
		status = mcpTaskStatus(value.Status)
		cancelRequested = value.Status == iapiserver.AtomicTaskStatusCancelRequested
	}
	ttl := time.Until(binding.ExpiresAt.Time).Milliseconds()
	if ttl < 1 {
		ttl = 1
	}
	return protocol.Task{
		TaskID: binding.MCPTaskID, Status: status, TTLMS: ttl,
		PollIntervalMS: s.config.TaskPollInterval.Milliseconds(), CancelRequested: cancelRequested,
	}
}
