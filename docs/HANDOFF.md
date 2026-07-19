# Project Handoff

## Current project goal

Complete the backend alignment for released `spec-v1.4.0`, replacing persisted ComfyUI object-info snapshots with the EngineInstance one-to-one current catalog.

## Completed in this session

- Updated the `ssot` submodule to released tag `spec-v1.4.0` at `6f75b7dd187bb0d05e3e42fc2d720f026df47c85` and synchronized `SSOT_VERSION`.
- Added `aiapp_comfyui_engine_object_info`, a current-fact extension keyed by `engine_instance_id`, with atomic upsert and `ON DELETE CASCADE`.
- Added current object-info read and manual refresh APIs. Read supports gzip negotiation; refresh requires an enabled, online ComfyUI instance and preserves the last success on failure.
- Added database row-lock serialization shared by manual refresh, scheduled refresh, and template conversion revalidation.
- Added the `application-platform.comfyui-object-info-refresh` SYSTEM RECONCILE handler and the unique daily `0 0 3 * * *` UTC schedule.
- Added EngineInstance list summary fields for object-info availability, refresh time, and derived 48-hour stale state.
- Removed Workflow lifecycle/archive/restore behavior, persisted parse caches, Validation object-info snapshots/checksums, TemplateVersion object-info/dependency snapshots, and WorkflowTestRun object-info snapshots.
- Changed nodes, input-candidates, output-candidates, and dependencies to require `engine_instance_id` and derive results from that instance's current usable catalog.
- Changed import, API conversion, validation, template conversion/publish, RuntimeForm resolution, test runs, ApplicationRun creation, and Worker execution to revalidate against current object-info where applicable.
- Regenerated application-platform error code source and documentation for errors `131243` and `131244`.
- Updated TaskWorker architecture documentation to describe the two SYSTEM RECONCILE paths.

## Files added or modified

- SSOT pin: `ssot`, `SSOT_VERSION`.
- API/meta/request models: `backend/apis/iapiserver/meta_application_platform.go`, `meta_comfyui_workflow.go`, `request_application_platform.go`, and `request_comfyui_workflow.go`.
- Service/runtime: application-platform object-info, workflow, test-run, runtime-form, executor, adapters, and reconcile files under `backend/internal/apiserver/service/v1/applicationplatform/`.
- Store/schema: `backend/internal/apiserver/store/store.go` and PostgreSQL application-platform/schema files.
- HTTP/bootstrap: application-platform controller, response mapping, routes, API Server bootstrap, and TaskWorker bootstrap.
- Generated errors and docs: `backend/internal/pkg/code/*`, `docs/guide/zh-CN/api/error_code_generated.md`.
- Architecture and handoff docs under `docs/guide/zh-CN/architecture/` and `docs/HANDOFF.md`.
- Added focused unit, race-safe reconcile, gzip, forbidden-field, and PostgreSQL integration tests.

## Key architectural decisions

- PostgreSQL stores exactly one current object-info body per ComfyUI EngineInstance. It has no ObjectMeta, checksum, resource version, status machine, or history because it is an EngineInstance fact extension.
- Refresh obtains an EngineInstance row lock before the provider call and retains it through atomic upsert. Conversion uses the same lock while revalidating and committing the first template version.
- Stale is derived at read time from `refreshed_at`; catalogs older than 48 hours remain readable for diagnosis but are rejected by all execution-capable paths.
- Workflow parse results are request-local only. Template revision covers API Workflow plus template contract, never object-info or derived dependencies.
- Per user direction, this work targets a fresh v1.4.0 schema. No legacy data backfill, dual read/write, or in-place compatibility migration was added.

## API, schema, and configuration changes

- Added `GET /api/v1/engine-instances/{engine_instance_id}/object-info`.
- Added `POST /api/v1/engine-instances/{engine_instance_id}/object-info/refresh`.
- Removed ComfyUI Workflow archive and restore routes.
- Workflow derive routes now require query `engine_instance_id`.
- Added EngineInstance summary fields `object_info_available`, `object_info_refreshed_at`, and `object_info_stale`.
- Added errors `ERR_AIAPP_COMFYUI_OBJECT_INFO_REFRESH_NOT_ALLOWED` and `ERR_AIAPP_COMFYUI_OBJECT_INFO_REFRESH_FAILED`.
- Added daily SYSTEM RECONCILE schedule `application-platform.comfyui-object-info-refresh` at `03:00 UTC`.

## Verification results

- Passed focused application-platform model, service, controller, and PostgreSQL store tests.
- Passed `go test ./backend/internal/apiserver/...`.
- Passed focused race tests for application-platform service, controller, and PostgreSQL store.
- Passed `go vet ./backend/apis/iapiserver ./backend/internal/apiserver/...`.
- Passed real PostgreSQL 16 integration coverage for current-catalog upsert, failure retention, and cascade deletion.
- Passed `git diff --check`, exact SSOT tag verification, and `SSOT_VERSION.commit` equality.
- Full `go test ./backend/...` still fails only in unrelated baseline packages: `backend/apis/imachinery`, `backend/pkg/validator`, `backend/pkg/grpccli`, and `backend/pkg/grpcsvr`.

## Outstanding tasks

1. Review the complete v1.4.0 diff and stage/commit it when ready.
2. Decide separately whether to repair the unrelated full-suite baseline failures.
3. Run deployment-level ComfyUI verification against a reachable instance if one is available, including version extraction and a real daily reconcile execution.

## Known issues and risks

- Existing pre-v1.4.0 databases are not upgraded or backfilled by design; use a fresh schema or a separately reviewed destructive reset.
- ComfyUI version is read best-effort from `/system_stats`; a valid object-info refresh can succeed with an empty version when the upstream omits it.
- No reachable ComfyUI instance was available in this session, so provider-level object-info payload compatibility was covered with contract-shaped fixtures rather than a live endpoint.

## Recommended next task

Review and stage the complete `spec-v1.4.0` backend change, then perform a live ComfyUI smoke test before deployment.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
