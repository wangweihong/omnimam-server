package openai

import (
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
)

const (
	DeepSeekAdapterID   = "deepseek_official"
	DeepSeekExecutorID  = "deepseek_chat_completions"
	ResponsesAdapterID  = "openai_responses"
	ResponsesExecutorID = "openai_responses_create"
	ImagesAdapterID     = "openai_images"
	ImagesExecutorID    = "openai_images_generate"
)

// Registrations 返回 DeepSeek、OpenAI Responses 与 OpenAI Images 的独立静态注册。
func Registrations() []appregistry.Registration {
	return []appregistry.Registration{deepSeekRegistration(), responsesRegistration(), imagesRegistration()}
}

func deepSeekRegistration() appregistry.Registration {
	definition := textChatDefinition()
	models := []iapiserver.ProviderCapabilityModel{
		{ID: "deepseek-v4-pro", ProviderModelID: "deepseek-v4-pro", DisplayNameI18n: bilingual("DeepSeek V4 Pro", "DeepSeek V4 Pro"), DescriptionI18n: bilingual("DeepSeek V4 高质量对话模型。", "High-quality DeepSeek V4 chat model."), Family: "deepseek-v4", Variant: "pro", Lifecycle: iapiserver.ProviderLifecycle{Status: "active", AvailableSince: "2026-04-24"}, ContextWindowTokens: 1000000, MaximumOutputTokens: 384000, Limits: deepSeekLimits()},
		{ID: "deepseek-v4-flash", ProviderModelID: "deepseek-v4-flash", DisplayNameI18n: bilingual("DeepSeek V4 Flash", "DeepSeek V4 Flash"), DescriptionI18n: bilingual("DeepSeek V4 低延迟对话模型。", "Low-latency DeepSeek V4 chat model."), Family: "deepseek-v4", Variant: "flash", Lifecycle: iapiserver.ProviderLifecycle{Status: "active", AvailableSince: "2026-04-24"}, ContextWindowTokens: 1000000, MaximumOutputTokens: 384000, Limits: deepSeekLimits()},
	}
	operation := operation("chat-completions", definition.ID, "对话补全", "Chat Completions", "DeepSeek 官方 OpenAI-compatible 对话补全。", "Official DeepSeek OpenAI-compatible chat completions.")
	result := registration(
		DeepSeekAdapterID, DeepSeekExecutorID, "DeepSeek 官方 API", "DeepSeek Official API", "DeepSeek 官方 OpenAI-compatible 对话服务。", "Official DeepSeek OpenAI-compatible chat service.",
		"https://www.deepseek.com/", "https://api-docs.deepseek.com/api/create-chat-completion", "https://api.deepseek.com", definition,
		[]string{iapiserver.EngineAuthAPIKey}, "deepseek-official", "DeepSeek 官方能力", "DeepSeek Official Capability", "DeepSeek 官方稳定对话模型。", "Official stable DeepSeek chat models.",
		map[string]any{"code": "deepseek", "name": "DeepSeek", "model_owner": "DeepSeek", "serving_platform": "DeepSeek Official API", "official_website": "https://www.deepseek.com/"},
		[]map[string]any{
			{"type": "api_reference", "title": "Create Chat Completion", "url": "https://api-docs.deepseek.com/api/create-chat-completion", "checked_at": "2026-07-31", "scope": "Chat Completions request and response protocol."},
			{"type": "changelog", "title": "DeepSeek API Change Log", "url": "https://api-docs.deepseek.com/updates/", "checked_at": "2026-07-31", "scope": "V4 Pro and Flash availability and alias retirement."},
		},
		models, operation,
	)
	result.ProviderCapabilities[0].Variants = deepSeekVariants(models, operation.ID)
	result.ProviderCapabilities[0].Notes = []string{
		"Prefix Completion and FIM Completion beta capabilities are excluded.",
		"Provider credentials and base URL belong to the EngineInstance, not this catalog.",
	}
	return result
}

func deepSeekLimits() map[string]any {
	return map[string]any{"thinking_modes": []string{"enabled", "disabled"}, "reasoning_effort": []string{"high", "max"}, "maximum_tools": 128}
}

func deepSeekVariants(models []iapiserver.ProviderCapabilityModel, operationID string) []iapiserver.ProviderCapabilityVariant {
	variants := make([]iapiserver.ProviderCapabilityVariant, 0, len(models))
	for _, item := range models {
		variantID := map[string]string{"deepseek-v4-pro": "v4-pro-chat-completions", "deepseek-v4-flash": "v4-flash-chat-completions"}[item.ID]
		variants = append(variants, iapiserver.ProviderCapabilityVariant{
			ID: variantID, ModelID: item.ID, OperationID: operationID,
			Lifecycle: iapiserver.ProviderLifecycle{Status: "active"}, InputSchema: deepSeekInputSchema(), OutputSchema: deepSeekOutputSchema(),
			UnsupportedParameters: []string{"frequency_penalty", "presence_penalty", "max_completion_tokens", "developer_role"},
		})
	}
	return variants
}

func deepSeekInputSchema() map[string]any {
	return map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"messages"},
		"properties": map[string]any{
			"messages":         map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "object", "required": []string{"role", "content"}, "additionalProperties": false, "properties": map[string]any{"role": map[string]any{"type": "string", "enum": []string{"system", "user", "assistant", "tool"}}, "content": map[string]any{"type": []string{"string", "null"}}, "name": map[string]any{"type": "string"}, "reasoning_content": map[string]any{"type": []string{"string", "null"}}, "tool_call_id": map[string]any{"type": "string"}, "tool_calls": map[string]any{"type": "array"}}}},
			"thinking":         map[string]any{"type": "object", "required": []string{"type"}, "additionalProperties": false, "properties": map[string]any{"type": map[string]any{"type": "string", "enum": []string{"enabled", "disabled"}, "default": "enabled"}}},
			"reasoning_effort": map[string]any{"type": "string", "enum": []string{"high", "max"}, "default": "high"},
			"max_tokens":       map[string]any{"type": "integer", "minimum": 1, "maximum": 384000},
			"response_format":  map[string]any{"type": "object", "required": []string{"type"}, "additionalProperties": false, "properties": map[string]any{"type": map[string]any{"type": "string", "enum": []string{"text", "json_object"}, "default": "text"}}},
			"stop":             map[string]any{"type": []string{"string", "array"}, "items": map[string]any{"type": "string"}, "maxItems": 16},
			"stream":           map[string]any{"type": "boolean", "default": false},
			"stream_options":   map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"include_usage": map[string]any{"type": "boolean"}}},
			"temperature":      map[string]any{"type": "number", "minimum": 0, "maximum": 2, "default": 1},
			"top_p":            map[string]any{"type": "number", "minimum": 0, "maximum": 1, "default": 1},
			"tools":            map[string]any{"type": "array", "maxItems": 128, "items": map[string]any{"type": "object"}},
			"tool_choice":      map[string]any{"type": []string{"string", "object"}},
			"logprobs":         map[string]any{"type": "boolean"}, "top_logprobs": map[string]any{"type": "integer", "minimum": 0, "maximum": 20},
			"user_id": map[string]any{"type": "string", "pattern": "^[A-Za-z0-9_-]{1,512}$"},
		},
	}
}

func deepSeekOutputSchema() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false, "required": []string{"id", "model", "choices", "usage"}, "properties": map[string]any{"id": map[string]any{"type": "string"}, "model": map[string]any{"type": "string"}, "choices": map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "object"}}, "usage": map[string]any{"type": "object", "additionalProperties": true}, "system_fingerprint": map[string]any{"type": "string"}}}
}

func responsesRegistration() appregistry.Registration {
	definition := textResponsesDefinition()
	models := []iapiserver.ProviderCapabilityModel{
		model("gpt-5.6", "gpt-5.6", "GPT-5.6", "GPT-5.6", "GPT-5.6 默认别名，路由到 GPT-5.6 Sol。", "Default GPT-5.6 alias routing to GPT-5.6 Sol.", "gpt-5.6", "alias"),
		model("gpt-5.6-sol", "gpt-5.6-sol", "GPT-5.6 Sol", "GPT-5.6 Sol", "GPT-5.6 旗舰能力模型。", "Flagship GPT-5.6 model.", "gpt-5.6", "sol"),
		model("gpt-5.6-terra", "gpt-5.6-terra", "GPT-5.6 Terra", "GPT-5.6 Terra", "平衡质量和成本的 GPT-5.6 模型。", "GPT-5.6 model balancing quality and cost.", "gpt-5.6", "terra"),
		model("gpt-5.6-luna", "gpt-5.6-luna", "GPT-5.6 Luna", "GPT-5.6 Luna", "面向高吞吐场景的 GPT-5.6 模型。", "GPT-5.6 model for efficient high-volume workloads.", "gpt-5.6", "luna"),
	}
	operation := operation("responses", definition.ID, "Responses 响应", "Responses", "OpenAI 官方 Responses API；ChatGPT 是产品名称而不是 API model ID。", "Official OpenAI Responses API; ChatGPT is a product name, not an API model ID.")
	return registration(
		ResponsesAdapterID, ResponsesExecutorID, "OpenAI Responses", "OpenAI Responses", "OpenAI 官方 Responses API 服务。", "Official OpenAI Responses API service.",
		"https://openai.com/", "https://developers.openai.com/api/reference/resources/responses/methods/create/", "https://api.openai.com/v1", definition,
		[]string{iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthBearerToken}, "openai-responses", "OpenAI Responses 能力", "OpenAI Responses Capability", "GPT-5.6 系列 Responses 能力。", "GPT-5.6 family Responses capability.",
		map[string]any{"code": "openai", "name": "OpenAI", "model_owner": "OpenAI", "serving_platform": "OpenAI API", "official_website": "https://openai.com/"},
		[]map[string]any{{"type": "api_reference", "title": "Create a model response", "url": "https://developers.openai.com/api/reference/resources/responses/methods/create/", "checked_at": "2026-07-31", "scope": "POST /v1/responses and current GPT-5.6 model IDs."}},
		models, operation,
	)
}

func imagesRegistration() appregistry.Registration {
	definition := imageGenerationDefinition()
	models := []iapiserver.ProviderCapabilityModel{
		model("gpt-image-2", "gpt-image-2", "GPT Image 2", "GPT Image 2", "OpenAI 当前 GPT Image 2 图像生成模型。", "Current OpenAI GPT Image 2 generation model.", "gpt-image", "2"),
	}
	operation := operation("image-generations", definition.ID, "图像生成", "Image Generations", "通过 OpenAI Images API 生成图像。", "Generate images through the OpenAI Images API.")
	return registration(
		ImagesAdapterID, ImagesExecutorID, "OpenAI 图像", "OpenAI Images", "OpenAI 官方图像生成 API 服务。", "Official OpenAI image generation API service.",
		"https://openai.com/", "https://developers.openai.com/api/reference/resources/images/methods/generate/", "https://api.openai.com/v1", definition,
		[]string{iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthBearerToken}, "openai-images", "OpenAI 图像能力", "OpenAI Images Capability", "GPT Image 2 图像生成能力。", "GPT Image 2 generation capability.",
		map[string]any{"code": "openai", "name": "OpenAI", "model_owner": "OpenAI", "serving_platform": "OpenAI API", "official_website": "https://openai.com/"},
		[]map[string]any{{"type": "model_card", "title": "GPT Image 2", "url": "https://developers.openai.com/api/docs/models/gpt-image-2", "checked_at": "2026-07-31", "scope": "gpt-image-2 and POST /v1/images/generations."}},
		models, operation,
	)
}

func registration(adapterID, executorID, nameZH, nameEN, descriptionZH, descriptionEN, website, documentation, baseURL string, definition iapiserver.CapabilityDefinition, authTypes []string, capabilityID, capabilityNameZH, capabilityNameEN, capabilityDescriptionZH, capabilityDescriptionEN string, provider map[string]any, sources []map[string]any, models []iapiserver.ProviderCapabilityModel, operation iapiserver.ProviderCapabilityOperation) appregistry.Registration {
	variants := make([]iapiserver.ProviderCapabilityVariant, 0, len(models))
	for _, item := range models {
		variants = append(variants, iapiserver.ProviderCapabilityVariant{ID: item.ID + "-" + operation.ID, ModelID: item.ID, OperationID: operation.ID, Lifecycle: iapiserver.ProviderLifecycle{Status: "active"}, InputSchema: map[string]any{"type": "object", "additionalProperties": true}, OutputSchema: map[string]any{"type": "object", "additionalProperties": true}})
	}
	return appregistry.Registration{
		CapabilityDefinitions: []iapiserver.CapabilityDefinition{definition},
		EngineAdapter:         iapiserver.EngineAdapterDefinition{ID: adapterID, Responsibilities: []string{"authentication", "base_url", "health_check", "common_error_mapping"}},
		OperationExecutors:    []iapiserver.OperationExecutorDefinition{{ID: executorID, EngineAdapterID: adapterID, CapabilityDefinitionIDs: []string{definition.ID}}},
		EngineType:            iapiserver.ApplicationEngineType{ID: adapterID, NameI18n: bilingual(nameZH, nameEN), DescriptionI18n: bilingual(descriptionZH, descriptionEN), OfficialWebsiteURL: website, OfficialDocumentationURL: documentation, DefaultAPIBaseURL: baseURL, Enabled: true, EngineAdapterID: adapterID, AuthenticationTypes: authTypes, AuthenticationConfigSchema: appregistry.AuthenticationConfigSchema(authTypes...), OperationExecutors: map[string]string{definition.ID: executorID}},
		ProviderCapabilities:  []iapiserver.AIAppProviderCapability{{SchemaVersion: "1.0", ID: capabilityID, NameI18n: bilingual(capabilityNameZH, capabilityNameEN), DescriptionI18n: bilingual(capabilityDescriptionZH, capabilityDescriptionEN), Kind: iapiserver.ProviderCapabilityKindCatalog, Origin: iapiserver.ProviderCapabilityOriginStatic, BindingPolicy: iapiserver.ProviderBindingPolicyManual, ApplicationEngineTypeID: adapterID, Revision: "2026-07-31.1", Enabled: true, Provider: provider, Sources: sources, Models: models, Operations: []iapiserver.ProviderCapabilityOperation{operation}, Variants: variants}},
	}
}

func textChatDefinition() iapiserver.CapabilityDefinition {
	return iapiserver.CapabilityDefinition{ID: "text.chat_completion", NameI18n: bilingual("对话补全", "Chat Completion"), InputMediaTypes: []string{"text", "json"}, OutputMediaTypes: []string{"text", "json"}}
}

func textResponsesDefinition() iapiserver.CapabilityDefinition {
	return iapiserver.CapabilityDefinition{ID: "text.responses", NameI18n: bilingual("统一响应", "Responses"), InputMediaTypes: []string{"text", "image", "json"}, OutputMediaTypes: []string{"text", "json"}}
}

func imageGenerationDefinition() iapiserver.CapabilityDefinition {
	return iapiserver.CapabilityDefinition{ID: "image.text_to_image", NameI18n: bilingual("文生图", "Text to Image"), InputMediaTypes: []string{"text"}, OutputMediaTypes: []string{"image"}}
}

func model(id, providerID, displayZH, displayEN, descriptionZH, descriptionEN, family, variant string) iapiserver.ProviderCapabilityModel {
	return iapiserver.ProviderCapabilityModel{ID: id, ProviderModelID: providerID, DisplayNameI18n: bilingual(displayZH, displayEN), DescriptionI18n: bilingual(descriptionZH, descriptionEN), Family: family, Variant: variant, Lifecycle: iapiserver.ProviderLifecycle{Status: "active"}}
}

func operation(id, capabilityID, nameZH, nameEN, descriptionZH, descriptionEN string) iapiserver.ProviderCapabilityOperation {
	return iapiserver.ProviderCapabilityOperation{ID: id, CapabilityDefinitionID: capabilityID, NameI18n: bilingual(nameZH, nameEN), DescriptionI18n: bilingual(descriptionZH, descriptionEN), ExecutionMode: "synchronous", InputMediaTypes: []string{"text", "image", "json"}, OutputMediaTypes: []string{"text", "image", "json"}}
}

func bilingual(chinese, english string) map[string]string {
	return map[string]string{"zh-CN": chinese, "en-US": english}
}
