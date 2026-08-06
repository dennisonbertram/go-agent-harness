# Plan: Issue #1215 harnessd lifecycle fixture stabilization

## Context

- Governing GitHub issue: #1215.
- Problem: three `cmd/harnessd` daemon tests use test-owned lifecycle inputs but
  still infer readiness from short, independent wall-clock waits. Under the
  aggregate race gate this can report a fixture failure while the target daemon
  is either still starting or has already failed for a useful causal reason.
- User impact: a red baseline blocks credible API, TUI, native GUI, cron, and
  callback convergence work despite no demonstrated product-runtime failure.
- Constraints: preserve the behavioral assertions; change test fixtures only;
  do not broaden production timing or hide an early daemon failure.

## Scope

- In scope: the cleaner lifecycle tests, invalid model-catalog startup fixture,
  test-local readiness helper, and required delivery records.
- Out of scope: `harnessd` runtime lifecycle, HTTP/API contracts, provider,
  cron/callback, persistence, TUI, GUI/macOS, and CI workflow policy.

## Documentation Contract

- Feature status: implemented locally; promotion remains a separate PR review
  and hosted-CI decision.
- Public docs affected: None; this changes no product behavior.
- Spec docs to update before code: this plan and the cross-surface impact map.
- Implementation notes to add after code: engineering, observational, system,
  and long-term-thinking logs plus their indexes.

## Test Plan (TDD)

- First failing test: migrate the invalid-catalog case to a not-yet-defined
  listener-aware matrix helper; expected red is a compile failure for that
  helper. This proves the test is no longer allowed to reserve-close-rebind or
  poll a guessed address directly.
- Existing tests to update: cleaner tests must wait for either their injected
  lifecycle channel or the owned daemon result, with a diagnostic deadline only
  after both causal paths remain unresolved.
- Regression tests required: focused normal/race stress of the three fixtures,
  `go test ./cmd/harnessd -race`, and `TMPDIR=/private/tmp
  ./scripts/test-regression.sh`.

## Cross-Surface Impact Map

- Required map: `2026-08-06-issue-1215-harnessd-fixtures-impact-map.md`.

## Implementation Checklist

- [x] Verify structured issue #1215, base worktree provenance, and current seam.
- [x] Record architecture/search evidence and cross-surface impact map.
- [x] Capture the expected compile-red for the listener-aware invalid-catalog test.
- [x] Implement only test-owned causal wait helpers and matrix routing.
- [x] Preserve cleaner cancellation/acknowledgement and invalid-catalog success assertions.
- [x] Update implementation logs and indexes.
- [x] Run focused normal/race, package race, and full regression gates.
- [ ] Commit, push, and open one PR with `Closes #1215`.

## Risks and Mitigations

- Risk: a longer generic timeout masks a real startup failure. Mitigation: race
  every lifecycle wait against the owned `runWithSignalsWithDeps` result and
  retain a diagnostic bound only for a true hang.
- Risk: converting the invalid-catalog case weakens its nonfatal assertion.
  Mitigation: retain the real daemon HTTP health probe and clean shutdown after
  the malformed catalog is loaded.
- Rollback: revert this isolated test-and-documentation PR; no data or runtime
  rollback is needed.
