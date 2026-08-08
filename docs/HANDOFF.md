# Project Handoff

## Current goal and status

- Goal: complete the released `spec-v1.18.0` AppStudio static-Web flow: idempotent create, Coding Agent chat/Invocation, source revisions, preview, publish, and history-preserving restore.
- Status: in progress. Application creation and Coding Invocation execution complete through the real OpenCode runtime. AppStudio is correctly fenced to `kind=coding`, profile `agent.coding`, `CODING` Invocation and the OpenCode adapter; it does not use the Platform Agent/Hermes CHAT path. The stale model-grant failure and terminal observer recovery starvation are fixed and target-package verified. Source mutation is blocked by a released SSOT contract gap described below; preview/publish/restore end-to-end validation remains incomplete.
- SSOT: `spec-v1.18.0`, submodule commit `b0de28e6b6d9462d95ae8e59a8412640047a218e`; `SSOT_VERSION` matches. Do not modify `ssot/` or add `backend/cmd/` binaries.

## Work completed in this session

- Confirmed and live-verified the AppStudio runtime boundary: Application `1f0b6b23-0c61-5f71-99c9-0010030c89a9` binds Agent `4b3c08b7-d5d4-510a-a1bb-9b281c1fb080` with `kind=coding`, profile/runtime profile `agent.coding`, workspace type `studio`, and a READY/HEALTHY OpenCode Runtime. Hermes remains limited to the Platform Agent CHAT branch.
- Repaired Infrastructure endpoint recovery after `infraserver` restart. `ResolveEndpoint` now rehydrates a missing/expired in-memory provider target through provider `Inspect`, validates the Runtime is still RUNNING and has an endpoint, and restores the provider state cache. Start/Reconcile also refresh provider endpoint state. Provider addresses with no intrinsic expiry receive a bounded one-minute resolve validity window.
- Rebuilt Compose and submitted a real follow-up instruction through the AppStudio UI. Coding Invocation `c55ffbd9-a914-44bc-8c5d-72aa4f31f698` and AtomicTask `10ed48c5-e311-48fc-8aab-1b3e94c1b9cc` succeeded after the Infrastructure service restart; operation events 1-3 are `invocation.started`, `message.completed`, and `invocation.completed`, and assistant Message `299debf6-b688-521b-a2d4-cd2e08137a55` was persisted.
- Confirmed the second-Invocation stale-grant root cause against released User Model S1/S2. `config_version` is the model configuration's monotonic version, but `testProviderModelOwned` currently calls the generic `ProviderModels().Update` on every health probe. This increments `user_provider_models.resource_version` even when only `last_checked_at` changed, so a freshly issued Agent model grant becomes stale before its Task attempt executes. The live default model reached resource version 6562 from repeated healthy probes.
- Repaired Docker Runtime endpoint resolution in `backend/internal/infrastructure/service.go`: a zero `ProviderEndpoint.ValidUntil` now means unspecified provider expiry, not an expired endpoint. The resolver issues a one-minute ephemeral validity window, bounded by any earlier provider or persisted endpoint expiry.
- Added bounded OpenCode non-2xx response detail in `backend/internal/taskworker/agentexecutor/invocation.go` for actionable Worker failures.
- Repaired the OpenCode model configuration adapter: Coding runtimes are started without a project directory, so `PATCH /config` always returns OpenCode 500. The same released configuration payload succeeds through `PATCH /global/config`; the executor now uses that endpoint.
- Live browser verification on `http://127.0.0.1:9990`, account `admin`: created Application `e9fc3725-7052-59af-9c67-132eeb5e9a73`. Its Invocation `dd00d032-9abc-532a-a97a-362619ebb79c` and AtomicTask `3b73d4fc-d97f-4f19-a38f-ed79e67aa32b` both succeeded. The persisted operation events are monotonic: `invocation.started`, `message.completed`, `invocation.completed`; the deterministic assistant Message `b24bcbde-5cc7-5684-9bda-b46bda7fd348` was written.
- `make compose` has rebuilt and started APIServer, Infrastructure, Notification Worker and Task Worker successfully after the fixes.
- Completed AppStudio Invocation source-result projection. `AppStudioStore.ResolveStudioInvocationChangeSets` resolves the final `APPLIED` ChangeSet at the maximum `target_revision`, and send/list/get/cancel facade responses now populate `resulting_change_set_id` and `resulting_source_revision` without inventing results for Invocations that produced no ChangeSet.
- Added the released `appstudio.agent.read` and `appstudio.agent.operate` definitions to Identity bootstrap and granted both to the built-in `USER`, `ADMIN`, and `SUPER_ADMIN` roles. This closes the route/bootstrap mismatch that returned `220606` before the facade could run.
- Added `ProviderModelStore.ProjectHealth` with an owner/model/config-version fence. A repeated health result now refreshes `health_checked_at` without changing `resource_version`; only a changed health status or reason increments the model configuration version. Older probes cannot overwrite a concurrent configuration change or a newer health fact, and health projection/record persistence errors are propagated.
- Confirmed the second facade Invocation `a9562c47-2752-4191-8d13-c1026b683870` / Task `d87b307c-428a-4a63-aa73-a39a7261aac5` failed because the old health updater advanced `resource_version` every 30 seconds. The grant stale check remains fail closed; no fallback authorization path was added.
- Passed `go test ./backend/internal/apiserver/service/v1/usermodel ./backend/internal/apiserver/store/postgresql` after the health projection repair, then completed `make compose` and restarted the affected services.
- Live verification across two later health projections confirmed the default model stayed at `resource_version=6579` while `last_checked_at` advanced from `2026-08-08 11:35:01Z` to `11:37:01Z` and `11:39:01Z`; both results remained `healthy`.
- Tightened `ListPendingAgentTerminalTaskIDs` in `backend/internal/apiserver/store/postgresql/agent.go` to join `atomic_tasks` and return only terminal Tasks still held by Runtime/Invocation fences. Non-terminal Runtime tasks can no longer consume the recovery window and starve a lost terminal observer.
- Confirmed `backend/internal/taskworker/taskworker.go` registers `agent.invocation.execute` with the existing `InvocationExecutor`; no second Worker registration or private Task handler was added.
- Audited the released Workspace Tool boundary. The Invocation grant currently carries only `workspace_id`; OpenCode receives no Workspace Tool endpoint or tool authorization. Released S2 requires CODING grants to encapsulate a short-lived AppStudio Workspace Tool grant and explicitly forbids the Worker from calling `ApplyChangeSet` on the Coding Agent's behalf, but it defines no internal endpoint, request/response schema, runtime tool-injection protocol or grant-resolution contract. No file-copy or private API workaround was introduced.

## Current in-progress work

- Source mutation cannot be implemented in this repository until a released SSOT defines the internal AppStudio Workspace Tool endpoint/protocol, short-lived grant claims and resolution boundary, and Runtime/OpenCode tool injection. The successful Coding Invocation currently creates `/tmp/opencode/index.html` inside the OpenCode container, while the AppStudio source remains Revision 0.
- Invocation Task create/bind recovery is implemented in `backend/internal/apiserver/store/postgresql/task_center.go`, `backend/internal/apiserver/service/v1/taskcenter/task_center.go`, and `backend/internal/apiserver/service/v1/agent/service.go`: idempotency conflicts return the canonical Task internally; pending canonical Tasks without a runtime execution reuse the stable runtime idempotency key; Agent validates the canonical Invocation identity before binding with the existing resource-version fence.
- Audit and repair the remaining AppStudio closure after a successful Coding Invocation: message facade/front-end consumption, source ChangeSet/revision application, Preview, Build/Release and Restore paths.
- The successful OpenCode message currently writes only to the ephemeral runtime filesystem (`/tmp/opencode/...`); it is not an AppStudio source ChangeSet. Do not claim source generation is complete until a released, concrete workspace/tool protocol authorizes applying it.
- The frontend is supplied by image `omnimam-frontend:cefc096`; this repository has no frontend source to repair. Backend changes must preserve the released AppStudio Agent facade consumed by that image.

## Files modified

- `backend/apis/iapiserver/deepcopy_generated.go`
- `backend/apis/iapiserver/meta_agent.go`
- `backend/apis/iapiserver/meta_agent_contract.go`
- `backend/internal/apiserver/server.go`
- `backend/internal/apiserver/service/v1/agent/service.go`
- `backend/internal/apiserver/service/v1/appstudio/service.go`
- `backend/internal/apiserver/service/v1/identity/identity.go`
- `backend/internal/apiserver/service/v1/taskcenter/reconcile_test.go`
- `backend/internal/apiserver/service/v1/taskcenter/reconciler.go`
- `backend/internal/apiserver/service/v1/taskcenter/task_center.go`
- `backend/internal/apiserver/service/v1/usermodel/service.go`
- `backend/internal/apiserver/store/postgresql/agent.go`
- `backend/internal/apiserver/store/postgresql/appstudio.go`
- `backend/internal/apiserver/store/postgresql/platform.go`
- `backend/internal/apiserver/store/postgresql/task_center.go`
- `backend/internal/apiserver/store/store.go`
- `backend/internal/infrastructure/client.go`
- `backend/internal/infrastructure/providers/dockerruntime/docker.go`
- `backend/internal/infrastructure/service.go`
- `backend/internal/pkg/agentgrant/codec.go`
- `backend/internal/taskworker/agentexecutor/invocation.go`
- `backend/internal/taskworker/taskworker.go`
- `docs/HANDOFF.md`

## Architectural decisions

- Task Center remains the sole retry/timeout owner. Do not introduce an APIServer scheduler, watchdog, durable grant table, or private runtime protocol.
- OpenCode endpoint and credential resolution remain ephemeral in the Worker. Model grants remain short-lived and fail closed.
- Runtime-specific provider IDs are deterministic per Invocation; provider configuration is written to OpenCode's global runtime config because the runtime has no project-scoped instance.
- AppStudio must always use the internal Coding Agent path. Public Platform Agents remain hidden from AppStudio creation and Hermes remains the CHAT adapter.
- Do not implement source synchronization by mounting StudioWorkspace, copying `/tmp/opencode`, or having Task Worker call `ApplyChangeSet`; all three violate the released Workspace/ChangeSet boundary.

## Verification

- Passed: `go test ./backend/internal/infrastructure ./backend/internal/taskworker/agentexecutor ./backend/internal/taskworker`
- Passed: `make compose`
- Passed: browser create-to-assistant verification described above.
- Passed: `go test ./backend/internal/apiserver/service/v1/appstudio ./backend/internal/apiserver/store/postgresql`
- Passed: `go test ./backend/internal/apiserver/service/v1/identity ./backend/internal/apiserver/service/v1/appstudio ./backend/internal/apiserver/store/postgresql`
- Passed: `go test ./backend/internal/apiserver/service/v1/usermodel ./backend/internal/apiserver/store/postgresql`
- Passed: `go test -run '^$' ./backend/internal/apiserver/store/postgresql ./backend/internal/apiserver/service/v1/taskcenter ./backend/internal/taskworker` after terminal recovery query change.
- Remaining: cancellation/retry recovery, a released Workspace Tool contract and subsequent source ChangeSet implementation, Preview, DEPLOY/UPGRADE/ROLLBACK and Restore validation.

## Known issues and risks

- An Invocation can remain `QUEUED` with `atomic_task_id IS NULL` after Task Center created its AtomicTask but the domain binding failed. The implemented recovery reuses and binds the canonical Task under the released Task ID/resource-version fences; remaining risk is live validation against historical orphan rows.
- Target-package compile verification passed for `service/v1/taskcenter`, PostgreSQL store, `service/v1/agent`, and `taskworker`. Full Task Center tests are currently blocked by pre-existing `TestAssignSystemName` localization output mismatch (`生成 thumbnail视图` vs `生成 thumbnail 表现形式`); no task-center recovery assertion failed.
- The model health projection fix is live and a fresh follow-up Coding Invocation succeeded after the rebuild; cancellation and retry recovery still need live validation.
- `/tmp/appstudio_followup_verify.go` cannot be used for verification: its Go OPAQUE client fails during `GenerateKE3` because it is incompatible with the frontend WASM OPAQUE implementation. Do not weaken or change the authentication service to accommodate this temporary client.
- The agent runtime is not bound to a Studio workspace. A model can create files in its runtime filesystem but that is not a source revision and must not be copied into source storage through an invented protocol.
- Released `spec-v1.18.0` states the Workspace Tool security semantics but does not define an implementable internal Workspace Tool API or Runtime/OpenCode protocol. Source mutation must wait for an SSOT release; changing only this server would create an unauthorized private contract.
- AppStudio source APIs and Restore semantics exist, but the complete Preview/Build/Release flow has not been revalidated after the Agent fixes.

## Exact recommended next step

Define and release the AppStudio Workspace Tool internal protocol in SSOT (endpoint/module interface, short-lived grant claims/resolution, ChangeSet request/response, and Runtime/OpenCode tool injection), then update the pinned submodule and implement it here. In parallel, verify the existing Preview, Build/Release and Restore paths only where they do not claim Coding Agent source mutation.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
