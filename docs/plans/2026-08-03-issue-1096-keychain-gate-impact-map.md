# Cross-Surface Impact Map: Issue #1096 Keychain regression gate

## Task

- Task / issue: #1096 — stabilize modelstore Keychain regression gate.
- Plan link: `2026-08-03-issue-1096-keychain-gate-plan.md`.
- Owner: Codex.
- Status: implemented and locally verified; awaiting independent review and parent promotion.

## Current Ownership, Callers, and Data Flow

- Entry points: `ResolveCredential`, `StoreCredential`, and `DeleteCredential` in `internal/modelstore/credref.go`.
- Owning source: `readKeychain` runs `security find-generic-password`; store runs `add-generic-password -U` and sends the secret over stdin; delete runs `delete-generic-password`.
- Callers: `Service.PutProvider`, service cleanup, harnessd settings seeding/resolution, HTTP settings routes. Tests call these functions directly.
- Similar abstractions searched: direct `exec.CommandContext` appears only in these Keychain operations; no reusable injected process seam exists in modelstore.
- Search evidence: `rg -n "exec\\.Command|CommandContext|security\\\"|StoreCredential\\(|DeleteCredential\\(|readKeychain\\(|KeychainAvailable\\(" internal/modelstore cmd internal docs scripts`; live mutations are in `keychain_darwin_test.go` and `service_test.go`.
- Duplication/ownership conclusion: one package-private command factory is the narrow shared seam; production remains sole owner of Keychain grammar and error translation.

## Config, API, CLI, and Tools

- User-facing config: `HARNESS_TEST_REAL_KEYCHAIN=1` is test-only, opt-in host-live configuration.
- Defaults / fallbacks: standard test runs skip real login-Keychain mutation with an explicit reason and use fakes.
- Environment/config files: only the new test flag; persisted credential references unchanged.
- Endpoints, CLI, tools: none; HTTP model settings and all provider routes retain behavior.
- Errors: fake tests preserve existing clear timeout/read/write error translation.

## Persistence and Compatibility

- Schemas/migrations/caches: none.
- Compatibility: `keychain:service/account` grammar and standard real behavior are unchanged when the opt-in is set.
- Mixed rollout: none; tests and runbook ship together.

## Lifecycle, Security, and Reliability

- Concurrency/cancellation: each real command keeps its existing 15-second context; the fake seam deterministically observes passed context rather than changing deadlines or adding retries.
- Security/privacy: test secret remains stdin-only. Standard CI never mutates a user login Keychain. Host-live accounts include the test/process identity and cleanup remains scoped to that exact account.
- Failure/recovery: command failures continue to surface as errors; delete remains idempotent. No error is swallowed beyond the pre-existing absent-delete behavior.

## Product and Integration Surfaces

- Server/runtime: indirect modelstore calls unchanged.
- TUI/web/macOS: none; no client behavior changes.
- Provider/model/tool catalog: none; only test process injection.
- External system: macOS `security(1)` is exercised solely by named host-live tests.
- UX/accessibility: none.

## Deployment and Operations

- Deployment: no runtime rollout.
- Diagnostics: skip reason names the required flag; runbook names deterministic and host-live commands.
- Rollback: revert test seam/gating commit; no persisted data migration exists.
- Runbook: update testing guidance with standard versus opt-in host-live lane.

## Regression Tests

- First expected red: fake command contract tests fail because production constructs `exec.Cmd` directly and no opt-in helper exists.
- Acceptance: create/update/read/delete arguments, secret stdin isolation, timeout/error translation, opt-in skip, unique account.
- Negative/lifecycle/security: unavailable command, timeout, no secret in argv, absent deletion and cleanup scope.
- Real-path: `HARNESS_TEST_REAL_KEYCHAIN=1 go test ./internal/modelstore -run 'Test(KeychainRoundTripAgainstRealKeychain|SavingAKeyForAnExistingProvider)$' -count=5 -v` outside sandbox on a logged-in macOS host.
- Commands: focused normal/race and `./scripts/test-regression.sh` standard lane.

## Documentation and Handoff

- Specs: plan/impact map before source.
- Handoff: logs describe prior failure and exact lanes; plan/log indexes list the new artifacts.
- Training/release: none; test-only operational change.
