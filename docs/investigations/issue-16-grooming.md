# Issue #16 Grooming: JSONL rollout recorder for session replay, fork, and audit

## Summary
Proposal for lightweight JSONL event logging for session replay, fork-from-checkpoint, and audit trails.

## Already Addressed?
**ALREADY RESOLVED** — Fully implemented:
- `internal/rollout/recorder.go` — `Recorder`, `RecorderConfig`, `RecordableEvent` types
- Date-partitioned JSONL storage at `<RolloutDir>/<YYYY-MM-DD>/<run_id>.jsonl`
- Integrated into `internal/harness/runner.go` via `r.config.RolloutDir`
- Mutex-protected for concurrent safety
- Tests in `internal/rollout/recorder_test.go`

Replay tooling, fork-from-checkpoint, privacy redaction, and auto-rotation are NOT implemented but those are future enhancements beyond this issue's scope.

## Clarity Assessment
Clear.

## Acceptance Criteria
Core recorder: done. Replay/fork/redaction: future work.

## Scope
Core feature complete.

## Blockers
None.

## Effort
Done.

## Label Recommendations
Recommended: `already-resolved`

## Recommendation
**already-resolved** — Close this issue. Future enhancements (replay tool, fork, redaction) should be filed as separate issues.
