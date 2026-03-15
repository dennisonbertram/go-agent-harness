# Issue #143 Grooming: Add user-facing HTTP endpoints for cron job management

## Summary
Expose cron job management (list, create, pause, resume, delete, trigger) via HTTP endpoints so users can schedule recurring agent runs.

## Already Addressed?
**NOT ADDRESSED** — No `/v1/cron/*` endpoints in `internal/server/http.go`. CronClient infrastructure exists but is not HTTP-exposed.

## Clarity Assessment
Excellent — 9 endpoints specified with request/response schemas and error cases.

## Acceptance Criteria
- CRUD + trigger endpoints for cron jobs
- Pause/resume support
- Cron expression validation
- Concurrent safety under `-race`
- ~12 unit tests + 4 regression tests

## Scope
Large — 9 endpoints but all wrapping existing `CronClient` API.

## Blockers
None.

## Effort
**Large** (2-3 days) — 9 endpoints, concurrent safety, comprehensive tests.

## Label Recommendations
Current: none. Recommended: `enhancement`, `large`

## Recommendation
**well-specified** — Ready to implement. All CronClient infrastructure exists.
