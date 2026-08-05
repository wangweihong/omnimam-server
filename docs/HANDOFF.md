# Project Handoff

## Current goal and status

- Goal: update the server to released SSOT tag `spec-v1.17.0` and implement its required User Model, Model Gateway, and AI Chat backend changes.
- Status: complete. The server is pinned to released `spec-v1.17.0`; the required User Model, Model Gateway, and AI Chat backend changes are implemented and all permitted focused checks pass.

## Work completed in this session

- Updated `ssot` to released tag `spec-v1.17.0` at commit `bb3a0e8ff182c740f8d08e46613a6876a8e2aa86`.
- Updated `SSOT_VERSION` and verified its commit matches the submodule HEAD.
- Read the repository rules, `backend/AGENTS.md`, and `skills/omnimam-server-backend/SKILL.md`.
- Read the `spec-v1.17.0` release entry and only its directly relevant User Model, Model Gateway, and AI Chat contract sections.
- Mapped the changed contracts to the existing Model Gateway registry, legacy model-management service/controller/routes, AI Chat service, DTOs, and stores.
- Confirmed the existing model tables already use `user_model_providers`, `user_provider_models`, and `user_default_model_configs`.
- Added canonical public ProviderType DTOs and changed the OpenAI-compatible stable ID from `openai-compatible` to `openai_compatible` without a compatibility alias.
- Extended the immutable Runtime Registry with ProviderType registrations, private Adapter/Executor mappings, validation, sorted redacted projections, and defensive deep copies.
- Registered `openai_compatible` and `deepseek_official` with their SSOT-defined authentication, configuration, discovery/probe, Adapter, and capability Executor facts.
- Added OpenAI-compatible Gateway Adapter/Executor bootstrap implementations by reusing the existing OpenAI-compatible protocol implementation.
- Extended the existing Registry tests for projection immutability and invalid internal mappings; focused Registry tests pass.
- Verified the static adapter registrations and OpenAI-compatible protocol reuse with the existing focused adapter tests.
- Added the controlled `ListProviderTypes`, `TestProviderConnection`, `DiscoverProviderModels`, `ProbeProviderModel`, and `ResolveUserModelCapabilities` module interfaces.
- Added an injected `CredentialResolver`; Gateway requests accept only scoped opaque handles and never return resolved authentication fields.
- Added request-time ProviderType, endpoint, authentication, credential, and non-sensitive config-schema validation without persisting transient connection data.
- Added OpenAI-compatible `/models` discovery from `data[].id`, model probe by directory lookup only, and `OpenAI-Organization` / `OpenAI-Project` header application.
- Generated `ProviderType.DeepCopy` and used it for defensive public Registry projections.
- Added the canonical User Model service and controller with owner-scoped Provider/model/default operations, saved and unsaved Provider tests, non-destructive model sync, model probes, health fact persistence, capability projection, and default-model eligibility checks.
- Installed only canonical `/api/v1/user-model/...` routes and removed legacy model-management route registration.
- Added focused tests for opaque credential handles, non-destructive model sync, and canonical route coverage; the related User Model, Gateway, adapter, store compile, route, and DTO compile checks pass.
- Added short-lived `UserModelExecutionContext` issuance with owner, provider/model eligibility, capability, configuration-version, and default-usage validation.
- Added `ExecuteOperation(UserModelTarget)` with issuer, principal scope, expiry, capability, configuration-version, Registry mapping, and executor implementation validation.
- Added focused tests for non-sensitive execution grants, cross-user rejection, expiry rejection, and Registry executor dispatch; User Model and Model Gateway package tests pass.
- Added the SSOT-required GenerationRun route snapshot fields: `capability_definition_id`, `model_config_version`, and non-sensitive `model_snapshot_json`, including persistence JSON hooks.
- Changed AI Chat generation transaction creation to persist a fixed User Model route snapshot before any remote provider execution.
- Routed chat, image chat, translation, regenerate, and edit-regenerate through User Model execution-context resolution and Model Gateway operation execution.
- Required both `text.chat_completion` and `image.understanding` for image chat; translation uses `text.translate`.
- Removed AI Chat's direct construction of the legacy OpenAI-compatible adapter and injected the complete AI Chat service from server bootstrap.
- Updated the existing AI Chat relation test to verify the User Model/Gateway boundary; focused AI Chat and API-server compile checks passed before the final generation pass.
- Regenerated API deep-copy code from the repository root and verified `AIChatGeneration.DeepCopyInto` deep-copies `ModelSnapshot` through `deepcopyGenAnyInto`.
- Fixed `text.translate` Gateway input to carry `target_language` as a system instruction while keeping source content in the user message.
- Extended the existing AI Chat test file to cover the OpenAI-compatible `values.choices[0].message.content` output shape and translation target propagation; the focused AI Chat package test passes.

## Current in-progress work

- None.

## Files added, modified, renamed, or removed

- Modified `SSOT_VERSION`.
- Updated the `ssot` submodule gitlink.
- Modified `docs/HANDOFF.md`.
- Modified `backend/apis/iapiserver/meta_platform.go`.
- Modified `backend/apis/iapiserver/request_platform.go`.
- Regenerated `backend/apis/iapiserver/deepcopy_generated.go`.
- Modified `backend/internal/apiserver/service/v1/modelgateway/registry.go` and existing `registry_test.go`.
- Added `backend/internal/apiserver/service/v1/modelgateway/user_model_gateway.go`.
- Modified `backend/internal/apiserver/store/store.go`, `factory.go`, `postgresql/0_pg.go`, and `postgresql/platform.go` for owner-scoped access, transactional cascade deletion, and model health-check persistence.
- Modified Model Gateway adapter bootstrap and DeepSeek registration.
- Added `backend/internal/apiserver/service/v1/modelgateway/adapters/providers/openaicompat/registration.go`.
- Modified the OpenAI-compatible protocol adapter, shared HTTP JSON transport, and existing adapter tests.
- Added `backend/internal/apiserver/service/v1/usermodel/credential.go` and `service.go`.
- Added `backend/internal/apiserver/controller/v1/usermodel/usermodel.go`.
- Modified `backend/internal/apiserver/server.go` and `route.go` to inject User Model and install only canonical `/api/v1/user-model/...` routes.
- Modified `backend/apis/iapiserver/meta_ai_chat.go`, `backend/internal/apiserver/store/store.go`, and `backend/internal/apiserver/store/postgresql/ai_chat.go` for immutable GenerationRun route snapshots.
- Modified `backend/internal/apiserver/service/v1/aichat/aichat.go`, its existing relation test, and `backend/internal/apiserver/controller/v1/aichat/aichat.go` to use the User Model/Gateway execution boundary.

## Key architectural or design decisions

- `user-model` immediately replaces `model-management`; canonical APIs use `/api/v1/user-model/...` and legacy model routes are removed without aliases or redirects.
- User Model owns user Provider records, models, defaults, health facts, execution eligibility, and issuance of `UserModelExecutionContext`.
- Model Gateway owns the ProviderType registry, adapters, discovery, probes, and operation execution using `UserModelTarget`.
- ApplicationEngineType remains a separate existing runtime concept and must not be exposed as the ProviderType registry.
- AI Chat must resolve a User Model execution context before invoking Model Gateway.
- GenerationRun must persist `model_id`, `capability_definition_id`, `model_config_version`, and a non-sensitive `model_snapshot_json`.
- Execution context connection details remain in-process only; `model_snapshot` excludes endpoint, credential reference, credential handle, and authentication configuration.
- Required capability IDs are `text.chat_completion`, `text.translate`, and additionally `image.understanding` for image chat.
- Provider connection, discovery, and model probe use request-local transient `EngineInstance` values solely to reuse the existing transport; they are never persisted or converted into platform Engine/Binding facts.
- OpenAI-compatible model probe performs only `GET /models`; it never issues a chat or generation request.

## API, schema, dependency, or configuration changes

- SSOT pin changed from `spec-v1.16.1` to `spec-v1.17.0`.
- Added the SSOT-defined in-process ProviderType DTO and registry contract.
- Provider responses now include read-only `config_version`; provider-model responses add feature labels, disabled/derived capability fields, execution eligibility fields, resolution status, health timestamp, and read-only config version.
- Canonical create/update requests no longer accept client-maintained capabilities or `stream_supported`; provider/model inputs now carry bounded string and URL validation.
- Added the `ModelHealthCheck` persistence model and store boundary; focused PostgreSQL package compilation passes.
- Added an in-process 30-second opaque credential handle broker scoped to ProviderType and authentication type.
- User Model CRUD now uses owner-scoped store reads/deletes; model sync only creates missing remote models and does not overwrite user-maintained fields.
- Removed legacy `/model-providers`, `/provider-models`, `/default-models`, and `/model-options` route registration without aliases.

## Verification performed and remaining checks

- Verified `SSOT_VERSION.commit == ssot HEAD == bb3a0e8ff182c740f8d08e46613a6876a8e2aa86` and exact tag `spec-v1.17.0`.
- Verified the release is eligible as a formal implementation basis.
- Passed focused Registry tests: `go test ./internal/apiserver/service/v1/modelgateway -run 'Test(StaticRegistries|StaticRegistryRejectsInvalidRegistrations|ProviderTypeRegistryRejectsInternalMappingErrors)$' -count=1`.
- Passed focused adapter bootstrap tests: `go test ./internal/apiserver/service/v1/modelgateway/adapters -run 'Test(StaticRegistrationsHaveImplementations|OpenAICompatibleProvidersUseProtocolPackage)$' -count=1`.
- Passed focused User Model Gateway tests: `go test ./internal/apiserver/service/v1/modelgateway -run 'Test(StaticRegistries|StaticRegistryRejectsInvalidRegistrations|ProviderTypeRegistryRejectsInternalMappingErrors|UserModelGatewayProviderOperations|UserModelGatewayRejectsUntrustedConnectionFields)$' -count=1`.
- Passed focused OpenAI-compatible discovery/probe tests: `go test ./internal/apiserver/service/v1/modelgateway/adapters -run 'Test(StaticRegistrationsHaveImplementations|OpenAICompatibleProvidersUseProtocolPackage|OpenAICompatibleDiscoversAndProbesModelsWithoutGeneration)$' -count=1`.
- Passed compile-only checks for `service/v1/usermodel`, `controller/v1/usermodel`, and `service/v1/modelgateway`.
- Passed full focused User Model and Model Gateway package tests, focused OpenAI-compatible adapter tests, focused PostgreSQL compile, canonical route test, API DTO compile, and `git diff --check` as recorded in the current session checkpoint.
- Re-ran focused User Model, Model Gateway, PostgreSQL compile, canonical route, and API DTO compile checks after exposing `last_checked_at`; all pass.
- Passed focused AI Chat package tests and API-server route compile/test checks before this checkpoint.
- Passed the final focused AI Chat package test after deep-copy generation and response-shape coverage.
- Verified by code review that chat, regenerate, edit-regenerate, and in-topic translation create their GenerationRun transaction before Gateway execution; standalone translation is a MessageTranslation operation and still resolves User Model before Gateway execution as required by BR-AICHAT-27.
- Re-ran `go test ./internal/apiserver/service/v1/usermodel -count=1`, `go test ./internal/apiserver/service/v1/modelgateway -count=1`, PostgreSQL compile, canonical route test, and API DTO compile; all pass.
- Verified AI Chat has no remaining `OpenAICompatibleAdapter` or `invokeProvider` execution path.
- Verified `ssot/` has no internal file changes and remains at exact tag `spec-v1.17.0`.
- Final `git diff --check` passes. The full repository test suite was intentionally not run per task constraints.

## Outstanding tasks

- None for the requested `spec-v1.17.0` backend update.

## Known issues and risks

- Existing provider type value `openai-compatible` differs from canonical `openai_compatible`; no client compatibility alias may be invented.
- The current worktree is dirty with the intentional SSOT and handoff changes; preserve all unrelated user changes.
- `ProviderTestRequest` now represents an unsaved-provider test and has required connection fields; the saved-provider test endpoint must use only its path provider ID instead of binding that DTO.
- Do not modify files inside `ssot/` in this repository.
- Do not add a new binary under `backend/cmd/` without explicit user approval.

## Exact recommended next step

Review the completed diff, then stage and commit the `spec-v1.17.0` server update when ready.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
