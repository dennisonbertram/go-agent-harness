# Plan: Issue #424 Event Journal Extraction

## Context

- Problem: `Runner.emit()` currently owns canonical event append, subscriber fanout, recorder interaction, terminal sealing, and store append setup inline inside one large method.
- User impact: The hottest event path is harder to review and evolve safely because several ownership boundaries are implicit in one place.
- Constraints:
  - Preserve existing event ordering and terminal behavior exactly.
  - Keep the public runner API and event payload contracts unchanged.
  - Follow strict TDD with a failing characterization test first.

## Scope

- In scope:
  - Extract the event journal/sink responsibilities behind a narrower internal boundary.
  - Add or update direct regression coverage for the extracted seam.
  - Keep `emit()` behavior-preserving.
- Out of scope:
  - Step-engine extraction.
  - Workspace/preflight refactors.
  - Store API redesign or transport changes.

## Test Plan (TDD)

- New failing tests to add first:
  - characterization coverage for the extracted event journal path, especially store append ordering / terminal handling at the seam.
- Existing tests to update:
  - `internal/harness/runner_forensics_test.go`
  - `internal/harness/runner_terminal_sealing_test.go`
  - `internal/harness/runner_store_durability_test.go`
- Regression tests required:
  - JSONL ledger still matches in-memory history.
  - Terminal events still seal the run before late events append.
  - Store append ordering relative to terminal observation remains intact.

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- This task does not touch provider/model flow surfaces.

## Implementation Checklist

- [ ] Define acceptance criteria in tests.
- [ ] For provider/model flow work, add or update the one-page impact map before implementation.
- [ ] Write failing tests first.
- [ ] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries.
- [ ] Implement minimal code changes.
- [ ] Refactor while tests remain green.
- [ ] Update docs and indexes.
- [ ] Update engineering/system/observational logs as needed.
- [ ] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: The extraction could subtly change terminal-event ordering or recorder/store timing.
- Mitigation: Add seam-level characterization coverage first, keep the extracted helper synchronous where ordering matters, and rerun the existing invariant suite after each change.
