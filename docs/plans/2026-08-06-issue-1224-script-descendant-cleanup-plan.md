# Plan: Issue #1224 deterministic script descendant cleanup fixture

## Context

- Governing GitHub issue: #1224.
- Problem: the descendant-cleanup regression uses the production configured
  timeout as a fixture-start deadline, so concurrent race load can report an
  ambiguous missing-PID failure before the child has published its barrier.
- User impact: a flaky test obscures the real process-group cleanup contract.
- Constraints: test-only control-flow change; preserve the existing independent
  configured-timeout test and production timeout/cleanup behavior.

## Scope

- In scope: long fixture timeout, cancelable parent context after a proven
  descendant-start barrier, result-aware readiness, prompt completion and child
  death assertions, docs/logs/indexes.
- Out of scope: `loader.go` runtime behavior, timeout policy, process-group
  semantics, #1221 PTY work, API/TUI/native changes.

## Documentation Contract

- Feature status: `implemented`.
- Public docs affected: None; this is test reliability only.
- Implementation notes: plan/map/logs record the cancellation-owned test
  boundary and exact verification.

## Test Plan (TDD)

- First red: add a readiness helper that observes a handler result before its
  PID barrier and requires the diagnostic to expose that result; the current
  helper cannot receive one and will fail to compile until added.
- Existing coverage retained: `TestScriptHandler_Timeout` remains the explicit
  configured one-second timeout contract.
- Regression: the real background descendant starts, parent cancellation fires
  only after the barrier, handler returns promptly, and the PID is gone.

## Cross-Surface Impact Map

- See [impact map](2026-08-06-issue-1224-script-descendant-cleanup-impact-map.md).

## Implementation Checklist

- [x] Link structured #1224 and record architecture search evidence.
- [x] Create/reconcile cross-surface impact map before test edits.
- [x] Capture the focused red test result.
- [x] Make the smallest test-only fixture/control-flow repair.
- [x] Run focused normal/race and full regression (normal, race, coverage, and
  coverage gate; 85.3% total coverage and zero uncovered functions).
- [ ] Update logs/indexes and publish one closing PR.

## Risks and Mitigations

- Risk: cancellation races before child start. Mitigation: wait for an on-disk
  descendant barrier before canceling.
- Risk: readiness failure hides a real handler error. Mitigation: select on the
  handler result as well as the barrier and report the exact result.
- Rollback: revert the isolated test/docs PR; no production state or migration.
