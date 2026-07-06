package iapiserver

import (
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

type AIChatModelListRequest struct {
	imachinery.BasicQueryParam
	Provider   string `json:"provider"   form:"provider"`
	Capability string `json:"capability" form:"capability"`
	Enabled    *bool  `json:"enabled"    form:"enabled"`
}

type AIChatModelListResponse struct {
	Total int64          `json:"total"`
	Items []*AIChatModel `json:"items"`
}

type AIChatAssistantUpsertRequest struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"                binding:"required,min=1,max=128"`
	SuggestedModelID  string         `json:"suggested_model_id"`
	UseSuggestedModel bool           `json:"use_suggested_model"`
	SystemPrompt      string         `json:"system_prompt"       binding:"omitempty,max=20000"`
	RuntimeConfig     map[string]any `json:"runtime_config"`
}

func (r *AIChatAssistantUpsertRequest) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return errors.Errorf("assistant name is required")
	}
	if len([]rune(r.Name)) > 128 {
		return errors.Errorf("assistant name length must be <= 128")
	}
	if len([]rune(r.SystemPrompt)) > 20000 {
		return errors.Errorf("system_prompt length must be <= 20000")
	}
	return nil
}

type AIChatAssistantListResponse struct {
	Items []*AIChatAssistant `json:"items"`
}

type AIChatTopicListRequest struct {
	imachinery.BasicQueryParam
	Q      string `json:"q"      form:"q"`
	Pinned *bool  `json:"pinned" form:"pinned"`
}

type AIChatTopicCreateRequest struct {
	Title       string `json:"title"        binding:"omitempty,max=160"`
	AssistantID string `json:"assistant_id"`
	ModelID     string `json:"model_id"`
}

func (r *AIChatTopicCreateRequest) Validate() error {
	r.Title = strings.TrimSpace(r.Title)
	if len([]rune(r.Title)) > 160 {
		return errors.Errorf("title length must be <= 160")
	}
	return nil
}

type AIChatTopicUpdateRequest struct {
	ID          string  `json:"id"`
	Title       *string `json:"title"        binding:"omitempty,min=1,max=160"`
	Pinned      *bool   `json:"pinned"`
	AssistantID *string `json:"assistant_id"`
	ModelID     *string `json:"model_id"`
}

func (r *AIChatTopicUpdateRequest) Validate() error {
	if r.Title != nil {
		title := strings.TrimSpace(*r.Title)
		if title == "" {
			return errors.Errorf("title is required")
		}
		if len([]rune(title)) > 160 {
			return errors.Errorf("title length must be <= 160")
		}
		*r.Title = title
	}
	return nil
}

type AIChatTopicListResponse struct {
	Total int64          `json:"total"`
	Items []*AIChatTopic `json:"items"`
}

type AIChatImageAttachmentInput struct {
	FileName   string `json:"file_name"`
	MimeType   string `json:"mime_type"   binding:"required,oneof=image/jpeg image/png image/webp image/bmp"`
	SizeBytes  int    `json:"size_bytes"  binding:"required,lte=5242880"`
	Base64Data string `json:"base64_data" binding:"required"`
}

type AIChatMessageCreateRequest struct {
	TopicID         string                        `json:"topic_id"`
	Operation       string                        `json:"operation"         binding:"required,oneof=chat translate"`
	ClientMessageID string                        `json:"client_message_id"`
	Content         string                        `json:"content"           binding:"omitempty,max=20000"`
	AssistantID     string                        `json:"assistant_id"`
	ModelID         string                        `json:"model_id"`
	SlashCommand    string                        `json:"slash_command"`
	TargetLanguage  string                        `json:"target_language"`
	Images          []*AIChatImageAttachmentInput `json:"images"`
}

func (r *AIChatMessageCreateRequest) Validate() error {
	r.Operation = strings.TrimSpace(r.Operation)
	if r.Operation != AIChatOperationChat && r.Operation != AIChatOperationTranslate {
		return errors.Errorf("operation must be chat or translate")
	}
	if len([]rune(r.Content)) > 20000 {
		return errors.Errorf("content length must be <= 20000")
	}
	if strings.TrimSpace(r.Content) == "" && len(r.Images) == 0 {
		return errors.Errorf("content or images is required")
	}
	if r.Operation == AIChatOperationTranslate && len(r.Images) > 0 {
		return errors.Errorf("translate operation does not support images")
	}
	for _, image := range r.Images {
		if err := validateAIChatImage(image); err != nil {
			return err
		}
	}
	return nil
}

type AIChatMessageListResponse struct {
	Items []*AIChatMessage `json:"items"`
}

type AIChatTranslationResponse struct {
	MessageID         string         `json:"message_id"`
	SourceLanguage    string         `json:"source_language"`
	TargetLanguage    string         `json:"target_language"`
	TranslatedContent string         `json:"translated_content"`
	ModelSnapshot     map[string]any `json:"model_snapshot"`
}

type AIChatEditRegenerateRequest struct {
	MessageID       string                        `json:"message_id"`
	Content         string                        `json:"content"           binding:"required,min=1,max=20000"`
	ClientMessageID string                        `json:"client_message_id"`
	Images          []*AIChatImageAttachmentInput `json:"images"`
}

func (r *AIChatEditRegenerateRequest) Validate() error {
	r.Content = strings.TrimSpace(r.Content)
	if r.Content == "" {
		return errors.Errorf("content is required")
	}
	if len([]rune(r.Content)) > 20000 {
		return errors.Errorf("content length must be <= 20000")
	}
	for _, image := range r.Images {
		if err := validateAIChatImage(image); err != nil {
			return err
		}
	}
	return nil
}

type AIChatQuickPhraseListRequest struct {
	imachinery.BasicQueryParam
	Q           string `json:"q"            form:"q"`
	Scope       string `json:"scope"        form:"scope"`
	AssistantID string `json:"assistant_id" form:"assistant_id"`
}

type AIChatQuickPhraseUpsertRequest struct {
	ID          string `json:"id"`
	Title       string `json:"title"        binding:"required,min=1,max=128"`
	Content     string `json:"content"      binding:"required,min=1,max=20000"`
	Scope       string `json:"scope"        binding:"required,oneof=global assistant"`
	AssistantID string `json:"assistant_id"`
	PhraseType  string `json:"phrase_type"`
}

func (r *AIChatQuickPhraseUpsertRequest) Validate() error {
	r.Title = strings.TrimSpace(r.Title)
	r.Content = strings.TrimSpace(r.Content)
	if r.Title == "" {
		return errors.Errorf("title is required")
	}
	if r.Content == "" {
		return errors.Errorf("content is required")
	}
	if len([]rune(r.Title)) > 128 {
		return errors.Errorf("title length must be <= 128")
	}
	if len([]rune(r.Content)) > 20000 {
		return errors.Errorf("content length must be <= 20000")
	}
	if r.Scope != AIChatQuickPhraseScopeGlobal && r.Scope != AIChatQuickPhraseScopeAssistant {
		return errors.Errorf("scope must be global or assistant")
	}
	if r.PhraseType == "" {
		r.PhraseType = AIChatQuickPhraseTypeText
	}
	if r.PhraseType != AIChatQuickPhraseTypeText && r.PhraseType != AIChatQuickPhraseTypePrompt {
		return errors.Errorf("phrase_type must be text or prompt")
	}
	return nil
}

type AIChatQuickPhraseListResponse struct {
	Items []*AIChatQuickPhrase `json:"items"`
}

type AIChatDeleteResponse struct {
	Deleted bool `json:"deleted"`
}

func validateAIChatImage(image *AIChatImageAttachmentInput) error {
	if image == nil {
		return errors.Errorf("image attachment is required")
	}
	switch image.MimeType {
	case "image/jpeg", "image/png", "image/webp", "image/bmp":
	default:
		return errors.Errorf("image mime_type is unsupported")
	}
	if image.SizeBytes > AIChatImageMaxSizeBytes {
		return errors.Errorf("image size_bytes must be <= 5242880")
	}
	if image.SizeBytes <= 0 {
		return errors.Errorf("image size_bytes is required")
	}
	if strings.TrimSpace(image.Base64Data) == "" {
		return errors.Errorf("image base64_data is required")
	}
	return nil
}
