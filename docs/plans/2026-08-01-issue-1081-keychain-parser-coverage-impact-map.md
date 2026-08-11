# Cross-Surface Impact Map: Issue #1081 Keychain Parser Coverage

## Task

- Task / issue: #1081, Ubuntu regression gate rejects unexercised Keychain reference parser.
- Plan link: [2026-08-01-issue-1081-keychain-parser-coverage-plan.md](2026-08-01-issue-1081-keychain-parser-coverage-plan.md).
- Owner: GoCode engineering.
- Status: targeted normal/race and authoritative foreground full regression
  pass locally; exact-head hosted verification remains pending.

## Current Ownership, Callers, and Data Flow

- Entry points: `ResolveCredential`, `StoreCredential`, and `DeleteCredential` Keychain branches.
- Owning source: `internal/modelstore/credref.go:keychainParts` splits `<service>/<account>` at the first slash.
- Callers and downstream data: `readKeychain` and Keychain store/delete pass the resulting strings to `security(1)` only after `KeychainAvailable` succeeds.
- Similar coverage searched: `internal/modelstore/keychain_darwin_test.go` covers real macOS CRUD; `store_test.go` covers unavailable Keychain behavior.
- Search evidence: `rg -n -C 4 'keychainParts|KeychainAvailable' internal/modelstore`; hosted run `30672776651` identifies this helper as the sole uncovered function.
- Conclusion: one portable unit test owns the missing platform-independent parser coverage; no source ownership changes.

## Config, API, CLI, and Tools

- User-facing config/defaults/environment/API/CLI/wire/tool formats: None. The existing `<service>/<account>` grammar is asserted, not changed.
- Error states/validation: existing error wording is characterized; no runtime validation changes.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data: None.
- Compatibility/mixed rollout: None; test-only change.

## Lifecycle, Security, and Reliability

- Concurrency/cancellation/retries/cleanup: None; `keychainParts` is pure and synchronous.
- Authentication/authorization/privacy/secrets: no secret value, Keychain item, or `security(1)` subprocess is accessed by the portable test.
- Failure/recovery/idempotency: malformed references keep their existing error contract.

## Product and Integration Surfaces

- Server/runtime: None at runtime.
- TUI/web/macOS clients: None; no client behavior changes.
- Provider/model/tool catalog and routing: None.
- External systems/automation: GitHub Ubuntu coverage workflow should no longer report `keychainParts 0.0%`; Darwin integration stays retained.
- UX/accessibility: None.

## Deployment and Operations

- Deployment/migration/flags: None.
- Diagnostics: hosted full-regression evidence is the operational success signal.
- Rollback: revert the test/docs commit; no state or credential recovery required.
- Runbooks: existing test-regression gate remains authoritative.

## Regression Tests

- First expected red: hosted exact-main Ubuntu run `30672776651` zero-function coverage failure, not a fabricated behavioral failure.
- Acceptance: table-driven portable valid/malformed parser cases.
- Edge/negative/security: retained slashes and missing separator/service/account; no secret or Keychain access.
- Real path: existing Darwin Keychain round trip is retained; hosted Ubuntu full regression validates portability.
- Commands: `go test ./internal/modelstore -count=1`; `go test -race ./internal/modelstore -count=1`; `./scripts/test-regression.sh` with the authoritative local environment.

## Documentation and Handoff

- Specs/public docs: no public change.
- Implementation logs/indexes: plan index, active plan, and engineering/observational/system/long-term logs record cause and evidence.
- Training/release notes: None; no user-visible behavior change.

## Warning Check

Every non-applicable surface is explicitly `None` with rationale above; the slice is intentionally portable test and documentation only.
