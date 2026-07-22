# Project Handoff

## Current project goal

Keep the server aligned with released OmniMAM SSOT contracts. The current workspace implements `spec-v1.7.1` TaskAttempt execution logs and the newly released `spec-v1.7.2` Task Center DAG observability plus Asset Library Artifact batch summaries.

## Completed in this session

1. Fetched released tag `spec-v1.7.2`, pinned `ssot` to commit `7607cb2bd93f34a2cce8be8fddf8564529acff8a`, and synchronized `SSOT_VERSION`.
2. Added DAG trigger snapshots (`API/SCHEDULE/CANVAS/DOMAIN_EVENT/RETRY`), start/completion times, `dag_node_key`, TaskAttempt executor snapshots, schema backfills, constraints, and query indexes.
3. Added deterministic Dynamic Fork child identity envelopes in `ConductorRuntime`; planner output now fails closed on malformed structures, duplicate references, unregistered functions, or expansion beyond `max_dynamic_tasks`, and the reconciler idempotently materializes actual child AtomicTasks without introducing a second execution engine.
4. Upgraded DAG detail to return trigger/time snapshots and deterministic declared-node execution aggregates with activity-first and terminal failure priority.
5. Added `node_key` child-task filtering, normalized DAG events, and per-AtomicTask dependency/queue/running/retry timeline segments with explicit incomplete-history markers.
6. Added admin-only executor summaries containing only stable type/display name; Worker IDs, queues, hosts, and addresses remain hidden.
7. Enhanced Attempt log reads with keyword/level/source filters, opaque forward/backward cursors, stable asc/desc sorting, and a UTF-8 download endpoint using the same authorization, filtering, redaction, and retention pipeline.
8. Added `POST /api/v1/artifacts/batch-summaries`: 1..200 ordered IDs, owner-scoped batch lookup, and uniform `artifact=null` for missing, deleted, or invisible targets.
9. Injected Asset Library's `ArtifactSummaryReader` as a Task Center consumer boundary and attached bounded, permission-trimmed summaries to task Artifact references without cross-domain private-table access.
10. Rebuilt DAG detail results from relation-enriched task outputs so Artifact summaries are visible in both task projections and the DAG result; rejected inverted event ranges and marked inverted timeline facts incomplete.
11. Added DTO, service, runtime adapter, dynamic projection, migration marker, route contract, log cursor/filter/download, node aggregation, Artifact batch-order, Dynamic Fork validation, DAG result enrichment, and PostgreSQL migration tests.
12. Updated TaskWorker/API Server architecture documentation for `spec-v1.7.2`.

## Files modified

- Updated `SSOT_VERSION` and the `ssot` submodule pointer.
- Updated Task Center and Asset Library DTOs under `backend/apis/iapiserver/`.
- Added `backend/internal/apiserver/service/v1/taskcenter/observability.go` and `task_logs.go`.
- Added `backend/internal/apiserver/service/v1/assetlibrary/summaries.go`.
- Updated Task Center/Asset Library controller, service, store interfaces, PostgreSQL adapters, runtime adapter, reconciler, routes, bootstrap, and tests.
- Added `backend/internal/apiserver/workflowruntime/conductor_test.go`.
- Added `backend/internal/apiserver/store/postgresql/task_center_observability_migration_integration_test.go`.
- Updated `docs/guide/zh-CN/architecture/taskworker-apiserver-collaboration.md` and this handoff.

Pre-existing untracked Asset Library architecture documents were not modified.

## Key architectural decisions

- Conductor remains the only scheduler/execution engine. Task Center stores authorized business projections and derives observable read models; it does not copy raw runtime payloads or create another history table.
- Dynamic children receive deterministic business IDs at the runtime adapter boundary and become AtomicTask projections during reconciliation. Invalid planner output is rejected before Conductor can schedule undeclared or excessive work.
- DAG trigger information is an immutable creation-time snapshot; deleted or invisible source resources are not re-read to rewrite history.
- Executor snapshots contain only stable category and display name and are returned only to the existing administrator principal.
- Asset Library owns Artifact visibility and summaries. Task Center consumes an injected bounded reader and never reads Asset Library private tables or caches a second fact source.
- Log online reads and downloads share one authorization/filter/redaction/retention pipeline. Conductor still owns log bodies and retention.

## API, schema, and configuration changes

- Enhanced `GET /api/v1/dag-task-groups/{dag_task_group_id}` to return `DAGTaskGroupDetail`.
- Added `GET /api/v1/dag-task-groups/{dag_task_group_id}/events` and `/timeline`.
- Added `node_key` filtering to `GET /api/v1/dag-task-groups/{dag_task_group_id}/tasks`.
- Enhanced Attempt log list filters/cursors and added `GET .../logs/download`.
- Added `POST /api/v1/artifacts/batch-summaries`.
- Added `atomic_tasks.dag_node_key`; `task_attempts.executor_type/executor_display_name`; DAG start/completion and trigger snapshot columns; required indexes and trigger-type constraint.
- Added idempotent backfills for legacy DAG node keys, trigger times/types, and verifiable source snapshots.
- No new error code, event type, permission code, table, environment variable, or configuration flag was introduced.

## Remaining work

1. Implement Workflow Canvas `reuse_valid_outputs` and `reuse_required` using the released Asset Library summary/reuse eligibility boundary.
2. Add the Asset Library event consumer that advances OutputBinding to READY/FAILED and emits progressive Canvas output events.
3. Implement the persisted Workflow Canvas reconciler for missed or out-of-order Task Center and Asset Library events.
4. Connect released fine-grained task/workflow permissions to a shared permission evaluator when the repository provides one.
5. Add API Server/TaskWorker/Conductor restart, Dynamic Fork, and retention integration coverage.

## Known issues and risks

- Historical runtime rows created before `spec-v1.7.2` may lack intermediate projection events; current Task/Attempt facts are used as safe fallback, and timeline gaps remain `complete=false`.
- DAG detail aggregation currently loads the authorized DAG's actual tasks and Attempts into memory. It is bounded by graph/runtime limits but should move to SQL aggregation before substantially increasing Dynamic Fork limits.
- The repository still recognizes `system-admin` as the administrator principal because no shared permission evaluator is available; ordinary users never receive executor summaries.
- Log availability remains bounded by Conductor retention, and the Conductor SDK decoder still depends on a correct JSON `Content-Type` for task-log responses.
- Production database backup/restore rehearsal remains required before deploying accumulated schema upgrades.

## Verification

- `go test ./backend/...` passes.
- `go vet ./backend/...` passes.
- Focused `go test -race` passes for WorkflowRuntime, Task Center, Asset Library, and the PostgreSQL store.
- `git diff --check` passes.
- Task Center and Asset Library route tests match the released OpenAPI files.
- `ssot` resolves exactly to released tag `spec-v1.7.2`; `SSOT_VERSION.commit` matches the submodule commit.
- The `integration`-tagged DAG observability migration test passes against a temporary isolated PostgreSQL database and verifies legacy backfills, idempotency, indexes, and the trigger-type constraint; the temporary database was removed afterward.
- A live Conductor Dynamic Fork/restart/retention test was not run in this session.

## Recommended next task

Implement the Asset Library OutputBinding event consumer and Workflow Canvas repair loop using the new bounded Artifact summary boundary, then add the combined restart/recovery integration suite.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
