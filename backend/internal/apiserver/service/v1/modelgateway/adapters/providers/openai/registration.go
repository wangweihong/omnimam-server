package openai

import (
	"fmt"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
)

const (
	ResponsesAdapterID  = "openai_responses"
	ResponsesExecutorID = "openai_responses_create"
	ImagesAdapterID     = "openai_images"
	ImagesExecutorID    = "openai_images_generate"
)

// Registrations 返回 OpenAI provider 的 Responses 与 Images 独立运行注册。
func Registrations() ([]modelgateway.Registration, error) {
	capabilities, err := modelgateway.ParseProviderCapabilityManifest("openai", capabilityManifestYAML)
	if err != nil {
		return nil, err
	}
	byEngineType := make(map[string][]iapiserver.AIAppProviderCapability, 2)
	for _, capability := range capabilities {
		byEngineType[capability.ApplicationEngineTypeID] = append(byEngineType[capability.ApplicationEngineTypeID], capability)
	}
	responses := textResponsesDefinition()
	images := imageGenerationDefinition()
	if len(byEngineType[ResponsesAdapterID]) != 1 || len(byEngineType[ImagesAdapterID]) != 1 {
		return nil, fmt.Errorf("openai manifest must define one capability for %s and %s", ResponsesAdapterID, ImagesAdapterID)
	}
	return []modelgateway.Registration{
		{
			CapabilityDefinitions: []iapiserver.CapabilityDefinition{responses},
			EngineAdapter:         iapiserver.EngineAdapterDefinition{ID: ResponsesAdapterID, Responsibilities: []string{"authentication", "base_url", "health_check", "common_error_mapping"}},
			OperationExecutors:    []iapiserver.OperationExecutorDefinition{{ID: ResponsesExecutorID, EngineAdapterID: ResponsesAdapterID, CapabilityDefinitionIDs: []string{responses.ID}}},
			EngineType: iapiserver.ApplicationEngineType{
				ID: ResponsesAdapterID, NameI18n: bilingual("OpenAI Responses", "OpenAI Responses"), DescriptionI18n: bilingual("OpenAI 官方 Responses API 服务。", "Official OpenAI Responses API service."),
				OfficialWebsiteURL: "https://openai.com/", OfficialDocumentationURL: "https://developers.openai.com/api/reference/resources/responses/methods/create/", DefaultAPIBaseURL: "https://api.openai.com/v1",
				Enabled: true, EngineAdapterID: ResponsesAdapterID, AuthenticationTypes: []string{iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthBearerToken}, AuthenticationConfigSchema: modelgateway.AuthenticationConfigSchema(iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthBearerToken), OperationExecutors: map[string]string{responses.ID: ResponsesExecutorID},
			},
			ProviderCapabilities: byEngineType[ResponsesAdapterID],
		},
		{
			CapabilityDefinitions: []iapiserver.CapabilityDefinition{images},
			EngineAdapter:         iapiserver.EngineAdapterDefinition{ID: ImagesAdapterID, Responsibilities: []string{"authentication", "base_url", "health_check", "common_error_mapping"}},
			OperationExecutors:    []iapiserver.OperationExecutorDefinition{{ID: ImagesExecutorID, EngineAdapterID: ImagesAdapterID, CapabilityDefinitionIDs: []string{images.ID}}},
			EngineType: iapiserver.ApplicationEngineType{
				ID: ImagesAdapterID, NameI18n: bilingual("OpenAI 图像", "OpenAI Images"), DescriptionI18n: bilingual("OpenAI 官方图像生成 API 服务。", "Official OpenAI image generation API service."),
				OfficialWebsiteURL: "https://openai.com/", OfficialDocumentationURL: "https://developers.openai.com/api/reference/resources/images/methods/generate/", DefaultAPIBaseURL: "https://api.openai.com/v1",
				Enabled: true, EngineAdapterID: ImagesAdapterID, AuthenticationTypes: []string{iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthBearerToken}, AuthenticationConfigSchema: modelgateway.AuthenticationConfigSchema(iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthBearerToken), OperationExecutors: map[string]string{images.ID: ImagesExecutorID},
			},
			ProviderCapabilities: byEngineType[ImagesAdapterID],
		},
	}, nil
}

func textResponsesDefinition() iapiserver.CapabilityDefinition {
	return iapiserver.CapabilityDefinition{ID: "text.responses", NameI18n: bilingual("统一响应", "Responses"), InputMediaTypes: []string{"text", "image", "json"}, OutputMediaTypes: []string{"text", "json"}}
}

func imageGenerationDefinition() iapiserver.CapabilityDefinition {
	return iapiserver.CapabilityDefinition{ID: "image.text_to_image", NameI18n: bilingual("文生图", "Text to Image"), InputMediaTypes: []string{"text"}, OutputMediaTypes: []string{"image"}}
}

func bilingual(chinese, english string) map[string]string {
	return map[string]string{"zh-CN": chinese, "en-US": english}
}
