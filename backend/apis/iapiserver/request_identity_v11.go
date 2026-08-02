package iapiserver

import (
	"encoding/json"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

// IdentityRegisterRequest 是公开注册入口；注册策略由 Platform Management 决定。
type IdentityRegisterRequest struct {
	Username    string `json:"username" binding:"required,min=3,max=128"`
	Email       string `json:"email" binding:"required,email,max=320"`
	Password    string `json:"password" binding:"required,min=8,max=256"`
	DisplayName string `json:"display_name" binding:"max=256"`
}

type IdentityLoginRequest struct {
	Login      string `json:"login" binding:"required,max=320"`
	Password   string `json:"password" binding:"required,max=256"`
	ClientID   string `json:"client_id" binding:"max=128"`
	DeviceInfo string `json:"device_info" binding:"max=512"`
}

type IdentityRefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}
type IdentityChangePasswordRequest struct {
	OldPassword     string `json:"old_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8,max=256"`
	ConfirmPassword string `json:"confirm_password" binding:"required,min=8,max=256"`
}
type IdentitySelfUpdateRequest struct {
	DisplayName *string `json:"display_name" binding:"omitempty,max=256"`
	Alias       *string `json:"alias" binding:"omitempty,max=128"`
	Email       *string `json:"email" binding:"omitempty,email,max=320"`
	Phone       *string `json:"phone" binding:"omitempty,max=64"`
}
type IdentityAdminUserCreateRequest struct {
	Username        string   `json:"username" binding:"required,min=3,max=128"`
	Email           string   `json:"email" binding:"required,email,max=320"`
	DisplayName     string   `json:"display_name" binding:"max=256"`
	InitialPassword string   `json:"initial_password" binding:"omitempty,min=8,max=256"`
	Status          string   `json:"status" binding:"omitempty,oneof=ACTIVE PENDING"`
	RoleIDs         []string `json:"role_ids" binding:"max=64,dive,max=128"`
}
type IdentityAdminUserUpdateRequest struct {
	DisplayName *string  `json:"display_name" binding:"omitempty,max=256"`
	Alias       *string  `json:"alias" binding:"omitempty,max=128"`
	Email       *string  `json:"email" binding:"omitempty,email,max=320"`
	Phone       *string  `json:"phone" binding:"omitempty,max=64"`
	Status      *string  `json:"status" binding:"omitempty,oneof=ACTIVE PENDING DISABLED LOCKED DELETED"`
	RoleIDs     []string `json:"role_ids" binding:"max=64,dive,max=128"`
}
type IdentityActionReasonRequest struct {
	Reason string `json:"reason" binding:"max=512"`
}

type IdentityRoleWriteRequest struct {
	Code        string `json:"code" binding:"required,min=1,max=128"`
	Name        string `json:"name" binding:"required,min=1,max=256"`
	Description string `json:"description" binding:"max=1024"`
	Status      string `json:"status" binding:"omitempty,oneof=ACTIVE DISABLED"`
}
type IdentityPermissionReplaceRequest struct {
	PermissionCodes []string `json:"permission_codes" binding:"max=256,dive,max=256"`
}
type IdentityGroupWriteRequest struct {
	Code        string `json:"code" binding:"required,min=1,max=128"`
	Name        string `json:"name" binding:"required,min=1,max=256"`
	Description string `json:"description" binding:"max=1024"`
	Status      string `json:"status" binding:"omitempty,oneof=ACTIVE DISABLED"`
}
type IdentityGroupMembersReplaceRequest struct {
	Items []string `json:"items" binding:"max=1000,dive,max=128"`
}
type IdentityRoleIDsReplaceRequest struct {
	Items []string `json:"items" binding:"max=256,dive,max=128"`
}
type IdentityResourceGrantCreateRequest struct {
	SubjectType string           `json:"subject_type" binding:"required,oneof=USER GROUP"`
	SubjectID   string           `json:"subject_id" binding:"required,max=128"`
	AccessLevel string           `json:"access_level" binding:"required,oneof=VIEW USE EDIT MANAGE"`
	ExpiresAt   *imachinery.Time `json:"expires_at"`
}
type IdentityResourceGrantUpdateRequest struct {
	AccessLevel *string          `json:"access_level" binding:"omitempty,oneof=VIEW USE EDIT MANAGE"`
	ExpiresAt   *imachinery.Time `json:"expires_at"`
}
type IdentityServiceAccountCreateRequest struct {
	Code            string   `json:"code" binding:"required,min=1,max=128"`
	Name            string   `json:"name" binding:"required,min=1,max=256"`
	Description     string   `json:"description" binding:"max=1024"`
	OwnerType       string   `json:"owner_type" binding:"required,oneof=SYSTEM WORKER AGENT APPLICATION EXTERNAL"`
	OwnerID         string   `json:"owner_id" binding:"max=128"`
	PermissionCodes []string `json:"permission_codes" binding:"max=256,dive,max=256"`
}
type IdentityServiceAccountUpdateRequest struct {
	Name            string   `json:"name" binding:"required,min=1,max=256"`
	Description     *string  `json:"description" binding:"omitempty,max=1024"`
	PermissionCodes []string `json:"permission_codes" binding:"max=256,dive,max=256"`
}
type IdentityPrincipalCheckRequest struct {
	PermissionCodes []string `json:"permission_codes" binding:"required,max=256,dive,max=256"`
	ActorUserID     string   `json:"actor_user_id" binding:"max=128"`
	ResourceType    string   `json:"resource_type" binding:"max=128"`
	ResourceID      string   `json:"resource_id" binding:"max=128"`
}

type IdentityAuthUserResponse struct {
	User               *IdentityUser `json:"user"`
	AccessToken        string        `json:"access_token"`
	RefreshToken       string        `json:"refresh_token"`
	TokenType          string        `json:"token_type"`
	ExpiresIn          int           `json:"expires_in"`
	FirstLoginRequired bool          `json:"first_login_required"`
}
type IdentityActionResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}
type IdentityPresenceHeartbeatResponse struct {
	Online       bool            `json:"online"`
	LastActiveAt imachinery.Time `json:"last_active_at"`
}
type IdentitySessionListResponse struct {
	Total int64                  `json:"total"`
	Items []*IdentityAuthSession `json:"items"`
}
type IdentityUserListResponse struct {
	Total int64           `json:"total"`
	Items []*IdentityUser `json:"items"`
}
type IdentityPermissionListResponse struct {
	Total int64                           `json:"total"`
	Items []*IdentityPermissionDefinition `json:"items"`
}
type IdentityRoleListResponse struct {
	Total int64           `json:"total"`
	Items []*IdentityRole `json:"items"`
}
type IdentityGroupListResponse struct {
	Total int64            `json:"total"`
	Items []*IdentityGroup `json:"items"`
}
type IdentityResourceGrantListResponse struct {
	Total int64                          `json:"total"`
	Items []*IdentityResourceAccessGrant `json:"items"`
}
type IdentityServiceAccountListResponse struct {
	Total int64                     `json:"total"`
	Items []*IdentityServiceAccount `json:"items"`
}
type IdentityPermissionProjection struct {
	PrincipalType        string   `json:"principal_type"`
	PrincipalID          string   `json:"principal_id"`
	ActorUserID          string   `json:"actor_user_id,omitempty"`
	AuthorizationVersion int64    `json:"authorization_version"`
	PermissionCodes      []string `json:"permission_codes"`
}
type IdentityPrincipalCheckResult struct {
	PermissionCode string `json:"permission_code"`
	Allowed        bool   `json:"allowed"`
}
type IdentityPrincipalCheckResponse struct {
	PrincipalType        string                         `json:"principal_type"`
	PrincipalID          string                         `json:"principal_id"`
	ActorUserID          string                         `json:"actor_user_id,omitempty"`
	AuthorizationVersion int64                          `json:"authorization_version"`
	Results              []IdentityPrincipalCheckResult `json:"results"`
}
type IdentityServiceAccountCredentialResponse struct {
	ServiceAccount *IdentityServiceAccount `json:"service_account"`
	Credential     string                  `json:"credential"`
	ExpiresAt      *imachinery.Time        `json:"expires_at,omitempty"`
}

type IdentityUserListRequest struct {
	imachinery.BasicQueryParam
	Statuses  string `form:"statuses"`
	SortOrder string `form:"sort_order"`
}
type IdentitySessionListRequest struct{ imachinery.BasicQueryParam }
type IdentityRoleListRequest struct{ imachinery.BasicQueryParam }
type IdentityPermissionListRequest struct{ imachinery.BasicQueryParam }
type IdentityGroupListRequest struct{ imachinery.BasicQueryParam }
type IdentityResourceGrantListRequest struct{ imachinery.BasicQueryParam }
type IdentityServiceAccountListRequest struct{ imachinery.BasicQueryParam }

// IdentityActionRequestBody is used only for binding endpoints whose body is optional.
type IdentityActionRequestBody struct {
	Raw json.RawMessage `json:"-"`
}
