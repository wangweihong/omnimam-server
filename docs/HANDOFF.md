# Project Handoff

## Current project goal

Implement the released `spec-v1.7.14` Canvas Application execution contract: a Workflow Canvas ApplicationVersion node should reuse the Canvas DAG AtomicTask, create and bind an idempotent ApplicationRun after final input resolution, and project Application Artifact references back into Canvas outputs.

## Completed in this session

1. Updated the server SSOT pin to released `spec-v1.7.14` commit `17c2d2f48c565b91d01686cc0081fee8d9e96b15`; the `ssot/` submodule points at the same commit.
2. Added Application Platform collaboration support for Canvas execution:
   - Validate published/canvas-enabled ApplicationVersion bindings.
   - Create or reuse an ApplicationRun for a Canvas DAG AtomicTask with resolved runtime inputs.
   - Bind ApplicationRun, CanvasRun, CanvasNodeRun, and AtomicTask through stable idempotency.
3. Updated Workflow Canvas execution to compile and route ApplicationVersion nodes through `application-platform.run`, resolve upstream inputs at node execution time, and project Application Artifact references into Canvas outputs.
4. Added Application catalog synchronization helpers so published ApplicationVersion facts can be exposed to Workflow Canvas discovery without direct ad hoc behavior.
5. Extended Task Center and PostgreSQL stores for Canvas/Application linkage, terminal status projection, outbox behavior, and Artifact output lookup.
6. Added focused unit and PostgreSQL integration coverage for Application catalog validation, ApplicationRun binding, Canvas output projection, and TaskWorker Application execution handling.
7. Preserved existing Asset Library Artifact reference projection work and TaskWorker durable terminal observer behavior.

## Files modified

- `SSOT_VERSION`
- `ssot/`
- `backend/apis/iapiserver/request_task_center.go`
- `backend/internal/apiserver/route.go`
- `backend/internal/apiserver/service/v1/applicationplatform/application_platform.go`
- `backend/internal/apiserver/service/v1/applicationplatform/application_platform_test.go`
- `backend/internal/apiserver/service/v1/taskcenter/task_center.go`
- `backend/internal/apiserver/service/v1/taskcenter/task_center_test.go`
- `backend/internal/apiserver/service/v1/workflowcanvas/application_artifact_projector.go` (added)
- `backend/internal/apiserver/service/v1/workflowcanvas/application_artifact_projector_test.go` (added)
- `backend/internal/apiserver/service/v1/workflowcanvas/application_catalog.go` (added)
- `backend/internal/apiserver/service/v1/workflowcanvas/application_catalog_test.go` (added)
- `backend/internal/apiserver/service/v1/workflowcanvas/workflow_canvas.go`
- `backend/internal/apiserver/store/postgresql/application_platform.go`
- `backend/internal/apiserver/store/postgresql/canvas_application_integration_test.go` (added)
- `backend/internal/apiserver/store/postgresql/outbox.go`
- `backend/internal/apiserver/store/postgresql/task_center.go`
- `backend/internal/apiserver/store/postgresql/workflow_canvas.go`
- `backend/internal/apiserver/store/postgresql/workflow_canvas_application_test.go` (added)
- `backend/internal/apiserver/store/postgresql/workflow_canvas_events.go`
- `backend/internal/apiserver/store/store.go`
- `backend/internal/apiserver/taskworker.go`
- `backend/internal/apiserver/taskworker_test.go`
- `docs/HANDOFF.md`

No file was removed. No new `backend/cmd` binary was introduced.

## Key architectural decisions

- Workflow Canvas owns DAG scheduling and the DAG AtomicTask; Application Platform owns ApplicationRun semantics and creates/binds the ApplicationRun only after Canvas resolves the final node inputs.
- The Canvas/Application bridge is idempotent and keyed by the existing Canvas run/node/task identity, so retries can safely reuse prior ApplicationRun bindings.
- Application-backed Canvas nodes use the released canonical function ref `application-platform.run`; the historical `application.execute` name must not be registered or compiled as a runtime task.
- Artifact lifecycle facts remain owned by Asset Library. Canvas outputs receive projected Application Artifact references, not raw Provider content or Blob facts.
- Cross-domain behavior is implemented through service/store collaboration points rather than direct creation of unrelated domain rows.

## API, schema, and configuration changes

- No new public endpoint, permission, error code, runtime configuration key, or `backend/cmd` binary.
- SSOT pin changed from `spec-v1.7.13` to released `spec-v1.7.14`.
- Internal request/store shapes now carry Canvas/Application binding information needed to reuse a Canvas DAG AtomicTask for ApplicationRun execution.
- Existing Task Center and Workflow Canvas outbox/event paths were extended for the released Canvas Application execution contract.

## Verification

- `git diff --check` passes.
- Focused packages pass:
  - `./backend/internal/apiserver/service/v1/applicationplatform`
  - `./backend/internal/apiserver/service/v1/workflowcanvas`
  - `./backend/internal/apiserver`
  - `./backend/internal/apiserver/store/postgresql`
- `go test ./backend/internal/apiserver/service/v1/applicationplatform ./backend/internal/apiserver/service/v1/workflowcanvas ./backend/internal/apiserver/service/v1/taskcenter ./backend/internal/apiserver ./backend/internal/apiserver/store/postgresql` still reaches the pre-existing Task Center thumbnail localization failure:
  - expected `生成 thumbnail 表现形式`
  - actual `生成 thumbnail视图`
- Known baseline from prior session: full `go test ./backend/...` had the same unrelated thumbnail localization expectation failure.

## Outstanding tasks

- Build/deploy updated apiserver and taskworker images, then verify a live Canvas ApplicationVersion node run end to end.
- Decide whether to fix the unrelated thumbnail localization expectations or catalog translation.
- Continue remaining Asset Library deletion integration and cross-domain cleanup tracked on this branch.
- Add broader restart/replay coverage after live validation if any retry behavior differs from local tests.

## Known issues and risks

- Full backend suite may still be red due to the pre-existing thumbnail localization failures.
- The new Canvas/Application runtime path still needs live Conductor/Worker verification with a real published ApplicationVersion node.
- A newly deployed terminal-projection consumer group starts at offset zero and will scan the existing AtomicTask event backlog once; unrelated and non-terminal events are Acked without Task Center lookup.
- Artifact download currently allows same-origin private Engine addresses and public HTTPS external sources by design; future Provider adapters should prefer controlled storage references when available.

## Recommended next task

Run focused verification, commit the local implementation, then deploy and validate one live Canvas ApplicationVersion execution from CanvasRun through ApplicationRun, AtomicTask terminal state, Artifact registration, and Canvas output projection.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
