package modelark

const capabilityManifestYAML = `
schema_version: "1.0"
id: seedance-byteplus
name_i18n: {zh-CN: BytePlus Seedance 2.0, en-US: BytePlus Seedance 2.0}
description_i18n: {zh-CN: BytePlus ModelArk 提供的 Seedance 2.0 视频生成能力。, en-US: Seedance 2.0 video generation served by BytePlus ModelArk.}
kind: catalog
binding_policy: manual
application_engine_type_id: byteplus_modelark
revision: "2026-08-01.1"
enabled: true
provider: {code: bytedance_seedance, name: ByteDance Seedance, model_owner: ByteDance Seed, serving_platform: BytePlus ModelArk, official_website: https://seed.bytedance.com/zh/seedance2_0}
sources:
  - {type: model_owner, title: ByteDance Seedance 2.0, url: https://seed.bytedance.com/zh/seedance2_0, checked_at: "2026-08-01", scope: Model positioning and multimodal reference capabilities.}
  - {type: api_reference, title: BytePlus ModelArk Seedance 2.0 API, url: https://docs.byteplus.com/en/docs/modelark/1520757, checked_at: "2026-08-01", scope: Stable model IDs, inputs, outputs, and asynchronous task protocol.}
models:
  - id: seedance-2.0
    provider_model_id: dreamina-seedance-2-0-260128
    display_name_i18n: {zh-CN: Dreamina Seedance 2.0, en-US: Dreamina Seedance 2.0}
    description_i18n: {zh-CN: Seedance 2.0 标准视频生成模型。, en-US: Standard Seedance 2.0 video generation model.}
    family: seedance-2.0
    variant: standard
    lifecycle: {status: active, available_since: "2026-01-28"}
    limits: {output_resolutions: [480p, 720p, 1080p, 4k], duration_seconds: [4, 15], maximum_reference_images: 9, maximum_reference_videos: 3, maximum_reference_audios: 3}
  - id: seedance-2.0-fast
    provider_model_id: dreamina-seedance-2-0-fast-260128
    display_name_i18n: {zh-CN: Dreamina Seedance 2.0 Fast, en-US: Dreamina Seedance 2.0 Fast}
    description_i18n: {zh-CN: 偏向速度的 Seedance 2.0 视频生成模型。, en-US: Speed-oriented Seedance 2.0 video generation model.}
    family: seedance-2.0
    variant: fast
    lifecycle: {status: active, available_since: "2026-01-28"}
    limits: {output_resolutions: [480p, 720p], duration_seconds: [4, 15], maximum_reference_images: 9, maximum_reference_videos: 3, maximum_reference_audios: 3}
operations:
  - id: text-to-video
    capability_definition_id: video.text_to_video
    name_i18n: {zh-CN: 文生视频, en-US: Text to Video}
    description_i18n: {zh-CN: 根据文本异步生成视频。, en-US: Asynchronously generate video from text.}
    execution_mode: asynchronous
    input_media_types: [text]
    output_media_types: [video, image, json]
    unsupported_parameters: [seed, camera_fixed, draft]
    input_schema: &text_input
      type: object
      additionalProperties: false
      required: [model, prompt]
      properties: &common_properties
        model: {type: string, enum: [seedance-2.0, seedance-2.0-fast]}
        prompt: {type: string, minLength: 1, maxLength: 10000}
        resolution: {type: string, enum: [480p, 720p, 1080p, 4k], default: 720p}
        ratio: {type: string, enum: ['16:9', '4:3', '1:1', '3:4', '9:16', '21:9', adaptive], default: adaptive}
        duration: {type: integer, enum: [-1, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15], default: 5}
        generate_audio: {type: boolean, default: true}
        watermark: {type: boolean, default: false}
        callback_url: {type: string, format: uri}
        return_last_frame: {type: boolean, default: false}
        priority: {type: integer, minimum: 0, maximum: 9, default: 0}
        safety_identifier: {type: string, minLength: 1, maxLength: 64}
    output_schema: &video_output
      type: object
      additionalProperties: true
      required: [id, status, video_url]
      properties:
        id: {type: string}
        status: {type: string}
        video_url: {type: string, format: uri}
        last_frame_url: {type: [string, 'null'], format: uri}
        resolution: {type: string}
        ratio: {type: string}
        duration: {type: integer}
        generate_audio: {type: boolean}
        usage: {type: object, additionalProperties: true}
  - id: image-to-video
    capability_definition_id: video.image_to_video
    name_i18n: {zh-CN: 图生视频, en-US: Image to Video}
    description_i18n: {zh-CN: 根据文本和图片异步生成视频。, en-US: Asynchronously generate video from text and images.}
    execution_mode: asynchronous
    input_media_types: [text, image]
    output_media_types: [video, image, json]
    unsupported_parameters: [seed, camera_fixed, draft]
    input_schema:
      type: object
      additionalProperties: false
      required: [model, input_images]
      properties:
        <<: *common_properties
        input_images: {type: array, minItems: 1, maxItems: 2, items: {type: string, format: asset_image}, x-omnimam-connectable: true}
    output_schema: *video_output
  - id: reference-to-video
    capability_definition_id: video.video_edit
    name_i18n: {zh-CN: 参考生成视频, en-US: Reference to Video}
    description_i18n: {zh-CN: 使用图片、视频或音频参考生成和编辑视频。, en-US: Generate and edit video from image, video, or audio references.}
    execution_mode: asynchronous
    input_media_types: [text, image, video, audio]
    output_media_types: [video, image, json]
    unsupported_parameters: [seed, camera_fixed, draft]
    input_schema:
      type: object
      additionalProperties: false
      required: [model]
      properties:
        <<: *common_properties
        reference_images: {type: array, minItems: 1, maxItems: 9, items: {type: string, format: asset_image}, x-omnimam-connectable: true}
        reference_videos: {type: array, minItems: 1, maxItems: 3, items: {type: string, format: asset_video}, x-omnimam-connectable: true}
        reference_audios: {type: array, minItems: 1, maxItems: 3, items: {type: string, format: asset_audio}, x-omnimam-connectable: true}
      anyOf: [{required: [reference_images]}, {required: [reference_videos]}]
    output_schema: *video_output
variants:
  - {id: standard-text-to-video, model_id: seedance-2.0, operation_id: text-to-video, lifecycle: {status: active}, input_schema: {type: object, properties: {model: {const: seedance-2.0}, resolution: {enum: [480p, 720p, 1080p, 4k]}}}, output_schema: {type: object}}
  - {id: fast-text-to-video, model_id: seedance-2.0-fast, operation_id: text-to-video, lifecycle: {status: active}, input_schema: {type: object, properties: {model: {const: seedance-2.0-fast}, resolution: {enum: [480p, 720p]}}}, output_schema: {type: object}}
  - {id: standard-image-to-video, model_id: seedance-2.0, operation_id: image-to-video, lifecycle: {status: active}, input_schema: {type: object, properties: {model: {const: seedance-2.0}, resolution: {enum: [480p, 720p, 1080p, 4k]}}}, output_schema: {type: object}}
  - {id: fast-image-to-video, model_id: seedance-2.0-fast, operation_id: image-to-video, lifecycle: {status: active}, input_schema: {type: object, properties: {model: {const: seedance-2.0-fast}, resolution: {enum: [480p, 720p]}}}, output_schema: {type: object}}
  - {id: standard-reference-to-video, model_id: seedance-2.0, operation_id: reference-to-video, lifecycle: {status: active}, input_schema: {type: object, properties: {model: {const: seedance-2.0}, resolution: {enum: [480p, 720p, 1080p, 4k]}}}, output_schema: {type: object}}
  - {id: fast-reference-to-video, model_id: seedance-2.0-fast, operation_id: reference-to-video, lifecycle: {status: active}, input_schema: {type: object, properties: {model: {const: seedance-2.0-fast}, resolution: {enum: [480p, 720p]}}}, output_schema: {type: object}}
notes:
  - Seedance 2.0 Mini is excluded because no verified stable API model ID is registered.
  - Media byte size, dimensions, and duration are revalidated by provider and artifact boundaries.
`
