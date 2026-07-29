# Project Handoff

## Current goal and status

Complete the released `spec-v1.8.0` Notification Center server implementation and move all Notification Center background processing into a separately deployable `notificationworker`.

Implementation is complete, committed locally, and ready for review or deployment. The standalone binary, bootstrap, lightweight image, Compose service, and architecture/deployment documentation now exist. Source consumption, rule processing, retention, and the Notification Outbox SSE projector are started only by `notificationworker`; current `taskworker` paths contain no Notification Center wiring. Focused and race tests, both worker builds, real PostgreSQL integration, Compose validation, an actual lightweight image build, generation, and the SSOT/contract audit passed.

The `ssot/` submodule is pinned to released tag `spec-v1.8.0`, commit `44e659294c3fc8fa948345cc42faf5fcc05b5aa8`. `SSOT_VERSION.commit` matches exactly.

## Work completed in this session

1. Read all Notification Center S1/S2 files, `backend/AGENTS.md`, the repository backend skill, and relevant Go database/error/testing guidance.
2. Confirmed that only `task-center.atomic_task_status_changed` and `workflow-canvas.canvas_run_status_changed` are first-phase `ACTIVE` inputs; `CONTRACT_GAP` and `FUTURE` topics remain disabled.
3. Audited inbox mutations, exact `notification_updated.changed_fields`, candidate leases, retry/dead-letter behavior, aggregation monotonicity, counters, Notification Outbox, SSE idempotency, and retention transactions.
4. Fixed structurally valid source events with missing `created_by` so they persist as candidates and reach the observable `ERR_NOTIFICATION_RECIPIENT_UNRESOLVED` retry/dead-letter lifecycle instead of remaining at the message NACK boundary.
5. Added regression coverage for unresolved-recipient candidates and idempotent UserEvent recovery when Notification Outbox confirmation fails after UserEvent persistence.
6. Fixed credential sanitization for `Authorization: Bearer ...`, JSON authorization values, API-key variants, full URLs, private addresses, and stack lines. The old Authorization regex could leave the bearer token suffix behind.
7. Made the optional `POST /api/v1/notifications/read-all` body accept an actual empty/chunked-empty request while still strictly rejecting unknown fields in non-empty bodies.
8. Extended real PostgreSQL integration coverage for candidate dead-letter state and Notification Outbox `FAILED → PUBLISHED` recovery, including payload preservation and idempotent publish confirmation.
9. Regenerated error-code documentation/code and deepcopy code.
10. Added the authorized standalone `notificationworker` command, lifecycle bootstrap, lightweight image, and Compose service.
11. Removed all Notification Center startup and store dependencies from `taskworker`; its generic Task/Asset/Canvas SSE projector remains unchanged.
12. Added lifecycle tests for missing dependencies, partial subscription startup cleanup, and context-driven shutdown.
13. Updated deployment/configuration/architecture documentation for the new process boundary and rolling-upgrade constraint.
14. Final reliability audit made registry registration errors explicit and preserved Notification Outbox failure-marking errors with the original projection error.
15. Performed the final repository-state audit and refreshed this handoff without changing implementation files.
16. Created the user-requested local commit with subject `feat(notification): implement notification center worker`; no remote push was performed.

## Current in-progress work

None. The complete Notification Center and `notificationworker` change set is committed locally. No remote push was performed.

## Files added, modified, renamed, or removed

Local commit inventory:

- 28 previously tracked paths modified.
- 30 new files added.
- 0 deleted or renamed paths.

Modified tracked paths:

- `AGENTS.md`
- `SSOT_VERSION`
- `backend/apis/iapiserver/deepcopy_generated.go`
- `backend/apis/iapiserver/meta_sse.go`
- `backend/apis/imachinery/list.go`
- `backend/apis/imachinery/opt.go`
- `backend/internal/apiserver/route.go`
- `backend/internal/apiserver/route_application_platform_test.go`
- `backend/internal/apiserver/server.go`
- `backend/internal/apiserver/service/v1/platform/service.go`
- `backend/internal/apiserver/service/v1/platform/service_test.go`
- `backend/internal/apiserver/store/postgresql/0_pg.go`
- `backend/internal/apiserver/store/postgresql/outbox.go`
- `backend/internal/apiserver/store/postgresql/task_center_events.go`
- `backend/internal/apiserver/store/postgresql/workflow_canvas_events.go`
- `backend/internal/apiserver/store/store.go`
- `backend/internal/pkg/code/base.go`
- `backend/internal/pkg/code/code_generated.go`
- `configs/README.md`
- `deployments/README.md`
- `deployments/docker-compose.verify.yaml`
- `deployments/docker-compose.yaml`
- `docs/HANDOFF.md`
- `docs/guide/zh-CN/api/error_code_generated.md`
- `docs/guide/zh-CN/architecture/taskworker-apiserver-collaboration.md`
- `scripts/make-rules/gen.mk`
- `scripts/make-rules/image.mk`
- `ssot` gitlink

Untracked files:

- `backend/apis/iapiserver/meta_notification.go`
- `backend/apis/iapiserver/request_notification.go`
- `backend/apis/iapiserver/request_notification_test.go`
- `backend/apis/iapiserver/response_notification.go`
- `backend/apis/imachinery/deepcopy_generated.go`
- `backend/cmd/notificationworker/notificationworker.go`
- `backend/internal/apiserver/controller/v1/notification/notification.go`
- `backend/internal/apiserver/controller/v1/notification/response.go`
- `backend/internal/apiserver/controller/v1/notification/response_test.go`
- `backend/internal/apiserver/notificationworker/adapters.go`
- `backend/internal/apiserver/notificationworker/registry.go`
- `backend/internal/apiserver/notificationworker/rules.go`
- `backend/internal/apiserver/notificationworker/rules_test.go`
- `backend/internal/apiserver/notificationworker/worker.go`
- `backend/internal/apiserver/notificationworker/worker_test.go`
- `backend/internal/apiserver/notificationworker_app.go`
- `backend/internal/apiserver/notificationworker_runner.go`
- `backend/internal/apiserver/notificationworker_runner_test.go`
- `backend/internal/apiserver/service/v1/notification/service.go`
- `backend/internal/apiserver/service/v1/notification/service_test.go`
- `backend/internal/apiserver/service/v1/sse/notification_projector.go`
- `backend/internal/apiserver/service/v1/sse/notification_projector_test.go`
- `backend/internal/apiserver/store/postgresql/notification_center.go`
- `backend/internal/apiserver/store/postgresql/notification_center_integration_test.go`
- `backend/internal/apiserver/store/postgresql/notification_center_test.go`
- `backend/internal/apiserver/store/postgresql/notification_store.go`
- `backend/internal/apiserver/store/postgresql/notification_worker_store.go`
- `backend/internal/pkg/notificationsanitize/sanitize.go`
- `backend/internal/pkg/notificationsanitize/sanitize_test.go`
- `build/docker/notificationworker/Dockerfile.build`

## Key architectural decisions

- Notification Center projects reliable business results; it is not a second Task Center or source-domain fact store.
- Source domains write their own outbox events. Notification Center persists only bounded, sanitized candidates and user-visible summaries.
- First-phase rules notify only eligible standalone AtomicTask and CanvasRun results.
- Notification, recipient counter, and Notification Outbox changes commit atomically. SSE owns UserEvent persistence, replay, and `/api/v1/events/stream`.
- Email, Webhook, mobile push, digest, quiet hours, dynamic rule CRUD, private notification SSE, `CONTRACT_GAP`, and `FUTURE` topics remain disabled.
- The user authorized `backend/cmd/notificationworker`. The new process starts the notification-specific projector, rule worker, source worker, and retention runner in dependency order and closes them in reverse order.
- `notificationworker` reuses the existing API Server configuration schema for database and SSE retention values, but does not initialize Conductor, ProviderCapability, FFmpeg, or asset storage.

## API, schema, dependency, and configuration changes

- Implements all released Notification Center REST paths under `/api/v1`.
- Implements the released notification tables, constraints, indexes, topic catalog, errors, permissions, Notification Outbox events, and SSE UserEvent types.
- `read-all` now handles the OpenAPI-optional empty request body independently of HTTP transfer framing.
- No third-party dependency or runtime configuration key was added.

## Verification performed

Passed focused tests:

```text
go test ./backend/apis/iapiserver ./backend/apis/imachinery ./backend/internal/pkg/notificationsanitize ./backend/internal/apiserver/service/v1/notification ./backend/internal/apiserver/controller/v1/notification ./backend/internal/apiserver/notificationworker ./backend/internal/apiserver/service/v1/sse ./backend/internal/apiserver/store/postgresql ./backend/internal/apiserver
```

After the worker split, this focused startup suite also passes:

```text
go test ./backend/internal/apiserver/notificationworker ./backend/internal/apiserver/service/v1/sse ./backend/internal/apiserver
```

Worker split formatting and static boundary checks pass:

```text
gofmt -d backend/cmd/notificationworker/notificationworker.go backend/internal/apiserver/notificationworker_app.go backend/internal/apiserver/notificationworker_runner.go backend/internal/apiserver/notificationworker_runner_test.go
rg -n "notificationworker|NotificationCandidates|NotificationOutbox|NotificationRetention|NewNotificationProjector" backend/internal/apiserver/taskworker.go
git diff --check
```

Passed race detection:

```text
go test -race ./backend/internal/apiserver/notificationworker ./backend/internal/apiserver/service/v1/sse ./backend/internal/apiserver/store/postgresql ./backend/internal/pkg/notificationsanitize
```

Both worker commands build:

```text
go build ./backend/cmd/notificationworker ./backend/cmd/taskworker
```

Passed against an isolated real PostgreSQL database, which was removed afterward:

```text
go test -tags=integration ./backend/internal/apiserver/store/postgresql -run '^TestNotificationCenterPostgres' -count=1
```

The worker split was re-run against isolated database `omnimam_notificationworker_test_20260729_a1`; all four integration cases passed and the database was force-dropped and confirmed absent afterward.

Compose configuration and the lightweight image pass:

```text
docker compose -f deployments/docker-compose.yaml -f deployments/docker-compose.verify.yaml config --quiet
make configs
make image.build IMAGES=notificationworker VERSION=spec-v1.8.0-notificationworker-test REGISTRY_PREFIX=omnimam
```

Image inspection confirmed the expected entrypoint/config, no FFmpeg executable, and no ProviderCapability directory. Docker emitted only its existing style warning about the resolved constant `FROM --platform=linux/amd64`.

Final SSOT and contract audit passed:

```text
git -C ssot status --short
git submodule status ssot
git -C ssot describe --tags --exact-match HEAD
git diff --submodule=short -- ssot
rg -n "notificationworker|NotificationCandidates|NotificationOutbox|NotificationRetention|NewNotificationProjector" backend/internal/apiserver/taskworker.go
git diff --check
```

The submodule is clean and pinned to released `spec-v1.8.0`; the parent repository changes only the gitlink from the prior release to commit `44e659294c3fc8fa948345cc42faf5fcc05b5aa8`. `SSOT_VERSION.commit` matches. The worker split adds no REST path, DTO field, database schema, error code, permission, or event type. Only the two released `ACTIVE` source mappings are registered; all `CONTRACT_GAP`, `FUTURE`, and external-channel capabilities remain disabled.

Final repository-state audit on 2026-07-29 after the local commit:

```text
branch: feature/task-center
HEAD subject: feat(notification): implement notification center worker
upstream: origin/feature/task-center
ahead/behind: ahead 3, behind 0
tracked modifications: 0
untracked files: 0
staged paths: 0
deleted paths: 0
renamed paths: 0
ssot exact tag: spec-v1.8.0
ssot commit: 44e659294c3fc8fa948345cc42faf5fcc05b5aa8
ssot internal worktree: clean
SSOT_VERSION.commit: exact match
```

The final audit confirmed a clean parent worktree and index after committing, and re-confirmed the clean `ssot/` internal worktree plus exact released tag/commit match. No tests, builds, generation, Compose validation, image build, deployment, or remote push were repeated during this commit-only pass.

Generation passed:

```text
make gen
make gen.deepcopy
```

`git diff --check` passes. SSOT tag, commit, and `SSOT_VERSION` match.

Full backend testing was previously attempted:

```text
go test ./backend/...
```

All Notification Center packages passed. The full command failed only on the pre-existing unrelated thumbnail localization mismatch:

- `backend/internal/apiserver/service/v1/taskcenter.TestAssignSystemName`
- `backend/internal/apiserver/taskname.TestResolve/parameterized_system_name`

Actual catalog: `生成 thumbnail视图`; tests expect `生成 thumbnail 表现形式`.

## Outstanding tasks

1. Review the local Notification Center commit.
2. Roll out new `taskworker` and `notificationworker` images together when deployment is authorized, then optionally run a live process-level event test.
3. Resolve the unrelated thumbnail localization mismatch separately.

## Known issues and risks

- Deployment must not run an older `taskworker` image that still starts the notification consumer groups alongside the new `notificationworker`; roll both images together.
- A completely malformed source message without stable identity/version cannot be represented by the released `notification_events` schema; it remains NACK/retry at the source subscription boundary. Missing recipient basis now correctly reaches candidate dead-letter state.
- A local test-tagged `notificationworker` image was built and inspected; no server image was deployed or running service changed.
- An older database named `omnimam_notification_test_20260729` was deliberately left untouched because its ownership is unknown.

## Exact recommended next step

Review the local commit, then deploy the new `taskworker` and `notificationworker` images together when deployment is authorized.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
