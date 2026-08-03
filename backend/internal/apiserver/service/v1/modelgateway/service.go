package modelgateway

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	identitymiddleware "github.com/wangweihong/omnimam/backend/internal/apiserver/middleware"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
	"github.com/wangweihong/omnimam/backend/pkg/helpers"
)

// ProviderCapabilitySrv 提供 Model Gateway 的只读静态 ProviderCapability 目录查询。
type ProviderCapabilitySrv interface {
	ListProviderCapabilities(context.Context, *iapiserver.ProviderCapabilityListRequest) (*iapiserver.ProviderCapabilityListResponse, error)
	GetProviderCapability(context.Context, string) (*iapiserver.AIAppProviderCapability, error)
}

// Principal 表示 Model Gateway 与 Application Platform 共享的调用方身份裁剪结果。
type Principal struct {
	UserID      string
	Admin       bool
	Permissions map[string]struct{}
}

// HasPermission 判断当前请求已验证的权限投影，通配权限仅用于受信的全量授权主体。
func (p Principal) HasPermission(permission string) bool {
	if _, ok := p.Permissions[permission]; ok {
		return true
	}
	_, ok := p.Permissions["*"]
	return ok
}

// PrincipalResolver 从请求上下文解析调用方身份与管理员权限。
type PrincipalResolver interface {
	Resolve(context.Context) (Principal, error)
}

// StorePrincipalResolver 使用 Identity store 解析请求身份与管理员角色。
type StorePrincipalResolver struct {
	store store.Factory
}

// NewStorePrincipalResolver 构造由持久化角色分配驱动的身份解析器。
func NewStorePrincipalResolver(factory store.Factory) *StorePrincipalResolver {
	return &StorePrincipalResolver{store: factory}
}

// Resolve 返回当前认证用户及其管理员标记，不写入任何身份事实。
func (r *StorePrincipalResolver) Resolve(ctx context.Context) (Principal, error) {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err != nil || user == nil || user.ID == "" {
		return Principal{}, errors.NewStatus(code.ErrAIAppPermissionDenied, "authenticated user is required")
	}
	principal := Principal{UserID: user.ID, Admin: user.ID == "system-admin"}
	if verified, ok := identitymiddleware.PrincipalFromContext(ctx); ok && verified.PrincipalID == user.ID {
		principal.Permissions = make(map[string]struct{}, len(verified.Permissions))
		for permission := range verified.Permissions {
			principal.Permissions[permission] = struct{}{}
		}
	}
	if principal.Admin {
		return principal, nil
	}
	principal.Admin, err = r.store.Identities().UserHasAnyRole(ctx, user.ID, []string{"ADMIN", "SUPER_ADMIN"})
	if err != nil {
		return Principal{}, errors.WithStack(err)
	}
	return principal, nil
}

// Dependencies 声明 ProviderCapability 服务需要的注册表与身份解析边界。
type Dependencies struct {
	Capabilities *ProviderCapabilityRegistry
	Principals   PrincipalResolver
}

// Service 实现 Model Gateway 的 ProviderCapability 查询服务。
type Service struct {
	capabilities *ProviderCapabilityRegistry
	principals   PrincipalResolver
}

// NewService 构造只读 ProviderCapability 服务，不触发目录加载或持久化写入。
func NewService(deps Dependencies) (*Service, error) {
	if deps.Capabilities == nil || deps.Principals == nil {
		return nil, errors.New("model gateway capability registry and principal resolver are required")
	}
	return &Service{capabilities: deps.Capabilities, principals: deps.Principals}, nil
}

func (s *Service) principal(ctx context.Context, admin bool) (Principal, error) {
	principal, err := s.principals.Resolve(ctx)
	if err != nil {
		return Principal{}, err
	}
	if admin && !principal.Admin {
		return Principal{}, errors.NewStatus(code.ErrAIAppPermissionDenied, "administrator permission is required")
	}
	return principal, nil
}

// ListProviderCapabilities 返回当前不可变目录快照，支持引擎类型、可用性与关键字过滤。
func (s *Service) ListProviderCapabilities(ctx context.Context, req *iapiserver.ProviderCapabilityListRequest) (*iapiserver.ProviderCapabilityListResponse, error) {
	if _, err := s.principal(ctx, false); err != nil {
		return nil, err
	}
	items := s.capabilities.Capabilities()
	filtered := items[:0]
	for _, item := range items {
		if req.ApplicationEngineTypeID != "" && item.ApplicationEngineTypeID != req.ApplicationEngineTypeID {
			continue
		}
		if req.Availability != "" && item.Availability != req.Availability {
			continue
		}
		if !helpers.MatchesKeyword(req.Keyword, item.ID, item.NameI18n["zh-CN"], item.NameI18n["en-US"], item.DescriptionI18n["zh-CN"], item.DescriptionI18n["en-US"]) {
			continue
		}
		filtered = append(filtered, item)
	}
	window, err := req.PagingParams.Normalize()
	if err != nil {
		return nil, err
	}
	return &iapiserver.ProviderCapabilityListResponse{
		Total: len(filtered), Items: imachinery.PaginateSlice(filtered, window),
	}, nil
}

// GetProviderCapability 返回指定 ProviderCapability 的当前不可变目录事实。
func (s *Service) GetProviderCapability(ctx context.Context, id string) (*iapiserver.AIAppProviderCapability, error) {
	if _, err := s.principal(ctx, false); err != nil {
		return nil, err
	}
	item, ok := s.capabilities.Get(id)
	if !ok {
		return nil, errors.NewStatus(code.ErrAIAppProviderCapabilityNotFound, "provider capability not found")
	}
	return item, nil
}

var _ ProviderCapabilitySrv = (*Service)(nil)
