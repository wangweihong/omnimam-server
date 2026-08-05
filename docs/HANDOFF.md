# Project Handoff

## Current goal and status

- Goal: initialize a default writable local `StorageBackend` during fresh API Server deployment.
- Status: complete. Implementation and all in-scope focused verification passed.
- SSOT gate: `SSOT_VERSION.commit` and `ssot` HEAD are both `35c2a582a53f9c416b71aa2695d99ad51797218e`, released as `spec-v1.17.1`.

## Work completed in this session

- Confirmed the root cause: migrations create `storage_backends`, but deployment startup did not seed a row. The only previous creation path was a legacy upload-time lazy initializer.
- Added `StorageBackendStore.EnsureDefaultLocal` with a PostgreSQL transaction-scoped advisory lock. It returns the oldest existing enabled, writable `local` backend and creates `default-local` only when none exists.
- Added `assetlibrary.ReconcileDefaultLocalStorageBackend` and invoked it after store/schema initialization during API Server construction, before request serving.
- The default root comes from `OMNIMAM_STORAGE_ROOT`; the non-deployment fallback remains `data/assets`, matching existing local storage behavior.
- Added and exported `OMNIMAM_STORAGE_ROOT` in `scripts/install/environment.sh` with deployment default `/var/lib/omnimam/assets`.
- Updated API Server and Task Worker Compose environments to use `${OMNIMAM_STORAGE_ROOT:-/var/lib/omnimam/assets}` so direct `docker compose` remains self-contained.
- Added focused unit and PostgreSQL integration coverage for creation, idempotency, and preservation of an existing administrator-selected writable local backend.

## Current in-progress work

- None.

## Files modified

- `backend/internal/apiserver/server.go`
- `backend/internal/apiserver/service/v1/assetlibrary/storage.go`
- `backend/internal/apiserver/service/v1/assetlibrary/storage_test.go`
- `backend/internal/apiserver/store/store.go`
- `backend/internal/apiserver/store/postgresql/platform.go`
- `backend/internal/apiserver/store/postgresql/asset_contract_integration_test.go`
- `deployments/docker-compose.yaml`
- `scripts/install/environment.sh`
- `docs/HANDOFF.md`

No files were added, renamed, or removed. No files under `ssot/` were modified.

## Key decisions

- Bootstrap is owned by API Server startup because that process initializes schema and is the authority that must make storage available before serving Asset Library requests.
- Existing enabled, writable local backends are never renamed, re-rooted, or otherwise overwritten.
- A PostgreSQL advisory transaction lock serializes only singleton default reconciliation across concurrently starting API Server replicas.
- No API, schema, error code, permission, event, dependency, or new binary was introduced.

## Configuration changes

- New deployment variable: `OMNIMAM_STORAGE_ROOT`.
- Installation-script and Compose defaults are synchronized at `/var/lib/omnimam/assets`.
- Local code fallback without the variable remains the absolute form of `data/assets`.

## Verification performed

- Final `go test ./internal/apiserver/service/v1/assetlibrary -count=1` passed.
- Final `go test ./internal/apiserver -run '^$' -count=1` passed.
- Final focused real-PostgreSQL `TestEnsureDefaultLocalStorageBackend` passed against an isolated database after verifying creation, idempotency, and preservation of the first writable local backend. The database was deleted afterward, and no `codex_storage_backend_bootstrap_%` database remains.
- `bash -n scripts/install/environment.sh` passed.
- `docker compose -f deployments/docker-compose.yaml config --quiet` passed.
- `git diff --check` passed.

## Remaining checks

- None within the requested task scope.

## Known issues and risks

- The normal package-wide integration command is blocked by an unrelated pre-existing compile error in `canvas_application_integration_test.go:103`: `map[string]any` is assigned to `iapiserver.TaskCancelPolicy`. Per task scope, this file is not modified. The target integration test is run with the package production files plus `asset_contract_integration_test.go`.
- The PostgreSQL integration validates creation and idempotency with a real database. It does not launch multiple API Server processes; concurrent startup safety relies on PostgreSQL `pg_advisory_xact_lock` semantics.

## Outstanding tasks

- None for this task.

## Exact recommended next step

Review and commit the nine modified files for the default `StorageBackend` bootstrap fix. Keep the unrelated integration compile error separate.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
