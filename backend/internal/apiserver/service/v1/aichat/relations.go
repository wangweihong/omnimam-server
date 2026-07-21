package aichat

import (
	"context"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

// ModelSummaryReader 由 AI Chat 消费方定义，用于读取当前用户可见的模型一跳投影。
type ModelSummaryReader interface {
	GetProviderModelRefSummaries(context.Context, string, []string) (map[string]*iapiserver.ProviderModelRefSummary, error)
}

func attachAssistantRelations(
	ctx context.Context,
	reader ModelSummaryReader,
	ownerUserID string,
	items []*iapiserver.AIChatAssistant,
) error {
	if reader == nil {
		return nil
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if item != nil && item.SuggestedModelID != "" {
			ids = append(ids, item.SuggestedModelID)
		}
	}
	summaries, err := reader.GetProviderModelRefSummaries(ctx, ownerUserID, uniqueRelationIDs(ids))
	if err != nil {
		return err
	}
	for _, item := range items {
		if item != nil {
			item.SuggestedModel = summaries[item.SuggestedModelID]
		}
	}
	return nil
}

func attachTopicRelations(
	ctx context.Context,
	chatStore store.AIChatStore,
	reader ModelSummaryReader,
	ownerUserID string,
	items []*iapiserver.AIChatTopic,
) error {
	assistantIDs := make([]string, 0, len(items))
	modelIDs := make([]string, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		assistantIDs = append(assistantIDs, item.AssistantID)
		modelIDs = append(modelIDs, item.ModelID)
	}
	assistants, err := assistantSummaries(ctx, chatStore, ownerUserID, assistantIDs)
	if err != nil {
		return err
	}
	models := map[string]*iapiserver.ProviderModelRefSummary{}
	if reader != nil {
		models, err = reader.GetProviderModelRefSummaries(ctx, ownerUserID, uniqueRelationIDs(modelIDs))
		if err != nil {
			return err
		}
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		item.Assistant = assistants[item.AssistantID]
		item.Model = models[item.ModelID]
	}
	return nil
}

func attachQuickPhraseRelations(
	ctx context.Context,
	chatStore store.AIChatStore,
	ownerUserID string,
	items []*iapiserver.AIChatQuickPhrase,
) error {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if item != nil && item.Scope == iapiserver.AIChatQuickPhraseScopeAssistant {
			ids = append(ids, item.AssistantID)
		}
	}
	summaries, err := assistantSummaries(ctx, chatStore, ownerUserID, ids)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item != nil && item.Scope == iapiserver.AIChatQuickPhraseScopeAssistant {
			item.Assistant = summaries[item.AssistantID]
		}
	}
	return nil
}

func assistantSummaries(
	ctx context.Context,
	chatStore store.AIChatStore,
	ownerUserID string,
	ids []string,
) (map[string]*iapiserver.AssistantSummary, error) {
	result := make(map[string]*iapiserver.AssistantSummary)
	ids = uniqueRelationIDs(ids)
	if len(ids) == 0 {
		return result, nil
	}
	items, err := chatStore.GetAssistantsByIDs(ctx, ownerUserID, ids)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item != nil {
			result[item.ID] = &iapiserver.AssistantSummary{ID: item.ID, Name: item.Name, IsSystem: item.System}
		}
	}
	return result, nil
}

func uniqueRelationIDs(ids []string) []string {
	result := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
