package auth

import (
	"net/http"

	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

// 认证方式接口
type Authenticator interface {
	Authenticate(credentials map[string]string) (string, error) // 返回用户ID
	Type() string
}

// 令牌生成器接口
type TokenProvider interface {
	GenerateToken(userID string) (string, error)
	ValidateToken(token string) (userID string, claims map[string]string, err error)
}

// 二次验证器接口
type TwoFactorAuthenticator interface {
	SendCode(userID string) error
	VerifyCode(userID, code string) bool
}

type SSOProvider interface {
	InitiateLogin(redirectURL string) (authURL string, err error)
	HandleCallback(r *http.Request) (userID string, claims map[string]string, err error)
	Type() string
}

type authService struct {
	store store.Factory
}
