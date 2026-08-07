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

- Feature status: in implementation (#1255 corrective trusted-origin follow-up).
- Public docs affected: none; this is internal run lifecycle behavior.
- Implementation notes: engineering, observational, and system logs plus their index.

## Test Plan (TDD)

- First red: a restrictive `RunForkedSkill` child receives only `task_complete`, calls it once, returns structured output, and provider call count stays one.
- Additional regressions: root remains denied; failed/malformed completion remains non-terminal; a completion mixed with another call rejects before sibling mutation; activation state is cleaned after every terminal child path.
- Commands: focused harness normal/race tests, daemon/API matrix, then `./scripts/test-regression.sh`.

## Implementation Checklist

- [x] Verify structured issue and current ownership/search evidence.
- [x] Create plan and cross-surface impact map.
- [x] Add failing deterministic tests.
- [x] Implement smallest lifecycle/filter/sentinel change.
- [x] Record red/green evidence and update logs/indexes.
- [ ] Run required regression and real API/TUI/native acceptance.

## Risks and Mitigations

- Risk: broadening normal allowlists weakens child policy. Mitigation: exempt only `task_complete` when `ForkDepth > 0`, preserving root and ordinary filters.
- Risk: malformed tool output falsely terminates a run. Mitigation: accept only successful `task_complete` execution with a validated JSON sentinel.
- #1255 correction: public `WithForkDepth` is forgeable. The step engine now
  installs a private Runner identity plus live parent-run capability; only that
  capability derives child depth and grants mandatory completion. Mandatory
  completion overrides child allow/profile/skill filters, while explicit
  `DeniedTools`, pre-tool hooks, and permission rules retain precedence.
- #1255 lifetime/policy correction: a captured `context.WithoutCancel` tool
  context loses authority after the private origin parent reaches a terminal
  status. The same private origin, rather than public `RunMetadata`, is the
  only source of inherited system prompt, permissions, and profile policy.
