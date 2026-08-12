# Handoff

## Current goal and status

- Goal: implement AppStudio GitLab Phase 2 using GitLab as the only source-content provider while preserving AppStudio Revision/ChangeSet semantics and Platform MCP.
- Status: implementation complete for the released reservation correction. The remaining acceptance work is the local GitLab smoke test, which requires a configured READY default GitLabServer.

## Work completed in this session

- Revalidated the current pin: `ssot` and `SSOT_VERSION` both point to released `spec-v1.23.1` commit `d1b24118091e52d60fc20a3faa4cf47647f467ab`.
- Confirmed the approved design decisions: fixed `web-react@v1` Blueprint, existing `agent.coding@1.0` profile, Runtime-scoped project token, Worker commit-to-Revision projection, and destructive clean-data rollout without BUILT_IN migration.
- Loaded the repository backend rules and the applicable Go design, database, security, error-handling, DI, naming, style, and testing guidance.
- Completed the scoped SSOT S1/S2/Context draft for AppStudio GitLab Phase 2 across appstudio, agent, gitlab, infrastructure, and task-center.
- Added the design contracts for `web-react@v1`, GitLab-only source bytes, `gitlab_project_id`, Revision `commit_sha`, unique READY default GitLabServer, Runtime Git access, archive injection, and Worker commit-to-ChangeSet synchronization.
- Published SSOT content commit `7010953df6130caa1f59c8b19dd9feaa5e06d467`, release commit/tag `ee0cf7f7732a74d1ca36cc1e73a5541a333ed025` (`spec-v1.23.0`), pushed and verified the remote tag.
- Updated the server `ssot` gitlink and `SSOT_VERSION` to the released `spec-v1.23.0` commit.
- Formatted and compiled the direct API server, Task Worker, Infrastructure, Docker provider, AppStudio, GitLab and PostgreSQL store packages.
- Removed the local `SourceContentStore` and `source_store.go`; AppStudio now requires a GitLab `SourceProvider` for creation, reads and commits.
- Implemented deterministic GitLab Project/Starter commit recovery, including empty-project initialization and verification-only reuse of an existing remote template.
- Implemented Worker-side single-commit synchronization: a Coding Invocation now projects only a single default-branch fast-forward commit from the frozen base CommitSHA to one idempotent ChangeSet and next Revision.
- Wired the Infrastructure source archive resolver through AppStudio/GitLab consumer interfaces for Preview archive resolution.
- Added Build Snapshot grant resolution to the AppStudio archive resolver; it binds Snapshot, StudioBuild, Application, resource version and the Snapshot Revision CommitSHA before an archive can be opened.
- Wired Snapshot archive resolution into Infrastructure for the static-web Build profile. Docker now injects the validated archive into a disposable `/workspace` tmpfs behind a startup gate, runs the fixed pnpm build command, then safely collects `bundle.tar.gz` from Docker's archive API with a size limit and SHA-256 descriptor.
- Updated the local static-web Build profile from Alpine to `node:22-alpine` so the fixed pnpm command has a Node/Corepack runtime.
- Completed Runtime Git access wiring: GitLab Project Access Token credentials are encrypted in `appstudio-runtime-git-access://` references, scope-bind owner/Agent/Runtime/generation/Application/Workspace/Project/expiry, reach Task Worker only as `SECRET_REF`, are resolved only in Infrastructure memory, and enter Docker only through stdin into a tmpfs credential helper. Coding Runtime clones the default branch into disposable `/workspace`, gates OpenCode on clone completion, recreates instead of restarting its stopped Infra runtime, and revokes the Project token on startup failure or runtime stop/delete with expiry as fallback.
- Published and pinned `spec-v1.23.1` at `d1b24118091e52d60fc20a3faa4cf47647f467ab`; it adds the released GitLabProject `CREATING/READY/ERROR` reservation representation.
- Implemented the reservation path: `CreateApplication` first creates CREATING Application/Repository/Workspace rows with deterministic IDs, GitLabSourceProvider persists the matching CREATING Project projection before remote Project creation, then the existing initialization aggregate completes Revision 0/Agent/Session/Bindings and READY statuses.
- Hardened reservation recovery: retries use the Server and deterministic path persisted in the reservation, remote/template failures move non-READY reservations to `ERROR` through a PostgreSQL row-locked transition, and a failed projection after a newly created remote Project triggers bounded best-effort deletion before retaining the failed reservation.
- Added the release-required PostgreSQL constraint migration for nullable GitLab `external_project_id` and `CREATING|READY|ERROR` project status validation.

## Current in-progress work

- No code change is in progress. Run the local GitLab smoke test against a clean-data deployment with one READY default GitLabServer.

## Files added, modified, renamed, or removed

- Modified: `SSOT_VERSION`, AppStudio/GitLab API metadata and requests, AppStudio/GitLab services and stores (including `service/v1/gitlab/source_provider.go` reservation recovery and `store/postgresql/0_pg.go` constraints), API/worker wiring, Agent invocation flow, Infrastructure/Docker source injection, deployment configuration, and `docs/HANDOFF.md`.
- Added: `backend/internal/apiserver/service/v1/appstudio/blueprint.go`, embedded `web-react@v1` blueprint assets, `appstudio/source_provider.go`, and `gitlab/source_provider.go`.
- Removed: `backend/internal/apiserver/service/v1/appstudio/workspace_tool.go` and its route/controller/MCP claim references.
- Removed: `backend/internal/apiserver/service/v1/appstudio/source_store.go`.
- Modified in the independent `ssot` repository: `GLOBAL_CONTEXT.md`, `CONTEXT_MAP.md`, five domain Context/S1 pairs, and the scoped AppStudio/GitLab/Agent/Infrastructure/Task Center S2 schema/API/module contracts.
- Existing unrelated untracked files under `docs/` remain untouched.

## Key architectural or design decisions

- GitLab stores canonical file bytes; AppStudio retains monotonically increasing Revision and atomic ChangeSet facts, linked by `StudioWorkspaceRevision.CommitSHA`.
- Coding Runtime uses a disposable `/workspace` Git clone. Platform MCP remains; AppStudio Workspace Tool is removed.
- Git credentials are Runtime-scoped, project-limited, resolved from an opaque reference, injected through tmpfs, and revoked on Runtime teardown or expiry.
- `web-react@v1` is embedded, read-only, versioned with the server, and fixed for current STATIC_WEB creation. `fix.md` ships but is not routed in this phase.
- Existing BUILT_IN data is not migrated. Rollout deletes only the PostgreSQL and legacy AppStudio source volumes before rebuilding.
- A GitLabProject reservation fixes its Server/path at first persistence. Later retries use those stored values even if the global default changes; non-READY reservations become `ERROR` through a row-locked transition that cannot downgrade READY, and a failed READY projection compensates a newly created remote Project within a bounded context.

## API, schema, dependency, or configuration changes

- Released SSOT changes: GitLabServer default flag; AppStudio Blueprint fields, GitLabProject reference and CommitSHA; Agent Git workspace claims; Infrastructure secret/source resolution; Task Worker commit synchronization.
- PostgreSQL now explicitly drops the obsolete non-null constraint on `gitlab_projects.external_project_id` and enforces `CREATING|READY|ERROR` for GitLabProject status.
- No new error code, permission, event, `.env` file, or `backend/cmd/` binary is planned.

## Verification performed and remaining checks

- Confirmed the server submodule is at released `spec-v1.23.1` commit `d1b24118091e52d60fc20a3faa4cf47647f467ab` and `SSOT_VERSION` declares the same release.
- Passed after Runtime Git wiring: focused `go test` for Agent, AppStudio, GitLab, Agent executor, Infrastructure, Docker provider and PostgreSQL store packages; the Docker provider suite verifies coding credential stdin injection, no Docker metadata leak, clone gate and disposable `/workspace` tmpfs.
- Passed after the reservation correction: `make gen.deepcopy`, `go test ./backend/internal/apiserver/service/v1/appstudio ./backend/internal/apiserver/service/v1/gitlab ./backend/internal/apiserver/store/postgresql`, `go test ./backend/internal/apiserver/service/v1/agent ./backend/internal/taskworker/agentexecutor ./backend/internal/taskworker ./backend/internal/taskworker/gitlabexecutor ./backend/internal/infrastructure ./backend/internal/infrastructure/providers/dockerruntime`, Blueprint `pnpm install --frozen-lockfile && pnpm build`, `docker compose -f deployments/docker-compose.yaml config`, and `git diff --check`.
- `make gen.deepcopy` reports its pre-existing unsupported alias warnings but exits successfully. The direct AppStudio/GitLab service packages currently contain no test files; repository rules prohibit adding arbitrary non-`pkg/` test files.

## Outstanding tasks

- Run the local GitLab smoke test for Create, initial/follow-up Invocation, Source, Restore, Preview and Build by fixed CommitSHA.

## Known issues and risks

- AppStudio/GitLab S2 is resolved by `spec-v1.23.1`; the server reservation/store/adapter state machine is implemented and directly compiled/tested. The unexecuted local GitLab smoke test is the remaining deployment-level risk.
- The worktree contains unrelated untracked design documents that must not be modified or committed.

## Exact recommended next step

Deploy against a clean local GitLab, configure/detect/set exactly one READY AppStudio default GitLabServer, then run the Create/initial-follow-up Invocation/Source/Restore/Preview/Build smoke test. Do not repeat completed reservation or Runtime Git work.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
