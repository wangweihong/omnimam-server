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
| ErrAIChatUnauthenticated | 110200 | 200 | User is not authenticated or the session is invalid. | 用户未登录或登录态失效。 |
| ErrAIChatTopicNotFound | 110201 | 200 | Topic does not exist or does not belong to the current user. | 话题不存在或不属于当前用户。 |
| ErrAIChatMessageNotFound | 110202 | 200 | Message does not exist or does not belong to the current user. | 消息不存在或不属于当前用户。 |
| ErrAIChatMessageEmpty | 110203 | 200 | Message content is empty and no image attachment was provided. | 消息内容为空且未提供图片附件。 |
| ErrAIChatAssistantNotFound | 110204 | 200 | Assistant does not exist or is unavailable to the current user. | 助手不存在或不可用于当前用户。 |
| ErrAIChatModelNotFound | 110205 | 200 | Model does not exist or does not belong to the current user. | 模型不存在或不属于当前用户。 |
| ErrAIChatModelDisabled | 110206 | 200 | Model exists but is disabled. | 模型存在但未启用。 |
| ErrAIChatModelCapabilityUnsupported | 110207 | 200 | Model does not support the capability required by this operation. | 模型不支持当前操作所需能力。 |
| ErrAIChatModelUnavailable | 110208 | 200 | Model configuration is valid but the provider or runtime is unavailable. | 模型配置有效但 provider 或运行时当前不可用。 |
| ErrAIChatGenerationConflict | 110209 | 200 | The topic already has an active generation. | 同一话题已有运行中的 generation。 |
| ErrAIChatGenerationNotFound | 110210 | 200 | Generation does not exist or does not belong to the current user. | generation 不存在或不属于当前用户。 |
| ErrAIChatBranchSourceMissing | 110211 | 200 | Branch source message does not exist or does not belong to the current user. | 分支来源消息不存在或不属于当前用户。 |
| ErrAIChatSystemAssistantProtected | 110212 | 200 | System assistant cannot be deleted or renamed by normal editing. | 系统助手禁止删除或普通名称编辑。 |
| ErrAIChatDuplicateAssistantName | 110213 | 200 | Assistant name is duplicated in the current user scope. | 当前用户范围内助手名称重复。 |
| ErrAIChatTranslationModelMissing | 110214 | 200 | Current user has no default translation model configured. | 当前用户未配置默认翻译模型。 |
| ErrAIChatTranslationModelNotFound | 110215 | 200 | Default translation model does not exist or does not belong to the current user. | 默认翻译模型不存在或不属于当前用户。 |
| ErrAIChatTranslationModelDisabled | 110216 | 200 | Default translation model exists but is disabled. | 默认翻译模型存在但未启用。 |
| ErrAIChatAttachmentUnsupported | 110217 | 200 | Attachment format is unsupported or the model lacks the required capability. | 附件格式不支持或模型能力不支持。 |
| ErrAIChatAttachmentTooLarge | 110218 | 200 | A single image exceeds 5MB. | 单张图片超过 5MB。 |
| ErrAIChatExportFailed | 110219 | 200 | Frontend export assembly failed. | 前端导出组装失败。 |
| ErrTaskDefinitionInvalid | 140200 | 200 | Task definition is invalid. | 任务定义不合法。 |
| ErrTaskDAGCycleDetected | 140201 | 200 | DAGFlowTask contains a cyclic dependency. | DAGFlowTask 存在环形依赖。 |
| ErrTaskRunNotFound | 140400 | 200 | Task run does not exist or is not visible to the current user. | 任务运行不存在或当前用户不可见。 |
| ErrTaskRunStateBlocked | 140401 | 200 | Current task run status does not allow this operation. | 任务当前状态不允许执行该操作。 |
| ErrTaskRetryPolicyInvalid | 140402 | 200 | Task retry policy is invalid. | 任务重试策略不合法。 |
| ErrTaskWorkerNotAvailable | 140600 | 200 | Worker does not exist, is unavailable, or capability does not match. | Worker 不存在、不可用或能力不匹配。 |
| ErrTaskLeaseInvalid | 140800 | 200 | ExecutionLease is invalid, expired, or does not belong to current worker. | ExecutionLease 无效、已过期或不属于当前 Worker。 |
| ErrTaskAttemptUpdateRejected | 141000 | 200 | Current task attempt is not allowed to update task result. | 当前执行尝试不允许更新任务结果。 |
| ErrTaskPermissionDenied | 141200 | 200 | Current user does not have task center permission. | 当前用户缺少任务中心操作权限。 |
| ErrUserNotFound | 110001 | 500 | Unset error message | 错误信息未设置 |
