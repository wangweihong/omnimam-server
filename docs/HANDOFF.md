# Project Handoff

## Current project goal

Keep the server aligned with released `spec-v1.7.10` and make `application-platform.engine-health` persist provider timeouts while exposing the corresponding EngineInstance in TaskSchedule execution summaries.

## Completed in this session

1. Diagnosed schedule `3c88b0d3-4d01-4baf-bc98-b323869285b5`: ComfyUI succeeds, while BytePlus ModelArk instance `ac2b1df1-6f66-4d6c-a3ee-74223a2e45fd` points to `http://10.30.12.123:888`, which accepts TCP but returns no HTTP bytes before timeout.
2. Fixed the context lifecycle defect. A provider deadline is classified as an `offline` health fact and now receives a detached one-second persistence context; explicit cancellation still returns without saving so the chunk remains retryable.
3. Added a bounded `engine_instances` array to the existing `last_execution.reconcile_summary.summary` extension point. Each item includes ID, name, engine type, enabled state, previous/current health status, checked time, safe failure summary, changed flag, and deferred flag.
4. Added `engine_instances_total` and `engine_instances_truncated`. At most 20 instances are retained, ordered by deferred first, health changes second, then stable ID, so actionable instances survive truncation.
5. Added regression coverage for a genuinely expired per-item context, offline persistence with a fresh context, explicit cancellation, JSON response shape, incomplete chunks, summary bounds, and deferred-instance priority.
6. Built local images `omnimam/apiserver:healthfix-20260728-amd64` and `omnimam/taskworker:healthfix-20260728-amd64`, then recreated only those two Compose services.
7. Verified the live `00:10:30Z` reconcile execution completed `SUCCESS` with `scanned=2`, `deferred=0`, and `cycle_completed=true`; consecutive failures reset to zero.
8. Verified the BytePlus instance is now persisted as `offline` with `last_health_check_at` and safe reason `provider request timed out`, and the schedule endpoint returns both EngineInstance summaries.

## Files modified

- `backend/apis/iapiserver/meta_task_center.go`
- `backend/internal/apiserver/service/v1/applicationplatform/application_platform.go`
- `backend/internal/apiserver/service/v1/applicationplatform/engine_health_reconcile.go`
- `backend/internal/apiserver/service/v1/applicationplatform/engine_health_reconcile_test.go`
- `backend/internal/apiserver/service/v1/applicationplatform/engine_health_test.go`
- `docs/HANDOFF.md`

No files were added or removed.

## Key architectural decisions

- Network timeout is a completed health observation and must be saved as `offline`; only explicit worker cancellation leaves the item retryable.
- Provider probing and health-fact persistence use separate context budgets after a probe deadline.
- Engine details use the existing OpenAPI `ReconcileSummary.summary` object instead of adding an uncontracted top-level API field.
- The summary is bounded and contains no base URL, auth config, credentials, raw upstream payload, or unbounded per-item data.
- Checkpoint advancement still requires every item in the chunk to save successfully.

## API, schema, and configuration changes

- `GET /api/v1/task-schedules/{id}` and the execution history endpoint now return `reconcile_summary.summary.engine_instances`, `engine_instances_total`, and `engine_instances_truncated` for new engine-health executions.
- No top-level API field, database schema, migration, permission, error code, event type, dependency, or runtime configuration changed.
- Existing historical executions are not backfilled.

## Verification

- Focused deadline and reconcile regression tests passed.
- Full Application Platform service tests passed.
- Application Platform race tests passed.
- Scoped `go vet` passed for Application Platform, Task Center, and API packages.
- `git diff --check` passed.
- `go test ./backend/...` passed all changed and dependent packages. The run remains red only in the two pre-existing thumbnail localization assertions described below.
- Live API and TaskWorker verification passed against the existing PostgreSQL and Conductor services.

## Remaining work

1. Correct or disable `http://10.30.12.123:888`; the code fix records its failure correctly but cannot make the upstream endpoint responsive.
2. Resolve the existing thumbnail localization mismatch.
3. Commit changes only when explicitly requested.

## Known issues and risks

- The running API Server and TaskWorker use the local uncommitted `healthfix-20260728-amd64` images; recreating Compose without `OMNIMAM_IMAGE_TAG=healthfix-20260728-amd64` may switch them back to another tag.
- Full backend tests still fail in `taskcenter.TestAssignSystemName` and `taskname.TestResolve`: expected `生成 thumbnail 表现形式`, actual `生成 thumbnail视图`. This is unrelated to engine health.
- A persistence operation taking longer than the detached one-second budget remains deferred, preserving checkpoint safety.

## Recommended next task

Correct the stalled BytePlus endpoint, then resolve the existing thumbnail localization mismatch and commit the verified server changes when requested.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
