package ollama

import (
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
)

// Registration 返回 Ollama 本地模型发现与 OpenAI-compatible 执行协议的静态注册。
func Registration() appregistry.Registration {
	chat := iapiserver.CapabilityDefinition{ID: "text.chat_completion", NameI18n: bilingual("对话补全", "Chat Completion"), InputMediaTypes: []string{"text", "json"}, OutputMediaTypes: []string{"text", "json"}}
	responses := iapiserver.CapabilityDefinition{ID: "text.responses", NameI18n: bilingual("统一响应", "Responses"), InputMediaTypes: []string{"text", "image", "json"}, OutputMediaTypes: []string{"text", "json"}}
	operations := []iapiserver.ProviderCapabilityOperation{
		{ID: "chat-completions", CapabilityDefinitionID: chat.ID, NameI18n: bilingual("Ollama 对话补全", "Ollama Chat Completions"), DescriptionI18n: bilingual("调用 Ollama `/v1/chat/completions`。", "Call Ollama `/v1/chat/completions`."), ExecutionMode: "synchronous", InputMediaTypes: []string{"text", "image", "json"}, OutputMediaTypes: []string{"text", "json"}},
		{ID: "responses", CapabilityDefinitionID: responses.ID, NameI18n: bilingual("Ollama Responses", "Ollama Responses"), DescriptionI18n: bilingual("调用 Ollama v0.13.3+ 无状态 `/v1/responses`。", "Call the stateless `/v1/responses` endpoint in Ollama v0.13.3+."), ExecutionMode: "synchronous", InputMediaTypes: []string{"text", "image", "json"}, OutputMediaTypes: []string{"text", "json"}},
	}
	return appregistry.Registration{
		CapabilityDefinitions: []iapiserver.CapabilityDefinition{chat, responses},
		EngineAdapter:         iapiserver.EngineAdapterDefinition{ID: AdapterID, Responsibilities: []string{"base_url", "model_discovery", "health_check", "common_error_mapping"}},
		OperationExecutors:    []iapiserver.OperationExecutorDefinition{{ID: ChatExecutorID, EngineAdapterID: AdapterID, CapabilityDefinitionIDs: []string{chat.ID}}, {ID: ResponsesExecutorID, EngineAdapterID: AdapterID, CapabilityDefinitionIDs: []string{responses.ID}}},
		EngineType:            iapiserver.ApplicationEngineType{ID: AdapterID, NameI18n: bilingual("Ollama", "Ollama"), DescriptionI18n: bilingual("本地 Ollama 模型服务；实际模型由实例发现。", "Local Ollama model service; actual models are discovered per instance."), OfficialWebsiteURL: "https://ollama.com/", OfficialDocumentationURL: "https://docs.ollama.com/api/openai-compatibility", DefaultAPIBaseURL: "http://localhost:11434/v1", Enabled: true, EngineAdapterID: AdapterID, AuthenticationTypes: []string{iapiserver.EngineAuthNone, iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthBearerToken}, AuthenticationConfigSchema: appregistry.AuthenticationConfigSchema(iapiserver.EngineAuthNone, iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthBearerToken), OperationExecutors: map[string]string{chat.ID: ChatExecutorID, responses.ID: ResponsesExecutorID}},
		ProviderCapabilities:  []iapiserver.AIAppProviderCapability{{SchemaVersion: "1.0", ID: "ollama-openai-compatible", NameI18n: bilingual("Ollama OpenAI 兼容能力", "Ollama OpenAI Compatibility"), DescriptionI18n: bilingual("声明 Ollama Chat Completions 与无状态 Responses 协议；不伪造本地模型目录。", "Declares Ollama Chat Completions and stateless Responses protocols without inventing a local model catalog."), Kind: iapiserver.ProviderCapabilityKindCatalog, Origin: iapiserver.ProviderCapabilityOriginStatic, BindingPolicy: iapiserver.ProviderBindingPolicyManual, ApplicationEngineTypeID: AdapterID, Revision: "2026-07-31.1", Enabled: true, Provider: map[string]any{"code": "ollama", "name": "Ollama", "model_owner": "Local model authors", "serving_platform": "Ollama", "official_website": "https://ollama.com/"}, Sources: []map[string]any{{"type": "api_reference", "title": "OpenAI compatibility", "url": "https://docs.ollama.com/api/openai-compatibility", "checked_at": "2026-07-31", "scope": "Default base URL, Chat Completions, stateless Responses, and model listing."}}, Models: []iapiserver.ProviderCapabilityModel{}, Operations: operations, Variants: []iapiserver.ProviderCapabilityVariant{}}},
	}
}

func bilingual(chinese, english string) map[string]string {
	return map[string]string{"zh-CN": chinese, "en-US": english}
}
