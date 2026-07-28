# Project Handoff

## Current project goal

Finish and verify the released `spec-v1.7.13` ApplicationRun history, durable terminal AtomicTask projection, and Asset Library-owned Artifact delivery while preserving the in-progress `spec-v1.7.12` Asset Library deletion work.

## Completed in this session

1. Completed the cross-domain Artifact ownership refactor:
   - Asset Library owns Artifact identity, content, processing, registration, Blob linkage, resource version, lifecycle events, and outbox.
   - Application Platform stores only monotonic `ApplicationArtifactRef` projections.
   - TaskWorker consumes Asset Library Artifact events and rebuilds ApplicationRun references idempotently.
2. Added controlled Provider output delivery through `ArtifactLifecycle`, including bounded download, same-origin Engine authentication, external HTTPS/private-address protection, shared content storage, processing failure, registration failure, and automatic Asset registration.
3. Added Blob reuse by `(storage_backend_id, object_key)` for identical controlled content paths and covered it with PostgreSQL integration assertions.
4. Preserved legacy `ApplicationArtifact` reads only for migration compatibility; new writes use Asset Library Artifact facts and `aiapp_application_artifact_refs`.
5. Fixed live ApplicationRun cancellation caused by an omitted Task Center timeout policy being registered as a one-second Conductor workflow timeout. ApplicationRun AtomicTask creation now inherits the selected EngineInstance `task_timeout_seconds`; legacy zero values use the existing 1800-second fallback.
6. Rebuilt and deployed `omnimam/apiserver:artifactref-live3-amd64` and `omnimam/taskworker:artifactref-live3-amd64`, then verified a safe ComfyUI run end to end:
   - ApplicationRun `b8722ca3-6d36-4e8d-a79a-ab5fa93a5563`
   - AtomicTask `978defb9-79c6-4f2a-8b18-2d79b009217f`, `SUCCESS`, timeout policy `3600`
   - Artifact `631af0a4-471e-4767-987c-287f34e46537`, `ready/registered`, resource version `5`
   - ApplicationArtifactRef `331b353e-6a81-47b2-9246-0c9b14716412`
   - Asset `47bb6eef-bdc7-456c-895a-9e348cc27f9e`
   - AssetVersion `caac9626-0a1f-4eb2-8fb6-7bcf33dc64dd`
7. Verified Web Application detail/history rendering: the newest run displays `成功` and `images:ready/registered`; run detail exposes Artifact and Asset navigation actions.
8. Replaced the bounded “200 most recently updated runs” repair scan with released-contract durable retry:
   - TaskWorker now uses fixed consumer group `application-platform-terminal-projection`.
   - It consumes Task Center `atomic_task_status_changed`, filters terminal ApplicationRun tasks, reloads the current full AtomicTask fact, and calls idempotent `Completed`.
   - A message is Acked only after terminal projection and Artifact delivery succeed; transient failure Nacks it for PostgreSQL outbox redelivery.
   - Watermill PostgreSQL offsets start at zero for a new group, replay historical events, and resume from the last Ack after restart.
   - Removed `ListApplicationRunProjectionCandidates` and `ReconcileTerminalProjections`; no bounded scan window remains.
9. Added unit coverage for terminal filtering, current-task reload, stable consumer-group identity, Ack after success, and Nack after failure.

## Files modified

- `backend/apis/iapiserver/deepcopy_generated.go`
- `backend/apis/iapiserver/meta_application_platform.go`
- `backend/apis/iapiserver/meta_sse.go`
- `backend/apis/iapiserver/response_application_platform.go`
- `backend/apis/iapiserver/response_application_platform_test.go`
- `backend/internal/apiserver/server.go`
- `backend/internal/apiserver/service/v1/applicationplatform/application_platform.go`
- `backend/internal/apiserver/service/v1/applicationplatform/application_platform_test.go`
- `backend/internal/apiserver/service/v1/applicationplatform/artifact_projector.go` (added)
- `backend/internal/apiserver/service/v1/applicationplatform/artifact_projector_test.go` (added)
- `backend/internal/apiserver/service/v1/applicationplatform/executor.go`
- `backend/internal/apiserver/service/v1/applicationplatform/executor_test.go`
- `backend/internal/apiserver/store/postgresql/0_pg.go`
- `backend/internal/apiserver/store/postgresql/application_platform.go`
- `backend/internal/apiserver/store/postgresql/application_platform_test.go`
- `backend/internal/apiserver/store/postgresql/asset_contract_integration_test.go`
- `backend/internal/apiserver/store/postgresql/asset_lifecycle_contract.go`
- `backend/internal/apiserver/store/postgresql/asset_upload_contract.go`
- `backend/internal/apiserver/store/postgresql/outbox.go`
- `backend/internal/apiserver/store/store.go`
- `backend/internal/apiserver/taskworker.go`
- `backend/internal/apiserver/taskworker_test.go`
- `docs/guide/zh-CN/architecture/taskworker-apiserver-collaboration.md`
- `docs/HANDOFF.md`

No file was removed. Existing Asset Library deletion changes remain preserved in branch history and were not reverted.

## Key architectural decisions

- AtomicTask remains the execution status fact source; terminal observers run only after Task Center commits the terminal projection.
- Durable terminal-observer retry reuses the released `atomic_task_status_changed` outbox contract and Watermill consumer offset. It does not add a table, event type, scheduler, checkpoint model, or binary.
- The consumer reads the full current AtomicTask from Task Center instead of treating the summarized outbox payload as the output fact.
- Delivery is at least once. ApplicationRun task-resource-version gates, stable Artifact producer keys, and Artifact resource-version gates make replay idempotent and monotonic.
- Asset Library is the only Artifact lifecycle fact source. Application Platform never stores Artifact content, Blob facts, processing facts, or registration facts.
- Provider URLs remain inside ApplicationExecutor. Asset Library receives bytes or controlled storage facts, never credentials, arbitrary private URLs, or raw Provider responses.
- ApplicationRun workflow timeout derives from the selected EngineInstance configuration instead of Task Center’s empty-policy fallback.

## API, schema, and configuration changes

- No new public endpoint, permission, error code, runtime configuration key, or `backend/cmd` binary.
- Added the runtime table/model `aiapp_application_artifact_refs`, already defined by released SSOT.
- Added reliable topic `application_run_artifact_ref_changed` and TaskWorker consumption of released Asset Library Artifact topics.
- Added durable consumption of the existing released `atomic_task_status_changed` topic with consumer group `application-platform-terminal-projection`.
- ApplicationRun response artifacts now come from `ApplicationArtifactRef`.
- SSOT remains pinned to released `spec-v1.7.13` commit `52fae755aaaefefec09f4cad33e429e998bf1edc`.

## Verification

- Focused packages pass:
  - `./backend/apis/iapiserver`
  - `./backend/internal/apiserver`
  - `./backend/internal/apiserver/service/v1/applicationplatform`
  - `./backend/internal/apiserver/store/postgresql`
  - `./backend/internal/apiserver/workflowruntime`
- `go test -race ./backend/internal/apiserver` passes.
- `git diff --check` passes.
- `go test ./backend/...` reaches only the two pre-existing thumbnail localization failures:
  - expected `生成 thumbnail 表现形式`
  - actual `生成 thumbnail视图`
- Live Conductor execution completed, Task Center persisted `SUCCESS`, the terminal observer delivered and registered the Artifact, Asset Library lifecycle events updated the reference projection, and Web rendered the durable history.
- Diagnostic run `c5d33725-1674-473c-bb9b-f2e724640cde` remains `CANCELED`; it is the reproduction that proved the old one-second timeout root cause.

## Outstanding tasks

- Finish the remaining Asset Library deletion integration and cross-domain cleanup already tracked on this branch.
- Build/deploy the next TaskWorker image and verify the new `application-platform-terminal-projection` consumer offset and replay behavior in the live environment.
- Decide whether to fix the two unrelated thumbnail localization expectations or the catalog translation.

## Known issues and risks

- A newly deployed terminal-projection consumer group starts at offset zero and will scan the existing AtomicTask event backlog once; unrelated and non-terminal events are Acked without Task Center lookup.
- Artifact download currently allows same-origin private Engine addresses and public HTTPS external sources by design; future Provider adapters should prefer controlled storage references when available.
- The two unrelated localization tests keep the full backend suite red.

## Recommended next task

Finish the remaining Asset Library deletion integration and cross-domain cleanup, then run the focused deletion contracts and full backend regression without changing released SSOT.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
