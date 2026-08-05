# Project Handoff

## Current goal and status

- Goal: fix backend defects reported in `/home/wwhvw/codespace/omnimam-web/docs/real-user-test-appstudio-agent-2026-08-05.md`.
- Status: RBUG-004 and RBUG-006 are implemented, tested, deployed, and live-verified. `spec-v1.17.1` is pinned and validated, and its independent StudioBuild owner/name/batch-summary contract is implemented, tested, and deployed. RT-006 is blocked by the released Infrastructure Endpoint S1/S2 conflict; RT-010 is blocked by the released Infrastructure RuntimeOutput S1/S2 conflict; RT-013 is blocked downstream by RT-010 because no conforming READY Build Artifact exists.

## Work completed in this session

- Read the retest report and identified six reported failures: default-model empty state, ProviderType catalog, Agent Invocation scheduling, AppStudio source snapshot persistence, build artifact registration, and Preview completion/access.
- Read `skills/omnimam-server-backend/SKILL.md`, `backend/AGENTS.md`, and the applicable Go troubleshooting, safety, and testing skills.
- Verified the implementation gate: `SSOT_VERSION.commit == ssot HEAD == 35c2a582a53f9c416b71aa2695d99ad51797218e`, released as `spec-v1.17.1`.
- Fetched released tag `spec-v1.17.1`, pinned `ssot` to that commit, synchronized `SSOT_VERSION`, and completed the independently implementable StudioBuild contract additions.
- Confirmed the worktree was clean at task start.
- Verified the frontend RBUG-001/RBUG-002 fix in the live environment: `/providers` no longer renders business code `120600` as an HTTP failure, and the ProviderType selector loads `DeepSeek Official API`.
- Created and tested a real DeepSeek provider through the local UI, synchronized its models, verified the selected model connection, and saved it as `assistant.default`. The user-provided credential was not persisted in repository files or this handoff.
- Retested the existing Agent session with the real default model. Invocation still fails with `ERR_AGENT_INVOCATION_TASK_UNAVAILABLE` and has no AtomicTask, confirming that model configuration does not unblock RBUG-003.
- Concluded that RBUG-003 remains an SSOT/runtime-adapter blocker: the released SSOT does not provide a confirmed canonical CHAT `functionRef`, so no task contract will be invented in this repository.
- Read the released AppStudio Snapshot sources: S1 `5.5 StudioSourceSnapshot`, OpenAPI `create_studio_source_snapshot`, and `ERR_APPSTUDIO_SNAPSHOT_INVALID (210601)`.
- Added a regression case to the existing `backend/internal/apiserver/store/postgresql/outbox_test.go` because backend rules prohibit creating a new non-`pkg` `_test.go` file.
- Ran the focused regression before the production fix. It failed as expected because `CreateSnapshot` returned and persisted a `READY` snapshot for revision `0` with the empty-tree digest.
- Updated `CreateSnapshot` to query the immutable revision's active source-file rows and return `ERR_APPSTUDIO_SNAPSHOT_INVALID (210601)` when none exist. This also rejects later revisions that delete all files rather than special-casing revision `0`.
- Expanded the regression into named empty/non-empty cases so valid source revisions remain able to create `READY` snapshots.
- Built `omnimam/apiserver:codex-rbug4-20260805-amd64`, replaced only the `apiserver` Compose service with `--no-deps`, and confirmed `GET http://127.0.0.1:8080/healthz` returns HTTP 200 with `{"status":"ok"}`.
- Live-retested project `4a2cbd8a-9e02-4387-b399-d007ebdeee4f`: clicking `创建源码快照` for revision `0` shows `source revision is empty`, does not show success, and leaves Revision `0` / Source Snapshot `-` unchanged.
- Queried `studio_source_snapshots` before and after the live action. Both checks show the same single historical `READY` row created at `2026-08-05 08:55:53.944656+00`; no new empty snapshot was persisted.
- Updated the web real-user report and web handoff with the RBUG-004 live result.
- Built and deployed `omnimam/apiserver:codex-rbug6-20260805-amd64` and `omnimam/taskworker:codex-rbug6-20260805-amd64`; both containers are running and API Server health returned HTTP 200 with `{"status":"ok"}`.
- Live-retested RBUG-006 in the authenticated AppStudio UI. Refreshing the project changed the Preview access entry from `-` to `infra-endpoint://0824b7f3-94bb-49bd-ba17-3fed51f59dfe` while status remained `RUNNING`.
- Submitted a new Preview check. AtomicTask `955ac7fb-d7e2-4d72-84af-26cffc31311a` completed as `SUCCESS`; Preview `e79bb3fe-5000-4f23-8ca5-a4501ffdb10a` is `RUNNING` with endpoint `infra-endpoint://f8d8e507-3378-4632-b754-7dfd91f2939d`, infrastructure runtime `d90540c4-4d21-40cf-986c-90c94aea0db0`, and persisted diagnostics `{"health_status":"HEALTHY"}`.

## Current in-progress work

- No further conforming RT-006/RT-010/RT-013 backend implementation is available under released `spec-v1.17.1`. Work is stopped at the two Infrastructure S1/S2 conflicts documented below; do not add a modelgateway shortcut, fabricate a callable endpoint, or register an empty Artifact.
- Added a transaction-rolled-back PostgreSQL regression in `task_center_reconcile_integration_test.go`. It proves `infra_runtime_id`, `endpoint_ref`, and `RUNNING` persist, but fails because `ProjectStudioTaskTerminal` omits the released mapping `result.health_status -> diagnostics_summary.health_status`.
- Fixed `ProjectStudioTaskTerminal` to preserve existing Preview diagnostics and persist successful task output `health_status` as `diagnostics_summary.health_status`, matching the released function registry mapping.
- Live database inspection showed the latest Preview task is `SUCCESS` and the Preview row already contains `infra_runtime_id`, `endpoint_ref`, and `RUNNING`; the UI still showed `-` because `EndpointRef` is intentionally not serialized and no read path populated the public `endpoint_summary`.
- Fixed `GetStudioPreviewRuntime` to expose a controlled `USER_ACCESSIBLE`/`READY` EndpointSummary for a running Preview with a persisted endpoint reference. It does not read or expose provider-private addresses or Host Ports.
- The normal package-level integration command is currently blocked by an unrelated pre-existing compile error in `canvas_application_integration_test.go:103` (`map[string]any` assigned to `TaskCancelPolicy`). The target regression is run with production `.go` files plus the one target integration test file; do not modify the unrelated test for this task.
- RBUG-005 AppStudio build worker fails closed after argument validation because artifact registration is not implemented.
- The released contract requires `appstudio.build.execute` to call Infrastructure, register output `bundle` as `producer_type: studio_build` with producer ID `arguments.studio_build_id` and idempotency key `studio-build:{studio_build_id}:bundle`, then return Artifact identity/digest/statuses plus `logs_ref`.
- `spec-v1.17.1` resolves the prior AppStudio/Asset producer conflict by releasing `producer_type: studio_build` and adds canonical StudioBuild `owner_user_id`, `name`, bundle idempotency, and batch-summary contracts.
- RBUG-005/RT-010 is still blocked by a direct released Infrastructure S1/S2 conflict. S1 `8.6 RuntimeOutput` requires `path`, `mediaType`, `size`, `contentDigest`, and `artifactRef`, while the released Infrastructure OpenAPI/schema and current server DTO expose only `output_key`, `status`, `artifact_id`, and `media_type` (plus an operation-level `ArtifactDigest`). No controlled content reference, output size, or per-output digest is available to satisfy Asset Library upload completion.
- The current Docker JOB provider only waits for container exit and returns `SUCCEEDED`; it does not collect output files, calculate digests, populate `Outputs`, or return a trustworthy content reference. It also rejects requested mounts when no source resolver is configured.
- Do not create an empty Artifact from the operation-level `ArtifactDigest`. Asset Library requires actual controlled content plus matching `sha256`, `size_bytes`, and `mime_type` before completion.
- Implemented the independent `spec-v1.17.1` StudioBuild contract additions: new Builds receive a stable server-generated `name`, `owner_user_id` is returned as the canonical owner, and `POST /api/v1/studio-builds/batch-summaries` returns the ordered `id/owner_user_id/name/status` projection with missing or invisible entries represented as `null`.
- Passed the focused DTO, AppStudio service, and PostgreSQL regressions for the StudioBuild batch-summary contract, including request-order preservation, owner scoping, and `studio_build: null` for missing or invisible Builds.
- Built and deployed `omnimam/apiserver:codex-spec-v1171-amd64`, replacing only the API Server. `GET http://127.0.0.1:8080/healthz` returns HTTP 200 and the container is running.
- Reused the authenticated in-app browser session and confirmed the AppStudio page still loads Build `fade5990-92d4-4fdb-a36f-6036b8d93628`, the Preview endpoint projection, and the expected disabled Release state. Direct API navigation does not carry the frontend-managed Bearer header; the deployed batch-summary route was therefore additionally checked at its authentication boundary and returns `ERR_IDENTITY_AUTH_HEADER_EMPTY (100205)` instead of a route error when called without a token.

## Files added, modified, renamed, or removed

- Modified `docs/HANDOFF.md` (checkpoint only).
- Modified `backend/internal/apiserver/store/postgresql/outbox_test.go` (failing empty-snapshot regression).
- Modified `backend/internal/apiserver/store/postgresql/task_center_reconcile_integration_test.go` (RBUG-006 successful Preview terminal projection regression).
- Modified `backend/internal/apiserver/store/postgresql/appstudio.go` (persist Preview health status during successful terminal projection and hydrate the public EndpointSummary on reads).
- Modified `backend/internal/apiserver/service/v1/appstudio/service.go` (reject empty source revisions before Snapshot persistence).
- Modified `backend/apis/iapiserver/meta_appstudio.go`, `request_appstudio.go`, and `response_appstudio.go` (StudioBuild owner visibility and batch-summary DTOs).
- Modified `backend/internal/apiserver/store/store.go` and `backend/internal/apiserver/store/postgresql/appstudio.go` (owner-scoped Build summary resolution).
- Modified `backend/internal/apiserver/controller/v1/appstudio/appstudio.go` and `backend/internal/apiserver/route.go` (released batch-summary endpoint).
- Expanded `backend/internal/apiserver/store/postgresql/outbox_test.go` with ordered/null Build summary regression coverage.

## Key architectural or design decisions

- Apply only released SSOT behavior and existing task/provider/runtime contracts; do not invent APIs, task function refs, error codes, persistence fields, or events.
- `spec-v1.17.1` releases the Agent invocation/runtime lifecycle. Platform CHAT requiring a Runtime must ensure/project a ready Runtime and invoke it through an `AgentRuntimeAdapter`; direct per-message modelgateway proxying remains outside the released boundary.
- Keep investigation and verification limited to the current AppStudio snapshot module, its direct SSOT entries, and its directly related tests.

## API, schema, dependency, or configuration changes

- Added the newly released StudioBuild API projection and batch-summary route. No database migration was required because `owner_user_id` and the embedded ObjectMeta `name` column already exist; new Builds now populate both. RBUG-004 reuses released `ERR_APPSTUDIO_SNAPSHOT_INVALID (210601)`.

## Verification performed and remaining checks

- Verified released SSOT pin and clean starting worktree.
- Confirmed through the live environment that RBUG-001/RBUG-002 are resolved and that RBUG-003 still fails after real DeepSeek default-model configuration.
- Focused pre-fix test: `go test ./internal/apiserver/store/postgresql -run '^TestCreateStudioSourceSnapshotRejectsEmptyRevision$' -count=1` failed at the expected assertion because an empty `READY` snapshot was returned.
- Post-fix AppStudio service compile: `go test ./internal/apiserver/service/v1/appstudio -count=1` passed (`[no test files]`).
- Post-fix direct tests: `go test ./internal/apiserver/store/postgresql -run 'Test(CreateStudioSourceSnapshotRejectsEmptyRevision|AppStudioEvent)' -count=1` passed.
- RBUG-006 pre-fix file-level integration regression failed at the expected assertion: persisted diagnostics were `{}` instead of `{"health_status":"HEALTHY"}`. The same run confirmed runtime ID, endpoint ref, and `RUNNING` were persisted.
- RBUG-006 post-fix file-level integration regression passed with real PostgreSQL in a rolled-back transaction.
- Direct AppStudio/PostgreSQL tests passed: `go test ./backend/internal/apiserver/store/postgresql -run 'Test(CreateStudioSourceSnapshotRejectsEmptyRevision|AppStudioEvent|NewOutboxMessage)' -count=1`.
- Task Worker package tests passed: `go test ./backend/internal/taskworker -count=1`.
- StudioBuild API DTO tests passed: `go test ./apis/iapiserver -count=1`.
- AppStudio service package compile passed: `go test ./internal/apiserver/service/v1/appstudio -count=1` (`[no test files]`).
- API Server compile check passed: `go test ./internal/apiserver -run '^$' -count=1`.
- Focused PostgreSQL/AppStudio regressions passed for ordered and owner-scoped Build summaries, missing/invisible `null` results, empty Snapshot rejection, and AppStudio outbox/event behavior.
- `git diff --check` passed before deployment. The patched API health check passed after deployment.
- Live RBUG-004 verification passed: the UI reports `source revision is empty` instead of success, and the database snapshot count/latest timestamp are unchanged. The UI does not render numeric business codes; code `210601` is confirmed by the deployed released error mapping and focused regression.
- Live RBUG-006 verification passed after refresh and after a newly submitted Preview check: the UI displays the endpoint reference, the latest AtomicTask is `SUCCESS`, the Preview is `RUNNING`, and `diagnostics_summary_json` contains `{"health_status":"HEALTHY"}`.
- Live `spec-v1.17.1` deployment verification passed for the existing AppStudio Build list and Preview projection. An authenticated browser POST to `batch-summaries` remains unverified because the in-app browser intentionally does not expose the frontend-managed Bearer token or arbitrary page networking; automated owner/order/null behavior is covered by the focused PostgreSQL regression, and the deployed route's authentication boundary is live.

## Outstanding tasks

- Update and release Infrastructure Endpoint SSOT: add a controlled Endpoint resolve API or equivalent internal contract; define how `AgentRuntimeAdapter` obtains the callable Hermes address; define Docker single-node port allocation, internal address publication, and authorization checks.
- Update and release Infrastructure RuntimeOutput SSOT: add canonical per-output `size_bytes`, `content_digest`, and `mime_type`; define whether `path` is provider-private, a trusted storage reference, or a controlled download reference; define how the worker reads the exact bytes registered in Asset Library; define reference authorization, lifetime, and cleanup.
- After a released SSOT provides the controlled Build content handoff, add focused regression coverage in an existing test file, implement Artifact upload/completion, and retest RT-010 and RT-013.
- After a released SSOT provides a callable controlled Endpoint contract, implement and inject `AgentRuntimeAdapter`, then retest RT-006 with the configured real DeepSeek model.

## Known issues and risks

- `spec-v1.17.1` defines Agent runtime lifecycle but the current repository has no confirmed `AgentRuntimeAdapter` injection; implementing a direct modelgateway shortcut would violate the released module boundary.
- Agent Invocation is additionally blocked by the Infrastructure Endpoint contract: Infrastructure S1 defines controlled resolution of `internalAddress`/`externalAddress`, while released S2 exposes only a non-address summary and no controlled resolve API. The current Docker provider publishes no port/address and returns only `infra-runtime://<runtime_id>`, so an AgentRuntimeAdapter has no callable Hermes endpoint.
- The prior `studio_build` producer conflict is resolved, but the remaining Infrastructure RuntimeOutput S1/S2 conflict prevents a conforming Build Artifact implementation. S2 must define the controlled content retrieval/reference semantics and `size_bytes`, `content_digest`, and `mime_type` fields before server implementation can continue.
- Do not modify files inside `ssot/` or add a binary under `backend/cmd/`.

## Exact recommended next step

Modify the Infrastructure Endpoint and RuntimeOutput S1/S2 contracts in the SSOT repository, release a new spec version, then update this repository's pinned submodule before continuing RT-006, RT-010, and RT-013. Do not route CHAT directly through modelgateway and do not create an Artifact without controlled source bytes.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
