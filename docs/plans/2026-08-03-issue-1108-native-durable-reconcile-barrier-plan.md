# Issue #1108 — Native durable reconciliation barrier

## Intent and success criteria

Issue #1108 repairs a flaky native regression fixture, not product behavior.
The callback-replay test must prove that a later external run C has accepted
ownership, terminal state, and exact usage before its deliberately delayed
durable `/messages` reconciliation is released. After release it must prove
the rendered durable C row as well as the same C accounting/state. Adjacent
stale-terminal fixtures must similarly wait for rendered durable rows rather
than merely observing that a request was issued.

## Scope

- In: `RunSessionConversationStreamTests` fixture gates and application-level
  assertions; associated plans/logs/indexes; focused, repeated, native live,
  and full Go regression verification.
- Out: `RunSession`, transcript reducer, server/API/SSE semantics, persistence,
  TUI, cron/callback dispatch, retries, sleeps, and generic reconciliation
  generations.

## Test-first plan

1. Delay C's second `/messages` fixture response behind `release_c_durable`.
   Confirm the old assertion is insufficient because C accounting can precede
   durable rendering.
2. Before opening that gate, wait for `accountingRunID == run_c`, completed
   state, and exact C usage; assert C durable text is absent.
3. Open the gate; wait for the rendered C durable assistant row plus the same
   ownership/state/exact accounting and assert final durable transcript.
4. Replace adjacent stale-terminal raw request fences with rendered A/B durable
   row conditions following their gates.

## Verification and rollback

Run focused/repeated conversation-stream tests, the complete Swift test suite,
the native live harness suite, and `./scripts/test-regression.sh`. The change
is test-only; rollback is a revert of this PR if it weakens any required
application-level invariant.

## Checklist

- [x] Issue #1108 and current ownership/search evidence reviewed.
- [x] Cross-surface impact map written.
- [x] Delayed durable-response red captured (`hasAssistantText` failed after C
  accounting completed while `release_c_durable` was closed).
- [x] Application-level barriers and assertions implemented.
- [x] Focused/repeated/full native/live/Go gates pass: focused stream suite
  (11), C regression x20, full Swift (190), live `RunSessionLiveTests` (2),
  and Go regression (85.5%, zero uncovered functions).
- [x] PR #1109 opened with `Closes #1108`; no merge in this slice.
