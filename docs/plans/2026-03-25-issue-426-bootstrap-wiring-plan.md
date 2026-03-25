# Plan: Issue #426 harnessd bootstrap wiring

## Context

- Problem: `cmd/harnessd/main.go` still assembles provider/catalog startup, persistence, cron, webhook adapters, and server wiring in one large `runWithSignals(...)` flow.
- User impact: Startup wiring is harder to review and extend safely because unrelated bootstrap concerns are interleaved in one function.
- Constraints: Preserve existing startup and shutdown behavior, follow strict TDD, and keep the extraction scoped to bootstrap composition rather than changing runtime features.

## Scope

- In scope:
  - Extract focused helpers for `harnessd` bootstrap assembly.
  - Add failing-first tests around the extracted seams.
  - Keep `runWithSignals(...)` as orchestration glue instead of detailed subsystem assembly.
- Out of scope:
  - Provider/model behavior changes.
  - HTTP API contract changes.
  - Runner loop redesign.

## Test Plan (TDD)

- New failing tests to add first:
  - Webhook/trigger bootstrap helper coverage for secret-driven validator and adapter registration.
  - Persistence/bootstrap helper coverage for run store, conversation store, and retention-cleaner wiring.
- Existing tests to update:
  - `cmd/harnessd/main_test.go` assertions that interact with the extracted seams.
- Regression tests required:
  - `go test ./cmd/harnessd -count=1`
  - `./scripts/test-regression.sh`

## Cross-Surface Impact Map

- None. This task is bootstrap decomposition only and does not change provider/model flow contracts, TUI state, or API behavior.

## Implementation Checklist

- [ ] Define acceptance criteria in tests.
- [ ] For provider/model flow work, add or update the one-page impact map before implementation.
- [ ] Write failing tests first.
- [ ] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries.
- [ ] Implement minimal code changes.
- [ ] Refactor while tests remain green.
- [ ] Update docs and indexes.
- [ ] Update engineering/system/observational logs as needed.
- [ ] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: Refactoring bootstrap assembly could subtly change which optional subsystems start under a given env/config combination.
- Mitigation: Extract only well-bounded helpers and pin the env-driven seams with direct tests before moving code.
