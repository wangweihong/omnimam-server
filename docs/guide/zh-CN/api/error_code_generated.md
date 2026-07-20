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
| ErrBind | 100003 | 400 | Error occurred while binding the request body to the struct. | 解析结构体出错 |
| ErrValidation | 100004 | 400 | Validation failed. | 参数校验失败 |
| ErrTokenInvalid | 100005 | 401 | Token invalid. | 令牌无效 |
| ErrPageNotFound | 100006 | 404 | Page not found. | 请求路由不存在 |
| ErrOperationBatchExecute | 100007 | 200 | Operation batch execute. | 批量执行操作 |
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
| ErrAIAppProviderCapabilityDirectoryUnreadable | 130220 | 200 | The ProviderCapability directory is missing or unreadable; the registry is degraded. | ProviderCapability 目录不存在或不可读取，能力注册表已降级。 |
| ErrAIAppProviderCapabilityYAMLInvalid | 130221 | 200 | The ProviderCapability file is not valid YAML. | ProviderCapability 文件不是合法 YAML。 |
| ErrAIAppProviderCapabilitySchemaInvalid | 130222 | 200 | ProviderCapability fields do not conform to the current schema. | ProviderCapability 字段不符合当前 Schema。 |
| ErrAIAppProviderCapabilitySchemaVersionUnsupported | 130223 | 200 | The ProviderCapability schema_version is not supported. | ProviderCapability schema_version 不受支持。 |
| ErrAIAppProviderCapabilityIDDuplicated | 130224 | 200 | Multiple ProviderCapability files declare the same ID; all conflicting entries are unavailable. | 多个 ProviderCapability 文件声明了相同 ID，所有冲突项均不可用。 |
| ErrAIAppProviderCapabilityEngineTypeMissing | 130225 | 200 | The ApplicationEngineType referenced by ProviderCapability is not registered. | ProviderCapability 引用的 ApplicationEngineType 未注册。 |
| ErrAIAppProviderCapabilityAdapterMissing | 130226 | 200 | The EngineAdapter required by ProviderCapability is not registered. | ProviderCapability 对应的 EngineAdapter 未注册。 |
| ErrAIAppProviderCapabilityExecutorMissing | 130227 | 200 | At least one ProviderCapability operation has no registered OperationExecutor. | ProviderCapability 中至少一个 Operation 缺少 OperationExecutor。 |
| ErrAIAppProviderCapabilityVariantInvalid | 130228 | 200 | ProviderCapability models, operations, variants, or parameter constraints are inconsistent. | ProviderCapability 的模型、Operation、Variant 或参数约束不一致。 |
| ErrAIAppProviderCapabilityUnavailable | 130229 | 200 | The ProviderCapability is currently unavailable or disabled. | ProviderCapability 当前不可用或已禁用。 |
| ErrAIAppProviderCapabilityNotFound | 130230 | 200 | The ProviderCapability does not exist. | ProviderCapability 不存在。 |
| ErrAIAppEngineInstanceNotFound | 130420 | 200 | The ApplicationEngineInstance does not exist. | ApplicationEngineInstance 不存在。 |
| ErrAIAppEngineAuthConfigInvalid | 130421 | 200 | The EngineInstance authentication configuration does not satisfy its EngineType. | EngineInstance 的鉴权配置不符合 EngineType 要求。 |
| ErrAIAppEngineBindingIncompatible | 130422 | 200 | The EngineCapabilityBinding is incompatible with its EngineType or ProviderCapability. | EngineCapabilityBinding 与 EngineType 或 ProviderCapability 不兼容。 |
| ErrAIAppEngineBindingRestrictionExpands | 130423 | 200 | EngineCapabilityBinding restrictions expand ProviderCapability capabilities. | EngineCapabilityBinding restrictions 扩张了 ProviderCapability 能力。 |
| ErrAIAppEngineUnavailable | 130424 | 200 | No enabled and healthy EngineInstance is currently available. | 当前没有可用且健康的 EngineInstance。 |
| ErrAIAppEngineReferenceBlocked | 130425 | 200 | The EngineInstance has historical ApplicationRun references and cannot be deleted. | EngineInstance 存在历史 ApplicationRun 引用，禁止删除。 |
| ErrAIAppEngineBindingNotFound | 130426 | 200 | The EngineCapabilityBinding does not exist. | EngineCapabilityBinding 不存在。 |
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
| ErrCanvasVersionNotFound | 160400 | 200 | CanvasVersion does not exist or is not visible to the caller. | CanvasVersion 不存在或当前调用方不可见。 |
| ErrCanvasPublishFailed | 160401 | 200 | CanvasVersion compilation or runtime definition registration failed. | CanvasVersion 编译或运行时定义注册失败。 |
| ErrCanvasRunNotFound | 160600 | 200 | CanvasRun does not exist or is not visible to the caller. | CanvasRun 不存在或当前调用方不可见。 |
| ErrCanvasRunStateBlocked | 160601 | 200 | Current CanvasRun status does not allow this operation. | CanvasRun 当前状态不允许该操作。 |
| ErrCanvasRunIdempotencyConflict | 160602 | 200 | CanvasRun idempotency key was used for a different version or input. | CanvasRun 幂等键已用于不同的版本或输入。 |
| ErrCanvasPermissionDenied | 160800 | 200 | Caller does not have the required workflow canvas permission. | 当前调用方缺少工作流画布操作权限。 |
| ErrUserNotFound | 110001 | 500 | Unset error message | 错误信息未设置 |
