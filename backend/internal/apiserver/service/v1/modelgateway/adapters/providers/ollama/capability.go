package ollama

const capabilityManifestYAML = `
schema_version: "1.0"
id: ollama-openai-compatible
name_i18n: {zh-CN: Ollama OpenAI 兼容能力, en-US: Ollama OpenAI Compatibility}
description_i18n:
  zh-CN: 声明 Ollama Chat Completions 与无状态 Responses 协议；模型按实例发现。
  en-US: Declares Ollama Chat Completions and stateless Responses; models are discovered per instance.
kind: catalog
binding_policy: manual
application_engine_type_id: ollama
revision: "2026-08-01.1"
enabled: true
provider: {code: ollama, name: Ollama, model_owner: Local model authors, serving_platform: Ollama, official_website: https://ollama.com/}
sources:
  - {type: api_reference, title: OpenAI compatibility, url: https://docs.ollama.com/api/openai-compatibility, checked_at: "2026-08-01", scope: Chat Completions, stateless Responses, and model listing.}
models: []
operations:
  - id: chat-completions
    capability_definition_id: text.chat_completion
    name_i18n: {zh-CN: Ollama 对话补全, en-US: Ollama Chat Completions}
    description_i18n: {zh-CN: 调用 Ollama /v1/chat/completions。, en-US: Call Ollama /v1/chat/completions.}
    execution_mode: synchronous
    input_media_types: [text, image, json]
    output_media_types: [text, json]
    unsupported_parameters: [stream]
    input_schema: &chat_input
      type: object
      additionalProperties: false
      x-omnimam-validator-ids: [ollama.model-installed]
      required: [model, messages]
      properties:
        model: {type: string, minLength: 1}
        messages:
          type: array
          minItems: 1
          items:
            type: object
            additionalProperties: false
            required: [role, content]
            properties:
              role: {type: string, enum: [system, user, assistant, tool]}
              content: {type: [string, array, 'null']}
              name: {type: string}
              tool_calls: {type: array, items: {type: object}}
              tool_call_id: {type: string}
        frequency_penalty: {type: number, minimum: -2, maximum: 2}
        logit_bias: {type: object, additionalProperties: {type: number}}
        logprobs: {type: boolean}
        max_completion_tokens: {type: integer, minimum: 1}
        max_tokens: {type: integer, minimum: 1}
        n: {type: integer, minimum: 1}
        presence_penalty: {type: number, minimum: -2, maximum: 2}
        response_format: {type: object}
        seed: {type: integer}
        stop: {type: [string, array], items: {type: string}}
        temperature: {type: number, minimum: 0, maximum: 2}
        tool_choice: {type: [string, object]}
        tools: {type: array, items: {type: object}}
        top_logprobs: {type: integer, minimum: 0, maximum: 20}
        top_p: {type: number, minimum: 0, maximum: 1}
        user: {type: string}
    output_schema: &chat_output
      type: object
      additionalProperties: true
      required: [id, choices]
      properties:
        id: {type: string}
        model: {type: string}
        choices: {type: array, minItems: 1, items: {type: object}}
        usage: {type: object, additionalProperties: true}
  - id: responses
    capability_definition_id: text.responses
    name_i18n: {zh-CN: Ollama Responses, en-US: Ollama Responses}
    description_i18n: {zh-CN: 调用 Ollama v0.13.3+ 无状态 /v1/responses。, en-US: Call stateless /v1/responses in Ollama v0.13.3+.}
    execution_mode: synchronous
    input_media_types: [text, image, json]
    output_media_types: [text, json]
    unsupported_parameters: [stream, background, conversation, previous_response_id]
    input_schema:
      type: object
      additionalProperties: false
      x-omnimam-validator-ids: [ollama.model-installed]
      required: [model, input]
      properties:
        model: {type: string, minLength: 1}
        input: {type: [string, array]}
        instructions: {type: string}
        max_output_tokens: {type: integer, minimum: 1}
        metadata: {type: object, additionalProperties: {type: string}}
        parallel_tool_calls: {type: boolean}
        reasoning: {type: object}
        store: {type: boolean}
        temperature: {type: number, minimum: 0, maximum: 2}
        text: {type: object}
        tool_choice: {type: [string, object]}
        tools: {type: array, items: {type: object}}
        top_p: {type: number, minimum: 0, maximum: 1}
        truncation: {type: string, enum: [auto, disabled]}
    output_schema:
      type: object
      additionalProperties: true
      required: [id, object, status, output]
      properties:
        id: {type: string}
        object: {type: string}
        status: {type: string}
        model: {type: string}
        output: {type: array, items: {type: object}}
        usage: {type: object, additionalProperties: true}
variants: []
notes:
  - Static capability declares protocol schemas only; installed models are discovered from each EngineInstance.
`
