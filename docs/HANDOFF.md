# Project Handoff

## Current project goal

Complete and validate the released `spec-v1.7.14` Canvas Application execution contract: Workflow Canvas owns the DAG AtomicTask, Application Platform creates and binds one idempotent ApplicationRun after Conductor resolves final inputs, and Application Artifact references project back into Canvas outputs.

## Completed in this session

1. Pinned `ssot/` and `SSOT_VERSION` to released `spec-v1.7.14` commit `17c2d2f48c565b91d01686cc0081fee8d9e96b15`.
2. Implemented and committed the primary Canvas Application execution path as `bdda046` (`feat(workflow): implement canvas application execution`):
   - ApplicationVersion catalog publication and realtime visibility/runtime validation.
   - Canonical `application-platform.run` DAG compilation with final Conductor input resolution.
   - Idempotent Canvas ApplicationRun creation and existing AtomicTask binding.
   - Reliable Application Artifact reference projection into Canvas output bindings.
   - Task Center, Application Platform, Workflow Canvas, outbox, worker, unit, and PostgreSQL integration support.
3. Built and deployed local apiserver/taskworker images from the new implementation against the existing PostgreSQL, Conductor, and Redis stack.
4. Live backlog replay exposed independent ApplicationRun Artifact events repeatedly Nacking because they had no Canvas task binding. The projector now reads the AtomicTask through the Task Center store, acknowledges non-Canvas tasks without touching Canvas state, and preserves retry behavior for Canvas tasks.
5. Live Canvas Artifact integration exposed partial `WorkflowCanvasRun` reads invoking JSON `AfterFind` hooks. Canvas event owner lookup now scans a lightweight scalar row so unrelated or historical JSON fields cannot roll back output events.
6. Completed a focused PostgreSQL integration run in an isolated temporary database and removed the database afterward.
7. Deployed `omnimam/apiserver:spec-v1.7.14-replay-amd64` and `omnimam/taskworker:spec-v1.7.14-replay-amd64`. Both containers are running and `/healthz` returns `{"status":"ok"}`.

## Files modified

The primary implementation in `bdda046` changed the SSOT pin and the Application Platform, Task Center, Workflow Canvas, PostgreSQL store, TaskWorker, and related tests documented by that commit.

The replay hardening follow-up modifies:

- `backend/internal/apiserver/service/v1/workflowcanvas/application_artifact_projector.go`
- `backend/internal/apiserver/service/v1/workflowcanvas/application_artifact_projector_test.go`
- `backend/internal/apiserver/store/postgresql/canvas_application_integration_test.go`
- `backend/internal/apiserver/store/postgresql/workflow_canvas_events.go`
- `backend/internal/apiserver/taskworker.go`
- `docs/HANDOFF.md`

No file was removed. No new `backend/cmd` binary was introduced.

## Key architectural decisions

- Workflow Canvas creates the only DAG AtomicTask; Application Platform owns ApplicationRun semantics and binds the run only after final Worker arguments are available.
- `application-platform.run` is the only canonical runtime functionRef. Historical `application.execute` text must not be registered or compiled.
- Artifact events remain Application Platform facts for all run origins. Workflow Canvas classifies relevance through the Task Center-owned AtomicTask instead of extending the released event payload with an unapproved origin field.
- Non-Canvas Artifact events are successful no-ops for the Canvas consumer. Canvas events retain Nack/retry semantics until their task and output bindings become readable.
- Scalar relation reads use lightweight scan targets where full model hooks would parse unrelated JSON snapshots.

## API, schema, and configuration changes

- No new public endpoint, permission, error code, runtime configuration key, database table/column, or binary.
- SSOT pin advanced from `spec-v1.7.13` to released `spec-v1.7.14`.
- Internal service/store contracts carry Canvas/Application binding identity and final resolved arguments.
- Existing outbox topics now support released Application publication, ApplicationRun binding, and Artifact output projection behavior.

## Verification

- `git diff --check` passes.
- Focused unit tests pass:
  - `go test ./backend/internal/apiserver/service/v1/workflowcanvas -run 'TestApplicationArtifactProjector' -count=1`
  - `go test ./backend/internal/apiserver -run 'TestConsumeReliablePayloadAcknowledgement|TestCanvasApplicationConsumerGroupsAreStable' -count=1`
- Focused PostgreSQL integration test passes:
  - `go test -tags=integration ./backend/internal/apiserver/store/postgresql -run '^TestCanvasApplicationOutboxAndArtifactProjection$' -count=1`
- Live application Artifact backlog contained 12 messages; durable Canvas consumer offset advanced to 12. Restarted worker logs no longer contain `workflow-canvas-application-artifact-projection` or `record not found` errors.
- Per user instruction, no full test suite was run in this continuation.
- Earlier focused verification still has the unrelated Task Center localization mismatch: expected `生成 thumbnail 表现形式`, actual `生成 thumbnail视图`.

## Outstanding tasks

- Validate one real published, Canvas-enabled ApplicationVersion with a currently executable Engine/runtime from CanvasRun through Provider execution, ApplicationRun binding, AtomicTask terminal state, Artifact READY, and Canvas output READY.
- Add an automated restart-window test that interrupts between ApplicationRun creation, AtomicTask binding, and ApplicationRun binding if live Provider validation reveals timing differences.
- Decide separately whether to update the unrelated thumbnail localization expectation or catalog value.
- Continue remaining Asset Library deletion integration and cross-domain cleanup tracked on this branch.

## Known issues and risks

- The deployed stack validates startup, migration compatibility, outbox backlog replay, and store-level Canvas Artifact projection, but not a real external Provider execution.
- Full backend status is intentionally unknown in this continuation because only targeted tests were authorized.
- Artifact download still allows same-origin private Engine addresses and public HTTPS external sources by design; future Provider adapters should prefer controlled storage references when available.

## Recommended next task

Prepare or identify one published ApplicationVersion with `canvas_enabled=true`, `run_enabled=true`, and a healthy compatible Engine/runtime, then execute a Canvas Application node end to end and record the resulting IDs and state transitions.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
