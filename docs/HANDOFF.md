# Handoff

## Current goal and status

- Goal: complete the Docker Exec stdin fix for `agent.runtime.ensure` by preserving writable HTTP 101 streams and the provider's configured timeout.
- Status: complete in the repository. The writable-upgrade fix and timeout preservation are implemented and verified; the latest timeout refinement has not been rebuilt or deployed.

## Work completed in this session

- Added a deterministic blocked-stream regression with `testing/synctest`; before the implementation change it fails immediately with a deadlock at `io.Copy(io.Discard, stream)`, proving the raw upgrade has no `client.Timeout` cancellation path.
- Added an operation context around the raw upgrade using `d.client.Timeout`; Go's nested deadline semantics preserve any earlier caller deadline.
- Applied the operation context to request startup, cancellation-driven body close, stdin write, half-close, stream drain, and terminal-state inspection, preserving `context.Canceled` / `context.DeadlineExceeded` identity without exposing stdin in errors.
- Confirmed the previously failing timeout regression and the complete Docker runtime provider test package pass.
- Verified released SSOT tag `spec-v1.20.0` at `0e6300c8e776df08972a229f48775ed71ad5bff9` matches `SSOT_VERSION`.
- Reviewed only the Docker runtime provider, its focused tests, and the direct `agent.runtime.ensure` call chain.
- Confirmed the immediate root cause in Go 1.26: `http.Client.Do` wraps a 101 response body with `cancelTimerBody` whenever `Client.Timeout` is non-zero; that wrapper exposes only `Read` and `Close`, hiding the underlying upgraded connection's `Write` and `CloseWrite` methods.
- Confirmed `NewDockerProvider` configures `client.Timeout = 2m`, so the earlier attached-Exec implementation deterministically failed its writable-stream assertion before writing configuration stdin.
- Confirmed the current worktree fix calls the configured `Transport.RoundTrip` for the upgrade request, retaining the actual HTTP 101 `readWriteCloserBody` while leaving ordinary Docker API requests on `client.Do`.
- Confirmed the error path is `agent.runtime.ensure` worker -> Infrastructure internal create command -> `Service.CreateRuntime` -> `DockerProvider.Ensure` -> `injectOpenCodeConfig` -> `execWithInput`; the provider error marks the Infrastructure runtime failed and is returned to the worker, so retries repeat the same deterministic failure.
- Confirmed the running `omnimam/infraserver:b90bf7d-amd64` image contains the writable-upgrade fix and has created a healthy OpenCode container; it predates the latest explicit timeout refinement.
- Verified the live container remains on a read-only rootfs, has writable config and gate tmpfs mounts, has a `0600` non-empty OpenCode config and a released startup gate, and does not expose common credential markers in container `Cmd` or `Env`. Configuration contents were not read.

## Current in-progress work

- None.

## Files added, modified, renamed, or removed

- Existing modified implementation: `backend/internal/infrastructure/providers/dockerruntime/docker.go`.
- Existing untracked focused tests: `backend/internal/infrastructure/providers/dockerruntime/docker_test.go`.
- Updated during this task: `docs/HANDOFF.md`.
- No files were renamed or removed.

## Key architectural or design decisions

- Keep the existing consumer/provider boundary and Unix-socket `net/http` transport; no Docker SDK dependency is needed.
- Use an attached, non-TTY, non-privileged Docker Exec to stream sensitive OpenCode configuration over stdin; do not place it in container/exec command metadata, environment variables, stdout/stderr, or returned transport errors.
- Preserve read-only rootfs, writable tmpfs mounts, dropped capabilities, `no-new-privileges`, and the startup gate. Touch the gate only after config write and `chmod 600` succeed.
- A raw transport call is necessary to preserve the writable 101 body, but it must receive an explicit deadline equivalent to the bypassed `http.Client.Timeout`.

## API, schema, dependency, or configuration changes

- No public API, schema, migration, dependency, SSOT, error-code, permission, event, environment-variable, or Compose changes.
- The existing internal change replaces Docker Archive injection with Docker Exec stdin injection.

## Verification performed and remaining checks

- Passed focused regression selection: `go test -count=1 -run 'TestExecWithInputPreservesWritableUpgradeWhenDockerClientHasTimeout|TestExecWithInputRejectsNonWritableUpgradeStream|TestEnsureDeletesContainerWhenConfigInjectionFails' ./backend/internal/infrastructure/providers/dockerruntime`.
- Passed the new timeout regression: `go test -count=1 -run '^TestExecWithInputHonorsDockerClientTimeout$' ./backend/internal/infrastructure/providers/dockerruntime`.
- Passed `go test -race -count=1 ./backend/internal/infrastructure/providers/dockerruntime`.
- Passed `go vet ./backend/internal/infrastructure/providers/dockerruntime`.
- Passed `go test -count=1 ./backend/internal/infrastructure` (package has no test files; compile succeeded).
- Passed final `git diff --check`.
- Live verification passed against the current infraserver image and newly created OpenCode runtime container.
- The latest timeout refinement is verified by deterministic unit/race tests but has not been rebuilt or deployed.

## Outstanding tasks

- If deployment is requested, rebuild and recreate only `omnimam-infraserver`, then create one fresh coding runtime and repeat the read-only rootfs/tmpfs/config-mode/start-gate checks without reading configuration contents.

## Known issues and risks

- The running infraserver image does not yet include the explicit timeout refinement added in this pass; repository tests cover it until a rebuild/deploy is authorized.
- Historical failed Runtime/AtomicTask attempts remain terminal; successful retries or new creates produce new runtime records as designed.

## Exact recommended next step

If live acceptance is requested, rebuild and redeploy only `omnimam-infraserver`, then create one fresh coding runtime and verify the timeout-refined implementation end to end.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
