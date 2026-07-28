# Project Handoff

## Current project goal

Finish and verify the released `spec-v1.7.13` ApplicationRun history and terminal AtomicTask projection fix while preserving the in-progress `spec-v1.7.12` Asset Library deletion implementation.

## Completed in this session

1. Published and pinned `spec-v1.7.13` (`52fae755aaaefefec09f4cad33e429e998bf1edc`).
2. Added `GET /api/v1/applications/{application_id}/runs` across DTO, store, service, controller, and route layers.
3. Added paginated ApplicationRun loading with application scope and one batched Artifact query per page.
4. Added terminal observers to Task Center reconciliation and wired `ApplicationRunExecutor.Completed` only after AtomicTask projection commits.
5. Added a regression test proving terminal observers run after persistence and updated the route contract count to 53.
6. Added a bounded periodic repair loop that backfills existing terminal runs and retries idempotent Artifact projection.
7. Preserved the existing Asset Library batch/direct hard-delete and trash-empty implementation and tests.
8. Fixed ApplicationRun list Artifact loading to sort by existing `created_at, id` columns; the initial implementation incorrectly referenced the absent `sequence` column and surfaced as a misleading input-validation error.

## Files modified

- `SSOT_VERSION`, `ssot`
- `backend/apis/iapiserver/meta_application_platform.go`
- `backend/apis/iapiserver/request_application_platform.go`
- `backend/internal/apiserver/controller/v1/applicationplatform/application_platform.go`
- `backend/internal/apiserver/route.go`
- `backend/internal/apiserver/route_application_platform_test.go`
- `backend/internal/apiserver/service/v1/applicationplatform/application_platform.go`
- `backend/internal/apiserver/service/v1/taskcenter/reconciler.go`
- `backend/internal/apiserver/service/v1/taskcenter/reconcile_test.go`
- `backend/internal/apiserver/store/postgresql/application_platform.go`
- `backend/internal/apiserver/store/store.go`
- `backend/internal/apiserver/taskworker.go`
- Existing Asset Library files listed by the prior handoff remain modified.

## Key architectural decisions

- ApplicationRun history is read from Application Platform, never reconstructed from Task Center by Web.
- AtomicTask remains the execution fact source. Completion projection runs only after Task Center's transaction persists the terminal task.
- Projection remains monotonic through `task_resource_version`; Artifact creation remains idempotent through existing stable run/output uniqueness.
- The run page batches Artifact loading. Relation summaries still use existing service helpers and can be optimized further if page-level profiling shows N+1 cost.
- Existing Asset Library deletion paths and unrelated user changes were not reverted.

## API, schema, and configuration changes

- Added `GET /api/v1/applications/{application_id}/runs` with pagination and created/updated sorting.
- No database migration, permission, error code, or runtime configuration change.
- SSOT is now `spec-v1.7.13`.

## Verification

- Focused Application Platform, Task Center projection, PostgreSQL store, and controller tests pass.
- Rebuilt/redeployed apiserver and taskworker. Existing run `5f3d2f31-015a-4fc2-af9c-460048e3e03d` automatically repaired to `SUCCESS` with output values and a registered `images` Artifact linked to Asset `ce906276-d34d-464b-9d5f-5b10da5c11e9`.
- `go test ./backend/...` reaches only the two existing thumbnail localization failures: expected `生成 thumbnail 表现形式`, actual `生成 thumbnail视图`.
- Application Platform SSOT route contract now matches all 53 operations.

## Remaining work

- Execute one new ApplicationRun to verify the live completion observer in addition to the confirmed historical repair path.
- Existing Asset Library integration and cross-domain cleanup tasks remain outstanding.

## Known issues and risks

- The repair loop is bounded to the 200 most recently updated bound runs. A future outbox/checkpoint design would scale recovery beyond that window.
- The two unrelated thumbnail localization tests remain red.

## Recommended next task

Complete Web integration, deploy both binaries, and verify a new ComfyUI run end to end; then design durable retry for failed terminal observers.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
