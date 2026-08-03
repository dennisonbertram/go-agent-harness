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
- Conclusion: ownership remains token-fenced at the store boundary; an ordinary contender can claim only pending/retry work after the previous live owner has returned and released its token. Startup recovery additionally owns a sidecar filesystem `flock` for the full manager lifetime, so kernel process-loss—not lease-wall-clock expiry—authorizes expired dispatch recovery.

## Config, API, CLI, and Tools

- User-facing config added or changed: None.
- Defaults/fallbacks: the existing lease duration/retry settings remain private manager defaults.
- Endpoints, CLI, wire formats: None; existing callback lifecycle event names and task response fields are retained.
- Errors/validation: database contention remains internal; no database error is exposed in callback status.

## Persistence and Compatibility

- Schema/migrations: None; existing `dispatch_token`, `dispatch_lease_until`, and `retry_wait` transition are used for release/recovery handoff. A sidecar local lock file is operational metadata, not callback data.
- Compatibility: existing rows and token-fenced terminal transitions remain compatible; legacy nullable lease fields recover as abandoned only while the workspace recovery fence is held.
- Mixed versions: a newer manager safely competes with an older manager through the existing columns; a stale/older owner cannot finish after token replacement.

## Lifecycle, Security, and Reliability

- Concurrency: every pooled SQLite connection receives WAL and busy timeout; claim/reclaim success is verified by both ID and caller token. Claim errors get bounded retry. At deadline, the admission returns before `ReleaseLease` atomically clears its token to `retry_wait`; ordinary timers do not reclaim expired `dispatching` rows.
- Security/privacy: dispatch tokens remain non-serialized and are never surfaced in lifecycle events.
- Failure/recovery: definitive `ok=false` abandons immediately; transient errors cancel only after deadline, then the live owner releases after its admission exits and re-arms its own bounded-backoff retry. A recovered future crash lease is rechecked at its timer deadline and converted through the same fenced transition. `Recover` fails closed without a filesystem process-loss fence and may convert expired/legacy-null dispatch only while holding it. Process-crash external side effects remain at-least-once by design, while durable run/conversation identity stays single.

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
- Acceptance: an armed contender waits for old cancellation plus durable release, one manager re-arms its own released retry, identical run ID remains durable, token mismatch is never reported as won, a live workspace blocks a second bootstrap after expiry, and startup recovery handles both abandoned expired and legacy-NULL leases.
- Edge/failure: repeated busy, pooled connection pragmas, reclaim race, future crash lease expiry, retry-budget exhaustion, non-authority/in-memory recovery rejection, killed-process lock release, shutdown/cancel.
- Real path: full harness regression plus existing API/TUI/native conversation tests retain visible lifecycle paths.
- Commands: `go test ./internal/harness/tools -run 'Callback.*(DuplicateManagers|Lease|Claim)' -count=1`, `go test -race ./internal/harness/tools -count=1`, `./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs: no public contract text changes.
- Implementation notes: update logs and both indexes after green verification.
- Training/release notes: no onboarding or release-note change required.
