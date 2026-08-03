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
- Conclusion: ownership remains token-fenced at the store boundary; an
  ordinary contender can claim only pending/retry work after the previous live
  owner has returned and released its token. Every filesystem-backed durable
  manager joins the sidecar `flock` before Set or dispatch. Current claims use
  the private persisted `dispatching_fenced` state, which the older binary's
  exact `state='dispatching'` reclaim cannot match. Old claims retain
  `dispatching`, which current recovery refuses to take over. Startup recovery
  may mutate only an expired `dispatching_fenced` row whose exact observed
  token still matches after kernel process-loss authority.

## Config, API, CLI, and Tools

- User-facing config added or changed: None.
- Defaults/fallbacks: the existing lease duration/retry settings remain private manager defaults.
- Endpoints, CLI, wire formats: None; existing callback lifecycle event names and task response fields are retained.
- Errors/validation: database contention remains internal; no database error is exposed in callback status.

## Persistence and Compatibility

- Schema/migrations: None; the existing textual state, `dispatch_token`,
  `dispatch_lease_until`, and `retry_wait` transition hold the compatibility
  fence and release/recovery handoff. A sidecar local lock file is operational
  metadata, not callback data.
- Compatibility: existing `dispatching` rows are deliberately preserved rather
  than guessed dead. New claims use `dispatching_fenced`; public manager/API
  reads normalize it to `dispatching`. Nullable leases are recoverable only on
  current fenced rows while workspace authority is held.
- Mixed versions: if old wins pending/retry admission, current fails closed on
  its `dispatching` row. If current wins, old claim and expired-reclaim SQL
  cannot match `dispatching_fenced`. Current crash recovery requires both the
  kernel lock and an expected-token CAS. Old-version rollback while a fenced
  row is active is fail-closed and requires draining/current-version recovery.

## Lifecycle, Security, and Reliability

- Concurrency: every pooled SQLite connection receives WAL and busy timeout;
  claim success and crash recovery are token-atomic. Claim errors rearm for the
  manager lifetime with cancellation-aware exponential backoff whose duration,
  not attempt count, is capped; no admission budget is consumed. At deadline,
  the admission returns before `ReleaseLease` clears its exact token to
  `retry_wait` with a safe API-persisted reason. Ordinary timers do not reclaim
  old `dispatching` rows.
- Security/privacy: dispatch tokens remain non-serialized and are never surfaced in lifecycle events.
- Failure/recovery: definitive `ok=false` abandons immediately; transient errors cancel only after deadline, then the live owner releases after its admission exits and re-arms its own bounded-backoff retry. A recovered future crash lease is rechecked at its timer deadline and converted through the same fenced transition. `Recover` fails closed without a filesystem process-loss fence and may convert only an expired or `NULL` current private fenced dispatch whose exact bootstrap token still matches; legacy public `dispatching` fails closed. Process-crash external side effects remain at-least-once by design, while durable run/conversation identity stays single.

## Product and Integration Surfaces

- Server/runtime: callback manager behavior changes only; harness/API/tui/macOS consume the same one durable callback run/events.
- TUI/web/macOS: no client code. This PR guarantees the durable state and safe
  error are available through the existing callback/API boundary. Actual
  client status/actions and full visible conversation continuation remain
  explicitly owned by #1007, #1009, and the assembled #1010 proof.
- Provider/model/tool catalog: None.
- UX/accessibility: None; no controls/copy change.

## Deployment and Operations

- Deployment/migration: ordinary binary rollout, no migration ordering needed.
- Observability: existing safe callback lifecycle states retain diagnostic truth; logs report contention without secrets.
- Rollback: revert source commit; no data rewrite needed.
- Runbooks: existing regression gate applies.

## Regression Tests

- Characterization/red: competing managers with transient heartbeat storage failure used to admit same reserved callback twice.
- Acceptance: an armed contender waits for old cancellation plus durable
  release, claim errors outlast the former finite cap and still dispatch the
  same ID, live old/current algorithms cannot overlap in either win order,
  stale expected-token recovery cannot clear a replacement, and a confirmed
  current-version process crash remains recoverable.
- Edge/failure: repeated busy, pooled connection pragmas, reclaim race, future crash lease expiry, retry-budget exhaustion, non-authority/in-memory recovery rejection, killed-process lock release, shutdown/cancel.
- Real path: full harness regression retains the existing API persistence and
  lifecycle contract. TUI/native presentation and full-conversation proof are
  not claimed by this backend PR and remain #1007/#1009/#1010.
- Commands: `go test ./internal/harness/tools -run 'Callback.*(DuplicateManagers|Lease|Claim)' -count=1`, `go test -race ./internal/harness/tools -count=1`, `./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs: no public contract text changes.
- Implementation notes: update logs and both indexes after green verification.
- Training/release notes: no onboarding or release-note change required.
