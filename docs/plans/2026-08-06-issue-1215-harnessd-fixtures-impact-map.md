# Cross-Surface Impact Map: Issue #1215 harnessd fixture stabilization

## Task

- Task / issue: #1215 — stabilize cleaner and invalid-catalog daemon fixtures.
- Plan link: `2026-08-06-issue-1215-harnessd-fixtures-plan.md`.
- Owner: `cmd/harnessd/main_test.go` test helpers and three test cases.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `TestStartupFailureCancelsConversationCleaner`,
  `TestShutdownConversationCleanerCancellation`, and
  `TestRunWithSignalsInvalidModelCatalogContinues`.
- Source of truth: test-owned `heldConversationCleaner` lifecycle channels and
  `runDeps.listen`; `runWithSignalsWithDeps` remains the real startup path.
- Callers/consumers: normal, race, coverage, and GitHub fast `cmd/harnessd`
  test runs. No production consumer uses these fixture helpers.
- Similar abstractions searched: `runMatrixTestWithListenerAndHealthTimeout`,
  `awaitHealthyOrRunFailure`, `heldConversationCleaner`, and `runDeps.listen`.
- Conclusion: extend the existing test-only actual-listener/early-return seam;
  do not alter `main.go` lifecycle ownership.

## Config, API, CLI, and Tools

- User-facing config: None. The tests retain private environment maps only.
- Defaults/fallbacks: no application environment, endpoint, CLI, tool, or wire
  behavior changes.
- Errors/validation: malformed catalog remains nonfatal; actual daemon failure
  is reported immediately by the fixture rather than presented as a timeout.

## Persistence and Compatibility

- Schema/migration/cache/generated data: None. SQLite test files remain under
  `t.TempDir`.
- Compatibility/mixed rollout: None; no built artifact changes.

## Lifecycle, Security, and Reliability

- Concurrency/cancellation/cleanup: waits pair cleaner channel events with the
  daemon result; cleaner release/acknowledgement remains required before return.
- Security/privacy: test-only loopback listeners and temporary files; no
  credentials are logged or read outside fixed fake values.
- Failure/recovery: listener acquisition identifies the exact socket; health
  waits surface an early daemon error instead of retrying a guessed endpoint.

## Product and Integration Surfaces

- Server/runtime: None — `cmd/harnessd/main.go` is intentionally unchanged.
- TUI/web/macOS/other clients: None — no client code or rendered behavior.
- Provider/model/tool catalog/routing: invalid-catalog behavior is asserted but
  routing/catalog implementation remains unchanged.
- External systems/automation/UX: None.

## Deployment and Operations

- Deployment/migration/feature flags: None.
- Diagnostics: fixture failures become causal (actual listener or daemon result)
  rather than a bare short timeout.
- Rollback: revert isolated test/docs commit; no operator action.
- Runbooks: testing command remains unchanged.

## Regression Tests

- Characterization/first expected red: invalid-catalog test references a
  listener-aware helper before it exists, causing a compile error.
- Acceptance: malformed catalog reaches real daemon health then clean shutdown;
  cleaner startup/cancellation/acknowledgement behavior remains asserted.
- Edge/failure/lifecycle: occupied listener causes startup cancellation; normal
  shutdown waits for cleaner acknowledgement; early daemon return is surfaced.
- Integration/e2e: real loopback harnessd startup via injected listener,
  no direct manager call.
- Exact commands: focused normal/race repetitions, `go test ./cmd/harnessd
  -race`, and `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs before code: this plan/map only; no public docs change.
- Implementation logs/indexes after code: engineering, observational, system,
  long-term-thinking, plan, and logs indexes.
- Training/onboarding/release notes: None; no product contract changed.

## Warning Check

- Every surface is mapped. Unaffected product surfaces are test-only by
  `main_test.go` ownership and the search evidence above.
