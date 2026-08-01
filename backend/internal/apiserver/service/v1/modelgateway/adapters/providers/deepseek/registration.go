package deepseek

import (
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
)

const (
	AdapterID  = "deepseek_official"
	ExecutorID = "deepseek_chat_completions"
)

// Registration 返回 DeepSeek 官方 OpenAI-compatible 协议与能力注册。
func Registration() (modelgateway.Registration, error) {
	capabilities, err := modelgateway.ParseProviderCapabilityManifest("deepseek", capabilityManifestYAML)
	if err != nil {
		return modelgateway.Registration{}, err
	}
	definition := iapiserver.CapabilityDefinition{ID: "text.chat_completion", NameI18n: bilingual("对话补全", "Chat Completion"), InputMediaTypes: []string{"text", "json"}, OutputMediaTypes: []string{"text", "json"}}
	return modelgateway.Registration{
		CapabilityDefinitions: []iapiserver.CapabilityDefinition{definition},
		EngineAdapter:         iapiserver.EngineAdapterDefinition{ID: AdapterID, Responsibilities: []string{"authentication", "base_url", "health_check", "common_error_mapping"}},
		OperationExecutors:    []iapiserver.OperationExecutorDefinition{{ID: ExecutorID, EngineAdapterID: AdapterID, CapabilityDefinitionIDs: []string{definition.ID}}},
		EngineType: iapiserver.ApplicationEngineType{
			ID: AdapterID, NameI18n: bilingual("DeepSeek 官方 API", "DeepSeek Official API"), DescriptionI18n: bilingual("DeepSeek 官方 OpenAI-compatible 对话服务。", "Official DeepSeek OpenAI-compatible chat service."),
			OfficialWebsiteURL: "https://www.deepseek.com/", OfficialDocumentationURL: "https://api-docs.deepseek.com/api/create-chat-completion", DefaultAPIBaseURL: "https://api.deepseek.com",
			Enabled: true, EngineAdapterID: AdapterID, AuthenticationTypes: []string{iapiserver.EngineAuthAPIKey}, AuthenticationConfigSchema: modelgateway.AuthenticationConfigSchema(iapiserver.EngineAuthAPIKey), OperationExecutors: map[string]string{definition.ID: ExecutorID},
		},
		ProviderCapabilities: capabilities,
	}, nil
}

func bilingual(chinese, english string) map[string]string {
	return map[string]string{"zh-CN": chinese, "en-US": english}
}
