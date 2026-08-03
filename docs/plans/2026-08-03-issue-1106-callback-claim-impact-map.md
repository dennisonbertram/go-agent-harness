# Cross-Surface Impact Map: Issue #1106 durable callback claim ownership

## Task

- Task / issue: #1106 callback claim race.
- Plan link: `2026-08-03-issue-1106-callback-claim-plan.md`.
- Owner: callback tools.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: durable timer/recovery invokes `CallbackManager.dispatchDurable`; heartbeat invokes `CallbackStore.ExtendLease`.
- Owning source: `internal/harness/tools/delayed_callback.go` owns dispatch lifetime; `delayed_callback_store.go` owns SQLite state transitions.
- Consumers: `CallbackRunStarter` creates the reserved run; lifecycle events bridge to conversation SSE and GUI/TUI replay.
- Search evidence: `rg -n "ClaimDue|ReclaimExpired|ExtendLease|CallbackStore" internal -g'*.go'` identifies only the manager/store and their tests.
- Conclusion: ownership must remain token-fenced at the store boundary and cancellation must be decided by the manager's last confirmed lease deadline.

## Config, API, CLI, and Tools

- User-facing config added or changed: None.
- Defaults/fallbacks: the existing lease duration/retry settings remain private manager defaults.
- Endpoints, CLI, wire formats: None; existing callback lifecycle event names and task response fields are retained.
- Errors/validation: database contention remains internal; no database error is exposed in callback status.

## Persistence and Compatibility

- Schema/migrations: None; existing `dispatch_token` and `dispatch_lease_until` columns are used.
- Compatibility: existing rows and token-fenced terminal transitions remain compatible.
- Mixed versions: a newer manager safely competes with an older manager through the existing columns; a stale/older owner cannot finish after token replacement.

## Lifecycle, Security, and Reliability

- Concurrency: every pooled SQLite connection receives WAL and busy timeout; claim/reclaim success is verified by both ID and caller token. Claim errors get bounded retry. Heartbeat treats a DB error as transient only through the last lease confirmed by a successful extend.
- Security/privacy: dispatch tokens remain non-serialized and are never surfaced in lifecycle events.
- Failure/recovery: definitive `ok=false` abandons immediately; transient errors cancel only after deadline, making later recovery takeover safe. Process-crash external side effects remain at-least-once by design, while durable run/conversation identity stays single.

## Product and Integration Surfaces

- Server/runtime: callback manager behavior changes only; harness/API/tui/macOS consume the same one durable callback run/events.
- TUI/web/macOS: no client code; fewer duplicate continuation turns is the intended visible improvement.
- Provider/model/tool catalog: None.
- UX/accessibility: None; no controls/copy change.

## Deployment and Operations

- Deployment/migration: ordinary binary rollout, no migration ordering needed.
- Observability: existing safe callback lifecycle states retain diagnostic truth; logs report contention without secrets.
- Rollback: revert source commit; no data rewrite needed.
- Runbooks: existing regression gate applies.

## Regression Tests

- Characterization/red: competing managers with transient heartbeat storage failure used to admit same reserved callback twice.
- Acceptance: only one starts, identical run ID remains durable, token mismatch is never reported as won, and deadline-expired owner cancels before takeover.
- Edge/failure: repeated busy, pooled connection pragmas, reclaim race, shutdown/cancel.
- Real path: full harness regression plus existing API/TUI/native conversation tests retain visible lifecycle paths.
- Commands: `go test ./internal/harness/tools -run 'Callback.*(DuplicateManagers|Lease|Claim)' -count=1`, `go test -race ./internal/harness/tools -count=1`, `./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs: no public contract text changes.
- Implementation notes: update logs and both indexes after green verification.
- Training/release notes: no onboarding or release-note change required.
