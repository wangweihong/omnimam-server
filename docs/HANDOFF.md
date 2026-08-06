# Project Handoff

## Current goal and status

- Goal: replace raw contract string literals in Agent service with domain- and lifecycle-specific constants defined in `backend/apis/iapiserver`.
- Status: complete; implementation, focused audit, formatting, and package verification passed.
- SSOT: released `spec-v1.17.2` at `5990e6054ec8b79a342ef6e979b6522b855378aa`, matching `SSOT_VERSION` and the `ssot` submodule.

## Work completed in this session

- Read `skills/omnimam-server-backend/SKILL.md`, `backend/AGENTS.md`, and the Go naming/code-style guidance.
- Confirmed this is a behavior-preserving internal contract-constant refactor.
- Confirmed the SSOT release gate passes.
- Added Agent-owned constants grouped by Profile, Agent, Session, Workspace Binding, Model Binding, Invocation, Runtime Binding, Runtime Action, and Task integration lifecycles.
- Replaced raw contract values throughout Agent service without sharing constants across lifecycles merely because their literals match.
- Confirmed the Agent service has no remaining raw all-uppercase status, mode, type, role, or action literals.

## Current in-progress work

- None.

## Files changed

- Modified: `docs/HANDOFF.md` (live checkpoint).
- Added: `backend/apis/iapiserver/meta_agent_contract.go`.
- Modified: `backend/internal/apiserver/service/v1/agent/service.go`.
- Pre-existing user change: `backend/internal/apiserver/service/v1/appstudio/service.go`; do not overwrite it.

## Key decisions

- Preserve every existing wire/storage value exactly; this task changes ownership and references only.
- Agent constants must be independently named for their Agent subdomain and lifecycle even when another domain uses the same literal.
- Do not modify `ssot/`, APIs, schemas, migrations, errors, permissions, events, dependencies, configuration, or binaries.

## API, schema, dependency, and configuration changes

- No API shape, schema, dependency, or configuration changes.
- Existing serialized and persisted contract values are unchanged.

## Verification performed

- `git submodule status ssot` matches `SSOT_VERSION.commit` and released contract version `spec-v1.17.2`.
- `gofmt` completed for `backend/apis/iapiserver/meta_agent_contract.go` and `backend/internal/apiserver/service/v1/agent/service.go`.
- From `backend/`, `go test ./internal/apiserver/service/v1/agent ./apis/iapiserver` passed; Agent service has no test files and the API package tests passed.
- `git diff --check` passed.
- Focused `rg` audit found no remaining raw all-uppercase contract literals in Agent service.

## Outstanding tasks

- None for this task.

## Known issues and risks

- Some identical literals may represent different Agent lifecycle concepts; they must not be collapsed into a shared constant solely by value.
- Existing unrelated AppStudio changes must remain untouched.
- Agent service currently has no direct `_test.go`; verification there is compile-only.

## Exact recommended next step

Review and commit the Agent contract-constant refactor while preserving the unrelated AppStudio working-tree change.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
