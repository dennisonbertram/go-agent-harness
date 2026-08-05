# Plan: Issue #1183 durable replay SSE fixture

## Context

- Governing GitHub issue: #1183.
- Problem: the TUI's durable `/replay run_*` path correctly returns a `RunStartedMsg`, which starts the returned run's SSE bridge. Its unit fixture only accepted the preceding POST and therefore failed when the bridge requested `/v1/runs/<returned>/events`.
- User impact: a red TUI race baseline blocks credible end-to-end scheduled-conversation acceptance.
- Constraints: fixture and documentation only; preserve production durable replay and rollout-file simulation semantics.

## Scope

- In scope: make the durable replay fixture validate its POST response, returned-run authenticated SSE request, streamed terminal event, and deterministic connection closure; assert simulation never opens a run SSE stream.
- Out of scope: replay API/server changes, TUI production code, cron/callback behavior, and cancellation seams.

## Documentation Contract

- Feature status: implemented production behavior; fixture repair implemented locally.
- Public docs affected: none.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: durable logs and indexes.

## Test Plan (TDD)

- First expected red: expand `TestRunControl_ReplayCommandCallsReplayEndpoint` to drive the `RunStartedMsg` lifecycle. Current fixture rejects the intended `GET /v1/runs/run_replayed_1/events` request, reproducing #1183.
- Existing tests to update: durable replay fixture; rollout simulation fixture gets an explicit no-SSE assertion.
- Regression tests: focused normal/race repetition, full TUI normal/race, and canonical-temp serial `./scripts/test-regression.sh`.

## Cross-Surface Impact Map

- Linked artifact: `2026-08-05-issue-1183-replay-sse-fixture-impact-map.md`.

## Implementation Checklist

- [x] Confirm structured issue and production subscription intent.
- [x] Record plan, impact analysis, and expected red.
- [x] Add deterministic fixture SSE lifecycle coverage.
- [x] Run focused normal/race and full TUI gates.
- [x] Run canonical-temp repository regression.
- [ ] Record final evidence, commit, push, PR, review, and CI handoff.

## Risks and Mitigations

- Risk: a fixture hides an unexpected endpoint or leaves an SSE goroutine behind. Mitigation: accept only the returned run's exact GET with `Accept: text/event-stream`, send a terminal frame, close the response, and synchronize the observed request.
- Risk: rollout simulation regresses into a live run. Mitigation: track every events request and require zero after its one-shot result.
- Rollback: revert the test/doc-only commit; no runtime state or production behavior changes.

## Local Evidence

- Expected red: the prior hosted race fixture rejected the intended returned-run
  `GET /v1/runs/run_replayed_1/events`; #1183 records that exact failure.
- Green: focused normal/race replay tests each passed 20 repetitions.
- Green: complete `cmd/harnesscli/tui` normal passed in 41.776s and race passed
  in 44.455s with the isolated temporary Go cache.
- Green: retained canonical-temp `TMPDIR=/private/tmp ./scripts/test-regression.sh`
  passed normal, race, coverage, and the coverage gate at 85.5% total coverage
  with zero uncovered functions.
