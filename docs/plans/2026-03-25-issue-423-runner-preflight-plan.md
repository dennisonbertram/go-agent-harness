# Plan: Issue #423 runner preflight extraction

## Context

- Problem: `internal/harness/runner.go` mixes profile loading, workspace provisioning, system-prompt re-resolution, and per-run MCP setup directly into `execute()`.
- User impact: small runner setup changes are hard to review and regressions in workspace/profile behavior are easy to miss because the preflight contract is not explicit.
- Constraints: preserve existing run behavior, follow strict TDD, keep the extraction narrow, and avoid touching later step-loop/event-journal concerns.

## Scope

- In scope:
  - characterize the current preflight contract in tests
  - extract the `execute()` preflight/setup path into a focused helper or small component
  - keep profile isolation fallback, workspace events, prompt re-resolution, and per-run MCP setup behavior unchanged
- Out of scope:
  - event journal extraction
  - step engine extraction
  - transport or provider behavior changes outside current preflight flow

## Test Plan (TDD)

- New failing tests to add first:
  - direct preflight characterization for workspace provisioning failure emitting `workspace.provision_failed`
  - direct preflight characterization for profile-driven workspace fallback
  - direct preflight characterization for system prompt re-resolution against a provisioned workspace path
  - direct preflight characterization for per-run MCP registry setup
- Existing tests to update:
  - `internal/harness/workspace_selection_test.go`
  - `internal/harness/profile_mcp_test.go`
- Regression tests required:
  - `TMPDIR=$PWD/.tmp/tmp GOCACHE=$PWD/.tmp/go-build go test ./internal/harness -count=1`

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- This task does not change provider/model flow contracts, so no impact map is required.

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

- Risk: the extraction could subtly change event ordering or run-state initialization.
- Mitigation: pin current preflight behavior with direct tests before moving code and rerun the full `internal/harness` package after refactoring.
