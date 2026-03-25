# Plan: Issue #427 HTTP Feature Decomposition

## Context

- Problem: `internal/server/http.go` still contains the main run, conversation, and catalog/provider transport logic even though many other server features have already been split into dedicated `http_*.go` files.
- User impact: server transport changes are harder to review and safer route evolution is slower because one file still concentrates too many unrelated handlers.
- Constraints: preserve existing HTTP contracts exactly; keep the change transport-only; follow strict TDD and repo doc/log requirements.

## Scope

- In scope:
  - Extract run-route registration and run handlers out of `internal/server/http.go`.
  - Extract conversation-route registration and conversation handlers out of `internal/server/http.go`.
  - Extract model/provider/summarize registration and handlers out of `internal/server/http.go`.
  - Add regression coverage that pins the new route-group seams.
- Out of scope:
  - API contract changes.
  - Runner or store behavior changes.
  - Refactoring unrelated feature files that are already decomposed.

## Test Plan (TDD)

- New failing tests to add first:
  - A route-group registration test that compile-fails until explicit run/conversation/catalog registration helpers exist and verifies those helpers register the expected key endpoints.
- Existing tests to update:
  - None expected unless extraction reveals a missing shared helper import or setup detail.
- Regression tests required:
  - `go test ./internal/server`
  - `./scripts/test-regression.sh`

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- This task does not change provider/model behavior; it only moves existing HTTP transport code. No dedicated impact map is required.

## Implementation Checklist

- [ ] Define acceptance criteria in tests.
- [ ] For provider/model flow work, add or update the one-page impact map before implementation.
- [ ] Write failing tests first.
- [ ] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries.
- [ ] Implement minimal code changes.
- [ ] Refactor while tests remain green.
- [ ] Update docs and indexes.
- [ ] Update engineering/system/observational logs as needed.
- [ ] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: path dispatch for nested run/conversation routes regresses during file movement.
- Mitigation: keep `buildMux()` registration explicit and add seam-level route registration coverage before moving handlers.
