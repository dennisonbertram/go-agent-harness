# Plan: Issue #1190 production MCP HTTP transport ownership

## Context

- Governing GitHub issue: #1190.
- Problem: production `dialHTTP` leaves `http.Client.Transport` nil. It therefore
  shares `http.DefaultTransport`; an unrelated `httptest.Server.Close` can close
  that global pool during an authentication request and turn an expected strict
  401 into a transport cancellation.
- User impact: an MCP authentication failure becomes nondeterministic and can
  mask the actionable `ErrUnauthorized` classification used by ClientManager.
- Constraints: one owned clone per production connection; close only that
  connection's idle pool; retain 30-second timeout, authorization precedence,
  and existing no-retry/error propagation behavior.

## Scope

- In scope: a production-owned default-transport clone factory, `dialHTTP` and
  test-helper adoption, idempotent connection-local idle-pool closure, and
  deterministic production-path authorization regression coverage.
- Out of scope: MCP protocol/API/configuration changes, retries, token changes,
  persistence, TUI/GUI behavior, and transport tuning.

## Documentation Contract

- Feature status: implementation after green verification.
- Public docs affected: none; transport ownership is internal.
- Implementation notes: plans/logs record production ownership, no global
  cleanup coupling, test evidence, and rollback.

## Test Plan (TDD)

- First red test: a nonparallel production `dialHTTP` authentication request
  blocks at its dial gate; an unrelated `httptest.Server.Close` closes the
  global idle pool; releasing the request must yield strict `ErrUnauthorized`,
  not a transport error.
- Characterization control: the exact historic nil-transport construction
  routes its held request through a cleanup-cancelling global transport; an
  unrelated `httptest.Server.Close` deterministically produces a non-auth
  transport cancellation before a 401 can be returned.
- Additional regression: production and test connections own distinct cloned
  transports; idempotent `Close` releases only its own idle pool and cannot
  close a sibling or global transport.
- Existing coverage: retain ClientManager explicit-token-provider precedence.
- Commands: focused normal/race stress, `go test ./internal/mcp -race`, and
  `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Cross-Surface Impact Map

- See `2026-08-05-issue-1190-mcp-owned-transport-impact-map.md`.

## Implementation Checklist

- [x] Read issue, ownership/runbook requirements, and prior #1156 boundary.
- [x] Search current production/test construction and ClientManager callers.
- [ ] Record expected red test result.
- [ ] Add cloned production factory and connection-local close.
- [ ] Prove focused normal/race, MCP race package, and canonical full gate.
- [ ] Update logs/indexes and PR handoff with exact evidence.

## Risks and Mitigations

- Risk: transport clone silently changes configured defaults. Mitigation: clone
  the current `http.DefaultTransport` configuration and assert ownership.
- Risk: closing a connection affects another pool. Mitigation: use the owned
  client only, make close idempotent, and prove sibling/global isolation.
- Rollback: revert factory, close behavior, tests, and internal docs. No
  persisted state, migration, or wire-contract rollback is required.
