# Grooming: Issue #387 — feat(profiles): persist per-profile usage history and efficiency stats

## Already Addressed?

No — The current efficiency implementation is entirely event-based and in-memory. `internal/profiles/efficiency.go` has `BuildEfficiencyReport()` and `ShouldEmitSuggestion()` which compute an in-memory `EfficiencyReport`, and `internal/harness/runner.go` calls `maybeEmitProfileEfficiencySuggestion()` to fire a `profile.efficiency_suggestion` SSE event. Neither function persists anything to the store. `internal/store/sqlite.go` has no `profile_history`, `profile_runs`, or `efficiency_reports` table. `store.Run` has no `profile_name` column. There is no query helper for per-profile rollups anywhere.

## Clarity

Clear — The issue is clear about what to build: a persistence layer for profile runs (status, steps, cost, timestamps, tool usage summary) and query helpers for recent-history and rollups. The design intent (replace one-off suggestion events with queryable history) is documented in the plan doc.

## Acceptance Criteria

Partial — The issue implies these deliverables:
- A new store table (or equivalent) for per-profile run records.
- Fields: profile name, run ID, status, step count, cost USD, timestamps, tool usage summary.
- `ListProfileHistory(profileName, limit)` or equivalent query helper.
- Rollup helper (aggregate stats per profile).
- The runner hooks that write to this table when a profiled run completes.

Missing explicit criteria:
- Schema definition (column names, types, indexes).
- Whether this extends `internal/store/Store` interface or lives in a separate store/package.
- Rollup granularity (last-N runs vs. rolling average vs. time-bucketed).
- Whether tool usage summary is a JSON blob or a normalized table.
- Test expectations (unit + integration coverage).

## Scope

Atomic — This is a self-contained persistence task. It touches the store schema, the runner post-completion hook, and adds query helpers. No user-visible surfaces are changed in this ticket (that is #388's job). Well-bounded.

## Blockers

The plan doc notes dependencies on Ticket 3 (profile CRUD, which includes the store integration for profiles) and Ticket 9 (subagent lifecycle). Specifically, #387 needs:
- `store.Run` or a sibling table that carries `profile_name` (not present today).
- A stable post-run hook in `runner.go` where stats can be captured.

The runner already calls `maybeEmitProfileEfficiencySuggestion` at run completion, which reads `state.profileName` and `state.currentStep` — this is the natural hook point for #387.

Blockers: #375/#376 (store schema and run completion wiring from earlier tickets in the plan).

## Recommended Labels

well-specified, medium, blocked

## Effort

Medium — Schema migration + store interface extension + runner hook + query helpers + tests. The existing `maybeEmitProfileEfficiencySuggestion` hook is a clean insertion point, and the store pattern (SQLite WAL, `CREATE TABLE IF NOT EXISTS`, typed scan helpers) is well-established and easy to extend.

## Recommendation

well-specified — The issue is clear and scoped. The main risk is that `store.Run` does not have a `profile_name` column; adding that (or creating a parallel `profile_runs` table) is well-defined work. Should be prioritized after the profile CRUD and run-lifecycle tickets stabilize the schema.

## Notes

- `internal/harness/runner.go` line 3195-3222: `maybeEmitProfileEfficiencySuggestion` reads `state.profileName` and `state.currentStep` at run-completion — ideal insertion point for persistence.
- `internal/profiles/efficiency.go`: `RunStats` struct already captures `{RunID, ProfileName, Steps, CostUSD, AllowedTools, UsedTools}` — can be reused as the persistence payload.
- `internal/store/sqlite.go`: schema has `runs` table with no `profile_name` column and no `profile_runs` table. Adding one is a straightforward migration.
- `internal/store/store.go`: `Store` interface would need `AppendProfileRun(ctx, stat)` and `ListProfileRuns(ctx, profileName, limit)` methods, or a separate `ProfileStore` sub-interface.
- `profiles.EfficiencyReport.CreatedAt` and `RunStats` are already well-typed — persistence mapping is mechanical.
- Plan doc confirms: "Track runs, status, steps, cost, tool usage, and timestamps by profile. Do not auto-mutate profiles in this ticket."
