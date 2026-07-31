# Plan: Make Provider-Key Matrix Startup Wait Contention-Tolerant

## Context

- Governing GitHub issue: #1062.
- Problem: hosted race run `30583930460` exhausted the provider API-key
  matrix fixture's three-second health wait, then logged the server listening
  roughly one second after the assertion failed.
- User impact: a contended runner can block an unrelated, valid PR even though
  the provider-capture behavior and server startup both work.
- Constraint: preserve the live health check, exact captured-key assertion, and
  bounded graceful shutdown.

## Scope

- In scope: use the established ten-second matrix startup budget in
  `TestMatrix_ProviderAPIKeyCapture`.
- Out of scope: production startup, provider construction, listener allocation,
  other deadlines, and broad cleanup of #958.

## Documentation Contract

- Feature status: test-reliability bug repair.
- Public docs affected: none; production behavior is unchanged.
- Implementation notes: engineering and long-term logs, this plan, its impact
  map, and the plans index.

## Test Plan (TDD)

- Existing red regression artifact: hosted `make test-race` run `30583930460`,
  where `TestMatrix_ProviderAPIKeyCapture` timed out at three seconds before the
  same address began listening.
- Focused green: normal and race test at `-count=100`.
- Adjacent green: matrix tests normal and race.
- Full green: `./scripts/test-regression.sh` in a foreground non-TTY shell so
  macOS Keychain-backed tests retain the login context.

## Cross-Surface Impact Map

- See `2026-07-31-issue-1062-provider-key-matrix-health-wait-impact-map.md`.

## Implementation Checklist

- [x] Create contract-complete bug #1062.
- [x] Capture the exact hosted red log before editing.
- [x] Record current helper/caller search evidence.
- [x] Write this plan and impact map before code.
- [x] Reuse the established ten-second matrix health budget.
- [x] Run focused normal/race stress and adjacent matrix tests.
- [x] Run the full regression gate.
- [x] Push the exact verified head to PR #1063 and pass hosted checks.

## Verification

- Provider-key capture passed normal and race at `-count=100`.
- The complete `TestMatrix_` slice passed normal and race.
- Subscriber-pinned retention passed normal and race at `-count=100`; all
  adjacent pruning tests passed normal and race at `-count=20`.
- The complete harness package passed normal and race.
- `./scripts/test-regression.sh` passed its normal, race, and coverage phases
  with `85.6%` total coverage and zero uncovered functions.
- PR #1063 hosted `test-fast` run `30592818353` and `test-race` run
  `30592818361` both passed on the pushed head.

## Risks and Mitigations

- Risk: a wider deadline could hide a deterministic startup defect.
- Mitigation: the fixture still requires HTTP 200 health, reports the exact
  address/deadline, verifies the captured key, and bounds shutdown separately.
- Risk: broad timeout changes weaken unrelated tests.
- Mitigation: change only the one fixture that failed and keep #958 out of
  scope.
