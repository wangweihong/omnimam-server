# Handoff

## Current goal and status

- Goal: publish `spec-v1.23.4`, version `agent.runtime.ensure` Runtime Git access, expose sanitized Attempt failure reasons, deploy the two affected services, and explicitly retry Application `27287fac-38f8-53fe-899d-6bfbbd66e977` once.
- Status: partially accepted and blocked by a new independent environment failure. The original Schema failure is fixed end to end, the four-stage initialization DAG succeeded, and the Application is `READY`; Runtime Provider then failed while initializing the Coding Git workspace, so the Coding Invocation finished `FAILED` instead of entering a usable runtime.

## Work completed in this session

- Verified upstream branch `codex/spec-v1.23.4` and annotated tag `spec-v1.23.4`; the tag peels to release commit `e3e00349604d9700caba40c3c7f68ecf5cbc22ab` and the post-release handoff commit is `8bc0aa14c800a9b1df208e04539b779f008b0aa6`.
- Verified Server `ssot` gitlink and `SSOT_VERSION` pin released commit `e3e00349604d9700caba40c3c7f68ecf5cbc22ab`.
- Verified local-only Server implementation commit `ebfd315` contains the registry pin/generation, Runtime contract selection coverage, and sanitized failure logging change. It has not been pushed.
- Regenerated the Task Function Registry with `go run ./backend/internal/taskfunctionregistry/internal/generate -root .`; embedded registry, schema, retryability, and `SOURCE` remained clean.
- Passed `go test ./backend/internal/taskfunctionregistry ./backend/internal/apiserver/workflowruntime ./backend/internal/taskworker`.
- Passed `make build BINS='apiserver taskworker'` for linux/amd64.
- Built `omnimam/apiserver:ebfd315-amd64` and `omnimam/taskworker:ebfd315-amd64` through `make image IMAGES='apiserver taskworker'`.
- Force-recreated only the `apiserver` and `taskworker` Compose services with `--no-deps`; no database, GitLab, Conductor, other persistent service, or frontend reset/restart was performed.
- Submitted exactly one retry using idempotency key `spec-v1.23.4-runtime-contract-fix`; new DAG is `1dcb317b-e4e4-5f24-934f-339024c89242`.
- Verified all four initialization stages succeeded on Attempt 1 and Application `27287fac-38f8-53fe-899d-6bfbbd66e977` became `READY`.
- Verified the retry reused the pre-existing Application, Project/Repository, Workspace, Agent, Session, Message, and Invocation IDs; all seven resources retain their original `2026-08-12 14:30:46-48Z` creation timestamps.
- Verified new Runtime AtomicTask `f6696c5d-1bd5-45bd-a1de-047de6d00997` pins `agent.runtime.ensure@1.1` with digest `sha256:48bb42c3793a3a4e3138d4b2108756addca8c751423d445f3264343b0a8d2d47`; its opaque `runtime_git_access_ref` exists and matches the required prefix.
- Verified the Runtime task received Conductor execution `b9c1b7b3-bd7a-4144-a3a9-e48a9ca68e67` and runtime task `92b6e401-1fe2-4970-8856-3b4cd6484d98`, proving it passed Schema validation and entered the Infra command path.
- Verified the deployed lifecycle log includes the concrete, single-line sanitized reason without URL or credential material: `initialize coding Git workspace` under a Runtime Provider operation failure.

## Current in-progress work

- None. Per task scope, no cross-module fix or second retry is authorized for the new environment failure.

## Files added, modified, renamed, or removed

- Modified in local implementation commit `ebfd315`: `SSOT_VERSION`, `ssot` gitlink, `backend/internal/taskfunctionregistry/assets/SOURCE`, `backend/internal/taskfunctionregistry/assets/function-registry.yaml`, `backend/internal/apiserver/workflowruntime/conductor.go`, `backend/internal/apiserver/workflowruntime/logs.go`, `backend/internal/apiserver/workflowruntime/logs_test.go`, `backend/internal/taskworker/taskworker_test.go`, and `docs/HANDOFF.md`.
- Modified after deployment verification: `docs/HANDOFF.md` only.
- Existing unrelated untracked files under `docs/` remain untouched.

## Key architectural or design decisions

- `agent.runtime.ensure@1.0` remains RETAINED with its original digest for historical tasks; only newly created Coding Runtime tasks select ACTIVE `1.1`.
- Runtime Git access remains an opaque secret reference. No Git URL, token, or credential is stored in task arguments beyond the opaque ref or emitted in lifecycle logs.
- Attempt failure logging remains best-effort and does not alter the task result.
- The repository `make compose` target performs a full-stack `down/up`; to honor the stricter requirement to rebuild only two services, deployment used its Make image pipeline plus the same Compose definition with `--force-recreate --no-deps` for `apiserver` and `taskworker`.

## API, schema, dependency, or configuration changes

- Spec function contract added ACTIVE `agent.runtime.ensure@1.1`; no public HTTP API, database schema/migration, error code, permission code, event type, dependency, environment variable, or new binary was added.

## Verification performed and remaining checks

- Passed: Spec release/tag/remote verification, registry generation/diff verification, requested package tests, requested binary builds, two amd64 image builds, service startup/health, registry-backed task creation, resource reuse, and four-stage initialization DAG.
- Passed: original `runtime_git_access_ref not allowed` failure is absent; the new AtomicTask validates against `1.1` and reaches Infra.
- Passed: new Attempt failure log contains the sanitized concrete reason and does not expose a token, Authorization value, URL, newline, or oversized body.
- Blocked: Runtime Provider failed all three runtime task attempts during Coding Git workspace initialization. The AtomicTask is `FAILED`, Agent runtime binding is `FAILED/UNHEALTHY`, and Invocation `553cbdae-46e9-53ed-8197-1131f5f81ea1` is `FAILED` with `ERR_AGENT_INVOCATION_TASK_UNAVAILABLE`.
- Remaining after the environment issue is corrected: authorize and perform a new explicit retry with a new idempotency key, then verify a non-empty Infra Runtime ID, healthy runtime binding, successful Invocation, and usable Coding Runtime.

## Outstanding tasks

- Diagnose the independent Runtime Provider environment failure reported as `initialize coding Git workspace`; keep this separate from the completed contract/logging fix.
- Do not retry again until the environment cause is corrected and a new explicit retry is authorized.

## Known issues and risks

- Application is `READY` because its four-stage initialization projection records successful submission, while the asynchronously executed first Coding Invocation later failed. Treat `READY` alone as insufficient proof of a usable Coding Runtime.
- Historical five Attempt logs were not backfilled; the new logging behavior applies only to post-deployment Attempts.
- The local Server branch is not pushed and includes pre-existing unrelated untracked documents that must remain excluded from commits.

## Exact recommended next step

Inspect the Infra Runtime Provider path for AtomicTask `f6696c5d-1bd5-45bd-a1de-047de6d00997` and determine why Coding Git workspace initialization fails. Record the environment correction separately; after explicit authorization, submit one new initialization retry with a new idempotency key and verify the runtime binding and Invocation reach success.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
