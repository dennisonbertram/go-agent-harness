# Issue #1056: Terminal Assistant Message Reconciliation

## Context

- Governing issue: https://github.com/dennisonbertram/go-code/issues/1056
- Pull request: https://github.com/dennisonbertram/go-code/pull/1059
- Problem: the TUI reduces `assistant.message.delta` but not the valid
  authoritative `assistant.message`, so a non-streaming provider can complete
  and persist a reply without showing or exporting it.
- User impact: a successful turn looks blank and later conversation state is
  not trustworthy from the TUI.

## Scope

- In scope:
  - reconcile non-empty terminal `assistant.message` content into the active
    assistant bubble;
  - keep delta-plus-final and replay delivery idempotent;
  - preserve an earlier streamed bubble, intervening tool card, and a later
    terminal-only response in exact viewport order;
  - finalize repeated `run.completed` once while allowing a later run to record
    its own reply;
  - prove two complete final-only turns in one conversation.
- Out of scope:
  - server/provider event changes or synthetic deltas;
  - API, persistence, schema, model routing, web, or macOS changes;
  - unrelated TUI rendering cleanup.

## Test Plan

- Required red-first behavior:
  `TestRegression_FinalOnlyAssistantMessagesPreserveTwoTurnConversation` sends
  user -> `run.started` -> `assistant.message` -> `run.completed` twice and
  requires exact viewport/transcript order user/assistant/user/assistant.
- Adjacent coverage:
  - final-only render and finalize once;
  - delta plus identical final does not duplicate;
  - a differing final replaces partial content;
  - delta -> tool lifecycle -> final-only preserves all blocks and replay is
    harmless;
  - repeated completion and later-run reset are idempotent.
- Required gates:
  focused TUI normal/race, complete harnesscli, repository regression, hosted
  fast/race, and real PTY multi-turn evidence correlated with SSE/API/SQLite.

## Implementation

- Extend the existing `Model.Update(SSEEventMsg)` reducer; do not add another
  transcript owner.
- Treat `assistant.message` as the authoritative full response.
- Close assistant-tail ownership when a tool card begins.
- Use a per-run finalization bit to consume transcript append exactly once and
  reset it on the next `RunStartedMsg`.

## Checklist

- [x] Structured issue and PR-sized scope exist.
- [x] Current ownership and cross-surface impact are recorded.
- [x] Two-turn behavior test failed on current main before implementation.
- [x] Minimal reducer fix and adjacent regressions are green.
- [x] Plan, impact map, logs, and indexes are current.
- [x] Root exact-diff review and independent rereview complete.
- [x] Full repository regression and real PTY proof pass on the candidate.
- [ ] Hosted checks pass on the final pushed SHA.
- [ ] Production two-pass merge gate passes and closes #1056.

## Risks and Rollback

- Duplicate streamed responses: pin delta-plus-final and replay idempotency.
- Tool-card corruption: tool start explicitly closes stale assistant tail
  ownership and the mixed-step test pins block order.
- Duplicate transcript rows: finalization is consumptive per run and resets for
  the next run.
- Roll back the isolated reducer change if any existing streamed reply is lost,
  duplicated, or reorders a tool card; no persisted repair or migration is
  required.
