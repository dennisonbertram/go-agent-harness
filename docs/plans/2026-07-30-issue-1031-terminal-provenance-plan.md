# Plan: Distinguish Authoritative and Local Terminal State

## Context

- Governing GitHub issue: #1031.
- Problem: durable reconciliation cannot distinguish a server `run.failed`
  event from the local failure placeholder used when an SSE transport dies.
- User impact: a completed scheduled or user run can remain permanently failed
  in the macOS GUI after a transient disconnect.
- Constraints: preserve authoritative failed/cancelled state, completed replay
  deduplication, and current server/wire contracts.

## Scope

- In scope: track the latest authoritative terminal event separately from
  rendered transcript state and use it during durable reconciliation.
- Out of scope: transport retry redesign, server changes, cron/callback schema
  changes, and historical failure retention across later runs (#1032).

## Documentation Contract

- Feature status: `implemented`.
- Public docs affected: none.
- Spec docs to update before code: this plan and impact map.
- Implementation notes to add after code: engineering log and plans index.

## Test Plan (TDD)

- New failing test: per-run transport failure creates a provisional local
  failure; an older durable snapshot cannot report success or enable another
  prompt before a delayed authoritative completion event arrives.
- Existing controls: authoritative failed/cancelled replay remains preserved;
  completed persisted replay remains deduplicated.
- Full verification: strict formatter, full Swift package, and repository
  normal/race/coverage gate.

## Cross-Surface Impact Map

- See `2026-07-30-issue-1031-terminal-provenance-impact-map.md`.

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

- Risk: an old authoritative failure could leak into a later run, or an old
  message snapshot could falsely complete a currently unresolved run.
- Mitigation: clear provenance on conversation replacement, track the
  accepted-run-to-terminal interval explicitly, and test both provisional
  blocking and eventual authoritative recovery.
