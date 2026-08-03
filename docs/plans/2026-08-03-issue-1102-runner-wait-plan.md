# Plan: Issue #1102 deterministic AskUser wait observation

## Context

- Governing GitHub issue: #1102.
- Problem: `TestRunnerAskUserQuestionWaitsAndResumes` treats broker pending-readiness as proof that the asynchronous waiting status/event transition is complete. Under race scheduling it can therefore observe `running`.
- User impact: none to the public runtime contract; this baseline test can falsely block safe promotion of unrelated work.
- Constraints: preserve pending-before-notification semantics, lifecycle event order, and all runner behavior; no sleeps or weakened assertion.

## Scope

- In scope: one deterministic runner test synchronization boundary, its documented architecture/search evidence, and regression evidence.
- Out of scope: runner lifecycle behavior, broker API/persistence, cron/callbacks, server routes, TUI/macOS clients, and unrelated test cleanup.

## Documentation Contract

- Feature status: implemented and post-rebase foreground-regression verified; PR #1104/hosted review remains required before promotion.
- Public docs affected: none; this changes no shipped behavior.
- Implementation notes: add a bug-fix log entry and indexes after the red-green loop.

## Test Plan (TDD)

- New failing test first: replace the pending-input polling wait with an event subscription boundary and show the old source fails to compile until the helper exists/uses event evidence; the runtime baseline under `-race -count=20` is the observed red evidence from hosted #1101.
- Existing tests: retain the exact waiting status assertion and full event-order assertion.
- Regression tests: exact race x20, affected harness normal/race suite, and `./scripts/test-regression.sh`.

## Cross-Surface Impact Map

- See `2026-08-03-issue-1102-runner-wait-impact-map.md`.

## Implementation Checklist

- [x] Verify contract-complete issue #1102 and current ownership/search evidence.
- [x] Record plan and impact map before code.
- [x] Capture exact red evidence: hosted #1101 race observed `running`; local old test passed 500 repetitions, confirming scheduling sensitivity rather than reproducibility by delay.
- [x] Add deterministic event-boundary regression before production edits.
- [x] Keep production behavior unchanged; the event/status implementation already preserves the required ordering.
- [x] Run focused normal/race.
- [x] Run foreground full regression: normal, race, coverage, 85.6% total, and no zero-coverage function gate passed.
- [x] Commit, push, rebase to current `origin/main`, and open closing PR #1104; hosted review remains pending.

## Risks and Mitigations

- Risk: subscribing after the event loses the boundary. Mitigation: subscribe immediately after start; `Subscribe` atomically returns history plus live channel under the event lock.
- Risk: test accidentally stops asserting state. Mitigation: retain the immediate `GetRun` assertion after observing `run.waiting_for_user`.
- Rollback: revert the isolated test synchronization change; no production behavior or persisted data changes.
