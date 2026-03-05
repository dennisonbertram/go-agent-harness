# Plan: Pre/Post Message Hooks and Baseline Tools

## Context

- Problem: The harness needs extensibility points around model messages and a stronger baseline toolset for coding workflows.
- User impact: Without hooks, policy/guardrail behavior is hard to inject cleanly; without baseline tools, the harness lacks practical repo-inspection and patch primitives.
- Constraints:
  - Implement strictly test-first.
  - Preserve existing event-driven service behavior.
  - Keep tools workspace-scoped and deterministic where possible.

## Scope

- In scope:
  - Add pre-message and post-message hook pipeline to runner.
  - Emit hook lifecycle events (`hook.started`, `hook.completed`, `hook.failed`).
  - Add baseline tools in order:
    1. `ls`, `glob`
    2. `grep`
    3. `apply_patch`
    4. `git_status`, `git_diff`
  - Keep existing `read`, `write`, `edit`, `bash` tools.
  - Add/expand tests for hooks and new tool behaviors.
  - Run full regression suite and live OpenAI task with local key.
- Out of scope:
  - Full sandboxing framework for arbitrary command execution.
  - Durable hook/tool audit storage beyond run event history.

## Test Plan (TDD)

- New failing tests to add first:
  - Hook pipeline mutation and blocking behavior.
  - Hook event emission and ordering.
  - `ls` and `glob` outputs and workspace boundary handling.
  - `grep` match reporting for line/path results.
  - `apply_patch` replacement behavior and missing-target failure.
  - `git_status` and `git_diff` behavior in a local initialized git repo.
- Existing tests to update:
  - Default tool contract regression test expected names.
  - Existing tools tests to include newly added tool assertions.
- Regression tests required:
  - Hook failure-mode behavior (`fail_open` vs `fail_closed`) for errors.
  - Tool boundary/path traversal rejections for all file-path arguments.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Write failing tests first.
- [x] Implement minimal code changes.
- [x] Refactor while tests remain green.
- [x] Update docs and indexes.
- [x] Update engineering/system/observational logs as needed.
- [x] Run full test suite.
- [x] Run live OpenAI end-to-end verification.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: Hook logic could break normal run loop progression.
- Mitigation: Add hook-order and blocking/mutation tests with explicit event assertions.

- Risk: `git_*` tools may fail unpredictably outside repos.
- Mitigation: Test in isolated temporary git repo and return explicit tool errors.
