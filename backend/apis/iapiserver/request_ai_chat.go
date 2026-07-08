package iapiserver

import (
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

type AIChatAssistantUpsertRequest struct {
	// ID 为空表示创建助手，非空表示更新当前用户可管理的助手。
	ID string `json:"id"`
	// Name 是助手展示名称，用户助手名称需在当前用户下可区分。
	Name string `json:"name"                binding:"required,min=1,max=128"`
	// SuggestedModelID 指向 model-management provider model，用作助手推荐模型。
	SuggestedModelID string `json:"suggested_model_id"`
	// UseSuggestedModel 控制生成时是否优先采用助手推荐模型。
	UseSuggestedModel bool `json:"use_suggested_model"`
	// SystemPrompt 是助手系统提示词，请求层限制最大长度。
	SystemPrompt string `json:"system_prompt"       binding:"omitempty,max=20000"`
	// RuntimeConfig 保存助手运行参数，服务端会写入 runtime_config_json。
	RuntimeConfig map[string]any `json:"runtime_config"`
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
	// Items 返回当前用户可见的系统助手和用户助手，不包含原始模型密钥信息。
	Items []*AIChatAssistant `json:"items"`
}

type AIChatTopicListRequest struct {
	imachinery.BasicQueryParam
	// Q 按话题标题做轻量搜索。
	Q string `json:"q"      form:"q"`
	// Pinned 过滤置顶或非置顶话题，空值表示不过滤。
	Pinned *bool `json:"pinned" form:"pinned"`
}

type AIChatTopicCreateRequest struct {
	// Title 是话题标题，允许为空并由后续消息生成默认标题。
	Title string `json:"title"        binding:"omitempty,max=160"`
	// AssistantID 指定话题默认助手，空值时使用系统默认助手逻辑。
	AssistantID string `json:"assistant_id"`
	// ModelID 指定话题默认 provider model，空值时使用默认模型配置。
	ModelID string `json:"model_id"`
}

func (r *AIChatTopicCreateRequest) Validate() error {
	r.Title = strings.TrimSpace(r.Title)
	if len([]rune(r.Title)) > 160 {
		return errors.Errorf("title length must be <= 160")
	}
	return nil
}

type AIChatTopicUpdateRequest struct {
	// ID 指定要更新的话题，由路径参数写入。
	ID string `json:"id"`
	// Title 更新话题标题，非空时需满足长度约束。
	Title *string `json:"title"        binding:"omitempty,min=1,max=160"`
	// Pinned 更新话题置顶状态，空值表示不修改。
	Pinned *bool `json:"pinned"`
	// AssistantID 更新话题默认助手，空值表示不修改。
	AssistantID *string `json:"assistant_id"`
	// ModelID 更新话题默认 provider model，空值表示不修改。
	ModelID *string `json:"model_id"`
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
	// Total 返回当前查询条件下的话题总数。
	Total int64 `json:"total"`
	// Items 返回当前页话题元数据，不返回消息正文列表。
	Items []*AIChatTopic `json:"items"`
}

type AIChatImageAttachmentInput struct {
	// ID 是客户端临时附件 ID，用于前端关联展示。
	ID string `json:"id"`
	// MessageID 预留给已存在消息附件关联，创建消息时通常为空。
	MessageID string `json:"message_id"`
	// FileName 保存客户端文件名，仅用于展示和诊断。
	FileName string `json:"file_name"`
	// MimeType 限制为受支持的图片 MIME 类型。
	MimeType string `json:"mime_type"   binding:"required,oneof=image/jpeg image/png image/webp image/bmp"`
	// SizeBytes 限制单张图片最大 5MiB。
	SizeBytes int `json:"size_bytes"  binding:"required,lte=5242880"`
	// Base64Data 保存请求内联图片内容，服务端不会在列表响应中原样返回。
	Base64Data string `json:"base64_data" binding:"required"`
	// NotPersisted 标识客户端仅临时传入图片，当前不进入 asset-library。
	NotPersisted bool `json:"not_persisted"`
}

type AIChatMessageCreateRequest struct {
	// TopicID 指定消息所属话题，由路径或请求体写入。
	TopicID string `json:"topic_id"`
	// Operation 表示本次请求是普通对话还是翻译。
	Operation string `json:"operation"         binding:"required,oneof=chat translate"`
	// ClientMessageID 是客户端幂等键，用于避免重复创建用户消息。
	ClientMessageID string `json:"client_message_id"`
	// Content 是用户输入正文，对话和翻译均限制最大长度。
	Content string `json:"content"           binding:"omitempty,max=20000"`
	// AssistantID 覆盖话题默认助手，仅影响本次生成。
	AssistantID string `json:"assistant_id"`
	// ModelID 覆盖话题或助手默认模型，引用 model-management provider model。
	ModelID string `json:"model_id"`
	// SlashCommand 保存前端解析出的快捷命令，服务端按当前草稿透传处理。
	SlashCommand string `json:"slash_command"`
	// TargetLanguage 是翻译操作的目标语言。
	TargetLanguage string `json:"target_language"`
	// Images 保存本次消息的图片附件，翻译操作不允许携带图片。
	Images []*AIChatImageAttachmentInput `json:"attachments"`
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
	// Items 按 created_at asc、version asc 返回话题消息。
	Items []*AIChatMessage `json:"items"`
}

type AIChatTranslationResponse struct {
	// MessageID 返回被翻译消息 ID；纯文本翻译时可为空。
	MessageID string `json:"message_id"`
	// SourceLanguage 返回检测或指定的源语言。
	SourceLanguage string `json:"source_language"`
	// TargetLanguage 返回请求的目标语言。
	TargetLanguage string `json:"target_language"`
	// TranslatedContent 返回翻译后的文本内容。
	TranslatedContent string `json:"translated_content"`
	// ModelSnapshot 返回本次翻译使用的 provider model 快照。
	ModelSnapshot map[string]any `json:"model_snapshot"`
}

type AIChatTranslationRequest struct {
	// MessageID 指定要翻译的消息；为空时按纯文本翻译处理。
	MessageID string `json:"message_id"`
	// Content 是纯文本翻译内容，有 MessageID 时可由服务端读取原消息。
	Content string `json:"content"        binding:"required,min=1,max=20000"`
	// TargetLanguage 是翻译目标语言。
	TargetLanguage string `json:"target_language" binding:"required"`
}

type AIChatEditRegenerateRequest struct {
	// MessageID 指向被编辑或再生成的消息。
	MessageID string `json:"message_id"`
	// Content 是编辑后的用户内容。
	Content string `json:"content"           binding:"required,min=1,max=20000"`
	// ClientMessageID 是客户端幂等键，用于避免重复再生成。
	ClientMessageID string `json:"client_message_id"`
	// Images 保存再生成时携带的图片附件。
	Images []*AIChatImageAttachmentInput `json:"images"`
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
	// Q 按标题或内容搜索快捷短语。
	Q string `json:"q"            form:"q"`
	// Scope 过滤 global 或 assistant 作用域。
	Scope string `json:"scope"        form:"scope"`
	// AssistantID 在 assistant 作用域下过滤某个助手的短语。
	AssistantID string `json:"assistant_id" form:"assistant_id"`
}

type AIChatQuickPhraseUpsertRequest struct {
	// ID 为空表示创建快捷短语，非空表示更新。
	ID string `json:"id"`
	// Title 是快捷短语展示标题。
	Title string `json:"title"        binding:"required,min=1,max=128"`
	// Content 是插入到输入框或提示词中的文本。
	Content string `json:"content"      binding:"required,min=1,max=20000"`
	// Scope 指定短语可见范围，assistant 范围需配合 AssistantID。
	Scope string `json:"scope"        binding:"required,oneof=global assistant"`
	// AssistantID 指定助手级短语所属助手。
	AssistantID string `json:"assistant_id"`
	// PhraseType 区分普通文本和提示词短语，空值默认 plain。
	PhraseType string `json:"phrase_type"`
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
	// Items 返回当前用户可见的快捷短语列表。
	Items []*AIChatQuickPhrase `json:"items"`
}

type AIChatDeleteResponse struct {
	// Deleted 表示软删除请求是否已完成。
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
