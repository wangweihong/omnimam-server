# Project Handoff

## Current goal and status

- Goal: replace Infrastructure and Task Worker business-contract magic strings with dedicated constants in the corresponding `iapiserver/request_*.go` files, without mixing the two constant sets.
- Status: implementation complete; scoped verification passed and the constants refactor was committed as `fd2fd6b`.
- SSOT: `spec-v1.17.2` at `5990e6054ec8b79a342ef6e979b6522b855378aa`, matching `SSOT_VERSION` and the `ssot` submodule.

## Work completed in this session

- Confirmed the previous refactor commit is `4efb066`; the constants refactor is committed as `fd2fd6b`.
- Audited the Task Worker implementation for remaining stable function refs, map keys, enum values, URI prefixes, and artifact metadata literals that should use constants.
- Added the Task Worker constant set in `backend/apis/iapiserver/request_task_worker.go` and replaced the audited values in the parent, Agent, and AppStudio executors.
- Added Infrastructure operation, visibility, mount/binding, reference, profile, provider, protocol, digest, log, and event constants in `backend/apis/iapiserver/request_infrastructure.go`.
- Replaced the remaining Infrastructure literals in the Service, command dispatcher, Docker provider, Client, and request validation code.

## Previous work completed

- Moved Agent runtime execution to `backend/internal/taskworker/agentexecutor/`.
- Moved AppStudio preview, build, and production execution to `backend/internal/taskworker/appstudioexecutor/`.
- Moved ComfyUI Worker handler registration to `backend/internal/taskworker/comfyuiexecutor/`.
- Added `backend/internal/taskworker/contracts/` with the shared Infrastructure command/build executor interfaces and response validation helpers.
- Kept parent-package compatibility wrappers for existing unexported test entry points.
- Added `registerWorkerHandler` and `registerAtomicTaskHandler`; all Worker registrations now use the appropriate helper, including application-platform and thumbnail handlers.
- Unified registration and AtomicTask loading error context, and corrected the ComfyUI dependency error string to lowercase.

## Current in-progress work

- No implementation work is in progress; post-commit worktree verification is complete.

## Files changed

- Modified: `backend/apis/iapiserver/request_task_worker.go`
- Modified: `backend/internal/taskworker/taskworker.go`
- Modified: `backend/internal/taskworker/agentexecutor/executor.go`
- Modified: `backend/internal/taskworker/appstudioexecutor/executor.go`
- Modified: `backend/internal/taskworker/comfyuiexecutor/executor.go`
- Modified: `backend/internal/taskworker/contracts/infrastructure.go`
- Modified: `backend/internal/taskworker/taskworker_test.go`
- Modified: `backend/internal/infrastructure/service.go`
- Modified: `backend/internal/infrastructure/server.go`
- Modified: `backend/internal/infrastructure/providers/dockerruntime/docker.go`
- Modified: `backend/internal/infrastructure/client.go`
- Modified: `backend/apis/iapiserver/request_infrastructure.go`
- Modified: `docs/HANDOFF.md`

Untracked and intentionally excluded from this task:
- `third_party/gotoolbox/pkg/timeutil/convert.go`

Previous refactor files:
- Added: `backend/internal/taskworker/contracts/infrastructure.go`

## Key decisions

- Preserve serialized values, function references, response shapes, error codes, and business behavior.
- Keep Infrastructure executor interfaces in a separate internal contracts package so Agent and AppStudio do not share runtime-specific constants or implementation code.
- Keep Infrastructure constants under the `Infra*` namespace and Task Worker constants under the `TaskWorker*` namespace, even when serialized values are equal.
- Keep compatibility wrappers in the parent package until existing tests can migrate without changing their call sites.
- Do not add new `_test.go` files outside `pkg/`; the existing `taskworker_test.go` remains the focused test seam required by repository rules.

## API, schema, dependency, and configuration changes

- None. This is a behavior-preserving internal refactor.
- No files under `ssot/` were modified.
- No new binary or dependency was added.

## Verification performed

- Previous refactor verification: `go test ./internal/taskworker/...` passed; `gofmt` and `git diff --check` passed.
- Current constants implementation: `go test ./internal/infrastructure/... ./internal/taskworker/...` passed from `backend/`.
- Current Go files are formatted (`gofmt -l` produced no output) and `git diff --check` passed.

## Outstanding tasks

- None for this refactor.

## Known issues and risks

- JSON struct tags, binding tags, import paths, HTTP routes, natural-language errors/logs, and fixture IDs remain literals because they are not status/mode contracts or cannot reference Go constants in tags.

## Exact recommended next step

Continue with the next requested task after verifying the committed state; leave the unrelated untracked file untouched.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
