package adapters

import (
	"fmt"

	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/providers/comfyui"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/providers/deepseek"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/providers/google"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/providers/modelark"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/providers/ollama"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/providers/openai"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/providers/openaicompat"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/providers/runninghub"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/providers/xai"
)

// ValidateImplementations 确认静态 Registry 的每个 Adapter/Executor 声明都有可执行实现。
func ValidateImplementations(runtime *modelgateway.RuntimeRegistry, engineAdapters map[string]modelgateway.Adapter, operationExecutors map[string]modelgateway.OperationExecutor) error {
	if runtime == nil {
		return fmt.Errorf("runtime registry is required")
	}
	for _, engineType := range runtime.EngineTypes() {
		adapter := engineAdapters[engineType.EngineAdapterID]
		if adapter == nil || adapter.ID() != engineType.EngineAdapterID {
			return fmt.Errorf("engine type %s has no implementation for adapter %s", engineType.ID, engineType.EngineAdapterID)
		}
		for capabilityID := range engineType.OperationExecutors {
			definition, ok := runtime.OperationExecutor(engineType.ID, capabilityID)
			if !ok {
				return fmt.Errorf("engine type %s has no executor definition for capability %s", engineType.ID, capabilityID)
			}
			executor := operationExecutors[definition.ID]
			if executor == nil || executor.ID() != definition.ID {
				return fmt.Errorf("engine type %s has no implementation for executor %s", engineType.ID, definition.ID)
			}
		}
	}
	return runtime.ValidateProviderTypeImplementations(engineAdapters, operationExecutors)
}

// NewRegistrations 显式汇总每个提供商包拥有的静态 Registry 事实。
func NewRegistrations() ([]modelgateway.Registration, error) {
	comfyUIRegistration, err := comfyui.Registration()
	if err != nil {
		return nil, fmt.Errorf("parse comfyui capability manifest: %w", err)
	}
	deepSeekRegistration, err := deepseek.Registration()
	if err != nil {
		return nil, fmt.Errorf("parse deepseek capability manifest: %w", err)
	}
	openAIRegistrations, err := openai.Registrations()
	if err != nil {
		return nil, fmt.Errorf("parse openai capability manifest: %w", err)
	}
	xAIRegistration, err := xai.Registration()
	if err != nil {
		return nil, fmt.Errorf("parse xai capability manifest: %w", err)
	}
	googleRegistration, err := google.Registration()
	if err != nil {
		return nil, fmt.Errorf("parse google capability manifest: %w", err)
	}
	ollamaRegistration, err := ollama.Registration()
	if err != nil {
		return nil, fmt.Errorf("parse ollama capability manifest: %w", err)
	}
	runningHubRegistration, err := runninghub.Registration()
	if err != nil {
		return nil, fmt.Errorf("parse runninghub capability manifest: %w", err)
	}
	modelArkRegistration, err := modelark.Registration()
	if err != nil {
		return nil, fmt.Errorf("parse modelark capability manifest: %w", err)
	}
	registrations := []modelgateway.Registration{comfyUIRegistration, deepSeekRegistration, modelArkRegistration, openaicompat.Registration()}
	registrations = append(registrations, openAIRegistrations...)
	registrations = append(registrations, xAIRegistration, googleRegistration, ollamaRegistration, runningHubRegistration)
	return registrations, nil
}

// NewEngineAdapters 构造由 Model Gateway bootstrap 注入的不可变提供商适配器集合。
func NewEngineAdapters() map[string]modelgateway.Adapter {
	return map[string]modelgateway.Adapter{
		comfyui.AdapterID:         comfyui.Adapter{},
		modelark.AdapterID:        modelark.Adapter{},
		deepseek.AdapterID:        deepseek.NewAdapter(),
		openai.ResponsesAdapterID: openai.NewResponsesAdapter(),
		openai.ImagesAdapterID:    openai.NewImagesAdapter(),
		openaicompat.AdapterID:    openaicompat.NewAdapter(),
		xai.AdapterID:             xai.Adapter{},
		google.AdapterID:          google.Adapter{},
		ollama.AdapterID:          ollama.Adapter{},
		runninghub.AdapterID:      runninghub.Adapter{},
	}
}

// NewOperationExecutors 构造 Runtime Registry 引用的不可变 Provider 操作执行器集合。
func NewOperationExecutors() map[string]modelgateway.OperationExecutor {
	executors := map[string]modelgateway.OperationExecutor{
		comfyui.ExecutorID:         comfyui.Executor{},
		deepseek.ExecutorID:        deepseek.NewExecutor(),
		openai.ResponsesExecutorID: openai.NewResponsesExecutor(),
		openai.ImagesExecutorID:    openai.NewImagesExecutor(),
		openaicompat.ExecutorID:    openaicompat.NewExecutor(),
		xai.ExecutorID:             xai.NewExecutor(),
		google.ExecutorID:          google.Executor{},
		ollama.ChatExecutorID:      ollama.NewExecutor(ollama.ChatExecutorID, "/chat/completions"),
		ollama.ResponsesExecutorID: ollama.NewExecutor(ollama.ResponsesExecutorID, "/responses"),
		runninghub.ExecutorID:      runninghub.Executor{},
	}
	for _, id := range modelark.ExecutorIDs {
		executors[id] = modelark.NewExecutor(id)
	}
	return executors
}

// NewCapabilityValidators 构造 manifest 命名复杂校验器集合。
func NewCapabilityValidators() []modelgateway.CapabilityValidator {
	return []modelgateway.CapabilityValidator{ollama.NewModelInstalledValidator()}
}
