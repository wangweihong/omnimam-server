# Project Handoff

## Current project goal

Implement the released `spec-v1.6.5` one-hop related-resource response rules across the backend while preserving stable IDs, owner/visibility boundaries, domain ownership, and bounded list query counts.

## Completed in this session

1. Pinned `ssot` and `SSOT_VERSION` to released `spec-v1.6.5` commit `e3cbfef1ee55bc9a497a128bc202b1e0b44bd28f`.
2. Implemented Task Center summaries for AtomicTask root/retry/owner, TaskAttempt task, Group/DAG retry source, ScheduleExecution schedule/target, plus controlled AtomicTask and DAG summary readers.
3. Implemented ApplicationRun response projection and Application/ApplicationVersion/TemplateVersion/ProviderCapability/EngineInstance/AtomicTask summaries. New runs save non-sensitive snapshots; old runs use bounded fallback reads.
4. Implemented Workflow Canvas Canvas/Version/Run/retry/DAGTaskGroup/AtomicTask summaries with fixed same-domain batches and Task Center controlled reads.
5. Implemented AI Chat Topic assistant/model, Assistant suggested-model, and assistant QuickPhrase summaries. Provider models/providers and assistants use owner-scoped batch reads; cross-user model invocation is rejected.
6. Implemented Asset Library current-version, Artifact producer/task/run/registered-asset, Collection parent/pinned-version, and AssetRelation endpoint summaries. Cross-domain Artifact relations use consumer-defined interfaces and source-domain controlled readers.
7. Replaced Collection list per-row depth/count reads with bounded ancestor batches plus one grouped membership count query.
8. Made Model Management `ProviderModel.provider_name` a real required projection across list/create/update/delete/default-model/model-option paths, using one owner-scoped Provider batch for lists.
9. Added JSON serialization coverage proving `ApplicationRunResponse.artifacts` overrides the persistence Artifact shape.

## Files modified or added

- SSOT pin: `ssot`, `SSOT_VERSION`.
- Public DTOs: `backend/apis/iapiserver/meta_{task_center,application_platform,workflow_canvas,ai_chat,asset_*}.go`, `request_asset_v1.go`, `response_task_center.go`, `response_application_platform.go`.
- Service projections: Task Center `relations.go`; AI Chat `relations.go`; Asset Library `relations.go`; Application Platform `run_summaries.go`; Workflow Canvas and Platform services.
- Store batches: PostgreSQL Task Center, Workflow Canvas, AI Chat, Model Management, Application Platform, and Asset Library stores plus `store.go` interfaces.
- Bootstrap: `backend/internal/apiserver/service/v1/service.go`, `backend/internal/apiserver/route.go`.
- Tests: focused summary, serialization, bounded-batch, missing-reference, and cross-user visibility tests in the affected packages.

## Key architectural decisions

- Stable IDs remain in every response; summaries are nullable one-hop projections and never contain credentials, large content, task input/output, provider payloads, or recursive relations.
- Missing, deleted, or invisible relations preserve the parent response and original ID while omitting the summary.
- Historical ApplicationRun and CanvasRun facts prefer creation-time non-sensitive snapshots; current table reads are fallback only.
- Cross-domain reads use consumer-defined interfaces implemented by the owning domain. Asset Library does not query Task Center, Application Platform, or Workflow Canvas private tables.
- List projections use fixed batch counts. Asset Collection hierarchy is capped at eight ancestor batches by the released depth limit.
- No database migration, new persisted field, error code, permission, event type, or runtime flag was added.

## API, schema, and configuration changes

- Backend now implements response-only fields released from `spec-v1.6.0` through `spec-v1.6.5`.
- Added store batch methods for related resources and source-domain controlled summary methods.
- `ProviderModel.provider_name` is populated from the current owner's Provider record and is required by `spec-v1.6.5`.
- `ApplicationRun` HTTP serialization uses the released `ApplicationArtifactRef` shape instead of the persistence row shape.

## Verification results

- Focused tests passed for Task Center, Application Platform, Workflow Canvas, AI Chat, Asset Library, Platform/Model Management, PostgreSQL stores, API DTOs, and route coverage during implementation.
- AI Chat owner-isolation and ApplicationRun JSON serialization tests passed after the final changes.
- `git diff --check` passed.
- Per user direction, no additional Model Management full-suite run was performed.

## Outstanding tasks

1. Run repository-wide `go test ./...` and `go vet ./backend/...` only if broader release validation is requested; the latest turn intentionally kept Model Management verification focused.
2. Run PostgreSQL integration tests with `OMNIMAM_TEST_POSTGRES_DSN` to validate Asset Collection/Artifact summary SQL against a real database.
3. Consider tightening Task Center controlled summary visibility beyond `created_by` when project/namespace sharing semantics are expanded.
4. Implement the previously outstanding Asset Representation backfill and broader media policies; they are independent of this response-projection change.

## Known issues and risks

- Summary omission is intentional for stale or invisible relations; clients must continue to handle the stable ID fallback.
- ApplicationRun and CanvasRun producer names fall back to a generic run label when historical rows have neither a name nor a saved source snapshot.
- Asset Collection ancestor resolution is bounded by the released maximum depth of eight; invalid legacy cycles stop without recursive expansion.
- No live deployment or browser/API smoke test was performed in this session.

## Recommended next task

Run the Asset Library PostgreSQL integration path and a live list/detail smoke test, then proceed to Representation backfill without revisiting completed summary work.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
