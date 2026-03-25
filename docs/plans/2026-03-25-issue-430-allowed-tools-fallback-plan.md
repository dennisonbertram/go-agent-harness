# Plan: Issue #430 Allowed-Tools Fallback Integrity

## Context

- Problem: constrained agent and skill requests can lose `allowed_tools` restrictions when execution falls back to plain `RunPrompt(...)`.
- User impact: a supposedly restricted run can regain the default toolset, which breaks containment expectations and creates a security-sensitive contract drift.
- Constraints: strict TDD, scoped change only, and the resulting PR must be cleanly mergeable.

## Scope

- In scope:
  - `/v1/agents` fallback behavior in `internal/server/http_agents.go`
  - flat skill fallback behavior in `internal/harness/tools/skill.go`
  - core skill fallback behavior in `internal/harness/tools/core/skill.go`
  - any narrow runner API/helper needed to preserve tool constraints on fallback execution
  - regression coverage for restricted and unrestricted fallback behavior
- Out of scope:
  - unrelated runner/tooling refactors
  - new authorization semantics beyond preserving the existing `allowed_tools` contract

## Test Plan (TDD)

- New failing tests to add first:
  - `/v1/agents` fallback path keeps `allowed_tools` restrictions
  - flat skill fallback path keeps `allowed_tools` restrictions
  - core skill fallback path keeps `allowed_tools` restrictions
- Existing tests to update:
  - agent/skill fallback tests that currently only assert output behavior
- Regression tests required:
  - disallowed tools stay blocked on fallback paths
  - allowed tools stay permitted on fallback paths
  - omitted `allowed_tools` still behaves as unrestricted

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- None. This issue affects tool constraint plumbing on fallback execution paths, not provider/model flow surfaces.

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

- Risk: fallback-path fixes diverge between HTTP and skill surfaces.
- Mitigation: prefer one shared constrained fallback execution path and cover all three surfaces directly.
