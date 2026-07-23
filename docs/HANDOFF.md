# Project Handoff

## Current project goal

Continue OmniMAM server development on released `spec-v1.7.4`. The current increment includes the Asset Library v0.6 administrator-only Blob/StorageBackend inspection contract, fixes the missing original-media metadata pipeline, and now includes a frontend Nginx compatibility fix for the diagnosed chunked-upload failure.

## Completed in this session

1. Updated the pinned `ssot` submodule and `SSOT_VERSION` from released `spec-v1.7.3` to released `spec-v1.7.4` commit `7fb6edf8ef69886eabe7e206379a7fe1858500ee`.
2. Added the five released operations:
   - `GET /api/v1/blobs/{blob_id}`
   - `GET /api/v1/storage-backends`
   - `POST /api/v1/storage-backends`
   - `GET /api/v1/storage-backends/{backend_id}`
   - `PATCH /api/v1/storage-backends/{backend_id}`
3. Moved StorageBackend HTTP ownership from the legacy platform controller/service into the Asset Library `storage-inspection` module and added consumer-side repository and authorization interfaces.
4. Enforced `ADMIN` or `SUPER_ADMIN` authorization before every target Blob/StorageBackend repository read or write. The existing `system-admin` identity remains the bootstrap administrator.
5. Added explicit administrator projections for Blob and StorageBackend. Blob details expose `storage_backend_id` and `object_key` without recursively embedding backend configuration; StorageBackend details expose complete `root` and `config` as required by the released contract.
6. Added released list filtering, pagination defaults/limits, create/update validation, local-root normalization, missing-resource error mapping, and single-query `items`/`backends` compatibility projection.
7. Added released errors `150610`–`150612` and regenerated Go registration plus Markdown error documentation with `make gen`.
8. Added `asset.storage.read` and `asset.storage.manage` to the system administrator `/me` permission projection.
9. Hardened persistent/legacy JSON models so `root`, `config`, `storage_backend_id`, and `object_key` cannot leak through accidental serialization. Only the explicit administrator detail DTOs serialize physical fields.
10. Updated the Asset/Representation/Blob architecture document with the administrator inspection boundary and compatibility-list behavior.
11. Added route, request validation, authorization ordering, ADMIN/SUPER_ADMIN acceptance, projection, missing-resource, create/update, and sensitive-field serialization tests.
12. Reproduced the media metadata defect: `representation.inspect` only checked AssetVersion existence, while upload completion stored only MIME/size and thumbnail generation wrote only derived Representation dimensions.
13. Added a consumer-side `MediaMetadataInspector` boundary. JPEG/PNG/GIF use Go `DecodeConfig`; WebP, video, and audio use a bounded local ffprobe adapter with context timeout, source-size verification, output limits, and a test fake seam.
14. Changed `representation.inspect` to read the ready original Representation through `ContentStorage`, extract metadata, and atomically merge it into AssetVersion and original Representation metadata.
15. Added guarded UserAsset projection updates: width, height, and duration are updated only when the inspected AssetVersion still equals `current_version_id`, preventing late historical-version tasks from overwriting current metadata.
16. Added the `assetmetadatabackfill` operator command. It scans current versions with missing typed metadata in stable Asset-ID batches and reuses the same inspection/writeback path without adding API, schema, event, error-code, permission, or Task types.
17. Executed the backfill against the running local environment. Five binary media assets were repaired with zero failures: WebP `1280x1707`, GIF `640x360`, JPEG `832x1216`, PNG `1344x1728`, and MP4 `854x480 / 31.402s`. The remaining two of seven assets are text/prompt, for which zero dimensions/duration are expected.
18. Diagnosed live error `151203 asset_upload_part_invalid`. The generated frontend sends `part_sha256` and `content_range`, while its Nginx proxy uses the default behavior that discards underscore-named request headers. API logs confirm `part_number=1` and the 8 MiB body arrived but both metadata headers did not. The affected 23,661,788-byte sessions remain `chunked`, `initialized`, and have zero recorded parts. No business implementation was changed during this diagnostic task.
19. Updated `/home/wwhvw/codespace/omnimam-web/web/nginx.conf` with `underscores_in_headers on`, validated it with Nginx 1.27, and reloaded the local frontend container. A live proxy request then reached API Server with both upload metadata headers present. The verification used an invalid token and made no upload-state mutation.

## Files added

- `backend/apis/iapiserver/meta_storage_security_test.go`
- `backend/apis/iapiserver/request_storage_test.go`
- `backend/cmd/assetmetadatabackfill/assetmetadatabackfill.go`
- `backend/internal/apiserver/asset_metadata_backfill_app.go`
- `backend/internal/apiserver/service/v1/assetlibrary/media_metadata.go`
- `backend/internal/apiserver/service/v1/assetlibrary/media_metadata_backfill.go`
- `backend/internal/apiserver/service/v1/assetlibrary/media_metadata_test.go`
- `backend/internal/apiserver/service/v1/assetlibrary/storage_inspection.go`
- `backend/internal/apiserver/service/v1/assetlibrary/storage_inspection_test.go`
- `backend/internal/apiserver/store/postgresql/asset_media_metadata_backfill.go`
- `backend/internal/apiserver/store/postgresql/asset_media_metadata_integration_test.go`

## Files modified

- `SSOT_VERSION` and the `ssot` submodule pointer.
- Asset and storage API models/requests under `backend/apis/iapiserver/`.
- Asset Library controller, service composition, route installation, store interfaces, PostgreSQL adapter, and tests.
- Representation executors, TaskWorker composition, PostgreSQL AssetVersion/Representation persistence, and executor tests for media metadata extraction/writeback.
- Legacy platform storage controller/service routes were removed; platform tests and storage fakes were adjusted.
- Error definitions and generated code/documentation.
- `docs/guide/zh-CN/architecture/asset-library-content-model.md`.
- This handoff, including the chunked-upload root-cause record.
- The sibling Web repository's `web/nginx.conf` and `docs/HANDOFF.md` for the deployed proxy compatibility fix.

No files were removed. The untracked `docs/guide/zh-CN/architecture/asset-library-server-bulk-import.md` remains a separate pre-existing draft and was not modified by this increment.

## Key architectural decisions

- Blob and StorageBackend are global infrastructure facts, not owner-scoped asset resources. Target repository access is allowed only after identity-role authorization succeeds.
- The consumed `StorageAdminAuthorizer` and `StorageInspectionStore` interfaces keep identity lookup and persistence replaceable and testable; the service does not query GORM directly.
- Persistent models are non-serializable for sensitive physical fields. Explicit administrator projection types are the only HTTP serialization boundary for `object_key`, `root`, and `config`.
- StorageBackend list performs one query and assigns the same projected slice to both canonical `items` and deprecated `backends`.
- Blob details return only the backend ID. Clients explicitly navigate to the StorageBackend detail endpoint when full physical configuration is required.
- Existing local StorageBackend behavior is preserved: empty local roots resolve through `OMNIMAM_STORAGE_ROOT` and then `data/assets`, and are stored as cleaned absolute paths.
- Original media metadata is a version fact first: it is persisted on AssetVersion and original Representation. UserAsset width/height/duration are only the current-version list/filter projection.
- Metadata inspection remains behind consumer interfaces: storage reads use `ContentStorage`, image probing uses the standard decoder adapter, and external probing uses the injected ffprobe adapter. Business logic does not resolve local paths or invoke a concrete provider directly.
- Historical repair is an explicit bounded operator command rather than read-time repair in list/detail APIs; list query count and latency remain independent of media size.

## API, schema, and configuration changes

- Added the five released administrator storage operations and the `AssetBlobDetail`, `StorageBackend`, and list projection shapes from Asset Library OpenAPI v0.6.0.
- Added business errors:
  - `150610 asset_storage_permission_denied`
  - `150611 asset_blob_not_found`
  - `150612 asset_storage_backend_not_found`
- Added frontend permission keys `asset.storage.read` and `asset.storage.manage` for the bootstrap administrator projection.
- StorageBackend GORM metadata now explicitly marks `root`, `config`, and `quota` non-null with released defaults. No new table or column was introduced because the implementation already stored the released fields.
- The existing `user_assets.width`, `height`, and `duration_seconds` fields are now populated from current original media; AssetVersion and original Representation reuse their existing metadata JSON.
- Added the source-built `assetmetadatabackfill` operator command. It uses the normal server database configuration and requires ffprobe plus access to the configured local StorageBackend.
- No public API field, table/column, event type, error code, permission code, runtime configuration variable, or Task type was added.
- No API/schema change was made for the upload fix. The sibling frontend Nginx now accepts and forwards the released `part_sha256` and `content_range` headers.

## Verification

- Confirmed the submodule resolves exactly to released tag `spec-v1.7.4`.
- Confirmed `SSOT_VERSION.commit` equals submodule commit `7fb6edf8ef69886eabe7e206379a7fe1858500ee`.
- `make gen` passed.
- Focused API model, Asset Library service, route, PostgreSQL store, platform, and error-code tests passed.
- Media inspector, inspect executor, historical backfiller, and current-version guard tests passed.
- PostgreSQL integration test `TestPostgresApplyAssetMediaMetadataProtectsCurrentVersionProjection` passed against the running PostgreSQL instance and rolled back its fixture.
- Live backfill reported `scanned=5 updated=5 failed=0`; a post-run SQL check found zero remaining image/video/audio current assets matching missing-metadata conditions and confirmed all three persistence layers.
- `go test ./backend/...` passed.
- `go vet ./backend/...` passed.
- `git diff --check` passed.
- Live PostgreSQL and the existing TaskWorker container were used only for the metadata integration test/backfill and read-after-write verification. No rebuilt service deployment, browser, HTTP end-to-end, or storage-inspection authorization verification was run.
- Frontend Nginx syntax validation passed with `nginx:1.27-alpine`; live proxy logs confirmed both underscore-named upload headers reached API Server after reload.

## Remaining work

1. Resolve the chunked-upload header contract upstream: use proxy-safe hyphenated header names (standard `Content-Range` plus a released hyphenated part-checksum header), release SSOT, update this submodule/controller, regenerate the frontend client, and add proxy/controller end-to-end coverage. The current frontend Nginx compatibility fix restores the released contract but should be removed after that migration.
2. Run PostgreSQL integration verification against migrated legacy rows and confirm AutoMigrate preserves/establishes the released non-null defaults.
3. Exercise all five endpoints with a normal user, `ADMIN`, and `SUPER_ADMIN`; confirm denied requests never query target storage resources and administrator responses include complete physical fields.
4. Verify the administrator UI consumes `items`, treats `backends` only as a compatibility alias, and never logs or forwards `root`/`config`.
5. Rebuild and restart the TaskWorker from the current source so future uploads execute the new metadata inspection path; then upload one image, one MP4, and one audio file and verify API/SSE projections without running the backfill command.
6. After this increment is accepted, resume the Workflow Canvas OutputBinding event consumer and persisted repair loop.

## Known issues and risks

- The design schema contains type and non-negative quota CHECK constraints. This repository follows its existing GORM AutoMigrate policy and does not independently add those CHECK constraints; request validation protects new HTTP writes, but live schema verification is still required.
- Authorization currently resolves role assignments on each request using the existing identity Role/UserRole stores. A centralized permission evaluator or cache is not introduced by this increment.
- `config` is intentionally returned without redaction to administrators and may contain credentials. Operational access logs and frontend telemetry must not record response bodies.
- Only the local storage adapter is implemented. Other released enum values remain reserved and require separately released/implemented adapters before production enablement.
- The one-off live backfill used a source-built command copied into the existing TaskWorker container, proving its ffprobe/runtime and storage mount. The long-running TaskWorker binary itself still uses the previous image until explicitly rebuilt/restarted, so future uploads in that environment will retain zero metadata until deployment.
- The ffprobe adapter intentionally caps unknown-size source files at 1 GiB; normal Blob-backed calls carry the verified size and therefore use that exact bound.
- The untracked bulk-import architecture draft belongs to separate work and must not be accidentally included with this increment.
- The source Nginx fix and live reload preserve the released upload headers, but an authenticated multi-part upload has not yet been repeated after the fix. Underscore header names remain fragile across arbitrary intermediaries until SSOT releases hyphenated replacements.

## Recommended next task

Run one authenticated multi-part upload through the corrected proxy, then correct and release the hyphenated header contract and update server/generated frontend together. Afterward rebuild/restart TaskWorker and continue the pending metadata and storage-inspection verification.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
