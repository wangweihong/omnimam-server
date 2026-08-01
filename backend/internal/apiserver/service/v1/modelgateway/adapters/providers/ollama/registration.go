package ollama

import (
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
)

// Registration 返回 Ollama 本地模型发现与 OpenAI-compatible 执行协议的静态注册。
func Registration() (modelgateway.Registration, error) {
	capabilities, err := modelgateway.ParseProviderCapabilityManifest("ollama", capabilityManifestYAML)
	if err != nil {
		return modelgateway.Registration{}, err
	}
	chat := iapiserver.CapabilityDefinition{ID: "text.chat_completion", NameI18n: bilingual("对话补全", "Chat Completion"), InputMediaTypes: []string{"text", "json"}, OutputMediaTypes: []string{"text", "json"}}
	responses := iapiserver.CapabilityDefinition{ID: "text.responses", NameI18n: bilingual("统一响应", "Responses"), InputMediaTypes: []string{"text", "image", "json"}, OutputMediaTypes: []string{"text", "json"}}
	return modelgateway.Registration{
		CapabilityDefinitions: []iapiserver.CapabilityDefinition{chat, responses},
		EngineAdapter:         iapiserver.EngineAdapterDefinition{ID: AdapterID, Responsibilities: []string{"base_url", "model_discovery", "health_check", "common_error_mapping"}},
		OperationExecutors:    []iapiserver.OperationExecutorDefinition{{ID: ChatExecutorID, EngineAdapterID: AdapterID, CapabilityDefinitionIDs: []string{chat.ID}}, {ID: ResponsesExecutorID, EngineAdapterID: AdapterID, CapabilityDefinitionIDs: []string{responses.ID}}},
		EngineType:            iapiserver.ApplicationEngineType{ID: AdapterID, NameI18n: bilingual("Ollama", "Ollama"), DescriptionI18n: bilingual("本地 Ollama 模型服务；实际模型由实例发现。", "Local Ollama model service; actual models are discovered per instance."), OfficialWebsiteURL: "https://ollama.com/", OfficialDocumentationURL: "https://docs.ollama.com/api/openai-compatibility", DefaultAPIBaseURL: "http://localhost:11434/v1", Enabled: true, EngineAdapterID: AdapterID, AuthenticationTypes: []string{iapiserver.EngineAuthNone, iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthBearerToken}, AuthenticationConfigSchema: modelgateway.AuthenticationConfigSchema(iapiserver.EngineAuthNone, iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthBearerToken), OperationExecutors: map[string]string{chat.ID: ChatExecutorID, responses.ID: ResponsesExecutorID}},
		ProviderCapabilities:  capabilities,
	}, nil
}

func bilingual(chinese, english string) map[string]string {
	return map[string]string{"zh-CN": chinese, "en-US": english}
}
