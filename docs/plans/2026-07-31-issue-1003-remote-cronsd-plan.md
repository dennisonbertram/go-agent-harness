# Plan: Issue #1003 — authenticated remote cronsd harness execution

## Context

- Governing issue: GitHub #1003, child of epic #1000; dependency #1001 is
  closed and its typed `cron.RunStartRequest` contract is present on
  `origin/main` (`fedcf607`).
- Problem: `cmd/cronsd` constructs only `cron.ShellExecutor`, so persisted
  harness jobs are scheduled through the shell boundary instead of entering
  `harnessd`.
- User impact: a remotely scheduled harness continuation can be accepted and
  recorded as a fire while never starting the intended agent run; a prompt
  must never become a shell command as a fallback.
- Constraints: implement only #1003; preserve shell jobs; keep scope and
  correlation explicit; do not change scheduler persistence, overlap policy,
  callback delivery, macOS UI, or broader convergence work in #1010.

## Dependency and current-architecture evidence

- `origin/main` contains `internal/cron/executor_dispatch.go` with
  `RunStartRequest`, `RunStarter`, `HarnessExecutor`, and `DispatchExecutor`.
- `cmd/harnessd/bootstrap_helpers.go` wires embedded cron as
  `DispatchExecutor{Shell: ShellExecutor, Harness: HarnessExecutor}`.
- `cmd/cronsd/main.go` currently wires `ShellExecutor` directly.
- `internal/server/http_runs.go` already authenticates `POST /v1/runs` with
  the `runs:write` scope and enforces the authenticated tenant. The remote
  child needs a dedicated cron request contract so job/execution correlation
  cannot be lost or silently treated as a normal shell request.
- Search performed before implementation:
  `rg -n "NewScheduler|NewServer|ShellExecutor|DispatchExecutor|HarnessExecutor|CRONSD_|RunStarter" --glob '*.go' .`
  and
  `rg -n "type RunRequest|POST /v1/runs|authMiddleware|runs:write" internal cmd`.

## Scope

- In scope:
  - authenticated HTTP `RemoteRunStarter` for remote `harnessd`;
  - explicit JSON scope/correlation contract and safe headers;
  - URL/token configuration and bounded connect/request timeouts;
  - readiness/startup and create-time validation for harness jobs;
  - authenticated standalone cronsd ingress, authoritative single-tenant
    ownership, authenticated readiness, and credentialed management clients;
  - distinct shell/harness dispatch in `cmd/cronsd` with no fallback;
  - structured, retry-aware remote failures and redacted observability;
  - authenticated correlation/idempotency dedupe before `StartRun`, including
    a durable tenant/key/fingerprint-to-run binding that survives harnessd
    restart, audit initiator-prefix propagation, and remote-timeout
    persistence as `timed_out`;
  - reserved run persistence before dispatch/acceptance and an in-flight-only
    process cache whose completed entries cannot grow without bound;
  - persisted per-job scheduling deadlines composed with earlier parent and
    daemon transport deadlines;
  - fresh-run-store bootstrap migration of the API-key schema required to
    authenticate the configured cronsd bearer credential;
  - unit, race, configuration/readiness, and real local cronsd-to-harnessd
    canary evidence;
  - operator runbook, plan/impact map, logs, and indexes.
- Out of scope:
  - scheduler persistence redesign, cron overlap policy, execution-to-run
    persistence linkage from #1004, callback durability, callback delivery,
    remote retries beyond classifying failures, model/provider changes,
    TUI/web/macOS work, or epic convergence #1010.

## Contract

- `CRONSD_HARNESS_URL` and `CRONSD_HARNESS_API_KEY` are required only when an
  active or newly created job has `execution_type="harness"`.
- `CRONSD_INGRESS_API_KEY` and `CRONSD_INGRESS_TENANT_ID` are mandatory and
  distinct from outbound harness authentication. `/healthz` remains safe
  unauthenticated liveness; `/readyz` and `/v1/jobs*` require the ingress
  bearer. The authenticated tenant is authoritative for create, list, get,
  update, delete, history, and scheduler startup.
- `CRONSD_HARNESS_CONNECT_TIMEOUT` and `CRONSD_HARNESS_REQUEST_TIMEOUT`
  accept Go durations and have bounded defaults of 5s and 15s.
- `RemoteRunStarter` posts to `/v1/cron/runs` with prompt, tenant, agent,
  conversation, job, execution, and deterministic idempotency/correlation
  fields. `Idempotency-Key` must match `correlation_key`; harnessd durably
  reserves the accepted run ID per tenant/key/fingerprint before `StartRun`,
  returns that run after daemon restart, and rejects a key reused with a
  different request. The bearer token is sent only in `Authorization`.
- Reserved-ID starts synchronously persist the initial run before dispatch;
  persistence failure is a typed service-unavailable response and cannot mark
  the durable binding accepted. Ordinary non-reserved runs retain their
  historical non-fatal persistence behavior. If shutdown wins after that
  insert but before dispatch, the unaccepted queued reservation is validated
  and resumed with the same ID by the replacement runner before acceptance.
- `harnessd` authenticates the endpoint with its existing API-key middleware
  and `runs:write` scope, validates tenant ownership, starts a `RunRequest`,
  copies the authenticated API-key prefix into the run's audit provenance,
  and returns a stable `run_id` with HTTP 202.
- When `HARNESS_RUN_DB` creates or reopens the built-in SQLite run store,
  harnessd applies both the base run schema and the API-key schema before
  exposing authenticated endpoints.
- The remote client does not follow redirects: the bearer credential and
  replayable POST terminate at the explicitly configured harnessd origin.
- Shell jobs continue through `ShellExecutor`; a missing or failed harness
  remote never invokes shell execution.
- Timeouts, including typed remote deadline errors, persist as scheduler
  `timed_out` executions; cancellation, auth failures, malformed responses,
  non-2xx responses, and other transport errors become typed execution
  errors. The effective scheduling bound is the earliest parent deadline,
  persisted job timeout, or remote transport timeout. Logs include only
  endpoint class, job ID, execution ID, status, latency, and retryability.

## Documentation Contract

- Feature status: `implemented; full regression and local canary passed; awaiting PR review`
- Public docs affected: no user-facing client API; operator runbook/config
  reference is required.
- Spec docs before code: this plan and its impact map.
- Implementation notes after code: engineering, observational, system, and
  long-term-thinking logs plus indexes.

## Test Plan (strict TDD)

- Baseline: `GOCACHE="$PWD/.tmp/go-build" go test ./internal/cron ./cmd/cronsd -count=1`.
- First red tests:
  - remote starter sends exact scope/correlation JSON, bearer auth, and
    idempotency key;
  - auth failure, malformed/non-2xx response, timeout, and cancellation map
    to typed safe errors;
  - redirects are surfaced as non-2xx failures without contacting the target
    or forwarding credentials;
  - dispatch never sends a harness prompt to shell;
  - missing remote config rejects harness jobs while shell jobs remain valid;
  - harnessd cron endpoint preserves scope/correlation and requires auth.
  - concurrent and sequential duplicate cron POSTs return one stable run ID
    and invoke `StartRun` once;
  - recreating the server/runner over the same reopened SQLite store and
    replaying an accepted cron POST returns the original run ID without a
    second `StartRun`;
  - the cron start audit entry records the authenticated key prefix;
  - a freshly bootstrapped `HARNESS_RUN_DB` can create and validate a
    `runs:write` API key without a test-only migration call;
  - a typed remote deadline persists as `timed_out`, not `failed`.
  - reserved-run CreateRun failure returns 503, leaves the binding unaccepted,
    and dispatches no in-memory run;
  - a deterministic shutdown after reserved persistence but before dispatch
    leaves one queued row; replacement replay resumes that same ID once and
    later replay remains stable;
  - a one-shot accepted-binding failure after successful queued dispatch is
    healed by same-process retry without resume or duplicate dispatch;
  - concurrent direct resumes permit exactly one same-ID dispatch;
  - replacement-runner resume sends the persisted model to the provider even
    if the replacement daemon default changed;
  - accepted queued work drained at shutdown is resumed on restart rather than
    returning an inert accepted ID;
  - explicit PATCH zero/negative timeouts are rejected without persistence or
    dispatch; a valid persisted timeout bounds the triggered harness start;
  - timeout/cancel while reading a stalled 202 body stays typed and bounded;
  - job timeout and parent cancellation/deadline compose by earliest deadline;
  - the process cache coalesces only in-flight starts and is empty after
    completion, while durable sequential/restart replay remains stable.
  - real HTTP CRUD/history requires ingress auth, rejects tenant spoofing,
    hides cross-tenant rows, and keeps unauthenticated liveness minimal;
  - cronsd, harnessd remote-cron, and cronctl reject missing ingress client
    configuration before a management call can fail late.
- Green/targeted commands:
  - `go test ./internal/cron -run 'Test.*Remote|TestDispatchExecutor|Test.*Readiness' -count=1`;
  - `go test ./cmd/cronsd -run 'Test.*Harness|Test.*Readiness|Test.*Config' -count=1`;
  - `go test ./internal/server -run 'Test.*CronRun|Test.*Cron.*Auth' -count=1`;
  - `go test ./internal/cron ./cmd/cronsd ./internal/server -race -count=1`;
  - `./scripts/test-regression.sh` with zero failures.
- Real-path proof: run actual local `harnessd` and `cronsd` processes, create
  a harness job through cronsd, trigger it through the local schedule path,
  and verify the returned run ID/scope in harnessd. Capture exact commands,
  status, and sanitized logs in the PR.

## Implementation Checklist

- [x] Add this plan and impact map before source changes.
- [x] Write the first failing remote and readiness tests and record the red.
- [x] Implement the smallest typed remote adapter and dedicated endpoint.
- [x] Wire distinct dispatch and validation without changing shell behavior.
- [x] Add negative-path, timeout, auth, malformed-response, and no-fallback tests.
- [x] Update runbook, logs, and indexes.
- [x] Run targeted, race, full regression, and real local canary proof.
- [x] Commit only #1003 files; push and open a reviewable PR with `Closes #1003`.
- [x] Do not merge and do not claim acceptance.

## Risks and Mitigations

- Risk: bearer token or prompt leaks through logs/errors. Mitigation: never
  include either in errors or log fields; cap/ignore remote response bodies.
- Risk: the privileged outbound harness credential turns an unauthenticated
  cronsd endpoint into a prompt-launch capability. Mitigation: a distinct
  constant-time bearer check protects readiness and all management routes;
  the credential binds one authoritative tenant and every error is bounded.
- Risk: filtering the HTTP list while the scheduler still loads foreign rows
  would hide rather than isolate them. Mitigation: startup claims only legacy
  shell rows and fails on tenantless harness or foreign-tenant rows before the
  scheduler starts.
- Risk: two differently configured cronsd processes could each treat the same
  tenantless legacy shell row as locally owned. Mitigation: the SQLite store
  owns an atomic compare-and-set tenant claim; the winner receives the
  persisted row and every loser observes not-found/fails startup. Stores that
  cannot provide this contract keep unclaimed rows invisible.
- Risk: adding `conversation_run_owners` without backfilling existing runs
  leaves upgraded conversations open to a conflicting first claim. Mitigation:
  migration normalizes and atomically backfills the unique historical owner
  before new run claims. Multiple historical owners or a conflicting existing
  owner abort migration/readiness without modifying run or owner rows.
- Risk: remote endpoint is slow or unavailable and blocks scheduler workers.
  Mitigation: bounded dial, TLS, response, and request contexts.
- Risk: a harness job degrades to shell. Mitigation: explicit dispatcher and
  typed harness validation; no default executor fallback.
- Risk: mixed-version harnessd lacks `/v1/cron/runs`. Mitigation: fail with a
  visible typed non-2xx execution failure; rollout requires harnessd first;
  rollback removes remote harness configuration, never redirects to shell.
- Risk: a response can be lost after harnessd accepts a replayable POST.
  Mitigation: reserve the tenant/key/fingerprint-to-run binding in the
  built-in run store before `StartRun`; process-local single-flight handles
  same-server concurrency, while an owner-qualified expiring lease in that
  durable binding elects one dispatcher across harnessd processes sharing
  SQLite. The durable record returns the original run after restart.
- Risk: a first-boot run database has run/idempotency tables but no API-key
  table, making configured cronsd auth fail before dispatch. Mitigation:
  production persistence bootstrap applies the idempotent API-key migration,
  covered by a fresh-database create-and-validate regression.
- Risk: accepted bindings can outlive a failed initial run insert. Mitigation:
  reserved starts make initial persistence fatal before dispatch and expose a
  typed persistence error to the authenticated boundary.
- Risk: persistence succeeds but shutdown prevents dispatch, leaving an
  unaccepted queued row. Mitigation: replay validates that exact queued row
  against the reserved request, resumes the same ID, then marks acceptance.
- Risk: dispatch succeeds but its accepted-binding write fails. Mitigation:
  same-process replay checks current runner state before restart recovery and
  retries only the mark; runner insertion atomically rejects concurrent resume.
- Risk: accepted queued work is drained before entering a worker. Mitigation:
  accepted bindings inspect durable state and resume queued rows absent from
  current runner state.
- Risk: PATCH or a stalled success body bypasses deadline semantics.
  Mitigation: reject explicit nonpositive PATCH timeouts before mutation and
  classify request-context termination during bounded body decode.
- Risk: unique recurring execution keys accumulate in the process cache.
  Mitigation: retain entries only while a start is in flight; durable storage
  remains authoritative for every later delivery.
- Risk: ConversationStore retains tenant metadata but not agent metadata, so
  an in-memory cache loss could admit another agent in the same tenant.
  Mitigation: where a run store is configured, conversation-scoped durable run
  rows are the authoritative tenant/agent ownership source after restart;
  mismatches and run-store errors are denied. The recorded agent remains able
  to resume.
- Risk: two different idempotency keys can concurrently pass an empty
  conversation-owner read before either reserved run exists. Mitigation:
  built-in `Store.CreateRun` atomically inserts the first normalized
  conversation tenant/agent claim and the reserved run in one lock/SQLite
  transaction. A different owner receives `ErrConversationOwnerConflict`,
  which the Runner maps to `ErrConversationAccessDenied`; a failed run insert
  rolls the provisional owner claim back.
- Risk: ordinary `StartRun` historically admitted in-memory state before its
  best-effort `CreateRun`, so it could ignore that typed owner conflict and
  still dispatch. Mitigation: its one persistence attempt now occurs before
  state admission; only `ErrConversationOwnerConflict` is propagated as access
  denied, while every unrelated persistence failure remains logged/non-fatal
  and is not retried or inserted twice.

## 2026-08-01 Cross-Process Dispatch Lease Repair

- Command intent: prevent two harnessd processes sharing SQLite from both
  dispatching one reserved cron run during initial delivery or queued recovery.
- Strict red evidence: synchronized two-server delivery produced one duplicate
  reserved-run persistence error; synchronized queued recovery invoked the
  shared provider twice.
- Store contract: `cron_run_starts` now carries an additive dispatch owner and
  nanosecond lease expiry. SQLite acquisition is one `UPDATE ... RETURNING`
  statement evaluated against the shared SQLite clock; acceptance is
  owner-qualified, stale owners receive
  `ErrCronRunDispatchLeaseLost`, and an expired lease can be taken over.
- Server contract: every `StartRunWithID` and `ResumeRunWithID` follows a
  successful durable lease acquisition. A non-owner waits for pending
  acceptance or returns the stable accepted identity; it never dispatches.
- Liveness and recovery: a local queued/running run renews its 30-second lease
  every 10 seconds. Renewal stops on terminal state, local absence, runner
  shutdown, or owner loss. Only then can expiry make the exact queued row
  recoverable. Deterministic tests drive heartbeat ticks and durable expiry
  directly, without wall-clock sleeps, across independent runners/SQLite
  handles.
- Compatibility: migration adds both lease columns to an existing pre-lease
  `cron_run_starts` table. Concurrent legacy migrations recheck availability
  after a duplicate-column race. #1004 still owns terminal cron execution
  linkage and general run no-overlap semantics.

## 2026-08-01 Lease Linearizability and Heartbeat Review

- Strict reds reproduced four independent failures: acquired A returned B's
  owner after an A-update/B-takeover/A-read interleaving; a +24-hour caller
  clock stole A's live lease; concurrent legacy migration failed on duplicate
  `dispatch_owner`; and B dispatched A's live worker-backlogged run.
- `UPDATE ... RETURNING` makes an acquired result the row produced by that
  exact mutation. A non-winning path may read current state only after it has
  already fixed `acquired=false`.
- SQLite derives both expiry comparison and renewed expiry from its own clock;
  the server supplies only the lease duration. A skewed process clock therefore
  cannot authorize takeover.
- Owner-qualified renewal is separate from acquisition: it extends only an
  unexpired lease with the same owner and cannot resurrect an expired or lost
  lease. The server heartbeat is scoped to the local runner lifecycle.
- Boundary: a process paused past expiry between its last fencing check and an
  external provider side effect still requires provider-side idempotency or a
  deeper fencing token. That distributed exactly-once guarantee remains out of
  scope, as does #1004 terminal cron linkage.

## 2026-08-02 Frontier Review Repair

- Red-first repairs cover pre-admission lease loss, cancellation of duplicate
  in-process idempotency waiters, canonical remote credentials/URL handling,
  persisted-model resume preflight, and bounded authenticated DELETE bodies.
- The dispatch heartbeat now starts before `StartRunWithID` or
  `ResumeRunWithID`; losing ownership cancels their new context-aware
  pre-admission path before it can publish a run. Once accepted, the heartbeat
  keeps the prior queued/running lifecycle behavior.
- An external-package integration test assembles Scheduler -> HarnessExecutor
  -> RemoteRunStarter -> authenticated harnessd, verifies the accepted remote
  run and conversation scope, and intentionally leaves `Execution.RunID`
  linkage to #1004.
