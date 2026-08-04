# Plan: Issue #1152 harnessd race-stable test fixtures

## Context

- Governing GitHub issue: #1152.
- Problem: PR #1151's hosted race lane exposed five unrelated `cmd/harnessd`
  lifecycle fixtures that start default callbacks and use sleep-based startup
  assumptions. The #1150 default callback run-store migration makes those
  unowned assumptions sensitive to aggregate I/O contention.
- User impact: a red race baseline prevents safe scheduler delivery even when
  its production behavior is correct.
- Constraints: tests/readiness wiring only; retain #1150's default callback
  durability and its dedicated callback-enabled coverage.

## Scope

- In scope: explicitly disable callbacks in the five non-callback fixtures;
  replace their sleeps with causal provider/listener readiness; bounded
  diagnostic timeouts; stress proof and logs.
- Out of scope: production callback, persistence, API, TUI, GUI, scheduler,
  authentication, or timeout-policy changes.

## Documentation Contract

- Feature status: implemented after verification.
- Public docs affected: none; this is internal test-fixture reliability work.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: durable logs and indexes.

## Test Plan (TDD)

- New failing tests to add first: a shared non-callback fixture/environment
  contract requiring `HARNESS_ENABLE_CALLBACKS=false`, plus signal barriers
  for each affected test before it sends shutdown.
- Existing tests to update: `TestStartupFailureCancelsConversationCleaner`,
  `TestShutdownConversationCleanerCancellation`,
  `TestShutdownCronOrderingDeterministic`,
  `TestLookupModelAPIWiredInRunWithSignals`, and
  `TestLookupModelAPIWithAlias`.
- Regression tests required: targeted normal/race stress of those five;
  complete `cmd/harnessd` race; full repository regression.

## Cross-Surface Impact Map

- See `2026-08-04-issue-1152-harnessd-race-stability-impact-map.md`.

## Implementation Checklist

- [x] Define acceptance criteria in tests and verify #1152.
- [x] Record current architecture/search evidence and complete impact map.
- [x] Capture the intended red test failure.
- [x] Isolate unrelated fixtures from callback bootstrap and make startup
  readiness causal.
- [x] Run targeted normal/race stress and complete `cmd/harnessd` race.
- [ ] Complete full regression: blocked by pre-existing zero coverage in
  `internal/server/cron_run_idempotency.go:266 waitForCronRunDispatch`, outside
  this test-only issue; no failure may be waived.
- [x] Update logs and indexes with implementation evidence.
- [x] Hand off a reviewable, uncommitted worktree to the parent agent.

## Risks and Mitigations

- Risk: accidentally masking a callback regression. Mitigation: scope the
  opt-out to the five listed non-callback fixtures; retain the existing
  callback-enabled matrix/shutdown coverage unchanged.
- Risk: a longer timeout masks a real lifecycle fault. Mitigation: no timeout
  extension; wait for authoritative provider/cleaner/listener signals and keep
  bounded diagnostics only.
