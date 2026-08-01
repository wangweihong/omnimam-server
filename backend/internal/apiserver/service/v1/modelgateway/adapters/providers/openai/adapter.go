package openai

import (
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway/adapters/protocols/openaicompat"
)

// NewResponsesAdapter 构造 OpenAI Responses 官方服务适配器。
func NewResponsesAdapter() modelgateway.Adapter {
	return openaicompat.NewAdapter(ResponsesAdapterID)
}

// NewImagesAdapter 构造 OpenAI Images 官方服务适配器。
func NewImagesAdapter() modelgateway.Adapter {
	return openaicompat.NewAdapter(ImagesAdapterID)
}

// NewResponsesExecutor 构造 OpenAI Responses 非流式操作执行器。
func NewResponsesExecutor() modelgateway.OperationExecutor {
	return openaicompat.NewJSONExecutor(ResponsesExecutorID, "/responses")
}

// NewImagesExecutor 构造 OpenAI Images 非流式操作执行器。
func NewImagesExecutor() modelgateway.OperationExecutor {
	return openaicompat.NewJSONExecutor(ImagesExecutorID, "/images/generations")
}
