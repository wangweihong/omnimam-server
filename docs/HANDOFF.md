# Project Handoff

## Current project goal

Implement the released `spec-v1.5.0` SSE user event stream for Task Center while preserving Task Center as the business fact source and Conductor as the internal runtime.

## Completed in this session

1. Updated the `ssot` submodule to remote released `spec-v1.5.0` marker commit `802d663278d52909cb9b4e1ed65008ce9794094a` and synchronized `SSOT_VERSION`.
2. Added `sse_user_events`, the current-user event envelope DTOs, history filters, sync state, cursor validation, retention metadata, and idempotent PostgreSQL store.
3. Added `GET /api/v1/events/stream`, `GET /api/v1/events`, and `GET /api/v1/events/sync-state`, matching the SSE OpenAPI paths.
4. Implemented SSE framing with `id/event/retry/data`, `connection.ready`, `connection.resync_required`, heartbeat comments, `connection.server_draining`, Last-Event-ID/after_event_id conflict checks, 5-second write deadlines, and per-user/per-client-instance connection limits.
5. Added Task Center reliable outbox events for AtomicTask creation/status/progress, TaskAttempt lifecycle, and TaskGroup/DAGTaskGroup creation/status/summary changes.
6. Added a TaskWorker-owned SSE projector that consumes Task Center outbox events, validates owner/version/source fields, removes internal routing fields, and idempotently writes UserEvent records. Projector failure Nacks the event and does not roll back task facts.
7. Updated runtime projection so first-seen TaskAttempts persist and publish `SCHEDULED` before observed RUNNING/terminal states. Group/DAG aggregate recalculation now increments `resource_version` and emits reliable events.
8. Added configurable SSE retention, poll interval, heartbeat interval, and per-user connection limit. API Server periodically removes expired UserEvent rows without changing business facts.
9. Generated all `170200-170800` SSE business errors and refreshed generated error documentation.
10. Updated TaskWorker/API Server architecture documentation for the Task Center outbox -> SSE projector -> UserEvent -> SSE gateway flow.

## Files modified or added

- SSOT pin: `ssot`, `SSOT_VERSION`.
- SSE API/meta: `backend/apis/iapiserver/meta_sse.go`, `request_sse.go`.
- SSE gateway/service/projector: `backend/internal/apiserver/controller/v1/sse/`, `backend/internal/apiserver/service/v1/sse/`.
- Store/schema initialization: `backend/internal/apiserver/store/{factory.go,store.go}` and PostgreSQL `0_pg.go`, `sse.go`, `outbox.go`, `task_center.go`, `task_center_events.go`.
- Bootstrap/config/routes: `options/options.go`, `route.go`, `server.go`, `taskworker.go`.
- Error code source/generated docs: `backend/internal/pkg/code/`, `docs/guide/zh-CN/api/error_code_generated.md`.
- Tests: SSE controller/service/projector tests, Task Center event mapping tests, route/OpenAPI checks, and PostgreSQL integration coverage.
- Documentation: `docs/guide/zh-CN/architecture/taskworker-apiserver-collaboration.md`, `docs/HANDOFF.md`.
- A pre-existing uncommitted change in `backend/internal/apiserver/service/v1/taskcenter/reconciler.go` was preserved and remains in the worktree; the SSE changes are compatible with its Group/DAG terminal projection.

## Key architectural decisions

- Task Center writes business facts and reliable outbox records in one transaction. It never writes directly to an active SSE connection.
- UserEvent projection is asynchronous and idempotent on `recipient_user_id + source_domain + source_event_id + event_type`; projection failure cannot block task execution.
- SSE gateway reads PostgreSQL in bounded batches instead of keeping an in-memory event backlog, so API instances remain stateless and reconnects work across instances.
- `event_sequence` orders the current user's replay stream only. `aggregate_version` remains the upstream Task Center `resource_version`.
- Arbitrary task result bodies are not pushed. SSE exposes only status/progress/error metadata and allowlisted `artifact_refs`/`representation_refs` summaries.
- No production database migration, compatibility migration, or historical backfill was added per user direction. The new table and constraints use the repository's existing `EnsureScheme/AutoMigrate` initialization path for the current/new schema.

## API, schema, and configuration changes

- Added SSE endpoints under `/api/v1/events` with permissions defined by SSOT (`sse.stream.read`, `sse.history.read`). Current authentication middleware enforces the user boundary.
- Added `sse_user_events` and Task Center outbox topics: `atomic_task_created`, `atomic_task_status_changed`, `task_attempt_status_changed`, and `task_group_status_changed`.
- Added flags: `sse.retention`, `sse.poll-interval`, `sse.heartbeat-interval`, and `sse.max-connections-per-user`.
- Added SSE error codes `170200`, `170201`, `170400-170402`, `170600-170601`, and `170800`.

## Verification results

- Passed `go test ./backend/internal/apiserver/...`.
- Passed race tests for SSE service/controller/projector (including projector shutdown), PostgreSQL store, and Task Center service.
- Passed `go vet ./backend/internal/apiserver/...` and `git diff --check`.
- Passed route-to-SSOT OpenAPI checks for SSE and existing application/task routes.
- Passed an isolated PostgreSQL 16 integration test for Task Center outbox persistence, UserEvent ordering, idempotency, retention fields, and cross-user cursor isolation.
- `go test ./...` still fails only in known unrelated baseline areas: `backend/apis/imachinery`, `backend/pkg/validator`, `backend/pkg/grpccli`, `backend/pkg/grpcsvr`, `third_party/k8s.io/utils/exec`, and `tools/ssot-s1-live`.

## Outstanding tasks

1. Implement the remaining spec-v1.5.0 asset-library Artifact and AssetVersion source events and feed them through the same SSE projector.
2. Run a deployment smoke test with rebuilt API Server and TaskWorker containers: create/cancel/retry AtomicTask, verify live SSE frames, restart API/Worker, and verify Last-Event-ID replay.
3. Review and commit the pre-existing `reconciler.go` terminal-status projection change separately if it is still desired.

## Known issues and risks

- Task Center SSE projection runs in TaskWorker. When TaskWorker is stopped, Task Center outbox rows accumulate and are projected after it returns; task facts remain valid.
- UserEvent cleanup runs hourly in API Server. Expired rows may remain physically present for up to one cleanup interval but are excluded from history and stream queries immediately at `expires_at`.
- No live multi-instance proxy/load-balancer SSE smoke test was run. Unit tests cover framing, cancellation, write deadlines, connection limits, resync, and replay; PostgreSQL coverage used an isolated local PostgreSQL 16 database.
- Artifact/AssetVersion SSE events listed in the full v1.5 SSE enum are not implemented in this task-center-scoped session.

## Recommended next task

Implement asset-library Artifact/AssetVersion reliable source events and projector mappings, then run the complete deployment-level SSE recovery smoke test.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
