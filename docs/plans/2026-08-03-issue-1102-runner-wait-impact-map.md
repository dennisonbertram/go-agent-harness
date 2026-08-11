# Cross-Surface Impact Map: Issue #1102

## Task

- Task / issue: #1102 deterministic AskUser wait test.
- Plan link: `2026-08-03-issue-1102-runner-wait-plan.md`.
- Owner: harness test suite.
- Status: implemented; post-rebase foreground full regression passed and PR #1104 is open pending hosted review.

## Current Ownership, Callers, and Data Flow

- Entry point: `internal/harness/runner_test.go:TestRunnerAskUserQuestionWaitsAndResumes`.
- Owning behavior: `Runner.Subscribe` snapshots event history atomically with live subscription; `Runner.setStatusAndEmitContext` commits `waiting_for_user` before its event; `InMemoryAskUserQuestionBroker.Pending` only proves broker registration.
- Callers/consumers: the test is the only changed consumer. Runtime API/TUI/macOS clients rely on event/status behavior but are unchanged.
- Search evidence: `rg -n 'TestRunnerAskUserQuestionWaitsAndResumes|WithAskUserQuestionPendingNotifier|EventRunWaitingForUser|Subscribe' internal/harness`.
- Conclusion: event observation, not broker pending-readiness, is the correct test synchronization boundary.

## Config, API, CLI, and Tools

- User-facing config/defaults/endpoints/CLI/tools: None; no production code changes.
- Error states/validation: None; existing assertions remain.

## Persistence and Compatibility

- Schemas/migrations/caches: None.
- Compatibility/mixed-version behavior: None; test-only synchronization preserves existing wire/status contract.

## Lifecycle, Security, and Reliability

- Concurrency: the test waits for `run.waiting_for_user` through the runner’s history-plus-live subscription instead of sleeping after broker registration.
- Cancellation/retries/cleanup: unsubscribe via `t.Cleanup`; no new goroutines or resources.
- Security/privacy: None; the existing question payload remains inside the test process.

## Product and Integration Surfaces

- Server/runtime: runner lifecycle is characterized, not changed.
- TUI/web/macOS: None; they consume the unchanged event/status pair.
- Provider/model/tool routing: stub provider and existing AskUserQuestion tool only.
- UX: event/status visibility remains asserted, preserving client intent.

## Deployment and Operations

- Deployment/flags: None.
- Observability: the regression now observes the same lifecycle event client integrations use.
- Rollback: revert test-only change if it proves incompatible with existing subscription semantics.

## Regression Tests

- First red: hosted #1101 `-race` failure observed `running` after pending availability; local old test is repeated under `-race -count=20` before changing it.
- Acceptance: event-boundary test immediately sees `waiting_for_user`, submits input, and retains full ordered lifecycle.
- Edge/failure: run history/live handoff is already covered by `Subscribe`; this test retains status and completion assertions.
- Commands: `go test ./internal/harness -run '^TestRunnerAskUserQuestionWaitsAndResumes$' -count=1`; same with `-race -count=20`; package normal/race; `./scripts/test-regression.sh`.

## Documentation and Handoff

- Public/spec docs: None; behavior is unchanged.
- Implementation notes: engineering/observational/system logs and plans index; PR closes #1102 with exact red/green evidence.
