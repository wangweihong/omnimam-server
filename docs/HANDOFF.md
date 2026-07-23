# Project Handoff

## Current project goal

Continue OmniMAM server development on released `spec-v1.7.3`. The current completed increment implements Task Center v1.4 multilingual names for newly created system-named AtomicTask, TaskGroup, DAGTaskGroup, and TaskSchedule resources without translating user names or heuristically backfilling legacy rows.

## Completed in this session

1. Updated the pinned `ssot` submodule and `SSOT_VERSION` from released `spec-v1.7.2` to released `spec-v1.7.3` commit `4949983e76c5f15448e1b94d9260c93fb2a55ad5`.
2. Added persisted Task Center name metadata: `name_source`, `system_name_key`, and `system_name_params_json`. `name_i18n` remains a query projection and is not stored.
3. Added the internal `taskname` catalog with stable keys and `zh-CN`/`en-US` names for Application Run, Asset thumbnail/Artifact/Representation jobs, ComfyUI test DAG tasks, and SYSTEM RECONCILE schedules.
4. Added internal-only `SystemNameSpec` creation metadata. Public JSON requests cannot set it. User-created resources remain `USER` and return no fabricated `name_i18n`.
5. System creation paths now resolve the compatibility `name` from the catalog's `en-US` value and persist stable key/parameters. This covers Application Run retry binding, ComfyUI workflow tests, Asset thumbnail/Artifact/Representation tasks, Representation backfill actions, and the three SYSTEM RECONCILE schedules.
6. Manual AtomicTask/TaskGroup/DAG retries and Group/DAG child expansion preserve system name keys and parameters.
7. Query projection now propagates `name_i18n` through resource lists/details and retry, owner, target, schedule source, DAG node, and timeline summaries.
8. Corrected SYSTEM RECONCILE schedule creation so it validates the catalog key, stores the English compatibility name, and returns the localized projection immediately. Existing schedules found by `system_key` are not rewritten or heuristically upgraded.
9. Added tests for catalog resolution/cloning, persistence columns, parameterized names, user/legacy non-translation, child metadata propagation, and localized SYSTEM schedule creation.
10. Updated the TaskWorker/API Server collaboration architecture document for `spec-v1.7.3` and the system-name localization boundary.

## Files added

- `backend/internal/apiserver/taskname/catalog.go`
- `backend/internal/apiserver/taskname/catalog_test.go`
- `backend/internal/apiserver/service/v1/taskcenter/localization.go`
- `backend/internal/apiserver/service/v1/taskcenter/localization_test.go`

## Files modified

- `SSOT_VERSION` and the `ssot` submodule pointer.
- Task Center API metadata/request/response models under `backend/apis/iapiserver/`.
- Task Center service, reconcile, relation, observability, controller, PostgreSQL store, and tests under `backend/internal/apiserver/`.
- Application Platform and Asset Library system task creation paths.
- TaskWorker event consumers, Representation DAG creation, and system schedule bootstrap.
- `docs/guide/zh-CN/architecture/taskworker-apiserver-collaboration.md`.
- This handoff.

No files were removed. The pre-existing untracked `docs/guide/zh-CN/architecture/asset-library-server-bulk-import.md` remains unrelated and was not modified.

## Key architectural decisions

- Stable system name metadata is a backend-only creation contract. Public DTOs cannot assert `SYSTEM` provenance.
- The persisted compatibility `name` is the catalog's `en-US` value. Localized maps are regenerated at query time so adding a language does not require row rewrites.
- The name catalog is a controlled package, not a database table or provider. Unknown keys and missing/unexpected parameters fail creation.
- Legacy rows default to `USER`; no inference from existing name text, createdBy, functionRef, or system schedule identity is allowed.
- Retry and child expansion copy stable name metadata. One-hop summaries resolve from the same catalog to prevent inconsistent translations.
- SYSTEM schedule ensure remains insert-only for configuration and does not mutate an existing schedule found by `system_key`.

## API, schema, and configuration changes

- Public Task Center responses gained optional `name_i18n` fields as defined by `spec-v1.7.3`.
- `atomic_tasks`, `task_groups`, `dag_task_groups`, and `task_schedules` gain the three released name metadata columns through the existing AutoMigrate initialization path.
- No public request field, endpoint, permission code, error code, event type, runtime configuration, or external provider changed.

## Verification

- Released tag verification: submodule commit resolves exactly to `spec-v1.7.3`.
- Focused Task Center/API model/store/controller/Application Platform/Asset Library/API Server tests passed.
- `go test ./backend/...` passed.
- `go vet ./backend/...` passed.
- `git diff --check` passed, and `SSOT_VERSION.commit` matches the submodule commit tagged exactly `spec-v1.7.3`.
- No PostgreSQL integration, Conductor, container build, deployment, browser, or end-to-end test was run in this session.

## Remaining work

1. When integration verification is authorized, start against PostgreSQL and confirm AutoMigrate adds the released columns with legacy rows reading as `USER`.
2. Build/deploy TaskWorker and verify a new system task plus SYSTEM schedule return consistent `name_i18n` in list, detail, relation, source, and timeline responses.
3. Resume the previously outstanding Workflow Canvas OutputBinding event consumer and persisted repair loop after this increment is accepted.

## Known issues and risks

- Database CHECK constraints in the SSOT design schema are not independently added by this increment; runtime initialization follows the repository's existing GORM AutoMigrate policy. Service/catalog validation enforces new writes.
- Existing Task Center rows intentionally do not gain translations. Existing SYSTEM schedules created before v1.7.3 remain legacy `USER` rows unless a separately released migration policy is introduced.
- The earlier TaskWorker FFmpeg image build remains unverified because package download timed out; live Representation backfill still requires deployment verification.
- The untracked bulk-import architecture draft belongs to separate work and must not be accidentally included with this increment.

## Recommended next task

After full verification and acceptance of the v1.7.3 localization increment, implement the Workflow Canvas OutputBinding event consumer and persisted repair loop using the current released SSOT contract.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
