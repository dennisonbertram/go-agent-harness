# Cross-Surface Impact Map: Issue #1282

## Task

- Issue: #1282; plan: `2026-08-08-issue-1282-stateful-pty-barrier-plan.md`; owner: `internal/acceptance/ptyrunner`; status: in implementation.

## Current Ownership, Callers, and Data Flow

- `RunFreshConversation` and `RunNonMutatingCommandBatch` own the existing fixed-size PTY, collector, frame seals, API probes, and cleanup.
- New stateful coverage must reuse that ownership rather than shell sleeps. Search evidence: `rg -n "RunFreshConversation|RunNonMutatingCommandBatch|beginAction|waitAndSeal" internal/acceptance/ptyrunner`.

## Config, API, CLI, and Tools

- No public config, API, CLI, tool, or wire-format change. The test-owned fake daemon remains loopback/auth-disabled and writes only beneath its artifact root.

## Persistence and Compatibility

- No schema or migration. The test reads the existing run/conversation store to correlate the sealed frame with completed output.

## Lifecycle, Security, and Reliability

- One collector remains the only PTY reader. Each next input follows an immutable frame seal; cancellation/cleanup follow existing owned-process logic. No credentials or user data are read.

## Product and Integration Surfaces

- Server/TUI/web/macOS/provider behavior: none changed. The acceptance path proves server persistence and actual terminal rendering together.

## Deployment and Operations

- No deployment. Failure artifacts remain in the configured private temporary root; rollback is reverting the acceptance-only change.

## Regression Tests

- First red: stateful real 100x30 PTY batch has no implementation.
- New proof: first prompt/API completion/rendered frame, then title, dashboard, workflow, tasks, undo, plugins, and quit frames, with API/store correlation.
- Commands: focused normal/race then `./scripts/test-regression.sh`.

## Documentation and Handoff

- This plan/map, plan index, active plan, engineering log, and long-term thinking log track the acceptance-only conclusion.
