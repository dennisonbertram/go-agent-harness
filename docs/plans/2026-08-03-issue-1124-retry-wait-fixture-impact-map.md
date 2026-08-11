# Cross-Surface Impact Map: Issue #1124 retry-wait fixture

## Task

- Task / issue: #1124 deterministic retry-wait recovery proof.
- Plan link: `2026-08-03-issue-1124-retry-wait-fixture-plan.md`.
- Owner: callback reliability lane. Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry: `TestCallbackManagerRecoveryHonorsRetryWaitAndReusesIdentity`.
- Source of truth: `CallbackManager.Recover` re-arms active SQLite rows;
  `SQLiteCallbackStore.ClaimDue` uses caller-provided `now` to enforce
  `next_attempt_at`.
- Test flow: persisted `retry_wait` row -> `Recover` -> manual `fire` before
  and after fake time advances -> store/starter assertions.
- Search evidence: `rg -n "retry|Retry|AfterFunc|sleep|Recover" internal/harness/tools -g '*.go'` identified the 60 ms deadline and 15 ms sleep as the only target.
- Conclusion: a test-owned, mutex-protected clock supplies a causal deadline;
  production clocks and timers remain untouched.

## Config, API, CLI, and Tools

- None. No environment variable, endpoint, CLI command, tool schema, or
  response field changes; the existing API task visibility contract is only
  asserted through durable state.

## Persistence and Compatibility

- No schema/migration/write-path change. The real SQLite test store verifies
  `retry_wait`, `next_attempt_at`, attempt, run ID, token, and lease fields.
- Mixed-version and recovery behaviour are unchanged; this does not alter
  callback state values or scheduling policy.

## Lifecycle, Security, and Reliability

- The fake clock is guarded by a mutex for normal/race safety. A real
  one-hour timer is stopped by manager shutdown; explicit `fire` controls the
  test's eligibility boundary without sleeping.
- No auth/privacy/secrets change. Exact token/lease-clearing assertions retain
  the existing ownership-fencing evidence.

## Product and Integration Surfaces

- Server/runtime: no product code change; callback recovery continues to own
  normal scheduling.
- TUI/web/macOS: none; no client behaviour is inferred from this fixture.
- Provider/model/tool catalog/external automation/UX: none, with source search
  limited to the callback test and durability seam.

## Deployment and Operations

- No flag, rollout, telemetry, runbook, or migration. Rollback is one PR
  revert if the test proves unsuitable.

## Regression Tests

- First red: focused Go test failed to compile with missing
  `newCallbackFixtureClock`, proving test-first fixture intent before helper
  implementation.
- Acceptance: normal/race x100 prove pre-deadline no admission and post-deadline
  same-run admission; tools normal/race and full regression guard neighbouring
  durable callback paths.
- Exact commands: `go test ./internal/harness/tools -run '^TestCallbackManagerRecoveryHonorsRetryWaitAndReusesIdentity$' -count=100`, the equivalent `-race`, `go test ./internal/harness/tools`, `go test -race ./internal/harness/tools`, and `./scripts/test-regression.sh`.

## Documentation and Handoff

- Adds this plan/impact map and updates plan/log indexes plus active and durable
  logs. No public documentation is changed because product behaviour is not.
