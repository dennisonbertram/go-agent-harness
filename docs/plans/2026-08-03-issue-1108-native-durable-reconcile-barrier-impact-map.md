# Issue #1108 — Native durable reconciliation barrier impact map

## Task

- Task / issue: #1108, native callback replay test asserts durable reply before reconciliation.
- Plan link: `2026-08-03-issue-1108-native-durable-reconcile-barrier-plan.md`.
- Owner: Codex.
- Status: In implementation; test-only fixture synchronization.

## Current Ownership, Callers, and Data Flow

- Entry points: `RunSessionConversationStreamTests.localFailureClearsPriorUsage`,
  `staleTerminalDoesNotReplaceNewerRun`, and
  `staleTerminalPreservesNewerFailureDetail`.
- Owning seam/source of truth: `ConversationStreamStub.Response.waitForGate`
  blocks scripted HTTP delivery; `RunSession` terminal events asynchronously
  invoke durable conversation-message reconciliation.
- Callers/consumers: Swift Testing native unit suite and `live-test.sh`; no
  production caller changes.
- Search evidence: `rg -n -C 42 'localFailureClearsPriorAccounting|staleTerminalDoesNotReplaceNewerRun|staleTerminalPreservesNewerFailureDetail|release_c_durable|waitForGate' macapp/Tests/GoCodeUITests/RunSessionConversationStreamTests.swift`.
- Conclusion: gates must be released by test-observable app state, not URL
  transport completion, to prove reducer/reconciliation ordering.

## Config, API, CLI, and Tools

- User-facing config/defaults/environment: None; test fixture only.
- Endpoints/wire formats: no contract change; fixture continues to script
  existing `/v1/conversations/{id}/messages` responses.
- CLI/TUI/tools/errors: None; API/TUI compatibility is protected by unchanged
  full regression.

## Persistence and Compatibility

- Schemas/migrations/caches: None.
- Compatibility/mixed versions: None; assertions only verify existing durable
  message behavior.

## Lifecycle, Security, and Reliability

- Concurrency: replaces race-prone raw request/transport observations with
  rendered transcript and exact accepted accounting preconditions.
- Security/auth/privacy/secrets: None; fixture contains synthetic text only.
- Failure/recovery/idempotency: preserves the no-SSE failed B -> external C
  ownership path and stale terminal durable replay assertions.

## Product and Integration Surfaces

- Server/runtime: No source change; real native live harness test validates
  the unchanged contract.
- Native GUI: strengthened proof that callback/cron-like continuation is both
  accounted and visibly durable in chat.
- TUI/provider/model/tool catalog/external automation: None; no source or
  wire change, Go regression remains the guard.
- UX/accessibility: no UI change; rendered assistant rows are asserted as the
  user-visible confirmation boundary.

## Deployment and Operations

- Deployment/migration/feature flags: None.
- Logs/metrics/runbooks: engineering and observational logs record fixture
  ordering; no production diagnostics change.
- Rollback: revert the test-only commit if any invariant is weakened.

## Regression Tests

- First expected red: delay C's second `/messages` response with
  `release_c_durable`; old terminal/accounting-only wait reaches final checks
  before the durable assistant row.
- Acceptance: pre-release C ownership/completed/exact usage with absent text;
  post-release rendered C text plus the same exact state; rendered A/B rows
  after both adjacent stale-terminal gates.
- Edge/failure/lifecycle: no-SSE B failure, stale completed A after newer B,
  and stale A after failed B.
- Real-path proof: focused and repeated Swift test, full Swift, native
  `live-test.sh --filter RunSessionLiveTests`, and full Go regression.

## Documentation and Handoff

- Public specs: None; behavior is unchanged.
- Implementation evidence: plan, this impact map, long-term-thinking,
  engineering, observational, and system logs; plans/logs indexes.
- PR handoff: `Closes #1108`, exact commands/results, residual risk that this
  remains fixture-level while live harness validates the actual client path.
