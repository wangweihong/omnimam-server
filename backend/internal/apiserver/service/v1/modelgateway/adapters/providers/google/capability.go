package google

const capabilityManifestYAML = `
schema_version: "1.0"
id: google-gemini
name_i18n: {zh-CN: Google Gemini 与 Nano Banana, en-US: Google Gemini and Nano Banana}
description_i18n: {zh-CN: Gemini 文本模型和三代 Nano Banana 图像模型。, en-US: Gemini text models and three Nano Banana image model generations.}
kind: catalog
binding_policy: manual
application_engine_type_id: google_gemini
revision: "2026-08-01.1"
enabled: true
provider: {code: google_gemini, name: Google Gemini, model_owner: Google, serving_platform: Gemini API, official_website: https://ai.google.dev/gemini-api}
sources:
  - {type: api_reference, title: Gemini Interactions API, url: https://ai.google.dev/gemini-api/docs, checked_at: "2026-08-01", scope: Stable non-streaming Interactions request and response fields.}
  - {type: api_reference, title: Nano Banana image generation, url: https://ai.google.dev/gemini-api/docs/image-generation, checked_at: "2026-08-01", scope: Image model IDs, inputs, generation configuration, and outputs.}
models:
  - {id: gemini-2.5-flash, provider_model_id: gemini-2.5-flash, display_name_i18n: {zh-CN: Gemini 2.5 Flash, en-US: Gemini 2.5 Flash}, description_i18n: {zh-CN: 通用低延迟 Gemini 文本模型。, en-US: General low-latency Gemini text model.}, family: gemini-2.5, variant: flash, lifecycle: {status: active}}
  - {id: gemini-2.5-pro, provider_model_id: gemini-2.5-pro, display_name_i18n: {zh-CN: Gemini 2.5 Pro, en-US: Gemini 2.5 Pro}, description_i18n: {zh-CN: 面向复杂任务的 Gemini 文本模型。, en-US: Gemini text model for complex tasks.}, family: gemini-2.5, variant: pro, lifecycle: {status: active}}
  - {id: gemini-3.1-flash-image, provider_model_id: gemini-3.1-flash-image, display_name_i18n: {zh-CN: Nano Banana 2, en-US: Nano Banana 2}, description_i18n: {zh-CN: 兼顾速度、4K 与文字渲染的图像模型。, en-US: Image model balancing speed, 4K, and text rendering.}, family: nano-banana, variant: "2", lifecycle: {status: active}}
  - {id: gemini-3-pro-image, provider_model_id: gemini-3-pro-image, display_name_i18n: {zh-CN: Nano Banana Pro, en-US: Nano Banana Pro}, description_i18n: {zh-CN: 面向复杂视觉任务的高精度图像模型。, en-US: High-precision image model for complex visual tasks.}, family: nano-banana, variant: pro, lifecycle: {status: active}}
  - {id: gemini-2.5-flash-image, provider_model_id: gemini-2.5-flash-image, display_name_i18n: {zh-CN: Nano Banana, en-US: Nano Banana}, description_i18n: {zh-CN: 快速低成本 Gemini 图像模型。, en-US: Fast, cost-efficient Gemini image model.}, family: nano-banana, variant: standard, lifecycle: {status: active}}
operations:
  - id: interactions-text
    capability_definition_id: text.responses
    name_i18n: {zh-CN: Gemini 文本交互, en-US: Gemini Text Interaction}
    description_i18n: {zh-CN: 通过 Gemini Interactions API 生成文本结果。, en-US: Generate text through Gemini Interactions.}
    execution_mode: synchronous
    input_media_types: [text, image, json]
    output_media_types: [text, json]
    unsupported_parameters: [stream]
    input_schema: &interaction_input
      type: object
      additionalProperties: false
      required: [model, input]
      properties:
        model: {type: string}
        input: {type: [string, array]}
        system_instruction: {type: [string, object]}
        generation_config:
          type: object
          additionalProperties: false
          properties:
            candidate_count: {type: integer, minimum: 1, maximum: 8}
            max_output_tokens: {type: integer, minimum: 1}
            response_mime_type: {type: string}
            response_schema: {type: object}
            seed: {type: integer}
            stop_sequences: {type: array, items: {type: string}, maxItems: 5}
            temperature: {type: number, minimum: 0, maximum: 2}
            thinking_config: {type: object}
            top_k: {type: integer, minimum: 1}
            top_p: {type: number, minimum: 0, maximum: 1}
        safety_settings: {type: array, items: {type: object}}
        tool_config: {type: object}
        tools: {type: array, items: {type: object}}
        cached_content: {type: string}
        labels: {type: object, additionalProperties: {type: string}}
    output_schema: &interaction_output
      type: object
      additionalProperties: true
      required: [id]
      properties:
        id: {type: string}
        status: {type: string}
        outputs: {type: array, items: {type: object}}
        usage_metadata: {type: object, additionalProperties: true}
  - id: interactions-image
    capability_definition_id: image.text_to_image
    name_i18n: {zh-CN: Nano Banana 图像生成, en-US: Nano Banana Image Generation}
    description_i18n: {zh-CN: 通过 Gemini Interactions API 生成或编辑图像。, en-US: Generate or edit images through Gemini Interactions.}
    execution_mode: synchronous
    input_media_types: [text, image]
    output_media_types: [image, text, json]
    unsupported_parameters: [stream]
    input_schema:
      type: object
      additionalProperties: false
      required: [model, input]
      properties:
        model: {type: string}
        input: {type: [string, array]}
        generation_config:
          type: object
          additionalProperties: false
          properties:
            candidate_count: {type: integer, minimum: 1, maximum: 4}
            image_config:
              type: object
              additionalProperties: false
              properties:
                aspect_ratio: {type: string, enum: ['1:1', '2:3', '3:2', '3:4', '4:3', '4:5', '5:4', '9:16', '16:9', '21:9']}
                image_size: {type: string, enum: [1K, 2K, 4K]}
            response_modalities: {type: array, items: {type: string, enum: [TEXT, IMAGE]}, minItems: 1}
            seed: {type: integer}
            temperature: {type: number, minimum: 0, maximum: 2}
        safety_settings: {type: array, items: {type: object}}
        labels: {type: object, additionalProperties: {type: string}}
    output_schema: *interaction_output
variants:
  - {id: gemini-2.5-flash-interactions-text, model_id: gemini-2.5-flash, operation_id: interactions-text, lifecycle: {status: active}, input_schema: {type: object, properties: {model: {const: gemini-2.5-flash}}}, output_schema: {type: object}}
  - {id: gemini-2.5-pro-interactions-text, model_id: gemini-2.5-pro, operation_id: interactions-text, lifecycle: {status: active}, input_schema: {type: object, properties: {model: {const: gemini-2.5-pro}}}, output_schema: {type: object}}
  - {id: gemini-3.1-flash-image-interactions-image, model_id: gemini-3.1-flash-image, operation_id: interactions-image, lifecycle: {status: active}, input_schema: {type: object, properties: {model: {const: gemini-3.1-flash-image}}}, output_schema: {type: object}}
  - {id: gemini-3-pro-image-interactions-image, model_id: gemini-3-pro-image, operation_id: interactions-image, lifecycle: {status: active}, input_schema: {type: object, properties: {model: {const: gemini-3-pro-image}}}, output_schema: {type: object}}
  - {id: gemini-2.5-flash-image-interactions-image, model_id: gemini-2.5-flash-image, operation_id: interactions-image, lifecycle: {status: active}, input_schema: {type: object, properties: {model: {const: gemini-2.5-flash-image}}}, output_schema: {type: object}}
notes:
  - Streaming endpoints are not part of the ApplicationRun execution contract.
`
