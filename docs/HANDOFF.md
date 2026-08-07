# Project Handoff

## Current goal and status

- Goal: implement the released `spec-v1.18.0` AppStudio end-to-end workflow for static Web applications: idempotent aggregate creation, Coding Agent chat/Invocation execution, source iteration, preview, publication, and history-preserving source restoration.
- Status: In progress. Milestones 1-2 (released create DTO/schema and idempotent aggregate initialization, plus the AppStudio-owned Coding Agent facade) are implemented and pass focused package compilation. The current milestone is fail-closed model/tool authorization.
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
- Assessed the requested AppStudio flow end to end without changing product code. Application/repository/default source creation, source revision restore, preview/build task submission, release/rollback resources, worker executors, and terminal projection are present.
- Confirmed Coding Agent chat is not usable: AppStudio returns no Coding Agent/Session projection, Coding Agents are hidden from public Agent listing/get, first-message runtime startup calls the Platform-only `GetAgent`, `agent.invocation.execute` has no registered worker handler, and `ProjectTaskTerminal` ignores Invocation tasks.
- Confirmed no Agent execution path applies an AppStudio ChangeSet or records a resulting source revision, so chat cannot currently iterate application source.
- Confirmed source restore creates a new revision from an explicitly supplied historical source revision, but there is no public revision-history/conversation-checkpoint facade and no session rollback operation.
- Confirmed production release submission and the worker disagree on two contract values: `RELEASE` versus `DEPLOY`, and `appstudio.production.static-web` versus `studioapp.runtime.static-web`; production reconcile therefore rejects current release tasks.
- Replaced the legacy AppStudio create DTO with the released `initial_requirement`, `coding_model_selection`, optional profile/application fields, attachments, and required create idempotency key; added the released composite create response projections.
- Added the current Coding Agent/Session/generation/create-key fields and PostgreSQL compatibility constraints to `studio_applications`.
- Implemented one PostgreSQL transaction for Application, Repository, Workspace revision 0, Coding Agent, Session, Workspace/Model bindings, first user Message, and first QUEUED CODING Invocation. Stable IDs plus `(owner_user_id, create_idempotency_key)` conflict handling make concurrent replay return the canonical first result.
- Removed the Coding Agent default-model fallback from the initialization path and require an explicit CODING model selection. The first Runtime ensure is triggered only after commit; Task delivery failure leaves the Invocation QUEUED for later scheduling.
- Added the complete AppStudio Agent facade with `appstudio.agent.read`/`appstudio.agent.operate` routes for status, message send, Invocation list/detail/cancel/SSE, suspend, resume, and replace. Every operation validates the current application owner, Agent/Session, Coding kind, and fixed Workspace before delegation; public `/agents` remains Platform-only.
- Added an atomic Coding Agent replacement transaction that creates the new Agent/Session/WorkspaceBinding/ModelBinding and increments/switches the application generation while retaining old history. Replays of the same current replacement are idempotent; reuse of an old replacement key after a later generation is rejected.

## Current in-progress work

- Milestone 3: inject the existing user model execution resolver, issue short-lived per-Invocation model/workspace grants, and remove string fallback grants. `PLATFORM_MODEL` must remain fail-closed unless the released platform resolver exists.
- Authorization implementation checkpoint: add an encrypted, short-lived `agent-model-access-grant://` reference shared by APIServer and Infrastructure. Plaintext claims bind owner, Agent, ModelBinding, purpose, source selection, issue time, and expiry, while Task/database only see authenticated ciphertext. APIServer performs User Model eligibility preflight before creating RuntimeBinding/Task; Infrastructure verifies the grant and re-resolves current User Model facts into an in-memory provider binding.
- Exact next files for this checkpoint: `backend/internal/pkg/agentgrant/model.go`, `backend/internal/apiserver/service/v1/agent/model_access.go`, `backend/internal/apiserver/service/v1/agent/service.go`, `backend/internal/apiserver/server.go`, `backend/internal/taskworker/agentexecutor/executor.go`, `backend/internal/infrastructure/service.go`, `backend/internal/infrastructure/app.go`, and provider request/Docker adapter files.
- Current authorization finding: `usermodel.CredentialBroker` and `UserModelExecutionContext` are process-local, while Task Worker runs separately. They cannot serve as a cross-process Task grant registry. Runtime startup can still be corrected to perform released preflight resolution before creating the RuntimeBinding/Task; Invocation authorization must reuse an existing service-identity resolution boundary or stop rather than introduce an unpublished durable grant protocol.
- Next milestones: Runtime READY queue submission; Invocation Worker adapters and terminal projection; source revision projection; release identifier alignment; focused verification.

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
- Modified `backend/apis/iapiserver/meta_appstudio.go`, `request_appstudio.go`, and `response_appstudio.go` for the released create and Agent projection contracts.
- Modified `backend/internal/apiserver/store/store.go` and `store/postgresql/appstudio.go` for atomic cross-aggregate initialization and idempotent canonical replay.
- Modified `backend/internal/apiserver/store/postgresql/0_pg.go` for AppStudio compatibility columns, backfill, unique index, and Coding Agent binding constraints.
- Modified `backend/internal/apiserver/service/v1/appstudio/service.go` and `service/v1/agent/service.go` for explicit model selection, stable initialization, first Invocation persistence, post-commit Runtime ensure, and internal Coding Agent startup.

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
- Assessment verification: `go test ./backend/internal/apiserver/service/v1/appstudio ./backend/internal/apiserver/service/v1/agent ./backend/internal/taskworker` passes; the two service packages report no test files, while focused Task Worker tests pass.
- Milestone 1 verification: `go test ./backend/apis/iapiserver ./backend/internal/apiserver/store/postgresql ./backend/internal/apiserver/service/v1/agent ./backend/internal/apiserver/service/v1/appstudio` passes.

## Outstanding tasks

- Replace placeholder model/workspace grant strings with short-lived, fail-closed authorization resolved across APIServer, Task Worker, and Infrastructure; keep `PLATFORM_MODEL` unavailable until a released resolver exists.
- Allow the internal Coding Agent runtime lifecycle path to load Coding Agents, submit queued Invocations after runtime readiness, register `agent.invocation.execute`, implement Hermes/OpenCode execution adapters, and project fenced terminal results/messages.
- Apply successful Coding Invocation edits through AppStudio ChangeSets and record the resulting application source revision/checkpoint.
- Add revision/checkpoint history and a conversation-stage restore operation, or clarify that the product only supports explicit source revision restore.
- Align production deployment reason and runtime profile identifiers between AppStudio submission, the registry, and Task Worker.
- Add focused service/integration tests for create-to-chat-to-source, preview, release, and restore/rollback paths.

## Known issues and risks

- Hermes and OpenCode expose different control protocols, so a false shared REST abstraction would be incompatible.
- The current model access reference is only a string and is not resolved into an injectable ModelAccessSpec.
- `taskcenter.Reconciler` exists but is not wired into APIServer startup.
- The complete `agent.invocation.execute@1.0` and changed runtime grant digests passed focused validation; queued-after-runtime submission, Task terminal projection, and runtime adapters remain to implement.
- `ssot/02_architecture/domains/appstudio.md` says the current S1/S2 are unreleased drafts even though the repository is pinned and marked released at `spec-v1.18.0`; this release-state wording must be reconciled upstream before formal acceptance.
- Production release tasks currently fail Task Worker validation because API Server and Worker use different deployment reason and runtime profile values.
- The current initialization source-content write precedes the database transaction because source bytes are outside PostgreSQL; stable workspace IDs make replay idempotent, but source storage and SQL cannot provide a distributed atomic commit.
- `go generate ./backend/internal/taskfunctionregistry` currently resolves its root one directory too high; use `go run ./backend/internal/taskfunctionregistry/internal/generate -root .` until the directive is corrected in a separate scoped change.

## Exact recommended next step

Implement fail-closed model/workspace authorization first, then complete the Coding Agent execution spine: register `agent.invocation.execute`, submit queued Invocations when runtime ensure becomes READY, project fenced terminal messages/results, and apply successful edits as AppStudio ChangeSets. Release identifier corrections follow that working core.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
