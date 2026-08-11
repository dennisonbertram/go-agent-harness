# Issue #1270: deterministic TUI replay-boundary snapshot fixture

## Context

- Governing issue: #1270.
- Problem: `TestResumedConversationReplayBoundarySnapshotIncludesQueuedFuture`
  timed out under the race suite because its fixture published a post-boundary
  live event before the model had necessarily reduced the atomic replay
  snapshot.
- User impact: the flaky fixture blocks confidence-sensitive TUI continuation
  promotion, even though the intended contract is that queued replay is
  atomically rendered before a later live continuation.
- Constraints: test-only repair; no product source, timeout increase, or
  assertion weakening.

## Scope

- In scope: causal server-to-model hand-off in the focused fixture, retained
  historic/queued/live assertions, and timeout-stage diagnostics.
- Out of scope: TUI reducer changes, API/protocol changes, retries, and broad
  test-driver redesign.

## Documentation Contract

- Feature status: implemented test-fixture reliability repair.
- Public docs affected: none; no user-facing behavior changed.
- Implementation notes: engineering log and indexes record the fixture
  ownership boundary and exact verification.

## Test Plan (TDD)

1. Add a withheld live-event gate after the replay marker; without model
   release, the existing six-second test must fail at the expected stop
   condition (red evidence).
2. Release that event only after the rendered model contains the two atomic
   snapshot entries (green behavior).
3. Preserve exact-once transcript and normal live-reducer assertions.
4. Run focused normal and race repetitions, package race, then
   `./scripts/test-regression.sh`.

## Implementation Checklist

- [x] Verify structured issue #1270 and current owner/caller evidence.
- [x] Record cross-surface impact map before implementation.
- [x] Record focused expected-red evidence.
- [x] Add causal replay-marker-to-live-event gate and failure-stage diagnostic.
- [ ] Complete focused/package/full verification and reviewable PR.

## Risks and Mitigations

- Risk: a fixture gate could conceal a product ordering defect. Mitigation:
  gate release only after the model visibly renders the server-provided marker
  snapshot, while retaining the post-boundary event through the normal reducer.
- Risk: a stuck gate obscures failure diagnosis. Mitigation: retain the
  six-second budget and log the exact fixture stage when the test fails; also
  select on request cancellation while the fixture waits for model release.
