# Project Handoff

## Current goal and status

- Goal: implement the released Agent MCP end-to-end contracts: Binding PUT/DELETE, immutable revisions, Runtime Grants, Infrastructure/OpenCode injection, AppStudio default platform Binding, and `AGENT_WORKLOAD` MCP authentication.
- Status: implementation and focused verification complete for the released `spec-v1.19.0` contracts. External non-empty `credential_ref` resolution remains intentionally fail closed because the released SSOT does not define a persistent Secret resolver protocol.
- SSOT: released `spec-v1.19.0`, tag/commit `aa3f843e6ad4987f0441d882bfa0d05e02e05065`; `SSOT_VERSION` matches the `ssot` submodule.

## Work completed in this session

- Implemented owner-isolated MCP Binding List/Create/PUT/Delete with optimistic locking, active-name conflict handling, `KEEP/SET/CLEAR`, credential hiding, soft deletion, and immutable revision snapshots.
- Added `agent_mcp_binding_revisions` and `agent_runtime_grants` models/schema registration, plus Agent Store create/get/revoke Grant methods.
- Runtime ensure selects enabled, non-deleted Bindings in stable Binding-ID order, rejects more than 50, persists exact revision refs before enqueue, and revokes the Grant on enqueue failure.
- Agent Executor converts `mcp_binding_refs` into Infrastructure `MCP_SERVER_REF` configuration bindings and preserves `authorization_ref`.
- Infrastructure and Docker paths resolve MCP configuration, use an OpenCode startup gate, write `/root/.config/opencode/opencode.json` through Docker Archive/Exec into tmpfs, set mode `0600`, and release the gate.
- AppStudio initialization and generation replacement transactions create the default platform Binding and revision. Current-generation lookup performs idempotent backfill and does not recreate a same-generation Binding after soft deletion.
- Added initial Identity claim/principal fields for `AGENT_WORKLOAD` and an MCP Dispatch Grant check with the fixed minimum workload permission set.
- Added neutral `internal/pkg/agentmcp` resolver contract; Infrastructure now consumes the injected Agent Service resolver and uses `APISERVER_MCP_PUBLIC_BASE_URL` for the platform endpoint. Removed the Infrastructure-side Agent Store resolver.
- Added dedicated runtime-scoped `AGENT_WORKLOAD` JWT signing. Authentication no longer requires USER credential/session rows and requires `aud=mcp`, Agent, generation, Application, Runtime, and Grant claims.
- Coding Runtime Grants now persist the current AppStudio Application/generation. MCP revalidates Grant status, exact scope, current generation, immutable platform Binding revisions, and exact allowed-tool union on every request, then injects only the resolved owner boundary for downstream object isolation.
- OpenCode config now emits fixture-compatible `tools` entries: `<binding_id>_*: false` plus exact allowed tool entries, so an empty allowlist denies all.
- Runtime Grant creation now reuses only an active, scope-identical row for `(runtime_binding_id, request_id)` and rejects expired, revoked, or scope-conflicting reuse.
- Runtime ensure now selects and validates the stable Binding revision set before Runtime creation, avoids pre-Task mutation of an existing Runtime, generates a deterministic authorization ref, and resumes only a canonical idempotent Task whose full arguments and lifecycle fence match.
- Failed ensure revokes its exact Grant only after the terminal projection fence applies. Successful stop/suspend/delete revokes all active Grants for that Runtime; stale terminal projections do not revoke Grants.
- Agent schema startup now checks active same-name Binding conflicts before `AutoMigrate`, then creates the active-name unique index and backfills immutable revisions.
- `AGENT_WORKLOAD` JWT validation now requires the exact single audience `mcp`.
- OpenCode configuration injection is limited to `agent.coding@1.0`; Hermes remains outside the MCP injection path.
- Declared and exported `OMNIMAM_MCP_PUBLIC_BASE_URL` in `scripts/install/environment.sh` with the same local default used by Compose.
- Binding List/Delete now verify parent Agent ownership first: cross-owner access maps to `ERR_AGENT_NOT_VISIBLE`, while deletion of a missing Binding under a visible Agent remains idempotent.
- Binding PUT now requires a positive `resource_version`, and `credential_ref` retains the released length bound.
- Infrastructure resolver construction now consumes the already validated Auth/MCP config instead of rereading environment variables.
- Runtime ensure now supplies the released 30-minute idle timeout and 8-hour maximum lifetime; Runtime Grants and static workload JWTs share the remaining 8-hour Grant boundary.
- MCP endpoint resolution now fails closed for `REMOTE` and `RUNTIME_LOCAL` until their released trusted registries/resolvers exist; `PLATFORM` resolves only through trusted platform configuration.
- Concurrent active Binding-name unique violations now map to `ERR_AGENT_MCP_BINDING_NAME_CONFLICT`.
- MCP Binding `configuration` now accepts only a JSON object, recursively rejects secret-bearing and resolver-owned keys, and is revalidated both when resolving historical revisions and before Docker rendering.
- Docker now merges validated non-sensitive Binding configuration into each OpenCode server entry while keeping trusted `type`, `url`, `enabled`, and runtime-only `headers` authoritative.
- Infrastructure now transitions an already-persisted `PREPARING` Runtime to `FAILED` on MCP resolver absence/failure/invalid output and returns a sanitized resolution error.
- AppStudio now requires the platform Binding backfill capability on its Coding Agent port and invokes it on canonical create retries as well as current-generation access; create/replace still write Binding and revision in their existing aggregate transaction.
- Centralized released MCP server types, credential modes, platform endpoint ref, and the 50-Binding Runtime limit in `meta_agent_contract.go`; MCP request dispatch also rejects persisted Grants exceeding that limit.
- Completed MCP workload authorization review: every request revalidates active Grant state, expiry, Agent, Runtime, Application, generation, exact historical revisions, owner scope, fixed minimum permissions, and the effective tool allowlist.

## Current in-progress work

- None for the released Agent MCP scope.

## Files modified or added

- `SSOT_VERSION`, `ssot` gitlink
- `backend/apis/iapiserver/meta_agent.go`
- `backend/apis/iapiserver/meta_agent_contract.go`
- `backend/apis/iapiserver/request_agent.go`
- `backend/apis/iapiserver/request_infrastructure.go`
- `backend/internal/apiserver/controller/v1/agent/agent.go`
- `backend/internal/apiserver/middleware/identity_v11.go`
- `backend/internal/apiserver/middleware/identity_v11_test.go`
- `backend/internal/apiserver/route.go`
- `backend/internal/apiserver/server.go`
- `backend/internal/apiserver/service/v1/agent/service.go`
- `backend/internal/apiserver/service/v1/agent/mcp_resolver.go`
- `backend/internal/apiserver/service/v1/appstudio/service.go`
- `backend/internal/apiserver/service/v1/mcp/service.go`
- `backend/internal/apiserver/store/postgresql/0_pg.go`
- `backend/internal/apiserver/store/postgresql/agent.go`
- `backend/internal/apiserver/store/postgresql/appstudio.go`
- `backend/internal/apiserver/store/store.go`
- `backend/internal/infrastructure/app.go`
- `backend/internal/pkg/agentmcp/resolver.go`
- `backend/internal/pkg/agentmcp/configuration.go`
- `backend/internal/pkg/agentmcp/configuration_test.go`
- `backend/internal/infrastructure/providers/basic.go`
- `backend/internal/infrastructure/providers/dockerruntime/docker.go`
- `backend/internal/infrastructure/service.go`
- `backend/internal/taskworker/agentexecutor/executor.go`
- `backend/internal/pkg/code/release_v119.go`
- `docs/HANDOFF.md`
- `scripts/install/environment.sh`

## Architectural and contract decisions

- Infrastructure receives only `MCP_SERVER_REF` and `authorization_ref`; an Agent-owned adapter resolves authorized immutable revisions. Worker, Infrastructure, and Docker do not read Agent tables.
- Binding changes affect the next Runtime start/recover/rebuild and do not interrupt an existing container. A valid pre-delete Grant may resolve its historical revision.
- Platform MCP workload permissions are fixed to protocol access plus capability/application read, application run, and asset read. They never inherit creator/admin permissions and exclude cancel/upload/delete.
- Secrets and workload JWTs must not enter Task arguments/results, revision snapshots, logs, environment variables, container command, or inspect-visible fields.
- OpenCode uses Binding ID as server key; empty `allowed_tools` denies all tools. Hermes MCP injection remains out of scope.
- No new `backend/cmd/` binary or parallel environment file is permitted.

## Verification performed and remaining checks

- Passed direct final-patch verification: `go test ./backend/apis/iapiserver ./backend/internal/apiserver/service/v1/agent ./backend/internal/apiserver/service/v1/appstudio ./backend/internal/apiserver/service/v1/mcp`.
- Passed complete focused verification: `go test ./backend/internal/apiserver/middleware ./backend/internal/apiserver/service/v1/identity ./backend/internal/apiserver/service/v1/mcp ./backend/internal/apiserver/service/v1/agent ./backend/internal/apiserver/service/v1/appstudio ./backend/internal/apiserver/store/postgresql ./backend/internal/infrastructure ./backend/internal/infrastructure/providers/dockerruntime ./backend/internal/taskworker/agentexecutor ./backend/internal/pkg/agentmcp`.
- Passed configuration package verification separately: `go test ./backend/internal/pkg/agentmcp`.
- Passed `bash -n scripts/install/environment.sh` and `git diff --check`.
- No full-repository test was run, per repository constraints. Several target packages have no test files; coverage there is compile-level through `go test` plus the permitted existing focused tests.

## Known issues and risks

- The Agent-owned resolver currently has no trusted credential resolver for non-empty `credential_ref` values.
- Released SSOT requires a trusted Secret/Identity resolver but does not define an implementable persistent Secret API/store protocol. Non-empty external `credential_ref` therefore fails closed; it must not be interpreted as plaintext or mapped to an ad hoc environment variable.
- Direct Runtime Grant recovery/revocation, AppStudio transaction, and Docker injection assertions remain limited because repository rules prohibit adding new `_test.go` files outside `pkg/`; the affected target packages compile successfully.

## Exact recommended next step

Review the completed focused diff, then stage or commit it together with the already pinned released SSOT update. Do not implement plaintext or environment-based credential resolution without a released Secret resolver contract.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
