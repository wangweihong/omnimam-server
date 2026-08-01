package modelark

import (
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
)

// Registration 返回 BytePlus ModelArk 与 Seedance 2.0 的静态协议和能力注册。
func Registration() appregistry.Registration {
	definitions := []iapiserver.CapabilityDefinition{
		{ID: "video.text_to_video", NameI18n: bilingual("文生视频", "Text to Video"), InputMediaTypes: []string{"text"}, OutputMediaTypes: []string{"video"}},
		{ID: "video.image_to_video", NameI18n: bilingual("图生视频", "Image to Video"), InputMediaTypes: []string{"text", "image"}, OutputMediaTypes: []string{"video"}},
		{ID: "video.video_edit", NameI18n: bilingual("视频编辑", "Video Editing"), InputMediaTypes: []string{"text", "image", "video", "audio"}, OutputMediaTypes: []string{"video"}},
	}
	operations := []iapiserver.ProviderCapabilityOperation{
		{ID: "text-to-video", CapabilityDefinitionID: "video.text_to_video", NameI18n: bilingual("文生视频", "Text to Video"), DescriptionI18n: bilingual("通过 Seedance 2.0 根据文本异步生成视频。", "Asynchronously generate video from text with Seedance 2.0."), ExecutionMode: "asynchronous", InputMediaTypes: []string{"text"}, OutputMediaTypes: []string{"video", "image", "json"}},
		{ID: "image-to-video", CapabilityDefinitionID: "video.image_to_video", NameI18n: bilingual("图生视频", "Image to Video"), DescriptionI18n: bilingual("通过 Seedance 2.0 根据文本和图片异步生成视频。", "Asynchronously generate video from text and images with Seedance 2.0."), ExecutionMode: "asynchronous", InputMediaTypes: []string{"text", "image"}, OutputMediaTypes: []string{"video", "image", "json"}},
		{ID: "reference-to-video", CapabilityDefinitionID: "video.video_edit", NameI18n: bilingual("参考生成视频", "Reference to Video"), DescriptionI18n: bilingual("使用图片、视频或音频参考生成和编辑视频。", "Generate and edit video from image, video, or audio references."), ExecutionMode: "asynchronous", InputMediaTypes: []string{"text", "image", "video", "audio"}, OutputMediaTypes: []string{"video", "image", "json"}},
	}
	models := []iapiserver.ProviderCapabilityModel{
		{ID: "seedance-2.0", ProviderModelID: "dreamina-seedance-2-0-260128", DisplayNameI18n: bilingual("Dreamina Seedance 2.0", "Dreamina Seedance 2.0"), DescriptionI18n: bilingual("Seedance 2.0 标准视频生成模型。", "Standard Seedance 2.0 video generation model."), Family: "seedance-2.0", Variant: "standard", Lifecycle: iapiserver.ProviderLifecycle{Status: "active", AvailableSince: "2026-01-28"}, Limits: seedanceLimits([]string{"480p", "720p", "1080p", "4k"})},
		{ID: "seedance-2.0-fast", ProviderModelID: "dreamina-seedance-2-0-fast-260128", DisplayNameI18n: bilingual("Dreamina Seedance 2.0 Fast", "Dreamina Seedance 2.0 Fast"), DescriptionI18n: bilingual("偏向速度的 Seedance 2.0 视频生成模型。", "Speed-oriented Seedance 2.0 video generation model."), Family: "seedance-2.0", Variant: "fast", Lifecycle: iapiserver.ProviderLifecycle{Status: "active", AvailableSince: "2026-01-28"}, Limits: seedanceLimits([]string{"480p", "720p"})},
	}
	variants := seedanceVariants(models, operations)
	return appregistry.Registration{
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
			Enabled: true, EngineAdapterID: AdapterID, AuthenticationTypes: []string{iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthAKSK},
			AuthenticationConfigSchema: appregistry.AuthenticationConfigSchema(iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthAKSK),
			OperationExecutors:         map[string]string{"video.text_to_video": ExecutorIDs[0], "video.image_to_video": ExecutorIDs[1], "video.video_edit": ExecutorIDs[2]},
		},
		ProviderCapabilities: []iapiserver.AIAppProviderCapability{{
			SchemaVersion: "1.0", ID: "seedance-byteplus", NameI18n: bilingual("BytePlus Seedance 2.0", "BytePlus Seedance 2.0"), DescriptionI18n: bilingual("BytePlus ModelArk 提供的 Seedance 2.0 视频生成能力。", "Seedance 2.0 video generation served by BytePlus ModelArk."),
			Kind: iapiserver.ProviderCapabilityKindCatalog, Origin: iapiserver.ProviderCapabilityOriginStatic, BindingPolicy: iapiserver.ProviderBindingPolicyManual,
			ApplicationEngineTypeID: AdapterID, Revision: "2026-07-31.1", Enabled: true,
			Provider: map[string]any{"code": "bytedance_seedance", "name": "ByteDance Seedance", "model_owner": "ByteDance Seed", "serving_platform": "BytePlus ModelArk", "official_website": "https://seed.bytedance.com/zh/seedance2_0"},
			Sources: []map[string]any{
				{"type": "model_owner", "title": "ByteDance Seedance 2.0", "url": "https://seed.bytedance.com/zh/seedance2_0", "checked_at": "2026-07-31", "scope": "Model positioning and multimodal reference capabilities."},
				{"type": "api_reference", "title": "BytePlus ModelArk Seedance 2.0 API", "url": "https://docs.byteplus.com/en/docs/modelark/1520757", "checked_at": "2026-07-31", "scope": "Executable model IDs, inputs, outputs, and task protocol."},
			},
			Models: models, Operations: operations, Variants: variants,
			Notes: []string{
				"Seedance 2.0 Mini is excluded because no verified stable API model ID is registered.",
				"Provider credentials, polling, and error mapping belong to the ModelArk adapter and executor.",
			},
		}},
	}
}

func seedanceLimits(resolutions []string) map[string]any {
	return map[string]any{
		"output_resolutions": resolutions, "duration_seconds": []int{4, 15},
		"maximum_reference_images": 9, "maximum_reference_videos": 3, "maximum_reference_audios": 3,
	}
}

func seedanceVariants(models []iapiserver.ProviderCapabilityModel, operations []iapiserver.ProviderCapabilityOperation) []iapiserver.ProviderCapabilityVariant {
	variants := make([]iapiserver.ProviderCapabilityVariant, 0, len(models)*len(operations))
	for _, model := range models {
		fast := model.Variant == "fast"
		for _, operation := range operations {
			variants = append(variants, iapiserver.ProviderCapabilityVariant{
				ID: model.Variant + "-" + operation.ID, ModelID: model.ID, OperationID: operation.ID,
				Lifecycle: iapiserver.ProviderLifecycle{Status: "active"}, InputSchema: seedanceInputSchema(operation.ID, fast), OutputSchema: seedanceOutputSchema(),
				UnsupportedParameters: []string{"seed", "camera_fixed", "draft"},
			})
		}
	}
	return variants
}

func seedanceInputSchema(operationID string, fast bool) map[string]any {
	resolutions := []string{"480p", "720p", "1080p", "4k"}
	if fast {
		resolutions = []string{"480p", "720p"}
	}
	properties := map[string]any{
		"prompt":         map[string]any{"type": "string", "maxLength": 10000},
		"resolution":     map[string]any{"type": "string", "enum": resolutions, "default": "720p"},
		"ratio":          map[string]any{"type": "string", "enum": []string{"16:9", "4:3", "1:1", "3:4", "9:16", "21:9", "adaptive"}, "default": "adaptive"},
		"duration":       map[string]any{"type": "integer", "enum": []int{-1, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}, "default": 5},
		"generate_audio": map[string]any{"type": "boolean", "default": true}, "watermark": map[string]any{"type": "boolean", "default": false},
		"callback_url": map[string]any{"type": "string", "format": "uri"}, "return_last_frame": map[string]any{"type": "boolean", "default": false},
		"priority": map[string]any{"type": "integer", "minimum": 0, "maximum": 9, "default": 0}, "safety_identifier": map[string]any{"type": "string", "minLength": 1, "maxLength": 64},
	}
	schema := map[string]any{"type": "object", "additionalProperties": false, "properties": properties}
	switch operationID {
	case "text-to-video":
		properties["prompt"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 10000, "x-provider-parameter": "content.text"}
		schema["required"] = []string{"prompt"}
	case "image-to-video":
		properties["input_images"] = map[string]any{"type": "array", "minItems": 1, "maxItems": 2, "items": map[string]any{"type": "string", "format": "asset_image"}, "x-omnimam-connectable": true}
		schema["required"] = []string{"input_images"}
	case "reference-to-video":
		properties["reference_images"] = map[string]any{"type": "array", "minItems": 1, "maxItems": 9, "items": map[string]any{"type": "string", "format": "asset_image"}, "x-omnimam-connectable": true}
		properties["reference_videos"] = map[string]any{"type": "array", "minItems": 1, "maxItems": 3, "items": map[string]any{"type": "string", "format": "asset_video"}, "x-omnimam-connectable": true}
		properties["reference_audios"] = map[string]any{"type": "array", "minItems": 1, "maxItems": 3, "items": map[string]any{"type": "string", "format": "asset_audio"}, "x-omnimam-connectable": true}
		schema["anyOf"] = []map[string]any{{"required": []string{"reference_images"}}, {"required": []string{"reference_videos"}}}
	}
	return schema
}

func seedanceOutputSchema() map[string]any {
	return map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"task_id", "video_url"},
		"properties": map[string]any{
			"task_id": map[string]any{"type": "string"}, "video_url": map[string]any{"type": "string", "format": "uri"},
			"last_frame_url": map[string]any{"type": []string{"string", "null"}, "format": "uri"}, "resolution": map[string]any{"type": "string"},
			"ratio": map[string]any{"type": "string"}, "duration": map[string]any{"type": "integer"}, "generate_audio": map[string]any{"type": "boolean"},
			"usage": map[string]any{"type": "object", "additionalProperties": true},
		},
	}
}

func bilingual(chinese, english string) map[string]string {
	return map[string]string{"zh-CN": chinese, "en-US": english}
}
