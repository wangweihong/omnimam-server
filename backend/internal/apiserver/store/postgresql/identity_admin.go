package postgresql

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	"github.com/wangweihong/omnimam/backend/internal/pkg/platformaudit"
)

const identityRegistrationReviewedEvent = "identity.registration.reviewed"

func (s *identityStore) ListRegistrationApplications(ctx context.Context, req *iapiserver.IdentityRegistrationApplicationListRequest) ([]*store.IdentityRegistrationApplicationView, int64, error) {
	query := req.BasicQueryParam.ToUnpaginatedQuery(ctx, s.ds.db.Model(&iapiserver.IdentityRegistrationApplication{}), nil)
	if req.Statuses != "" {
		statuses := strings.Split(req.Statuses, ",")
		for _, status := range statuses {
			if status != iapiserver.IdentityRegistrationPending && status != iapiserver.IdentityRegistrationApproved && status != iapiserver.IdentityRegistrationRejected {
				return nil, 0, errors.NewStatus(code.ErrIdentityRegistrationStateInvalid, "registration application status is invalid")
			}
		}
		query = query.Where("identity_registration_applications.status IN ?", statuses)
	}
	if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
		pattern := "%" + keyword + "%"
		query = query.Joins("JOIN identity_users ON identity_users.id = identity_registration_applications.user_id").Where(
			"identity_users.username ILIKE ? OR identity_users.email ILIKE ? OR identity_users.display_name ILIKE ?", pattern, pattern, pattern,
		)
	}
	query = query.Order("identity_registration_applications.submitted_at DESC, identity_registration_applications.id DESC")
	var applications []*iapiserver.IdentityRegistrationApplication
	total, err := CountAndFindPage(query, req.PagingParams, &applications)
	if err != nil {
		return nil, 0, err
	}
	views, err := s.registrationViews(ctx, applications)
	return views, total, err
}

func (s *identityStore) GetRegistrationApplication(ctx context.Context, id string) (*store.IdentityRegistrationApplicationView, error) {
	var application iapiserver.IdentityRegistrationApplication
	if err := s.ds.db.WithContext(ctx).Where("id = ?", id).First(&application).Error; err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.NewStatus(code.ErrIdentityRegistrationApplicationNotVisible, "registration application is not visible")
		}
		return nil, err
	}
	views, err := s.registrationViews(ctx, []*iapiserver.IdentityRegistrationApplication{&application})
	if err != nil {
		return nil, err
	}
	if len(views) != 1 {
		return nil, errors.NewStatus(code.ErrIdentityRegistrationApplicationNotVisible, "registration application is not visible")
	}
	return views[0], nil
}

func (s *identityStore) registrationViews(ctx context.Context, applications []*iapiserver.IdentityRegistrationApplication) ([]*store.IdentityRegistrationApplicationView, error) {
	if len(applications) == 0 {
		return []*store.IdentityRegistrationApplicationView{}, nil
	}
	userIDs := make([]string, 0, len(applications))
	for _, application := range applications {
		userIDs = append(userIDs, application.UserID)
	}
	var users []*iapiserver.IdentityUser
	if err := s.ds.db.WithContext(ctx).Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	byID := make(map[string]*iapiserver.IdentityUser, len(users))
	for _, user := range users {
		user.PasswordHash = ""
		user.NormalizedEmail = nil
		byID[user.ID] = user
	}
	views := make([]*store.IdentityRegistrationApplicationView, 0, len(applications))
	for _, application := range applications {
		user := byID[application.UserID]
		if user == nil {
			return nil, errors.NewStatus(code.ErrIdentityRegistrationApplicationNotVisible, "registration application is not visible")
		}
		views = append(views, &store.IdentityRegistrationApplicationView{Application: application, User: user})
	}
	return views, nil
}

func (s *identityStore) CreatePendingRegistration(ctx context.Context, user *iapiserver.IdentityUser) (*store.IdentityRegistrationApplicationView, error) {
	var application iapiserver.IdentityRegistrationApplication
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing iapiserver.IdentityUser
		lookup := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("normalized_username = ? OR normalized_email = ?", user.NormalizedUsername, valueOrEmptyIdentity(user.NormalizedEmail)).First(&existing)
		if lookup.Error == nil {
			if existing.NormalizedUsername != user.NormalizedUsername || valueOrEmptyIdentity(existing.NormalizedEmail) != valueOrEmptyIdentity(user.NormalizedEmail) {
				return errors.NewStatus(code.ErrIdentityUsernameAlreadyExists, "username or email already exists")
			}
			if existing.Status != iapiserver.IdentityUserRejected {
				return errors.NewStatus(code.ErrIdentityUsernameAlreadyExists, "username or email already exists")
			}
			var attempt int
			if err := tx.Model(&iapiserver.IdentityRegistrationApplication{}).Where("user_id = ?", existing.ID).Select("COALESCE(MAX(attempt_no), 0)").Scan(&attempt).Error; err != nil {
				return err
			}
			now := imachinery.Now()
			if err := tx.Model(&iapiserver.IdentityUser{}).Where("id = ?", existing.ID).Updates(map[string]any{
				"password_hash": user.PasswordHash, "password_changed_at": user.PasswordChangedAt,
				"display_name": user.DisplayName, "email": user.Email, "normalized_email": user.NormalizedEmail,
				"status": iapiserver.IdentityUserPending, "updated_at": now, "resource_version": existing.ResourceVersion + 1,
			}).Error; err != nil {
				return err
			}
			user.ID = existing.ID
			user.ResourceVersion = existing.ResourceVersion + 1
			application = iapiserver.IdentityRegistrationApplication{ObjectMeta: imachinery.ObjectMeta{Name: "registration:" + existing.ID}, UserID: existing.ID, AttemptNo: attempt + 1, Status: iapiserver.IdentityRegistrationPending, SubmittedAt: now}
			if err := tx.Create(&application).Error; err != nil {
				return err
			}
			if err := createIdentityOutbox(tx, "identity.user.status_changed", "user", existing.ID, user.ResourceVersion, map[string]any{
				"user_id": existing.ID, "previous_status": iapiserver.IdentityUserRejected, "status": iapiserver.IdentityUserPending,
				"security_version": existing.SecurityVersion, "actor_principal_type": "USER", "actor_principal_id": existing.ID,
			}, fmt.Sprintf("identity.user.status_changed:%s:%d", existing.ID, user.ResourceVersion)); err != nil {
				return err
			}
			return appendIdentityRegistrationAuditSubmission(tx, &application, "resubmitted", application.SubmittedAt)
		}
		if !stderrors.Is(lookup.Error, gorm.ErrRecordNotFound) {
			return lookup.Error
		}
		user.Status = iapiserver.IdentityUserPending
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		application = iapiserver.IdentityRegistrationApplication{ObjectMeta: imachinery.ObjectMeta{Name: "registration:" + user.ID}, UserID: user.ID, AttemptNo: 1, Status: iapiserver.IdentityRegistrationPending, SubmittedAt: imachinery.Now()}
		if err := tx.Create(&application).Error; err != nil {
			return err
		}
		if err := createIdentityOutbox(tx, "identity.user.created", "user", user.ID, user.ResourceVersion, map[string]any{
			"user_id": user.ID, "status": iapiserver.IdentityUserPending, "source": "LOCAL",
			"actor_principal_type": "USER", "actor_principal_id": user.ID,
		}, fmt.Sprintf("identity.user.created:%s:%d", user.ID, user.ResourceVersion)); err != nil {
			return err
		}
		return appendIdentityRegistrationAuditSubmission(tx, &application, "submitted", application.SubmittedAt)
	})
	if err != nil {
		return nil, err
	}
	user.Status = iapiserver.IdentityUserPending
	user.PasswordHash = ""
	user.NormalizedEmail = nil
	return &store.IdentityRegistrationApplicationView{Application: &application, User: user}, nil
}

func (s *identityStore) ReviewRegistrationApplication(ctx context.Context, id, decision, reason, actorPrincipalType, actorPrincipalID, actorUserID string) (*store.IdentityRegistrationApplicationView, error) {
	decision = strings.ToUpper(strings.TrimSpace(decision))
	if decision != iapiserver.IdentityRegistrationApproved && decision != iapiserver.IdentityRegistrationRejected {
		return nil, errors.NewStatus(code.ErrIdentityRegistrationStateInvalid, "registration decision is invalid")
	}
	if decision == iapiserver.IdentityRegistrationRejected && strings.TrimSpace(reason) == "" {
		return nil, errors.NewStatus(code.ErrIdentityRegistrationReasonRequired, "rejection reason is required")
	}
	var application iapiserver.IdentityRegistrationApplication
	var user iapiserver.IdentityUser
	var updatedAuthorizationVersion int64
	var updatedUserVersion int64
	var updatedApplicationVersion int64
	var updatedStatus string
	var decisionAt imachinery.Time
	decisionChanged := false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&application).Error; err != nil {
			if stderrors.Is(err, gorm.ErrRecordNotFound) {
				return errors.NewStatus(code.ErrIdentityRegistrationApplicationNotVisible, "registration application is not visible")
			}
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", application.UserID).First(&user).Error; err != nil {
			return err
		}
		if application.Status != iapiserver.IdentityRegistrationPending {
			if application.Status == decision {
				return nil
			}
			return errors.NewStatus(code.ErrIdentityRegistrationDecisionConflict, "registration application has an opposite decision")
		}
		if user.Status != iapiserver.IdentityUserPending {
			return errors.NewStatus(code.ErrIdentityUserStateInvalid, "registration user state is invalid")
		}
		now := imachinery.Now()
		decisionAt = now
		decisionChanged = true
		appVersion := application.ResourceVersion + 1
		userVersion := user.ResourceVersion + 1
		updatedApplicationVersion, updatedUserVersion = appVersion, userVersion
		if err := tx.Model(&iapiserver.IdentityRegistrationApplication{}).Where("id = ? AND status = ?", id, iapiserver.IdentityRegistrationPending).Updates(map[string]any{
			"status": decision, "decided_at": now, "decided_by": actorPrincipalID,
			"decision_reason": nullableIdentityReason(decision, reason), "updated_at": now, "resource_version": appVersion,
		}).Error; err != nil {
			return err
		}
		authorizationVersion := user.AuthorizationVersion
		if decision == iapiserver.IdentityRegistrationApproved {
			authorizationVersion++
		}
		updatedAuthorizationVersion = authorizationVersion
		updatedStatus = map[string]string{iapiserver.IdentityRegistrationApproved: iapiserver.IdentityUserActive, iapiserver.IdentityRegistrationRejected: iapiserver.IdentityUserRejected}[decision]
		if err := tx.Model(&iapiserver.IdentityUser{}).Where("id = ? AND status = ?", user.ID, iapiserver.IdentityUserPending).Updates(map[string]any{
			"status":                updatedStatus,
			"authorization_version": authorizationVersion, "updated_at": now, "resource_version": userVersion,
		}).Error; err != nil {
			return err
		}
		if decision == iapiserver.IdentityRegistrationApproved {
			var role iapiserver.IdentityRole
			if err := tx.Where("code = ? AND builtin = ? AND status = ?", "USER", true, "ACTIVE").First(&role).Error; err != nil {
				return errors.NewStatus(code.ErrIdentityUserCreateInvalid, "active builtin USER role is unavailable")
			}
			if err := tx.Create(&iapiserver.IdentityUserRoleGrant{ID: uuid.NewString(), UserID: user.ID, RoleID: role.ID, CreatedBy: actorPrincipalID, CreatedAt: now}).Error; err != nil {
				return err
			}
			if err := createIdentityOutbox(tx, "identity.authorization.changed", "user", user.ID, authorizationVersion, map[string]any{
				"principal_type": "USER", "principal_id": user.ID, "authorization_version": authorizationVersion,
				"change_source_type": "USER_ROLE", "change_source_id": role.ID, "changed_permission_codes": []string{},
				"actor_principal_type": actorPrincipalType, "actor_principal_id": actorPrincipalID, "actor_user_id": nullableIdentityString(actorUserID),
			}, fmt.Sprintf("identity.authorization.changed:USER:%s:%d", user.ID, authorizationVersion)); err != nil {
				return err
			}
		}
		if err := createIdentityOutbox(tx, "identity.user.status_changed", "user", user.ID, userVersion, map[string]any{
			"user_id": user.ID, "previous_status": iapiserver.IdentityUserPending, "status": updatedStatus,
			"security_version": user.SecurityVersion, "actor_principal_type": actorPrincipalType, "actor_principal_id": actorPrincipalID, "actor_user_id": nullableIdentityString(actorUserID),
		}, fmt.Sprintf("identity.user.status_changed:%s:%d", user.ID, userVersion)); err != nil {
			return err
		}
		if err := createIdentityOutbox(tx, identityRegistrationReviewedEvent, "registration_application", application.ID, appVersion, map[string]any{
			"registration_application_id": application.ID, "user_id": user.ID, "attempt_no": application.AttemptNo,
			"decision": decision, "decision_reason": nullableIdentityReason(decision, reason), "authorization_version": authorizationVersion,
			"actor_principal_type": actorPrincipalType, "actor_principal_id": actorPrincipalID, "actor_user_id": nullableIdentityString(actorUserID),
		}, fmt.Sprintf("identity.registration.reviewed:%s:%d", application.ID, appVersion)); err != nil {
			return err
		}
		return appendIdentityRegistrationAudit(tx, &application, &user, decision, reason, actorPrincipalType, actorPrincipalID, actorUserID, now, appVersion)
	})
	if err != nil {
		return nil, err
	}
	if decisionChanged {
		application.Status = decision
		application.DecidedAt = &decisionAt
		application.DecidedBy = &actorPrincipalID
		if decision == iapiserver.IdentityRegistrationRejected {
			reason = strings.TrimSpace(reason)
			application.DecisionReason = &reason
		}
		application.ResourceVersion = updatedApplicationVersion
		user.Status = updatedStatus
		user.AuthorizationVersion = updatedAuthorizationVersion
		user.ResourceVersion = updatedUserVersion
	}
	user.PasswordHash = ""
	user.NormalizedEmail = nil
	return &store.IdentityRegistrationApplicationView{Application: &application, User: &user}, nil
}

func createIdentityOutbox(tx *gorm.DB, eventName, aggregateType, aggregateID string, aggregateVersion int64, fields map[string]any, idempotencyKey string) error {
	eventID := uuid.NewString()
	payload := map[string]any{"event_id": eventID, "event_name": eventName, "occurred_at": imachinery.Now(), "aggregate_id": aggregateID, "aggregate_version": aggregateVersion}
	for key, value := range fields {
		payload[key] = value
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	event := &iapiserver.IdentityOutboxEvent{ObjectMeta: imachinery.ObjectMeta{ID: eventID, Name: idempotencyKey}, EventName: eventName, AggregateType: aggregateType, AggregateID: aggregateID, AggregateVersion: aggregateVersion, PayloadJSON: string(raw), IdempotencyKey: idempotencyKey, Status: "PENDING"}
	return tx.Create(event).Error
}

func appendIdentityRegistrationAuditSubmission(tx *gorm.DB, application *iapiserver.IdentityRegistrationApplication, source string, occurredAt imachinery.Time) error {
	detail, err := json.Marshal(map[string]any{"attempt_no": application.AttemptNo, "source": source})
	if err != nil {
		return err
	}
	record := &iapiserver.PlatformAuditLog{
		SourceDomain: "identity", SourceModule: "user", PrincipalType: "ANONYMOUS", Action: "identity.registration.submit",
		TargetType: "registration_application", TargetID: application.ID, Result: "SUCCESS", OccurredAt: occurredAt,
		Detail: detail, IdempotencyKey: fmt.Sprintf("identity.registration.submitted:%s:%d", application.ID, application.ResourceVersion),
	}
	fingerprint, err := platformaudit.CanonicalJSONDigest(platformAuditFingerprintContent(record))
	if err != nil {
		return err
	}
	record.ContentFingerprint = fingerprint
	if err := createPlatformAudit(tx, record); err != nil {
		return errors.NewStatus(code.ErrPlatformAuditWriteUnavailable, "platform audit boundary is unavailable")
	}
	return nil
}

func appendIdentityOpenRegistrationAudit(tx *gorm.DB, user *iapiserver.IdentityUser, occurredAt imachinery.Time) error {
	detail, err := json.Marshal(map[string]any{"status": iapiserver.IdentityUserActive})
	if err != nil {
		return err
	}
	record := &iapiserver.PlatformAuditLog{
		SourceDomain: "identity", SourceModule: "user", PrincipalType: "ANONYMOUS", Action: "identity.registration.open",
		TargetType: "user", TargetID: user.ID, Result: "SUCCESS", OccurredAt: occurredAt, Detail: detail,
		IdempotencyKey: fmt.Sprintf("identity.registration.open:%s:%d", user.ID, user.ResourceVersion),
	}
	fingerprint, err := platformaudit.CanonicalJSONDigest(platformAuditFingerprintContent(record))
	if err != nil {
		return err
	}
	record.ContentFingerprint = fingerprint
	if err := createPlatformAudit(tx, record); err != nil {
		return errors.NewStatus(code.ErrPlatformAuditWriteUnavailable, "platform audit boundary is unavailable")
	}
	return nil
}

func appendIdentityRegistrationAudit(tx *gorm.DB, application *iapiserver.IdentityRegistrationApplication, user *iapiserver.IdentityUser, decision, reason, actorPrincipalType, actorPrincipalID, actorUserID string, occurredAt imachinery.Time, version int64) error {
	detail, err := json.Marshal(map[string]any{"decision": decision, "attempt_no": application.AttemptNo})
	if err != nil {
		return err
	}
	record := &iapiserver.PlatformAuditLog{SourceDomain: "identity", SourceModule: "user", PrincipalType: actorPrincipalType, PrincipalID: actorPrincipalID, ActorUserID: actorUserID, Action: "identity.registration." + strings.ToLower(decision), TargetType: "registration_application", TargetID: application.ID, Result: "SUCCESS", ReasonCode: "REGISTRATION_REVIEW", OccurredAt: occurredAt, Detail: detail, IdempotencyKey: fmt.Sprintf("identity.registration.reviewed:%s:%d", application.ID, version)}
	fingerprint, err := platformaudit.CanonicalJSONDigest(platformAuditFingerprintContent(record))
	if err != nil {
		return err
	}
	record.ContentFingerprint = fingerprint
	if err := createPlatformAudit(tx, record); err != nil {
		return errors.NewStatus(code.ErrPlatformAuditWriteUnavailable, "platform audit boundary is unavailable")
	}
	return nil
}

func nullableIdentityReason(decision, reason string) any {
	if decision == iapiserver.IdentityRegistrationRejected {
		return strings.TrimSpace(reason)
	}
	return nil
}

func nullableIdentityString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func valueOrEmptyIdentity(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

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
	return &iapiserver.IdentityServiceAccountCredentialResponse{ServiceAccount: account, Credential: credential, ClientSecret: raw}, nil
}

var _ store.IdentityAdminStore = (*identityStore)(nil)
