# Issue #22 Grooming: Research - TTSR (Time Traveling Streamed Rules)

## Summary
Research TTSR from oh-my-pi: pattern-triggered context injection that keeps rules at zero token cost until matching pattern appears, then injects for that turn only. Solves the problem of irrelevant rules consuming context in long sessions.

## Evaluation
- **Clarity**: Clear — Problem (irrelevant rules cost tokens), solution (pattern-triggered injection), and impact (valuable for long sessions) are well-explained. Research questions are specific and grounded.
- **Acceptance Criteria**: Present — Four explicit criteria: document algorithm, estimate savings, design proposal for integration, identify interactions with existing systems.
- **Scope**: Atomic — Pure research into alien pattern, can proceed independently.
- **Blockers**: None — Informational, can research in parallel with other work.
- **Effort**: Small-Medium — Primarily code reading + design analysis. Estimating token savings requires modeling which could be more involved (4-8 hours).

## Recommended Labels
research, well-specified, small

## Missing Clarifications
1. What constitutes a valid "trigger pattern"? Regex only, or keyword matching, semantic matching?
2. For "per-line scanning" vs "per-message" vs "tool output" granularity — should research recommend one, or leave open?
3. Interaction with Conversations compaction (#17) — should research provide initial thoughts, or full design?
4. Should scopes (fire-once, per-turn, persistent) be tested or just researched?

## Notes
- Labeled as "research" in GitHub
- Related issues identified: #17 (compaction), #4 (deferred tools), existing YAML prompt catalog
- Integration point clear: Talents layer in prompt composition pipeline
- Token savings estimation is key value metric — research should quantify
- Architectural fit good: pattern matching is straightforward, doesn't require new infrastructure
- Risk: If pattern matching is expensive (slow regexes on large context), benefit may be minimal on small sessions
