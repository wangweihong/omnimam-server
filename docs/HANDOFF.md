# Project Handoff

## Current goal and status

The layered-context refactor of `skills/omnimam-server-backend/SKILL.md` is
complete and validated. It replaces unconditional full Domain
S1/S2/Architecture loading with:

```text
GLOBAL_CONTEXT -> CONTEXT_MAP -> Domain Context -> minimum released S1/S2
-> relevant backend/AGENTS.md sections -> current implementation
```

The Skill will remain the repository's sole backend workflow Skill. No backend
business code, SSOT content, submodule revision, API, schema, dependency,
configuration, event, permission, error code, or binary will change.

## Work completed in this session

- Read the complete change request, current backend Skill, repository rules,
  `backend/AGENTS.md`, and the `skill-creator` Skill.
- Confirmed the worktree and `ssot/` internal worktree were clean before edits.
- Confirmed `ssot/` is pinned to released `spec-v1.8.0` commit
  `44e659294c3fc8fa948345cc42faf5fcc05b5aa8`, matching `SSOT_VERSION.commit`.
- Confirmed the pinned release has no `GLOBAL_CONTEXT.md`, `CONTEXT_MAP.md`, or
  Domain `context.md` files.
- Chose strict missing-Context behavior: block formal product and contract
  implementation; allow only verified semantics-preserving documentation,
  tests, formatting, or internal refactors.
- Recorded this live checkpoint before the Skill rewrite.
- Rewrote `skills/omnimam-server-backend/SKILL.md` in place with standard YAML
  frontmatter, layered Context loading, minimum-context and caching rules,
  backend task matrices, cross-Domain limits, and before/after checklists.
- Preserved the released S1/S2 authority, submodule pin, `SSOT_VERSION`, API,
  schema, migration, errors, permissions, events, module boundary, and
  provider/runtime restrictions without creating a second Skill.
- Made the implementation checklist conditional so verified semantics-
  preserving tasks can use the documented missing-Context exception without
  weakening the formal implementation gate.
- Completed structural, content, changed-file scope, whitespace, release, and
  submodule integrity validation.

## Current in-progress work

None. The documentation-only Skill refactor is ready for review.

## Files added, modified, renamed, or removed

- Modified: `docs/HANDOFF.md`
- Modified: `skills/omnimam-server-backend/SKILL.md`
- Added, renamed, or removed files: none

## Key architectural and design decisions

- Context files navigate to authoritative sources; they never replace released
  S1/S2.
- Domain Context paths must come from `CONTEXT_MAP.md`; the Skill will not
  hard-code a mutable Domain catalog.
- Missing required Context files never trigger silent filename-based fallback.
- The existing Skill path and top-level `AGENTS.md` routing remain unchanged.

## API, schema, dependency, or configuration changes

None. This task changes workflow documentation only.

## Verification performed and remaining checks

Performed:

- Compared `SSOT_VERSION.commit` with the current submodule commit.
- Confirmed released tag `spec-v1.8.0` and a clean `ssot/` internal worktree.
- `quick_validate.py skills/omnimam-server-backend` passed with
  `Skill is valid!`.
- Required heading and rule assertions passed for layered loading, minimum
  context, task classification, task-specific loading, caching, cross-Domain
  rules, strict missing-Context handling, and both checklists.
- Negative assertions confirmed the obsolete unconditional full-load rule and
  hard-coded Domain catalog are absent.
- `git diff --check` passed, and the only changed files are this handoff and
  `skills/omnimam-server-backend/SKILL.md`.
- The pinned submodule contains no `GLOBAL_CONTEXT.md`, `CONTEXT_MAP.md`, or
  Domain `context.md`, matching the documented strict gate.
- No backend test or build was run because this task changes only Markdown and
  the Skill explicitly requires structural/path/scope validation for this case.

Remaining: none for this task.

## Outstanding tasks

- Review the backend Skill rewrite.
- Pin a future released SSOT revision containing the Context layer before
  resuming formal product or contract implementation under the new workflow.
- Review the existing local Notification Center commit and deploy matching
  `taskworker` and `notificationworker` images together when authorized.
- Resolve the unrelated thumbnail localization test mismatch separately.

## Known issues and risks

- The pinned `spec-v1.8.0` release lacks the new Context layer, so formal
  product and contract implementation will be blocked until a compatible
  released SSOT revision is pinned.
- Deployment must not mix an older `taskworker` that starts Notification Center
  consumers with the new standalone `notificationworker`.
- Full `go test ./backend/...` has an unrelated existing thumbnail localization
  mismatch: the catalog has `生成 thumbnail视图` while tests expect
  `生成 thumbnail 表现形式`.
- The older database `omnimam_notification_test_20260729` remains untouched
  because its ownership is unknown.

## Exact recommended next step

Review the Skill diff. Before the next formal backend feature, release and pin
an SSOT revision containing Global Context, Context Map, and Domain Contexts,
then update `SSOT_VERSION` to that exact commit.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
