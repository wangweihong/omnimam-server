# Handoff

## Current goal and status

- Goal: diagnose and fix why AppStudio agent messages contain `The MCP workspace tools aren't loaded into this session...` even though the invocation reports `SUCCEEDED`.
- Status: minimal TaskWorker fix is implemented, package-level verification passed, and the fixed image is deployed. Fresh AppStudio invocation verification remains outstanding.

## Work completed in this session

- Confirmed `ssot` and `SSOT_VERSION` are pinned to released `spec-v1.21.0` commit `5a654a1c1e14c1f454e17a5b4190af379f13bb5c`.
- Confirmed the reported English text is a persisted assistant message, not an API-generated backend error.
- Traced the message to invocation `b0bdaa8e-8f91-5b58-9147-b5aefc2269a2`, which is recorded as `SUCCEEDED`.
- Confirmed that invocation's OpenCode model session exposed built-in tools but not the four expected Workspace MCP tools.
- Confirmed the model worked around the missing tools by reading runtime configuration and issuing direct MCP JSON-RPC HTTP calls through shell scripts.
- Confirmed `configureOpenCode` enables the temporary MCP server, requests `/mcp/{server}/connect`, waits for `/mcp` status `connected`, then creates the OpenCode session.
- Confirmed cleanup intentionally disables the temporary Workspace Tool after invocation completion; the current disabled configuration is not the original failure.
- Confirmed `/experimental/tool/ids` in OpenCode `1.18.13` lists built-in tools only and cannot prove remote MCP tool availability.
- Confirmed `/experimental/tool` also lists only built-in `ToolRegistry` entries in OpenCode `1.18.13`; it does not expose session-resolved MCP tools despite its broad OpenAPI description.
- Traced OpenCode `v1.18.13` source: `PATCH /global/config` invalidates configuration and forks `disposeAllInstancesAndEmitGlobalDisposed` after returning the response. Instance disposal clears the MCP client and cached tool definitions.
- Established the failure race: TaskWorker can connect and observe `/mcp = connected`, then OpenCode's delayed global disposal removes that client before `SessionTools.resolve` assembles the model request.
- Confirmed the released SSOT requires wildcard deny plus exact allow entries for Workspace Tool IDs. Changing the wildcard to allow all tools would violate the contract.
- Added a synchronous `POST /global/dispose` barrier after `PATCH /global/config` and before Workspace MCP connect.
- Added startup transport retry coverage for the idempotent `POST /global/dispose` request.
- Built and deployed `omnimam/taskworker:811dde4-dispose-amd64`; the replacement TaskWorker started successfully.

## Current in-progress work

- None. Live invocation verification was stopped after local browser attachment and API authentication attempts did not provide a usable authenticated session.

## Files added, modified, renamed, or removed

- Modified: `backend/internal/taskworker/agentexecutor/invocation.go`.
- Modified: `docs/HANDOFF.md`.

## Key architectural or design decisions

- The exact SSOT allowlist form is authoritative: `binding-id_*: false`, with each allowed full tool ID set to `true`.
- `/mcp` status `connected` proves MCP initialization and `tools/list` succeeded, but it has not yet been proven to mean the tools are included in the next model request.
- The OpenCode global config endpoint is asynchronous with respect to instance disposal. A successful config response is not a safe point for immediately connecting instance-owned MCP state.
- `POST /global/dispose` is a supported OpenCode `1.18.13` API and synchronously waits for instance disposal; using it as a barrier preserves the SSOT configuration and exact allowlist contract.
- The backend must not accept a successful invocation when the model silently bypasses the Workspace Tool boundary through shell/HTTP fallback.

## API, schema, dependency, or configuration changes

- Runtime request sequence now includes `POST /global/dispose` between global configuration update and MCP connect. No public API, schema, dependency, or persisted configuration changed.

## Verification performed and remaining checks

- Verified the persisted message and invocation records through the local API/database/runtime paths.
- Inspected the affected OpenCode session and found no Workspace MCP tool calls.
- Passed: `go test ./internal/taskworker/agentexecutor` (package compiles; it currently has no test files).
- Passed: `git diff --check`.
- Built and deployed `omnimam/taskworker:811dde4-dispose-amd64`; container startup logs show TaskWorker workers registered normally.
- Remaining: reproduce on a fresh authenticated AppStudio invocation and inspect actual MCP tool parts.
- Full-repository tests must not be run for this task.

## Outstanding tasks

- Perform one fresh live AppStudio invocation and confirm real MCP tool parts are present instead of shell/HTTP fallback.

## Known issues and risks

- A model can currently bypass the intended Workspace Tool boundary by extracting runtime MCP authorization and calling the endpoint from shell code.
- API debug access logs include authorization headers. Do not reproduce credentials in source, tests, logs, or this handoff.
- Existing Coding Runtime containers may retain startup scripts generated before the preceding forwarder fix.
- `backend/AGENTS.md` prohibits adding `_test.go` files outside `pkg/`; no focused request-order unit test was added to `internal/taskworker/agentexecutor`.
- Live verification is incomplete: the in-app browser could not attach to the local page, Basic Auth was rejected, and the deployed Identity login uses a two-step authenticated flow.

## Exact recommended next step

Submit one fresh AppStudio invocation through an already authenticated UI session, then verify its OpenCode message parts include the four `omnimam-workspace_*` tools and no shell/HTTP MCP fallback.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
