# Plan: Issue #1264 Runner test lock-snapshot repair

## Context

- Governing GitHub issue: #1264.
- Problem: two `runner_task_complete_test.go` child-policy assertions hold
  `Runner.mu.RLock` and call `forkedRunID`, which acquires the same lock again.
  Once `completeRun` queues `pruneCompletedRuns` as writer, Go's `RWMutex`
  blocks the nested reader and the outer reader cannot release.
- User impact: the test-only deadlock makes the mandatory GitHub race lane
  time out and blocks otherwise reviewed delivery; it does not describe a
  production Runner deadlock.
- Constraints: preserve all production locking, pruning, and assertions;
  change only test snapshot ownership plus required documentation.

## Scope

- In scope: capture child fields under exactly one `RLock` in the shared test
  helper, migrate both recursive-lock call sites, and retain/repeat the
  focused race regression.
- Out of scope: `Runner`, `completeRun`, `pruneCompletedRuns`, scheduler,
  persistence, API, CLI, TUI, GUI, and #1263 continuation behavior.

## Documentation Contract

- Feature status: `in implementation`.
- Public docs affected: None; this is test-only.
- Spec docs to update before code: this plan and impact map.
- Implementation notes to add after code: engineering log, plan status, and
  plans/logs indexes.

## Test Plan (TDD)

- New failing test first: bounded `-race` execution of
  `TestRunForkedSkill_UntrustedMetadataCannotInheritParentPolicy` exposes the
  known recursive-lock failure when writer scheduling is adversarial; record
  its pre-fix outcome without extending a timeout or relaxing assertions.
- Existing tests to update: both child-policy tests use one immutable child
  snapshot rather than an outer lock plus helper lock.
- Regression tests required: targeted child-policy normal/race repeated runs,
  `go test -race ./internal/harness/...`, and `./scripts/test-regression.sh`.

## Cross-Surface Impact Map

See `2026-08-07-issue-1264-runner-test-deadlock-impact-map.md`.

## Implementation Checklist

- [x] Verify structured issue #1264 and search current helper/call sites.
- [x] Record architecture, scope, and test-first plan.
- [x] Capture the pre-fix bounded race result.
- [x] Replace nested reads with immutable single-lock child snapshots.
- [x] Run repeated focused normal/race tests.
- [x] Run package race and full regression.
- [x] Update status/log/index documentation.
- [ ] Create a closing PR after independent review.

## Risks and Mitigations

- Risk: a shallow pointer snapshot hides an assertion race. Mitigation: copy
  only the asserted scalar fields (`Sandbox` and `Approval`) while holding the
  sole `RLock`; never retain `PermissionConfig` after unlock.
- Risk: fixing the test masks a production lock defect. Mitigation: production
  files are out of scope and package race/full regression remain mandatory.
