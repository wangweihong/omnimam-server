package modelark

import (
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
)

// Registration 返回 BytePlus ModelArk 与 Seedance 2.0 的静态协议和能力注册。
func Registration() (modelgateway.Registration, error) {
	capabilities, err := modelgateway.ParseProviderCapabilityManifest("modelark", capabilityManifestYAML)
	if err != nil {
		return modelgateway.Registration{}, err
	}
	definitions := []iapiserver.CapabilityDefinition{
		{ID: "video.text_to_video", NameI18n: bilingual("文生视频", "Text to Video"), InputMediaTypes: []string{"text"}, OutputMediaTypes: []string{"video"}},
		{ID: "video.image_to_video", NameI18n: bilingual("图生视频", "Image to Video"), InputMediaTypes: []string{"text", "image"}, OutputMediaTypes: []string{"video"}},
		{ID: "video.video_edit", NameI18n: bilingual("视频编辑", "Video Editing"), InputMediaTypes: []string{"text", "image", "video", "audio"}, OutputMediaTypes: []string{"video"}},
	}
	return modelgateway.Registration{
		CapabilityDefinitions: definitions,
		EngineAdapter:         iapiserver.EngineAdapterDefinition{ID: AdapterID, Responsibilities: []string{"authentication", "base_url", "upload", "health_check", "common_error_mapping"}},
		OperationExecutors: []iapiserver.OperationExecutorDefinition{
			{ID: ExecutorIDs[0], EngineAdapterID: AdapterID, CapabilityDefinitionIDs: []string{"video.text_to_video"}},
			{ID: ExecutorIDs[1], EngineAdapterID: AdapterID, CapabilityDefinitionIDs: []string{"video.image_to_video"}},
			{ID: ExecutorIDs[2], EngineAdapterID: AdapterID, CapabilityDefinitionIDs: []string{"video.video_edit"}},
		},
		EngineType: iapiserver.ApplicationEngineType{
			ID: AdapterID, NameI18n: bilingual("BytePlus ModelArk", "BytePlus ModelArk"), DescriptionI18n: bilingual("BytePlus ModelArk 视频生成服务。", "BytePlus ModelArk video generation service."),
			OfficialWebsiteURL: "https://www.byteplus.com/en/product/modelark", OfficialDocumentationURL: "https://docs.byteplus.com/en/docs/modelark", DefaultAPIBaseURL: "https://ark.ap-southeast.bytepluses.com",
			Enabled: true, EngineAdapterID: AdapterID, AuthenticationTypes: []string{iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthAKSK}, AuthenticationConfigSchema: modelgateway.AuthenticationConfigSchema(iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthAKSK),
			OperationExecutors: map[string]string{"video.text_to_video": ExecutorIDs[0], "video.image_to_video": ExecutorIDs[1], "video.video_edit": ExecutorIDs[2]},
		},
		ProviderCapabilities: capabilities,
	}, nil
}

func bilingual(chinese, english string) map[string]string {
	return map[string]string{"zh-CN": chinese, "en-US": english}
}
