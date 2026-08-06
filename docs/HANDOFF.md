# Project Handoff

## Current goal and status

- Goal: replace AppStudio contract magic strings with AppStudio-specific constants defined in `backend/apis/iapiserver/`, without changing serialized values or behavior.
- Status: complete; AppStudio contract literals are centralized and weakly typed task/event payloads are modeled with `iapiserver` structs.
- SSOT: `spec-v1.17.2` at `5990e6054ec8b79a342ef6e979b6522b855378aa`, matching `SSOT_VERSION` and the `ssot` submodule.

## Work completed in this session

- Confirmed the previous refactor commit is `4efb066`; the constants refactor is committed as `fd2fd6b`.
- Audited the Task Worker implementation for remaining stable function refs, map keys, enum values, URI prefixes, and artifact metadata literals that should use constants.
- Added the Task Worker constant set in `backend/apis/iapiserver/request_task_worker.go` and replaced the audited values in the parent, Agent, and AppStudio executors.
- Added Infrastructure operation, visibility, mount/binding, reference, profile, provider, protocol, digest, log, and event constants in `backend/apis/iapiserver/request_infrastructure.go`.
- Replaced the remaining Infrastructure literals in the Service, command dispatcher, Docker provider, Client, and request validation code.
- Added `backend/internal/taskworker/consumer/` and moved canvas application, application-run terminal projection, asset-library, representation orchestration, and thumbnail consumers into domain-focused files.
- Updated Task Worker startup wiring and existing tests to call the exported consumer entry points while preserving subscription order, topics, consumer groups, and Ack/Nack behavior.
- Moved published application catalog reconciliation and representation DAG construction with their consumers.
- Added Task Worker-specific payload key constants for `project_id`, `namespace`, `requested_representations`, and `idempotency_key`; these do not reuse Infrastructure constants.
- Reduced `backend/internal/taskworker/taskworker.go` from 1026 to 646 lines.
- Added AppStudio-specific status, operation, environment, functionRef, profile, reference-prefix, aggregate, and event constants in `backend/apis/iapiserver/request_appstudio.go`.
- Replaced AppStudio service and PostgreSQL projection magic strings with the new constants without changing serialized values.
- Added typed Build, Preview, Production, Stop, projection, output, and outbox event payload structures under `iapiserver`; no `AppStudioKey*` map-key constants remain.
- Converted typed task arguments only at the existing `AtomicTaskCreateRequest.Arguments` compatibility boundary and decode Task arguments/output into typed structs before projection.

## Previous work completed

- Moved Agent runtime execution to `backend/internal/taskworker/agentexecutor/`.
- Moved AppStudio preview, build, and production execution to `backend/internal/taskworker/appstudioexecutor/`.
- Moved ComfyUI Worker handler registration to `backend/internal/taskworker/comfyuiexecutor/`.
- Added `backend/internal/taskworker/contracts/` with the shared Infrastructure command/build executor interfaces and response validation helpers.
- Kept parent-package compatibility wrappers for existing unexported test entry points.
- Added `registerWorkerHandler` and `registerAtomicTaskHandler`; all Worker registrations now use the appropriate helper, including application-platform and thumbnail handlers.
- Unified registration and AtomicTask loading error context, and corrected the ComfyUI dependency error string to lowercase.

## Current in-progress work

- None.

## Files changed

- Modified: `backend/apis/iapiserver/request_appstudio.go`
- Modified: `backend/apis/iapiserver/response_appstudio.go`
- Modified: `backend/internal/apiserver/service/v1/appstudio/service.go`
- Modified: `backend/internal/apiserver/store/postgresql/appstudio.go`
- Modified: `backend/internal/apiserver/store/postgresql/outbox_test.go`
- Modified: `backend/internal/taskworker/appstudioexecutor/executor.go`
- Modified: `docs/HANDOFF.md`

## Key decisions

- Preserve serialized values, function references, response shapes, error codes, and business behavior.
- Keep Infrastructure executor interfaces in a separate internal contracts package so Agent and AppStudio do not share runtime-specific constants or implementation code.
- Keep Infrastructure constants under the `Infra*` namespace and Task Worker constants under the `TaskWorker*` namespace, even when serialized values are equal.
- Use one `consumer` subpackage with domain-focused files and no parent-package compatibility wrappers; existing tests call the exported consumer entry points.
- Keep AppStudio constants under the `AppStudio*` namespace rather than reusing Infrastructure or Task Worker constants with equal serialized values.
- Model AppStudio task and outbox payloads as typed structs; do not introduce map-key constants as a substitute for parameter types.
- AppStudio task argument structs expose explicit `AtomicTaskArguments` mappings for the existing Task Center map contract; do not use generic `any` or JSON round-trip conversion helpers.
- AppStudio worker and terminal projection decode task maps directly with typed generic mapstructure helpers; JSON is no longer used as a map-to-struct conversion mechanism.
- Do not add new `_test.go` files outside `pkg/`; the existing `taskworker_test.go` remains the focused test seam required by repository rules.

## API, schema, dependency, and configuration changes

- None. This is a behavior-preserving internal refactor.
- No files under `ssot/` were modified.
- No new binary or dependency was added.

## Verification performed

- Previous refactor verification: `go test ./internal/taskworker/...` passed; `gofmt` and `git diff --check` passed.
- Current constants implementation: `go test ./internal/infrastructure/... ./internal/taskworker/...` passed from `backend/`.
- Current Go files are formatted (`gofmt -l` produced no output) and `git diff --check` passed.
- Consumer extraction files were formatted with `gofmt`; `git diff --check` passed.
- `go test ./internal/taskworker/...` compiled `consumer`, `agentexecutor`, `appstudioexecutor`, `comfyuiexecutor`, and `contracts`, but the parent package test setup failed before compilation because `third_party/gotoolbox/pkg/timeutil/featival.go` imports `github.com/Lofanmi/chinese-calendar-golang/calendar` without a matching `go.sum` entry.
- AppStudio constant and payload refactor: `go test ./apis/iapiserver ./internal/apiserver/controller/v1/appstudio ./internal/apiserver/service/v1/appstudio ./internal/apiserver/store/postgresql ./internal/taskworker/appstudioexecutor` passed from `backend/`.

## Outstanding tasks

- None.

## Known issues and risks

- JSON struct tags, binding tags, import paths, HTTP routes, natural-language errors/logs, and fixture IDs remain literals because they are not status/mode contracts or cannot reference Go constants in tags.
- `AtomicTaskCreateRequest.Arguments` remains `map[string]any`; AppStudio builds typed argument structs and uses their explicit mappings only at this existing compatibility boundary.

## Exact recommended next step

Verify the committed AppStudio constants and typed payload refactor against any subsequent review feedback.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
