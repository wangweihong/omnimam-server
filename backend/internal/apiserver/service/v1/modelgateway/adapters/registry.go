package adapters

import (
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/comfyui"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/modelark"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/openai"
)

// NewEngineAdapters 构造由 Model Gateway bootstrap 注入的不可变引擎协议适配器集合。
func NewEngineAdapters() map[string]modelgateway.Adapter {
	return map[string]modelgateway.Adapter{
		comfyui.AdapterID:   comfyui.Adapter{},
		"deepseek_official": openai.NewAdapter("deepseek_official"),
		modelark.AdapterID:  modelark.Adapter{},
	}
}

// NewOperationExecutors 构造 Runtime Registry 引用的不可变 Provider 操作执行器集合。
func NewOperationExecutors() map[string]modelgateway.OperationExecutor {
	executors := map[string]modelgateway.OperationExecutor{
		comfyui.ExecutorID:          comfyui.Executor{},
		"deepseek_chat_completions": openai.NewExecutor("deepseek_chat_completions"),
	}
	for _, id := range modelark.ExecutorIDs {
		executors[id] = modelark.NewExecutor(id)
	}
	return executors
}
