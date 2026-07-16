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
	// @CN 平台适配器或操作已禁用。
	// @EN Provider adapter or operation is disabled.
	ErrAdapterDisabled int = 130200

	// @HTTP 200
	// @CN 平台操作版本与应用或引擎不兼容。
	// @EN Provider operation version is incompatible with the application or engine.
	ErrOperationVersionIncompatible int = 130201

	// @HTTP 200
	// @CN 工作流模板解析失败。
	// @EN Workflow template parsing failed.
	ErrTemplateParseFailed int = 130210

	// @HTTP 200
	// @CN 所选端口尚未解析完成。
	// @EN Selected port has not been resolved.
	ErrPortUnresolved int = 130211

	// @HTTP 200
	// @CN 候选输出尚未确认。
	// @EN Candidate output has not been confirmed.
	ErrCandidateOutputUnconfirmed int = 130212

	// @HTTP 200
	// @CN 模板存在应用引用，禁止删除。
	// @EN Template has application references and cannot be deleted.
	ErrTemplateReferenceBlocked int = 130213

	// @HTTP 200
	// @CN 应用输入映射非法。
	// @EN Application input mapping is invalid.
	ErrInputMappingInvalid int = 130300

	// @HTTP 200
	// @CN 应用输出映射非法。
	// @EN Application output mapping is invalid.
	ErrOutputMappingInvalid int = 130301

	// @HTTP 200
	// @CN 同一参数路径不能同时开放和固化。
	// @EN A parameter path cannot be both exposed and fixed.
	ErrFixedParameterConflicted int = 130302

	// @HTTP 200
	// @CN 应用存在运行引用，禁止删除。
	// @EN Application has run references and cannot be deleted.
	ErrApplicationReferenceBlocked int = 130303

	// @HTTP 200
	// @CN 自动路由没有找到可用应用引擎。
	// @EN Automatic routing found no available application engine.
	ErrEngineUnavailable int = 130400

	// @HTTP 200
	// @CN 用户指定的应用引擎不可用。
	// @EN The selected application engine is unavailable.
	ErrSelectedEngineUnavailable int = 130401

	// @HTTP 200
	// @CN 应用引擎认证配置非法。
	// @EN Application engine authentication configuration is invalid.
	ErrAppEngineAuthConfigInvalid int = 130402

	// @HTTP 200
	// @CN 应用引擎存在运行引用，禁止删除。
	// @EN Application engine has run references and cannot be deleted.
	ErrAppEngineReferenceBlocked int = 130403

	// @HTTP 200
	// @CN 平台适配器调用失败。
	// @EN Provider adapter invocation failed.
	ErrAdapterInvocationFailed int = 130500

	// @HTTP 200
	// @CN 已提交的外部任务无法恢复查询。
	// @EN Previously submitted external task could not be recovered.
	ErrExternalTaskRecoveryFailed int = 130501

	// @HTTP 200
	// @CN 应用真实测试失败。
	// @EN Application live test failed.
	ErrApplicationTestFailed int = 130502

	// @HTTP 200
	// @CN TaskRun 投影事件版本已过期。
	// @EN TaskRun projection event version is stale.
	ErrProjectionVersionStale int = 130503

	// @HTTP 200
	// @CN 当前用户缺少资源管理权或引擎使用权。
	// @EN Current user lacks resource management or engine usage permission.
	ErrAIAppPermissionDenied int = 130600
)

const (
	ErrTemplateNameDuplicated            = ErrTemplateReferenceBlocked
	ErrTemplateContentImmutable          = ErrTemplateParseFailed
	ErrMappingPathInvalid                = ErrInputMappingInvalid
	ErrFieldKeyDuplicated                = ErrInputMappingInvalid
	ErrFieldMappingIncomplete            = ErrInputMappingInvalid
	ErrFieldTypeInvalid                  = ErrInputMappingInvalid
	ErrApplicationRunCreateFailed        = ErrAdapterInvocationFailed
	ErrApplicationRunNotVisible          = ErrAIAppPermissionDenied
	ErrAppEngineNotVisible               = ErrAIAppPermissionDenied
	ErrAppEngineNameDuplicated           = ErrAppEngineAuthConfigInvalid
	ErrAppEngineUnhealthy                = ErrSelectedEngineUnavailable
	ErrAppEngineTypeMismatched           = ErrSelectedEngineUnavailable
	ErrSaaSPlatformMismatched            = ErrSelectedEngineUnavailable
	ErrAppEngineCapabilityUnsupported    = ErrSelectedEngineUnavailable
	ErrAppEngineHealthCheckConfigInvalid = ErrAppEngineAuthConfigInvalid
)

// task-center: SSOT scoped business errors.
const (
	// @HTTP 200
	// @CN 任务定义不合法。
	// @EN Task definition is invalid.
	ErrTaskDefinitionInvalid int = 140200

	// @HTTP 200
	// @CN DAGFlowTask 存在环形依赖。
	// @EN DAGFlowTask contains a cyclic dependency.
	ErrTaskDAGCycleDetected int = 140201

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
	// @CN Worker 不存在、不可用或能力不匹配。
	// @EN Worker does not exist, is unavailable, or capability does not match.
	ErrTaskWorkerNotAvailable int = 140600

	// @HTTP 200
	// @CN ExecutionLease 无效、已过期或不属于当前 Worker。
	// @EN ExecutionLease is invalid, expired, or does not belong to current worker.
	ErrTaskLeaseInvalid int = 140800

	// @HTTP 200
	// @CN 当前执行尝试不允许更新任务结果。
	// @EN Current task attempt is not allowed to update task result.
	ErrTaskAttemptUpdateRejected int = 141000

	// @HTTP 200
	// @CN 当前用户缺少任务中心操作权限。
	// @EN Current user does not have task center permission.
	ErrTaskPermissionDenied int = 141200
)
