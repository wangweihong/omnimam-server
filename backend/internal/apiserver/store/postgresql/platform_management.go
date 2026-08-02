package postgresql

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type platformManagementStore struct{ ds *datastore }

func newPlatformManagementStore(ds *datastore) *platformManagementStore {
	return &platformManagementStore{ds: ds}
}

func (s *platformManagementStore) GetSystemAuthConfig(ctx context.Context) (*iapiserver.PlatformSystemAuthConfig, error) {
	var config iapiserver.PlatformSystemAuthConfig
	err := s.ds.db.WithContext(ctx).Where("name = ?", "default").First(&config).Error
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		config = defaultSystemAuthConfig()
		if createErr := s.ds.db.WithContext(ctx).Create(&config).Error; createErr != nil {
			return nil, createErr
		}
		return &config, nil
	}
	return &config, err
}

func defaultSystemAuthConfig() iapiserver.PlatformSystemAuthConfig {
	return iapiserver.PlatformSystemAuthConfig{
		ObjectMeta: imachinery.ObjectMeta{Name: "default"}, RegistrationMode: "OPEN",
		PasswordPolicy:              []byte(`{"min_length":8,"require_upper":false,"require_lower":true,"require_digit":true,"require_symbol":false}`),
		LoginFailurePolicy:          []byte(`{"max_attempts":5,"lock_seconds":900}`),
		OnlinePresenceWindowSeconds: 300, AccessTokenLifetimeSeconds: 900, RefreshTokenLifetimeSeconds: 2592000,
		UpdatedByPrincipalType: "SERVICE_ACCOUNT", UpdatedByPrincipalID: "system",
	}
}

func (s *platformManagementStore) ReplaceSystemAuthConfig(ctx context.Context, config *iapiserver.PlatformSystemAuthConfig, expectedVersion int64) (*iapiserver.PlatformSystemAuthConfig, error) {
	var current iapiserver.PlatformSystemAuthConfig
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("name = ?", "default").First(&current).Error; err != nil {
			return err
		}
		if current.ResourceVersion != expectedVersion {
			return fmt.Errorf("platform auth config version conflict")
		}
		config.ID = current.ID
		config.Name = current.Name
		config.ResourceVersion = current.ResourceVersion
		if err := tx.Model(&current).Updates(map[string]any{
			"registration_mode":              config.RegistrationMode,
			"password_policy_json":           string(config.PasswordPolicy),
			"login_failure_policy_json":      string(config.LoginFailurePolicy),
			"online_presence_window_seconds": config.OnlinePresenceWindowSeconds,
			"access_token_lifetime_seconds":  config.AccessTokenLifetimeSeconds,
			"refresh_token_lifetime_seconds": config.RefreshTokenLifetimeSeconds,
			"updated_by_principal_type":      config.UpdatedByPrincipalType,
			"updated_by_principal_id":        config.UpdatedByPrincipalID,
		}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", current.ID).First(config).Error
	})
	return config, err
}

func (s *platformManagementStore) AppendAuditLog(ctx context.Context, record *iapiserver.PlatformAuditLog) (*iapiserver.PlatformAuditLog, error) {
	if err := validateAuditRecord(record); err != nil {
		return nil, err
	}
	var existing iapiserver.PlatformAuditLog
	if err := s.ds.db.WithContext(ctx).Where("idempotency_key = ?", record.IdempotencyKey).First(&existing).Error; err == nil {
		return &existing, nil
	} else if !stderrors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if record.ID == "" {
		record.ID = uuid.NewString()
	}
	if err := s.ds.db.WithContext(ctx).Create(record).Error; err != nil {
		var duplicate iapiserver.PlatformAuditLog
		if lookupErr := s.ds.db.WithContext(ctx).Where("idempotency_key = ?", record.IdempotencyKey).First(&duplicate).Error; lookupErr == nil {
			return &duplicate, nil
		}
		return nil, err
	}
	return record, nil
}

func (s *platformManagementStore) GetAuditLog(ctx context.Context, id string) (*iapiserver.PlatformAuditLog, error) {
	var record iapiserver.PlatformAuditLog
	err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&record).Error
	return &record, err
}

func (s *platformManagementStore) ListAuditLogs(ctx context.Context, req *iapiserver.PlatformAuditLogListRequest) ([]*iapiserver.PlatformAuditLog, int64, error) {
	var records []*iapiserver.PlatformAuditLog
	// CountAndFindPage runs Count before Find; the model must be explicit because
	// ToUnpaginatedQuery only adds filters and ordering to the supplied DB.
	query := req.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.PlatformAuditLog{}), func(q *gorm.DB) *gorm.DB {
		if req.SourceDomain != "" {
			q = q.Where("source_domain = ?", req.SourceDomain)
		}
		if req.SourceModule != "" {
			q = q.Where("source_module = ?", req.SourceModule)
		}
		if req.Principal != "" {
			q = q.Where("principal_id = ?", req.Principal)
		}
		if req.Action != "" {
			q = q.Where("action = ?", req.Action)
		}
		if req.Result != "" {
			q = q.Where("result = ?", req.Result)
		}
		return q
	})
	total, err := CountAndFindPage(query, req.PagingParams, &records)
	return records, total, err
}

func validateAuditRecord(record *iapiserver.PlatformAuditLog) error {
	if record == nil || strings.TrimSpace(record.SourceDomain) == "" || strings.TrimSpace(record.SourceModule) == "" || strings.TrimSpace(record.Action) == "" || strings.TrimSpace(record.IdempotencyKey) == "" {
		return fmt.Errorf("audit record required fields are missing")
	}
	if record.PrincipalType != "USER" && record.PrincipalType != "SERVICE_ACCOUNT" && record.PrincipalType != "ANONYMOUS" {
		return fmt.Errorf("audit principal type is invalid")
	}
	if record.Result != "SUCCESS" && record.Result != "FAILED" && record.Result != "DENIED" {
		return fmt.Errorf("audit result is invalid")
	}
	if len(record.Detail) > 16*1024 || strings.Contains(strings.ToLower(string(record.Detail)), "authorization") || strings.Contains(strings.ToLower(string(record.Detail)), "password") || strings.Contains(strings.ToLower(string(record.Detail)), "secret") || strings.Contains(strings.ToLower(string(record.Detail)), "token") {
		return fmt.Errorf("audit detail contains prohibited data")
	}
	return nil
}

var _ store.PlatformManagementStore = (*platformManagementStore)(nil)
