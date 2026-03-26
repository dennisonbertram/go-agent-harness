# Plan: Issue #384 Parent Context Handoff Bundles

## Context

- Problem: delegated child runs currently rely on ad hoc prompt assembly, so parent context is not carried in a typed, inspectable, size-bounded way across `run_agent`, `spawn_agent`, and forked skills.
- User impact: subagent delegation is harder to debug and replay, and large parent context can either disappear or bloat child prompts unpredictably.
- Constraints:
  - Strict TDD with failing tests first.
  - Keep the change scoped to delegation handoff and storage surfaces.
  - Preserve behavior when no parent context is available.

## Scope

- In scope:
  - Add a typed parent-context handoff contract in the tools layer.
  - Bound and serialize parent transcript metadata/messages deterministically.
  - Render the handoff before child task prompts for `run_agent`, `spawn_agent`, and fork-context skills.
  - Propagate/store the handoff through subagent manager and runner child-run requests.
  - Add regression coverage for truncation markers, prompt ordering, and omitted-large-context behavior.
- Out of scope:
  - Redesigning child-result payloads.
  - Broad profile/runtime isolation refactors.
  - Carrying the full unbounded parent transcript into child prompts.

## Test Plan (TDD)

- New failing tests to add first:
  - `internal/harness/tools/types_context_test.go` for handoff serialization, truncation, and prompt rendering order.
  - `internal/harness/tools/deferred/run_agent_test.go` and `internal/harness/tools/deferred/spawn_agent_test.go` for forwarding/rendering the handoff.
  - `internal/harness/tools/core/skill_test.go` for forked skill prompt/handoff propagation.
  - `internal/subagents/system_prompt_test.go` and `internal/harness/runner_profile_propagation_test.go` for manager/runner propagation and storage.
- Existing tests to update:
  - delegation-path tests that currently assume plain task-only prompts.
- Regression tests required:
  - truncation markers on oversized messages
  - message ordering after byte-count bounding
  - omission of oversized older messages
  - child prompt ordering: handoff block before task boundary/body

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- This issue does not touch those surfaces directly.

## Implementation Checklist

- [ ] Define acceptance criteria in tests.
- [ ] For provider/model flow work, add or update the one-page impact map before implementation.
- [x] Write failing tests first.
- [ ] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries.
- [ ] Implement minimal code changes.
- [ ] Refactor while tests remain green.
- [ ] Update docs and indexes.
- [ ] Update engineering/system/observational logs as needed.
- [ ] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: prompt rendering or runner propagation may diverge between `run_agent`, `spawn_agent`, and skill forks.
- Mitigation: pin the shared ordering/propagation contract with tests across all three call sites plus the runner/subagent seams.
