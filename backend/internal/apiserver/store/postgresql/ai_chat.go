package postgresql

import (
	"context"
	stderrors "errors"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

type aiChatStore struct{ ds *datastore }

func newAIChat(ds *datastore) *aiChatStore { return &aiChatStore{ds: ds} }

func (s *aiChatStore) ListModels(
	ctx context.Context,
	ownerUserID string,
	req *iapiserver.AIChatModelListRequest,
) ([]*iapiserver.AIChatModel, int64, error) {
	var items []*iapiserver.AIChatModel
	var total int64
	filter := func(q *gorm.DB) *gorm.DB {
		q = q.Where("owner_user_id = ?", ownerUserID)
		if req.Provider != "" {
			q = q.Where("provider = ?", req.Provider)
		}
		if req.Enabled != nil {
			q = q.Where("enabled = ?", *req.Enabled)
		}
		if req.Capability != "" {
			q = q.Where("capabilities::text LIKE ?", "%"+req.Capability+"%")
		}
		return q
	}
	query := req.ToQuery(ctx, s.ds.db.Model(&iapiserver.AIChatModel{}), filter)
	if err := query.Find(&items).Count(&total).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}
	return items, total, nil
}

func (s *aiChatStore) GetModel(ctx context.Context, ownerUserID, id string) (*iapiserver.AIChatModel, error) {
	var item iapiserver.AIChatModel
	err := s.ds.db.WithContext(ctx).
		Where("owner_user_id = ? AND id = ?", ownerUserID, id).
		First(&item).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *aiChatStore) GetDefaultTranslationModel(
	ctx context.Context,
	ownerUserID string,
) (*iapiserver.AIChatModel, error) {
	var item iapiserver.AIChatModel
	err := s.ds.db.WithContext(ctx).
		Where("owner_user_id = ? AND is_default_translation = ?", ownerUserID, true).
		First(&item).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *aiChatStore) ListAssistants(
	ctx context.Context,
	ownerUserID string,
) ([]*iapiserver.AIChatAssistant, error) {
	var items []*iapiserver.AIChatAssistant
	if err := s.ds.db.WithContext(ctx).
		Where("deleted_at IS NULL AND (system = ? OR owner_user_id = ?)", true, ownerUserID).
		Order("system DESC, created_at ASC").
		Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func (s *aiChatStore) GetAssistant(
	ctx context.Context,
	ownerUserID, id string,
) (*iapiserver.AIChatAssistant, error) {
	var item iapiserver.AIChatAssistant
	err := s.ds.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL AND (system = ? OR owner_user_id = ?)", id, true, ownerUserID).
		First(&item).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *aiChatStore) CreateAssistant(
	ctx context.Context,
	ownerUserID string,
	data *iapiserver.AIChatAssistant,
) (*iapiserver.AIChatAssistant, error) {
	owner := ownerUserID
	data.OwnerUserID = &owner
	data.System = false
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureAssistantNameAvailable(tx, ownerUserID, data.Name, ""); err != nil {
			return err
		}
		return tx.Create(data).Error
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *aiChatStore) UpdateAssistant(
	ctx context.Context,
	ownerUserID string,
	data *iapiserver.AIChatAssistant,
) (*iapiserver.AIChatAssistant, error) {
	var updated iapiserver.AIChatAssistant
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND deleted_at IS NULL AND (system = ? OR owner_user_id = ?)", data.ID, true, ownerUserID).
			First(&updated).Error; err != nil {
			return err
		}
		if updated.System && data.Name != "" && data.Name != updated.Name {
			return errors.NewStatusF(code.ErrAIChatSystemAssistantProtected, "system assistant name cannot be changed")
		}
		if !updated.System && data.Name != "" && data.Name != updated.Name {
			if err := ensureAssistantNameAvailable(tx, ownerUserID, data.Name, data.ID); err != nil {
				return err
			}
			updated.Name = data.Name
		}
		updated.SuggestedModelID = data.SuggestedModelID
		updated.UseSuggestedModel = data.UseSuggestedModel
		updated.SystemPrompt = data.SystemPrompt
		updated.ContextMessageCount = data.ContextMessageCount
		updated.Stream = data.Stream
		updated.ToolMode = data.ToolMode
		updated.MaxToolCalls = data.MaxToolCalls
		updated.Temperature = data.Temperature
		updated.TopP = data.TopP
		updated.MaxTokens = data.MaxTokens
		updated.CustomParameters = data.CustomParameters
		return tx.Save(&updated).Error
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &updated, nil
}

func (s *aiChatStore) DeleteAssistant(ctx context.Context, ownerUserID, id string) error {
	now := time.Now()
	return errors.WithStack(s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item iapiserver.AIChatAssistant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND deleted_at IS NULL AND (system = ? OR owner_user_id = ?)", id, true, ownerUserID).
			First(&item).Error; err != nil {
			return err
		}
		if item.System {
			return errors.NewStatusF(code.ErrAIChatSystemAssistantProtected, "system assistant cannot be deleted")
		}
		return tx.Model(&item).Update("deleted_at", now).Error
	}))
}

func (s *aiChatStore) ListTopics(
	ctx context.Context,
	ownerUserID string,
	req *iapiserver.AIChatTopicListRequest,
) ([]*iapiserver.AIChatTopic, int64, error) {
	var items []*iapiserver.AIChatTopic
	var total int64
	filter := func(q *gorm.DB) *gorm.DB {
		q = q.Where("owner_user_id = ? AND deleted_at IS NULL", ownerUserID)
		if req.Q != "" {
			q = q.Where("title LIKE ?", "%"+req.Q+"%")
		}
		if req.Pinned != nil {
			q = q.Where("pinned = ?", *req.Pinned)
		}
		return q
	}
	query := req.ToQuery(ctx, s.ds.db.Model(&iapiserver.AIChatTopic{}), filter).
		Order("pinned DESC, last_active_at DESC")
	if err := query.Find(&items).Count(&total).Error; err != nil {
		return nil, 0, errors.WithStack(err)
	}
	return items, total, nil
}

func (s *aiChatStore) GetTopic(ctx context.Context, ownerUserID, id string) (*iapiserver.AIChatTopic, error) {
	var item iapiserver.AIChatTopic
	err := s.ds.db.WithContext(ctx).
		Where("owner_user_id = ? AND id = ? AND deleted_at IS NULL", ownerUserID, id).
		First(&item).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *aiChatStore) CreateTopic(
	ctx context.Context,
	ownerUserID string,
	data *iapiserver.AIChatTopic,
) (*iapiserver.AIChatTopic, error) {
	data.OwnerUserID = ownerUserID
	if data.Title == "" {
		data.Title = "新对话"
	}
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *aiChatStore) UpdateTopic(
	ctx context.Context,
	ownerUserID string,
	data *iapiserver.AIChatTopic,
) (*iapiserver.AIChatTopic, error) {
	var item iapiserver.AIChatTopic
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("owner_user_id = ? AND id = ? AND deleted_at IS NULL", ownerUserID, data.ID).
			First(&item).Error; err != nil {
			return err
		}
		if data.Title != "" {
			item.Title = data.Title
		}
		item.Pinned = data.Pinned
		item.AssistantID = data.AssistantID
		item.ModelID = data.ModelID
		return tx.Save(&item).Error
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *aiChatStore) DeleteTopic(ctx context.Context, ownerUserID, id string) error {
	now := time.Now()
	return errors.WithStack(
		s.ds.db.WithContext(ctx).
			Model(&iapiserver.AIChatTopic{}).
			Where("owner_user_id = ? AND id = ? AND deleted_at IS NULL", ownerUserID, id).
			Update("deleted_at", now).Error,
	)
}

func (s *aiChatStore) BranchTopic(ctx context.Context, ownerUserID, messageID string) (*iapiserver.AIChatTopic, error) {
	var branched *iapiserver.AIChatTopic
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		source, err := getMessageForUpdate(tx, ownerUserID, messageID)
		if err != nil {
			return err
		}
		if source.Role != iapiserver.AIChatMessageRoleAssistant {
			return errors.NewStatusF(code.ErrAIChatBranchSourceMissing, "branch source must be an assistant message")
		}
		var topic iapiserver.AIChatTopic
		if err := tx.Where("owner_user_id = ? AND id = ? AND deleted_at IS NULL", ownerUserID, source.TopicID).
			First(&topic).Error; err != nil {
			return err
		}
		var latest iapiserver.AIChatMessage
		err = tx.Where("owner_user_id = ? AND topic_id = ? AND role = ?", ownerUserID, source.TopicID, iapiserver.AIChatMessageRoleAssistant).
			Order("created_at DESC, version DESC").
			First(&latest).Error
		if err == nil && latest.ID == source.ID {
			branched = &topic
			return nil
		}
		branch := &iapiserver.AIChatTopic{
			OwnerUserID:           ownerUserID,
			Title:                 topic.Title + " 分支",
			AssistantID:           topic.AssistantID,
			ModelID:               topic.ModelID,
			BranchSourceTopicID:   topic.ID,
			BranchSourceMessageID: source.ID,
		}
		if err := tx.Create(branch).Error; err != nil {
			return err
		}
		branched = branch
		return nil
	})
	return branched, errors.WithStack(err)
}

func (s *aiChatStore) ListMessages(
	ctx context.Context,
	ownerUserID, topicID string,
) ([]*iapiserver.AIChatMessage, error) {
	var items []*iapiserver.AIChatMessage
	if err := s.ds.db.WithContext(ctx).
		Where("owner_user_id = ? AND topic_id = ?", ownerUserID, topicID).
		Order("created_at ASC, version ASC").
		Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	if err := s.loadAttachments(ctx, ownerUserID, items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *aiChatStore) GetMessage(ctx context.Context, ownerUserID, id string) (*iapiserver.AIChatMessage, error) {
	var item iapiserver.AIChatMessage
	if err := s.ds.db.WithContext(ctx).
		Where("owner_user_id = ? AND id = ?", ownerUserID, id).
		First(&item).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	if err := s.loadAttachments(ctx, ownerUserID, []*iapiserver.AIChatMessage{&item}); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *aiChatStore) CreateMessageGeneration(
	ctx context.Context,
	ownerUserID string,
	topic *iapiserver.AIChatTopic,
	req *iapiserver.AIChatMessageCreateRequest,
	model *iapiserver.AIChatModel,
	assistant *iapiserver.AIChatAssistant,
) (*store.AIChatGenerationBundle, error) {
	var bundle *store.AIChatGenerationBundle
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureNoActiveGeneration(tx, ownerUserID, topic.ID); err != nil {
			return err
		}
		now := time.Now()
		userMsg := &iapiserver.AIChatMessage{
			TopicID:         topic.ID,
			OwnerUserID:     ownerUserID,
			Role:            iapiserver.AIChatMessageRoleUser,
			Content:         req.Content,
			Status:          iapiserver.AIChatStatusDone,
			ClientMessageID: req.ClientMessageID,
		}
		if err := tx.Create(userMsg).Error; err != nil {
			return err
		}
		if err := createAttachmentMetadata(tx, ownerUserID, userMsg.ID, req.Images); err != nil {
			return err
		}
		assistantMsg := &iapiserver.AIChatMessage{
			TopicID:           topic.ID,
			OwnerUserID:       ownerUserID,
			Role:              iapiserver.AIChatMessageRoleAssistant,
			Status:            iapiserver.AIChatStatusGenerating,
			ParentMessageID:   userMsg.ID,
			ModelSnapshot:     modelSnapshot(model),
			AssistantSnapshot: assistantSnapshot(assistant),
		}
		if err := tx.Create(assistantMsg).Error; err != nil {
			return err
		}
		generation := &iapiserver.AIChatGeneration{
			TopicID:            topic.ID,
			OwnerUserID:        ownerUserID,
			AssistantMessageID: assistantMsg.ID,
			Operation:          req.Operation,
			Status:             iapiserver.AIChatStatusGenerating,
			ModelID:            model.ID,
			AssistantID:        assistantID(assistant),
			StartedAt:          &now,
		}
		if err := tx.Create(generation).Error; err != nil {
			return err
		}
		if err := tx.Model(&iapiserver.AIChatTopic{}).
			Where("owner_user_id = ? AND id = ?", ownerUserID, topic.ID).
			Updates(map[string]any{
				"last_active_at": now,
				"assistant_id":   assistantID(assistant),
				"model_id":       model.ID,
			}).Error; err != nil {
			return err
		}
		bundle = &store.AIChatGenerationBundle{
			UserMessage:      userMsg,
			AssistantMessage: assistantMsg,
			Generation:       generation,
		}
		return nil
	})
	return bundle, errors.WithStack(err)
}

func (s *aiChatStore) CreateEditRegenerateGeneration(
	ctx context.Context,
	ownerUserID string,
	source *iapiserver.AIChatMessage,
	req *iapiserver.AIChatEditRegenerateRequest,
	model *iapiserver.AIChatModel,
	assistant *iapiserver.AIChatAssistant,
) (*store.AIChatGenerationBundle, error) {
	var topic iapiserver.AIChatTopic
	if err := s.ds.db.WithContext(ctx).
		Where("owner_user_id = ? AND id = ? AND deleted_at IS NULL", ownerUserID, source.TopicID).
		First(&topic).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	createReq := &iapiserver.AIChatMessageCreateRequest{
		TopicID:         source.TopicID,
		Operation:       iapiserver.AIChatOperationChat,
		ClientMessageID: req.ClientMessageID,
		Content:         req.Content,
		Images:          req.Images,
	}
	bundle, err := s.CreateMessageGeneration(ctx, ownerUserID, &topic, createReq, model, assistant)
	if err != nil {
		return nil, err
	}
	if err := s.ds.db.WithContext(ctx).Model(&iapiserver.AIChatMessage{}).
		Where("owner_user_id = ? AND id = ?", ownerUserID, source.ID).
		Update("truncated_after", true).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return bundle, nil
}

func (s *aiChatStore) CompleteGeneration(
	ctx context.Context,
	ownerUserID, generationID, content string,
) (*iapiserver.AIChatGeneration, error) {
	var generation iapiserver.AIChatGeneration
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("owner_user_id = ? AND id = ?", ownerUserID, generationID).
			First(&generation).Error; err != nil {
			return err
		}
		now := time.Now()
		if err := tx.Model(&iapiserver.AIChatMessage{}).
			Where("owner_user_id = ? AND id = ?", ownerUserID, generation.AssistantMessageID).
			Updates(map[string]any{"content": content, "status": iapiserver.AIChatStatusDone}).Error; err != nil {
			return err
		}
		return tx.Model(&generation).Updates(map[string]any{
			"status":       iapiserver.AIChatStatusDone,
			"completed_at": now,
		}).Error
	})
	return &generation, errors.WithStack(err)
}

func (s *aiChatStore) FailGeneration(ctx context.Context, ownerUserID, generationID, errorCode, errorMessage string) error {
	return errors.WithStack(s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var generation iapiserver.AIChatGeneration
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("owner_user_id = ? AND id = ?", ownerUserID, generationID).
			First(&generation).Error; err != nil {
			return err
		}
		if err := tx.Model(&iapiserver.AIChatMessage{}).
			Where("owner_user_id = ? AND id = ?", ownerUserID, generation.AssistantMessageID).
			Update("status", iapiserver.AIChatStatusFailed).Error; err != nil {
			return err
		}
		now := time.Now()
		return tx.Model(&generation).Updates(map[string]any{
			"status":        iapiserver.AIChatStatusFailed,
			"completed_at":  now,
			"error_code":    errorCode,
			"error_message": errorMessage,
		}).Error
	}))
}

func (s *aiChatStore) StopGeneration(
	ctx context.Context,
	ownerUserID, generationID string,
) (*iapiserver.AIChatGeneration, error) {
	var generation iapiserver.AIChatGeneration
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("owner_user_id = ? AND id = ?", ownerUserID, generationID).
			First(&generation).Error; err != nil {
			return err
		}
		if generation.Status != iapiserver.AIChatStatusQueued && generation.Status != iapiserver.AIChatStatusGenerating {
			return nil
		}
		now := time.Now()
		if err := tx.Model(&iapiserver.AIChatMessage{}).
			Where("owner_user_id = ? AND id = ?", ownerUserID, generation.AssistantMessageID).
			Update("status", iapiserver.AIChatStatusInterrupted).Error; err != nil {
			return err
		}
		generation.Status = iapiserver.AIChatStatusInterrupted
		generation.StoppedAt = &now
		return tx.Model(&generation).Updates(map[string]any{
			"status":     generation.Status,
			"stopped_at": now,
		}).Error
	})
	return &generation, errors.WithStack(err)
}

func (s *aiChatStore) ListQuickPhrases(
	ctx context.Context,
	ownerUserID string,
	req *iapiserver.AIChatQuickPhraseListRequest,
) ([]*iapiserver.AIChatQuickPhrase, error) {
	var items []*iapiserver.AIChatQuickPhrase
	query := s.ds.db.WithContext(ctx).
		Where("owner_user_id = ? AND deleted_at IS NULL", ownerUserID)
	if req.Scope != "" {
		query = query.Where("scope = ?", req.Scope)
	}
	if req.AssistantID != "" {
		query = query.Where("assistant_id = ?", req.AssistantID)
	}
	if req.Q != "" {
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+req.Q+"%", "%"+req.Q+"%")
	}
	if err := query.Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func (s *aiChatStore) GetQuickPhrase(
	ctx context.Context,
	ownerUserID, id string,
) (*iapiserver.AIChatQuickPhrase, error) {
	var item iapiserver.AIChatQuickPhrase
	if err := s.ds.db.WithContext(ctx).
		Where("owner_user_id = ? AND id = ? AND deleted_at IS NULL", ownerUserID, id).
		First(&item).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *aiChatStore) CreateQuickPhrase(
	ctx context.Context,
	ownerUserID string,
	data *iapiserver.AIChatQuickPhrase,
) (*iapiserver.AIChatQuickPhrase, error) {
	data.OwnerUserID = ownerUserID
	if err := s.ds.db.WithContext(ctx).Create(data).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (s *aiChatStore) UpdateQuickPhrase(
	ctx context.Context,
	ownerUserID string,
	data *iapiserver.AIChatQuickPhrase,
) (*iapiserver.AIChatQuickPhrase, error) {
	var item iapiserver.AIChatQuickPhrase
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("owner_user_id = ? AND id = ? AND deleted_at IS NULL", ownerUserID, data.ID).
			First(&item).Error; err != nil {
			return err
		}
		item.Title = data.Title
		item.Content = data.Content
		item.Scope = data.Scope
		item.AssistantID = data.AssistantID
		item.PhraseType = data.PhraseType
		return tx.Save(&item).Error
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (s *aiChatStore) DeleteQuickPhrase(ctx context.Context, ownerUserID, id string) error {
	now := time.Now()
	return errors.WithStack(
		s.ds.db.WithContext(ctx).
			Model(&iapiserver.AIChatQuickPhrase{}).
			Where("owner_user_id = ? AND id = ? AND deleted_at IS NULL", ownerUserID, id).
			Update("deleted_at", now).Error,
	)
}

func (s *aiChatStore) CreateTranslation(
	ctx context.Context,
	translation *iapiserver.AIChatMessageTranslation,
) (*iapiserver.AIChatMessageTranslation, error) {
	if err := s.ds.db.WithContext(ctx).Create(translation).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return translation, nil
}

func (s *aiChatStore) loadAttachments(
	ctx context.Context,
	ownerUserID string,
	messages []*iapiserver.AIChatMessage,
) error {
	if len(messages) == 0 {
		return nil
	}
	ids := make([]string, 0, len(messages))
	byID := make(map[string]*iapiserver.AIChatMessage, len(messages))
	for _, message := range messages {
		ids = append(ids, message.ID)
		byID[message.ID] = message
	}
	var attachments []*iapiserver.AIChatMessageAttachment
	if err := s.ds.db.WithContext(ctx).
		Where("owner_user_id = ? AND message_id IN ?", ownerUserID, ids).
		Find(&attachments).Error; err != nil {
		return errors.WithStack(err)
	}
	for _, attachment := range attachments {
		if msg := byID[attachment.MessageID]; msg != nil {
			msg.Attachments = append(msg.Attachments, attachment)
		}
	}
	return nil
}

func ensureAssistantNameAvailable(tx *gorm.DB, ownerUserID, name, exceptID string) error {
	var existing iapiserver.AIChatAssistant
	query := tx.Where(
		"owner_user_id = ? AND name = ? AND system = ? AND deleted_at IS NULL",
		ownerUserID,
		name,
		false,
	)
	if exceptID != "" {
		query = query.Where("id <> ?", exceptID)
	}
	err := query.First(&existing).Error
	if err == nil {
		return errors.NewStatusF(code.ErrAIChatDuplicateAssistantName, "assistant name already exists")
	}
	if !stderrors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return nil
}

func ensureNoActiveGeneration(tx *gorm.DB, ownerUserID, topicID string) error {
	var count int64
	if err := tx.Model(&iapiserver.AIChatGeneration{}).
		Where(
			"owner_user_id = ? AND topic_id = ? AND status IN ?",
			ownerUserID,
			topicID,
			[]string{iapiserver.AIChatStatusQueued, iapiserver.AIChatStatusGenerating},
		).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.NewStatusF(code.ErrAIChatGenerationConflict, "topic has active generation")
	}
	return nil
}

func getMessageForUpdate(tx *gorm.DB, ownerUserID, messageID string) (*iapiserver.AIChatMessage, error) {
	var item iapiserver.AIChatMessage
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("owner_user_id = ? AND id = ?", ownerUserID, messageID).
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func createAttachmentMetadata(
	tx *gorm.DB,
	ownerUserID string,
	messageID string,
	images []*iapiserver.AIChatImageAttachmentInput,
) error {
	for _, image := range images {
		attachment := &iapiserver.AIChatMessageAttachment{
			MessageID:   messageID,
			OwnerUserID: ownerUserID,
			Kind:        iapiserver.AIChatAttachmentKindImage,
			MimeType:    image.MimeType,
			FileName:    image.FileName,
			SizeBytes:   image.SizeBytes,
			DisplayMode: iapiserver.AIChatAttachmentDisplayIcon,
			Stored:      false,
		}
		if err := tx.Create(attachment).Error; err != nil {
			return err
		}
	}
	return nil
}

func modelSnapshot(model *iapiserver.AIChatModel) map[string]any {
	if model == nil {
		return map[string]any{}
	}
	return map[string]any{
		"id":                model.ID,
		"name":              model.Name,
		"provider":          model.Provider,
		"provider_model_id": model.ProviderModelID,
		"capabilities":      model.Capabilities,
	}
}

func assistantSnapshot(assistant *iapiserver.AIChatAssistant) map[string]any {
	if assistant == nil {
		return map[string]any{}
	}
	return map[string]any{
		"id":                    assistant.ID,
		"name":                  assistant.Name,
		"system":                assistant.System,
		"system_prompt":         assistant.SystemPrompt,
		"context_message_count": assistant.ContextMessageCount,
	}
}

func assistantID(assistant *iapiserver.AIChatAssistant) string {
	if assistant == nil {
		return ""
	}
	return assistant.ID
}

func hasAnyCapability(model *iapiserver.AIChatModel, capabilities ...string) bool {
	if model == nil {
		return false
	}
	allowed := map[string]struct{}{}
	for _, capability := range capabilities {
		allowed[capability] = struct{}{}
	}
	for _, capability := range model.Capabilities {
		if _, ok := allowed[strings.TrimSpace(capability)]; ok {
			return true
		}
	}
	return false
}
