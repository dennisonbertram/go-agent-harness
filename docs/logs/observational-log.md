# Observational Log

## 2026-08-05 — Issue #1187 profile directory observation

- A profile mutation endpoint can be correctly implemented yet unavailable in
  a real daemon when its writable directory is omitted during composition.
  `GET /v1/tools` is the direct catalog evidence; the prior live daemon showed
  no create/update/delete entries before the first HTTP request.
- A temporary absolute profile directory is sufficient isolation. Redirecting
  `HOME` would change unrelated config, credentials, and runtime paths, so it
  is neither needed nor accepted as proof.
## 2026-08-05 — Issue #1188 observation

- A profile name on a request is not policy enforcement. Before composition,
  both first and follow-up ordinary turns resolved `server-default` even when
  the TUI had selected a profile with a distinct model and read-only tools.
- A broad API `allowed_tools` input is not safe precedence for a selected
  capability profile. The profile must be an upper bound, otherwise a client
  can visually select restricted mode then silently recover `bash`.
- Tool names alone are not a complete permission taxonomy: `download` has a
  distinct Action but both network and write side effects. The deny mapping is
  intentionally audited for that dual-capability tool, with a side-effect test
  rather than only provider-tool-list inspection.
- The original registry deliberately stripped `tools.Definition.Action` when
  adapting tool definitions into Harness definitions. A name list is therefore
  inherently stale as optional/deferred tools evolve; action metadata is the
  stable policy identity at both the rendered and direct-call boundaries.

## 2026-08-05 — Issue #1174 observation

- The server's normal stream sends authoritative final `assistant.message`; a tool call is not required for `/init` content.
- A successful SSE terminal is distinct from the legacy synthetic test path, so transcript completion alone was insufficient acceptance evidence.
- Workspace file appearance after preflight is treated as a conflict, not implicit user confirmation.

## 2026-08-05 (Issue #1183 durable replay SSE fixture)

- Observed: durable `/replay run_*` produces `RunStartedMsg`, so its next
  lifecycle action is a real run SSE subscription, not a duplicate replay POST
  or simulation response. A terminal stream frame ends that active state.
- Interpretation: the hosted failure was a mock completeness gap, not evidence
  that production should suppress the subscription. A rollout-path result
  remains one-shot and does not enter that lifecycle.

## 2026-08-05 (Issue #1177 harnessd readiness observation)

- A free port is not an owned listener. `freeLocalAddr` proves only that a
  port was available before it is closed; under parallel race load another
  daemon can acquire it before the target startup path. The listener returned
  by `runWithSignalsWithDeps` is the authoritative identity for health checks.
- A longer health timeout would preserve the ownership ambiguity. The existing
  matrix helper instead turns the daemon's listener acquisition and early
  completion into causal test boundaries while retaining parallel coverage.
- The normal full regression under the host default temporary-directory alias
  is retained as evidence: bootstrap fixtures compared `/var` to canonical
  `/private/var` and failed before #1177's race/coverage gates. Canonical
  `TMPDIR=/private/tmp` isolates that host-path representation from the test
  contract; it does not change daemon readiness behavior.

## 2026-08-04 (Issue #1169 bootstrap VCS provenance observation)

- A clean linked worktree is not sufficient evidence for Go 1.26 VCS stamping:
  its `.git` file can lead automatic discovery to a dirty parent checkout. The
  stable authority is an explicit child `GIT_DIR` paired with child
  `GIT_WORK_TREE`, followed by inspection of the generated executable itself.
- Ambient Git environment variables are another cross-checkout input. A
  bootstrapper must own them or reject the output; it cannot safely trust an
  inherited external repository context.
- Fetching `origin/main` but passing local `main` to `git worktree add` is the
  same provenance defect one layer earlier: a clean binary can still be for a
  stale source commit. The resolved `FETCH_HEAD` commit is the required target.

## 2026-08-04 (Issue #830 retry-budget observation)

- A 100ms `MaxTotal` in a `t.Parallel` `httptest` fixture measures scheduler
  and coverage-instrumentation contention, not the 429/503 retry contract.
  Focused normal and race stress keep passing; the issue's red occurred only
  during the full coverage phase. A one-second test-only bound preserves the
  same three attempts, disabled jitter, and exact two-request assertions.
## 2026-08-04 (Issue #1165 runtime provenance observation)

- A binary can be stale even when adjacent artifact text claims the desired
  source revision. `go version -m <binary>` is the runtime authority; a dirty
  or missing VCS record is an egress/cost and verification-integrity failure,
  not a warning suitable for a fallback run.

## 2026-08-04 (Issue #1161 scheduled routing observation)

- Current main can accept an unknown model with `allow_fallback:true`, while a
  later cron/callback continuation retains conversation ownership but rebuilds
  a request without that policy and fails provider resolution before output.
- The defect is a boundary-narrowing error, not a catalog or transcript error:
  both observed scheduled lanes create linked runs, but their reconstructed
  admission payload contains prompt/scope only.
- Routing values are safe durable intent; provider credentials, endpoints, and
  client configuration are not and must never enter these payloads or logs.
- Even after the typed scheduled payload was repaired, the queued durable run
  initially persisted an empty provider until asynchronous resolution. A crash
  in that window could still narrow replay, so the requested safe provider name
  must be part of initial queued state as well as the scheduler-owned payload.
## 2026-08-04 (Issue #1162 fake-provider routing — pre-fix observation)

- With an explicit fake environment setting, a loaded catalog can still select OpenAI for `gpt-4.1-mini`; an absent fixture model fails when fallback is false. This is an egress/cost risk, not merely a test-fixture mismatch. The catalog must remain observable while execution is forced to fake.
- Final daemon evidence shows catalog metadata remains available while both catalog-known and absent fixture models report fake execution and leave the configured real-client factory untouched.

## 2026-08-04 (Issue #1158 conversation snapshot observation)

- Message text cannot identify an event: a historical assistant `hello` and a
  later callback `hello` are two valid turns. The stable identity is the
  durable SSE event ID, but it is safe only when paired with the exact message
  snapshot rather than sampled independently in the HTTP handler.
- The deterministic two-run regression holds the second provider call after it
  has begun. While that scheduled-style run is in flight, `/messages` still
  returns the prior two messages and prior cursor; after completion it returns
  four messages (including the second identical `hello`) with a new cursor.
- The implementation intentionally does not remove #1148's provisional
  content suppression in this slice because that selected-conversation reducer
  is unmerged. #1148 consumes `LastEventID`, removes content identity, and owns
  bounded exact-ID overlap/reconnect dedupe.
- Independent review demonstrated that event-lock atomicity alone is
  insufficient when two run lifetimes overlap: global event order can contain
  B before A publishes A's messages. The exact safe result is an empty cursor,
  not the newest conversation event. The inverted regression completes B
  before A and requires that fallback.
- A process restart also erases the process-local pair. Because undo, rewind,
  and compaction can change durable messages without an event-store version,
  `run.completed` is not snapshot-equivalence evidence. The restart-after-undo
  regression requires empty rather than a cursor that skips removed history.

## 2026-08-04 (Issue #1156 MCP HTTP transport observation)

- A nil `http.Client.Transport` is not per-client isolation: it delegates to
  the process-global default transport. The deterministic ownership assertion
  and cleanup spy make that hidden cross-test dependency visible; existing
  strict 401/403 mapping tests remain the classification authority.

## 2026-08-04 (Issue #1152 harnessd fixture observation)

- A fixed 80–150 ms delay is neither evidence that `runWithSignals` reached
  the model factory nor that the daemon acquired its own listener. Under race
  and parallel SQLite migration load it may signal shutdown during bootstrap.
  The provider invocation, injected cleaner gates, and listener returned by
  the daemon are the authoritative fixture boundaries.
- Default callbacks are production behavior and must not be globally disabled
  to make unrelated tests pass. Per-fixture opt-out isolates the five paths
  that do not exercise callbacks while the callback-enabled matrix/shutdown
  coverage continues to exercise the default.

## 2026-08-03 (Issue #1144 transient heartbeat observation)

- A timer delay cannot demonstrate ownership preservation after a transient
  renewal error: it can cross the last confirmed durable deadline. The
  authoritative boundary is the real successful `ExtendLease` result followed
  by a re-read of its persisted lease row.
- The fixture's channels are buffered one-shot observations, so the heartbeat
  never waits for the test. The test waits with bounded three-second diagnostics
  for the injected failure and successful delegated extension, then proves the
  durable deadline advanced while the original attempt/token remain owner.

## 2026-08-03 (Issue #1140 matrix listener identity)

- Hosted race run 30848795397 recorded two harnesses on the same recycled
  `127.0.0.1` address. The intended process logged one loaded custom skill,
  while the test observed `{"skills":[]}` from a different process.
- Listener readiness is now an ownership signal rather than a prediction: the
  helper uses the address returned by the exact listener passed to
  `httpServer.Serve`; it also surfaces early startup failure instead of waiting
  out the health timeout.

## 2026-08-03 (Issue #1124 retry-wait recovery observation)

- A sleep after `Recover` cannot establish that the recovered timer did not
  run: it observes host scheduling rather than the durable `next_attempt_at`
  authority. The SQLite claim predicate already accepts a supplied manager
  clock, so a test-owned clock and explicit fire create the required causal
  before/after boundary without changing production timers.
- The pre-deadline checkpoint includes retry state, exact due time, reserved
  run ID, attempt one, and empty token/lease; checking all of them prevents a
  no-call assertion from masking an accidental claim or fence leak.
## 2026-08-03 (Issue #1136 timeout capability proof)

- A real deadline is suitable for #1133 wait-policy coverage but is not enough
  if any caller can turn a submission handle into authority. The opaque ticket
  is absent before deadline and can be constructed only at Runner's deadline
  boundary; deterministic consumption makes B -> C -> A exact-one dispatch
  and terminal/failure/reset non-dispatch observable.
- A single mutable stream task would leave displaced A running when C starts.
  The handle-keyed task registry permits reset/load to stop both streams.

## 2026-08-03 (Issue #1133 passive outcome observation)

- The #1130 handle correctly retained A lifecycle after B selection, but the
  consumer stopped polling it on `.displaced`; durable A evidence was therefore
  present but unobserved. A control authority and outcome observation are
  separate concerns.
- Gated integration runs show B can precede A terminal, A stream EOF, A timeout,
  or A start acknowledgement. Each retains B as the selected scheduled run;
  only the deadline scenario emits an A cancel request.
- B can itself terminal before A while a user submits C. This proved timeout
  authorization must follow A's stream lifetime, not `activeSubmission` or the
  one current local-stream pointer. The final contract uses an immutable A
  handle owner token plus reset/load generation and cancels every live local
  submission stream when detaching a session.

## 2026-08-03 (Issue #1130 submission-outcome observation)

- The original single `State` made displacement overwrite terminal/failure
  evidence. ToolWalk then saw only a nonterminal displaced handle and could
  report an A timeout even when A had completed.
- A late `startRun` response is not a stale response to discard: its run ID is
  needed for A-local diagnostics and outcome handling. It is stale only for
  shared selection, accounting, streams, and visible error state.
- EOF is an ownership-sensitive failure: the visible A must stop spinning, but
  the same EOF after B selection must not make B look failed.

## 2026-08-03 (Issue #1128 submission observation)

- A rendered run ID is insufficient when the action type is re-derived at
  click time. Both the mode and owner must be captured together. Likewise,
  shared session state is a presentation authority, not proof of which run a
  ToolWalk submission started.
- The red regression additionally showed that a local run must record its own
  first timestamped lifecycle frame. Otherwise it remains permanently
  provisional and a genuinely newer scheduled continuation cannot become the
  selected owner.

## 2026-08-03 (Issue #1125 action-owner observation)

- Stop and steer are authority-bearing UI actions: retaining a SwiftUI closure
  across A-to-B selection means a fresh current-ID lookup changes the user's
  target. The identity must be captured and checked before draft/cancel state.

## 2026-08-03 (Issue #1007 Rebase Observation)

- The original #1007 action fixture attempted approve, deny, answer, steer,
  and cancel concurrently. Main's #994 contract intentionally permits exactly
  one acknowledged control request; the repaired test now proves the competing
  calls are rejected until the same run emits a decision lifecycle frame.
- Focused native evidence covers external action routing, stale conversation
  rejection, foreign-terminal shielding, terminal tombstones, first active
  evidence, and timestamp ordering. Full macapp (217 tests/43 suites) and
  repository normal/race/coverage gates now pass; this remains distinct from
  the later live installed-app acceptance proof.

## 2026-08-03 (Issue #1120 heartbeat ordering observation)

- The prior test attempted second recovery before proving a heartbeat existed;
  consequently a load-delayed first heartbeat could never exercise the stated
  blocked-renewal scenario. The new fixture's one-second lease is only enough
  headroom to establish that scenario, not a production-policy change.
- Once `blocking.entered` is observed, recovery from the second manager must
  fail at the process fence. When the last confirmed lease expires, the
  original admission observes cancellation and releases its exact token before
  any replacement can be admitted.

## 2026-08-03 (Issue #1117 callback fixture observation)

- The duplicate-manager test's second manager already fails exclusive recovery
  while the first is live. Its later 100 ms sleep does not prove duplicate
  prevention; with a 30 ms lease it instead invites a valid sequential retry
  under aggregate scheduling pressure. Exact starter invocation count and the
  persisted attempt/run are the relevant observable outcome.
- The old normal x100 fixture reproduced `attempts = 2, want 1`; focused race
  x100 did not, so no deterministic production race was established. The
  default-lease fixture passes normal/race x100 while retaining the direct
  authority rejection and exact-one-admission checks.
- The initial full-suite invocation overlapped other repository gates and was
  red in unrelated packages, so it was not accepted. Once all observed
  `test-regression.sh` processes exited, the isolated foreground rerun exited
  0 with normal/race coverage evidence at 85.5% and zero uncovered functions.

## 2026-08-03 — Issue #1112 race authentication timing classification

- Exact base: `51230be1122ae9db70aeaedfdd3e6b6db7a5e2fb`.
- Before the fixture repair, authenticated assembly POST latency was about
  200-225 ms normally, 2,466 ms in an isolated race run, and 2,256 ms in the
  complete cron race package. The hosted full-repository race failure observed
  the handler/start at about 5,000 ms, after the client deadline won.
- The complete local cron race package passed before the repair, so the defect
  required aggregate contention; the deterministic bcrypt-cost assertion
  replaces that environment-dependent timing reproduction.
- After rehashing only the synthetic stored key at `bcrypt.MinCost`, focused
  assembly normal x25 and race x10 plus complete cron normal/race all passed.
  The full repository regression then passed normal, all-package race, 85.5%
  total coverage, and zero uncovered functions.
- The corresponding test-only repair merged in #1113; it is baseline test
  stabilization, not a production behavior change.

## 2026-08-03 (Issue #1106 final liveness observations)

- A process-lifetime recovery lock acquired only in `Recover` left ordinary
  durable `Set`/timer managers invisible to the authority protocol. Every
  filesystem-backed durable manager must instead acquire and hold the common
  fence before Set/dispatch for its lifetime; unavailable authority fails
  closed and is released after failed bootstrap, shutdown, or process exit.
  Rolling upgrades cannot rely on the lock alone because the old binary ignores
  it. The persisted compatibility fence covers both directions: an old winner
  remains public `dispatching` that current recovery refuses to take over, and
  a current winner is private `dispatching_fenced` that old SQL cannot claim or
  reclaim.
- Holding the lock is still insufficient inside one live process. A duplicate
  timer formerly recovered its own expired current token while StartCallback
  was still unwinding. Only a token captured from the bootstrap crash snapshot
  carries recovery provenance; expected-token CAS preserves a later owner.
  Recovery may transition only current private `dispatching_fenced` rows with
  that exact token, including expired or `NULL` leases; legacy public
  `dispatching` rows fail closed.
- Nine synthetic claim failures with one claim per window exceeded the former
  cap and stranded a pending callback. Lifetime rearming with a capped delay
  resumes the same reserved callback identity and leaves durable attempt at one
  when admission finally succeeds. Deadline release persists the sanitized
  `callback admission unavailable` retry reason rather than a store or context
  error.
- The private state initially escaped through cancel-conflict text. Normalizing
  manager list/event/error output preserves the existing public `dispatching`
  API while retaining the persisted compatibility fence internally.
- The repository race gate exposed a separate cron assembly timeout: its
  authenticated local remote start crossed the configured 5-second request
  deadline under suite load. Five focused host-local race repetitions then
  passed, but with only 289ms of observed latency margin at the slowest remote
  start. That evidence is tracked as #1112 and is not treated as proof that the
  failed full gate is green.

## 2026-08-03 (Issue #1110)

- Observed `Runner.StartRun` activates `notify_parent` before it begins the
  asynchronous execution path whenever `ParentContextHandoff.ParentRunID` is
  non-empty. The pre-change test instead queried activation after return from
  `StartRun`, where terminal cleanup may already have run. The correct test
  lifetime is the first captured provider request, followed by an explicit
  terminal-cleanup check after release.
- Repeated evidence: the new gate held the exact first request until the test
  observed it, so success cannot be caused by a sleep or a terminal race. After
  release, the same test confirms cleanup rather than treating that cleanup as
  a missing activation.

## 2026-08-03 (Issue #994 — Terminal Control Ordering)

- A control request can be accepted or rejected after the run it targets has
  already emitted `run.completed`. The user-facing invariant is not merely
  that the request finishes: the composer must become usable, and a rejected
  steer must return its text, while a reset or another conversation must stay
  untouched by that late result.
- The deterministic native fixture observes the terminal run state before
  releasing delayed HTTP response delivery. Approval releases the composer;
  steering surfaces its failure and restores its draft. Reset and conversation
  switch both suppress that same delayed stale failure.
- Return-key submission is the same user intent as the composer button. During
  an unacknowledged terminal-era control, it now stays disabled and preserves
  the next draft rather than starting a second run with stale A ownership.

## 2026-08-03 (Issue #1108 — Native Durable Reconciliation Barrier)

- Hosted `live-harnessd` observed run C completed with correct accounting
  before its durable assistant row was reconciled. This classifies the current
  failure as a test ordering defect, not yet a product callback durability
  defect. A user-visible transcript row is the required post-gate observation.
- After the application-level gate repair, C x20 and the live native harness
  suite passed. The fixture remains intentionally scoped to deterministic test
  observability; the live test is the separate check of the actual daemon path.
## 2026-08-03 (Issue #1106 callback SQLite contention)

- `PRAGMA busy_timeout` executed through `sql.DB.Exec` configured only the
  borrowed connection. A second manager using another pooled connection could
  fail immediately with `SQLITE_BUSY`, so heartbeat error handling must not
  equate a transient database result with loss of durable ownership.
- A successful lease extension supplies a concrete safety deadline. Before
  that deadline, retrying is safe; at/after it, cancellation and later durable
  takeover are required to avoid an owner continuing beyond an expired lease.
- A context deadline on the renewal call alone is insufficient: it only ends
  the blocked database operation. Cancellation of the independently-running
  callback admission must be guarded by its own deadline timer.
- A literal filesystem `?` is not a SQLite query delimiter. Normalizing a
  filesystem path before URI escaping avoids both query truncation and the
  relative `file:.harness/...` modernc allocation path.
- A pre-expiry cancellation interval has no cross-manager happens-before
  guarantee. The observable handoff boundary must be a durable, exact-token
  release after the canceled admission returns; only bootstrap recovery may
  convert a stale dispatching lease without that acknowledgement.

## 2026-08-03 (Issue #1102 — Pending Is Not a Wait-State Boundary)

- Observation: pending input readiness is intentionally a broker-registration signal, while `run.waiting_for_user` is the externally observable runner lifecycle boundary. A test that needs to inspect `RunStatusWaitingForUser` must synchronize on the latter.
- Consequence: retries and sleeps can mask the test race but cannot establish causality. Runner `Subscribe` returns history and registers live delivery under the event lock, making it the appropriate no-gap test boundary.

## 2026-08-03 (Issue #1006 Callback Dispatch Verification)

- The exact initial manager red turned a transient start error into durable
  `fired` with attempt zero and no next attempt. The new state machine instead
  retained the same reserved run ID through retry-wait and started on attempt 2.
- Two managers opened on the same SQLite database produced one admission call;
  a concurrent cancel after the claim returned the typed conflict. A recovered
  retry-wait row did not dispatch before its next-attempt timestamp.
- A simulated failure after admission retried through the same reserved ID, and
  a create race where the old owner was canceled left a queued durable row that
  the new owner reconciled. Repeated race tests passed for these boundaries.
- SQLite treated an inserted zero Go timestamp as a non-NULL year-one value;
  `COALESCE(next_attempt_at,fires_at)` then admitted a future pending callback
  early. The exact red won an early claim; persisting SQL NULL for absent retry
  time restored the intended `fires_at` gate.
- The first repository race run timed out after ten minutes with the cancel-race
  test waiting for admission and a rapidly cycling SQLite rows goroutine. The
  exact cause was mixed local/UTC timestamp text: claim lost, but rescheduling
  parsed the same instant as overdue and immediately fired again. UTC migration
  normalization plus bounded admission waits removed the loop; the complete
  normal/race phases then passed twice.
- An assembled `CallbackManager` → `callbackRunStarter` → `Runner` recovery test
  produced the expected callback run under the originating tenant, agent, and
  conversation. This is deterministic harness integration evidence, not the
  final live API/TUI/native full-conversation proof owned by #1010.
- The first coverage gate then found only the compatibility `ListPending`
  method uncovered. A real pending-row assertion closed that gap; the final
  unmodified regression script passed at 85.5% with zero uncovered functions.
- Independent-review lifecycle reds timed out for `retry_wait`, `failed`, and a
  fast `started` continuation once the scheduling run was terminal. Direct
  conversation publication made those events replayable without violating the
  terminal-run event boundary. A replacement Runner test then proved startup
  recovery republishes durable retry-wait, failed, and started snapshots with
  their stable run IDs before new subscribers attach.
- The review's durable-list red returned partial in-memory state as successful
  tasks; the error-aware contract now returns HTTP 500. Final diff audit found
  the agent list tool similarly converted the same failure into successful
  `[]`; its exact red now propagates the durable read error. Store, manager,
  task, and event reds also proved a bearer/password string was retained by the
  old truncation path; the allowlist repair exposes only generic callback-owned
  summaries. Focused and complete affected normal/race suites pass.
- The post-review full normal, race, and coverage phases reached 85.5% total,
  then correctly failed the zero-function gate on the now-unused
  `SQLiteCallbackStore.ListActive`. Recovery had superseded that helper with
  `ListAll` so terminal state could be republished. Removing the dead method is
  the narrow fix; it does not fabricate coverage for a superseded API.


## 2026-08-03 (Issue #1098 — Coverage Gate Observation)

- The cached merged main `224d667a` contains `finishUnavailableExecution` and
  `reconciledScope`; stale local main `2709fa1a` does not. The initial bootstrap
  fetch had DNS failure, so the clean worktree was realigned to fetched
  `origin/main` before any test or implementation work.
- Existing tests characterize cancellation and transient job lookup but do not
  directly execute the definitive `ErrJobNotFound`/`sql.ErrNoRows` terminal
  branch, explaining the zero-function coverage report.
- After #1096 merged, the test-only candidate was cleanly rebased to
  `5c4ed8c8`; the host full regression passed at 85.6% total coverage with no
  zero functions, including the repaired helper entries.

## 2026-08-01 (Issue #1086 inventory boundary observation)

- An isolated fake-provider `harnessd` reported 64 resolved entries at
  `/v1/tools`; the read-only inventory command produced a hashed report from
  that live catalog and the built-in TUI registry. This is inventory/reconciliation
  evidence only, not API/SSE, PTY, native GUI, or cron/callback acceptance.
- An absent dynamic server has no tool names in the resolved registry. Its
  `not-applicable` evidence must therefore retain the runtime condition source
  and stable reason; a static guessed list would mask drift.
- Registry inspection exposed a different omission class: a present tool can
  still be unaccountable when a flat builder drops the branch that enabled it.
  Carrying provenance alongside the tool at append time preserves conditional
  ownership without reverse-engineering names after registration.
- MCP server identity is available at discovery and connection time. Retaining
  it as a structural tag and Registry condition supports stable inventory rows;
  deriving it later from the sanitized public tool name would be lossy.
- A live report that reads only resolved present tools cannot distinguish “no
  provider configured” from “provider configured but discovery failed.” The
  configured and observed unavailable records must cross the same daemon
  boundary as the present catalog, and their owner/condition/resolver/provider
  tuple must match exactly.
- A report renderer is a trust boundary: accepting raw `Outcome: pass` values
  lets a malformed record bypass the validator even when the validator itself
  is strict. Rendering now revalidates each item/surface record against its
  intent case and the compiled inventory.
- Empty resolver arrays carry meaning only when the daemon explicitly emits
  them. JSON decoding into plain slices collapsed absent, null, and empty into
  the same state; pointer-backed boundary fields preserve that distinction.
- A generic MCP discovery error without configured provider names cannot be
  reconciled to honest not-applicable identities. Explicitly incomplete
  resolution and HTTP 503 preserve the uncertainty without inventing names.
- TUI registry replacement is extensible, so command provenance cannot be
  inferred from the fact that a compiler received an entry. The entry's owning
  registration path must carry the metadata that the inventory hashes.
- Per-surface runners need completeness over their applicable subset without a
  different inventory identity. Filtering and re-hashing would make API, TUI,
  and native evidence incomparable; selected-surface validation instead keeps
  the same complete inventory hash and narrows only the required mappings.
- An alias is not just documentation in a PTY acceptance lane: parser routing,
  arguments, and lifecycle behavior can drift independently. Inventory-derived
  invocation IDs make `/resume` and `/continue` separate proof obligations
  without inventing a second command item.
- Local slash commands have no honest run/conversation/event identity. An
  evidence-class contract distinguishes that absence from missing conversation
  proof and rejects IDs supplied to make a local result look run-backed.
- One boolean probe cannot independently establish rendered output, durable
  state, and exactly-once continuation. Typed assertion/observation sets expose
  which dimension is missing, while digested artifact references make the
  evidence bytes reproducible and their redaction status explicit.
- Unknown-command and invalid-form behavior belongs to the runner contract, not
  the runtime registry. A separately hashed required-scenario catalog preserves
  strict completeness without polluting or second-guessing inventory identity.
- Runtime tool presence does not prove a native GUI analogue. Making the native
  mapping a reviewed, source-referenced suite input prevents the native runner
  from silently treating terminal-only tools as either covered or N/A.
- A screenshot alone cannot correlate native rendering with the conversation
  or durable state. Native proof therefore needs AX structure, raw SSE/event
  bytes, and an API/store probe tied to the exact build, daemon, and isolated
  workspace alongside the screenshot.
## 2026-08-03 (Issue #1096 — Keychain Host-Live Boundary)

- Observation: `exec.LookPath("security")` proves only binary availability; it
  does not prove a process has login-Keychain authorization. Standard tests
  must therefore exercise command construction with fakes, not infer mutation
  readiness from the binary path.
- Operational consequence: the separate `HARNESS_TEST_REAL_KEYCHAIN=1` lane
  is explicit evidence of host session access. Its account is process-specific
  and cleanup is limited to that account, so concurrent host runs cannot share
  a self-test item.
## 2026-08-03 (Issue #1038 Native Usage Reconciliation)

- A terminal state can be rendered correctly while a separate transcript rebuild quietly resets accounting. Therefore terminal visual state and visible usage must be tested together at the reconciliation boundary, not inferred from server-side event order.
- Direct fake-provider SSE capture contained cumulative `usage.delta` followed by `run.completed.usage_totals`; the missing usage was not a provider, server, or transport failure.
## 2026-08-03 (Issue #994 Native Control State)

- A request-generation guard is required in addition to a boolean in-flight flag: a prior answer/control completion can arrive after reset and otherwise clear the guard for a newer request.
- The pending-input fetch also needs run and generation validation because two waiting events can race and an older HTTP response can overwrite a newer prompt.

- HTTP acknowledgement is not a run decision: a 2xx can precede, follow, or race the relevant SSE lifecycle. Native controls must therefore retain their acknowledged-disabled state until the matching run advances, while an older run's replay is observational only.
## 2026-08-03 (Issue #1115 Workflow Subscriber Fixture)

- Scheduling observation: `Engine.Start` emits `workflow.started` synchronously but launches the script asynchronously; a fast script may still reach terminal state before the next `Subscribe` call runs.
- Contract observation: terminal channel closure applies to subscribers registered at the terminal transition. A late subscriber receives terminal replay history, and the SSE handler returns from that history without waiting on its live channel.
- Test-design observation: a comment that says "subscribe before start" is not synchronization. A channel gate released after `Subscribe` returns provides a deterministic happens-before edge without a timing sleep.
- Ordering observation: closing a buffered Go channel preserves queued receive order; a consumer reads the 64 accepted log events before observing `ok=false`, while dropped overflow and terminal payloads do not prevent termination visibility.

## 2026-08-01

- Platform-coverage observation: a function can be portable while its only
  callers sit behind a platform availability guard. Linux correctly avoids
  `security(1)`, so direct portable parser coverage is needed in addition to
  the Darwin integration test to satisfy the repository's zero-function gate.
- Real Keychain integration is launch-context sensitive: the tmux-hosted suite
  could not complete `security(1)`, while the same exact candidate passed in
  the logged-in foreground host context. Foreground remains authoritative.

Use this file for observations about system behavior without immediately prescribing code changes.

## 2026-08-02 (Issue #1093 Cleaner Shutdown)

- Cancellation is a request, not an exit acknowledgement. A fake cleaner that observes `ctx.Done()` but intentionally holds completion made the original daemon return before the goroutine was known to be finished.
- Channel handshakes remove the prior startup sleep from the failure reproduction. A foreground macOS launch context remains required for the repository's real Keychain checks; tmux can make those external tests look red even when the same candidate passes foreground.

## 2026-08-01 (Issue #1056 Final-Only TUI Reproduction)

- Raw run/conversation SSE and stored messages already carry the correct
  `assistant.message` before `run.completed`; the missing reply is isolated to
  the TUI reducer.
- The deterministic current-main two-turn reproduction renders and records both
  user prompts but records neither final-only assistant reply.
- A streamed assistant step followed by a tool card disproves the assumption
  that the assistant bubble always owns the viewport tail; later provider
  content must append after the card.
- Replayed terminal messages and completion events require separate visual and
  transcript idempotency; preserving `lastAssistantText` for copy cannot make it
  repeatedly consumable.
- The exact-candidate real PTY confirmed the repaired reducer agrees with both
  durable surfaces: two `assistant.message` SSE events became two assistant
  rows in the same four-message conversation, and reconnect replay introduced
  no visible duplicate.
- A continuation can terminate successfully or fail without emitting new
  assistant content. Before the exact-head review fix, reopening finalization
  without clearing the accumulator made either terminal path re-export the
  previous run's reply; run start must reset both pieces of per-run ownership.

## 2026-08-01 (Conversational Cron Identity and Lifecycle)

- Identity observation: a human-readable cron name is scoped display identity, not a globally stable mutation key; model CRUD/history needs the generated job ID.
- Lookup observation: global name lookup becomes intrinsically ambiguous once independent scopes may share conventional names, so arbitrary first-row selection is data leakage rather than convenience.
- Scheduler observation: the entries map is not the live robfig registry; overwriting its value without removing the former entry hides duplicate future fires.
- Atomicity observation: an inert prepared replacement retains the old entry;
  durable CAS then an infallible in-memory commit makes failure preserve the
  original active job without rollback writes.
- Lifecycle ordering observation: create and resume are paused-first, but
  active replacement must not stage paused. Monotonic registration identities
  plus a final post-jitter/reload guard suppress queued stale callbacks after
  pause/resume or replacement.
- Registry observation: putting scope only in a helper does not protect live model tools; the shared registry constructor is the boundary common to top-level, worktree per-run, and subagent catalogs. Idempotent wrapping prevents callers from stacking redundant scope clients.
- Concurrency observation: pause, resume, and delete are mutations of the same versioned row as edit; ID-only lookup without the `updated_at` read token still permits stale intent.
- Migration observation: SQLite DDL text is not a semantic API. `index_list` plus `index_xinfo` identifies inline, named, quoted, and collated one-column uniqueness while distinguishing composite and partial indexes.
- Provider observation: a direct handler test can pass while the model path is
  unusable. OpenAI rejected `cron_create` before inference when its function
  schema used root `oneOf`; a plain object schema plus fail-closed handler
  validation works across the actual provider boundary.
- Conversation observation: the scheduled executor started two new run IDs
  with the exact original tenant/conversation/agent tuple. Conversation SSE
  remained open across terminal runs and emitted each `run.started` and final
  `assistant.message`; durable messages appended both continuations to the same
  transcript.
- CAS observation: `last_run_at` updates the job version. An update using the
  prior fire's version conflicted after the next minute fired, while a retry
  using the fresh version succeeded. This is visible concurrency protection,
  not a test-only branch.
## 2026-08-01 — Standalone cronsd ingress review

- Filtering jobs only in the HTTP layer is insufficient: the scheduler loads
  the same database independently and would still fire a hidden foreign row.
  Tenant validation therefore runs before scheduler startup.
- Reusing `CRONSD_HARNESS_API_KEY` for ingress would collapse two privilege
  boundaries. A separate ingress secret authenticates management callers and
  never becomes a harnessd credential.
- A liveness-only unauthenticated health response is safe, but operational
  clients must probe authenticated readiness to prove CRUD is actually usable.
- Mutating a `Job` value returned from `ListJobs` is not an ownership claim;
  two server instances can each mutate their copy. The durable conditional
  update must precede visibility.
- Creating an ownership table is not an upgrade migration. Without a
  historical backfill, the first new run after restart can claim an existing
  conversation for a conflicting tenant/agent.

Use this file for observations about system behavior without immediately prescribing code changes.

## 2026-08-01 (Issue #1003 persisted agent ownership)

- Restart removes the Runner's in-memory `conversationOwners` cache.
  ConversationStore retained the tenant axis, while the durable run store
  already retained tenant, agent, and conversation. The repair composes those
  sources at the cache-miss authorization boundary.

- A read-then-create boundary was insufficient for a never-before-seen
  conversation: distinct idempotency keys do not share the server's per-key
  single-flight cache, so both agents could snapshot no rows. Ownership must be
  claimed at the same serialization point as durable run creation.
- Atomic storage alone was also insufficient while ordinary `StartRun` ignored
  `CreateRun` errors after state admission. The authorization decision must
  consume the typed conflict before the first possible provider dispatch.

## 2026-07-31 (Source-Workflow Initial Write Lifecycle)

- Lifecycle observation: a successful `cmd.Start` transfers child ownership to
  the parent even if the first protocol write fails; returning before
  `cmd.Wait` loses both reaping and the primary exit evidence.
- Ordering observation: an initial EPIPE can be a consequence of the child
  already exiting, so it cannot by itself classify the workflow failure.
- Testing observation: a FIFO establishes that the child reached its terminal
  path, while an advisory lock released only when the process exits lets the
  parent prove exit-before-write without sleeps or probabilistic scheduling.
- Scope observation: #1064 starts arbitration only after protocol serving; this
  earlier lifecycle branch must feed that same resolver rather than create a
  second outcome policy.
- Cleanup observation: a live child that already closed stdin turns the initial
  write into EPIPE; terminating and reaping it then produces `signal: killed`.
  That parent-requested cleanup status must not be presented as an independent
  workflow failure.
- Attribution observation: `kill(-pgid, SIGKILL)` success records a cleanup
  request, not exclusive signal provenance. After requesting the same signal,
  EPIPE remains the truthful ordered error for a SIGKILL wait; natural exit 7
  remains distinguishable and primary.

## 2026-07-31 (Runner Dispatcher Identity Under Parallel Load)

- Aggregate observation: the original full-package race command reproduced the
  reported 4/5 failure rate, while the same Runner's instance-owned wait group
  completed normally.
- Repetition observation: `go test -count=5` reuses one test process. Five
  worker-pool construction sites created seven bounded Runners per repetition
  without calling `Shutdown`; their dispatchers therefore outlived their tests
  and contaminated later repetitions. They were real fixture leaks even though
  they did not prove the target Runner leaked.
- Identity observation: a function-name match in `runtime.Stack(all=true)` can
  establish that some dispatcher exists, but cannot establish which Runner
  owns it. Keeping a second Runner alive makes that ambiguity deterministic.
- Ordering observation: blocking the target dispatcher's final defer prevents
  its wait-group completion and therefore prevents target `Shutdown` from
  returning; releasing that exact hook permits return even while the control
  dispatcher remains live.
- Cleanup observation: a failure-safe bounded fixture must unblock provider
  calls before invoking `Shutdown`; otherwise cleanup can wait on the very run
  the fixture still holds blocked.

## 2026-07-31 (Terminal Run Publication Window)

- Concurrency observation: a terminal event can be prepared under the Runner
  lock and persisted outside it without blocking unrelated run queries, but the
  matching status needs one explicit commit point between persistence and
  subscriber fanout. Status before preparation yields incomplete replay;
  status after fanout lets terminal-event consumers briefly read `running`.
- Testing observation: a phase channel at the terminal transition boundary
  deterministically exposes the forbidden state for completed, failed, and
  cancelled paths without relying on aggregate load or fixed sleeps.
- Replay observation: after an HTTP status poll returns terminal, reconnecting
  run SSE from the first event ID must replay exactly one matching terminal
  event and no terminal event of another status.
- Conversation-stream observation: terminal persistence and terminal fanout
  must share the same per-conversation sequence; the global journal lock can be
  released for slow recorder/status persistence only if later events and
  subscriptions on that conversation cannot overtake the terminal event.
- Durability observation: the current store interface has separate
  `AppendEvent` and `UpdateRun` calls, so it cannot promise two-way atomicity.
  The enforceable direction is: never attempt retained terminal status
  persistence after terminal append reports failure. If status update then
  fails or times out, durable event may lead durable status while bounded
  in-memory status and fanout still complete.
- Resource observation: a keyed sequence lock needs waiter-inclusive reference
  accounting. Deleting only after owners and queued waiters release prevents a
  second lock generation for the same key and avoids an unbounded conversation
  map.
- Retention observation: event append success alone is insufficient proof that
  a terminal state can fall back to the store. Pruning also needs acknowledged
  terminal status persistence; otherwise fallback resurrects a non-terminal
  durable row after evicting the only truthful process-local result.
- Outage observation: protecting unpersisted truth and bounding memory require
  one admission boundary. Both ambiguous event appends and failed status
  updates consume it; already-admitted runs may finish above the numeric cap,
  but later admissions stop growth once the outage is observed.
- Recovery observation: `UpdateRun` is an idempotent overwrite and can be
  retried under one shared short context. `AppendEvent` is not safe to retry
  after an ambiguous third-party error because the append may already exist.
  Once status retries succeed, pruning newly durable candidates immediately
  restores the retention bound before another admission is accepted.
- Policy observation: no-store and `StorageModeNone` are distinct. No-store has
  no durable fallback and stays process-local; StorageModeNone intentionally
  resolves the event side while its final status can still make safe pruning
  possible.
- Continuation observation: preserving a source only in Continue's own recovery
  prune is insufficient because concurrent Start recovery calls the same prune
  policy without that local argument. A reservation stored on the source and
  checked by the shared candidate filter protects it across every prune caller.
- Contract observation: terminal replay can lead the later status commit during
  event-first publication. Tests that assert both must wait independently for
  status; only terminal status is guaranteed to imply matching replay.
- Helper-audit observation: aggregate race load repeatedly finds stale tests
  when a helper named as event collection is treated implicitly as run
  settlement. Shared callers all want settlement, while intentional window
  probes use direct `Subscribe`; encoding the distinction once in the test
  helper prevents the next immediate-`GetRun` variant without changing replay.
- Settlement observation: waiting for any terminal status is insufficient if
  the collected transcript is absent or contradictory. A settled test result
  requires exactly one terminal event whose completed/failed/cancelled meaning
  matches status; stream closure is not evidence of transcript completeness.
- Synchronization observation: a timed non-return assertion is meaningful only
  after the tested goroutine proves it reached the intended blocking phase. An
  explicit settlement-entry handshake removes scheduler delay as a false pass.

## 2026-07-31 (Source-Workflow Dual-Error Arbitration)

- Process observation: a child can exit non-zero while closing its stdin also
  reports a broken pipe; both errors are truthful, but only the process exit
  explains the workflow failure and retains bounded stderr diagnostics.
- Testing observation: scheduling a real child to produce both errors is not a
  deterministic regression. Injecting the already-captured outcome signals
  into a pure arbitration seam proves precedence without sleeps while existing
  subprocess tests retain the real process/wait integration coverage.

## 2026-07-30 (Scheduled Conversation Continuation Re-entry)

- Native GUI observation: while Chat stayed visible, a later cron recurrence
  caused both the previously missed and current scheduled assistant replies to
  appear. After deleting the cron and navigating Activity -> Chat again, those
  scheduled replies disappeared even though the messages API still held all
  18 conversation messages. This separated scheduler/execution correctness
  from client replay/reconciliation correctness.
- Cursor observation: a `<run-id>:<seq>` value is only numerically ordered
  within one run. Treating the suffix as a conversation cursor collides as soon
  as two scheduled continuations each emit their own `:0`, `:1`, and later
  events; the complete event ID must be resolved against global append order.
- UI observation: event replay keeps an open transcript current, but it cannot
  by itself repair a view that was absent during the completed run. Re-entering
  Chat needs a durable-message reconciliation boundary, with the live event
  stream continuing to provide low-latency updates afterward.

## 2026-06-28 (Config-Driven Hooks Epic #737)

- Security observation: the trust boundary that matters is directory ownership, not file content. Any directory a project can influence (its own `.harness/hooks/`, plus extra `[hooks] dirs` that could be named by a project-level config) must classify as trust-required; only the user-global dir can be implicit-trust. Classifying extra dirs as project-level closed an injection path where a malicious repo config names its own "trusted" directory.
- Testing observation: process-timeline tests (exec a script, wait for timeout, assert kill) have two independent flake axes — reaping lag (fix: poll for death) and startup lag under suite contention (fix: coordinate pid discovery before the timeout budget). Tests that assert on real PIDs need both handled explicitly.
- Testing observation: Go's `net/http` server only propagates client disconnect into `r.Context().Done()` after the handler consumes the request body; a timeout-test handler that blocks without reading the body hangs until its backstop, which looked like a 30s "slow suite" but was actually a protocol-semantics bug in the test.
- API-shape observation: computing the `/v1/hooks` listing once at startup (rather than re-discovering per request) guarantees the listing can never disagree with what the runner actually registered — the summary is the registration record, not a second query of the filesystem.

## 2026-06-26

- Eval observation: the useful boundary is adapter facts versus oracle facts. The harness can report run status, tokens, cost, tools, and logs, but Terminal-Bench must remain the only source for task pass/fail.
- Baseline observation: the existing `baseline.json` still reads as sample data until a green real-provider campaign records full provenance; accepting it without a live run would make future regressions misleading.
- CI observation: a fake-provider preflight and postprocessing smoke can cover most artifact contract regressions without requiring Docker or paid model calls in pull requests.
- Real-provider smoke observation: the first 2026-06-27 smoke correctly identified an adapter/client stream parsing defect (`harnesscli` rejected SSE keepalive comments), and the accepted rerun proved the fix by producing per-task benchmark and telemetry artifacts for all seven tasks.
- Artifact observation: command transcripts can become secret-bearing artifacts if provider credentials are placed inline in tmux commands; using copied env files keeps `commands.txt` useful without exposing key values.
- Cost observation: the accepted baseline is operationally real but not priced for dollars yet because `gpt-5-mini` is absent from `catalog/pricing.json`; cost gates should treat `cost_status=unpriced_model` as an explicit caveat, not as free execution.

## 2026-04-05

- Process observation: separating umbrella plans, stage specs, implementation logs, and public docs makes it much harder for “planned” orchestration routes or features to leak into operator-facing documentation by accident.
- Refactor observation: `cmd/harnessd` already had enough bootstrap seams that the first runtime-container step could stay inside the package and remain behavior-preserving instead of forcing a broad new `internal/runtime` package immediately.
- Testing observation: direct helper tests for runtime assembly are a useful complement to the existing full-entrypoint startup tests because they pin the extraction seam without weakening the higher-level behavior contract.

## 2026-03-29

- Concurrency observation: training data-structure exercises that try to be clever with fine-grained locking can become less correct than a coarse RW lock when the tests care about determinism more than throughput.
- Testing observation: parent tests that wait on `t.Parallel()` subtests can deadlock because the subtests are scheduled only after the parent returns.
- Matching observation: for these training regex packages, direct AST-based full-string matching was easier to reason about and align with the test expectations than repairing the existing buggy NFA execution paths.

## 2026-03-28

- Repository-shape observation: when experimental snippets live in the module root, they blur the entrypoint for new contributors and can break `go test ./...` before product packages are even evaluated.
- Boundary observation: a separate `playground/` module is a clean way to preserve exploratory code without making product verification depend on training-example correctness.

## 2026-03-18

- Runner observation: concurrent non-terminal emits can reach the recorder channel in a different order than their assigned `Seq` even when the code is race-clean.
- Recorder observation: flushing JSONL by contiguous `Seq` restores file-line ordering to match the canonical in-memory event ledger.
- Message-state observation: the durable contract is still `state.messages` as the only source of truth; step-local snapshots are safe only when reloaded at step boundaries.
- Process observation: recent provider/model feature history landed the core behavior first and then needed follow-up fixes in adjacent surfaces such as gateway config, TUI routing/navigation, API key management, and server `ProviderRegistry` wiring.
- Process observation: making the integration surface explicit under four headings (`config`, `server API`, `TUI state`, `regression tests`) is a lightweight way to expose missing follow-through before merge.

## 2026-03-25

- Step-engine observation: the cleanest extraction seam is the existing `runStepEngine(...)` boundary itself, with `Runner` continuing to own lifecycle/state APIs while a dedicated helper type owns the loop internals.
- Step-boundary observation: steering is drained after `run.step.started` and before the next `llm.turn.requested`, and that ordering is stable enough to pin directly in a focused harness test.
- Persistence observation: before this fix, both direct `/v1/runs` and external-trigger start/continue paths attempted `CreateRun` twice when the server and runner shared the same store.
- Ownership observation: the cleanest contract is runner-owned initial persistence with the server staying read/transport-focused.
- Discovery observation: OpenRouter is the current provider where live model discovery materially reduces backend drift from the real model surface.
- Safety observation: keeping live discovery additive over the static catalog preserves deterministic pricing and alias behavior while still exposing dynamic OpenRouter slugs.
- Failure-mode observation: a TTL cache with stale-cache fallback is enough to keep `/v1/models` and runtime routing from degenerating into fetch-on-every-request behavior.

## 2026-04-05

- Checkpoint observation: the existing approval and ask-user seams were already broker-shaped, which made it practical to replace in-memory maps with a persisted checkpoint service without changing the runner’s public pause/resume API.
- Workflow observation: the harness runner and registry were already separated enough that a workflow layer could stay above them and treat `run` and `tool` steps as orchestration primitives instead of rewriting the step loop.
- Memory-layer observation: explicit working memory works best as a small scoped key/value surface injected ahead of observational recall, not as another transcript mutation mechanism.
- Network observation: compiling v1 networks into workflow-backed sequential run steps kept the new role topology feature from turning into a second orchestration engine.

## 2026-03-05

- Streaming observation: the harness can now surface provider text/tool-call deltas before `llm.turn.completed`, which means clients no longer need to wait for the entire turn to render assistant output.
- Streaming observation: OpenAI streamed tool-call arguments arrive in partial chunks and must be assembled by tool-call `index` before execution.

## 2026-03-04

- Baseline observation: repository initialized with no implementation code yet.
- Harness observation: a run started through `POST /v1/runs` can be consumed via SSE from `GET /v1/runs/{runID}/events` even if the subscriber attaches after initial events, because event history is replayed before live streaming.
- Tool safety observation: default file tools reject workspace-escape paths and the test runner tool bounds execution with a timeout.
- Toolset observation: replacing tools with `read/write/edit/bash` preserved harness loop behavior and SSE outputs; only tool-call semantics changed.
- Bash observation: deny-list command guardrails reject clearly dangerous inputs (for example `rm -rf /`) while still allowing bounded command execution in workspace context.
- Coverage observation: after adding targeted tests for entrypoint, runner failure paths, and HTTP error handlers, all functions now show non-zero execution coverage in `go tool cover -func`.
- Regression observation: automated regression script now catches both total coverage drops and per-function `0.0%` coverage regressions before merge.
- CI observation: regression workflow is runnable in GitHub Actions without extra repository-specific setup beyond Go toolchain availability.
- Hook observation: hook events are emitted around LLM turns and can be consumed by clients for pre/post policy visibility (`hook.started`, `hook.completed`, `hook.failed`).
- Baseline tools observation: `ls`, `glob`, `grep`, `apply_patch`, `git_status`, and `git_diff` are callable through the same tool loop and appear with full lifecycle events.
- Live-run observation: model-driven `apply_patch` replaced the first matching occurrence in the file (title) when `find` was broad, demonstrating deterministic but occurrence-sensitive patch behavior.
- CLI observation: the new `harnesscli` client can attach to the existing SSE API and reliably terminate on `run.completed`/`run.failed` without hanging, making it a practical test harness for manual integration checks.

## Entry Template

- Date:
- Environment/context:
- Observation:
- Evidence:
- Hypothesis:
- Suggested follow-up:
- Modular-tooling observation: moving tools into `internal/harness/tools/` preserved registry-driven execution semantics while making per-tool changes isolated and easier to test.
- Policy observation: `permissions` mode cleanly blocks mutating/fetch/execute actions with structured `permission_denied`/`permission_error` payloads, while `full_auto` remains fast-path default.
- Live schema observation: OpenAI tool schema validation rejects array properties without `items`; adding explicit `items` on `apply_patch.edits` and `todos.todos` resolved request-time failures.
- Live-run observation: after schema fix, a tmux-hosted `gpt-5-nano` run completed successfully and exercised new `read` pagination/line metadata in event stream outputs.
- AskUserQuestion observation: a run now exposes a deterministic paused state (`waiting_for_user`) with explicit `run.waiting_for_user` and `run.resumed` events, enabling frontend clients to render input prompts without polling ambiguous tool state.
- Broker observation: invalid answer submissions no longer break run execution; they return `400` while preserving pending question state until a valid submission arrives.
- Timeout observation: when no answer is submitted before `HARNESS_ASK_USER_TIMEOUT_SECONDS`, the run fails immediately after the AskUserQuestion tool call with a timeout error, preventing indefinite stalled runs.

- Observational-memory observation: run-level transcript snapshots now exist in runner state and can be consumed by tools through a read-only context interface, avoiding direct mutable message-array access from tools.
- Observational-memory observation: local mode uses SQLite WAL + per-scope in-process ordering, which keeps standalone behavior deterministic while preserving a migration path to external coordination.
- Observational-memory observation: model-backed observer updates are emitted as explicit `memory.observe.*` events, giving client UIs an auditable trace for automatic memory writes.
- Observational-memory observation: memory control is now explicit and reversible (`enable`/`disable`) through a first-class tool, keeping default execution behavior unchanged unless memory is activated.

- Prompt-system observation: static prompt composition is now deterministic and file-backed; section ordering remains stable across runs for the same intent/model/extensions input.
- Runtime-context observation: runtime metadata is injected every turn without transcript growth, so previous runtime snapshots do not accumulate across tool loops.
- Validation observation: invalid intent/profile/extension identifiers now fail run creation immediately, preventing silent prompt drift.
- Compatibility observation: explicit `system_prompt` requests continue to bypass prompt composition, preserving previous operator override behavior.
- Benchmark observation: a small private Terminal Bench suite is the right level of signal for this repo right now because it can exercise the real harness loop without turning paid benchmark runs into a pre-merge gate.
- Benchmark observation: copying the live checkout into each Terminal Bench task container avoids drift between benchmark code and the code under test, which is especially useful for local operator runs.

## 2026-07-30 (Issue #1026 Live Feedback Publication)

- UX observation: the existing Ctrl-V attachment chip is sufficient for
  `/feedback`; no path picker or browser composer is needed.
- GitHub observation: documented release-asset URLs render inline when used as
  image Markdown in an issue, making the supported releases API a viable
  attachment store even though `gh issue create` has no attachment option.
- Live evidence: issue #1030 contained an inline 2,462,402-byte PNG and a linked
  2,450,864-byte diagnostic zip. Downloading the published PNG reproduced the
  source SHA-256
  `68a9cc58a06ba009b3a73d2e6e5e5c8d72e1dcbb313860abc5aa8f445a447b08`;
  the zip held all nine expected members.
- Lifecycle observation: keeping the chip until the asynchronous issue result
  arrives provides a natural retry path; path-selective cleanup also avoids
  deleting an attachment added while publication is running.
- Test-environment observation: macOS Keychain integration tests can time out
  when launched from a tmux bootstrap namespace even though the same binaries
  pass directly in the logged-in GUI context. Final regression evidence must
  therefore record the launch context as well as the command.

## 2026-07-31 (Issue #1003 Review Fixes)

- The remote start contract already emits the same deterministic correlation as
  `Idempotency-Key`; enforcing equality at the authenticated endpoint made
  the retry boundary explicit without changing the typed #1001 request shape.
- Process-local dedupe was not sufficient: the HTTP transport can replay an
  accepted request after the response is lost, and recreating harnessd erased
  the cache. The built-in run store is the correct narrow owner for the
  tenant/key/fingerprint-to-run reservation because harnessd and its runner
  already share it across restart.
- Client-side retry avoidance and server-side idempotency are independent.
  The client does not loop or follow redirects; the server still reserves the
  identity before `StartRun` and returns it for later replays.

## 2026-07-31 (Issue #1003 Remote cronsd Harness Dispatch)

- Boundary observation: the existing harnessd run API already has tenant
  ownership checks, but a dedicated cron endpoint is needed to keep scheduled
  job/execution correlation explicit across the standalone daemon boundary.
- Failure observation: remote start failures can be classified from status and
  context without retaining response bodies; prompt and bearer contents are
  absent from the observed log shape.
- Readiness observation: validating active jobs at cronsd startup and before
  job persistence prevents a configured harness job from becoming silently
  unschedulable, while preserving shell-only deployments.
- Verification observation: the exact unmodified `./scripts/test-regression.sh`
  passed in foreground execution at 85.6% total coverage with zero uncovered
  functions; the live local canary returned a stable harness run ID and a
  successful cronsd execution.

## 2026-07-31 (Issue #1003 Fresh-Store Authentication Review)

- A test that manually calls `MigrateAPIKeys` can prove store behavior while
  missing the production startup contract. Exercising
  `buildPersistenceBootstrap` against a fresh database exposed the absent
  `api_keys` table immediately.
- The narrow owner is harnessd persistence bootstrap: it already owns the
  configured run-store lifecycle and can apply the existing idempotent schema
  before authenticated HTTP traffic begins.

## 2026-07-31 (Issue #1003 Final Review Reliability)

- Durable idempotency does not permit non-fatal initial persistence: the
  accepted binding is useful only when its reserved run exists in the same
  durable store before dispatch.
- Once durable storage answers sequential and restart replay, retaining
  completed results in the process-local map is both redundant and an
  unbounded-memory risk. Single-flight state should end with the flight.
- Scheduler timeout configuration and transport timeout configuration are
  independent bounds. Nesting contexts naturally preserves the earliest job,
  parent, or daemon deadline and the standard cancellation cause.
- A queued durable row under an unaccepted binding is a recoverable
  persistence-before-dispatch state, not evidence that work was accepted.
  Recovery must revalidate request identity, resume the same reserved ID, and
  mark acceptance only after dispatch succeeds.
- The inverse partial failure also matters: dispatch may succeed while the
  acceptance mark fails. A retry in that same process must prefer the active
  run over the still-queued durable snapshot; restart-only resume is correct
  only when no current runner state exists.
- Hydrating only the public run state is insufficient after restart: the
  internal request drives provider/model selection and prompt resolution.
  Durable resume must carry the persisted model into both representations.
- An accepted idempotency binding and a queued durable run can coexist after
  shutdown drains the worker queue. Accepted status cannot be a replay
  shortcut; the durable run status and current runner state decide recovery.
- HTTP success headers do not complete a remote start. The bounded JSON body
  read remains part of the request and may terminate through deadline/cancel.

## 2026-08-01 (Issue #1003 Cross-Process Dispatch Lease)

- A durable idempotency key does not itself serialize dispatch. Two processes
  can read the same queued row and each admit it into a different runner.
- The narrow durable transition is a conditional lease acquisition before
  runner admission. Returning `accepted` without considering a live/expired
  lease either duplicates live work or strands crashed queued work.
- Owner-qualified acceptance prevents a delayed first process from marking a
  takeover performed by a replacement. Expiry provides recovery; it is not a
  terminal execution or no-overlap policy.

## 2026-08-01 (Issue #1003 Lease Review Observations)

- Conditional update plus an unguarded read is not a linearizable acquired
  result: another owner can replace the row between those operations.
  `RETURNING` must supply the winner's result directly.
- A lease timestamp compared to the caller's clock turns clock skew into
  authorization. Shared SQLite time makes every process evaluate one clock;
  caller timestamps contribute only a duration.
- Accepted queued status is not liveness. Heartbeat renewal while the local run
  is queued/running distinguishes backlog from a dead owner; runner shutdown,
  terminal status, absence, and renewal loss terminate that claim.
- Concurrent additive migration must tolerate only a proven winner: rechecking
  the column after ALTER failure preserves availability without masking a real
  migration error.
## 2026-08-03 (Issue #1106 liveness/recovery evidence)

- Focused red cases reproduced all three review findings: a single daemon
  stayed at `retry_wait`; a second manager recovered after mere lease expiry;
  and a legacy NULL lease remained dispatching.
- After the repair, focused normal tests pass for same-manager retry rearm,
  live-owner/second-bootstrap exclusion, and nullable-lease recovery. The
  complete callback normal suite and repeated callback race suite are green.
## 2026-08-03 (Issue #1106 final review evidence)

- New deterministic reds showed future crash leases remained dispatching,
  deadline handoff exceeded its retry budget, and recovery could proceed with
  no process-loss authority. All pass after the repair, alongside a real
  killed-child flock-release regression.
- Complete callback tools normal and race suites pass after the repair. The
  prior takeover test now verifies the deliberately persisted backoff rather
  than assuming immediate retry eligibility.

## 2026-08-03 (Issue #1132 wait-state observation)

- Hosted race CI observed `running` at the fixture's status assertion. This
  was not evidence that a run published an incorrect terminal/intermediate
  lifecycle; it was evidence that the fixture had observed the intentionally
  earlier pending-broker boundary.
- Event history plus live subscription establishes the consumer-visible
  `run.waiting_for_user` boundary without sleeps or polling. Repeated normal
  and `-race` focused execution now reaches that boundary before compacting.
## 2026-08-03 (Issue #1135 cron recovery observation)

- The hosted race did not establish that terminal persistence or scope release
  was broken. The old channel only showed mock-store entry; it could be
  observed before the reconciliation goroutine returned and released its lease.
  The repaired fixtures use the scheduler-owned `reconcileWG` as the causal
  completion boundary and keep the pre-return no-overlap assertion explicit.
- Focused normal/race x100 and complete cron normal/race pass without a sleep,
  and the repository regression's normal, race, and coverage phases pass.
## 2026-08-03 (Issue #1141 callback deadline-release observation)

- A heartbeat's absence from `ExtendLease` is not evidence that deadline cancellation failed: under CI load the independent deadline can cancel the admission first. Fixtures now observe both the deadline and starter context cancellation, so their outcome no longer depends on heartbeat scheduling. Normal and race stress x20 and the full regression passed without callback runtime changes.
## 2026-08-03 (Issue #1122 ownership observation)

- A visible native interaction is an authority-bearing object, not just a
  transcript decoration. If it is retained across `currentRunID` replacement,
  resolving the action at click time changes the user's target from A to B.
- Generation checks alone cannot protect this: they invalidate asynchronous
  completions, while a stale SwiftUI closure can issue a fresh request. The
  captured run ID must be checked before creating its task and again before
  issuing its network operation.
## 2026-08-04 (Issue #1147 default callback admission observation)

- In an isolated merged-main harness, a five-second callback recorded
  `callback.dispatching` three times and then `callback.failed`, with attempt
  3 and safe error `callback admission unavailable`. The originating
  conversation did not gain an assistant continuation.
- The deterministic regression now composes the default persistence bootstrap,
  durable callback store/recovery, callback starter, and Runner. It observes
  one reserved ID admitted with preserved tenant/agent/conversation scope;
  future live API/TUI/native proof must still show the actual transcript turn.

## 2026-08-04 (Issue #1153 cron lease-poll observation)

- Two-server delivery tests can legitimately avoid `waitForCronRunDispatch`
  when their synchronized release lets one process acquire immediately. That
  observation does not establish polling or cancellation correctness.
- A scripted first `acquired=false, Accepted=false` result makes the intended
  higher-level retry observable without sleeping for a real lease duration;
  a cancelled context wins over an intentionally hour-long poll interval.

## 2026-08-05 (Issue #1175 fixture observation)

- Git returns a canonical registered worktree path; on macOS `/var` aliases `/private/var`.
  Canonical fixture identity avoids a spelling-only failure while preserving the
  test's registered-worktree evidence.

## 2026-08-05 (Issue #1173 durable replay observation)

- A durable run ID is not evidence that a rollout file exists. Replay must
  choose its source explicitly and return a fresh lifecycle ID for existing
  SSE/transcript rendering to observe.
## 2026-08-04 (Issue #1149 cron history observation)

- Durable execution listing already existed behind the cron adapters and agent
  tool; the missing behavior was the public HTTP route and ownership guard.
## 2026-08-04 (Issue #1148 TUI stream observation)

- Cancelling an SSE request cannot retract already queued results; every bridge
  message must carry and validate source conversation identity before reduction.

## 2026-08-05 (Issue #1180 bootstrap observation)

- Observation: in the authoritative linked worktree at `971d9eba`, direct
  `go build -buildvcs=true` emitted no `vcs`, `vcs.revision`, or
  `vcs.modified` settings. The initializer consequently rejected its own
  candidate with revision/modified reported missing.
- Interpretation: this is a local compilation-provenance seam, not a runtime
  scheduled-task failure. The repair must preserve independent binary metadata
  evidence instead of accepting the missing fields.

## 2026-08-05 (Issue #1190 MCP transport observation)

- A focused production-path regression was red before the repair because
  `dialHTTP` exposed a nil transport, even though the same gated request could
  occasionally still reach its 401 endpoint. The ownership assertion is the
  deterministic causal boundary; the gated global-cleanup sequence validates
  the intended strict-auth outcome after the clone is present.
- The added legacy control makes the opposite path directly observable: a nil
  client transport delegates to the global cleanup target, which cancels the
  held request before any authorization response can be classified.
# 2026-08-05 (Issue #1186 cron validation observation)

- A raw `400 validation_error` cannot be reconstructed safely from a generic
  HTTP error string after the remote boundary. Its error class must survive the
  client parse and adapter translation. The public create handler also must
  retain JSON field presence to distinguish an omitted timeout (default) from
  an explicit zero (invalid).
# 2026-08-05 (Issue #1194 blame porcelain observation)

- A porcelain metadata line is not self-describing merely because it is long:
  parser ownership requires the complete header grammar. Treating a path as a
  final-line position silently produces zero and corrupts the following content
  row. Optional Git enrichment needs its own success boundary because command
  stdout/stderr are merged by the shared runner.

# 2026-08-05 (Issue #1195 diff-stat observation)

- Git's human-readable stat summary preserves a stable grammar for the file
  count (`<positive integer> file(s) changed`) distinct from the optional
  insertion/deletion clauses. Parsing the checked full clause keeps structured
  aggregate data consistent with the stat text across normal and `stat_only`
  requests without requiring client-side reconstruction.
