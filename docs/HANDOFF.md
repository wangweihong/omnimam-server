# Project Handoff

## Current project goal

Implement the released `spec-v1.7.12` Asset Library deletion contract: ordered batch soft/hard deletion, direct hard deletion without entering trash, and emptying the current user's trash with per-item results.

## Completed in this session

1. Updated the `ssot` submodule and `SSOT_VERSION` to released tag `spec-v1.7.12` at commit `e7daf0f80cb2d739201b6aef5054e4cc799c08f8`.
2. Added `hard_delete` query binding to single-asset DELETE through a request-local decoder, preserving default soft-delete behavior.
3. Added `POST /api/v1/assets/batch-delete` with 1..200 item validation, unique IDs, one deletion mode per request, stable input ordering, independent item commits, and structured partial-failure results.
4. Added `POST /api/v1/assets/trash/empty`, scoped to the authenticated owner's `deleted` assets and processed in stable `created_at`, `id` order through the deleted-only permanent-delete path.
5. Added direct hard-delete and trash-list store operations while reusing the existing permanent-delete transaction and Blob sharing checks.
6. Added business errors `150613` (`asset_batch_delete_request_invalid`) and `150614` (`asset_delete_failed`), then regenerated code and error documentation with `make gen`.
7. Replaced string matching for blocked deletion with `store.ErrAssetDeleteBlocked` and ensured soft-deleted Artifacts still retain shared Blobs.
8. Added DTO/query binding, service, route-contract, owner-isolation, empty-trash, partial-failure, invalid-size, and PostgreSQL integration coverage.

## Files modified

- `SSOT_VERSION`
- `ssot` submodule pin
- `backend/apis/iapiserver/request_asset_v1.go`
- `backend/apis/iapiserver/request_asset_v1_test.go`
- `backend/internal/apiserver/controller/v1/assetlibrary/controller.go`
- `backend/internal/apiserver/route.go`
- `backend/internal/apiserver/route_asset_library_contract_test.go`
- `backend/internal/apiserver/service/v1/assetlibrary/service.go`
- `backend/internal/apiserver/service/v1/assetlibrary/service_test.go`
- `backend/internal/apiserver/store/store.go`
- `backend/internal/apiserver/store/postgresql/asset_contract.go`
- `backend/internal/apiserver/store/postgresql/asset_contract_integration_test.go`
- `backend/internal/pkg/code/base.go`
- `backend/internal/pkg/code/code_generated.go`
- `docs/guide/zh-CN/api/error_code_generated.md`
- `docs/HANDOFF.md`

## Key architectural decisions

- Batch deletion is intentionally not one database transaction. Each item performs its own owner check and commit so one failure cannot roll back successful siblings.
- Direct hard deletion and trash permanent deletion share one store transaction; direct deletion allows `active`, `archived`, or `deleted`, while trash clearing rechecks `status=deleted` for every item.
- Trash enumeration returns IDs only, is owner-scoped, and uses a stable order. A concurrently restored asset fails the deleted-only per-item check instead of being permanently deleted.
- Single DELETE query decoding is local to `DeleteAssetRequest`; the shared request decoder was not broadened for all DELETE endpoints.
- The release defines no new deletion event, permission, database table, field, migration, dependency, or binary.

## API, schema, and configuration changes

- `DELETE /api/v1/assets/{asset_id}?hard_delete=true` now directly performs permanent deletion.
- `POST /api/v1/assets/batch-delete` accepts `{ "items": [{"id": "..."}], "hard_delete": false }` and returns `total`, `success`, `fail`, and ordered `results`.
- `POST /api/v1/assets/trash/empty` returns the same batch result shape and accepts no request body.
- Error codes `150613` and `150614` were added from the released SSOT.
- No runtime schema migration or configuration change was required.

## Verification

- `make gen` passed.
- Focused Asset Library/API/core/API Server tests passed.
- Focused race tests passed for Asset Library service, API DTOs, and request decoding.
- `go vet ./backend/...` passed.
- `git diff --check` passed.
- The PostgreSQL integration test compiles with `-tags=integration` but was skipped because `OMNIMAM_TEST_POSTGRES_DSN` is not set.
- `go test ./backend/... -count=1` reaches unrelated existing failures only in `taskcenter.TestAssignSystemName` and `taskname.TestResolve`: expected `生成 thumbnail 表现形式`, actual `生成 thumbnail视图`.

## Remaining work

- Run `TestPostgresAssetContractLifecycle` against a disposable PostgreSQL instance to execute the new owner-isolation and trash-query assertions instead of skipping them.
- Add end-to-end HTTP tests that assert the exact batch response JSON for mixed success/failure cases.
- Design a released cross-domain deletion-reference boundary for canvas, application version, AtomicTask, ApplicationRun, and CanvasRun facts; the current permanent-delete implementation only checks locally available reference tables.
- Design recoverable object cleanup so StorageAdapter failure cannot occur after the database facts have already committed as deleted.

## Known issues and risks

- Permanent deletion commits database removal before `StorageAdapter.Delete`; if physical cleanup fails, the API reports `asset_delete_failed` but the asset can no longer remain in trash. Fixing this requires a released recoverable cleanup design rather than an ad hoc table or event.
- Strong-reference checking remains incomplete across domain fact sources and currently blocks every active Collection membership rather than specifically modeling pinned-version references.
- Full backend tests retain the two unrelated thumbnail localization failures described above.

## Recommended next task

Define and release the cross-domain reference-check and recoverable Blob cleanup contract, then update permanent deletion so every blocked or cleanup-failed trash item is guaranteed to remain recoverable.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
