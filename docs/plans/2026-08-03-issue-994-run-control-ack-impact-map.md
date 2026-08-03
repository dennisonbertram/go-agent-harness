# Issue #994 — Cross-Surface Impact Map

## Task

- Task / issue: #994 acknowledged macOS run controls.
- Plan: `2026-08-03-issue-994-run-control-ack-plan.md`.
- Owner: Codex isolated worktree.
- Status: in implementation.

## Current ownership, callers, and data flow

- Entry points: `ApprovalBar`, `PlanApprovalView`, `AskUserView`, and `Composer` call `RunSession.approve/deny/answer/steer`; the active-run interrupt calls `RunSession.cancel`.
- Source of truth: `RunSession` owns current run identity, request guards, `pendingQuestions`, draft, and `connectionError`; `HarnessClient` supplies acknowledgement-bearing HTTP calls.
- Consumers: SwiftUI binds control enabled state and pending prompt rendering. Search evidence: `rg` over `macapp/Sources` and `macapp/Tests` for control names, question state, and `try?`.
- Conclusion: retain one `RunSession` ownership boundary; do not duplicate HTTP retry/state logic in views.

## Config, API, CLI, and tools

- Config/defaults/environment: None; controls use existing authenticated `HarnessClient`.
- API/wire: Existing cancel/approve/deny/steer/input endpoints only; request/response schemas unchanged.
- CLI/TUI/tools: None, searched current control call sites; no harness tool catalog impact.
- Errors: HTTP `HarnessError` and transport errors surface inline through the existing visible session error state; controls remain retryable.

## Persistence and compatibility

- Schemas/migrations/caches: None.
- Compatibility: additive client-only state; old server responses retain current decoding.
- Partial rollout: mixed client versions are safe because endpoint calls do not change.

## Lifecycle, security, and reliability

- Async ownership: generation/run/prompt checks prevent stale completions from clearing a newer in-flight state or prompt. A request remains single-flight until acknowledgement/failure.
- Security/privacy: no new authority or secret handling. Server failures are shown as returned messages, not serialized/persisted by this slice.
- Recovery: failures reset the matching guard, preserve the entered steering/answer draft, and permit explicit retry. Cooperative cancel escalates only after acknowledgement.

## Product and integration surfaces

- Server/runtime: no server change; focused stubs prove client endpoint interaction.
- macOS: action controls disable in-flight; answer sends only trimmed nonempty values; failure stays visible via `connectionError` and accessibility announcement.
- TUI/web/provider/external automation: None; source search found no shared Swift control abstraction.

## Deployment and operations

- Deployment: normal app release; no flag or migration.
- Diagnostics: error remains visible at the active UI control/session; live daemon control smoke is required before promotion if executable environment permits.
- Rollback: revert client PR; no data repair.

## Regression tests

- First red: copied focused #994 stub suite must fail on `main` because `try?` swallows errors, questions clear early, controls duplicate, and empty answers pass dictionary-count validation.
- Acceptance: success, transport/server failure, retry, duplicate suppression, stale completion ownership, answer preservation, empty answer rejection, and control reachability.
- Exact commands: focused `swift test --filter RunSessionControlAcknowledgements`; full `swift test`; `swift build`; formatting/lint scripts; `./scripts/test-regression.sh`.

## Documentation and handoff

- Internal plan/impact, engineering/observational/system log entries and indexes are updated in this slice.
- No public feature documentation because behavior is not accepted until code/test evidence lands.
