# Cross-Surface Impact Map: Waiting-for-user pending-input ordering

## Task

- Task / issue: #1054
- Plan link: `2026-07-30-issue-1054-waiting-pending-order-plan.md`
- Owner: Codex
- Status: Implemented; promotion pending

## Current Ownership, Callers, and Data Flow

- Entry points: Runner tool-call lifecycle and `AskUserQuestionTool`.
- Owning packages/types/functions and source of truth:
  `runner_step_engine.go` owns run status/events; in-memory and checkpoint
  brokers own pending question state.
- Callers, consumers, events, and downstream data: HTTP/TUI/macOS clients read
  run status/events and then call `PendingInput`.
- Similar abstractions searched: Approval brokers, `PendingInput`,
  `EventRunWaitingForUser`, AskUserQuestion broker implementations.
- Search commands/evidence:
  `rg -n "RunStatusWaitingForUser|PendingInput|AskUserQuestionBroker" internal/harness`.
- Duplication/ownership conclusion: Registration stays broker-owned; status and
  event publication stays runner-owned, joined by a typed post-registration
  callback.

## Config, API, CLI, and Tools

- User-facing config added or changed: None.
- Defaults / fallbacks: None.
- Environment variables, config files, or saved settings touched: None.
- Endpoints, request fields, response fields, or server wiring affected:
  Existing pending-input endpoint becomes immediately consistent with visible
  wait state; wire shape unchanged.
- CLI commands, tools, wire formats, or integrations affected:
  AskUserQuestion internal request gains an optional readiness callback.
- Error states / validation changes: Removes transient `ErrNoPendingInput`
  after visible wait state.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, or ownership changes: None.
- Backward/forward compatibility and versioning: Internal Go API addition;
  existing question/checkpoint records unchanged.
- Partial rollout and mixed-version behavior: Single binary; no mixed-version
  protocol.

## Lifecycle, Security, and Reliability

- Concurrency, cancellation, retries, cleanup, and resource ownership:
  Notification starts exactly once after successful registration in a
  deadline-bound goroutine. The broker does not consume a buffered answer until
  notification finishes, which preserves wait-before-resume ordering, while
  cancellation and timeout remain independent so a stalled notifier cannot
  hang `Ask`.
- Authentication, authorization, permissions, trust, privacy, and secrets:
  No new data exposure; callback receives the already-public pending shape.
- Failure modes, recovery, idempotency, and data repair: Registration failure
  does not publish wait state; timeout/cancel continue through existing paths.
  Checkpoint expiry is conditional on unresolved state, and stale run-status
  writes repair themselves to the newest in-memory status before returning.
  Accepted in-memory answers win deadline cleanup, and callers receive an
  explicit already-resolved error when a checkpoint transition loses a race.
  AskUser notification and ordinary wait deadlines share the same atomic
  pending-only expiry and accepted-resume recovery path. Harness input and
  approval operations translate a lost resolution to their established
  no-pending result; generic checkpoint HTTP resume exposes the distinct
  durable conflict as `409 already_resolved` while preserving terminal data.

## Product and Integration Surfaces

- Server/runtime: Strengthened lifecycle invariant.
- TUI/web/macOS/other clients: Wait event/status can be rendered without a
  retry gap before the question is readable.
- Provider/model/tool catalog and routing: AskUserQuestion core tool forwards
  the readiness hook; no catalog changes.
- External systems and automation: Hosted race suite becomes deterministic.
- UX states, keyboard/focus/accessibility/motion: Eliminates transient empty
  input UI; no visual design change.

## Deployment and Operations

- Deployment/migration order and feature flags: None.
- Logs, metrics, traces, alerts, and support diagnostics: Event payload/order
  retained.
- Rollback triggers and recovery steps: Revert if exact-head lifecycle tests or
  manual question flow regress.
- Runbooks and operator docs: None.

## Regression Tests

- Characterization and first expected red test: Gated broker proves visible wait
  state precedes broker registration on current code.
- New acceptance tests required: Both brokers expose `Pending` inside readiness
  callback; tool forwards callback; runner invariant.
- Edge, negative, failure, lifecycle, and security tests: Registration error,
  timeout, cancellation, denied tool, and exactly-once notification.
- Integration/e2e/real-path proof: Exact-head hosted normal/race checks and
  final native GUI conversation test.
- Cross-surface regressions to guard: Existing event order and status
  restoration suites.
- Exact targeted and full commands:
  `go test ./internal/harness ./internal/harness/tools/core -run 'AskUser|WaitForUser' -count=100`;
  same with `-race`; `./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs before code: Plan and impact map.
- Implementation notes/logs/indexes after code: Plan index, active plan,
  engineering log, long-term-thinking log.
- Training/onboarding/release notes: None; existing public contract is tightened.

## Warning Check

- Every relevant surface is mapped or explicitly unaffected.
