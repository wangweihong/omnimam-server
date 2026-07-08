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
| ErrTemplateParseFailed | 130200 | 200 | Template parsing failed and the template cannot be created. | 模板解析失败，无法创建模板。 |
| ErrTemplateReferenceBlocked | 130201 | 200 | Template has references and cannot be deleted. | 模板存在引用，禁止删除。 |
| ErrTemplateNameDuplicated | 130203 | 200 | Template name is duplicated for the same owner. | 同一用户下模板名称重复。 |
| ErrTemplateContentImmutable | 130204 | 200 | Template kind, content, and parsed fields are immutable after creation. | 模板类型、内容或解析变量创建后不可修改。 |
| ErrMappingPathInvalid | 130300 | 200 | Field mapping source path is not present in parsed template fields. | 字段映射路径不在模板解析变量中。 |
| ErrFieldKeyDuplicated | 130301 | 200 | Field key is duplicated in the same application. | 同一应用内字段标识重复。 |
| ErrFieldMappingIncomplete | 130302 | 200 | Application field mappings are incomplete. | 应用字段映射不完整。 |
| ErrFieldTypeInvalid | 130303 | 200 | Field type is not derived from parsed template fields. | 字段类型不来自模板解析变量。 |
| ErrAIAppPermissionDenied | 130500 | 200 | Current user does not have permission. | 当前用户缺少操作权限。 |
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
