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
| ErrAIAppTaskProjectionStale | 130821 | 200 | The TaskRun status projection version is stale. | TaskRun 状态投影版本过旧。 |
| ErrAIAppProviderRuntimeCapabilityMismatch | 130822 | 200 | The provider rejected a capability combination declared by ProviderCapability. | 外部平台拒绝了当前 ProviderCapability 声明的能力组合。 |
| ErrAIAppTaskRunCreateFailed | 130823 | 200 | TaskRun creation failed; the ApplicationRun snapshot was retained. | TaskRun 创建失败，ApplicationRun 快照已保留。 |
| ErrAIAppArtifactRegistrationFailed | 130824 | 200 | Artifact registration as a UserAsset failed. | Artifact 登记 UserAsset 失败。 |
| ErrAIAppApplicationRunNotFound | 130825 | 200 | The ApplicationRun does not exist or is not visible to the current user. | ApplicationRun 不存在或当前用户不可见。 |
| ErrAIAppPermissionDenied | 131020 | 200 | The current user lacks the required application-platform permission. | 当前用户缺少所需的应用平台权限。 |
| ErrTaskDefinitionInvalid | 140200 | 200 | Task definition is invalid. | 任务定义不合法。 |
| ErrTaskDAGCycleDetected | 140201 | 200 | DAGFlowTask contains a cyclic dependency. | DAGFlowTask 存在环形依赖。 |
| ErrTaskRunNotFound | 140400 | 200 | Task run does not exist or is not visible to the current user. | 任务运行不存在或当前用户不可见。 |
| ErrTaskRunStateBlocked | 140401 | 200 | Current task run status does not allow this operation. | 任务当前状态不允许执行该操作。 |
| ErrTaskRetryPolicyInvalid | 140402 | 200 | Task retry policy is invalid. | 任务重试策略不合法。 |
| ErrTaskRunIdempotencyConflict | 140403 | 200 | The same application run and idempotency key were used with a different task run creation request. | 相同应用运行和幂等键已用于不同的任务运行创建请求。 |
| ErrTaskWorkerNotAvailable | 140600 | 200 | Worker does not exist, is unavailable, or capability does not match. | Worker 不存在、不可用或能力不匹配。 |
| ErrTaskLeaseInvalid | 140800 | 200 | ExecutionLease is invalid, expired, or does not belong to current worker. | ExecutionLease 无效、已过期或不属于当前 Worker。 |
| ErrTaskAttemptUpdateRejected | 141000 | 200 | Current task attempt is not allowed to update task result. | 当前执行尝试不允许更新任务结果。 |
| ErrTaskPermissionDenied | 141200 | 200 | Current user does not have task center permission. | 当前用户缺少任务中心操作权限。 |
| ErrUserNotFound | 110001 | 500 | Unset error message | 错误信息未设置 |
