# Impact Map: Issue #1156 MCP HTTP test transport isolation

## Task

- Task / issue: #1156 test-owned MCP HTTP transports.
- Plan link: `2026-08-04-issue-1156-mcp-http-transport-plan.md`.
- Owner: agent implementation slice.
- Status: implementation verified; awaiting PR review and merge.

## Current Ownership, Callers, and Data Flow

- Entry points: `NewHTTPConnForTest` in `internal/mcp/export_test.go`; external
  package tests construct `httpConn` through it. Production enters through
  `dialHTTP` in `internal/mcp/http_conn.go`.
- Ownership: a nil `http.Client.Transport` delegates to shared
  `http.DefaultTransport`; `httptest.Server.Close` can disturb its idle pool.
- Consumers: HTTP initialize, tools, resources, token/header, and typed-auth
  tests. Search: `rg -n "HTTPConn|httpConn|DefaultTransport|httptest"
  internal/mcp` found only the test constructor as the shared-client seam.
- Conclusion: the isolated clone belongs in test-only construction; production
  pooling/timeout ownership remains `dialHTTP`.

## Config, API, CLI, and Tools

- User-facing config/defaults/environment/API/CLI/tool wires: None; source
  search confirms this changes test fixture construction only.
- Error contract: 401 remains `ErrUnauthorized`, 403 remains `ErrForbidden`;
  other non-2xx responses remain generic. No retry or masking is added.

## Persistence and Compatibility

- Persistence/schema/cache: None. Transport instances are process-local test
  resources.
- Compatibility: production client semantics and external MCP JSON-RPC wire
  behavior are unchanged.

## Lifecycle, Security, and Reliability

- Lifecycle: each test `http.Client` owns a clone of default transport settings
  and its idle pool; test teardown cannot close another test's pool.
- Security: authentication headers and typed authorization classification stay
  strict; no credentials are logged or altered.
- Failure/recovery: transport failures remain errors; ownership tests prevent
  test fixtures from sharing the process-global idle-pool cleanup target.

## Product and Integration Surfaces

- Server/runtime/TUI/web/macOS/provider routing/external automation/UX: None,
  with rationale: no production code or external interface changes.
- Internal MCP HTTP tests: affected through `NewHTTPConnForTest` only.

## Deployment and Operations

- Deployment/migration/flags: None; test code ships with normal source.
- Observability: issue, plan, and logs record the shared-transport ownership
  boundary and standard-library cleanup behavior to make future CI errors
  diagnosable.
- Rollback: revert test-only factory/tests; no runtime recovery required.

## Regression Tests

- Characterization/red: test helper has a nonnil cloned transport retaining
  default configuration; a cleanup spy proves the legacy nil-transport path
  reaches the mutable global while the clone does not share it.
- Negative/lifecycle: preserve the existing exact 401/403 sentinel and generic
  error checks.
- Commands: focused normal/race ownership and auth coverage, complete
  `internal/mcp` race, and full regression script.

## Documentation and Handoff

- No public spec/release note; plan, impact map, engineering/observational/
  system logs and `docs/plans/INDEX.md` are updated in this slice.
- Handoff includes exact test commands and no-production-behavior boundary.
