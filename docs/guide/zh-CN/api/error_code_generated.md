# 错误码

！！系统错误码列表，由 `codegen -type=int -doc` 命令生成，不要对此文件做任何更改。

## 功能说明

如果返回结果中存在 `code` 字段，则表示调用 API 接口失败。例如：

```json
{
  "code": 100101,
  "messageEN": "Database error",
  "messageCN": "数据库出错"
}
```

上述返回中 `code` 表示错误码，`message` 表示该错误的具体信息。每个错误同时也对应一个 HTTP 状态码，比如上述错误码对应了 HTTP 状态码 500(Internal Server Error)。

## 错误码列表

系统支持的错误码列表如下：

| Identifier | Code | HTTP Code | Description |  中文描述 	|
| ---------- | ---- | --------- | ----------- | ----------- |
| ErrSuccess | 100001 | 200 | Success. | 请求成功 |
| ErrUnknown | 100002 | 500 | Internal server error. | 服务器出错 |
| ErrBind | 100003 | 200 | Error occurred while binding the request body to the struct. | 解析结构体出错 |
| ErrValidation | 100004 | 200 | Validation failed. | 参数校验失败 |
| ErrTokenInvalid | 100005 | 401 | Token invalid. | 令牌无效 |
| ErrPageNotFound | 100006 | 404 | Page not found. | 请求路由不存在 |
| ErrOperationBatchExecute | 100007 | 200 | Operation batch execute. | 批量执行操作 |
| ErrMCPProtocolVersionUnsupported | 190200 | 200 | The requested MCP protocol version is not supported. | MCP 协议版本不受支持。 |
| ErrMCPRequestInvalid | 190201 | 200 | The MCP JSON-RPC request is invalid. | MCP JSON-RPC 请求结构无效。 |
| ErrMCPRequiredHeaderMissing | 190202 | 200 | A required MCP transport header is missing. | MCP 请求缺少必需的传输 Header。 |
| ErrMCPHeaderBodyMismatch | 190203 | 200 | MCP transport headers do not match the JSON-RPC body. | MCP Header 与 JSON-RPC Body 不一致。 |
| ErrMCPMethodUnsupported | 190204 | 200 | The MCP method is not supported. | 当前 MCP 方法不受支持。 |
| ErrMCPNameInvalid | 190205 | 200 | The MCP tool name or resource URI metadata is invalid. | MCP Tool 名称或 Resource URI 元数据无效。 |
| ErrMCPResponseMediaUnsupported | 190206 | 200 | The client did not advertise an acceptable MCP response media type. | 客户端未声明可接受的 MCP 响应媒体类型。 |
| ErrMCPToolNotVisible | 190400 | 200 | The tool does not exist or is not visible to the current principal. | Tool 不存在或当前主体不可见。 |
| ErrMCPToolArgumentInvalid | 190401 | 200 | Tool arguments do not conform to the published input schema. | Tool 参数不符合已发布输入 Schema。 |
| ErrMCPToolResultInvalid | 190402 | 200 | A downstream result cannot be converted to the tool output schema. | 下游结果无法转换为 Tool 输出 Schema。 |
| ErrMCPResourceURIInvalid | 190403 | 200 | The resource URI format or type is invalid. | Resource URI 格式或类型无效。 |
| ErrMCPResourceNotVisible | 190404 | 200 | The resource does not exist or is not visible to the current principal. | Resource 不存在或当前主体不可见。 |
| ErrMCPResourceTypeUnsupported | 190405 | 200 | The requested resource type is not supported. | 当前 Resource 类型不受支持。 |
| ErrMCPCursorInvalid | 190406 | 200 | The MCP pagination cursor is invalid or expired. | MCP 分页游标无效或已过期。 |
| ErrMCPTaskExtensionNotNegotiated | 190600 | 200 | The MCP Tasks extension was not negotiated for this request. | 客户端未为当前请求协商 MCP Tasks 扩展。 |
| ErrMCPTaskNotVisible | 190601 | 200 | The MCP task does not exist or is not visible to the current principal. | MCP Task 不存在或当前主体不可见。 |
| ErrMCPTaskBindingUnavailable | 190602 | 200 | The MCP task does not have a complete ApplicationRun/AtomicTask binding. | MCP Task 尚未形成完整的 ApplicationRun/AtomicTask 映射。 |
| ErrMCPTaskBindingExpired | 190603 | 200 | The MCP task binding has expired; query the ApplicationRun instead. | MCP Task 映射已过期，请通过 ApplicationRun 查询事实。 |
| ErrMCPTaskNotCancellable | 190604 | 200 | The MCP task cannot be canceled in its current state. | MCP Task 当前状态不允许请求取消。 |
| ErrMCPTaskSourceUnavailable | 190605 | 200 | The ApplicationRun or AtomicTask backing the MCP task is unavailable. | MCP Task 对应的 ApplicationRun 或 AtomicTask 当前不可读取。 |
| ErrMCPAuthenticationRequired | 190800 | 200 | The request does not contain a valid Identity JWT. | 当前请求缺少有效 Identity JWT。 |
| ErrMCPPermissionDenied | 190801 | 200 | The current principal is not allowed to use this MCP protocol capability. | 当前主体无权使用该 MCP 协议能力。 |
| ErrMCPOriginRejected | 190802 | 200 | The request Origin is not allowed. | 请求 Origin 不在允许范围内。 |
| ErrMCPRequestTooLarge | 190803 | 200 | The MCP request body or headers exceed the configured limit. | MCP 请求体或 Header 超过允许上限。 |
| ErrMCPRateLimited | 190804 | 200 | The MCP request exceeds a rate or cost limit. | MCP 请求超过速率或费用限制。 |
| ErrMCPConcurrencyLimited | 190805 | 200 | The MCP ApplicationRun concurrency limit has been reached. | MCP ApplicationRun 并发超过允许上限。 |
| ErrMCPAuditUnavailable | 190806 | 200 | The security audit boundary is unavailable and the controlled operation was not executed. | 安全审计边界当前不可用，受控操作未执行。 |
| ErrNotificationNotVisible | 180200 | 200 | The notification does not exist or is not visible to the current user. | 通知不存在或不属于当前用户。 |
| ErrNotificationStateConflict | 180201 | 200 | The current notification state does not allow this inbox action. | 当前通知状态不允许执行该收件箱操作。 |
| ErrNotificationBatchInvalid | 180202 | 200 | A batch notification request must contain 1 to 200 unique notification identifiers. | 批量通知请求必须包含 1 至 200 个唯一通知标识。 |
| ErrNotificationQueryInvalid | 180203 | 200 | A notification filter, search, or sort parameter is invalid. | 通知筛选、搜索或排序参数无效。 |
| ErrNotificationPreferenceInvalid | 180400 | 200 | Notification preferences contain a duplicate scope, unknown topic, or incompatible category and topic. | 通知偏好包含重复范围、未知主题或不兼容的分类与主题。 |
| ErrNotificationMandatoryTopicDisabled | 180401 | 200 | In-app delivery cannot be disabled for mandatory system or security notifications. | 严重系统或安全通知不能关闭站内投递。 |
| ErrNotificationChannelUnsupported | 180402 | 200 | The requested notification channel or digest capability is not enabled in the current phase. | 当前阶段尚未启用所请求的通知渠道或摘要能力。 |
| ErrNotificationSourceEventInvalid | 180600 | 200 | The source event lacks a stable event identifier, aggregate version, recipient basis, or required rule fields. | 源事件缺少稳定事件标识、聚合版本、接收者依据或通知规则必需字段。 |
| ErrNotificationSourceEventUnsupported | 180601 | 200 | The source event, event version, or notification topic is not enabled. | 源事件、事件版本或通知主题尚未启用。 |
| ErrNotificationRecipientUnresolved | 180602 | 200 | A notification recipient cannot be resolved from the source event or controlled projection. | 无法从源事件或受控投影确定通知接收者。 |
| ErrNotificationRuleProcessingFailed | 180603 | 200 | Notification rule processing, deduplication, aggregation, or inbox persistence has temporarily failed. | 通知规则处理、去重、聚合或收件箱写入暂时失败。 |
| ErrNotificationPermissionDenied | 180800 | 200 | The caller cannot read or modify Notification Center resources. | 当前调用方无权读取或修改通知中心资源。 |
| ErrNotificationAdminScopeRequired | 180801 | 200 | The current subject is not eligible for the administrator notification scope. | 当前主体不满足管理员通知接收范围。 |
| ErrSSEConnectionLimitReached | 170200 | 200 | The real-time connection limit for the current user or client instance has been reached. | 当前用户或客户实例的实时连接数已达上限。 |
| ErrSSEStreamUnavailable | 170201 | 200 | The real-time event stream is temporarily unavailable; use fact-query fallback. | 实时事件流暂时不可用，请使用事实查询降级。 |
| ErrSSECursorConflict | 170400 | 200 | Last-Event-ID and after_event_id identify different resume positions. | Last-Event-ID 与 after_event_id 指向不同的恢复位置。 |
| ErrSSECursorNotVisible | 170401 | 200 | The resume cursor does not exist or is not visible to the current user; resynchronization is required. | 恢复游标不存在或不属于当前用户，需要重新同步。 |
| ErrSSECursorExpired | 170402 | 200 | The resume cursor is outside the retained event range; resynchronization is required. | 恢复游标已超出事件保留范围，需要重新同步。 |
| ErrSSESourceEventInvalid | 170600 | 200 | The upstream event lacks owner, resource version, or required projection fields. | 上游事件缺少所有者、资源版本或必需投影字段。 |
| ErrSSESourceEventUnsupported | 170601 | 200 | The upstream event version is not supported by the current SSE projector. | 上游事件版本尚不受当前 SSE 投影器支持。 |
| ErrSSEPermissionDenied | 170800 | 200 | The caller cannot open the event stream or read event history. | 当前调用方无权建立事件流或读取历史事件。 |
| ErrDatabase | 100101 | 500 | Database error. | 数据库出错 |
| ErrEncrypt | 100201 | 401 | Error occurred while encrypting the user password. | 用户密码加密失败 |
| ErrSignatureInvalid | 100202 | 401 | Signature is invalid. | 签名无效 |
| ErrExpired | 100203 | 401 | Token expired. | 令牌 |
| ErrInvalidAuthHeader | 100204 | 401 | Invalid authorization header. | 无效的请求授权头部 |
| ErrMissingHeader | 100205 | 401 | The `Authorization` header was empty. | 请求授权头部为空 |
| ErrPasswordIncorrect | 100206 | 401 | Password was incorrect. | 密码验证失败 |
| ErrPermissionDenied | 100207 | 403 | Permission denied. | 请求无权限执行 |
| ErrEncodingFailed | 100301 | 500 | Encoding failed due to an error with the data. | 数据编码出错 |
| ErrDecodingFailed | 100302 | 500 | Decoding failed due to an error with the data. | 数据解码出错 |
| ErrInvalidJSON | 100303 | 500 | Data is not valid JSON. | 数据非有效JSON结构 |
| ErrEncodingJSON | 100304 | 500 | JSON data could not be encoded. | JSON数据编码失败 |
| ErrDecodingJSON | 100305 | 500 | JSON data could not be decoded. | JSON数据解码失败 |
| ErrInvalidYaml | 100306 | 500 | Data is not valid Yaml. | 数据非有效YAML结构 |
| ErrEncodingYaml | 100307 | 500 | Yaml data could not be encoded. | YAML数据编码失败 |
| ErrDecodingYaml | 100308 | 500 | Yaml data could not be decoded. | YAML数据编码失败 |
| ErrHTTPError | 100501 | 500 | HTTP request error. | HTTP请求失败 |
| ErrHTTPResponseDataParseError | 100502 | 500 | Decode data from http response error. | 解析HTTP服务返回数据失败 |
| ErrHTTPClientGenerateError | 100503 | 500 | Generate HTTP client error. | 生成HTTP客户端失败 |
| ErrProviderUnauthorized | 100504 | 401 | Provider authentication failed. | 模型服务认证失败 |
| ErrProviderUnsupported | 100505 | 400 | Provider protocol is unsupported. | 模型服务协议不支持 |
| ErrProviderUnavailable | 100506 | 500 | Provider service is unavailable. | 模型服务不可用 |
| ErrProviderResponseParseError | 100507 | 500 | Provider response parse failed. | 模型服务响应解析失败 |
| ErrGRPCClientGenerateError | 100701 | 500 | Generate gRPC client error. | 生成gRPC客户端失败 |
| ErrGRPCClientCertificateError | 100702 | 500 | Validate gRPC client certificate error. | gRPC客户端证书错误 |
| ErrGRPCClientDialError | 100703 | 500 | Dial to gRPC server error. | gRPC客户端连接失败 |
| ErrGRPCClientInvokeServiceError | 100704 | 500 | Invoke gRPC server service function error. | gRPC客户端访问服务接口失败 |
| ErrGRPCResponseDataParseError | 100705 | 500 | Decode data from gRPC service error. | 解析gRPC服务返回数据失败 |
| ErrAIChatTopicNotFound | 110200 | 200 | Topic does not exist or is not visible to current user. | 话题不存在或当前用户不可见。 |
| ErrAIChatMessageNotFound | 110400 | 200 | Message does not exist or is not visible to current user. | 消息不存在或当前用户不可见。 |
| ErrAIChatMessageEmpty | 110401 | 200 | Message input is empty and has no image attachment. | 输入为空且没有图片附件。 |
| ErrAIChatAssistantNotFound | 110600 | 200 | Assistant does not exist or is not visible to current user. | 助手不存在或当前用户不可见。 |
| ErrAIChatAssistantSystemProtected | 110601 | 200 | System assistant cannot be deleted or protected fields cannot be changed. | 系统助手不可删除或修改受保护字段。 |
| ErrAIChatQuickPhraseInvalid | 110800 | 200 | Quick phrase is invalid or assistant-scoped phrase misses assistant. | 快捷短语不合法或助手级短语缺少助手。 |
| ErrAIChatConcurrentGeneration | 111000 | 200 | The topic already has an active generation. | 同一话题已有运行中的生成。 |
| ErrAIChatGenerationNotFound | 111001 | 200 | Generation run does not exist or is not visible to current user. | 生成运行不存在或当前用户不可见。 |
| ErrAIChatTranslationModelMissing | 111200 | 200 | Current user has not configured a default translation model. | 当前用户未配置默认翻译模型。 |
| ErrAIChatTranslationModelUnhealthy | 111201 | 200 | Default translation model is unavailable. | 默认翻译模型不可用。 |
| ErrAIChatAccessDenied | 111400 | 200 | Current user cannot access this chat resource. | 当前用户不能访问该聊天资源。 |
| ErrModelProviderNotFound | 120200 | 200 | Model provider does not exist or is not visible to current user. | 模型提供商不存在或当前用户不可见。 |
| ErrModelProviderNameDuplicated | 120201 | 200 | Provider name is duplicated in current user scope. | 当前用户范围内提供商名称重复。 |
| ErrModelProviderTestFailed | 120202 | 200 | Model provider connection test failed. | 模型提供商连接检测失败。 |
| ErrProviderModelNotFound | 120400 | 200 | Provider model does not exist or is not visible to current user. | 模型不存在或当前用户不可见。 |
| ErrProviderModelIdentifierInvalid | 120401 | 200 | Model identifier is empty or invalid. | 模型标识不能为空或不合法。 |
| ErrProviderModelDuplicated | 120402 | 200 | Model identifier or display name is duplicated under the same provider. | 同一提供商下模型标识或显示名重复。 |
| ErrDefaultModelMissing | 120600 | 200 | Current user has not configured a default model for the requested usage. | 当前用户未配置指定用途的默认模型。 |
| ErrDefaultModelInvalid | 120601 | 200 | Default model candidate is not available. | 默认模型候选不可用。 |
| ErrModelHealthCheckFailed | 120800 | 200 | Model health check failed. | 模型健康检测失败。 |
| ErrModelAccessDenied | 121000 | 200 | Current user is not allowed to access this model configuration. | 当前用户无权访问该模型配置。 |
| ErrAIAppProviderCapabilityUnavailable | 130229 | 200 | The ProviderCapability is currently unavailable or disabled. | ProviderCapability 当前不可用或已禁用。 |
| ErrAIAppProviderCapabilityNotFound | 130230 | 200 | The ProviderCapability does not exist. | ProviderCapability 不存在。 |
| ErrAIAppEngineInstanceNotFound | 130420 | 200 | The ApplicationEngineInstance does not exist. | ApplicationEngineInstance 不存在。 |
| ErrAIAppEngineAuthConfigInvalid | 130421 | 200 | The EngineInstance authentication configuration does not satisfy its EngineType. | EngineInstance 的鉴权配置不符合 EngineType 要求。 |
| ErrAIAppEngineBindingIncompatible | 130422 | 200 | The EngineCapabilityBinding is incompatible with its EngineType or ProviderCapability. | EngineCapabilityBinding 与 EngineType 或 ProviderCapability 不兼容。 |
| ErrAIAppEngineBindingRestrictionExpands | 130423 | 200 | EngineCapabilityBinding restrictions expand ProviderCapability capabilities. | EngineCapabilityBinding restrictions 扩张了 ProviderCapability 能力。 |
| ErrAIAppEngineUnavailable | 130424 | 200 | No enabled and healthy EngineInstance is currently available. | 当前没有可用且健康的 EngineInstance。 |
| ErrAIAppEngineReferenceBlocked | 130425 | 200 | The EngineInstance has historical ApplicationRun references and cannot be deleted. | EngineInstance 存在历史 ApplicationRun 引用，禁止删除。 |
| ErrAIAppEngineBindingNotFound | 130426 | 200 | The EngineCapabilityBinding does not exist. | EngineCapabilityBinding 不存在。 |
| ErrAIAppSystemEngineBindingImmutable | 130427 | 200 | A system-managed EngineCapabilityBinding cannot be created, modified, disabled, or deleted. | 系统维护的 EngineCapabilityBinding 不允许创建、修改、禁用或删除。 |
| ErrAIAppRequiredEngineBindingFailed | 130428 | 200 | The required system capability binding could not be written atomically while creating the EngineInstance; the instance was not created. | 创建 EngineInstance 时未能原子写入系统必需能力绑定，实例未创建。 |
| ErrAIAppEngineInstanceNameDuplicated | 130429 | 200 | An ApplicationEngineInstance with the same name already exists. | 已存在同名的 ApplicationEngineInstance。 |
| ErrAIAppTemplateSourceInvalid | 130620 | 200 | The ApplicationTemplate union capability source is invalid. | ApplicationTemplate 的联合能力来源不合法。 |
| ErrAIAppApplicationVersionImmutable | 130621 | 200 | A published ApplicationTemplateVersion or ApplicationVersion is immutable. | 已发布的 ApplicationTemplateVersion 或 ApplicationVersion 不可原地修改。 |
| ErrAIAppRuntimeFormNoValidVariant | 130622 | 200 | No executable CapabilityVariant exists under the current constraints. | 当前约束下没有可执行的 CapabilityVariant。 |
| ErrAIAppApplicationInputInvalid | 130623 | 200 | ApplicationRun input does not conform to RuntimeFormSchema. | ApplicationRun 输入不符合 RuntimeFormSchema。 |
| ErrAIAppTemplateNotFound | 130624 | 200 | The ApplicationTemplate does not exist or is not visible. | ApplicationTemplate 不存在或不可见。 |
| ErrAIAppTemplateVersionNotFound | 130625 | 200 | The ApplicationTemplateVersion does not exist. | ApplicationTemplateVersion 不存在。 |
| ErrAIAppApplicationNotFound | 130626 | 200 | The Application does not exist or is not visible. | Application 不存在或不可见。 |
| ErrAIAppApplicationVersionNotFound | 130627 | 200 | The ApplicationVersion does not exist. | ApplicationVersion 不存在。 |
| ErrAIAppTemplateVersionNotPublishable | 130628 | 200 | The ApplicationTemplateVersion failed publish validation. | ApplicationTemplateVersion 未通过发布校验。 |
| ErrAIAppApplicationVersionNotPublishable | 130629 | 200 | The ApplicationVersion failed publish validation. | ApplicationVersion 未通过发布校验。 |
| ErrAIAppApplicationSemanticVersionDuplicated | 130630 | 200 | The Application already has the same semantic version. | 同一 Application 已存在相同语义版本。 |
| ErrAIAppResourceVersionConflict | 130631 | 200 | The resource version changed; refresh and retry. | 资源版本已变化，请刷新后重试。 |
| ErrAIAppApplicationRunCreateFailed | 130820 | 200 | The ApplicationRun could not be created. | ApplicationRun 创建失败。 |
| ErrAIAppTaskProjectionStale | 130821 | 200 | The AtomicTask status projection version is stale. | AtomicTask 状态投影版本过旧。 |
| ErrAIAppProviderRuntimeCapabilityMismatch | 130822 | 200 | The provider rejected a capability combination declared by ProviderCapability. | 外部平台拒绝了当前 ProviderCapability 声明的能力组合。 |
| ErrAIAppTaskRunCreateFailed | 130823 | 200 | AtomicTask creation failed; the ApplicationRun snapshot was retained. | AtomicTask 创建失败，ApplicationRun 快照已保留。 |
| ErrAIAppArtifactRegistrationFailed | 130824 | 200 | Artifact registration as a UserAsset failed. | Artifact 登记 UserAsset 失败。 |
| ErrAIAppApplicationRunNotFound | 130825 | 200 | The ApplicationRun does not exist or is not visible to the current user. | ApplicationRun 不存在或当前用户不可见。 |
| ErrAIAppAtomicTaskCreateFailed | 130826 | 200 | AtomicTask creation failed; the ApplicationRun snapshot is retained and may be retried with the same idempotency key. | AtomicTask 创建暂时失败，ApplicationRun 快照已保留，可使用相同幂等键重试。 |
| ErrAIAppAtomicTaskProjectionStale | 130827 | 200 | The AtomicTask projection event is older than the current ApplicationRun projection. | AtomicTask 投影事件版本早于当前 ApplicationRun 投影。 |
| ErrAIAppArtifactRegistrationAtomicTaskUnchanged | 130828 | 200 | The Artifact could not be registered as a UserAsset; the AtomicTask terminal state is unchanged. | Artifact 未能登记为 UserAsset，AtomicTask 终态不受影响。 |
| ErrAIAppArtifactProcessingFailed | 130829 | 200 | Artifact transfer or processing failed. | Artifact 传输或处理失败。 |
| ErrAIAppArtifactStateInvalid | 130830 | 200 | The Artifact processing or registration state does not allow this transition. | Artifact 当前处理或登记状态不允许该转换。 |
| ErrAIAppProviderResponseInvalid | 130831 | 200 | The external provider response does not conform to the current ProviderCapability output contract. | 外部平台响应不符合当前 ProviderCapability 输出契约。 |
| ErrAIAppPermissionDenied | 131020 | 200 | The current user lacks the required application-platform permission. | 当前用户缺少所需的应用平台权限。 |
| ErrAIAppComfyUIWorkflowFileInvalid | 131220 | 200 | The API Workflow file is missing, invalid JSON, or not a valid ComfyUI API Workflow structure. | API Workflow 文件缺失、不是合法 JSON 或不符合 ComfyUI API Workflow 基础结构。 |
| ErrAIAppComfyUIEngineTypeInvalid | 131221 | 200 | The selected source or target EngineInstance is not a ComfyUI engine. | 指定的来源或目标 EngineInstance 不是 ComfyUI 类型。 |
| ErrAIAppComfyUIObjectInfoUnavailable | 131222 | 200 | The selected ComfyUI EngineInstance has no usable current object_info, or the catalog is older than 48 hours. | 指定 ComfyUI EngineInstance 不存在可用的当前 object_info，或目录已超过 48 小时。 |
| ErrAIAppComfyUIWorkflowReferenceInvalid | 131223 | 200 | The workflow contains an invalid node, input connection, or output index reference. | 工作流节点、输入连接或输出索引引用无效。 |
| ErrAIAppComfyUIWorkflowIncompatible | 131224 | 200 | The workflow nodes, parameters, or runtime dependencies are incompatible with the target ComfyUI engine. | 工作流节点、参数或运行依赖与目标 ComfyUI 实例不兼容。 |
| ErrAIAppComfyUIWorkflowNotFound | 131225 | 200 | The ComfyUI workflow does not exist or is not visible to the current user. | ComfyUI 工作流不存在或当前用户不可见。 |
| ErrAIAppComfyUIWorkflowArchived | 131226 | 200 | An archived workflow cannot be validated or converted into a template. | 已归档工作流不能创建兼容性校验或转换为模板。 |
| ErrAIAppComfyUIValidationNotFound | 131227 | 200 | The compatibility validation does not exist, is not visible, or does not belong to the selected workflow. | 兼容性校验记录不存在、不可见或不属于指定工作流。 |
| ErrAIAppComfyUIValidationNotCompatible | 131228 | 200 | The selected validation is not compatible and cannot be used for template conversion. | 所选校验结果不是 compatible，不能用于模板转换。 |
| ErrAIAppComfyUITemplateContractInvalid | 131229 | 200 | The template inputs, fixed parameters, mappings, output extraction, or engine restrictions are incomplete. | 模板输入、固定参数、转换规则、输出提取或 Engine 约束不完整。 |
| ErrAIAppComfyUIWorkflowAlreadyConverted | 131230 | 200 | The workflow has already been converted into an application template and cannot be converted again. | 工作流已转换为应用模板，不能再次转换。 |
| ErrAIAppComfyUIConversionIdempotencyConflict | 131231 | 200 | The conversion idempotency key is already used by another workflow owned by the same user. | 转换幂等键已被同一所有者的其他工作流使用。 |
| ErrAIAppComfyUIResourceVersionConflict | 131232 | 200 | The ComfyUI workflow resource version has changed; refresh and retry. | ComfyUI 工作流资源版本已变化，请刷新后重试。 |
| ErrAIAppComfyUIWorkflowAccessDenied | 131233 | 200 | The current user is not allowed to access or administer this ComfyUI workflow. | 当前用户无权访问或代管该 ComfyUI 工作流。 |
| ErrAIAppComfyUIWorkflowSourceInvalid | 131234 | 200 | The file is not a supported ComfyUI Workflow or API Workflow JSON. | 文件不是受支持的 ComfyUI Workflow 或 API Workflow JSON。 |
| ErrAIAppComfyUIAPIConversionBlocked | 131235 | 200 | Blocking diagnostics prevent conversion to an API Workflow. | 普通 Workflow 存在阻断诊断，不能生成 API Workflow。 |
| ErrAIAppComfyUIAPINotReady | 131236 | 200 | The API Workflow is not ready for validation or testing. | API Workflow 尚未就绪，不能进行校验或试运行。 |
| ErrAIAppComfyUITestEngineUnavailable | 131237 | 200 | The selected ComfyUI instance is disabled or unhealthy. | 所选 ComfyUI 实例未启用或当前不健康。 |
| ErrAIAppComfyUITestIncompatible | 131238 | 200 | The workflow is incompatible with the selected ComfyUI instance. | 工作流与所选 ComfyUI 实例不兼容。 |
| ErrAIAppComfyUITestParameterInvalid | 131239 | 200 | A test parameter is unknown, cannot be overridden, or violates its type or range. | 试运行参数不存在、不允许覆盖或未通过类型与范围校验。 |
| ErrAIAppComfyUITestRunNotFound | 131240 | 200 | The workflow test run does not exist or is not visible. | 试运行不存在或当前用户不可见。 |
| ErrAIAppComfyUITestPreviewUnavailable | 131241 | 200 | The temporary preview is missing, was removed upstream, or cannot be read safely. | 临时预览不存在、已被上游清理或无法安全读取。 |
| ErrAIAppComfyUITestRunStateBlocked | 131242 | 200 | The workflow test run state does not allow this operation. | 当前试运行状态不允许执行该操作。 |
| ErrAIAppComfyUIObjectInfoRefreshNotAllowed | 131243 | 200 | Object-info refresh is allowed only for enabled, healthy online ComfyUI EngineInstances. | 只有已启用且健康在线的 ComfyUI EngineInstance 可以刷新 object_info。 |
| ErrAIAppComfyUIObjectInfoRefreshFailed | 131244 | 200 | A complete valid object_info could not be retrieved from the ComfyUI EngineInstance; the last successful catalog was retained. | 未能从 ComfyUI EngineInstance 获取并校验完整 object_info，已保留最后一次成功目录。 |
| ErrTaskDefinitionInvalid | 140200 | 200 | Task definition is invalid. | 任务定义不合法。 |
| ErrTaskDAGCycleDetected | 140201 | 200 | DAGTaskGroup contains a cyclic dependency. | DAGTaskGroup 存在环形依赖。 |
| ErrTaskGroupInvalid | 140202 | 200 | TaskGroup template or execution policy is invalid. | TaskGroup 模板或执行策略不合法。 |
| ErrDAGTaskGroupInvalid | 140203 | 200 | DAGTaskGroup nodes, edges, or input references are invalid. | DAGTaskGroup 节点、边或输入引用不合法。 |
| ErrTaskFunctionRefNotRegistered | 140204 | 200 | functionRef is not registered or is unavailable to the caller. | functionRef 未注册或当前调用方不可使用。 |
| ErrTaskFunctionInputInvalid | 140205 | 200 | AtomicTask arguments do not satisfy the registered functionRef input contract. | AtomicTask arguments 不符合已注册 functionRef 的精确输入合同。 |
| ErrTaskFunctionOutputInvalid | 140206 | 200 | The Task Worker result does not satisfy the pinned functionRef output contract. | Task Worker 结果不符合固定 functionRef 输出合同。 |
| ErrTaskFunctionContractUnavailable | 140207 | 200 | The function contract version or digest pinned by the AtomicTask cannot currently be loaded. | AtomicTask 固定的 function contract 版本或摘要当前不可加载。 |
| ErrTaskFunctionCapabilityUnavailable | 140208 | 200 | No Task Worker currently satisfies the capabilities required by the pinned function contract. | 当前没有满足固定函数合同所需能力的 Task Worker。 |
| ErrTaskRunNotFound | 140400 | 200 | Task run does not exist or is not visible to the current user. | 任务运行不存在或当前用户不可见。 |
| ErrTaskRunStateBlocked | 140401 | 200 | Current task run status does not allow this operation. | 任务当前状态不允许执行该操作。 |
| ErrTaskRetryPolicyInvalid | 140402 | 200 | Task retry policy is invalid. | 任务重试策略不合法。 |
| ErrTaskRunIdempotencyConflict | 140403 | 200 | The same application run and idempotency key were used with a different task run creation request. | 相同应用运行和幂等键已用于不同的任务运行创建请求。 |
| ErrAtomicTaskNotFound | 140405 | 200 | AtomicTask does not exist or is not visible to the caller. | AtomicTask 不存在或当前调用方不可见。 |
| ErrAtomicTaskStateBlocked | 140406 | 200 | Current AtomicTask status does not allow this operation. | AtomicTask 当前状态不允许该操作。 |
| ErrAtomicTaskIdempotencyConflict | 140407 | 200 | AtomicTask idempotency key was used for a different request. | AtomicTask 幂等键已用于不同请求。 |
| ErrTaskGroupNotFound | 140408 | 200 | TaskGroup does not exist or is not visible to the caller. | TaskGroup 不存在或当前调用方不可见。 |
| ErrDAGTaskGroupNotFound | 140409 | 200 | DAGTaskGroup does not exist or is not visible to the caller. | DAGTaskGroup 不存在或当前调用方不可见。 |
| ErrTaskWorkerNotAvailable | 140600 | 200 | Worker does not exist, is unavailable, or capability does not match. | Worker 不存在、不可用或能力不匹配。 |
| ErrWorkflowRuntimeUnavailable | 140601 | 200 | Workflow runtime is currently unavailable. | 工作流运行时当前不可用。 |
| ErrWorkflowRuntimeRejected | 140602 | 200 | Workflow runtime rejected the register, start, cancel, or retry request. | 工作流运行时拒绝注册、启动、取消或重试请求。 |
| ErrTaskLeaseInvalid | 140800 | 200 | ExecutionLease is invalid, expired, or does not belong to current worker. | ExecutionLease 无效、已过期或不属于当前 Worker。 |
| ErrTaskAttemptUpdateRejected | 141000 | 200 | Current task attempt is not allowed to update task result. | 当前执行尝试不允许更新任务结果。 |
| ErrTaskAttemptNotFound | 141001 | 200 | TaskAttempt does not exist or does not belong to the AtomicTask. | TaskAttempt 不存在或不属于指定 AtomicTask。 |
| ErrTaskAttemptLogUnavailable | 141002 | 200 | Runtime log history for the TaskAttempt is no longer available. | TaskAttempt 对应的运行时日志历史已不可用。 |
| ErrTaskPermissionDenied | 141200 | 200 | Current user does not have task center permission. | 当前用户缺少任务中心操作权限。 |
| ErrTaskScheduleInvalid | 141400 | 200 | TaskSchedule target, cron, timezone, or runAt is invalid. | TaskSchedule 的目标、cron、时区或 runAt 不合法。 |
| ErrTaskScheduleNotFound | 141401 | 200 | TaskSchedule does not exist or is not visible to the caller. | TaskSchedule 不存在或当前调用方不可见。 |
| ErrTaskScheduleStateBlocked | 141402 | 200 | Current TaskSchedule status does not allow this operation. | TaskSchedule 当前状态不允许该操作。 |
| ErrTaskReconcileConfigInvalid | 141403 | 200 | Reconcile config, parallelism, item limit, or timeout is invalid. | 巡检配置、并发、单轮上限或超时不合法。 |
| ErrTaskReconcileRefUnregistered | 141404 | 200 | The reconcile handler referenced by the schedule is not registered. | 计划引用的巡检器未在后端注册。 |
| ErrTaskSystemScheduleOperationRestricted | 141405 | 200 | A system schedule cannot be created, deleted, or have protected fields changed. | 系统内置计划不允许创建、删除或修改受保护字段。 |
| ErrAssetSelectorInvalid | 150200 | 200 | The unified asset selector expression is invalid. | 统一选择器表达式语法错误。 |
| ErrAssetSelectorTooComplex | 150201 | 200 | The unified asset selector exceeds complexity limits. | 统一选择器超过复杂度限制。 |
| ErrAssetSearchParseFailed | 150202 | 200 | Natural language could not be resolved into valid asset query conditions. | 自然语言无法形成合法素材查询条件。 |
| ErrAssetSearchDependencyFailed | 150203 | 200 | The asset search resolution dependency is unavailable. | 素材搜索解析依赖不可用。 |
| ErrAssetListFailed | 150204 | 200 | The asset list or filter query failed. | 素材列表或过滤查询失败。 |
| ErrAssetQueryParametersInvalid | 150205 | 200 | The asset query parameter combination is invalid. | 素材查询参数组合无效。 |
| ErrAssetLabelInvalid | 150400 | 200 | The asset label is invalid. | Label 不满足约束。 |
| ErrAssetTagInvalid | 150401 | 200 | The asset tag is invalid. | Tag 不满足约束。 |
| ErrAssetLabelLimitExceeded | 150402 | 200 | The asset label limit is exceeded. | Labels 数量超过上限。 |
| ErrAssetTagLimitExceeded | 150403 | 200 | The asset tag limit is exceeded. | Tags 数量超过上限。 |
| ErrAssetBatchLabelRequestInvalid | 150404 | 200 | The batch asset labeling request is invalid. | 批量打标请求无效。 |
| ErrAssetNotFoundOrNotWritable | 150600 | 200 | The asset does not exist, is not writable, or does not belong to the current user. | 素材不存在、不可写或不属于当前用户。 |
| ErrAssetNotFoundOrNotVisible | 150601 | 200 | The asset does not exist or is not visible to the current user. | 素材不存在或不属于当前用户。 |
| ErrAssetNameInvalid | 150602 | 200 | The asset name is invalid. | 素材名称无效。 |
| ErrAssetStateInvalid | 150603 | 200 | The asset state does not allow this operation. | 素材当前状态不允许该操作。 |
| ErrAssetVersionNotFoundOrNotVisible | 150604 | 200 | The asset version does not exist or is not visible. | 素材版本不存在或不可见。 |
| ErrAssetRepresentationNotFoundOrNotVisible | 150605 | 200 | The asset representation does not exist or is not visible. | Representation 不存在或不可见。 |
| ErrAssetContentUnavailable | 150606 | 200 | The asset content is unavailable. | 素材内容不可读取。 |
| ErrAssetPermanentDeleteBlocked | 150607 | 200 | The asset is strongly referenced and cannot be permanently deleted. | 素材仍被强引用，不能永久删除。 |
| ErrAssetResourceVersionConflict | 150608 | 200 | The asset or collection resource version conflicts. | 素材或 Collection 版本冲突。 |
| ErrAssetVersionContentInvalid | 150609 | 200 | The asset version content is invalid. | 素材版本内容无效。 |
| ErrAssetStoragePermissionDenied | 150610 | 200 | The current user is not an administrator and cannot inspect or manage Blobs and StorageBackends. | 当前用户不是管理员，不能查看或管理 Blob 与 StorageBackend。 |
| ErrAssetBlobNotFound | 150611 | 200 | The Blob does not exist. | Blob 不存在。 |
| ErrAssetStorageBackendNotFound | 150612 | 200 | The StorageBackend does not exist. | StorageBackend 不存在。 |
| ErrAssetBatchDeleteRequestInvalid | 150613 | 200 | The batch asset delete request is empty, exceeds the limit, or contains duplicate asset IDs. | 批量删除请求为空、超过上限或包含重复素材 ID。 |
| ErrAssetDeleteFailed | 150614 | 200 | The asset deletion database commit, trash query, or content cleanup did not complete. | 素材删除的数据库提交、回收站查询或内容清理未完成。 |
| ErrArtifactRegistrationInvalid | 150800 | 200 | The Artifact creation or registration request is invalid. | Artifact 创建或登记请求无效。 |
| ErrArtifactOwnerMismatch | 150801 | 200 | The Artifact owner does not match. | Artifact owner 不匹配。 |
| ErrArtifactContentUnavailable | 150802 | 200 | The Artifact content is unavailable. | Artifact 内容不可读取。 |
| ErrArtifactMediaInvalid | 150803 | 200 | The Artifact media information is invalid. | Artifact 媒体信息无效。 |
| ErrArtifactIdempotencyConflict | 150804 | 200 | The Artifact idempotency key conflicts. | Artifact 幂等键冲突。 |
| ErrArtifactStateInvalid | 150805 | 200 | The Artifact state does not allow this operation. | Artifact 当前状态不允许该操作。 |
| ErrArtifactSourceForbidden | 150806 | 200 | The Artifact content source is forbidden. | Artifact 内容来源被禁止。 |
| ErrRepresentationPlanInvalid | 151000 | 200 | The representation plan is invalid. | Representation 计划无效。 |
| ErrRepresentationWriteConflict | 151001 | 200 | The representation write conflicts. | Representation 写入冲突。 |
| ErrRepresentationSourceIrrecoverable | 151002 | 200 | The representation source is irrecoverable. | Representation 源内容不可恢复。 |
| ErrRepresentationBackfillDeferred | 151003 | 200 | The representation backfill was deferred. | Representation 补全被延后。 |
| ErrAssetUploadRequestInvalid | 151200 | 200 | The asset upload initialization request is invalid. | 上传初始化请求无效。 |
| ErrAssetUploadNotFoundOrNotVisible | 151201 | 200 | The asset upload session does not exist or is not visible. | 上传会话不存在或不可见。 |
| ErrAssetUploadStateInvalid | 151202 | 200 | The asset upload session state does not allow this operation. | 上传会话状态不允许该操作。 |
| ErrAssetUploadPartInvalid | 151203 | 200 | The asset upload part is invalid. | 上传分片无效。 |
| ErrAssetUploadChecksumMismatch | 151204 | 200 | The uploaded content SHA256 does not match. | 上传内容 SHA256 不匹配。 |
| ErrAssetUploadStorageFailed | 151205 | 200 | The asset upload storage operation failed. | 上传存储操作失败。 |
| ErrCollectionNotFoundOrNotVisible | 151400 | 200 | The Collection does not exist or is not visible. | Collection 不存在或不可见。 |
| ErrCollectionNameConflict | 151401 | 200 | The Collection name conflicts. | Collection 名称冲突。 |
| ErrCollectionHierarchyInvalid | 151402 | 200 | The Collection hierarchy is invalid. | Collection 层级无效。 |
| ErrCollectionItemInvalid | 151403 | 200 | The Collection item is invalid. | Collection 成员无效。 |
| ErrCollectionPinnedVersionInvalid | 151404 | 200 | The Collection pinned version is invalid. | Collection 固定版本无效。 |
| ErrCanvasNotFound | 160200 | 200 | Canvas does not exist or is not visible to the caller. | Canvas 不存在或当前调用方不可见。 |
| ErrCanvasRevisionConflict | 160201 | 200 | Canvas draft revision has changed; refresh before retrying. | Canvas 草稿 revision 已变化，请刷新后重试。 |
| ErrCanvasGraphInvalid | 160202 | 200 | Canvas graph nodes, edges, ports, or input bindings are invalid. | Canvas 图的节点、边、端口或输入绑定不合法。 |
| ErrCanvasCycleDetected | 160203 | 200 | Canvas graph contains a cyclic dependency. | Canvas 图存在环形依赖。 |
| ErrCanvasLimitExceeded | 160204 | 200 | Canvas node, edge, or dynamic expansion limit is exceeded. | Canvas 节点、边或动态展开上限超出允许范围。 |
| ErrCanvasNodeReferenceInvalid | 160205 | 200 | ApplicationVersion or functionRef referenced by the node is unavailable. | 节点引用的 ApplicationVersion 或 functionRef 不可用。 |
| ErrWorkflowNodeDefinitionNotFound | 160206 | 200 | The node definition version is missing, deprecated for new references, or invisible. | 节点定义版本不存在、已对新引用下线或当前调用方不可见。 |
| ErrWorkflowControllerStateInvalid | 160207 | 200 | Interactive node state violates its fixed schema, coordinate system, resource reference, or size limits. | 交互式节点状态不符合固定 schema、坐标系、资源引用或大小限制。 |
| ErrWorkflowUnsafeNodeConfiguration | 160208 | 200 | Node configuration contains unregistered script, HTTP, worker, credential, or internal runtime data. | 节点配置包含未注册的脚本、HTTP、Worker、凭证或内部运行时信息。 |
| ErrWorkflowNodeDefinitionConflict | 160209 | 200 | The same node type and definition version exists with different content. | 相同 node_type 和 definition_version 已存在且内容不同。 |
| ErrCanvasVersionNotFound | 160400 | 200 | CanvasVersion does not exist or is not visible to the caller. | CanvasVersion 不存在或当前调用方不可见。 |
| ErrCanvasPublishFailed | 160401 | 200 | CanvasVersion compilation or runtime definition registration failed. | CanvasVersion 编译或运行时定义注册失败。 |
| ErrCanvasVersionImmutable | 160402 | 200 | A published CanvasVersion cannot be modified or deleted. | 已发布 CanvasVersion 不允许修改或删除。 |
| ErrCanvasRunNotFound | 160600 | 200 | CanvasRun does not exist or is not visible to the caller. | CanvasRun 不存在或当前调用方不可见。 |
| ErrCanvasRunStateBlocked | 160601 | 200 | Current CanvasRun status does not allow this operation. | CanvasRun 当前状态不允许该操作。 |
| ErrCanvasRunIdempotencyConflict | 160602 | 200 | CanvasRun idempotency key was used for a different version or input. | CanvasRun 幂等键已用于不同的版本或输入。 |
| ErrCanvasRunScopeInvalid | 160603 | 200 | Run scope is empty, contains invalid targets, or uses an unavailable mode. | 运行范围为空、包含重复或无效目标，或使用未开放的模式。 |
| ErrCanvasRunInputClosureInvalid | 160604 | 200 | A required input outside the run scope cannot be satisfied. | 运行范围外的必需输入无法满足。 |
| ErrCanvasReuseRequiredUnavailable | 160605 | 200 | A required reusable result is unavailable. | reuse_required 没有可复用结果。 |
| ErrCanvasArtifactUnavailable | 160606 | 200 | A required Artifact is missing, invisible, or unavailable. | 必需 Artifact 不存在、不可见或未达到可用状态。 |
| ErrCanvasTaskCreationUnavailable | 160607 | 200 | Task Center is temporarily unavailable and the CanvasRun remains recoverable. | Task Center 暂时不可用；CanvasRun 已保留为可恢复状态。 |
| ErrCanvasNodeRunNotFound | 160608 | 200 | CanvasNodeRun does not exist or is not visible. | CanvasNodeRun 不存在或当前调用方不可见。 |
| ErrCanvasOutputNotReadyTimeout | 160609 | 200 | A required output did not become available within its declared timeout. | 必需输出在声明等待时间内未达到可用状态。 |
| ErrCanvasRetryTargetInvalid | 160610 | 200 | Retry intent lacks a valid target or cannot satisfy its input closure. | 重跑意图缺少目标、目标无效或无法满足输入闭包。 |
| ErrCanvasPhaseCapabilityUnsupported | 160611 | 200 | The requested Canvas capability is unavailable in this phase. | 当前阶段不支持该画布能力。 |
| ErrCanvasPermissionDenied | 160800 | 200 | Caller does not have the required workflow canvas permission. | 当前调用方缺少工作流画布操作权限。 |
| ErrCanvasResourceReferenceDenied | 160801 | 200 | Caller cannot reference the requested node, application, function, input, or output. | 当前调用方无权引用节点、应用版本、函数、输入或输出资源。 |
| ErrCanvasQuotaExceeded | 160802 | 200 | Canvas run quota is exhausted for the current project, namespace, or user. | 当前 project、namespace 或用户的画布运行配额不足。 |
| ErrUserNotFound | 110001 | 500 | Unset error message | 错误信息未设置 |
| ErrIdentityInvalidCredentials | 220200 | 200 | The username or password is invalid. | 用户名或密码错误。 |
| ErrIdentityAccountPending | 220201 | 200 | The account is pending approval. | 账号正在等待审批。 |
| ErrIdentityAccountDisabled | 220202 | 200 | The account is disabled. | 账号已禁用。 |
| ErrIdentityAccountLocked | 220203 | 200 | The account is temporarily locked. | 账号已临时锁定。 |
| ErrIdentityFirstLoginRequired | 220204 | 200 | The first-login flow must be completed. | 必须完成首次登录引导。 |
| ErrIdentityTokenExpired | 220205 | 200 | The access token has expired. | Access Token 已过期。 |
| ErrIdentityTokenRevoked | 220206 | 200 | The token or session has been revoked. | Token 或会话已撤销。 |
| ErrIdentityRefreshTokenInvalid | 220207 | 200 | The refresh token is invalid. | Refresh Token 无效。 |
| ErrIdentityRefreshTokenReused | 220208 | 200 | Refresh token reuse was detected and the session was revoked. | 检测到 Refresh Token 重用，会话已撤销。 |
| ErrIdentityPasswordPolicyFailed | 220209 | 200 | The password does not satisfy the active security policy. | 密码不符合当前安全策略。 |
| ErrIdentityOldPasswordInvalid | 220210 | 200 | The current password is invalid. | 原密码错误。 |
| ErrIdentityAccountRejected | 220213 | 200 | The latest registration application was rejected and may be resubmitted. | 最近一次注册申请已被拒绝，可以重新提交申请。 |
| ErrIdentityRegistrationStateInvalid | 220214 | 200 | The active registration mode or account application state does not allow registration. | 当前注册模式或账号申请状态不允许提交注册。 |
| ErrIdentityPasswordProtocolUnsupported | 220215 | 200 | The legacy single-stage password protocol is unsupported; use the OPAQUE two-step API. | 不支持旧的单阶段密码协议，请使用 OPAQUE 二阶段接口。 |
| ErrIdentityPasswordProtocolInvalid | 220216 | 200 | The OPAQUE message or exchange state is invalid. | OPAQUE 消息或交换状态无效。 |
| ErrIdentityPasswordExchangeExpired | 220217 | 200 | The OPAQUE exchange has expired. | OPAQUE 交换已过期。 |
| ErrIdentityPasswordExchangeReplayed | 220218 | 200 | The OPAQUE exchange has already been consumed. | OPAQUE 交换已被提交，不能重复使用。 |
| ErrIdentityUserNotVisible | 220400 | 200 | The user does not exist or is not visible to the current principal. | 用户不存在或当前主体不可见。 |
| ErrIdentityUsernameAlreadyExists | 220401 | 200 | The username already exists. | 用户名已存在。 |
| ErrIdentityEmailAlreadyExists | 220402 | 200 | The email already exists. | 邮箱已存在。 |
| ErrIdentityUserStateInvalid | 220403 | 200 | The user state does not allow this operation. | 用户当前状态不允许该操作。 |
| ErrIdentityUserDeleteBlocked | 220404 | 200 | The user still owns business resources, tasks, service accounts, or grants. | 用户仍拥有未处理的业务资源、任务、服务账号或共享关系。 |
| ErrIdentitySelfDeleteForbidden | 220405 | 200 | A user cannot delete itself. | 用户不能删除自己。 |
| ErrIdentityLastSuperAdminProtected | 220406 | 200 | The last effective SUPER_ADMIN cannot be deleted or disabled. | 不能删除或禁用最后一个有效 SUPER_ADMIN。 |
| ErrIdentityUserCreateInvalid | 220407 | 200 | The user creation input or default grant is invalid. | 用户创建参数或默认授权无效。 |
| ErrIdentityRegistrationApplicationNotVisible | 220408 | 200 | The registration application does not exist or is not visible to the current principal. | 注册申请不存在或当前主体不可见。 |
| ErrIdentityRegistrationDecisionConflict | 220409 | 200 | The registration application already has an opposite immutable decision. | 注册申请已存在不可覆盖的相反审批结果。 |
| ErrIdentityRegistrationReasonRequired | 220410 | 200 | A reason is required to reject a registration application. | 拒绝注册申请必须填写原因。 |
| ErrIdentityUserDeleteDependencyUnavailable | 220411 | 200 | A user-deletion dependency source is unavailable or incomplete, so the user cannot be deleted. | 用户删除依赖来源不可用或返回结果不完整，当前不能删除用户。 |
| ErrIdentityUserDeleteCheckStale | 220412 | 200 | The user-deletion dependency check is expired or no longer matches current source facts. | 用户删除依赖检查已过期或不再匹配当前来源事实，请重新检查。 |
| ErrIdentityRoleNotVisible | 220600 | 200 | The role does not exist or is not visible to the current principal. | 角色不存在或当前主体不可见。 |
| ErrIdentityRoleGrantInvalid | 220602 | 200 | The role grant does not exist, is expired, or is not allowed. | 角色授权不存在、已过期或不允许。 |
| ErrIdentityAuthzContextInvalid | 220607 | 200 | The authorization context is invalid. | 当前主体授权上下文无效。 |
| ErrIdentityAuthzDenied | 220606 | 200 | The current principal is not authorized for the operation. | 当前主体无权执行该操作。 |
| ErrIdentityServiceAccountNotVisible | 221000 | 200 | The service account does not exist or is not visible to the current principal. | 服务主体不存在或当前主体不可见。 |
| ErrIdentityServiceAccountAlreadyExists | 221001 | 200 | The service account code already exists. | 服务账号编码已存在。 |
| ErrIdentityServiceAccountDisabled | 221002 | 200 | The service account is disabled. | 服务账号已禁用。 |
| ErrIdentityServiceAccountCredentialInvalid | 221003 | 200 | The service account credential is invalid. | 服务账号凭据无效。 |
| ErrIdentityServiceAccountCredentialRotationFailed | 221004 | 200 | Service account credential rotation failed. | 服务账号凭据轮换失败。 |
| ErrIdentityServiceAccountStateInvalid | 221005 | 200 | The service account state does not allow this operation. | 服务账号当前状态不允许该操作。 |
| ErrIdentityServiceAccountOwnerInvalid | 221006 | 200 | The service-account owner does not exist, is not visible, or cannot own this account. | 服务账号 owner 不存在、不可见或不允许作为该账号归属。 |
| ErrIdentityServiceAccountOwnerUnavailable | 221007 | 200 | Service-account owner validation is unavailable, so credential exchange was denied. | 服务账号 owner 校验当前不可用，凭据交换已被拒绝。 |
| ErrIdentityServiceAccountCredentialNotVisible | 221008 | 200 | The service-account credential does not exist or is not visible to the current principal. | 服务账号凭据不存在或当前主体不可见。 |
| ErrIdentityPrincipalContextInvalid | 221205 | 200 | The principal context is invalid. | 当前主体上下文无效。 |
| ErrPlatformAuthConfigInvalid | 230200 | 200 | The system authentication configuration is invalid. | 平台认证配置无效。 |
| ErrPlatformAuthConfigVersionConflict | 230202 | 200 | The system authentication configuration version conflicts with the current version. | 系统认证配置版本冲突。 |
| ErrPlatformAuditLogNotVisible | 230400 | 200 | The audit record does not exist or is not visible to the current principal. | 审计记录不存在或当前主体不可见。 |
| ErrPlatformAuditQueryInvalid | 230401 | 200 | The platform audit query is invalid. | 平台审计查询参数无效。 |
| ErrPlatformOverviewUnavailable | 230601 | 200 | The platform overview is temporarily unavailable. | 平台系统概览暂不可用。 |
| ErrPlatformAuditRecordInvalid | 230402 | 200 | The audit record is invalid or contains prohibited fields. | 审计记录无效或包含禁止字段。 |
| ErrPlatformAuditWriteUnavailable | 230403 | 200 | The platform audit boundary is unavailable and the controlled operation was not executed. | 平台审计边界不可用，受控操作未执行。 |
| ErrPlatformAuditIdempotencyConflict | 230404 | 200 | The audit idempotency key has already been used for different content. | 审计幂等键已用于不同内容。 |
| ErrAgentNotVisible | 200200 | 200 | The Agent does not exist or is not visible to the current principal. | Agent 不存在或当前主体不可见。 |
| ErrAgentProfileInvalid | 200201 | 200 | The AgentProfile is missing, disabled, or unavailable at the requested revision. | AgentProfile 不存在、禁用或版本不可用。 |
| ErrAgentStateInvalid | 200202 | 200 | The Agent is not in a state that permits this operation. | Agent 当前状态不允许该操作。 |
| ErrAgentInitializationFailed | 200203 | 200 | Agent initialization failed. | Agent 初始化失败。 |
| ErrAgentAccessDenied | 201000 | 200 | The current principal is not authorized to access the Agent resource. | 无权访问 Agent 资源。 |
| ErrAgentSessionNotVisible | 200400 | 200 | The Session does not exist or is not visible to the current principal. | Session 不存在或当前主体不可见。 |
| ErrAgentSessionClosed | 200401 | 200 | The Session is closed or archived and cannot accept new messages. | Session 已关闭或归档，不能接收新消息。 |
| ErrAgentInvocationConflict | 200402 | 200 | The Session has a conflicting active Invocation. | 当前 Session 存在不允许并发的 Invocation。 |
| ErrAgentInvocationTaskUnavailable | 200403 | 200 | The AtomicTask bound to the Invocation is unavailable. | Invocation 关联的 AtomicTask 不可用。 |
| ErrAgentModelBindingInvalid | 200204 | 200 | The Agent model binding is invalid or unsupported for the requested purpose. | Agent 模型绑定无效或不支持当前用途。 |
| ErrAgentMemoryInvalid | 200205 | 200 | The memory scope, type, or source reference is invalid. | Memory scope、类型或来源引用无效。 |
| ErrAgentRuntimeNotVisible | 200800 | 200 | The AgentRuntime does not exist or is not visible to the current principal. | AgentRuntime 不存在或当前主体不可见。 |
| ErrAgentRuntimeOperationFailed | 200801 | 200 | The AgentRuntime operation failed. | AgentRuntime 操作失败。 |
| ErrAgentRuntimeUnhealthy | 200802 | 200 | The AgentRuntime health check failed. | AgentRuntime 健康检查失败。 |
| ErrAppStudioApplicationNotVisible | 210200 | 200 | The StudioApplication does not exist or is not visible to the current principal. | StudioApplication 不存在或当前主体不可见。 |
| ErrAppStudioApplicationInvalidState | 210201 | 200 | The StudioApplication is not in a state that permits this operation. | StudioApplication 当前状态不允许该操作。 |
| ErrAppStudioSourceNotVisible | 210400 | 200 | The Studio Source does not exist or is not visible to the current principal. | Studio Source 不存在或当前主体不可见。 |
| ErrAppStudioSourceRevisionConflict | 210401 | 200 | The Source current Revision conflicts with base_revision. | Source 当前 Revision 与 base_revision 冲突。 |
| ErrAppStudioSourceChangeRejected | 210402 | 200 | The ChangeSet failed path, dependency, or security validation. | ChangeSet 未通过路径、依赖或安全校验。 |
| ErrAppStudioSourceAccessInvalid | 210403 | 200 | The Source access credential is invalid, scope-mismatched, or does not allow the operation. | Source 访问凭证无效、范围不匹配或不允许当前操作。 |
| ErrAppStudioSnapshotNotVisible | 210600 | 200 | The Source Snapshot does not exist or is not visible to the current principal. | Source Snapshot 不存在或当前主体不可见。 |
| ErrAppStudioSnapshotInvalid | 210601 | 200 | The Source Snapshot is incomplete or its digest validation failed. | Source Snapshot 未完成或 digest 校验失败。 |
| ErrAppStudioBuildNotVisible | 210800 | 200 | The StudioBuild does not exist or is not visible to the current principal. | StudioBuild 不存在或当前主体不可见。 |
| ErrAppStudioBuildFailed | 210801 | 200 | The Build execution or Artifact delivery failed. | Build 执行或 Artifact 交付失败。 |
| ErrAppStudioBuildCanceled | 210802 | 200 | The Build was canceled. | Build 已取消。 |
| ErrAppStudioBuildArtifactNotReady | 210803 | 200 | The Build task completed, but the Artifact is not READY or registration failed. | Build 任务已完成，但 Artifact 尚未 READY 或登记失败。 |
| ErrAppStudioBuildArtifactDigestMismatch | 210804 | 200 | The Asset Library Artifact digest does not match the Build output. | Asset Library Artifact digest 与 Build 输出不一致。 |
| ErrAppStudioReleaseInvalid | 211000 | 200 | The Release Build, Artifact, configuration, or environment is invalid. | Release 的 Build、Artifact、配置或环境无效。 |
| ErrAppStudioRuntimeDeployFailed | 211001 | 200 | StudioRuntimeInstance deployment or health checking failed. | StudioRuntimeInstance 部署或健康检查失败。 |
| ErrAppStudioReleaseImmutable | 211002 | 200 | An immutable Release cannot be modified. | 不允许修改不可变 Release。 |
| ErrAppStudioAccessDenied | 211200 | 200 | The current principal is not authorized to access the AppStudio resource. | 无权访问 AppStudio 资源。 |
| ErrInfraRequestInvalid | 240200 | 200 | The Infra request lacks a trusted identity, ownership, or runtime parameter. | Infra 请求缺少受控身份、归属或运行参数。 |
| ErrInfraRuntimeProfileNotFound | 240201 | 200 | The RuntimeProfile does not exist or its revision is unavailable. | RuntimeProfile 不存在或版本不可用。 |
| ErrInfraUnsupportedRuntimeMode | 240202 | 200 | The requested runtime mode is not supported in the first phase. | 当前第一阶段不支持该运行模式。 |
| ErrInfraIdempotencyConflict | 240203 | 200 | The request fingerprint differs for the same requestingService/requestId scope. | 相同 requestingService/requestId 对应的请求摘要不一致。 |
| ErrInfraNoEligibleNode | 240400 | 200 | No node satisfies the requested resource and status requirements. | 没有满足资源和状态要求的节点。 |
| ErrInfraResourceInsufficient | 240401 | 200 | Available CPU, memory, disk, or GPU resources are insufficient. | 可用 CPU、内存、磁盘或 GPU 资源不足。 |
| ErrInfraRuntimeNotFound | 240600 | 200 | The InfraRuntime does not exist or is not visible. | InfraRuntime 不存在或不可见。 |
| ErrInfraRuntimeOperationFailed | 240601 | 200 | The Runtime Provider operation failed. | Runtime Provider 操作失败。 |
| ErrInfraRuntimeStateConflict | 240602 | 200 | The InfraRuntime state conflicts with the requested operation. | InfraRuntime 当前状态与操作不一致。 |
| ErrInfraMountNotAllowed | 240800 | 200 | The mount reference, target path, or read-only policy violates security rules. | 挂载引用、目标路径或只读策略不符合安全规则。 |
| ErrInfraSecretResolutionFailed | 240801 | 200 | SecretRef or ModelAccessSpec resolution failed. | SecretRef 或 ModelAccessSpec 解析失败。 |
| ErrInfraEndpointAllocationFailed | 240802 | 200 | Endpoint allocation or refresh failed. | Endpoint 分配或刷新失败。 |
| ErrInfraEndpointAccessDenied | 240803 | 200 | The current principal, owner, or short-lived grant cannot resolve the endpoint. | 当前主体、owner 或短期授权不允许解析该 Endpoint。 |
| ErrInfraEndpointNotReady | 240804 | 200 | The endpoint or its runtime is not ready for resolution, or the endpoint has expired or been revoked. | Endpoint 或所属 Runtime 未达到可解析状态，或 Endpoint 已过期、撤销。 |
| ErrInfraOutputCollectionFailed | 240805 | 200 | The declared output is missing, is not a regular file, escapes the output root, or could not be collected. | 声明输出缺失、不是普通文件、路径逃逸或实际字节收集失败。 |
| ErrInfraOutputContentUnavailable | 240806 | 200 | The RuntimeOutput content has not been collected, has been cleaned up, or is unavailable to the current Task Worker. | RuntimeOutput 内容尚未收集、已清理或当前 Task Worker 无法读取。 |
| ErrInfraOutputIntegrityMismatch | 240807 | 200 | The size or SHA-256 differs across the RuntimeOutput, transferred bytes, and Artifact. | RuntimeOutput、传输字节与 Artifact 的大小或 SHA-256 不一致。 |
| ErrAgentMCPBindingInvalid | 200206 | 200 | The MCP binding is invalid. | MCP Binding 无效。 |
| ErrAgentMCPBindingNameConflict | 200207 | 200 | An active MCP binding with the same name already exists for the Agent. | 当前 Agent 下存在同名的活动 MCP Binding。 |
| ErrAgentMCPBindingVersionConflict | 200208 | 200 | The MCP binding was modified; refresh it and retry. | MCP Binding 已被更新，请刷新后重试。 |
| ErrAgentMCPBindingRevisionUnavailable | 200209 | 200 | The MCP binding revision is unavailable. | MCP Binding revision 不可用。 |
| ErrGitLabServerNameConflict | 250200 | 200 | The GitLabServer name already exists. | GitLabServer 名称已存在。 |
| ErrGitLabServerNotFound | 250201 | 200 | The GitLabServer does not exist or is not visible. | GitLabServer 不存在或当前不可见。 |
| ErrGitLabServerConnectionFailed | 250202 | 200 | GitLabServer connectivity, credential, or Namespace validation failed. | GitLabServer 连接、credential 或 Namespace 检测失败。 |
| ErrGitLabServerHasProjects | 250203 | 200 | The GitLabServer still has GitLabProject projections and cannot be deleted. | GitLabServer 仍有关联 GitLabProject，不能删除。 |
| ErrGitLabProjectNotFound | 250400 | 200 | The GitLabProject does not exist or is not visible. | GitLabProject 不存在或当前不可见。 |
| ErrGitLabProjectServerNotReady | 250401 | 200 | The GitLabServer has not passed the connectivity test. | GitLabServer 尚未通过连接检测。 |
| ErrGitLabProjectRemoteFailed | 250402 | 200 | The remote GitLab Project operation failed. | GitLab 远端 Project 操作失败。 |
| ErrGitLabProjectProjectionFailed | 250403 | 200 | The local GitLabProject projection could not be persisted. | GitLabProject 本地投影写入失败。 |
| ErrGitLabPipelineInvalid | 250600 | 200 | The GitLab Pipeline task input is invalid. | GitLab Pipeline 任务输入无效。 |
| ErrGitLabPipelineCreateFailed | 250601 | 200 | The GitLab Pipeline could not be created. | GitLab Pipeline 创建失败。 |
| ErrGitLabPipelineFailed | 250602 | 200 | The GitLab Pipeline failed. | GitLab Pipeline 执行失败。 |
| ErrGitLabPipelineCanceled | 250603 | 200 | The GitLab Pipeline was canceled. | GitLab Pipeline 已取消。 |
| ErrGitLabAccessDenied | 250800 | 200 | The current principal is not authorized to access the GitLab resource. | 当前主体无权访问 GitLab 资源。 |
