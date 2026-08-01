package deepseek

const capabilityManifestYAML = `
schema_version: "1.0"
id: deepseek-official
name_i18n: {zh-CN: DeepSeek 官方能力, en-US: DeepSeek Official Capability}
description_i18n: {zh-CN: DeepSeek 官方稳定非流式对话模型。, en-US: Official stable non-streaming DeepSeek chat models.}
kind: catalog
binding_policy: manual
application_engine_type_id: deepseek_official
revision: "2026-08-01.1"
enabled: true
provider: {code: deepseek, name: DeepSeek, model_owner: DeepSeek, serving_platform: DeepSeek Official API, official_website: https://www.deepseek.com/}
sources:
  - {type: api_reference, title: Create Chat Completion, url: https://api-docs.deepseek.com/api/create-chat-completion, checked_at: "2026-08-01", scope: Stable Chat Completions request and response protocol.}
  - {type: changelog, title: DeepSeek API Change Log, url: https://api-docs.deepseek.com/updates/, checked_at: "2026-08-01", scope: V4 Pro and Flash availability and alias retirement.}
models:
  - id: deepseek-v4-pro
    provider_model_id: deepseek-v4-pro
    display_name_i18n: {zh-CN: DeepSeek V4 Pro, en-US: DeepSeek V4 Pro}
    description_i18n: {zh-CN: DeepSeek V4 高质量对话模型。, en-US: High-quality DeepSeek V4 chat model.}
    family: deepseek-v4
    variant: pro
    lifecycle: {status: active, available_since: "2026-04-24"}
    context_window_tokens: 1000000
    maximum_output_tokens: 384000
    limits: {thinking_modes: [enabled, disabled], reasoning_effort: [high, max], maximum_tools: 128}
  - id: deepseek-v4-flash
    provider_model_id: deepseek-v4-flash
    display_name_i18n: {zh-CN: DeepSeek V4 Flash, en-US: DeepSeek V4 Flash}
    description_i18n: {zh-CN: DeepSeek V4 低延迟对话模型。, en-US: Low-latency DeepSeek V4 chat model.}
    family: deepseek-v4
    variant: flash
    lifecycle: {status: active, available_since: "2026-04-24"}
    context_window_tokens: 1000000
    maximum_output_tokens: 384000
    limits: {thinking_modes: [enabled, disabled], reasoning_effort: [high, max], maximum_tools: 128}
operations:
  - id: chat-completions
    capability_definition_id: text.chat_completion
    name_i18n: {zh-CN: 对话补全, en-US: Chat Completions}
    description_i18n: {zh-CN: DeepSeek 官方 OpenAI-compatible 对话补全。, en-US: Official DeepSeek OpenAI-compatible chat completions.}
    execution_mode: synchronous
    input_media_types: [text, json]
    output_media_types: [text, json]
    unsupported_parameters: [stream, frequency_penalty, presence_penalty, max_completion_tokens, developer_role]
    input_schema: &chat_input
      type: object
      additionalProperties: false
      required: [model, messages]
      properties:
        model: {type: string, enum: [deepseek-v4-pro, deepseek-v4-flash]}
        messages:
          type: array
          minItems: 1
          items:
            type: object
            additionalProperties: false
            required: [role, content]
            properties:
              role: {type: string, enum: [system, user, assistant, tool]}
              content: {type: [string, 'null']}
              name: {type: string}
              reasoning_content: {type: [string, 'null']}
              tool_call_id: {type: string}
              tool_calls: {type: array, items: {type: object}}
        thinking:
          type: object
          additionalProperties: false
          required: [type]
          properties: {type: {type: string, enum: [enabled, disabled], default: enabled}}
        reasoning_effort: {type: string, enum: [high, max], default: high}
        max_tokens: {type: integer, minimum: 1, maximum: 384000}
        response_format:
          type: object
          additionalProperties: false
          required: [type]
          properties: {type: {type: string, enum: [text, json_object], default: text}}
        stop: {type: [string, array], items: {type: string}, maxItems: 16}
        temperature: {type: number, minimum: 0, maximum: 2, default: 1}
        top_p: {type: number, minimum: 0, maximum: 1, default: 1}
        tools: {type: array, maxItems: 128, items: {type: object}}
        tool_choice: {type: [string, object]}
        logprobs: {type: boolean}
        top_logprobs: {type: integer, minimum: 0, maximum: 20}
        user_id: {type: string, pattern: '^[A-Za-z0-9_-]{1,512}$'}
      allOf:
        - if: {required: [top_logprobs]}
          then: {required: [logprobs], properties: {logprobs: {const: true}}}
        - if: {required: [thinking], properties: {thinking: {properties: {type: {const: enabled}}}}}
          then: {not: {required: [tool_choice]}}
    output_schema: &chat_output
      type: object
      additionalProperties: true
      required: [id, model, choices, usage]
      properties:
        id: {type: string}
        model: {type: string}
        choices: {type: array, minItems: 1, items: {type: object}}
        usage: {type: object, additionalProperties: true}
        system_fingerprint: {type: string}
variants:
  - {id: v4-pro-chat-completions, model_id: deepseek-v4-pro, operation_id: chat-completions, lifecycle: {status: active}, input_schema: {type: object, properties: {model: {const: deepseek-v4-pro}}}, output_schema: {type: object}}
  - {id: v4-flash-chat-completions, model_id: deepseek-v4-flash, operation_id: chat-completions, lifecycle: {status: active}, input_schema: {type: object, properties: {model: {const: deepseek-v4-flash}}}, output_schema: {type: object}}
notes:
  - Prefix Completion, FIM Completion, and streaming are excluded.
  - Provider credentials and base URL belong to EngineInstance.
`
