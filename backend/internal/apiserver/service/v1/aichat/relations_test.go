package aichat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

type assistantRelationStore struct {
	store.AIChatStore
	items        map[string]*iapiserver.AIChatAssistant
	calls        int
	lastOwner    string
	lastBatchLen int
}

func (s *assistantRelationStore) GetAssistantsByIDs(
	_ context.Context,
	ownerUserID string,
	ids []string,
) ([]*iapiserver.AIChatAssistant, error) {
	s.calls++
	s.lastOwner = ownerUserID
	s.lastBatchLen = len(ids)
	result := make([]*iapiserver.AIChatAssistant, 0, len(ids))
	for _, id := range ids {
		item := s.items[id]
		if item == nil || (!item.System && (item.OwnerUserID == nil || *item.OwnerUserID != ownerUserID)) {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

type modelSummaryReaderStub struct {
	items        map[string]*iapiserver.ProviderModelRefSummary
	calls        int
	lastOwner    string
	lastBatchLen int
}

type modelOwnerFactory struct {
	store.Factory
	modelStore    store.ProviderModelStore
	providerStore store.ProviderStore
}

func (f *modelOwnerFactory) ProviderModels() store.ProviderModelStore { return f.modelStore }
func (f *modelOwnerFactory) Providers() store.ProviderStore           { return f.providerStore }

type modelOwnerStore struct {
	store.ProviderModelStore
	item *iapiserver.ProviderModel
}

func (s *modelOwnerStore) Get(context.Context, string) (*iapiserver.ProviderModel, error) {
	return s.item, nil
}

type providerOwnerStore struct {
	store.ProviderStore
	item *iapiserver.Provider
}

func (s *providerOwnerStore) Get(context.Context, string) (*iapiserver.Provider, error) {
	return s.item, nil
}

func (r *modelSummaryReaderStub) GetProviderModelRefSummaries(
	_ context.Context,
	ownerUserID string,
	ids []string,
) (map[string]*iapiserver.ProviderModelRefSummary, error) {
	r.calls++
	r.lastOwner = ownerUserID
	r.lastBatchLen = len(ids)
	result := make(map[string]*iapiserver.ProviderModelRefSummary)
	for _, id := range ids {
		if item := r.items[id]; item != nil {
			result[id] = item
		}
	}
	return result, nil
}

func TestAttachTopicRelationsUsesBoundedBatchesAndOmitsInvisibleRelations(t *testing.T) {
	userA := "user-a"
	userB := "user-b"
	chatStore := &assistantRelationStore{items: map[string]*iapiserver.AIChatAssistant{
		"assistant-a": {ObjectMeta: imachinery.ObjectMeta{ID: "assistant-a", Name: "Writer"}, OwnerUserID: &userA},
		"assistant-b": {ObjectMeta: imachinery.ObjectMeta{ID: "assistant-b", Name: "Private"}, OwnerUserID: &userB},
	}}
	modelReader := &modelSummaryReaderStub{items: map[string]*iapiserver.ProviderModelRefSummary{
		"model-a": {ID: "model-a", DisplayName: "GPT A"},
	}}
	items := make([]*iapiserver.AIChatTopic, 0, 50)
	for i := 0; i < 50; i++ {
		items = append(items, &iapiserver.AIChatTopic{AssistantID: "assistant-a", ModelID: "model-a"})
	}
	items[49].AssistantID = "assistant-b"
	items[49].ModelID = "model-missing"

	if err := attachTopicRelations(context.Background(), chatStore, modelReader, userA, items); err != nil {
		t.Fatalf("attach topic relations: %v", err)
	}
	if chatStore.calls != 1 || modelReader.calls != 1 {
		t.Fatalf("batch calls assistants=%d models=%d", chatStore.calls, modelReader.calls)
	}
	if chatStore.lastBatchLen != 2 || modelReader.lastBatchLen != 2 {
		t.Fatalf("deduplicated batch sizes assistants=%d models=%d", chatStore.lastBatchLen, modelReader.lastBatchLen)
	}
	if chatStore.lastOwner != userA || modelReader.lastOwner != userA {
		t.Fatalf("owner scopes assistants=%q models=%q", chatStore.lastOwner, modelReader.lastOwner)
	}
	if items[0].Assistant == nil || items[0].Assistant.Name != "Writer" || items[0].Model == nil || items[0].Model.DisplayName != "GPT A" {
		t.Fatalf("visible relations = %#v / %#v", items[0].Assistant, items[0].Model)
	}
	if items[49].Assistant != nil || items[49].Model != nil {
		t.Fatalf("invisible relations leaked = %#v / %#v", items[49].Assistant, items[49].Model)
	}
}

func TestAIChatRelationSummariesSerializeAlongsideIDs(t *testing.T) {
	topic := &iapiserver.AIChatTopic{
		AssistantID: "assistant-a",
		Assistant:   &iapiserver.AssistantSummary{ID: "assistant-a", Name: "Writer", IsSystem: false},
		ModelID:     "model-a",
		Model:       &iapiserver.ProviderModelRefSummary{ID: "model-a", DisplayName: "GPT A"},
	}
	data, err := json.Marshal(topic)
	if err != nil {
		t.Fatalf("marshal topic: %v", err)
	}
	text := string(data)
	for _, expected := range []string{`"assistant_id":"assistant-a"`, `"assistant":{"id":"assistant-a"`, `"model_id":"model-a"`, `"model":{"id":"model-a"`} {
		if !containsJSONFragment(text, expected) {
			t.Fatalf("response %s missing %s", text, expected)
		}
	}
}

func TestGetModelRejectsCrossUserModelAndProvider(t *testing.T) {
	tests := []struct {
		name          string
		modelOwner    string
		providerOwner string
	}{
		{name: "model belongs to another user", modelOwner: "user-b", providerOwner: "user-a"},
		{name: "provider belongs to another user", modelOwner: "user-a", providerOwner: "user-b"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factory := &modelOwnerFactory{
				modelStore:    &modelOwnerStore{item: &iapiserver.ProviderModel{ObjectMeta: imachinery.ObjectMeta{ID: "model"}, OwnerUserID: test.modelOwner, ProviderID: "provider", Enabled: true}},
				providerStore: &providerOwnerStore{item: &iapiserver.Provider{ObjectMeta: imachinery.ObjectMeta{ID: "provider"}, OwnerUserID: test.providerOwner, Enabled: true}},
			}
			service := &aiChatService{store: factory}
			if _, err := service.getModel(context.Background(), "user-a", "model"); !isRecordNotFound(err) {
				t.Fatalf("cross-user model error = %v", err)
			}
		})
	}
}

func containsJSONFragment(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
