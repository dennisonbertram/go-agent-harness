# Plan: Portable Keychain Reference Parser Coverage

## Context

- Governing GitHub issue: #1081.
- Problem: hosted Ubuntu regression run `30672776651` reaches 85.6% total coverage but fails the zero-function gate because `internal/modelstore/credref.go:keychainParts` is unexercised.
- User impact: `main` cannot be trusted as green while the coverage gate fails, even though no credential runtime defect has been demonstrated.
- Constraints: retain the Darwin real-Keychain integration; introduce no coverage waiver, runtime behavior change, secret access, or Keychain invocation.

## Scope

- In scope: portable table-driven coverage for valid and malformed `<service>/<account>` Keychain reference targets; required plan, impact, log, and index updates.
- Out of scope: Keychain CRUD behavior, credential grammar changes, runtime/provider changes, platform detection, schemas, API/CLI/TUI/macOS changes, and coverage-gate configuration.

## Documentation Contract

- Feature status: implemented after the test and docs land; this is a test-only regression repair.
- Public docs affected: none; there is no user-visible feature change.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: engineering, observational, system, long-term, and active-plan records plus the plans index.

## Test Plan (TDD)

- Strict first red: exact-main hosted Ubuntu `test-regression` run `30672776651` failed after normal/race/coverage collection with `keychainParts 0.0%`. A behavioral red would be untruthful because current parser behavior is correct.
- New coverage: table-driven `TestKeychainPartsValidation` checks ordinary parsing, slash preservation in the account, and three malformed forms returning the established `<service>/<account>` error contract.
- Existing tests retained: Darwin `TestKeychainRoundTripAgainstRealKeychain` remains unchanged and is still the integration proof where `security(1)` is available.
- Verification: targeted normal/race modelstore tests, diff check, outside-sandbox full regression, and hosted Ubuntu regression on the exact PR head.

## Cross-Surface Impact Map

See [2026-08-01-issue-1081-keychain-parser-coverage-impact-map.md](2026-08-01-issue-1081-keychain-parser-coverage-impact-map.md).

## Implementation Checklist

- [x] Verify structured issue #1081, callers, existing Darwin integration, and hosted baseline evidence.
- [x] Record the truthful hosted-only coverage red before adding coverage.
- [x] Add portable validation coverage without changing production code.
- [x] Update required plans, logs, and indexes.
- [x] Run targeted normal/race, diff, and full regression gates.
- [ ] Commit intentional files, push, and open a ready PR with `Closes #1081`.

## Risks and Mitigations

- Risk: portable coverage accidentally replaces real Keychain integration. Mitigation: leave Darwin integration test unchanged and state it as a retained required proof.
- Risk: a test alters reference grammar. Mitigation: assert existing grammar and error substring only; production code is untouched.
