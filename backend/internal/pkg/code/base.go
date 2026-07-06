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
	// @CN 用户未登录或登录态失效。
	// @EN User is not authenticated or the session is invalid.
	ErrAIChatUnauthenticated int = 110200

	// @HTTP 200
	// @CN 话题不存在或不属于当前用户。
	// @EN Topic does not exist or does not belong to the current user.
	ErrAIChatTopicNotFound int = 110201

	// @HTTP 200
	// @CN 消息不存在或不属于当前用户。
	// @EN Message does not exist or does not belong to the current user.
	ErrAIChatMessageNotFound int = 110202

	// @HTTP 200
	// @CN 消息内容为空且未提供图片附件。
	// @EN Message content is empty and no image attachment was provided.
	ErrAIChatMessageEmpty int = 110203

	// @HTTP 200
	// @CN 助手不存在或不可用于当前用户。
	// @EN Assistant does not exist or is unavailable to the current user.
	ErrAIChatAssistantNotFound int = 110204

	// @HTTP 200
	// @CN 模型不存在或不属于当前用户。
	// @EN Model does not exist or does not belong to the current user.
	ErrAIChatModelNotFound int = 110205

	// @HTTP 200
	// @CN 模型存在但未启用。
	// @EN Model exists but is disabled.
	ErrAIChatModelDisabled int = 110206

	// @HTTP 200
	// @CN 模型不支持当前操作所需能力。
	// @EN Model does not support the capability required by this operation.
	ErrAIChatModelCapabilityUnsupported int = 110207

	// @HTTP 200
	// @CN 模型配置有效但 provider 或运行时当前不可用。
	// @EN Model configuration is valid but the provider or runtime is unavailable.
	ErrAIChatModelUnavailable int = 110208

	// @HTTP 200
	// @CN 同一话题已有运行中的 generation。
	// @EN The topic already has an active generation.
	ErrAIChatGenerationConflict int = 110209

	// @HTTP 200
	// @CN generation 不存在或不属于当前用户。
	// @EN Generation does not exist or does not belong to the current user.
	ErrAIChatGenerationNotFound int = 110210

	// @HTTP 200
	// @CN 分支来源消息不存在或不属于当前用户。
	// @EN Branch source message does not exist or does not belong to the current user.
	ErrAIChatBranchSourceMissing int = 110211

	// @HTTP 200
	// @CN 系统助手禁止删除或普通名称编辑。
	// @EN System assistant cannot be deleted or renamed by normal editing.
	ErrAIChatSystemAssistantProtected int = 110212

	// @HTTP 200
	// @CN 当前用户范围内助手名称重复。
	// @EN Assistant name is duplicated in the current user scope.
	ErrAIChatDuplicateAssistantName int = 110213

	// @HTTP 200
	// @CN 当前用户未配置默认翻译模型。
	// @EN Current user has no default translation model configured.
	ErrAIChatTranslationModelMissing int = 110214

	// @HTTP 200
	// @CN 默认翻译模型不存在或不属于当前用户。
	// @EN Default translation model does not exist or does not belong to the current user.
	ErrAIChatTranslationModelNotFound int = 110215

	// @HTTP 200
	// @CN 默认翻译模型存在但未启用。
	// @EN Default translation model exists but is disabled.
	ErrAIChatTranslationModelDisabled int = 110216

	// @HTTP 200
	// @CN 附件格式不支持或模型能力不支持。
	// @EN Attachment format is unsupported or the model lacks the required capability.
	ErrAIChatAttachmentUnsupported int = 110217

	// @HTTP 200
	// @CN 单张图片超过 5MB。
	// @EN A single image exceeds 5MB.
	ErrAIChatAttachmentTooLarge int = 110218

	// @HTTP 200
	// @CN 前端导出组装失败。
	// @EN Frontend export assembly failed.
	ErrAIChatExportFailed int = 110219
)
