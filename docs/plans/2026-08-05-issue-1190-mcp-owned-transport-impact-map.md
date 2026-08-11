# Impact Map: Issue #1190 production MCP HTTP transport ownership

## Task

- Task / issue: #1190 production `dialHTTP` owned transport.
- Plan link: `2026-08-05-issue-1190-mcp-owned-transport-plan.md`.
- Owner: agent implementation slice.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `dialHTTP` in `internal/mcp/http_conn.go`; `ClientManager`
  config registration reaches it through the HTTP connection factory.
- Owner/source: `httpConn.client`; a nil transport delegates to the mutable
  process-global `http.DefaultTransport`. `NewHTTPConnForTest` separately
  cloned it after #1156.
- Consumers: initialize, tool/resource calls, token provider headers, strict
  401/403 mapping, `ClientManager.DiscoverTools`, and test constructors.
- Search evidence: `rg -n 'dialHTTP|NewHTTPConnForTest|httpConn|CloseIdleConnections|ClientManager|SetTokenProvider' internal/mcp cmd/harnessd`.
- Conclusion: move the shared factory into production code so both paths own a
  clone; no caller can own or close a sibling pool.

## Config, API, CLI, and Tools

- User-facing config/defaults/environment/API/CLI/tool wire changes: None.
- Existing default settings: clone preserves default transport configuration;
  timeout remains 30 seconds.
- Error contract: strict `ErrUnauthorized`, `ErrForbidden`, and all ordinary
  transport errors remain unmasked; no retry is introduced.

## Persistence and Compatibility

- Persistence/schema/cache: None; transports are process-local.
- Compatibility: MCP JSON-RPC and ClientManager provider precedence are
  unchanged; rollout is source-only and compatible across mixed binaries.

## Lifecycle, Security, and Reliability

- Lifecycle: each `httpConn` owns one cloned pool. `Close` is idempotent and
  closes only its owned idle pool.
- Security: auth headers, token-provider precedence, and sentinel error
  classification remain unchanged; credentials are neither logged nor copied.
- Failure/recovery: unrelated global cleanup cannot cancel an owned in-flight
  request; real dial/auth errors remain observable without masking.

## Product and Integration Surfaces

- Server/runtime: harnessd's configured HTTP MCP clients receive isolated
  pools through existing ClientManager wiring.
- TUI/web/macOS: None directly; they retain current server-provided MCP tools.
- Provider/model/tool routing/external automation/UX: None; no catalog or
  endpoint behavior changes.

## Deployment and Operations

- Deployment/migration/flags: None.
- Diagnostics: existing precise HTTP/auth errors remain intact; issue and
  durable logs document the ownership boundary.
- Rollback: revert this slice; no data recovery/runbook action is needed.

## Regression Tests

- Characterization/red: nonparallel gated production auth dial plus unrelated
  `httptest.Server.Close` must return `ErrUnauthorized`, not cancellation.
- Historic control: an explicit nil-transport connection must demonstrate that
  unrelated global cleanup reaches and cancels its held request, producing a
  non-`ErrUnauthorized` transport error.
- New acceptance: owned clone and idempotent close isolate sibling/global
  pools; test helper reuses factory; ClientManager explicit provider still wins.
- Exact commands: focused normal/race stress, `go test ./internal/mcp -race`,
  and `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Documentation and Handoff

- Public specs/release notes: None.
- Internal plan/impact, plan/log indexes, and durable logs record the result;
  PR includes the red command, green commands, scope, and rollback.
