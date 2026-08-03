package code

const (
	// @HTTP 200
	// @CN 用户名或密码错误。
	// @EN The username or password is invalid.
	ErrIdentityInvalidCredentials int = 220200
	// @HTTP 200
	// @CN 账号正在等待审批。
	// @EN The account is pending approval.
	ErrIdentityAccountPending int = 220201
	// @HTTP 200
	// @CN 账号已禁用。
	// @EN The account is disabled.
	ErrIdentityAccountDisabled int = 220202
	// @HTTP 200
	// @CN 账号已临时锁定。
	// @EN The account is temporarily locked.
	ErrIdentityAccountLocked int = 220203
	// @HTTP 200
	// @CN 必须完成首次登录引导。
	// @EN The first-login flow must be completed.
	ErrIdentityFirstLoginRequired int = 220204
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
	// @CN 最近一次注册申请已被拒绝，可以重新提交申请。
	// @EN The latest registration application was rejected and may be resubmitted.
	ErrIdentityAccountRejected int = 220213
	// @HTTP 200
	// @CN 当前注册模式或账号申请状态不允许提交注册。
	// @EN The active registration mode or account application state does not allow registration.
	ErrIdentityRegistrationStateInvalid int = 220214
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
	// @CN 用户仍拥有未处理的业务资源、任务、服务账号或共享关系。
	// @EN The user still owns business resources, tasks, service accounts, or grants.
	ErrIdentityUserDeleteBlocked int = 220404
	// @HTTP 200
	// @CN 用户不能删除自己。
	// @EN A user cannot delete itself.
	ErrIdentitySelfDeleteForbidden int = 220405
	// @HTTP 200
	// @CN 不能删除或禁用最后一个有效 SUPER_ADMIN。
	// @EN The last effective SUPER_ADMIN cannot be deleted or disabled.
	ErrIdentityLastSuperAdminProtected int = 220406
	// @HTTP 200
	// @CN 用户创建参数或默认授权无效。
	// @EN The user creation input or default grant is invalid.
	ErrIdentityUserCreateInvalid int = 220407
	// @HTTP 200
	// @CN 注册申请不存在或当前主体不可见。
	// @EN The registration application does not exist or is not visible to the current principal.
	ErrIdentityRegistrationApplicationNotVisible int = 220408
	// @HTTP 200
	// @CN 注册申请已存在不可覆盖的相反审批结果。
	// @EN The registration application already has an opposite immutable decision.
	ErrIdentityRegistrationDecisionConflict int = 220409
	// @HTTP 200
	// @CN 拒绝注册申请必须填写原因。
	// @EN A reason is required to reject a registration application.
	ErrIdentityRegistrationReasonRequired int = 220410
	// @HTTP 200
	// @CN 用户删除依赖来源不可用或返回结果不完整，当前不能删除用户。
	// @EN A user-deletion dependency source is unavailable or incomplete, so the user cannot be deleted.
	ErrIdentityUserDeleteDependencyUnavailable int = 220411
	// @HTTP 200
	// @CN 用户删除依赖检查已过期或不再匹配当前来源事实，请重新检查。
	// @EN The user-deletion dependency check is expired or no longer matches current source facts.
	ErrIdentityUserDeleteCheckStale int = 220412
	// @HTTP 200
	// @CN 角色不存在或当前主体不可见。
	// @EN The role does not exist or is not visible to the current principal.
	ErrIdentityRoleNotVisible int = 220600
	// @HTTP 200
	// @CN 角色授权不存在、已过期或不允许。
	// @EN The role grant does not exist, is expired, or is not allowed.
	ErrIdentityRoleGrantInvalid int = 220602
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
	// @CN 服务账号编码已存在。
	// @EN The service account code already exists.
	ErrIdentityServiceAccountAlreadyExists int = 221001
	// @HTTP 200
	// @CN 服务账号已禁用。
	// @EN The service account is disabled.
	ErrIdentityServiceAccountDisabled int = 221002
	// @HTTP 200
	// @CN 服务账号凭据无效。
	// @EN The service account credential is invalid.
	ErrIdentityServiceAccountCredentialInvalid int = 221003
	// @HTTP 200
	// @CN 服务账号凭据轮换失败。
	// @EN Service account credential rotation failed.
	ErrIdentityServiceAccountCredentialRotationFailed int = 221004
	// @HTTP 200
	// @CN 服务账号当前状态不允许该操作。
	// @EN The service account state does not allow this operation.
	ErrIdentityServiceAccountStateInvalid int = 221005
	// @HTTP 200
	// @CN 服务账号 owner 不存在、不可见或不允许作为该账号归属。
	// @EN The service-account owner does not exist, is not visible, or cannot own this account.
	ErrIdentityServiceAccountOwnerInvalid int = 221006
	// @HTTP 200
	// @CN 服务账号 owner 校验当前不可用，凭据交换已被拒绝。
	// @EN Service-account owner validation is unavailable, so credential exchange was denied.
	ErrIdentityServiceAccountOwnerUnavailable int = 221007
	// @HTTP 200
	// @CN 服务账号凭据不存在或当前主体不可见。
	// @EN The service-account credential does not exist or is not visible to the current principal.
	ErrIdentityServiceAccountCredentialNotVisible int = 221008
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
	// @CN 审计记录不存在或当前主体不可见。
	// @EN The audit record does not exist or is not visible to the current principal.
	ErrPlatformAuditLogNotVisible int = 230400
	// @HTTP 200
	// @CN 平台审计查询参数无效。
	// @EN The platform audit query is invalid.
	ErrPlatformAuditQueryInvalid int = 230401
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
	// @HTTP 200
	// @CN 审计幂等键已用于不同内容。
	// @EN The audit idempotency key has already been used for different content.
	ErrPlatformAuditIdempotencyConflict int = 230404
)
