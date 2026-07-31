# Cross-Surface Impact Map: Issue #1076 Workflow Initial Write Exit

## Task

- Task / issue: #1076, initial workflow `start` write masks child exit stderr.
- Plan: `2026-07-31-issue-1076-workflow-initial-write-exit-plan.md`.
- Owner: Codex.
- Status: implemented and fully verified locally; closing PR and hosted checks
  pending parent promotion.

## Current Ownership, Callers, and Data Flow

- Entry point: `SourceManager.runSourceWorkflow` after `cmd.Start` and before
  `serveProtocol`.
- Owning source of truth: `runSourceWorkflow` owns stdin/stdout pipes, bounded
  stderr capture, process-group cleanup, close, wait, and the terminal signals
  passed to `resolveSourceWorkflowOutcome`.
- Callers and consumers: source bundles register a `Script` with `Engine`; the
  resulting error is stored on failed workflow runs and displayed unchanged by
  API, CLI, TUI, web, and macOS consumers.
- Similar abstractions searched: `rg -n "bounded stderr|child stderr|broken
  pipe|RunWorkflow|Stderr|sourceWorkflowOutcome|cmd.Wait|StdinPipe"
  internal/workflow docs/plans docs/logs`.
- Duplication conclusion: extend the existing outcome record and resolver; do
  not add another arbiter or client-specific error handling.

## Config, API, CLI, and Tools

- User-facing config/defaults/env/files: none after the source and issue search;
  the existing workflow timeout remains the lifecycle bound.
- Endpoints, requests, responses, CLI commands, tools, and wire formats: no
  schema or routing change.
- Error-state change: a non-zero child exit plus bounded stderr outranks the
  initial transport-write symptom. A standalone initial-write error remains
  visible.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, or ownership changes: none.
- Compatibility: successful results, missing-result errors, timeouts, semantic
  protocol failures, later stdin-close errors, and stderr bounds remain.
- Mixed-version behavior: none; arbitration is local to one workflow process.

## Lifecycle, Security, and Reliability

- Concurrency/lifecycle: child exit races the first parent write. Every started
  child must follow one bounded cleanup path and be waited exactly once.
- Cancellation/retries/cleanup: initial-write and protocol errors both retain
  process-group termination; retries are not added.
- Authentication, authorization, permissions, secrets, and privacy: none after
  data-flow search. Child stderr remains capped at `maxWorkflowStderrBytes`.
- Failure/recovery: deadline and semantic protocol remain first. A natural
  non-zero process exit, including exit 7, retains bounded stderr and beats the
  initial write. When the initial write itself requests process-group SIGKILL,
  that cleanup wait signal does not mask the earlier write error. Later close,
  missing result, and success contracts remain ordered afterward.

## Product and Integration Surfaces

- Server/runtime: corrected source-workflow failure provenance and child reaping.
- TUI/web/macOS/other clients: no code change; they receive the corrected stored
  error through existing APIs.
- Provider/model/tool catalog and routing: none after repository search.
- External systems and automation: hosted race checks become stable for this
  scheduling path.
- UX, keyboard, focus, accessibility, and motion: none; no client presentation
  contract changes.

## Deployment and Operations

- Migration/order/flags: separate baseline PR lands before #1070 is rebased;
  no flag or data migration.
- Observability: preserve child exit status and bounded stderr instead of raw
  EPIPE-only diagnostics.
- Rollback: revert if timeout/protocol precedence changes, any child is not
  reaped exactly once, successful workflows regress, or stderr becomes unbounded.
- Attribution boundary: SIGKILL observed after this path successfully requests
  the same signal is cleanup evidence; distinguishing an indistinguishable
  concurrent SIGKILL would require broader pre-reap process supervision outside
  #1076. Natural exit statuses are not ambiguous and remain primary.
- Runbooks/operator docs: no operator procedure changes.

## Regression Tests

- First expected red: a real child writes stderr and exits 7 before the initial
  parent write is released; pre-fix code returns raw EPIPE and skips wait.
- New acceptance controls: initial-write plus natural wait-error precedence,
  standalone initial-write visibility, and live-child cleanup SIGKILL causality.
- Edge/failure/lifecycle/security: deadline, semantic protocol, close-only,
  missing result, success, exact reaping, process-group cleanup, and bounded
  stderr.
- Real-path proof: focused integration normal/race x100, complete workflow
  normal/race, `make test-race`, unchanged full regression, then hosted checks.
- Exact targeted command: `go test ./internal/workflow -run
  '^(TestSourceManagerRunWorkflowInitialStartWriteReapsChildExit|TestResolveSourceWorkflowOutcomePrecedence)$'
  -count=1` and its race/stress variants.

## Documentation and Handoff

- Specs/public docs before code: this plan/map; no public docs because no new
  user-facing capability is introduced.
- Implementation logs/indexes after code: engineering, observational, system,
  long-term, plans index, and active-plan tracker.
- Training/onboarding/release notes: none; parent handoff records exact commit,
  commands, results, and the required #1070 rebase ordering.

## Warning Check

- Every cross-surface heading is resolved. Unaffected surfaces include the
  repository search and data-flow rationale rather than blank `None` claims.
