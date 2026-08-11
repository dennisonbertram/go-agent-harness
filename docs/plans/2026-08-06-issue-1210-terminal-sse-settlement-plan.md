# Plan: Issue #1210 terminal SSE settlement

## Context

- Governing GitHub issue: #1210.
- Problem: the Runner journals a terminal event before it commits the matching
  terminal run status. A run-scoped SSE reconnect can therefore replay
  `run.completed`, `run.failed`, or `run.cancelled` while `GET /v1/runs/{id}`
  remains non-terminal.
- User impact: SSE clients cannot safely treat a received terminal frame as a
  terminal read-model boundary.

## Scope

- In scope: gate terminal frames in `GET /v1/runs/{id}/events` until the
  Runner's public status is terminal and matches the terminal event; cover
  replay, live delivery, all terminal outcomes, and `Last-Event-ID`.
- Out of scope: run-agent acceptance retries, terminal persistence ordering,
  provider behavior, schema/auth changes, and GUI/TUI changes.

## Test Plan (TDD)

- First expected red: a server regression blocks terminal `UpdateRun` after
  the terminal event has entered replay history. It proves the SSE request
  cannot complete with the terminal frame during that block, then proves one
  matching frame and matching terminal GET state after release.
- Coverage: completed, failed, cancelled, plus `Last-Event-ID` replay that
  skips `run.started` without skipping or duplicating the terminal event.
- Verification: focused normal/race server tests and canonical-temp full
  regression.

## Cross-Surface Impact Map

- Linked artifact: `2026-08-06-issue-1210-terminal-sse-settlement-impact-map.md`.

## Implementation Checklist

- [x] Verify current origin/main, structured issue, and current event/status ordering.
- [x] Record plan and impact map before production code.
- [x] Capture deterministic server red.
- [x] Gate terminal SSE delivery on matching public terminal status.
- [x] Run focused normal/race; canonical-temp regression pending.
- [ ] Update durable logs/indexes; commit and push branch for parent PR handoff.

## Risks and Rollback

- Risk: a terminal event whose status can never settle could keep its SSE
  connection open. Mitigation: the wait observes request cancellation; normal
  Runner terminal transitions always commit the in-memory terminal read model
  even when durable status persistence fails.
- Risk: a replay gate reorders events. Mitigation: only terminal writes wait;
  prior history and live non-terminal frames retain their existing order.
- Rollback: revert this isolated server/Runner settlement change; no migration
  or wire-shape change is involved.

## Local Evidence

- Red: `TestTerminalSSEReplayWaitsForMatchingStatusSettlement` returned a
  terminal replay while the terminal `UpdateRun` barrier remained blocked for
  completed, failed, and cancelled outcomes.
- Green: the server waits on the Runner read-model notification; the focused
  server normal/race and adjacent Runner terminal-order normal/race tests each
  passed 20 repetitions.
