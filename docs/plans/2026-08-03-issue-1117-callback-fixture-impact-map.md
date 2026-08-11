# Cross-Surface Impact Map: Issue #1117 Callback Claim Fixture

## Task

- Task / issue: #1117 deterministic callback duplicate-manager fixture.
- Plan link: `2026-08-03-issue-1117-callback-fixture-plan.md`.
- Owner: Codex.
- Status: in implementation; test/docs only, stacked on #1106 head `74e21270`.

## Current Ownership, Callers, and Data Flow

- Entry points: `TestCallbackManagerDuplicateManagersClaimOneDispatch` and
  `TestCallbackManagerRetriesTransientClaimContention` in
  `internal/harness/tools/delayed_callback_retry_red_test.go`.
- Source of truth: production `CallbackManager` and `SQLiteCallbackStore` are
  exercised unchanged through two SQLite handles and `callbackAdmissionStarter`.
- Search evidence: `rg -n "StartCallback|duplicate.*manager|claim.*contention|lease" internal/harness/tools --glob '*test.go'` located both fixtures and the
  dedicated heartbeat/lease tests. No new ownership abstraction is introduced.
- Conclusion: only fixture timing/assertion ownership changes; production data
  flow and consumers are untouched.

## Config, API, CLI, and Tools

- Config/API/CLI/tools: None. No flags, endpoints, schemas, wire formats, or
  tool definitions change.

## Persistence and Compatibility

- Schemas/migrations: None. The existing temporary SQLite database remains a
  test fixture.
- Compatibility/mixed version: None; #1106's current/legacy fencing contract
  is exercised but not modified.

## Lifecycle, Security, and Reliability

- Concurrency: removes an accidental test demand that a 30 ms lease survive a
  100 ms blocked admission. Retains direct process-fence failure and exact
  single admission/run assertions.
- Security/privacy: None; test IDs and local temp databases only.
- Failure/recovery: transient SQLite claim contention now also proves it does
  not create a second external starter admission.

## Product and Integration Surfaces

- Server/runtime, TUI/web/macOS, provider/model/catalog, external systems, UX:
  None. No production source or client artifact changes.

## Deployment and Operations

- Deployment/rollback: no runtime deployment. Revert the isolated test commit
  if fixture evidence proves inaccurate.
- Diagnostics: PR records the prior hosted diagnosis, exact stack base, and
  normal/race/full command evidence.
- Runbooks: none beyond testing/log records.

## Regression Tests

- Characterization/expected red: historical hosted run established the
  pre-change 30 ms lease plus 100 ms wait as an invalid fixture assumption;
  no production red is manufactured because this slice intentionally makes no
  behavior change.
- Acceptance: duplicate manager rejects second recovery; exactly one starter
  call, one durable attempt, one reserved run. Transient claim contention also
  asserts one starter call, attempt one, and the original run.
- Commands: focused normal/race `-count=100`; `go test ./internal/harness/tools`
  and `go test -race ./internal/harness/tools`; foreground
  `./scripts/test-regression.sh`.

## Documentation and Handoff

- Public specs: none.
- Implementation notes: plan, map, plan/log indexes, active plan, and three
  engineering logs.
- Handoff: PR uses `Closes #1117`, declares stack on #1106, and remains
  unmerged for independent review.
