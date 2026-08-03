package postgresql

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type agentStore struct{ ds *datastore }

func newAgentStore(ds *datastore) *agentStore { return &agentStore{ds: ds} }

func (s *agentStore) CreateAgentAggregate(ctx context.Context, agent *iapiserver.Agent, session *iapiserver.AgentSession, binding *iapiserver.AgentWorkspaceBinding, model *iapiserver.AgentModelBinding) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, value := range []any{agent, session, binding} {
			if err := tx.Create(value).Error; err != nil {
				return err
			}
		}
		if model != nil {
			if err := tx.Create(model).Error; err != nil {
				return err
			}
		}
		return appendAgentOutbox(tx, "Agent", agent.ID, "agent_lifecycle_changed", agent.ResourceVersion, map[string]any{
			"agent_id": agent.ID, "owner_user_id": agent.OwnerUserID, "kind": agent.Kind,
			"workspace_type": agent.WorkspaceType, "workspace_id": agent.WorkspaceID,
			"from_status": nil, "to_status": agent.Status,
		})
	})
}

func (s *agentStore) ListAgents(ctx context.Context, req *iapiserver.AgentListRequest) ([]*iapiserver.Agent, int64, error) {
	var items []*iapiserver.Agent
	filter := func(query *gorm.DB) *gorm.DB {
		query = query.Where("owner_user_id = ?", req.OwnerUserID)
		if req.Statuses != "" {
			query = query.Where("status IN ?", splitCSV(req.Statuses))
		}
		return query
	}
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.Agent{}), filter)
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *agentStore) GetAgent(ctx context.Context, id, ownerUserID string) (*iapiserver.Agent, error) {
	var item iapiserver.Agent
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND owner_user_id = ?", id, ownerUserID).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrAgentNotVisible, "agent not visible")
	}
	return &item, nil
}

func (s *agentStore) UpdateAgent(ctx context.Context, agent *iapiserver.Agent, expectedVersion int64) (*iapiserver.Agent, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previous iapiserver.Agent
		if err := tx.Where("id = ? AND owner_user_id = ?", agent.ID, agent.OwnerUserID).First(&previous).Error; err != nil {
			return mapNotFound(err, code.ErrAgentNotVisible, "agent not visible")
		}
		if previous.ResourceVersion != expectedVersion {
			return errors.NewStatus(code.ErrAgentStateInvalid, "agent resource version conflicts")
		}
		agent.ResourceVersion = previous.ResourceVersion
		if err := tx.Save(agent).Error; err != nil {
			return err
		}
		return appendAgentOutbox(tx, "Agent", agent.ID, "agent_lifecycle_changed", agent.ResourceVersion, map[string]any{
			"agent_id": agent.ID, "owner_user_id": agent.OwnerUserID, "kind": agent.Kind,
			"workspace_type": agent.WorkspaceType, "workspace_id": agent.WorkspaceID,
			"from_status": previous.Status, "to_status": agent.Status,
		})
	})
	return agent, err
}

func (s *agentStore) ListAgentSessions(ctx context.Context, req *iapiserver.AgentSessionListRequest) ([]*iapiserver.AgentSession, int64, error) {
	var items []*iapiserver.AgentSession
	filter := func(query *gorm.DB) *gorm.DB {
		return query.Where("agent_id = ? AND owner_user_id = ?", req.AgentID, req.OwnerUserID)
	}
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.AgentSession{}), filter)
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *agentStore) CreateAgentSession(ctx context.Context, session *iapiserver.AgentSession) (*iapiserver.AgentSession, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(session).Error; err != nil {
			return err
		}
		return appendAgentOutbox(tx, "AgentSession", session.ID, "agent_session_status_changed", session.ResourceVersion, map[string]any{
			"session_id": session.ID, "agent_id": session.AgentID, "owner_user_id": session.OwnerUserID,
			"from_status": nil, "to_status": session.Status,
		})
	})
	return session, err
}

func (s *agentStore) GetAgentSession(ctx context.Context, id, ownerUserID string) (*iapiserver.AgentSession, error) {
	var item iapiserver.AgentSession
	if err := s.ds.db.WithContext(ctx).Where("id = ? AND owner_user_id = ?", id, ownerUserID).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrAgentSessionNotVisible, "agent session not visible")
	}
	return &item, nil
}

func (s *agentStore) UpdateAgentSession(ctx context.Context, session *iapiserver.AgentSession, expectedVersion int64) (*iapiserver.AgentSession, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previous iapiserver.AgentSession
		if err := tx.Where("id = ? AND owner_user_id = ?", session.ID, session.OwnerUserID).First(&previous).Error; err != nil {
			return mapNotFound(err, code.ErrAgentSessionNotVisible, "agent session not visible")
		}
		if previous.ResourceVersion != expectedVersion {
			return errors.NewStatus(code.ErrAgentSessionClosed, "agent session resource version conflicts")
		}
		session.ResourceVersion = previous.ResourceVersion
		if err := tx.Save(session).Error; err != nil {
			return err
		}
		return appendAgentOutbox(tx, "AgentSession", session.ID, "agent_session_status_changed", session.ResourceVersion, map[string]any{
			"session_id": session.ID, "agent_id": session.AgentID, "owner_user_id": session.OwnerUserID,
			"from_status": previous.Status, "to_status": session.Status,
		})
	})
	return session, err
}

func (s *agentStore) CreateAgentInvocation(ctx context.Context, message *iapiserver.AgentMessage, invocation *iapiserver.AgentInvocation) (*iapiserver.AgentInvocation, error) {
	var result *iapiserver.AgentInvocation
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing iapiserver.AgentInvocation
		err := tx.Where("agent_id = ? AND idempotency_key = ?", invocation.AgentID, invocation.IdempotencyKey).First(&existing).Error
		if err == nil {
			result = &existing
			return nil
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		if err := tx.Create(message).Error; err != nil {
			return err
		}
		if err := tx.Create(invocation).Error; err != nil {
			return err
		}
		if err := tx.Model(&iapiserver.AgentSession{}).Where("id = ?", invocation.SessionID).Updates(map[string]any{"last_message_at": message.CreatedAt.Time, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		if err := appendAgentOutbox(tx, "AgentInvocation", invocation.ID, "agent_invocation_status_changed", invocation.ResourceVersion, map[string]any{
			"invocation_id": invocation.ID, "agent_id": invocation.AgentID, "session_id": invocation.SessionID,
			"atomic_task_id": agentNullableString(invocation.AtomicTaskID), "from_status": nil, "to_status": invocation.Status,
		}); err != nil {
			return err
		}
		result = invocation
		return nil
	})
	return result, err
}

func (s *agentStore) ListAgentMessages(ctx context.Context, req *iapiserver.AgentMessageListRequest, ownerUserID string) ([]*iapiserver.AgentMessage, int64, error) {
	var items []*iapiserver.AgentMessage
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.AgentMessage{}), func(query *gorm.DB) *gorm.DB {
		return query.Joins("JOIN agent_sessions ON agent_sessions.id = agent_messages.session_id").Where("agent_messages.session_id = ? AND agent_sessions.owner_user_id = ?", req.SessionID, ownerUserID)
	})
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *agentStore) ListAgentInvocations(ctx context.Context, req *iapiserver.AgentInvocationListRequest, ownerUserID string) ([]*iapiserver.AgentInvocation, int64, error) {
	var items []*iapiserver.AgentInvocation
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.AgentInvocation{}), func(query *gorm.DB) *gorm.DB {
		return query.Joins("JOIN agent_sessions ON agent_sessions.id = agent_invocations.session_id").Where("agent_invocations.session_id = ? AND agent_sessions.owner_user_id = ?", req.SessionID, ownerUserID)
	})
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *agentStore) GetAgentInvocation(ctx context.Context, id, ownerUserID string) (*iapiserver.AgentInvocation, error) {
	var item iapiserver.AgentInvocation
	err := s.ds.db.WithContext(ctx).Joins("JOIN agent_sessions ON agent_sessions.id = agent_invocations.session_id").Where("agent_invocations.id = ? AND agent_sessions.owner_user_id = ?", id, ownerUserID).First(&item).Error
	if err != nil {
		return nil, mapNotFound(err, code.ErrAgentSessionNotVisible, "agent invocation not visible")
	}
	return &item, nil
}

func (s *agentStore) UpdateAgentInvocation(ctx context.Context, invocation *iapiserver.AgentInvocation) (*iapiserver.AgentInvocation, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previous iapiserver.AgentInvocation
		if err := tx.Where("id = ?", invocation.ID).First(&previous).Error; err != nil {
			return err
		}
		if err := tx.Save(invocation).Error; err != nil {
			return err
		}
		return appendAgentOutbox(tx, "AgentInvocation", invocation.ID, "agent_invocation_status_changed", invocation.ResourceVersion, map[string]any{
			"invocation_id": invocation.ID, "agent_id": invocation.AgentID, "session_id": invocation.SessionID,
			"atomic_task_id": agentNullableString(invocation.AtomicTaskID), "from_status": previous.Status, "to_status": invocation.Status,
			"error_code": agentNullableString(invocation.FailureCode),
		})
	})
	return invocation, err
}

func (s *agentStore) ListAgentMemories(ctx context.Context, req *iapiserver.AgentMemoryListRequest, ownerUserID string) ([]*iapiserver.AgentMemory, int64, error) {
	var items []*iapiserver.AgentMemory
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.AgentMemory{}), func(query *gorm.DB) *gorm.DB {
		query = query.Joins("JOIN agents ON agents.id = agent_memories.agent_id").Where("agent_memories.agent_id = ? AND agents.owner_user_id = ? AND agent_memories.deleted_at IS NULL", req.AgentID, ownerUserID)
		if req.Scope != "" {
			query = query.Where("agent_memories.scope = ?", req.Scope)
		}
		return query
	})
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *agentStore) CreateAgentMemory(ctx context.Context, memory *iapiserver.AgentMemory) (*iapiserver.AgentMemory, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(memory).Error; err != nil {
			return err
		}
		return appendAgentOutbox(tx, "AgentMemory", memory.ID, "agent_memory_changed", memory.ResourceVersion, map[string]any{
			"memory_id": memory.ID, "agent_id": memory.AgentID, "session_id": agentNullableString(memory.SessionID),
			"change_type": "CREATED", "scope": memory.Scope,
		})
	})
	return memory, err
}

func (s *agentStore) GetAgentMemory(ctx context.Context, id, ownerUserID string) (*iapiserver.AgentMemory, error) {
	var item iapiserver.AgentMemory
	err := s.ds.db.WithContext(ctx).Joins("JOIN agents ON agents.id = agent_memories.agent_id").Where("agent_memories.id = ? AND agents.owner_user_id = ? AND agent_memories.deleted_at IS NULL", id, ownerUserID).First(&item).Error
	if err != nil {
		return nil, mapNotFound(err, code.ErrAgentMemoryInvalid, "agent memory not visible")
	}
	return &item, nil
}

func (s *agentStore) UpdateAgentMemory(ctx context.Context, memory *iapiserver.AgentMemory, expectedVersion int64) (*iapiserver.AgentMemory, error) {
	if memory.ResourceVersion != expectedVersion {
		return nil, errors.NewStatus(code.ErrAgentMemoryInvalid, "agent memory resource version conflicts")
	}
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(memory).Error; err != nil {
			return err
		}
		return appendAgentOutbox(tx, "AgentMemory", memory.ID, "agent_memory_changed", memory.ResourceVersion, map[string]any{
			"memory_id": memory.ID, "agent_id": memory.AgentID, "session_id": agentNullableString(memory.SessionID),
			"change_type": "UPDATED", "scope": memory.Scope,
		})
	})
	return memory, err
}

func (s *agentStore) DeleteAgentMemory(ctx context.Context, id, ownerUserID string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var memory iapiserver.AgentMemory
		err := tx.Joins("JOIN agents ON agents.id = agent_memories.agent_id").Where("agent_memories.id = ? AND agents.owner_user_id = ? AND agent_memories.deleted_at IS NULL", id, ownerUserID).First(&memory).Error
		if err != nil {
			return mapNotFound(err, code.ErrAgentMemoryInvalid, "agent memory not visible")
		}
		now := imachinery.Now()
		if err := tx.Model(&memory).Update("deleted_at", now.Time).Error; err != nil {
			return err
		}
		return appendAgentOutbox(tx, "AgentMemory", memory.ID, "agent_memory_changed", memory.ResourceVersion+1, map[string]any{
			"memory_id": memory.ID, "agent_id": memory.AgentID, "session_id": agentNullableString(memory.SessionID),
			"change_type": "DELETED", "scope": memory.Scope,
		})
	})
}

func (s *agentStore) GetAgentWorkspaceBinding(ctx context.Context, agentID, ownerUserID string) (*iapiserver.AgentWorkspaceBinding, error) {
	var item iapiserver.AgentWorkspaceBinding
	err := s.ds.db.WithContext(ctx).Joins("JOIN agents ON agents.id = agent_workspace_bindings.agent_id").Where("agent_workspace_bindings.agent_id = ? AND agents.owner_user_id = ?", agentID, ownerUserID).First(&item).Error
	if err != nil {
		return nil, mapNotFound(err, code.ErrAgentWorkspaceBindingInvalid, "agent workspace binding not visible")
	}
	return &item, nil
}

func (s *agentStore) ListAgentModelBindings(ctx context.Context, agentID, ownerUserID string) ([]*iapiserver.AgentModelBinding, error) {
	var items []*iapiserver.AgentModelBinding
	err := s.ds.db.WithContext(ctx).Joins("JOIN agents ON agents.id = agent_model_bindings.agent_id").Where("agent_model_bindings.agent_id = ? AND agents.owner_user_id = ?", agentID, ownerUserID).Find(&items).Error
	return items, err
}

func (s *agentStore) ReplaceAgentModelBinding(ctx context.Context, agentID, ownerUserID string, binding *iapiserver.AgentModelBinding) (*iapiserver.AgentModelBinding, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&iapiserver.Agent{}).Where("id = ? AND owner_user_id = ?", agentID, ownerUserID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errors.NewStatus(code.ErrAgentNotVisible, "agent not visible")
		}
		var existing iapiserver.AgentModelBinding
		err := tx.Where("agent_id = ? AND purpose = ? AND name = ?", agentID, binding.Purpose, binding.Name).First(&existing).Error
		if err == nil {
			binding.ID, binding.CreatedAt, binding.ResourceVersion = existing.ID, existing.CreatedAt, existing.ResourceVersion
			return tx.Save(binding).Error
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		return tx.Create(binding).Error
	})
	return binding, err
}

func (s *agentStore) ListAgentSkillBindings(ctx context.Context, agentID, ownerUserID string) ([]*iapiserver.AgentSkillBinding, error) {
	var items []*iapiserver.AgentSkillBinding
	err := s.ds.db.WithContext(ctx).Joins("JOIN agents ON agents.id = agent_skill_bindings.agent_id").Where("agent_skill_bindings.agent_id = ? AND agents.owner_user_id = ?", agentID, ownerUserID).Find(&items).Error
	return items, err
}

func (s *agentStore) ListAgentMCPBindings(ctx context.Context, agentID, ownerUserID string) ([]*iapiserver.AgentMCPBinding, error) {
	var items []*iapiserver.AgentMCPBinding
	err := s.ds.db.WithContext(ctx).Joins("JOIN agents ON agents.id = agent_mcp_bindings.agent_id").Where("agent_mcp_bindings.agent_id = ? AND agents.owner_user_id = ?", agentID, ownerUserID).Find(&items).Error
	return items, err
}

func (s *agentStore) CreateAgentMCPBinding(ctx context.Context, ownerUserID string, binding *iapiserver.AgentMCPBinding) (*iapiserver.AgentMCPBinding, error) {
	var count int64
	if err := s.ds.db.WithContext(ctx).Model(&iapiserver.Agent{}).Where("id = ? AND owner_user_id = ?", binding.AgentID, ownerUserID).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, errors.NewStatus(code.ErrAgentNotVisible, "agent not visible")
	}
	return binding, s.ds.db.WithContext(ctx).Create(binding).Error
}

func (s *agentStore) GetCurrentAgentRuntime(ctx context.Context, agentID, ownerUserID string) (*iapiserver.AgentRuntimeBinding, error) {
	var item iapiserver.AgentRuntimeBinding
	err := s.ds.db.WithContext(ctx).Joins("JOIN agents ON agents.id = agent_runtime_bindings.agent_id").Where("agent_runtime_bindings.agent_id = ? AND agents.owner_user_id = ? AND agent_runtime_bindings.state NOT IN ?", agentID, ownerUserID, []string{"DELETED", "STOPPED", "FAILED"}).Order("agent_runtime_bindings.created_at DESC").First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, mapNotFound(err, code.ErrAgentRuntimeNotVisible, "agent runtime not visible")
	}
	return &item, nil
}

func (s *agentStore) GetAgentRuntimeByID(ctx context.Context, id string) (*iapiserver.AgentRuntimeBinding, error) {
	var item iapiserver.AgentRuntimeBinding
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, mapNotFound(err, code.ErrAgentRuntimeNotVisible, "agent runtime not visible")
	}
	return &item, nil
}

func (s *agentStore) CreateAgentRuntime(ctx context.Context, ownerUserID string, runtime *iapiserver.AgentRuntimeBinding) (*iapiserver.AgentRuntimeBinding, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&iapiserver.Agent{}).Where("id = ? AND owner_user_id = ?", runtime.AgentID, ownerUserID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errors.NewStatus(code.ErrAgentNotVisible, "agent not visible")
		}
		if err := tx.Create(runtime).Error; err != nil {
			return err
		}
		return appendAgentRuntimeOutbox(tx, nil, runtime)
	})
	return runtime, err
}

func (s *agentStore) UpdateAgentRuntime(ctx context.Context, runtime *iapiserver.AgentRuntimeBinding) (*iapiserver.AgentRuntimeBinding, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previous iapiserver.AgentRuntimeBinding
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", runtime.ID).First(&previous).Error; err != nil {
			return mapNotFound(err, code.ErrAgentRuntimeNotVisible, "agent runtime not visible")
		}
		runtime.ResourceVersion = previous.ResourceVersion
		if err := tx.Save(runtime).Error; err != nil {
			return err
		}
		return appendAgentRuntimeOutbox(tx, &previous, runtime)
	})
	return runtime, err
}

func (s *agentStore) ProjectAgentRuntime(ctx context.Context, runtime *iapiserver.AgentRuntimeBinding, agentStatus string) (*iapiserver.AgentRuntimeBinding, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previousRuntime iapiserver.AgentRuntimeBinding
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", runtime.ID).First(&previousRuntime).Error; err != nil {
			return mapNotFound(err, code.ErrAgentRuntimeNotVisible, "agent runtime not visible")
		}
		var agent iapiserver.Agent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", runtime.AgentID).First(&agent).Error; err != nil {
			return mapNotFound(err, code.ErrAgentNotVisible, "agent not visible")
		}
		runtimeChanged := previousRuntime.State != runtime.State || previousRuntime.ActivityState != runtime.ActivityState ||
			previousRuntime.HealthStatus != runtime.HealthStatus || previousRuntime.InfraRuntimeID != runtime.InfraRuntimeID ||
			previousRuntime.EndpointRef != runtime.EndpointRef
		if runtimeChanged {
			runtime.ResourceVersion = previousRuntime.ResourceVersion
			if err := tx.Save(runtime).Error; err != nil {
				return err
			}
			if err := appendAgentRuntimeOutbox(tx, &previousRuntime, runtime); err != nil {
				return err
			}
		} else {
			*runtime = previousRuntime
		}

		if agent.Disabled || agent.Status == "DISABLED" {
			agentStatus = "DISABLED"
		} else if agent.Status == "DELETING" {
			agentStatus = "DELETING"
		}
		if agent.Status == agentStatus {
			return nil
		}
		previousAgent := agent
		agent.Status = agentStatus
		if err := tx.Save(&agent).Error; err != nil {
			return err
		}
		return appendAgentOutbox(tx, "Agent", agent.ID, "agent_lifecycle_changed", agent.ResourceVersion, map[string]any{
			"agent_id": agent.ID, "owner_user_id": agent.OwnerUserID, "kind": agent.Kind,
			"workspace_type": agent.WorkspaceType, "workspace_id": agent.WorkspaceID,
			"from_status": previousAgent.Status, "to_status": agent.Status,
		})
	})
	return runtime, err
}

func (s *agentStore) AppendAgentOperationEvent(ctx context.Context, event *iapiserver.AgentOperationEvent) (*iapiserver.AgentOperationEvent, error) {
	return event, s.ds.db.WithContext(ctx).Create(event).Error
}

func (s *agentStore) ListAgentOperationEvents(ctx context.Context, invocationID string, afterSequence int) ([]*iapiserver.AgentOperationEvent, error) {
	var items []*iapiserver.AgentOperationEvent
	err := s.ds.db.WithContext(ctx).Where("invocation_id = ? AND sequence_no > ?", invocationID, afterSequence).Order("sequence_no ASC").Limit(500).Find(&items).Error
	return items, err
}

func appendAgentOutbox(tx *gorm.DB, aggregateType, aggregateID, eventType string, resourceVersion int64, payload map[string]any) error {
	payload["resource_version"] = resourceVersion
	payload["occurred_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	event := &iapiserver.AgentOutbox{
		ObjectMeta: imachinery.ObjectMeta{ID: uuid.NewString()}, AggregateType: aggregateType, AggregateID: aggregateID,
		EventType: eventType, Payload: raw, IdempotencyKey: fmt.Sprintf("%s:%d", aggregateID, resourceVersion),
		DeliveryStatus: "PENDING", NextAttemptAt: imachinery.Now(),
	}
	return tx.Create(event).Error
}

func appendAgentRuntimeOutbox(tx *gorm.DB, previous, runtime *iapiserver.AgentRuntimeBinding) error {
	fromState := any(nil)
	if previous != nil {
		fromState = previous.State
	}
	return appendAgentOutbox(tx, "AgentRuntime", runtime.ID, "agent_runtime_status_changed", runtime.ResourceVersion, map[string]any{
		"runtime_binding_id": runtime.ID,
		"agent_id":           runtime.AgentID,
		"infra_runtime_id":   agentNullableString(runtime.InfraRuntimeID),
		"from_state":         fromState,
		"to_state":           runtime.State,
		"activity_state":     runtime.ActivityState,
		"health_status":      runtime.HealthStatus,
		"error_code":         nil,
	})
}

func agentNullableString(value any) any {
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return nil
		}
		return typed
	case *string:
		if typed == nil || *typed == "" {
			return nil
		}
		return *typed
	default:
		return nil
	}
}
