# Project Handoff

## Current project goal

Implement released `spec-v1.7.11` TaskSchedule manual runs across API, persistence, WorkflowRuntime, Worker recovery, and Web.

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
- `docs/HANDOFF.md`

## Key architectural decisions

- Manual runs use a fixed controller definition and never execute schedule work in the HTTP goroutine or call Conductor scheduler run-now.
- API idempotency is `(schedule_id, idempotency_key)`; runtime and target idempotency derive from ScheduleExecution ID.
- Manual and scheduled triggers share the same active-execution lock and overlap history semantics.
- PAUSED manual runs do not resume the schedule or change cron, runAt, next trigger, or future scheduled executions.
- Runtime start failure is returned as a persisted `TRIGGER_FAILED` ScheduleExecution so the caller still receives the accepted execution fact.

## API, schema, and configuration changes

- New endpoint: `POST /api/v1/task-schedules/{task_schedule_id}/run`.
- New request: `{ "idempotency_key": "<uuid>" }`.
- ScheduleExecution now returns `trigger_source`, `triggered_by`, and optional `idempotency_key`.
- Database migration adds those columns and unique partial index `idx_schedule_execution_manual_key`.
- `task_schedule_execution_recorded` includes trigger source and actor.
- No new error code, permission code, binary, dependency, or runtime configuration was introduced.

## Verification

- Focused manual schedule, store, controller, and API Server compile tests passed.
- Scoped `go vet` passed for API types, Task Center service/store/controller, and API Server.
- Full backend test run is green except the two pre-existing thumbnail localization assertions below.
- Live RECONCILE manual execution reached `SUCCESS` with runtime binding and summary.
- Same idempotency key returned the same ScheduleExecution ID.
- Concurrent schedule activity returned a persisted `SKIPPED_OVERLAP` without runtime start.
- PAUSED manual execution left status PAUSED and future configuration unchanged.
- Temporary MATERIALIZED verification created an AtomicTask target and returned its readable target summary; the temporary schedule was soft-deleted afterward.

## Remaining work

- Add PostgreSQL concurrency integration coverage if a dedicated CI database is introduced for this path.

## Known issues and risks

- Full backend tests still fail only in `taskcenter.TestAssignSystemName` and `taskname.TestResolve`: expected `生成 thumbnail 表现形式`, actual `生成 thumbnail视图`.
- The local API Server and TaskWorker currently use `manualrun-20260728-amd64`; recreating Compose without that tag will switch images.
- The live MATERIALIZED verification target used a deliberately synthetic asset ID and may finish failed; its schedule has been soft-deleted.

## Recommended next task

Add a real-browser end-to-end test that runs a paused user MATERIALIZED schedule and verifies navigation to the returned execution and target.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
