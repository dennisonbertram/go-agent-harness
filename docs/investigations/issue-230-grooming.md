# Issue #230 Grooming: reliability — Recorder Can Drop Non-Terminal Events After recorderMu Fix

## Summary
The `recorderMu` fix (commit 1bc5d36) prevents panics but not event drops. A goroutine capturing `rec` before the terminal event can lose the race to write, causing non-terminal events to silently drop from JSONL forensic output despite being in memory.

## Already Addressed?
**No (partially).**
- Commit 1bc5d36 added `recorderMu` lock to prevent crashes (2026-03-12)
- `runner.go:3560–3580` — `emit()` locks recorderMu, checks `!recorderClosed`, calls `rec.Record()`, closes if terminal
- **Race window still exists**: goroutine can pass the `recorderClosed == false` check, be preempted, terminal event fires and closes recorder, then preempted goroutine resumes and `Record()` drops silently
- No tests validate event drop scenarios under concurrent terminal+non-terminal emission

## Clarity
**4/5** — Root cause (TOCTOU race on `recorderClosed` check) clearly stated. Proposed fix (per-run recorder goroutine with channel) is concrete. Missing: acceptance criteria for ordering guarantees and completeness verification.

## Acceptance Criteria
**Missing explicit criteria.** Implicit requirements:
- All non-terminal events written before `Close()`
- No events dropped even under preemption/concurrent terminal emission
- Definition of "complete" JSONL needed (all events in state.events present in file)
- Test scenario specification needed (concurrent emitters + terminal event)

## Scope
**Medium** — Requires refactoring `emit()`, possible new channel-based recorder goroutine, integration with run lifecycle cleanup. Not fully atomic — interacts with terminal sealing, redaction pipeline, CompletionResult handling.

## Blockers
None.

## Recommended Labels
`bug`, `reliability`, `medium`, `needs-clarification`

## Effort
**Medium** — ~100–200 lines code + ~200–300 lines tests.
- Refactor recorder lifecycle in `emit()` + terminal handling
- Implement per-run goroutine + channel (new pattern)
- Concurrent emission tests with race detector
- Potential performance impact: needs benchmarking

## Recommendation
**needs-clarification** — Must add explicit acceptance criteria for event completeness, test plan for concurrent scenarios, and evaluation of channel-based goroutine lifecycle (drain + close on shutdown).
