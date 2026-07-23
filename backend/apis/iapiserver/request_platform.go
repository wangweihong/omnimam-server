package iapiserver

import (
	"errors"
	"strings"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

type (
	MeResponse struct {
		User         MeUser          `json:"user"`
		Roles        []string        `json:"roles"`
		Permissions  []string        `json:"permissions"`
		FeatureFlags map[string]bool `json:"feature_flags"`
	}

	MeUser struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
)

type (
	ProviderListRequest struct {
		imachinery.BasicQueryParam
		// OwnerUserID 由服务端上下文注入，客户端不能直接指定。
		OwnerUserID string `json:"-"            form:"-"`
		// Type 按 provider_type 过滤 provider 列表。
		Type string `json:"provider_type" form:"provider_type"`
		// Enabled 过滤启用或禁用的 provider，空值表示不过滤。
		Enabled *bool `json:"enabled"      form:"enabled"`
	}

	ProviderListResponse struct {
		// Total 返回当前查询条件下的 provider 总数。
		Total int64 `json:"total"`
		// Items 返回当前页 provider，不包含 API key 明文。
		Items []*Provider `json:"items"`
	}

	ProviderCreateRequest struct {
		// Name 是 provider 展示名称。
		Name string `json:"name"           binding:"required"`
		// Type 表示 provider 协议类型，当前 S2 使用 openai-compatible。
		Type string `json:"provider_type"   binding:"required"`
		// Enabled 控制 provider 创建后是否立即可用，空值使用服务端默认。
		Enabled *bool `json:"enabled"`
		// BaseURL 是 provider API 入口地址。
		BaseURL string `json:"api_base_url"     binding:"required"`
		// AuthType 表示鉴权方式，当前实现使用 api_key。
		AuthType string `json:"auth_type"       binding:"required"`
		// CredentialRef 引用凭据存储中的 API key，不接收明文密钥。
		CredentialRef string `json:"api_key_ref"`
		// PresetKey 是旧 preset 导入入口的内部参数，不属于 S2 请求体。
		PresetKey string `json:"-"`
		// Config 保存 provider 额外连接配置。
		Config map[string]any `json:"extra_config"`
		// Description 保存 provider 说明文本。
		Description string `json:"description"`
	}

	ProviderUpdateRequest struct {
		// ID 指定要更新的 provider，由路径参数写入。
		ID string `json:"id"`
		// Name 更新 provider 展示名称，空指针表示不修改。
		Name *string `json:"name"`
		// Type 更新 provider 协议类型，空指针表示不修改。
		Type *string `json:"provider_type"`
		// Enabled 更新 provider 可用状态，空指针表示不修改。
		Enabled *bool `json:"enabled"`
		// BaseURL 更新 provider API 入口地址，空指针表示不修改。
		BaseURL *string `json:"api_base_url"`
		// AuthType 更新鉴权方式，空指针表示不修改。
		AuthType *string `json:"auth_type"`
		// CredentialRef 更新凭据引用，空指针表示不修改。
		CredentialRef *string `json:"api_key_ref"`
		// Config 更新 provider 额外连接配置，空指针表示不修改。
		Config *map[string]any `json:"extra_config"`
		// Description 更新 provider 描述，空指针表示不修改。
		Description *string `json:"description"`
	}

	ProviderModelTypeRule struct {
		// Contains 保存匹配上游模型名的关键字集合。
		Contains []string `json:"contains"`
		// ModelTypes 是匹配成功后归类出的模型类型。
		ModelTypes []string `json:"model_types"`
		// GroupName 是模型选项展示分组。
		GroupName string `json:"group_name"`
		// EndpointType 是 provider 同步时推导的内部端点类型。
		EndpointType string `json:"endpoint_type"`
	}

	// ProviderTestRequest tests a provider connection with optional unsaved form overrides.
	// It only validates metadata/API reachability and does not persist credentials or create tasks.
	ProviderTestRequest struct {
		// ID 指定已保存的 provider；为空时使用请求中的临时配置做检测。
		ID string `json:"id"`
		// BaseURL 是未保存配置的临时 API 入口地址。
		BaseURL string `json:"api_base_url"`
		// AuthType 是未保存配置的临时鉴权方式。
		AuthType string `json:"auth_type"`
		// CredentialRef 是未保存配置的临时凭据引用。
		CredentialRef string `json:"api_key_ref"`
	}

	ProviderTestResponse struct {
		// TargetType 表示检测目标是 provider 还是 provider model。
		TargetType string `json:"target_type"`
		// ProviderID 返回检测关联的 provider ID。
		ProviderID string `json:"provider_id"`
		// ModelID 返回检测关联的 provider model ID，provider 级检测可为空。
		ModelID string `json:"model_id,omitempty"`
		// Success 表示连接检测是否成功。
		Success bool `json:"success"`
		// HealthStatus 返回检测后的健康状态。
		HealthStatus string `json:"health_status"`
		// Message 返回检测结果说明或失败原因。
		Message string `json:"message"`
		// CheckedAt 记录检测发生时间。
		CheckedAt imachinery.Time `json:"checked_at"`
	}

	ProviderModelListRequest struct {
		imachinery.BasicQueryParam
		// OwnerUserID 由服务端上下文注入，客户端不能直接指定。
		OwnerUserID string `json:"-"          form:"-"`
		// ProviderID 过滤某个 provider 下的模型。
		ProviderID string `json:"provider_id" form:"provider_id"`
		// Enabled 过滤启用或禁用的模型，空值表示不过滤。
		Enabled *bool `json:"enabled"     form:"enabled"`
		// Capability 过滤具备某项业务能力的模型。
		Capability string `json:"capability"  form:"capability"`
		// Usage 按默认模型用途过滤可选模型。
		Usage string `json:"usage"       form:"usage"`
	}

	ProviderModelListResponse struct {
		// Total 返回当前查询条件下的模型总数。
		Total int64 `json:"total"`
		// Items 返回 provider model 列表，包含健康状态但不包含凭据。
		Items []*ProviderModel `json:"items"`
	}

	ProviderModelCreateRequest struct {
		// ProviderID 指定模型所属 provider。
		ProviderID string `json:"provider_id"`
		// Name 是模型展示名，对外字段为 display_name。
		Name string `json:"display_name"    binding:"required"`
		// Model 是上游 provider 的真实模型标识。
		Model string `json:"model"          binding:"required"`
		// EndpointType 是旧同步逻辑内部字段，不属于 S2 请求体。
		EndpointType string `json:"-"`
		// GroupName 是模型选项展示分组。
		GroupName string `json:"group"`
		// Capabilities 声明模型支持的业务能力。
		Capabilities []string `json:"capabilities"`
		// ModelTypes 是旧同步逻辑内部分类结果，不属于 S2 请求体。
		ModelTypes []string `json:"-"`
		// StreamSupported 表示模型是否支持流式输出，空值使用默认 true。
		StreamSupported *bool `json:"stream_supported"`
		// Enabled 控制模型创建后是否可选，空值使用服务端默认。
		Enabled *bool `json:"enabled"`
		// DefaultParams 是旧草稿字段，当前 S2 不接收。
		DefaultParams map[string]any `json:"-"`
		// Pricing 是旧草稿字段，当前 S2 不接收。
		Pricing map[string]any `json:"-"`
	}

	ProviderModelUpdateRequest struct {
		// ID 指定要更新的 provider model，由路径参数写入。
		ID string `json:"id"`
		// ProviderID 指定模型所属 provider，通常由路径或查询上下文确定。
		ProviderID string `json:"provider_id"`
		// Name 更新模型展示名，空指针表示不修改。
		Name *string `json:"display_name"`
		// Model 更新上游 provider 的真实模型标识，空指针表示不修改。
		Model *string `json:"model"`
		// EndpointType 是旧同步逻辑内部字段，当前不接收。
		EndpointType *string `json:"-"`
		// GroupName 更新模型选项展示分组，空指针表示不修改。
		GroupName *string `json:"group"`
		// Capabilities 更新模型能力集合，空指针表示不修改。
		Capabilities *[]string `json:"capabilities"`
		// ModelTypes 是旧同步逻辑内部字段，当前不接收。
		ModelTypes *[]string `json:"-"`
		// StreamSupported 更新流式输出能力，空指针表示不修改。
		StreamSupported *bool `json:"stream_supported"`
		// Enabled 更新模型可用状态，空指针表示不修改。
		Enabled *bool `json:"enabled"`
		// DefaultParams 是旧草稿字段，当前不接收。
		DefaultParams *map[string]any `json:"-"`
		// Pricing 是旧草稿字段，当前不接收。
		Pricing *map[string]any `json:"-"`
	}

	// ProviderModelSyncRequest imports remote OpenAI-compatible model metadata into ProviderModel rows.
	// It creates or updates model metadata only; it never invokes a model generation task.
	ProviderModelSyncRequest struct {
		// ProviderID 指定需要同步远端模型列表的 provider。
		ProviderID string `json:"provider_id"`
	}

	ProviderModelSyncResponse struct {
		// Total 返回本次同步扫描到的远端模型数量。
		Total int `json:"total"`
		// Created 返回本次新增的 provider model 数量。
		Created int `json:"created"`
		// Updated 返回本次更新的 provider model 数量。
		Updated int `json:"updated"`
		// Skipped 返回因规则或重复而跳过的模型数量。
		Skipped int `json:"skipped"`
		// Models 在 S2 允许时返回本次创建或更新后的模型列表。
		Models []*ProviderModel `json:"models,omitempty"`
	}

	// ProviderModelHealthCheckResponse 返回单个模型的最新连接健康状态。
	// 该接口只执行模型 metadata 检测，不触发模型生成任务。
	ProviderModelHealthCheckResponse = ProviderTestResponse
)

func (r *ProviderUpdateRequest) Validate() error {

	return nil
}

type (
	SystemLLMConfigListResponse struct {
		// Configs 返回当前用户所有用途的默认模型配置。
		Configs []*SystemLLMConfig `json:"configs"`
	}

	SystemLLMConfigUpsertRequest struct {
		// Configs 批量保存默认模型配置，当前请求会按 usage 覆盖。
		Configs []*SystemLLMConfigSpec `json:"configs" binding:"required"`
	}

	SystemLLMConfigSpec struct {
		// Purpose 对应 S2 usage，表示 chat、translation 等业务用途。
		Purpose string `json:"purpose"     binding:"required"`
		// ProviderID 指定默认模型所属 provider。
		ProviderID string `json:"provider_id" binding:"required"`
		// ModelID 指定默认模型配置选中的 provider model。
		ModelID string `json:"model_id"`
		// Model 是旧草稿兼容字段，S2 默认模型配置以 model_id 为准。
		Model string `json:"model"`
		// Enabled 是旧草稿兼容字段，当前以配置记录存在表示启用。
		Enabled *bool `json:"enabled"`
	}

	DefaultModelSaveRequest struct {
		// ProviderID 指定默认模型所属 provider。
		ProviderID string `json:"provider_id" binding:"required"`
		// ModelID 指定某个 usage 的默认 provider model。
		ModelID string `json:"model_id"    binding:"required"`
	}

	ModelOptionListResponse struct {
		// Total 返回可选模型总数。
		Total int64 `json:"total"`
		// Items 返回可选 provider model 列表，按健康状态和启用状态过滤。
		Items []*ProviderModel `json:"items"`
	}
)

type (
	StorageBackendListRequest struct {
		imachinery.BasicQueryParam
		// Type 按已发布的存储后端类型过滤全局配置。
		Type string `json:"type" form:"type" binding:"omitempty,oneof=local s3 minio oss cos azure_blob"`
		// Enabled 按启用状态过滤；空值表示不过滤。
		Enabled *bool `json:"enabled" form:"enabled"`
	}

	StorageBackendListResponse struct {
		imachinery.ListRet
		// Items 是规范列表字段，单次查询后生成。
		Items []*StorageBackendDetail `json:"items"`
		// Backends 是旧客户端兼容别名，内容必须与 Items 相同。
		Backends []*StorageBackendDetail `json:"backends"`
	}

	StorageBackendCreateRequest struct {
		// Name 是管理员识别该物理后端的显示名称。
		Name string `json:"name" binding:"required,max=255"`
		// Type 选择存储协议；运行时是否已提供对应 adapter 由 bootstrap 决定。
		Type string `json:"type" binding:"required,oneof=local s3 minio oss cos azure_blob"`
		// Root 是完整物理根位置，最长 1024 字符。
		Root string `json:"root" binding:"max=1024"`
		// Config 是完整后端配置，管理员接口原样保存和返回。
		Config map[string]any `json:"config"`
		// Enabled 控制运行时是否可选择该后端；省略时默认 true。
		Enabled *bool `json:"enabled"`
		// Readonly 禁止运行时向该后端写入；省略时默认 false。
		Readonly *bool `json:"readonly"`
		// Quota 是非负字节配额，零表示未设置。
		Quota int64 `json:"quota" binding:"gte=0"`
	}

	StorageBackendUpdateRequest struct {
		// ID 来自 backend_id 路径参数，客户端 JSON 不能覆盖。
		ID string `json:"-"`
		// Name 非空时替换显示名称。
		Name *string `json:"name" binding:"omitempty,min=1,max=255"`
		// Type 非空时替换存储协议。
		Type *string `json:"type" binding:"omitempty,oneof=local s3 minio oss cos azure_blob"`
		// Root 非空时替换完整物理根位置。
		Root *string `json:"root" binding:"omitempty,max=1024"`
		// Config 非空时整体替换完整后端配置。
		Config *map[string]any `json:"config"`
		// Enabled 非空时切换运行时选择状态。
		Enabled *bool `json:"enabled"`
		// Readonly 非空时切换只读状态。
		Readonly *bool `json:"readonly"`
		// Quota 非空时替换非负字节配额。
		Quota *int64 `json:"quota" binding:"omitempty,gte=0"`
	}
)

// SetDefaults 对齐 StorageBackend 列表契约的默认分页大小 50。
func (r *StorageBackendListRequest) SetDefaults() {
	if r.PageSize == 0 {
		r.PageSize = 50
	}
}

// Validate 限制管理员存储列表每页最多返回 200 项。
func (r *StorageBackendListRequest) Validate() error {
	if r.PageSize > 200 {
		return errors.New("page_size must be less than or equal to 200")
	}
	_, err := r.PagingParams.Normalize()
	return err
}

// Validate 确保 PATCH 至少包含一个已发布的可更新字段。
func (r *StorageBackendUpdateRequest) Validate() error {
	if r.Name == nil && r.Type == nil && r.Root == nil && r.Config == nil && r.Enabled == nil && r.Readonly == nil && r.Quota == nil {
		return errors.New("at least one storage backend field is required")
	}
	return nil
}

type (
	AssetListRequest struct {
		imachinery.BasicQueryParam
		MediaType        string   `json:"media_type"         form:"media_type"`
		MimeType         string   `json:"mime_type"          form:"mime_type"`
		StorageBackendID string   `json:"storage_backend_id" form:"storage_backend_id"`
		SourceType       string   `json:"source_type"        form:"source_type"`
		Format           string   `json:"format"             form:"format"`
		Status           string   `json:"status"             form:"status"`
		Deleted          *bool    `json:"deleted"            form:"deleted"`
		MinSize          int64    `json:"min_size"           form:"min_size"`
		MaxSize          int64    `json:"max_size"           form:"max_size"`
		Width            int      `json:"width"              form:"width"`
		Height           int      `json:"height"             form:"height"`
		MinWidth         int      `json:"min_width"          form:"min_width"`
		MaxWidth         int      `json:"max_width"          form:"max_width"`
		MinHeight        int      `json:"min_height"         form:"min_height"`
		MaxHeight        int      `json:"max_height"         form:"max_height"`
		MinDuration      int64    `json:"min_duration"       form:"min_duration"`
		MaxDuration      int64    `json:"max_duration"       form:"max_duration"`
		Tags             []string `json:"tags"               form:"tags"`
	}

	AssetSearchRequest struct {
		Query AssetListRequest `json:"query"`
	}

	AssetSearchParseRequest struct {
		Text string `json:"text" binding:"required"`
	}

	AssetSearchParseResponse struct {
		Query        AssetListRequest `json:"query"`
		AtomicTaskID string           `json:"atomic_task_id,omitempty"`
	}

	AssetListResponse struct {
		imachinery.ListRet
		Assets []*AssetRecord `json:"assets"`
	}

	AssetRecord struct {
		*Asset
		Thumbnail *AssetThumbnail `json:"thumbnail,omitempty"`
		Tags      []*Tag          `json:"tags,omitempty"`
	}

	AssetUploadResponse struct {
		Asset       *AssetRecord  `json:"asset"`
		AtomicTasks []*AtomicTask `json:"atomic_tasks,omitempty"`
	}

	AssetChunkUploadInitRequest struct {
		Filename    string   `json:"filename"     binding:"required"`
		Size        int64    `json:"size"         binding:"required"`
		Checksum    string   `json:"checksum"     binding:"required"`
		ChunkSize   int64    `json:"chunk_size"   binding:"required"`
		TotalChunks int      `json:"total_chunks" binding:"required"`
		TagNames    []string `json:"tag_names"`
		SourceType  string   `json:"source_type"`
	}

	AssetChunkUploadInitResponse struct {
		Checksum       string `json:"checksum"`
		UploadedChunks []int  `json:"uploaded_chunks"`
		ChunkSize      int64  `json:"chunk_size"`
		TotalChunks    int    `json:"total_chunks"`
		ExpiresHours   int    `json:"expires_hours"`
	}

	AssetChunkUploadPartResponse struct {
		Checksum string `json:"checksum"`
		Index    int    `json:"index"`
		Size     int64  `json:"size"`
	}

	AssetChunkUploadCompleteRequest struct {
		Filename    string   `json:"filename"     binding:"required"`
		Size        int64    `json:"size"         binding:"required"`
		Checksum    string   `json:"checksum"     binding:"required"`
		ChunkSize   int64    `json:"chunk_size"   binding:"required"`
		TotalChunks int      `json:"total_chunks" binding:"required"`
		TagNames    []string `json:"tag_names"`
		SourceType  string   `json:"source_type"`
	}

	AssetChunkUploadCancelResponse struct {
		Checksum string `json:"checksum"`
		Deleted  bool   `json:"deleted"`
	}

	AssetUpdateRequest struct {
		ID          string          `json:"id"`
		Name        *string         `json:"name"`
		SourceType  *string         `json:"source_type"`
		SourceRef   *string         `json:"source_ref"`
		Metadata    *map[string]any `json:"metadata"`
		TagNames    *[]string       `json:"tag_names"`
		TagSource   string          `json:"tag_source"`
		Description *string         `json:"description"`
	}
)

func (r *AssetSearchRequest) PostBind() error {
	return r.Query.PostBind()
}

func (r *AssetListRequest) PostBind() error {
	r.Tags = splitListValues(r.Tags)
	r.SearchFields = splitListValues(r.SearchFields)
	r.Status = strings.ToLower(strings.TrimSpace(r.Status))
	return nil
}

func splitListValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		parts := strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == '，' || r == ';' || r == '；' || r == '\n' || r == '\t'
		})
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				result = append(result, part)
			}
		}
	}
	return result
}

type (
	AssetGroupCreateRequest struct {
		Name        string         `json:"name"         binding:"required"`
		Type        string         `json:"type"`
		Description string         `json:"description"`
		DynamicRule map[string]any `json:"dynamic_rule"`
		AssetIDs    []string       `json:"asset_ids"`
	}

	AssetGroupCreateResponse struct {
		Group   *AssetGroup         `json:"group"`
		Members []*AssetGroupMember `json:"members"`
	}
)

type ()
