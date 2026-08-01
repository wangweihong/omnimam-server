package xai

import (
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
)

// Registration 返回 xAI Grok Responses 协议与当前常用模型的静态注册。
func Registration() (modelgateway.Registration, error) {
	capabilities, err := modelgateway.ParseProviderCapabilityManifest("xai", capabilityManifestYAML)
	if err != nil {
		return modelgateway.Registration{}, err
	}
	definition := iapiserver.CapabilityDefinition{ID: "text.responses", NameI18n: bilingual("统一响应", "Responses"), InputMediaTypes: []string{"text", "image", "json"}, OutputMediaTypes: []string{"text", "json"}}
	return modelgateway.Registration{
		CapabilityDefinitions: []iapiserver.CapabilityDefinition{definition},
		EngineAdapter:         iapiserver.EngineAdapterDefinition{ID: AdapterID, Responsibilities: []string{"authentication", "base_url", "health_check", "common_error_mapping"}},
		OperationExecutors:    []iapiserver.OperationExecutorDefinition{{ID: ExecutorID, EngineAdapterID: AdapterID, CapabilityDefinitionIDs: []string{definition.ID}}},
		EngineType:            iapiserver.ApplicationEngineType{ID: AdapterID, NameI18n: bilingual("xAI Grok Responses", "xAI Grok Responses"), DescriptionI18n: bilingual("xAI 官方 Grok Responses API 服务。", "Official xAI Grok Responses API service."), OfficialWebsiteURL: "https://x.ai/", OfficialDocumentationURL: "https://docs.x.ai/docs/overview", DefaultAPIBaseURL: "https://api.x.ai/v1", Enabled: true, EngineAdapterID: AdapterID, AuthenticationTypes: []string{iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthBearerToken}, AuthenticationConfigSchema: modelgateway.AuthenticationConfigSchema(iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthBearerToken), OperationExecutors: map[string]string{definition.ID: ExecutorID}},
		ProviderCapabilities:  capabilities,
	}, nil
}

func bilingual(chinese, english string) map[string]string {
	return map[string]string{"zh-CN": chinese, "en-US": english}
}
