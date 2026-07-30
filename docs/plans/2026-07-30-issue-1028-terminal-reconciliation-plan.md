# Plan: Preserve Terminal State During Transcript Reconciliation

## Context

- Governing GitHub issue: #1028.
- Problem: durable message reconciliation reloads transcript rows and
  unconditionally changes a failed or cancelled scheduled run to completed.
- User impact: a deployment watcher or other automation can look successful in
  the macOS GUI even though its authoritative terminal event failed or was
  cancelled.
- Constraints: preserve strict TDD, existing SSE/message contracts, historical
  replay deduplication, and the active-user-run guard.

## Scope

- In scope: preserve authoritative failed/cancelled terminal state and event
  detail while rebuilding persisted transcript rows.
- Out of scope: server replay, provider/model behavior, cron/callback schemas,
  transcript redesign, and new persistence.

## Documentation Contract

- Feature status: `implemented`.
- Public docs affected: none; the public wire contract is unchanged.
- Spec docs to update before code: this plan and its cross-surface impact map.
- Implementation notes to add after code: engineering log and plan/index status.

## Test Plan (TDD)

- New failing tests to add first: failed and cancelled terminal conversation
  replay followed by successful durable-message reconciliation.
- Existing tests to update: none.
- Regression tests required: completed replay still deduplicates persisted rows;
  failed retains error detail; cancelled retains cancelled state.

## Cross-Surface Impact Map

- See `2026-07-30-issue-1028-terminal-reconciliation-impact-map.md`.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Link a contract-complete structured GitHub issue before implementation.
- [x] Record current architecture, callers, consumers, and source-of-truth search evidence.
- [x] Document feature status and exact contract before code.
- [x] Complete and reconcile the cross-surface impact map before implementation.
- [x] Add characterization coverage before structural refactors.
- [x] Write failing tests first.
- [x] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries.
- [x] Implement minimal code changes.
- [x] Refactor while tests remain green.
- [x] Update docs, status ledgers, and indexes.
- [x] Update engineering/system/observational logs as needed.
- [x] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: preserving the entire pre-load transcript state could leave a completed
  reconciliation stuck in an active state.
- Mitigation: preserve only authoritative failed/cancelled terminal states and
  their event-derived error rows; retain existing completed behavior and cover
  every terminal variant.
