# Plan: External scheduled-run controls in macOS chat

## Context

- Governing GitHub issue: #1007 (child of #1000; coordinates with merged #994).
- Problem: `RunSession` renders cron/callback conversation SSE but only assigns
  `currentRunID` after its own `startRun` response. External runs therefore
  leave approval, input, steer, and cancel controls without a safe target; a
  terminal event for an older run can also make a newer run appear inactive.
- User impact: an operator watching a deployment cannot safely take the next
  conversational action when a scheduled continuation asks for approval/input.
- Constraints: Swift-only state/UI slice; no harness protocol, persistence, or
  scheduler change; preserve self-submitted stream/replay behavior.

## Scope

- In scope: centralized RunSession active-run identity reduction from scoped
  conversation SSE; correct terminal/replay behavior; action targeting;
  accessible scheduled-run status; Swift regression coverage and required docs.
- Out of scope: cron/callback durability (#1005/#1006), SSE durability (#1008),
  task-lifecycle/activity UI (#1009), and backend/API changes.

## Documentation Contract

- Feature status: in implementation.
- Public docs affected: none; this is an internal macOS state contract.
- Spec docs updated before code: this plan and its impact map.
- Implementation notes after code: engineering, observational, system logs.

## Test Plan (TDD)

- Add a stubbed conversation-SSE test whose external `run.started` followed by
  approval/input events is actionable; run it red before changing RunSession.
- Cover the endpoints/run IDs for approve, deny, input, steer, and cancel;
  terminal A while B is selected; duplicate/replayed A after B; and stale
  events arriving from a conversation no longer selected.
- Preserve existing self-submitted dedupe coverage. Run focused RunSession
  tests, all macapp tests, then `./scripts/test-regression.sh`.

## Cross-Surface Impact Map

See `2026-08-03-issue-1007-external-run-controls-impact-map.md`.

## Implementation Checklist

- [x] Verify structured issue #1007 and its #994 dependency.
- [x] Record architecture and impact map before code.
- [x] Add and capture the meaningful failing Swift regression test.
- [x] Centralize scoped active-run identity reduction in RunSession.
- [x] Add accessible scheduled-run state copy.
- [x] Run the focused external-control regression suite.
- [x] Run full macapp and repository regression gates from the final rebased tree.
- [x] Update logs/indexes with final exact evidence; draft PR remains unmerged.

## Risks and Mitigations

- Concurrent/replayed streams could retarget controls: order control ownership
  before accounting; retain terminal tombstones; preserve a timestamp-less
  local provisional owner; and apply a selected terminal before resuming a
  still-active fallback run.
- A cancelled old conversation stream can still yield: bind stream deliveries
  to the currently selected conversation before any transcript/control update.
- A self-started run is delivered twice: keep event-ID dedupe and preserve the
  response-derived current ID.
- macOS Keychain integration blocks under a detached tmux launch context:
  record that attempt, but use the logged-in foreground regression gate as the
  authoritative repository check.
