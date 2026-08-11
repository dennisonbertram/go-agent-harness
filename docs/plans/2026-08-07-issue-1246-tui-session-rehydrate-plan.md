# Plan: Rehydrate selected TUI session transcript

## Context

- Governing GitHub issue: #1246.
- Problem: the global `Submit` key handler claims Enter before the generic
  sessions-overlay router. The picker never emits the model-level selection
  message, so the history/SSE rehydration path on merged main cannot run.
- User impact: a user can see a durable session but cannot review, search, or
  continue it from the real TUI.
- Constraint: preserve the merged atomic replay/SSE protocol. A selected
  session starts the boundary stream first; only an unsupported boundary uses
  the existing legacy history fallback.

## Scope

- In: TUI sessions-overlay Enter routing; selected-session atomic replay,
  legacy fallback, visible transcript behavior, and stale-history isolation.
- Out: replay protocol, API/persistence redesign, native GUI, run-ID routing,
  and unrelated overlay behavior.

## Test-First Plan

1. Add deterministic keyboard and selected-boundary regressions: `/sessions`
   plus Enter emits `SessionPickerSelectedMsg`; supported replay renders one
   snapshot then a same-text future turn; unsupported empty-cursor fallback is
   snapshot-only.
2. Run it red on exact `f8b43be`, recording the expected swallowed-Enter
   failure.
3. Implement only the session-picker command wrapper and explicit Submit path.
4. Run focused normal/race, the full regression gate, and a 30x100 real PTY
   conversation that selects, searches, and sends a continued prompt.

## Cross-Surface Impact Map

See `2026-08-07-issue-1246-tui-session-rehydrate-impact-map.md`.

## Checklist

- [x] Verify #1246, exact base, and historical diagnosis.
- [x] Record plan and impact map before code.
- [x] Add and record red deterministic regression.
- [x] Route sessions-overlay Submit through `sessionPicker.Update`.
- [x] Preserve selected-run atomic boundary SSE and legacy fallback lifecycle from merged main.
- [x] Update implementation logs and indexes.
- [x] Run focused normal/race, full regression, and real 30x100 PTY proof.
- [ ] Push one closing PR with exact-head evidence; do not merge.

## Rollout and Rollback

- Rollout: ordinary TUI binary update; the existing durable messages endpoint
  and conversation SSE stream are unchanged.
- Observability: loading/status text, focused HTTP assertion, and retained PTY
  artifact show the selected-session path.
- Rollback: revert the local TUI routing change; no data migration or protocol
  rollback is required.
