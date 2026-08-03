package postgresql

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/platformaudit"
)

const (
	platformSystemAuthConfigID = "default"
	platformAuthConfigAction   = "platform.auth_config.update"
	platformAuthConfigEvent    = "platform.system_auth_config.changed"
	platformAuditEvent         = "platform.audit.recorded"
)

type platformManagementStore struct{ ds *datastore }

func newPlatformManagementStore(ds *datastore) *platformManagementStore {
	return &platformManagementStore{ds: ds}
}

func (s *platformManagementStore) GetSystemAuthConfig(ctx context.Context) (*iapiserver.PlatformSystemAuthConfig, error) {
	var config iapiserver.PlatformSystemAuthConfig
	err := s.ds.db.WithContext(ctx).Where("id = ?", platformSystemAuthConfigID).First(&config).Error
	if !stderrors.Is(err, gorm.ErrRecordNotFound) {
		return &config, err
	}

	config = defaultSystemAuthConfig()
	createErr := s.ds.db.WithContext(ctx).Create(&config).Error
	if createErr == nil {
		return &config, nil
	}
	if lookupErr := s.ds.db.WithContext(ctx).Where("id = ?", platformSystemAuthConfigID).First(&config).Error; lookupErr != nil {
		return nil, createErr
	}
	return &config, nil
}

func defaultSystemAuthConfig() iapiserver.PlatformSystemAuthConfig {
	return iapiserver.PlatformSystemAuthConfig{
		RegistrationMode: "OPEN",
		PasswordPolicy: iapiserver.PlatformPasswordPolicy{
			MinLength:               12,
			MaxLength:               128,
			RequireUppercase:        true,
			RequireLowercase:        true,
			RequireDigit:            true,
			RequireSpecialCharacter: false,
			DisallowUsername:        true,
		},
		LoginFailurePolicy: iapiserver.PlatformLoginFailurePolicy{
			MaxFailedAttempts:      5,
			FailureWindowSeconds:   900,
			LockoutDurationSeconds: 900,
		},
		OnlinePresenceWindowSeconds: 300,
		AccessTokenLifetimeSeconds:  900,
		RefreshTokenLifetimeSeconds: 2592000,
		UpdatedByPrincipalType:      "SERVICE_ACCOUNT",
		UpdatedByPrincipalID:        "system",
	}
}

func (s *platformManagementStore) ReplaceSystemAuthConfig(ctx context.Context, config *iapiserver.PlatformSystemAuthConfig, expectedVersion int64) (*iapiserver.PlatformSystemAuthConfig, error) {
	var updated iapiserver.PlatformSystemAuthConfig
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current iapiserver.PlatformSystemAuthConfig
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", platformSystemAuthConfigID).First(&current).Error; err != nil {
			return err
		}
		if current.ResourceVersion != expectedVersion {
			return errors.NewStatus(code.ErrPlatformAuthConfigVersionConflict, "system auth config version conflict")
		}

		passwordPolicy, err := json.Marshal(config.PasswordPolicy)
		if err != nil {
			return err
		}
		loginFailurePolicy, err := json.Marshal(config.LoginFailurePolicy)
		if err != nil {
			return err
		}
		now := imachinery.Now()
		nextVersion := current.ResourceVersion + 1
		result := tx.Model(&iapiserver.PlatformSystemAuthConfig{}).
			Where("id = ? AND resource_version = ?", platformSystemAuthConfigID, current.ResourceVersion).
			UpdateColumns(map[string]any{
				"registration_mode":              config.RegistrationMode,
				"password_policy_json":           string(passwordPolicy),
				"login_failure_policy_json":      string(loginFailurePolicy),
				"online_presence_window_seconds": config.OnlinePresenceWindowSeconds,
				"access_token_lifetime_seconds":  config.AccessTokenLifetimeSeconds,
				"refresh_token_lifetime_seconds": config.RefreshTokenLifetimeSeconds,
				"updated_by_principal_type":      config.UpdatedByPrincipalType,
				"updated_by_principal_id":        config.UpdatedByPrincipalID,
				"updated_at":                     now,
				"resource_version":               nextVersion,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.NewStatus(code.ErrPlatformAuthConfigVersionConflict, "system auth config version conflict")
		}

		if err := appendPlatformAuthConfigAudit(tx, &current, config, nextVersion, now); err != nil {
			return err
		}
		if err := createPlatformAuthConfigOutbox(tx, config, nextVersion, now); err != nil {
			return err
		}
		return tx.Where("id = ?", platformSystemAuthConfigID).First(&updated).Error
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func appendPlatformAuthConfigAudit(tx *gorm.DB, current, replacement *iapiserver.PlatformSystemAuthConfig, version int64, occurredAt imachinery.Time) error {
	detail, err := json.Marshal(map[string]any{
		"before": map[string]string{"registration_mode": current.RegistrationMode},
		"after":  map[string]string{"registration_mode": replacement.RegistrationMode},
	})
	if err != nil {
		return err
	}
	record := &iapiserver.PlatformAuditLog{
		SourceDomain:   "platform-management",
		SourceModule:   "auth-config",
		PrincipalType:  replacement.UpdatedByPrincipalType,
		PrincipalID:    replacement.UpdatedByPrincipalID,
		ActorUserID:    replacement.ActorUserID,
		Action:         platformAuthConfigAction,
		TargetType:     "system_auth_config",
		TargetID:       platformSystemAuthConfigID,
		Result:         "SUCCESS",
		OccurredAt:     occurredAt,
		Detail:         detail,
		IdempotencyKey: fmt.Sprintf("%s:%d", platformAuthConfigAction, version),
	}
	fingerprint, err := platformaudit.CanonicalJSONDigest(platformAuditFingerprintContent(record))
	if err != nil {
		return err
	}
	record.ContentFingerprint = fingerprint
	return createPlatformAudit(tx, record)
}

func createPlatformAuthConfigOutbox(tx *gorm.DB, config *iapiserver.PlatformSystemAuthConfig, version int64, occurredAt imachinery.Time) error {
	id := uuid.NewString()
	payload, err := json.Marshal(map[string]any{
		"event_id":             id,
		"event_name":           platformAuthConfigEvent,
		"occurred_at":          occurredAt,
		"aggregate_id":         platformSystemAuthConfigID,
		"aggregate_version":    version,
		"actor_principal_type": config.UpdatedByPrincipalType,
		"actor_principal_id":   config.UpdatedByPrincipalID,
		"actor_user_id":        nullablePlatformString(config.ActorUserID),
	})
	if err != nil {
		return err
	}
	idempotencyKey := fmt.Sprintf("%s:%d", platformAuthConfigEvent, version)
	event := &iapiserver.PlatformOutboxEvent{
		EventName:        platformAuthConfigEvent,
		AggregateType:    "system_auth_config",
		AggregateID:      platformSystemAuthConfigID,
		AggregateVersion: version,
		PayloadJSON:      string(payload),
		IdempotencyKey:   idempotencyKey,
	}
	event.ID = id
	event.Name = idempotencyKey
	return tx.Create(event).Error
}

func (s *platformManagementStore) AppendAuditLog(ctx context.Context, record *iapiserver.PlatformAuditLog) (*iapiserver.PlatformAuditLog, error) {
	if record == nil {
		return nil, fmt.Errorf("platform audit record is required")
	}
	if existing, err := findPlatformAuditByIdempotency(s.ds.db.WithContext(ctx), record); err == nil {
		return comparePlatformAuditFingerprint(existing, record.ContentFingerprint)
	} else if !stderrors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return createPlatformAudit(tx, record)
	})
	if err == nil {
		return record, nil
	}
	if !isPlatformAuditUniqueViolation(err) {
		return nil, err
	}
	existing, lookupErr := findPlatformAuditByIdempotency(s.ds.db.WithContext(ctx), record)
	if lookupErr != nil {
		return nil, err
	}
	return comparePlatformAuditFingerprint(existing, record.ContentFingerprint)
}

func createPlatformAudit(tx *gorm.DB, record *iapiserver.PlatformAuditLog) error {
	if err := tx.Create(record).Error; err != nil {
		return err
	}
	return createPlatformAuditOutbox(tx, record)
}

func createPlatformAuditOutbox(tx *gorm.DB, record *iapiserver.PlatformAuditLog) error {
	id := uuid.NewString()
	payload, err := json.Marshal(map[string]any{
		"event_id":          id,
		"event_name":        platformAuditEvent,
		"occurred_at":       record.CreatedAt,
		"aggregate_id":      record.ID,
		"aggregate_version": record.ResourceVersion,
		"audit_log_id":      record.ID,
		"source_domain":     record.SourceDomain,
		"source_module":     record.SourceModule,
		"principal_type":    record.PrincipalType,
		"principal_id":      nullablePlatformString(record.PrincipalID),
		"actor_user_id":     nullablePlatformString(record.ActorUserID),
		"action":            record.Action,
		"target_type":       nullablePlatformString(record.TargetType),
		"target_id":         nullablePlatformString(record.TargetID),
		"result":            record.Result,
		"reason_code":       nullablePlatformString(record.ReasonCode),
		"audit_occurred_at": record.OccurredAt,
		"created_at":        record.CreatedAt,
	})
	if err != nil {
		return err
	}
	idempotencyKey := platformAuditEvent + ":" + record.ID
	event := &iapiserver.PlatformOutboxEvent{
		EventName:        platformAuditEvent,
		AggregateType:    "audit_log",
		AggregateID:      record.ID,
		AggregateVersion: record.ResourceVersion,
		PayloadJSON:      string(payload),
		IdempotencyKey:   idempotencyKey,
	}
	event.ID = id
	event.Name = idempotencyKey
	return tx.Create(event).Error
}

func findPlatformAuditByIdempotency(db *gorm.DB, record *iapiserver.PlatformAuditLog) (*iapiserver.PlatformAuditLog, error) {
	var existing iapiserver.PlatformAuditLog
	err := db.Where(
		"source_domain = ? AND source_module = ? AND idempotency_key = ?",
		record.SourceDomain,
		record.SourceModule,
		record.IdempotencyKey,
	).First(&existing).Error
	return &existing, err
}

func comparePlatformAuditFingerprint(existing *iapiserver.PlatformAuditLog, fingerprint string) (*iapiserver.PlatformAuditLog, error) {
	if existing.ContentFingerprint == fingerprint {
		return existing, nil
	}
	return nil, errors.NewStatus(code.ErrPlatformAuditIdempotencyConflict, "platform audit idempotency key is already used for different content")
}

func isPlatformAuditUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "idx_platform_audit_idempotency") ||
		strings.Contains(message, "uq_platform_audit_idempotency") ||
		strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "unique constraint")
}

func platformAuditFingerprintContent(record *iapiserver.PlatformAuditLog) iapiserver.PlatformAuditRecordRequest {
	return iapiserver.PlatformAuditRecordRequest{
		SourceDomain:   record.SourceDomain,
		SourceModule:   record.SourceModule,
		PrincipalType:  record.PrincipalType,
		PrincipalID:    record.PrincipalID,
		ActorUserID:    record.ActorUserID,
		Action:         record.Action,
		TargetType:     record.TargetType,
		TargetID:       record.TargetID,
		OwnerUserID:    record.OwnerUserID,
		Result:         record.Result,
		ReasonCode:     record.ReasonCode,
		RequestID:      record.RequestID,
		TraceID:        record.TraceID,
		IPAddress:      record.IPAddress,
		UserAgent:      record.UserAgent,
		OccurredAt:     record.OccurredAt,
		Detail:         record.Detail,
		IdempotencyKey: record.IdempotencyKey,
	}
}

func nullablePlatformString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (s *platformManagementStore) GetAuditLog(ctx context.Context, id string) (*iapiserver.PlatformAuditLog, error) {
	var record iapiserver.PlatformAuditLog
	err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&record).Error
	return &record, err
}

func (s *platformManagementStore) ListAuditLogs(ctx context.Context, req *iapiserver.PlatformAuditLogListRequest) ([]*iapiserver.PlatformAuditLog, int64, error) {
	var records []*iapiserver.PlatformAuditLog
	query := s.ds.db.WithContext(ctx).Model(&iapiserver.PlatformAuditLog{})
	exactFilters := map[string]string{
		"source_domain":  req.SourceDomain,
		"source_module":  req.SourceModule,
		"principal_type": req.PrincipalType,
		"principal_id":   req.PrincipalID,
		"action":         req.Action,
		"target_type":    req.TargetType,
		"target_id":      req.TargetID,
		"request_id":     req.RequestID,
		"result":         req.Result,
	}
	for column, value := range exactFilters {
		if value != "" {
			query = query.Where(column+" = ?", value)
		}
	}

	var err error
	query, err = applyPlatformAuditTimeRange(query, "occurred_at", req.OccurredAfter, req.OccurredBefore)
	if err != nil {
		return nil, 0, err
	}
	query, err = applyPlatformAuditTimeRange(query, "created_at", req.CreatedAfter, req.CreatedBefore)
	if err != nil {
		return nil, 0, err
	}

	searchColumns := map[string]string{
		"action":      "action",
		"reason_code": "reason_code",
		"request_id":  "request_id",
		"target_type": "target_type",
		"target_id":   "target_id",
	}
	if req.Keyword != "" {
		conditions := make([]string, 0, len(req.SearchFields))
		args := make([]any, 0, len(req.SearchFields))
		for _, field := range req.SearchFields {
			column, ok := searchColumns[field]
			if !ok {
				continue
			}
			conditions = append(conditions, "LOWER("+column+") LIKE LOWER(?)")
			args = append(args, "%"+req.Keyword+"%")
		}
		if len(conditions) > 0 {
			query = query.Where("("+strings.Join(conditions, " OR ")+")", args...)
		}
	}

	sortColumns := map[string]string{
		"occurred_at":   "occurred_at",
		"created_at":    "created_at",
		"action":        "action",
		"result":        "result",
		"source_domain": "source_domain",
	}
	sortColumn, ok := sortColumns[req.SortField]
	if !ok {
		sortColumn = "occurred_at"
	}
	descending := strings.ToLower(req.SortOrder) != "asc"
	query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: sortColumn}, Desc: descending})
	query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: "id"}, Desc: descending})

	total, err := CountAndFindPage(query, req.PagingParams, &records)
	return records, total, err
}

func applyPlatformAuditTimeRange(query *gorm.DB, column, after, before string) (*gorm.DB, error) {
	if after != "" {
		parsed, err := time.Parse(time.RFC3339, after)
		if err != nil {
			return nil, err
		}
		query = query.Where(column+" >= ?", parsed)
	}
	if before != "" {
		parsed, err := time.Parse(time.RFC3339, before)
		if err != nil {
			return nil, err
		}
		query = query.Where(column+" <= ?", parsed)
	}
	return query, nil
}

var _ store.PlatformManagementStore = (*platformManagementStore)(nil)
