# Issue #235 Grooming: feat(recursion) — Recursive Agent Spawning with DB-Backed Suspension, Result Pointers, and JSONL Grep

## Summary
Full recursive agent spawning system: depth counter, DB-backed suspension/resume, result pointers stored in DB, JSONL grep for subagent results, and orchestrator pressure management.

## Already Addressed?
**No.**
- Binary ContextKeyForkedSkill flag prevents all nesting (explicit check: "nested skill forking is not supported")
- No run_results table, JSONL extraction, or result pointer infrastructure
- RunForkedSkill() interface exists but unimplemented in runner
- Very comprehensive spec (31 acceptance criteria) but too large for a single PR

## Clarity
**3/5** — Detailed spec but lacks migration/staging strategy and depends on #234 without formally marking it.

## Acceptance Criteria
**Explicit but numerous** — 31 acceptance criteria across 8 sections. Comprehensive but should be split across phased issues.

## Scope
**Needs splitting** — Recommend 4 phases:
1. Phase 1: Depth counter + suspension infrastructure (DB schema, basic suspend/resume)
2. Phase 2: Result pointer storage + JSONL grep tooling
3. Phase 3: Orchestrator backpressure management
4. Phase 4: Oversight hooks and kill switches

## Blockers
- **#234** (per-run tool filtering) — prerequisite for subagent control; should be formally marked

## Recommended Labels
enhancement, needs-clarification, large, blocked

## Effort
**Large** — 10–15 person-days across all phases.

## Recommendation
**needs-clarification** — Break into separate phased issues. Implement #234 first, then Phase 1 of this issue. As-written scope is too large for a single PR.
