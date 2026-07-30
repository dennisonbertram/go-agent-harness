# Plan: Make the Workflow Failure-Event Test Contention-Tolerant

## Context

- Governing GitHub issue: #1049.
- Problem: `TestEngineDefinitionSubscribeAndFailureEvents` allows only two
  wall-clock seconds for live `workflow.failed` delivery after the stored run
  reaches failed.
- Red evidence: the full repository race gate timed out with
  `workflow.started` and `workflow.step.started` already observed.
- Constraint: preserve the live-event assertion and production ordering.

## Scope

- In scope: a leak-safe, ten-second test deadline for this event assertion.
- Out of scope: workflow runtime ordering, event fanout, storage, production
  timeouts, and unrelated timing tests.

## Documentation Contract

- Feature status: test-only bug repair.
- Public docs affected: none.
- Evidence: engineering and long-term logs, plan, impact map, plans index.

## Test Plan

- Red: retain the exact full-race timeout as regression evidence.
- Green: focused normal and race tests at `-count=100`.
- Package: `internal/workflows` normal and race.
- Full: `./scripts/test-regression.sh` and GitHub required checks.

## Cross-Surface Impact Map

- See `2026-07-30-issue-1049-workflow-failure-timeout-impact-map.md`.

## Implementation Checklist

- [x] Create contract-complete bug #1049.
- [x] Capture the full-race failure and architecture search evidence.
- [x] Write the plan and impact map before code.
- [x] Replace the short one-shot deadline with a stopped ten-second timer.
- [x] Run focused stress, package, and full local gates.
- [ ] Pass hosted required checks.
- [ ] Merge through a closing PR.

## Verification

- Focused normal and race tests passed at `-count=100`.
- The complete `internal/workflows` package passed normal and race tests.
- `./scripts/test-regression.sh` passed normal, race, and the 85.6% coverage
  gate with zero uncovered functions.

## Risks and Mitigations

- Risk: a longer wait could conceal a missing event.
- Mitigation: the test still fails with the complete event history after a
  bounded deadline and still requires `workflow.failed`.
- Risk: `time.After` retains its timer until firing on the fast path.
- Mitigation: use `time.NewTimer` with `defer Stop`.
