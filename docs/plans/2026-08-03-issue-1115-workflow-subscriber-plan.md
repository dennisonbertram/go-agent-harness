# Issue #1115 Workflow Subscriber Terminal-Close Plan

## Context

- Governing GitHub issue: [#1115](https://github.com/dennisonbertram/go-code/issues/1115)
- Problem: the full-buffer terminal-close regression starts the asynchronous workflow before subscribing, so a loaded host can finish the run before the channel is registered and fail a test intended to cover a pre-terminal subscriber.
- User impact: flaky baseline CI blocks reviewed callback work and weakens confidence that a registered slow subscriber cannot hang after terminal workflow execution.
- Constraints: test first; do not change callback/cron code or PR #1107; preserve the existing workflow event-ordering, replay, close/cancel, and late-subscription contracts.

## Scope

- In scope: add an explicit workflow execution gate so the subscriber is registered before the >64-event burst; keep waits bounded; stress normal/race; document the diagnosis and evidence.
- Out of scope: production workflow changes; late-subscriber semantic changes; API/config/schema/auth/client/provider/tool-catalog work; callback or cron changes.

## Documentation Contract

- Feature status: `implemented`
- Public docs affected: None; runtime behavior does not change.
- Spec docs to update before code: this plan and its linked impact map.
- Implementation notes to add after code: engineering, observational, and system log entries plus plan/index status.

## Test Plan (TDD)

- New failing tests to add first: gate the chatty script and first prove that withholding the release prevents terminal completion within a bounded interval.
- Existing tests to update: `TestSubscriberChannelClosesOnTerminalEventEvenWithFullBuffer` will release the workflow only after `Subscribe` returns.
- Regression tests required: full-buffer terminal close, buffered ordering-before-close, and cancel-after-terminal safety under repeated normal and race execution.

## Cross-Surface Impact Map

- See `2026-08-03-issue-1115-workflow-subscriber-impact-map.md`.
- The change is test-only, but the map covers ownership, lifecycle, server consumption, compatibility, and operational evidence so the existing production contract is not accidentally broadened.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Link a contract-complete structured GitHub issue before implementation.
- [x] Record current architecture, callers, consumers, and source-of-truth search evidence.
- [x] Document feature status and exact contract before code.
- [x] Complete and reconcile the cross-surface impact map before implementation.
- [x] Add characterization coverage before structural refactors (existing subscriber concurrency suite; no structural refactor planned).
- [x] Write failing tests first.
- [x] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries (no type or ownership change).
- [x] Implement minimal test-fixture change.
- [x] Refactor while tests remain green.
- [x] Update docs, status ledgers, and indexes.
- [x] Update engineering/system/observational logs as needed.
- [x] Run full test suite.
- [ ] Merge branch back to `main` after tests pass and independent review completes.

## Risks and Mitigations

- Risk: a sleep-based fixture remains flaky. Mitigation: channel handshake establishes `Subscribe` completion before release; all waits remain bounded.
- Risk: closing outside `Engine.mu` reintroduces send-on-closed/double-close. Mitigation: no production close-path change.
- Risk: a test goroutine leaks on failure. Mitigation: release is owned by the test, closed exactly once, and cleanup is bounded by the workflow wait.
- Risk: the test stops filling the 64-slot channel. Mitigation: emit 100 logs plus terminal event without draining until the run is terminal.
