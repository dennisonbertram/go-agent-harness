# Plan: Issue #1140 matrix listener identity

## Context

- Governing GitHub issue: #1140.
- Problem: parallel matrix tests release a selected port before harnessd binds it, then can query a sibling daemon at that recycled address.
- User impact: CI can validate the wrong skill registry and report a false product failure.
- Constraints: preserve production `net.Listen` behavior; do not serialize tests or add arbitrary waits.

## Scope

- In scope: optional listener dependency, actual listener-address handoff in `runMatrixTest`, deterministic readiness/failure wait, and matrix fixture migration to `127.0.0.1:0`.
- Out of scope: skill loading, HTTP API behavior, persistence, production configuration, and test-suite serialization.

## Documentation Contract

- Feature status: `in implementation`.
- Public docs affected: None; this is an internal harness/test-fixture correction.
- Spec docs to update before code: this plan and impact map.
- Implementation notes to add after code: engineering/observational/system logs and indexes.

## Test Plan (TDD)

- New failing test first: force `runMatrixTest` to start on `127.0.0.1:0`, require the check callback to receive the listener's actual address, and use a custom global skill endpoint assertion.
- Existing tests to update: all `TestMatrix_` cases stop reserving a free address and pass `127.0.0.1:0` through `baseEnv`.
- Regression tests required: focused custom-skill and listener identity normal/race stress, full matrix normal/race, `cmd/harnessd` normal/race, and `./scripts/test-regression.sh`.

## Cross-Surface Impact Map

See `2026-08-03-issue-1140-matrix-listener-impact-map.md`.

## Implementation Checklist

- [x] Verify structured issue and current source evidence.
- [x] Record plan and complete impact map.
- [ ] Add and capture deterministic red listener-identity regression.
- [ ] Add defaulted listener injection and actual-address handoff.
- [ ] Remove free-port reservation from matrix fixtures.
- [ ] Run focused, package, and full regression gates.
- [ ] Update logs/indexes and PR evidence.

## Risks and Mitigations

- Risk: injected listener changes startup ownership or callback-recovery ordering.
- Mitigation: preserve the existing single listener acquisition point and default it to `net.Listen`; tests assert the server’s returned listener is the one probed.
