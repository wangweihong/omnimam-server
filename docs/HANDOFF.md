# Handoff

## Current goal and status

- Goal: publish `spec-v1.23.4`, version the `agent.runtime.ensure` Runtime Git access contract, expose sanitized Attempt failure reasons, then deploy and retry the existing AppStudio reservation end to end.
- Status: in progress; root cause and implementation plan are confirmed, with SSOT release work starting before server changes.

## Work completed in this session

- Resumed the live checkpoint, reloaded the mandatory backend/Go troubleshooting and testing guidance, and reconfirmed released `spec-v1.23.3` pin consistency.
- Re-ran the target GitLab and AppStudio service package tests plus Compose configuration validation; all passed.
- Rebuilt and restarted the Compose backend successfully with the PAX archive fix included; persistent data and the existing frontend container were preserved.
- Browser retry created fenced DAG `4e05ffc2-0515-5938-be71-7cb757d3d45d`; the previous PAX `no project root` failure disappeared, exposing a second archive validation bug: GitLab's normal generated root directory becomes an empty relative path after stripping the root segment and was rejected before the directory skip.
- Added a narrow root-directory classification guard before path cleaning; regular root files, nested directories, symlinks, traversal paths, types, and size limits remain subject to the existing validation.
- Final archive acceptance DAG `dabb5c97-c9aa-50fe-a215-c39268e56a0d` proved Finalize succeeds on its first attempt. Invocation then reached Agent runtime startup and exposed a separate Task Worker bootstrap omission: unlike API Server, its Agent service did not receive `Scopes: appStudioService` and failed closed with `coding agent workload scope resolver is unavailable`.
- Added the missing existing `WorkloadScopeResolver` injection to the Task Worker Agent service construction; no interface or contract change was introduced.
- After the resolver fix, retry DAG `2f61ac1c-8a89-5494-9b41-31a43b2f9c96` exposed a Finalize replay gap: revision 0 had already been committed by the prior DAG, but Finalize attempted the same deterministic primary key again.
- Finalize now reuses an existing canonical revision only when revision number, commit SHA, and blueprint content digest all match; missing revisions follow the original create path and mismatches remain rejected.
- Final retry reached Runtime Git access; GitLab access logs proved all five project-token creates returned `201`, but GitLab 19.2 returns `user_id` rather than `username` in the token payload. The client now resolves `/users/:user_id` only when username is absent, preserving older response compatibility and revoking an unusable token if username resolution fails.
- After Git access resolution succeeded, recovery hit a previously revoked Runtime grant because the unbound `STARTING` runtime kept the same version-derived authorization request. Coding Invocation runtime startup now supplies a deterministic request identity from invocation ID plus submission generation, so each explicit submission retry receives an independently revocable grant while already-bound runtime tasks remain protected by the existing current-task fence.
- Final DAG `0762b5d0-c68e-5a21-b6af-f5ea64054e75` proved Project, Webhook, and idempotent Finalize succeed on attempt 1 and Invocation reaches Runtime Task submission. Task Center then rejected `runtime_git_access_ref` because the released function schema omits it under `additionalProperties: false`.
- Published upstream `spec-v1.23.4`: content commit `0da3dd236d687643e0b34a71cc115b51f5de485f`, release commit `e3e00349604d9700caba40c3c7f68ecf5cbc22ab`, remote annotated tag peeled to the release commit.
- The release retains `agent.runtime.ensure@1.0` unchanged and makes `@1.1` ACTIVE with conditional Coding Runtime Git access plus sanitized Attempt failure diagnostics.
- Existing `go generate ./backend/internal/taskfunctionregistry` failed because its directive resolves `../../../..` one level above the repository; use the same generator explicitly as `go run ./backend/internal/taskfunctionregistry/internal/generate -root .` for this task. The directive itself remains unchanged as out of scope.
- Updated the server gitlink/`SSOT_VERSION` to release commit `e3e00349604d9700caba40c3c7f68ecf5cbc22ab` and regenerated the embedded function registry/source metadata.
- Added scoped compatibility coverage: new Coding inputs select `agent.runtime.ensure@1.1`, missing/invalid Runtime Git refs fail schema validation, and retained `1.0` resolves with its original digest.
- Attempt failures now log the concrete error through the existing sanitizer; sensitive references are additionally redacted alongside credentials, authorization, URLs, control whitespace, and oversized messages.
- Revoked all 5 active `omnimam-runtime-*` GitLab project access tokens left by failed browser acceptance attempts; no plaintext token was logged or persisted by this cleanup.
- Resumed from the live checkpoint, re-read the mandatory backend skill and `backend/AGENTS.md`, and verified the working tree still pins released `spec-v1.23.3` commit `6e292e8387e35c678c8bdc94777a8d19d6e5e59c` consistently with `SSOT_VERSION`.
- Ran `make compose` successfully; rebuilt all server binaries/images and restarted the Compose stack while preserving the existing frontend container and persistent data.
- Browser acceptance reached the new four-stage initialization detail for the existing failed reservation and confirmed the frontend renders 25% progress, stage states, attempt counts, failure time, DAG link, and retry control.
- Configured the existing local GitLab through the frontend using the devops bootstrap PAT; the temporary readable credential copy was deleted immediately after form submission.
- Diagnosed GitLab connection checks failing because gotoolbox `WithPath` replaced the `/api/v4` path in `GitLabServer.api_url`; updated all GitLab HTTP request builders to preserve the configured API base path and added a regression test in the existing GitLab test file.
- Added `host.docker.internal:host-gateway` to the API Server and Task Worker Compose services so both GitLab consumers can reach the separately managed local GitLab without requiring an external Docker network to exist.
- Browser retry created DAG `85e0d674-e551-55a2-93ed-720b1a02a44c`; GitLab Project and Webhook completed successfully, proving reservation reuse and the GitLab path/network fixes. Finalize exhausted five attempts with `appstudio starter commit is unavailable`, leaving Invocation blocked and the application safely in `ERROR` at 50%.
- A diagnostic retry exposed the precise finalize error: `gitlab repository archive entry has no project root`. Raw archive inspection confirmed GitLab prepends a standard `pax_global_header`; the source archive parser now skips only standard PAX/GNU metadata headers before applying the existing path/type/size safety checks.
- Reproduced failed application `366897bd-925e-55d7-9a7b-8da416731434`.
- Confirmed initialization DAG `55ba067a-ff42-56dd-a6a7-3a502741ff22` failed at `appstudio.initialization.project.ensure` after five attempts.
- Confirmed PostgreSQL has no GitLabServer, so no READY AppStudio default exists.
- Confirmed the current worker replaces the root cause with an unrelated source-change error and the application API exposes only `ERROR`.
- Rechecked `git submodule status ssot`, `SSOT_VERSION`, local tags, and remote tags; all current server inputs resolve to `spec-v1.23.2` commit `9e1bf2291dd1925e982a5dd728e05a27c334f8d9`, and `refs/tags/spec-v1.23.3` is absent upstream.
- Loaded the repository backend implementation rules and mandatory Go API/error/testing guidance before implementation.
- Published upstream `omnimam-spec` branch `codex/spec-v1.23.3` and annotated tag `spec-v1.23.3`; remote tag resolves to release commit `6e292e8387e35c678c8bdc94777a8d19d6e5e59c` and records content commit `31c9be36314153ea4f24be39866c93d5c295d7d2`.
- Updated the server `ssot` gitlink working tree and `SSOT_VERSION` target to `spec-v1.23.3`.
- Added initialization GET/retry DTOs, routes, controller/service methods, fixed four-stage safe DAG aggregation, retry DAG derivation, atomic retry begin/conditional rollback, and current-DAG owner fence.
- GitLab SourceProvider now preserves the default-server, connection, remote Project, and projection business errors; AppStudio initialization no longer rewrites provider failures to `ERR_APPSTUDIO_SOURCE_CHANGE_REJECTED`.
- Added `ERR_GITLAB_APPSTUDIO_DEFAULT_SERVER_UNAVAILABLE` (`250204`) and regenerated error/deepcopy code.
- Extended existing tests for missing default Server, four-stage aggregation/sensitive filtering, and old-DAG fencing without adding a new test file.
- Re-ran the error/deepcopy generators and scoped AppStudio, GitLab, PostgreSQL store, controller/route, and error-code package tests; all passed.

## Current in-progress work

- Building the target binaries/images, restarting only API Server and Task Worker, then retrying the existing AppStudio initialization.

## Files added, modified, renamed, or removed

- Modified: `docs/HANDOFF.md`.
- Modified: `SSOT_VERSION` and `ssot` gitlink target in the working tree.
- Modified: AppStudio API DTO/controller/route/service/store, GitLab SourceProvider, error code sources/generated code, generated deepcopy code, and existing target tests.
- Modified: `backend/internal/apiserver/service/v1/gitlab/client_gitlab.go` and existing `source_provider_test.go` to preserve `/api/v4` in outbound GitLab requests.
- Modified: `backend/internal/apiserver/service/v1/gitlab/source_provider.go` and its existing test to accept standard tar metadata without weakening symlink/path/content validation.
- Modified: `deployments/docker-compose.yaml` to provide a stable local GitLab host route to API Server and Task Worker.
- Added: `backend/internal/pkg/code/release_v123.go`.
- Existing unrelated untracked documents under `docs/` remain untouched.

## Key architectural or design decisions

- Explicit retry only; no background recovery scanner.
- Reuse the existing reservation and initialize through a newly fenced DAG.
- Expose a safe AppStudio-owned diagnostic projection rather than raw Task Center payloads.

## API, schema, dependency, or configuration changes

- Added initialization GET/retry endpoints and GitLab default-server error code `ERR_GITLAB_APPSTUDIO_DEFAULT_SERVER_UNAVAILABLE` from released `spec-v1.23.3`.
- No database schema change planned.

## Verification performed and remaining checks

- Passed: `make compose`; PostgreSQL, Redis, Conductor, and InfraServer reached healthy status and the API/worker containers started.
- Passed: `go test ./backend/internal/apiserver/service/v1/gitlab` including the `/api/v4/version` regression test.
- Passed again after the PAX fix: target GitLab and AppStudio service package tests.
- Passed after all current fixes: `go test ./backend/internal/apiserver/service/v1/agent ./backend/internal/apiserver/service/v1/gitlab ./backend/internal/apiserver/service/v1/appstudio ./backend/internal/taskworker`.
- Browser/DAG acceptance: Project, Webhook, and Finalize all succeed on first attempt; Invocation reaches `agent.runtime.ensure` contract validation.
- Passed: `docker compose -f deployments/docker-compose.yaml config -q` after the host-gateway change.
- Browser reproduction, container logs, and target database rows inspected.
- Release gate verification: `git fetch --tags origin` and targeted remote ref checks confirmed `spec-v1.23.3` is not published.
- Release verification now complete: remote `refs/tags/spec-v1.23.3^{}` equals `6e292e8387e35c678c8bdc94777a8d19d6e5e59c`.
- Passed: AppStudio service, GitLab service, PostgreSQL store, AppStudio controller, API server, and code packages; targeted integration test compiles/runs under the existing DSN gate.
- Known unrelated failure: Task Center `TestAssignSystemName` expects `thumbnail 表现形式` but current output is `thumbnail视图`; not modified under task scope.
- Remaining after a new SSOT release: sync the embedded function registry/digest, rebuild Task Worker, and repeat browser acceptance through Runtime startup.

## Outstanding tasks

- In `omnimam-spec`, add the required opaque `runtime_git_access_ref` field and conditional Coding Agent requirements/secret-resolution mapping to `agent_runtime_ensure_input`, update the contract digest and directly affected S1/S2 references, release a new pinned spec version, then update this server repository.

## Known issues and risks

- Existing failed reservation references an old terminal DAG; retry must fence late projections.
- Do not expose credentials, internal workspace IDs, task arguments, or raw runtime payloads.
- Current retry reaches GitLab successfully but finalize maps both branch-head and archive-read failures to the same generic `appstudio starter commit is unavailable`; remote GitLab access logs show branch and archive GETs both returned 200, so the remaining failure is local archive handling/validation and needs more precise internal diagnostics.
- Released `spec-v1.23.3` conflict: `ssot/01_contracts/domains/task-center/function-registry.yaml` and the embedded registry define `agent_runtime_ensure_input.additionalProperties: false` without `runtime_git_access_ref`, while `agent.Service` emits it for Coding Runtime and `agentexecutor` consumes it. Do not modify the embedded registry until SSOT is fixed and released.

## Exact recommended next step

Build `apiserver` and `taskworker`, deploy only those services, then retry application `27287fac-38f8-53fe-899d-6bfbbd66e977` with the approved idempotency key.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
