package comfyui

import (
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
)

// Registration 返回 ComfyUI 协议、工作流执行器和系统绑定能力的静态注册。
func Registration() (modelgateway.Registration, error) {
	capabilities, err := modelgateway.ParseProviderCapabilityManifest("comfyui", capabilityManifestYAML)
	if err != nil {
		return modelgateway.Registration{}, err
	}
	definitions := []iapiserver.CapabilityDefinition{
		{ID: "video.text_to_video", NameI18n: bilingual("文生视频", "Text to Video"), InputMediaTypes: []string{"text"}, OutputMediaTypes: []string{"video"}},
		{ID: "video.image_to_video", NameI18n: bilingual("图生视频", "Image to Video"), InputMediaTypes: []string{"text", "image"}, OutputMediaTypes: []string{"video"}},
		{ID: "video.video_edit", NameI18n: bilingual("视频编辑", "Video Editing"), InputMediaTypes: []string{"text", "image", "video", "audio"}, OutputMediaTypes: []string{"video"}},
		{ID: "image.text_to_image", NameI18n: bilingual("文生图", "Text to Image"), InputMediaTypes: []string{"text"}, OutputMediaTypes: []string{"image"}},
		{ID: "image.edit", NameI18n: bilingual("图像编辑", "Image Editing"), InputMediaTypes: []string{"text", "image"}, OutputMediaTypes: []string{"image"}},
		{ID: "image.upscale", NameI18n: bilingual("图像放大", "Image Upscaling"), InputMediaTypes: []string{"image"}, OutputMediaTypes: []string{"image"}},
	}
	capabilityIDs := make([]string, 0, len(definitions))
	operationExecutors := make(map[string]string, len(definitions))
	for _, definition := range definitions {
		capabilityIDs = append(capabilityIDs, definition.ID)
		operationExecutors[definition.ID] = ExecutorID
	}
	return modelgateway.Registration{
		CapabilityDefinitions: definitions,
		EngineAdapter:         iapiserver.EngineAdapterDefinition{ID: AdapterID, Responsibilities: []string{"authentication", "base_url", "upload", "health_check", "common_error_mapping"}},
		OperationExecutors:    []iapiserver.OperationExecutorDefinition{{ID: ExecutorID, EngineAdapterID: AdapterID, CapabilityDefinitionIDs: capabilityIDs}},
		EngineType: iapiserver.ApplicationEngineType{
			ID: AdapterID, NameI18n: bilingual("ComfyUI", "ComfyUI"),
			DescriptionI18n:    bilingual("自托管 ComfyUI 工作流执行环境。", "Self-hosted ComfyUI workflow execution environment."),
			OfficialWebsiteURL: "https://www.comfy.org/", OfficialDocumentationURL: "https://docs.comfy.org/development/core-concepts/api", DefaultAPIBaseURL: "http://localhost:8188",
			Enabled: true, EngineAdapterID: AdapterID, AuthenticationTypes: []string{iapiserver.EngineAuthNone, iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthBearerToken},
			AuthenticationConfigSchema: modelgateway.AuthenticationConfigSchema(iapiserver.EngineAuthNone, iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthBearerToken),
			OperationExecutors:         operationExecutors,
		},
		ProviderCapabilities: capabilities,
	}, nil
}

func bilingual(chinese, english string) map[string]string {
	return map[string]string{"zh-CN": chinese, "en-US": english}
}
