package comfyui

const capabilityManifestYAML = `
schema_version: "1.0"
id: comfyui-workflow-runtime
name_i18n: {zh-CN: ComfyUI 工作流运行时, en-US: ComfyUI Workflow Runtime}
description_i18n:
  zh-CN: ComfyUI 实例的系统绑定能力；具体能力由工作流契约与当前 object_info 决定。
  en-US: System binding for ComfyUI instances; workflow contracts and current object_info define executable capabilities.
kind: engine_binding
binding_policy: required_immutable
application_engine_type_id: comfyui
revision: "2026-08-01.1"
enabled: true
provider:
  code: comfyui
  name: ComfyUI
  model_owner: ComfyUI Workflow Authors
  serving_platform: Self-hosted ComfyUI
  official_website: https://www.comfy.org/
sources:
  - type: api_reference
    title: ComfyUI API
    url: https://docs.comfy.org/development/core-concepts/api
    checked_at: "2026-08-01"
    scope: Workflow API, queue execution, and instance runtime.
models: []
operations: []
variants: []
`
