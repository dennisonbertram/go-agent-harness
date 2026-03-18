# Plan: Repo-Wide Zero-Coverage Gate

## Context

- Problem: `./scripts/test-regression.sh` is blocking pushes because the zero-function coverage gate reports uncovered functions across `./internal/...` and `./cmd/...`, and the current profile generation uses package-local coverage that misses repo-wide execution.
- User impact: The repo cannot be pushed even when the touched feature work is otherwise ready, because the shared regression gate fails before merge/push.
- Constraints:
  - Preserve the existing minimum total coverage and zero-function guardrails.
  - Keep runtime behavior stable.
  - Fix any regression blocker encountered on the way to the coverage gate if it prevents end-to-end verification.

## Scope

- In scope:
  - Update regression coverage collection to use repo-wide instrumentation where needed.
  - Add focused tests for the remaining truly unexecuted functions.
  - Rerun the regression script and verify it passes.
  - Update planning/logging docs for the new gate behavior.
- Out of scope:
  - Lowering the coverage threshold.
  - Removing the zero-function validation.
  - Unrelated feature work.

## Test Plan (TDD)

- New failing tests to add first:
  - CLI/TUI helper coverage tests for the remaining zero-covered functions in `cmd/harnesscli`, `cmd/harnesscli/tui`, and lightweight component packages.
  - Small utility coverage tests for `internal/forensics/redaction`, `internal/mcp`, and `internal/provider/openai`.
  - Entrypoint tests for `cmd/forensics` and `cmd/trainerd` `main()` functions.
- Existing tests to update:
  - Extend existing component/TUI test files where that keeps coverage assertions close to the current behavior under test.
  - Update the regression script to generate its coverage profile with repo-wide `-coverpkg` instrumentation.
- Regression tests required:
  - `go test ./internal/... ./cmd/...`
  - `go test ./internal/... ./cmd/... -race`
  - `./scripts/test-regression.sh`

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- This task does not change provider/model flow behavior. None.

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

- Risk: Switching to repo-wide `-coverpkg` changes which functions are considered executed and may expose a different residual zero list than the current package-local profile.
- Mitigation: Validate the before/after zero list explicitly and only rely on the instrumentation change where tests already exercise the code indirectly.
