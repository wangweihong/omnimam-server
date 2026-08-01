# Project Handoff

## Current goal and status

Replace direct Go ProviderCapability values with strict adapter-local const
YAML manifests, complete the registered providers' stable non-streaming
schemas, and enforce input/output validation consistently in API Server and
TaskWorker.

Status: implementation and scoped verification complete. `spec-v1.9.1` is
released and pinned at `5c66c724fa57e9107bfdea0244bc41beff9ed4ed`.
Provider/protocol/transport separation and registry ownership consolidation in
`modelgateway` are complete; runtime IDs and external behavior remain unchanged.

## Work completed in this session

- Revalidated the current `spec-v1.9.1` pin and loaded the backend/spec rules.
- Confirmed the independent spec checkout is dirty and selected the clean
  `/home/wwhvw/codespace/omnimam-spec-provider-static` worktree instead.
- Confirmed current gaps: permissive schemas for OpenAI/xAI/Google, no complete
  JSON Schema enforcement, and stale image packaging of the deleted directory.
- Released `spec-v1.9.1` from the isolated spec worktree and updated the server
  submodule/`SSOT_VERSION` pin.
- Added strict multi-document YAML parsing, Draft 2020-12 schema compilation,
  named validator resolution, and input/output validation methods.
- Migrated ComfyUI, DeepSeek, Google, ModelArk, Ollama, OpenAI, RunningHub, and
  xAI capability facts into one adapter-local `capability.go` per provider;
  DeepSeek now owns a distinct provider package while reusing wire helpers.
- Integrated strict input validation before API persistence and Worker calls,
  compatible output validation after values normalization, and error 130831.
- Closed schema fallback gaps found during verification: fixed catalog providers
  now require an exact operation/model variant, while dynamic Ollama operations
  retain operation-level fallback; successful provider execution must return a
  normalized `values` object before output schema validation.
- Added Ollama `/api/tags` discovery, transient RuntimeForm variants, and the
  `ollama.model-installed` execution-time validator.
- Removed all backend image packaging of an external provider-capabilities
  directory and updated the capability registry architecture guide.
- Separated implementation ownership into `adapters/providers`,
  `adapters/protocols`, and `adapters/transports`; DeepSeek/OpenAI provider
  packages now bind themselves to the neutral `openaicompat` protocol package,
  xAI reuses that protocol without importing the OpenAI provider, and generic
  authenticated JSON calls live in `transports/httpjson`.
- Added a package-ownership regression test for the OpenAI-compatible provider
  mappings and documented the one-way dependency rules.
- Moved RuntimeRegistry, ProviderCapabilityRegistry, manifest parsing, schema
  compilation/validation, and their tests into the `modelgateway` root package;
  removed the standalone `internal/apiserver/applicationplatform` package and
  renamed adapter aggregation to `adapters/bootstrap.go`.

## Current in-progress work

- None for provider capability, adapter package separation, or registry
  ownership. The worktree is ready for final review and commit.

## Files added, modified, renamed, or removed

- Modified: `SSOT_VERSION`, `ssot`, API metadata/error/generated targets,
  registry/bootstrap/provider registration code and tests, and handoff.
- Added: shared manifest/schema validation code and provider `capability.go`
  files; added the DeepSeek provider registration package.
- Moved: provider directories under
  `backend/internal/apiserver/service/v1/modelgateway/adapters/providers/`;
  shared OpenAI-compatible code to `adapters/protocols/openaicompat/`; generic
  HTTP JSON transport to `adapters/transports/httpjson/`.
- Moved: `internal/apiserver/applicationplatform/{registry,manifest,
  capability_validation}*` into
  `internal/apiserver/service/v1/modelgateway/`; no compatibility package or
  legacy empty asset directory remains.

## Key architectural or design decisions

- YAML is an immutable Go const owned by each provider adapter; no disk,
  directory, environment, database, import API, reload, or runtime override.
- YAML owns ProviderCapability facts only. EngineType, Adapter, Executor, and
  validator implementations remain explicit Go registrations.
- Inputs are strict; outputs validate required structure while allowing new
  upstream fields. Streaming remains unsupported.
- Package ownership is explicit: `adapters/providers/<provider>` owns provider
  facts and provider-specific behavior; `adapters/protocols/openaicompat` owns
  reusable OpenAI-compatible wire behavior; `adapters/transports/httpjson` owns
  generic authenticated JSON HTTP transport. Bootstrap consumes provider
  constructors and does not select a provider's wire protocol directly.
- Runtime registry, provider capability manifest parsing, and schema validation
  belong to the `modelgateway` root package; no standalone
  `internal/apiserver/applicationplatform` package remains after migration.

## API, schema, dependency, or configuration changes

- Released S2 adds operation-level input/output schemas and provider response
  validation error `130831`; no database migration is planned.

## Verification performed and remaining checks

- Passed current SSOT pin/release consistency and worktree isolation checks.
- Passed: manifest/registry, adapter, Ollama discovery/model validation, and
  ApplicationRun input/output execution tests.
- Re-ran focused application-platform, provider adapter, and TaskWorker package
  tests successfully; `git diff --check` also passed.
- Regenerated error-code and deepcopy artifacts successfully; generated diffs
  are limited to error 130831 and the new operation schema fields.
- Focused golangci-lint, `make build`, and `go vet ./backend/...` passed.
- Capability-focused `go test -race` passed for registry, application service,
  all provider adapters, and TaskWorker.
- Re-ran focused tests and race after the package separation for all provider,
  protocol, transport, Application Platform, API Server bootstrap, and
  TaskWorker packages; scoped golangci-lint, backend vet, and all three binary
  builds passed.
- Re-ran focused tests and race after consolidating Registry, manifest, and
  schema validation into `modelgateway`; modelgateway/adapters,
  Application Platform, API Server bootstrap, and TaskWorker all passed.
  Scoped golangci-lint, `go vet ./backend/...`, and all three binary builds also
  passed with the old package path and alias absent.
- API Server and TaskWorker images built successfully, and both images were
  verified not to contain `/opt/omnimam/provider-capabilities`.
- Rebuilt API Server and TaskWorker images after the package separation and
  repeated the image layout check. Existing compose containers were not
  replaced because this refactor does not change runtime behavior.
- Rebuilt and layout-checked both images again after Registry ownership
  consolidation; existing compose containers remain untouched.
- Live smoke against the already-running provider-validation deployment passed:
  `/healthz` returned OK and the capability endpoint returned all 9 registered
  capabilities with input/output schemas on every catalog operation. No
  capability bootstrap fatal/panic appeared in API Server or TaskWorker logs.
- Full `make test` runs all packages but currently fails only in pre-existing
  task-name localization expectations (`thumbnail视图` versus
  `thumbnail 表现形式`) in taskname/taskcenter; capability-focused packages
  pass in the same run.
- Final stale directory search, generated diff review, SSOT pin check,
  formatting check, and `git diff --check` passed.

## Outstanding tasks

- No provider capability, adapter package-separation, or Registry ownership
  task remains.
- Fix the unrelated repository-wide lint/test/vet baselines only under a
  separate scope.

## Known issues and risks

- The independent `/home/wwhvw/codespace/omnimam-spec` checkout has unrelated
  changes and must not be modified.
- `make verify` currently reaches lint but the repository-wide misspell linter
  also fails on pre-existing `cancelled` status literals in
  `backend/internal/apiserver/store/postgresql/asset_upload_contract.go`; that
  unrelated asset contract has not been changed in this task. RunningHub's
  upstream-compatible `cancelled` response spelling now has a focused,
  justified `nolint:misspell` directive.
- Repository-wide `go vet ./...` also reports the pre-existing
  `tools/deepcopy-gen/generators/deepcopy.go:120: append with no values`; scoped
  `go vet ./backend/...` passes.
- `make test` is blocked by pre-existing task-name localization failures in
  `backend/internal/apiserver/taskname` and the dependent task-center test;
  provider capability packages pass.
- Provider documentation breadth is large; every declared field must remain
  tied to an executable non-streaming path and a cited checked source.

## Exact recommended next step

Review `git diff --find-renames`, then stage and commit the provider capability,
adapter package separation, and modelgateway Registry consolidation without
including unrelated baseline fixes.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
