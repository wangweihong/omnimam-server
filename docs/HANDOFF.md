# Project Handoff

## Current goal and status

- Goal: restore `make compose` while preserving the in-progress Agent business-flow work against `spec-v1.18.0`.
- Status: Complete. `make compose` exits successfully and all Compose services are running; Agent business-flow reconciliation remains the next feature task.
- Current SSOT: `spec-v1.18.0` at `b0de28e6b6d9462d95ae8e59a8412640047a218e`; `SSOT_VERSION` matches the submodule commit.

## Work completed in this session

- Reproduced `make compose`; the `apiserver` build failed on the nonexistent legacy `service/v1/canvas` import.
- Confirmed the replacement `workflowcanvas` service is wired independently with Task Center dependencies and no code calls the stale aggregate `Canvases()` method.
- Removed the stale `canvas` import, `Service.Canvases()` declaration, and implementation from the v1 service aggregator.
- The next `make compose` run passed the service aggregator and exposed the matching stale PostgreSQL `Canvases()` factory method; confirmed it has no callers, then removed it from both `store.Factory` and the PostgreSQL datastore while retaining the unused legacy type to keep the change narrow.
- The following `make compose` run reached APIServer wiring and exposed an AppStudio/Agent interface mismatch; updated `CreateCodingAgentForStudio` to accept the released optional model binding and persist it through the existing binding helper, with the Coding default retained when omitted.
- The next build produced the APIServer, Infrastructure, and Notification Worker images, then found the missing checksum for the version-pinned `chinese-calendar-golang` transitive dependency used by local `third_party/gotoolbox`; added the existing pinned version to the main module graph and its checksums without upgrading dependencies.
- Compose then reached runtime startup but APIServer rejected one legacy no-Task `CANCELED` Invocation while installing the released Task-binding constraint; added an idempotent compatibility backfill that projects any invalid legacy no-Task terminal row to `FAILED / ERR_AGENT_INVOCATION_TASK_UNAVAILABLE` before adding the constraint.
- APIServer then rejected the released registry YAML against a stale infra-only embedded schema; regenerated the target assets from pinned `spec-v1.18.0`, adding the Agent execution adapter schema, current retryability entries, and correct `SOURCE` commit/hash.
- Confirmed the released S1/S2 gaps for Agent execution, AppStudio Coding Agent projection, model access, deletion finalization, and Task terminal projection.
- Confirmed public `/api/v1/agents` must remain Platform-only.
- Confirmed Task Center already owns retry, cancellation, and timeout; Agent must not add a watchdog.
- User authorized coordinated upstream SSOT changes, release/tag/push, server implementation, and focused new Go tests.
- Verified the pinned Hermes and OpenCode images and their distinct headless protocols; the implementation must use profile-specific adapters.
- Upstream S1 now requires Task-backed CHAT/CODING execution, soft-delete finalization, current-task monotonic projection, preflight model grants, AppStudio Coding Agent generations, and explicit Hermes/OpenCode adapters.
- Upstream draft now includes the complete AppStudio creation request/response, selected Coding ModelBinding, first Invocation retry boundary, Invocation runtime/event concurrency fields, application-level resulting Revision projection, and source restore history semantics.
- Upstream `agent.invocation.execute@1.0` arguments/result projection now excludes sensitive fields and includes only stable references, authorization, resource version, and recovery cursors.
- Reconciled the server Task Function Registry asset with released `spec-v1.18.0`: added `agent.invocation.execute@1.0`, changed `agent.runtime.ensure@1.0` to `model_access_grant_ref`, removed the unpublished `agent.coding.execute` function/schema, and removed its `TBD` digest.
- Added released Invocation runtime/recovery/concurrency fields and Runtime current-task fields to API models and PostgreSQL compatibility constraints/indexes.
- Unified CHAT/CODING submission on `agent.invocation.execute`, removed message/owner content from Task arguments, enforced an ACTIVE primary model binding, changed runtime startup to `model_access_grant_ref`, and corrected API cancellation to remain `CANCELING`.

## Current in-progress work

- None for the Compose repair. Agent service reconciliation remains paused at generation-aware Task binding and fenced terminal projection.

## Files changed

- Modified: `backend/internal/apiserver/service/v1/service.go` to remove the stale legacy Canvas aggregate dependency.
- Modified: `backend/internal/apiserver/store/postgresql/0_pg.go` to remove the stale legacy Canvas factory method.
- Modified: `backend/internal/apiserver/store/postgresql/0_pg.go` to migrate legacy no-Task Invocation terminal rows before enforcing the released binding constraint.
- Modified: `backend/internal/apiserver/store/factory.go` to remove the matching stale factory contract method.
- Modified: `backend/internal/apiserver/service/v1/agent/service.go` to complete the AppStudio Coding Agent model-binding interface implementation.
- Modified: `go.mod` and `go.sum` to record the existing pinned calendar dependency required by `taskworker`.
- Modified: `SSOT_VERSION`, `docs/HANDOFF.md`, and `backend/internal/taskfunctionregistry/registry.go`.
- Modified: `backend/internal/taskfunctionregistry/registry.go` to model optional Infra and Agent execution adapters without injecting absent fields into canonical digest payloads.
- Temporary validation `backend/internal/taskfunctionregistry/upstream_tmp_test.go` was removed after its passing run.
- Pre-existing user changes to preserve: `backend/apis/iapiserver/meta_agent.go`, `backend/apis/iapiserver/request_appstudio.go`, `backend/internal/apiserver/service/v1/agent/service.go`, `backend/internal/apiserver/service/v1/appstudio/service.go`, `backend/internal/taskfunctionregistry/assets/function-registry.yaml`.
- Modified: `backend/internal/taskfunctionregistry/assets/function-registry.yaml` to match the released nine-function registry contract.
- Modified generated assets: `backend/internal/taskfunctionregistry/assets/function-registry.schema.yaml`, `error-retryability.json`, and `SOURCE` to match the same released registry.
- Modified Agent API models, Agent store interface/PostgreSQL implementation, PostgreSQL constraints, and Agent service task submission/cancellation behavior.

## Key decisions

- Coding Agents remain hidden from public Agent management and are exposed only through AppStudio application-level projections/actions.
- Workspace retention, user-authored Memory without Invocation, strict Coding Agent workspace idempotency, Enable without automatic Runtime start, and Session Close without implicit cancellation remain unchanged.
- Runtime lifecycle and Invocation execution use Task Center; API Server must not launch process-local execution goroutines.
- Server formal implementation is now gated by released `spec-v1.18.0`; the submodule and `SSOT_VERSION` must remain equal.

## API, schema, dependency, and configuration changes

- SSOT pin updated from `spec-v1.17.2` to released `spec-v1.18.0`; server behavior changes remain in progress.

## Verification performed

- Baseline target tests: Agent, AppStudio, Infrastructure, Task Worker Agent executor, and API packages compile/pass; they currently have no focused tests.
- Task Center package has a pre-existing unrelated `TestAssignSystemName` localization failure; do not modify it for this task.
- Hermes JSON-RPC/WebSocket and OpenCode REST/SSE session, message, event, cancellation, and health surfaces were verified against the pinned images.
- `go test ./internal/taskfunctionregistry -run TestLoadUpstreamRegistry -v` reproduced the stale Invocation digest: the incomplete loader calculated `sha256:d792c3fbfe73d0594d44f2b401357a48ac28a95505af9e5c9204fa79bab1d468` versus declared `sha256:0cdeed61f6378670501e720b7ac102fe6132bc53e4af2b4c6b92fa86cd52c370`. This value is not publishable because the loader omitted `execution_adapter` and injected an absent empty `infra_adapter`.
- On 2026-08-07, `go test ./internal/taskworker/agentexecutor` and `go test ./internal/taskworker` were blocked before package compilation because `backend/internal/apiserver/service/v1/service.go` imports the absent `internal/apiserver/service/v1/canvas` package. The broader worker package also reports the existing missing `github.com/Lofanmi/chinese-calendar-golang/calendar` go.sum entry. Neither blocker is caused by the Agent Invocation changes, and unrelated modules were not modified to hide them.
- After optional adapter modeling and loading the referenced Asset Library retry catalog, the same focused test passed with all nine calculated digests matching their declarations.
- `git submodule status ssot` and `SSOT_VERSION` both resolve to `b0de28e6b6d9462d95ae8e59a8412640047a218e` (`spec-v1.18.0`).
- `go test ./internal/apiserver/service/v1/agent` passes after replacing the stale nonexistent `CreateAtomicTask` call with the domain Task API.
- `go test ./backend/internal/apiserver/service/v1`, `go test ./backend/internal/apiserver/store/postgresql`, and `go test ./backend/internal/apiserver/service/v1/agent` pass for the Compose fixes.
- `go test -mod=mod ./backend/internal/taskworker` compiles and runs but fails existing AppStudio tests because their embedded registry fixture still enforces the old infra-only registry schema; no unrelated fixture changes were made for the Compose build repair.
- `go test ./backend/internal/taskfunctionregistry` passes after regenerating the released registry schema assets.
- Final `make compose` exits 0 and starts PostgreSQL, Redis, Conductor, APIServer, Infraserver, Notification Worker, and Taskworker.
- Final `docker compose -f deployments/docker-compose.yaml ps` shows all services running, declared health checks healthy, and restart count 0 for all four OmniMAM backend processes.
- `git diff --check` passes.

## Outstanding tasks

- Complete generation-aware Task binding and fenced terminal projection, then implement the worker/runtime adapter and AppStudio facade against the released SSOT.

## Known issues and risks

- Hermes and OpenCode expose different control protocols, so a false shared REST abstraction would be incompatible.
- The current model access reference is only a string and is not resolved into an injectable ModelAccessSpec.
- `taskcenter.Reconciler` exists but is not wired into APIServer startup.
- The complete `agent.invocation.execute@1.0` and changed runtime grant digests passed focused validation; queued-after-runtime submission, Task terminal projection, runtime adapters, and AppStudio facade remain to implement.
- `go generate ./backend/internal/taskfunctionregistry` currently resolves its root one directory too high; use `go run ./backend/internal/taskfunctionregistry/internal/generate -root .` until the directive is corrected in a separate scoped change.

## Exact recommended next step

Correct the stale registry fixtures in the focused Taskworker tests, then complete generation-aware Invocation Task binding and fenced terminal projection in `backend/internal/apiserver/service/v1/agent/service.go`.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
