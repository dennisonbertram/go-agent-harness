# Cross-Surface Impact Map: Issue #1039 Worktree Containment CI

## Task

- Task / issue: synchronize containment assertions with workspace teardown,
  #1039.
- Plan: `2026-07-30-issue-1039-worktree-containment-ci-plan.md`.
- Owner: Codex.
- Status: implemented and fully verified; merge pending.

## Current Ownership, Callers, and Data Flow

- Entry: `TestWorktreeContainment_ToolCwdIsWorktree`.
- Source of truth: runner provisions the worktree, the real bash tool writes
  cwd-relative files, and terminal cleanup removes the worktree.
- Event consumer: the test subscribes through a buffered channel and examines
  files after receiving `tool.call.completed`.
- Similar abstractions searched: `stubProvider.Complete`, runner subscription
  tests, workspace destruction, and other provider release handshakes.
- Conclusion: the test owns the invalid consumer-timing assumption; production
  routing and cleanup ownership remain unchanged.

## Config, API, CLI, and Tools

- Config/env/defaults: none; test-only channels.
- API/CLI/wire formats: none.
- Tools: retain the real registered bash tool and its existing command.
- Error states: retain explicit tool-error, missing-event, and timeout failures.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data: none.
- Compatibility and mixed versions: none; no runtime change.

## Lifecycle, Security, and Reliability

- Concurrency/lifecycle: add a bounded test-only handshake before cleanup.
- Authentication/authorization/privacy/secrets: none.
- Recovery/idempotency: deferred release prevents deadlock after a failed
  assertion; timeouts diagnose either side failing to progress.

## Product and Integration Surfaces

- Server/runtime: production unchanged.
- TUI/web/macOS/providers/models: none.
- External automation: GitHub Actions fast and race gates become deterministic.
- UX/accessibility: none.

## Deployment and Operations

- Deployment/migration/flags: none.
- Diagnostics: focused failure names the missing handshake or containment
  invariant.
- Rollback: revert the test-only commit if it masks a deliberately broken cwd.

## Regression Tests

- Characterization/red: allow cleanup to win before event consumption and
  observe missing files.
- Acceptance: release terminal completion only after containment assertions.
- Negative controls: daemon-cwd leak, wrong `pwd`, tool error, missing
  provision/completion event, or never-released provider still fail.
- Commands: focused normal/race `-count=100`, harness package tests, and
  `./scripts/test-regression.sh`.

## Documentation and Handoff

- No public docs.
- Update engineering log, long-term log, plan, impact map, and plans index.

## Warning Check

- All required surfaces are covered; runtime surfaces are explicitly unchanged
  because the patch is confined to the regression fixture.
