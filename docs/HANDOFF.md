# Project Handoff

## Current project goal

Implement the released `spec-v1.5.1` contracts while keeping domain ownership boundaries intact. The current planning priority is a contract-first related-resource response projection so frontend detail views receive useful one-hop summaries with existing foreign IDs; Representation backfill and remaining media policies are still outstanding.

## Completed in this session

1. Updated the `ssot` submodule to released tag `spec-v1.5.1` at `f55c2439c6527e1716c2824ee3b84b84ebaf623c` and synchronized `SSOT_VERSION`.
2. Added all 45 released asset-library OpenAPI operations under `/api/v1`, while retaining non-conflicting legacy upload/content/thumbnail routes.
3. Added canonical DTOs and persistence models for Blob, AssetRepresentation, upload sessions, Collection, CollectionItem, Asset fields, normalized Label/Tag ownership, and Artifact registration versions.
4. Implemented current-user scoped Asset create/list/detail/update/trash/restore/permanent-delete, canonical version creation, current-version selection, and Representation queries/writes.
5. Implemented normal/chunked upload sessions, part/range/SHA256 validation, content-addressed local storage, completion transactions, original Representation creation, and reliable outbox publication.
6. Implemented Collection hierarchy, name uniqueness, pinned versions, idempotent membership, Label/Tag replacement and item operations, and partial-success batch operations.
7. Implemented Artifact create/list/get/content/complete/delete/register, ready-state enforcement, idempotent registration, Blob reuse, original Representation creation, and registration outbox.
8. Added a bounded selector parser for Label, Tag, `@group`, AND/OR, parentheses, existence, equality, `in`, and `notin`; PostgreSQL receives only typed, parameterized conditions.
9. Added relation, lineage, reference, and usage projections from visible AssetRelation rows, Collection memberships, registered Artifacts, ApplicationRun artifact projections, and stored lightweight reference summaries.
10. Added `artifact_content_completed` and `asset_version_representation_requested` outbox schema, TaskWorker consumers, and controlled `asset-library.artifact.process` / `asset-library.representation.finalize` handlers.
11. Added all released `150200-151404` business error values and regenerated Go registration plus error documentation.
12. Added route coverage for all 45 operations, owner/security/selector/storage/executor unit tests, and a PostgreSQL integration lifecycle test.
13. Updated the TaskWorker/API Server architecture documentation for the canonical asset-library flow.
14. Reviewed and reverted the uncommitted `reconciler.go` owner-terminal projection change because it could make Group/DAG state inconsistent with child projections and silently discarded errors.
15. Fixed `/api/v1/me` so the system administrator receives the released frontend-facing Asset Library, Task Center, and SSE permissions, with a regression test covering all 14 previously missing keys.
16. Fixed public AtomicTask responses so `key` is projected from the internal child key and persistence-only idempotency, runtime revision, scheduling, cancellation, description, and cross-domain linkage fields are not leaked as an old response shape.
17. Built `omnimam/apiserver:codex-atomic-response-amd64`, recreated only the local `omnimam-apiserver` container, and confirmed the live AtomicTask list returns the released structure.
18. Investigated the reported Asset relations/lineage response swap. API Server, frontend Nginx, Vite proxy, generated client, routes, controller, service, and store all use the released mapping: relations returns `total/items`, lineage returns `asset_id/nodes/edges`. Added a handler-binding regression assertion.
19. Diagnosed the Asset Library thumbnail failure without changing business code. Canonical upload publishes `asset_version_representation_requested`, but TaskWorker converts it directly into a successful `asset-library.representation.finalize` AtomicTask instead of the required inspect/generate/finalize DAG. Runtime PostgreSQL confirms both uploaded images remain `thumbnail_status=pending`, their versions are incorrectly `ready`, and only `original` Representations exist; there are zero thumbnail/preview Representations. The legacy `asset.thumbnail.generate` executor is healthy but only serves the old `assets`/`asset_thumbnails` model and is not scheduled for canonical AssetVersion records.
20. Fixed canonical image thumbnail generation: image versions now expect original plus a `list-320` thumbnail; the reliable event carries media policy, routing, and idempotency fields; TaskWorker creates an inspect/generate/finalize DAG; the generator reads original content through `ContentStorage`, writes a content-addressed PNG Blob, registers the thumbnail Representation, and projects `thumbnail_status`. Optional generation failure is recorded as a failed Representation so finalize can produce `ready_with_warnings`.
21. Built and deployed `omnimam/apiserver:codex-thumbnail-amd64` and `omnimam/taskworker:codex-thumbnail-amd64`. A live upload produced a ready `thumbnail:list-320` PNG Representation, advanced the version to `ready` with `expected_count=2/completed_count=2`, projected `thumbnail_status=ready`, and served the derived content as `image/png`; both smoke-test Assets and their Blobs were then permanently deleted.
22. Audited related-resource response gaps across Task Center, workflow-canvas, model-management, application-platform, asset-library, and ai-chatting without changing business code. Released `spec-v1.5.1` mostly defines scalar IDs, so response enrichment must start in SSOT. The proposed shape preserves every `*_id` and adds non-recursive, permission-filtered summary objects; same-domain references use explicit joins or bounded batch lookups, while cross-domain references use owner-provided read projections rather than direct table coupling.

## Files modified or added

- SSOT pin: `ssot`, `SSOT_VERSION`.
- Asset API and models: `backend/apis/iapiserver/meta_asset_v1.go`, `meta_asset_lifecycle.go`, `meta_asset_contract.go`, `request_asset_v1.go`.
- Controller/service/storage/parser/executors: `backend/internal/apiserver/controller/v1/assetlibrary/`, `backend/internal/apiserver/service/v1/assetlibrary/`.
- PostgreSQL stores: `asset_contract.go`, `asset_upload_contract.go`, `asset_collection_contract.go`, `asset_lifecycle_contract.go`, plus `asset_v1.go`, `outbox.go`, and `0_pg.go`.
- Bootstrap/task integration: `backend/internal/apiserver/route.go`, `server.go`, `taskworker.go`, and store interfaces.
- Errors/generated docs: `backend/internal/pkg/code/`, `docs/guide/zh-CN/api/error_code_generated.md`.
- Tests: route contract test, asset-library unit tests, selector SQL test, and PostgreSQL integration test.
- Documentation: `docs/guide/zh-CN/architecture/taskworker-apiserver-collaboration.md`, `docs/HANDOFF.md`.
- Frontend permission discovery: `backend/internal/apiserver/service/v1/platform/service.go`, `service_test.go`.
- Task Center public response projection: `backend/apis/iapiserver/response_task_center.go`, `backend/internal/apiserver/controller/v1/taskcenter/task_center.go`, `task_center_test.go`.
- Asset relation/lineage contract guard: `backend/internal/apiserver/controller/v1/assetlibrary/controller.go`, `backend/internal/apiserver/route_asset_library_contract_test.go`.
- Restored `backend/internal/apiserver/service/v1/taskcenter/reconciler.go` to the committed implementation after rejecting the uncommitted owner-terminal projection approach.
- Canonical thumbnail pipeline: `backend/internal/apiserver/taskworker.go`, `backend/internal/apiserver/service/v1/assetlibrary/executor.go`, `storage.go`, `executor_test.go`, `backend/internal/apiserver/store/store.go`, `backend/internal/apiserver/store/postgresql/asset_contract.go`, `asset_upload_contract.go`, `asset_lifecycle_contract.go`.
- Architecture and handoff: `docs/guide/zh-CN/architecture/taskworker-apiserver-collaboration.md`, `docs/HANDOFF.md`.
- Related-resource response analysis: `docs/HANDOFF.md` only; no API, schema, service, store, or runtime implementation changed.

## Key architectural decisions

- Canonical API facts use `user_assets`, `asset_versions`, `asset_representations`, `blobs`, `artifacts`, upload sessions, Collections, and normalized labels/tags. Legacy `assets` routes remain compatibility-only.
- owner is always injected from the authenticated user. Producer, task, application, canvas, or compatibility fields never grant cross-user access.
- File access is behind the service-owned `ContentStorage` interface. `LocalContentStorage` performs path confinement, atomic writes, bounded streaming, SHA256 validation, and content-addressed Blob placement.
- Selector parsing is database-independent. The store receives a typed AST and compiles static SQL fragments with bound values.
- Artifact content completion and AssetVersion representation requests are reliable outbox facts. TaskWorker creates idempotent AtomicTasks and state remains recoverable while Worker/Conductor is unavailable.
- `representation.finalize` only summarizes existing Representation facts; it does not invent thumbnail/preview/playback policy or infer ready from AtomicTask success.
- Public AtomicTask HTTP handlers use an explicit SSOT response DTO; service/store and Worker/runtime integrations continue using the complete persistence model.
- Asset relation and lineage endpoints remain distinct: the former is a pageable edge list and the latter is a graph projection. Empty relation rows must not be treated as a response-shape swap.
- The canonical Asset Library cannot reuse the legacy `asset.thumbnail.generate` executor as-is: canonical derived media must be stored as Blob-backed `AssetRepresentation` facts and orchestrated through the released Task Center DAG contract.
- The first released implementation policy is intentionally narrow: image versions expect original plus optional `thumbnail:list-320`; derived bytes cross the replaceable `ContentStorage` boundary and Blob/Representation facts remain owned by asset-library.
- The existing durable consumer group name is retained while its handler changes from direct finalize to DAG creation, preventing a deployment from replaying every acknowledged historical request under a new offset.
- Related resources should be exposed as one-hop domain-specific summaries beside the existing ID fields, not as recursively nested persistence models. Same-domain relationships may be resolved in the owning store; cross-domain relationships must respect module contracts and be composed from controlled batch read projections or immutable snapshots.

## API, schema, and configuration changes

- Added all 45 operations from `ssot/01_contracts/domains/asset-library/openapi.yaml`.
- Added current-schema initialization for `blobs`, `asset_representations`, `user_asset_upload_sessions`, `user_asset_groups`, and `user_asset_group_memberships`, plus the owner/name partial unique Collection index.
- Added outbox topics `artifact_content_completed` and `asset_version_representation_requested`.
- Added function refs `asset-library.artifact.process` and `asset-library.representation.finalize`.
- `/api/v1/me` now includes the released frontend-facing `asset.*`, `task.*`, and `sse.*` permissions requested by the current admin UI; internal-only permissions remain excluded.
- AtomicTask list, create, detail, cancel, retry, and Group/DAG child-list endpoints now return the released AtomicTask schema rather than serializing the database model directly.
- The local environment currently runs `omnimam/apiserver:codex-thumbnail-amd64`. An external compose/build flow subsequently replaced the verified Worker with `omnimam/taskworker:3dc3e40-amd64`; its startup logs confirm the new inspect/generate/finalize handlers are registered. PostgreSQL, Conductor, frontend, and other stateful services were not recreated by this task.
- No Asset relation/lineage API shape changed; the existing implementation already matches released OpenAPI.
- No new runtime flags were added. Upload chunk size currently defaults to 8 MiB in the asset-library service.
- `asset_version_representation_requested` now includes `project_id`, `namespace`, `media_type`, `requested_representations`, and the released DAG idempotency key. No HTTP API, database schema, or runtime configuration changed.
- Registered TaskWorker function refs now include `asset-library.representation.inspect` and `asset-library.representation.generate` in addition to finalize.
- Initial derived status follows the actual policy: image thumbnail starts pending, while unrequested preview and unsupported media-derived states start as none.
- The related-resource audit made no API, database schema, event, permission, error-code, or configuration change. Adding summary fields is currently blocked on an updated released SSOT contract.

## Verification results

- Passed `go test ./...`.
- Passed `go vet ./backend/internal/apiserver/... ./backend/apis/iapiserver/...`.
- Passed focused race tests for asset-library service/store/routes.
- Passed `git diff --check` and exact SSOT tag/commit synchronization.
- Passed route coverage for all 45 released operations.
- Passed isolated PostgreSQL 16 integration for canonical Asset lifecycle, cross-user isolation, Collection pinned version, Artifact registration, and reliable outbox rows.
- Passed focused Task Center `go test`, `go test -race`, `go vet`, and `git diff --check` after restoring the committed reconciler.
- Passed focused Platform service/controller tests and `go vet` after adding the `/me` permission regression coverage.
- Passed focused Task Center API/controller/service tests and `go vet` for the AtomicTask response projection; the HTTP regression test covers `GET /api/v1/atomic-tasks?page_num=0&page_size=1`.
- Passed repository-wide `go test ./...`, focused Task Center race tests, backend `go vet`, and a live HTTP smoke test against the rebuilt API Server container.
- Passed the Asset Library route contract test plus focused controller/service tests and `go vet`; live checks through ports 8080, 9990, and 9991 confirmed the correct relation/lineage shapes.
- Re-ran focused legacy thumbnail executor and canonical asset-library service tests; both passed.
- Queried the live PostgreSQL 16 database: 2 canonical image Assets are `thumbnail_status=pending`; their versions are `ready` with `expected_count=1`; each has only a ready `original` Representation; thumbnail and preview Representation counts are both 0.
- Queried live Task Center records and TaskWorker startup logs: both canonical requests produced only successful `asset-library.representation.finalize` AtomicTasks, while no `asset-library.representation.inspect`, thumbnail/preview generate, generic representation generate, or Representation backfill handler is registered.
- Added a generator regression test that creates a 640x320 image, verifies a 320x160 PNG output, and verifies Blob-backed ready Representation registration.
- Passed focused asset-library/store/apiserver tests, repository-wide `go test ./...`, and `git diff --check` after the thumbnail fix.
- Passed a live multi-process upload smoke test: the event created an inspect/generate/finalize DAG, the image version reached ready `2/2`, the Asset reached `thumbnail_status=ready`, and the Representation content endpoint returned HTTP 200 with `Content-Type: image/png` and 380 bytes.
- Verified `SSOT_VERSION`, the `ssot` submodule commit, and released tag `spec-v1.5.1` are synchronized. Reviewed released OpenAPI response schemas, module contracts, architecture boundaries, SQL foreign keys, response DTOs, and store/service query paths for the related-resource plan.

## Outstanding tasks

1. Update and release SSOT for related-resource summaries, starting with AtomicTask `root_task`, `retry_of_task`, and polymorphic `owner`, then define the same list/detail projection rule for the other domains before server implementation.
2. Implement the released related-resource response contract domain by domain with explicit response DTOs, permission-scoped batch resolvers, bounded query counts, and contract/integration tests.
3. Implement and register the `asset-library.representation-backfill` SYSTEM RECONCILE handler/schedule from S1/S2, including repair of image versions created before this fix.
4. Extend the media policy and generator adapters for preview, playback, package, and manifest; the current generator intentionally supports image thumbnails only.
5. Inject a natural-language resolver. Until configured, `natural_language_query` correctly returns `asset_search_dependency_failed` without keyword fallback.
6. Add explicit cross-domain reference projection APIs/events for Canvas, Application, and Task Center JSON-embedded asset references; permanent delete currently checks normalized Collection, Artifact, ApplicationArtifact, AssetRelation, and legacy group tables only.
7. Replace the remaining legacy application-platform Artifact registrar with the canonical Artifact create/content/complete/register workflow.
8. Run deployment smoke tests with rebuilt API Server and TaskWorker containers, including chunk upload, Artifact processing, Last-Event-ID SSE replay, and process restarts.
9. Add trusted producer projection paths for AssetRelation facts. The store supports relation writes, but current business flows do not create relation rows, so ordinary uploaded assets correctly return empty relation edges.

## Known issues and risks

- Historical image versions whose old representation request was already acknowledged remain pending until the Representation backfill handler is implemented or their request is replayed.
- The runtime schema still has no `representation_build_requests` or `user_asset_processing_tasks` models; the expected image plan is currently captured by AssetVersion counts plus the reliable event/DAG snapshot.
- Only image thumbnail generation is implemented. Video/audio/document preview, playback, package, and manifest policies remain absent.
- Image resizing currently uses deterministic nearest-neighbor sampling from the Go standard library. It is correct for functional thumbnails but should move behind a production image-processing adapter before advanced quality/orientation/color-profile requirements.
- Natural-language asset search has no resolver dependency configured.
- References embedded only inside arbitrary Canvas/Application/Task JSON cannot be discovered reliably for permanent deletion until owning domains publish normalized reference projections.
- The repository-wide response writer returns numeric `code`; asset-library OpenAPI also describes string `code` plus numeric `value`. Existing backend compatibility was preserved, so this response-shape mismatch remains.
- Local Blob file deletion occurs after the database deletion transaction. A filesystem failure can leave an unreferenced physical object that requires cleanup, but cannot delete referenced content.
- The first smoke upload was intentionally executed before rebuilding API Server and therefore reproduced the old `expected_count=1` path; its test Asset and Blob were deleted. The second smoke upload used both rebuilt processes and passed; its test Asset and Blobs were also deleted.
- The current sample asset has no AssetRelation rows; its relations list and lineage edges are empty even though both endpoint schemas are correct.
- Most released response schemas still expose only relationship IDs. Implementing nested summaries directly in this repository would violate SSOT, while naive per-row GORM loading would introduce N+1 queries and direct cross-domain joins would break module ownership and risk authorization leaks.

## Recommended next task

Prepare and release the related-resource summary contract in `omnimam-ssot`, beginning with Task Center and establishing the reusable one-hop summary, authorization, missing-reference, list/detail, and query-budget rules. Then update this repository's submodule pin before implementing Task Center as the first vertical slice.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
