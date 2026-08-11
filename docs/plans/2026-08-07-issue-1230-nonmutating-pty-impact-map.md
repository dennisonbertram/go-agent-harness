# Cross-Surface Impact Map: Issue #1230 non-mutating TUI PTY evidence

## Task

- Task / issue: #1230, child of #1088.
- Plan link: `2026-08-07-issue-1230-nonmutating-pty-plan.md`.
- Owner: `internal/acceptance/ptyrunner`.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `RunFreshConversation`, `Run`, and `freshFrameCollector` in
  `internal/acceptance/ptyrunner`; slash dispatch in `cmd/harnesscli/tui/model.go`.
- Source of truth: the runner owns fake daemon/process and artifact collection;
  the actual `harnesscli -tui` process owns key handling and rendered states.
- Search evidence: `rg -n 'execute(Help|Cost|Stats|Config|Context|Doctor|Permissions|Search|Resume)Command|RunFreshConversation|freshFrameCollector' cmd/harnesscli/tui internal/acceptance/ptyrunner`.
- Conclusion: extend the direct-owned runner; do not introduce a pipe/script
  harness, reducer-only substitute, or bypass of public HTTP/SSE probes.

## Config, API, CLI, and Tools

- User-facing configuration: None.
- Fixture configuration: 30x100 direct PTY, fake deterministic turns, isolated
  environment/databases under `ArtifactRoot`.
- API/CLI: existing public `harnesscli -tui` plus read-only run/event/
  conversation probes; no endpoint or wire changes.
- Tools: None; cron/callback explicitly excluded.

## Persistence and Compatibility

- Schemas/migrations: None; only disposable SQLite rows are read.
- Compatibility: product command registry and behavior are unmodified.
- Rollout: test infrastructure only; revert-only rollback.

## Lifecycle, Security, and Reliability

- Lifecycle: one master collector; each action must seal an append-only frame
  before the next input; bounded waits and owner-only PTY/daemon termination.
- Security: fake text only, private mode-0700 artifact root, no credentials,
  no HOME/source-worktree writes.
- Failure behavior: return a typed local error and retain artifacts; no retry or
  product patch in this issue.

## Product and Integration Surfaces

- Server/runtime: exercised through isolated HTTP/SSE/store, unchanged.
- TUI: visible overlays, unknown-command feedback, search/Escape focus, and
  same-conversation resume/continue exactly-once continuation rendering.
- Native GUI: None; owned by #1089.
- Provider/catalog: fake provider only, no routing/catalog modification.
- Automation: None; cron/callback owned by their dedicated slices.

## Deployment and Operations

- Deployment: None.
- Diagnostics: terminal, per-action screen/frame, keys, daemon log, SSE, and
  API/store probes are retained in the caller-owned bundle.
- Runbook: update live-testing/artifact guidance after verified implementation.

## Regression Tests

- First red: a batch test references the absent causal scenario and requires
  every named action/frame.
- New tests: frame order, expected visible strings, Escape restoration,
  resume/continue identity and exactly-once events, cleanup.
- Exact commands: focused normal/race `go test ./internal/acceptance/ptyrunner`,
  real batch test, and repository `./scripts/test-regression.sh` with isolated caches.

## Documentation and Handoff

- Before code: this impact map and plan.
- After code: plan checklist, live-testing runbook, logs and their indexes.
- Release notes: None.

## Warning Check

Every affected surface is explicit; unaffected product/client surfaces are
owned by separate issues rather than treated as proof here.
