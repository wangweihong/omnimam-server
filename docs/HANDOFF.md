# Project Handoff

## Current project goal

Complete and prepare the released `spec-v1.3.0` Task Center RECONCILE and Application Platform EngineInstance health implementation for staging/commit.

## Completed in this session

- Verified `SSOT_VERSION.commit` and the checked-out `ssot` submodule both point to released commit `34e1994c9170553f3bcceb7b01b9d640a3698361`.
- Rebuilt and ran the isolated `omnimam-reconcile-verify` stack with PostgreSQL metadata/history and AOF-enabled Redis Conductor queues.
- Fixed Conductor 3.31 startup: `management.health.redis.enabled=true` incorrectly loads the redis-lock health indicator without a RedissonClient. Redis remains independently covered by the Compose healthcheck.
- Proved 30-second `:00/:30` cadence, `runCatchupScheduleInstances=false`, one unique SYSTEM schedule, fixed controller version 1, enabled EngineInstance scanning, persisted admin parameters, bounded history, and no materialized health tasks.
- Created one controlled enabled ComfyUI EngineInstance at an unreachable local endpoint. The first check persisted `offline` with safe summary `provider is unavailable` and atomically wrote one `engine_instance_health_changed` outbox row; later unchanged checks did not duplicate the event.
- Updated the SYSTEM schedule to `maxParallelism=7`, `maxItemsPerRun=17`, `perItemTimeoutSeconds=2`, and `overallTimeoutSeconds=6`; API and Worker restarts did not restore defaults.
- Completed controlled Worker, API Server, Conductor, PostgreSQL, and Redis restart tests. The schedule, parameters, Engine health state, cumulative reconcile state, Redis AOF queue, and outbox survived.
- Runtime testing exposed that Redis AOF restores an already-enqueued overdue scheduler message even when Conductor catch-up generation is disabled. Added a Task Center misfire guard with a 5-second jitter allowance: a first late trigger is skipped without business history, while an existing `scheduleId + scheduledAt` continues through the recovery path.
- Rebuilt the images and proved the fix: Conductor missed the 16:51:30 UTC tick, recovered at 16:51:47, did not backfill 16:51:30, and executed the next 16:52:00 tick normally.
- Added explicit `integration` build isolation and PostgreSQL coverage for lookup-by-schedule-time, overlap, and retention.

## Files added or modified in this session

- Added/updated Task Center misfire behavior and tests in `backend/internal/apiserver/service/v1/taskcenter/reconcile.go` and `reconcile_test.go`.
- Added `GetScheduleExecutionAt` to the store contract and PostgreSQL adapter; used it in RECONCILE and MATERIALIZED trigger paths.
- Updated `backend/internal/apiserver/store/postgresql/task_center_reconcile_integration_test.go` with an integration build tag and lookup coverage.
- Updated `deployments/conductor/config-postgres.properties` and `deployments/README.md` for the Redis queue/health configuration.
- Refreshed this handoff. The broader uncommitted `spec-v1.3.0` implementation remains across Task Center, Application Platform, runtime, schema migration, generated errors, deployment files, `SSOT_VERSION`, and the `ssot` pointer.

## Key architectural and design decisions

- PostgreSQL remains the business fact store, outbox, Conductor metadata store, and workflow history store; Redis is only the low-latency persistent Conductor queue.
- Task Center, not Conductor queue recovery, enforces final V1 no-catch-up semantics.
- Misfire detection allows 5 seconds of ordinary scheduler/worker jitter. It skips only when no business execution exists; existing executions remain recoverable after Worker failure or redelivery.
- Engine health changes update EngineInstance and write the status-change outbox message in one PostgreSQL transaction. Unchanged health only advances check metadata.

## API, schema, and configuration changes present

- TaskSchedule supports MATERIALIZED/RECONCILE and USER/SYSTEM modes, reconcile limits/config, history retention, reconcile summaries, and ScheduleReconcileState.
- Added `GET /api/v1/task-schedules/{task_schedule_id}/reconcile-state` and RECONCILE list/update fields.
- Added Task Center PostgreSQL columns, constraints, indexes, and `task_schedule_reconcile_states`; no new schema was required for the misfire fix.
- Conductor uses `conductor.queue.type=redis_standalone`; Redis runs with AOF/everysec. The incompatible Conductor redis-lock health indicator is disabled.

## Verification results

- Passed focused unit tests, `go vet`, and race tests for Task Center, Application Platform, PostgreSQL store, and WorkflowRuntime.
- Passed real PostgreSQL integration test with `-tags=integration`: lookup by schedule time, overlap, and retention.
- `git diff --check` passed.
- `go test ./...` remains red only in unrelated baseline areas: `backend/apis/imachinery`, `backend/pkg/validator`, `backend/pkg/grpccli`, `backend/pkg/grpcsvr`, `third_party/k8s.io/utils/exec`, and `tools/ssot-s1-live` (missing `skills/ssot-product-workflow/SKILL.md`).

## Outstanding tasks

1. Decide whether to fix the unrelated full-suite baseline failures now or track them separately.
2. Review and stage the released SSOT submodule pointer plus the complete implementation. The parent index still differs from working submodule commit `34e1994...`.
3. Commit/push only after reviewing the full uncommitted feature diff; no files are staged or committed.
4. Remove the controlled verification EngineInstance or tear down the isolated stack when it is no longer needed. The stack is intentionally still running for inspection.

## Known issues and risks

- The 5-second misfire allowance is an internal implementation constant because SSOT defines SKIP semantics but no configurable grace period. Changing it should remain an implementation-only decision unless exposed as a contract field.
- The verification database contains the fixture `reconcile-verification-offline`; it is isolated under the `omnimam-reconcile-verify` Compose project.
- The complete feature and SSOT pointer remain unstaged/uncommitted.

## Recommended next task

Review the complete diff, isolate or fix the unrelated baseline failures, then stage the released SSOT pointer and `spec-v1.3.0` implementation as one coherent change.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
