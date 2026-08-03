package identity

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	statuserrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	identitymiddleware "github.com/wangweihong/omnimam/backend/internal/apiserver/middleware"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

const opaqueExchangeLifetime = 2 * time.Minute

// RegisterStart validates account metadata and starts a password-free OPAQUE registration exchange.
func (s *Service) RegisterStart(ctx context.Context, req *iapiserver.IdentityRegisterStartRequest) (*iapiserver.IdentityOpaqueStartResponse, error) {
	if s.opaque == nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "opaque is not configured")
	}
	config, err := s.store.PlatformManagement().GetSystemAuthConfig(ctx)
	if err != nil || config.RegistrationMode != "OPEN" && config.RegistrationMode != "ADMIN_APPROVAL" {
		return nil, statuserrors.NewStatus(code.ErrIdentityRegistrationStateInvalid, "registration mode is invalid")
	}
	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(req.Email)
	normalizedUsername := normalize(username)
	normalizedEmail := normalize(email)
	if existing, lookupErr := s.store.Identities().GetUserByLogin(ctx, normalizedUsername); lookupErr == nil && existing.ID != "" {
		return nil, statuserrors.NewStatus(code.ErrIdentityUsernameAlreadyExists, "username already exists")
	}
	if existing, lookupErr := s.store.Identities().GetUserByLogin(ctx, normalizedEmail); lookupErr == nil && existing.ID != "" {
		return nil, statuserrors.NewStatus(code.ErrIdentityEmailAlreadyExists, "email already exists")
	}
	request, err := decodeOpaqueMessage(req.RegistrationRequest)
	if err != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "registration message is invalid")
	}
	user := &iapiserver.IdentityUser{ObjectMeta: imachinery.ObjectMeta{Name: username}, Username: username, NormalizedUsername: normalizedUsername, DisplayName: strings.TrimSpace(req.DisplayName), Email: &email, NormalizedEmail: &normalizedEmail, Status: iapiserver.IdentityUserPending, SecurityVersion: 1, AuthorizationVersion: 1}
	var userID string
	if config.RegistrationMode == "ADMIN_APPROVAL" {
		created, createErr := s.store.Identities().CreatePendingRegistration(ctx, user)
		if createErr != nil {
			return nil, statuserrors.NewStatus(code.ErrIdentityUsernameAlreadyExists, "username or email already exists")
		}
		userID = created.User.ID
	} else {
		created, createErr := s.store.Identities().CreateUser(ctx, user)
		if createErr != nil {
			return nil, statuserrors.NewStatus(code.ErrIdentityUsernameAlreadyExists, "username or email already exists")
		}
		userID = created.ID
	}
	registrationResponse, err := s.opaque.RegistrationResponse(request, []byte(userID))
	if err != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "registration message is invalid")
	}
	exchangeID := uuid.NewString()
	userIDCopy := userID
	exchange := &iapiserver.IdentityOpaqueExchange{ObjectMeta: imachinery.ObjectMeta{ID: exchangeID, Name: "opaque-register:" + exchangeID}, ExchangeType: "REGISTER", UserID: &userIDCopy, UserIdentifier: userID, ServerOutput: "", ExpiresAt: imachinery.NewTime(time.Now().Add(opaqueExchangeLifetime))}
	if err := s.store.Identities().CreateOpaqueExchange(ctx, exchange); err != nil {
		return nil, err
	}
	return &iapiserver.IdentityOpaqueStartResponse{ExchangeID: exchangeID, RegistrationResponse: encodeOpaqueMessage(registrationResponse)}, nil
}

// RegisterFinish consumes a registration exchange and stores the client-produced OPAQUE record.
func (s *Service) RegisterFinish(ctx context.Context, req *iapiserver.IdentityRegisterFinishRequest) (any, error) {
	exchange, err := s.consumeExchange(ctx, req.ExchangeID, "REGISTER")
	if err != nil {
		return nil, err
	}
	record, err := decodeOpaqueMessage(req.RegistrationRecord)
	if err != nil || s.opaque == nil || s.opaque.ValidateRegistrationRecord(record) != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "registration record is invalid")
	}
	if exchange.UserID == nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "registration exchange has no user")
	}
	user, err := s.store.Identities().GetUser(ctx, *exchange.UserID)
	if err != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityUserNotVisible, "user is not visible")
	}
	config, err := s.store.PlatformManagement().GetSystemAuthConfig(ctx)
	if err != nil {
		return nil, statuserrors.NewStatus(code.ErrPlatformAuthConfigInvalid, "system auth config is unavailable")
	}
	user.OpaqueRegistrationRecord = req.RegistrationRecord
	user.PasswordChangedAt = pointerTime(imachinery.Now())
	user.FirstLoginRequired = false
	active := config.RegistrationMode == "OPEN"
	if !active {
		user.Status = iapiserver.IdentityUserPending
	}
	updated, err := s.store.Identities().FinalizeOpaqueRegistration(ctx, user, active)
	if err != nil {
		return nil, err
	}
	if !active {
		application, appErr := s.store.Identities().GetRegistrationApplication(ctx, *exchange.UserID)
		if appErr == nil && application.Application != nil {
			return &iapiserver.IdentityPendingRegistrationResponse{RegistrationApplicationID: application.Application.ID, UserID: updated.ID, Status: application.Application.Status, SubmittedAt: application.Application.SubmittedAt}, nil
		}
		return &iapiserver.IdentityPendingRegistrationResponse{UserID: updated.ID, Status: iapiserver.IdentityRegistrationPending, SubmittedAt: imachinery.Now()}, nil
	}
	return s.issueSession(ctx, updated, "register", "", "")
}

// LoginStart starts OPAQUE login and uses a fake record for an unknown identifier.
func (s *Service) LoginStart(ctx context.Context, req *iapiserver.IdentityLoginStartRequest) (*iapiserver.IdentityOpaqueStartResponse, error) {
	if s.opaque == nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "opaque is not configured")
	}
	ke1, err := decodeOpaqueMessage(req.KE1)
	if err != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "login message is invalid")
	}
	user, lookupErr := s.store.Identities().GetUserByLogin(ctx, normalize(req.Login))
	identifier := []byte(normalize(req.Login))
	var record []byte
	var userID *string
	if lookupErr == nil && user != nil && user.ID != "" && user.OpaqueRegistrationRecord != "" {
		record, err = decodeOpaqueMessage(user.OpaqueRegistrationRecord)
		userID = &user.ID
		identifier = []byte(user.ID)
	} else {
		record, err = s.opaque.FakeRecord(identifier)
	}
	if err != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "login record is invalid")
	}
	ke2, serverMAC, err := s.opaque.GenerateKE2(ke1, record, identifier)
	if err != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "login message is invalid")
	}
	exchangeID := uuid.NewString()
	if err := s.store.Identities().CreateOpaqueExchange(ctx, &iapiserver.IdentityOpaqueExchange{ObjectMeta: imachinery.ObjectMeta{ID: exchangeID, Name: "opaque-login:" + exchangeID}, ExchangeType: "LOGIN", UserID: userID, UserIdentifier: string(identifier), ServerOutput: encodeOpaqueMessage(serverMAC), ExpiresAt: imachinery.NewTime(time.Now().Add(opaqueExchangeLifetime))}); err != nil {
		return nil, err
	}
	return &iapiserver.IdentityOpaqueStartResponse{ExchangeID: exchangeID, KE2: encodeOpaqueMessage(ke2)}, nil
}

// LoginFinish verifies KE3 before running existing account, lock, audit, session and token behavior.
func (s *Service) LoginFinish(ctx context.Context, req *iapiserver.IdentityLoginFinishRequest, ip, userAgent string) (*iapiserver.IdentityAuthUserResponse, error) {
	exchange, err := s.consumeExchange(ctx, req.ExchangeID, "LOGIN")
	if err != nil {
		return nil, err
	}
	if exchange.UserID == nil || s.opaque == nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityInvalidCredentials, "invalid credentials")
	}
	ke3, err := decodeOpaqueMessage(req.KE3)
	serverMAC, macErr := decodeOpaqueMessage(exchange.ServerOutput)
	if err != nil || macErr != nil || s.opaque.VerifyKE3(ke3, serverMAC) != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityInvalidCredentials, "invalid credentials")
	}
	user, err := s.store.Identities().GetUser(ctx, *exchange.UserID)
	if err != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityInvalidCredentials, "invalid credentials")
	}
	switch user.Status {
	case iapiserver.IdentityUserPending:
		return nil, statuserrors.NewStatus(code.ErrIdentityAccountPending, "account is pending approval")
	case iapiserver.IdentityUserRejected:
		return nil, statuserrors.NewStatus(code.ErrIdentityAccountRejected, "account registration was rejected")
	case iapiserver.IdentityUserDisabled, iapiserver.IdentityUserDeleted:
		return nil, statuserrors.NewStatus(code.ErrIdentityAccountDisabled, "account is disabled")
	case iapiserver.IdentityUserLocked:
		if user.LockedUntil != nil && user.LockedUntil.Time.After(time.Now()) {
			return nil, statuserrors.NewStatus(code.ErrIdentityAccountLocked, "account is locked")
		}
	case iapiserver.IdentityUserActive:
	default:
		return nil, statuserrors.NewStatus(code.ErrIdentityUserStateInvalid, "account state is invalid")
	}
	user.FailedLoginCount = 0
	user.LastLoginAt = pointerTime(imachinery.Now())
	user.Status = iapiserver.IdentityUserActive
	updated, err := s.store.Identities().UpdateUser(ctx, user)
	if err != nil {
		return nil, err
	}
	return s.issueSession(ctx, updated, "omnimam-web", ip, userAgent)
}

// ChangePasswordStart authenticates the current session context and starts an OPAQUE old-password exchange.
func (s *Service) ChangePasswordStart(ctx context.Context, req *iapiserver.IdentityChangePasswordStartRequest) (*iapiserver.IdentityOpaqueStartResponse, error) {
	p, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok || p.PrincipalType != "USER" {
		return nil, statuserrors.NewStatus(code.ErrIdentityAuthzContextInvalid, "user principal context is missing")
	}
	user, err := s.store.Identities().GetUser(ctx, p.PrincipalID)
	if err != nil || user.OpaqueRegistrationRecord == "" {
		return nil, statuserrors.NewStatus(code.ErrIdentityUserNotVisible, "user is not visible")
	}
	ke1, err := decodeOpaqueMessage(req.KE1)
	record, recordErr := decodeOpaqueMessage(user.OpaqueRegistrationRecord)
	registrationRequest, registrationErr := decodeOpaqueMessage(req.RegistrationRequest)
	if err != nil || recordErr != nil || registrationErr != nil || s.opaque == nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "change-password message is invalid")
	}
	registrationResponse, err := s.opaque.RegistrationResponse(registrationRequest, []byte(user.ID))
	if err != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "change-password message is invalid")
	}
	ke2, serverMAC, err := s.opaque.GenerateKE2(ke1, record, []byte(user.ID))
	if err != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "change-password message is invalid")
	}
	exchangeID := uuid.NewString()
	userID := user.ID
	if err := s.store.Identities().CreateOpaqueExchange(ctx, &iapiserver.IdentityOpaqueExchange{ObjectMeta: imachinery.ObjectMeta{ID: exchangeID, Name: "opaque-change:" + exchangeID}, ExchangeType: "CHANGE_PASSWORD", UserID: &userID, UserIdentifier: user.ID, ServerOutput: encodeOpaqueMessage(serverMAC), ExpiresAt: imachinery.NewTime(time.Now().Add(opaqueExchangeLifetime))}); err != nil {
		return nil, err
	}
	return &iapiserver.IdentityOpaqueStartResponse{ExchangeID: exchangeID, KE2: encodeOpaqueMessage(ke2), RegistrationResponse: encodeOpaqueMessage(registrationResponse)}, nil
}

// ChangePasswordFinish verifies the old password and atomically replaces the OPAQUE record.
func (s *Service) ChangePasswordFinish(ctx context.Context, req *iapiserver.IdentityChangePasswordRequest) (*iapiserver.IdentityActionResult, error) {
	p, ok := identitymiddleware.PrincipalFromContext(ctx)
	if !ok || p.PrincipalType != "USER" {
		return nil, statuserrors.NewStatus(code.ErrIdentityAuthzContextInvalid, "user principal context is missing")
	}
	exchange, err := s.consumeExchange(ctx, req.ExchangeID, "CHANGE_PASSWORD")
	if err != nil {
		return nil, err
	}
	ke3, ke3Err := decodeOpaqueMessage(req.KE3)
	mac, macErr := decodeOpaqueMessage(exchange.ServerOutput)
	record, recordErr := decodeOpaqueMessage(req.RegistrationRecord)
	if ke3Err != nil || macErr != nil || recordErr != nil || s.opaque == nil || s.opaque.VerifyKE3(ke3, mac) != nil || s.opaque.ValidateRegistrationRecord(record) != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityOldPasswordInvalid, "current password is invalid")
	}
	user, err := s.store.Identities().GetUser(ctx, p.PrincipalID)
	if err != nil || exchange.UserID == nil || *exchange.UserID != user.ID {
		return nil, statuserrors.NewStatus(code.ErrIdentityUserNotVisible, "user is not visible")
	}
	user.OpaqueRegistrationRecord = req.RegistrationRecord
	user.PasswordChangedAt = pointerTime(imachinery.Now())
	user.SecurityVersion++
	if _, err := s.store.Identities().FinalizeOpaqueRegistration(ctx, user, user.Status == iapiserver.IdentityUserActive); err != nil {
		return nil, err
	}
	if err := s.store.Identities().RevokeUserSessions(ctx, p.PrincipalID, "PASSWORD_CHANGED"); err != nil {
		return nil, err
	}
	return &iapiserver.IdentityActionResult{Success: true, Message: "password changed"}, nil
}

// AdminRegisterStart starts the same client-side OPAQUE registration flow for an administrator-created user.
func (s *Service) AdminRegisterStart(ctx context.Context, req *iapiserver.IdentityAdminUserRegisterStartRequest) (*iapiserver.IdentityOpaqueStartResponse, error) {
	if s.opaque == nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "opaque is not configured")
	}
	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(req.Email)
	if existing, lookupErr := s.store.Identities().GetUserByLogin(ctx, normalize(username)); lookupErr == nil && existing.ID != "" {
		return nil, statuserrors.NewStatus(code.ErrIdentityUsernameAlreadyExists, "username already exists")
	}
	if existing, lookupErr := s.store.Identities().GetUserByLogin(ctx, normalize(email)); lookupErr == nil && existing.ID != "" {
		return nil, statuserrors.NewStatus(code.ErrIdentityEmailAlreadyExists, "email already exists")
	}
	registrationRequest, err := decodeOpaqueMessage(req.RegistrationRequest)
	if err != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "registration message is invalid")
	}
	user := &iapiserver.IdentityUser{ObjectMeta: imachinery.ObjectMeta{Name: username}, Username: username, NormalizedUsername: normalize(username), DisplayName: strings.TrimSpace(req.DisplayName), Email: &email, Status: iapiserver.IdentityUserActive, FirstLoginRequired: true, SecurityVersion: 1, AuthorizationVersion: 1}
	created, err := s.store.Identities().CreateUser(ctx, user)
	if err != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityUsernameAlreadyExists, "username or email already exists")
	}
	registrationResponse, err := s.opaque.RegistrationResponse(registrationRequest, []byte(created.ID))
	if err != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "registration message is invalid")
	}
	exchangeID := uuid.NewString()
	userID := created.ID
	if err := s.store.Identities().CreateOpaqueExchange(ctx, &iapiserver.IdentityOpaqueExchange{ObjectMeta: imachinery.ObjectMeta{ID: exchangeID, Name: "opaque-admin-register:" + exchangeID}, ExchangeType: "ADMIN_REGISTER", UserID: &userID, UserIdentifier: user.ID, ExpiresAt: imachinery.NewTime(time.Now().Add(opaqueExchangeLifetime))}); err != nil {
		return nil, err
	}
	return &iapiserver.IdentityOpaqueStartResponse{ExchangeID: exchangeID, RegistrationResponse: encodeOpaqueMessage(registrationResponse)}, nil
}

// AdminRegisterFinish commits an administrator-created user's OPAQUE record without returning a password.
func (s *Service) AdminRegisterFinish(ctx context.Context, req *iapiserver.IdentityAdminUserRegisterFinishRequest) (*iapiserver.IdentityAdminUserCreatedResponse, error) {
	exchange, err := s.consumeExchange(ctx, req.ExchangeID, "ADMIN_REGISTER")
	if err != nil {
		return nil, err
	}
	if exchange.UserID == nil || s.opaque == nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "registration exchange is invalid")
	}
	record, err := decodeOpaqueMessage(req.RegistrationRecord)
	if err != nil || s.opaque.ValidateRegistrationRecord(record) != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "registration record is invalid")
	}
	user, err := s.store.Identities().GetUser(ctx, *exchange.UserID)
	if err != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityUserNotVisible, "user is not visible")
	}
	user.OpaqueRegistrationRecord = req.RegistrationRecord
	user.FirstLoginRequired = true
	user.PasswordChangedAt = pointerTime(imachinery.Now())
	updated, err := s.store.Identities().FinalizeOpaqueRegistration(ctx, user, true)
	if err != nil {
		return nil, err
	}
	return &iapiserver.IdentityAdminUserCreatedResponse{User: updated, FirstLoginRequired: true}, nil
}

// AdminResetInitialPasswordStart starts a fresh registration exchange for a first-login user.
func (s *Service) AdminResetInitialPasswordStart(ctx context.Context, userID string, registrationRequest string) (*iapiserver.IdentityOpaqueStartResponse, error) {
	if s.opaque == nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "opaque is not configured")
	}
	user, err := s.store.Identities().GetUser(ctx, userID)
	if err != nil || !user.FirstLoginRequired {
		return nil, statuserrors.NewStatus(code.ErrIdentityUserStateInvalid, "initial password reset is not allowed")
	}
	request, err := decodeOpaqueMessage(registrationRequest)
	if err != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "registration message is invalid")
	}
	response, err := s.opaque.RegistrationResponse(request, []byte(user.ID))
	if err != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "registration message is invalid")
	}
	exchangeID := uuid.NewString()
	uid := user.ID
	if err := s.store.Identities().CreateOpaqueExchange(ctx, &iapiserver.IdentityOpaqueExchange{ObjectMeta: imachinery.ObjectMeta{ID: exchangeID, Name: "opaque-admin-reset:" + exchangeID}, ExchangeType: "ADMIN_RESET", UserID: &uid, UserIdentifier: user.ID, ExpiresAt: imachinery.NewTime(time.Now().Add(opaqueExchangeLifetime))}); err != nil {
		return nil, err
	}
	return &iapiserver.IdentityOpaqueStartResponse{ExchangeID: exchangeID, RegistrationResponse: encodeOpaqueMessage(response)}, nil
}

// AdminResetInitialPasswordFinish stores a new first-login registration record and invalidates old sessions.
func (s *Service) AdminResetInitialPasswordFinish(ctx context.Context, userID string, req *iapiserver.IdentityAdminUserRegisterFinishRequest) (*iapiserver.IdentityAdminUserCreatedResponse, error) {
	exchange, err := s.consumeExchange(ctx, req.ExchangeID, "ADMIN_RESET")
	if err != nil {
		return nil, err
	}
	if exchange.UserID == nil || *exchange.UserID != userID || s.opaque == nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "registration exchange is invalid")
	}
	record, err := decodeOpaqueMessage(req.RegistrationRecord)
	if err != nil || s.opaque.ValidateRegistrationRecord(record) != nil {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "registration record is invalid")
	}
	user, err := s.store.Identities().GetUser(ctx, userID)
	if err != nil || !user.FirstLoginRequired {
		return nil, statuserrors.NewStatus(code.ErrIdentityUserStateInvalid, "initial password reset is not allowed")
	}
	user.OpaqueRegistrationRecord = req.RegistrationRecord
	user.SecurityVersion++
	user.PasswordChangedAt = pointerTime(imachinery.Now())
	updated, err := s.store.Identities().FinalizeOpaqueRegistration(ctx, user, true)
	if err != nil {
		return nil, err
	}
	if err := s.store.Identities().RevokeUserSessions(ctx, userID, "INITIAL_PASSWORD_RESET"); err != nil {
		return nil, err
	}
	return &iapiserver.IdentityAdminUserCreatedResponse{User: updated, FirstLoginRequired: true}, nil
}

func (s *Service) consumeExchange(ctx context.Context, id, expectedType string) (*iapiserver.IdentityOpaqueExchange, error) {
	exchange, err := s.store.Identities().ConsumeOpaqueExchange(ctx, id)
	if err != nil {
		message := strings.ToLower(err.Error())
		if strings.Contains(message, "expired") {
			return nil, statuserrors.NewStatus(code.ErrIdentityPasswordExchangeExpired, "opaque exchange expired")
		}
		if strings.Contains(message, "consumed") {
			return nil, statuserrors.NewStatus(code.ErrIdentityPasswordExchangeReplayed, "opaque exchange already consumed")
		}
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "opaque exchange is invalid")
	}
	if exchange.ExchangeType != expectedType {
		return nil, statuserrors.NewStatus(code.ErrIdentityPasswordProtocolInvalid, "opaque exchange type is invalid")
	}
	return exchange, nil
}
