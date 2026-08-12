# Handoff

## Current goal and status

- Goal: implement AppStudio phase 3 against released `spec-v1.23.2`: asynchronous initialization DAG, GitLab webhooks, push-to-build/artifact/preview orchestration, and the required Infrastructure/DevOps integration.
- Status: implementation complete for code/configuration scope; authorized local cleanup and full E2E acceptance remain pending.

## Work completed in this session

- Read the phase-3 design, repository rules, backend skill, relevant Go implementation skills, and current handoff state.
- Confirmed the current Server pin is released `spec-v1.23.2` commit `9e1bf2291dd1925e982a5dd728e05a27c334f8d9`.
- Locked implementation boundaries: HTTP 200 creation response, hashed per-project webhook secrets, canonical Revision ancestry, SourceArchive previews, existing Release/Artifact production authority, and precise domain-only cleanup.
- Released and pushed `spec-v1.23.2`; Server `ssot` and `SSOT_VERSION` now point to release commit `9e1bf2291dd1925e982a5dd728e05a27c334f8d9`.
- Added the phase-3 DTO fields, webhook error codes, and a trusted AppStudio-only `CreateDomainDAGTaskGroup` path while preserving the public DAG `gitlab.pipeline.run` rejection.
- Replaced synchronous application creation with deterministic CREATING reservation plus a stable four-node initialization DAG.
- Added idempotent initialization handlers, GitLab Project Hook client/adapter support, and Worker registrations. Hook tokens use `crypto/rand`; only `sha256:<hex>` is persisted.
- Added `appstudio.webhook-base-url` configuration and injected it into API Server and Task Worker GitLab adapters.
- Added unauthenticated `POST /api/v1/appstudio/webhook`, constant-time Project token authentication, stable Push DAG submission, Pipeline Hook projection, canonical Revision-gated Snapshot/Build handlers, fixed-commit Pipeline validation, constrained Bundle download/validation, Artifact completion, and automatic SourceArchive Preview submission.
- Fixed manual Snapshot/Build creation to populate the new immutable CommitSHA/GitRef fields.

## Current in-progress work

- No implementation work is in progress. Local domain-only cleanup and E2E acceptance remain.

## Files added, modified, renamed, or removed

- Modified: `ssot` gitlink, `SSOT_VERSION`, AppStudio/GitLab/Task Center APIs/services, Infrastructure provider, Compose/configuration, install environment, and `docs/HANDOFF.md`.
- Existing unrelated untracked design documents under `docs/` remain untouched.

## Key architectural or design decisions

- AppStudio submits trusted internal DAGs while the public Task Center DAG API continues to reject `gitlab.pipeline.run`.
- GitLab webhook plaintext tokens are generated with `crypto/rand`, sent once to GitLab, and never persisted or logged; only a SHA-256 digest is stored on the GitLab project projection.
- Push processing waits for the canonical AppStudio Revision projector before creating Snapshot/Build state.
- Preview writes remain Task Worker to Infrastructure operations; GitLab CI only builds a constrained Bundle artifact.

## API, schema, dependency, or configuration changes

- Server API structs contain the released phase-3 fields; deepcopy/error-code generation has been refreshed. GORM AutoMigrate will add the new columns, but destructive cleanup has not run.
- No dependency change has been made.

## Verification performed and remaining checks

- `make gen.deepcopy` and `make gen.errcode.code` passed.
- `go test ./backend/internal/apiserver/service/v1/appstudio`, `go test ./backend/internal/apiserver/service/v1/gitlab`, `go test ./backend/internal/taskworker`, and targeted Task Center DAG/functionRef tests passed.
- AppStudio tests now cover invalid token rejection, duplicate Push stable DAG identity, and canonical Revision race; GitLab tests cover digest-only token authentication and tar traversal rejection.
- Full Task Center package currently fails only `TestAssignSystemName` because an unrelated existing Chinese localization value differs; this task did not modify that module.
- Passed: targeted Go tests, DevOps YAML/shell checks, Compose config rendering, and diff checks. Not run: destructive cleanup, image build, and full E2E.

## Outstanding tasks

- Execute the approved local cleanup, restart Compose, and perform admin-authenticated E2E acceptance.

## Known issues and risks

- Task Center is shared; cleanup must select only AppStudio/GitLab-owned records and their Conductor executions.
- Existing unrelated untracked files must not be modified.

## Exact recommended next step

Inspect the local deployment, identify exact owner predicates, perform authorized domain-only cleanup, and record results before restart.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
