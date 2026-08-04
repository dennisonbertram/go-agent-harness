# Plan: Issue #1156 MCP HTTP test transport isolation

## Context

- Governing GitHub issue: #1156.
- Problem: parallel `httptest.Server.Close` calls can close idle connections in
  the process-global default HTTP transport used by MCP test clients. A 401
  authentication response can consequently surface as an unrelated transport
  failure.
- User impact: CI can intermittently fail strict authentication classification
  tests, hiding whether an actual MCP authentication regression occurred.
- Constraints: retain production `dialHTTP` semantics; do not weaken typed
  authentication errors, retry failed requests, or mutate shared transports.

## Scope

- In scope: a test-owned HTTP transport factory, test-helper adoption, and
  deterministic ownership/configuration and global-cleanup coupling tests.
- Out of scope: production HTTP configuration, authentication behavior, retries,
  server behavior, API wire contracts, or client-visible changes.

## Documentation Contract

- Feature status: implemented after green verification.
- Public docs affected: none; this is an internal test-ownership repair.
- Implementation notes: capture source ownership, the standard-library cleanup
  boundary, and exact normal/race/full-regression evidence in the logs and plan
  index.

## Test Plan (TDD)

- New failing test first: `TestNewHTTPConnForTestOwnsTransport` requires the
  helper to create a non-default cloned transport that retains the relevant
  default configuration. `TestHTTPConnTestTransportIsolatedFromDefaultCleanup`
  uses a nonparallel global cleanup spy to prove `httptest.Server.Close` reaches
  the default transport while the prebuilt clone does not share it.
- Existing tests: preserve `TestHTTPConn_TypedAuthErrors` strict status/error
  assertions unchanged.
- Regression commands: focused normal and race ownership/auth tests, complete
  `go test ./internal/mcp -race`, then `./scripts/test-regression.sh`.

## Cross-Surface Impact Map

- See `2026-08-04-issue-1156-mcp-http-transport-impact-map.md`.

## Implementation Checklist

- [x] Read structured issue and search current HTTP client/test ownership.
- [x] Record plan and cross-surface impact map before code.
- [x] Add deterministic ownership/configuration and global-cleanup coupling
  regression tests.
- [x] Add the smallest test-only isolated transport factory and use it.
- [x] Preserve strict typed auth tests and run normal/race coverage.
- [x] Run complete MCP race package and repository regression gate.
- [x] Record logs/indexes and handoff evidence.

## Risks and Mitigations

- Risk: claiming that default idle-pool cleanup deterministically interrupts an
  active request would be false. Mitigation: assert the actual ownership and
  global-cleanup boundary deterministically, while retaining strict existing
  401/403 mapping tests.
- Risk: moving transport ownership into production changes pooling behavior.
  Mitigation: keep the factory in `_test.go`; `dialHTTP` remains untouched.
- Rollback: revert the test helper/factory and its regression test; no persisted
  state or runtime behavior is involved.
