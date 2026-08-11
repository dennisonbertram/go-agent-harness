# Plan: Provider API-key capture synchronization

## Context

- Governing GitHub issue: #1052
- Problem: `TestMatrix_ProviderAPIKeyCapture` waits for the complete HTTP server
  to become healthy within three seconds even though its contract is only to
  observe the provider factory input. Hosted race execution exceeded that
  unrelated deadline and blocked PR #1051.
- User impact: A false-negative CI gate prevents verified cron/callback repairs
  from reaching `main`.
- Constraints: Test-only change; do not raise a global timeout or change
  production startup behavior.

## Scope

- In scope: Direct synchronization on provider-factory invocation, bounded
  process shutdown, focused repeated normal/race coverage, and the full gate.
- Out of scope: HTTP health semantics, production startup sequencing, and
  global test timeout policy.

## Documentation Contract

- Feature status: `implemented`
- Public docs affected: None; this is test infrastructure only.
- Spec docs to update before code: This plan and its impact map.
- Implementation notes to add after code: Engineering and long-term-thinking
  logs, plus final verification evidence.

## Test Plan (TDD)

- New failing tests to add first: Replace the HTTP readiness dependency with a
  provider-invocation signal; the initial test edit must fail until the signal
  is emitted from the factory seam.
- Existing tests to update: `TestMatrix_ProviderAPIKeyCapture`.
- Regression tests required: Repeated focused normal and race runs, the whole
  `cmd/harnessd` package in normal/race mode, and
  `./scripts/test-regression.sh`.

## Cross-Surface Impact Map

- See `2026-07-30-issue-1052-provider-key-capture-sync-impact-map.md`.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Link a contract-complete structured GitHub issue before implementation.
- [x] Record current architecture, callers, consumers, and source-of-truth search evidence.
- [x] Document feature status and exact contract before code.
- [x] Complete and reconcile the cross-surface impact map before implementation.
- [x] Add characterization coverage before structural refactors.
- [x] Write failing tests first.
- [x] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries.
- [x] Implement minimal code changes.
- [x] Refactor while tests remain green.
- [x] Update docs, status ledgers, and indexes.
- [x] Update engineering/system/observational logs as needed.
- [x] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: Signaling too early could leave `runWithSignals` blocked or leak a
  server goroutine.
- Mitigation: Keep the bounded interrupt/shutdown assertion and run the focused
  test repeatedly under the race detector.

## Verification

- Expected red: the new direct provider signal timed out before the factory
  emitted it.
- Focused normal and race tests each passed 100 consecutive runs.
- The complete `cmd/harnessd` package passed in normal and race modes.
- `./scripts/test-regression.sh` passed normal, race, and coverage with 85.6%
  total coverage and zero uncovered functions.
