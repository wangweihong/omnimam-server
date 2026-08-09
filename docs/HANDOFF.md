# Handoff

## Current goal and status

- Goal: verify and minimally fix the reported `workflowcanvas` defects against the released SSOT.
- Status: implementation complete for the confirmed local defects; focused verification passes.
- SSOT: released `spec-v1.19.0`, commit `aa3f843e6ad4987f0441d882bfa0d05e02e05065`; `SSOT_VERSION` matches the `ssot` submodule.

## Work completed in this session

- Added explicit `retry_failed` handling. It loads every source NodeRun page, selects failed nodes plus contiguous downstream `SKIPPED` nodes caused by those failures, and preserves CanvasVersion graph order.
- Removed the redundant ignored `sliceutil.Append` call while collecting DAGTaskGroup IDs.
- Replaced unsafe-config substring matching with separator/camel-case token matching. Forbidden fields such as `auth_token`, `callbackURL`, and `authorizationHeader` remain blocked while `author`, `curl_mode`, and `duration` are allowed.
- Made `resolveDefinitions` reject a nil Store result immediately even when the Store returns no error.
- Added regression coverage in the existing target-package test file, including paginated NodeRun loading.
- Confirmed the reported `definitionName` collision is not reachable because Canvas IDs are server-generated UUIDs and UUID hyphen positions are fixed.
- Confirmed `CancelRun` mutates only the request-local loaded model; an update failure does not alter persisted state or shared cache state.
- Confirmed `schemaStrings` is used by `application_catalog.go` and is not dead code.

## Current in-progress work

- None.

## Files modified

- `backend/internal/apiserver/service/v1/workflowcanvas/workflow_canvas.go`
- `backend/internal/apiserver/service/v1/workflowcanvas/workflow_canvas_test.go`
- `docs/HANDOFF.md`

## Key decisions and contract impact

- No API, schema, dependency, migration, permission, event, or environment-variable changes were made.
- `retry_failed` uses `only_nodes` with the failed and failure-caused skipped node IDs; it retains the source CanvasVersion, input snapshot, run policy, and retry linkage.
- Unsafe config checks now match field-name tokens rather than arbitrary substrings.
- The changes stay within the existing workflow-canvas service and test boundaries.

## Verification

Passed after the final implementation and handoff refresh:

```text
go test -count=1 ./backend/internal/apiserver/service/v1/workflowcanvas
go vet ./backend/internal/apiserver/service/v1/workflowcanvas
git diff --check -- backend/internal/apiserver/service/v1/workflowcanvas/workflow_canvas.go backend/internal/apiserver/service/v1/workflowcanvas/workflow_canvas_test.go docs/HANDOFF.md
```

No full-repository tests were run, per task constraints.

## Known issues and risks

- The existing execution compiler does not implement the complete SSOT output-reuse contract: `buildExecutionPlan` rejects `reuse_required`, `ReuseDecisions` remains empty, and `dagRequest` does not bind omitted successful predecessor outputs into a partial-run DAG. The new `retry_failed` target selection fixes the reported scope fallthrough, but data-dependent partial retries still rely on this missing broader capability.
- Implementing reuse requires execution-fingerprint validation, source terminal/output completeness checks, Artifact availability and authorization checks, TTL and side-effect policy enforcement, REUSED NodeRun/output binding persistence, and DAG input binding. It was not expanded into this focused defect fix.

## Outstanding tasks

- Implement the released SSOT result-reuse contract before treating data-dependent `retry_failed`, `retry_node`, or other partial scopes as end-to-end complete.

## Exact recommended next step

Design and implement reuse preflight in `backend/internal/apiserver/service/v1/workflowcanvas/workflow_canvas.go`, starting at `buildExecutionPlan` and `createRun`, with focused tests for successful predecessor output binding and `reuse_required` rejection/acceptance.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
