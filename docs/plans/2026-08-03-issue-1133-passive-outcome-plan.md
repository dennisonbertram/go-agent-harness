# Plan: Issue #1133 passive displaced-submission outcomes

## Context

- Governing GitHub issue: #1133.
- Problem: after B replaces visible submitted A, ToolWalk returned `.displaced`
  immediately. A terminal/failure therefore could never be judged even though
  `RunSubmission` still retained the correct A-only evidence.
- User impact: scheduled callbacks/crons can continue the visible conversation
  without causing the initiating tool walk to falsely fail or control B.
- Constraints: stack on #1131 `63cf9fcd`; no harness, TUI, persistence, or wire
  contract change; B selection permanently revokes A automatic controls.

## Scope

- In scope: retain passive A observation through terminal/failure/deadline and
  prove the timeout policy sends A-only transport without a B action, using
  gated `RunSession.submit()` plus `Runner` integration coverage. #1136 owns
  the immutable capability implementation and its stronger authority proof.
- Out of scope: changing selected-run UI ownership, server scheduling, ToolWalk
  grammar, retry behavior, or live #1010 acceptance.

## Documentation Contract

- Feature status: implemented locally, pending review and hosted gates.
- Public docs affected: none; this is an internal native/ToolWalk ownership
  policy.
- Implementation notes: durable logs and indexes record the corrected contract.

## Test Plan (TDD)

- First failing test: B is selected before A terminal/EOF/timeout/delayed ACK;
  old Runner returns `.displaced`, so it neither observes A nor cancels timed-out
  A.
- Acceptance tests: actual URLSession-gated `RunSession.submit()` +
  `Runner.waitFor…` tests prove A terminal, A EOF failure, A-only timeout POST,
  and delayed A acknowledgement, each with zero B action endpoints. #1136
  owns the B -> C immutable authority, revocation, and all-stream proof.
- Regression tests: existing action-owner, delayed-ACK, submission-handle, and
  ToolWalk outcome suites; full Swift and repository regression gates.

## Implementation Checklist

- [x] Verify #1133 and stacked #1131 base.
- [x] Write and capture the gated integration red.
- [x] Keep displacement sticky while observing terminal/failure passively.
- [x] Make timeout transport-only for exact locally owned displaced A.
- [x] Update plans/logs/indexes, including the stale displaced-result wording.
- [x] Run strict format and focused/full Swift; final combined focused
  `PassiveSubmissionOutcomeIntegrationTests` evidence is 10/10 (not the
  intermediate #1133-only 4/4 count).
- [ ] Run full regression after the independent #1135 baseline fixture repair.
- [ ] Publish stacked draft `Closes #1133`; obtain independent cheap review.

## Risks and Mitigations

- Risk: passive waiting accidentally sends an A prompt/approval to selected B.
  Mitigation: explicit `isDisplaced || currentRunID != A` control fence and
  per-test zero-B endpoint assertions.
- Risk: A timeout does nothing because B is selected, or mutates B state.
  Mitigation: narrow local-submission ownership predicate with an A-only
  transport POST that does not change shared selected/transcript state.
