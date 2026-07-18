package code

//go:generate codegen -type=int
//go:generate codegen -type=int -doc -output ../../../docs/guide/zh-CN/api/error_code_generated.md

// Common: basic errors.
// Code must start with 1xxxxx.
const (

	// @HTTP 200
	// @CN 请求成功
	// @EN Success.
	ErrSuccess int = iota + 100001

	// @HTTP 500
	// @CN 服务器出错
	// @EN Internal server error.
	ErrUnknown

	// @HTTP 400
	// @CN 解析结构体出错
	// @EN Error occurred while binding the request body to the struct.
	ErrBind

	// @HTTP 400
	// @CN  参数校验失败
	// @EN  Validation failed.
	ErrValidation

	// @HTTP 401
	// @CN  令牌无效
	// @EN  Token invalid.
	ErrTokenInvalid

	// @HTTP 404
	// @CN  请求路由不存在
	// @EN  Page not found.
	ErrPageNotFound

	// ErrOperationBatchExecute 表明当前操作为批量操作,需解析结构确认批量结果
	// @HTTP 200
	// @CN  批量执行操作
	// @EN  Operation batch execute.
	ErrOperationBatchExecute
)

// common: database errors.
const (
	// @HTTP 500
	// @CN  数据库出错
	// @EN  Database error.
	ErrDatabase int = iota + 100101
)

// common: authorization and authentication errors.
const (
	// @HTTP 401
	// @CN  用户密码加密失败
	// @EN  Error occurred while encrypting the user password.
	ErrEncrypt int = iota + 100201

	// @HTTP 401
	// @CN  签名无效
	// @EN  Signature is invalid.
	ErrSignatureInvalid

	// @HTTP 401
	// @CN  令牌
	// @EN  Token expired.
	ErrExpired

	// @HTTP 401
	// @CN  无效的请求授权头部
	// @EN  Invalid authorization header.
	ErrInvalidAuthHeader

	// @HTTP 401
	// @CN  请求授权头部为空
	// @EN  The `Authorization` header was empty.
	ErrMissingHeader

	// @HTTP 401
	// @CN  密码验证失败
	// @EN  Password was incorrect.
	ErrPasswordIncorrect

	// @HTTP 403
	// @CN  请求无权限执行
	// @EN  Permission denied.
	ErrPermissionDenied
)

// common: encode/decode errors.
const (
	// @HTTP 500
	// @CN  数据编码出错
	// @EN  Encoding failed due to an error with the data.
	ErrEncodingFailed int = iota + 100301

	// @HTTP 500
	// @CN  数据解码出错
	// @EN  Decoding failed due to an error with the data.
	ErrDecodingFailed

	// @HTTP 500
	// @CN  数据非有效JSON结构
	// @EN   Data is not valid JSON.
	ErrInvalidJSON

	// @HTTP 500
	// @CN  JSON数据编码失败
	// @EN  JSON data could not be encoded.
	ErrEncodingJSON

	// @HTTP 500
	// @CN  JSON数据解码失败
	// @EN  JSON data could not be decoded.
	ErrDecodingJSON

	// @HTTP 500
	// @CN  数据非有效YAML结构
	// @EN  Data is not valid Yaml.
	ErrInvalidYaml

	// @HTTP 500
	// @CN  YAML数据编码失败
	// @EN  Yaml data could not be encoded.
	ErrEncodingYaml

	// @HTTP 500
	// @CN  YAML数据编码失败
	// @EN  Yaml data could not be decoded.
	ErrDecodingYaml
)

// common: Http  server error.
const ()

// common: Http  client error.
const (
	// @HTTP 500
	// @CN  HTTP请求失败
	// @EN  HTTP request error.
	ErrHTTPError int = iota + 100501

	// @HTTP 500
	// @CN  解析HTTP服务返回数据失败
	// @EN  Decode data from http response error.
	ErrHTTPResponseDataParseError

	// @HTTP 500
	// @CN  生成HTTP客户端失败
	// @EN  Generate HTTP client error.
	ErrHTTPClientGenerateError

	// @HTTP 401
	// @CN  模型服务认证失败
	// @EN  Provider authentication failed.
	ErrProviderUnauthorized

	// @HTTP 400
	// @CN  模型服务协议不支持
	// @EN  Provider protocol is unsupported.
	ErrProviderUnsupported

	// @HTTP 500
	// @CN  模型服务不可用
	// @EN  Provider service is unavailable.
	ErrProviderUnavailable

	// @HTTP 500
	// @CN  模型服务响应解析失败
	// @EN  Provider response parse failed.
	ErrProviderResponseParseError
)

// common: gRPC  server error.
const ()

// common: gRPC  client error.
const (
	// @HTTP 500
	// @CN  生成gRPC客户端失败
	// @EN  Generate gRPC client error.
	ErrGRPCClientGenerateError int = iota + 100701

	// @HTTP 500
	// @CN  gRPC客户端证书错误
	// @EN   Validate gRPC client certificate error.
	ErrGRPCClientCertificateError

	// @HTTP 500
	// @CN  gRPC客户端连接失败
	// @EN   Dial to gRPC server error.
	ErrGRPCClientDialError

	// @HTTP 500
	// @CN  gRPC客户端访问服务接口失败
	// @EN   Invoke gRPC server service function error.
	ErrGRPCClientInvokeServiceError

	// @HTTP 500
	// @CN  解析gRPC服务返回数据失败
	// @EN  Decode data from gRPC service error.
	ErrGRPCResponseDataParseError
)

// ai chat: feature scoped business errors.
const (
	// @HTTP 200
	// @CN 话题不存在或当前用户不可见。
	// @EN Topic does not exist or is not visible to current user.
	ErrAIChatTopicNotFound int = 110200

	// @HTTP 200
	// @CN 消息不存在或当前用户不可见。
	// @EN Message does not exist or is not visible to current user.
	ErrAIChatMessageNotFound int = 110400

	// @HTTP 200
	// @CN 输入为空且没有图片附件。
	// @EN Message input is empty and has no image attachment.
	ErrAIChatMessageEmpty int = 110401

	// @HTTP 200
	// @CN 助手不存在或当前用户不可见。
	// @EN Assistant does not exist or is not visible to current user.
	ErrAIChatAssistantNotFound int = 110600

	// @HTTP 200
	// @CN 系统助手不可删除或修改受保护字段。
	// @EN System assistant cannot be deleted or protected fields cannot be changed.
	ErrAIChatAssistantSystemProtected int = 110601

	// @HTTP 200
	// @CN 快捷短语不合法或助手级短语缺少助手。
	// @EN Quick phrase is invalid or assistant-scoped phrase misses assistant.
	ErrAIChatQuickPhraseInvalid int = 110800

	// @HTTP 200
	// @CN 同一话题已有运行中的生成。
	// @EN The topic already has an active generation.
	ErrAIChatConcurrentGeneration int = 111000

	// @HTTP 200
	// @CN 生成运行不存在或当前用户不可见。
	// @EN Generation run does not exist or is not visible to current user.
	ErrAIChatGenerationNotFound int = 111001

	// @HTTP 200
	// @CN 当前用户未配置默认翻译模型。
	// @EN Current user has not configured a default translation model.
	ErrAIChatTranslationModelMissing int = 111200

	// @HTTP 200
	// @CN 默认翻译模型不可用。
	// @EN Default translation model is unavailable.
	ErrAIChatTranslationModelUnhealthy int = 111201

	// @HTTP 200
	// @CN 当前用户不能访问该聊天资源。
	// @EN Current user cannot access this chat resource.
	ErrAIChatAccessDenied int = 111400
)

// model-management: feature scoped business errors.
const (
	// @HTTP 200
	// @CN 模型提供商不存在或当前用户不可见。
	// @EN Model provider does not exist or is not visible to current user.
	ErrModelProviderNotFound int = 120200

	// @HTTP 200
	// @CN 当前用户范围内提供商名称重复。
	// @EN Provider name is duplicated in current user scope.
	ErrModelProviderNameDuplicated int = 120201

	// @HTTP 200
	// @CN 模型提供商连接检测失败。
	// @EN Model provider connection test failed.
	ErrModelProviderTestFailed int = 120202

	// @HTTP 200
	// @CN 模型不存在或当前用户不可见。
	// @EN Provider model does not exist or is not visible to current user.
	ErrProviderModelNotFound int = 120400

	// @HTTP 200
	// @CN 模型标识不能为空或不合法。
	// @EN Model identifier is empty or invalid.
	ErrProviderModelIdentifierInvalid int = 120401

	// @HTTP 200
	// @CN 同一提供商下模型标识或显示名重复。
	// @EN Model identifier or display name is duplicated under the same provider.
	ErrProviderModelDuplicated int = 120402

	// @HTTP 200
	// @CN 当前用户未配置指定用途的默认模型。
	// @EN Current user has not configured a default model for the requested usage.
	ErrDefaultModelMissing int = 120600

	// @HTTP 200
	// @CN 默认模型候选不可用。
	// @EN Default model candidate is not available.
	ErrDefaultModelInvalid int = 120601

	// @HTTP 200
	// @CN 模型健康检测失败。
	// @EN Model health check failed.
	ErrModelHealthCheckFailed int = 120800

	// @HTTP 200
	// @CN 当前用户无权访问该模型配置。
	// @EN Current user is not allowed to access this model configuration.
	ErrModelAccessDenied int = 121000
)

const (
	ErrAIChatModelNotFound              = ErrProviderModelNotFound
	ErrAIChatModelDisabled              = ErrDefaultModelInvalid
	ErrAIChatModelCapabilityUnsupported = ErrDefaultModelInvalid
	ErrAIChatModelUnavailable           = ErrAIChatTranslationModelUnhealthy
	ErrAIChatGenerationConflict         = ErrAIChatConcurrentGeneration
	ErrAIChatBranchSourceMissing        = ErrAIChatTopicNotFound
	ErrAIChatSystemAssistantProtected   = ErrAIChatAssistantSystemProtected
	ErrAIChatDuplicateAssistantName     = ErrAIChatQuickPhraseInvalid
	ErrAIChatTranslationModelNotFound   = ErrAIChatTranslationModelMissing
	ErrAIChatTranslationModelDisabled   = ErrAIChatTranslationModelUnhealthy
	ErrAIChatAttachmentUnsupported      = ErrAIChatMessageEmpty
	ErrAIChatAttachmentTooLarge         = ErrAIChatMessageEmpty
	ErrAIChatExportFailed               = ErrAIChatAccessDenied
)

// application-platform: SSOT scoped business errors.
const (
	// @HTTP 200
	// @CN ProviderCapability 目录不存在或不可读取，能力注册表已降级。
	// @EN The ProviderCapability directory is missing or unreadable; the registry is degraded.
	ErrAIAppProviderCapabilityDirectoryUnreadable int = 130220
	// @HTTP 200
	// @CN ProviderCapability 文件不是合法 YAML。
	// @EN The ProviderCapability file is not valid YAML.
	ErrAIAppProviderCapabilityYAMLInvalid int = 130221
	// @HTTP 200
	// @CN ProviderCapability 字段不符合当前 Schema。
	// @EN ProviderCapability fields do not conform to the current schema.
	ErrAIAppProviderCapabilitySchemaInvalid int = 130222
	// @HTTP 200
	// @CN ProviderCapability schema_version 不受支持。
	// @EN The ProviderCapability schema_version is not supported.
	ErrAIAppProviderCapabilitySchemaVersionUnsupported int = 130223
	// @HTTP 200
	// @CN 多个 ProviderCapability 文件声明了相同 ID，所有冲突项均不可用。
	// @EN Multiple ProviderCapability files declare the same ID; all conflicting entries are unavailable.
	ErrAIAppProviderCapabilityIDDuplicated int = 130224
	// @HTTP 200
	// @CN ProviderCapability 引用的 ApplicationEngineType 未注册。
	// @EN The ApplicationEngineType referenced by ProviderCapability is not registered.
	ErrAIAppProviderCapabilityEngineTypeMissing int = 130225
	// @HTTP 200
	// @CN ProviderCapability 对应的 EngineAdapter 未注册。
	// @EN The EngineAdapter required by ProviderCapability is not registered.
	ErrAIAppProviderCapabilityAdapterMissing int = 130226
	// @HTTP 200
	// @CN ProviderCapability 中至少一个 Operation 缺少 OperationExecutor。
	// @EN At least one ProviderCapability operation has no registered OperationExecutor.
	ErrAIAppProviderCapabilityExecutorMissing int = 130227
	// @HTTP 200
	// @CN ProviderCapability 的模型、Operation、Variant 或参数约束不一致。
	// @EN ProviderCapability models, operations, variants, or parameter constraints are inconsistent.
	ErrAIAppProviderCapabilityVariantInvalid int = 130228
	// @HTTP 200
	// @CN ProviderCapability 当前不可用或已禁用。
	// @EN The ProviderCapability is currently unavailable or disabled.
	ErrAIAppProviderCapabilityUnavailable int = 130229
	// @HTTP 200
	// @CN ProviderCapability 不存在。
	// @EN The ProviderCapability does not exist.
	ErrAIAppProviderCapabilityNotFound int = 130230
	// @HTTP 200
	// @CN ApplicationEngineInstance 不存在。
	// @EN The ApplicationEngineInstance does not exist.
	ErrAIAppEngineInstanceNotFound int = 130420
	// @HTTP 200
	// @CN EngineInstance 的鉴权配置不符合 EngineType 要求。
	// @EN The EngineInstance authentication configuration does not satisfy its EngineType.
	ErrAIAppEngineAuthConfigInvalid int = 130421
	// @HTTP 200
	// @CN EngineCapabilityBinding 与 EngineType 或 ProviderCapability 不兼容。
	// @EN The EngineCapabilityBinding is incompatible with its EngineType or ProviderCapability.
	ErrAIAppEngineBindingIncompatible int = 130422
	// @HTTP 200
	// @CN EngineCapabilityBinding restrictions 扩张了 ProviderCapability 能力。
	// @EN EngineCapabilityBinding restrictions expand ProviderCapability capabilities.
	ErrAIAppEngineBindingRestrictionExpands int = 130423
	// @HTTP 200
	// @CN 当前没有可用且健康的 EngineInstance。
	// @EN No enabled and healthy EngineInstance is currently available.
	ErrAIAppEngineUnavailable int = 130424
	// @HTTP 200
	// @CN EngineInstance 存在历史 ApplicationRun 引用，禁止删除。
	// @EN The EngineInstance has historical ApplicationRun references and cannot be deleted.
	ErrAIAppEngineReferenceBlocked int = 130425
	// @HTTP 200
	// @CN EngineCapabilityBinding 不存在。
	// @EN The EngineCapabilityBinding does not exist.
	ErrAIAppEngineBindingNotFound int = 130426
	// @HTTP 200
	// @CN ApplicationTemplate 的联合能力来源不合法。
	// @EN The ApplicationTemplate union capability source is invalid.
	ErrAIAppTemplateSourceInvalid int = 130620
	// @HTTP 200
	// @CN 已发布的 ApplicationTemplateVersion 或 ApplicationVersion 不可原地修改。
	// @EN A published ApplicationTemplateVersion or ApplicationVersion is immutable.
	ErrAIAppApplicationVersionImmutable int = 130621
	// @HTTP 200
	// @CN 当前约束下没有可执行的 CapabilityVariant。
	// @EN No executable CapabilityVariant exists under the current constraints.
	ErrAIAppRuntimeFormNoValidVariant int = 130622
	// @HTTP 200
	// @CN ApplicationRun 输入不符合 RuntimeFormSchema。
	// @EN ApplicationRun input does not conform to RuntimeFormSchema.
	ErrAIAppApplicationInputInvalid int = 130623
	// @HTTP 200
	// @CN ApplicationTemplate 不存在或不可见。
	// @EN The ApplicationTemplate does not exist or is not visible.
	ErrAIAppTemplateNotFound int = 130624
	// @HTTP 200
	// @CN ApplicationTemplateVersion 不存在。
	// @EN The ApplicationTemplateVersion does not exist.
	ErrAIAppTemplateVersionNotFound int = 130625
	// @HTTP 200
	// @CN Application 不存在或不可见。
	// @EN The Application does not exist or is not visible.
	ErrAIAppApplicationNotFound int = 130626
	// @HTTP 200
	// @CN ApplicationVersion 不存在。
	// @EN The ApplicationVersion does not exist.
	ErrAIAppApplicationVersionNotFound int = 130627
	// @HTTP 200
	// @CN ApplicationTemplateVersion 未通过发布校验。
	// @EN The ApplicationTemplateVersion failed publish validation.
	ErrAIAppTemplateVersionNotPublishable int = 130628
	// @HTTP 200
	// @CN ApplicationVersion 未通过发布校验。
	// @EN The ApplicationVersion failed publish validation.
	ErrAIAppApplicationVersionNotPublishable int = 130629
	// @HTTP 200
	// @CN 同一 Application 已存在相同语义版本。
	// @EN The Application already has the same semantic version.
	ErrAIAppApplicationSemanticVersionDuplicated int = 130630
	// @HTTP 200
	// @CN 资源版本已变化，请刷新后重试。
	// @EN The resource version changed; refresh and retry.
	ErrAIAppResourceVersionConflict int = 130631
	// @HTTP 200
	// @CN ApplicationRun 创建失败。
	// @EN The ApplicationRun could not be created.
	ErrAIAppApplicationRunCreateFailed int = 130820
	// @HTTP 200
	// @CN AtomicTask 状态投影版本过旧。
	// @EN The AtomicTask status projection version is stale.
	ErrAIAppTaskProjectionStale int = 130821
	// @HTTP 200
	// @CN 外部平台拒绝了当前 ProviderCapability 声明的能力组合。
	// @EN The provider rejected a capability combination declared by ProviderCapability.
	ErrAIAppProviderRuntimeCapabilityMismatch int = 130822
	// @HTTP 200
	// @CN AtomicTask 创建失败，ApplicationRun 快照已保留。
	// @EN AtomicTask creation failed; the ApplicationRun snapshot was retained.
	ErrAIAppTaskRunCreateFailed int = 130823
	// @HTTP 200
	// @CN Artifact 登记 UserAsset 失败。
	// @EN Artifact registration as a UserAsset failed.
	ErrAIAppArtifactRegistrationFailed int = 130824
	// @HTTP 200
	// @CN ApplicationRun 不存在或当前用户不可见。
	// @EN The ApplicationRun does not exist or is not visible to the current user.
	ErrAIAppApplicationRunNotFound int = 130825
	// @HTTP 200
	// @CN AtomicTask 创建暂时失败，ApplicationRun 快照已保留，可使用相同幂等键重试。
	// @EN AtomicTask creation failed; the ApplicationRun snapshot is retained and may be retried with the same idempotency key.
	ErrAIAppAtomicTaskCreateFailed int = 130826
	// @HTTP 200
	// @CN AtomicTask 投影事件版本早于当前 ApplicationRun 投影。
	// @EN The AtomicTask projection event is older than the current ApplicationRun projection.
	ErrAIAppAtomicTaskProjectionStale int = 130827
	// @HTTP 200
	// @CN Artifact 未能登记为 UserAsset，AtomicTask 终态不受影响。
	// @EN The Artifact could not be registered as a UserAsset; the AtomicTask terminal state is unchanged.
	ErrAIAppArtifactRegistrationAtomicTaskUnchanged int = 130828
	// @HTTP 200
	// @CN 当前用户缺少所需的应用平台权限。
	// @EN The current user lacks the required application-platform permission.
	ErrAIAppPermissionDenied int = 131020
	// @HTTP 200
	// @CN API Workflow 文件缺失、不是合法 JSON 或不符合 ComfyUI API Workflow 基础结构。
	// @EN The API Workflow file is missing, invalid JSON, or not a valid ComfyUI API Workflow structure.
	ErrAIAppComfyUIWorkflowFileInvalid int = 131220
	// @HTTP 200
	// @CN 指定的来源或目标 EngineInstance 不是 ComfyUI 类型。
	// @EN The selected source or target EngineInstance is not a ComfyUI engine.
	ErrAIAppComfyUIEngineTypeInvalid int = 131221
	// @HTTP 200
	// @CN 无法从指定 ComfyUI EngineInstance 获取有效的 object_info。
	// @EN Valid object_info could not be retrieved from the selected ComfyUI EngineInstance.
	ErrAIAppComfyUIObjectInfoUnavailable int = 131222
	// @HTTP 200
	// @CN 工作流节点、输入连接或输出索引引用无效。
	// @EN The workflow contains an invalid node, input connection, or output index reference.
	ErrAIAppComfyUIWorkflowReferenceInvalid int = 131223
	// @HTTP 200
	// @CN 工作流节点、参数或运行依赖与目标 ComfyUI 实例不兼容。
	// @EN The workflow nodes, parameters, or runtime dependencies are incompatible with the target ComfyUI engine.
	ErrAIAppComfyUIWorkflowIncompatible int = 131224
	// @HTTP 200
	// @CN ComfyUI 工作流不存在或当前用户不可见。
	// @EN The ComfyUI workflow does not exist or is not visible to the current user.
	ErrAIAppComfyUIWorkflowNotFound int = 131225
	// @HTTP 200
	// @CN 已归档工作流不能创建兼容性校验或转换为模板。
	// @EN An archived workflow cannot be validated or converted into a template.
	ErrAIAppComfyUIWorkflowArchived int = 131226
	// @HTTP 200
	// @CN 兼容性校验记录不存在、不可见或不属于指定工作流。
	// @EN The compatibility validation does not exist, is not visible, or does not belong to the selected workflow.
	ErrAIAppComfyUIValidationNotFound int = 131227
	// @HTTP 200
	// @CN 所选校验结果不是 compatible，不能用于模板转换。
	// @EN The selected validation is not compatible and cannot be used for template conversion.
	ErrAIAppComfyUIValidationNotCompatible int = 131228
	// @HTTP 200
	// @CN 模板输入、固定参数、转换规则、输出提取或 Engine 约束不完整。
	// @EN The template inputs, fixed parameters, mappings, output extraction, or engine restrictions are incomplete.
	ErrAIAppComfyUITemplateContractInvalid int = 131229
	// @HTTP 200
	// @CN 工作流已转换为应用模板，不能再次转换。
	// @EN The workflow has already been converted into an application template and cannot be converted again.
	ErrAIAppComfyUIWorkflowAlreadyConverted int = 131230
	// @HTTP 200
	// @CN 转换幂等键已被同一所有者的其他工作流使用。
	// @EN The conversion idempotency key is already used by another workflow owned by the same user.
	ErrAIAppComfyUIConversionIdempotencyConflict int = 131231
	// @HTTP 200
	// @CN ComfyUI 工作流资源版本已变化，请刷新后重试。
	// @EN The ComfyUI workflow resource version has changed; refresh and retry.
	ErrAIAppComfyUIResourceVersionConflict int = 131232
	// @HTTP 200
	// @CN 当前用户无权访问或代管该 ComfyUI 工作流。
	// @EN The current user is not allowed to access or administer this ComfyUI workflow.
	ErrAIAppComfyUIWorkflowAccessDenied int = 131233
)

// task-center: SSOT scoped business errors.
const (
	// @HTTP 200
	// @CN 任务定义不合法。
	// @EN Task definition is invalid.
	ErrTaskDefinitionInvalid int = 140200

	// @HTTP 200
	// @CN DAGTaskGroup 存在环形依赖。
	// @EN DAGTaskGroup contains a cyclic dependency.
	ErrTaskDAGCycleDetected int = 140201
	// @HTTP 200
	// @CN TaskGroup 模板或执行策略不合法。
	// @EN TaskGroup template or execution policy is invalid.
	ErrTaskGroupInvalid int = 140202
	// @HTTP 200
	// @CN DAGTaskGroup 节点、边或输入引用不合法。
	// @EN DAGTaskGroup nodes, edges, or input references are invalid.
	ErrDAGTaskGroupInvalid int = 140203
	// @HTTP 200
	// @CN functionRef 未注册或当前调用方不可使用。
	// @EN functionRef is not registered or is unavailable to the caller.
	ErrTaskFunctionRefNotRegistered int = 140204

	// @HTTP 200
	// @CN 任务运行不存在或当前用户不可见。
	// @EN Task run does not exist or is not visible to the current user.
	ErrTaskRunNotFound int = 140400

	// @HTTP 200
	// @CN 任务当前状态不允许执行该操作。
	// @EN Current task run status does not allow this operation.
	ErrTaskRunStateBlocked int = 140401

	// @HTTP 200
	// @CN 任务重试策略不合法。
	// @EN Task retry policy is invalid.
	ErrTaskRetryPolicyInvalid int = 140402

	// @HTTP 200
	// @CN 相同应用运行和幂等键已用于不同的任务运行创建请求。
	// @EN The same application run and idempotency key were used with a different task run creation request.
	ErrTaskRunIdempotencyConflict int = 140403
	// @HTTP 200
	// @CN AtomicTask 不存在或当前调用方不可见。
	// @EN AtomicTask does not exist or is not visible to the caller.
	ErrAtomicTaskNotFound int = 140405
	// @HTTP 200
	// @CN AtomicTask 当前状态不允许该操作。
	// @EN Current AtomicTask status does not allow this operation.
	ErrAtomicTaskStateBlocked int = 140406
	// @HTTP 200
	// @CN AtomicTask 幂等键已用于不同请求。
	// @EN AtomicTask idempotency key was used for a different request.
	ErrAtomicTaskIdempotencyConflict int = 140407
	// @HTTP 200
	// @CN TaskGroup 不存在或当前调用方不可见。
	// @EN TaskGroup does not exist or is not visible to the caller.
	ErrTaskGroupNotFound int = 140408
	// @HTTP 200
	// @CN DAGTaskGroup 不存在或当前调用方不可见。
	// @EN DAGTaskGroup does not exist or is not visible to the caller.
	ErrDAGTaskGroupNotFound int = 140409

	// @HTTP 200
	// @CN Worker 不存在、不可用或能力不匹配。
	// @EN Worker does not exist, is unavailable, or capability does not match.
	ErrTaskWorkerNotAvailable int = 140600
	// @HTTP 200
	// @CN 工作流运行时当前不可用。
	// @EN Workflow runtime is currently unavailable.
	ErrWorkflowRuntimeUnavailable int = 140601
	// @HTTP 200
	// @CN 工作流运行时拒绝注册、启动、取消或重试请求。
	// @EN Workflow runtime rejected the register, start, cancel, or retry request.
	ErrWorkflowRuntimeRejected int = 140602

	// @HTTP 200
	// @CN ExecutionLease 无效、已过期或不属于当前 Worker。
	// @EN ExecutionLease is invalid, expired, or does not belong to current worker.
	ErrTaskLeaseInvalid int = 140800

	// @HTTP 200
	// @CN 当前执行尝试不允许更新任务结果。
	// @EN Current task attempt is not allowed to update task result.
	ErrTaskAttemptUpdateRejected int = 141000
	// @HTTP 200
	// @CN TaskAttempt 不存在或不属于指定 AtomicTask。
	// @EN TaskAttempt does not exist or does not belong to the AtomicTask.
	ErrTaskAttemptNotFound int = 141001

	// @HTTP 200
	// @CN 当前用户缺少任务中心操作权限。
	// @EN Current user does not have task center permission.
	ErrTaskPermissionDenied int = 141200
	// @HTTP 200
	// @CN TaskSchedule 的目标、cron、时区或 runAt 不合法。
	// @EN TaskSchedule target, cron, timezone, or runAt is invalid.
	ErrTaskScheduleInvalid int = 141400
	// @HTTP 200
	// @CN TaskSchedule 不存在或当前调用方不可见。
	// @EN TaskSchedule does not exist or is not visible to the caller.
	ErrTaskScheduleNotFound int = 141401
	// @HTTP 200
	// @CN TaskSchedule 当前状态不允许该操作。
	// @EN Current TaskSchedule status does not allow this operation.
	ErrTaskScheduleStateBlocked int = 141402
)

// workflow-canvas: spec-v1.0.0 business errors.
const (
	// @HTTP 200
	// @CN Canvas 不存在或当前调用方不可见。
	// @EN Canvas does not exist or is not visible to the caller.
	ErrCanvasNotFound int = 160200
	// @HTTP 200
	// @CN Canvas 草稿 revision 已变化，请刷新后重试。
	// @EN Canvas draft revision has changed; refresh before retrying.
	ErrCanvasRevisionConflict int = 160201
	// @HTTP 200
	// @CN Canvas 图的节点、边、端口或输入绑定不合法。
	// @EN Canvas graph nodes, edges, ports, or input bindings are invalid.
	ErrCanvasGraphInvalid int = 160202
	// @HTTP 200
	// @CN Canvas 图存在环形依赖。
	// @EN Canvas graph contains a cyclic dependency.
	ErrCanvasCycleDetected int = 160203
	// @HTTP 200
	// @CN Canvas 节点、边或动态展开上限超出允许范围。
	// @EN Canvas node, edge, or dynamic expansion limit is exceeded.
	ErrCanvasLimitExceeded int = 160204
	// @HTTP 200
	// @CN 节点引用的 ApplicationVersion 或 functionRef 不可用。
	// @EN ApplicationVersion or functionRef referenced by the node is unavailable.
	ErrCanvasNodeReferenceInvalid int = 160205
	// @HTTP 200
	// @CN CanvasVersion 不存在或当前调用方不可见。
	// @EN CanvasVersion does not exist or is not visible to the caller.
	ErrCanvasVersionNotFound int = 160400
	// @HTTP 200
	// @CN CanvasVersion 编译或运行时定义注册失败。
	// @EN CanvasVersion compilation or runtime definition registration failed.
	ErrCanvasPublishFailed int = 160401
	// @HTTP 200
	// @CN CanvasRun 不存在或当前调用方不可见。
	// @EN CanvasRun does not exist or is not visible to the caller.
	ErrCanvasRunNotFound int = 160600
	// @HTTP 200
	// @CN CanvasRun 当前状态不允许该操作。
	// @EN Current CanvasRun status does not allow this operation.
	ErrCanvasRunStateBlocked int = 160601
	// @HTTP 200
	// @CN CanvasRun 幂等键已用于不同的版本或输入。
	// @EN CanvasRun idempotency key was used for a different version or input.
	ErrCanvasRunIdempotencyConflict int = 160602
	// @HTTP 200
	// @CN 当前调用方缺少工作流画布操作权限。
	// @EN Caller does not have the required workflow canvas permission.
	ErrCanvasPermissionDenied int = 160800
)
