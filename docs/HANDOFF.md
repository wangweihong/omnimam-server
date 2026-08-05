# Project Handoff

## Current goal and status

- Goal: update the pinned SSOT from `spec-v1.17.1` to released `spec-v1.17.2` and implement the required backend changes.
- Status: complete for the `spec-v1.17.2` release scope. The SSOT pin, generated contracts, Infrastructure delivery APIs, and `appstudio.build.execute@1.1` Artifact delivery are implemented and focused verification passes.
- Baseline SSOT: `spec-v1.17.1` at `35c2a582a53f9c416b71aa2695d99ad51797218e`.
- Target SSOT: annotated tag `spec-v1.17.2`, dereferenced release commit `5990e6054ec8b79a342ef6e979b6522b855378aa`.

## Work completed in this session

- Confirmed the server worktree is clean before starting.
- Read `skills/omnimam-server-backend/SKILL.md` and `backend/AGENTS.md`.
- Confirmed the remote release tag with `git ls-remote --tags origin`.
- Confirmed the current `SSOT_VERSION` and `ssot` gitlink both point to released `spec-v1.17.1`.
- Updated the `ssot` submodule to the released `spec-v1.17.2` tag commit and synchronized `SSOT_VERSION`.
- Read the `spec-v1.17.2` release entry and directly referenced S1/S2 diffs.
- Confirmed the implementation gate: add Endpoint resolve and RuntimeOutput content delivery contracts, upgrade `appstudio.build.execute` to `1.1`, and do not add a database migration or implement new Docker Provider behavior in this release task.
- Synchronized the embedded Function Registry and schema from the pinned SSOT, updated source metadata/retryability, added typed output delivery declarations, and normalized `status` to `ACTIVE` during contract digest calculation.
- Added the released Infrastructure request/response metadata, stable errors `240804..240807`, store operations, trusted process-local Provider endpoint/output state, output content validation, Endpoint resolution, output content reads, and idempotent Artifact attachment.
- Extended Asset Library producer validation to permit the released `studio_build` producer type.
- Added the released Infrastructure HTTP routes and client operations for Endpoint resolution, RuntimeOutput content reads, and idempotent Artifact attachment.
- Implemented `appstudio.build.execute@1.1` Task Worker delivery through the existing Artifact lifecycle, including content size/digest verification and ready-Artifact reuse.
- Replaced the Function Registry schema's unsupported negative-lookahead regex with an equivalent JSON Schema `allOf`/`not` constraint compatible with the backend's Go validator.
- Formatted the Task Worker files, fixed their missing imports, and verified `go test ./internal/taskworker` passes.
- Reviewed output materialization and delivery edge cases. Added fail-closed handling for a nil Provider result before dereference; confirmed successful jobs require every declared output, `CollectedAt` is preserved, and ready Artifact metadata supports JSON `float64` sizes.
- Ran `PATH=/home/wwhvw/goprogram/bin:$PATH make gen.deepcopy`; the existing API deepcopy output was regenerated successfully.
- Ran `PATH=/home/wwhvw/goprogram/bin:$PATH make gen.errcode`; generated registration and documentation now include errors `240804..240807`.
- Reviewed HTTP/client and store attachment behavior. Updated content-response detection to use the required `X-Content-Digest` header so legitimate `application/json` RuntimeOutput content is not mistaken for a business-error DTO.
- Ran final formatting and verified `go test ./internal/taskfunctionregistry`, `go test ./internal/infrastructure`, and `go test ./internal/taskworker` pass.

## Current in-progress work

- None.

## Files modified

- `docs/HANDOFF.md`
- `SSOT_VERSION`
- `ssot` gitlink
- `backend/internal/taskfunctionregistry/registry.go`
- `backend/internal/taskfunctionregistry/assets/function-registry.yaml`
- `backend/internal/taskfunctionregistry/assets/function-registry.schema.yaml`
- `backend/internal/taskfunctionregistry/assets/error-retryability.json`
- `backend/internal/taskfunctionregistry/assets/SOURCE`
- `backend/apis/iapiserver/meta_asset_lifecycle.go`
- `backend/apis/iapiserver/meta_infrastructure.go`
- `backend/apis/iapiserver/request_asset_v1.go`
- `backend/apis/iapiserver/request_infrastructure.go`
- `backend/apis/iapiserver/response_infrastructure.go`
- `backend/apis/iapiserver/deepcopy_generated.go`
- `backend/internal/apiserver/store/store.go`
- `backend/internal/apiserver/store/postgresql/asset_lifecycle_events.go`
- `backend/internal/apiserver/store/postgresql/infrastructure.go`
- `backend/internal/infrastructure/provider.go`
- `backend/internal/infrastructure/service.go`
- `backend/internal/infrastructure/server.go`
- `backend/internal/infrastructure/client.go`
- `backend/internal/taskworker/taskworker.go`
- `backend/internal/taskworker/taskworker_test.go`
- `backend/internal/pkg/code/release_v1172.go` (added)
- `backend/internal/pkg/code/code_generated.go`
- `docs/guide/zh-CN/api/error_code_generated.md`

No files were renamed or removed. No files under `ssot/` were modified.

## Key decisions

- Pin the server repository to tag commit `5990e6054ec8b79a342ef6e979b6522b855378aa`; the release record identifies `64435e32db213bf4483d057039036375ee545183` as the contract-content commit contained in that tag.
- Do not change backend behavior until the released S1/S2 diff identifies the owning domain and exact contract changes.
- Keep repository reads limited to the release entry, directly referenced specifications, the target module, and directly related tests.
- Do not use the registry generator for this task because it recursively reads every SSOT domain error catalog; mechanically synchronize only the two explicitly selected Task Center registry sources.
- Existing Docker Provider behavior remains unchanged. Until a later scoped Provider implementation supplies trusted endpoint targets and collected output content, resolve/content delivery must fail closed with the released stable errors.
- Because this release forbids a database migration, endpoint targets and output content descriptors/readers remain trusted process-local Provider state; only the existing output identity/status/media type/Artifact attachment fields are persisted.
- A nil Provider result is treated as `ERR_INFRA_RUNTIME_OPERATION_FAILED` and persists the Runtime as failed instead of panicking.

## API, schema, dependency, and configuration changes

- Required API additions: Endpoint resolve, RuntimeOutput content read, and RuntimeOutput Artifact attach.
- Required request/response additions: named Endpoint request, declared outputs, resolve DTOs, collected output metadata, and attach DTO.
- Required error additions: `ERR_INFRA_ENDPOINT_NOT_READY` and output collection/content/integrity errors `240805..240807`.
- Required permissions: `infra.endpoint.resolve`, `infra.output.content.read`, and `infra.output.artifact.attach`.
- Function Registry schema becomes `1.1`; `appstudio.build.execute@1.0` becomes RETAINED and `1.1` becomes ACTIVE with output delivery policy.
- Design schema changed, but the release explicitly excludes a server database migration from this implementation task.
- No dependency or deployment configuration change identified yet.

## Verification performed

- `git status --short` was clean at task start.
- Remote tag verification passed for `spec-v1.17.2` and its dereferenced commit.
- Current submodule/version consistency passed for `spec-v1.17.1`.
- Target submodule/tag verification passed for `spec-v1.17.2` at `5990e6054ec8b79a342ef6e979b6522b855378aa`.
- `go test ./internal/taskfunctionregistry` passed.
- `go test ./internal/infrastructure` passed.
- `go test ./internal/taskworker` passed after the schema compatibility fix.
- `git diff --check` passed after formatting the current Go changes.

## Outstanding tasks

- None for the requested `spec-v1.17.2` implementation scope.

## Known issues and risks

- The schema contract adds endpoint/output persistence fields, but the release gate forbids a migration. Endpoint targets and staged output readers are therefore lost on process restart and correctly fail closed until Provider support is separately scoped.
- Existing Docker Provider output collection and endpoint target resolution remain intentionally unimplemented by this release task, so those paths fail closed until separately scoped.

## Exact recommended next step

Push the current branch and open or update the review for the `spec-v1.17.2` implementation.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
