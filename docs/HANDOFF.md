# Project Handoff

## Current project goal

Maintain the released `spec-v1.7.11` implementation and verify its API coverage, most recently the Asset Library Collection rename path.

## Completed in this session

1. Updated the server SSOT pin to released `spec-v1.7.11`.
2. Added `POST /api/v1/task-schedules/{task_schedule_id}/run` with required UUID idempotency key and full ScheduleExecution response.
3. Added ScheduleExecution `trigger_source`, `triggered_by`, and `idempotency_key`, plus the partial manual-idempotency unique index and event payload fields.
4. Added fixed `task_center_manual_schedule_controller` workflow and `task.schedule.manual` Worker handler. HTTP only persists and starts the controller; MATERIALIZED/RECONCILE work remains asynchronous.
5. Reused the parent-schedule row lock and active-execution rule. Duplicate keys return one record; overlap records `SKIPPED_OVERLAP`; ACTIVE and PAUSED may run; protected SYSTEM schedules require system admin.
6. Added stable target idempotency from ScheduleExecution ID for AtomicTask, TaskGroup, and DAG targets, and recovery for accepted manual executions without a live runtime binding.
7. Added service tests for duplicate requests, paused-state preservation, fixed controller identity, and persisted runtime-start failure.
8. Built and deployed `omnimam/apiserver:manualrun-20260728-amd64` and `omnimam/taskworker:manualrun-20260728-amd64` to the local Compose services.
9. Live-verified RECONCILE success, duplicate-key identity, overlap skip, PAUSED execution without future-schedule mutation, and MATERIALIZED target creation with target summary.
10. Committed the implementation as `1004d4e` (`feat(task-center): run schedules on demand`).
11. Verified the existing execution API already exposes per-engine health outcomes and `failure_summary` inside `reconcile_summary.summary.engine_instances`; no backend or SSOT change was required.
12. Updated the Web schedule detail to render those outcomes as a structured instance table and to focus details by `execution_id`; Web implementation commit is `4df900e`.
13. Deployed `frontend:4df900e` and live-verified the failed ModelArk check displays `provider request timed out`.
14. Verified that Collection rename is available through `PATCH /api/v1/collections/{collection_id}` with `UpdateCollectionRequest.name`; the released OpenAPI, route, controller, service, and PostgreSQL store are wired end to end.
15. Confirmed the update path trims the new name, enforces per-user name uniqueness, supports optional `resource_version`, and maps name/version conflicts to released business errors.
16. Aligned `UpdateCollectionRequest` with the released OpenAPI: empty update objects and trim-empty names are now rejected before the controller action runs; focused DTO and real decode-pipeline tests cover the behavior.

## Files added or modified

- `SSOT_VERSION`
- `ssot` submodule pin
- `backend/apis/iapiserver/meta_task_center.go`
- `backend/apis/iapiserver/request_task_center.go`
- `backend/internal/apiserver/controller/v1/taskcenter/task_center.go`
- `backend/internal/apiserver/route.go`
- `backend/internal/apiserver/service/v1/taskcenter/task_center.go`
- `backend/internal/apiserver/service/v1/taskcenter/reconcile.go`
- `backend/internal/apiserver/service/v1/taskcenter/reconciler.go`
- `backend/internal/apiserver/service/v1/taskcenter/manual_schedule_test.go`
- `backend/internal/apiserver/store/postgresql/0_pg.go`
- `backend/internal/apiserver/store/postgresql/task_center.go`
- `backend/internal/apiserver/taskworker.go`
- `backend/apis/iapiserver/request_asset_v1.go`
- `backend/apis/iapiserver/request_asset_v1_test.go`
- `docs/HANDOFF.md`
- Web-only follow-up files are recorded in `/home/wwhvw/codespace/omnimam-web/docs/HANDOFF.md`.

## Key architectural decisions

- Manual runs use a fixed controller definition and never execute schedule work in the HTTP goroutine or call Conductor scheduler run-now.
- API idempotency is `(schedule_id, idempotency_key)`; runtime and target idempotency derive from ScheduleExecution ID.
- Manual and scheduled triggers share the same active-execution lock and overlap history semantics.
- PAUSED manual runs do not resume the schedule or change cron, runAt, next trigger, or future scheduled executions.
- Runtime start failure is returned as a persisted `TRIGGER_FAILED` ScheduleExecution so the caller still receives the accepted execution fact.
- Per-instance engine diagnostics continue using the released generic reconcile-summary contract; Task Center API, schema, and persistence were not expanded for the Web presentation fix.
- Collection update presence and trim-empty checks live in the request DTO `Validate()` path, so invalid PATCH bodies are rejected by `core.DecodeParameter` before service/store side effects.

## API, schema, and configuration changes

- New endpoint: `POST /api/v1/task-schedules/{task_schedule_id}/run`.
- New request: `{ "idempotency_key": "<uuid>" }`.
- ScheduleExecution now returns `trigger_source`, `triggered_by`, and optional `idempotency_key`.
- Database migration adds those columns and unique partial index `idx_schedule_execution_manual_key`.
- `task_schedule_execution_recorded` includes trigger source and actor.
- No new error code, permission code, binary, dependency, or runtime configuration was introduced.
- No backend contract change was made for instance diagnostics because `engine_instances`, health state, and `failure_summary` are already present in the response.
- No API, schema, configuration, or business-code change was needed for Collection rename; the capability already exists in released `spec-v1.7.11`.
- No SSOT change was required for Collection validation; the backend now enforces existing `UpdateCollectionRequest.minProperties: 1` and `name.minLength: 1` semantics, including trim-empty rejection.

## Verification

- Focused manual schedule, store, controller, and API Server compile tests passed.
- Scoped `go vet` passed for API types, Task Center service/store/controller, and API Server.
- Full backend test run is green except the two pre-existing thumbnail localization assertions below.
- Live RECONCILE manual execution reached `SUCCESS` with runtime binding and summary.
- Same idempotency key returned the same ScheduleExecution ID.
- Concurrent schedule activity returned a persisted `SKIPPED_OVERLAP` without runtime start.
- PAUSED manual execution left status PAUSED and future configuration unchanged.
- Temporary MATERIALIZED verification created an AtomicTask target and returned its readable target summary; the temporary schedule was soft-deleted afterward.
- Live execution `8486f2c4-37d5-47e1-b891-b97fe8b22012` returned two engine results; the frontend shows success for ComfyUI and the timeout failure reason for ModelArk.
- `go test ./backend/internal/apiserver -run '^TestAssetLibraryRoutesCoverReleasedOpenAPI$' -count=1` passed and confirms the Collection PATCH route is installed.
- Collection DTO tests verify empty objects and trim-empty names fail while valid partial updates pass; decode-pipeline tests verify the same behavior through `core.DecodeParameter`.
- `go test ./backend/apis/iapiserver ./backend/pkg/core -count=1`, the full asset-library service package, the released route-contract test, and scoped `go vet` all passed.

## Remaining work

- Add PostgreSQL concurrency integration coverage if a dedicated CI database is introduced for this path.
- Add PostgreSQL-backed Collection update tests for rename persistence, trimmed-name conflicts, owner isolation, and stale `resource_version` handling.

## Known issues and risks

- Full backend tests still fail only in `taskcenter.TestAssignSystemName` and `taskname.TestResolve`: expected `生成 thumbnail 表现形式`, actual `生成 thumbnail视图`.
- The local API Server and TaskWorker currently use `manualrun-20260728-amd64`; recreating Compose without that tag will switch images.
- The live MATERIALIZED verification target used a deliberately synthetic asset ID and may finish failed; its schedule has been soft-deleted.
- Request and decode validation are covered, but no PostgreSQL-backed test currently exercises rename persistence or name/version conflict mappings.

## Recommended next task

Add PostgreSQL-backed Collection update behavior tests, then continue real-browser coverage for schedule runs and per-instance RECONCILE diagnostics.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
