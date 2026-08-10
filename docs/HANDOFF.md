# Handoff

## Current goal and status

- Goal: make the Workspace MCP endpoint reachable from the separate Coding Runtime over HTTPS, rebuild the affected services, replace the Coding Agent so it receives a fresh enabled MCP configuration, and restore Source Revision advancement.
- Status: in progress (resumed 2026-08-10). Source Snapshot and live Preview content pass. Generations 14 and 15 reached the internal Workspace Tool route and completed initialize/initialized/tools-list POSTs, but the `/experimental/tool/ids` gate still failed. Generation 15 exposed the immediate deployment issue: the Runtime TCP forwarder listened on IPv6 only, while Task Worker connects over Docker IPv4, so the OpenCode endpoint was not reachable reliably. Queued cancellation still leaves the Invocation at `CANCELING` although its AtomicTask is `CANCELED`. Timeout verification has not started.
- SSOT: released `spec-v1.20.0` at `0e6300c8e776df08972a229f48775ed71ad5bff9`; `ssot` submodule and `SSOT_VERSION` match.

## Current in-progress work

- Run `make compose` with the persistent internal Workspace Tool SSE stream, replace the Coding Agent Runtime, and verify the next live Source iteration; the executor still requires all four Workspace tool IDs after MCP connection.
- Trace terminal projection for an already `CANCELED` AtomicTask and identify the narrowest fix before timeout verification.

## Work completed in this session

- Reproduced the Generation 5 failure live: HTTPS/CA/network injection worked, but OpenCode exposed no Workspace MCP tools. Its `/mcp` status showed the MCP client could not create `/root/.local/state/opencode/locks` on the read-only root filesystem.
- Deployed `omnimam/apiserver:mcpfix-amd64`, `omnimam/infraserver:mcpfix-amd64`, and `omnimam/taskworker:mcpfix-amd64`; verified the new Coding Runtime is read-only except for the intended tmpfs paths, trusts only the injected public MCP CA, and can reach `https://apiserver:8443` through OpenCode.
- Verified live Invocation `d8d0096c-8bc8-428f-a1a0-0daa3f624b42` succeeded and applied ChangeSet `f7142905-2f53-4cc6-b77d-c74512bed049`, advancing Workspace `dad813a2-d63f-5393-92b2-b2abf8843d7f` from Revision `0` to `1`.
- Verified the successful Invocation's Assistant result names the applied ChangeSet and resulting Revision, while Agent Runtime Binding `c1a030f6-082b-442a-a4de-3ef56827d1e2` returned to `READY / IDLE / HEALTHY` with cleared task/operation ownership.
- Restarted API Server and verified persistence: the local MCP CA SHA-256 fingerprint was unchanged, all three Revision `1` source-file SHA-256 values were unchanged, and Workspace state remained `READY` at Revision `1`.
- Logged into the Web UI, confirmed Revision `1`, Generation `6`, the successful Agent result, and `IDLE` projection, then created READY Source Snapshot `8ab49569-37e3-4d9a-9619-4768f03e6fc8` for Revision `1`.
- Added a focused Docker Runtime fix and regression test for static-web Preview: retain read-only rootfs and the image's original command, mount writable tmpfs at `/var/cache/nginx` and `/var/run`, and restore only Nginx's required `CHOWN`, `SETGID`, and `SETUID` capabilities. The isolated `nginx:1.27-alpine` probe and focused package tests pass.
- Added the missing Preview content handoff: Task Worker now declares one read-only `STUDIO_WORKSPACE_REVISION` mount; Docker Runtime validates the Workspace/Preview grant scope, exposes only the named volume's matching Revision subpath, copies it into a dedicated Nginx content tmpfs on every container start, and then executes the image's original entrypoint/command.
- Added cross-scope/path-traversal regression cases and made the Compose Source volume name explicit so Docker Runtime can resolve it without exposing the full canonical volume.
- Built and deployed `omnimam/infraserver:mcpfix-amd64` and `omnimam/taskworker:mcpfix-amd64`, stopped default-page Preview `cb94e93a-975c-4efe-9ef4-73081965c990`, and created Preview `92da3af6-47c2-44d6-9a81-befda9b5e97d` for Revision `1`.
- Verified live Docker Runtime `1b16a2ad-f2a6-4230-9293-744d5a784885`/container `d9d5fdb75c31`: source mount is `RW=false` with subpath `dad813a2-d63f-5393-92b2-b2abf8843d7f/1`, rootfs remains read-only, capabilities remain minimal, and the served page is the Revision `1` “创意灵感看板” before and after a container stop/start.
- Ran deterministic queued cancellation Invocation `6276b592-e9d1-4c7c-940d-c493d9c05ef9`: its AtomicTask `f4a9f593-63be-49f7-8abe-eb9cca0154d8` reached `CANCELED`, Agent returned to `IDLE`, but Invocation remained `CANCELING` after 30 seconds.
- Ran second-iteration Invocations `33976cae-4499-4a19-9359-9e2b317ceabc`, `198105f5-2e8f-4f45-8f0d-9431e6c9c695`, and `9da9fc79-379f-4fb2-a8a7-f55db4a0ee6b`; all correctly failed the no-ChangeSet success gate and left Source Revision at `1` because the reused OpenCode session exposed no Workspace Tool.
- Added and deployed explicit OpenCode MCP connect/disconnect plus a bounded `/mcp` connected-state wait around each Invocation. Target Task Worker tests/vet pass, but the final allowed live retry still lacked tool IDs; this change is partial and must not be described as resolving Runtime reuse.
- Added a deterministic `/experimental/tool/ids` readiness gate in `backend/internal/taskworker/agentexecutor/invocation.go`; Invocation setup now refuses to prompt until all four Workspace tool IDs are present.
- Added grant-validated `GET /internal/appstudio/workspace-tool` SSE acknowledgement for OpenCode's remote MCP transport probe; the released public `POST /mcp` route remains unchanged.
- Changed the internal Workspace Tool SSE GET acknowledgement to remain open until Runtime disconnect and emit a 15-second heartbeat; the one-shot response caused OpenCode to reconnect without registering tools.
- Generation 15 live retry confirmed the Runtime forwarder was IPv6-only (`/proc/net/tcp6` had `:3710`, `/proc/net/tcp` did not), blocking Task Worker IPv4 access to the OpenCode endpoint. The next implementation step is to bind the listener explicitly to `0.0.0.0`.
- Added the missing writable OpenCode state tmpfs to the Coding Runtime profile and extended the existing Docker Runtime test.
- Diagnosed Generation 7 Invocation `a3c61daa-51bf-4da6-81a8-9d0fed0bbfd9`: Task Worker `ReconcileQueuedInvocations` used an Agent service with `Workspaces == nil`, so its retry path returned `appstudio workspace tool is unavailable` before task binding. Injected a Task Worker-local AppStudio service backed by the shared Source store and grant codec into the Agent projector.
- Fixed Coding Runtime's TCP forwarder command in `backend/internal/infrastructure/providers/dockerruntime/docker.go`: removed `nc -lk`, which accumulated orphaned per-connection processes under the non-reaping Runtime PID 1, and replaced it with a shell loop that runs one `nc -l` connection at a time. Added a focused regression assertion in `docker_test.go`.
- Hardened the forwarder loop to tolerate transient `nc`/upstream failures under `set -e` by sleeping briefly and continuing to accept the next connection; the previous live Generation 8 attempt failed during Runtime health startup before any Invocation task binding.
- Added an initialize-handshake compatibility path for the internal grant-authenticated Workspace Tool (`initialize`, `notifications/initialized`, `ping`, `tools/list`, `tools/call`) while preserving the strict released public `/mcp` 2026-07-28 path.
- Passed focused tests and `go vet` for MCP, AppStudio Workspace Tool/controller, Docker Runtime, and Infrastructure Server, then rebuilt `omnimam/apiserver:mcpfix-amd64` and `omnimam/infraserver:mcpfix-amd64`.
- Unified the local Compose MCP Origin as `https://apiserver:8443` across API Server, Infrastructure Server, Task Worker, and Notification Worker; removed Infrastructure Server's remaining hard-coded loopback Origin.
- Added persistent local-development API Server TLS generation in the shared `omnimam_mcp_tls` volume. The generated leaf certificate is scoped to `apiserver`/localhost and the private key is never mounted into Coding Runtime.
- Added Coding Runtime CA loading/validation and pre-start stdin injection. New Coding Runtime containers pin `NODE_EXTRA_CA_CERTS=/run/omnimam/mcp-ca.crt`, while the public CA bundle is stored only in Runtime tmpfs.
- Added focused Docker Runtime tests for CA bundle validation and protection against a configuration binding overriding the trusted CA path.
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

- Investigate why OpenCode reports MCP transport readiness before Workspace tool IDs are visible to a reused Session, without running another Source iteration until a deterministic readiness/session test exists. Separately repair queued-cancellation terminal projection. Preview implementation and live verification are complete.

## Files added, modified, renamed, or removed

- Added: `backend/internal/apiserver/service/v1/appstudio/workspace_tool.go`.
- Modified API/internal contracts: `backend/apis/iapiserver/meta_agent_contract.go`, `backend/apis/iapiserver/request_appstudio.go`, `backend/apis/iapiserver/request_infrastructure.go`, `backend/pkg/mcp/types.go`, `backend/internal/pkg/agentgrant/codec.go`.
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
- Keep the released public `/mcp` transport strict. OpenCode 1.18.13 does not expose the 2026-07-28 `server/discover` tools, so the Invocation-grant-authenticated internal Workspace Tool route will additionally adapt the standard initialize lifecycle (`initialize`, `notifications/initialized`, `tools/list`, `tools/call`) to the existing Workspace dispatcher without adding a public API or changing Workspace semantics.
- Preview uses Docker API v1.45+ volume-subpath mounts to expose only the authorized immutable `workspace/revision` directory read-only. Nginx serves a per-container tmpfs copy so canonical Source remains immutable and Nginx workers can read content without weakening Source volume permissions.

## API, schema, dependency, or configuration changes

- No public API, database schema/table/field, migration, permission, public event type, business error code, task schema, or dependency changes.
- Added one internal grant-authenticated MCP route and internal grant claim type.
- Added `AgentRuntimeActivityActive = "ACTIVE"`, which is projected from existing Invocation lifecycle facts.
- Agent grant TTL is now one hour in API Server and Task Worker.
- Added `OMNIMAM_APPSTUDIO_SOURCE_DIR`, defaulting to `/var/lib/omnimam/appstudio-source`, and Compose volume `omnimam_appstudio_source`.
- Added `OMNIMAM_APPSTUDIO_SOURCE_VOLUME`, defaulting to the explicit Compose volume name `deployments_omnimam_appstudio_source`; local Docker API default is now v1.45 because volume subpath requires v1.45 or newer.
- API Server and Task Worker now consume the same `OMNIMAM_MCP_PUBLIC_BASE_URL`; local startup defaults to loopback HTTP, while a remotely reachable Coding Runtime requires an explicit HTTPS Origin.

## Verification performed and remaining checks

- Passed focused tests for `internal/apiserver`, AppStudio controller/service, Agent service, Agent executor, Task Worker, PostgreSQL store, agent grant package, and MCP package.
- Passed `go test ./backend/internal/taskworker` after the queue-reconciliation dependency fix, plus `git diff --check`.
- Passed focused `go vet` for AppStudio service, Agent service, Agent executor, PostgreSQL store, and MCP package.
- Passed `docker compose -f deployments/docker-compose.yaml config --quiet`.
- Passed `bash -n scripts/install/environment.sh` and `git diff --check`.
- Rendered Task Worker configuration contains `APISERVER_MCP_PUBLIC_BASE_URL=http://127.0.0.1:8080`.
- Recreated Task Worker and observed it continuously `running` with restart count zero; startup logs no longer contain `remote MCP public base URL and allowed origins must use HTTPS`.
- No full-repository tests were run, per repository scope rules.
- Live Generation 5 verification proved Runtime HTTPS reachability but failed before the OpenCode state tmpfs and initialize compatibility fixes.
- Live Generation 6 verification passed Tool discovery/call, ChangeSet apply, and Source Revision advancement (`0 -> 1`).
- Live Generation 6 result fields and terminal/idle projection also passed.
- API Server restart persistence passed for both TLS trust material and immutable Revision `1` source content.
- Preview `92da3af6-47c2-44d6-9a81-befda9b5e97d` passed `RUNNING/HEALTHY/READY`, exact read-only Revision subpath isolation, real Revision `1` HTML delivery, and container stop/start content reconstruction.
- Remaining external verification: Compose rebuild with persistent SSE, fresh Runtime/Session replacement, second source iteration, cancellation terminal projection, and timeout behavior.

## Outstanding tasks

- Verify the persistent internal SSE stream with a fresh Runtime/Session and the second Source iteration; then repair queued-cancellation terminal projection and verify timeout behavior. Preview, Snapshot, restart persistence, and initial Revision advancement are verified.
- If the Web client must disable Snapshot before submission rather than display the server business error, implement that presentation-only preflight in the Web repository; this server repository already enforces the prerequisite.

## Known issues and risks

- The current local Compose topology is verified with the Runtime-reachable `https://apiserver:8443` Origin and generated local CA. Non-Compose deployments must still provide an equivalent reachable HTTPS Origin and trusted public CA.
- Runtime-local Tool cleanup is best-effort after process-level crashes; the encrypted Workspace Tool grant still expires after one hour and every request revalidates its scope/window.
- Generations `14` and `15` logs show OpenCode POSTed initialize/initialized/tools-list successfully and received `200` from `GET /internal/appstudio/workspace-tool`; the persistent stream is present, but the Runtime forwarder currently binds IPv6-only and the tool-ID gate still times out. Explicit IPv4 binding is pending rebuild and live confirmation.
- Task Worker queue reconciliation previously lacked the AppStudio Workspace grant issuer; this is fixed in source but requires a Compose image rebuild before live retry.
- Generation 9 Runtime `3d47b610-e537-4728-b710-009aa14a4269` and Generation 10 Runtime `992ca08a-a3be-4de3-ab76-a92c345b55bb` were created during live verification. Both reached OpenCode, but `/auth` could still race with or follow an HTTP keep-alive health connection while the single listener was occupied. The next image includes both `/proc/net/tcp` readiness gating and `nc -w 1`; live verification must be repeated after rebuild.
- Queued cancellation currently leaves Invocation `6276b592-e9d1-4c7c-940d-c493d9c05ef9` at `CANCELING` even though its AtomicTask is terminal `CANCELED`; Agent projection is already `IDLE`. Do not count this cancellation case as passed.
- Directly mounting `omnimam_appstudio_source` into Coding Runtime would violate the released SSOT and must not be added during deployment troubleshooting.
- Debug access logs currently include complete `Authorization` header values, including Agent JWTs and short-lived Workspace Tool grants. Fixing the shared logging middleware is outside this task's allowed target-module scope; treat captured logs as sensitive and schedule a focused redaction task.
- Historical Preview rows `43d6c0dc-44a9-434e-9f6f-f73971384c58` and `23d9ca23-cc09-4d0c-ac4c-5c5c2b475f11` may remain falsely persisted as `RUNNING/READY` although their old Docker containers exited. The current Preview is correct; a separate reconciler repair is still needed if historical row cleanup is required.

## Exact recommended next step

Bind the Coding Runtime forwarder to `0.0.0.0`, run focused tests, execute `make compose`, replace/recreate the Coding Agent through the Web UI, and perform the next allowed live Source iteration. Then trace why Agent terminal projection does not consume the already `CANCELED` AtomicTask before attempting timeout verification.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
