package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/options"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/codec"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

// Authentication 在 API handler 执行前解析并注入当前用户；匿名管理员仅允许显式开启的非 release 环境使用。
func Authentication(authOptions *options.AuthOptions, mode string, userStore store.UserStore) gin.HandlerFunc {
	allowAnonymous := authOptions != nil && authOptions.AllowAnonymousDevelopment && mode != "release"
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/v1/auth/") {
			c.Next()
			return
		}
		user, err := resolveUser(c, userStore)
		if err != nil {
			if allowAnonymous && isMissingCredentials(c) {
				user = &iapiserver.User{}
				user.ID = "system-admin"
				user.Name = "system-admin"
			} else {
				core.WriteResponse(c, err, nil)
				c.Abort()
				return
			}
		}
		setUserContext(c, user)
		c.Next()
	}
}

func resolveUser(c *gin.Context, userStore store.UserStore) (*iapiserver.User, error) {
	raw := strings.TrimSpace(c.GetHeader("Authorization"))
	if raw == "" {
		if token, err := c.Cookie(iapiserver.CookieKeyToken); err == nil {
			raw = "Bearer " + token
		}
	}
	if raw == "" {
		return nil, errors.NewStatus(code.ErrMissingHeader, "authentication credentials are required")
	}
	parts := strings.Fields(raw)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return nil, errors.NewStatus(code.ErrInvalidAuthHeader, "invalid bearer authorization header")
	}
	claims, err := codec.ParseUserTokenStr(parts[1])
	if err != nil || claims.UserUUID == "" {
		return nil, errors.NewStatus(code.ErrTokenInvalid, "token is invalid")
	}
	if userStore == nil {
		return nil, errors.NewStatus(code.ErrTokenInvalid, "user store is not configured")
	}
	user, err := userStore.Get(c.Request.Context(), claims.UserUUID)
	if err != nil || user == nil || user.ID == "" {
		return nil, errors.NewStatus(code.ErrTokenInvalid, "authenticated user was not found")
	}
	return user, nil
}

func isMissingCredentials(c *gin.Context) bool {
	if strings.TrimSpace(c.GetHeader("Authorization")) != "" {
		return false
	}
	_, err := c.Cookie(iapiserver.CookieKeyToken)
	return err != nil
}

func setUserContext(c *gin.Context, user *iapiserver.User) {
	c.Set(iapiserver.GinContextKeyUser, user)
	ctx := context.WithValue(c.Request.Context(), iapiserver.GinContextKeyUser, user)
	c.Request = c.Request.WithContext(ctx)
}
