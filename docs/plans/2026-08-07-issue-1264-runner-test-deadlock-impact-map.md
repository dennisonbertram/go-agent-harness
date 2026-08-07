# Cross-Surface Impact Map: Issue #1264 Runner test lock-snapshot repair

## Task

- Task / issue: #1264 test-only recursive `Runner.mu.RLock` deadlock.
- Plan link: `2026-08-07-issue-1264-runner-test-deadlock-plan.md`.
- Owner: Codex.
- Status: implementation complete; verification in progress.

## Current Ownership, Callers, and Data Flow

- Entry points: `TestRunForkedSkill_CapturedContextCannotMintCompletionAfterParentTerminal`
  and `TestRunForkedSkill_UntrustedMetadataCannotInheritParentPolicy`.
- Owning packages/types/functions and source of truth: test helper
  `forkedRunID` and test-owned `Runner.runs` observations in
  `internal/harness/runner_task_complete_test.go`; production ownership stays
  `Runner.mu`.
- Callers, consumers, events, and downstream data: the tests invoke
  `RunForkedSkill`; its terminal path may queue the production pruning writer.
  No product consumer receives the test snapshot.
- Similar abstractions searched: `forkedRunID`, all `RLock` call sites in the
  file, and adjacent direct child snapshots.
- Search commands/evidence: `rg -n "forkedRunID|RLock\\(" internal/harness/runner_task_complete_test.go` found the helper and recursive sites at 222/256.
- Duplication/ownership conclusion: one test-local immutable snapshot helper
  owns the lock; callers must not lock around it.

## Config, API, CLI, and Tools

- User-facing config added or changed: None; test-only.
- Defaults / fallbacks: None.
- Environment variables, config files, or saved settings touched: None.
- Endpoints, request fields, response fields, or server wiring affected: None.
- CLI commands, tools, wire formats, or integrations affected: None.
- Error states / validation changes: None.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, or ownership changes: None.
- Backward/forward compatibility and versioning: None; no shipped contract.
- Partial rollout and mixed-version behavior: None; test code ships atomically.

## Lifecycle, Security, and Reliability

- Concurrency, cancellation, retries, cleanup, and resource ownership: test
  reads no longer recursively acquire `Runner.mu` after a writer is queued;
  only scalar assertion fields are copied under one lock, with no mutable
  `PermissionConfig` retained after unlock.
- Authentication, authorization, permissions, trust, privacy, and secrets:
  tests continue asserting forged context metadata cannot inherit privileged
  policy; no authorization behavior changes.
- Failure modes, recovery, idempotency, and data repair: removes a CI-hanging
  test failure; no runtime recovery or data repair.

## Product and Integration Surfaces

- Server/runtime: None; production Runner source excluded.
- TUI/web/macOS/other clients: None; no client source or behavior.
- Provider/model/tool catalog and routing: None; fake provider test setup only.
- External systems and automation: GitHub race CI no longer deadlocks.
- UX states, keyboard/focus/accessibility/motion: None.

## Deployment and Operations

- Deployment/migration order and feature flags: ordinary test PR; no deploy or flags.
- Logs, metrics, traces, alerts, and support diagnostics: bounded focused race
  logs and GitHub race CI are retained as diagnosis evidence.
- Rollback triggers and recovery steps: revert the isolated test/docs commit
  if assertions weaken; no state migration.
- Runbooks and operator docs: testing, worktree, and issue-triage rules are
  followed; no operator runbook semantics change.

## Regression Tests

- Characterization and first expected red test: bounded focused `-race` run of
  the affected untrusted-policy test; issue #1264 records its GitHub timeout
  stack and this local branch records environment outcome.
- New acceptance tests required: existing security-policy child assertions
  remain, now sourced from immutable single-lock snapshots.
- Edge, negative, failure, lifecycle, and security tests: trusted/forged
  policy cases, repeated normal/race runs, and full package race execution.
- Integration/e2e/real-path proof: None applicable; test-only lock ownership.
- Cross-surface regressions to guard: #1263 race CI must rerun after merge;
  no timeout waiver.
- Exact targeted and full commands: `go test ./internal/harness -run
  'TestRunForkedSkill_(CapturedContextCannotMintCompletionAfterParentTerminal|UntrustedMetadataCannotInheritParentPolicy)$' -count=50`, same with `-race`,
  `go test -race ./internal/harness/...`, and `./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs before code: no public spec; plan and impact map completed.
- Implementation notes/logs/indexes after code: engineering log, active plan,
  plans index, and logs index.
- Training/onboarding/release notes: concise test-only handoff in closing PR;
  no release note.

## Warning Check

- Every surface is explicitly mapped; unaffected surfaces are marked None with
  a test-only rationale.
