package apiserver

import (
	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	agentctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/agent"
	aichatctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/aichat"
	aiappctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/applicationplatform"
	appstudioctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/appstudio"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/asset"
	assetlibraryctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/assetlibrary"
	identityctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/identity"
	mcpctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/mcp"
	notificationctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/notification"
	platformctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/platform"
	platformmanagementctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/platformmanagement"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/prompt"
	ssectrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/sse"
	taskcenterctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/taskcenter"
	usermodelctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/usermodel"
	workflowcanvasctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/workflowcanvas"
	authmiddleware "github.com/wangweihong/omnimam/backend/internal/apiserver/middleware"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/options"
	agentsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/agent"
	aichatsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/aichat"
	appplatformsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	legacyappsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	appstudiosvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/appstudio"
	assetlibrarysvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/assetlibrary"
	identitysvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/identity"
	notificationsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/notification"
	platformmanagementsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/platformmanagement"
	taskcentersvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	usermodelsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/usermodel"
	workflowcanvassvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/workflowcanvas"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/pkg/core"
	"github.com/wangweihong/omnimam/backend/pkg/httpsvr/genericmiddleware"
	mcpprotocol "github.com/wangweihong/omnimam/backend/pkg/mcp"
)

func initRouter(
	g *gin.Engine,
	applicationPlatform appplatformsvc.ApplicationPlatformSrv,
	taskCenter taskcentersvc.TaskCenterSrv,
	userModel *usermodelsvc.Service,
	aiChat aichatsvc.AIChatSrv,
	agent *agentsvc.Service,
	appStudio *appstudiosvc.Service,
	authOptions *options.AuthOptions,
	sseOptions *options.SSEOptions,
	mcpProcessor *mcpprotocol.Processor,
	mcpOptions *options.MCPOptions,
) {
	InstallMiddleware(g)
	installApis(g, applicationPlatform, taskCenter, userModel, aiChat, agent, appStudio, authOptions, sseOptions, mcpProcessor, mcpOptions)
}

func InstallMiddleware(g *gin.Engine) {
	g.Use(genericmiddleware.RequestID())
	g.Use(genericmiddleware.Context())
	g.Use(genericmiddleware.LoggerMiddleware())
}

func InstallApis(
	g *gin.Engine,
	applicationPlatform appplatformsvc.ApplicationPlatformSrv,
	taskCenter taskcentersvc.TaskCenterSrv,
) *gin.Engine {
	return installApis(g, applicationPlatform, taskCenter, nil, nil, nil, nil, options.NewAuthOptions(), options.NewSSEOptions(), nil, options.NewMCPOptions())
}

func installApis(
	g *gin.Engine,
	applicationPlatform appplatformsvc.ApplicationPlatformSrv,
	taskCenter taskcentersvc.TaskCenterSrv,
	userModel *usermodelsvc.Service,
	aiChat aichatsvc.AIChatSrv,
	agent *agentsvc.Service,
	appStudio *appstudiosvc.Service,
	authOptions *options.AuthOptions,
	sseOptions *options.SSEOptions,
	mcpProcessor *mcpprotocol.Processor,
	mcpOptions *options.MCPOptions,
) *gin.Engine {
	g.NoRoute(func(c *gin.Context) {
		core.WriteResponse(c, errors.NewStatusF(code.ErrPageNotFound, "Page not found."), nil)
	})
	storeIns := store.Client()
	if storeIns != nil {
		if mcpProcessor != nil && mcpOptions != nil && mcpOptions.Enabled {
			g.POST("/mcp", mcpctrl.New(
				mcpProcessor,
				storeIns.Identities(),
				authmiddleware.IdentityJWTSecret(authOptions),
				mcpOptions,
			).Handle)
		}
		v1 := g.Group("/api/v1")
		{
			v1.Use(authmiddleware.IdentityAuthenticationForAPIs(authOptions, storeIns.Identities()))
			installIdentityPlatformApis(v1, storeIns, authOptions)
			installSSEApis(v1, storeIns, sseOptions)
			installNotificationApis(v1, storeIns)
			installPlatformApis(v1, storeIns, nil)
			if userModel != nil {
				installUserModelApis(v1, userModel)
			}
			installAssetLibraryContractApis(v1, storeIns, taskCenter, applicationPlatform)
			installAssetApis(v1, storeIns)
			installPromptApis(v1, storeIns)
			if agent != nil {
				installAgentApis(v1, agent)
			}
			if appStudio != nil {
				installAppStudioApis(v1, appStudio)
			}
			if taskCenter != nil {
				installTaskCenterApis(v1, taskCenter)
				installCanvasApis(v1, workflowcanvassvc.New(storeIns, taskCenter, applicationPlatform))
			}
			if aiChat == nil {
				aiChat = aichatsvc.NewService(aichatsvc.Dependencies{Store: storeIns, ModelReader: legacyappsvc.NewLegacyService(storeIns)})
			}
			installAIChatApis(v1, aiChat)
			if applicationPlatform != nil {
				installApplicationPlatformApis(v1, applicationPlatform)
			}
		}
	}

	return g
}

func installAppStudioApis(rg *gin.RouterGroup, service *appstudiosvc.Service) {
	c := appstudioctrl.NewController(service)
	applicationRead := rg.Group("/studio-applications")
	applicationRead.Use(authmiddleware.RequireIdentityPermission("appstudio.application.read"))
	applicationRead.GET("", c.ListApplications)
	applicationRead.GET("/:studio_application_id", c.GetApplication)
	applicationManage := rg.Group("/studio-applications")
	applicationManage.Use(authmiddleware.RequireIdentityPermission("appstudio.application.manage"))
	applicationManage.POST("", c.CreateApplication)
	applicationManage.PATCH("/:studio_application_id", c.UpdateApplication)
	applicationManage.POST("/:studio_application_id/archive", c.ArchiveApplication)

	agentRead := rg.Group("/studio-applications/:studio_application_id/agent")
	agentRead.Use(authmiddleware.RequireIdentityPermission("appstudio.agent.read"))
	agentRead.GET("", c.GetAgentStatus)
	agentRead.GET("/invocations", c.ListAgentInvocations)
	agentRead.GET("/invocations/:agent_invocation_id", c.GetAgentInvocation)
	agentRead.GET("/invocations/:agent_invocation_id/events", c.StreamAgentInvocationEvents)
	agentOperate := rg.Group("/studio-applications/:studio_application_id/agent")
	agentOperate.Use(authmiddleware.RequireIdentityPermission("appstudio.agent.operate"))
	agentOperate.POST("/messages", c.SendAgentMessage)
	agentOperate.POST("/invocations/:agent_invocation_id/cancel", c.CancelAgentInvocation)
	agentOperate.POST("/suspend", c.SuspendAgent)
	agentOperate.POST("/resume", c.ResumeAgent)
	agentOperate.POST("/replace", c.ReplaceAgent)

	sourceRead := rg.Group("/studio-applications/:studio_application_id/source")
	sourceRead.Use(authmiddleware.RequireIdentityPermission("appstudio.source.read"))
	sourceRead.GET("", c.GetSource)
	sourceRead.GET("/files", c.ListFiles)
	sourceRead.GET("/file-content", c.GetFileContent)
	sourceRead.GET("/search", c.SearchSource)
	sourceWrite := rg.Group("/studio-applications/:studio_application_id/source")
	sourceWrite.Use(authmiddleware.RequireIdentityPermission("appstudio.source.write"))
	sourceWrite.POST("/change-sets", c.ApplyChangeSet)
	sourceWrite.POST("/restore", c.RestoreRevision)

	snapshots := rg.Group("")
	snapshots.Use(authmiddleware.RequireIdentityPermission("appstudio.snapshot.manage"))
	snapshots.POST("/studio-applications/:studio_application_id/source-snapshots", c.CreateSnapshot)
	snapshots.GET("/studio-applications/:studio_application_id/source-snapshots/:source_snapshot_id", c.GetSnapshot)
	snapshots.POST("/studio-applications/:studio_application_id/versions", c.CreateVersion)
	snapshots.GET("/studio-applications/:studio_application_id/versions", c.ListVersions)

	builds := rg.Group("")
	builds.Use(authmiddleware.RequireIdentityPermission("appstudio.build.manage"))
	builds.GET("/studio-applications/:studio_application_id/builds", c.ListBuilds)
	builds.POST("/studio-applications/:studio_application_id/builds", c.CreateBuild)
	builds.POST("/studio-builds/batch-summaries", c.BatchBuildSummaries)
	builds.GET("/studio-builds/:studio_build_id", c.GetBuild)
	builds.POST("/studio-builds/:studio_build_id/cancel", c.CancelBuild)
	builds.GET("/studio-builds/:studio_build_id/logs", c.BuildLogs)

	preview := rg.Group("/studio-applications/:studio_application_id")
	preview.Use(authmiddleware.RequireIdentityPermission("appstudio.preview.operate"))
	preview.GET("/preview-runtime", c.GetPreview)
	preview.POST("/preview-checks", c.RefreshPreview)
	preview.POST("/preview-runtime/stop", c.StopPreview)

	runtimeConfigs := rg.Group("/studio-application-versions/:studio_application_version_id/runtime-configs/:environment")
	runtimeConfigs.Use(authmiddleware.RequireIdentityPermission("appstudio.runtime_config.manage"))
	runtimeConfigs.GET("", c.GetRuntimeConfig)
	runtimeConfigs.PUT("", c.ReplaceRuntimeConfig)

	releases := rg.Group("")
	releases.Use(authmiddleware.RequireIdentityPermission("appstudio.release.manage"))
	releases.GET("/studio-applications/:studio_application_id/releases", c.ListReleases)
	releases.POST("/studio-applications/:studio_application_id/releases", c.CreateRelease)
	releases.GET("/studio-releases/:studio_release_id", c.GetRelease)
	releases.POST("/studio-releases/:studio_release_id/rollback", c.RollbackRelease)
	releases.GET("/studio-applications/:studio_application_id/runtime-instances", c.ListRuntimeInstances)
	releases.GET("/studio-runtime-instances/:studio_runtime_instance_id", c.GetRuntimeInstance)
	releases.POST("/studio-runtime-instances/:studio_runtime_instance_id/stop", c.StopRuntimeInstance)
	releases.GET("/studio-runtime-instances/:studio_runtime_instance_id/logs", c.RuntimeLogs)
}

func installAgentApis(rg *gin.RouterGroup, service *agentsvc.Service) {
	controller := agentctrl.NewController(service)
	profiles := rg.Group("/agent-profiles")
	profiles.Use(authmiddleware.RequireIdentityPermission("agent.profile.read"))
	profiles.GET("", controller.ListProfiles)

	agentsRead := rg.Group("/agents")
	agentsRead.Use(authmiddleware.RequireIdentityPermission("agent.read"))
	agentsRead.GET("", controller.ListAgents)
	agentsRead.GET("/:agent_id", controller.GetAgent)

	agentsManage := rg.Group("/agents")
	agentsManage.Use(authmiddleware.RequireIdentityPermission("agent.manage"))
	agentsManage.POST("", controller.CreateAgent)
	agentsManage.PATCH("/:agent_id", controller.UpdateAgent)
	agentsManage.DELETE("/:agent_id", controller.DeleteAgent)
	agentsManage.POST("/:agent_id/enable", controller.EnableAgent)
	agentsManage.POST("/:agent_id/disable", controller.DisableAgent)
	agentsManage.GET("/:agent_id/model-bindings", controller.ListModelBindings)
	agentsManage.PUT("/:agent_id/model-bindings", controller.ReplaceModelBinding)
	agentsManage.GET("/:agent_id/skill-bindings", controller.ListSkillBindings)
	agentsManage.GET("/:agent_id/mcp-bindings", controller.ListMCPBindings)
	agentsManage.POST("/:agent_id/mcp-bindings", controller.CreateMCPBinding)

	runtimes := rg.Group("/agents/:agent_id/runtime")
	runtimes.Use(authmiddleware.RequireIdentityPermission("agent.runtime.operate"))
	runtimes.POST("/start", controller.StartRuntime)
	runtimes.POST("/suspend", controller.SuspendRuntime)
	runtimes.POST("/recover", controller.RecoverRuntime)
	runtimes.POST("/stop", controller.StopRuntime)

	sessionRead := rg.Group("")
	sessionRead.Use(authmiddleware.RequireIdentityPermission("agent.session.read"))
	sessionRead.GET("/agents/:agent_id/sessions", controller.ListSessions)
	sessionRead.GET("/agent-sessions/:session_id", controller.GetSession)
	sessionRead.GET("/agent-sessions/:session_id/messages", controller.ListMessages)

	sessionManage := rg.Group("")
	sessionManage.Use(authmiddleware.RequireIdentityPermission("agent.session.manage"))
	sessionManage.POST("/agents/:agent_id/sessions", controller.CreateSession)
	sessionManage.PATCH("/agent-sessions/:session_id", controller.UpdateSession)
	sessionManage.POST("/agent-sessions/:session_id/close", controller.CloseSession)
	sessionManage.POST("/agent-sessions/:session_id/archive", controller.ArchiveSession)

	invocations := rg.Group("")
	invocations.Use(authmiddleware.RequireIdentityPermission("agent.invoke"))
	invocations.POST("/agent-sessions/:session_id/messages", controller.SendMessage)
	invocations.GET("/agent-sessions/:session_id/operations", controller.ListInvocations)
	invocations.GET("/agent-invocations/:invocation_id", controller.GetInvocation)
	invocations.POST("/agent-invocations/:invocation_id/cancel", controller.CancelInvocation)
	invocations.GET("/agent-invocations/:invocation_id/events", controller.StreamInvocationEvents)

	memoryRead := rg.Group("")
	memoryRead.Use(authmiddleware.RequireIdentityPermission("agent.memory.read"))
	memoryRead.GET("/agents/:agent_id/memories", controller.ListMemories)
	memoryRead.GET("/agent-memories/:memory_id", controller.GetMemory)
	memoryManage := rg.Group("")
	memoryManage.Use(authmiddleware.RequireIdentityPermission("agent.memory.manage"))
	memoryManage.POST("/agents/:agent_id/memories", controller.CreateMemory)
	memoryManage.PATCH("/agent-memories/:memory_id", controller.UpdateMemory)
	memoryManage.DELETE("/agent-memories/:memory_id", controller.DeleteMemory)

}

// installIdentityPlatformApis 在统一 JWT、权限和审计链后安装已发布的 Identity 与 Platform Management API。
func installIdentityPlatformApis(rg *gin.RouterGroup, factory store.Factory, authOptions *options.AuthOptions) {
	if authOptions == nil {
		authOptions = options.NewAuthOptions()
	}
	if len(authmiddleware.IdentityJWTSecret(authOptions)) < 32 {
		return
	}
	identityStore := factory.Identities()
	platformStore := factory.PlatformManagement()
	if identityStore == nil || platformStore == nil {
		return
	}

	identityService := identitysvc.NewService(factory, authOptions.JWTSecret, authOptions.OpaqueServerSetup)
	identityController := identityctrl.NewController(identityService)

	iam := rg.Group("/iam")
	iam.Use(authmiddleware.Audit(platformStore, "identity"))
	iam.Use(authmiddleware.IdentityAuthentication(authOptions, identityStore))
	auth := iam.Group("/auth")
	{
		auth.POST("/register/start", identityController.RegisterStart)
		auth.POST("/register/finish", identityController.RegisterFinish)
		auth.POST("/login/start", identityController.LoginStart)
		auth.POST("/login/finish", identityController.LoginFinish)
		auth.POST("/refresh", identityController.Refresh)
		protected := auth.Group("")
		protected.Use(authmiddleware.RequireIdentityPermission("identity.auth.session"))
		protected.POST("/presence/heartbeat", identityController.Heartbeat)
		protected.POST("/logout", identityController.Logout)
		protected.POST("/logout-all", identityController.LogoutAll)
		protected.POST("/change-password/start", identityController.ChangePasswordStart)
		protected.POST("/change-password/finish", identityController.ChangePasswordFinish)
		protected.GET("/sessions", identityController.Sessions)
		protected.DELETE("/sessions/:session_id", identityController.RevokeSession)
		me := auth.Group("/me")
		me.Use(authmiddleware.RequireIdentityPermission("identity.user.read"))
		me.GET("", identityController.Me)
	}
	permissions := iam.Group("/auth/permissions")
	permissions.Use(authmiddleware.RequireIdentityPermission("identity.permission.read"))
	permissions.GET("", identityController.Permissions)
	permissionDefinitions := iam.Group("/admin/permissions")
	permissionDefinitions.Use(authmiddleware.RequireIdentityPermission("identity.permission.read"))
	permissionDefinitions.GET("", identityController.ListPermissionDefinitions)
	users := iam.Group("/admin/users")
	users.Use(authmiddleware.RequireIdentityPermission("identity.user.manage"))
	users.GET("", identityController.ListUsers)
	users.POST("/register/start", identityController.AdminRegisterStart)
	users.POST("/register/finish", identityController.AdminRegisterFinish)
	users.GET("/:user_id", identityController.GetUser)
	users.PUT("/:user_id", identityController.UpdateUser)
	users.DELETE("/:user_id", identityController.DeleteUser)
	users.POST("/:user_id/disable", identityController.DisableUser)
	users.POST("/:user_id/enable", identityController.EnableUser)
	users.POST("/:user_id/unlock", identityController.UnlockUser)
	users.POST("/:user_id/reset-initial-password/start", identityController.AdminResetInitialPasswordStart)
	users.POST("/:user_id/reset-initial-password/finish", identityController.AdminResetInitialPasswordFinish)
	registrations := iam.Group("/admin/registration-applications")
	registrations.Use(authmiddleware.RequireIdentityPermission("identity.registration.review"))
	registrations.GET("", identityController.ListRegistrationApplications)
	registrations.GET("/:registration_application_id", identityController.GetRegistrationApplication)
	registrations.POST("/:registration_application_id/approve", identityController.ApproveRegistrationApplication)
	registrations.POST("/:registration_application_id/reject", identityController.RejectRegistrationApplication)
	roles := iam.Group("/admin/roles")
	roles.Use(authmiddleware.RequireIdentityPermission("identity.role.manage"))
	roles.GET("", identityController.ListRoles)
	roles.POST("", identityController.CreateRole)
	roles.GET("/:role_id", identityController.GetRole)
	roles.PUT("/:role_id", identityController.UpdateRole)
	roles.PUT("/:role_id/permissions", identityController.ReplaceRolePermissions)
	groups := iam.Group("/admin/groups")
	groups.Use(authmiddleware.RequireIdentityPermission("identity.group.manage"))
	groups.GET("", identityController.ListGroups)
	groups.POST("", identityController.CreateGroup)
	groups.GET("/:group_id", identityController.GetGroup)
	groups.PUT("/:group_id", identityController.UpdateGroup)
	groups.PUT("/:group_id/members", identityController.ReplaceGroupMembers)
	groups.PUT("/:group_id/roles", identityController.ReplaceGroupRoles)
	grants := iam.Group("/resources/:resource_type/:resource_id/grants")
	grants.GET("", authmiddleware.RequireIdentityPermission("identity.resource_grant.read"), identityController.ListResourceGrants)
	grants.POST("", authmiddleware.RequireIdentityPermission("identity.resource_grant.manage"), identityController.CreateResourceGrant)
	grants.PATCH("/:grant_id", authmiddleware.RequireIdentityPermission("identity.resource_grant.manage"), identityController.UpdateResourceGrant)
	grants.DELETE("/:grant_id", authmiddleware.RequireIdentityPermission("identity.resource_grant.manage"), identityController.RevokeResourceGrant)
	serviceAccounts := iam.Group("/admin/service-accounts")
	serviceAccounts.Use(authmiddleware.RequireIdentityPermission("identity.service_account.read"))
	serviceAccounts.GET("", identityController.ListServiceAccounts)
	serviceAccounts.GET("/:service_account_id", identityController.GetServiceAccount)
	serviceAccounts.POST("/:service_account_id/disable", authmiddleware.RequireIdentityPermission("identity.service_account.manage"), identityController.DisableServiceAccount)
	serviceAccounts.POST("/:service_account_id/enable", authmiddleware.RequireIdentityPermission("identity.service_account.manage"), identityController.EnableServiceAccount)
	serviceAccounts.POST("/:service_account_id/rotate-credential", authmiddleware.RequireIdentityPermission("identity.service_account.manage"), identityController.RotateServiceAccountCredential)
	serviceAccounts.POST("", authmiddleware.RequireIdentityPermission("identity.service_account.manage"), identityController.CreateServiceAccount)
	serviceAccounts.PUT("/:service_account_id", authmiddleware.RequireIdentityPermission("identity.service_account.manage"), identityController.UpdateServiceAccount)

	platformService := platformmanagementsvc.NewService(platformStore)
	platformController := platformmanagementctrl.NewController(platformService)
	platform := rg.Group("/platform")
	platform.Use(authmiddleware.Audit(platformStore, "platform-management"))
	platform.Use(authmiddleware.IdentityAuthentication(authOptions, identityStore))
	platform.GET("/overview", authmiddleware.RequireIdentityPermission("platform.overview.read"), platformController.Overview)
	platform.GET("/auth-config", authmiddleware.RequireIdentityPermission("platform.auth_config.read"), platformController.GetAuthConfig)
	platform.PUT("/auth-config", authmiddleware.RequireIdentityPermission("platform.auth_config.manage"), platformController.ReplaceAuthConfig)
	platform.GET("/audit-logs", authmiddleware.RequireIdentityPermission("platform.audit.read"), platformController.ListAudit)
	platform.GET("/audit-logs/:audit_log_id", authmiddleware.RequireIdentityPermission("platform.audit.read"), platformController.GetAudit)
	internal := platform.Group("/internal")
	internal.GET("/system-auth-config", authmiddleware.RequireIdentityPermission("platform.auth_config.read_internal"), authmiddleware.RequireServicePrincipal(), platformController.GetAuthConfig)
	internal.Use(authmiddleware.RequireIdentityPermission("platform.audit.record"))
	internal.Use(authmiddleware.RequireServicePrincipal())
	internal.POST("/audit-records", platformController.AppendAudit)
}

func installNotificationApis(rg *gin.RouterGroup, factory store.Factory) {
	notificationFactory, ok := factory.(store.NotificationStoreFactory)
	if !ok || notificationFactory.Notifications() == nil {
		return
	}
	controller := notificationctrl.NewController(notificationsvc.New(notificationFactory.Notifications()))
	notifications := rg.Group("/notifications")
	{
		notifications.GET("", controller.List)
		notifications.GET("/unread-count", controller.UnreadCount)
		notifications.POST("/read", controller.BatchRead)
		notifications.POST("/read-all", controller.ReadAll)
		notifications.POST("/:notification_id/read", controller.Read)
		notifications.POST("/:notification_id/unread", controller.Unread)
		notifications.POST("/:notification_id/archive", controller.Archive)
		notifications.POST("/:notification_id/unarchive", controller.Unarchive)
	}
	rg.GET("/notification-preferences", controller.GetPreferences)
	rg.PUT("/notification-preferences", controller.PutPreferences)
}

func installSSEApis(rg *gin.RouterGroup, storeIns store.Factory, config *options.SSEOptions) {
	if storeIns.UserEvents() == nil {
		return
	}
	controller := ssectrl.NewController(storeIns, config)
	rg.GET("/events/stream", controller.StreamEvents)
	rg.GET("/events", controller.ListEvents)
	rg.GET("/events/sync-state", controller.GetSyncState)
}

func installApplicationPlatformApis(rg *gin.RouterGroup, service appplatformsvc.ApplicationPlatformSrv) {
	controller := aiappctrl.NewController(service)
	providerCapabilities := rg.Group("/provider-capabilities")
	{
		providerCapabilities.GET("", controller.ListProviderCapabilities)
		providerCapabilities.GET("/:provider_capability_id", controller.GetProviderCapability)
	}
	rg.GET("/application-engine-types", controller.ListApplicationEngineTypes)

	engines := rg.Group("/engine-instances")
	{
		engines.GET("", controller.ListEngineInstances)
		engines.POST("", controller.CreateEngineInstance)
		engines.GET("/:engine_instance_id", controller.GetEngineInstance)
		engines.PATCH("/:engine_instance_id", controller.UpdateEngineInstance)
		engines.DELETE("/:engine_instance_id", controller.DeleteEngineInstance)
		engines.POST("/:engine_instance_id/health-check", controller.CheckEngineInstanceHealth)
		engines.GET("/:engine_instance_id/object-info", controller.GetComfyUIEngineObjectInfo)
		engines.POST("/:engine_instance_id/object-info/refresh", controller.RefreshComfyUIEngineObjectInfo)
	}

	bindings := rg.Group("/engine-capability-bindings")
	{
		bindings.GET("", controller.ListEngineBindings)
		bindings.POST("", controller.CreateEngineBinding)
		bindings.PATCH("/:binding_id", controller.UpdateEngineBinding)
		bindings.DELETE("/:binding_id", controller.DeleteEngineBinding)
	}

	workflows := rg.Group("/comfyui-workflows")
	{
		workflows.GET("", authmiddleware.RequireIdentityPermission("aiapp.comfyui_workflow.read"), controller.ListComfyUIWorkflows)
		workflows.POST("", authmiddleware.RequireIdentityPermission("aiapp.comfyui_workflow.manage"), controller.ImportComfyUIWorkflow)
		workflows.GET("/:workflow_id", authmiddleware.RequireIdentityPermission("aiapp.comfyui_workflow.read"), controller.GetComfyUIWorkflow)
		workflows.PATCH("/:workflow_id", authmiddleware.RequireIdentityPermission("aiapp.comfyui_workflow.manage"), controller.UpdateComfyUIWorkflow)
		workflows.GET("/:workflow_id/nodes", authmiddleware.RequireIdentityPermission("aiapp.comfyui_workflow.read"), controller.ListComfyUIWorkflowNodes)
		workflows.GET("/:workflow_id/input-candidates", authmiddleware.RequireIdentityPermission("aiapp.comfyui_workflow.read"), controller.ListComfyUIWorkflowInputCandidates)
		workflows.GET("/:workflow_id/output-candidates", authmiddleware.RequireIdentityPermission("aiapp.comfyui_workflow.read"), controller.ListComfyUIWorkflowOutputCandidates)
		workflows.GET("/:workflow_id/dependencies", authmiddleware.RequireIdentityPermission("aiapp.comfyui_workflow.read"), controller.ListComfyUIWorkflowDependencies)
		workflows.GET("/:workflow_id/validations", authmiddleware.RequireIdentityPermission("aiapp.comfyui_workflow.read"), controller.ListComfyUIWorkflowValidations)
		workflows.POST("/:workflow_id/validations", authmiddleware.RequireIdentityPermission("aiapp.comfyui_workflow.validate"), controller.ValidateComfyUIWorkflow)
		workflows.POST("/:workflow_id/convert-to-application-template", authmiddleware.RequireIdentityPermission("aiapp.comfyui_workflow.convert"), controller.ConvertComfyUIWorkflow)
		workflows.POST("/:workflow_id/convert-to-api-workflow", authmiddleware.RequireIdentityPermission("aiapp.comfyui_workflow.manage"), controller.ConvertComfyUIWorkflowToAPI)
		workflows.GET("/:workflow_id/test-runs", authmiddleware.RequireIdentityPermission("aiapp.comfyui_workflow.test"), controller.ListComfyUIWorkflowTestRuns)
		workflows.POST("/:workflow_id/test-runs", authmiddleware.RequireIdentityPermission("aiapp.comfyui_workflow.test"), controller.CreateComfyUIWorkflowTestRun)
	}
	rg.GET("/comfyui-workflow-validations/:workflow_validation_id", authmiddleware.RequireIdentityPermission("aiapp.comfyui_workflow.read"), controller.GetComfyUIWorkflowValidation)
	rg.GET("/comfyui-workflow-test-runs/:test_run_id", authmiddleware.RequireIdentityPermission("aiapp.comfyui_workflow.test"), controller.GetComfyUIWorkflowTestRun)
	rg.POST("/comfyui-workflow-test-runs/:test_run_id/cancel", authmiddleware.RequireIdentityPermission("aiapp.comfyui_workflow.test"), controller.CancelComfyUIWorkflowTestRun)
	rg.GET("/comfyui-workflow-test-runs/:test_run_id/outputs/:output_id/content", authmiddleware.RequireIdentityPermission("aiapp.comfyui_workflow.test"), controller.GetComfyUIWorkflowTestOutputContent)

	templates := rg.Group("/application-templates")
	{
		templates.GET("", authmiddleware.RequireIdentityPermission("aiapp.application.manage"), controller.ListTemplates)
		templates.POST("", authmiddleware.RequireIdentityPermission("aiapp.application.manage"), controller.CreateTemplate)
		templates.GET("/:application_template_id", authmiddleware.RequireIdentityPermission("aiapp.application.manage"), controller.GetTemplate)
		templates.GET("/:application_template_id/versions", authmiddleware.RequireIdentityPermission("aiapp.application.manage"), controller.ListTemplateVersions)
		templates.POST("/:application_template_id/versions", authmiddleware.RequireIdentityPermission("aiapp.application.manage"), controller.CreateTemplateVersion)
	}
	rg.GET("/application-template-versions/:application_template_version_id", authmiddleware.RequireIdentityPermission("aiapp.application.manage"), controller.GetTemplateVersion)
	rg.POST("/application-template-versions/:application_template_version_id/publish", authmiddleware.RequireIdentityPermission("aiapp.application.manage"), controller.PublishTemplateVersion)

	applications := rg.Group("/applications")
	{
		applications.GET("", authmiddleware.RequireIdentityPermission("aiapp.application.read"), controller.ListApplications)
		applications.POST("", authmiddleware.RequireIdentityPermission("aiapp.application.manage"), controller.CreateApplication)
		applications.GET("/:application_id", authmiddleware.RequireIdentityPermission("aiapp.application.read"), controller.GetApplication)
		applications.PATCH("/:application_id", authmiddleware.RequireIdentityPermission("aiapp.application.manage"), controller.UpdateApplication)
		applications.GET("/:application_id/versions", authmiddleware.RequireIdentityPermission("aiapp.application.read"), controller.ListApplicationVersions)
		applications.POST("/:application_id/versions", authmiddleware.RequireIdentityPermission("aiapp.application.manage"), controller.CreateApplicationVersion)
		applications.POST("/:application_id/runtime-form", authmiddleware.RequireIdentityPermission("aiapp.application.run"), controller.ResolveRuntimeForm)
		applications.GET("/:application_id/runs", authmiddleware.RequireIdentityPermission("aiapp.application.run"), controller.ListApplicationRuns)
		applications.POST("/:application_id/runs", authmiddleware.RequireIdentityPermission("aiapp.application.run"), controller.CreateApplicationRun)
	}
	rg.GET("/application-versions/:application_version_id", authmiddleware.RequireIdentityPermission("aiapp.application.read"), controller.GetApplicationVersion)
	rg.POST("/application-versions/:application_version_id/publish", authmiddleware.RequireIdentityPermission("aiapp.application.manage"), controller.PublishApplicationVersion)

	applicationRuns := rg.Group("/application-runs")
	{
		applicationRuns.GET("/:application_run_id", authmiddleware.RequireIdentityPermission("aiapp.application.run"), controller.GetApplicationRun)
	}
}

func installTaskCenterApis(rg *gin.RouterGroup, service taskcentersvc.TaskCenterSrv) {
	taskCenterController := taskcenterctrl.NewController(service)
	atomicTasks := rg.Group("/atomic-tasks")
	{
		atomicTasks.GET("", taskCenterController.ListAtomicTasks)
		atomicTasks.POST("", taskCenterController.CreateAtomicTask)
		atomicTasks.GET("/:atomic_task_id", taskCenterController.GetAtomicTask)
		atomicTasks.GET("/:atomic_task_id/attempts", taskCenterController.ListAtomicTaskAttempts)
		atomicTasks.GET("/:atomic_task_id/attempts/:task_attempt_id/logs", taskCenterController.ListAtomicTaskAttemptLogs)
		atomicTasks.GET("/:atomic_task_id/attempts/:task_attempt_id/logs/download", taskCenterController.DownloadAtomicTaskAttemptLogs)
		atomicTasks.POST("/:atomic_task_id/cancel", taskCenterController.CancelAtomicTask)
		atomicTasks.POST("/:atomic_task_id/retry", taskCenterController.RetryAtomicTask)
	}
	taskGroups := rg.Group("/task-groups")
	{
		taskGroups.GET("", taskCenterController.ListTaskGroups)
		taskGroups.POST("", taskCenterController.CreateTaskGroup)
		taskGroups.GET("/:task_group_id", taskCenterController.GetTaskGroup)
		taskGroups.GET("/:task_group_id/tasks", taskCenterController.ListTaskGroupTasks)
		taskGroups.POST("/:task_group_id/cancel", taskCenterController.CancelTaskGroup)
		taskGroups.POST("/:task_group_id/retry", taskCenterController.RetryTaskGroup)
	}
	dagGroups := rg.Group("/dag-task-groups")
	{
		dagGroups.GET("", taskCenterController.ListDAGTaskGroups)
		dagGroups.POST("", taskCenterController.CreateDAGTaskGroup)
		dagGroups.GET("/:dag_task_group_id", taskCenterController.GetDAGTaskGroup)
		dagGroups.GET("/:dag_task_group_id/tasks", taskCenterController.ListDAGTaskGroupTasks)
		dagGroups.GET("/:dag_task_group_id/events", taskCenterController.ListDAGTaskGroupEvents)
		dagGroups.GET("/:dag_task_group_id/timeline", taskCenterController.ListDAGTaskGroupTimeline)
		dagGroups.POST("/:dag_task_group_id/cancel", taskCenterController.CancelDAGTaskGroup)
		dagGroups.POST("/:dag_task_group_id/retry", taskCenterController.RetryDAGTaskGroup)
	}
	schedules := rg.Group("/task-schedules")
	{
		schedules.GET("", taskCenterController.ListTaskSchedules)
		schedules.POST("", taskCenterController.CreateTaskSchedule)
		schedules.GET("/:task_schedule_id", taskCenterController.GetTaskSchedule)
		schedules.PATCH("/:task_schedule_id", taskCenterController.UpdateTaskSchedule)
		schedules.DELETE("/:task_schedule_id", taskCenterController.DeleteTaskSchedule)
		schedules.POST("/:task_schedule_id/pause", taskCenterController.PauseTaskSchedule)
		schedules.POST("/:task_schedule_id/resume", taskCenterController.ResumeTaskSchedule)
		schedules.POST("/:task_schedule_id/run", taskCenterController.RunTaskSchedule)
		schedules.GET("/:task_schedule_id/executions", taskCenterController.ListScheduleExecutions)
		schedules.GET("/:task_schedule_id/reconcile-state", taskCenterController.GetScheduleReconcileState)
	}
}

func installAIChatApis(rg *gin.RouterGroup, service aichatsvc.AIChatSrv) {
	aiChatController := aichatctrl.NewController(service)
	aiChat := rg.Group("/ai-chat")
	{
		assistantRead := aiChat.Group("/assistants")
		assistantRead.Use(authmiddleware.RequireIdentityPermission("ai_chat.assistant.read"))
		assistantRead.GET("", aiChatController.ListAssistants)
		assistantManage := aiChat.Group("/assistants")
		assistantManage.Use(authmiddleware.RequireIdentityPermission("ai_chat.assistant.manage"))
		assistantManage.POST("", aiChatController.CreateAssistant)
		assistantManage.PATCH("/:assistant_id", aiChatController.UpdateAssistant)
		assistantManage.DELETE("/:assistant_id", aiChatController.DeleteAssistant)

		topicRead := aiChat.Group("/topics")
		topicRead.Use(authmiddleware.RequireIdentityPermission("ai_chat.topic.read"))
		topicRead.GET("", aiChatController.ListTopics)
		topicRead.GET("/:topic_id", aiChatController.GetTopic)
		topicManage := aiChat.Group("/topics")
		topicManage.Use(authmiddleware.RequireIdentityPermission("ai_chat.topic.manage"))
		topicManage.POST("", aiChatController.CreateTopic)
		topicManage.PATCH("/:topic_id", aiChatController.UpdateTopic)
		topicManage.DELETE("/:topic_id", aiChatController.DeleteTopic)
		aiChat.GET("/topics/:topic_id/messages", authmiddleware.RequireIdentityPermission("ai_chat.message.read"), aiChatController.ListMessages)
		aiChat.POST("/topics/:topic_id/messages", authmiddleware.RequireIdentityPermission("ai_chat.message.send"), aiChatController.CreateMessage)

		generation := aiChat.Group("/generations")
		generation.Use(authmiddleware.RequireIdentityPermission("ai_chat.generation.operate"))
		generation.POST("/:generation_id/stop", aiChatController.StopGeneration)
		generation.GET("/:generation_id/events", aiChatController.StreamGenerationEvents)
		messageGeneration := aiChat.Group("/messages")
		messageGeneration.Use(authmiddleware.RequireIdentityPermission("ai_chat.generation.operate"))
		messageGeneration.POST("/:message_id/regenerate", aiChatController.RegenerateMessage)
		messageGeneration.POST("/:message_id/edit-regenerate", aiChatController.EditRegenerateMessage)

		quickPhraseRead := aiChat.Group("/quick-phrases")
		quickPhraseRead.Use(authmiddleware.RequireIdentityPermission("ai_chat.quick_phrase.read"))
		quickPhraseRead.GET("", aiChatController.ListQuickPhrases)
		quickPhraseManage := aiChat.Group("/quick-phrases")
		quickPhraseManage.Use(authmiddleware.RequireIdentityPermission("ai_chat.quick_phrase.manage"))
		quickPhraseManage.POST("", aiChatController.CreateQuickPhrase)
		quickPhraseManage.PATCH("/:quick_phrase_id", aiChatController.UpdateQuickPhrase)
		quickPhraseManage.DELETE("/:quick_phrase_id", aiChatController.DeleteQuickPhrase)

		aiChat.POST("/translations", authmiddleware.RequireIdentityPermission("ai_chat.translation.create"), aiChatController.TranslateContent)
	}
}

func installPlatformApis(rg *gin.RouterGroup, storeIns store.Factory, dispatcher legacyappsvc.TaskDispatcher) {
	platformController := platformctrl.NewController(storeIns, dispatcher)

	rg.GET("/me", platformController.Me)

	assets := rg.Group("/assets")
	{
		assets.POST("/upload", platformController.UploadAsset)
		assets.POST("/uploads/chunks/init", platformController.InitAssetChunkUpload)
		assets.PUT("/uploads/chunks/:checksum/:index", platformController.UploadAssetChunk)
		assets.POST("/uploads/chunks/:checksum/complete", platformController.CompleteAssetChunkUpload)
		assets.DELETE("/uploads/chunks/:checksum", platformController.CancelAssetChunkUpload)
		assets.POST("/search", platformController.SearchAssets)
		assets.POST("/search/parse", platformController.ParseAssetSearch)
		assets.GET("/:asset_id/content", platformController.GetAssetContent)
		assets.GET("/:asset_id/thumbnail", platformController.GetAssetThumbnail)
	}
	rg.POST("/asset-groups", platformController.CreateAssetGroup)

	canvasAssets := rg.Group("/canvas-assets")
	{
		canvasAssets.POST("/download", platformController.DownloadCanvasAssets)
		canvasAssets.POST("/check", platformController.SearchAssets)
		canvasAssets.POST("/register-output", platformController.RegisterCanvasOutput)
	}

}

func installUserModelApis(rg *gin.RouterGroup, service *usermodelsvc.Service) {
	controller := usermodelctrl.New(service)
	userModel := rg.Group("/user-model")
	userModel.GET("/provider-types", authmiddleware.RequireIdentityPermission("MODEL_CONFIG_READ"), controller.ListProviderTypes)

	providers := userModel.Group("/providers")
	providers.GET("", authmiddleware.RequireIdentityPermission("MODEL_CONFIG_READ"), controller.ListProviders)
	providers.POST("", authmiddleware.RequireIdentityPermission("MODEL_CONFIG_WRITE"), controller.CreateProvider)
	providers.POST("/test", authmiddleware.RequireIdentityPermission("MODEL_HEALTH_TEST"), controller.TestUnsavedProvider)
	providers.GET("/:provider_id", authmiddleware.RequireIdentityPermission("MODEL_CONFIG_READ"), controller.GetProvider)
	providers.PATCH("/:provider_id", authmiddleware.RequireIdentityPermission("MODEL_CONFIG_WRITE"), controller.UpdateProvider)
	providers.DELETE("/:provider_id", authmiddleware.RequireIdentityPermission("MODEL_CONFIG_WRITE"), controller.DeleteProvider)
	providers.POST("/:provider_id/test", authmiddleware.RequireIdentityPermission("MODEL_HEALTH_TEST"), controller.TestProvider)
	providers.GET("/:provider_id/models", authmiddleware.RequireIdentityPermission("MODEL_CONFIG_READ"), controller.ListProviderModels)
	providers.POST("/:provider_id/models", authmiddleware.RequireIdentityPermission("MODEL_CONFIG_WRITE"), controller.CreateProviderModel)
	providers.POST("/:provider_id/models/sync", authmiddleware.RequireIdentityPermission("MODEL_CONFIG_WRITE"), controller.SyncProviderModels)

	models := userModel.Group("/models")
	models.PATCH("/:model_id", authmiddleware.RequireIdentityPermission("MODEL_CONFIG_WRITE"), controller.UpdateProviderModel)
	models.DELETE("/:model_id", authmiddleware.RequireIdentityPermission("MODEL_CONFIG_WRITE"), controller.DeleteProviderModel)
	models.POST("/:model_id/test", authmiddleware.RequireIdentityPermission("MODEL_HEALTH_TEST"), controller.TestProviderModel)

	defaults := userModel.Group("/defaults")
	defaults.GET("/:usage", authmiddleware.RequireIdentityPermission("MODEL_CONFIG_READ"), controller.GetDefaultModel)
	defaults.PUT("/:usage", authmiddleware.RequireIdentityPermission("MODEL_DEFAULT_WRITE"), controller.SaveDefaultModel)
	userModel.GET("/options", authmiddleware.RequireIdentityPermission("MODEL_CONFIG_READ"), controller.ListModelOptions)
}

func installAssetLibraryContractApis(
	rg *gin.RouterGroup,
	storeIns store.Factory,
	tasks taskcentersvc.TaskCenterSrv,
	applications appplatformsvc.ApplicationPlatformSrv,
) {
	assetStore := optionalAssetV1Store(storeIns)
	if assetStore == nil {
		return
	}
	service := newAssetLibraryService(storeIns, tasks, applications)
	if service == nil {
		return
	}
	controller := assetlibraryctrl.New(service)

	rg.GET("/blobs/:blob_id", controller.GetBlob)
	storageBackends := rg.Group("/storage-backends")
	{
		storageBackends.GET("", controller.ListStorageBackends)
		storageBackends.POST("", controller.CreateStorageBackend)
		storageBackends.GET("/:backend_id", controller.GetStorageBackend)
		storageBackends.PATCH("/:backend_id", controller.UpdateStorageBackend)
	}

	assets := rg.Group("/assets")
	{
		assets.GET("", controller.ListAssets)
		assets.POST("", controller.CreateAsset)
		assets.POST("/batch-delete", controller.BatchDeleteAssets)
		assets.POST("/batch-labels", controller.BatchLabels)
		assets.POST("/trash/empty", controller.EmptyTrash)
		assets.GET("/:asset_id", controller.GetAsset)
		assets.PATCH("/:asset_id", controller.UpdateAsset)
		assets.DELETE("/:asset_id", controller.DeleteAsset)
		assets.POST("/:asset_id/restore", controller.RestoreAsset)
		assets.DELETE("/:asset_id/permanent", controller.PermanentlyDeleteAsset)
		assets.GET("/:asset_id/versions", controller.ListVersions)
		assets.POST("/:asset_id/versions", controller.CreateVersion)
		assets.POST("/:asset_id/versions/:version_id/set-current", controller.SetCurrentVersion)
		assets.PUT("/:asset_id/labels", controller.ReplaceLabels)
		assets.DELETE("/:asset_id/labels/:label_id", controller.DeleteLabel)
		assets.POST("/:asset_id/tags", controller.AddTags)
		assets.DELETE("/:asset_id/tags/:tag_id", controller.DeleteTag)
		assets.GET("/:asset_id/relations", controller.ListRelations)
		assets.GET("/:asset_id/lineage", controller.Lineage)
		assets.GET("/:asset_id/references", controller.ListReferences)
		assets.GET("/:asset_id/usages", controller.ListUsages)
	}

	uploads := rg.Group("/asset-uploads")
	{
		uploads.POST("", controller.CreateUploads)
		uploads.POST("/:upload_id/content", controller.UploadContent)
		uploads.POST("/:upload_id/complete", controller.CompleteUpload)
		uploads.DELETE("/:upload_id", controller.CancelUpload)
	}

	collections := rg.Group("/collections")
	{
		collections.GET("", controller.ListCollections)
		collections.POST("", controller.CreateCollection)
		collections.GET("/:collection_id", controller.GetCollection)
		collections.PATCH("/:collection_id", controller.UpdateCollection)
		collections.DELETE("/:collection_id", controller.DeleteCollection)
		collections.POST("/:collection_id/items", controller.AddCollectionItems)
		collections.PATCH("/:collection_id/items/:item_id", controller.UpdateCollectionItem)
		collections.DELETE("/:collection_id/items/:item_id", controller.DeleteCollectionItem)
	}

	artifacts := rg.Group("/artifacts")
	{
		artifacts.GET("", controller.ListArtifacts)
		artifacts.POST("", controller.CreateArtifact)
		artifacts.POST("/batch-summaries", controller.BatchArtifactSummaries)
		artifacts.GET("/:artifact_id", controller.GetArtifact)
		artifacts.DELETE("/:artifact_id", controller.DeleteArtifact)
		artifacts.POST("/:artifact_id/content", controller.UploadArtifactContent)
		artifacts.POST("/:artifact_id/complete", controller.CompleteArtifact)
		artifacts.POST("/:artifact_id/register", controller.RegisterArtifact)
	}
	rg.POST("/artifact-registrations", controller.RegisterArtifactCompat)

	rg.GET("/asset-versions/:version_id", controller.GetVersion)
	representations := rg.Group("/asset-versions/:version_id/representations")
	{
		representations.GET("", controller.ListRepresentations)
		representations.POST("", controller.RegisterRepresentation)
	}
	rg.GET("/asset-representations/:representation_id", controller.GetRepresentation)
	rg.GET("/asset-representations/:representation_id/content", controller.ReadRepresentation)
	rg.GET("/asset-representations/:representation_id/access-url", controller.RepresentationAccess)
}

func newAssetLibraryService(
	storeIns store.Factory,
	tasks taskcentersvc.TaskCenterSrv,
	applications appplatformsvc.ApplicationPlatformSrv,
) assetlibrarysvc.Service {
	assetStore := optionalAssetV1Store(storeIns)
	if assetStore == nil {
		return nil
	}
	if tasks == nil {
		tasks = taskcentersvc.NewService(storeIns)
	}
	return assetlibrarysvc.NewStoreWithRelationsAndPolicy(assetStore, assetlibrarysvc.NewLocalContentStorage(storeIns), assetlibrarysvc.RelationReaders{
		AtomicTasks:     tasks,
		ApplicationRuns: appplatformsvc.NewRunSummaryReader(storeIns.ApplicationPlatforms()),
		CanvasRuns:      workflowcanvassvc.New(storeIns, tasks, applications),
	}, assetlibrarysvc.DefaultRepresentationPolicy{}, assetlibrarysvc.WithStorageInspection(
		storeIns.StorageBackends(),
		assetlibrarysvc.NewIdentityStorageAdminAuthorizer(storeIns.Identities()),
	))
}

func optionalAssetV1Store(factory store.Factory) (assetStore store.AssetV1Store) {
	defer func() {
		if recover() != nil {
			assetStore = nil
		}
	}()
	return factory.AssetsV1()
}

func installAssetApis(rg *gin.RouterGroup, storeIns store.Factory) {
	assetController := asset.NewController(storeIns)

	assetv1 := rg.Group("/asset-library")
	{
		assetv1.GET("/libraries", assetController.ListLibraries)
		assetv1.POST("/libraries", assetController.CreateLibrary)
		assetv1.PATCH("/libraries/:library_id", assetController.UpdateLibrary)
		assetv1.DELETE("/libraries/:library_id", assetController.DeleteLibrary)

		assetv1.GET("/categories", assetController.ListCategories)
		assetv1.POST("/categories", assetController.CreateCategory)
		assetv1.PATCH("/categories/:category_id", assetController.UpdateCategory)
		assetv1.DELETE("/categories/:category_id", assetController.DeleteCategory)

		assetv1.GET("/items", assetController.ListItems)
		assetv1.POST("/items", assetController.CreateItem)
		assetv1.POST("/items/batch", assetController.BatchCreateItems)
		assetv1.PATCH("/items/:item_id", assetController.UpdateItem)
		assetv1.DELETE("/items/:item_id", assetController.DeleteItem)
		assetv1.POST("/items/delete", assetController.BatchDeleteItems)
		assetv1.POST("/items/move", assetController.BatchMoveItems)
		assetv1.POST("/items/classify", assetController.ClassifyItems)
	}
}

func installPromptApis(rg *gin.RouterGroup, storeIns store.Factory) {
	promptController := prompt.NewController(storeIns)

	promptv1 := rg.Group("/prompt-libraries")
	{
		promptv1.GET("", promptController.ListLibraries)
		promptv1.POST("", promptController.CreateLibrary)
		promptv1.PATCH("/:library_id", promptController.UpdateLibrary)
		promptv1.DELETE("/:library_id", promptController.DeleteLibrary)

		promptv1.POST("/items", promptController.CreateItem)
		promptv1.PATCH("/items/:item_id", promptController.UpdateItem)
		promptv1.DELETE("/items/:item_id", promptController.DeleteItem)
		promptv1.POST("/items/delete", promptController.BatchDeleteItems)

		promptv1.POST("/categories", promptController.CreateCategory)
		promptv1.PATCH("/categories/:category_id", promptController.UpdateCategory)
		promptv1.DELETE("/categories/:category_id", promptController.DeleteCategory)
	}
}

func installCanvasApis(rg *gin.RouterGroup, service workflowcanvassvc.Service) {
	canvasController := workflowcanvasctrl.NewController(service)
	definitions := rg.Group("/node-definitions")
	{
		definitions.GET("", canvasController.ListNodeDefinitions)
		definitions.POST("", canvasController.RegisterNodeDefinition)
		definitions.GET("/:node_type/versions/:definition_version", canvasController.GetNodeDefinition)
		definitions.POST("/:node_type/versions/:definition_version/deprecate", canvasController.DeprecateNodeDefinition)
	}
	canvasv1 := rg.Group("/canvases")
	{
		canvasv1.GET("", canvasController.List)
		canvasv1.POST("", canvasController.Create)
		canvasv1.GET("/:canvas_id", canvasController.Get)
		canvasv1.PATCH("/:canvas_id", canvasController.Update)
		canvasv1.DELETE("/:canvas_id", canvasController.Delete)
		canvasv1.POST("/:canvas_id/validate", canvasController.ValidateDraft)
		canvasv1.POST("/:canvas_id/publish", canvasController.Publish)
		canvasv1.GET("/:canvas_id/versions", canvasController.ListVersions)
	}
	rg.GET("/canvas-versions/:canvas_version_id", canvasController.GetVersion)
	rg.POST("/canvas-versions/:canvas_version_id/validate-run", canvasController.ValidateRun)
	runs := rg.Group("/canvas-runs")
	{
		runs.GET("", canvasController.ListRuns)
		runs.POST("", canvasController.CreateRun)
		runs.GET("/:canvas_run_id", canvasController.GetRun)
		runs.GET("/:canvas_run_id/flows", canvasController.ListFlowRuns)
		runs.GET("/:canvas_run_id/nodes", canvasController.ListNodeRuns)
		runs.POST("/:canvas_run_id/cancel", canvasController.CancelRun)
		runs.POST("/:canvas_run_id/retry", canvasController.RetryRun)
	}
	rg.GET("/canvas-node-runs/:canvas_node_run_id", canvasController.GetNodeRun)
}
