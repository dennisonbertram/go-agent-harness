# Cross-Surface Impact Map: Issue #1048 Pinned Retention Quota

## Task

- Task / issue: exclude subscriber-pinned terminal states from the drainable
  retention quota, #1048.
- Plan: `2026-07-30-issue-1048-pinned-retention-quota-plan.md`.
- Owner: Codex.
- Status: implemented and fully verified locally; hosted checks pending.

## Current Ownership, Callers, and Data Flow

- Entry: terminal completion/failure/cancellation calls
  `pruneCompletedRuns`.
- Source of truth: `Runner.runs`; only persisted terminal states with zero
  subscribers are deletion candidates.
- Current calculation: all persisted terminal states increment
  `terminalCount`, while only drainable states enter `candidates`.
- Caller impact: `StartRun` followed by `Subscribe` can lose a fast run when a
  pinned terminal state consumes the quota.

## Config, API, CLI, and Tools

- Config: retain `MaxCompletedRetention` and its default.
- API: `Subscribe` no longer loses the newest drainable run solely because a
  different run remains pinned.
- CLI/TUI/tools: wire shapes and commands unchanged.

## Persistence and Compatibility

- Persistent Store records/events: unchanged and never deleted by this path.
- Schemas/migrations/caches: none.
- Compatibility: memory may hold configured drainable states plus pinned
  exceptions, matching the existing after-subscribers-drain documentation.

## Lifecycle, Security, and Reliability

- Concurrency: existing Runner lock remains the sole synchronization boundary.
- Cleanup: subscriber cancellation re-runs pruning.
- Auth/privacy/secrets: none.
- Failure/recovery: persistent store fallback remains available.

## Product and Integration Surfaces

- Harness/API: fast completed runs remain subscribable under retention pressure.
- TUI/macOS GUI: run-scoped streams avoid a transient `run not found` failure.
- Conversation-level callback/cron streams: wire behavior unchanged; their run
  state remains stable while related run subscribers are pinned.
- Providers/web/accessibility: none.

## Deployment and Operations

- Deployment/migration/flags: none.
- Observability: fewer premature `run not found` errors.
- Rollback: revert if unpinned candidates exceed the configured cap after
  pruning or cancellation fails to reclaim pinned state.

## Regression Tests

- Red: hosted `collect extra run 2 events: run ... not found`.
- Green: focused normal/race `-count=100`.
- Controls: active subscriber remains pinned; the newest unpinned run remains
  available; older unpinned runs prune; cancellation restores the cap.
- Full: adjacent pruning, harness normal/race, and
  `./scripts/test-regression.sh`.

## Documentation and Handoff

- Update engineering log, long-term log, plan, impact map, and plans index.
- No public docs because behavior now matches the existing config contract.

## Warning Check

- Retention semantics and run-stream consumers are the only affected surfaces;
  persistence, conversation mirrors, tools, providers, and schemas were
  searched and are unchanged.
