# `configs`

组件配置模板：

+ apiserver.yaml: Go API Server 配置模板，提供 `/api/v1` HTTP API。

`taskworker` 与 `notificationworker` 复用该 Options/配置结构。两者不监听模板中的
HTTP 端口；`notificationworker` 仅使用数据库与 SSE UserEvent 保留配置，并显式关闭
WorkflowRuntime。

`infraserver` 也复用该 Options/配置模板，但通过 `OMNIMAM_INFRA_*` 环境变量配置
Docker socket、服务 token、监听地址和 profile image 映射。

#
