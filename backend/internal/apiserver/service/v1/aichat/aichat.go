package aichat

import (
	"context"
	stderrors "errors"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/provider"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/internal/pkg/ctxvalue"
)

type AIChatSrv interface {
	ListAssistants(ctx context.Context) (*iapiserver.AIChatAssistantListResponse, error)
	CreateAssistant(ctx context.Context, req *iapiserver.AIChatAssistantUpsertRequest) (*iapiserver.AIChatAssistant, error)
	UpdateAssistant(ctx context.Context, req *iapiserver.AIChatAssistantUpsertRequest) (*iapiserver.AIChatAssistant, error)
	DeleteAssistant(ctx context.Context, id string) (*iapiserver.AIChatDeleteResponse, error)
	ListTopics(ctx context.Context, req *iapiserver.AIChatTopicListRequest) (*iapiserver.AIChatTopicListResponse, error)
	GetTopic(ctx context.Context, id string) (*iapiserver.AIChatTopic, error)
	CreateTopic(ctx context.Context, req *iapiserver.AIChatTopicCreateRequest) (*iapiserver.AIChatTopic, error)
	UpdateTopic(ctx context.Context, req *iapiserver.AIChatTopicUpdateRequest) (*iapiserver.AIChatTopic, error)
	DeleteTopic(ctx context.Context, id string) (*iapiserver.AIChatDeleteResponse, error)
	ListMessages(ctx context.Context, topicID string) (*iapiserver.AIChatMessageListResponse, error)
	CreateMessage(ctx context.Context, req *iapiserver.AIChatMessageCreateRequest) (*MessageCreateResult, error)
	StopGeneration(ctx context.Context, generationID string) (*iapiserver.AIChatGeneration, error)
	RegenerateMessage(ctx context.Context, messageID string) (*MessageCreateResult, error)
	EditRegenerateMessage(ctx context.Context, req *iapiserver.AIChatEditRegenerateRequest) (*MessageCreateResult, error)
	BranchMessage(ctx context.Context, messageID string) (*iapiserver.AIChatTopic, error)
	ListQuickPhrases(ctx context.Context, req *iapiserver.AIChatQuickPhraseListRequest) (*iapiserver.AIChatQuickPhraseListResponse, error)
	CreateQuickPhrase(ctx context.Context, req *iapiserver.AIChatQuickPhraseUpsertRequest) (*iapiserver.AIChatQuickPhrase, error)
	UpdateQuickPhrase(ctx context.Context, req *iapiserver.AIChatQuickPhraseUpsertRequest) (*iapiserver.AIChatQuickPhrase, error)
	DeleteQuickPhrase(ctx context.Context, id string) (*iapiserver.AIChatDeleteResponse, error)
	TranslateContent(ctx context.Context, req *iapiserver.AIChatTranslationRequest) (*iapiserver.AIChatMessageTranslation, error)
}

type MessageCreateResult struct {
	Translation *iapiserver.AIChatTranslationResponse
	Bundle      *store.AIChatGenerationBundle
	Content     string
	Err         error
}

type aiChatService struct {
	store       store.Factory
	modelReader ModelSummaryReader
}

// NewService 创建 AI Chat 服务，并可注入 model-management 的受控模型摘要读取能力。
func NewService(str store.Factory, modelReaders ...ModelSummaryReader) AIChatSrv {
	service := &aiChatService{store: str}
	if len(modelReaders) > 0 {
		service.modelReader = modelReaders[0]
	}
	return service
}

func (s *aiChatService) ListAssistants(ctx context.Context) (*iapiserver.AIChatAssistantListResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.store.AIChat().ListAssistants(ctx, userID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatAssistantNotFound, "assistant not found")
	}
	if err := attachAssistantRelations(ctx, s.modelReader, userID, items); err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.AIChatAssistantListResponse{Items: items}, nil
}

func (s *aiChatService) CreateAssistant(
	ctx context.Context,
	req *iapiserver.AIChatAssistantUpsertRequest,
) (*iapiserver.AIChatAssistant, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	assistant := assistantFromRequest(req)
	created, err := s.store.AIChat().CreateAssistant(ctx, userID, assistant)
	if err != nil {
		return nil, err
	}
	if err := attachAssistantRelations(ctx, s.modelReader, userID, []*iapiserver.AIChatAssistant{created}); err != nil {
		return nil, errors.WithStack(err)
	}
	return created, nil
}

func (s *aiChatService) UpdateAssistant(
	ctx context.Context,
	req *iapiserver.AIChatAssistantUpsertRequest,
) (*iapiserver.AIChatAssistant, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	assistant := assistantFromRequest(req)
	assistant.ID = req.ID
	updated, err := s.store.AIChat().UpdateAssistant(ctx, userID, assistant)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatAssistantNotFound, "assistant not found")
	}
	if err := attachAssistantRelations(ctx, s.modelReader, userID, []*iapiserver.AIChatAssistant{updated}); err != nil {
		return nil, errors.WithStack(err)
	}
	return updated, nil
}

func (s *aiChatService) DeleteAssistant(ctx context.Context, id string) (*iapiserver.AIChatDeleteResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.store.AIChat().DeleteAssistant(ctx, userID, id); err != nil {
		return nil, mapNotFound(err, code.ErrAIChatAssistantNotFound, "assistant not found")
	}
	return &iapiserver.AIChatDeleteResponse{Deleted: true}, nil
}

func (s *aiChatService) ListTopics(
	ctx context.Context,
	req *iapiserver.AIChatTopicListRequest,
) (*iapiserver.AIChatTopicListResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	items, total, err := s.store.AIChat().ListTopics(ctx, userID, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if err := attachTopicRelations(ctx, s.store.AIChat(), s.modelReader, userID, items); err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.AIChatTopicListResponse{Total: total, Items: items}, nil
}

func (s *aiChatService) GetTopic(ctx context.Context, id string) (*iapiserver.AIChatTopic, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	topic, err := s.store.AIChat().GetTopic(ctx, userID, id)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatTopicNotFound, "topic not found")
	}
	if err := attachTopicRelations(ctx, s.store.AIChat(), s.modelReader, userID, []*iapiserver.AIChatTopic{topic}); err != nil {
		return nil, errors.WithStack(err)
	}
	return topic, nil
}

func (s *aiChatService) CreateTopic(
	ctx context.Context,
	req *iapiserver.AIChatTopicCreateRequest,
) (*iapiserver.AIChatTopic, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	topic := &iapiserver.AIChatTopic{
		Title:       req.Title,
		AssistantID: req.AssistantID,
		ModelID:     req.ModelID,
	}
	created, err := s.store.AIChat().CreateTopic(ctx, userID, topic)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if err := attachTopicRelations(ctx, s.store.AIChat(), s.modelReader, userID, []*iapiserver.AIChatTopic{created}); err != nil {
		return nil, errors.WithStack(err)
	}
	return created, nil
}

func (s *aiChatService) UpdateTopic(
	ctx context.Context,
	req *iapiserver.AIChatTopicUpdateRequest,
) (*iapiserver.AIChatTopic, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	topic := &iapiserver.AIChatTopic{
		ObjectMeta: imachinery.ObjectMeta{ID: req.ID},
	}
	if req.Title != nil {
		topic.Title = *req.Title
	}
	if req.Pinned != nil {
		topic.Pinned = *req.Pinned
	}
	if req.AssistantID != nil {
		topic.AssistantID = *req.AssistantID
	}
	if req.ModelID != nil {
		topic.ModelID = *req.ModelID
	}
	updated, err := s.store.AIChat().UpdateTopic(ctx, userID, topic)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatTopicNotFound, "topic not found")
	}
	if err := attachTopicRelations(ctx, s.store.AIChat(), s.modelReader, userID, []*iapiserver.AIChatTopic{updated}); err != nil {
		return nil, errors.WithStack(err)
	}
	return updated, nil
}

func (s *aiChatService) DeleteTopic(ctx context.Context, id string) (*iapiserver.AIChatDeleteResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.store.AIChat().DeleteTopic(ctx, userID, id); err != nil {
		return nil, mapNotFound(err, code.ErrAIChatTopicNotFound, "topic not found")
	}
	return &iapiserver.AIChatDeleteResponse{Deleted: true}, nil
}

func (s *aiChatService) ListMessages(
	ctx context.Context,
	topicID string,
) (*iapiserver.AIChatMessageListResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.store.AIChat().GetTopic(ctx, userID, topicID); err != nil {
		return nil, mapNotFound(err, code.ErrAIChatTopicNotFound, "topic not found")
	}
	items, err := s.store.AIChat().ListMessages(ctx, userID, topicID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.AIChatMessageListResponse{Items: items}, nil
}

func (s *aiChatService) CreateMessage(
	ctx context.Context,
	req *iapiserver.AIChatMessageCreateRequest,
) (*MessageCreateResult, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	topic, err := s.store.AIChat().GetTopic(ctx, userID, req.TopicID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatTopicNotFound, "topic not found")
	}
	if req.Operation == iapiserver.AIChatOperationTranslate {
		return s.translate(ctx, userID, topic, req)
	}
	model, assistant, err := s.resolveModelAndAssistant(ctx, userID, topic, req.ModelID, req.AssistantID, len(req.Images) > 0)
	if err != nil {
		return nil, err
	}
	bundle, err := s.store.AIChat().CreateMessageGeneration(ctx, userID, topic, req, model, assistant)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatGenerationConflict, "generation conflict")
	}
	content, invokeErr := s.invokeProvider(ctx, model, req.Content)
	if invokeErr != nil {
		_ = s.store.AIChat().FailGeneration(ctx, userID, bundle.Generation.ID, "AI_CHAT_MODEL_UNAVAILABLE", invokeErr.Error())
		return &MessageCreateResult{Bundle: bundle, Err: invokeErr}, nil
	}
	if _, err := s.store.AIChat().CompleteGeneration(ctx, userID, bundle.Generation.ID, content); err != nil {
		return nil, errors.WithStack(err)
	}
	return &MessageCreateResult{Bundle: bundle, Content: content}, nil
}

func (s *aiChatService) StopGeneration(
	ctx context.Context,
	generationID string,
) (*iapiserver.AIChatGeneration, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	generation, err := s.store.AIChat().StopGeneration(ctx, userID, generationID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatGenerationNotFound, "generation not found")
	}
	return generation, nil
}

func (s *aiChatService) RegenerateMessage(ctx context.Context, messageID string) (*MessageCreateResult, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	source, err := s.store.AIChat().GetMessage(ctx, userID, messageID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatMessageNotFound, "message not found")
	}
	topic, err := s.store.AIChat().GetTopic(ctx, userID, source.TopicID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatTopicNotFound, "topic not found")
	}
	model, assistant, err := s.resolveModelAndAssistant(ctx, userID, topic, topic.ModelID, topic.AssistantID, false)
	if err != nil {
		return nil, err
	}
	req := &iapiserver.AIChatMessageCreateRequest{
		TopicID:   topic.ID,
		Operation: iapiserver.AIChatOperationChat,
		Content:   "regenerate",
	}
	bundle, err := s.store.AIChat().CreateMessageGeneration(ctx, userID, topic, req, model, assistant)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatGenerationConflict, "generation conflict")
	}
	content, invokeErr := s.invokeProvider(ctx, model, source.Content)
	if invokeErr != nil {
		_ = s.store.AIChat().FailGeneration(ctx, userID, bundle.Generation.ID, "AI_CHAT_MODEL_UNAVAILABLE", invokeErr.Error())
		return &MessageCreateResult{Bundle: bundle, Err: invokeErr}, nil
	}
	if _, err := s.store.AIChat().CompleteGeneration(ctx, userID, bundle.Generation.ID, content); err != nil {
		return nil, err
	}
	return &MessageCreateResult{Bundle: bundle, Content: content}, nil
}

func (s *aiChatService) EditRegenerateMessage(
	ctx context.Context,
	req *iapiserver.AIChatEditRegenerateRequest,
) (*MessageCreateResult, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	source, err := s.store.AIChat().GetMessage(ctx, userID, req.MessageID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatMessageNotFound, "message not found")
	}
	topic, err := s.store.AIChat().GetTopic(ctx, userID, source.TopicID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatTopicNotFound, "topic not found")
	}
	model, assistant, err := s.resolveModelAndAssistant(ctx, userID, topic, topic.ModelID, topic.AssistantID, len(req.Images) > 0)
	if err != nil {
		return nil, err
	}
	bundle, err := s.store.AIChat().CreateEditRegenerateGeneration(ctx, userID, source, req, model, assistant)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatGenerationConflict, "generation conflict")
	}
	content, invokeErr := s.invokeProvider(ctx, model, req.Content)
	if invokeErr != nil {
		_ = s.store.AIChat().FailGeneration(ctx, userID, bundle.Generation.ID, "AI_CHAT_MODEL_UNAVAILABLE", invokeErr.Error())
		return &MessageCreateResult{Bundle: bundle, Err: invokeErr}, nil
	}
	if _, err := s.store.AIChat().CompleteGeneration(ctx, userID, bundle.Generation.ID, content); err != nil {
		return nil, err
	}
	return &MessageCreateResult{Bundle: bundle, Content: content}, nil
}

func (s *aiChatService) BranchMessage(ctx context.Context, messageID string) (*iapiserver.AIChatTopic, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	topic, err := s.store.AIChat().BranchTopic(ctx, userID, messageID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatBranchSourceMissing, "branch source missing")
	}
	if err := attachTopicRelations(ctx, s.store.AIChat(), s.modelReader, userID, []*iapiserver.AIChatTopic{topic}); err != nil {
		return nil, errors.WithStack(err)
	}
	return topic, nil
}

func (s *aiChatService) ListQuickPhrases(
	ctx context.Context,
	req *iapiserver.AIChatQuickPhraseListRequest,
) (*iapiserver.AIChatQuickPhraseListResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.store.AIChat().ListQuickPhrases(ctx, userID, req)
	if err != nil {
		return nil, err
	}
	if err := attachQuickPhraseRelations(ctx, s.store.AIChat(), userID, items); err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.AIChatQuickPhraseListResponse{Items: items}, nil
}

func (s *aiChatService) CreateQuickPhrase(
	ctx context.Context,
	req *iapiserver.AIChatQuickPhraseUpsertRequest,
) (*iapiserver.AIChatQuickPhrase, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	created, err := s.store.AIChat().CreateQuickPhrase(ctx, userID, quickPhraseFromRequest(req))
	if err != nil {
		return nil, err
	}
	if err := attachQuickPhraseRelations(ctx, s.store.AIChat(), userID, []*iapiserver.AIChatQuickPhrase{created}); err != nil {
		return nil, errors.WithStack(err)
	}
	return created, nil
}

func (s *aiChatService) UpdateQuickPhrase(
	ctx context.Context,
	req *iapiserver.AIChatQuickPhraseUpsertRequest,
) (*iapiserver.AIChatQuickPhrase, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	phrase := quickPhraseFromRequest(req)
	phrase.ID = req.ID
	updated, err := s.store.AIChat().UpdateQuickPhrase(ctx, userID, phrase)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatMessageNotFound, "quick phrase not found")
	}
	if err := attachQuickPhraseRelations(ctx, s.store.AIChat(), userID, []*iapiserver.AIChatQuickPhrase{updated}); err != nil {
		return nil, errors.WithStack(err)
	}
	return updated, nil
}

func (s *aiChatService) DeleteQuickPhrase(ctx context.Context, id string) (*iapiserver.AIChatDeleteResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.store.AIChat().DeleteQuickPhrase(ctx, userID, id); err != nil {
		return nil, mapNotFound(err, code.ErrAIChatMessageNotFound, "quick phrase not found")
	}
	return &iapiserver.AIChatDeleteResponse{Deleted: true}, nil
}

func (s *aiChatService) TranslateContent(
	ctx context.Context,
	req *iapiserver.AIChatTranslationRequest,
) (*iapiserver.AIChatMessageTranslation, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	model, err := s.defaultTranslationModel(ctx, userID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatTranslationModelMissing, "translation model missing")
	}
	if !model.Enabled || isUnhealthyModel(model) {
		return nil, errors.NewStatusF(code.ErrAIChatTranslationModelUnhealthy, "translation model unhealthy")
	}
	if req.MessageID != "" {
		if _, err := s.store.AIChat().GetMessage(ctx, userID, req.MessageID); err != nil {
			return nil, mapNotFound(err, code.ErrAIChatMessageNotFound, "message not found")
		}
	}
	content, err := s.invokeProvider(ctx, model, req.Content)
	if err != nil {
		return nil, err
	}
	return s.store.AIChat().CreateTranslation(ctx, &iapiserver.AIChatMessageTranslation{
		MessageID:         req.MessageID,
		OwnerUserID:       userID,
		TargetLanguage:    req.TargetLanguage,
		TranslatedContent: content,
		ModelSnapshot:     map[string]any{"id": model.ID, "name": model.Name},
	})
}

func (s *aiChatService) translate(
	ctx context.Context,
	userID string,
	topic *iapiserver.AIChatTopic,
	req *iapiserver.AIChatMessageCreateRequest,
) (*MessageCreateResult, error) {
	model, err := s.defaultTranslationModel(ctx, userID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatTranslationModelMissing, "translation model missing")
	}
	if !model.Enabled {
		return nil, errors.NewStatusF(code.ErrAIChatTranslationModelDisabled, "translation model disabled")
	}
	if isUnhealthyModel(model) {
		return nil, errors.NewStatusF(code.ErrAIChatModelUnavailable, "translation model unhealthy: %s", model.HealthReason)
	}
	content, err := s.invokeProvider(ctx, model, req.Content)
	if err != nil {
		return nil, err
	}
	messageReq := &iapiserver.AIChatMessageCreateRequest{
		TopicID:   topic.ID,
		Operation: iapiserver.AIChatOperationChat,
		Content:   req.Content,
	}
	bundle, err := s.store.AIChat().CreateMessageGeneration(ctx, userID, topic, messageReq, model, nil)
	if err != nil {
		return nil, err
	}
	_, _ = s.store.AIChat().CompleteGeneration(ctx, userID, bundle.Generation.ID, content)
	translation := &iapiserver.AIChatMessageTranslation{
		MessageID:         bundle.UserMessage.ID,
		OwnerUserID:       userID,
		TargetLanguage:    req.TargetLanguage,
		TranslatedContent: content,
		ModelSnapshot:     map[string]any{"id": model.ID, "name": model.Name},
	}
	created, err := s.store.AIChat().CreateTranslation(ctx, translation)
	if err != nil {
		return nil, err
	}
	return &MessageCreateResult{Translation: &iapiserver.AIChatTranslationResponse{
		MessageID:         created.MessageID,
		SourceLanguage:    created.SourceLanguage,
		TargetLanguage:    created.TargetLanguage,
		TranslatedContent: created.TranslatedContent,
		ModelSnapshot:     created.ModelSnapshot,
	}}, nil
}

func (s *aiChatService) defaultTranslationModel(ctx context.Context, ownerUserID string) (*iapiserver.AIChatModel, error) {
	configs, err := s.store.SystemLLMConfigs().List(ctx)
	if err != nil {
		return nil, err
	}
	for _, cfg := range configs {
		if cfg.OwnerUserID == ownerUserID && cfg.Purpose == "translation" {
			return s.getModel(ctx, ownerUserID, cfg.ModelID)
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *aiChatService) resolveModelAndAssistant(
	ctx context.Context,
	userID string,
	topic *iapiserver.AIChatTopic,
	modelID string,
	assistantID string,
	requiresImage bool,
) (*iapiserver.AIChatModel, *iapiserver.AIChatAssistant, error) {
	if modelID == "" {
		modelID = topic.ModelID
	}
	if modelID == "" {
		return nil, nil, errors.NewStatusF(code.ErrAIChatModelNotFound, "model is required")
	}
	model, err := s.getModel(ctx, userID, modelID)
	if err != nil {
		return nil, nil, mapNotFound(err, code.ErrAIChatModelNotFound, "model not found")
	}
	if !model.Enabled {
		return nil, nil, errors.NewStatusF(code.ErrAIChatModelDisabled, "model disabled")
	}
	if isUnhealthyModel(model) {
		return nil, nil, errors.NewStatusF(code.ErrAIChatModelUnavailable, "model unhealthy: %s", model.HealthReason)
	}
	if requiresImage && !containsCapability(model, "image", "vision") {
		return nil, nil, errors.NewStatusF(code.ErrAIChatModelCapabilityUnsupported, "model does not support image")
	}
	if assistantID == "" {
		assistantID = topic.AssistantID
	}
	var assistant *iapiserver.AIChatAssistant
	if assistantID != "" {
		assistant, err = s.store.AIChat().GetAssistant(ctx, userID, assistantID)
		if err != nil {
			return nil, nil, mapNotFound(err, code.ErrAIChatAssistantNotFound, "assistant not found")
		}
	}
	return model, assistant, nil
}

func (s *aiChatService) invokeProvider(ctx context.Context, model *iapiserver.AIChatModel, input string) (string, error) {
	providerMeta, err := s.store.Providers().Get(ctx, model.Provider)
	if err != nil {
		return "", errors.NewStatusF(code.ErrAIChatModelUnavailable, "provider is unavailable")
	}
	if !providerMeta.Enabled {
		return "", errors.NewStatusF(code.ErrAIChatModelUnavailable, "provider is disabled")
	}
	if providerMeta.Type != iapiserver.ProviderTypeOpenAICompatible {
		return "", errors.NewStatusF(code.ErrAIChatModelUnavailable, "provider type is unsupported")
	}
	adapter := provider.NewOpenAICompatibleAdapter(provider.OpenAICompatibleConfig{
		ID:            providerMeta.ID,
		BaseURL:       providerMeta.BaseURL,
		CredentialRef: providerMeta.CredentialRef,
		Capabilities:  toProviderCapabilities(model.Capabilities),
	})
	result, err := adapter.Invoke(ctx, provider.InvokeRequest{
		Capability: provider.Capability(iapiserver.CapabilityLLMChat),
		Model:      model.ProviderModelID,
		Messages: []provider.ChatMessage{
			{Role: "user", Content: input},
		},
	})
	if err != nil {
		return "", errors.NewStatusF(code.ErrAIChatModelUnavailable, "provider runtime is unavailable")
	}
	return result.Content, nil
}

func currentUserID(ctx context.Context) (string, error) {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err != nil || user == nil || user.ID == "" {
		return "system-admin", nil
	}
	return user.ID, nil
}

func (s *aiChatService) getModel(ctx context.Context, ownerUserID, id string) (*iapiserver.AIChatModel, error) {
	providerModel, err := s.store.ProviderModels().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if providerModel.OwnerUserID != ownerUserID {
		return nil, gorm.ErrRecordNotFound
	}
	provider, err := s.store.Providers().Get(ctx, providerModel.ProviderID)
	if err != nil {
		if isRecordNotFound(err) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	if provider.OwnerUserID != ownerUserID {
		return nil, gorm.ErrRecordNotFound
	}
	if !provider.Enabled || !providerModel.Enabled {
		return nil, gorm.ErrRecordNotFound
	}
	if providerModel.HealthStatus == iapiserver.ProviderModelHealthUnhealthy {
		return nil, errors.NewStatusF(code.ErrAIChatModelUnavailable, "model unhealthy: %s", providerModel.HealthReason)
	}
	return providerModelToAIChatModel(ownerUserID, provider, providerModel), nil
}

func providerModelToAIChatModel(
	ownerUserID string,
	provider *iapiserver.Provider,
	providerModel *iapiserver.ProviderModel,
) *iapiserver.AIChatModel {
	name := providerModel.DisplayName
	if name == "" {
		name = providerModel.Model
	}
	return &iapiserver.AIChatModel{
		ID:              providerModel.ID,
		OwnerUserID:     ownerUserID,
		Provider:        provider.ID,
		ProviderModelID: providerModel.Model,
		Name:            name,
		Capabilities:    normalizeProviderCapabilities(providerModel),
		Enabled:         provider.Enabled && providerModel.Enabled,
		HealthStatus:    providerModel.HealthStatus,
		HealthReason:    providerModel.HealthReason,
	}
}

func normalizeProviderCapabilities(providerModel *iapiserver.ProviderModel) []string {
	seen := map[string]struct{}{}
	var values []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	for _, capability := range providerModel.Capabilities {
		add(capability)
	}
	for _, modelType := range providerModel.ModelTypes {
		add(modelType)
	}
	if len(values) == 0 || providerModel.EndpointType == "chat" {
		add("text")
	}
	return values
}

func mapNotFound(err error, errCode int, message string) error {
	if isRecordNotFound(err) {
		return errors.NewStatusF(errCode, "%s", message)
	}
	return err
}

func isRecordNotFound(err error) bool {
	return stderrors.Is(errors.Cause(err), gorm.ErrRecordNotFound)
}

func assistantFromRequest(req *iapiserver.AIChatAssistantUpsertRequest) *iapiserver.AIChatAssistant {
	assistant := &iapiserver.AIChatAssistant{
		ObjectMeta:        imachinery.ObjectMeta{Name: req.Name},
		SuggestedModelID:  req.SuggestedModelID,
		UseSuggestedModel: req.UseSuggestedModel,
		SystemPrompt:      req.SystemPrompt,
		Stream:            true,
		ToolMode:          "none",
		CustomParameters:  map[string]any{},
	}
	if req.RuntimeConfig != nil {
		assistant.CustomParameters = req.RuntimeConfig
	}
	return assistant
}

func quickPhraseFromRequest(req *iapiserver.AIChatQuickPhraseUpsertRequest) *iapiserver.AIChatQuickPhrase {
	return &iapiserver.AIChatQuickPhrase{
		Title:       req.Title,
		Content:     req.Content,
		Scope:       req.Scope,
		AssistantID: req.AssistantID,
		PhraseType:  req.PhraseType,
	}
}

func containsCapability(model *iapiserver.AIChatModel, names ...string) bool {
	allowed := map[string]struct{}{}
	for _, name := range names {
		allowed[name] = struct{}{}
	}
	for _, capability := range model.Capabilities {
		if _, ok := allowed[strings.TrimSpace(capability)]; ok {
			return true
		}
	}
	return false
}

func isUnhealthyModel(model *iapiserver.AIChatModel) bool {
	return model != nil && model.HealthStatus == iapiserver.ProviderModelHealthUnhealthy
}

func toProviderCapabilities(values []string) []provider.Capability {
	capabilities := make([]provider.Capability, 0, len(values))
	for _, value := range values {
		capabilities = append(capabilities, provider.Capability(value))
	}
	return capabilities
}
