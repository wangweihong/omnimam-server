package xai

import (
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
)

// Registration 返回 xAI Grok Responses 协议与当前常用模型的静态注册。
func Registration() appregistry.Registration {
	definition := iapiserver.CapabilityDefinition{ID: "text.responses", NameI18n: bilingual("统一响应", "Responses"), InputMediaTypes: []string{"text", "image", "json"}, OutputMediaTypes: []string{"text", "json"}}
	models := []iapiserver.ProviderCapabilityModel{
		{ID: "grok-4.5", ProviderModelID: "grok-4.5", DisplayNameI18n: bilingual("Grok 4.5", "Grok 4.5"), DescriptionI18n: bilingual("xAI 旗舰代码与通用模型。", "xAI flagship model for code and general workloads."), Family: "grok-4", Variant: "4.5", Lifecycle: iapiserver.ProviderLifecycle{Status: "active"}},
		{ID: "grok-4.3", ProviderModelID: "grok-4.3", DisplayNameI18n: bilingual("Grok 4.3", "Grok 4.3"), DescriptionI18n: bilingual("xAI 长上下文推理模型。", "xAI long-context reasoning model."), Family: "grok-4", Variant: "4.3", Lifecycle: iapiserver.ProviderLifecycle{Status: "active"}},
	}
	operation := iapiserver.ProviderCapabilityOperation{ID: "responses", CapabilityDefinitionID: definition.ID, NameI18n: bilingual("Grok Responses", "Grok Responses"), DescriptionI18n: bilingual("通过 xAI 官方 Responses API 生成文本和工具调用结果。", "Generate text and tool-call results through the official xAI Responses API."), ExecutionMode: "synchronous", InputMediaTypes: []string{"text", "image", "json"}, OutputMediaTypes: []string{"text", "json"}}
	variants := make([]iapiserver.ProviderCapabilityVariant, 0, len(models))
	for _, item := range models {
		variants = append(variants, iapiserver.ProviderCapabilityVariant{ID: item.ID + "-responses", ModelID: item.ID, OperationID: operation.ID, Lifecycle: iapiserver.ProviderLifecycle{Status: "active"}, InputSchema: map[string]any{"type": "object", "additionalProperties": true}, OutputSchema: map[string]any{"type": "object", "additionalProperties": true}})
	}
	return appregistry.Registration{
		CapabilityDefinitions: []iapiserver.CapabilityDefinition{definition},
		EngineAdapter:         iapiserver.EngineAdapterDefinition{ID: AdapterID, Responsibilities: []string{"authentication", "base_url", "health_check", "common_error_mapping"}},
		OperationExecutors:    []iapiserver.OperationExecutorDefinition{{ID: ExecutorID, EngineAdapterID: AdapterID, CapabilityDefinitionIDs: []string{definition.ID}}},
		EngineType:            iapiserver.ApplicationEngineType{ID: AdapterID, NameI18n: bilingual("xAI Grok Responses", "xAI Grok Responses"), DescriptionI18n: bilingual("xAI 官方 Grok Responses API 服务。", "Official xAI Grok Responses API service."), OfficialWebsiteURL: "https://x.ai/", OfficialDocumentationURL: "https://docs.x.ai/docs/overview", DefaultAPIBaseURL: "https://api.x.ai/v1", Enabled: true, EngineAdapterID: AdapterID, AuthenticationTypes: []string{iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthBearerToken}, AuthenticationConfigSchema: appregistry.AuthenticationConfigSchema(iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthBearerToken), OperationExecutors: map[string]string{definition.ID: ExecutorID}},
		ProviderCapabilities:  []iapiserver.AIAppProviderCapability{{SchemaVersion: "1.0", ID: "xai-grok-responses", NameI18n: bilingual("xAI Grok Responses 能力", "xAI Grok Responses Capability"), DescriptionI18n: bilingual("Grok 4.5 和 Grok 4.3 的官方 Responses 能力。", "Official Responses capability for Grok 4.5 and Grok 4.3."), Kind: iapiserver.ProviderCapabilityKindCatalog, Origin: iapiserver.ProviderCapabilityOriginStatic, BindingPolicy: iapiserver.ProviderBindingPolicyManual, ApplicationEngineTypeID: AdapterID, Revision: "2026-07-31.1", Enabled: true, Provider: map[string]any{"code": "xai", "name": "xAI", "model_owner": "xAI", "serving_platform": "xAI API", "official_website": "https://x.ai/"}, Sources: []map[string]any{{"type": "api_reference", "title": "xAI API Overview", "url": "https://docs.x.ai/docs/overview", "checked_at": "2026-07-31", "scope": "POST https://api.x.ai/v1/responses and official Grok model IDs."}}, Models: models, Operations: []iapiserver.ProviderCapabilityOperation{operation}, Variants: variants}},
	}
}

func bilingual(chinese, english string) map[string]string {
	return map[string]string{"zh-CN": chinese, "en-US": english}
}
