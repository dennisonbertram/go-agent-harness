# Observational Log

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
