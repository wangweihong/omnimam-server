# Project Handoff

## Current project goal

Implement the released `spec-v1.5.1` asset-library API contract while keeping asset-library as the fact source for Asset, AssetVersion, AssetRepresentation, Blob, Artifact, upload, Collection, Label, and Tag state.

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

## Files modified or added

- SSOT pin: `ssot`, `SSOT_VERSION`.
- Asset API and models: `backend/apis/iapiserver/meta_asset_v1.go`, `meta_asset_lifecycle.go`, `meta_asset_contract.go`, `request_asset_v1.go`.
- Controller/service/storage/parser/executors: `backend/internal/apiserver/controller/v1/assetlibrary/`, `backend/internal/apiserver/service/v1/assetlibrary/`.
- PostgreSQL stores: `asset_contract.go`, `asset_upload_contract.go`, `asset_collection_contract.go`, `asset_lifecycle_contract.go`, plus `asset_v1.go`, `outbox.go`, and `0_pg.go`.
- Bootstrap/task integration: `backend/internal/apiserver/route.go`, `server.go`, `taskworker.go`, and store interfaces.
- Errors/generated docs: `backend/internal/pkg/code/`, `docs/guide/zh-CN/api/error_code_generated.md`.
- Tests: route contract test, asset-library unit tests, selector SQL test, and PostgreSQL integration test.
- Documentation: `docs/guide/zh-CN/architecture/taskworker-apiserver-collaboration.md`, `docs/HANDOFF.md`.
- A pre-existing uncommitted change in `backend/internal/apiserver/service/v1/taskcenter/reconciler.go` was preserved and was not modified as part of this task.

## Key architectural decisions

- Canonical API facts use `user_assets`, `asset_versions`, `asset_representations`, `blobs`, `artifacts`, upload sessions, Collections, and normalized labels/tags. Legacy `assets` routes remain compatibility-only.
- owner is always injected from the authenticated user. Producer, task, application, canvas, or compatibility fields never grant cross-user access.
- File access is behind the service-owned `ContentStorage` interface. `LocalContentStorage` performs path confinement, atomic writes, bounded streaming, SHA256 validation, and content-addressed Blob placement.
- Selector parsing is database-independent. The store receives a typed AST and compiles static SQL fragments with bound values.
- Artifact content completion and AssetVersion representation requests are reliable outbox facts. TaskWorker creates idempotent AtomicTasks and state remains recoverable while Worker/Conductor is unavailable.
- `representation.finalize` only summarizes existing Representation facts; it does not invent thumbnail/preview/playback policy or infer ready from AtomicTask success.

## API, schema, and configuration changes

- Added all 45 operations from `ssot/01_contracts/domains/asset-library/openapi.yaml`.
- Added current-schema initialization for `blobs`, `asset_representations`, `user_asset_upload_sessions`, `user_asset_groups`, and `user_asset_group_memberships`, plus the owner/name partial unique Collection index.
- Added outbox topics `artifact_content_completed` and `asset_version_representation_requested`.
- Added function refs `asset-library.artifact.process` and `asset-library.representation.finalize`.
- No new runtime flags were added. Upload chunk size currently defaults to 8 MiB in the asset-library service.

## Verification results

- Passed `go test ./...`.
- Passed `go vet ./backend/internal/apiserver/... ./backend/apis/iapiserver/...`.
- Passed focused race tests for asset-library service/store/routes.
- Passed `git diff --check` and exact SSOT tag/commit synchronization.
- Passed route coverage for all 45 released operations.
- Passed isolated PostgreSQL 16 integration for canonical Asset lifecycle, cross-user isolation, Collection pinned version, Artifact registration, and reliable outbox rows.

## Outstanding tasks

1. Implement media-policy inspect/generate/finalize DAGs for thumbnail, preview, playback, package, and manifest. Current finalize only aggregates already registered representations.
2. Implement and register the `asset-library.representation-backfill` SYSTEM RECONCILE handler/schedule from S1/S2.
3. Inject a natural-language resolver. Until configured, `natural_language_query` correctly returns `asset_search_dependency_failed` without keyword fallback.
4. Add explicit cross-domain reference projection APIs/events for Canvas, Application, and Task Center JSON-embedded asset references; permanent delete currently checks normalized Collection, Artifact, ApplicationArtifact, AssetRelation, and legacy group tables only.
5. Replace the remaining legacy application-platform Artifact registrar with the canonical Artifact create/content/complete/register workflow.
6. Run deployment smoke tests with rebuilt API Server and TaskWorker containers, including chunk upload, Artifact processing, Last-Event-ID SSE replay, and process restarts.

## Known issues and risks

- The media-derived Representation DAG and periodic backfill are not implemented, so uploaded binary versions can become ready based on original content without generated thumbnail/preview/playback.
- Natural-language asset search has no resolver dependency configured.
- References embedded only inside arbitrary Canvas/Application/Task JSON cannot be discovered reliably for permanent deletion until owning domains publish normalized reference projections.
- The repository-wide response writer returns numeric `code`; asset-library OpenAPI also describes string `code` plus numeric `value`. Existing backend compatibility was preserved, so this response-shape mismatch remains.
- Local Blob file deletion occurs after the database deletion transaction. A filesystem failure can leave an unreferenced physical object that requires cleanup, but cannot delete referenced content.
- No deployment-level multi-process smoke test was run in this session.

## Recommended next task

Implement `asset-library.representation.inspect/generate/finalize` as a policy-driven DAG plus the `asset-library.representation-backfill` SYSTEM RECONCILE schedule, then run a rebuilt deployment smoke test.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
