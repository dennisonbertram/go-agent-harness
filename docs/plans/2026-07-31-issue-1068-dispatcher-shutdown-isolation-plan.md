# Issue #1068: Instance-Scoped Dispatcher Shutdown

## Context

- Governing GitHub issue: [#1068](https://github.com/dennisonbertram/go-code/issues/1068)
- Problem: the shutdown regression scans every process goroutine for the shared
  `poolDispatcher` function name, so an unrelated live Runner can make a target
  Runner look leaked after its own `Shutdown` has completed. Review also proved
  that five bounded worker-pool construction sites create seven Runners per
  repetition and omit `Shutdown`, leaving real dispatchers alive across
  `go test -count` repetitions.
- User impact: the race gate can block independent changes and obscure a real
  lifecycle leak.
- Constraints: strict red-green TDD; retain queue draining, cancellation,
  idempotent shutdown, and the existing instance-owned wait-group contract;
  do not touch PR #1060, PR #1055, or issue #1067.

## Scope

- In scope: reproduce the aggregate failure, add a deterministic two-Runner
  control, make dispatcher lifecycle assertions instance-safe, and repair the
  Runner seam only if the instance signal proves a runtime leak; deterministically
  prove and clean up every bounded worker-pool test fixture, releasing blocked
  providers before shutdown.
- Out of scope: worker-pool redesign, remote cron recovery, terminal event
  publication, or generic goroutine accounting.

## Documentation Contract

- Feature status: `implemented and locally verified; review-fix promotion pending`
- Public docs affected: none; no user-facing contract changes.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: engineering, observational, system,
  and long-term-thinking logs.

## Test Plan (TDD)

- First red: a two-Runner regression keeps a control dispatcher live, shuts
  down the target, proves the target's instance signal completed, and then
  demonstrates that the old process-global substring assertion still reports
  a dispatcher.
- Review red: a bounded Runner created inside a subtest must fire its exact
  dispatcher-exit hook when fixture cleanup completes; before the shared
  cleanup is added, the parent observes the dispatcher still live and then
  explicitly shuts it down to keep the red itself leak-free.
- Green contract: target `Shutdown` cannot return until the target dispatcher's
  instance-owned exit signal completes; the control remains live until its own
  cleanup.
- Stress: focused normal/race at `-count=100`, complete harness race at
  `-count=5`, affected harness/server normal/race/vet, and unchanged foreground
  non-TTY `./scripts/test-regression.sh`.

## Cross-Surface Impact Map

- See `2026-07-31-issue-1068-dispatcher-shutdown-isolation-impact-map.md`.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Link a contract-complete structured GitHub issue before implementation.
- [x] Record current architecture, callers, consumers, and source-of-truth search evidence.
- [x] Document feature status and exact contract before code.
- [x] Complete and reconcile the cross-surface impact map before implementation.
- [x] Add characterization coverage before structural refactors (no structural refactor planned).
- [x] Write failing tests first.
- [x] Review ownership/copy semantics (no exported or copied mutable type changes).
- [x] Implement review-requested bounded-fixture cleanup.
- [x] Refactor while tests remain green.
- [x] Update docs, status ledgers, and indexes.
- [x] Update engineering/system/observational logs as needed.
- [x] Rerun the full test suite for the review fix.
- [x] Update existing PR #1069 and leave it unmerged.

## Verification Outcome

- Red aggregate: `go test -race ./internal/harness -count=5` failed 4/5
  repetitions at the process-global post-Shutdown stack assertion.
- Red deterministic: the two-Runner control failed immediately when the old
  assertion treated the live control dispatcher as the target's leak.
- Review finding: five pre-existing bounded construction sites in
  `runner_worker_pool_test.go` created seven Runners per repetition and omitted
  `Shutdown`; other bounded harness tests were audited and already own explicit
  shutdown paths.
- Review red: `go test ./internal/harness -run
  '^TestWorkerPoolTestRunnerCleanupStopsDispatcher$' -count=1` failed because
  the bounded Runner's exact dispatcher-exit hook had not fired after subtest
  cleanup; the test then explicitly released and shut down that Runner.
- Review green: the cleanup regression passed normal and race at `-count=1`;
  all worker-pool tests passed normal and race at `-count=100`.
- Green focused: normal and race `-count=100` passed.
- Green aggregate: complete `internal/harness` race `-count=5` passed.
- Green affected: harness/server normal, race, and vet passed.
- Green repository: unchanged foreground non-TTY `./scripts/test-regression.sh`
  passed at 85.6% total coverage with zero uncovered functions.
- Promotion state: the review fix is committed locally for parent handoff; PR
  #1069 remains open and unmerged, with no review-fix push performed here.

## Risks and Mitigations

- Risk: a test-only hook could become a second lifecycle source of truth.
- Mitigation: expose only a close-only signal tied to the same defer that calls
  `dispatcherWG.Done`; `Shutdown` continues to own waiting through the existing
  wait group.
- Risk: shutdown changes could deadlock queue draining or active-run timeout.
- Mitigation: preserve production ordering unless the deterministic instance
  test proves it wrong, and rerun all existing shutdown/worker-pool regressions.
