# Issue #232 Grooming: bug — CompactRun Is a Nondeterministic No-Op

## Summary
execute() reads state.messages once at start (line 913) and holds a local copy across all steps. If CompactRun() is called concurrently, execute() overwrites the compacted messages with its stale local copy on the next setMessages() call, making manual compaction nondeterministically a no-op depending on scheduling.

## Already Addressed?
**No.**
- execute() at line 913 captures messages once, held as local variable across entire step loop
- Multiple setMessages() calls inside loop (lines 924, 1096, 1324, 1344, 1451, 1480, 1530, 1652, 2118) all write the stale local copy
- CompactRun() calls setMessages() at line 2893 — but next execute() iteration overwrites it
- Existing CompactRun tests (runner_context_compact_test.go) do NOT verify concurrent execute() + CompactRun() interaction

## Clarity
**5/5** — Root cause precisely identified with specific line numbers and execution flow.

## Acceptance Criteria
**Explicit** — execute() must treat state.messages as single source of truth and re-read at start of each step.

Required tests:
1. CompactRun succeeds, execute continues, final messages reflect compaction (not stale copy)
2. Concurrent execute + CompactRun under -race flag
3. CompactRun during mid-step yields correct final state

## Scope
**Atomic** — Confined to execute() function and message re-read logic at step start.

## Blockers
None.

## Recommended Labels
bug, correctness, small

## Effort
**Small-to-Medium** — 2–6 hours.
- Refactor execute() to re-read state.messages at each step start under lock
- Risk: must ensure compactMu synchronization is correct
- Per-step message copy overhead acceptable for typical message counts

## Recommendation
**well-specified** — Ready to implement with care. Synchronization around compactMu must be verified. Must run under -race flag.
