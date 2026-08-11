# Cross-Surface Impact Map: Issue #1132 fixture synchronization

## Task

- Task / issue: #1132 — stabilize compaction-after-waiting user-state proof.
- Plan link: `2026-08-03-issue-1132-compaction-fixture-plan.md`.
- Owner: harness test suite.
- Status: planned test-only repair.

## Current Ownership, Callers, and Data Flow

- Entry point: `TestCompactRunWhileWaitingForUserPreservesCompactionAfterResume`
  in `internal/harness/runner_boundary_test.go`.
- Source of truth: `Runner.Subscribe` supplies event history/live stream and
  `EventRunWaitingForUser` is the public readiness boundary; `PendingInput`
  intentionally becomes available earlier.
- Consumers: this test protects `CompactRun` while an AskUserQuestion pause
  resumes. No production callers change.
- Search evidence: `rg` found the shared `waitForRunEventType` helper and
  neighboring AskUserQuestion test using the same event-first contract.
- Conclusion: reuse that helper; do not duplicate polling or alter ownership.

## Config, API, CLI, and Tools

- Config/defaults/environment/API/CLI/tools: None. The test exercises existing
  Runner APIs only; no wire contract changes.
- Errors/validation: None; original assertions remain.

## Persistence and Compatibility

- Schemas/migrations/caches: None.
- Compatibility/mixed versions: None; no runtime artifact changes.

## Lifecycle, Security, and Reliability

- Concurrency: fixture uses the published event rather than an earlier broker
  registration to establish causal readiness under `-race` scheduling.
- Security/privacy: None; no data, authorization, or secret handling changes.
- Recovery/idempotency: None; original complete/resume assertions remain.

## Product and Integration Surfaces

- Server/runtime: no production change; test validates existing lifecycle.
- TUI/web/macOS: no code change; they retain the existing event contract.
- Provider/model/tool routing: existing AskUserQuestion and `echo_json` fixture
  only; no catalog/routing changes.
- External automation/UX: None.

## Deployment and Operations

- Deployment/migration: None.
- Diagnostics/rollback: test-only PR can be reverted without runtime impact.
- Runbooks: existing regression gate remains authoritative.

## Regression Tests

- Characterization/red: hosted race showed `waiting_for_user` status observed
  as `running` before public event publication.
- Acceptance: focused normal/race `-count=100`; `./internal/harness` normal
  and race; full `./scripts/test-regression.sh`.
- Edge/lifecycle: retain pending call ID, status, compaction result/message
  reduction, event order, resume output, and exact final message/tool deltas.
- Integration/e2e: no new real-path test needed because production behavior is
  unchanged; full repository gate guards integrations.

## Documentation and Handoff

- Specs/public docs: no public update.
- Notes/logs/indexes: plan/index, log entries, and issue/PR evidence.
- Training/release: none; test-only baseline repair.
