# Issue #1038 — Native Live Terminal Usage Reconciliation Plan

## Intent and scope

- Issue: #1038, native live run completes before transcript usage is recorded.
- Command intent: make `RunSessionLiveTests.submitProducesTranscript` observe real fake-provider usage at the same terminal boundary the UI renders, without a timing delay or a server/protocol change.
- User intent: a completed native transcript must include its real usage summary; a visually completed run with zero usage is incomplete.
- In scope: native `Transcript` reduction of the existing terminal-event accounting snapshot, a deterministic regression test, the existing live test, required engineering documentation, and the existing regression gate.
- Out of scope: harness event ordering/protocol changes, persistence/schema changes, TUI changes, provider behavior, arbitrary waits, and #1037 resource resolution.

## Search and diagnosis

- `internal/harness/runner_step_engine.go` emits `usage.delta` before `llm.turn.completed`; `internal/harness/runner.go` also embeds final `usage_totals` and `cost_totals` in every terminal event.
- `macapp/Sources/HarnessKit/Transcript.swift` applies only `usage.delta`; its `run.completed`, `run.failed`, and `run.cancelled` cases mark terminal state without consuming the terminal snapshot.
- `RunSessionLiveTests` is intentionally event-driven and observes `runState == .completed`; therefore the terminal snapshot is the correct final reconciliation source, not a delayed test poll.
- Live raw-SSE capture proved harnessd emits both `usage.delta` and terminal totals. The failing path is native: `RunSession.streamConversation` applies the terminal event, then calls `Transcript.reconcile`; `reconcile` calls `load`, which resets the value-type transcript and drops usage/cost state.
- Sol review follow-up: retaining a bare transcript `UsageTotals` through every durable sync leaks run A's accounting into a later callback/cron run B when B is observed only through message sync, has incomplete terminal totals, or fails before accounting arrives. The ownership boundary must be `RunSession`'s run identity, not `Transcript`'s message rebuild.

## Test-first plan

1. Add a `TranscriptTests` regression that applies only a decoded `run.completed` event containing harnessd's real `usage_totals`/`cost_totals` shape. Expected red: terminal state is completed while all usage fields remain zero.
2. Add the smallest reducer helper that reconciles terminal totals without erasing already-present usage-delta values when a terminal payload is incomplete, then preserve that accounting snapshot across durable-message reconciliation.
3. Repair the review finding with run-scoped accounting admission: clear on each local/external run boundary; admit usage/terminal totals only for the active/newest run; preserve totals across reconciliation only for that accepted terminal; clear standalone durable sync. Add deterministic multi-run local-failure, incomplete-terminal, duplicate/reconnect, and stale-order regressions plus a conversation-SSE terminal/reconciliation accounting assertion.
3. Run the focused Swift test, full Swift suite, strict lint/build, live `RunSessionLiveTests`, and `./scripts/test-regression.sh` from an active macOS session.

## Acceptance and rollback

- Completed fake-provider runs expose nonzero prompt, completion, total-token, and priced-cost state before transcript completion is observable.
- Missing terminal totals retain the previously reduced usage; terminal event order and wire compatibility stay unchanged.
- Rollback is one native reducer change; no stored data or migration needs repair.

## Checklist

- [x] Issue contract and current ownership read.
- [x] Cross-surface impact map created.
- [x] Red regression captured.
- [x] Minimal reconciliation implemented.
- [x] Review P1 repair red/green and full verification captured.
- [ ] Engineering log and indexes updated.
