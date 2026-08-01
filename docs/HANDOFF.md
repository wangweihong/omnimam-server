# Project Handoff

## Current goal and status

Replace file/directory-loaded ProviderCapability manifests with immutable Go
registrations owned by concrete Model Gateway adapters. Add the released
OpenAI Responses/GPT Image 2, xAI Grok, Google Gemini/Nano Banana, Ollama, and
RunningHub protocol scope with bilingual metadata and distinct official
website, documentation, and API base URLs.

Status: implementation complete in the working tree. The server is pinned to
released `spec-v1.9.0` commit
`deae3f15d65b78f9e811f53f99149c1f74cab0c5`; its formal S1/S2 content commit is
the direct parent `7900509`.

## Work completed in this session

- Revalidated the SSOT release gate, Application Platform and Task Center
  Context/S1/S2, backend rules, and current worktree.
- Removed ProviderCapability YAML/schema/runtime-registry assets, directory
  configuration, environment/deployment wiring, load diagnostics, and
  loader-only errors and permissions.
- Added explicit adapter-owned static registrations for ComfyUI, ModelArk,
  DeepSeek, OpenAI Responses/Images, xAI, Google, Ollama, and RunningHub.
- Added bilingual provider/engine/model/operation metadata, official URLs,
  strict auth schemas, precise released model IDs, and atomic startup
  validation for IDs, references, URLs, auth, lifecycle, modes, and schemas.
- Added native protocol execution/health behavior, including Google
  `x-goog-api-key`, Ollama `/api/tags`, RunningHub body `apiKey`, bounded health
  probes, normalized provider model IDs, and safe error classification.
- Reworked RunningHub onto Conductor `IN_PROGRESS` callbacks: submit once,
  persist `external_job_id` in runtime output, poll once per callback, recover
  retried task output, and cancel CANCELED/TIMEOUT external jobs.
- Fixed the truncated `Vary: Accept-Encoding` controller header found during
  broad compilation.
- Updated generated deepcopy/error artifacts and the capability-registry
  architecture guide.

## Current in-progress work

None. Changes are not committed.

## Files added, modified, renamed, or removed

- Modified: `SSOT_VERSION`, `ssot`, `docs/HANDOFF.md`, Application Platform API
  types/generated deepcopy, registry/controller/service/bootstrap code, Model
  Gateway adapters and tests, WorkflowRuntime/TaskWorker recovery wiring,
  configs/deployment/install defaults, error code sources/generated docs, and
  `docs/guide/zh-CN/architecture/application-platform-capability-registry.md`.
- Added: adapter registration/implementation packages for Google, Ollama,
  RunningHub, and xAI; registration files for ComfyUI, ModelArk, and OpenAI.
- Removed: embedded/root ProviderCapability manifests, loader schema/runtime
  registry assets, and directory-only configuration surfaces.
- No binary was added under `backend/cmd/` and no file inside `ssot/` was
  modified.

## Key architectural and design decisions

- Static adapter registrations are explicitly assembled by API Server and
  TaskWorker bootstrap; invalid facts or missing implementations fail startup
  atomically. There is no partial/degraded registry or runtime override.
- OpenAI, xAI, and Ollama may reuse wire helpers but retain distinct EngineType,
  auth, metadata, and capability facts. Google and RunningHub use native
  adapters.
- ComfyUI and RunningHub use `engine_binding + static + required_immutable`;
  Ollama declares protocols without inventing a local model catalog.
- Runtime checkpoints remain Conductor output/TaskAttempt facts. The worker
  loads them lazily, including the retried runtime task fallback; application
  code does not read Task Center private tables or add a second state machine.
- ModelScope, Kling, and standalone Jimeng remain out of scope.

## API, schema, dependency, or configuration changes

- ProviderCapability remains read-only but now exposes released bilingual
  fields and static origin; load-result API/DTO/configuration was removed.
- ApplicationEngineType now exposes bilingual descriptions, official website,
  official documentation, default executable API base URL, auth types/schema,
  and adapter/executor mappings.
- Loader-only error codes `130220..130228` and `130231` were removed; released
  AtomicTask and Artifact mappings were regenerated.
- No database migration or new dependency was added.

## Verification performed and remaining checks

Passed:

- `make gen.deepcopy gen.errcode` (existing alias/`any` generator warnings only)
- Focused API/registry/Application Platform/Model Gateway/WorkflowRuntime/
  TaskWorker/NotificationWorker tests and all three backend binary builds
- RunningHub recovery/auth/cancel tests with `-count=10`
- `go test -race` for WorkflowRuntime, Application Platform executor, and
  Model Gateway adapters
- `go vet ./backend/...`
- `git diff --check`
- Every backend package except the two known thumbnail-localization packages

`go test ./backend/...` still fails only:

- `backend/internal/apiserver/service/v1/taskcenter: TestAssignSystemName`
- `backend/internal/apiserver/taskname: TestResolve/parameterized_system_name`

Both expect `生成 thumbnail 表现形式`; current shared localization returns
`生成 thumbnail视图`. This behavior predates and is unrelated to this task.

## Outstanding tasks

- Review and commit the current working-tree diff.
- Address the separate thumbnail-localization expectation mismatch in its own
  scoped task if a fully green repository test run is required.

## Known issues and risks

- RunningHub cannot persist or cancel a job accepted upstream if cancellation
  occurs before the create response yields `taskId`; recovery is durable as
  soon as that ID is returned, and the upstream protocol exposes no earlier
  local identifier.
- The independent `/home/wwhvw/codespace/omnimam-spec` checkout contains
  unrelated uncommitted work and was not modified.

## Exact recommended next step

Review `git diff`, then commit the ProviderCapability static-registry and
protocol-adapter implementation. Keep the thumbnail localization fix separate.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
