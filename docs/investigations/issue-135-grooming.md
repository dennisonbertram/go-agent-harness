# Issue #135 Grooming: observationalmemory: enhance reflection to detect and surface supersessions and contradictions between observations

## Summary
Enhance the reflection system to detect when new observations contradict or supersede earlier ones, and surface that in `Snippet()` output.

## Already Addressed?
**NOT ADDRESSED** — `ActiveReflection` is stored as a plain string. No structured reflection output, no supersession/contradiction detection or parsing logic.

## Clarity Assessment
Clear motivation. The "how" (LLM-based detection vs. heuristic) needs more specificity in acceptance criteria.

## Acceptance Criteria
- Reflection step detects contradictions/supersessions between observation chunks
- Superseded chunks marked or deprioritized in `Snippet()` output
- Contradictions surfaced explicitly in the snippet context
- Tests for contradiction detection

## Scope
Medium — touches reflection pipeline and Snippet() selection.

## Blockers
Logically after #134 (importance scoring), though not strictly required.

## Effort
**Medium** (5-8h) — LLM prompt for detection + structured reflection parsing + Snippet() updates + tests.

## Label Recommendations
Current: none. Recommended: `enhancement`, `medium`

## Recommendation
**well-specified** — Ready to implement, though consider implementing #134 first.
