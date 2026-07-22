package apiserver

import (
	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	aichatctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/aichat"
	aiappctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/asset"
	assetlibraryctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/assetlibrary"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/authentication"
	platformctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/platform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/prompt"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/setting"
	ssectrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/sse"
	taskcenterctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/taskcenter"
	workflowcanvasctrl "github.com/wangweihong/omnimam/backend/internal/apiserver/controller/v1/workflowcanvas"
	authmiddleware "github.com/wangweihong/omnimam/backend/internal/apiserver/middleware"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/options"
	appplatformsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	assetlibrarysvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/assetlibrary"
	platformsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/platform"
	taskcentersvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	workflowcanvassvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/workflowcanvas"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/pkg/core"
	"github.com/wangweihong/omnimam/backend/pkg/httpsvr/genericmiddleware"
)

func initRouter(
	g *gin.Engine,
	applicationPlatform appplatformsvc.ApplicationPlatformSrv,
	taskCenter taskcentersvc.TaskCenterSrv,
	authOptions *options.AuthOptions,
	sseOptions *options.SSEOptions,
	mode string,
) {
	InstallMiddleware(g)
	installApis(g, applicationPlatform, taskCenter, authOptions, sseOptions, mode)
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
	return installApis(g, applicationPlatform, taskCenter, options.NewAuthOptions(), options.NewSSEOptions(), "release")
}

func installApis(
	g *gin.Engine,
	applicationPlatform appplatformsvc.ApplicationPlatformSrv,
	taskCenter taskcentersvc.TaskCenterSrv,
	authOptions *options.AuthOptions,
	sseOptions *options.SSEOptions,
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
			installSSEApis(v1, storeIns, sseOptions)
			installPlatformApis(v1, storeIns, nil)
			installAssetLibraryContractApis(v1, storeIns)
			installAuthApis(v1, storeIns)
			InstallSettingApis(v1, storeIns)
			installAssetApis(v1, storeIns)
			installPromptApis(v1, storeIns)
			if taskCenter != nil {
				installTaskCenterApis(v1, taskCenter)
				installCanvasApis(v1, workflowcanvassvc.New(storeIns, taskCenter))
			}
			installAIChatApis(v1, storeIns)
			if applicationPlatform != nil {
				installApplicationPlatformApis(v1, applicationPlatform)
			}
		}
	}

	return g
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
		workflows.GET("", controller.ListComfyUIWorkflows)
		workflows.POST("", controller.ImportComfyUIWorkflow)
		workflows.GET("/:workflow_id", controller.GetComfyUIWorkflow)
		workflows.PATCH("/:workflow_id", controller.UpdateComfyUIWorkflow)
		workflows.GET("/:workflow_id/nodes", controller.ListComfyUIWorkflowNodes)
		workflows.GET("/:workflow_id/input-candidates", controller.ListComfyUIWorkflowInputCandidates)
		workflows.GET("/:workflow_id/output-candidates", controller.ListComfyUIWorkflowOutputCandidates)
		workflows.GET("/:workflow_id/dependencies", controller.ListComfyUIWorkflowDependencies)
		workflows.GET("/:workflow_id/validations", controller.ListComfyUIWorkflowValidations)
		workflows.POST("/:workflow_id/validations", controller.ValidateComfyUIWorkflow)
		workflows.POST("/:workflow_id/convert-to-application-template", controller.ConvertComfyUIWorkflow)
		workflows.POST("/:workflow_id/convert-to-api-workflow", controller.ConvertComfyUIWorkflowToAPI)
		workflows.GET("/:workflow_id/test-runs", controller.ListComfyUIWorkflowTestRuns)
		workflows.POST("/:workflow_id/test-runs", controller.CreateComfyUIWorkflowTestRun)
	}
	rg.GET("/comfyui-workflow-validations/:workflow_validation_id", controller.GetComfyUIWorkflowValidation)
	rg.GET("/comfyui-workflow-test-runs/:test_run_id", controller.GetComfyUIWorkflowTestRun)
	rg.POST("/comfyui-workflow-test-runs/:test_run_id/cancel", controller.CancelComfyUIWorkflowTestRun)
	rg.GET("/comfyui-workflow-test-runs/:test_run_id/outputs/:output_id/content", controller.GetComfyUIWorkflowTestOutputContent)

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
		schedules.GET("/:task_schedule_id/executions", taskCenterController.ListScheduleExecutions)
		schedules.GET("/:task_schedule_id/reconcile-state", taskCenterController.GetScheduleReconcileState)
	}
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

func installAssetLibraryContractApis(rg *gin.RouterGroup, storeIns store.Factory) {
	assetStore := optionalAssetV1Store(storeIns)
	if assetStore == nil {
		return
	}
	tasks := taskcentersvc.NewService(storeIns)
	service := assetlibrarysvc.NewStoreWithRelationsAndPolicy(assetStore, assetlibrarysvc.NewLocalContentStorage(storeIns), assetlibrarysvc.RelationReaders{
		AtomicTasks:     tasks,
		ApplicationRuns: appplatformsvc.NewRunSummaryReader(storeIns.ApplicationPlatforms()),
		CanvasRuns:      workflowcanvassvc.New(storeIns, tasks),
	}, assetlibrarysvc.DefaultRepresentationPolicy{})
	controller := assetlibraryctrl.New(service)

	assets := rg.Group("/assets")
	{
		assets.GET("", controller.ListAssets)
		assets.POST("", controller.CreateAsset)
		assets.POST("/batch-labels", controller.BatchLabels)
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

func optionalAssetV1Store(factory store.Factory) (assetStore store.AssetV1Store) {
	defer func() {
		if recover() != nil {
			assetStore = nil
		}
	}()
	return factory.AssetsV1()
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
