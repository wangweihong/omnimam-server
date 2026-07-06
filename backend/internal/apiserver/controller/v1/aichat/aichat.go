package aichat

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	srvv1 "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1"
	aichatsrv "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/aichat"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

type AIChatController struct {
	srv srvv1.Service
}

func NewController(storeIns store.Factory) *AIChatController {
	return &AIChatController{srv: srvv1.NewService(storeIns)}
}

// ListModels 返回当前用户可用于 AI 聊天的模型摘要，只暴露 metadata 和 capability。
func (ac *AIChatController) ListModels(c *gin.Context) {
	core.Run(c, &iapiserver.AIChatModelListRequest{}, func(r *iapiserver.AIChatModelListRequest) (any, error) {
		return ac.srv.AIChat().ListModels(c, r)
	})
}

// ListAssistants 返回系统助手和当前用户助手，不返回其他用户助手。
func (ac *AIChatController) ListAssistants(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return ac.srv.AIChat().ListAssistants(c)
	})
}

// CreateAssistant 创建当前用户助手；系统助手只能由系统预置，不由该接口创建。
func (ac *AIChatController) CreateAssistant(c *gin.Context) {
	core.Run(c, &iapiserver.AIChatAssistantUpsertRequest{}, func(r *iapiserver.AIChatAssistantUpsertRequest) (any, error) {
		return ac.srv.AIChat().CreateAssistant(c, r)
	})
}

// UpdateAssistant 更新当前用户助手；系统助手名称受保护。
func (ac *AIChatController) UpdateAssistant(c *gin.Context) {
	req := &iapiserver.AIChatAssistantUpsertRequest{ID: c.Param("assistant_id")}
	core.Run(c, req, func(r *iapiserver.AIChatAssistantUpsertRequest) (any, error) {
		return ac.srv.AIChat().UpdateAssistant(c, r)
	})
}

// DeleteAssistant 删除当前用户非系统助手，系统助手返回业务错误码。
func (ac *AIChatController) DeleteAssistant(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return ac.srv.AIChat().DeleteAssistant(c, c.Param("assistant_id"))
	})
}

// ListTopics 按当前用户隔离返回话题列表。
func (ac *AIChatController) ListTopics(c *gin.Context) {
	core.Run(c, &iapiserver.AIChatTopicListRequest{}, func(r *iapiserver.AIChatTopicListRequest) (any, error) {
		return ac.srv.AIChat().ListTopics(c, r)
	})
}

// CreateTopic 创建当前用户话题，不创建跨用户共享入口。
func (ac *AIChatController) CreateTopic(c *gin.Context) {
	core.Run(c, &iapiserver.AIChatTopicCreateRequest{}, func(r *iapiserver.AIChatTopicCreateRequest) (any, error) {
		return ac.srv.AIChat().CreateTopic(c, r)
	})
}

// UpdateTopic 更新当前用户话题标题、置顶、助手或模型选择。
func (ac *AIChatController) UpdateTopic(c *gin.Context) {
	req := &iapiserver.AIChatTopicUpdateRequest{ID: c.Param("topic_id")}
	core.Run(c, req, func(r *iapiserver.AIChatTopicUpdateRequest) (any, error) {
		return ac.srv.AIChat().UpdateTopic(c, r)
	})
}

// DeleteTopic 软删除当前用户话题和消息视图。
func (ac *AIChatController) DeleteTopic(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return ac.srv.AIChat().DeleteTopic(c, c.Param("topic_id"))
	})
}

// ListMessages 按 created_at asc、version asc 返回当前用户话题消息。
func (ac *AIChatController) ListMessages(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return ac.srv.AIChat().ListMessages(c, c.Param("topic_id"))
	})
}

// CreateMessage 创建聊天或翻译请求；chat 返回 SSE，translate 返回 JSON DTO。
func (ac *AIChatController) CreateMessage(c *gin.Context) {
	req := &iapiserver.AIChatMessageCreateRequest{TopicID: c.Param("topic_id")}
	if err := core.DecodeParameter(c, req); err != nil {
		core.WriteResponse(c, err, nil)
		return
	}
	result, err := ac.srv.AIChat().CreateMessage(c, req)
	if err != nil {
		core.WriteResponse(c, err, nil)
		return
	}
	if req.Operation == iapiserver.AIChatOperationTranslate {
		core.WriteResponse(c, nil, result.Translation)
		return
	}
	writeAIChatSSE(c, result)
}

// StopGeneration 原子停止当前用户 generation，并更新 assistant message 状态。
func (ac *AIChatController) StopGeneration(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return ac.srv.AIChat().StopGeneration(c, c.Param("generation_id"))
	})
}

// RegenerateMessage 对当前用户 assistant message 重新生成并返回 SSE。
func (ac *AIChatController) RegenerateMessage(c *gin.Context) {
	result, err := ac.srv.AIChat().RegenerateMessage(c, c.Param("message_id"))
	if err != nil {
		core.WriteResponse(c, err, nil)
		return
	}
	writeAIChatSSE(c, result)
}

// EditRegenerateMessage 编辑 user message 后截断上下文并重新生成。
func (ac *AIChatController) EditRegenerateMessage(c *gin.Context) {
	req := &iapiserver.AIChatEditRegenerateRequest{MessageID: c.Param("message_id")}
	if err := core.DecodeParameter(c, req); err != nil {
		core.WriteResponse(c, err, nil)
		return
	}
	result, err := ac.srv.AIChat().EditRegenerateMessage(c, req)
	if err != nil {
		core.WriteResponse(c, err, nil)
		return
	}
	writeAIChatSSE(c, result)
}

// BranchMessage 从当前用户 assistant message 继续原话题或创建分支话题。
func (ac *AIChatController) BranchMessage(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return ac.srv.AIChat().BranchMessage(c, c.Param("message_id"))
	})
}

// ListQuickPhrases 返回当前用户全局和助手级快捷短语。
func (ac *AIChatController) ListQuickPhrases(c *gin.Context) {
	core.Run(c, &iapiserver.AIChatQuickPhraseListRequest{}, func(r *iapiserver.AIChatQuickPhraseListRequest) (any, error) {
		return ac.srv.AIChat().ListQuickPhrases(c, r)
	})
}

// CreateQuickPhrase 创建当前用户快捷短语。
func (ac *AIChatController) CreateQuickPhrase(c *gin.Context) {
	core.Run(c, &iapiserver.AIChatQuickPhraseUpsertRequest{}, func(r *iapiserver.AIChatQuickPhraseUpsertRequest) (any, error) {
		return ac.srv.AIChat().CreateQuickPhrase(c, r)
	})
}

// UpdateQuickPhrase 更新当前用户快捷短语。
func (ac *AIChatController) UpdateQuickPhrase(c *gin.Context) {
	req := &iapiserver.AIChatQuickPhraseUpsertRequest{ID: c.Param("quick_phrase_id")}
	core.Run(c, req, func(r *iapiserver.AIChatQuickPhraseUpsertRequest) (any, error) {
		return ac.srv.AIChat().UpdateQuickPhrase(c, r)
	})
}

// DeleteQuickPhrase 删除当前用户快捷短语。
func (ac *AIChatController) DeleteQuickPhrase(c *gin.Context) {
	core.Run(c, nil, func(_ any) (any, error) {
		return ac.srv.AIChat().DeleteQuickPhrase(c, c.Param("quick_phrase_id"))
	})
}

func writeAIChatSSE(c *gin.Context, result *aichatsrv.MessageCreateResult) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)
	if result == nil || result.Bundle == nil {
		c.SSEvent("failed", gin.H{"message": "generation result is empty"})
		return
	}
	c.SSEvent("generation", result.Bundle.Generation)
	if result.Err != nil {
		c.SSEvent("failed", gin.H{
			"generation_id": result.Bundle.Generation.ID,
			"code":          "AI_CHAT_MODEL_UNAVAILABLE",
			"message":       result.Err.Error(),
		})
		return
	}
	if result.Content != "" {
		c.SSEvent("delta", gin.H{
			"message_id": result.Bundle.AssistantMessage.ID,
			"delta":      result.Content,
		})
	}
	c.SSEvent("done", gin.H{
		"generation_id": result.Bundle.Generation.ID,
		"message_id":    result.Bundle.AssistantMessage.ID,
	})
}
