# Cross-Surface Impact Map: Issue #1133

## Task

- Task / issue: #1133 passive observation of displaced A submission outcomes.
- Plan link: `2026-08-03-issue-1133-passive-outcome-plan.md`.
- Owner: native-client delivery lane.
- Status: implemented locally, stacked on #1131 `63cf9fcd`.

## Current Ownership, Callers, and Data Flow

- Entry points: `ProjectSession.submit`/composer -> `RunSession.submit` ->
  `RunSubmission`; `ToolWalk.Runner` waits, controls, and judges the handle.
- Source of truth: `RunSubmission.lifecycle` is A outcome; `isDisplaced` and
  `RunSession.currentRunID` are authority fences for shared UI/actions.
- Search evidence: `rg -n "RunSubmission|waitForTerminal|cancelTimedOutSubmission|activeSubmission" macapp`.
- Conclusion: submission-local result and shared selected-run authority are
  intentionally separate; wait policy must observe the former while honoring
  the latter.

## Config, API, CLI, and Tools

- Config/defaults/environment: none.
- HTTP/schema/server: uses existing exact `/v1/runs/{A}/cancel`; no schema or
  server change.
- CLI/tools: ToolWalk wait policy only; command grammar/tool catalog unchanged.
- Errors: A EOF/failure remains a truthful ToolWalk result, never B UI error.

## Persistence and Compatibility

- Schemas/migrations/caches: none.
- Compatibility: no public wire change; `RunSubmission` keeps existing state
  projection and sticky displacement semantics.
- Mixed version: client-local behavior; no server compatibility concern.

## Lifecycle, Security, and Reliability

- Concurrency: A SSE, B conversation SSE, start ACK, and timeout race. A is
  passive after displacement until terminal/failure/deadline. #1136 owns the
  immutable B/C authority and revocation contract.
- Authorization/trust: #1133 relies on the existing exact A-only path; #1136
  hardens the capability boundary. No credentials change.
- Failure/recovery: timeout sends A-only transport without touching B's
  selection, pending UI, transcript, or cancel state.

## Product and Integration Surfaces

- Server/runtime and TUI: none.
- macOS/ToolWalk: B stays visually selected; initiating ToolWalk receives the
  actual A terminal/failure verdict.
- Providers/catalog/automation: none; cron/callback B is the motivating normal
  conversation continuation.
- UX/accessibility: no new controls; avoids stale controls acting on B.

## Deployment and Operations

- Deployment/flags: standard native PR; no migration.
- Diagnostics: the retained submission handle is the A-local evidence.
- Rollback: revert PR if an A deadline can affect B; gated tests isolate this.

## Regression Tests

- Red: four initial actual gated URLSession tests failed pre-fix: A terminal/EOF
  and timeout returned `.displaced`; delayed ACK returned before A had an
  identity.
- Acceptance: B-before-terminal, EOF, timeout, and delayed ACK use
  `RunSession.submit()` plus `Runner`, assert B selection and zero B endpoint
  actions. #1136 owns B -> C exact-one A dispatch and revocation coverage.
- Full commands: strict format, focused native/ToolWalk tests, `swift test
  --package-path macapp`, and `./scripts/test-regression.sh`.

## Documentation and Handoff

- Public docs: none.
- Internal docs: plan/map, active plan, four durable logs, and both indexes.
- PR handoff: `Closes #1133`, red/green commands, exact stacked base, review,
  and hosted check evidence.
