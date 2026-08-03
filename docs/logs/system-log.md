# System Log

## 2026-08-03 (Issue #1136 immutable timeout capability)

- `RunSubmission` privately binds owner token, generation, lifecycle, and a
  consumed bit. `RunSession` is the only authority that can consume it, and
  `cancelTimedOutSubmission` dispatches a transport-only A cancel only on that
  success. A handle-keyed task registry lets reset/load cancel all local streams.

## 2026-08-03 (Issue #1133 passive A outcome after B selection)

- Flow: ToolWalk captures `RunSubmission(A)` -> conversation stream selects B
  and marks A displaced -> Runner disables all automatic controls yet continues
  reading A-local lifecycle -> A terminal/failure is judged, or deadline sends
  the existing A cancel endpoint through a local-ownership fence.
- The selected-run reducer remains the only B UI authority. The displaced A
  timeout path intentionally performs no shared-state transition, so it cannot
  clear B pending controls, selection, transcript, or acknowledgement state.
- `RunSubmission` carries a session-owner token and reset/load generation plus
  a one-shot started-only timeout capability. It preserves A-only authority
  through B -> C replacement without reconstructing it from an ID/set; terminal
  or failure consume no capability, and reset/load cancels all live submission
  tasks while invalidating old handles.

## 2026-08-03 (Issue #1130 submission-local outcome flow)

- Flow: local composer/ToolWalk A -> `RunSubmission.lifecycle` plus
  `isDisplaced`; conversation SSE can select scheduled B without rewriting A.
  A start/stream tasks settle the handle, while `RunSession` shared transcript,
  accounting, and controls require exact active-handle identity and selected A.
- ToolWalk order is terminal -> failure -> displaced -> timeout. The first two
  are judged from A's transcript, displacement performs no automatic control,
  and only timeout calls existing expected-run cancellation for A.
- Reset/load synchronously displace and detach active A. A late response cannot
  select the replacement conversation; a cancelled detached task is not
  reported as a transport failure.

## 2026-08-03 (Issue #1128 submission lifecycle)

- Flow: Composer/ToolWalk -> `ProjectSession.submit` -> `RunSession.submit` ->
  `RunSubmission` -> `startRun` response assigns A -> A-only per-run SSE
  reduces the handle -> terminal/failure/displacement is observed by ToolWalk.
- A selected B synchronously marks a started A handle displaced. ToolWalk then
  performs no automatic input/approval/timeout action against B. Reset/load
  displaces unresolved submissions; a late server response exits before it can
  reactivate the reset session.

## 2026-08-03 (Issue #1125 native action owner)

- Ownership path: rendered Stop/Composer or ToolWalk timeout -> expected run ID
  -> RunSession guard -> existing run-specific HarnessClient cancel/steer endpoint.
  Mismatch returns before local force-stop, draft clear, Task, or network.

## 2026-08-03 (Issue #1007 Native External-Run Reducer)

- `RunSession` now has separate in-memory active/control ownership from
  transcript accounting. It rejects prior-conversation frames before mutation,
  preserves local provisional ownership until timestamped evidence, and keeps
  terminal run IDs as replay tombstones for the selected conversation.
- No harness API, persistence, scheduler, callback, cron, TUI, configuration,
  or deployment surface changes in this slice. Full local combined verification
  passes; hosted CI/review remain required before promotion.

## 2026-08-03 (Issue #1120 fixture contract)

- Component boundary: only callback test helpers and the blocked-heartbeat
  regression fixture change. The test exercises the existing process-wide
  recovery authority, durable private dispatch token, deadline cancellation,
  and token-CAS release unchanged.
- Required observable sequence: first manager starts; its heartbeat blocks;
  second manager fails process fencing; the original deadline cancels; the
  same claimed token releases to `retry_wait`; durable run ID/attempt remain
  stable; second manager has zero admissions.
- Verification: focused normal/race x100, complete affected package normal/race,
  and the isolated repository regression gate passed at 85.5% coverage with
  zero uncovered functions.

## 2026-08-03 (Issue #1117 fixture contract)

- System/component: callback manager test fixtures only. Production
  `CallbackManager`, `SQLiteCallbackStore`, fence, dispatch token, lease, and
  retry state machine are not changed. The duplicate-manager fixture validates
  live workspace authority rejection and a single durable starter admission;
  transient claim contention validates the same external-admission cardinality.
- Verification: focused callback ownership/claim contention normal x100 and
  race x100, then complete tools normal/race, pass with the strengthened
  assertions. The isolated repository-wide foreground gate also passes:
  normal/race, 85.5% total coverage, and zero uncovered functions.

## 2026-08-03 (Issue #1106 durable ownership participation)

- Every filesystem-backed durable callback manager acquires the common sidecar
  authority before `Set` or dispatch and retains it for the manager lifetime.
  `Recover` additionally requires that authority; unavailable authority fails
  closed. Authority installation is serialized per manager and is released
  after failed bootstrap, on shutdown, or by process exit. Current claims atomically
  enter private persisted state `dispatching_fenced`; old claims use public
  `dispatching`. Each version's literal claim/reclaim predicates therefore
  preserve whichever admission won first without cross-version takeover.
- Recovery is a two-part authorization: kernel release proves the previous
  current process is gone, and the exact token captured at bootstrap is the CAS
  precondition. It may recover only current private `dispatching_fenced` rows,
  including expired or `NULL` leases. Ordinary timers have no recovery token
  and cannot reclaim a live in-process admission. Legacy public `dispatching`
  rows remain fail-closed, even expired or `NULL`, because their process never
  participated in the kernel authority protocol.
- A deadline release records the owned sanitized `callback admission
  unavailable` retry reason in the durable row. Local claim contention retries
  until manager cancellation with a capped
  backoff duration only before ownership; it does not change callback
  admission-attempt semantics. Store state is private: manager/API/event/error
  boundaries normalize it back to `dispatching`.
- The durable/API status contract is covered here. TUI and native macOS status,
  actions, and visible full-conversation continuation remain #1007/#1009/#1010.

## 2026-08-03 (Issue #994 Terminal/Control Ownership)

- System/component: macOS `RunSession` control request task, per-run terminal
  SSE stream, and conversation/reset ownership transition.
- Ordering: terminal SSE may clear `currentRunID` before the POST completes.
  `runControlRequestGeneration` is the control result owner; it is incremented
  for a newer control and invalidated by `load`/`reset`, whereas same-run
  terminal lifecycle is intentionally not an invalidation boundary.
- Effect: a matching late control completion clears its in-flight state and
  optionally reports/restores its failure. Completion from an old session is
  ignored, preserving the selected conversation's UI state.
- Composer admission: `canSubmit` and `submit` share `runControlInFlight` as
  a hard boundary, so button and Return-key paths cannot start B while A's
  terminal-era control result still owns the session.

## 2026-08-03 (Issue #1108 Native Durable Reconciliation Fixture)

- System/component: native `RunSession` conversation SSE terminal handling and
  asynchronous `GET /v1/conversations/{id}/messages` reconciliation, exercised
  by `ConversationStreamStub`.
- Ordering boundary: terminal reducer state/accounting can become visible
  before durable transcript rows. Test gates therefore open only after accepted
  application state and final assertions wait for rendered assistant text.
- Product contract: unchanged; this isolates test observability of the existing
  GUI durable-continuation contract.
- Verification: release of C's durable fixture response occurs only after C
  owns visible terminal accounting; the final wait then requires the durable C
  assistant row and exact C state together.
## 2026-08-03 (Issue #1106 callback dispatch lease)

- System/component: `SQLiteCallbackStore` and `CallbackManager.dispatchDurable`.
- Ownership/order: a conditional SQLite `UPDATE ... RETURNING` installs and
  returns one dispatch token in one statement; only that returned token owns
  `MarkStarted`, `MarkRetry`, or `MarkFailed`. A manager remembers the expiry
  from its most recent successful claim/renewal.
- Failure boundary: a heartbeat `ok=false` is definitive loss and cancels
  immediately. A database error is transient only before the remembered
  deadline; expiry cancels admission, then the owner releases its exact token
  into retry-wait only after `StartCallback` has returned. Normal timers never
  reclaim an expired live row. Bootstrap recovery alone converts an expired
  abandoned row after process loss is confirmed. This preserves one durable
  reserved run/conversation turn, but does not assert exactly-once external
  effects across process crash.
- Deadline ownership: a dedicated guard timer cancels the admission at the
  last confirmed expiry even if the heartbeat is blocked inside SQLite. A
  successful renewal must arrive before the old guard deadline to reset it.

## 2026-08-03 (Issue #1102 — AskUser Waiting Lifecycle)

- Flow: AskUser tool call -> broker stores `AskUserQuestionPending` -> broker invokes `OnPending` -> `Runner.setStatusAndEmitContext` commits `waiting_for_user` and publishes `run.waiting_for_user` -> client/test observes status/event -> `SubmitInput` resolves broker -> runner emits tool completion, sets `running`, emits `run.resumed`.
- Test ownership: `PendingInput` validates pending-question payload; `Runner.Subscribe` is the synchronization source for public lifecycle visibility. No persistence, API, TUI, or macOS contract changes in this issue.

## 2026-08-03 (Issue #1006 Callback Dispatch State Machine)

- System/component: `CallbackManager`, `SQLiteCallbackStore`,
  `callbackRunStarter`, `Runner.EnsureRunWithIDContext`, callback event bridge,
  and `/v1/tasks` callback rows.
- Ownership/order: Set persists a callback-derived run ID before acknowledgement.
  One token owner claims due work, renews its lease during Runner admission, and
  may commit started/retry/failed only while its token still owns dispatching.
  Cancel can win only from pending/retry-wait; a dispatch claim wins as conflict.
- Recovery: pending uses `fires_at`, retry-wait uses `next_attempt_at`, and
  dispatching uses lease expiry. Re-admission always asks Runner to reconcile
  the same reserved identity, including a queued row left by a canceled/crashed
  owner, so post-admission callback-link recovery does not allocate another run.
- Visibility/security: task and lifecycle payloads expose stable run linkage,
  attempt, next-attempt, and allowlisted generic error summaries. Dispatch
  tokens, lease deadlines, and raw provider/store errors remain internal.
- Conversation replay: scheduling remains run-scoped while the run is live;
  later lifecycle changes are conversation-owned and never append after a
  terminal run. Startup enumerates every durable callback state, republishes
  one current-state lifecycle snapshot, then rearms active rows. This rebuilds
  restart-visible current state; it is not a complete historical event ledger.
- Failure truth: durable callback listing is an error-returning boundary. Task
  status, the agent list tool, and tenant-scoped cancel authorization return an
  error rather than substituting partial process memory, successful empty
  state, or a false not-found result.
- Timestamp compatibility: startup migration parses both #1005 driver-native
  and textual timestamp forms and rewrites them UTC before due/lease predicates.
  This makes cross-restart lexical SQLite comparisons deterministic and avoids
  zero-delay redispatch loops for already-overdue local-zone rows.


## 2026-08-03 (Deleted-Job Cron Recovery Boundary — Issue #1098)

- System/component: `internal/cron.Scheduler.reconcileExecutionRows`,
  `finishUnavailableExecution`, `reconciledLeases`, and `activeScopes`.
- Ownership/order: definitive `IsJobNotFound` is the only lookup result that
  may create an unavailable terminal record. Under the lifecycle gate the
  scheduler writes that record first, then resolves the recorded scope and
  releases its local/durable admission lease; it never touches a deleted job.
- Failure boundary: cancellation and transient lookup remain nonterminal;
  failed persistence retains the recovered lease and prevents duplicate scoped
  work. Stop retains its existing winner semantics.
- Verification boundary: test-only direct coverage executes both definitive
  missing-job variants and observes the same durable row/lease ordering; no
  runtime ownership or schema change was required.

## 2026-08-02 (Issue #1093 Conversation-Cleaner Lifecycle)

- System/component: `harness.ConversationCleaner`, `persistenceBootstrap`, and `runWithSignalsWithDeps` shutdown/unwind paths.
- Ownership/order: starting a cleaner returns its completion acknowledgement; the bootstrap-owned lifecycle cancels it and waits exactly once before `ConversationStore.Close`. Normal signal handling may call shutdown early, while the deferred owner makes startup-failure and later-return paths safe without double waiting.
- Inputs/outputs: no wire, configuration, or persistence contract changed. A positive existing retention policy starts the same daily cleaner; disabled retention returns an already-complete lifecycle.
- Reliability boundary: cancellation is no longer treated as proof of exit. Store closure is ordered after acknowledgement, so a cleaner cannot race a closed SQLite store.

## 2026-08-01 (Issue #1086 acceptance inventory flow)

`harness.Registry.DefinitionsWithMetadata()` and the daemon's `/v1/tools`
route represent the same resolved tool catalog. `tui.NewCommandRegistry().All()`
is the command source. `internal/acceptance/inventory` normalizes those snapshots
into canonical item IDs, a schema version, and a SHA-256 hash; later runners
attach proof records to item/surface pairs. The report command is read-only and
does not take ownership of tool execution, PTY state, macOS UI, persistence, or
scheduled-work continuation.

The default builder keeps each `tools.Tool` paired with owner and enabling
condition until `Registry.RegisterWithOptions` stores it. Core/deferred tiers
remain activation policy, not ownership inference. Dynamic MCP constructors add
the exact server tag; runtime MCP registration and tag-based replacement store
equivalent metadata directly. `/v1/tools` serializes the Registry snapshot's
owner/condition fields, and the acceptance compiler rejects generic direct-
registration provenance rather than guessing from a tool name.

Configured MCP servers that fail tool discovery now produce a paired,
redacted resolver snapshot: configured identity plus observed unavailable
reason/provenance. A typed per-call discovery error carries that snapshot while
healthy providers still contribute their partial catalog, avoiding mutable
last-result races during concurrent registry construction. The Registry copies
the snapshot, `/v1/tools` emits it additively, and the inventory command hashes
the resulting provenance-bearing not-applicable row. Evidence validation and
report rendering both resolve the exact compiled item and applicable surface.

`toolCatalog` requires both the present definitions and the paired resolver
snapshot. A snapshot can additionally be marked incomplete when a generic MCP
failure has no provider identity; the server then returns 503 and emits no
authoritative catalog. The inventory CLI requires non-null resolver arrays,
using explicit empty arrays as the only valid zero-unavailable representation.
TUI command entries carry owner/condition at their built-in, bundle-plugin, or
legacy-plugin registration boundary, and the compiler rejects missing command
provenance rather than supplying a default. Independent surface runners retain
the full inventory/hash and use selected-surface completeness validation.

Evidence schema v2 expands each TUI command item into inventory-derived
canonical and alias invocation requirements. Cases and evidence bind the stable
invocation ID; report rows remain separate. `EvidenceClass` distinguishes local
TUI behavior from conversation-backed behavior, controlling whether runtime
identities must be absent or present. Passes carry typed expected-postcondition
and observed-probe sets plus typed artifact references with SHA-256 and explicit
redaction declarations.

`SuiteContract` owns required runner-negative scenarios that cannot come from a
registry. It contains stable typed scenario IDs, is hashed with the complete
inventory hash, and is validated per selected surface. Suite evidence carries
both hashes, while suite rendering places synthetic scenarios in a separate
table so they cannot masquerade as registered commands or tools.

The suite contract also owns the complete `native_gui` applicability overlay.
Registry-derived tool rows cover API and TUI mechanically; each available item
must then be mapped to native `available` or `not-applicable` with source refs
and UX rationale. Suite validation derives native case completeness from this
hash-bound overlay. Passing native evidence additionally carries the four-part
screenshot/AX/raw-event/API-store artifact minimum and exact build, bundle,
daemon, and workspace-isolation metadata.
## 2026-08-03 (Issue #1038 Native Terminal Accounting)

- System path: harnessd accounting events -> `HarnessEvent` -> `Transcript.apply` -> `RunSession.streamConversation` terminal message reconciliation -> SwiftUI usage views.
- Contract: `usage.delta` reports cumulative per-turn fields; terminal `usage_totals` uses final `*_total` keys and `cost_totals` uses final cost/status fields. Either source can establish native usage, and a durable-message rebuild preserves the latest event-derived accounting because message rows do not contain it.
- Compatibility: no endpoint, schema, persistence, authentication, provider, or TUI change. Older terminal payloads with absent accounting preserve prior delta-derived values.
## 2026-08-03 (Issue #994 Native Run-Control Acknowledgement)

- System: SwiftUI `ApprovalBar`, `PlanApprovalView`, `AskUserView`, and composer controls call the single `RunSession` acknowledgement boundary, which uses existing `HarnessClient` cancel/approve/deny/steer/input endpoints.
- Ownership: `RunSession` retains the active run ID, request generations, `pendingQuestions`, typed draft, and action flags. A completion may update state only when it still owns the matching run and generation; reset/load invalidates outstanding ownership.
- UX: controls disable while a request is pending; a failed action exposes the retryable server/transport message in `InlineRunStatus`, which is announced politely to assistive technology. Structured answers must be trimmed and nonempty and stay editable until acknowledgement.

- Review-repair state machine: steer guards before draft consumption; retry clears stale local error; approve/deny transition `requesting -> acknowledged` only after HTTP success and leave the shared control disabled until a same-run grant/deny/terminal SSE frame increments that run's lifecycle generation. A stale run is filtered before this transition.
## 2026-08-03 (Workflow Subscriber Terminal Close — Issue #1115)

- System/component: `internal/workflow.Engine.Subscribe`, `subEntry`, `Engine.emit`, and the script-workflow SSE history/live consumer.
- Ownership/order: `emit` sequences, persists, and performs nonblocking fan-out while holding `Engine.mu`; on completed/failed events it closes every currently registered channel and deletes the run's subscriber map in that same critical section. `cancel` uses map membership under the same mutex, preventing send-on-closed and double-close.
- Initialization boundary: while `Subscribe` copies history without the engine lock, `subEntry.pending` captures later events in order. If terminal occurs during that copy, `emit` closes the channel and removes the map while the retained entry pointer still folds pending events into returned history.
- Replay boundary: a subscriber registered after terminal transition is not retroactively closed. It receives terminal history; the SSE handler short-circuits on that terminal record and always invokes cancellation.
- Test boundary: #1115 gates the script until registration, fills the 64-slot live channel, and proves ordered buffered reads followed by closure. Runtime code, persistence, API, callbacks, crons, and product clients do not change.

## 2026-08-01 (Issue #1081 Keychain Parser Coverage)

- System/component: `internal/modelstore/credref.go:keychainParts` and the
  regression coverage gate.
- Responsibilities: parse the target portion of a Keychain credential
  reference into service and account; on Linux, `KeychainAvailable` prevents
  Keychain process execution before this parser is reached.
- Inputs/outputs: a `<service>/<account>` target produces two strings; missing
  separator, service, or account returns the existing validation error.
- Dependencies: Darwin real-path verification still depends on `security(1)`;
  portable unit coverage depends on no external credential service.
- Operational note: hosted Ubuntu run `30672776651` exposed the missing direct
  coverage after normal and race suites completed; the repair does not change
  runtime behavior, persistence, or secrets handling.

## 2026-08-01 (TUI Assistant Response Reconciliation)

- System: run SSE -> TUI bridge -> `Model.Update` -> viewport bubble ->
  `SSEDoneMsg` -> transcript export.
- `assistant.message.delta` builds incremental content;
  `assistant.message` is the authoritative full response and may arrive without
  deltas.
- Tool start is an assistant-tail ownership boundary. A later provider response
  begins a new bubble and cannot replace the intervening tool card.
- Terminal transcript finalization consumes the current run once; replayed
  assistant/completion events are no-ops, and `RunStartedMsg` opens the next
  run's lifecycle by clearing both the prior assistant accumulator and its
  finalization state. This boundary covers initial and continuation API starts.
- Server emission, persistence, authentication, provider behavior, and other
  clients remain unchanged.

## 2026-08-01 (Conversational Cron CRUD Ownership — Issue #1002)

- Components: every model registry constructor -> one idempotent scoped cron client -> deferred `cron_*` tools -> embedded adapter or HTTP client/server -> `SQLiteStore` -> `Scheduler`. Operator/server endpoints retain the raw adapter outside this boundary.
- Identity contract: model get/update/history/pause/resume/delete accept job IDs only. Explicit operator name lookup uses `/v1/jobs/by-name?name=...`; query encoding round-trips every non-empty allowed name. Unscoped collisions return typed `ErrJobAmbiguous`, while scoped lookup selects by the complete ownership tuple.
- Persistence contract: non-deleted names are unique within `(tenant_id, conversation_id, agent_id)`. SQLite index metadata identifies a non-partial, one-key-column global name constraint regardless of DDL spelling/collation; legacy global uniqueness is transactionally rebuilt with jobs and executions copied before old tables are dropped.
- Lifecycle contract: create and paused→active resume are paused-first, so registration or activation failure retains a paused restart-safe row. Active schedule replacement is inert `Prepare` → durable CAS → infallible in-memory `Commit`; failed prepare/CAS leaves the old durable row and live entry untouched. Registration identities are monotonic and are checked again after jitter/reload before execution allocation, suppressing queued stale callbacks. Pause/delete remove live dispatch under the same mutation lock.
- Concurrency/security boundary: remote and embedded model paths apply tenant/conversation/agent predicates at store lookup before reading history or mutating. Update/pause/resume/delete require the version returned by `cron_get`; stale model calls return typed conflict/HTTP 409. Concurrent update/delete serializes to either update-then-delete or delete-then-not-found, never a post-delete re-arm. Authentication of raw cronsd requests remains #1003.
- Provider boundary: every initially visible tool schema has a top-level object
  shape without provider-forbidden root composition. Shell-versus-harness
  pairing remains enforced by the cron handler, so provider compatibility does
  not weaken execution validation.
- Compatibility/rollback: existing jobs/history and shell/harness payloads remain readable. Operator callers must use the distinct name route; reverting this slice requires restoring the global identity policy only if duplicate scoped names have not been created.
## 2026-08-01 — cronsd authenticated management boundary

- Flow: harnessd or cronctl sends `Authorization: Bearer <ingress key>` to
  cronsd; cronsd resolves the configured tenant principal, rejects conflicting
  body tenant data, and scopes CRUD/history before reaching the store or live
  scheduler. Scheduled harness dispatch then uses the separate outbound
  `CRONSD_HARNESS_API_KEY` at harnessd's `/v1/cron/runs` boundary.
- Lifecycle: configuration and persisted-job ownership validate before
  scheduler start. `/healthz` reports process liveness; authenticated
  `/readyz` proves the management boundary is configured and reachable.
- Compatibility: legacy tenantless shell rows are assigned to the configured
  instance tenant. Tenantless harness and foreign-tenant rows fail closed.
- Persistence invariant: a legacy shell job is visible only after
  `ClaimJobTenant` returns the persisted tenant winner. Conversation owner
  migration similarly derives one normalized owner from legacy runs in an
  immediate transaction; disagreement blocks migration/readiness.

## 2026-08-01 (Issue #1003 durable conversation authorization)

- Restart-time conversation authorization combines transcript-store tenant
  metadata with run-store tenant/agent records when the latter is configured.
  A run-store read error is returned as an ownership-check failure, never
  treated as a new conversation.
- First ownership is serialized by `Store.CreateRun`: MemoryStore owns a
  conversation claim map under its run mutex; SQLite owns the additive
  `conversation_run_owners` row and run insert in one transaction. Reserved
  conflict errors cross the Runner boundary as conversation access denial.
- Ordinary `StartRun` now calls that same store boundary once, before local run
  state becomes visible. Owner conflict aborts admission; generic persistence
  failure retains the prior log-and-continue contract. `ContinueRun` keeps its
  inherited-owner, non-fatal helper path.

## 2026-07-31 (Source-Workflow Initial Write Lifecycle)

- System/component: `internal/workflow.SourceManager.runSourceWorkflow`, the
  initial parent-to-child `start` write, process-group cleanup, and
  `sourceWorkflowOutcome`.
- Ownership/order: every successfully started child remains parent-owned until
  one close/wait path reaps it. Terminal resolution remains deadline, semantic
  protocol, cleanup-attributed initial-write failure, natural non-zero process
  exit with bounded stderr, standalone initial-write error, later close error,
  then missing result or success.
- Inputs/outputs: add the already-observed initial-write error to the internal
  outcome record; no API, CLI, protocol, persistence, config, or client schema
  changes.
- Reliability/security boundary: write/protocol failures still terminate the
  process group, wait occurs exactly once, and stderr remains limited to
  `maxWorkflowStderrBytes`.
- Termination attribution: the runtime records whether initial-write cleanup
  successfully requested process-group SIGKILL and whether `cmd.Wait` observed
  that signal. That wait status is cleanup evidence rather than a new primary
  failure; natural exit statuses and other signals retain process-failure
  precedence. Exact concurrent same-signal provenance is outside this narrow
  lifecycle contract.
- Rollback boundary: revert if timeout/protocol precedence, standalone
  transport errors, process reaping, successful results, or stderr bounds
  change.

## 2026-07-31 (Runner Dispatcher Shutdown Identity)

- System/component: bounded `internal/harness.Runner`, `poolDispatcher`,
  `done`, and `dispatcherWG`.
- Ownership/order: each Runner closes only its own `done`; its dispatcher calls
  an optional narrow test hook and then `dispatcherWG.Done`; Shutdown waits
  that same instance wait group after inflight work is accounted for.
- Test boundary: one target Runner's hook supplies lifecycle identity while a
  second control Runner remains live. Process-global stack inspection is used
  only to prove the control still exists, never to classify target cleanup.
- Worker-pool fixture boundary: the shared test constructor owns cleanup for
  bounded test Runners. Its cleanup first releases provider gates, then calls
  the public `Shutdown` boundary; this preserves the same production ownership
  contract and prevents dispatchers surviving across `-count` repetitions.
- Compatibility/failure modes: no API, config, persistence, client, provider,
  or tool contract changes; existing queue-drain, timeout, and idempotency
  behavior remains the rollback boundary.

## 2026-07-31 (Terminal Run Transition Publication — Issue #1067)

- System/component: `Runner.transitionTerminal`, `Runner.emit`, and
  `eventJournal` terminal persistence/fanout in `internal/harness`.
- Responsibilities: the event journal remains the only event-ledger writer and
  terminal-seal owner; the transition seam binds the winning terminal event to
  its completed, failed, or cancelled `Run` record.
- Order: prior causal/error events -> terminal ledger append/seal -> bounded
  event-store append -> ordered terminal recorder dispatch/drain -> matching
  conditional status persistence -> matching in-memory status -> subscriber
  fanout -> backup/pruning lifecycle.
- Concurrency boundary: store and recorder I/O remain outside `Runner.mu`.
  A per-conversation sequence guard preserves replay-to-live ordering across
  the whole transition. The global `conversationEventMu` is released around
  recorder and status-store I/O, so unrelated conversation journals progress;
  the in-memory status commit briefly reacquires only the Runner state lock.
- Consumers: `GetRun`, run summary, run SSE replay/live delivery,
  conversation replay, CLI/TUI exit handling, and macOS transcript state keep
  existing schemas and event IDs.
- Failure boundary: retained terminal `UpdateRun` is attempted only after
  `AppendEvent` reports success. Append failure leaves durable status
  non-terminal; later status-update failure/timeout may leave durable terminal
  event ahead of durable status. Both remain non-fatal to bounded in-memory
  status/fanout, so this is explicitly one-way rather than transactional
  two-way atomicity. Terminal redaction drops remain the explicit no-event
  exception and now drain the recorder before publishing status.
- Status/resource boundary: every status transition shares a per-run mutex, so
  delayed non-terminal writes cannot overwrite terminal state. The
  per-conversation sequence lock counts queued waiters and deletes idle keys;
  external terminal I/O never holds the global conversation journal mutex.
- Retention/admission boundary: store-backed terminal pruning requires event
  persistence (or intentional StorageModeNone suppression) plus acknowledged
  final status persistence. Both unresolved event and status records consume
  the `MaxCompletedRetention` durability backlog. Once full, StartRun and
  ContinueRun retry status-only gaps under one shared deadline capped at 250 ms
  with no store I/O under Runner/status/journal/conversation locks, then return
  typed fail-closed backpressure if unresolved.
- API/recovery boundary: the two run-admission HTTP routes map typed durability
  backpressure to 503 `terminal_durability_unavailable`; Continue validates
  missing/non-completed sources first and revalidates before its single-winner
  mutation. Successful status retry immediately prunes newly durable candidates
  back to the configured retention limit before reopening admission. Ambiguous
  append errors remain blocked until process/operator recovery rather than
  risking a duplicate forensic event. No-store runs intentionally remain
  process-local and ungated.
- Continuation reservation boundary: source validation and reservation are one
  `Runner.mu` mutation; unlocked durability recovery follows; the existing
  write-lock revalidation remains the only single-winner mutation. The shared
  prune candidate filter excludes nonzero reservations regardless of caller.
  Deferred release decrements the in-state counter and immediately invokes the
  shared lock-held prune policy, so no keyed side map or stale reservation entry
  survives success, backpressure, revalidation loss, or dispatch failure.
- Test settlement boundary: `collectRunEvents` and its configurable-timeout
  variant retain terminal history exactly, require exactly one terminal event,
  then use the remaining shared deadline to require the matching terminal
  `GetRun` status. Closed streams without a terminal event and contradictory
  event/status pairs fail explicitly. This is test-only and does not move the
  production commit or fanout boundaries. Direct phase tests bypass the settled
  helper and continue observing the intentional publication window.

## 2026-07-31 (Source-Workflow Terminal Error Arbitration)

- System/component: `internal/workflow.SourceManager.runSourceWorkflow` and its
  internal `sourceWorkflowOutcome` resolver.
- Inputs: workflow result, deadline state, protocol error, stdin-close error,
  process wait error, and bounded child stderr captured after protocol serving.
- Ownership/order: the workflow runtime resolves deadline first, protocol
  second, process exit third, stdin-close cleanup fourth, then missing result
  or success. Cleanup still closes stdin and waits for the child exactly once.
- Consumers: failed-run error strings flow through the existing workflow engine
  to API/CLI/TUI/web/macOS clients; no client-specific error mapping changes.
- Failure boundary: a close-only failure remains visible, while simultaneous
  close and wait failures report the process exit and bounded stderr.

## 2026-07-30 (Conversation Event Replay and GUI Reconciliation)

- System/components: `store.ConversationEventReader`, the memory and SQLite run
  stores, `Runner.SubscribeConversationFrom`, the conversation SSE handler,
  and macOS `ProjectSession.syncCurrentConversation`.
- Source of truth and flow:
  - run events retain their existing `<run-id>:<seq>` public identity;
  - SQLite `run_events.id` supplies global append order across every run in a
    tenant/conversation, without introducing a second wire cursor;
  - subscription registration and replay snapshot creation share the same
    runner boundary as event persistence/fanout, preventing a reconnect gap;
  - clients consume bounded replay pages, then remain attached for live events;
  - the macOS app also fetches persisted messages when Chat reappears, so
    completed scheduled turns are restored even if no stream was open;
  - each terminal conversation replay event reconciles persisted messages,
    preventing a freshly opened historical snapshot from being rendered a
    second time by the complete event replay that follows it.
- Fallbacks: a runner without a compatible durable store retains the newest
  4096 conversation events in process memory. A store-query failure logs the
  error and uses that bounded journal; it cannot recover pre-restart history.
- Recovery metadata:
  - an unknown non-empty cursor returns
    `X-Harness-Conversation-Resync: required` and replays retained history;
  - a full replay page returns `X-Harness-Conversation-Replay: more`, closes
    after the page, and lets the client reconnect from its last exact event ID.
- Security boundary: every durable query requires conversation scope and
  optionally tenant scope in addition to the endpoint's existing owner and
  `runs:read` checks. Event payloads and identifiers retain their prior
  compatibility and secret-handling contract.
- Failure modes: stale cursors require a full retained-history resync; the
  no-store fallback is process-local and bounded; transcript reconciliation is
  skipped while a user-started run is active so it cannot overwrite local
  in-flight rendering.

## 2026-07-29 (Issue-Driven Engineering Process Boundary)

- System/component: GitHub Issue Forms, `.github/pull_request_template.md`,
  `AGENTS.md`, `CLAUDE.md`, issue/plan/runbook documentation, and
  `internal/quality/repostructure/issue_process_test.go`.
- Responsibilities:
  - Issue Forms own work classification, acceptance, architecture/search
    evidence, impact analysis, TDD plan, verification, rollout, and handoff.
  - The PR template owns final reconciliation of the issue contract with the
    actual diff and evidence.
  - Agent instructions own the before-implementation stop condition and
    classification rules.
  - Repository drift tests own the static contract between the five forms, PR
    template, private-security route, and agent-policy language.
- Inputs/outputs:
  - Input: issue authoring and PR review by humans or agents.
  - Output: an auditable issue-to-plan-to-tests-to-PR narrative with explicit
    scope, impact, evidence, and rollback.
- Security boundary: confidential vulnerabilities use private reporting rather
  than public issue content.
- Restricted work:
  - epics cannot ship directly;
  - research PRs are documentation-only;
  - minor PRs are documentation-only, at most five files and 150 changed lines.
- Failure modes:
  - an agent may ignore policy because no GitHub technical gate exists in the
    pilot;
  - generic or dishonest form answers can still pass static structure tests;
  - changing a required form/PR field or removing agent policy breaks
    repository tests before merge.
- Operational note: this pilot measures whether explicit agent instructions and
  high-friction authoring are sufficient. Tests and review still own
  correctness, architecture quality, scope discipline, and real-path proof.
- Regression integrity: the rollout also restored meaningful execution coverage
  for the nine seams recorded in Issue #989. The repository gate remains
  unchanged at at least 80% aggregate statement coverage and zero uncovered
  production functions.

## 2026-07-20 (Issue #854 — Live TUI subscription import)

- System/component: `/keys` in `harnesscli`, `/v1/providers/{name}/import-subscription`, and the existing Codex/Kimi stores plus provider registry.
- Flow: pressing `i` on a subscription row makes a bodyless authenticated POST to the daemon; the daemon reads its own vendor CLI file, writes only the harness-owned store through the existing importer, constructs the same token source as startup, and replaces the live registry entry. The TUI then refetches `GET /v1/providers`.
- Failure mode and boundary: the vendor login must be present on the **harnessd host**. A remote TUI cannot import a credential located only on its own machine; the daemon reports the normal `codex login` or `kimi-code login` remediation instead. No credential value is accepted or returned by the endpoint.

## 2026-07-19 (ACP Server Mode — Epic #746)

- System/component: `cmd/harness-acp` and `internal/harnessacp`.
- Responsibilities: ACP stdio lifecycle and wire translation only; harnessd
  remains responsible for execution, event production, approval brokerage,
  cancellation, and todo state.
- Inputs/outputs: ACP JSON-RPC on stdio; harnessd REST for run commands and
  SSE for streamed events; ACP session updates/permission requests to editor.
- Failure modes: daemon/network failures are returned as ACP request errors;
  session cancellation maps to the underlying run cancel route; an editor
  permission denial maps to the existing deny route.

## 2026-07-19 (Enforced Plan Mode — Epic #740)

- `RunRequest.PlanMode` initializes `Active`; the step engine places a runner-owned gate in real tool contexts, and `ApplyPolicy` returns `plan_mode_denied` before normal approval handling for non-plan mutations. A terminal plan response transitions to `ExitPending`, emits plan approval events, uses the configured `ApprovalBroker`, and returns to `Active` on deny or `Inactive` on approve. `conversation_plans` is run/conversation scoped.

## 2026-07-19 (Session Rewind — Epic #739)

- `SQLiteConversationStore` owns cascade-deleted rewind snapshots; restore writes/deletes captured paths after hash safety checks, then truncates messages.

Use this file to document systems, interfaces, and interactions as they are built.

## 2026-07-19 (TUI Multi-run Dashboard — Epic #738)

- System/component: `cmd/harnesscli/tui` dashboard overlay.
- Responsibilities: lifecycle-bound `tea.Tick` list refreshes consume existing `GET /v1/runs`; the selected-run peek consumes the existing SSE bridge; control/dispatch consume existing run endpoints. Closing the peek or overlay cancels its bridge without affecting the attached session.
- Failure modes: list/control failures surface in the dashboard status path; an inactive overlay does not reschedule polling; a new peek stops the previous subscription.

## 2026-06-28 (Config-Driven Lifecycle Hooks — Epic #737)

- System/component: `internal/hooks` (schema, loader, adapters, trust store, builder), `internal/config` `[hooks]` section, `cmd/harnessd` bootstrap wiring, `internal/server` `GET /v1/hooks`, `cmd/harnesscli` `hooks` subcommand + TUI `/hooks`.
- Responsibilities:
  - `internal/hooks` owns the hook-file JSON schema, discovery (`Load`/`LoadWithOptions`), the JSON wire protocol (`wire.go`, shared verbatim by both transports), command/HTTP adapters onto the existing `harness` hook interfaces, the content-hash trust store, and the def→adapter `Build` plus startup `Summary`.
  - `internal/config` owns `[hooks] enabled/dirs` via the existing rawLayer merge.
  - `cmd/harnessd` owns startup-time loading (trust-aware), adapter registration onto existing `RunnerConfig` slices (after compiled-in plugins), and startup logging; it keeps no hook logic of its own.
  - The runner owns all failure policy (`HookFailureMode`) and hook event emission; adapters only return decisions/errors.
  - `internal/server` serves the startup summary read-only; `harnesscli hooks trust|revoke|list` owns trust management offline; the TUI renders server truth.
- Inputs/outputs:
  - Input: `*.json` hook files in `~/.harness/hooks/` (implicit trust) and `<workspace>/.harness/hooks/` + `[hooks] dirs` (trust-required); stdin/stdout JSON for command hooks; POST/response JSON for HTTP hooks.
  - Output: hook decisions (allow/deny/block + reason, modified args/results) into the runner's existing hook loops; structured startup logs; `{"hooks": [...], "skipped": [...]}` listing.
- Dependencies: stdlib only in `internal/hooks` (`os/exec`, `net/http`, `crypto/sha256`); `internal/harness` interfaces unchanged; trust store at `~/.harness/hooks-trust.json` (never the project tree).
- Failure modes:
  - Hook process/HTTP failure → adapter error → runner `HookFailureMode` (fail_closed default: deny/abort; fail_open: continue).
  - Hung hook → per-hook timeout kills the whole process group / aborts the request.
  - Untrusted or modified project hook file → skipped with structured reason (`untrusted` / `modified_since_trusted`), surfaced in startup logs and `/v1/hooks`.
  - Unreadable trust store → fail closed: project hooks disabled, startup continues.
  - Oversized hook output → 1 MiB cap fails the hook call.

## 2026-06-26 (Terminal-Bench Artifact Boundary)

- System/component: `benchmarks/terminal_bench/agent.py`, `scripts/run-terminal-bench.sh`, and `scripts/terminal_bench_artifacts.py`.
- Responsibilities:
  - The Terminal-Bench adapter owns harness execution and emits harness-grounded facts only: run record, run summary, telemetry, and logs.
  - Terminal-Bench owns task oracle results: `is_resolved` and `parser_results`.
  - The postprocessor owns the merge boundary, schema validation, failure classification, baseline comparison summary, and report generation.
- Inputs/outputs:
  - Input: Terminal-Bench `results.json` plus per-trial `benchmark_result.json`.
  - Output: schema-validated `results.jsonl`, `summary.json`, `run-env.json`, and `report.md`.
- Dependencies:
  - `tb` or `uv tool run --python 3.12 terminal-bench` must be runnable.
  - Docker and tmux must be available for real Terminal-Bench campaigns.
  - Fake-provider smoke mode requires `HARNESS_PROVIDER=fake` and `HARNESS_FAKE_TURNS`.
- Failure modes:
  - Missing adapter artifacts become synthetic `infra_error` rows rather than silently disappearing.
  - Harness/tool/provider/workspace failures are classified from run status and error messages but do not override Terminal-Bench oracle truth.
- Operational notes:
  - `is_resolved` must never be written by the adapter.
  - `baseline.json` is authoritative for the smoke tier as of the accepted 2026-06-27 real-provider campaign at `.tmp/terminal-bench/real-smoke-20260627-002630/2026-06-27__00-26-42`.
  - The accepted campaign preserved raw `results.json`, merged `results.jsonl`, `run-env.json`, `summary.json`, `report.md`, task `commands.txt`, task pane logs, per-task `benchmark_result.json`, per-task `harness_telemetry.json`, and per-task `agent-logs/harnessd.log`.
  - Adapter credential propagation uses copied env files so Terminal-Bench command artifacts do not contain raw provider keys.
  - Missing adapter artifacts still fail baseline promotion, because cost, steps, tool calls, final harness status, and replay data would be synthetic or unavailable.
  - The current smoke baseline records `cost_status=unpriced_model`; cost-sensitive gates require a pricing-catalog update for `gpt-5-mini` or a rerun on a priced model.

## 2026-04-05 (Harnessd Runtime Composition Boundary)

- System/component: `cmd/harnessd/main.go` and `cmd/harnessd/runtime_container.go`.
- Responsibilities:
  - `runMCPStdio(...)` remains the public stdio entrypoint but now delegates catalog/server assembly to `buildMCPStdioRuntime(...)`.
  - `runWithSignals(...)` remains the public HTTP entrypoint but now delegates runner/subagent/server assembly to `buildHTTPRuntime(...)`.
  - The runtime helpers own internal composition only; they do not change route, config, or runner semantics.
- Inputs/outputs:
  - Input: already-resolved workspace/config/provider/tool-registry/bootstrap dependencies from `main.go`.
  - Output: assembled MCP stdio runtime or HTTP runtime objects with the same startup/shutdown behavior as before.
- Dependencies:
  - Existing bootstrap helpers still own catalog, cron, persistence, trigger, and server-option subassembly.
  - `buildHTTPRuntime(...)` depends on the existing runner, subagent manager, and server option contracts rather than inventing a new runtime subsystem package.
- Failure modes:
  - MCP tool-catalog or stdio-server creation errors still fail startup immediately.
  - Subagent manager creation errors still fail HTTP startup before listening begins.
- Operational notes:
  - This is a stage-1 internal refactor only.
  - The broader orchestration runtime, checkpoints, workflows, memory layering, and agent networks remain planned work documented in stage specs, not implemented behavior.

## 2026-03-28 (Product Module vs Playground Boundary)

- System/component: repo root, `playground/`, and `internal/quality/repostructure`.
- Responsibilities:
  - The main module root now acts as a navigation boundary for first-class product directories and repo metadata only.
  - `playground/` owns exploratory, training, and snippet-style Go code behind its own `go.mod`.
  - `internal/quality/repostructure` enforces that the root stays free of Go source and that `playground/` remains isolated.
- Inputs/outputs:
  - Input: contributor file placement decisions for new snippets or experiments.
  - Output: deterministic repo structure plus focused product-module verification that excludes playground code.
- Dependencies:
  - `go test ./...` in the main module depends on the root staying free of product-unrelated Go files.
  - Playground verification, when desired, runs from inside `playground/`.
- Failure modes:
  - If new Go files are added at the root, the structure guard test fails.
  - If `playground/` loses its own module boundary, product verification can inherit snippet-package failures again.
- Operational notes:
  - This is a structural separation only; it does not impose product-quality expectations on all playground snippets.
  - Contributors should place future experiments in `playground/` rather than at the repo root.

## 2026-03-25 (Runner Step Engine Boundary)

- System/component: `internal/harness/runner.go` and `internal/harness/runner_step_engine.go`.
- Responsibilities:
  - `Runner.runStepEngine(...)` owns the public boundary and delegates execution to an internal `stepEngine` helper.
  - `stepEngine` owns the per-step LLM/tool loop, including steering drain timing, hook application, tool execution, accounting, memory observation, compaction, and terminal step completion.
- Inputs/outputs:
  - Input: preflighted run execution state (`runPreflightResult`), run request metadata, max-step budget, fork depth, and approval policy.
  - Output: unchanged runner events, message mutations, tool side effects, run completion/failure transitions, and accounting snapshots.
- Dependencies:
  - `Runner` remains the authority for run state, event emission, tool registry access, and persistence/memory helpers.
  - `stepEngine` depends on those runner-owned APIs rather than reaching into external packages directly.
- Failure modes:
  - Context cancellation still terminates the run through the existing `cancelledRun(...)` path.
  - Hook/tool/provider failures still route through the same `failRun(...)` and approval/wait-state handling.
- Operational notes:
  - This extraction is intentionally behavior-preserving; it narrows ownership without changing event or transport contracts.
  - Step-boundary ordering is now pinned directly by `runner_step_engine_test.go`.

## 2026-03-25 (Run Persistence Ownership Boundary)

- System/component: `internal/harness/runner.go`, `internal/server/http.go`, and `internal/server/http_external_trigger.go`.
- Responsibilities:
  - The runner owns initial run-record persistence for both `StartRun(...)` and `ContinueRun(...)`.
  - HTTP transports return run IDs/status and rely on the runner’s shared store wiring rather than duplicating `CreateRun`.
- Inputs/outputs:
  - Input: transport requests that create runs through direct `/v1/runs` or external-trigger `start`/`continue`.
  - Output: exactly one `CreateRun` attempt per logical new run record, followed by the existing non-fatal update path as the run progresses.
- Dependencies:
  - A shared `store.Store` must be wired into the runner when persistence is desired.
  - The server may still read from the store for historical `GET /v1/runs` and list surfaces.
- Failure modes:
  - If the shared store is absent from the runner, the server no longer compensates by inserting the run record itself.
  - Store create failures remain non-fatal where the runner already treats persistence as best-effort.
- Operational notes:
  - This makes the runner/domain layer the single persistence authority for new run records.
  - External-trigger flows now match the same ownership rule as direct runner-driven continuation.

## 2026-03-25 (Forked Child-Run Failure Contract)

- System/component: `/v1/agents` forked execution plus fork-context skill tools in `internal/server/http_agents.go`, `internal/harness/tools/skill.go`, and `internal/harness/tools/core/skill.go`.
- Responsibilities:
  - Treat `ForkResult.Error` as authoritative terminal child-run failure information even when the transport call returned a nil Go error.
  - Preserve the existing `Summary`-then-`Output` success rendering for healthy forked runs.
- Inputs/outputs:
  - Input: `RunForkedSkill(...)` responses containing `Output`, `Summary`, and optional terminal failure text in `Error`.
  - Output: HTTP execution errors or tool-call failures when `Error` is populated; successful response payloads only when the child run actually succeeded.
- Dependencies:
  - `/v1/agents` uses a local result guard because the server package cannot import the harness tools package without creating the wrong dependency shape.
  - Tool-layer callers share `ForkResultExecutionError(...)` in `internal/harness/tools/fork_result.go`.
- Failure modes:
  - If a child run fails normally and returns `ForkResult.Error`, callers now fail fast instead of reporting `status: completed`.
  - If the fork transport itself fails, the existing Go `error` path remains authoritative.
- Operational notes:
  - This change is behavior-preserving for successful forked runs.
  - Fallback `RunPrompt(...)` paths are unchanged in this pass.

## 2026-03-18 (Runner Event Ledger Ordering Contract)

- System/component: `internal/harness/runner.go`
- Responsibilities:
  - Treat `emit()` as the canonical per-run event ledger writer.
  - Mirror that ledger to the rollout recorder without reordering relative to assigned `Seq`.
  - Preserve `state.messages` as the source of truth across compaction and step execution.
- Inputs/outputs:
  - Input: concurrently emitted runner events carrying pre-assigned `Seq` values.
  - Output: in-memory `state.events`, subscriber fanout, and JSONL rollout lines in the same logical order.
- Dependencies:
  - `r.mu` for canonical event sequencing.
  - `compactMu` for message replacement serialization.
  - `copyMessages` / payload deep-clone helpers for ownership isolation.
- Failure modes:
  - If the recorder channel overflows, the dropped event is represented by `recorder.drop_detected` at the same `Seq`.
  - Recorder write panics are isolated from the run loop, but the in-memory ledger remains canonical.
- Operational notes:
  - The recorder goroutine buffers out-of-order arrivals and flushes only contiguous `Seq` values, so file order matches logical event order.
  - Existing compaction tests remain the guardrail for `state.messages` source-of-truth behavior.

## 2026-03-18 (Provider/Model Impact Mapping Workflow)

- System/component: planning and worktree workflow docs (`AGENTS.md`, `docs/plans/PLAN_TEMPLATE.md`, `docs/runbooks/worktree-flow.md`, `docs/runbooks/provider-model-impact-mapping.md`).
- Responsibilities:
  - Require provider/model flow work to map cross-surface impact before implementation begins.
  - Keep the required surfaces explicit: config, server API, TUI state, regression tests.
  - Make missing sections visible as process warnings instead of silent omissions.
- Inputs/outputs:
  - Input: planned feature or bugfix that changes provider/model selection, routing, API-key handling, model catalogs, or provider plumbing.
  - Output: task-specific impact map in `docs/plans/` linked from the task plan.
- Dependencies:
  - Contributor adherence to the documented planning workflow.
  - Existing plan and worktree runbooks as the entry points for implementation.
- Failure modes:
  - If the impact map is skipped, adjacent integration surfaces may remain under-scoped until follow-up fixes are needed.
  - If headings are left blank, reviewers lack a clear signal about whether the surface was checked.
- Operational notes:
  - This is process-guided enforcement only in the current pass.
  - Unaffected surfaces must be documented as `None` with rationale rather than left blank.

## 2026-03-25 (Hybrid Model Discovery Path)

- System/component: `internal/provider/catalog/discovery.go`, `internal/provider/catalog/registry.go`, `internal/server/http.go`, `cmd/harnessd/main.go`.
- Responsibilities:
  - Fetch live OpenRouter model ids and names on demand.
  - Cache discovery results in memory with a TTL.
  - Merge live OpenRouter results with static catalog metadata for runtime routing and `GET /v1/models`.
  - Preserve the static catalog as the baseline behavior for non-OpenRouter providers.
- Inputs/outputs:
  - Input: static provider/model catalog plus `GET https://openrouter.ai/api/v1/models` responses.
  - Output: merged provider resolution decisions and merged `/v1/models` response rows.
- Dependencies:
  - The loaded model catalog must contain an `openrouter` provider entry before live discovery is enabled.
  - `ProviderRegistry` remains the central provider-resolution surface for server/runtime callers.
- Failure modes:
  - Live fetch failure returns stale cached data when present.
  - If there is no cache, callers fall back to the static catalog view.
  - Startup never depends on a successful discovery request.
- Operational notes:
  - Static metadata remains authoritative on overlap, especially aliases, pricing, and default model attributes.
  - OpenRouter-only live models are surfaced with minimal metadata when no static overlay exists.

## 2026-03-05 (Provider Token Streaming)

- System/component: `internal/provider/openai/client.go` + `internal/harness/runner.go`.
- Responsibilities:
  - Consume streamed OpenAI chat completion chunks in real time.
  - Reassemble assistant text and tool-call arguments into the existing final completion shape.
  - Emit incremental SSE events for client-side progressive rendering.
- Inputs/outputs:
  - Input: streaming `/v1/chat/completions` SSE chunks with `choices[].delta` content/tool-call fields and optional usage.
  - Output: `assistant.message.delta` and `tool.call.delta` events during a turn, followed by the existing final turn/tool events.
- Dependencies:
  - OpenAI chat completions streaming semantics.
  - Existing runner event fanout/subscriber model.
- Failure modes:
  - Malformed stream chunks fail the run via provider error propagation.
  - Invalid streamed tool-call indexes are rejected before tool execution.
  - If the provider stream ends before `[DONE]`, the turn fails explicitly.
- Operational notes:
  - Tool execution still waits for fully assembled tool-call arguments.
  - Existing REST endpoints remain unchanged; only the event taxonomy expands.

## 2026-03-04

- System state: foundational workflow and documentation system only.
- Notable interfaces:
  - `AGENTS.md` defines operational policy.
  - `docs/runbooks/*` define execution playbooks.
  - `scripts/verify-and-merge.sh` operationalizes test-gated merges.

## 2026-03-04 (OpenAI Harness POC)

- System/component: `cmd/harnessd` + `internal/harness` + `internal/provider/openai` + `internal/server`.
- Responsibilities:
  - Accept run requests and execute deterministic LLM/tool loop.
  - Expose run status and event stream for external clients (GUI/TUI).
  - Execute bounded workspace tools for coding-oriented actions.
- Inputs/outputs:
  - Input: HTTP JSON request (`POST /v1/runs`), OpenAI API responses, tool arguments.
  - Output: run state (`GET /v1/runs/{runID}`), SSE lifecycle events (`/events`), tool result envelopes back to model.
- Dependencies:
  - OpenAI API (`/v1/chat/completions`) via `OPENAI_API_KEY`.
  - Local Go toolchain for `run_go_test`.
- Failure modes:
  - Provider request failures or malformed model outputs result in `run.failed`.
  - Unknown tool/tool argument errors are returned as tool-output error payloads to continue loop.
  - Slow SSE clients may miss live events but can retrieve persisted event history for the run.
- Operational notes:
  - Runtime state is in-memory only.
  - `HARNESS_MAX_STEPS` bounds loop depth.
  - Tool execution is bounded and event-emitting per run step.

## 2026-03-04 (Toolset Interface Revision)

- System/component: `internal/harness/tools_default.go`.
- Responsibilities:
  - Provide standardized coding tool interface: `read`, `write`, `edit`, `bash`.
  - Enforce workspace path boundaries for file operations.
  - Execute bounded shell commands for command-line workflows.
- Inputs/outputs:
  - Input: structured JSON arguments from model tool calls.
  - Output: JSON result envelopes (`content`, `bytes_written`, `replacements`, `exit_code`, etc.).
- Dependencies:
  - Local filesystem permissions.
  - `/bin/bash` availability for `bash` tool execution.
- Failure modes:
  - `edit` fails when `old_text` cannot be matched.
  - `bash` rejects commands matching danger deny-list patterns.
  - Path traversal attempts fail before filesystem access.
- Operational notes:
  - `bash` command execution remains timeout-bounded and workspace-rooted.
  - Deny-list guardrails are heuristic and should be reviewed before production exposure.

## 2026-03-04 (Entrypoint Testability and Coverage)

- System/component: `cmd/harnessd/main.go` testability boundary.
- Responsibilities:
  - Keep `main` as process entrypoint while allowing deterministic tests for startup/exit behavior.
  - Preserve server startup/shutdown behavior with signal-driven termination.
- Inputs/outputs:
  - Input: environment variables + signal channel.
  - Output: process exit behavior in `main`, error returns from `run`/`runWithSignals`.
- Dependencies:
  - OpenAI provider construction callback.
  - HTTP server lifecycle (`ListenAndServe`, `Shutdown`).
- Failure modes:
  - Missing API key/provider construction failure now return explicit errors through `runWithSignals`.
  - Server startup fatal errors surface through returned error channel.
- Operational notes:
  - Added lightweight test hooks (`runMain`, `exitFunc`, `runWithSignalsFunc`) to isolate process-level behavior in unit tests.

## 2026-03-05 (Regression Quality Gate System)

- System/component: `scripts/test-regression.sh` + `cmd/coveragegate` + `internal/quality/coveragegate`.
- Responsibilities:
  - Execute standard regression suite locally and in CI.
  - Enforce minimum total statement coverage threshold.
  - Enforce non-zero function coverage across codebase.
- Inputs/outputs:
  - Input: coverage profile (`coverage.out`), configured minimum threshold (`MIN_TOTAL_COVERAGE`).
  - Output: pass/fail exit code and gate summary (`PASS` with total and zero-function count).
- Dependencies:
  - Go toolchain (`go test`, `go tool cover`).
  - GitHub Actions runner for CI execution.
- Failure modes:
  - Missing/invalid coverage profile fails gate.
  - Any function at `0.0%` fails gate.
  - Total coverage below threshold fails gate.
- Operational notes:
  - Default threshold is `80.0%`, configurable via environment variable.
  - Workflow file: `.github/workflows/test-regression.yml`.

## 2026-03-05 (Hook Pipeline + Tool Surface Expansion)

- System/component: `internal/harness/runner.go` hook pipeline and `internal/harness/tools_default.go` baseline tools.
- Responsibilities:
  - Execute hook chain before and after each provider turn.
  - Allow hook-driven request/response mutation or blocking.
  - Emit hook lifecycle events for UI/TUI observability.
  - Provide repository-oriented baseline tools for traversal, search, patching, and git inspection.
- Inputs/outputs:
  - Input: hook implementations in `RunnerConfig`, model tool-call arguments.
  - Output: updated requests/responses, run failures on blocked/error hooks (depending on mode), tool JSON outputs.
- Dependencies:
  - Local filesystem and git binary availability for `git_status`/`git_diff`.
  - Provider call loop in runner execution.
- Failure modes:
  - Hook fail-closed mode converts hook errors into `run.failed`.
  - Hook fail-open mode emits `hook.failed` and continues run.
  - Tool validation errors are returned as tool error payloads and surfaced in `tool.call.completed`.
- Operational notes:
  - Hook failure mode defaults to `fail_closed`.
  - Baseline tool names now include:
    - `ls`, `glob`, `grep`, `apply_patch`, `git_status`, `git_diff`
    - plus `read`, `write`, `edit`, `bash`.

## 2026-03-05 (CLI Test Client)

- System/component: `cmd/harnesscli`.
- Responsibilities:
  - Provide a minimal operator-facing CLI to test the harness API without manual `curl` orchestration.
  - Start a run and stream run events until terminal completion/failure.
- Inputs/outputs:
  - Input: command flags (`-base-url`, `-prompt`, `-model`, `-system-prompt`).
  - Output: run id and line-by-line event stream in terminal, plus terminal event summary.
- Dependencies:
  - Harness HTTP API endpoints (`POST /v1/runs`, `GET /v1/runs/{id}/events`).
  - JSON SSE event payload format from server.
- Failure modes:
  - Non-2xx create/stream responses return non-zero exit with API error context.
  - Invalid SSE `data` payload returns non-zero exit (`invalid sse data`).
  - Missing prompt returns immediate validation error.
- Operational notes:
  - Stream reader handles framed SSE blocks and stops explicitly on `run.completed` or `run.failed`.

## Entry Template

- Date:
- System/component:
- Responsibilities:
- Inputs/outputs:
- Dependencies:
- Failure modes:
- Operational notes:

## 2026-03-05 (Modular Tool Registry + Approval Modes)

- System/component: `internal/harness/tools` modular tool subsystem + compatibility wrapper in `internal/harness/tools_default.go`.
- Responsibilities:
  - Provide a catalog-based, pluggable tool registration flow.
  - Isolate each tool into its own implementation unit.
  - Apply approval policy middleware (`full_auto` or `permissions`) at tool handler boundary.
- Inputs/outputs:
  - Input: `BuildOptions` (workspace root, approval mode, integrations, HTTP client, sourcegraph config).
  - Output: sorted tool catalog with wrapped handlers and JSON result envelopes.
- Dependencies:
  - Optional external integrations for LSP (`gopls`), Sourcegraph HTTP endpoint/token, MCP registry, agent runner, and web fetcher.
- Failure modes:
  - In `permissions` mode, mutating/fetch/execute actions emit structured denial payloads when policy denies or errors.
  - Missing external dependencies produce deterministic runtime errors from the affected tool handlers.
  - Invalid tool JSON schema (for arrays without `items`) causes provider-side request rejection; fixed for current arrays.
- Operational notes:
  - Default server mode remains `full_auto` via `HARNESS_TOOL_APPROVAL_MODE` default.
  - Run-scoped context key (`run_id`) is now injected for tool execution to support run-local state (`todos`).

## 2026-03-05 (AskUserQuestion Pause/Resume Interface)

- System/component: `internal/harness/tools/ask_user_question.go`, `internal/harness/ask_user_broker.go`, `internal/harness/runner.go`, `internal/server/http.go`.
- Responsibilities:
  - Allow model turns to issue structured user clarification requests through `AskUserQuestion`.
  - Pause a run in `waiting_for_user` state until answers are submitted.
  - Resume execution after valid answers or fail the run on timeout.
- Inputs/outputs:
  - Input: tool args `{questions:[...]}` and API submissions `{answers:{...}}`.
  - Output: tool result JSON `{questions:[...], answers:{...}}`, run state transitions, and wait/resume events.
- Dependencies:
  - In-memory `AskUserQuestionBroker` shared by runner and tool layer.
  - HTTP input endpoints (`GET/POST /v1/runs/{id}/input`) for user answer submission.
- Failure modes:
  - Invalid tool question shape returns tool-call error payload (run continues unless timeout path).
  - Invalid submitted answers return `400 invalid_request` and keep question pending.
  - Missing pending input returns `409 no_pending_input`.
  - Timeout returns typed error and transitions run to `run.failed`.
- Operational notes:
  - `HARNESS_ASK_USER_TIMEOUT_SECONDS` controls per-question wait timeout (default 300s).
  - Event stream now includes `run.waiting_for_user` and `run.resumed` for UI/CLI orchestration.

## 2026-03-05 (Observational Memory Subsystem)

- System/component: `internal/observationalmemory` + runner/tool integration.
- Responsibilities:
  - Persist optional observational memory by `(tenant_id, conversation_id, agent_id)` scope.
  - Inject bounded memory snippets into model turns when enabled.
  - Execute ordered per-scope memory mutations in local coordinator mode.
  - Expose operator/model control via `observational_memory` tool.
- Inputs/outputs:
  - Input: run transcript snapshots, tool actions (`enable|disable|status|export|review|reflect_now`), environment memory settings.
  - Output: memory records/operations/markers in DB, SSE memory lifecycle events, optional export files.
- Dependencies:
  - SQLite store in v1 (`modernc.org/sqlite`).
  - Existing provider for observer/reflector model calls (tools disabled).
- Failure modes:
  - Observer/reflector failures emit `memory.observe.failed` and preserve run continuity.
  - Misconfigured memory store startup fails harness boot with explicit error.
  - Postgres mode currently returns explicit not-implemented errors.
- Operational notes:
  - `HARNESS_MEMORY_MODE=off|auto|local_coordinator`.
  - `auto` resolves to local coordinator behavior in v1.
  - Transcript is exposed to tools as read-only snapshot through context interfaces.

## 2026-03-05 (System Prompt Composition Pipeline)

- System/component: `internal/systemprompt` + runner integration in `internal/harness/runner.go`.
- Responsibilities:
  - Resolve static prompt layers by intent/model/extensions at run creation.
  - Inject per-turn runtime context as ephemeral system message.
  - Emit prompt-resolution telemetry events for clients.
- Inputs/outputs:
  - Input: `RunRequest` prompt fields (`agent_intent`, `task_context`, `prompt_profile`, `prompt_extensions`) and `prompts/catalog.yaml` assets.
  - Output: provider-facing system messages and run events (`prompt.resolved`, `prompt.warning`).
- Dependencies:
  - YAML catalog parser (`gopkg.in/yaml.v3`).
  - Prompt asset files under `prompts/`.
- Failure modes:
  - Invalid prompt catalog/paths fail harness startup.
  - Unknown intent/profile/behavior/talent fails `POST /v1/runs` as `invalid_request`.
  - Reserved `skills` field is ignored with warning event.
- Operational notes:
  - `system_prompt` request field bypasses prompt engine completely.
- Runtime context includes `run_started_at_utc`, `current_time_utc`, `elapsed_seconds`, `step`, and phase-1 cost placeholder.
- New config vars: `HARNESS_PROMPTS_DIR`, `HARNESS_DEFAULT_AGENT_INTENT`.

## 2026-03-05 (Usage and Cost Accounting Pipeline)

- System/component: `internal/provider/openai`, `internal/provider/pricing`, `internal/harness/runner`, `internal/systemprompt/runtime_context`.
- Responsibilities:
  - Normalize per-turn provider usage into harness accounting fields.
  - Compute per-turn USD cost when pricing metadata/catalog is available.
  - Accumulate run-level usage/cost totals and expose them to APIs/events.
  - Inject live accounting fields into runtime context on every turn.
- Inputs/outputs:
  - Input: provider completion response usage fields, optional explicit provider cost fields, optional pricing catalog JSON.
  - Output:
    - `usage.delta` event per completion turn.
    - `run.completed` / `run.failed` payload totals (`usage_totals`, `cost_totals`).
    - `GET /v1/runs/{id}` totals in run state.
    - runtime context fields (`prompt_tokens_total`, `cost_usd_total`, etc.).
- Dependencies:
  - Optional env-configured pricing catalog path: `HARNESS_PRICING_CATALOG_PATH`.
  - OpenAI usage response schema (`prompt_tokens`, `completion_tokens`, details objects).
- Failure modes:
  - Missing usage from provider does not fail run; accounting defaults to zero with `provider_unreported`.
  - Missing model pricing does not fail run; cost remains zero with `unpriced_model`.
  - Invalid pricing catalog path/content fails startup with explicit load error.
- Operational notes:
  - No bundled default price table is required; pricing is opt-in via catalog path.
  - `CostUSD` remains populated for backward compatibility while richer cost structure is also exposed.

## 2026-03-06 (Terminal Bench Smoke Benchmark System)

- System/component: `benchmarks/terminal_bench/agent.py` + `benchmarks/terminal_bench/tasks/*` + `scripts/run-terminal-bench.sh` + `.github/workflows/terminal-bench-periodic.yml`.
- Responsibilities:
  - Execute a small recurring benchmark against the real harness implementation.
  - Bridge Terminal Bench task execution to `harnessd` and `harnesscli`.
  - Produce reproducible per-task artifacts for regression triage.
- Inputs/outputs:
  - Input: Terminal Bench task instructions, current repository checkout, `OPENAI_API_KEY`, optional benchmark model/env overrides.
  - Output: Terminal Bench run artifacts in `.tmp/terminal-bench/`, uploaded workflow artifacts, and task pass/fail outcomes.
- Dependencies:
  - Terminal Bench CLI (`tb` or `uv tool run terminal-bench`).
  - Docker, tmux, and asciinema in task containers.
  - OpenAI-compatible API access for the harness under test.
- Failure modes:
  - Missing API key returns agent installation failure before task execution.
  - Harness startup failures surface through `/tmp/harnessd.log` in task logs.
  - Upstream Terminal Bench import-path or CLI contract changes can break the runner script.
- Operational notes:
  - The benchmark agent copies the current checkout into `/opt/go-agent-harness` inside each task container rather than cloning a remote branch.
  - The suite is intentionally small and suited for nightly smoke coverage, not merge gating.

## 2026-04-05 (Orchestration Runtime Stack)

- System/component: `internal/checkpoints`, `internal/workflows`, `internal/workingmemory`, `internal/networks`, plus `cmd/harnessd` runtime wiring.
- Responsibilities:
  - persist human-in-the-loop pause state through checkpoints
  - execute deterministic workflow graphs over runner/tool/checkpoint primitives
  - maintain explicit scoped working memory alongside observational memory
  - compile sequential network role definitions into workflow-backed execution
- Inputs/outputs:
  - Input: YAML definitions from `HARNESS_WORKFLOWS_DIR` and `HARNESS_NETWORKS_DIR`, checkpoint resume payloads, scoped working-memory tool writes.
  - Output:
    - checkpoint records in shared SQLite state
    - workflow run state, step state, and workflow SSE event streams
    - network launch surface backed by workflow runs
    - provider-facing prompt context with working-memory snippet ahead of observational-memory recall
- Dependencies:
  - shared SQLite runtime state database (same path family as runtime memory state)
  - existing runner/tool registry/subagent bootstrap
  - checkpoint-backed approval and ask-user brokers
- Failure modes:
  - invalid YAML definitions fail load during startup wiring
  - missing checkpoint/workflow/network services return explicit `not_implemented` from HTTP routes
  - workflow or network execution failures are persisted as terminal failed runs with step-level error text
- Operational notes:
  - workflow and network routes are now real but remain intentionally conservative in v1
- sequential network execution is implemented; parallel fan-out remains deferred

## 2026-07-20 (Live model discovery — Epic #849)

- System/component: `internal/provider/catalog`, OpenAI/Anthropic provider clients, and `harnessd` startup.
- Configured OpenRouter, OpenAI, Anthropic, and DeepSeek providers refresh their model lists on a five-minute in-memory TTL using their existing credentials.
- A failed initial refresh preserves static catalog models; a failed later refresh serves stale cached models. Static metadata, including pricing and model guidance, wins over sparse live records.
# 2026-07-19 — Plugin bundle subsystem

- `internal/plugins` owns bundle validation, safe staged installation, persisted lifecycle state, marketplace index parsing, and discovery. `harnessd` loads enabled skills/commands and gates agents/MCP/hooks on trust.
# 2026-07-20 — Subscription-auth provider foundation (Epic #846)

- `internal/provider.TokenSource` is the credential boundary: provider clients call `Token(ctx)` at request construction, allowing a future expiring-credential provider to refresh without creating a parallel HTTP client.
- `internal/provider/tokencache.Cache` owns only in-memory cache, expiry-margin, and single-flight synchronization. Provider-specific code supplies refresh transport and any credential persistence; this foundation never imports, stores, or logs provider credentials.
- `openai.Config.TokenSource` and `ExtraHeaders` flow through the existing chat-completions and responses request paths. Nil `TokenSource` uses the existing static `APIKey` behavior; headers are copied on client construction.
- `catalog.ProviderRegistry` holds optional runtime token sources alongside API-key overrides. A source marks a provider configured and causes `GetClient` to forward it to `ClientFactory`; changing either credential mode evicts the cached provider client.
## 2026-07-20 (Kimi subscription auth — Epic #848)

- System/component: `internal/provider/kimi`, Kimi catalog/bootstrap wiring, and `harnesscli auth kimi`.
- Credentials flow: vendor `~/.kimi-code/credentials/kimi-code.json` is read once on explicit import; mutable state is only `~/.harness/subscription-auth/kimi.json` with mode `0600`.
- Requests use the existing OpenAI-compatible client with a refreshable token source and static `X-Kimi-Client-*` headers. Refresh persistence is contained in the harness-owned store.
- Operational limitation: live POST capability, not authenticated OAuth request shape or completion compatibility, was verified; manual service verification remains required.

## 2026-07-20 — Codex ChatGPT-Subscription Authentication (Epic #847)

- System/component: `internal/provider/codex`, catalog loader/registry, OpenAI-compatible client, `harnessd` bootstrap, and `harnesscli` auth/TUI surfaces.
- Credential flow: `harnesscli auth codex login` reads `~/.codex/auth.json` without changing it, copies the ChatGPT token pair/account id to `~/.harness/subscription-auth/codex.json` (`0700` parent, `0600` file), and `harnessd` loads only that copy. Refresh calls update only the copy.
- Request flow: the registry receives a refreshable `TokenSource` for `codex-subscription`; the existing OpenAI-compatible client obtains bearer credentials per request and sends `chatgpt-account-id` to `https://chatgpt.com/backend-api/codex/{responses,chat/completions}` without adding `/v1`.
- Failure mode: no imported credential leaves the provider unconfigured and reports the vendor-login then harness-import remediation before an upstream request. Credentials are never emitted by the new log/error paths.
# 2026-07-28 (macOS collection loading boundary)

- System/component: `ProjectSession`, model-settings state, and SwiftUI collection surfaces.
- Responsibilities: the session owns a `CollectionLoadState` alongside each fetched collection; views ask the state whether an empty result is truthful instead of inferring it from an array's temporary initial value. The DesignSystem owns the reusable placeholder geometry and motion policy.
- Inputs/outputs: refresh methods transition `idle → loading → loaded|failed`; successful empty responses permit their explicit empty states, while pending and failed empty arrays retain an inline skeleton region.
- Failure mode: a transport failure no longer renders as "nothing" or a missing run-store configuration. The existing status-message channel carries the error while the collection surface avoids asserting false absence.

## 2026-07-30 (Direct Feedback Publication Boundary)

- System/component: TUI input attachments, `/feedback`, local diagnostic
  bundles, the `gh` CLI, GitHub release assets, and GitHub issues.
- Data flow: pending image chips -> synchronous validation and bundle/image
  sidecar snapshot -> asynchronous release view/create/upload -> direct issue
  creation -> result message -> selective captured-chip cleanup.
- Source of truth: the input-area attachment list owns pending local paths; the
  feedback bundle owns immutable evidence; the publisher owns copied path
  slices and returns a result instead of mutating TUI state.
- Persistence: every invocation keeps a timestamped local zip. Published
  invocations also keep private image sidecars and add uniquely named assets to
  the reusable `go-code-feedback-assets` prerelease.
- Failure mode: validation fails before publication; release, upload, or issue
  failures retain local paths and original chips. Only a returned issue URL
  permits cleanup and a success status.
- Security boundary: textual evidence still passes through feedback redaction.
  Attached pixels are intentionally uploaded unchanged under the current
  single-user contract.

## 2026-07-31 (Issue #1003 Remote cronsd Harness Dispatch)

- System/component: `cmd/cronsd`, `internal/cron` remote dispatch, and the
  authenticated `harnessd` `/v1/cron/runs` boundary.
- Inputs/outputs: persisted harness jobs enter `DispatchExecutor`, then
  `RemoteRunStarter` sends typed scope and correlation metadata and receives a
  stable `run_id`; shell jobs remain on `ShellExecutor`.
- Config/dependencies: remote URL and bearer credential are explicit
  `CRONSD_HARNESS_*` settings; connect/request timeouts are finite; active
  harness jobs fail readiness if the boundary is incomplete.
- Failure modes: auth, timeout/cancellation, malformed response, non-2xx, and
  transport errors become safe typed execution failures with retryability and
  no prompt/credential contents.
- Verification: current local `harnessd` and `cronsd` processes completed a
  scheduled harness job through the remote boundary; execution and run IDs
  were recorded, and the exact foreground regression script passed at 85.6%
  coverage with zero uncovered functions.

## 2026-07-31 (Issue #1003 Durable Remote-Start Idempotency)

- System/component: authenticated `/v1/cron/runs`, server single-flight,
  built-in run-store migration, and `Runner.StartRunWithID`.
- Source of truth: authenticated tenant plus `Idempotency-Key` selects one
  stored fingerprint and reserved run ID. The raw prompt is hashed into the
  fingerprint and is not stored in the idempotency row.
- Lifecycle: reserve before start; mark accepted before HTTP success. A replay
  after restart returns an accepted binding. An interrupted reservation first
  recovers a persisted run, otherwise it starts the same reserved ID.
- Failure modes: missing durable-store support fails 503 without starting;
  fingerprint reuse conflicts with 409; redirects are returned as non-2xx and
  never receive the bearer credential.
- Compatibility: additive SQLite table and optional store interface implemented
  by both built-in stores; ordinary `StartRun` IDs and `/v1/runs` are unchanged.

## 2026-07-31 (Issue #1003 Fresh-Store API-Key Bootstrap)

- System/component: `cmd/harnessd` persistence bootstrap and the existing
  built-in SQLite API-key schema.
- Lifecycle: when `HARNESS_RUN_DB` is configured, startup opens the store,
  applies the base run/idempotency schema, then applies the API-key schema
  before constructing authenticated server routes.
- Failure mode: either migration failure aborts startup and closes the store;
  harnessd never advertises an authenticated endpoint backed by a partial
  schema.
- Compatibility: both migrations remain additive and idempotent. The cron
  execution store and #1004 terminal run linkage are unchanged.

## 2026-07-31 (Issue #1003 Reserved-Start and Deadline Boundary)

- System/component: `HarnessExecutor`, `Runner.StartRunWithID`, authenticated
  cron start single-flight, and the shared built-in run store.
- Persistence order: reserve the tenant/key/fingerprint/run ID, synchronously
  persist that reserved run, register/dispatch it in memory, then mark the
  binding accepted. A persistence failure returns 503 before dispatch.
- Timeout order: the persisted job timeout wraps the inherited scheduler
  context; the remote starter adds its transport timeout. Context propagation
  selects the earliest deadline and retains `Canceled`/`DeadlineExceeded`.
- Cache lifecycle: one entry exists only while a start callback is running;
  completion closes existing waiters and deletes the entry. Later delivery
  re-enters the durable binding path.
- Restart recovery: an unaccepted binding whose durable run is still queued
  enters `ResumeRunWithID`; exact scope/prompt/status validation precedes
  same-ID dispatch, and acceptance follows dispatch. Nonqueued persisted runs
  keep the prior already-started recovery path.
- Same-process recovery: when the run is already active after a transient
  acceptance-write failure, retry reuses it and retries only the mark.
  `ResumeRunWithID` also performs an atomic under-lock identity check so
  concurrent direct callers cannot overwrite or double-dispatch one run ID.
- Resume execution identity: the persisted model/provider hydrate both the
  public run and the internal request; prompt resolution and provider
  execution therefore cannot drift to a replacement process's new defaults.
- Accepted queued recovery: existing bindings always inspect the durable run.
  Queued rows absent from current runner state resume with the same ID whether
  the binding was accepted or still pending.
- Timeout validation: omitted create timeouts default to 30 seconds, while
  explicit nonpositive create and PATCH values are rejected before persistence
  or dispatch; harness dispatch derives its deadline from a validated record.
- Remote response lifecycle: success is logged only after the bounded response
  body is decoded. Deadline/cancel during decode returns the same typed
  `timeout`/`cancelled` classification as transport failure before headers.
- Compatibility: ordinary non-reserved run persistence remains non-fatal and
  #1004 still owns terminal cron execution-to-run linkage.

## 2026-08-01 (Issue #1003 Cross-Process Cron Dispatch Lease)

- Component: `internal/server` remote-cron idempotency and built-in
  `internal/store` implementations.
- Durable state: each tenant/key binding records dispatcher owner and lease
  expiry. SQLite acquisition is atomic across independent connections;
  acceptance is conditional on the stored owner.
- Lifecycle: reserve identity, acquire lease, persist/admit or resume the exact
  queued run, then mark accepted. A competing process never dispatches while a
  lease is live; an expired lease allows recovery of crash-orphaned queued work.
- Schema: two additive columns migrate both fresh and existing
  `cron_run_starts` tables. API payloads and cron execution linkage are
  unchanged.

## 2026-08-01 (Issue #1003 Linearizable Renewable Dispatch Lease)

- Component: SQLite cron binding store, remote-cron server heartbeat, and the
  Runner's read-only shutdown signal.
- Atomicity: lease acquisition/renewal returns the row from its conditional
  mutation. An acquired result cannot be overwritten by a later read.
- Clock: SQLite calculates current time and expiry for every persisted lease;
  process time determines duration only.
- Liveness: the owner renews while its local run is queued/running. Heartbeat
  termination follows terminal state, absence, shutdown, or lost ownership;
  expired stopped/crashed work can then resume under one replacement owner.
- Availability: concurrent pre-lease migrations recheck each additive column
  after an ALTER race. Other migration failures remain fatal.
## 2026-08-03 (Issue #1106 callback workspace authority)

- The callback database now has a sidecar recovery lock at
  `<callbacks.db>.recovery.lock`. It is advisory filesystem metadata and is
  joined by every filesystem-backed callback manager before `Set`, dispatch, or
  recovery; failed bootstrap and shutdown release it. It does not alter callback
  rows, public API, SSE payloads, TUI, or native UI behavior.
## 2026-08-03 (Issue #1106 recovery authority boundary)

- Delayed callback recovery is supported only for filesystem-backed workspace
  stores. In-memory and opaque URI stores remain usable for non-recovery
  operations but fail closed on durable bootstrap because process-loss
  authority cannot be established.
## 2026-08-03 (Issue #1122 native interaction owner)

- Ownership path: conversation SSE `HarnessEvent.runID` -> `Transcript`
  approval/plan or `AskUserPrompt.runID` -> `RunSession.currentRunID` fence ->
  Chat/ToolWalk captured ID -> run-specific HarnessClient endpoint.
- Selection/selected terminal/fallback/reset clear all pending interaction
  state immediately. A foreign terminal retires only its own run and cannot
  clear the currently selected run's interaction state.
