# Grooming: Issue #386 — feat(profiles): add deterministic profile recommendation and auto-routing

## Already Addressed?

No — No profile recommendation or auto-routing logic exists in the codebase. The `run_agent` tool in `internal/harness/tools/deferred/run_agent.go` defaults to the `"full"` profile when the caller does not specify one (hardcoded string), but there is no recommendation layer, no heuristics, no scoring against task metadata, and no routing table. `internal/profiles/` contains only `profile.go`, `loader.go`, `efficiency.go`, and builtins — no recommender. `internal/systemprompt/` has matcher/validation but no profile selection logic.

## Clarity

Clear — The issue is directionally clear: add a deterministic heuristics/rules layer that selects the best profile for a task, fall back to `"full"`, avoid model-based routing in this ticket. The referenced files (`internal/profiles/*`, `internal/systemprompt/*`, `internal/harness/tools_default.go`) are slightly stale — `tools_default.go` does not exist under that name in `internal/harness/` — but the intent is understood.

## Acceptance Criteria

Partial — The issue implies:
- Given a task description/prompt, return a recommended profile name.
- Logic is deterministic (rules/heuristics, not LLM-based).
- Falls back to `"full"` if no rule matches.

Missing explicit criteria:
- Input shape (raw task string? structured metadata?).
- How recommendations are surfaced (a new function? a new HTTP endpoint? injected into `run_agent`?).
- Whether callers can override the recommendation.
- Test-coverage expectations.

## Scope

Atomic — This ticket is reasonably scoped: implement one recommender function (or small package), wire it into the `run_agent` default-profile path, cover with tests. The plan doc notes it depends on tickets 3, 5, and 7 (profile CRUD, context handoff, lifecycle).

## Blockers

Per the plan doc this is listed after #384 in delivery order and depends on profile CRUD and lifecycle tickets. Core recommendation logic can be written independently against the existing profile loader, but wiring into the HTTP/run surfaces requires earlier tickets to be stable.

Blockers: #377 (profile CRUD/HTTP), #379 (context handoff contract), #383 (structured child result). These precede #386 in the plan's delivery order.

## Recommended Labels

well-specified, medium, blocked

## Effort

Medium — The heuristics function itself is small (pattern matching against task string/tags against profile descriptions). Wiring it into `run_agent` and testing multiple recommendation paths is moderate work. The main uncertainty is what input signals are available (just the task string, or structured metadata from an upstream context handoff?).

## Recommendation

well-specified — The issue is directionally clear and scoped to deterministic heuristics. It should be held until the profile CRUD and context-handoff tickets (#377, #379, #383) land so the recommender has a stable profile list and caller context to operate on.

## Notes

- `internal/harness/tools/deferred/run_agent.go` line 67-68: `profileName = "full"` is the current hard-coded fallback — the recommender would replace this decision point.
- `internal/profiles/loader.go`: `ListProfiles()` gives all available profile names — the recommender can iterate these.
- `internal/profiles/profile.go`: `ProfileMeta.Description` is available per profile — usable as a signal for rule matching.
- `internal/profiles/builtins/`: six profiles with distinct names/descriptions (`github`, `file-writer`, `researcher`, `bash-runner`, `reviewer`, `full`).
- Referenced file `internal/harness/tools_default.go` does not exist; the relevant file appears to be the tool registry wiring in `internal/server/` or `internal/harness/registry.go`.
- The plan doc explicitly calls for deterministic/explainable routing first, model-based later.
