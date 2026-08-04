# Project Handoff

## Current goal and status

- Goal: finish the AppStudio event and Identity permission implementation after publishing the corrected `spec-v1.16.1` contract.
- Status: complete. Server is pinned to the released `spec-v1.16.1` commit `63defea97f45761acf300030cad1c00d6b83bb5a`, and the event and permission implementation passes focused verification.

## Work completed in this session

- Preserved all existing v1.16.0 Workspace-internalization work and unrelated user changes.
- Confirmed the deterministic collision: application creation emits `appID:1`, and the first source Revision must also use current revision `1`.
- Confirmed the implementation plan: explicit fully qualified event keys, exact payload alignment for seven event types, initialization Revision event, and corrected Identity source resources.
- Published `spec-v1.16.1`, updated the `ssot` submodule to its Release commit, and synchronized `SSOT_VERSION`.
- Implemented explicit fully qualified keys for all seven AppStudio events and decoupled event idempotency keys from payload resource versions.
- Added the initialization Revision event to application creation and aligned all seven payloads with the released event contract.
- Added explicit Release runtime-instance propagation and corrected the two Identity source permission resource mappings.
- Extended the existing `outbox_test.go` with table-driven key, collision, Revision stability, and exact payload-field coverage.

## Current in-progress work

- None for this task.

## Files added, modified, renamed, or removed

- Updated `SSOT_VERSION`, the `ssot` submodule pointer, and `docs/HANDOFF.md` in this phase.
- Modified `backend/internal/apiserver/store/postgresql/appstudio.go`, `backend/internal/apiserver/store/postgresql/outbox_test.go`, and `backend/internal/apiserver/service/v1/identity/identity.go` for this fix.
- Existing task changes in the Server working tree must be preserved, especially `backend/internal/apiserver/store/postgresql/appstudio.go`, AppStudio service/controller files, Identity service files, `SSOT_VERSION`, and the `ssot` submodule pointer.
- Existing unrelated edits in `backend/internal/apiserver/service/v1/agent/service.go` and `third_party/gotoolbox/pkg/generic/generic.go` must not be overwritten.

## Key architectural or design decisions

- Keep the Outbox table's single-column global uniqueness contract; event types namespace all new idempotency keys.
- `appendAppStudioOutbox` will receive an explicit idempotency key; payload `resource_version` remains independent.
- Revision keys use `revision.Revision`, not the per-row `revision.ResourceVersion`.
- Application creation emits an initialization Revision event with `current_revision=0`.
- No SourceContentStore deletion-on-failure workaround is allowed; cross-store atomicity remains a follow-up design task.

## API, schema, dependency, or configuration changes

- Pin: `ssot` and `SSOT_VERSION` now reference released `spec-v1.16.1` commit `63defea97f45761acf300030cad1c00d6b83bb5a`.
- Implemented Identity resource mapping:
  - `appstudio.source.read`: `studio_application_source, studio_source_file`
  - `appstudio.source.write`: `studio_change_set, studio_source_revision`
- No Server migration or dependency change is planned.

## Verification performed and remaining checks

- Ran `gofmt` on the three modified Go files.
- `go test ./backend/internal/apiserver/store/postgresql` passes, including seven key formats, the lifecycle/first-Revision collision case, repeated and consecutive Revision behavior, and exact payload field sets.
- `go test ./backend/internal/apiserver/service/v1/appstudio`, `go test ./backend/internal/apiserver/controller/v1/appstudio`, and `go test ./backend/internal/apiserver/service/v1/identity` pass; these packages currently have no test files beyond compilation.
- `git diff --check` passes.
- Confirmed `ssot`, `SSOT_VERSION`, and `spec-v1.16.1` use Release commit `63defea97f45761acf300030cad1c00d6b83bb5a`.
- Confirmed the public AppStudio API models do not expose `workspace_id`, `workspace_revision`, or bare `revision` JSON fields.
- No full-repository test was run, per task constraints.

## Outstanding tasks

- None for the Outbox conflict and permission-alignment task.
- Follow-up only: design cross-store `SourceContentStore` atomicity independently.

## Known issues and risks

- The working tree is dirty and contains user changes; all overlapping edits require careful preservation.
- Existing Outbox rows are not rewritten; consumers continue using each row's own idempotency key.
- Preview, Build, and Release do not have independent persisted error-code facts and must emit `error_code: null`; RuntimeInstance uses its persisted `ErrorCode`.
- SourceContentStore/database atomicity remains unresolved and outside this task.

## Exact recommended next step

Design a `SourceContentStore` `prepare/promote/discard` lifecycle under the Workspace row lock, including crash recovery and orphan cleanup, without changing this completed Outbox fix.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
