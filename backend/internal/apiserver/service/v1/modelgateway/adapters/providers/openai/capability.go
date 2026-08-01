package openai

const capabilityManifestYAML = `
schema_version: "1.0"
id: openai-responses
name_i18n: {zh-CN: OpenAI Responses 能力, en-US: OpenAI Responses Capability}
description_i18n: {zh-CN: GPT-5.6 系列稳定非流式 Responses 能力。, en-US: Stable non-streaming Responses capability for the GPT-5.6 family.}
kind: catalog
binding_policy: manual
application_engine_type_id: openai_responses
revision: "2026-08-01.1"
enabled: true
provider: {code: openai, name: OpenAI, model_owner: OpenAI, serving_platform: OpenAI API, official_website: https://openai.com/}
sources:
  - {type: api_reference, title: Create a model response, url: https://developers.openai.com/api/reference/resources/responses/methods/create/, checked_at: "2026-08-01", scope: Stable non-streaming POST /v1/responses request and response fields.}
models:
  - {id: gpt-5.6, provider_model_id: gpt-5.6, display_name_i18n: {zh-CN: GPT-5.6, en-US: GPT-5.6}, description_i18n: {zh-CN: GPT-5.6 默认别名。, en-US: Default GPT-5.6 alias.}, family: gpt-5.6, variant: alias, lifecycle: {status: active}}
  - {id: gpt-5.6-sol, provider_model_id: gpt-5.6-sol, display_name_i18n: {zh-CN: GPT-5.6 Sol, en-US: GPT-5.6 Sol}, description_i18n: {zh-CN: GPT-5.6 旗舰能力模型。, en-US: Flagship GPT-5.6 model.}, family: gpt-5.6, variant: sol, lifecycle: {status: active}}
  - {id: gpt-5.6-terra, provider_model_id: gpt-5.6-terra, display_name_i18n: {zh-CN: GPT-5.6 Terra, en-US: GPT-5.6 Terra}, description_i18n: {zh-CN: 平衡质量和成本的 GPT-5.6 模型。, en-US: GPT-5.6 model balancing quality and cost.}, family: gpt-5.6, variant: terra, lifecycle: {status: active}}
  - {id: gpt-5.6-luna, provider_model_id: gpt-5.6-luna, display_name_i18n: {zh-CN: GPT-5.6 Luna, en-US: GPT-5.6 Luna}, description_i18n: {zh-CN: 面向高吞吐场景的 GPT-5.6 模型。, en-US: GPT-5.6 model for efficient workloads.}, family: gpt-5.6, variant: luna, lifecycle: {status: active}}
operations:
  - id: responses
    capability_definition_id: text.responses
    name_i18n: {zh-CN: Responses 响应, en-US: Responses}
    description_i18n: {zh-CN: OpenAI 官方 Responses API；ChatGPT 不是 API model ID。, en-US: Official OpenAI Responses API; ChatGPT is not an API model ID.}
    execution_mode: synchronous
    input_media_types: [text, image, json]
    output_media_types: [text, json]
    unsupported_parameters: [stream, background]
    input_schema: &responses_input
      type: object
      additionalProperties: false
      required: [model, input]
      properties:
        model: {type: string, enum: [gpt-5.6, gpt-5.6-sol, gpt-5.6-terra, gpt-5.6-luna]}
        input: {type: [string, array]}
        instructions: {type: string}
        include: {type: array, items: {type: string}}
        max_output_tokens: {type: integer, minimum: 1}
        max_tool_calls: {type: integer, minimum: 1}
        metadata: {type: object, additionalProperties: {type: string}}
        parallel_tool_calls: {type: boolean, default: true}
        previous_response_id: {type: string}
        prompt: {type: object}
        prompt_cache_key: {type: string}
        reasoning: {type: object}
        safety_identifier: {type: string, minLength: 1, maxLength: 64}
        service_tier: {type: string, enum: [auto, default, flex, priority]}
        store: {type: boolean}
        temperature: {type: number, minimum: 0, maximum: 2}
        text: {type: object}
        tool_choice: {type: [string, object]}
        tools: {type: array, items: {type: object}}
        top_logprobs: {type: integer, minimum: 0, maximum: 20}
        top_p: {type: number, minimum: 0, maximum: 1}
        truncation: {type: string, enum: [auto, disabled]}
        user: {type: string}
    output_schema: &responses_output
      type: object
      additionalProperties: true
      required: [id, object, status, output]
      properties:
        id: {type: string}
        object: {type: string, const: response}
        status: {type: string, enum: [completed, failed, incomplete, in_progress, queued]}
        model: {type: string}
        output: {type: array, items: {type: object}}
        output_text: {type: string}
        error: {type: [object, 'null']}
        incomplete_details: {type: [object, 'null']}
        usage: {type: [object, 'null'], additionalProperties: true}
variants:
  - {id: gpt-5.6-responses, model_id: gpt-5.6, operation_id: responses, lifecycle: {status: active}, input_schema: {type: object, properties: {model: {const: gpt-5.6}}}, output_schema: {type: object}}
  - {id: gpt-5.6-sol-responses, model_id: gpt-5.6-sol, operation_id: responses, lifecycle: {status: active}, input_schema: {type: object, properties: {model: {const: gpt-5.6-sol}}}, output_schema: {type: object}}
  - {id: gpt-5.6-terra-responses, model_id: gpt-5.6-terra, operation_id: responses, lifecycle: {status: active}, input_schema: {type: object, properties: {model: {const: gpt-5.6-terra}}}, output_schema: {type: object}}
  - {id: gpt-5.6-luna-responses, model_id: gpt-5.6-luna, operation_id: responses, lifecycle: {status: active}, input_schema: {type: object, properties: {model: {const: gpt-5.6-luna}}}, output_schema: {type: object}}
notes: [Streaming is unsupported by ApplicationRun.]
---
schema_version: "1.0"
id: openai-images
name_i18n: {zh-CN: OpenAI 图像能力, en-US: OpenAI Images Capability}
description_i18n: {zh-CN: GPT Image 2 稳定图像生成能力。, en-US: Stable GPT Image 2 generation capability.}
kind: catalog
binding_policy: manual
application_engine_type_id: openai_images
revision: "2026-08-01.1"
enabled: true
provider: {code: openai, name: OpenAI, model_owner: OpenAI, serving_platform: OpenAI API, official_website: https://openai.com/}
sources:
  - {type: api_reference, title: Generate images, url: https://developers.openai.com/api/reference/resources/images/methods/generate/, checked_at: "2026-08-01", scope: Stable GPT Image 2 image generation request and response fields.}
models:
  - {id: gpt-image-2, provider_model_id: gpt-image-2, display_name_i18n: {zh-CN: GPT Image 2, en-US: GPT Image 2}, description_i18n: {zh-CN: OpenAI GPT Image 2 图像生成模型。, en-US: OpenAI GPT Image 2 generation model.}, family: gpt-image, variant: "2", lifecycle: {status: active}}
operations:
  - id: image-generations
    capability_definition_id: image.text_to_image
    name_i18n: {zh-CN: 图像生成, en-US: Image Generations}
    description_i18n: {zh-CN: 通过 OpenAI Images API 生成图像。, en-US: Generate images through OpenAI Images.}
    execution_mode: synchronous
    input_media_types: [text]
    output_media_types: [image, json]
    input_schema:
      type: object
      additionalProperties: false
      required: [model, prompt]
      properties:
        model: {type: string, const: gpt-image-2}
        prompt: {type: string, minLength: 1, maxLength: 32000}
        background: {type: string, enum: [auto, opaque, transparent], default: auto}
        moderation: {type: string, enum: [auto, low], default: auto}
        n: {type: integer, minimum: 1, maximum: 10, default: 1}
        output_compression: {type: integer, minimum: 0, maximum: 100}
        output_format: {type: string, enum: [png, jpeg, webp], default: png}
        quality: {type: string, enum: [auto, low, medium, high], default: auto}
        size: {type: string, enum: [auto, 1024x1024, 1536x1024, 1024x1536], default: auto}
        user: {type: string}
    output_schema:
      type: object
      additionalProperties: true
      required: [data]
      properties:
        created: {type: integer}
        data:
          type: array
          minItems: 1
          items:
            type: object
            additionalProperties: true
            properties:
              b64_json: {type: string}
              url: {type: string, format: uri}
              revised_prompt: {type: string}
        usage: {type: object, additionalProperties: true}
variants:
  - {id: gpt-image-2-image-generations, model_id: gpt-image-2, operation_id: image-generations, lifecycle: {status: active}, input_schema: {type: object, properties: {model: {const: gpt-image-2}}}, output_schema: {type: object}}
`
