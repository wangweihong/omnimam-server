# Project Handoff

## Current project goal

Make Task Center the sole execution-state source for ComfyUI workflow test runs while preserving the released Application Platform response contract.

## Completed in this session

1. Fixed the creator/Worker lost-update race by replacing broad test-run updates with field-scoped DAG binding, provider job ID, selected preview output, and task-creation failure writes.
2. Corrected Task Center aggregation so a failed task plus blocked descendants reaches a terminal failed owner, while independent active branches still keep the owner running.
3. Added reconciliation repair for historical TaskGroup/DAG owners whose tasks are already terminal but whose stored owner projection is stale.
4. Removed `steps_json` marshaling and all persisted workflow-test execution state writes. `status`, `progress`, `current_step`, and `failure_summary` are now read projections only.
5. Changed workflow-test run and step progress to the released `0..1` scale used by Task Center.
6. Made submit persist only the ComfyUI prompt ID, poll return provider state through AtomicTask output, and collect persist only the selected temporary preview descriptors.
7. Added one batch DAG summary lookup for list projections. Detail `steps` are rebuilt from DAG node order and AtomicTasks, including current node, provider state, queue position, prompt ID, and the latest failure.
8. Kept task-creation failures as Application Platform facts because no DAG exists in that failure path.
9. Added regression coverage for scoped binding, dynamic detail projection, batch summary projection, terminal aggregation, historical repair, and executor output behavior.

## Files modified

- `backend/apis/iapiserver/meta_comfyui_workflow.go`
- `backend/apis/iapiserver/meta_comfyui_workflow_test_run_test.go`
- `backend/internal/apiserver/service/v1/applicationplatform/comfyui_test_executor.go`
- `backend/internal/apiserver/service/v1/applicationplatform/comfyui_test_executor_test.go`
- `backend/internal/apiserver/service/v1/applicationplatform/comfyui_workflow_test_run.go`
- `backend/internal/apiserver/service/v1/taskcenter/reconciler.go`
- `backend/internal/apiserver/service/v1/taskcenter/reconcile_test.go`
- `backend/internal/apiserver/store/store.go`
- `backend/internal/apiserver/store/postgresql/comfyui_workflow.go`
- `backend/internal/apiserver/store/postgresql/task_center.go`
- `backend/internal/apiserver/store/postgresql/task_center_test.go`
- `docs/HANDOFF.md`

No files were added or removed.

## Key architectural decisions

- Task Center owns DAG and AtomicTask execution state. Application Platform stores only workflow-test identity, immutable request snapshots, provider job identity, selected temporary previews, and pre-DAG creation failure facts.
- The released OpenAPI still requires `ComfyUIWorkflowTestRunDetail.steps`, so it remains as a compatibility projection dynamically derived from Task Center. It is not a second stored snapshot.
- The target released schema already omits the legacy execution-state columns. Existing deployed legacy columns are left untouched and ignored until normal schema rollout removes them.
- Poll output intentionally excludes full ComfyUI history. Collect reads history when needed and stores only previews for the selected output nodes.
- Creator, Worker, and reconciler writes remain field-scoped to prevent stale whole-record updates from erasing concurrent facts.

## API, schema, and configuration changes

- No endpoint, permission, event, error code, migration, dependency, or runtime configuration was added.
- Public response fields remain compatible. Workflow-test run and step `progress` now follow the released numeric `0..1` contract.
- Internal store operations were narrowed to DAG binding, external job persistence, preview output persistence, and creation-failure persistence.
- The public `steps` field can only be removed after a future released SSOT deprecates or removes it.

## Verification

- Focused Application Platform, API metadata, and PostgreSQL store tests passed.
- `go test ./backend/...` passed.
- `go vet ./backend/...` passed.
- Rebuilt and restarted API Server and TaskWorker with local `codex-taskcenter-source-amd64` images; both started cleanly.
- Browser verification confirmed the latest failed workflow test and DAG `1031bd50-db99-404f-a284-9c2e99d5549c` both show `FAILED`, 67%, one success, one failure, one blocked node, three retries, and the same latest error.
- The workflow-test detail dynamically displayed 4 attempts and 3 retries for `comfyui.poll` and linked directly to the matching Task Center node log view.

## Remaining work

1. Run a new workflow test when an execution side effect is desired and verify selected previews still exclude unselected output nodes.
2. Remove the compatibility `steps` projection only after a future released SSOT contract change.
3. Commit only when explicitly requested.

## Known issues and technical debt

- Existing databases may still contain unused legacy `steps_json`, `status`, `progress`, `current_step`, and `failure_summary` columns until schema rollout removes them.
- Provider state and queue position are internal AtomicTask output details. They remain exposed through the compatibility Application Platform step projection but are not part of the generic Task Center Web client contract.

## Recommended next task

Execute one new selected-output workflow test when authorized, then prepare the server and Web commits when requested.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
