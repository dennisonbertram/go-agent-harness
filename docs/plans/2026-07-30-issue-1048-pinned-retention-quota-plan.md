# Plan: Exclude Subscriber-Pinned Runs from the Drainable Retention Quota

## Context

- Governing GitHub issue: #1048.
- Problem: persisted terminal runs with active subscribers count toward
  `MaxCompletedRetention` even though pruning cannot delete them. Under pressure
  a just-completed run can be deleted before its caller subscribes.
- Red evidence: hosted race CI failed
  `TestRunner_PruneKeepsCompletedRunWithActiveSubscriber` because an extra run
  was already missing at subscription time.
- Constraint: preserve pinned-run protection, persistent records, and bounded
  retention after subscribers drain.

## Scope

- In scope: count only eligible zero-subscriber terminal candidates when
  calculating pruning pressure.
- Out of scope: conversation retention, store deletion, subscription delivery,
  terminal persistence, and configuration shape/defaults.

## Documentation Contract

- Feature status: harness reliability bug repair.
- Public docs affected: none; the change aligns behavior with the existing
  `RunnerConfig` comment.
- Evidence: engineering and long-term logs, plan, impact map, plans index.

## Test Plan

- Red: retain the exact hosted run-not-found failure as regression evidence.
- Green: focused pruning test normal/race at `-count=100`.
- Adjacent/package: all pruning tests and `internal/harness` normal/race.
- Full: `./scripts/test-regression.sh` and GitHub required checks.

## Cross-Surface Impact Map

- See `2026-07-30-issue-1048-pinned-retention-quota-impact-map.md`.

## Implementation Checklist

- [x] Create contract-complete bug #1048.
- [x] Capture hosted failure and ownership/search evidence.
- [x] Write plan and impact map before code.
- [x] Calculate the cap from drainable terminal candidates.
- [x] Run focused stress, adjacent, package, and full local gates.
- [ ] Pass hosted required checks.
- [ ] Merge through a closing PR.

## Verification

- The focused regression passed normal/race at `-count=100`.
- All adjacent pruning tests passed normal/race at `-count=20`.
- The complete `internal/harness` package passed normal and race.
- The isolated coverage phase and a subsequent complete
  `./scripts/test-regression.sh` passed with 85.6% coverage and zero uncovered
  functions.

## Risks and Mitigations

- Risk: pinned runs can temporarily exceed the numeric cap.
- Mitigation: this is the documented exception; cancellation re-runs pruning.
- Risk: the change could retain too many unpinned runs.
- Mitigation: assert the zero-subscriber candidate set stays at or below the
  configured limit after every pruning trigger.
