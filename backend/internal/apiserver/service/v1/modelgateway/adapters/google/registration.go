package google

import (
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
)

// Registration 返回 Google Gemini 与 Nano Banana 官方 Interactions 协议的静态注册。
func Registration() appregistry.Registration {
	textDefinition := iapiserver.CapabilityDefinition{ID: "text.responses", NameI18n: bilingual("统一响应", "Responses"), InputMediaTypes: []string{"text", "image", "json"}, OutputMediaTypes: []string{"text", "json"}}
	imageDefinition := iapiserver.CapabilityDefinition{ID: "image.text_to_image", NameI18n: bilingual("文生图", "Text to Image"), InputMediaTypes: []string{"text"}, OutputMediaTypes: []string{"image"}}
	models := []iapiserver.ProviderCapabilityModel{
		model("gemini-2.5-flash", "Gemini 2.5 Flash", "通用低延迟 Gemini 文本模型。", "General low-latency Gemini text model.", "gemini-2.5", "flash"),
		model("gemini-2.5-pro", "Gemini 2.5 Pro", "面向复杂任务的 Gemini 文本模型。", "Gemini text model for complex tasks.", "gemini-2.5", "pro"),
		model("gemini-3.1-flash-image", "Nano Banana 2", "兼顾速度、4K 生成与文字渲染的通用图像模型。", "Versatile image model balancing speed, 4K generation, and text rendering.", "nano-banana", "2"),
		model("gemini-3-pro-image", "Nano Banana Pro", "面向复杂视觉任务的高精度图像模型。", "High-precision image model for complex visual tasks.", "nano-banana", "pro"),
		model("gemini-2.5-flash-image", "Nano Banana", "面向快速低成本生成的 Gemini 图像模型。", "Gemini image model for fast, cost-efficient generation.", "nano-banana", "standard"),
	}
	operations := []iapiserver.ProviderCapabilityOperation{
		{ID: "interactions-text", CapabilityDefinitionID: textDefinition.ID, NameI18n: bilingual("Gemini 文本交互", "Gemini Text Interaction"), DescriptionI18n: bilingual("通过 Gemini Interactions API 生成文本结果。", "Generate text results through the Gemini Interactions API."), ExecutionMode: "synchronous", InputMediaTypes: []string{"text", "image", "json"}, OutputMediaTypes: []string{"text", "json"}},
		{ID: "interactions-image", CapabilityDefinitionID: imageDefinition.ID, NameI18n: bilingual("Nano Banana 图像生成", "Nano Banana Image Generation"), DescriptionI18n: bilingual("通过 Gemini Interactions API 生成或编辑图像。", "Generate or edit images through the Gemini Interactions API."), ExecutionMode: "synchronous", InputMediaTypes: []string{"text", "image"}, OutputMediaTypes: []string{"image", "text", "json"}},
	}
	variants := []iapiserver.ProviderCapabilityVariant{}
	for _, item := range models[:2] {
		variants = append(variants, variant(item.ID, operations[0].ID))
	}
	for _, item := range models[2:] {
		variants = append(variants, variant(item.ID, operations[1].ID))
	}
	return appregistry.Registration{
		CapabilityDefinitions: []iapiserver.CapabilityDefinition{textDefinition, imageDefinition},
		EngineAdapter:         iapiserver.EngineAdapterDefinition{ID: AdapterID, Responsibilities: []string{"authentication", "base_url", "health_check", "common_error_mapping"}},
		OperationExecutors:    []iapiserver.OperationExecutorDefinition{{ID: ExecutorID, EngineAdapterID: AdapterID, CapabilityDefinitionIDs: []string{textDefinition.ID, imageDefinition.ID}}},
		EngineType:            iapiserver.ApplicationEngineType{ID: AdapterID, NameI18n: bilingual("Google Gemini", "Google Gemini"), DescriptionI18n: bilingual("Google Gemini 原生文本与 Nano Banana 图像服务。", "Native Google Gemini text and Nano Banana image service."), OfficialWebsiteURL: "https://ai.google.dev/gemini-api", OfficialDocumentationURL: "https://ai.google.dev/gemini-api/docs", DefaultAPIBaseURL: "https://generativelanguage.googleapis.com/v1beta", Enabled: true, EngineAdapterID: AdapterID, AuthenticationTypes: []string{iapiserver.EngineAuthAPIKey}, AuthenticationConfigSchema: appregistry.AuthenticationConfigSchema(iapiserver.EngineAuthAPIKey), OperationExecutors: map[string]string{textDefinition.ID: ExecutorID, imageDefinition.ID: ExecutorID}},
		ProviderCapabilities:  []iapiserver.AIAppProviderCapability{{SchemaVersion: "1.0", ID: "google-gemini", NameI18n: bilingual("Google Gemini 与 Nano Banana", "Google Gemini and Nano Banana"), DescriptionI18n: bilingual("Gemini 文本模型和三代 Nano Banana 图像模型。", "Gemini text models and three Nano Banana image model generations."), Kind: iapiserver.ProviderCapabilityKindCatalog, Origin: iapiserver.ProviderCapabilityOriginStatic, BindingPolicy: iapiserver.ProviderBindingPolicyManual, ApplicationEngineTypeID: AdapterID, Revision: "2026-07-31.1", Enabled: true, Provider: map[string]any{"code": "google_gemini", "name": "Google Gemini", "model_owner": "Google", "serving_platform": "Gemini API", "official_website": "https://ai.google.dev/gemini-api"}, Sources: []map[string]any{{"type": "api_reference", "title": "Nano Banana image generation", "url": "https://ai.google.dev/gemini-api/docs/image-generation", "checked_at": "2026-07-31", "scope": "Nano Banana marketing names, exact model IDs, Interactions endpoint, and x-goog-api-key authentication."}}, Models: models, Operations: operations, Variants: variants}},
	}
}

func model(id, displayName, descriptionZH, descriptionEN, family, variant string) iapiserver.ProviderCapabilityModel {
	return iapiserver.ProviderCapabilityModel{ID: id, ProviderModelID: id, DisplayNameI18n: bilingual(displayName, displayName), DescriptionI18n: bilingual(descriptionZH, descriptionEN), Family: family, Variant: variant, Lifecycle: iapiserver.ProviderLifecycle{Status: "active"}}
}

func variant(modelID, operationID string) iapiserver.ProviderCapabilityVariant {
	return iapiserver.ProviderCapabilityVariant{ID: modelID + "-" + operationID, ModelID: modelID, OperationID: operationID, Lifecycle: iapiserver.ProviderLifecycle{Status: "active"}, InputSchema: map[string]any{"type": "object", "additionalProperties": true}, OutputSchema: map[string]any{"type": "object", "additionalProperties": true}}
}

func bilingual(chinese, english string) map[string]string {
	return map[string]string{"zh-CN": chinese, "en-US": english}
}
