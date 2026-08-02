package postgresql

import (
	"context"
	stderrors "errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type identityV11Store struct{ ds *datastore }

func newIdentityV11Store(ds *datastore) *identityV11Store { return &identityV11Store{ds: ds} }

func (s *identityV11Store) GetUser(ctx context.Context, id string) (*iapiserver.IdentityUser, error) {
	var user iapiserver.IdentityUser
	err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	return &user, err
}

func (s *identityV11Store) GetUserByLogin(ctx context.Context, login string) (*iapiserver.IdentityUser, error) {
	var user iapiserver.IdentityUser
	normalized := strings.ToLower(strings.TrimSpace(login))
	err := s.ds.db.WithContext(ctx).Where("normalized_username = ? OR normalized_email = ?", normalized, normalized).First(&user).Error
	return &user, err
}

func (s *identityV11Store) ListUsers(ctx context.Context, req *iapiserver.IdentityUserListRequest) ([]*iapiserver.IdentityUser, int64, error) {
	var users []*iapiserver.IdentityUser
	query := req.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.IdentityUser{}), func(q *gorm.DB) *gorm.DB {
		if req.Statuses == "" {
			return q.Where("status <> ?", iapiserver.IdentityUserDeleted)
		}
		return q.Where("status IN ?", strings.Split(req.Statuses, ","))
	})
	total, err := CountAndFindPage(query, req.PagingParams, &users)
	return users, total, err
}

func (s *identityV11Store) ListPermissionDefinitions(ctx context.Context, req *iapiserver.IdentityPermissionListRequest) ([]*iapiserver.IdentityPermissionDefinition, int64, error) {
	var items []*iapiserver.IdentityPermissionDefinition
	query := req.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.IdentityPermissionDefinition{}), nil)
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *identityV11Store) CreateUser(ctx context.Context, user *iapiserver.IdentityUser) (*iapiserver.IdentityUser, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&iapiserver.IdentityUser{}).Where("status <> ?", iapiserver.IdentityUserDeleted).Count(&count).Error; err != nil {
			return err
		}
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		// The first local account receives the bootstrap role; subsequent authorization changes remain explicit RBAC facts.
		if count == 0 {
			var role iapiserver.IdentityRole
			if err := tx.Where("code = ?", "super_admin").First(&role).Error; err == nil {
				grant := &iapiserver.IdentityUserRoleGrant{ID: uuid.NewString(), UserID: user.ID, RoleID: role.ID, CreatedAt: imachinery.Now()}
				if err := tx.Create(grant).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	return user, err
}

func (s *identityV11Store) UpdateUser(ctx context.Context, user *iapiserver.IdentityUser) (*iapiserver.IdentityUser, error) {
	var current iapiserver.IdentityUser
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", user.ID).First(&current).Error; err != nil {
			return err
		}
		updates := map[string]any{
			"display_name": user.DisplayName, "alias": user.Alias, "phone": user.Phone,
			"status": user.Status, "first_login_required": user.FirstLoginRequired,
			"security_version": user.SecurityVersion, "authorization_version": user.AuthorizationVersion,
		}
		if user.Email != nil {
			updates["email"] = *user.Email
			normalized := strings.ToLower(strings.TrimSpace(*user.Email))
			updates["normalized_email"] = normalized
		}
		if user.PasswordHash != "" {
			updates["password_hash"] = user.PasswordHash
			updates["password_changed_at"] = user.PasswordChangedAt
		}
		if user.LastLoginAt != nil {
			updates["last_login_at"] = user.LastLoginAt
		}
		if err := tx.Model(&current).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", user.ID).First(&current).Error
	})
	return &current, err
}

func (s *identityV11Store) GetSession(ctx context.Context, id string) (*iapiserver.IdentityAuthSession, error) {
	var session iapiserver.IdentityAuthSession
	err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&session).Error
	return &session, err
}

func (s *identityV11Store) ListSessions(ctx context.Context, userID string, req *iapiserver.IdentitySessionListRequest) ([]*iapiserver.IdentityAuthSession, int64, error) {
	var sessions []*iapiserver.IdentityAuthSession
	query := req.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.IdentityAuthSession{}), func(q *gorm.DB) *gorm.DB { return q.Where("user_id = ?", userID) })
	total, err := CountAndFindPage(query, req.PagingParams, &sessions)
	return sessions, total, err
}

func (s *identityV11Store) CreateSession(ctx context.Context, session *iapiserver.IdentityAuthSession) (*iapiserver.IdentityAuthSession, error) {
	if session.ID == "" {
		session.ID = uuid.NewString()
	}
	err := s.ds.db.WithContext(ctx).Create(session).Error
	return session, err
}

func (s *identityV11Store) TouchSession(ctx context.Context, id string, at imachinery.Time) (*iapiserver.IdentityAuthSession, error) {
	if err := s.ds.db.WithContext(ctx).Model(&iapiserver.IdentityAuthSession{}).Where("id = ? AND status = ?", id, "ACTIVE").Updates(map[string]any{"last_active_at": at, "updated_at": at}).Error; err != nil {
		return nil, err
	}
	return s.GetSession(ctx, id)
}

func (s *identityV11Store) RevokeSession(ctx context.Context, id, reason string) error {
	now := imachinery.Now()
	return s.ds.db.WithContext(ctx).Model(&iapiserver.IdentityAuthSession{}).Where("id = ? AND status = ?", id, "ACTIVE").Updates(map[string]any{"status": "REVOKED", "revoked_at": now, "revoke_reason": reason, "updated_at": now}).Error
}

func (s *identityV11Store) RevokeUserSessions(ctx context.Context, userID, reason string) error {
	now := imachinery.Now()
	return s.ds.db.WithContext(ctx).Model(&iapiserver.IdentityAuthSession{}).Where("user_id = ? AND status = ?", userID, "ACTIVE").Updates(map[string]any{"status": "REVOKED", "revoked_at": now, "revoke_reason": reason, "updated_at": now}).Error
}

func (s *identityV11Store) CreateTokenCredential(ctx context.Context, token *iapiserver.IdentityTokenCredential) error {
	return s.ds.db.WithContext(ctx).Create(token).Error
}

func (s *identityV11Store) GetTokenCredentialByJTI(ctx context.Context, jti string) (*iapiserver.IdentityTokenCredential, error) {
	var token iapiserver.IdentityTokenCredential
	err := s.ds.db.WithContext(ctx).Where("access_token_jti = ?", jti).First(&token).Error
	return &token, err
}

func (s *identityV11Store) RevokeTokenCredential(ctx context.Context, jti string) error {
	now := imachinery.Now()
	return s.ds.db.WithContext(ctx).Model(&iapiserver.IdentityTokenCredential{}).Where("access_token_jti = ? AND status = ?", jti, "ACTIVE").Updates(map[string]any{"status": "REVOKED", "revoked_at": now, "updated_at": now}).Error
}

func (s *identityV11Store) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*iapiserver.IdentityRefreshToken, error) {
	var token iapiserver.IdentityRefreshToken
	err := s.ds.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&token).Error
	return &token, err
}

func (s *identityV11Store) CreateRefreshToken(ctx context.Context, token *iapiserver.IdentityRefreshToken) error {
	return s.ds.db.WithContext(ctx).Create(token).Error
}

func (s *identityV11Store) MarkRefreshTokenUsed(ctx context.Context, id string) error {
	now := imachinery.Now()
	result := s.ds.db.WithContext(ctx).Model(&iapiserver.IdentityRefreshToken{}).Where("id = ? AND status = ?", id, "ACTIVE").Updates(map[string]any{"status": "USED", "used_at": now, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return stderrors.New("refresh token was already used or revoked")
	}
	return nil
}

func (s *identityV11Store) RevokeSessionRefreshTokens(ctx context.Context, sessionID, reason string) error {
	now := imachinery.Now()
	return s.ds.db.WithContext(ctx).Model(&iapiserver.IdentityRefreshToken{}).Where("session_id = ? AND status IN ?", sessionID, []string{"ACTIVE", "USED"}).Updates(map[string]any{"status": "REVOKED", "revoked_at": now, "updated_at": now, "description": reason}).Error
}

func (s *identityV11Store) PermissionCodes(ctx context.Context, principalType, principalID string) ([]string, int64, error) {
	if principalType != "USER" {
		return nil, 0, nil
	}
	var user iapiserver.IdentityUser
	if err := s.ds.db.WithContext(ctx).Where("id = ?", principalID).First(&user).Error; err != nil {
		return nil, 0, err
	}
	var codes []string
	query := `SELECT DISTINCT rp.permission_code FROM identity_user_role_grants ur JOIN identity_role_permission_grants rp ON rp.role_id = ur.role_id WHERE ur.user_id = ? UNION SELECT DISTINCT rp.permission_code FROM identity_group_members gm JOIN identity_group_role_grants gr ON gr.group_id = gm.group_id JOIN identity_role_permission_grants rp ON rp.role_id = gr.role_id WHERE gm.user_id = ?`
	if err := s.ds.db.WithContext(ctx).Raw(query, principalID, principalID).Scan(&codes).Error; err != nil {
		return nil, 0, err
	}
	return codes, user.AuthorizationVersion, nil
}

func (s *identityV11Store) EnsureDefaultPermissions(ctx context.Context, permissions []*iapiserver.IdentityPermissionDefinition) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, permission := range permissions {
			var existing iapiserver.IdentityPermissionDefinition
			if err := tx.Where("code = ?", permission.Code).First(&existing).Error; stderrors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(permission).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
		}
		var role iapiserver.IdentityRole
		if err := tx.Where("code = ?", "super_admin").First(&role).Error; stderrors.Is(err, gorm.ErrRecordNotFound) {
			role = iapiserver.IdentityRole{Code: "super_admin", Builtin: true, Status: "ACTIVE"}
			role.Name = "Super Administrator"
			if err := tx.Create(&role).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		for _, permission := range permissions {
			grant := &iapiserver.IdentityRolePermissionGrant{RoleID: role.ID, PermissionCode: permission.Code, CreatedAt: imachinery.Now()}
			if err := tx.Where("role_id = ? AND permission_code = ?", role.ID, permission.Code).FirstOrCreate(grant).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

var _ store.IdentityV11Store = (*identityV11Store)(nil)
