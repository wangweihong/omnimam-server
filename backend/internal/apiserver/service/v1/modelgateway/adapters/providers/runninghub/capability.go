package runninghub

const capabilityManifestYAML = `
schema_version: "1.0"
id: runninghub-workflow-runtime
name_i18n: {zh-CN: RunningHub 工作流运行时, en-US: RunningHub Workflow Runtime}
description_i18n:
  zh-CN: RunningHub 实例的系统绑定能力；workflowId 与节点映射由模板和运行快照固定。
  en-US: System binding for RunningHub instances; templates and run snapshots pin workflowId and node mappings.
kind: engine_binding
binding_policy: required_immutable
application_engine_type_id: runninghub_workflow
revision: "2026-08-01.1"
enabled: true
provider:
  code: runninghub
  name: RunningHub
  model_owner: Workflow authors
  serving_platform: RunningHub
  official_website: https://www.runninghub.cn/
sources:
  - type: api_reference
    title: RunningHub OpenAPI
    url: https://www.runninghub.cn/runninghub-api-doc-cn/api-425749013
    checked_at: "2026-08-01"
    scope: POST /task/openapi/create, /outputs, /cancel, apiKey, workflowId, and nodeInfoList.
models: []
operations: []
variants: []
`
