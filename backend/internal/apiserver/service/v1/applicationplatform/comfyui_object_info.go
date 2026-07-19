package applicationplatform

import (
	"context"
	"fmt"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

// GetComfyUIEngineObjectInfo 返回实例最后一次成功刷新的当前目录；stale 目录仅允许用于诊断读取。
func (s *applicationPlatformService) GetComfyUIEngineObjectInfo(ctx context.Context, engineID string) (*iapiserver.ComfyUIEngineObjectInfoResponse, error) {
	if _, err := s.principal(ctx, false); err != nil {
		return nil, err
	}
	engine, err := s.Store.ApplicationPlatforms().GetEngineInstance(ctx, engineID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppEngineInstanceNotFound, "engine instance not found")
	}
	if engine.ApplicationEngineTypeID != "comfyui" {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIEngineTypeInvalid, "engine instance is not ComfyUI")
	}
	catalog, err := s.Store.ApplicationPlatforms().GetComfyUIEngineObjectInfo(ctx, engineID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppComfyUIObjectInfoUnavailable, "current object_info is unavailable")
	}
	return &iapiserver.ComfyUIEngineObjectInfoResponse{
		EngineInstanceID: engineID,
		Available:        true,
		Stale:            catalog.Stale(time.Now()),
		ComfyUIVersion:   catalog.ComfyUIVersion,
		RefreshedAt:      catalog.RefreshedAt,
		ObjectInfo:       catalog.ObjectInfo,
	}, nil
}

// RefreshComfyUIEngineObjectInfo 手动刷新当前目录，仅管理员可调用。
func (s *applicationPlatformService) RefreshComfyUIEngineObjectInfo(ctx context.Context, engineID string) (*iapiserver.ComfyUIEngineObjectInfoStatus, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	return s.refreshComfyUIEngineObjectInfo(ctx, engineID)
}

// RefreshComfyUIEngineObjectInfoInternal 供 SYSTEM RECONCILE 使用，不经过 HTTP 用户鉴权。
func (s *applicationPlatformService) RefreshComfyUIEngineObjectInfoInternal(ctx context.Context, engineID string) (*iapiserver.ComfyUIEngineObjectInfoStatus, error) {
	return s.refreshComfyUIEngineObjectInfo(ctx, engineID)
}

func (s *applicationPlatformService) refreshComfyUIEngineObjectInfo(ctx context.Context, engineID string) (*iapiserver.ComfyUIEngineObjectInfoStatus, error) {
	catalog, err := s.Store.ApplicationPlatforms().RefreshComfyUIEngineObjectInfo(ctx, engineID, func(engine *iapiserver.EngineInstance) (*iapiserver.ComfyUIEngineObjectInfo, error) {
		if engine.ApplicationEngineTypeID != "comfyui" {
			return nil, errors.NewStatus(code.ErrAIAppComfyUIEngineTypeInvalid, "engine instance is not ComfyUI")
		}
		if !engine.Enabled || engine.HealthStatus != iapiserver.EngineHealthOnline {
			return nil, errors.NewStatus(code.ErrAIAppComfyUIObjectInfoRefreshNotAllowed, "engine instance must be enabled and online")
		}
		reader, err := s.comfyUIObjectInfoReader()
		if err != nil {
			return nil, errors.NewStatus(code.ErrAIAppComfyUIObjectInfoRefreshFailed, err.Error())
		}
		objectInfo, err := reader.ReadObjectInfo(ctx, engine)
		if err != nil {
			return nil, errors.NewStatus(code.ErrAIAppComfyUIObjectInfoRefreshFailed, "provider object_info request failed")
		}
		if err := validateCurrentObjectInfo(objectInfo); err != nil {
			return nil, errors.NewStatus(code.ErrAIAppComfyUIObjectInfoRefreshFailed, err.Error())
		}
		version := ""
		if versionReader, ok := reader.(ComfyUIVersionReader); ok {
			version, _ = versionReader.ReadComfyUIVersion(ctx, engine)
		}
		return &iapiserver.ComfyUIEngineObjectInfo{EngineInstanceID: engineID, ObjectInfo: objectInfo, ComfyUIVersion: version, RefreshedAt: imachinery.Now()}, nil
	})
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppEngineInstanceNotFound, "engine instance not found")
	}
	var version *string
	if catalog.ComfyUIVersion != "" {
		value := catalog.ComfyUIVersion
		version = &value
	}
	refreshedAt := catalog.RefreshedAt
	return &iapiserver.ComfyUIEngineObjectInfoStatus{EngineInstanceID: engineID, Available: true, Stale: false, ComfyUIVersion: version, RefreshedAt: &refreshedAt}, nil
}

func validateCurrentObjectInfo(objectInfo map[string]any) error {
	if len(objectInfo) == 0 {
		return fmt.Errorf("comfyui object_info is empty")
	}
	for classType, raw := range objectInfo {
		definition, ok := raw.(map[string]any)
		if !ok || classType == "" {
			return fmt.Errorf("comfyui object_info contains an invalid node definition")
		}
		if _, hasInput := definition["input"]; !hasInput {
			return fmt.Errorf("comfyui object_info node %s has no input definition", classType)
		}
		if _, hasOutput := definition["output"]; !hasOutput {
			return fmt.Errorf("comfyui object_info node %s has no output definition", classType)
		}
	}
	return nil
}

func (s *applicationPlatformService) comfyUIObjectInfoReader() (ComfyUIObjectInfoReader, error) {
	typeDef, ok := s.Runtime.EngineType("comfyui")
	if !ok {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIEngineTypeInvalid, "ComfyUI engine type is not registered")
	}
	reader, ok := s.Adapters[typeDef.EngineAdapterID].(ComfyUIObjectInfoReader)
	if !ok {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIObjectInfoUnavailable, "ComfyUI object_info reader is unavailable")
	}
	return reader, nil
}

func (s *applicationPlatformService) usableComfyUIObjectInfo(ctx context.Context, engineID string) (*iapiserver.EngineInstance, *iapiserver.ComfyUIEngineObjectInfo, error) {
	engine, err := s.Store.ApplicationPlatforms().GetEngineInstance(ctx, engineID)
	if err != nil {
		return nil, nil, mapNotFound(err, code.ErrAIAppEngineInstanceNotFound, "engine instance not found")
	}
	if engine.ApplicationEngineTypeID != "comfyui" {
		return nil, nil, errors.NewStatus(code.ErrAIAppComfyUIEngineTypeInvalid, "engine instance is not ComfyUI")
	}
	if !engine.Enabled || engine.HealthStatus != iapiserver.EngineHealthOnline {
		return nil, nil, errors.NewStatus(code.ErrAIAppComfyUIObjectInfoUnavailable, "ComfyUI engine instance is not enabled and online")
	}
	catalog, err := s.Store.ApplicationPlatforms().GetComfyUIEngineObjectInfo(ctx, engineID)
	if err != nil || catalog.Stale(time.Now()) {
		return nil, nil, errors.NewStatus(code.ErrAIAppComfyUIObjectInfoUnavailable, "current object_info is missing or stale")
	}
	return engine, catalog, nil
}
