# Project Handoff

## Current goal and status

Fix API Server startup failure:
`MCP public base URL and allowed origins must be absolute HTTP origins`.

Status: MCP implementation and startup fix complete, verified locally, and
committed as one cohesive change. The repository and clean
`ssot/` submodule are pinned to released tag `spec-v1.9.2` commit
`32ae994256beb84a59e2e5c9fcc00c2f6d029756`, matching `SSOT_VERSION`.

Full SSOT completion is still gated by upstream facts absent from this server:
a shared quota/cost service and durable Identity role-permission/audit
boundaries. Consumer-owned MCP interfaces exist, but the production composition
currently uses bounded local admission and structured logging adapters.

## Work completed in this session

- Re-read repository/backend rules, `skills/omnimam-server-backend/SKILL.md`,
  and applicable Go troubleshooting, safety, viper/cobra, and testing skills.
- Re-ran the MCP-focused race/unit tests, API Server integration package tests,
  both binary builds, Compose rendering, and patch-format checks immediately
  before committing the complete MCP change set.
- Confirmed `git submodule status ssot` reports
  `32ae994256beb84a59e2e5c9fcc00c2f6d029756 ssot (spec-v1.9.2)` and
  `SSOT_VERSION.commit` matches with `status: released`.
- Root-caused the startup failure to `deployments/docker-compose.yaml`:
  `/tmp/apiserver.yaml` was rendered without setting or replacing
  `APISERVER_MCP_PUBLIC_BASE_URL`, leaving the literal placeholder in
  `mcp.public-base-url`.
- Added `APISERVER_MCP_PUBLIC_BASE_URL` to the API Server compose environment
  and render substitution list. The default is
  `http://127.0.0.1:${OMNIMAM_APISERVER_PORT:-8080}`, with
  `OMNIMAM_MCP_PUBLIC_BASE_URL` available for explicit production HTTPS
  origins.
- Re-read repository/backend rules, the MCP backend skill, and applicable Go
  security, safety, testing, concurrency, context, and database guidance.
- Revalidated the SSOT release gate and preserved the existing dirty MCP work.
- Hardened extensible `_meta` decoding while rejecting null known objects.
- Enforced HTTPS for every non-loopback proxy endpoint, public link Origin, and
  allowed browser Origin; redirect following remains disabled.
- Added bounded per-Principal request and per-Principal/Tool token buckets plus
  pre-create single-upload size admission.
- Added startup and hourly physical cleanup of expired `McpTaskBinding` rows.
- Completed audit correlation for Resource URI, MCP Task/ApplicationRun IDs,
  and Tool business failures; sanitized log fields remain token/payload-free.
- Preserved released MCP business errors inside Tool `isError=true` results
  and mapped unknown internal source failures to
  `ERR_MCP_TOOL_RESULT_INVALID` without returning internal descriptions.
- Tightened published Tool output schemas for nested AtomicTask,
  Representation, error causes, and controlled upload URL shape; added a
  regression test rejecting undeclared nested projection fields.
- Added the Chinese MCP operator/security guide.
- Regenerated error code code/docs and API deepcopy code.

## Current in-progress work

No implementation command is currently running. The MCP implementation,
compose startup fix, and requested commit are complete.

## Files added, modified, renamed, or removed

Added:

- `backend/apis/iapiserver/meta_mcp.go`
- `backend/cmd/omnimam-mcp-proxy/main.go`
- `backend/internal/apiserver/controller/v1/mcp/controller.go`
- `backend/internal/apiserver/service/v1/mcp/{admission,cursor,errors,projection,projection_service,resources,service,tasks,tools}.go`
- `backend/internal/apiserver/store/mcp.go`
- `backend/internal/apiserver/store/postgresql/mcp.go`
- `backend/pkg/mcp/{arguments,catalog,processor,types,validation}.go`
- `backend/pkg/mcp/{output_contract,processor}_test.go`
- `backend/pkg/mcpproxy/{proxy,proxy_test}.go`
- `docs/guide/zh-CN/architecture/mcp-server.md`

Modified:

- `SSOT_VERSION` and the `ssot` gitlink
- `deployments/docker-compose.yaml`
- `backend/apis/iapiserver/deepcopy_generated.go`
- `backend/apis/iapiserver/request_application_platform.go`
- `backend/internal/apiserver/middleware/authentication.go`
- `backend/internal/apiserver/options/options.go`
- `backend/internal/apiserver/{route,server}.go`
- `backend/internal/apiserver/service/v1/assetlibrary/service.go`
- `backend/internal/apiserver/service/v1/modelgateway/registry.go`
- `backend/internal/apiserver/service/v1/platform/service.go`
- `backend/internal/apiserver/store/postgresql/{0_pg,application_platform}.go`
- `backend/internal/pkg/code/{base,code_generated}.go`
- `configs/apiserver.yaml`
- `docs/guide/zh-CN/api/error_code_generated.md`
- `scripts/install/environment.sh`
- `docs/HANDOFF.md`

Renamed or removed: none.

## Key architectural or design decisions

- MCP HTTP is installed in the existing stateless API Server; the separately
  requested stdio binary only translates newline-delimited stdio to HTTP.
- MCP consumes source domains through consumer-owned interfaces and owns only
  `McpTaskBinding`; it never reads their private tables.
- Tool business failures use `isError=true`; protocol/Resource/Task failures
  use JSON-RPC errors. Successful structured Tool output is validated against
  compiled Draft 2020-12 schemas before response.
- Credential-bearing links use normalized configured origins, never the
  request `Host` header. Remote HTTP is rejected.
- Identity JWT parsing and object visibility reuse existing boundaries.
  `Authorizer`, `Auditor`, and `AdmissionController` remain injectable so
  future shared services do not require Tool/service rewrites.

## API, schema, dependency, or configuration changes

- SSOT pin changed from released `spec-v1.9.1` to released
  `spec-v1.9.2`.
- Added `POST /mcp` and fixed 11 Tools, 6 Resource templates, and Tasks
  extension handling.
- Added runtime `mcp_task_bindings` creation/constraints/indexes matching the
  released design schema; no additional MCP table or domain event was added.
- Added `mcp` API Server flags/YAML configuration and
  `APISERVER_MCP_PUBLIC_BASE_URL`.
- Added the explicitly requested `omnimam-mcp-proxy` binary.
- No new Go module dependency was introduced by this session.

## Verification performed and remaining checks

Passed:

- Manual render of `configs/apiserver.yaml` with the compose substitution list
  now produces `mcp.public-base-url: http://127.0.0.1:8080`.
- `docker compose -f deployments/docker-compose.yaml config`
- Confirmed compose-expanded environment contains
  `APISERVER_MCP_PUBLIC_BASE_URL: http://127.0.0.1:8080`.
- `go test ./backend/internal/apiserver/options`
- `go build ./backend/cmd/apiserver` (removed generated root `apiserver`
  binary afterward).
- Rendered a temporary API Server config and ran
  `timeout 8s go run ./backend/cmd/apiserver -c <temp config>`; the previous
  MCP origin error did not recur, `POST /mcp` was registered, and `/healthz`
  became healthy before timeout stopped the server.
- `make gen`
- `make gen.deepcopy` (completed with existing unsupported-alias warnings)
- focused MCP protocol/proxy/controller/service/PostgreSQL tests
- `go test -race ./backend/pkg/mcp ./backend/pkg/mcpproxy`
- scoped `go vet`
- API Server and stdio proxy builds
- `git diff --check` and changed-Go-file `gofmt` check
- live API Server health, missing/invalid JWT business error, illegal Origin
  HTTP 403, and stdio request-ID/one-line JSON-RPC smoke tests

Repository baselines:

- `go test ./backend/...` reaches all packages and fails only the existing
  task-name localization expectations: `thumbnail视图` versus
  `thumbnail 表现形式` in taskcenter/taskname tests.
- `make lint` fails only existing `cancelled` misspell findings in
  `backend/internal/apiserver/store/postgresql/asset_upload_contract.go`.

Remaining checks:

- Run a successful authenticated live `server/discover` and one read-only
  Tool after a real Identity user/JWT fixture is available. The current dev
  database contained no persisted users, so smoke testing stopped at the
  authentication boundary without creating test identity data.
- Re-run focused checks after any quota or Identity adapter is added.

## Outstanding tasks

1. Supply or explicitly defer SSOT-aligned shared facts/adapters for concurrent
   ApplicationRun, daily runs, cost ceiling, and total storage quota. Current
   local admission covers request rate, Tool rate, and single-upload bytes.
2. Supply or explicitly defer durable Identity role-permission, token
   revocation, and audit-write boundaries. Current defaults authenticate each
   request, reuse source object visibility, recognize released permissions, and
   emit sanitized structured audit logs.
3. Perform authenticated success-path live smoke testing once Identity data is
   available.

## Known issues and risks

- The released S1 quota ordering requires idempotency before quota admission.
  Application Platform owns idempotency, but exposes no read-only preflight
  boundary; MCP must not inspect its private store or invent a second table.
- Shared cost and total-storage facts have no current consumer contract.
- Identity S2 has no durable role-permission or audit persistence contract in
  this repository. The default adapters are intentionally replaceable and
  should not be mistaken for those missing facts.
- The stdio binary is under `backend/cmd/`; prior explicit MCP Server work was
  treated as permission to add this required binary.
- The live smoke server used the normal development PostgreSQL schema bootstrap
  and was stopped afterward. Its temporary copied configuration was moved to
  the system trash and is recoverable until trash cleanup.

## Exact recommended next step

Run a successful authenticated live `server/discover` and one read-only Tool
when an Identity user/JWT fixture is available. Then address or explicitly
defer the remaining shared quota and durable Identity adapter gaps.

Next Prompt:

Read docs/HANDOFF.md, verify the current implementation, and continue with the next outstanding task. Do not repeat completed work.
