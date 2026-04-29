# Plan: Issue #557 Container Test Unique Name

## Context

- Problem: `TestContainerWorkspace_Provision_Success` uses the fixed workspace ID `test-provision`, which maps to Docker container name `workspace-test-provision`; a leaked container from an aborted run can make later runs fail with a Docker name conflict.
- User impact: developers and agents must manually run `docker rm -f workspace-test-provision` before they can rerun the workspace test after a failure or interrupted test process.
- Constraints: keep the fix scoped to the test leak, follow strict TDD, and preserve production container naming behavior unless the tests prove it must change.

## Scope

- In scope:
  - Add a failing-first test that requires the provision success test helper to generate a unique workspace ID per invocation.
  - Update `TestContainerWorkspace_Provision_Success` to use that unique ID.
  - Add cleanup for successfully provisioned containers so normal failures do not leak.
- Out of scope:
  - Redesigning `ContainerWorkspace` production naming.
  - Changing non-container workspace behavior.
  - Adding new public operator docs.

## Documentation Contract

- Feature status: `implemented`
- Public docs affected: None; this is test-infrastructure behavior.
- Spec docs to update before code: This plan captures the contract.
- Implementation notes to add after code: Engineering log entry for the bug fix and validation.

## Test Plan (TDD)

- New failing tests to add first:
  - `TestContainerWorkspace_Provision_TestIDUniquePerCall` requires two generated provision IDs with the same prefix to differ and remain readable.
- Existing tests to update:
  - `TestContainerWorkspace_Provision_Success` should use the unique ID helper and register `t.Cleanup` for `Destroy`.
- Regression tests required:
  - Targeted `go test ./internal/workspace -run 'TestContainerWorkspace_Provision_(TestIDUniquePerCall|Success)' -count=1`.
  - Full `./scripts/test-regression.sh` before handoff when feasible.

## Cross-Surface Impact Map

- Config: None; no runtime configuration changes.
- Server API: None; no HTTP behavior changes.
- TUI state: None; no TUI behavior changes.
- Regression tests: Workspace package tests cover the test-leak regression.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Document feature status and exact contract before code.
- [x] For provider/model flow work, add or update the one-page impact map before implementation.
- [x] Add characterization coverage before structural refactors.
- [x] Write failing tests first.
- [x] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries.
- [x] Implement minimal code changes.
- [x] Refactor while tests remain green.
- [x] Update docs, status ledgers, and indexes.
- [ ] Run full test suite. Blocked locally by sandbox network restrictions that prevent tests from binding `:0` / `[::1]:0`; see engineering log.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: The Docker-backed success test may be skipped or fail in environments without Docker or without the `go-agent-harness:latest` image.
- Mitigation: Add the deterministic helper regression separately from the Docker-dependent provision path, then report any environment-specific Docker blocker explicitly.
