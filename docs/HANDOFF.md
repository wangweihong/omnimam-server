# Handoff

## Current goal and status

- Goal: fix the local AppStudio create failure `The StudioApplication is not in a state that permits this operation.:web-react blueprint is unavailable`.
- Status: complete. Full Compose rebuild and post-deployment acceptance passed.

## Work completed in this session

- Ran `make compose`; all backend images were rebuilt at `2cb63c2-amd64`, the Compose stack was recreated successfully, and persistent volumes were preserved.
- Confirmed the running API Server binary itself contains both `blueprints/web-react/v1/template/.gitignore` and `.gitlab-ci.yml`.
- Refreshed the authenticated AppStudio page after deployment; the create form and Coding model options load normally.
- Reproduced and traced the failure to `LoadBlueprint("web-react", "v1")` in `CreateApplication`.
- Confirmed `blueprint.yaml` requires `.gitignore` and `.gitlab-ci.yml`, while the directory-form `//go:embed blueprints` excluded both dotfiles.
- Added a regression test to the existing AppStudio test file. Before the fix it failed with `read blueprint file ".gitignore": ... file does not exist`.
- Explicitly embedded both controlled dotfiles without using `all:blueprints`, which would also include unrelated hidden generated content.
- Built `omnimam/apiserver:2cb63c2-amd64` and recreated only the `apiserver` Compose service.

## Current in-progress work

- None.

## Files added, modified, renamed, or removed

- Modified: `backend/internal/apiserver/service/v1/appstudio/blueprint.go`.
- Modified: `backend/internal/apiserver/service/v1/appstudio/webhook_test.go`.
- Modified: `docs/HANDOFF.md`.
- No files were added, renamed, or removed. Existing unrelated untracked documents under `docs/` remain untouched.

## Key architectural or design decisions

- Keep the Blueprint allowlist defined by `blueprint.yaml` and explicitly embed only its required dotfiles.
- Do not use Go's `all:` embed prefix because the template tree contains unrelated hidden generated content such as package-manager state.

## API, schema, dependency, or configuration changes

- No API, schema, dependency, environment-variable, or configuration changes.
- SSOT remains released `spec-v1.23.2` commit `9e1bf2291dd1925e982a5dd728e05a27c334f8d9`, matching `SSOT_VERSION`.

## Verification performed and remaining checks

- `make compose` passed.
- Compose status after rebuild: PostgreSQL, Redis, Conductor, and Infrastructure Server are healthy; API Server, Task Worker, Notification Worker, and frontend are running.
- API Server `GET http://127.0.0.1:8080/healthz` returned `{"status":"ok"}`; Infrastructure health endpoint also succeeded.
- The deployed `/opt/omnimam/bin/apiserver` contains both required embedded Blueprint paths, verified by copying the binary to a temporary directory and inspecting its strings.
- API Server logs since the rebuild contain no `web-react blueprint is unavailable` or `read blueprint file` failure.
- Browser acceptance after reload passed for the authenticated AppStudio create page and model selector.
- Confirmed the regression test fails before the production fix and passes afterward.
- `go test ./backend/internal/apiserver/service/v1/appstudio -count=1` passed.
- `go list` confirms both `template/.gitignore` and `template/.gitlab-ci.yml` are compiled into the AppStudio package.
- `git diff --check` passed.
- `make image IMAGES=apiserver` passed and produced `omnimam/apiserver:2cb63c2-amd64`.
- Recreated only `omnimam-apiserver`; `GET http://127.0.0.1:8080/healthz` returned `{"status":"ok"}`.
- A real UI create was not submitted because it would write persistent AppStudio/Agent/Task records and the local GitLabServer projection is currently empty. The original Blueprint loading failure is covered at the regression-test, package-build, deployed-binary, service-health, and browser-load boundaries.

## Outstanding tasks

- Configure a local READY GitLabServer, then run the broader create-to-GitLab initialization E2E if that workflow needs acceptance beyond this bug fix.

## Known issues and risks

- Full AppStudio initialization still depends on a configured READY GitLabServer; this is separate from the fixed Blueprint embedding failure.
- `CreateApplication` still exposes the stable public error message rather than the internal `LoadBlueprint` cause. The regression test prevents this specific asset omission from recurring.

## Exact recommended next step

Configure a local READY GitLabServer and create one AppStudio application to validate initialization DAG completion through READY, recording the application and task IDs.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
