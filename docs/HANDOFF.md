# Project Handoff

## Current goal and status

- Goal: diagnose and fix backend/runtime issues A-003, A-005, A-006, A-007, and A-011 from `/home/wwhvw/codespace/omnimam-web/docs/agent-appstudio-real-user-test-2026-08-04.md`.
- Status: focused fixes are implemented, deployed, and verified. Remaining Agent Coding/Hermes execution, browser endpoint, and deletion semantics are explicit contract/runtime blockers documented below.

## Work completed in this session

- Read the repository and backend implementation rules plus the required backend skill.
- Confirmed `ssot` and `SSOT_VERSION` both reference released `spec-v1.16.1` commit `63defea97f45761acf300030cad1c00d6b83bb5a`.
- Read only the relevant report sections for A-003, A-005, A-006, A-007, A-011, and the Hermes permission evidence.
- Identified the first concrete environment risk: Hermes runs as UID/GID `10000`, while `/opt/data/kanban.db.init.lock` is owned by UID/GID `1000` with mode `0644`.
- Repaired the active standalone `hermes` container data mount with `chown -R 10000:10000 /opt/data` and `chmod -R u+rwX /opt/data`; the directory, database, and lock file now belong to `10000:10000`, with no new immediate `PermissionError` observed.
- Confirmed Agent Runtime task failures report `agent runtime ensure returned an invalid ready endpoint` after three attempts.
- Confirmed AppStudio Preview task failures report `appstudio preview ensure returned an invalid ready runtime` after three attempts.
- Confirmed CHAT Invocation `545be5fb-ae4b-4d13-8759-410e5a3f6767` had no AtomicTask or execution completion path and remained `RUNNING` until manually canceled.
- Confirmed AppStudio keyword search currently uses PostgreSQL case-sensitive `LIKE` over `name` and `description`.
- Confirmed Agent `1dca9028-637d-4644-99ca-4daee44f0801` remains `DELETING`; deletion can skip cleanup task submission when the current Runtime binding is already terminal, while no final deletion transition is present.
- Fixed the direct A-003/A-007 validation defect: Infra stores endpoint references as `infra-endpoint://<id>`, while Task Worker compared them to the bare endpoint ID and therefore rejected every otherwise-ready response.
- Changed AppStudio application keyword filtering to explicit PostgreSQL `ILIKE` over `name` and `description`, scoped only to AppStudio.
- Changed unsupported CHAT execution from indefinite `RUNNING` to an immediately persisted `FAILED` Invocation with `ERR_AGENT_INVOCATION_TASK_UNAVAILABLE` and a clear failure message; idempotent replays now return the existing Invocation without mutating its terminal state.
- Passed focused tests for Task Worker, AppStudio PostgreSQL store, and Agent service; built and deployed `apiserver` and `taskworker` image `3812662-amd64` with targeted container recreation.
- Verified deployed AppStudio filtering directly: lowercase `e2e` returns only the two mixed-case `E2E` projects.
- Verified a new AppStudio Preview reaches `RUNNING` with a canonical `infra-endpoint://...` reference, eliminating the previous invalid-ready-runtime failure.
- Found that the internal Task Worker to Infra Server JSON protocol drops `AuthorizationRef`, `EndpointVisibility`, `FunctionRef`, and `FunctionArguments` because those fields are intentionally hidden from the public API DTO with `json:"-"`; the resulting Preview endpoint incorrectly defaults to `INTERNAL` and has no browser URL.
- Added the internal `create_context` command object, rebuilt/redeployed `infraserver` and `taskworker`, and verified Preview `67440f2a-9ee8-4499-9f17-2c67beffbad7` reaches `RUNNING` with a `READY`, `USER_ACCESSIBLE` endpoint.
- Confirmed the generated Runtime containers use `nginx:1.27-alpine` and exit with code 1 because `ReadonlyRootfs=true` prevents nginx from creating `/var/cache/nginx/client_temp`.
- Confirmed two AppStudio Coding Agents (`agent.coding`) exist in `READY` but have no `agent_runtime_bindings`; AppStudio initialization creates only Agent metadata and never submits a Coding Runtime ensure task. The public runtime start path also intentionally hides non-platform Agents.
- Docker service Runtime creation now inspects the container immediately after start and returns a wrapped failure when it has already exited; the failed container is deleted instead of persisting a false `RUNNING` result. Rebuilt and recreated `omnimam-infraserver` with image tag `3812662-amd64`.
- Updated `agent.coding@1.0` to `ghcr.io/anomalyco/opencode:1.18.13` in `scripts/install/environment.sh` and the self-contained Compose fallback, pulled the image (digest `sha256:246ebd75d25380cde481b7507d38ed01ce813d541eb4c7412a25ba7d9bd1abe7`), and recreated a healthy `omnimam-infraserver` with the new mapping.
- Updated `agent.hermes@1.0` to `nousresearch/hermes-agent:v2026.8.3` in both deployment default sources, pulled digest `sha256:16788311e2fa3035456bdc1bafb8ec2b1777db64ebf020af9bb7eb73c3712c9e`, and recreated a healthy `omnimam-infraserver` with both pinned Agent profile mappings.

## Current in-progress work

- No implementation is currently in progress.
- Profile-specific Agent Runtime lifecycle work requires explicit authorization and released-SSOT-compatible mounts, endpoint routing, and Invocation execution contracts.

## Files added, modified, renamed, or removed

- Modified `docs/HANDOFF.md` for this live checkpoint.
- Modified `backend/internal/taskworker/taskworker.go` and its existing test for canonical Infra endpoint references.
- Modified `backend/internal/apiserver/store/postgresql/appstudio.go` and existing `pagination_test.go` for case-insensitive AppStudio keyword filtering.
- Modified `backend/internal/apiserver/service/v1/agent/service.go` so unavailable CHAT execution reaches a failed terminal state.
- Modified `backend/internal/infrastructure/protocol.go`, `client.go`, and `server.go` so service-internal create context survives JSON transport without exposing the fields on the public API DTO.
- Preserve all pre-existing user changes in the dirty working tree, including prior AppStudio, Agent, Identity, SSOT pin, and gotoolbox edits.

## Key architectural or design decisions

- The active Hermes storage ownership problem is fixed, but Runtime/Preview have separate, explicit invalid-ready-output failures that require orchestration/configuration fixes.
- Runtime failure must propagate to dependent Invocation and deletion workflows using existing SSOT-defined terminal states and errors; no new states or error codes will be invented.
- AppStudio keyword filtering will be fixed at the existing query boundary without changing the public API.
- Task Worker must validate the canonical `infra-endpoint://<id>` reference produced by Infrastructure; the endpoint readiness requirement itself remains strict.
- `ERR_AGENT_INVOCATION_TASK_UNAVAILABLE` is reused for fail-fast CHAT terminalization because no released Agent Invocation execution functionRef or adapter contract exists.
- Public Infrastructure API fields remain unchanged; service-only authorization, endpoint visibility, and function metadata travel in a separate internal command context.

## API, schema, dependency, or configuration changes

- Active container state only: `/opt/data` ownership changed to UID/GID `10000`; no repository deployment configuration has changed yet.
- No public API, schema, dependency, or environment-variable changes were made. The internal Infrastructure command wire format gains an optional `create_context` object.
- Any deployment environment variable change must originate in `scripts/install/environment.sh`; no `.env` or Compose `env_file` will be introduced.

## Verification performed and remaining checks

- Verified the SSOT release pin, exact reported reproduction evidence, active Hermes ownership repair, focused tests, image builds, targeted deployment, AppStudio `ILIKE` behavior, internal create-context transport, and Preview transition to `RUNNING` with `USER_ACCESSIBLE` visibility.
- Final verification passed: `go test ./internal/infrastructure ./internal/taskworker ./internal/apiserver/store/postgresql ./internal/apiserver/service/v1/agent`, `bash -n scripts/install/environment.sh`, `docker compose -f deployments/docker-compose.yaml config`, and `git diff --check`.
- A browser HTTP(S) endpoint and functional Agent runtime cannot be verified with the current Docker profile implementation.

## Outstanding tasks

- Make Hermes `/opt/data` ownership safe in repository-managed deployment if the relevant existing deployment path is confirmed.
- Preserve user-accessible endpoint visibility across the internal Infrastructure command transport.
- Implement a real endpoint allocation/display URL and profile-specific Docker runtime behavior only after confirming the released runtime contract and deployment design; the current provider returns only `infra-runtime://...`.
- Add an SSOT-approved AppStudio Coding Runtime lifecycle and OpenCode-compatible runtime adapter; current AppStudio creation stops after creating Coding Agent metadata.
- Ensure Agent deletion cannot remain indefinitely in `DELETING`; use existing retry/failure semantics.
- Recheck legacy Agent `1dca9028-637d-4644-99ca-4daee44f0801` without deleting unrelated data.

## Known issues and risks

- The working tree is dirty; overlapping user edits must be preserved.
- Runtime and Preview no longer fail on canonical endpoint validation, but the current Docker provider still does not create a functional Hermes runtime or browser-accessible Preview endpoint.
- `agent.hermes@1.0` now maps to `nousresearch/hermes-agent:v2026.8.3`, but the Docker provider still supplies no persistent `/opt/data` bind mount or Hermes port publication.
- `agent.coding@1.0` now maps to `ghcr.io/anomalyco/opencode:1.18.13`; no Coding Runtime is currently requested, and the Docker provider still has no profile-specific command, workspace mount, endpoint publication, or Invocation adapter.
- Infrastructure persists Runtime status `RUNNING` immediately after Docker start, while the actual containers can already be `Exited (1)`; no background provider reconciliation corrects the stale database state.
- The active standalone Hermes image is `nousresearch/hermes-agent:latest`, exposes ports `8642` and `9119`, and bind-mounts `/home/wwhvw/.hermes` to `/opt/data`; the repository Docker provider currently has no profile-specific command, port publishing, persistent mount, or source resolver, so changing only the image would not constitute a working Agent Runtime.
- A-011 cannot currently be finalized cleanly: Agent S1 permits deleting or soft-deleting the Agent, but released S2 has no Agent soft-delete field and foreign keys retain Session/Message/Invocation facts. Hard deletion would require deleting historical facts; adding a field is prohibited without SSOT. Do not choose either behavior without an upstream contract correction.
- `ssot/domains/agent/context.md` contains stale unreleased navigation text, while `RELEASE.md`, `GLOBAL_CONTEXT.md`, and `CONTEXT_MAP.md` confirm Agent S1/S2 release in `spec-v1.16.0`; do not modify `ssot/` here.
- The test environment at port `9990` may be running an older image than the current source tree.

## Exact recommended next step

Define and release the Agent Coding/Hermes runtime execution contract, then implement profile-specific mounts, ports/endpoints, lifecycle reconciliation, and Invocation adapters without repeating the completed fixes above.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
