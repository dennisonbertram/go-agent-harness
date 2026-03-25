# Plan: Issue 421 Config Runtime Contract

## Context

- Problem: `cmd/harnessd/main.go` only projects part of the merged `config.Config` into `harness.RunnerConfig`, so declared `auto_compact` and `forensics` settings can merge correctly but never affect the live runner.
- User impact: operators can believe runtime controls are active when the harness silently ignores them, and future config growth risks more drift.
- Constraints: keep the change narrow, preserve config/env precedence, use strict TDD, and avoid unrelated bootstrap refactors.

## Scope

- In scope:
  - Add failing tests for config-to-runner projection in `cmd/harnessd/main_test.go`.
  - Introduce one focused helper for projecting merged config into `harness.RunnerConfig`.
  - Wire the currently supported `auto_compact` and `forensics` fields deliberately.
  - Update docs/logs/indexes required by the repo process.
- Out of scope:
  - Broad `harnessd` bootstrap decomposition.
  - Changing runner semantics beyond honoring already-declared config.
  - New config fields or API shape changes.

## Test Plan (TDD)

- New failing tests to add first:
  - projection copies `auto_compact.enabled/mode/threshold/keep_last/model_context_window`
  - projection copies core `forensics` runtime fields and rollout directory
  - projection preserves explicit environment-derived runtime knobs alongside config-derived fields
- Existing tests to update:
  - `cmd/harnessd/main_test.go`
- Regression tests required:
  - `go test ./cmd/harnessd ./internal/config`
  - relevant full-package regression after the fix lands

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- Create a one-page impact map from `IMPACT_MAP_TEMPLATE.md` covering:
  - Config
  - Server API
  - TUI state
  - Regression tests
- A blank heading is a warning. Write `None` with rationale when a surface is truly unaffected.

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

- Risk: projecting fields incorrectly could change runtime behavior beyond the ignored-config bug.
- Mitigation: pin the intended mapping in focused tests before introducing the helper, then keep the helper small and explicit.
