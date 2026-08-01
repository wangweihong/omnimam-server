package runninghub

import (
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
)

// Registration 返回 RunningHub 工作流运行时及官方异步任务协议的静态注册。
func Registration() appregistry.Registration {
	definition := iapiserver.CapabilityDefinition{ID: "workflow.execute", NameI18n: bilingual("工作流执行", "Workflow Execution"), InputMediaTypes: []string{"text", "image", "audio", "video", "json"}, OutputMediaTypes: []string{"image", "audio", "video", "json"}}
	return appregistry.Registration{
		CapabilityDefinitions: []iapiserver.CapabilityDefinition{definition},
		EngineAdapter:         iapiserver.EngineAdapterDefinition{ID: AdapterID, Responsibilities: []string{"authentication", "base_url", "health_check", "task_submit", "task_poll", "task_cancel", "common_error_mapping"}},
		OperationExecutors:    []iapiserver.OperationExecutorDefinition{{ID: ExecutorID, EngineAdapterID: AdapterID, CapabilityDefinitionIDs: []string{definition.ID}}},
		EngineType:            iapiserver.ApplicationEngineType{ID: AdapterID, NameI18n: bilingual("RunningHub 工作流", "RunningHub Workflow"), DescriptionI18n: bilingual("RunningHub 托管 ComfyUI 工作流异步执行服务。", "RunningHub hosted asynchronous ComfyUI workflow service."), OfficialWebsiteURL: "https://www.runninghub.cn/", OfficialDocumentationURL: "https://www.runninghub.cn/runninghub-api-doc-cn", DefaultAPIBaseURL: "https://www.runninghub.cn", Enabled: true, EngineAdapterID: AdapterID, AuthenticationTypes: []string{iapiserver.EngineAuthAPIKey}, AuthenticationConfigSchema: appregistry.AuthenticationConfigSchema(iapiserver.EngineAuthAPIKey), OperationExecutors: map[string]string{definition.ID: ExecutorID}},
		ProviderCapabilities:  []iapiserver.AIAppProviderCapability{{SchemaVersion: "1.0", ID: "runninghub-workflow-runtime", NameI18n: bilingual("RunningHub 工作流运行时", "RunningHub Workflow Runtime"), DescriptionI18n: bilingual("RunningHub 实例的系统绑定能力；workflowId 与节点映射由模板和运行快照固定。", "System binding for RunningHub instances; templates and run snapshots pin workflowId and node mappings."), Kind: iapiserver.ProviderCapabilityKindEngineBinding, Origin: iapiserver.ProviderCapabilityOriginStatic, BindingPolicy: iapiserver.ProviderBindingPolicyRequiredImmutable, ApplicationEngineTypeID: AdapterID, Revision: "2026-07-31.1", Enabled: true, Provider: map[string]any{"code": "runninghub", "name": "RunningHub", "model_owner": "Workflow authors", "serving_platform": "RunningHub", "official_website": "https://www.runninghub.cn/"}, Sources: []map[string]any{{"type": "api_reference", "title": "RunningHub OpenAPI", "url": "https://www.runninghub.cn/runninghub-api-doc-cn/api-425749013", "checked_at": "2026-07-31", "scope": "POST /task/openapi/create, /outputs, /cancel, apiKey, workflowId, and nodeInfoList."}}, Models: []iapiserver.ProviderCapabilityModel{}, Operations: []iapiserver.ProviderCapabilityOperation{}, Variants: []iapiserver.ProviderCapabilityVariant{}}},
	}
}

func bilingual(chinese, english string) map[string]string {
	return map[string]string{"zh-CN": chinese, "en-US": english}
}
