# Project Handoff

## Current goal and status

- Goal: replace Infrastructure service status and RuntimeMode string literals with constants defined in the iapiserver request contract file.
- Status: complete. The requested constants are defined and all identified service references use them.
- SSOT: `spec-v1.17.2` at `5990e6054ec8b79a342ef6e979b6522b855378aa`, matching `SSOT_VERSION` and the `ssot` submodule.

## Work completed in this session

- Read `skills/omnimam-server-backend/SKILL.md` and `backend/AGENTS.md`.
- Confirmed the task is a behavior-preserving internal refactor.
- Located the requested files at `backend/internal/infrastructure/service.go` and `backend/apis/iapiserver/request_infrastructure.go`.
- Identified RuntimeMode values `JOB` and `SERVICE`, plus Infrastructure runtime/profile/node/endpoint/output/config-binding status literals used by the service.
- Added exported RuntimeMode and Infrastructure status constants to `backend/apis/iapiserver/request_infrastructure.go`.
- Replaced the corresponding literals in `backend/internal/infrastructure/service.go` and `InfraCreateRuntimeRequest.Validate`.
- Ran `gofmt` and `git diff --check` successfully.

## Current in-progress work

- Run `go test ./internal/infrastructure` from `backend` and inspect the final diff/status.

## Files modified

- `docs/HANDOFF.md`
- `backend/apis/iapiserver/request_infrastructure.go`
- `backend/internal/infrastructure/service.go`

No files under `ssot/` will be modified.

## Key decisions

- Preserve every existing serialized string value and only change source-level references.
- Name constants by Infrastructure resource and semantic role, keeping them in the user-specified iapiserver request file.
- Do not add tests outside the existing target package because `backend/AGENTS.md` restricts new tests outside `pkg/`.

## API, schema, dependency, and configuration changes

- No API, schema, dependency, configuration, error code, permission, or event changes.
- No new binary is introduced.

## Verification performed

- `git submodule status ssot` and `SSOT_VERSION` were checked and match the released pin.
- Relevant source symbols and literals were inspected with `rg` and line-scoped `sed`/`nl` reads.

Completed checks: `gofmt` and `git diff --check`.
`go test ./internal/infrastructure` from `backend` passed.

## Outstanding tasks

- None for this task.

## Known issues and risks

- None identified. The change must remain value-preserving.

## Exact recommended next step

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
