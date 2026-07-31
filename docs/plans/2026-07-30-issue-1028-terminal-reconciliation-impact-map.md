# Cross-Surface Impact Map: Issue #1028 Terminal Reconciliation

## Task

- Task / issue: preserve failed/cancelled GUI state during durable transcript
  reconciliation, #1028.
- Plan link: `2026-07-30-issue-1028-terminal-reconciliation-plan.md`.
- Owner: Codex.
- Status: implemented; verification and merge pending.

## Current Ownership, Callers, and Data Flow

- Entry points: conversation SSE terminal events and Chat durable-message sync.
- Owning packages/types/functions and source of truth:
  `RunSession.streamConversation` applies authoritative events;
  `Transcript.apply` owns event-derived state; `Transcript.load` rebuilds
  persisted rows.
- Callers, consumers, events, and downstream data:
  callback/cron conversation streams, Chat status, tool/error rows.
- Similar abstractions searched: historical conversation load, Chat re-entry
  sync, per-run stream application, transcript reset/load.
- Search commands/evidence:
  `rg -n 'reconcilePersistedMessages|runState = \.completed|runFailed|runCancelled' macapp/Sources macapp/Tests`.
- Duplication/ownership conclusion: repair the existing reconciliation
  boundary; do not add a second transcript or state owner.

## Config, API, CLI, and Tools

- User-facing config added or changed: none.
- Defaults / fallbacks: existing successful durable reconciliation remains.
- Environment variables, config files, or saved settings touched: none.
- Endpoints, request fields, response fields, or server wiring affected: none.
- CLI commands, tools, wire formats, or integrations affected: none.
- Error states / validation changes: failed/cancelled remain visible after
  message hydration instead of being mislabeled completed.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, or ownership changes: none.
- Backward/forward compatibility and versioning: Swift client-only state
  projection; event/message contracts remain compatible.
- Partial rollout and mixed-version behavior: older clients keep the display
  bug; newer clients interpret the same server responses correctly.

## Lifecycle, Security, and Reliability

- Concurrency, cancellation, retries, cleanup, and resource ownership:
  reconciliation still skips active user-started runs; terminal state is
  captured/applied on the main actor.
- Authentication, authorization, permissions, trust, privacy, and secrets:
  none; no new data access or payload.
- Failure modes, recovery, idempotency, and data repair: repeated hydration is
  idempotent for rows and must not erase authoritative failure/cancellation.
  No durable data repair is needed.

## Product and Integration Surfaces

- Server/runtime: unchanged.
- TUI/web/macOS/other clients: macOS only; TUI/API remain event-driven.
- Provider/model/tool catalog and routing: none; search shows reconciliation is
  downstream of provider/tool execution.
- External systems and automation: callback/cron are affected consumers only.
- UX states, keyboard/focus/accessibility/motion: terminal status and error
  content correctness; no input, layout, accessibility, or motion change.

## Deployment and Operations

- Deployment/migration order and feature flags: native app patch; no flag or
  migration.
- Logs, metrics, traces, alerts, and support diagnostics: deterministic Swift
  regression and engineering-log evidence.
- Rollback triggers and recovery steps: revert if completed replay stops
  deduplicating or transcript rows disappear; no data repair.
- Runbooks and operator docs: none; public operating contract is unchanged.

## Regression Tests

- Characterization and first expected red test: failed terminal replay followed
  by message hydration currently ends with `.completed`.
- New acceptance tests required: failed preserves `.failed` plus error;
  cancelled preserves `.cancelled`.
- Edge, negative, failure, lifecycle, and security tests: successful completed
  replay remains green and active runs remain guarded.
- Integration/e2e/real-path proof: native callback/cron Activity-to-Chat flow
  after both GUI follow-up fixes land.
- Cross-surface regressions to guard: persisted-row deduplication and normal
  completed state.
- Exact targeted and full commands:
  `swift test --package-path macapp --filter failedReplayReconciliationPreservesFailureState`;
  `swift test --package-path macapp`;
  `./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs before code: this plan and impact map.
- Implementation notes/logs/indexes after code: engineering log, plan status,
  and plans index.
- Training/onboarding/release notes: none; bugfix requires no operator action.

## Warning Check

- Every surface is mapped above; `none` entries include searched rationale.
