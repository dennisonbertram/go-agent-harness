# Issue #137 Grooming: Add /models slash command to demo-cli REPL

## Summary
Add a `/models` command to the demo-cli REPL that lists available models with their providers and costs. Requires a `GET /v1/models` HTTP endpoint on the server.

## Already Addressed?
**PARTIALLY ADDRESSED** — Commit 94b8842 added `/model` (singular, sets current model) and `/help` commands. However:
- No `/models` (plural, list all available) command exists
- No `GET /v1/models` HTTP endpoint found in `internal/server/http.go`
- `internal/harness/tools/list_models.go` exists (agent-side tool) but no user-facing HTTP endpoint

## Clarity Assessment
Clear. Depends on `/v1/models` endpoint (see issue #142).

## Acceptance Criteria
- `GET /v1/models` HTTP endpoint on server
- `/models` REPL command calls the endpoint and displays formatted list
- Shows provider, model ID, cost per token
- Optional `--provider` filter flag

## Scope
Atomic per component (HTTP endpoint + REPL command).

## Blockers
Logically depends on issue #142 (GET /v1/models endpoint). Can be implemented together.

## Effort
**Small** (2-3h) — HTTP endpoint (see #142) + REPL command handler.

## Label Recommendations
Current: `enhancement`. Good.

## Recommendation
**well-specified** — Implement alongside issue #142. The REPL command is trivial once the endpoint exists.
