# Issue #1002 — Conversational Cron CRUD

## Context

- Governing GitHub issue: [#1002](https://github.com/dennisonbertram/go-code/issues/1002)
- Parent epic: [#1000](https://github.com/dennisonbertram/go-code/issues/1000)
- Dependency: [#1001](https://github.com/dennisonbertram/go-code/issues/1001), closed and verified as an ancestor of `origin/main` at `fedcf6073135deb7cce1fa49921aa698a9cc7cd7`.
- Repair provenance: local branch `codex/issue-1002-repair-v2` at base
  `4cfa5b63e8f1857cf82b5c5000c5d8d8e47e09e0`; the acceptance-audit repair is
  locally review-clear. Its promotion candidate is rebased onto `origin/main`
  `3506e01c997231c46920b45bf8947c50087dd863`; exact-tree full regression and
  live same-conversation CRUD/fire evidence are green, and the guarded PR
  update remains pending.
- Problem: the deferred model-facing catalog exposed shell-only creation and lacked a safe in-place update path. An agent could schedule a shell command, but could not create a typed harness continuation or safely edit a recurring job.
- User impact: an operator can ask the agent to create, inspect, change, pause/resume, and delete a recurring conversation job while preserving its stable job ID, immutable scope, execution history, and same-conversation harness behavior.

## Scope

- In scope: model-facing shell-compatible and explicit harness `cron_create`, typed prompt execution config, immutable RunMetadata binding at the shared model-registry constructor, ID-only model CRUD/history, distinct ambiguity-safe operator name lookup, scoped SQLite identity and semantic index-metadata migration, atomic store/scheduler reconciliation, mandatory update/pause/resume/delete version tokens, timeout validation, lifecycle/concurrency tests, registration, descriptions, and required documentation/log/index updates.
- Out of scope: raw `cronsd` authentication and readiness (#1003), terminal linkage/overlap (#1004), callback persistence/retry, macOS controls, and any other epic child.

## Documentation Contract

- Feature status: `acceptance-audit repair implemented locally, review-clear, rebased, full-regression green, and real-provider same-conversation CRUD/fire green; guarded push, hosted checks, and merge remain pending`
- Public docs affected: embedded tool description and plan/log artifacts; no user guide route list is changed because this is a model-facing tool.
- Spec docs to update before code: this plan and impact map.
- Implementation notes to add after code: engineering, observational, and system logs with exact red/green/full-gate evidence.

## Test Plan (TDD)

- New failing tests first:
  - `TestCronCreateHarnessJobUsesImmutableRunScope` proves typed harness config and scope override rejection.
  - `TestIntegrationCronUpdateCompareAndSwapAllowsOneConcurrentWriter` proves atomic one-winner persistence and no stale active scheduler resurrection.
  - `TestServerUpdateJobRejectsNonPositiveTimeout` proves authoritative unsafe-timeout rejection.
  - `TestEmbeddedCronModelToolsFullScopedLifecycle` proves create/list/get/update/pause/resume/delete through model tools and a stateful scoped adapter.
  - ID-only schema/behavior tests prove model CRUD/history never falls back to names; operator lookup uses a distinct route and returns typed ambiguity.
  - legacy SQLite migration variants prove quoted identifier recognition, durable job/history/timestamp preservation, idempotence, scoped uniqueness, and integrity.
  - remote owned lifecycle and concurrent update/delete tests prove scoped CRUD/history and no post-delete re-arm under normal/race execution.
  - registry, comprehensive tool-list, schema, and embedded-description tests prove catalog coverage.
  - assembled default-registry tests prove raw embedded and remote adapters are
    scoped automatically from RunMetadata, operator access remains raw, and
    stale pause/resume/delete requests conflict without a manual wrapper.
  - SQLite `index_list`/`index_xinfo` tests prove single-column collated global
    uniqueness is migrated while composite and partial indexes are ignored.
  - Remote and embedded lifecycle tests inject scheduler prepare/CAS failure
    and restart; create/resume remain paused-first on failure, while active
    schedule replacement preserves the old active row/entry until
    `Prepare` → CAS → infallible `Commit` succeeds.
  - `cron_get` distinguishes a successful empty history from an unavailable
    history query while preserving the readable job and array result shape.
- Existing tests to update: cron registry/tool lists, description manifest, remote adapter request mapping, and the assembled embedded harness path.
- Regression tests required: invalid JSON, missing ID/version, invalid timestamp/timeout, client/service errors, omitted-field preservation, concurrent CAS, scope isolation, and harness starter correlation.

## Cross-Surface Impact Map

See `2026-07-31-issue-1002-conversational-cron-crud-impact-map.md`.

## Implementation Checklist

- [x] Dependency readiness checked against current `origin/main`.
- [x] Current ownership and duplicate-tool search recorded.
- [x] Plan and impact map created before implementation.
- [x] Add failing tests and capture the expected red result.
- [x] Implement atomic persistence CAS and move scheduler reconciliation after a successful write.
- [x] Add explicit model-facing harness creation while preserving legacy shell creation.
- [x] Enforce authoritative non-empty shell commands, non-empty harness prompts, valid schedules, and positive explicitly supplied timeouts on create/update boundaries.
- [x] Replace the test-only scope wrapper with the production model-facing
  client boundary; fail closed for cross-scope read/history/mutation across
  embedded and remote clients.
- [x] Expose typed harness prompt updates, enforce mutually exclusive execution
  inputs, and prove the updated prompt reaches the assembled harness starter.
- [x] Make model CRUD/history ID-only and separate operator name lookup with a
  typed ambiguity result.
- [x] Move remote and embedded ownership checks into authoritative SQLite scope
  predicates; allow the same name across independent ownership tuples.
- [x] Make active schedule replacement collision-safe with inert `Prepare` →
  durable CAS → infallible `Commit`; abort prepare/CAS failure without changing
  the prior active durable row or live entry, while create/resume remain
  paused-first.
- [x] Migrate inline, named, quoted, bracketed, and backtick global name
  constraints without losing jobs or execution history.
- [x] Scope cron once at `NewDefaultRegistryWithOptions`, covering top-level,
  worktree per-run, and subagent registries while leaving operator adapters raw;
  make an already-scoped client idempotent at that boundary.
- [x] Require `expected_updated_at` from `cron_get` for model pause/resume and,
  after auditing the remaining mutations, delete; propagate CAS conflicts
  through embedded and remote adapters while preserving raw operator delete.
- [x] Replace SQL-text uniqueness detection with semantic SQLite
  `index_list`/`index_xinfo` inspection, including `COLLATE NOCASE`, exact
  single-key-column recognition, composite/partial exclusion, idempotence, and
  integrity checks.
- [x] Blocking-review affected packages passed normal/race with
  `go test [ -race ] ./internal/cron ./internal/harness/tools/... ./internal/harness ./cmd/harnessd -count=1`; normal timings were
  9.284s/11.486s/1.553s/9.323s/1.295s/1.573s/4.473s/6.469s/3.333s and race timings were
  10.958s/12.295s/2.499s/10.693s/1.971s/2.268s/4.424s/6.985s/3.122s in command output order.
- [x] Lifecycle-convergence follow-up passed the same bounded nine-package
  normal/race gate. Normal timings were
  8.993s/11.698s/1.642s/9.359s/1.307s/1.547s/4.757s/5.893s/3.169s; race
  timings were
  11.223s/12.901s/2.103s/10.298s/1.416s/2.454s/4.639s/7.720s/4.077s.
- [x] Run focused affected-package normal/race: normal passed in
  8.806s/8.565s/1.451s and race passed in 10.886s/10.070s/2.963s for
  `internal/cron`, deferred tools, and `cmd/harnessd` respectively.
- [x] Review follow-up: move operator name lookup from a decoded path segment
  to a query value; slash, spaces, percent, Unicode, empty input, and method
  behavior pass focused normal/race, with the complete cron package green in
  8.783s/10.807s.
- [x] Prior PR candidate passed focused normal/race verification and the
  foreground `./scripts/test-regression.sh` at
  `coveragegate: PASS (total=85.6%, min=80.0%, zero-functions=0)`; this is
  historical evidence only and does not prove the current uncommitted repair.
- [x] Latest admission-lock and history-availability candidate passes
  `go test [ -race ] ./internal/cron ./internal/harness ./internal/harness/tools ./internal/harness/tools/deferred ./cmd/harnessd -count=1`.
  Normal timings were 9.522s/5.596s/11.403s/9.466s/2.402s; race timings were
  11.202s/8.741s/12.588s/10.954s/4.512s.
- [x] Rebase the repaired candidate onto current `origin/main` at `3506e01c`;
  all production/test changes applied cleanly and shared log/index conflicts
  retained both histories.
- [x] Run the rebased full foreground regression: normal, complete race, and
  coverage pass at 85.7% total with zero uncovered production functions.
- [x] Run a real OpenAI-backed same-conversation canary. Eleven completed runs
  exercised all eight model cron tools, rejected a stale update version,
  produced two scheduler-started continuations in the original conversation,
  preserved linked execution run IDs, and ended with an empty list plus 404
  direct read after deletion.
- [ ] Guarded-force update PR #1057 (`Closes #1002`) and merge only after hosted
  checks and the production merge gate pass.

## Risks and Mitigations

- Risk: a model mutation could overwrite or delete after a concurrent operator edit. Mitigation: `cron_update`, `cron_pause`, `cron_resume`, and `cron_delete` require `expected_updated_at` from `cron_get`; the service performs persistence-level compare-and-swap and returns typed/HTTP 409 on zero-row matches. Raw operator endpoints retain their existing compatibility contract.
- Risk: omitted JSON fields could erase existing configuration. Mitigation: pointer fields plus tests that assert nil for omitted values.
- Risk: ownership could become model-mutable. Mitigation: update request has no tenant, agent, conversation, or job-ID mutation fields; existing #1001 scope binding remains authoritative.
- Risk: harness creation could silently degrade to shell. Mitigation: explicit `execution_type` validation, typed `{prompt}` config, distinct shell/harness inputs, and an assembled starter test; remote cronsd transport remains #1003.
- Risk: malformed execution config or an unsafe timeout could reach persistence. Mitigation: `ValidateExecutionConfig` runs at HTTP and embedded create/update boundaries; explicit HTTP/tool timeout values must be positive while omitted create values retain the 30-second default.
- Risk: a name-only operator call can match several scoped jobs. Mitigation:
  `/v1/jobs/{id}` is ID-only; `/v1/jobs/by-name?name=...` is explicit and returns
  typed `ErrJobAmbiguous` unless ownership scope selects one row.
- Risk: scheduler replacement or a queued stale callback could diverge from a
  completed lifecycle mutation. Mitigation: active replacement uses inert
  `Prepare` → durable CAS → infallible `Commit`, while create/resume remain
  paused-first. Globally monotonic registration identities are checked after
  jitter and atomically with execution-row creation under the same lock used by
  prepare/commit/remove.
