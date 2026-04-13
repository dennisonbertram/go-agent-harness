# Plan: Per-Run Sandbox Boundary Hardening

## Context

- Problem: `PermissionConfig.Sandbox` was stored on each run, but bash/job execution still enforced the registry startup sandbox instead of the current run's sandbox.
- User impact: continuing a conversation with a different permission set did not reliably change live tool behavior, which blurred the trust boundary between runs.
- Constraints: keep the fix narrow, preserve existing tool registration behavior, and avoid introducing cross-run races on shared tool state.

## Scope

- In scope:
  - Move sandbox resolution for bash foreground/background execution to the per-tool execution context.
  - Wire the current run's sandbox scope into the step-engine tool context.
  - Add regression coverage for start-run overrides, continuation overrides, and JobManager context overrides.
- Out of scope:
  - OS-level sandboxing implementation.
  - Redesigning the sandbox enum semantics beyond correcting mismatched comments.
  - Broader network policy changes for non-bash tools.

## Test Plan (TDD)

- New failing tests to add first:
  - `TestStartRunPermissionsSandboxOverridesRegistryDefault`
  - `TestContinueRunWithOptions_UpdatesSandboxBoundaryAtExecutionTime`
  - `TestJobManagerContextSandboxScopeOverridesDefault`
  - `TestJobManagerContextSandboxScopeBlocksBackgroundCommand`
- Existing tests to update:
  - none
- Regression tests required:
  - continuation/server permission override coverage remains green
  - full impacted package suites for `internal/harness`, `internal/server`, `internal/harness/tools`, and `internal/harness/tools/core`

## Cross-Surface Impact Map

- None. This change does not touch provider/model routing, config projection, TUI provider state, or API-key plumbing.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Write failing tests first.
- [x] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries.
- [x] Implement minimal code changes.
- [x] Refactor while tests remain green.
- [x] Update docs and indexes.
- [x] Update engineering/system/observational logs as needed.
- [ ] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: using shared mutable sandbox state would create cross-run races under concurrent execution.
- Mitigation: pass sandbox scope through `context.Context` per tool call and keep the manager-level sandbox only as a fallback default for non-run callers.
