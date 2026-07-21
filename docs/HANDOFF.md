# Project Handoff

## Current project goal

Keep canonical Asset Library image uploads reliable from `asset_version_representation_requested` through the Task Center inspect/generate/finalize DAG and Blob-backed thumbnail Representation.

## Completed in this session

1. Diagnosed historical image Assets that stayed `thumbnail_status=pending`: their old event payloads lacked the released media policy and created only `inspect -> finalize` DAGs.
2. Per user direction, removed legacy-payload fallback instead of implementing compatibility or historical backfill for the deleted data.
3. Fixed the durable consumer identity to `task-center-representation-orchestrator`, matching the released domain responsibility instead of a handler-specific name.
4. Added typed event decoding and strict validation for asset/version owner, Task Center scope, media type, profile version, requested Representation plan, and idempotency key. Incomplete events are rejected instead of silently creating a DAG without generate nodes.
5. Added structured error logging before Nack so repeated decode or DAG creation failures identify the consumer group, message ID, and cause.
6. Added regression tests proving a complete image event creates `inspect -> thumbnail:list-320 -> finalize`, legacy incomplete events fail, and the consumer group constant remains stable.
7. Rebuilt API Server and TaskWorker as `thumbnail-pending-fix-amd64`, tagged the verified images as local `latest-amd64`, and deployed both.
8. Deleted and recreated the local PostgreSQL, Conductor/Redis, log/debug, and asset volumes. Recreated one writable local StorageBackend at `/var/lib/omnimam/assets`.
9. Completed two clean image upload smoke tests. Both reached Asset thumbnail `ready`, AssetVersion `ready 2/2`, three successful AtomicTasks, and a ready `thumbnail:list-320` PNG Representation.
10. Restarted TaskWorker between smoke tests. The same durable consumer group resumed from offset 1 and advanced to offset 2 without replaying the first event or creating a second group.
11. Verified thumbnail content returned HTTP 200 as a 320x180 PNG, then permanently deleted both smoke Assets and all four associated Blobs. The canonical Asset, Representation, and Blob tables are empty again.

## Files modified

- `backend/internal/apiserver/taskworker.go`
- `backend/internal/apiserver/taskworker_test.go`
- `docs/guide/zh-CN/architecture/taskworker-apiserver-collaboration.md`
- `docs/HANDOFF.md`

No SSOT, HTTP API, database schema, permission, error code, or event type was changed.

## Key architectural decisions

- Representation orchestration accepts only the complete released event contract; it does not infer missing media policy or route legacy payloads.
- The durable consumer group is a stable domain-level identifier and must not be renamed when internal handlers change.
- `thumbnail_status` is projected only by registering a thumbnail Representation. Task success alone never invents an Asset thumbnail fact.
- Invalid durable messages remain retryable through Nack, but failures are now observable in structured logs.
- Task Center continues to own DAG/AtomicTask state; asset-library owns Blob, Representation, AssetVersion, and thumbnail projection facts.

## API, schema, and configuration changes

- No API or schema change.
- Local deployment data was intentionally reset; prior users, assets, tasks, events, schedules, application data, and stored media were deleted.
- Current containers use `omnimam/{apiserver,taskworker}:thumbnail-pending-fix-amd64`.
- The same verified image IDs are locally tagged `omnimam/{apiserver,taskworker}:latest-amd64` for normal compose startup.
- The fresh database contains one enabled, writable local StorageBackend rooted at `/var/lib/omnimam/assets`.
- SSOT remains pinned to released contract `spec-v1.6.5` at `e3cbfef1ee55bc9a497a128bc202b1e0b44bd28f`.

## Verification results

- `go test ./... -count=1` passed.
- `go test -race ./backend/internal/apiserver -run 'TestRepresentation(DAGRequest|Orchestrator)' -count=1` passed.
- `go vet ./backend/internal/apiserver/...` passed.
- Focused API DTO, PostgreSQL store, Asset Library, Task Center, and TaskWorker tests passed.
- Live smoke 1: 640x360 source produced a 320x180, 614-byte PNG thumbnail; DAG and all three AtomicTasks succeeded.
- Live smoke 2 after TaskWorker restart: 480x480 source produced a 320x320 PNG; DAG and all three AtomicTasks succeeded.
- The only representation consumer group is `task-center-representation-orchestrator`, with acknowledged offset 2 after both uploads.
- No `asset representation orchestration failed` log was emitted.
- Smoke cleanup left `user_assets=0`, `asset_representations=0`, and `blobs=0`.

## Remaining work

1. Implement the released `asset-library.representation-backfill` SYSTEM RECONCILE handler if periodic repair is still required for future transient failures. It is no longer needed for deleted historical data but remains an SSOT completeness gap.
2. Consider bootstrapping or explicitly provisioning the default local StorageBackend during clean installation; canonical uploads currently require it to exist before content upload.
3. Extend the Representation policy and adapters for preview, playback, package, and manifest independently of the fixed image thumbnail path.

## Known issues and risks

- Backfill RECONCILE is not implemented, so a future event that permanently fails still requires operator action until that released recovery path exists.
- A clean database has no StorageBackend seed. The current environment is configured, but another full volume reset requires recreating the local backend.
- The frontend browser retained an old Last-Event-ID across the database reset and may log harmless missing-event lookups until its local SSE cursor is refreshed.
- Image resizing still uses deterministic nearest-neighbor sampling; production quality/orientation/color-profile handling remains future adapter work.

## Recommended next task

Implement the released Representation backfill RECONCILE path and clean-install StorageBackend provisioning, then test API Server, TaskWorker, Conductor, and PostgreSQL restart recovery with an event deliberately interrupted between outbox publication and DAG completion.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
