# Project Handoff

## Current project goal

Keep the server aligned with released `spec-v1.7.10`, including the ComfyUI workflow conversion boundary, frontend permissions, and correct EngineInstance duplicate-name errors.

## Completed in this session

1. Published upstream `omnimam-spec` release `spec-v1.7.7` and pushed `master` plus the release tag.
2. Pinned the server `ssot` submodule and `SSOT_VERSION` to release commit `7a78d017023538cb810a2299679b5cfb2fa3731e`.
3. Removed `source_engine_instance_id` from workflow import, list filters, public workflow projections, persistence models, store queries, and engine deletion reference checks.
4. Changed Visual Workflow import to persist only the source canvas with `api_conversion_status=pending`; import no longer reads object_info or calls comfy2go.
5. Kept API Workflow import self-contained: server-side source detection and basic `class_type`/`inputs` structure validation produce a ready API snapshot and RFC 8785 checksum.
6. Changed explicit Visual-to-API conversion to require `engine_instance_id` plus `resource_version`; conversion validates ComfyUI type, enabled/online state, and a non-stale current object_info before calling the injected parser.
7. Added an injectable `ComfyWorkflowParser` dependency so import/conversion boundaries and parser failures are directly testable.
8. Added a one-time destructive Application Platform reset keyed by the legacy workflow source column. It serializes multi-replica startup with an advisory lock, cleans linked Task Center, Artifact registration, SSE, runtime projection, and outbox records, then drops Application Platform tables for AutoMigrate to rebuild.
9. Preserved unrelated Task Center records, user assets, and blobs. The reset is idempotent and does not run after the legacy column disappears.
10. Retained the prior architecture decision that workflow test runs use their dedicated `comfyui.submit -> comfyui.poll -> comfyui.collect_preview` Task Center DAG rather than the application-run `OperationExecutor` path.
11. Pinned the current worktree to released `spec-v1.7.9` commit `f4241befda82bccfe8de22846dc65ff1c00885c4`.
12. Replaced obsolete default `canvas.read`, `canvas.write`, and `canvas.execute` keys with the released Workflow Canvas permissions for node definitions, canvases, and runs.
13. Added the released `asset.artifact.delete` permission required by the Web artifact actions.
14. Expanded `/api/v1/me` regression coverage across Asset Library, Application Platform, Workflow Canvas, Task Center, and SSE frontend permissions, including an assertion that obsolete Canvas keys are not returned.
15. Published upstream `omnimam-spec` release `spec-v1.7.10` with `ERR_AIAPP_ENGINE_INSTANCE_NAME_DUPLICATED` (`130429`) and pushed `master` plus the release tag.
16. Pinned the server to `spec-v1.7.10`, generated the error registry/documentation, and mapped EngineInstance create/update name uniqueness conflicts to the dedicated error instead of `ERR_AIAPP_ENGINE_AUTH_CONFIG_INVALID`.

## Files modified

- `SSOT_VERSION`
- `ssot` submodule pointer
- `backend/apis/iapiserver/meta_comfyui_workflow.go`
- `backend/apis/iapiserver/request_comfyui_workflow.go`
- `backend/internal/apiserver/controller/v1/applicationplatform/application_platform.go`
- `backend/internal/apiserver/controller/v1/applicationplatform/response.go`
- `backend/internal/apiserver/controller/v1/applicationplatform/response_test.go`
- `backend/internal/apiserver/service/v1/applicationplatform/application_platform.go`
- `backend/internal/apiserver/service/v1/applicationplatform/comfyui_workflow.go`
- `backend/internal/apiserver/service/v1/applicationplatform/comfyui_workflow_test.go`
- `backend/internal/apiserver/service/v1/platform/service.go`
- `backend/internal/apiserver/service/v1/platform/service_test.go`
- `backend/internal/apiserver/store/postgresql/0_pg.go`
- `backend/internal/apiserver/store/postgresql/application_platform.go`
- `backend/internal/apiserver/store/postgresql/application_platform_test.go`
- `backend/internal/apiserver/store/postgresql/comfyui_workflow.go`
- `backend/internal/pkg/code/base.go`
- `backend/internal/pkg/code/code_generated.go`
- `docs/HANDOFF.md`
- `docs/guide/zh-CN/api/error_code_generated.md`

Added: `backend/internal/apiserver/store/postgresql/application_platform_reset_integration_test.go`.

No files were removed.

## Key architectural decisions

- Workflow import is a file-ingestion boundary, not an engine compatibility check. Engine/object_info facts begin at Visual conversion and remain required for derived nodes, compatibility validation, template publication, and execution.
- The conversion EngineInstance is an operation input and is not persisted on `ComfyUIWorkflow`.
- Existing Application Platform data is intentionally discarded during this breaking schema transition; no legacy data conversion or response compatibility field is provided.
- Cross-domain cleanup is scoped by captured Application Platform, application-run, test-run DAG, task, attempt, and artifact IDs so unrelated domain data survives.
- Task Center remains the execution-state source for workflow test runs.
- `/api/v1/me` exposes released permission identifiers as the frontend capability source. Legacy `canvas.*` aliases are removed instead of being returned alongside `workflow.*` permissions.
- `workflow.projection.internal` remains excluded because it is reserved for the Workflow Canvas service identity, not the interactive system administrator.
- Duplicate EngineInstance names use the dedicated engine-module business error `ERR_AIAPP_ENGINE_INSTANCE_NAME_DUPLICATED`; the existing database unique index remains the concurrency-safe source of enforcement.

## API, schema, and configuration changes

- The server submodule and `SSOT_VERSION` target released `spec-v1.7.10` commit `d29a4248b08a77e7f13f1546e377638cbed6ef98`.
- `ComfyUIWorkflowImportRequest`, workflow responses, and list filters no longer contain `source_engine_instance_id`.
- `ComfyUIWorkflowAPIConversionRequest` requires `engine_instance_id` and `resource_version`.
- `aiapp_comfyui_workflows.source_engine_instance_id` and its foreign key are removed after the destructive reset.
- Added business error `ERR_AIAPP_ENGINE_INSTANCE_NAME_DUPLICATED` (`130429`, HTTP 200, non-retryable). No endpoint, schema, permission, event, dependency, or runtime configuration changed.

## Verification

- Upstream OpenAPI YAML parsed successfully; `spec-v1.7.7` and its tag were pushed.
- Focused import, conversion, controller, API metadata, route-contract, and PostgreSQL tests passed.
- The destructive reset integration test passed against the local PostgreSQL 16 container, including repeat execution and preservation of unrelated records.
- Scoped `go test -race` passed for Application Platform service and PostgreSQL store packages.
- Scoped `go vet` passed.
- `go test ./backend/internal/apiserver/service/v1/platform` passed.
- `go test -race ./backend/internal/apiserver/service/v1/platform` passed.
- `gofmt` and `git diff --check` passed.
- `make gen` regenerated the error registry and error-code documentation.
- Focused Application Platform service/controller tests and scoped `go vet` passed after the `spec-v1.7.10` mapping change.
- A fresh `go test ./backend/...` run passed all permission-related packages but remains red in two unrelated localization assertions: `taskcenter.TestAssignSystemName` and `taskname.TestResolve` expect `生成 thumbnail 表现形式`, while the current catalog returns `生成 thumbnail视图`.

## Outstanding tasks

1. Update the Web client to remove EngineInstance selection from import and require it in the Visual-to-API action.
2. Deploy/restart the new API Server when destructive reset of the current Application Platform data is intended.
3. Perform one real Visual Workflow import followed by explicit conversion against a healthy ComfyUI instance.
4. Commit the server changes only when explicitly requested.
5. Resolve the existing Task Center task-name localization mismatch, then rerun `go test ./backend/...`.

## Known issues and risks

- Starting the new API Server against a database with the legacy source column permanently deletes Application Platform data and linked execution projections by design.
- Registered user assets and blobs are preserved after their Application Platform provenance projections are removed.
- The Web client remains on the old request shape until separately updated.
- The full backend suite is currently red only because of the pre-existing thumbnail localization expectation mismatch described above; the platform permission package and its race run pass.

## Recommended next task

Resolve the existing Task Center task-name localization mismatch, then rerun the full backend suite.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
