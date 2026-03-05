# Plan: Modular Crush-Informed Tooling Implementation

## Context

- Problem: Tool logic was concentrated in a single large file and difficult to extend safely.
- User impact: Adding or evolving tools required editing a monolith, raising regression risk and slowing iteration.
- Constraints:
  - Strict TDD and regression gate enforcement.
  - Keep `NewDefaultRegistry(...)` compatibility.
  - Preserve service/event API behavior.

## Scope

- In scope:
  - Migrate tool implementations to `internal/harness/tools/` with catalog registration.
  - Add crush-informed tool surface incrementally with dependency-gated registration.
  - Add approval modes (`full_auto`, `permissions`) and policy seam.
  - Add/expand tests to keep zero-function coverage and coverage gate passing.
  - Live OpenAI tmux smoke validation.
- Out of scope:
  - UI-level permission prompts.
  - Full production hardening of optional external integrations.

## Test Plan (TDD)

- New failing tests added first:
  - Modular catalog shape and deterministic tool ordering.
  - Tool behavior coverage for each added tool family.
  - Policy-mode allow/deny/error behavior.
  - Env parser for approval mode.
- Existing tests updated:
  - `internal/harness/tools_contract_test.go` expected tool surface.
- Regression tests required:
  - Full `./scripts/test-regression.sh` pass with coverage gate.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Write failing tests first.
- [x] Implement modular code changes.
- [x] Refactor while tests remain green.
- [x] Update docs and indexes.
- [x] Update engineering/system/observational/long-term logs.
- [x] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: Expanded tool surface can fail OpenAI schema validation at runtime.
- Mitigation: Ensure array schema fields include explicit `items` and verify with live run.
- Risk: Larger tool package can drop aggregate coverage under enforced gate.
- Mitigation: Add targeted branch/behavior tests per tool and keep no-zero-function policy.
