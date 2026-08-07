# Issue #1254: Mandatory child `task_complete`

## Context

- Governing issue: #1254.
- Problem: forked children inherited restrictive tool filters but did not activate or retain their required deferred completion control; a validated completion marker also did not terminate the child.
- User impact: a parent can receive a child’s structured result without consuming an extra provider turn or failing with `max_empty_responses`.
- Constraints: preserve root denial, do not change `subagents.Manager`, persistence, or wire schemas.

## Scope

- In scope: activate `task_complete` for every depth-positive fork, exempt that mandatory child control from child restrictive filters, detect only validated successful sentinel output, reject a mixed completion turn before sibling execution, and retain terminal cleanup.
- Out of scope: Manager/API persistence, migrations, root permissions, and changes to ordinary deferred-tool discovery.

## Documentation Contract

- Feature status: implemented.
- Public docs affected: none; this is internal run lifecycle behavior.
- Implementation notes: engineering, observational, and system logs plus their index.

## Test Plan (TDD)

- First red: a restrictive `RunForkedSkill` child receives only `task_complete`, calls it once, returns structured output, and provider call count stays one.
- Additional regressions: root remains denied; failed/malformed completion remains non-terminal; a completion mixed with another call rejects before sibling mutation; activation state is cleaned after every terminal child path.
- Commands: focused harness normal/race tests, daemon/API matrix, then `./scripts/test-regression.sh`.

## Implementation Checklist

- [x] Verify structured issue and current ownership/search evidence.
- [x] Create plan and cross-surface impact map.
- [ ] Add failing deterministic tests.
- [ ] Implement smallest lifecycle/filter/sentinel change.
- [ ] Record red/green evidence and update logs/indexes.
- [ ] Run required regression and real API/TUI/native acceptance.

## Risks and Mitigations

- Risk: broadening normal allowlists weakens child policy. Mitigation: exempt only `task_complete` when `ForkDepth > 0`, preserving root and ordinary filters.
- Risk: malformed tool output falsely terminates a run. Mitigation: accept only successful `task_complete` execution with a validated JSON sentinel.
