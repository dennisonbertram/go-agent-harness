# Grooming: Issue #388 — feat(profiles): add efficiency report retrieval and suggest-only refinement surfaces

## Already Addressed?

No — The current system emits a `profile.efficiency_suggestion` SSE event via `maybeEmitProfileEfficiencySuggestion()` in `internal/harness/runner.go`, but this is fire-and-forget. There are no HTTP endpoints for listing or retrieving efficiency reports, no stored report records (as confirmed by the absence of any persistence from #387), and no API surface that returns `EfficiencyReport` values to callers. The `EfficiencyReport` and `ProfileRefinements` types exist in `internal/profiles/profile.go` but are only produced transiently by `BuildEfficiencyReport()` and never surfaced to clients.

## Clarity

Clear — The issue is clear: expose persisted efficiency reports via read-only list/get endpoints; keep the surface suggest-only with no auto-application of refinements. The boundary with #387 (persistence) and #389 (async reviewer loop) is well-drawn.

## Acceptance Criteria

Partial — The issue implies:
- `GET /api/v1/profiles/{name}/efficiency` (or similar) — returns the persisted efficiency reports for a profile.
- `GET /api/v1/profiles/{name}/efficiency/{report_id}` — returns a single report.
- Responses include `EfficiencyReport` data including `SuggestedRefinements`.
- No write surfaces in this ticket — suggest-only.

Missing explicit criteria:
- Exact HTTP route paths and response shapes.
- Pagination / limit semantics for list endpoint.
- What `EfficiencyReport` JSON shape clients receive (full struct? summarized?).
- Whether the endpoint is authenticated/scoped per tenant.
- Test expectations (unit, integration, HTTP).

## Scope

Atomic — Assuming #387 lands first, this is a thin HTTP handler + server wiring task. It reads from the profile history store and serializes `EfficiencyReport` values. No new computation or mutation is needed.

## Blockers

Hard blocker: #387 (persist per-profile usage history). Without the persistence layer, there is nothing to retrieve.

Also depends on the HTTP router and server pattern being stable, which is established by the existing server in `internal/server/`.

Blockers: #387 (must land first).

## Recommended Labels

well-specified, small, blocked

## Effort

Small — Given #387 is complete, this is primarily a thin HTTP handler that calls `ListProfileRuns(profileName)`, builds `EfficiencyReport` values, and serializes them. The server pattern (handler registration, JSON encoding, error handling) is well-established in `internal/server/`. The main work is route design and test coverage.

## Recommendation

well-specified — The issue is clearly scoped and has no ambiguity beyond exact route names (which are a detail implementers can choose). Hard-blocked on #387.

## Notes

- `internal/profiles/profile.go`: `EfficiencyReport` and `ProfileRefinements` types are already defined — they just need a store-backed source.
- `internal/profiles/efficiency.go`: `BuildEfficiencyReport(stats RunStats) EfficiencyReport` is the computation function — already exists and tested.
- `internal/harness/runner.go` line 3192-3221: `maybeEmitProfileEfficiencySuggestion` fires an event but does not persist — this ticket assumes #387 adds persistence.
- `internal/server/`: existing HTTP handler pattern (e.g., `http_prompt_test.go`, various handler files) gives a clear template for new read-only endpoints.
- Plan doc confirms: "Expose read-only reports first. Keep this ticket suggest-only."
- Plan doc also confirms this ticket depends on Ticket 13 (#387) and must not auto-apply refinements.
- No existing endpoint for profile efficiency exists — confirmed by absence of any route for `efficiency` in `internal/server/`.
