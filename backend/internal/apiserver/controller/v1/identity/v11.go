package identity

import (
	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	identitysvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/identity"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

// Controller 暴露已发布 Identity 契约中的认证、用户和授权管理接口。
type Controller struct{ service *identitysvc.Service }

func NewController(service *identitysvc.Service) *Controller { return &Controller{service: service} }

func (c *Controller) RegisterStart(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityRegisterStartRequest{}, func(req *iapiserver.IdentityRegisterStartRequest) (any, error) {
		return c.service.RegisterStart(ctx, req)
	})
}
func (c *Controller) RegisterFinish(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityRegisterFinishRequest{}, func(req *iapiserver.IdentityRegisterFinishRequest) (any, error) {
		return c.service.RegisterFinish(ctx, req)
	})
}
func (c *Controller) LoginStart(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityLoginStartRequest{}, func(req *iapiserver.IdentityLoginStartRequest) (any, error) { return c.service.LoginStart(ctx, req) })
}
func (c *Controller) LoginFinish(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityLoginFinishRequest{}, func(req *iapiserver.IdentityLoginFinishRequest) (any, error) {
		return c.service.LoginFinish(ctx, req, ctx.ClientIP(), ctx.Request.UserAgent())
	})
}
func (c *Controller) Refresh(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityRefreshRequest{}, func(req *iapiserver.IdentityRefreshRequest) (any, error) { return c.service.Refresh(ctx, req) })
}
func (c *Controller) Me(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.Me(ctx) })
}
func (c *Controller) Heartbeat(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.Heartbeat(ctx) })
}
func (c *Controller) Logout(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.Logout(ctx) })
}
func (c *Controller) LogoutAll(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.LogoutAll(ctx) })
}
func (c *Controller) ChangePasswordStart(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityChangePasswordStartRequest{}, func(req *iapiserver.IdentityChangePasswordStartRequest) (any, error) {
		return c.service.ChangePasswordStart(ctx, req)
	})
}
func (c *Controller) ChangePasswordFinish(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityChangePasswordRequest{}, func(req *iapiserver.IdentityChangePasswordRequest) (any, error) {
		return c.service.ChangePasswordFinish(ctx, req)
	})
}
func (c *Controller) Sessions(ctx *gin.Context) {
	req := &iapiserver.IdentitySessionListRequest{}
	core.Run(ctx, req, func(r *iapiserver.IdentitySessionListRequest) (any, error) { return c.service.Sessions(ctx, r) })
}
func (c *Controller) RevokeSession(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.RevokeSession(ctx, ctx.Param("session_id")) })
}
func (c *Controller) Permissions(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.Permissions(ctx) })
}
func (c *Controller) ListUsers(ctx *gin.Context) {
	req := &iapiserver.IdentityUserListRequest{}
	core.Run(ctx, req, func(r *iapiserver.IdentityUserListRequest) (any, error) { return c.service.ListUsers(ctx, r) })
}
func (c *Controller) AdminCreateUser(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityAdminUserCreateRequest{}, func(r *iapiserver.IdentityAdminUserCreateRequest) (any, error) {
		return c.service.AdminCreateUser(ctx, r)
	})
}
func (c *Controller) AdminRegisterStart(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityAdminUserRegisterStartRequest{}, func(r *iapiserver.IdentityAdminUserRegisterStartRequest) (any, error) {
		return c.service.AdminRegisterStart(ctx, r)
	})
}
func (c *Controller) AdminRegisterFinish(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityAdminUserRegisterFinishRequest{}, func(r *iapiserver.IdentityAdminUserRegisterFinishRequest) (any, error) {
		return c.service.AdminRegisterFinish(ctx, r)
	})
}
func (c *Controller) AdminResetInitialPasswordStart(ctx *gin.Context) {
	var request struct {
		RegistrationRequest string `json:"registration_request" binding:"required,max=16384"`
	}
	core.Run(ctx, &request, func(r *struct {
		RegistrationRequest string `json:"registration_request" binding:"required,max=16384"`
	}) (any, error) { return c.service.AdminResetInitialPasswordStart(ctx, ctx.Param("user_id"), r.RegistrationRequest) })
}
func (c *Controller) AdminResetInitialPasswordFinish(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityAdminUserRegisterFinishRequest{}, func(r *iapiserver.IdentityAdminUserRegisterFinishRequest) (any, error) {
		return c.service.AdminResetInitialPasswordFinish(ctx, ctx.Param("user_id"), r)
	})
}
func (c *Controller) GetUser(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetUser(ctx, ctx.Param("user_id")) })
}
func (c *Controller) UpdateUser(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityAdminUserUpdateRequest{}, func(r *iapiserver.IdentityAdminUserUpdateRequest) (any, error) {
		return c.service.UpdateUser(ctx, ctx.Param("user_id"), r)
	})
}
func (c *Controller) DisableUser(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		return c.service.SetUserStatus(ctx, ctx.Param("user_id"), iapiserver.IdentityUserDisabled)
	})
}
func (c *Controller) EnableUser(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		return c.service.SetUserStatus(ctx, ctx.Param("user_id"), iapiserver.IdentityUserActive)
	})
}
func (c *Controller) UnlockUser(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		return c.service.SetUserStatus(ctx, ctx.Param("user_id"), iapiserver.IdentityUserActive)
	})
}
func (c *Controller) DeleteUser(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		return c.service.SetUserStatus(ctx, ctx.Param("user_id"), iapiserver.IdentityUserDeleted)
	})
}

// ListRegistrationApplications 查询管理员可见的自主注册申请。
func (c *Controller) ListRegistrationApplications(ctx *gin.Context) {
	req := &iapiserver.IdentityRegistrationApplicationListRequest{Statuses: iapiserver.IdentityRegistrationPending}
	core.Run(ctx, req, func(r *iapiserver.IdentityRegistrationApplicationListRequest) (any, error) {
		return c.service.ListRegistrationApplications(ctx, r)
	})
}

// GetRegistrationApplication 返回注册申请详情和关联用户管理摘要。
func (c *Controller) GetRegistrationApplication(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		return c.service.GetRegistrationApplication(ctx, ctx.Param("registration_application_id"))
	})
}

// ApproveRegistrationApplication 批准注册申请并激活用户。
func (c *Controller) ApproveRegistrationApplication(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		return c.service.ReviewRegistrationApplication(ctx, ctx.Param("registration_application_id"), iapiserver.IdentityRegistrationApproved, "")
	})
}

// RejectRegistrationApplication 按必填原因拒绝注册申请。
func (c *Controller) RejectRegistrationApplication(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityRegistrationRejectRequest{}, func(req *iapiserver.IdentityRegistrationRejectRequest) (any, error) {
		return c.service.ReviewRegistrationApplication(ctx, ctx.Param("registration_application_id"), iapiserver.IdentityRegistrationRejected, req.Reason)
	})
}

func (c *Controller) ListPermissionDefinitions(ctx *gin.Context) {
	req := &iapiserver.IdentityPermissionListRequest{}
	core.Run(ctx, req, func(r *iapiserver.IdentityPermissionListRequest) (any, error) {
		return c.service.ListPermissionDefinitions(ctx, r)
	})
}
func (c *Controller) ListRoles(ctx *gin.Context) {
	req := &iapiserver.IdentityRoleListRequest{}
	core.Run(ctx, req, func(r *iapiserver.IdentityRoleListRequest) (any, error) { return c.service.ListRoles(ctx, r) })
}
func (c *Controller) CreateRole(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityRoleWriteRequest{}, func(r *iapiserver.IdentityRoleWriteRequest) (any, error) { return c.service.CreateRole(ctx, r) })
}
func (c *Controller) GetRole(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetRole(ctx, ctx.Param("role_id")) })
}
func (c *Controller) UpdateRole(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityRoleWriteRequest{}, func(r *iapiserver.IdentityRoleWriteRequest) (any, error) {
		return c.service.UpdateRole(ctx, ctx.Param("role_id"), r)
	})
}
func (c *Controller) ReplaceRolePermissions(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityPermissionReplaceRequest{}, func(r *iapiserver.IdentityPermissionReplaceRequest) (any, error) {
		return c.service.ReplaceRolePermissions(ctx, ctx.Param("role_id"), r)
	})
}
func (c *Controller) ListGroups(ctx *gin.Context) {
	req := &iapiserver.IdentityGroupListRequest{}
	core.Run(ctx, req, func(r *iapiserver.IdentityGroupListRequest) (any, error) { return c.service.ListGroups(ctx, r) })
}
func (c *Controller) CreateGroup(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityGroupWriteRequest{}, func(r *iapiserver.IdentityGroupWriteRequest) (any, error) { return c.service.CreateGroup(ctx, r) })
}
func (c *Controller) GetGroup(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetGroup(ctx, ctx.Param("group_id")) })
}
func (c *Controller) UpdateGroup(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityGroupWriteRequest{}, func(r *iapiserver.IdentityGroupWriteRequest) (any, error) {
		return c.service.UpdateGroup(ctx, ctx.Param("group_id"), r)
	})
}
func (c *Controller) ReplaceGroupMembers(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityGroupMembersReplaceRequest{}, func(r *iapiserver.IdentityGroupMembersReplaceRequest) (any, error) {
		return c.service.ReplaceGroupMembers(ctx, ctx.Param("group_id"), r)
	})
}
func (c *Controller) ReplaceGroupRoles(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityRoleIDsReplaceRequest{}, func(r *iapiserver.IdentityRoleIDsReplaceRequest) (any, error) {
		return c.service.ReplaceGroupRoles(ctx, ctx.Param("group_id"), r)
	})
}
func (c *Controller) ListResourceGrants(ctx *gin.Context) {
	req := &iapiserver.IdentityResourceGrantListRequest{}
	core.Run(ctx, req, func(r *iapiserver.IdentityResourceGrantListRequest) (any, error) {
		return c.service.ListResourceGrants(ctx, ctx.Param("resource_type"), ctx.Param("resource_id"), r)
	})
}
func (c *Controller) CreateResourceGrant(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityResourceGrantCreateRequest{}, func(r *iapiserver.IdentityResourceGrantCreateRequest) (any, error) {
		return c.service.CreateResourceGrant(ctx, ctx.Param("resource_type"), ctx.Param("resource_id"), r)
	})
}
func (c *Controller) UpdateResourceGrant(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityResourceGrantUpdateRequest{}, func(r *iapiserver.IdentityResourceGrantUpdateRequest) (any, error) {
		return c.service.UpdateResourceGrant(ctx, ctx.Param("grant_id"), r)
	})
}
func (c *Controller) RevokeResourceGrant(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.RevokeResourceGrant(ctx, ctx.Param("grant_id")) })
}
func (c *Controller) ListServiceAccounts(ctx *gin.Context) {
	req := &iapiserver.IdentityServiceAccountListRequest{}
	core.Run(ctx, req, func(r *iapiserver.IdentityServiceAccountListRequest) (any, error) {
		return c.service.ListServiceAccounts(ctx, r)
	})
}
func (c *Controller) CreateServiceAccount(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityServiceAccountCreateRequest{}, func(r *iapiserver.IdentityServiceAccountCreateRequest) (any, error) {
		return c.service.CreateServiceAccount(ctx, r)
	})
}
func (c *Controller) GetServiceAccount(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetServiceAccount(ctx, ctx.Param("service_account_id")) })
}
func (c *Controller) UpdateServiceAccount(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.IdentityServiceAccountUpdateRequest{}, func(r *iapiserver.IdentityServiceAccountUpdateRequest) (any, error) {
		return c.service.UpdateServiceAccount(ctx, ctx.Param("service_account_id"), r)
	})
}
func (c *Controller) DisableServiceAccount(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		return c.service.SetServiceAccountStatus(ctx, ctx.Param("service_account_id"), "DISABLED")
	})
}
func (c *Controller) EnableServiceAccount(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		return c.service.SetServiceAccountStatus(ctx, ctx.Param("service_account_id"), "ACTIVE")
	})
}
func (c *Controller) RotateServiceAccountCredential(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) {
		return c.service.RotateServiceAccountCredential(ctx, ctx.Param("service_account_id"))
	})
}
