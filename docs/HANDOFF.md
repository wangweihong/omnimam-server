# Project Handoff

## Current goal and status

- Goal: implement the released Agent business-flow closure after publishing the coordinated `spec-v1.18.0` SSOT update.
- Status: Agent Invocation persistence and Task submission reconciliation is in progress; the Agent service now compiles against the released function contract.
- Current SSOT: `spec-v1.18.0` at `b0de28e6b6d9462d95ae8e59a8412640047a218e`; `SSOT_VERSION` matches the submodule commit.

## Work completed in this session

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

- Agent service reconciliation is in progress: centralize generation-aware task submission, submit queued Invocations after Runtime READY, and add fenced terminal projection.

## Files changed

- Modified: `SSOT_VERSION`, `docs/HANDOFF.md`, and `backend/internal/taskfunctionregistry/registry.go`.
- Modified: `backend/internal/taskfunctionregistry/registry.go` to model optional Infra and Agent execution adapters without injecting absent fields into canonical digest payloads.
- Temporary validation `backend/internal/taskfunctionregistry/upstream_tmp_test.go` was removed after its passing run.
- Pre-existing user changes to preserve: `backend/apis/iapiserver/meta_agent.go`, `backend/apis/iapiserver/request_appstudio.go`, `backend/internal/apiserver/service/v1/agent/service.go`, `backend/internal/apiserver/service/v1/appstudio/service.go`, `backend/internal/taskfunctionregistry/assets/function-registry.yaml`.
- Modified: `backend/internal/taskfunctionregistry/assets/function-registry.yaml` to match the released nine-function registry contract.
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

## Outstanding tasks

- Complete generation-aware Task binding and fenced terminal projection, then implement the worker/runtime adapter and AppStudio facade against the released SSOT.

## Known issues and risks

- Hermes and OpenCode expose different control protocols, so a false shared REST abstraction would be incompatible.
- The current model access reference is only a string and is not resolved into an injectable ModelAccessSpec.
- `taskcenter.Reconciler` exists but is not wired into APIServer startup.
- The complete `agent.invocation.execute@1.0` and changed runtime grant digests passed focused validation; queued-after-runtime submission, Task terminal projection, runtime adapters, and AppStudio facade remain to implement.

## Exact recommended next step

Complete generation-aware Invocation Task binding and fenced terminal projection in `backend/internal/apiserver/service/v1/agent/service.go`, then run the focused Agent and registry package tests.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
