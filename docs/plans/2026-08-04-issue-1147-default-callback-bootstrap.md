# Plan: Default callback bootstrap admits durable continuation runs

## Context

- Governing GitHub issue: #1147 (child of #1000).
- Problem: a default callbacks-enabled `harnessd` creates/persists a callback but cannot admit its reserved continuation run because `HARNESS_RUN_DB` is absent.
- User impact: an acknowledged “in two minutes say hello” retries and fails instead of advancing the same visible conversation.
- Constraints: strict red-green TDD; one bug/PR only; preserve explicit `HARNESS_RUN_DB` behavior and its authentication semantics; worktree head is `bf99206`.

## Scope

- In scope: the existing persistence bootstrap/default run-store ownership, callback run admission integration test, explicit-store compatibility controls, logs/docs/indexes, and live API/TUI/GUI verification after merge.
- Out of scope: retry-policy redesign, callback schema redesign, cron behavior, TUI stream repair (#1148), execution API (#1149), and native-app provenance/UI controls (#1009).

## Documentation Contract

- Feature status: in implementation.
- Public docs affected: callback scheduling/operator contract and run persistence behavior.
- Spec docs before code: this plan and linked impact map.
- Implementation notes after code: engineering log, relevant docs/indexes, PR verification evidence.

## Test Plan (TDD)

- New failing test first: `TestDefaultBootstrapDelayedCallbackStartsContinuation` in `cmd/harnessd`, exercising no `HARNESS_RUN_DB`, tool scheduling, due dispatch, one reserved run ID, and same-conversation assistant result.
- Existing tests to update: bootstrap helper default/explicit-path coverage and callback starter/integration tests as needed.
- Regression tests required: explicit store/auth compatibility, retry/restart idempotency, callback cancellation, and live API path.

## Implementation Checklist

- [x] Link issue #1147 and inspect bootstrap, callback starter, runner admission, tool/task/SSE callers.
- [x] Create and reconcile `2026-08-04-issue-1147-default-callback-bootstrap-impact-map.md`.
- [x] Capture default-bootstrap red regression result (`callbacks-enabled default bootstrap must provide a durable run store`).
- [ ] Repair existing persistence bootstrap without parallel callback runner.
- [x] Add compatibility/recovery plus HTTP/tool/same-conversation acceptance tests.
- [ ] Update issue if root cause or scope changes.
- [ ] Update logs/docs/indexes.
- [ ] Run targeted, race, full regression, and live matrix evidence.
- [ ] Obtain cheap independent review, push PR with `Closes #1147`, and merge only after green review.

## Risks and Mitigations

- Risk: internal default store changes implicit auth. Mitigation: isolate “store exists” from “explicit persistence/auth requested” in a characterization test.
- Risk: retry duplicates a continuation. Mitigation: preserve the existing callback-reserved run ID and prove a restart/retry uses it once.
- Risk: workspace filesystem failure creates a misleading schedule success. Mitigation: fail callback-enabled bootstrap clearly before accepting schedules.
