# Project Handoff

## Current project goal

Complete canonical video thumbnail generation for `AssetVersion / AssetRepresentation` on released `spec-v1.7.2`. The implementation must keep media execution replaceable through a consumer-owned `FFmpegRuntime`, preserve Task Center/Conductor as the only scheduling path, and repair existing image/video versions through the scheduled Representation backfill.

## Completed in this session

1. Added `RepresentationPolicy`. Image and video now expect `original/default + thumbnail/list-320`; other media retain the existing original-only policy. New image/video versions start with `expected_count=2` and `thumbnail_status=pending`, and their outbox event includes the thumbnail request.
2. Added `ThumbnailGenerator` as the media strategy boundary. The image implementation retains the Go decoder/scaler; the video implementation owns `list-320` request/result validation and delegates frame extraction to `FFmpegRuntime`.
3. Added the consumer-owned `FFmpegRuntime` interface and `LocalFFmpegRuntime` adapter. The adapter resolves `ffmpeg` at TaskWorker startup, uses `exec.CommandContext` without a shell, writes the verified input stream to a temporary file, extracts a representative PNG frame with `thumbnail=100`, limits the longest side to 320, and applies timeout, output, stderr, and concurrency bounds.
4. Injected the runtime and thumbnail generators explicitly from the TaskWorker composition root. Missing FFmpeg now prevents TaskWorker startup; API Server does not gain a media-tool dependency.
5. Added `media_type` to Representation generation tasks and mapped Task Center retry policy into Conductor inline task definitions. Generation uses three attempts, a 5-second initial delay, exponential backoff, and a 30-second policy cap.
6. Added the Worker-only `CompleteRepresentationGeneration` transaction. It supports pending/failed repair, same-Blob idempotency, different-Blob conflict detection, Blob availability checks, error/retry cleanup on success, and atomic AssetVersion/thumbnail projection updates.
7. Added `asset-library.representation-backfill` as the daily `03:30 UTC` SYSTEM RECONCILE schedule. It scans by stable AssetVersion ID checkpoint, caps each run at 1000 scanned versions and 100 repair actions, reuses the generate executor with stable idempotency keys, updates legacy video expected counts, and records irreparable source failures.
8. Restricted thumbnail projection updates to the Asset's current version so historical repairs cannot overwrite the current-version list state.
9. Added the pinned Ubuntu 24.04 FFmpeg package to the TaskWorker image only. The Dockerfile exposes `FFMPEG_VERSION` and `UBUNTU_MIRROR` build arguments.
10. Updated the Asset Library content-model documentation and TaskWorker/API Server collaboration documentation.

## Files added

- `backend/internal/apiserver/service/v1/assetlibrary/representation_policy.go`
- `backend/internal/apiserver/service/v1/assetlibrary/thumbnail_generator.go`
- `backend/internal/apiserver/service/v1/assetlibrary/ffmpeg_runtime.go`
- `backend/internal/apiserver/service/v1/assetlibrary/representation_backfill.go`
- `backend/internal/apiserver/service/v1/assetlibrary/ffmpeg_runtime_test.go`
- `backend/internal/apiserver/service/v1/assetlibrary/thumbnail_generator_test.go`
- `backend/internal/apiserver/service/v1/assetlibrary/representation_backfill_test.go`
- `backend/internal/apiserver/store/postgresql/asset_representation_backfill.go`

## Files modified

- Asset Library service/executor/store contracts and PostgreSQL lifecycle/upload implementations under `backend/internal/apiserver/`.
- TaskWorker composition, schedule registration, DAG generation, Task Center retry mapping, and Conductor runtime mapping/tests.
- `build/docker/taskworker/Dockerfile.build`.
- `docs/guide/zh-CN/architecture/asset-library-content-model.md`.
- `docs/guide/zh-CN/architecture/taskworker-apiserver-collaboration.md`.
- This handoff.

No files were removed. The pre-existing untracked `docs/guide/zh-CN/architecture/asset-library-server-bulk-import.md` was not changed for this task.

## Key architectural decisions

- `RepresentationPolicy` belongs to Asset Library and decides the expected set. Task Center only schedules the supplied plan.
- `ThumbnailGenerator` is the image/video strategy boundary. `FFmpegRuntime` is the narrower execution boundary consumed by the video generator.
- `LocalFFmpegRuntime` is an adapter, not business logic. A future remote or sidecar implementation only needs to satisfy the same interface and be replaced at TaskWorker bootstrap.
- Runtime dependencies use explicit constructor injection. No global registry, `init()` registration, service locator, shell invocation, binary path, command argument, or temporary path leaks into the executor/generator contract.
- Video thumbnails are optional PNG Representations. Final failure keeps the original usable and projects the version as `ready_with_warnings`.
- Backfill is scheduled through the released Task Center SYSTEM RECONCILE model; no startup-wide scan or second scheduler was introduced.
- The legacy `assets/asset_thumbnails` path remains separate and is not used by new AssetVersion work.

## API, schema, and configuration changes

- No public API, database schema, migration, permission code, event type, or public error code changed.
- Internal Go contracts gained `RepresentationPolicy`, `ThumbnailGenerator`, `FFmpegRuntime`, Representation plan/mutation/backfill DTOs, and workflow runtime retry mapping.
- TaskWorker image configuration gained pinned `FFMPEG_VERSION=7:6.1.1-3ubuntu5` and a configurable Ubuntu mirror build argument. API Server remains unchanged.
- `ssot/`, its pinned commit, and `SSOT_VERSION` were not changed. Released `spec-v1.7.2` already defines the Representation policy/backfill semantics and required error codes, so no SSOT release is required.

## Verification

- `go test ./backend/...` passed after the implementation.
- `go vet ./backend/...` passed after the implementation.
- Focused Asset Library, WorkflowRuntime, and Task Center package tests passed.
- The host-FFmpeg representative-frame test passed during the earlier implementation run.
- `git diff --check` passed before documentation refresh.
- Per the latest user instruction, no further integration, PostgreSQL, Conductor, container-runtime, deployment, browser, or end-to-end tests are to be run.
- The TaskWorker image build is not verified: dependency download from the configured mirror exceeded the 600-second build timeout after reaching FFmpeg package resolution.

## Remaining work

1. Validate the TaskWorker image in the target build environment when integration/build verification is explicitly resumed.
2. Deploy the updated TaskWorker and allow the daily backfill, or an explicitly authorized equivalent repair action, to generate thumbnails for existing videos such as `堕落天使.mp4`.
3. Continue the previously outstanding Workflow Canvas OutputBinding event consumer and persisted repair loop after this feature is accepted.

## Known issues and risks

- The running environment is not changed by this workspace implementation. Existing video rows will continue to show the media-type placeholder until the new TaskWorker is built/deployed and backfill runs successfully.
- The TaskWorker image build remains unverified because package download timed out; this is an environment/mirror verification gap, not a completed image artifact.
- No live PostgreSQL/Conductor restart recovery or browser rendering verification was performed for this change, as requested.
- The current scope generates thumbnails only. Playback, preview, poster, and additional video metadata remain out of scope.
- FFmpeg is a mandatory TaskWorker startup dependency for the current local adapter. Replacing it with a remote/sidecar runtime requires a new adapter and bootstrap selection, not changes to Representation execution or persistence.

## Recommended next task

When build/integration verification is authorized again, build the TaskWorker image in the target environment, deploy it, verify the scheduled backfill on an existing video, and confirm the resulting ready `thumbnail/list-320` Representation through the Asset Library UI/API.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
