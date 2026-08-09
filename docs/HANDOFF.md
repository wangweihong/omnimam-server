# Handoff

## Current goal and status

- Goal: update the pinned SSOT to released `spec-v1.20.0` and implement the SSOT-defined SSE integration.
- Status: complete; the released SSOT pin, SSE implementation, focused tests, race check, vet, and consistency checks all pass.

## Work completed in this session

- Read `skills/omnimam-server-backend/SKILL.md`, `backend/AGENTS.md`, and the applicable Go context, concurrency, error-handling, and testing skills.
- Confirmed the worktree was clean at task start.
- Confirmed `SSOT_VERSION` matches the current `ssot` submodule commit and tag before the requested upgrade.
- Fetched only the explicit `spec-v1.20.0` tag, verified its released implementation gate, checked out its exact commit, and updated `SSOT_VERSION`.
- Read the Agent/AppStudio contexts plus the v1.20.0 SSE sections and directly referenced module, runtime-fixture, and OpenAPI fragments.
- Confirmed the AppStudio and Platform Agent SSE routes and dependency injection already exist.
- Identified contract gaps: permissive cursor parsing, database-object JSON instead of the typed envelope, no terminal close, empty event payloads, and event append not advancing the Invocation cursor.
- Added the shared `invocationsse` protocol helper with canonical decimal cursor parsing, exact `id/event/data` encoding, the five-field typed envelope, and terminal classifiers.
- Updated both existing Agent and AppStudio stream handlers to replay only greater sequences, drain paged history, follow request cancellation, emit comment-only heartbeats, and close after terminal flush or after draining an already-terminal Invocation.
- Added all 12 released event constants and payload DTOs. Hermes/OpenCode execution now persists typed `invocation.started`, a stable-message `message.delta`, `message.completed`, success/cancel terminal events, while the Task terminal observer fills a typed failure terminal event when execution fails before producing one.
- Reserved stable assistant Message IDs when creating Invocations and exposed both stable Message IDs through the AppStudio facade projection.
- Made event append transactional and monotonic: the Invocation row is locked, sequence conflicts are rejected, and `last_event_sequence` advances without changing the Task resource-version fence.
- Ran `make gen.deepcopy`; retained only the new envelope DeepCopy output and removed unrelated stale generated changes produced by the repository-wide generator.

## Current in-progress work

- None.

## Files modified

- `docs/HANDOFF.md`
- `SSOT_VERSION`
- `ssot` submodule pointer
- `backend/apis/iapiserver/meta_agent_contract.go`
- `backend/apis/iapiserver/response_agent.go`
- `backend/apis/iapiserver/response_appstudio.go`
- `backend/apis/iapiserver/deepcopy_generated.go`
- `backend/internal/pkg/invocationsse/sse.go`
- `backend/internal/pkg/invocationsse/sse_test.go`
- `backend/internal/apiserver/controller/v1/agent/agent.go`
- `backend/internal/apiserver/controller/v1/appstudio/appstudio.go`
- `backend/internal/apiserver/service/v1/agent/service.go`
- `backend/internal/apiserver/service/v1/appstudio/service.go`
- `backend/internal/apiserver/store/postgresql/agent.go`
- `backend/internal/taskworker/agentexecutor/invocation.go`

## Key decisions and contract impact

- The implementation will be limited to the released v1.20.0 SSE contract and the existing target module; no uncontracted API, schema, error, permission, or event will be introduced.
- SSE connection lifetime will follow the incoming HTTP request context, replay only greater sequence numbers, send comment-only heartbeats, and close immediately after a terminal event or after draining an already-terminal Invocation.
- The existing AppStudio facade route remains the public Coding Agent boundary; no event is added to the generic UserEvent stream.
- No route, permission, error code, database column/table, migration, dependency, environment variable, or binary was added.
- A single final assistant response may be emitted as one `message.delta`; the stable Message ID still makes delta, completion event, persisted message, and Invocation projection converge on the same message.

## Verification

- Pre-change checks: `git submodule status ssot`, exact tag lookup, `git -C ssot rev-parse HEAD`, and `SSOT_VERSION` comparison.
- Passed twice: `go test -count=1 ./backend/internal/pkg/invocationsse ./backend/internal/apiserver/controller/v1/agent ./backend/internal/apiserver/controller/v1/appstudio ./backend/internal/apiserver/service/v1/agent ./backend/internal/apiserver/service/v1/appstudio ./backend/internal/apiserver/store/postgresql ./backend/internal/taskworker/agentexecutor`.
- Passed: `go test -race -count=1 ./backend/internal/pkg/invocationsse`.
- Passed: `go vet ./backend/internal/pkg/invocationsse ./backend/internal/apiserver/controller/v1/agent ./backend/internal/apiserver/controller/v1/appstudio ./backend/internal/apiserver/service/v1/agent ./backend/internal/apiserver/service/v1/appstudio ./backend/internal/apiserver/store/postgresql ./backend/internal/taskworker/agentexecutor`.
- Passed: `git diff --check`.
- Confirmed `git submodule status ssot`, exact tag, submodule HEAD, and `SSOT_VERSION.commit` all resolve to released `spec-v1.20.0` commit `0e6300c8e776df08972a229f48775ed71ad5bff9`.
- `make gen.deepcopy` completed with pre-existing unsupported-type warnings; unrelated generated changes were not retained.
- No full-repository tests were run, per task constraints.

## Known issues and risks

- Pre-existing, out-of-scope risk: workflow result reuse remains incomplete as documented by the preceding task.

## Outstanding tasks

- None for this server-side SSE task.

## Exact recommended next step

Review the server diff and coordinate the Web client to consume the AppStudio Invocation SSE endpoint with stable Message ID reconciliation.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
