# Plan: Issue #427 HTTP Feature Decomposition

## Context

- Problem: `internal/server/http.go` still owns route registration plus large run/conversation transport handlers, which makes the server layer harder to extend and review.
- User impact: Server changes are riskier than they should be because unrelated run and conversation transport logic still shares one large file.
- Constraints:
  - Preserve all existing HTTP paths, status codes, auth behavior, and payload shapes.
  - Keep the change focused on transport decomposition only.
  - Use the existing server test suite as the primary safety net unless a missing behavior seam appears.

## Scope

- In scope:
  - Extract the run transport slice out of `internal/server/http.go`.
  - Extract the conversation transport slice out of `internal/server/http.go`.
  - Keep route registration readable while preserving the current handler wiring.
- Out of scope:
  - API contract changes.
  - Runner or store behavior changes.
  - Bootstrap or provider-model work.

## Test Plan (TDD)

- New failing tests to add first:
  - None planned initially. `./internal/server` already has direct route and behavior coverage for the run and conversation surfaces being extracted.
- Existing tests to update:
  - None expected unless a moved helper needs a test import/path adjustment.
- Regression tests required:
  - `go test ./internal/server -count=1`
  - `./scripts/test-regression.sh`

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- This task does not touch provider/model flows, so no impact map is required.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [ ] For provider/model flow work, add or update the one-page impact map before implementation.
- [x] Write failing tests first.
- [ ] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries.
- [ ] Implement minimal code changes.
- [ ] Refactor while tests remain green.
- [ ] Update docs and indexes.
- [ ] Update engineering/system/observational logs as needed.
- [ ] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: Route precedence or method/scope checks could change accidentally during extraction.
- Mitigation: Keep `buildMux` wiring unchanged and rely on the existing `internal/server` integration tests before and after the move.
