# Issue #1098: Deleted-Job Cron Reconciliation Coverage

## Context

- Governing GitHub issue: #1098, `Cover deleted-job cron reconciliation and restore regression gate`.
- Problem: the merged #1004 recovery path has two reachable helpers without
  direct execution coverage: `finishUnavailableExecution` and
  `reconciledScope`. The repository's zero-function coverage gate therefore
  blocks otherwise unrelated promotion work.
- User impact: a recovered active cron execution whose job was soft-deleted
  must become a durable unavailable failure, preserve its `RunID`, and release
  the recovered no-overlap lease only after persistence succeeds. It must not
  attempt to update the deleted job.

## Scope

- In scope: deterministic `internal/cron` reconciliation tests and only a
  production change required by a genuinely red behavioural test.
- Out of scope: callback durability (#1005), retry/idempotency (#1006),
  Keychain isolation (#1096), new cron features, and API/TUI/macOS contract
  changes.

## Documentation Contract

- This plan and the linked impact map are written before test or source edits.
- The engineering, observational, system, long-term-thinking, plan, and log
  indexes record the exact retention and release contract plus red/green
  evidence.

## Test Plan (TDD)

- First red: recover a scoped active execution whose absent job returns both
  `ErrJobNotFound` and `sql.ErrNoRows`; require a failed unavailable terminal
  row, preserved `RunID`, no `TouchJobRun`, release only after durable
  `UpdateExecution`, and a later duplicate admission.
- Failure red: force terminal persistence to fail and require returned error,
  zero job touches, preserved durable/local lease, and duplicate denial.
- Controls: retain existing cancellation, transient lookup, Stop-wins, and
  commit-wins reconciliation regressions unchanged.
- Verification: focused normal and `-race -count=20`; package coverage with
  function report showing no zero entries for both helpers; then unchanged
  foreground `./scripts/test-regression.sh`.

## Implementation Checklist

- [x] Verify #1098 issue contract and refreshed `origin/main` provenance.
- [x] Record plan and cross-surface map before code.
- [x] Add test-only direct coverage regressions after baseline coverage showed
  both helpers at 0.0%.
- [x] Determine source changes are unnecessary: the merged implementation
  already meets the unavailable terminal/release contract.
- [x] Run focused normal/race, coverage, and full regression.
- [x] Update evidence docs/indexes and prepare the reviewable slice without push.

## Rollout and Rollback

- Rollout is a test-only coverage repair if base behaviour meets the stated
  contract; no migration, API, or configuration rollout is needed.
- If a source defect is proven, use the existing terminal lifecycle gate and
  release ordering; rollback is a normal reversion of this isolated commit.

## Evidence

- Baseline red coverage: `go test ./internal/cron -coverprofile=...` followed
  by `go tool cover -func` reported `finishUnavailableExecution 0.0%` and
  `reconciledScope 0.0%` on `224d667a`.
- Rebased green base: `5c4ed8c8` after #1096; targeted normal and
  `-race -count=20` passed. The helper report is 91.7% and 100.0%.
- Full foreground host regression: `./scripts/test-regression.sh` passed
  normal, race, coverage (85.6%), and zero-function gate on the rebased
  candidate.
