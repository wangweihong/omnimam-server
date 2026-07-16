package apiserver

import (
	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	aichatctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/aichat"
	aiappctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/asset"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/authentication"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/canvas"
	platformctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/platform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/prompt"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/setting"
	taskcenterctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/taskcenter"
	authmiddleware "github.com/wangweihong/omnimam/backend/internal/apiserver/middleware"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/options"
	appplatformsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	platformsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/platform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/pkg/core"
	"github.com/wangweihong/omnimam/backend/pkg/httpsvr/genericmiddleware"
)

func initRouter(g *gin.Engine, applicationPlatform appplatformsvc.ApplicationPlatformSrv, dispatcher platformsvc.TaskDispatcher, authOptions *options.AuthOptions, mode string) {
	InstallMiddleware(g)
	installApis(g, applicationPlatform, dispatcher, authOptions, mode)
}

func InstallMiddleware(g *gin.Engine) {
	g.Use(genericmiddleware.RequestID())
	g.Use(genericmiddleware.Context())
	g.Use(genericmiddleware.LoggerMiddleware())
}

func InstallApis(
	g *gin.Engine,
	applicationPlatform appplatformsvc.ApplicationPlatformSrv,
	dispatcher platformsvc.TaskDispatcher,
) *gin.Engine {
	return installApis(g, applicationPlatform, dispatcher, options.NewAuthOptions(), "release")
}

func installApis(
	g *gin.Engine,
	applicationPlatform appplatformsvc.ApplicationPlatformSrv,
	dispatcher platformsvc.TaskDispatcher,
	authOptions *options.AuthOptions,
	mode string,
) *gin.Engine {
	g.NoRoute(func(c *gin.Context) {
		core.WriteResponse(c, errors.NewStatusF(code.ErrPageNotFound, "Page not found."), nil)
	})
	storeIns := store.Client()
	if storeIns != nil {
		v1 := g.Group("/api/v1")
		{
			v1.Use(authmiddleware.Authentication(authOptions, mode, storeIns.Users()))
			installPlatformApis(v1, storeIns, dispatcher)
			installAuthApis(v1, storeIns)
			InstallSettingApis(v1, storeIns)
			installAssetApis(v1, storeIns)
			installPromptApis(v1, storeIns)
			installCanvasApis(v1, storeIns)
			installTaskCenterApis(v1, storeIns)
			installAIChatApis(v1, storeIns)
			if applicationPlatform != nil {
				installApplicationPlatformApis(v1, applicationPlatform)
			}
		}
	}

	return g
}

func installApplicationPlatformApis(rg *gin.RouterGroup, service appplatformsvc.ApplicationPlatformSrv) {
	controller := aiappctrl.NewController(service)
	providerCapabilities := rg.Group("/provider-capabilities")
	{
		providerCapabilities.GET("", controller.ListProviderCapabilities)
		providerCapabilities.GET("/:provider_capability_id", controller.GetProviderCapability)
	}
	rg.GET("/provider-capability-load-results", controller.ListProviderCapabilityLoadResults)
	rg.GET("/application-engine-types", controller.ListApplicationEngineTypes)

	engines := rg.Group("/engine-instances")
	{
		engines.GET("", controller.ListEngineInstances)
		engines.POST("", controller.CreateEngineInstance)
		engines.GET("/:engine_instance_id", controller.GetEngineInstance)
		engines.PATCH("/:engine_instance_id", controller.UpdateEngineInstance)
		engines.DELETE("/:engine_instance_id", controller.DeleteEngineInstance)
		engines.POST("/:engine_instance_id/health-check", controller.CheckEngineInstanceHealth)
	}

	bindings := rg.Group("/engine-capability-bindings")
	{
		bindings.GET("", controller.ListEngineBindings)
		bindings.POST("", controller.CreateEngineBinding)
		bindings.PATCH("/:binding_id", controller.UpdateEngineBinding)
		bindings.DELETE("/:binding_id", controller.DeleteEngineBinding)
	}

	templates := rg.Group("/application-templates")
	{
		templates.GET("", controller.ListTemplates)
		templates.POST("", controller.CreateTemplate)
		templates.GET("/:application_template_id", controller.GetTemplate)
		templates.GET("/:application_template_id/versions", controller.ListTemplateVersions)
		templates.POST("/:application_template_id/versions", controller.CreateTemplateVersion)
	}
	rg.GET("/application-template-versions/:application_template_version_id", controller.GetTemplateVersion)
	rg.POST("/application-template-versions/:application_template_version_id/publish", controller.PublishTemplateVersion)

	applications := rg.Group("/applications")
	{
		applications.GET("", controller.ListApplications)
		applications.POST("", controller.CreateApplication)
		applications.GET("/:application_id", controller.GetApplication)
		applications.PATCH("/:application_id", controller.UpdateApplication)
		applications.GET("/:application_id/versions", controller.ListApplicationVersions)
		applications.POST("/:application_id/versions", controller.CreateApplicationVersion)
		applications.POST("/:application_id/runtime-form", controller.ResolveRuntimeForm)
		applications.POST("/:application_id/runs", controller.CreateApplicationRun)
	}
	rg.GET("/application-versions/:application_version_id", controller.GetApplicationVersion)
	rg.POST("/application-versions/:application_version_id/publish", controller.PublishApplicationVersion)

	applicationRuns := rg.Group("/application-runs")
	{
		applicationRuns.GET("/:application_run_id", controller.GetApplicationRun)
	}
}

func installTaskCenterApis(rg *gin.RouterGroup, storeIns store.Factory) {
	taskCenterController := taskcenterctrl.NewController(storeIns)
	rg.GET("/task-definitions", taskCenterController.ListTaskDefinitions)
	rg.POST("/atomic-tasks", taskCenterController.CreateAtomicTask)
	rg.POST("/task-groups", taskCenterController.CreateTaskGroup)
	rg.POST("/dag-flow-tasks", taskCenterController.CreateDAGFlowTask)

	taskRuns := rg.Group("/task-runs")
	{
		taskRuns.GET("", taskCenterController.ListTaskRuns)
		taskRuns.POST("", taskCenterController.CreateTaskRun)
		taskRuns.GET("/:run_id", taskCenterController.GetTaskRun)
		taskRuns.DELETE("/:run_id", taskCenterController.DeleteTaskRun)
		taskRuns.GET("/:run_id/attempts", taskCenterController.ListTaskAttempts)
		taskRuns.POST("/:run_id/cancel", taskCenterController.CancelTaskRun)
		taskRuns.POST("/:run_id/retry", taskCenterController.RetryTaskRun)
		taskRuns.POST("/:run_id/progress", taskCenterController.UpdateTaskRunProgress)
		taskRuns.POST("/:run_id/complete", taskCenterController.CompleteTaskRun)
		taskRuns.POST("/:run_id/fail", taskCenterController.FailTaskRun)
	}

	workers := rg.Group("/workers")
	{
		workers.POST("", taskCenterController.RegisterWorker)
		workers.POST("/:worker_id/heartbeat", taskCenterController.HeartbeatWorker)
		workers.POST("/:worker_id/claim", taskCenterController.ClaimTaskRun)
	}

	rg.POST("/leases/:lease_id/renew", taskCenterController.RenewExecutionLease)
	rg.GET("/task-center/health", taskCenterController.GetTaskCenterHealth)
}

func installAIChatApis(rg *gin.RouterGroup, storeIns store.Factory) {
	aiChatController := aichatctrl.NewController(storeIns)
	aiChat := rg.Group("/ai-chat")
	{
		aiChat.GET("/assistants", aiChatController.ListAssistants)
		aiChat.POST("/assistants", aiChatController.CreateAssistant)
		aiChat.PATCH("/assistants/:assistant_id", aiChatController.UpdateAssistant)
		aiChat.DELETE("/assistants/:assistant_id", aiChatController.DeleteAssistant)

		aiChat.GET("/topics", aiChatController.ListTopics)
		aiChat.POST("/topics", aiChatController.CreateTopic)
		aiChat.GET("/topics/:topic_id", aiChatController.GetTopic)
		aiChat.PATCH("/topics/:topic_id", aiChatController.UpdateTopic)
		aiChat.DELETE("/topics/:topic_id", aiChatController.DeleteTopic)
		aiChat.GET("/topics/:topic_id/messages", aiChatController.ListMessages)
		aiChat.POST("/topics/:topic_id/messages", aiChatController.CreateMessage)

		aiChat.POST("/generations/:generation_id/stop", aiChatController.StopGeneration)
		aiChat.GET("/generations/:generation_id/events", aiChatController.StreamGenerationEvents)
		aiChat.POST("/messages/:message_id/regenerate", aiChatController.RegenerateMessage)
		aiChat.POST("/messages/:message_id/edit-regenerate", aiChatController.EditRegenerateMessage)

		aiChat.GET("/quick-phrases", aiChatController.ListQuickPhrases)
		aiChat.POST("/quick-phrases", aiChatController.CreateQuickPhrase)
		aiChat.PATCH("/quick-phrases/:quick_phrase_id", aiChatController.UpdateQuickPhrase)
		aiChat.DELETE("/quick-phrases/:quick_phrase_id", aiChatController.DeleteQuickPhrase)
		aiChat.POST("/translations", aiChatController.TranslateContent)
	}
}

func installPlatformApis(rg *gin.RouterGroup, storeIns store.Factory, dispatcher platformsvc.TaskDispatcher) {
	platformController := platformctrl.NewController(storeIns, dispatcher)

	rg.GET("/me", platformController.Me)

	// 模型提供商
	providers := rg.Group("/model-providers")
	{
		providers.GET("", platformController.ListProviders)
		providers.POST("", platformController.CreateProvider)
		providers.POST("/test", platformController.TestUnsavedProvider)
		providers.GET("/:provider_id", platformController.GetProvider)
		providers.PATCH("/:provider_id", platformController.UpdateProvider)
		providers.DELETE("/:provider_id", platformController.DeleteProvider)
		providers.POST("/:provider_id/test", platformController.TestProvider)
		providers.GET("/:provider_id/models", platformController.ListProviderModels)
		providers.POST("/:provider_id/models", platformController.CreateProviderModel)
		providers.POST("/:provider_id/models/sync", platformController.SyncProviderModels)
	}

	rg.PATCH("/provider-models/:model_id", platformController.UpdateProviderModel)
	rg.DELETE("/provider-models/:model_id", platformController.DeleteProviderModel)
	rg.POST("/provider-models/:model_id/test", platformController.CheckProviderModelHealth)
	rg.GET("/default-models/:usage", platformController.GetDefaultModel)
	rg.PUT("/default-models/:usage", platformController.PutDefaultModel)
	rg.GET("/model-options", platformController.ListModelOptions)

	storage := rg.Group("/storage-backends")
	{
		storage.GET("", platformController.ListStorageBackends)
		storage.POST("", platformController.CreateStorageBackend)
		storage.PATCH("/:backend_id", platformController.UpdateStorageBackend)
	}

	assets := rg.Group("/assets")
	{
		assets.GET("", platformController.ListAssets)
		assets.POST("/upload", platformController.UploadAsset)
		assets.POST("/uploads/chunks/init", platformController.InitAssetChunkUpload)
		assets.PUT("/uploads/chunks/:checksum/:index", platformController.UploadAssetChunk)
		assets.POST("/uploads/chunks/:checksum/complete", platformController.CompleteAssetChunkUpload)
		assets.DELETE("/uploads/chunks/:checksum", platformController.CancelAssetChunkUpload)
		assets.POST("/search", platformController.SearchAssets)
		assets.POST("/search/parse", platformController.ParseAssetSearch)
		assets.GET("/:asset_id", platformController.GetAsset)
		assets.PATCH("/:asset_id", platformController.UpdateAsset)
		assets.DELETE("/:asset_id", platformController.DeleteAsset)
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

	rg.POST("/canvases/:canvas_id/run", platformController.RunCanvas)
	rg.POST("/canvases/:canvas_id/nodes/:node_id/run", platformController.RunCanvasNode)
}

func installAuthApis(rg *gin.RouterGroup, storeIns store.Factory) {
	authv1 := rg.Group("/auth")
	{
		authController := authentication.NewController(storeIns)
		otp := authv1.Group("/otp")
		{
			otp.GET("qrcode", authController.OTPGenerateOrGet)
			otp.POST("validate", authController.OTPValidate)
		}
		// 修改以下路由需要同步修改iapiserver.SsoURL相关的常量
		sso := authv1.Group("/sso")
		{
			sp := sso.Group("/sp")
			{
				sp.GET("/saml/metadata", authController.SpSsoSamlInitiator)
				sp.POST("/saml/initiator", authController.SpSsoSamlInitiator)
				sp.POST("/saml/acs", authController.SpSsoSamlAcs)
				sp.POST("/saml/slo", authController.SpSsoSamlSLO)
				//oauth2
				// sp.POST("/oauth2/initiator", authController.SpSsoInitiator)
				// sp.POST("/oauth2/acs", authController.SpSsoInitiator)

			}

			idp := sso.Group("/idp")
			{
				// //saml
				idp.POST("/saml/answer", authController.IdpServeSAMLProtocolSSO)
				// sp.GET("/saml/metadata", authController.SpSsoInitiator)
				// //oauth2
				// idp.POST("/oauth2/answer", authController.SpSsoInitiator)
			}
		}
	}
}

func InstallSettingApis(rg *gin.RouterGroup, storeIns store.Factory) {
	settingv1 := rg.Group("/setting")
	{
		settingController := setting.NewController(storeIns)
		sso := settingv1.Group("/sso")
		{
			saml := sso.Group("/saml")
			{
				saml.POST("/idp/metadata/upsert", settingController.IdentityProviderSAMLMetadataUpsert)
				saml.GET("/idp/metadata/get", settingController.IdentityProviderSAMLMetadataGet)
				saml.GET("/idp/metadata/download", settingController.IdentityProviderSAMLMetadataDownload)

				saml.POST("/sp/metadata/upsert", settingController.ServiceProviderSAMLMetadataUpsert)
				saml.GET("/sp/metadata/get", settingController.ServiceProviderSAMLMetadataGet)
				saml.GET("/sp/metadata/download", settingController.ServiceProviderSAMLMetadataDownload)

			}

			ssoapp := sso.Group("/app")
			{
				ssoapp.POST("/idp/add", settingController.IdentityProviderAdd)
				ssoapp.POST("/idp/delete", settingController.IdentityProviderDelete)
				ssoapp.POST("/idp/update", settingController.IdentityProviderUpdate)
				ssoapp.GET("/idp/get", settingController.IdentityProviderGet)
				ssoapp.GET("/idp/list", settingController.IdentityProviderList)

				ssoapp.POST("/sp/add", settingController.ServiceProviderAdd)
				ssoapp.POST("/sp/delete", settingController.ServiceProviderDelete)
				ssoapp.POST("/sp/update", settingController.ServiceProviderUpdate)
				ssoapp.GET("/sp/get", settingController.ServiceProviderGet)
				ssoapp.GET("/sp/redirect_url", settingController.ServiceProviderRedirectURL)
				ssoapp.GET("/sp/list", settingController.ServiceProviderList)
			}
		}
	}
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

func installCanvasApis(rg *gin.RouterGroup, storeIns store.Factory) {
	canvasController := canvas.NewController(storeIns)

	canvasv1 := rg.Group("/canvases")
	{
		canvasv1.GET("", canvasController.ListCanvases)
		canvasv1.GET("/trash", canvasController.ListTrash)
		canvasv1.POST("", canvasController.CreateCanvas)
		canvasv1.POST("/import", canvasController.ImportCanvas)
		canvasv1.GET("/:canvas_id", canvasController.GetCanvas)
		canvasv1.GET("/:canvas_id/export", canvasController.ExportCanvas)
		canvasv1.PATCH("/:canvas_id", canvasController.UpdateCanvasMeta)
		canvasv1.GET("/:canvas_id/meta", canvasController.GetCanvasMeta)
		canvasv1.POST("/:canvas_id/meta", canvasController.UpdateCanvasMeta)
		canvasv1.PUT("/:canvas_id", canvasController.SaveCanvas)
		canvasv1.POST("/:canvas_id/workflows/export", canvasController.ExportWorkflow)
		canvasv1.POST("/:canvas_id/workflows/import", canvasController.ImportWorkflow)
		canvasv1.POST("/:canvas_id/workflows/export-package", canvasController.ExportWorkflowPackage)
		canvasv1.POST("/:canvas_id/workflows/import-package", canvasController.ImportWorkflowPackage)
		canvasv1.POST("/:canvas_id/touch", canvasController.TouchCanvas)
		canvasv1.DELETE("/:canvas_id", canvasController.DeleteCanvas)
		canvasv1.POST("/:canvas_id/restore", canvasController.RestoreCanvas)
		canvasv1.DELETE("/:canvas_id/purge", canvasController.PurgeCanvas)
	}

	projectv1 := rg.Group("/projects")
	{
		projectv1.GET("", canvasController.ListProjects)
		projectv1.POST("", canvasController.CreateProject)
		projectv1.POST("/:project_id", canvasController.UpdateProject)
		projectv1.DELETE("/:project_id", canvasController.DeleteProject)
	}
}
