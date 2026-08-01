# Impact Map: Issue #1003 authenticated remote cronsd harness dispatch

## Task

- Task / issue: GitHub #1003, remote `cronsd` harness execution child of #1000.
- Plan link: `2026-07-31-issue-1003-remote-cronsd-plan.md`.
- Owner: `cmd/cronsd`, `internal/cron`, and the authenticated harnessd HTTP
  boundary.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry point: `cmd/cronsd/main.go` loads the cron store, scheduler, and HTTP
  server; scheduled fires enter `cron.Scheduler.fireJob`.
- Source of truth: persisted `cron.Job` owns execution type, prompt config,
  tenant, agent, conversation, job ID, and scheduler-generated execution ID.
  `DispatchExecutor` owns shell-versus-harness selection.
- New flow: `cronsd -> DispatchExecutor -> HarnessExecutor -> RemoteRunStarter
  -> authenticated harnessd /v1/cron/runs -> Runner.StartRun`.
- Existing sibling searched: embedded `harnessd` wiring in
  `cmd/harnessd/bootstrap_helpers.go`; existing `internal/harnessmcp`
  HTTP client; server auth and `/v1/runs` route. No parallel shell fallback
  or second scheduler is introduced.
- Search evidence: `rg -n "ShellExecutor|DispatchExecutor|HarnessExecutor|RunStarter|NewServer|NewScheduler|/v1/runs|authMiddleware" internal cmd`.

## Config, API, CLI, and Tools

- Config added: `CRONSD_HARNESS_URL`, `CRONSD_HARNESS_API_KEY`,
  `CRONSD_HARNESS_CONNECT_TIMEOUT`, and `CRONSD_HARNESS_REQUEST_TIMEOUT`.
- Ingress config: mandatory `CRONSD_INGRESS_API_KEY` plus
  `CRONSD_INGRESS_TENANT_ID`; harnessd sends the former as
  `HARNESS_CRON_API_KEY`, while cronctl sends it as `CRONSD_API_KEY`.
- Defaults/fallbacks: URL and token have no fallback for harness jobs; timeout
  defaults are finite and bounded. Shell jobs do not need remote config.
- API: new authenticated `POST /v1/cron/runs` with prompt, tenant, agent,
  conversation, job, execution, and idempotency/correlation fields; returns
  `202 {run_id,status}`. `Idempotency-Key` must equal `correlation_key`; a
  per-tenant single-flight cache makes concurrent deliveries wait on one
  start, while an additive built-in-store record binds the authenticated
  tenant/key/fingerprint to the accepted run ID before `StartRun`.
  Sequential and restart-spanning replays return that run without another
  start; fingerprint conflicts return 409. Existing `/v1/runs` behavior is
  unchanged.
- CLI/tools: no model-facing tool or CLI schema changes; cron job creation
  remains the existing typed request.
- Errors: readiness and create-time validation reject missing remote config;
  execution records retain safe typed error text for auth, timeout, malformed,
  non-2xx, cancellation, and transport classes.

## Persistence and Compatibility

- Additive run-store migration: built-in stores retain cron-start tenant,
  idempotency key, fingerprint, and reserved run ID. Existing shell/harness
  job rows are read through the #1001 additive scope contract; the cron
  execution schema and #1004 terminal linkage remain unchanged.
- Fresh-store auth: harnessd persistence bootstrap applies the existing
  idempotent API-key migration immediately after the base run-store migration,
  so a first-boot `HARNESS_RUN_DB` can authenticate cronsd without test-only
  setup.
- Legacy shell jobs remain runnable. Legacy harness rows still use the stored
  prompt/config and must opt into remote config at deployment.
- Legacy ownership repair: run-store migration derives one normalized
  tenant/agent owner per existing non-empty conversation and inserts it in one
  immediate transaction. Historical ambiguity or disagreement with an
  existing owner fails startup without choosing or rewriting history. Cron
  tenantless shell rows use an atomic store claim; HTTP and daemon-startup
  races have one persisted tenant winner.
- Mixed versions: a new cronsd against old harnessd fails visibly on the
  dedicated endpoint; it never falls back to shell. Rollout is harnessd route
  first, then cronsd remote configuration.

## Lifecycle, Security, and Reliability

- Concurrency/cancellation: request contexts are bounded by connect/request
  timeouts, the persisted job scheduling timeout, and inherited parent
  cancellation; the earliest bound wins. Response bodies are closed and
  bounded. The server single-flight serializes only in-flight duplicate
  correlation keys and evicts completed entries; the run-store reservation is
  authoritative for sequential/restart replay. No application retry loop is
  added in this slice.
- Authentication: the standalone cronsd management API now requires a
  separate bearer token, derives tenant ownership from its static principal,
  rejects conflicting tenant input, and protects authenticated readiness plus
  every job/history route. Unauthenticated health exposes only liveness.
  The outbound bearer token is required by cronsd configuration and sent
  only to the configured harnessd URL; redirects are not followed. Harnessd uses existing API-key
  middleware plus `runs:write`; authenticated tenant is authoritative and its
  short API-key prefix is copied into run audit provenance.
- Privacy: prompt and credentials are excluded from logs and error strings;
  remote response bodies are not copied into execution errors.
- Failure handling: typed `RemoteRunError` includes code/status/retryability;
  remote deadline errors are recognized as scheduler `timed_out` executions;
  non-2xx and malformed responses are failed executions and never shell work.
  A reserved run must persist before dispatch or acceptance; a failed initial
  insert returns service unavailable and leaves no in-memory run. If shutdown
  wins after persistence but before dispatch, the replacement runner validates
  and resumes the exact queued reserved row before marking it accepted. If
  dispatch wins but the accepted-binding write fails, same-process retry
  reuses the active run and retries only that durable mark. Restart resume
  executes with the persisted model rather than a replacement daemon default.
  Accepted bindings whose queued work was drained by shutdown follow the same
  durable inspection and resume path. Omitted create timeouts default to 30
  seconds; explicit nonpositive create or PATCH values are rejected before
  persistence or dispatch, preserving the merged conversational CRUD contract.

## Product and Integration Surfaces

- Server/runtime: `cmd/cronsd`, `internal/cron`, `internal/server`, and the
  existing `harnessd` runner boundary change.
- TUI/web/macOS: None — no client routes or state are changed; the returned
  run is already visible through existing harnessd run/conversation APIs.
- Provider/model/tool catalog: None — remote harnessd uses its configured
  provider/model path; this slice only starts the run.
- External systems: HTTP/TLS connection between cronsd and harnessd; no cloud
  service or external automation contract is introduced.
- UX/accessibility: None — daemon/operator behavior only.

## Deployment and Operations

- Order: deploy/verify harnessd with `/v1/cron/runs` and its bootstrapped
  API-key schema, create a `runs:write` key, configure cronsd URL/token and
  finite timeouts, then enable harness jobs; absence of active harness jobs
  permits shell-only cronsd startup without remote configuration.
- Observability: structured safe log fields endpoint class, job ID,
  execution ID, HTTP status, latency, and retryability; no prompt/token.
- Rollback: unset remote harness configuration or stop harness jobs; shell
  jobs continue independently. A remote failure is not redirected to shell.
- Runbook: add operator config, readiness, canary, and rollback guidance to
  `docs/runbooks/remote-cronsd.md` and index it.

## Regression Tests

- First red: `RemoteRunStarter` request contract and missing-config validation.
- Acceptance: valid authenticated request returns stable run ID and preserves
  scope/correlation; shell dispatch is still shell; harness dispatch is never
  shell.
- Replay/provenance: concurrent, sequential, and reopened-store restart
  duplicate requests return the same run with one accepted `StartRun`,
  conflicting fingerprints are rejected, and the audit entry contains the
  authenticated API-key prefix. Completed process-cache entries are evicted.
  Dispatch-failure replay reuses the one queued reserved run ID rather than
  accepting an orphan or inserting a duplicate. Concurrent direct resume and
  same-process accepted-mark retry each prove only one dispatch. Accepted
  queued work drained by shutdown resumes on the replacement runner.
- Negative: missing/invalid auth, timeout/cancel, malformed body, non-2xx, and
  missing run ID; redirects never contact a target or forward credentials;
  no secret/prompt leakage in error text. Timeout/cancel while reading a
  successful but stalled response body retains typed transport classification.
- Lifecycle/security: active-job startup readiness, create-time validation,
  authenticated tenant mismatch, fresh-run-DB API-key create/validate, and no
  remote fallback. Reserved persistence failure does not dispatch or accept;
  job, parent, and transport deadlines use the earliest bound.
- Integration/real path: local cronsd and harnessd processes with a canary job;
  API endpoint and run status/scope are checked after dispatch.
- Exact commands: focused commands in the plan, `-race` packages, and
  `./scripts/test-regression.sh`.

## Documentation and Handoff

- Before code: this plan and impact map.
- After code: runbook, plans/runbooks indexes, engineering/observational/
  system/long-term-thinking logs, and PR evidence.
- Training/onboarding: no new model-facing behavior; runbook is the operator
  handoff.

## Warning Check

- All impact surfaces are explicit. `None` entries include searched reasons.

## 2026-08-01 Ownership Repair

- Durable ownership: ConversationStore establishes the transcript's tenant;
  the configured run store supplies the authoritative agent ownership for
  persisted remote runs after a runner restart. The authorization boundary
  denies every nonmatching durable row and fails closed on a run-store error.
- Regression: reopen both SQLite stores, deny a same-tenant second agent for
  the persisted conversation, then allow the original tenant/agent pair to
  continue it.
- Atomic first ownership: SQLite adds `conversation_run_owners`, keyed by
  conversation ID, and `CreateRun` claims normalized tenant/agent ownership in
  the same transaction as the run insert. MemoryStore applies the same contract
  under one mutex. Conflicts are typed; failed inserts leave neither a run nor
  a false owner. Existing run rows remain the restart/read-time defense for
  legacy databases.
- Concurrency proof: a barrier forces two different agents to observe no prior
  run, both in-process and across independent runners sharing SQLite. Exactly
  one claim, persistence operation, and dispatch succeeds.
- Ordinary-start ordering: `StartRun` performs its single best-effort
  `CreateRun` before recorder/state admission. An ownership conflict is the
  sole fatal persistence result and prevents dispatch; other store failures
  preserve the legacy non-fatal run behavior. Reserved starts retain their
  stricter all-persistence-is-fatal rule.

## 2026-08-01 Cross-Process Dispatch Lease Repair

- Persistence: `cron_run_starts` adds `dispatch_owner` and
  `dispatch_lease_until`; migration coverage starts from the previous table
  shape and proves lease acquisition after upgrade.
- Concurrency: SQLite's conditional update is the dispatch election boundary.
  The winning path returns its row from the same `UPDATE ... RETURNING`
  statement. In-memory storage uses the same contract under its mutex.
  Acceptance writes are fenced by owner so a stale process cannot acknowledge
  another process's dispatch.
- Runtime: initial reserved start and queued resume both acquire before runner
  admission. Competing servers poll only while acceptance is pending; accepted
  work returns its stable run identity while a live lease exists.
- Recovery: live queued/running ownership renews every 10 seconds; terminal,
  absent, shut down, or owner-lost runners stop. Lease expiry then permits
  takeover. SQLite's shared clock prevents a skewed caller from shortening the
  30-second recovery window.
- UI/TUI/macOS: no routes or payloads changed. Existing run and conversation
  APIs continue to expose the single reserved run.
- Out of scope: terminal `cron_executions.run_id` linkage and generalized
  overlap policy remain #1004.

## 2026-08-01 Lease Review Impact Addendum

- Runner lifecycle: one read-only `ShutdownSignal` lets lease heartbeats stop
  when shutdown begins; it does not alter shutdown ownership or cancellation.
- Persistence concurrency: acquired results never perform a later unguarded
  read. Renewal cannot acquire, and a lost/expired owner stops heartbeating.
- Migration concurrency: simultaneous old-schema startup may race on either
  additive column; after an ALTER error, migration succeeds only when a schema
  recheck proves the column now exists.
- Test seams: deterministic heartbeat tick/stop channels and SQLite
  post-mutation/migration barriers are nil in production.
- External effects: no provider API, tool protocol, or execution-linkage
  contract changed. Provider-side distributed exactly-once remains explicit
  residual risk outside #1003.
