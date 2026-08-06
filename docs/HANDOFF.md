# Project Handoff

## Current goal and status

- Goal: strengthen the backend rule for status, mode, type, and action contract literals.
- Status: complete; `backend/AGENTS.md` now requires domain-specific constants in `backend/apis/iapiserver` and forbids cross-domain reuse based only on equal literal values.
- SSOT: released `spec-v1.17.2` at `5990e6054ec8b79a342ef6e979b6522b855378aa`, matching `SSOT_VERSION` and the `ssot` submodule.

## Work completed in this session

- Strengthened the existing `backend/AGENTS.md` constants rule instead of adding a duplicate rule.
- Required contract constants to live in `backend/apis/iapiserver` and remain domain/lifecycle specific.
- Read `skills/omnimam-server-backend/SKILL.md`, `backend/AGENTS.md`, and the focused Go database/debugging/error-handling guidance.
- Located the AppStudio domain through `ssot/GLOBAL_CONTEXT.md`, `ssot/CONTEXT_MAP.md`, and `ssot/domains/appstudio/context.md`.
- Confirmed `CreateApplication` writes the empty source revision, commits application/repository/workspace/revision plus outbox records, then invokes `CreateCodingAgentForStudio`.
- Confirmed the nil-Agent and Agent-error branches only attempt to set the application status to `ERROR`; they do not remove the committed aggregate.
- Confirmed `CreateStudioApplicationAggregate` is internally transactional only for the AppStudio database records and its outbox writes.

## Current in-progress work

- Constants-rule task is complete; the pre-existing AppStudio partial-aggregate diagnosis remains paused at the SSOT and Agent creation-path verification step.

## Files changed

- Modified: `backend/AGENTS.md`.
- Modified: `docs/HANDOFF.md` (live diagnostic checkpoint only).
- Pre-existing user change: `backend/internal/apiserver/service/v1/appstudio/service.go` contains an unrelated uncommitted `validateFileContent` helper; do not overwrite it.

## Key decisions

- This task is diagnosis only; do not modify runtime behavior unless the user explicitly asks for a fix.
- Treat AppStudio and Agent as separate domain persistence boundaries until the concrete Agent store path proves otherwise.
- Do not assume that wrapping a remote/service call in a GORM transaction provides atomicity across both domains.

## API, schema, dependency, and configuration changes

- None.
- No files under `ssot/` were modified; no dependency, migration, API, error code, permission, event type, or binary was added.

## Verification performed

- `git submodule status ssot` and `SSOT_VERSION` match the released `spec-v1.17.2` commit.
- Static source trace completed through `CreateApplication` and `CreateStudioApplicationAggregate`.
- No tests have been run yet; this is still an analysis task.

## Outstanding tasks

- Read only the S1/S2 sections directly referenced by the AppStudio context for application creation and initialization failure.
- Trace `CreateCodingAgentForStudio` to its direct store calls and determine whether any existing compensation or retry path makes the state recoverable.
- Inspect directly related existing tests and report severity, impact, and the technically valid fix boundary.

## Known issues and risks

- The `ERROR` status update error is discarded, so the application may remain `CREATING` if that compensating update also fails.
- The empty source revision is written before the database aggregate transaction; its cleanup behavior is not yet verified.
- A single database transaction may be impossible or inappropriate if AppStudio and Agent writes use independent service/store transaction ownership.

## Exact recommended next step

Commit `backend/AGENTS.md` and `docs/HANDOFF.md` without staging the unrelated AppStudio service change, then resume the outstanding AppStudio diagnosis.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
