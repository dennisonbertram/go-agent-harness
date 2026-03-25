# Plan: Issue #421 Config Runtime Contract

## Context

- Problem: `cmd/harnessd/main.go` manually assembles `harness.RunnerConfig` and currently drops merged `auto_compact` and `forensics` values that already exist in `internal/config.Config`.
- User impact: Operators can configure these surfaces successfully in TOML/env/profile layers and still get runtime behavior that silently ignores them.
- Constraints:
  - Strict TDD.
  - Keep the change narrow and behavior-preserving outside the config drift.
  - Do not weaken existing config precedence or merge semantics.

## Scope

- In scope:
  - Add failing tests for config-to-`RunnerConfig` projection.
  - Introduce one authoritative helper/builder for runner config assembly in `cmd/harnessd`.
  - Wire all currently-supported `auto_compact` and `forensics` fields deliberately.
  - Update plan/log/index docs required by repo workflow.
- Out of scope:
  - Large bootstrap decomposition in `cmd/harnessd`.
  - New config fields or new runtime behavior outside projection correctness.
  - Unrelated coverage-gate cleanup unless it directly blocks this issue’s mergeability.

## Test Plan (TDD)

- New failing tests to add first:
  - projection of `auto_compact` fields into `harness.RunnerConfig`
  - projection of `forensics` fields into `harness.RunnerConfig`
  - preservation of existing defaults/role-model behavior in the new helper
- Existing tests to update:
  - `cmd/harnessd/main_test.go`
- Regression tests required:
  - `go test ./cmd/harnessd`
  - `go test ./internal/config`
  - `./scripts/test-regression.sh`

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- Create a one-page impact map from `IMPACT_MAP_TEMPLATE.md` covering:
  - Config
  - Server API
  - TUI state
  - Regression tests
- A blank heading is a warning. Write `None` with rationale when a surface is truly unaffected.

This task does not change provider/model flow behavior, so no separate impact map is required.

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

- Risk: The helper accidentally changes unrelated startup defaults while centralizing config projection.
- Mitigation: Keep the helper small, preserve existing call sites, and assert unchanged core fields in focused tests.
