package postgresql

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type identityStore struct{ ds *datastore }

func newIdentityStore(ds *datastore) *identityStore { return &identityStore{ds: ds} }

func (s *identityStore) GetUser(ctx context.Context, id string) (*iapiserver.IdentityUser, error) {
	var user iapiserver.IdentityUser
	err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	return &user, err
}

func (s *identityStore) GetUserByLogin(ctx context.Context, login string) (*iapiserver.IdentityUser, error) {
	var user iapiserver.IdentityUser
	normalized := strings.ToLower(strings.TrimSpace(login))
	err := s.ds.db.WithContext(ctx).Where("normalized_username = ? OR normalized_email = ?", normalized, normalized).First(&user).Error
	return &user, err
}

func (s *identityStore) ListUsers(ctx context.Context, req *iapiserver.IdentityUserListRequest) ([]*iapiserver.IdentityUser, int64, error) {
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

func (s *identityStore) ListPermissionDefinitions(ctx context.Context, req *iapiserver.IdentityPermissionListRequest) ([]*iapiserver.IdentityPermissionDefinition, int64, error) {
	var items []*iapiserver.IdentityPermissionDefinition
	query := req.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.IdentityPermissionDefinition{}), nil)
	total, err := CountAndFindPage(query, req.PagingParams, &items)
	return items, total, err
}

func (s *identityStore) CreateUser(ctx context.Context, user *iapiserver.IdentityUser) (*iapiserver.IdentityUser, error) {
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
			if err := tx.Where("code = ?", "SUPER_ADMIN").First(&role).Error; err == nil {
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

// CreateOpenRegistration 原子创建 OPEN 注册用户、默认 USER 授权、可靠事件和脱敏审计。
func (s *identityStore) CreateOpenRegistration(ctx context.Context, user *iapiserver.IdentityUser) (*iapiserver.IdentityUser, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var activeCount int64
		if err := tx.Model(&iapiserver.IdentityUser{}).Where("status <> ?", iapiserver.IdentityUserDeleted).Count(&activeCount).Error; err != nil {
			return err
		}
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		if activeCount == 0 {
			var bootstrapRole iapiserver.IdentityRole
			if err := tx.Where("code = ?", "SUPER_ADMIN").First(&bootstrapRole).Error; err != nil {
				return errors.NewStatus(code.ErrIdentityUserCreateInvalid, "bootstrap role is unavailable")
			}
			if err := tx.Create(&iapiserver.IdentityUserRoleGrant{ID: uuid.NewString(), UserID: user.ID, RoleID: bootstrapRole.ID, CreatedAt: imachinery.Now()}).Error; err != nil {
				return err
			}
		}
		var role iapiserver.IdentityRole
		if err := tx.Where("code = ? AND builtin = ? AND status = ?", "USER", true, "ACTIVE").First(&role).Error; err != nil {
			return errors.NewStatus(code.ErrIdentityUserCreateInvalid, "active builtin USER role is unavailable")
		}
		now := imachinery.Now()
		if err := tx.Create(&iapiserver.IdentityUserRoleGrant{ID: uuid.NewString(), UserID: user.ID, RoleID: role.ID, CreatedAt: now}).Error; err != nil {
			return err
		}
		if err := createIdentityOutbox(tx, "identity.user.created", "user", user.ID, user.ResourceVersion, map[string]any{
			"user_id": user.ID, "status": iapiserver.IdentityUserActive, "source": "LOCAL", "actor_principal_type": "USER", "actor_principal_id": user.ID,
		}, fmt.Sprintf("identity.user.created:%s:%d", user.ID, user.ResourceVersion)); err != nil {
			return err
		}
		if err := createIdentityOutbox(tx, "identity.authorization.changed", "user", user.ID, user.AuthorizationVersion, map[string]any{
			"principal_type": "USER", "principal_id": user.ID, "authorization_version": user.AuthorizationVersion,
			"change_source_type": "USER_ROLE", "change_source_id": role.ID, "changed_permission_codes": []string{},
			"actor_principal_type": "USER", "actor_principal_id": user.ID,
		}, fmt.Sprintf("identity.authorization.changed:USER:%s:%d", user.ID, user.AuthorizationVersion)); err != nil {
			return err
		}
		return appendIdentityOpenRegistrationAudit(tx, user, now)
	})
	return user, err
}

func (s *identityStore) UpdateUser(ctx context.Context, user *iapiserver.IdentityUser) (*iapiserver.IdentityUser, error) {
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
		if user.OpaqueRegistrationRecord != "" {
			updates["opaque_registration_record"] = user.OpaqueRegistrationRecord
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

func (s *identityStore) GetSession(ctx context.Context, id string) (*iapiserver.IdentityAuthSession, error) {
	var session iapiserver.IdentityAuthSession
	err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&session).Error
	return &session, err
}

func (s *identityStore) ListSessions(ctx context.Context, userID string, req *iapiserver.IdentitySessionListRequest) ([]*iapiserver.IdentityAuthSession, int64, error) {
	var sessions []*iapiserver.IdentityAuthSession
	query := req.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.IdentityAuthSession{}), func(q *gorm.DB) *gorm.DB { return q.Where("user_id = ?", userID) })
	total, err := CountAndFindPage(query, req.PagingParams, &sessions)
	return sessions, total, err
}

func (s *identityStore) CreateSession(ctx context.Context, session *iapiserver.IdentityAuthSession) (*iapiserver.IdentityAuthSession, error) {
	if session.ID == "" {
		session.ID = uuid.NewString()
	}
	err := s.ds.db.WithContext(ctx).Create(session).Error
	return session, err
}

func (s *identityStore) TouchSession(ctx context.Context, id string, at imachinery.Time) (*iapiserver.IdentityAuthSession, error) {
	if err := s.ds.db.WithContext(ctx).Model(&iapiserver.IdentityAuthSession{}).Where("id = ? AND status = ?", id, "ACTIVE").Updates(map[string]any{"last_active_at": at, "updated_at": at}).Error; err != nil {
		return nil, err
	}
	return s.GetSession(ctx, id)
}

func (s *identityStore) RevokeSession(ctx context.Context, id, reason string) error {
	now := imachinery.Now()
	return s.ds.db.WithContext(ctx).Model(&iapiserver.IdentityAuthSession{}).Where("id = ? AND status = ?", id, "ACTIVE").Updates(map[string]any{"status": "REVOKED", "revoked_at": now, "revoke_reason": reason, "updated_at": now}).Error
}

func (s *identityStore) RevokeUserSessions(ctx context.Context, userID, reason string) error {
	now := imachinery.Now()
	return s.ds.db.WithContext(ctx).Model(&iapiserver.IdentityAuthSession{}).Where("user_id = ? AND status = ?", userID, "ACTIVE").Updates(map[string]any{"status": "REVOKED", "revoked_at": now, "revoke_reason": reason, "updated_at": now}).Error
}

func (s *identityStore) CreateTokenCredential(ctx context.Context, token *iapiserver.IdentityTokenCredential) error {
	return s.ds.db.WithContext(ctx).Create(token).Error
}

func (s *identityStore) GetTokenCredentialByJTI(ctx context.Context, jti string) (*iapiserver.IdentityTokenCredential, error) {
	var token iapiserver.IdentityTokenCredential
	err := s.ds.db.WithContext(ctx).Where("access_token_jti = ?", jti).First(&token).Error
	return &token, err
}

func (s *identityStore) RevokeTokenCredential(ctx context.Context, jti string) error {
	now := imachinery.Now()
	return s.ds.db.WithContext(ctx).Model(&iapiserver.IdentityTokenCredential{}).Where("access_token_jti = ? AND status = ?", jti, "ACTIVE").Updates(map[string]any{"status": "REVOKED", "revoked_at": now, "updated_at": now}).Error
}

func (s *identityStore) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*iapiserver.IdentityRefreshToken, error) {
	var token iapiserver.IdentityRefreshToken
	err := s.ds.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&token).Error
	return &token, err
}

func (s *identityStore) CreateRefreshToken(ctx context.Context, token *iapiserver.IdentityRefreshToken) error {
	return s.ds.db.WithContext(ctx).Create(token).Error
}

func (s *identityStore) MarkRefreshTokenUsed(ctx context.Context, id string) error {
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

func (s *identityStore) RevokeSessionRefreshTokens(ctx context.Context, sessionID, reason string) error {
	now := imachinery.Now()
	return s.ds.db.WithContext(ctx).Model(&iapiserver.IdentityRefreshToken{}).Where("session_id = ? AND status IN ?", sessionID, []string{"ACTIVE", "USED"}).Updates(map[string]any{"status": "REVOKED", "revoked_at": now, "updated_at": now, "description": reason}).Error
}

func (s *identityStore) PermissionCodes(ctx context.Context, principalType, principalID string) ([]string, int64, error) {
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

// UserHasAnyRole 判断用户是否通过直接授权或组授权持有指定 Identity 角色。
func (s *identityStore) UserHasAnyRole(ctx context.Context, userID string, roleCodes []string) (bool, error) {
	if len(roleCodes) == 0 {
		return false, nil
	}
	var count int64
	if err := s.ds.db.WithContext(ctx).
		Model(&iapiserver.IdentityUserRoleGrant{}).
		Joins("JOIN identity_roles ON identity_roles.id = identity_user_role_grants.role_id").
		Where("identity_user_role_grants.user_id = ? AND identity_roles.code IN ? AND identity_roles.status = ?", userID, roleCodes, "ACTIVE").
		Count(&count).Error; err != nil {
		return false, err
	}
	if count != 0 {
		return true, nil
	}
	if err := s.ds.db.WithContext(ctx).
		Model(&iapiserver.IdentityGroupMember{}).
		Joins("JOIN identity_group_role_grants ON identity_group_role_grants.group_id = identity_group_members.group_id").
		Joins("JOIN identity_roles ON identity_roles.id = identity_group_role_grants.role_id").
		Where("identity_group_members.user_id = ? AND identity_roles.code IN ? AND identity_roles.status = ?", userID, roleCodes, "ACTIVE").
		Count(&count).Error; err != nil {
		return false, err
	}
	return count != 0, nil
}

func (s *identityStore) EnsureDefaultPermissions(ctx context.Context, permissions []*iapiserver.IdentityPermissionDefinition, rolePermissions map[string][]string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, permission := range permissions {
			var existing iapiserver.IdentityPermissionDefinition
			if err := tx.Where("code = ?", permission.Code).First(&existing).Error; stderrors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(permission).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else if existing.Name != permission.Name || existing.Domain != permission.Domain ||
				existing.Resource != permission.Resource || existing.Action != permission.Action ||
				existing.RiskLevel != permission.RiskLevel || existing.Status != permission.Status {
				updates := map[string]any{
					"name": permission.Name, "domain": permission.Domain, "resource": permission.Resource,
					"action": permission.Action, "risk_level": permission.RiskLevel, "status": permission.Status,
				}
				if err := tx.Model(&existing).Updates(updates).Error; err != nil {
					return err
				}
			}
		}
		var legacyRole iapiserver.IdentityRole
		if err := tx.Where("code = ?", "super_admin").First(&legacyRole).Error; err == nil {
			for _, grant := range []any{
				&iapiserver.IdentityRolePermissionGrant{},
				&iapiserver.IdentityUserRoleGrant{},
				&iapiserver.IdentityGroupRoleGrant{},
				&iapiserver.IdentityServiceAccountRoleGrant{},
			} {
				if err := tx.Where("role_id = ?", legacyRole.ID).Delete(grant).Error; err != nil {
					return err
				}
			}
			if err := tx.Unscoped().Delete(&legacyRole).Error; err != nil {
				return err
			}
		} else if !stderrors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		builtinRoles := []struct{ code, name string }{{"USER", "User"}, {"ADMIN", "Administrator"}, {"SUPER_ADMIN", "Super Administrator"}}
		var role iapiserver.IdentityRole
		for _, builtin := range builtinRoles {
			role = iapiserver.IdentityRole{}
			if err := tx.Where("code = ?", builtin.code).First(&role).Error; stderrors.Is(err, gorm.ErrRecordNotFound) {
				role = iapiserver.IdentityRole{Code: builtin.code, Builtin: true, Status: "ACTIVE"}
				role.Name = builtin.name
				if err := tx.Create(&role).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else if role.Name != builtin.name || !role.Builtin || role.Status != "ACTIVE" {
				if err := tx.Model(&role).Updates(map[string]any{
					"name": builtin.name, "builtin": true, "status": "ACTIVE",
				}).Error; err != nil {
					return err
				}
			}
		}
		grantPermissions := func(roleID string, permissionCodes []string) error {
			for _, permissionCode := range permissionCodes {
				grant := &iapiserver.IdentityRolePermissionGrant{RoleID: roleID, PermissionCode: permissionCode, CreatedAt: imachinery.Now()}
				if err := tx.Where("role_id = ? AND permission_code = ?", roleID, permissionCode).FirstOrCreate(grant).Error; err != nil {
					return err
				}
			}
			return nil
		}

		for roleCode, permissionCodes := range rolePermissions {
			role = iapiserver.IdentityRole{}
			if err := tx.Where("code = ?", roleCode).First(&role).Error; err != nil {
				return err
			}
			if err := tx.Where("role_id = ?", role.ID).Delete(&iapiserver.IdentityRolePermissionGrant{}).Error; err != nil {
				return err
			}
			if err := grantPermissions(role.ID, permissionCodes); err != nil {
				return err
			}
		}
		return nil
	})
}

var _ store.IdentityStore = (*identityStore)(nil)
