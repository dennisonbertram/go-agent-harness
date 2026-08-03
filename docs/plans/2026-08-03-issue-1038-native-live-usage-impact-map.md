# Issue #1038 — Native Live Terminal Usage Reconciliation Impact Map

## Task

- Task / issue: #1038
- Plan link: `2026-08-03-issue-1038-native-live-usage-plan.md`
- Owner: native transcript reducer
- Status: in implementation

## Current Ownership, Callers, and Data Flow

- Entry points: `RunSession.apply`, per-run and conversation SSE streams.
- Owning source of truth: `Transcript.apply` reduces values, while `RunSession` owns which run's accounting may be retained; harness terminal events contain the final accounting snapshot.
- Consumers: SwiftUI transcript usage views and `RunSessionLiveTests`; terminal UI state becomes observable directly after reduction.
- Search evidence: `rg 'usage.delta|usage_totals|RunSessionLiveTests' macapp/Sources macapp/Tests internal/harness` showed `recordAccounting` emission and terminal payload ownership.
- Conclusion: normalize at the single reducer boundary rather than coordinate independent stream tasks or modify server order.

## Config, API, CLI, and Tools

- Config/defaults/environment: None; the live script's fake provider stays unchanged.
- Endpoints/wire: None; consume established `usage_totals` and `cost_totals` fields.
- CLI/TUI/tools: None; no behavior contract changes.
- Errors/validation: incomplete terminal snapshots must preserve prior reduced totals.

## Persistence and Compatibility

- Schema/migration/cache: None.
- Compatibility: older servers omitting terminal accounting keep current delta-derived behavior; newer servers get a terminal reconciliation fallback.
- Partial rollout: safe because each event is independently reducible.

## Lifecycle, Security, and Reliability

- Concurrency/cancellation: value-type reducer remains main-actor-owned by `RunSession`; no task timing or sleep is added.
- Security/privacy: no new data exposed; existing accounting data already arrives over the authenticated stream.
- Failure recovery: terminal totals repair a missing/dropped prior delta only for their admitted run; a new/incomplete/local-failed run clears prior-run accounting; standalone durable message sync clears unknown-run accounting. A duplicate terminal replay retains totals when its run is still the accounting owner even though deduplication skips its reducer call. A stale terminal from a different run is not lifecycle-reduced, but its durable rows are reconciled while the current owner’s accounting and lifecycle are retained; duplicate/replayed/stale terminal events cannot overwrite a newer run's totals.
- Ordering proof: stale-terminal regressions use an explicit application gate that the test opens only after `accountingRunID`, `runState`, and all five B accounting fields have been observed. A failed-B replay additionally retains its event-only error while durable rows rebuild.
- State ownership: rejected foreign lifecycle, approval, and waiting frames are suppressed before transcript reduction; rejected foreign content remains renderable durable history. This prevents stale `run.started` from making the current completed run busy and blocking terminal durable reconciliation.

## Product and Integration Surfaces

- Server/runtime: no modification; existing final event is authoritative.
- TUI/web/macOS: macOS only changes; TUI/web payload compatibility verified by no server modification.
- Provider/model/tools: fake and real provider accounting share the existing event contract.
- UX: usage is rendered consistently with the terminal completion state, avoiding a completed-with-zero summary.

## Deployment and Operations

- Deployment/rollback: normal native app release; revert reducer commit if needed.
- Observability: live `RunSessionLiveTests` is the direct user-visible proof.
- Runbooks: no operator action changes.

## Regression Tests

- First red: terminal-only `run.completed` transcript reduction with real JSON accounting keys.
- Acceptance: terminal reconciliation, existing transcript deltas, focused live session.
- Negative/lifecycle: missing fields preserve previous usage; a forced per-run-first duplicate terminal reconciles durable rows without clearing same-run usage; an application-gated stale-A `run.started` plus terminal retains both durable rows while preserving B’s exact totals/owner/state; failed B retains event-only error detail; terminal event does not require delay.
- Exact commands: `swift test --filter TranscriptTests`, `./scripts/live-test.sh --filter RunSessionLiveTests`, lint/build commands, and `./scripts/test-regression.sh`.

## Documentation and Handoff

- Docs: plan, impact map, plan index, engineering log, logs index.
- Handoff: PR cites the exact terminal-payload contract and test-first failure.
