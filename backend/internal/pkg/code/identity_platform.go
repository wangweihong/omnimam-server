package code

const (
	// @HTTP 200
	// @CN 用户名或密码错误。
	// @EN The username or password is invalid.
	ErrIdentityInvalidCredentials int = 220200
	// @HTTP 200
	// @CN 账号已禁用。
	// @EN The account is disabled.
	ErrIdentityAccountDisabled int = 220202
	// @HTTP 200
	// @CN 账号已临时锁定。
	// @EN The account is temporarily locked.
	ErrIdentityAccountLocked int = 220203
	// @HTTP 200
	// @CN Access Token 已过期。
	// @EN The access token has expired.
	ErrIdentityTokenExpired int = 220205
	// @HTTP 200
	// @CN Token 或会话已撤销。
	// @EN The token or session has been revoked.
	ErrIdentityTokenRevoked int = 220206
	// @HTTP 200
	// @CN Refresh Token 无效。
	// @EN The refresh token is invalid.
	ErrIdentityRefreshTokenInvalid int = 220207
	// @HTTP 200
	// @CN 检测到 Refresh Token 重用，会话已撤销。
	// @EN Refresh token reuse was detected and the session was revoked.
	ErrIdentityRefreshTokenReused int = 220208
	// @HTTP 200
	// @CN 密码不符合当前安全策略。
	// @EN The password does not satisfy the active security policy.
	ErrIdentityPasswordPolicyFailed int = 220209
	// @HTTP 200
	// @CN 用户不存在或当前主体不可见。
	// @EN The user does not exist or is not visible to the current principal.
	ErrIdentityUserNotVisible int = 220400
	// @HTTP 200
	// @CN 用户名已存在。
	// @EN The username already exists.
	ErrIdentityUsernameAlreadyExists int = 220401
	// @HTTP 200
	// @CN 邮箱已存在。
	// @EN The email already exists.
	ErrIdentityEmailAlreadyExists int = 220402
	// @HTTP 200
	// @CN 用户当前状态不允许该操作。
	// @EN The user state does not allow this operation.
	ErrIdentityUserStateInvalid int = 220403
	// @HTTP 200
	// @CN 用户不能删除自己。
	// @EN A user cannot delete itself.
	ErrIdentitySelfDeleteForbidden int = 220405
	// @HTTP 200
	// @CN 角色不存在或当前主体不可见。
	// @EN The role does not exist or is not visible to the current principal.
	ErrIdentityRoleNotVisible int = 220600
	// @HTTP 200
	// @CN 当前主体授权上下文无效。
	// @EN The authorization context is invalid.
	ErrIdentityAuthzContextInvalid int = 220607
	// @HTTP 200
	// @CN 当前主体无权执行该操作。
	// @EN The current principal is not authorized for the operation.
	ErrIdentityAuthzDenied int = 220606
	// @HTTP 200
	// @CN 服务主体不存在或当前主体不可见。
	// @EN The service account does not exist or is not visible to the current principal.
	ErrIdentityServiceAccountNotVisible int = 221000
	// @HTTP 200
	// @CN 当前主体上下文无效。
	// @EN The principal context is invalid.
	ErrIdentityPrincipalContextInvalid int = 221205
	// @HTTP 200
	// @CN 平台认证配置无效。
	// @EN The system authentication configuration is invalid.
	ErrPlatformAuthConfigInvalid int = 230200
	// @HTTP 200
	// @CN 系统认证配置版本冲突。
	// @EN The system authentication configuration version conflicts with the current version.
	ErrPlatformAuthConfigVersionConflict int = 230202
	// @HTTP 200
	// @CN 平台系统概览暂不可用。
	// @EN The platform overview is temporarily unavailable.
	ErrPlatformOverviewUnavailable int = 230601
	// @HTTP 200
	// @CN 审计记录无效或包含禁止字段。
	// @EN The audit record is invalid or contains prohibited fields.
	ErrPlatformAuditRecordInvalid int = 230402
	// @HTTP 200
	// @CN 平台审计边界不可用，受控操作未执行。
	// @EN The platform audit boundary is unavailable and the controlled operation was not executed.
	ErrPlatformAuditWriteUnavailable int = 230403
)
