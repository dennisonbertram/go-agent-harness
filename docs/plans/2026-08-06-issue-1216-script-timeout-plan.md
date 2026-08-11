# Plan: Issue #1216 script timeout process-tree cleanup

## Context

- Governing GitHub issue: #1216.
- Problem: `TestScriptHandler_Timeout` can take the full child lifetime under
  aggregate race/load despite its one-second timeout when a descendant retains
  the script's stdio pipes.
- User impact: a configured script-tool timeout may leave a conversation turn
  blocked long after its declared deadline.
- Constraints: retain the real process tree, the five-second bound, outer
  context semantics, and stderr/non-zero error behavior. Do not weaken the
  test or replace the child with a mock.

## Scope

- In scope: one handler lifecycle repair plus a deterministic real-descendant
  regression proving prompt return and process-tree cleanup.
- Out of scope: sandbox execution, other tool runners, timeout values, API/TUI/
  GUI contracts, retry policy, and workflow fixtures.

## Documentation Contract

- Feature status: implemented timeout behavior; this is a reliability repair.
- Public docs affected: none; the tool contract is unchanged.
- Spec docs to update before code: this plan and the linked impact map.
- Implementation notes to add after code: engineering log and plans index.

## Test Plan (TDD)

- New failing test first: a script starts a background descendant that holds
  stdout/stderr open after its parent exits; the handler must return on the
  configured deadline and the recorded descendant PID must be gone.
- Existing tests: retain the ordinary slow-script timeout and non-zero/stderr
  tests unchanged.
- Regression tests: focused normal/race stress for the script package and the
  full `TMPDIR=/private/tmp ./scripts/test-regression.sh` gate.

## Cross-Surface Impact Map

- `docs/plans/2026-08-06-issue-1216-script-timeout-impact-map.md` records the
  searched ownership and affected lifecycle boundary.

## Implementation Checklist

- [x] Verify structured issue #1216 and current handler ownership.
- [x] Record architecture search evidence and impact map.
- [x] Add real descendant test first; the original aggregate race red is the
  expected failure evidence, while pre-change focused normal/race stress did
  not reproduce its scheduling-sensitive 31-second delay.
- [x] Implement handler-owned cancellation/group kill and bounded wait drain.
- [x] Record local full regression evidence; publish one closing PR for hosted review.

## Risks and Mitigations

- Risk: cancellation kills only the direct script and an inherited pipe keeps
  `Wait` blocked. Mitigation: the handler exclusively owns process-group kill
  and sets a bounded `WaitDelay`; the regression observes a real descendant PID.
- Risk: outer cancellation is confused with the handler timeout. Mitigation:
  preserve the existing deadline-derived public timeout error while retaining
  normal child exit/stderr handling.
