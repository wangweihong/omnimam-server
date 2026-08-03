package identity

import (
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

// Service 同时承载旧身份接口与已发布 Identity 契约，统一复用同一 Store Factory 和 JWT 配置。
type Service struct {
	store  store.Factory
	secret []byte
	opaque OpaqueAdapter
}

// NewService 创建 Identity service；参数依次为 JWT secret 和稳定 OPAQUE setup hex。
func NewService(str store.Factory, secrets ...string) *Service {
	secret := ""
	setup := ""
	if len(secrets) > 0 && secrets[0] != "" {
		secret = secrets[0]
	}
	if len(secrets) > 1 {
		setup = secrets[1]
	}
	adapter, _ := newOpaqueAdapter(setup)
	return &Service{store: str, secret: []byte(secret), opaque: adapter}
}
