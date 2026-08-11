# Handoff

## Current goal and status

- Goal: implement released independent GitLab domain Phase 1 without changing AppStudio contracts, models, APIs, source storage, or behavior.
- Status: implementation and focused verification are complete in `omnimam-spec`, `omnimam-server`, and `omnimam-devops`.

## Work completed in this session

- Published `omnimam-spec` content commit `78122d28b412a52279c69cc2ec239b41af2a47a1`, release commit/tag `edcdbcebf8daecec8eaefd129338e829512b00fe` / `spec-v1.22.0`, and follow-up handoff commit `20695380cefb4426b8352bc81e27e21e1a212e9e`.
- Pinned `ssot` and `SSOT_VERSION` to released `spec-v1.22.0` commit `edcdbcebf8daecec8eaefd129338e829512b00fe`.
- Added GitLabServer/GitLabProject API types, requests, PostgreSQL Store, HTTP client, service, controller routes, permissions, errors, API Server wiring, and Task Worker executor.
- Added exact internal-caller/input validation for `gitlab.pipeline.run`; public AtomicTask/Group/DAG creation cannot bypass the GitLab domain boundary.
- Added recoverable Pipeline execution using `external_job_id`, `IN_PROGRESS`, five-second callbacks, retry/restart checkpoint recovery, and small credential-free outputs.
- Added non-retryable worker cancellation projection: Conductor blocks DAG continuation, while Task Center records AtomicTask/TaskAttempt `CANCELED` and strips the internal runtime marker.
- Added API-side GitLab cancellation handler injection. Task Center reads the current runtime checkpoint, invokes best-effort `CancelPipeline`, then terminates the Conductor execution.
- Added bounded GitLab response parsing, structured `{message}` errors, context-aware HTTP calls, detached Project compensation, remote-404 deletion, SQL constraints/indexes, and SQL-log suppression for credential writes.
- Repaired legacy `release_v119.go` manual error registration so `make gen.errcode` is reproducible and does not double-register codes.
- Added devops bootstrap for non-admin user `omnimam-appstudio-api`, Owner membership in Group `omnimam-appstudio`, reusable `api` PAT validation/rotation, and atomic mode-0600 token persistence.
- The username differs from the original plan because GitLab globally conflicts user personal namespaces with the existing `omnimam-appstudio` Group path; the user authorized the naming adjustment.

## Current in-progress work

- None.

## Files added, modified, renamed, or removed

- Added: `backend/apis/iapiserver/meta_gitlab.go`, `backend/apis/iapiserver/request_gitlab.go`.
- Added: `backend/internal/apiserver/controller/v1/gitlab/gitlab.go`.
- Added: `backend/internal/apiserver/service/v1/gitlab/client.go`, `client_gitlab.go`, `service.go`.
- Added: `backend/internal/apiserver/store/postgresql/gitlab.go`, `backend/internal/pkg/code/release_v122.go`, `backend/internal/taskworker/gitlabexecutor/executor.go`.
- Modified: `SSOT_VERSION`, `ssot`, API Server route/bootstrap, Store interfaces/schema bootstrap, identity defaults, WorkflowRuntime/Task Center cancellation projection, Task Worker registration, generated deepcopy/error files and focused existing tests.
- Modified in devops: `bootstrap/bootstrap.sh`, `deploy.sh`, and `README.md`.
- Existing unrelated server docs and devops `.gitignore`, plus spec `archive/`, `docs/identity_fix.md`, and `设计图/`, remain untouched.

## Key architectural or design decisions

- GitLab is an independent domain; AppStudio has no Phase 1 dependency or binding.
- Credential persists only in `GitLabServer.Credential`, uses `json:"-"`, is redacted from errors, and is excluded from SQL logging sessions.
- `gitlab.pipeline.run` is not Infra-backed and is not in the Agent/AppStudio Docker Function Registry.
- External cancellation is a source-domain handler injected into Task Center; the handler performs only the remote side effect and never writes Task Center state.
- No new `backend/cmd/` binary, Secret Provider, deployment environment variable, or `.env` management path was added.

## API, schema, dependency, or configuration changes

- Added administrator GitLab Server/Project APIs under `/api/v1/gitlab` with `gitlab.server.read/manage` and `gitlab.project.read/manage`.
- Added `gitlab_servers` and `gitlab_projects`, status/FK/unique/index constraints, and `ON DELETE RESTRICT` Server ownership.
- Added GitLab errors in `250200-250999` and generated documentation.
- Devops writes the PAT to `/state/appstudio-api-token`, exposed on the host as `./data/bootstrap/appstudio-api-token`; deploy output prints only this path.

## Verification performed and remaining checks

- Ran `make gen.deepcopy` and `make gen.errcode`; generated code compiles without duplicate registration.
- Passed focused Task Center tests for caller/input validation, cancellation checkpoint dispatch, and canceled projection cleanup.
- Passed focused Task Worker tests for credential redaction, Server READY/ERROR, delete restriction, Project compensation, remote 404 deletion, Pipeline success/failure/cancel, checkpoint recovery/no duplicate create, remote cancellation, and HTTP token/error parsing.
- Passed focused WorkflowRuntime checkpoint/retry recovery tests and compile checks for API, code, GitLab service/controller/store/executor, API Server, and Task Worker packages.
- Passed `git diff --check` in server and devops.
- Passed `sh -n bootstrap/bootstrap.sh`, `bash -n deploy.sh`, and `docker compose --env-file .env.runtime config --quiet`.
- Ran bootstrap twice against healthy local GitLab 19.2.1: second run reused the same PAT digest; token file remained mode `0600`; user is `admin=false`; Group membership access level is `50` (Owner).
- Remaining checks: none for the requested scope. Devops intentionally does not call the OmniMAM API to create the first GitLabServer.

## Outstanding tasks

- Administrator operational step: use `./data/bootstrap/appstudio-api-token` to create and test the first GitLabServer through the new API.

## Known issues and risks

- Existing unrelated dirty/untracked files were excluded from the GitLab implementation commits and must remain untouched.
- GitLab remote cancellation is best-effort. If no Pipeline checkpoint exists yet, local Task Center cancellation still completes without a remote ID to cancel.
- `make gen.deepcopy` emits pre-existing unsupported-type warnings but completes successfully.

## Exact recommended next step

Configure the first GitLabServer with the generated token file and call its test endpoint.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
