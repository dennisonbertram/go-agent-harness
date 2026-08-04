# Cross-surface impact map — Issue #1149

## Task

- Task / issue: #1149 cron execution-history HTTP API.
- Plan link: `2026-08-04-issue-1149-cron-executions-api.md`.
- Owner: Codex.
- Status: implementing.

## Current Ownership, Callers, and Data Flow

- Entry point: `internal/server/http_cron.go` receives `/v1/cron/jobs/`.
- Source of truth: `Server.cronClient.ListExecutions`; remote
  `cronClientAdapter` and embedded `embeddedCronAdapter` already translate the
  corresponding scheduler/store results.
- Consumers: deferred `cron_history`, `cronctl`, and now public API clients.
- Search evidence: `rg -n "ListExecutions|cron_history|handleCronJobByID"
  internal cmd` found the server interface and both adapters, but no route.
- Conclusion: add at the server boundary; do not duplicate scheduler/store
  history logic.

## Config, API, CLI, and Tools

- No config/default/environment changes.
- Add authenticated read route `GET /v1/cron/jobs/{id}/executions` with
  `limit` and `offset`; output mirrors established cron history shape.
- No CLI/tool wire-format change; existing adapters are reused.
- Invalid pagination follows the existing cron server convention: defaults for
  invalid/non-positive limit and negative offset; limit is capped to prevent
  unbounded reads.

## Persistence and Compatibility

- No schema/migration/cache changes; `CronExecution` is existing durable data.
- Additive API compatible with old clients and binaries.
- Mixed versions: only upgraded harness HTTP servers expose the route; client
  can retain its existing tool/cron-daemon history path.

## Lifecycle, Security, and Reliability

- Read only; no scheduling, retry, or resource ownership change.
- Requires existing `runs:read` scope and authenticates through normal server
  middleware.
- The handler resolves and tenant-authorizes the job before querying history;
  foreign and missing IDs both return 404, preventing cross-tenant disclosure.
- Backend errors are 500 and no partial response is emitted.

## Product and Integration Surfaces

- Server/runtime: new read route only.
- TUI/web/macOS: no direct change; product clients can use the additive API in
  later status rendering work.
- Provider/model/tool catalog: None; search showed `cron_history` already
  uses the same interface.
- External automation/UX: no direct UI state; JSON uses existing execution
  fields including `run_id`, status, timestamps, and error summary.

## Deployment and Operations

- Deploy ordinary server binary; no migration/feature flag.
- Route failures use the existing structured `not_found`, `forbidden`, and
  `internal_error` HTTP contracts.
- Roll back by reverting handler code; no data repair/runbook change needed.

## Regression Tests

- First red: `TestCronExecutionHistory_ReturnsOwnedPaginatedExecutions` on the
  missing route.
- Acceptance: query forwarding/normalization, JSON output, auth scope,
  tenant isolation, missing ID, configured adapter failure, and nil client.
- Integration: existing remote and embedded adapter `ListExecutions` tests
  continue to prove both runtime adapters implement the server interface.
- Commands: `go test ./internal/server -run CronExecution -count=1`,
  `go test -race ./internal/server -count=1`, then
  `GOCACHE=/private/tmp/gocode-1149-gocache ./scripts/test-regression.sh`.

## Documentation and Handoff

- This plan and map define the API before implementation.
- Add durable engineering/observational/system/intent entries and maintain
  plan/log indexes after verification.
- No public docs until route and tests exist.

