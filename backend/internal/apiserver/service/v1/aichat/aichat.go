package aichat

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/modelgateway"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/usermodel"
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
	userModels  UserModelExecutionContextResolver
	gateway     OperationGateway
}

type UserModelExecutionContextResolver interface {
	ResolveUserModelExecutionContext(context.Context, usermodel.ExecutionContextRequest) (*modelgateway.UserModelExecutionContext, error)
}

type OperationGateway interface {
	ExecuteOperation(context.Context, modelgateway.OperationExecutionRequest) (*modelgateway.OperationExecutionResult, error)
}

type Dependencies struct {
	Store       store.Factory
	ModelReader ModelSummaryReader
	UserModels  UserModelExecutionContextResolver
	Gateway     OperationGateway
}

// NewService creates AI Chat with explicit User Model and Model Gateway execution boundaries.
func NewService(deps Dependencies) AIChatSrv {
	return &aiChatService{
		store: deps.Store, modelReader: deps.ModelReader, userModels: deps.UserModels, gateway: deps.Gateway,
	}
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
	assistant, err := s.resolveAssistant(ctx, userID, topic, req.AssistantID)
	if err != nil {
		return nil, err
	}
	grant, err := s.resolveExecutionContext(ctx, firstNonEmpty(req.ModelID, topic.ModelID), usermodel.CapabilityTextChatCompletion, len(req.Images) > 0, "assistant.default")
	if err != nil {
		return nil, err
	}
	bundle, err := s.store.AIChat().CreateMessageGeneration(ctx, userID, topic, req, generationRoute(grant), assistant)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatGenerationConflict, "generation conflict")
	}
	content, invokeErr := s.invokeGateway(ctx, userID, grant, req.Content, req.Images, "")
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
	assistant, err := s.resolveAssistant(ctx, userID, topic, topic.AssistantID)
	if err != nil {
		return nil, err
	}
	grant, err := s.resolveExecutionContext(ctx, topic.ModelID, usermodel.CapabilityTextChatCompletion, false, "assistant.default")
	if err != nil {
		return nil, err
	}
	req := &iapiserver.AIChatMessageCreateRequest{
		TopicID:   topic.ID,
		Operation: iapiserver.AIChatOperationChat,
		Content:   "regenerate",
	}
	bundle, err := s.store.AIChat().CreateMessageGeneration(ctx, userID, topic, req, generationRoute(grant), assistant)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatGenerationConflict, "generation conflict")
	}
	content, invokeErr := s.invokeGateway(ctx, userID, grant, source.Content, nil, "")
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
	assistant, err := s.resolveAssistant(ctx, userID, topic, topic.AssistantID)
	if err != nil {
		return nil, err
	}
	grant, err := s.resolveExecutionContext(ctx, topic.ModelID, usermodel.CapabilityTextChatCompletion, len(req.Images) > 0, "assistant.default")
	if err != nil {
		return nil, err
	}
	bundle, err := s.store.AIChat().CreateEditRegenerateGeneration(ctx, userID, source, req, generationRoute(grant), assistant)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatGenerationConflict, "generation conflict")
	}
	content, invokeErr := s.invokeGateway(ctx, userID, grant, req.Content, req.Images, "")
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
	grant, err := s.resolveExecutionContext(ctx, "", usermodel.CapabilityTextTranslate, false, "translation")
	if err != nil {
		return nil, errors.NewStatusF(code.ErrAIChatTranslationModelMissing, "translation model missing or unavailable")
	}
	if req.MessageID != "" {
		if _, err := s.store.AIChat().GetMessage(ctx, userID, req.MessageID); err != nil {
			return nil, mapNotFound(err, code.ErrAIChatMessageNotFound, "message not found")
		}
	}
	content, err := s.invokeGateway(ctx, userID, grant, req.Content, nil, req.TargetLanguage)
	if err != nil {
		return nil, err
	}
	return s.store.AIChat().CreateTranslation(ctx, &iapiserver.AIChatMessageTranslation{
		MessageID:         req.MessageID,
		OwnerUserID:       userID,
		TargetLanguage:    req.TargetLanguage,
		TranslatedContent: content,
		ModelSnapshot:     grant.ModelSnapshot,
	})
}

func (s *aiChatService) translate(
	ctx context.Context,
	userID string,
	topic *iapiserver.AIChatTopic,
	req *iapiserver.AIChatMessageCreateRequest,
) (*MessageCreateResult, error) {
	grant, err := s.resolveExecutionContext(ctx, "", usermodel.CapabilityTextTranslate, false, "translation")
	if err != nil {
		return nil, errors.NewStatusF(code.ErrAIChatTranslationModelMissing, "translation model missing or unavailable")
	}
	messageReq := &iapiserver.AIChatMessageCreateRequest{
		TopicID:   topic.ID,
		Operation: iapiserver.AIChatOperationTranslate,
		Content:   req.Content,
	}
	bundle, err := s.store.AIChat().CreateMessageGeneration(ctx, userID, topic, messageReq, generationRoute(grant), nil)
	if err != nil {
		return nil, err
	}
	content, invokeErr := s.invokeGateway(ctx, userID, grant, req.Content, nil, req.TargetLanguage)
	if invokeErr != nil {
		_ = s.store.AIChat().FailGeneration(ctx, userID, bundle.Generation.ID, "AI_CHAT_MODEL_UNAVAILABLE", invokeErr.Error())
		return &MessageCreateResult{Bundle: bundle, Err: invokeErr}, nil
	}
	if _, err := s.store.AIChat().CompleteGeneration(ctx, userID, bundle.Generation.ID, content); err != nil {
		return nil, errors.WithStack(err)
	}
	translation := &iapiserver.AIChatMessageTranslation{
		MessageID:         bundle.UserMessage.ID,
		OwnerUserID:       userID,
		TargetLanguage:    req.TargetLanguage,
		TranslatedContent: content,
		ModelSnapshot:     grant.ModelSnapshot,
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

func (s *aiChatService) resolveAssistant(ctx context.Context, userID string, topic *iapiserver.AIChatTopic, assistantID string) (*iapiserver.AIChatAssistant, error) {
	if assistantID == "" {
		assistantID = topic.AssistantID
	}
	if assistantID == "" {
		return nil, nil
	}
	assistant, err := s.store.AIChat().GetAssistant(ctx, userID, assistantID)
	if err != nil {
		return nil, mapNotFound(err, code.ErrAIChatAssistantNotFound, "assistant not found")
	}
	return assistant, nil
}

func (s *aiChatService) resolveExecutionContext(
	ctx context.Context,
	modelID, capabilityID string,
	requiresImage bool,
	defaultUsage string,
) (*modelgateway.UserModelExecutionContext, error) {
	if s.userModels == nil || s.gateway == nil {
		return nil, errors.NewStatusF(code.ErrAIChatModelUnavailable, "user model execution is unavailable")
	}
	required := []string(nil)
	if requiresImage {
		required = append(required, "image.understanding")
	}
	grant, err := s.userModels.ResolveUserModelExecutionContext(ctx, usermodel.ExecutionContextRequest{
		ModelID: modelID, CapabilityDefinitionID: capabilityID,
		RequiredCapabilityDefinitionIDs: required, DefaultUsage: defaultUsage,
	})
	if err != nil {
		return nil, errors.NewStatusF(code.ErrAIChatModelUnavailable, "user model is not execution eligible")
	}
	return grant, nil
}

func (s *aiChatService) invokeGateway(
	ctx context.Context,
	userID string,
	grant *modelgateway.UserModelExecutionContext,
	input string,
	images []*iapiserver.AIChatImageAttachmentInput,
	targetLanguage string,
) (string, error) {
	result, err := s.gateway.ExecuteOperation(ctx, modelgateway.OperationExecutionRequest{
		PrincipalUserID:        userID,
		Target:                 modelgateway.UserModelTarget{ExecutionContext: *grant},
		CapabilityDefinitionID: grant.CapabilityDefinitionID,
		Input:                  map[string]any{"messages": gatewayMessages(input, images, targetLanguage)},
	})
	if err != nil {
		return "", errors.NewStatusF(code.ErrAIChatModelUnavailable, "provider runtime is unavailable")
	}
	content, ok := gatewayOutputContent(result.Output)
	if !ok {
		return "", errors.NewStatusF(code.ErrAIChatModelUnavailable, "provider response does not contain message content")
	}
	return content, nil
}

func gatewayMessages(input string, images []*iapiserver.AIChatImageAttachmentInput, targetLanguage string) []any {
	messages := []any{}
	if targetLanguage = strings.TrimSpace(targetLanguage); targetLanguage != "" {
		messages = append(messages, map[string]any{
			"role": "system",
			"content": fmt.Sprintf(
				"Translate the next user message into %q. Return only the translated text.",
				targetLanguage,
			),
		})
	}
	return append(messages, map[string]any{"role": "user", "content": gatewayMessageContent(input, images)})
}

func gatewayMessageContent(input string, images []*iapiserver.AIChatImageAttachmentInput) any {
	if len(images) == 0 {
		return input
	}
	parts := []any{map[string]any{"type": "text", "text": input}}
	for _, image := range images {
		if image == nil {
			continue
		}
		parts = append(parts, map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": fmt.Sprintf("data:%s;base64,%s", image.MimeType, image.Base64Data)},
		})
	}
	return parts
}

func gatewayOutputContent(output map[string]any) (string, bool) {
	values, _ := output["values"].(map[string]any)
	choices, _ := values["choices"].([]any)
	if len(choices) == 0 {
		return "", false
	}
	choice, _ := choices[0].(map[string]any)
	message, _ := choice["message"].(map[string]any)
	content, ok := message["content"].(string)
	return content, ok && content != ""
}

func generationRoute(grant *modelgateway.UserModelExecutionContext) store.AIChatGenerationRoute {
	return store.AIChatGenerationRoute{
		ModelID: grant.ModelID, CapabilityDefinitionID: grant.CapabilityDefinitionID,
		ModelConfigVersion: grant.ConfigVersion, ModelSnapshot: grant.ModelSnapshot,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func currentUserID(ctx context.Context) (string, error) {
	user, err := ctxvalue.GetValue[*iapiserver.User](ctx, iapiserver.GinContextKeyUser)
	if err != nil || user == nil || user.ID == "" {
		return "system-admin", nil
	}
	return user.ID, nil
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
