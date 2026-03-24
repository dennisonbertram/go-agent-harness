# Grooming: Issue #393 — test(profiles): add a profile-backed subagent smoke and integration suite

## Already Addressed?

No — The existing smoke test (`scripts/smoke-test.sh`) tests the basic run lifecycle (healthz, providers, models, create run, poll to completion, stream events) but has no profile-backed subagent path at all.

Evidence from `scripts/smoke-test.sh`:
- Steps 1-8 cover: prerequisites, server start, healthz, providers, models, create run, poll completion, stream events
- No steps for: profile discovery, child subagent creation, structured completion validation, persistence readback
- The `--profile full` flag is passed to harnessd (line 88) but profiles are not inspected or validated as part of the test

Evidence from `docs/runbooks/golden-path-deployment.md`:
- Documents the existing `full` profile smoke test
- No mention of profile-backed subagent orchestration as a tested path
- Step 4 in the smoke test section is "Verifies `GET /v1/providers`" — no profile validation

Evidence from `internal/subagents/`:
- `manager.go` and `manager_test.go` have unit-level tests for subagent lifecycle, but these are isolated unit tests, not end-to-end smoke tests
- No integration test that exercises the full path: server start → profile discovery → child run creation → structured completion → readback

## Clarity

Clear — The acceptance scenario is well-specified in both the issue body and the plan document:
- Start server
- Inspect profiles (requires #377)
- Create child run using a profile
- Observe structured completion (requires #383 for structured result contract)
- Read persistence back

The issue is clear about what the golden path must demonstrate. The constraint "one narrow golden path" is appropriate and prevents scope creep.

## Acceptance Criteria

Partial — The issue body states: "Validate profile discovery, child start, structured completion, and persistence readback." The plan document adds: "Start server, inspect profiles, create child run, observe structured completion, read persistence back." These together form good acceptance criteria, but they need to be formalized as numbered smoke test steps:

Suggested formal criteria:
1. Server starts with a profile that includes subagent capability
2. `GET /v1/profiles` (or equivalent) returns at least one profile
3. `POST /v1/runs` with `profile` field creates a child run using that profile
4. Run reaches `completed` status within timeout
5. Completion response includes structured result (not just raw output string)
6. Completed run is readable from the conversation/persistence store

## Scope

Atomic — One golden path smoke test. The scope is well-bounded. The issue explicitly says "one supported smoke/integration path" which prevents it from expanding into a full regression suite.

## Blockers

Blocked on multiple upstream tickets:

- **#377** (list/get profile HTTP surfaces) — required for step 2 (profile discovery validation)
- **#382** (async subagent lifecycle HTTP surfaces) — required for step 3 (child run creation via subagent control surface)
- **#383** (structured completion contract) — required for step 5 (structured completion validation)
- **#376** (fail closed on unknown profiles) — recommended baseline before testing known-profile behavior

Without these four tickets, the smoke test can only validate that `harnessd --profile full` starts and serves basic runs. The profile-backed subagent path cannot be tested until the server endpoints exist.

This is the second-to-last ticket in the plan's delivery order (position 18 of 20), correctly placed after the implementation tickets.

## Recommended Labels

blocked, medium

## Effort

Medium — Estimated 2-4 days (once unblocked):
- Extend `scripts/smoke-test.sh` with 3-4 new steps (profile list, child run, structured completion, readback)
- Update `docs/runbooks/golden-path-deployment.md` to document the new smoke path
- The test infrastructure (bash + curl + python3) is already established in the existing smoke script
- The main complexity is defining what "structured completion" looks like in the HTTP response (dependent on #383)

## Recommendation

blocked — This ticket is well-specified and scoped correctly, but it is the validation layer for 4 upstream implementation tickets that are not yet done. The grooming is sound; execution should not begin until #376, #377, #382, and #383 are all merged. Recommend marking as `blocked` and revisiting after those tickets land.

## Notes

- `scripts/smoke-test.sh` is a solid foundation — it already handles server start, healthz polling, port randomization, and cleanup via trap. Extending it with profile steps is straightforward.
- The existing smoke test uses `--profile full` but does not validate profile properties — it is only used as a server startup argument.
- `docs/runbooks/golden-path-deployment.md` will need a new section for the profile-backed path (currently only documents the basic run lifecycle).
- The plan document notes "Use tmux-backed process management for live smoke scripts" as a potential enhancement — but this is optional. The existing bash + trap approach is sufficient for the first pass.
- `internal/subagents/manager_test.go` exists and covers unit-level lifecycle — this is separate from the integration smoke test and should not be confused with it.
- The "persistence readback" step requires the conversation store to be enabled in the smoke run. The existing smoke test uses `HARNESS_AUTH_DISABLED=true` but does not configure a persistent store. This detail needs to be addressed: either configure a temp SQLite store for the smoke test, or clarify that "persistence readback" means reading the run status from the in-memory store.
