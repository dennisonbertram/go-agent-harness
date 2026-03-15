# Issue #24 Grooming: Research - Session Tree Branching

## Summary
Research session tree branching from Pi agent: conversations modeled as trees (not linear), allowing forking at any point to explore alternatives, A/B test, or recover from degradation.

## Evaluation
- **Clarity**: Clear — Problem (linear conversations limit exploration), solution (tree structure with fork/navigate), impact (recovery from compaction degradation, A/B testing) well-explained. Related issues identified (#7, #5, #16, #17).
- **Acceptance Criteria**: Present — Four explicit criteria: document tree serialization format, design API surface, analyze interaction with memory/compaction, minimal API.
- **Scope**: Atomic — Research and light design, no implementation yet.
- **Blockers**: Depends on #7 (persistence layer) for implementation, but research can proceed now.
- **Effort**: Medium — Requires Pi codebase study, JSONL format analysis, design of branch semantics, interaction analysis with memory/compaction. 5-8 hours.

## Recommended Labels
research, well-specified, medium, blocked-by-7

## Missing Clarifications
1. "Branches can be merged" — what does merge mean? Cherry-pick turns from one branch into another? Manual reconciliation?
2. For observational memory: should branches share memory or have independent memory per branch?
3. Navigation UX for API clients: just /fork endpoint, or also /switch, /tree, /list-branches?
4. Can branches be deleted? If a branch is deleted, is its memory retained or garbage-collected?
5. Storage overhead concern: "tree vs linear" — should research quantify (e.g., 10% overhead per branch)?

## Notes
- Labeled as "research" in GitHub
- Pi's JSONL format with parentId is well-documented, good reference material
- Integration cascade: #7 (persist) → this (branch storage) → compaction interaction (#17) → memory interaction
- UX dimension: branching only valuable if accessible; headless/API needs good UX design
- Compaction interaction complex: if conversation compacts, can you still branch to pre-compaction history?
- Multi-tenant complexity: tenant_id, conversation_id, agent_id scoping in tree structure
- Risk: JSONL with tree structure may not scale well (seeking to branch point in large histories)

## Related Issues
- #7 (persistence layer — prerequisite for implementation)
- #5 (run continuation — branching is superset)
- #16 (JSONL rollout recorder — aligned data format)
- #17 (conversation compaction — must coordinate)
