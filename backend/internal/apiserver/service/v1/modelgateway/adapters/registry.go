package adapters

import (
	"fmt"

	appregistry "github.com/wangweihong/omnimam/backend/internal/apiserver/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/comfyui"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/google"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/modelark"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/ollama"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/openai"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/runninghub"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/xai"
)

// ValidateImplementations 确认静态 Registry 的每个 Adapter/Executor 声明都有可执行实现。
func ValidateImplementations(runtime *appregistry.RuntimeRegistry, engineAdapters map[string]modelgateway.Adapter, operationExecutors map[string]modelgateway.OperationExecutor) error {
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
	return nil
}

// NewRegistrations 显式汇总每个协议适配器拥有的静态 Registry 事实。
func NewRegistrations() []appregistry.Registration {
	registrations := []appregistry.Registration{comfyui.Registration(), modelark.Registration()}
	registrations = append(registrations, openai.Registrations()...)
	registrations = append(registrations, xai.Registration(), google.Registration(), ollama.Registration(), runninghub.Registration())
	return registrations
}

// NewEngineAdapters 构造由 Model Gateway bootstrap 注入的不可变引擎协议适配器集合。
func NewEngineAdapters() map[string]modelgateway.Adapter {
	return map[string]modelgateway.Adapter{
		comfyui.AdapterID:         comfyui.Adapter{},
		modelark.AdapterID:        modelark.Adapter{},
		openai.DeepSeekAdapterID:  openai.NewAdapter(openai.DeepSeekAdapterID),
		openai.ResponsesAdapterID: openai.NewAdapter(openai.ResponsesAdapterID),
		openai.ImagesAdapterID:    openai.NewAdapter(openai.ImagesAdapterID),
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
		openai.DeepSeekExecutorID:  openai.NewExecutor(openai.DeepSeekExecutorID),
		openai.ResponsesExecutorID: openai.NewJSONExecutor(openai.ResponsesExecutorID, "/responses"),
		openai.ImagesExecutorID:    openai.NewJSONExecutor(openai.ImagesExecutorID, "/images/generations"),
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
