# Plan: Keep the Swarm Activation Control Run Live

## Context

- Governing GitHub issue: #1046.
- Problem: `TestAgentSwarmDeniedForMemberRuns` reuses an exhausted scripted
  provider for its unrestricted control. The control run can complete and clear
  its activation between `Activate` and `filteredToolsForRun`.
- Red evidence: hosted fast CI failed at `runner_swarm_test.go:242` with
  `agent_swarm missing from unrestricted run definitions after activation`.
- Constraint: preserve production activation cleanup and both sides of the
  denied/unrestricted contract.

## Scope

- In scope: a dedicated, test-local blocking provider for the control run.
- Out of scope: production Runner lifecycle, activation tracking, agent swarm,
  permissions, or deferred-tool filtering.

## Documentation Contract

- Feature status: test-only bug repair.
- Public docs affected: none.
- Evidence: engineering and long-term logs, plan, impact map, plans index.

## Test Plan

- Red: retain the exact hosted lifecycle-race failure as regression evidence.
- Green: focused normal and race tests at `-count=100`.
- Package: `internal/harness` normal and race.
- Full: `./scripts/test-regression.sh` and GitHub required checks.

## Cross-Surface Impact Map

- See `2026-07-30-issue-1046-swarm-activation-control-impact-map.md`.

## Implementation Checklist

- [x] Create contract-complete bug #1046.
- [x] Capture hosted failure and architecture search evidence.
- [x] Write plan and impact map before code.
- [x] Hold the unrestricted control run live through its assertion.
- [x] Run focused stress, package, and full local gates.
- [ ] Pass hosted required checks.
- [ ] Merge through a closing PR.

## Verification

- Focused normal and race tests passed at `-count=100`.
- The complete `internal/harness` package passed normal and race tests.
- `./scripts/test-regression.sh` passed normal, race, and the 85.6% coverage
  gate with zero uncovered functions.

## Risks and Mitigations

- Risk: the control provider could leak when an assertion fails.
- Mitigation: register release and shutdown cleanup before starting the run.
- Risk: the control could stop proving a real unrestricted run.
- Mitigation: start a separate Runner normally, activate its actual run ID, and
  assert through `filteredToolsForRun`.
