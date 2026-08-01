package comfyui

import (
	"context"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"gorm.io/gorm"
)

// ObjectInfoReader 读取 ComfyUI 实例当前节点目录，不提交工作流。
type ObjectInfoReader interface {
	ReadObjectInfo(context.Context, *iapiserver.EngineInstance) (map[string]any, error)
}

// VersionReader 读取 ComfyUI 实例版本元数据。
type VersionReader interface {
	ReadComfyUIVersion(context.Context, *iapiserver.EngineInstance) (string, error)
}

// ObjectInfoSrv 提供 ComfyUI 节点目录的读取、刷新和执行前解析能力。
type ObjectInfoSrv interface {
	GetComfyUIEngineObjectInfo(context.Context, string) (*iapiserver.ComfyUIEngineObjectInfoResponse, error)
	RefreshComfyUIEngineObjectInfo(context.Context, string) (*iapiserver.ComfyUIEngineObjectInfoStatus, error)
	RefreshComfyUIEngineObjectInfoInternal(context.Context, string) (*iapiserver.ComfyUIEngineObjectInfoStatus, error)
	ResolveUsableObjectInfo(context.Context, string) (*iapiserver.EngineInstance, *iapiserver.ComfyUIEngineObjectInfo, error)
}

// ObjectInfoReaderResolver 延迟解析当前 Runtime Registry 对应的 ComfyUI adapter。
type ObjectInfoReaderResolver func() (ObjectInfoReader, error)

// ObjectInfoService 实现 ComfyUI 节点目录管理。
type ObjectInfoService struct {
	store      store.Factory
	principals modelgateway.PrincipalResolver
	reader     ObjectInfoReaderResolver
}

// NewObjectInfoService 构造 ComfyUI 节点目录服务。
func NewObjectInfoService(factory store.Factory, principals modelgateway.PrincipalResolver, reader ObjectInfoReaderResolver) (*ObjectInfoService, error) {
	if factory == nil || principals == nil || reader == nil {
		return nil, fmt.Errorf("comfyui object-info store, principal resolver, and reader resolver are required")
	}
	return &ObjectInfoService{store: factory, principals: principals, reader: reader}, nil
}

func (s *ObjectInfoService) principal(ctx context.Context, admin bool) (modelgateway.Principal, error) {
	principal, err := s.principals.Resolve(ctx)
	if err != nil {
		return modelgateway.Principal{}, err
	}
	if admin && !principal.Admin {
		return modelgateway.Principal{}, errors.NewStatus(code.ErrAIAppPermissionDenied, "administrator permission is required")
	}
	return principal, nil
}

// GetComfyUIEngineObjectInfo 返回实例最后一次成功刷新的当前目录；stale 目录仅允许用于诊断读取。
func (s *ObjectInfoService) GetComfyUIEngineObjectInfo(ctx context.Context, engineID string) (*iapiserver.ComfyUIEngineObjectInfoResponse, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	engine, err := s.store.ApplicationPlatforms().GetEngineInstance(ctx, engineID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppEngineInstanceNotFound, "engine instance not found")
	}
	if engine.ApplicationEngineTypeID != "comfyui" {
		return nil, errors.NewStatus(code.ErrAIAppComfyUIEngineTypeInvalid, "engine instance is not ComfyUI")
	}
	catalog, err := s.store.ApplicationPlatforms().GetComfyUIEngineObjectInfo(ctx, engineID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIAppComfyUIObjectInfoUnavailable, "current object_info is unavailable")
	}
	return &iapiserver.ComfyUIEngineObjectInfoResponse{
		EngineInstanceID: engineID, Available: true, Stale: catalog.Stale(time.Now()),
		ComfyUIVersion: catalog.ComfyUIVersion, RefreshedAt: catalog.RefreshedAt, ObjectInfo: catalog.ObjectInfo,
	}, nil
}

// RefreshComfyUIEngineObjectInfo 手动刷新当前目录，仅管理员可调用。
func (s *ObjectInfoService) RefreshComfyUIEngineObjectInfo(ctx context.Context, engineID string) (*iapiserver.ComfyUIEngineObjectInfoStatus, error) {
	if _, err := s.principal(ctx, true); err != nil {
		return nil, err
	}
	return s.refreshComfyUIObjectInfo(ctx, engineID)
}

// RefreshComfyUIEngineObjectInfoInternal 供 SYSTEM RECONCILE 使用，不经过 HTTP 用户鉴权。
func (s *ObjectInfoService) RefreshComfyUIEngineObjectInfoInternal(ctx context.Context, engineID string) (*iapiserver.ComfyUIEngineObjectInfoStatus, error) {
	return s.refreshComfyUIObjectInfo(ctx, engineID)
}

func (s *ObjectInfoService) refreshComfyUIObjectInfo(ctx context.Context, engineID string) (*iapiserver.ComfyUIEngineObjectInfoStatus, error) {
	catalog, err := s.store.ApplicationPlatforms().RefreshComfyUIEngineObjectInfo(ctx, engineID, func(engine *iapiserver.EngineInstance) (*iapiserver.ComfyUIEngineObjectInfo, error) {
		if engine.ApplicationEngineTypeID != "comfyui" {
			return nil, errors.NewStatus(code.ErrAIAppComfyUIEngineTypeInvalid, "engine instance is not ComfyUI")
		}
		if !engine.Enabled || engine.HealthStatus != iapiserver.EngineHealthOnline {
			return nil, errors.NewStatus(code.ErrAIAppComfyUIObjectInfoRefreshNotAllowed, "engine instance must be enabled and online")
		}
		reader, err := s.reader()
		if err != nil {
			return nil, errors.NewStatus(code.ErrAIAppComfyUIObjectInfoRefreshFailed, err.Error())
		}
		objectInfo, err := reader.ReadObjectInfo(ctx, engine)
		if err != nil {
			return nil, errors.NewStatus(code.ErrAIAppComfyUIObjectInfoRefreshFailed, "provider object_info request failed")
		}
		if err := ValidateCurrentObjectInfo(objectInfo); err != nil {
			return nil, errors.NewStatus(code.ErrAIAppComfyUIObjectInfoRefreshFailed, err.Error())
		}
		version := ""
		if versionReader, ok := reader.(VersionReader); ok {
			version, _ = versionReader.ReadComfyUIVersion(ctx, engine)
		}
		return &iapiserver.ComfyUIEngineObjectInfo{
			EngineInstanceID: engineID, ObjectInfo: objectInfo, ComfyUIVersion: version, RefreshedAt: imachinery.Now(),
		}, nil
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
	return &iapiserver.ComfyUIEngineObjectInfoStatus{
		EngineInstanceID: engineID, Available: true, Stale: false,
		ComfyUIVersion: version, RefreshedAt: &refreshedAt,
	}, nil
}

// ValidateCurrentObjectInfo 拒绝缺少输入或输出定义的不完整 ComfyUI 节点目录。
func ValidateCurrentObjectInfo(objectInfo map[string]any) error {
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

// ResolveUsableObjectInfo 返回可执行工作流的 enabled、online 且未过期实例目录。
func (s *ObjectInfoService) ResolveUsableObjectInfo(ctx context.Context, engineID string) (*iapiserver.EngineInstance, *iapiserver.ComfyUIEngineObjectInfo, error) {
	engine, err := s.store.ApplicationPlatforms().GetEngineInstance(ctx, engineID)
	if err != nil {
		return nil, nil, mapNotFound(err, code.ErrAIAppEngineInstanceNotFound, "engine instance not found")
	}
	if engine.ApplicationEngineTypeID != "comfyui" {
		return nil, nil, errors.NewStatus(code.ErrAIAppComfyUIEngineTypeInvalid, "engine instance is not ComfyUI")
	}
	if !engine.Enabled || engine.HealthStatus != iapiserver.EngineHealthOnline {
		return nil, nil, errors.NewStatus(code.ErrAIAppComfyUIObjectInfoUnavailable, "ComfyUI engine instance is not enabled and online")
	}
	catalog, err := s.store.ApplicationPlatforms().GetComfyUIEngineObjectInfo(ctx, engineID)
	if err != nil || catalog.Stale(time.Now()) {
		return nil, nil, errors.NewStatus(code.ErrAIAppComfyUIObjectInfoUnavailable, "current object_info is missing or stale")
	}
	return engine, catalog, nil
}

func mapNotFound(err error, businessCode int, message string) error {
	if err == nil {
		return nil
	}
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return errors.NewStatus(businessCode, message)
	}
	return err
}
