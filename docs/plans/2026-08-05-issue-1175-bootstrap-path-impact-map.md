# Cross-Surface Impact Map: Issue #1175

## Task

- Issue: #1175 bootstrap provenance fixture path canonicalization.
- Status: implemented; focused normal/race and repository regression pass.

## Current Ownership, Callers, and Data Flow

- Owner: `internal/acceptance/bootstrapprovenance` test fixture.
- Flow: `t.TempDir` -> fixture root/worktree expectation -> `scripts/init.sh` -> Git porcelain path comparison.

## Config, API, CLI, and Tools

- None. No runtime interface changes.

## Persistence and Compatibility

- None. The fixture retains the same Git repository and child-worktree contract on all hosts.

## Lifecycle, Security, and Reliability

- No production lifecycle change. Security assertion remains strict after both expected and Git paths share canonical filesystem identity.

## Product and Integration Surfaces

- None: no server, TUI, GUI, provider, scheduler, or external integration changes.

## Deployment and Operations

- Test-only. Rollback is a one-file test revert.

## Regression Tests

- Full bootstrap provenance normal/race package and repository regression gate.

## Documentation and Handoff

- Plan, impact map, logs, and plans index record the fixture-only boundary.
