package postgresql

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

func (s *identityStore) ListRoles(ctx context.Context, req *iapiserver.IdentityRoleListRequest) ([]*iapiserver.IdentityRole, int64, error) {
	var items []*iapiserver.IdentityRole
	total, err := CountAndFindPage(req.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.IdentityRole{}), nil), req.PagingParams, &items)
	return items, total, err
}
func (s *identityStore) GetRole(ctx context.Context, id string) (*iapiserver.IdentityRole, error) {
	var item iapiserver.IdentityRole
	err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error
	return &item, err
}
func (s *identityStore) CreateRole(ctx context.Context, role *iapiserver.IdentityRole) (*iapiserver.IdentityRole, error) {
	if len(role.PermissionCodes) > 0 {
		data, _ := json.Marshal(role.PermissionCodes)
		role.PermissionCodesShadow = string(data)
	}
	err := s.ds.db.WithContext(ctx).Create(role).Error
	return role, err
}
func (s *identityStore) UpdateRole(ctx context.Context, role *iapiserver.IdentityRole) (*iapiserver.IdentityRole, error) {
	var current iapiserver.IdentityRole
	err := s.ds.db.WithContext(ctx).Where("id = ?", role.ID).First(&current).Error
	if err != nil {
		return nil, err
	}
	if role.Name != "" {
		current.Name = role.Name
	}
	current.Description = role.Description
	if role.Status != "" {
		current.Status = role.Status
	}
	if err := s.ds.db.WithContext(ctx).Save(&current).Error; err != nil {
		return nil, err
	}
	return &current, nil
}
func (s *identityStore) ReplaceRolePermissions(ctx context.Context, id string, permissionCodes []string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", id).Delete(&iapiserver.IdentityRolePermissionGrant{}).Error; err != nil {
			return err
		}
		for _, permission := range permissionCodes {
			if err := tx.Create(&iapiserver.IdentityRolePermissionGrant{RoleID: id, PermissionCode: permission, CreatedAt: imachinery.Now()}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *identityStore) ListGroups(ctx context.Context, req *iapiserver.IdentityGroupListRequest) ([]*iapiserver.IdentityGroup, int64, error) {
	var items []*iapiserver.IdentityGroup
	total, err := CountAndFindPage(req.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.IdentityGroup{}), nil), req.PagingParams, &items)
	return items, total, err
}
func (s *identityStore) GetGroup(ctx context.Context, id string) (*iapiserver.IdentityGroup, error) {
	var item iapiserver.IdentityGroup
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return &item, err
	}
	_ = s.ds.db.WithContext(ctx).Table("identity_group_members").Where("group_id = ?", id).Pluck("user_id", &item.MemberUserIDs).Error
	_ = s.ds.db.WithContext(ctx).Table("identity_group_role_grants").Where("group_id = ?", id).Pluck("role_id", &item.RoleIDs).Error
	return &item, nil
}
func (s *identityStore) CreateGroup(ctx context.Context, group *iapiserver.IdentityGroup) (*iapiserver.IdentityGroup, error) {
	err := s.ds.db.WithContext(ctx).Create(group).Error
	return group, err
}
func (s *identityStore) UpdateGroup(ctx context.Context, group *iapiserver.IdentityGroup) (*iapiserver.IdentityGroup, error) {
	var current iapiserver.IdentityGroup
	if err := s.ds.db.WithContext(ctx).Where("id = ?", group.ID).First(&current).Error; err != nil {
		return nil, err
	}
	current.Name, current.Description, current.Status = group.Name, group.Description, group.Status
	if err := s.ds.db.WithContext(ctx).Save(&current).Error; err != nil {
		return nil, err
	}
	return s.GetGroup(ctx, group.ID)
}
func (s *identityStore) ReplaceGroupMembers(ctx context.Context, id string, userIDs []string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", id).Delete(&iapiserver.IdentityGroupMember{}).Error; err != nil {
			return err
		}
		for _, userID := range userIDs {
			if err := tx.Create(&iapiserver.IdentityGroupMember{GroupID: id, UserID: userID, CreatedAt: imachinery.Now()}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *identityStore) ReplaceGroupRoles(ctx context.Context, id string, roleIDs []string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", id).Delete(&iapiserver.IdentityGroupRoleGrant{}).Error; err != nil {
			return err
		}
		for _, roleID := range roleIDs {
			if err := tx.Create(&iapiserver.IdentityGroupRoleGrant{GroupID: id, RoleID: roleID, CreatedAt: imachinery.Now()}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *identityStore) ListResourceGrants(ctx context.Context, resourceType, resourceID string, req *iapiserver.IdentityResourceGrantListRequest) ([]*iapiserver.IdentityResourceAccessGrant, int64, error) {
	var items []*iapiserver.IdentityResourceAccessGrant
	total, err := CountAndFindPage(req.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.IdentityResourceAccessGrant{}), func(q *gorm.DB) *gorm.DB {
		return q.Where("resource_type = ? AND resource_id = ? AND revoked_at IS NULL", resourceType, resourceID)
	}), req.PagingParams, &items)
	return items, total, err
}
func (s *identityStore) CreateResourceGrant(ctx context.Context, grant *iapiserver.IdentityResourceAccessGrant) (*iapiserver.IdentityResourceAccessGrant, error) {
	err := s.ds.db.WithContext(ctx).Create(grant).Error
	return grant, err
}
func (s *identityStore) UpdateResourceGrant(ctx context.Context, grant *iapiserver.IdentityResourceAccessGrant) (*iapiserver.IdentityResourceAccessGrant, error) {
	var current iapiserver.IdentityResourceAccessGrant
	if err := s.ds.db.WithContext(ctx).Where("id = ?", grant.ID).First(&current).Error; err != nil {
		return nil, err
	}
	if grant.AccessLevel != "" {
		current.AccessLevel = grant.AccessLevel
	}
	current.ExpiresAt = grant.ExpiresAt
	if err := s.ds.db.WithContext(ctx).Save(&current).Error; err != nil {
		return nil, err
	}
	return &current, nil
}
func (s *identityStore) RevokeResourceGrant(ctx context.Context, id string) error {
	now := imachinery.Now()
	return s.ds.db.WithContext(ctx).Model(&iapiserver.IdentityResourceAccessGrant{}).Where("id = ?", id).Updates(map[string]any{"revoked_at": now, "updated_at": now}).Error
}

func (s *identityStore) ListServiceAccounts(ctx context.Context, req *iapiserver.IdentityServiceAccountListRequest) ([]*iapiserver.IdentityServiceAccount, int64, error) {
	var items []*iapiserver.IdentityServiceAccount
	total, err := CountAndFindPage(req.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.IdentityServiceAccount{}), nil), req.PagingParams, &items)
	return items, total, err
}
func (s *identityStore) GetServiceAccount(ctx context.Context, id string) (*iapiserver.IdentityServiceAccount, error) {
	var item iapiserver.IdentityServiceAccount
	err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&item).Error
	return &item, err
}
func (s *identityStore) CreateServiceAccount(ctx context.Context, account *iapiserver.IdentityServiceAccount, _ []string) (*iapiserver.IdentityServiceAccount, error) {
	err := s.ds.db.WithContext(ctx).Create(account).Error
	return account, err
}
func (s *identityStore) UpdateServiceAccount(ctx context.Context, account *iapiserver.IdentityServiceAccount, _ []string) (*iapiserver.IdentityServiceAccount, error) {
	var current iapiserver.IdentityServiceAccount
	if err := s.ds.db.WithContext(ctx).Where("id = ?", account.ID).First(&current).Error; err != nil {
		return nil, err
	}
	current.Name, current.Description = account.Name, account.Description
	if err := s.ds.db.WithContext(ctx).Save(&current).Error; err != nil {
		return nil, err
	}
	return &current, nil
}
func (s *identityStore) SetServiceAccountStatus(ctx context.Context, id, status string) (*iapiserver.IdentityServiceAccount, error) {
	var current iapiserver.IdentityServiceAccount
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&current).Error; err != nil {
		return nil, err
	}
	current.Status = status
	current.SecurityVersion++
	if err := s.ds.db.WithContext(ctx).Save(&current).Error; err != nil {
		return nil, err
	}
	return &current, nil
}
func (s *identityStore) RotateServiceAccountCredential(ctx context.Context, id string) (*iapiserver.IdentityServiceAccountCredentialResponse, error) {
	account, err := s.GetServiceAccount(ctx, id)
	if err != nil {
		return nil, err
	}
	raw := uuid.NewString() + uuid.NewString()
	sum := sha256.Sum256([]byte(raw))
	credential := &iapiserver.IdentityServiceAccountCredential{ObjectMeta: imachinery.ObjectMeta{Name: "credential"}, ServiceAccountID: id, CredentialHash: hex.EncodeToString(sum[:]), Prefix: raw[:8], Status: "ACTIVE", IssuedAt: imachinery.Now()}
	if err := s.ds.db.WithContext(ctx).Model(&iapiserver.IdentityServiceAccountCredential{}).Where("service_account_id = ? AND status = ?", id, "ACTIVE").Updates(map[string]any{"status": "REVOKED", "revoked_at": imachinery.Now()}).Error; err != nil {
		return nil, err
	}
	if err := s.ds.db.WithContext(ctx).Create(credential).Error; err != nil {
		return nil, err
	}
	return &iapiserver.IdentityServiceAccountCredentialResponse{ServiceAccount: account, Credential: raw}, nil
}

var _ store.IdentityAdminStore = (*identityStore)(nil)
