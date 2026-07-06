package auth

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type passwordAuthenticator struct {
	store store.Factory
}

func (a *passwordAuthenticator) Authenticate(ctx context.Context, creds map[string]string) (string, error) {
	if username, ok := creds["username"]; ok {
		user, err := store.Client().Users().GetByName(ctx, username)
		if err != nil {
			log.Errorf("users %v not exist", username)
			return "", errors.New("invalid credentials")
		}

		if password, ok := creds["password"]; ok {
			if password == user.Password {
				return user.Name, nil
			}
		}
	}
	return "", errors.New("invalid credentials")
}

func (a *passwordAuthenticator) Type() string { return "password" }

type LDAPAuthenticator struct {
	store store.Factory
}

func (a *LDAPAuthenticator) Authenticate(ctx context.Context, creds map[string]string) (string, error) {
	//ldap.Authentication()
	// 实现LDAP验证逻辑
	return "ldap_user", nil
}
func (a *LDAPAuthenticator) Type() string { return "ldap" }
