# Cross-Surface Impact Map: Issue #1144 transient callback heartbeat fixture

## Task

- Task / issue: #1144, causal proof for transient callback-heartbeat recovery.
- Plan link: `2026-08-03-issue-1144-transient-heartbeat-plan.md`.
- Owner: harness callback test maintainers.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry point: `TestCallbackManagerTransientHeartbeatBusyRetainsClaim`.
- Owning fixture: `internal/harness/tools/delayed_callback_retry_red_test.go`.
- Production path observed: `CallbackManager.dispatchDurable` calls the
  `CallbackStore.ExtendLease` interface; the wrapped SQLite store persists the
  token-fenced renewal.
- Search evidence: `rg -n "TransientHeartbeatBusy|transientLeaseStore|ExtendLease|leaseTime" internal/harness/tools`.
- Conclusion: the seam is test-only and delegates durable behavior to the real
  SQLite store; no production source change is required.

## Config, API, CLI, and Tools

- User-facing configuration: none.
- API/CLI/tools/TUI/web/macOS: none; no protocol, catalog, command, transcript,
  or visual behavior changes.
- Errors: fixture reports bounded causal-wait failures rather than a raw sleep.

## Persistence and Compatibility

- Schema/migrations/caches: none.
- SQLite persistence: read-only test verification of existing durable callback
  state after the real delegated extension.
- Compatibility/mixed rollout: none; channels are unexported test fixture data.

## Lifecycle, Security, and Reliability

- Concurrency: one-shot buffered channels record first injected failure and
  first successful delegated renewal without blocking the heartbeat.
- Lifecycle: LIFO cleanup releases the blocked starter before `Shutdown` waits
  for dispatch work.
- Security/privacy: no new inputs, authority, secrets, or network exposure.
- Failure/recovery: preserves the production transient-error path and proves
  same-token, attempt-one ownership after renewal.

## Product and Integration Surfaces

- Server/runtime: none; manager code is observed but untouched.
- Cron/callback product semantics: unchanged.
- Native GUI/TUI/harness API: unchanged; later acceptance work still must
  exercise visible continuation behavior independently.

## Deployment and Operations

- Deployment/migration: none.
- Observability: no production log changes; test diagnostics include causal
  lease deadlines and durable callback rows.
- Rollback: revert test/docs only.

## Regression Tests

- First red: test references causal-store gates absent from the pre-fix fixture.
- Acceptance: injected busy error occurs, a subsequent real extension succeeds,
  its durable deadline exceeds the captured initial deadline, and attempt stays
  one with the same dispatch token.
- Edge/lifecycle: cleanup releases a blocked starter before manager shutdown.
- Commands: focused normal/race `-count=100`, callback package normal/race,
  and `./scripts/test-regression.sh`.

## Documentation and Handoff

- Public docs: none.
- Internal notes: plan/impact map, four durable logs, and their indexes.
- Training/release notes: none; test-only reliability repair.
