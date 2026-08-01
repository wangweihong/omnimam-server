package xai

const capabilityManifestYAML = `
schema_version: "1.0"
id: xai-grok-responses
name_i18n: {zh-CN: xAI Grok Responses 能力, en-US: xAI Grok Responses Capability}
description_i18n: {zh-CN: Grok 4.5 和 Grok 4.3 的官方非流式 Responses 能力。, en-US: Official non-streaming Responses capability for Grok 4.5 and Grok 4.3.}
kind: catalog
binding_policy: manual
application_engine_type_id: xai_responses
revision: "2026-08-01.1"
enabled: true
provider: {code: xai, name: xAI, model_owner: xAI, serving_platform: xAI API, official_website: https://x.ai/}
sources:
  - {type: api_reference, title: xAI API Overview, url: https://docs.x.ai/docs/overview, checked_at: "2026-08-01", scope: POST /v1/responses, stable request fields, response shape, and Grok model IDs.}
models:
  - id: grok-4.5
    provider_model_id: grok-4.5
    display_name_i18n: {zh-CN: Grok 4.5, en-US: Grok 4.5}
    description_i18n: {zh-CN: xAI 旗舰代码与通用模型。, en-US: xAI flagship model for code and general workloads.}
    family: grok-4
    variant: "4.5"
    lifecycle: {status: active}
  - id: grok-4.3
    provider_model_id: grok-4.3
    display_name_i18n: {zh-CN: Grok 4.3, en-US: Grok 4.3}
    description_i18n: {zh-CN: xAI 长上下文推理模型。, en-US: xAI long-context reasoning model.}
    family: grok-4
    variant: "4.3"
    lifecycle: {status: active}
operations:
  - id: responses
    capability_definition_id: text.responses
    name_i18n: {zh-CN: Grok Responses, en-US: Grok Responses}
    description_i18n: {zh-CN: 通过 xAI Responses API 生成文本和工具调用结果。, en-US: Generate text and tool-call results through xAI Responses.}
    execution_mode: synchronous
    input_media_types: [text, image, json]
    output_media_types: [text, json]
    unsupported_parameters: [stream]
    input_schema: &responses_input
      type: object
      additionalProperties: false
      required: [model, input]
      properties:
        model: {type: string, enum: [grok-4.5, grok-4.3]}
        input: {type: [string, array]}
        instructions: {type: string}
        max_output_tokens: {type: integer, minimum: 1}
        metadata: {type: object, additionalProperties: {type: string}}
        parallel_tool_calls: {type: boolean, default: true}
        previous_response_id: {type: string}
        reasoning: {type: object, additionalProperties: true}
        safety_identifier: {type: string, minLength: 1, maxLength: 64}
        store: {type: boolean}
        temperature: {type: number, minimum: 0, maximum: 2}
        text: {type: object, additionalProperties: true}
        tool_choice: {type: [string, object]}
        tools: {type: array, items: {type: object}}
        top_logprobs: {type: integer, minimum: 0, maximum: 20}
        top_p: {type: number, minimum: 0, maximum: 1}
        truncation: {type: string, enum: [auto, disabled]}
    output_schema: &responses_output
      type: object
      additionalProperties: true
      required: [id, object, status, output]
      properties:
        id: {type: string}
        object: {type: string, const: response}
        status: {type: string, enum: [completed, failed, incomplete, in_progress]}
        model: {type: string}
        output: {type: array, items: {type: object}}
        output_text: {type: string}
        usage: {type: object, additionalProperties: true}
variants:
  - id: grok-4.5-responses
    model_id: grok-4.5
    operation_id: responses
    lifecycle: {status: active}
    input_schema: {type: object, properties: {model: {const: grok-4.5}}}
    output_schema: {type: object}
  - id: grok-4.3-responses
    model_id: grok-4.3
    operation_id: responses
    lifecycle: {status: active}
    input_schema: {type: object, properties: {model: {const: grok-4.3}}}
    output_schema: {type: object}
notes:
  - Streaming is deliberately unsupported by the ApplicationRun execution contract.
`
