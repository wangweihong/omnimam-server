# Project Handoff

## Current goal and status

- Goal: replace Task Worker status, mode, action, and policy string literals with existing iapiserver constants.
- Status: implementation and target-package verification are complete; commit is pending.
- SSOT: `spec-v1.17.2` at `5990e6054ec8b79a342ef6e979b6522b855378aa`, matching `SSOT_VERSION` and the `ssot` submodule.

## Work completed in this session

- Read `skills/omnimam-server-backend/SKILL.md` and `backend/AGENTS.md`.
- Confirmed the task is a behavior-preserving internal refactor.
- Located the requested files at `backend/internal/infrastructure/service.go` and `backend/apis/iapiserver/request_infrastructure.go`.
- Identified RuntimeMode values `JOB` and `SERVICE`, plus Infrastructure runtime/profile/node/endpoint/output/config-binding status literals used by the service.
- Added exported RuntimeMode and Infrastructure status constants to `backend/apis/iapiserver/request_infrastructure.go`.
- Replaced the corresponding literals in `backend/internal/infrastructure/service.go` and `InfraCreateRuntimeRequest.Validate`.
- Ran `gofmt` and `git diff --check` successfully.
- Confirmed the prior refactor was committed as `201738e` and the worktree was clean at audit start.
- Audited `backend/internal/infrastructure`, the directly related Infrastructure iapiserver files, and available direct tests without scanning unrelated modules.
- Found remaining Docker provider literals for `ONLINE`, `JOB`, `SUCCEEDED`, `RUNNING`, `DELETED`, `STOPPED`, and `FAILED` in `backend/internal/infrastructure/providers/dockerruntime/docker.go`.
- Confirmed the Infrastructure module has no direct `_test.go` files for this provider/service path.
- Added `InfraNodeStatusOnline` and replaced the Docker provider's status and RuntimeMode references with constants from `backend/apis/iapiserver/request_infrastructure.go`.
- Located Task Worker implementation and direct tests under `backend/internal/taskworker`; existing Task Center and Infrastructure constants were inspected before editing.
- Added independent Task Worker contract constants to `backend/apis/iapiserver/request_task_worker.go`; these deliberately do not reuse `InfraRuntime*` constants.
- Replaced Task Worker operation, mode, status, visibility, action, deployment reason, restart policy, health, validation, and retry backoff literals in production code and direct tests.
- Replaced Task Center status values in direct JSON test fixtures with existing Task Center constants.

## Current in-progress work

- Commit the completed Task Worker constant refactor.

## Files modified

- `docs/HANDOFF.md`
- `backend/apis/iapiserver/request_infrastructure.go`
- `backend/apis/iapiserver/request_task_worker.go`
- `backend/internal/infrastructure/providers/dockerruntime/docker.go`
- `backend/internal/taskworker/taskworker.go`
- `backend/internal/taskworker/taskworker_test.go`

These files were changed across the Infrastructure and Task Worker refactors; the current Task Worker commit contains only the three Task Worker files and this handoff. No files under `ssot/` will be modified.

## Key decisions

- Keep the audit within the target Infrastructure module and directly related iapiserver files; do not scan unrelated domains or the entire repository.
- Preserve every serialized value and only replace source-level references with existing or directly corresponding iapiserver constants.
- The request validation tag `oneof=JOB SERVICE` is a struct-tag literal and cannot directly reference Go constants; it is not treated as an executable duplicate.
- Keep JSON payload fixtures and free-form operation identifiers unchanged unless a matching exported contract constant already exists and the replacement is source-level only.

## API, schema, dependency, and configuration changes

- No API, schema, dependency, configuration, error code, permission, or event changes.
- No new binary is introduced.

## Verification performed

- `git submodule status ssot` and `SSOT_VERSION` were checked and match the released pin.
- Relevant source symbols and literals were inspected with `rg` and line-scoped `sed`/`nl` reads.

Completed checks: `go test ./internal/taskworker`, `gofmt -d` on the changed Go files, `git diff --check`, and the scoped Task Worker literal/constant audit all passed.

## Outstanding tasks

- Stage and commit the verified Task Worker constant replacement.

## Known issues and risks

- Current risk: Task Worker contracts are backed by registry YAML enum definitions; this refactor preserves their serialized values and only changes source-level references.

## Exact recommended next step

Run `git add` for the three Task Worker files and this handoff, then commit the verified refactor; record the commit hash here.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
