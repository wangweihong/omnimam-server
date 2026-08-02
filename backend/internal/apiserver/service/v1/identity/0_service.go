package identity

import (
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

// Service 同时承载旧身份接口与已发布 Identity 契约，统一复用同一 Store Factory 和 JWT 配置。
type Service struct {
	store  store.Factory
	secret []byte
}

// NewService 创建 Identity service；可选 jwtSecret 用于签发与 middleware 验签一致的 Access Token。
func NewService(str store.Factory, jwtSecret ...string) *Service {
	secret := ""
	if len(jwtSecret) > 0 && jwtSecret[0] != "" {
		secret = jwtSecret[0]
	}
	return &Service{store: str, secret: []byte(secret)}
}
