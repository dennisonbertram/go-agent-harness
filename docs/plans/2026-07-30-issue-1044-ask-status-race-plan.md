# Plan: Synchronize AskUserQuestion Status-Test Run Identity

## Context

- Governing GitHub issue: #1044.
- Problem: `TestDeniedAskUserQuestionDoesNotStrandRunStatus` assigns its
  closure-captured run ID after `StartRun` has already dispatched the provider
  goroutine. The provider can read that string concurrently.
- Red evidence: GitHub Actions `make test-race` reported the write at line 170
  racing the provider read at line 150, followed by an empty status assertion.
- Constraint: preserve the production status contract and the test's mid-run
  observation.

## Scope

- In scope: a test-local, one-shot run-ID handoff.
- Out of scope: production dispatch ordering, provider requests, permissions,
  and status transitions.

## Documentation Contract

- Feature status: test-only bug repair.
- Public docs affected: none.
- Evidence: engineering and long-term logs, this plan, impact map, plans index.

## Test Plan

- Red: retain the exact GitHub Actions race-detector report as the failing
  regression evidence.
- Green: focused normal and race tests at `-count=100`.
- Package: `internal/harness` normal and race.
- Full: `./scripts/test-regression.sh` and GitHub required checks.

## Cross-Surface Impact Map

- See `2026-07-30-issue-1044-ask-status-race-impact-map.md`.

## Implementation Checklist

- [x] Create contract-complete bug #1044.
- [x] Capture the exact CI race report and architecture search evidence.
- [x] Write the plan and impact map before code.
- [x] Add an explicit run-ID publication handoff.
- [x] Run focused stress, package, and full local gates.
- [ ] Pass GitHub required checks.
- [ ] Merge through a closing PR.

## Verification

- Focused normal and race tests passed at `-count=100`.
- The complete `internal/harness` package passed normal and race tests.
- `./scripts/test-regression.sh` passed normal, race, and the 85.6% coverage
  gate with zero uncovered functions.

## Risks and Mitigations

- Risk: synchronization could move the observation after completion.
- Mitigation: the provider still reads status in completion step two, before it
  returns the terminal content.
- Risk: an unbuffered handoff could deadlock if dispatch timing changes.
- Mitigation: use a capacity-one channel and publish immediately after
  `StartRun` returns.
