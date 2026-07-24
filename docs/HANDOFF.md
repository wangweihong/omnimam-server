# Project Handoff

## Current project goal

Ship the released Application Platform contracts through `spec-v1.7.8`: preserve the `spec-v1.7.7` ComfyUI import/conversion boundary, add the builtin `comfyui-workflow-runtime` ProviderCapability, and guarantee a required immutable system binding for every ComfyUI EngineInstance.

## Completed in this session

1. Preserved the released `spec-v1.7.7` workflow change: import is engine-independent, while Visual-to-API conversion explicitly requires a usable ComfyUI instance and current object_info.
2. Updated and published `/home/wwhvw/codespace/omnimam-spec` as `spec-v1.7.8`; pushed release commit `62a3eda58bc8f8cc34449be1eaa056cde3340200` and tag `spec-v1.7.8`.
3. Pinned the server `ssot` submodule and `SSOT_VERSION` to that released commit.
4. Added `kind`, loader-derived read-only `origin`, and `binding_policy` to ProviderCapability. DeepSeek and Seedance use `catalog + directory + manual`.
5. Embedded `comfyui-workflow-runtime` as `engine_binding + builtin + required_immutable`, revision `2026-07-24.1`. It has no models, operations, or variants and cannot be a Provider template source.
6. Made builtin loading strict and first. External directory failures degrade only directory capabilities; reserved builtin IDs cannot be overridden; returned registry snapshots are defensive copies.
7. Added transactional EngineInstance creation with required bindings. A ComfyUI binding failure rolls back the instance and maps to `ERR_AIAPP_REQUIRED_ENGINE_BINDING_FAILED`; non-ComfyUI creation receives no required binding.
8. Added synchronous startup reconcile to API Server and TaskWorker. Conditional upsert repairs missing or drifted bindings without changing `resource_version` on a no-op and converges across replicas through the existing unique index.
9. Added read-only derived `system_managed`. Manual create, update, disable, restriction changes, and delete of a required system binding return `ERR_AIAPP_SYSTEM_ENGINE_BINDING_IMMUTABLE`.
10. Changed the binding-to-engine foreign key to `ON DELETE CASCADE`; migration constraint lookup is scoped by `conrelid` because PostgreSQL constraint names are not schema-global.
11. Added error `130231 ERR_AIAPP_PROVIDER_CAPABILITY_ID_RESERVED`, `130427 ERR_AIAPP_SYSTEM_ENGINE_BINDING_IMMUTABLE`, and `130428 ERR_AIAPP_REQUIRED_ENGINE_BINDING_FAILED`, then regenerated Go and Markdown outputs with `make gen`.
12. Added the capability registry architecture guide and regression coverage for registry loading, service behavior, startup reconcile, PostgreSQL rollback/upsert/cascade, and unchanged ComfyUI workflow paths.

## Files changed

- Contract pin: `SSOT_VERSION`, `ssot` submodule.
- API metadata: `backend/apis/iapiserver/meta_application_platform.go`, `meta_comfyui_workflow.go`, and `request_comfyui_workflow.go`.
- Registry: `backend/internal/apiserver/applicationplatform/registry.go`, `registry_test.go`, schema and provider manifests, plus `assets/builtin-provider-capabilities/comfyui.yaml`.
- Controller/bootstrap: Application Platform controller/response files and tests, `server.go`, and `taskworker.go`.
- Service: Application Platform, ComfyUI workflow, executor, runtime form, and related tests including `system_binding_test.go`.
- Store/migration: `store.go`, PostgreSQL Application Platform files/tests, `0_pg.go`, `application_platform_reset_integration_test.go`, and `application_platform_system_binding_integration_test.go`.
- Generated errors: `backend/internal/pkg/code/base.go`, `code_generated.go`, and `docs/guide/zh-CN/api/error_code_generated.md`.
- Deployment manifests: root and embedded DeepSeek/Seedance YAML files.
- Documentation: `docs/guide/zh-CN/architecture/application-platform-capability-registry.md` and this handoff.

No files were removed. Existing user changes in `/home/wwhvw/codespace/omnimam-spec/AGENTS.md` were preserved and not included in the release.

## Key architectural decisions

- `kind`, `origin`, and `binding_policy` are orthogonal: purpose, load source, and binding ownership respectively.
- `origin` is assigned by the loader. Directory YAML cannot claim builtin origin or replace a builtin ID.
- Required system bindings are initialization and management facts, not ComfyUI model or parameter facts.
- ComfyUI execution remains governed by API Workflow, workflow contract/manual mappings, the selected instance's current object_info, health, and template restrictions.
- New ComfyUI EngineInstance and required binding are one database transaction. Existing instances converge through startup reconcile.
- `system_managed` is derived from the current registry policy and is not persisted.
- Workflow import remains independent of EngineInstance. A target instance is selected only for Visual-to-API conversion and later compatibility/runtime checks.
- The breaking `spec-v1.7.7` removal of the legacy workflow source column still uses the one-time destructive Application Platform reset already present in this worktree.

## API, schema, and configuration changes

- Application Platform OpenAPI is `1.5.0` under `spec-v1.7.8`.
- ProviderCapability now returns `kind`, read-only `origin`, and `binding_policy`.
- EngineCapabilityBinding now returns read-only `system_managed`.
- `aiapp_engine_capability_bindings.engine_instance_id` now references EngineInstance with `ON DELETE CASCADE`.
- No endpoint, permission code, event type, dependency, or runtime configuration was added.

## Verification

- Upstream `spec-v1.7.8` release and tag are pushed; the server submodule and `SSOT_VERSION` commit match.
- Modified SSOT YAML parses successfully. Redocly validates the OpenAPI with no errors; embedded schema and all three manifests byte-match the released SSOT copies.
- `make gen` passed.
- Focused Application Platform unit tests passed.
- PostgreSQL integration `TestPostgresRequiredEngineBindings` passed against the local PostgreSQL container, including rollback, drift repair, no-op revision stability, FK migration, and cascade delete.
- `go test ./backend/...` passed.
- Scoped `go test -race` passed for the registry, Application Platform service, and PostgreSQL store packages.
- `go vet ./backend/...` and `git diff --check` passed.

## Outstanding tasks

1. Update the Web client for the `spec-v1.7.7` import/conversion request shape if it has not already been migrated.
2. Deploy or restart API Server and TaskWorker so startup reconcile backfills existing ComfyUI instances.
3. Run a real Visual Workflow import, explicit conversion, validation, test run, and formal run against a healthy ComfyUI instance.
4. Commit the server work only when explicitly requested.

## Known issues and risks

- Starting this server against a database that still has the legacy workflow source column triggers the intentional destructive Application Platform reset from `spec-v1.7.7`; linked execution projections are removed while unrelated Task Center data, user assets, and blobs are preserved.
- Startup now fails when strict builtin registry loading or required-binding reconcile fails. This is intentional, because serving a ComfyUI instance without its system binding violates the released contract.
- The Web client may still use the old ComfyUI import request until its separate migration is completed.
- Redocly reports 74 non-blocking warnings in the existing Application Platform OpenAPI, mainly missing 4xx responses/descriptions and inherited required-property modeling; this release adds no OpenAPI validation errors.

## Recommended next task

Update the Web import/conversion flow to the released contract, then run the end-to-end ComfyUI acceptance sequence after deploying this server revision.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
