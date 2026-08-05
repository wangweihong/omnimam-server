package openaicompat

import (
	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	protocol "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/protocols/openaicompat"
)

const (
	AdapterID  = "openai_compatible"
	ExecutorID = "openai_chat_completions"
)

func Registration() modelgateway.Registration {
	capabilities := []iapiserver.CapabilityDefinition{
		{ID: "text.chat_completion", NameI18n: bilingual("对话补全", "Chat Completion"), InputMediaTypes: []string{"text", "json"}, OutputMediaTypes: []string{"text", "json"}},
		{ID: "text.translate", NameI18n: bilingual("文本翻译", "Text Translation"), InputMediaTypes: []string{"text", "json"}, OutputMediaTypes: []string{"text", "json"}},
		{ID: "image.understanding", NameI18n: bilingual("图片理解", "Image Understanding"), InputMediaTypes: []string{"text", "image", "json"}, OutputMediaTypes: []string{"text", "json"}},
	}
	capabilityIDs := []string{capabilities[0].ID, capabilities[1].ID, capabilities[2].ID}
	return modelgateway.Registration{
		CapabilityDefinitions: capabilities,
		EngineAdapter: iapiserver.EngineAdapterDefinition{
			ID:               AdapterID,
			Responsibilities: []string{"authentication", "base_url", "model_discovery", "model_probe", "health_check", "common_error_mapping"},
		},
		OperationExecutors: []iapiserver.OperationExecutorDefinition{{ID: ExecutorID, EngineAdapterID: AdapterID, CapabilityDefinitionIDs: capabilityIDs}},
		ProviderType: &modelgateway.ProviderTypeRegistration{
			ID: iapiserver.ProviderTypeOpenAICompatible, DisplayName: "OpenAI Compatible",
			AuthenticationTypes: []string{iapiserver.EngineAuthNone, iapiserver.EngineAuthAPIKey, iapiserver.EngineAuthBearerToken},
			ConfigurationSchema: map[string]any{
				"type": "object", "additionalProperties": false,
				"properties": map[string]any{"organization": map[string]any{"type": "string"}, "project": map[string]any{"type": "string"}},
			},
			SupportsModelDiscovery: true, SupportsModelProbe: true, AdapterID: AdapterID,
			OperationExecutors: map[string]string{
				capabilities[0].ID: ExecutorID,
				capabilities[1].ID: ExecutorID,
				capabilities[2].ID: ExecutorID,
			},
		},
	}
}

func NewAdapter() modelgateway.Adapter { return protocol.NewAdapter(AdapterID) }

func NewExecutor() modelgateway.OperationExecutor {
	return protocol.NewChatCompletionsExecutor(ExecutorID)
}

func bilingual(chinese, english string) map[string]string {
	return map[string]string{"zh-CN": chinese, "en-US": english}
}
