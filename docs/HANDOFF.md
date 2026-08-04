# Project Handoff

## Current goal and status

- Goal: update the pinned SSOT to released `spec-v1.15.3` and implement the missing Infrastructure service APIs in `infra-server`.
- Status: complete. The SSOT pin and all 15 released `/api/v1/infra/*` operations are implemented and verified with the allowed focused checks.

## Work completed in this session

- Validated released tag `spec-v1.15.3` at `0c93e518f64d11b466e2fef7dae47f20ac3150b0` and updated the `ssot` submodule plus root `SSOT_VERSION`.
- Confirmed the released Infrastructure OpenAPI defines 15 service-authenticated operations for runtimes, endpoints, logs, nodes, profiles, and outputs.
- Registered all 15 operations in the existing `infra-server` HTTP server without adding a binary or controller module.
- Added static service Bearer authentication for `/api/v1/infra/*`, preserving the existing internal command response contract.
- Added Infrastructure-specific symbolic/numeric error responses and retryability mapping without adding error codes.
- Added `owner_reference` Runtime filtering, node `status` filtering, and Infrastructure pagination defaults of 50 with a maximum of 200.
- Added output/log pagination, changed Job stop semantics to `CANCELED`, and changed delete to return the deleted Runtime summary.

## Current in-progress work

- None.

## Files added, modified, renamed, or removed

- Modified: `SSOT_VERSION`.
- Modified: `ssot` submodule pointer.
- Modified: `backend/apis/iapiserver/request_infrastructure.go`.
- Modified: `backend/internal/apiserver/store/postgresql/infrastructure.go`.
- Modified: `backend/internal/infrastructure/service.go`.
- Modified: `backend/internal/infrastructure/server.go`.
- Modified: `docs/HANDOFF.md`.
- Existing unrelated modifications in `backend/internal/apiserver/service/v1/agent/service.go` and `third_party/gotoolbox/pkg/generic/generic.go` remain untouched.

## Key architectural or design decisions

- `/api/v1/infra/*` remains a service boundary for the `task-center` trusted identity; this does not authorize direct Web/browser calls.
- The existing `infra-server` binary, Service, Store, and provider abstractions are reused; no new schema, migration, dependency, environment variable, or binary is introduced.
- Ordinary Infrastructure business errors use HTTP 200; authentication uses the existing token-invalid code and HTTP 401; unexpected errors use HTTP 500.
- API write operations project `InfraOperationResult.Runtime` because the released OpenAPI response is `InfraRuntime`.
- Runtime and node filtering is performed in PostgreSQL; bounded output/log result sets are paginated in the Infrastructure Service.

## API, schema, dependency, or configuration changes

- Added the 15 released `/api/v1/infra/*` HTTP operations to `infra-server`.
- Added request support for Runtime `owner_reference` and node `status` filters.
- Infrastructure list pagination now defaults to 50 and rejects values above 200 at the HTTP boundary.
- No database schema, migration, dependency, error-code registry, permission-code registry, event type, environment variable, or binary changes.

## Verification performed and remaining checks

- `git submodule status ssot` reports `0c93e518f64d11b466e2fef7dae47f20ac3150b0 ssot (spec-v1.15.3)`.
- Ran `gofmt` on the four modified Go implementation files; the pre-existing user whitespace change in `backend/internal/infrastructure/service.go` was restored afterward rather than discarded.
- `go test ./backend/apis/iapiserver ./backend/internal/infrastructure ./backend/internal/apiserver/store/postgresql` passes; Infrastructure compiles and has no test files, while the API DTO and PostgreSQL Store tests pass.
- `git diff --check` passes.
- Confirmed `SSOT_VERSION.commit`, `SSOT_VERSION.contract_version`, the submodule commit, and the exact tag agree on `0c93e518f64d11b466e2fef7dae47f20ac3150b0` / `spec-v1.15.3`.
- Confirmed the public Runtime JSON hides `ProviderRuntimeRef`, selected node, source ref, timeout-policy shadow, mount target/authorization references, and provider event identifiers via `json:"-"`; Docker endpoint display refs use the controlled `infra-runtime://` form.
- No full-repository test was run, in accordance with the task verification scope.

## Outstanding tasks

- None for this task.

## Known issues and risks

- Runtime log pagination is limited to the provider's existing retained tail of up to 5000 sanitized entries; the API never exposes raw provider responses.
- The package has no pre-existing focused HTTP test file, and repository rules prohibit adding an ad hoc test file outside the approved test layout; verification therefore relies on focused package compilation/tests and source-level contract checks.
- The existing working tree contains unrelated user changes that must remain preserved.

## Exact recommended next step

Restart `infra-server` and perform an authenticated Task Center service smoke test against the list, create, action, and read endpoints using the configured service Bearer token.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
