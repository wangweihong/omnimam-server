package deepseek

import (
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/protocols/openaicompat"
)

// NewAdapter 构造使用 OpenAI-compatible 线协议的 DeepSeek 官方适配器。
func NewAdapter() modelgateway.Adapter {
	return openaicompat.NewAdapter(AdapterID)
}

// NewExecutor 构造 DeepSeek Chat Completions 操作执行器。
func NewExecutor() modelgateway.OperationExecutor {
	return openaicompat.NewChatCompletionsExecutor(ExecutorID)
}
