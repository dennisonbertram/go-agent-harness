# Issue #1130 Cross-Surface Impact Map

## Task

- Task / issue: #1130, submission-local terminal and failure outcomes.
- Plan link: `2026-08-03-issue-1130-submission-outcomes-plan.md`.
- Owner: native-client delivery lane.
- Status: implementation planned; stacked on #1128 `654b7da`.

## Current Ownership, Callers, and Data Flow

- Entry points: `ProjectSession.submit` and composer -> `RunSession.submit` ->
  `RunSubmission`; headless `ToolWalk.Runner.walk` waits and judges that handle.
- Source of truth: `RunSubmission` owns A-local lifecycle/transcript; selected
  conversation state remains `RunSession.currentRunID`/`Transcript`.
- Consumers: run-specific SSE, conversation SSE, reset/load, guarded controls,
  ToolWalk wait/timeout/verdict.
- Search evidence: `rg -n "RunSubmission|activeSubmission|waitForTerminal|markDisplaced" macapp`.
- Conclusion: lifecycle must be local to the handle and selection/displacement
  must remain independent; shared session state cannot judge A.

## Config, API, CLI, and Tools

- Config/defaults/environment: None.
- HTTP endpoint/schema/server wiring: existing run start/events/control endpoints
  only; no request or response change.
- CLI/tools: ToolWalk internal waiting semantics only; no command grammar change.
- Errors: A-local failure is preserved; B-visible error remains untouched.

## Persistence and Compatibility

- Schemas/migrations/caches: None.
- Compatibility: Swift internal types only; the existing public submission
  handle retains `state`, `failure`, and `isDisplaced` accessors.
- Mixed-version: None; a client update is self-contained.

## Lifecycle, Security, and Reliability

- Concurrency/cancellation: response, per-run SSE, and conversation SSE may
  race selection/reset. Identity and run guards make late work harmless.
- Auth/secrets: no changes.
- Failure/recovery: EOF/start error becomes A-local; only the owning A can
  fail shared visible state; reset/load permanently detach unresolved A.

## Product and Integration Surfaces

- Server/runtime and TUI: None; API/TUI retain existing behavior.
- macOS GUI: scheduled B remains visible while A result is retained privately
  for the initiating UI/ToolWalk flow.
- Provider/tool routing/external systems: None.
- UX/accessibility: no new controls; prevents an incorrect B error/timeout.

## Deployment and Operations

- Deployment: normal native/ToolWalk PR rollout, no migration/flag.
- Diagnostics: exact A failure/result stays on `RunSubmission`; no new logs.
- Rollback: revert the PR if ownership evidence regresses.

## Regression Tests

- First expected red: terminal A followed by B reports no terminal wait outcome
  under the old mutually-exclusive `.displaced` state.
- Acceptance: barrier sequencing for terminal/failure/late ACK/reset/load/EOF;
  typed ToolWalk outcome proves only timeout cancels A.
- Negative: zero B endpoint requests and no B transcript/lifecycle/error mutation.
- Integration: focused Swift targets, full `swift test`, format, and
  `./scripts/test-regression.sh`; live GUI/TUI/API acceptance remains #1010.

## Documentation and Handoff

- Before code: this plan/map and #1130 design/TDD comment.
- After code: active plan, four durable logs, both indexes, PR test evidence.
- No public documentation/release note: internal ownership repair only.
