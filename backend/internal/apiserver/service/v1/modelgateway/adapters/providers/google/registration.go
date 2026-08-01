package google

import (
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
)

// Registration 返回 Google Gemini 与 Nano Banana 官方 Interactions 协议的静态注册。
func Registration() (modelgateway.Registration, error) {
	capabilities, err := modelgateway.ParseProviderCapabilityManifest("google", capabilityManifestYAML)
	if err != nil {
		return modelgateway.Registration{}, err
	}
	textDefinition := iapiserver.CapabilityDefinition{ID: "text.responses", NameI18n: bilingual("统一响应", "Responses"), InputMediaTypes: []string{"text", "image", "json"}, OutputMediaTypes: []string{"text", "json"}}
	imageDefinition := iapiserver.CapabilityDefinition{ID: "image.text_to_image", NameI18n: bilingual("文生图", "Text to Image"), InputMediaTypes: []string{"text"}, OutputMediaTypes: []string{"image"}}
	return modelgateway.Registration{
		CapabilityDefinitions: []iapiserver.CapabilityDefinition{textDefinition, imageDefinition},
		EngineAdapter:         iapiserver.EngineAdapterDefinition{ID: AdapterID, Responsibilities: []string{"authentication", "base_url", "health_check", "common_error_mapping"}},
		OperationExecutors:    []iapiserver.OperationExecutorDefinition{{ID: ExecutorID, EngineAdapterID: AdapterID, CapabilityDefinitionIDs: []string{textDefinition.ID, imageDefinition.ID}}},
		EngineType:            iapiserver.ApplicationEngineType{ID: AdapterID, NameI18n: bilingual("Google Gemini", "Google Gemini"), DescriptionI18n: bilingual("Google Gemini 原生文本与 Nano Banana 图像服务。", "Native Google Gemini text and Nano Banana image service."), OfficialWebsiteURL: "https://ai.google.dev/gemini-api", OfficialDocumentationURL: "https://ai.google.dev/gemini-api/docs", DefaultAPIBaseURL: "https://generativelanguage.googleapis.com/v1beta", Enabled: true, EngineAdapterID: AdapterID, AuthenticationTypes: []string{iapiserver.EngineAuthAPIKey}, AuthenticationConfigSchema: modelgateway.AuthenticationConfigSchema(iapiserver.EngineAuthAPIKey), OperationExecutors: map[string]string{textDefinition.ID: ExecutorID, imageDefinition.ID: ExecutorID}},
		ProviderCapabilities:  capabilities,
	}, nil
}

func bilingual(chinese, english string) map[string]string {
	return map[string]string{"zh-CN": chinese, "en-US": english}
}
