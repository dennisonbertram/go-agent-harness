# Plan: Issue #1110 notify-parent activation test synchronization

## Context

- Governing GitHub issue: #1110.
- Problem: `TestStartRun_AutoActivatesNotifyParentForSubagentRuns` starts an
  instant provider and reads activation state after `StartRun` returns. The run
  may already have completed and correctly removed its transient activation.
- User impact: none from this fixture defect; the test must prove that a
  recorded-parent subagent actually receives `notify_parent` in its first model
  request, without hiding a real activation regression.
- Constraints: test-only slice; no Runner, registry, API, callback, or tool
  catalog behavior change. No sleeps.

## Scope

- In scope: deterministic in-flight provider fixture; assertion of the first
  `CompletionRequest.Tools`; terminal cleanup assertion; adjacent positive,
  negative, and parent-handoff coverage.
- Out of scope: changing the `StartRun` activation policy, callback durability,
  persistence, API/SSE, TUI, native UI, or provider implementation.

## Documentation Contract

- Feature status: `implemented` production behavior, `in implementation` test
  reliability repair.
- Public docs affected: None; no public contract changes.
- Spec docs to update before code: this plan and impact map.
- Implementation notes to add after code: engineering/observational logs and
  indexes.

## Test Plan (TDD)

- First red evidence: preserve the old instant-provider reproduction under
  `-race -count=1000`; it demonstrates that post-`StartRun` activation is not
  a valid lifetime boundary because terminal cleanup may already occur.
- New deterministic regression: block `Complete` after recording the first
  request, require `notify_parent` in that request for a recorded parent,
  release the gate, then await terminal state and verify activation cleanup.
- Existing tests: retain top-level/no-parent and empty-parent negatives;
  execute adjacent parent-handoff storage tests.
- Verification: focused normal and race repetitions (at least 1000 where
  practical), adjacent handoff tests, and `./scripts/test-regression.sh`.

## Cross-Surface Impact Map

See `2026-08-03-issue-1110-notify-parent-activation-test-impact-map.md`.

## Implementation Checklist

- [x] Verify structured issue and search the activation/parent-handoff seams.
- [x] Record plan and cross-surface impact map.
- [x] Capture pre-change semantic red: awaiting terminal before the former
  post-StartRun read fails because cleanup is correct.
- [x] Write deterministic gated provider regression before changing fixture.
- [x] Verify normal/race repetitions and adjacent parent-handoff tests:
  positive/negative focused normal x1000 and race x1000; handoff x100.
- [x] Update logs/indexes and run full regression: foreground full regression
  passed normal, race, 85.7% coverage, and zero uncovered functions.
- [x] Open PR #1111 with `Closes #1110`; awaiting independent review and
  hosted checks after rebase to the current `origin/main`; do not merge.

## Risks and Mitigations

- Risk: a gate could conceal completion cleanup. Mitigation: assert the first
  recorded request before release and explicitly await terminal cleanup after.
- Risk: a test-only repair could weaken negative coverage. Mitigation: retain
  no-parent and empty-parent tests unchanged and run them with the positive
  test.
