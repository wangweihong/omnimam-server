# Handoff

## Current goal and status

- Goal: diagnose and resolve AppStudio coding-agent workspace tool registration and runtime disconnect failures so a project can be created, code generated, and iterated through follow-up chat.
- Status: complete. Readiness, stable-listener, and long-running response fixes are deployed and verified through project creation plus follow-up chat iteration.

## Work completed in this session

- Confirmed the SSOT submodule and `SSOT_VERSION` both point to released `spec-v1.21.0` commit `5a654a1c1e14c1f454e17a5b4190af379f13bb5c`.
- Reproduced OpenCode `1.18.13` behavior in two live runtimes: `/experimental/tool/ids` returns built-in tools only and never includes remote MCP tools.
- Confirmed `/mcp` reaches `connected` only after the Workspace MCP initialize and `tools/list` requests succeed; removed the invalid `/experimental/tool/ids` readiness check and retained `/mcp` connected as the readiness gate.
- Reproduced one `connection refused` in 100 direct requests against the single-connection `nc` forwarder; added transport retries for idempotent MCP connect/disconnect and auth PUT/DELETE cleanup calls.
- Built and deployed an intermediate TaskWorker, created AppStudio project `2fdc267b-6c35-5e9b-a6bc-ed2733130dec`, and confirmed the old secondary check was the only remaining failure.
- Rebuilt and redeployed TaskWorker after removing the invalid readiness check. Fresh invocation `c405f2bc-6f86-5675-a88c-d792923a820b` then passed MCP readiness but failed on `GET /session` because runtime `omnimam-e848bff1-fe5b-4066-969f-b126598c4996` refused the connection on port `14096`.
- Replaced the Coding Runtime one-shot `nc -l -e` rebind loop with BusyBox's supported `nc -lk -e` persistent listener, eliminating the proven gap between listening sockets.
- Created fresh project `1be36b1c-5e9b-5546-aba3-99c4c8061589`; its new runtime passed 200/200 direct requests without a listener gap. The invocation then failed because `/run/omnimam/forward` used `nc -w 1`, which aborted the model response after one second of idle stream time. OpenCode logged `stream` followed about 1.45 seconds later by `error=Aborted`.
- Removed the obsolete one-second upstream `nc` timeout; persistent `-lk` listener mode no longer needs it to release the listening socket.
- Rebuilt and redeployed `omnimam/infraserver:e1aa3f6-amd64` and `omnimam/taskworker:e1aa3f6-amd64`; Infrastructure is healthy and TaskWorker is running.
- Created final verification project `8dfb0e2c-a5f9-521a-8d34-3a6b82f30ab8`. Its first Coding Agent task `611bda75-508c-42fb-b8a5-6b1308f3cbad` succeeded and created `index.html` at Revision 1.
- Sent follow-up chat instruction to the same project. Task `a0f246af-7f7d-4fd4-b50a-1097ff406d40` succeeded and applied a second ChangeSet, advancing source to Revision 2.

## Files added, modified, renamed, or removed

- Modified: `backend/internal/infrastructure/providers/dockerruntime/docker.go`, `backend/internal/infrastructure/providers/dockerruntime/docker_test.go`, `backend/internal/taskworker/agentexecutor/invocation.go`, `docs/HANDOFF.md`.

## Key architectural or design decisions

- OpenCode `/mcp` `connected` is the supported remote MCP readiness signal for the pinned runtime. `/experimental/tool/ids` is not a remote MCP inventory endpoint in OpenCode `1.18.13`.
- Retry only transport failures for idempotent configuration/cleanup calls and never replay session creation or prompts.
- The Coding Runtime `14096` forwarder must keep a stable listener and its per-connection upstream proxy must allow long-running model responses. Listener persistence is provided by BusyBox `nc -lk -e`; the upstream `nc` must not use the previous one-second idle timeout.

## API, schema, dependency, or configuration changes

- No external API, schema, dependency, or configuration changes.

## Verification performed and remaining checks

- Passed after the final forwarding change: `go test ./internal/infrastructure/providers/dockerruntime` and `go test ./internal/taskworker/agentexecutor`.
- `git diff --check` passed.
- The final Coding Runtime listener remained present and passed 200/200 direct health requests without a connection refusal.
- Live AppStudio verification passed project creation, Runtime provisioning, MCP initialize and tool discovery, source ChangeSet Revision `0 -> 1`, and follow-up chat ChangeSet Revision `1 -> 2`.
- No remaining checks for the requested workflow. Full-repository tests were intentionally not run per task scope.

## Outstanding tasks

- None for the requested workflow.

## Known issues and risks

- API debug access logs currently include authorization headers. This pre-existing credential exposure was observed during diagnosis and remains a security risk outside the functional fix.
- Coding Runtime containers created before the final provider deployment retain their generated old startup script. Replace the Coding Agent in an older project before continuing that project; newly created runtimes use the fixed script.

## Exact recommended next step

The requested workflow is complete. When reopening a project created before this fix, use `替换 Coding Agent` once so it receives a new Coding Runtime with the stable forwarder.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
