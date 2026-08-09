# Handoff

## Current goal and status

- Goal: stop the Compose `taskworker` crash loop caused by its invalid remote HTTP MCP public base URL, rebuild the service, and restore the path to Source Revision/Preview verification.
- Status: fixed and deployed locally. Task Worker now starts with the shared local loopback default, remains `running`, and has `RestartCount=0`; the rejected remote HTTP value and crash loop are gone.
- SSOT: released `spec-v1.20.0` at `0e6300c8e776df08972a229f48775ed71ad5bff9`; `ssot` submodule and `SSOT_VERSION` match.

## Work completed in this session

- Replaced Task Worker's hard-coded `http://apiserver:8080` MCP public base URL with the shared `OMNIMAM_MCP_PUBLIC_BASE_URL` interpolation and loopback local-development default; remote deployments retain the mandatory reachable HTTPS override.
- Documented the shared API Server/Task Worker MCP public-Origin requirement in `deployments/README.md`.
- Force-recreated `omnimam-taskworker` with the already deployed `omnimam/taskworker:b2d143a-amd64` image so the Compose-only configuration correction took effect without changing binary versions.
- Added the grant-authenticated internal `POST /internal/appstudio/workspace-tool` MCP transport with source status/list/read and atomic ChangeSet apply tools.
- Added nested encrypted Workspace Tool claims scoped to owner, application, Workspace, Agent, Session, Invocation, initial Revision, allowed actions, path scope, and expiry.
- Added Task Worker OpenCode remote-MCP injection with initial Revision/path instructions; canonical StudioWorkspace storage remains unmounted.
- Added a success gate requiring the same Invocation to own an applied ChangeSet whose target Revision is newer than the issued initial Revision. Missing source results now produce a structured FAILED Invocation rather than false success.
- Added Runtime-local Workspace Tool disable/credential cleanup on normal completion and recovery.
- Added idempotent ChangeSet replay validation and immutable source-revision content verification for retry recovery.
- Added transactional `invocation.started` projection to `Invocation=RUNNING` and `Runtime=ACTIVE`, terminal projection to `Runtime=IDLE`, Runtime-row serialization for concurrent Invocations, and periodic recovery for pre-existing projection gaps.
- Extended Agent grant lifetime from 10 minutes to 60 minutes, matching `agent.invocation.execute@1.0` overall timeout.
- Added Preview empty-source rejection before Runtime/Task creation and retained Snapshot rejection while preserving non-empty-check store errors.
- Added persistent AppStudio source storage through `OMNIMAM_APPSTUDIO_SOURCE_DIR` and the Compose volume `omnimam_appstudio_source`; documented the deployment and MCP security boundary.

## Current in-progress work

- None in the repository.

## Files added, modified, renamed, or removed

- Added: `backend/internal/apiserver/service/v1/appstudio/workspace_tool.go`.
- Modified API/internal contracts: `backend/apis/iapiserver/meta_agent_contract.go`, `backend/apis/iapiserver/request_appstudio.go`, `backend/pkg/mcp/types.go`, `backend/internal/pkg/agentgrant/codec.go`.
- Modified AppStudio transport/service/store: `backend/internal/apiserver/controller/v1/appstudio/appstudio.go`, `backend/internal/apiserver/route.go`, `backend/internal/apiserver/server.go`, `backend/internal/apiserver/service/v1/appstudio/service.go`, `backend/internal/apiserver/service/v1/appstudio/source_store.go`, `backend/internal/apiserver/store/postgresql/appstudio.go`, `backend/internal/apiserver/store/store.go`.
- Modified Agent/Task Worker projection and execution: `backend/internal/apiserver/service/v1/agent/service.go`, `backend/internal/apiserver/store/postgresql/agent.go`, `backend/internal/taskworker/agentexecutor/invocation.go`, `backend/internal/taskworker/taskworker.go`.
- Modified deployment/docs: `scripts/install/environment.sh`, `deployments/docker-compose.yaml`, `deployments/README.md`, `docs/guide/zh-CN/architecture/mcp-server.md`, `docs/HANDOFF.md`.
- No files renamed or removed; `ssot/` was not modified.

## Key architectural or design decisions

- Do not mount canonical StudioWorkspace into Coding Runtime. All source reads/writes go through the Invocation-scoped Workspace Tool and AppStudio-owned ChangeSet/base-revision fencing.
- Keep Workspace Tool credentials nested inside the encrypted Invocation grant and Runtime-local OpenCode configuration; AtomicTask arguments/output do not expose Workspace IDs, paths, endpoint URLs, or credentials.
- Use the existing `StudioChangeSet.agent_invocation_id` query as the source-result projection; do not extend the released `agent.invocation.execute@1.0` output schema.
- Keep Task Center/Conductor as the sole scheduler. State recovery uses the existing reconciler and does not introduce leases, dispatchers, or a second state machine.
- Activity projection uses column updates without advancing the Invocation/Runtime resource-version fences used by current AtomicTask binding.

## API, schema, dependency, or configuration changes

- No public API, database schema/table/field, migration, permission, public event type, business error code, task schema, or dependency changes.
- Added one internal grant-authenticated MCP route and internal grant claim type.
- Added `AgentRuntimeActivityActive = "ACTIVE"`, which is projected from existing Invocation lifecycle facts.
- Agent grant TTL is now one hour in API Server and Task Worker.
- Added `OMNIMAM_APPSTUDIO_SOURCE_DIR`, defaulting to `/var/lib/omnimam/appstudio-source`, and Compose volume `omnimam_appstudio_source`.
- API Server and Task Worker now consume the same `OMNIMAM_MCP_PUBLIC_BASE_URL`; local startup defaults to loopback HTTP, while a remotely reachable Coding Runtime requires an explicit HTTPS Origin.

## Verification performed and remaining checks

- Passed focused tests for `internal/apiserver`, AppStudio controller/service, Agent service, Agent executor, Task Worker, PostgreSQL store, agent grant package, and MCP package.
- Passed focused `go vet` for AppStudio service, Agent service, Agent executor, PostgreSQL store, and MCP package.
- Passed `docker compose -f deployments/docker-compose.yaml config --quiet`.
- Passed `bash -n scripts/install/environment.sh` and `git diff --check`.
- Rendered Task Worker configuration contains `APISERVER_MCP_PUBLIC_BASE_URL=http://127.0.0.1:8080`.
- Recreated Task Worker and observed it continuously `running` with restart count zero; startup logs no longer contain `remote MCP public base URL and allowed origins must use HTTPS`.
- No full-repository tests were run, per repository scope rules.
- Remaining external verification: rebuild/redeploy API Server and Task Worker, recreate the Coding Runtime if necessary, then run a real AppStudio instruction and confirm Tool discovery/call, Source Revision advancement, result fields, active/idle projection, Preview, Snapshot, restart persistence, cancellation, and timeout behavior.

## Outstanding tasks

- Before real Coding Runtime verification, set `OMNIMAM_MCP_PUBLIC_BASE_URL` to a Runtime-reachable HTTPS Origin and recreate API Server/Task Worker if the Runtime is not co-located with a usable loopback proxy.
- Perform the real UI/OpenCode Source Revision, Preview, Snapshot, cancellation, timeout, restart-persistence, and second-iteration verification described above.
- If the Web client must disable Snapshot before submission rather than display the server business error, implement that presentation-only preflight in the Web repository; this server repository already enforces the prerequisite.

## Known issues and risks

- The local loopback default is sufficient for process startup but is not automatically reachable from a separate Coding Runtime container; that deployment must supply a valid reachable HTTPS Origin before end-to-end Workspace Tool verification.
- OpenCode remote-MCP behavior was verified against the pinned configuration contract and compile-time integration, but not against a newly deployed live Runtime in this session.
- Runtime-local Tool cleanup is best-effort after process-level crashes; the encrypted Workspace Tool grant still expires after one hour and every request revalidates its scope/window.
- Directly mounting `omnimam_appstudio_source` into Coding Runtime would violate the released SSOT and must not be added during deployment troubleshooting.

## Exact recommended next step

Configure a Coding Runtime-reachable HTTPS `OMNIMAM_MCP_PUBLIC_BASE_URL`, recreate API Server/Task Worker and the Coding Runtime, send one AppStudio coding instruction, and verify that the Invocation reaches RUNNING/ACTIVE, calls the Workspace Tool, returns an applied ChangeSet/new Source Revision, then settles to terminal/IDLE before continuing Preview and the second iteration.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
