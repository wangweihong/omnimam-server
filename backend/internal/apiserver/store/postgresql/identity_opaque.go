package postgresql

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

func (s *identityStore) CreateOpaqueExchange(ctx context.Context, exchange *iapiserver.IdentityOpaqueExchange) error {
	return s.ds.db.WithContext(ctx).Create(exchange).Error
}

func (s *identityStore) GetOpaqueExchange(ctx context.Context, id string) (*iapiserver.IdentityOpaqueExchange, error) {
	var exchange iapiserver.IdentityOpaqueExchange
	err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&exchange).Error
	return &exchange, err
}

// ConsumeOpaqueExchange atomically claims an exchange. The row lock makes parallel finish requests mutually exclusive.
func (s *identityStore) ConsumeOpaqueExchange(ctx context.Context, id string) (*iapiserver.IdentityOpaqueExchange, error) {
	var exchange iapiserver.IdentityOpaqueExchange
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&exchange).Error; err != nil {
			return err
		}
		if exchange.ConsumedAt != nil {
			return errors.New("opaque exchange already consumed")
		}
		if !exchange.ExpiresAt.Time.After(time.Now()) {
			return errors.New("opaque exchange expired")
		}
		now := imachinery.Now()
		if err := tx.Model(&exchange).Updates(map[string]any{"consumed_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		exchange.ConsumedAt = &now
		return nil
	})
	return &exchange, err
}

// FinalizeOpaqueRegistration stores a verified record and, for OPEN registration, grants the default USER role atomically.
func (s *identityStore) FinalizeOpaqueRegistration(ctx context.Context, user *iapiserver.IdentityUser, active bool) (*iapiserver.IdentityUser, error) {
	var current iapiserver.IdentityUser
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", user.ID).First(&current).Error; err != nil {
			return err
		}
		updates := map[string]any{
			"opaque_registration_record": user.OpaqueRegistrationRecord,
			"password_changed_at":        user.PasswordChangedAt,
			"first_login_required":       user.FirstLoginRequired,
			"status":                     user.Status,
			"security_version":           user.SecurityVersion,
			"updated_at":                 imachinery.Now(),
		}
		if active {
			updates["status"] = iapiserver.IdentityUserActive
		}
		if err := tx.Model(&current).Updates(updates).Error; err != nil {
			return err
		}
		if active {
			var role iapiserver.IdentityRole
			if err := tx.Where("code = ? AND builtin = ? AND status = ?", "USER", true, "ACTIVE").First(&role).Error; err != nil {
				return err
			}
			var count int64
			if err := tx.Model(&iapiserver.IdentityUserRoleGrant{}).Where("user_id = ? AND role_id = ?", current.ID, role.ID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				if err := tx.Create(&iapiserver.IdentityUserRoleGrant{ID: uuid.NewString(), UserID: current.ID, RoleID: role.ID, CreatedAt: imachinery.Now()}).Error; err != nil {
					return err
				}
			}
		}
		return tx.Where("id = ?", current.ID).First(&current).Error
	})
	return &current, err
}
