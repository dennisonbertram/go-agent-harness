# Issue #134 Grooming: observationalmemory: add importance scoring to observation chunks and use it in Snippet() selection

## Summary
Add an importance score to each `ObservationChunk` and use it to weight `Snippet()` selection beyond simple newest-first ordering.

## Already Addressed?
**NOT ADDRESSED** — `ObservationChunk` struct has no `Importance` field. `Snippet()` uses newest-first selection only. No importance scoring algorithm exists.

## Clarity Assessment
Clear problem and approach.

## Acceptance Criteria
- `importance float64` field added to `ObservationChunk` struct
- Schema migration adds `importance` column
- LLM-based or heuristic scoring assigns importance at chunk creation time
- `Snippet()` uses importance-weighted selection
- Tests cover importance weighting

## Scope
Atomic.

## Blockers
None.

## Effort
**Medium** (4-6h) — Schema migration + scoring logic + updated Snippet() + tests.

## Label Recommendations
Current: none. Recommended: `enhancement`, `medium`

## Recommendation
**well-specified** — Ready to implement.
