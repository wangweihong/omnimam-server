# Handoff

## Current goal and status

- Goal: extend the OmniMAM derived-image pattern to the Hermes agent and all related default Alpine runtime images, install Git in each, and update the corresponding Infrastructure profile image defaults.
- Status: complete and verified locally.

## Work completed in this session

- Started the follow-up image task and revalidated the released SSOT pin and backend implementation rules.
- Read `skills/omnimam-server-backend/SKILL.md` and `backend/AGENTS.md`.
- Verified `ssot` and `SSOT_VERSION` both point to released `spec-v1.23.1` commit `d1b24118091e52d60fc20a3faa4cf47647f467ab`.
- Classified this as a deployment/runtime packaging change that does not alter SSOT product semantics or S1/S2 contracts, so no additional SSOT files are required.
- Located and replaced the former direct `ghcr.io/anomalyco/opencode:1.18.13` coding-agent defaults in `scripts/install/environment.sh` and the self-contained Compose fallback.
- Added an independent coding-agent Dockerfile and Make target; the image remains based on OpenCode `1.18.13` and installs Git through Alpine `apk`.
- Updated the synchronized `agent.coding@1.0` defaults to `omnimam/coding-agent:1.18.13` and documented the build command.
- Added derived Dockerfiles for Hermes, AppStudio Nginx and AppStudio Node; Hermes explicitly installs Git with Debian `apt`, while both Alpine images use `apk`.
- Replaced the single coding image Make rule with `infrastructure.images` plus four independently callable image targets.
- Updated all eight Infrastructure profile defaults: Hermes and Coding each use their own agent image, Preview/Production share the AppStudio Nginx image, and Build profiles share the AppStudio Node image.

## Current in-progress work

- None.

## Files added, modified, renamed, or removed

- Added: `build/docker/coding-agent/Dockerfile`, `build/docker/hermes-agent/Dockerfile`, `build/docker/appstudio-nginx/Dockerfile`, `build/docker/appstudio-node/Dockerfile`, `scripts/make-rules/infrastructure-images.mk`.
- Removed/replaced: `scripts/make-rules/coding-agent-image.mk` was superseded before commit by the consolidated Infrastructure image rules.
- Modified: `Makefile`, `scripts/make-rules/frontend-image.mk`, `scripts/install/environment.sh`, `deployments/docker-compose.yaml`, `deployments/README.md`, `docs/HANDOFF.md`.
- Existing unrelated untracked design documents under `docs/` remain untouched.

## Key architectural or design decisions

- Keep OpenCode pinned at `1.18.13`; add only Git in a derived OmniMAM image.
- Keep Hermes pinned at `v2026.8.3`, Nginx at `1.27-alpine`, and Node at `22-alpine`; each derived image guarantees Git independently of upstream contents.
- Reuse one Nginx-derived image for all Preview/Production profiles and one Node-derived image for all Build profiles.
- Preserve `scripts/install/environment.sh` as the deployment environment-variable fact source and keep its Compose fallback synchronized.
- Keep the Go Docker provider image-agnostic: its existing `ProfileImageResolver` consumes the injected profile mapping, so no hardcoded image defaults were added to backend code.

## API, schema, dependency, or configuration changes

- No API, database schema, error code, permission, event, or Go dependency changes.
- Configuration change: every `OMNIMAM_INFRA_PROFILE_IMAGES` entry now defaults to one of the four local `omnimam/*` derived images in both the environment fact source and Compose fallback.

## Verification performed and remaining checks

- `make infrastructure.images` passed and produced all four local images.
- Container Git checks passed: Coding/Node use Git `2.54.0`; Hermes/Nginx use Git `2.47.3`.
- Runtime checks passed: OpenCode `1.18.13`, Hermes Agent `v0.20.0 (2026.8.3)`, Nginx `1.27.5`, and Node `v22.23.0`.
- Image inspection confirmed all upstream entrypoints, commands and users remain inherited.
- `go test ./backend/internal/infrastructure/providers/dockerruntime` passed.
- `docker compose -f deployments/docker-compose.yaml config` passed without sourcing an environment file and rendered all eight new profile defaults.
- A `jq` assertion confirmed `scripts/install/environment.sh` exports the exact intended eight-entry mapping.
- `make -pn` confirmed all four standalone runtime image directories remain excluded from the Go service image list.
- `git diff --check` passed.

## Outstanding tasks

- This derived-image task has no remaining implementation work.
- Pre-existing project acceptance work remains: run the local GitLab AppStudio smoke test against a clean-data deployment with one READY default GitLabServer.

## Known issues and risks

- All four Infrastructure runtime image targets are intentionally independent of the Go service image rules because they contain no repository Go binary.
- The four derived images exist only in the local Docker daemon until explicitly published; another deployment host must build them or override `OMNIMAM_INFRA_PROFILE_IMAGES` with published tags.
- Existing unrelated untracked files under `docs/` must not be modified.

## Exact recommended next step

Use the four locally built images for the AppStudio smoke test; publish or override their tags first if the Runtime Docker daemon is on another host.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
