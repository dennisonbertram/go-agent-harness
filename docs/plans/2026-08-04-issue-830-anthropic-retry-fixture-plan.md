# Plan: Issue #830 Anthropic Retry Fixture Budget

## Context

- Governing GitHub issue: #830
- Problem: `TestClientRetriesOn503` can exhaust the test-local 100ms retry
  budget only when full-suite coverage instrumentation creates scheduling load.
- User impact: an unrelated full regression can be red despite correct retry
  handling, obscuring real failures.
- Constraints: strict red-green TDD; change only the Anthropic test fixture;
  retain the production retry policy and the 429/503 request-count contract.

## Scope

- In scope: `internal/provider/anthropic/client_test.go`, this plan and impact
  map, engineering log, and their indexes.
- Out of scope: production retry implementation/defaults, OpenAI/shared retry
  fixtures, API/config/CLI/persistence, and mock-server behavior.

## Documentation Contract

- Feature status: `implemented`
- Public docs affected: None; this is test-only behavior.
- Spec docs to update before code: this plan and the impact map.
- Implementation notes to add after code: engineering log entry and indexes.

## Test Plan (TDD)

- Characterization/red: run unchanged 429/503 tests normally and under race
  stress, recording that the existing 100ms budget is the only timing limit.
- Existing tests: preserve `TestClientRetriesOn429` and
  `TestClientRetriesOn503`, including their successful completion and two
  request assertions.
- Regression: focused normal/race stress, full Anthropic normal/race, and
  `./scripts/test-regression.sh`.

## Cross-Surface Impact Map

- See `2026-08-04-issue-830-anthropic-retry-fixture-impact-map.md`.

## Implementation Checklist

- [x] Define acceptance criteria in issue #830.
- [x] Record ownership/caller search evidence.
- [x] Complete the cross-surface impact map before code.
- [x] Capture red/characterization test evidence before the fixture change.
- [x] Make the minimal test-only fixture change.
- [x] Preserve 429/503 assertions and run required verification.
- [x] Update logs and indexes.

## Risks and Mitigations

- Risk: raising the test-only budget could mask a genuine retry failure.
- Mitigation: retain bounded three-attempt retry behavior, disabled jitter,
  both 429/503 acceptance assertions, and full regression verification.
