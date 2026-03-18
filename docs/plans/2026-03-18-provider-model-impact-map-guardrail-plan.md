# Plan: Provider/Model Impact Map Guardrail

## Context

- Problem: Recent provider/model feature work has repeatedly needed follow-up fixes in adjacent surfaces such as config, server wiring, TUI navigation, and regression coverage because the initial implementation scope was mapped too narrowly.
- User impact: Features can appear complete at the core behavior layer while still breaking or drifting across routing, API, UI state, and regression coverage.
- Constraints:
  - Keep the process lightweight and one-page.
  - Fit the new requirement into the existing planning/worktree flow instead of inventing a parallel workflow.
  - Avoid hard CI enforcement for now; this pass is process-guided documentation.

## Scope

- In scope:
  - Add a reusable impact-map template for provider/model flow work.
  - Add an operational runbook for when and how to use the impact map.
  - Update bootstrap, planning, and worktree docs so the requirement is visible before implementation starts.
  - Update indexes and logs for the new process artifact.
- Out of scope:
  - CI or hook-based enforcement.
  - Retrofitting older feature work with new impact maps.
  - Changing runtime behavior.

## Test Plan (TDD)

- New failing tests to add first: N/A for documentation/process-only work.
- Existing tests to update: N/A.
- Regression tests required:
  - Verification that affected docs cross-reference the new impact-map requirement and template.

## Implementation Checklist

- [x] Define acceptance criteria in docs and policy.
- [x] Add a provider/model impact-map template.
- [x] Add a runbook describing when the impact map is required and how to fill it out.
- [x] Update bootstrap, planning, and worktree docs to require the artifact.
- [x] Update docs and indexes.
- [x] Update engineering/system/observational logs as needed.
- [ ] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: A documentation-only rule may be skipped during fast-moving feature work.
- Mitigation: Surface it in the highest-traffic docs (`AGENTS.md`, plan template, worktree flow) and make blank sections an explicit warning.
