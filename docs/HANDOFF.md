# Project Handoff

## Current project goal

Upgrade the server from released `spec-v1.6.5` to released `spec-v1.7.0` and align Workflow Canvas, Task Center projection, persistence, error codes, and SSE contracts with the redesigned workflow-canvas S1/S2.

## Completed in this session

1. Fetched and pinned the `ssot` submodule to released `spec-v1.7.0` commit `1090a2531d07283d96475b3af8ffee8367697041`; updated `SSOT_VERSION`.
2. Reviewed the released workflow-canvas S1/S2, architecture, schema, errors, permissions, events, and related SSE changes.
3. Evaluated open-source reuse: retained Conductor OSS for DAG execution, existing `santhosh-tekuri/jsonschema/v6` for node schemas, and existing request validator; used ComfyUI, n8n, and Dify patterns as design references without importing a second workflow engine.
4. Added immutable NodeDefinition registration, list/get/deprecate APIs and persistence with typed ports, JSON schemas, controlled execution binding, renderer metadata, scope, and deprecation state.
5. Replaced the legacy graph/run contract with typed node IDs, definition versions, edges, flows, run scope, run policy, execution-plan digest, FlowRun, 1:N TaskBinding, OutputBinding, outbox, and reconcile cursor models.
6. Added draft validation, run validation, FlowRun list, and NodeRun detail endpoints; the route set now matches the workflow-canvas v1.7 OpenAPI.
7. Added deterministic scope compilation for `all`, `flows`, `only_nodes`, `until_nodes`, and `from_nodes`, including duplicate/unknown target validation and stable JCS/SHA256 digests.
8. Made workflow definition identity content-addressed and CanvasVersion publish idempotent by content digest.
9. Updated CanvasRun creation/retry/cancel semantics, recoverable `RETRYABLE_FAILED` task creation state, stable producer keys, FlowRun creation, and TaskBinding/OutputBinding persistence.
10. Updated Task Center projection to resolve bindings by AtomicTask ID, reject stale task versions, aggregate 1:N tasks into NodeRun, aggregate FlowRun/CanvasRun, and publish Canvas semantic events.
11. Added reliable workflow-canvas Watermill topics plus `workflow_canvas_outbox` audit records and SSE mappings for `canvas.run.*` and `canvas.node.*`.
12. Added all new workflow-canvas business errors and regenerated Go error registration and API error documentation with `make gen`.
13. Added a backward-compatible v1.0 -> v1.7 database backfill for workflow definition fields, run scope/policy, node IDs/execution keys, legacy AtomicTask bindings, latest published version, and new unique indexes.
14. Added unit tests for scope compilation, invalid reuse/scope, NodeDefinition binding, response DTOs, flattened NodeRun detail, and Canvas SSE projection.
15. Verified the legacy migration against a disposable PostgreSQL 16 instance.

## Files modified

- Updated `SSOT_VERSION` and the `ssot` submodule pointer.
- Updated workflow-canvas and SSE API metadata/request/response DTOs under `backend/apis/iapiserver/`.
- Added `backend/apis/iapiserver/meta_workflow_canvas_v17.go`.
- Added `backend/apis/iapiserver/response_workflow_canvas.go` and its tests.
- Updated workflow-canvas controller, routes, service, store interfaces, PostgreSQL store, server schema registration, and tests.
- Added `backend/internal/apiserver/store/postgresql/workflow_canvas_events.go`.
- Added the tagged PostgreSQL migration integration test `workflow_canvas_migration_integration_test.go`.
- Updated Task Center Canvas projection and SSE projector/tests.
- Updated generated error code source and `docs/guide/zh-CN/api/error_code_generated.md`.
- Added `docs/guide/zh-CN/architecture/workflow-canvas-runtime.md`.
- Updated `docs/HANDOFF.md`.

Pre-existing untracked Asset Library architecture documents were not modified.

## Key architectural decisions

- Workflow Canvas owns editing/version/run projections; Task Center and Conductor remain the only execution and retry engine.
- NodeDefinition versions are immutable. Deprecation blocks new references but preserves historical CanvasVersion interpretation.
- Publishing freezes definition snapshots and uses content-addressed workflow definition identity.
- Every CanvasRun persists its fixed request and ExecutionPlan before Task Center creation; runtime failure never falls back to local goroutines.
- NodeRun to AtomicTask cardinality is 0..N. FlowRun is a Canvas projection, not a Task Center Group.
- Task, Artifact, NodeRun, and CanvasRun resource versions are independent and cannot be compared across aggregates.
- Domain facts, Canvas outbox audit rows, and Watermill reliable messages are committed transactionally.
- Existing database data is upgraded in place and historical AtomicTask links are converted into TaskBinding rows.

## API, schema, and configuration changes

- Added 8 workflow-canvas operations: NodeDefinition list/register/get/deprecate, draft validate, run validate, FlowRun list, and NodeRun detail.
- Added workflow-canvas errors `160206-160209`, `160402`, `160603-160611`, and `160801-160802`.
- Added NodeDefinition, FlowRun, NodeRun flow refs, TaskBinding, OutputBinding, Canvas outbox, and reconcile cursor tables/models.
- Expanded CanvasVersion, CanvasRun, CanvasNodeRun, UserEvent, and related indexes/fields.
- Added Canvas reliable outbox topics and SSE envelope IDs/event types.
- No new runtime environment variable is required.

## Remaining work

1. Implement real `reuse_valid_outputs` and `reuse_required` using an Asset Library batch-summary boundary that verifies fingerprint, TTL, owner visibility, Artifact READY state, and required output completeness. Current `reuse_valid_outputs` safely reruns; `reuse_required` returns `ERR_CANVAS_REUSE_REQUIRED_UNAVAILABLE`.
2. Add the Asset Library event consumer that advances OutputBinding to READY/FAILED, emits `canvas_node_output_available`, and only marks NodeRun successful after all required outputs are available.
3. Implement the workflow-canvas reconciler using the persisted Task Center and Asset Library cursors to repair missed/out-of-order events.
4. Connect the released workflow permissions to a shared fine-grained permission evaluator when that evaluator exists; current access remains constrained by authenticated creator/project/namespace/visibility checks.
5. Add automatic flow discovery for graphs without explicit flows if product delivery requires it; current compiler preserves explicit flows and still runs all nodes with `scope=all`.
6. Run an end-to-end API/Worker/Conductor/Artifact recovery test after the Artifact consumer and reuse boundary are available.

## Known issues and risks

- OutputBinding projection is not yet driven by Asset Library events, so progressive Artifact availability is not end-to-end complete.
- Reuse policies are contract-visible but only the safe rerun behavior is active; required reuse fails explicitly rather than silently violating intent.
- Fine-grained workflow permission codes are not enforced because the repository has no shared permission evaluator yet.
- Passive-only CanvasVersion publication still depends on Task Center accepting at least one executable DAG node; a passive-only product path needs an explicit SSOT/runtime decision.
- The migration test covers PostgreSQL 16 and the known v1.0 schema. Production backup and restore rehearsal is still required before deployment.

## Verification

- `go test ./backend/...` passes.
- `go vet ./backend/...` passes.
- `git diff --check` passes.
- `WORKFLOW_CANVAS_TEST_DSN=... go test -tags=integration ./backend/internal/apiserver/store/postgresql -run TestWorkflowCanvasV17MigrationBackfillsLegacyBindings -v` passes against disposable PostgreSQL 16.

## Recommended next task

Implement the Asset Library -> Workflow Canvas output projection boundary first, then add reuse qualification on top of the same batch Artifact visibility/readiness API. This closes the largest remaining correctness gap without introducing a second execution engine.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
