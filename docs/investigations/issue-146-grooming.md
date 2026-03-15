# Issue #146 Grooming: Add user-facing HTTP endpoint for direct subagent spawning

## Summary
Add `POST /v1/agents` endpoint for directly spawning subagents via HTTP, bypassing the need to use the `agent` tool inside a running session.

## Already Addressed?
**NOT ADDRESSED** — No `POST /v1/agents` endpoint. `AgentRunner` and `ForkedAgentRunner` interfaces exist (`internal/harness/tools/types.go`).

## Clarity Assessment
Clear. Single endpoint with well-defined request/response.

## Acceptance Criteria
- `POST /v1/agents` spawns a subagent with a given prompt and optional skill fork config
- Returns run_id for tracking
- Timeout handling
- ~10 test cases

## Scope
Atomic — single endpoint.

## Blockers
None.

## Effort
**Small-Medium** (2-3 days).

## Label Recommendations
Current: none. Recommended: `enhancement`, `medium`

## Recommendation
**well-specified** — Ready to implement.
