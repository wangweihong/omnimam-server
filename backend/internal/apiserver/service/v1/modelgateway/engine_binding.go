package modelgateway

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/pkg/helpers"
)

// ListEngineBindings 返回引擎能力绑定及基于当前 ProviderCapability 目录解析的有效状态。
func (s *EngineService) ListEngineBindings(ctx context.Context, req *iapiserver.EngineCapabilityBindingListRequest) (*iapiserver.EngineCapabilityBindingListResponse, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	items, total, err := s.store.ApplicationPlatforms().ListEngineBindings(ctx, req)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		s.ResolveBindingStatus(item)
	}
	return &iapiserver.EngineCapabilityBindingListResponse{Total: total, Items: items}, nil
}

// CreateEngineBinding 创建管理员可维护的引擎能力绑定并禁止扩大目录定义范围。
func (s *EngineService) CreateEngineBinding(ctx context.Context, req *iapiserver.EngineCapabilityBindingCreateRequest) (*iapiserver.EngineCapabilityBinding, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	if req.Enabled == nil {
		return nil, errors.NewStatus(code.ErrAIAppEngineBindingIncompatible, "enabled is required")
	}
	engine, err := s.store.ApplicationPlatforms().GetEngineInstance(ctx, req.EngineInstanceID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppEngineInstanceNotFound, "engine instance not found")
	}
	capability, ok := s.capabilities.Get(req.ProviderCapabilityID)
	if !ok || capability.Availability != iapiserver.ProviderCapabilityAvailable {
		return nil, errors.NewStatus(code.ErrAIAppProviderCapabilityUnavailable, "provider capability is unavailable")
	}
	if capability.BindingPolicy == iapiserver.ProviderBindingPolicyRequiredImmutable {
		return nil, errors.NewStatus(code.ErrAIAppSystemEngineBindingImmutable, "system-managed engine binding cannot be created")
	}
	if capability.ApplicationEngineTypeID != engine.ApplicationEngineTypeID {
		return nil, errors.NewStatus(code.ErrAIAppEngineBindingIncompatible, "engine type does not match capability")
	}
	if err := validateRestrictions(capability, req.Restrictions); err != nil {
		return nil, err
	}
	item := &iapiserver.EngineCapabilityBinding{
		EngineInstanceID: engine.ID, ProviderCapabilityID: capability.ID,
		ProviderCapabilityRevision: capability.Revision, Enabled: *req.Enabled,
		Restrictions: req.Restrictions, EffectiveStatus: iapiserver.BindingEffectiveAvailable,
	}
	item.Name, item.Description = req.Name, req.Description
	ret, err := s.store.ApplicationPlatforms().AddEngineBinding(ctx, item)
	return ret, mapUnique(err, "idx_aiapp_binding_engine_capability", code.ErrAIAppEngineBindingIncompatible, "binding already exists")
}

// UpdateEngineBinding 按 resourceVersion 更新非系统绑定及其限制集合。
func (s *EngineService) UpdateEngineBinding(ctx context.Context, req *iapiserver.EngineCapabilityBindingUpdateRequest) (*iapiserver.EngineCapabilityBinding, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	item, err := s.store.ApplicationPlatforms().GetEngineBinding(ctx, req.ID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppEngineBindingNotFound, "engine binding not found")
	}
	if s.isSystemManagedBinding(item) {
		return nil, errors.NewStatus(code.ErrAIAppSystemEngineBindingImmutable, "system-managed engine binding cannot be updated")
	}
	capability, ok := s.capabilities.Get(item.ProviderCapabilityID)
	if !ok {
		return nil, errors.NewStatus(code.ErrAIAppProviderCapabilityUnavailable, "provider capability is unavailable")
	}
	if req.Restrictions != nil {
		if err := validateRestrictions(capability, req.Restrictions); err != nil {
			return nil, err
		}
		item.Restrictions = req.Restrictions
	}
	helpers.ApplyString(&item.Name, req.Name)
	helpers.ApplyString(&item.Description, req.Description)
	if req.Enabled != nil {
		item.Enabled = *req.Enabled
	}
	ret, err := s.store.ApplicationPlatforms().UpdateEngineBinding(ctx, item, req.ResourceVersion)
	if ret != nil {
		s.ResolveBindingStatus(ret)
	}
	return ret, err
}

// DeleteEngineBinding 删除非系统管理的能力绑定。
func (s *EngineService) DeleteEngineBinding(ctx context.Context, id string) (*iapiserver.DeleteResult, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	item, err := s.store.ApplicationPlatforms().GetEngineBinding(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppEngineBindingNotFound, "engine binding not found")
	}
	if s.isSystemManagedBinding(item) {
		return nil, errors.NewStatus(code.ErrAIAppSystemEngineBindingImmutable, "system-managed engine binding cannot be deleted")
	}
	if err := s.store.ApplicationPlatforms().DeleteEngineBinding(ctx, id); err != nil {
		return nil, err
	}
	return &iapiserver.DeleteResult{ID: id, Deleted: true}, nil
}

// ResolveBindingStatus 按当前不可变目录修正绑定的系统管理标记与有效状态投影。
func (s *EngineService) ResolveBindingStatus(binding *iapiserver.EngineCapabilityBinding) {
	capability, ok := s.capabilities.Get(binding.ProviderCapabilityID)
	binding.SystemManaged = ok && capability.Origin == iapiserver.ProviderCapabilityOriginStatic && capability.BindingPolicy == iapiserver.ProviderBindingPolicyRequiredImmutable
	switch {
	case !binding.Enabled:
		binding.EffectiveStatus = iapiserver.BindingEffectiveDisabled
	case !ok || capability.Availability != iapiserver.ProviderCapabilityAvailable || capability.Revision != binding.ProviderCapabilityRevision:
		binding.EffectiveStatus = iapiserver.BindingEffectiveUnavailable
	default:
		binding.EffectiveStatus = iapiserver.BindingEffectiveAvailable
	}
}

func (s *EngineService) isSystemManagedBinding(binding *iapiserver.EngineCapabilityBinding) bool {
	if binding == nil {
		return false
	}
	capability, ok := s.capabilities.Get(binding.ProviderCapabilityID)
	return ok && capability.Origin == iapiserver.ProviderCapabilityOriginStatic && capability.BindingPolicy == iapiserver.ProviderBindingPolicyRequiredImmutable
}
