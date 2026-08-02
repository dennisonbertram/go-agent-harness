# Plan: deterministic Keychain regression gate

## Context

- Governing GitHub issue: #1096.
- Problem: standard regression currently executes real login-Keychain mutation whenever `security(1)` exists; that environment-dependent lane has killed unrelated regression runs at the command deadline.
- User impact: standard confidence must be deterministic, while an explicit host-live lane retains actual login-Keychain proof.
- Constraints: no timeout extension, retries, global serialization, ignored failures, provider behavior changes, or secrets-storage semantic changes.

## Scope

- In scope: inject the `security` command boundary for deterministic modelstore coverage; gate every real Keychain mutation test on `HARNESS_TEST_REAL_KEYCHAIN=1`; unique live accounts; documented lanes.
- Out of scope: provider routing, saved credential semantics, callbacks, schemas, and product clients.

## Documentation Contract

- Feature status: implemented and verified locally; parent promotion/review remains.
- Public docs affected: none; this is contributor/test-runbook behavior.
- Spec docs before code: this plan and its impact map.
- Implementation notes after code: engineering and observational logs plus indexes.

## Test Plan (TDD)

- New failing tests first: fake command tests prove create/update arguments, stdin-only secret delivery, read/delete, timeout/error propagation, and an opt-in gate test proves a missing flag skips real mutation before command construction.
- Existing tests to update: the Darwin round trip and existing provider-save integration use the opt-in helper and unique account names.
- Regression tests: `go test ./internal/modelstore -count=1`, `go test ./internal/modelstore -race -count=20`, full regression, then repeated named host-live opt-in run.

## Cross-Surface Impact Map

See `2026-08-03-issue-1096-keychain-gate-impact-map.md`.

## Implementation Checklist

- [x] Verify issue #1096 and architecture search evidence.
- [x] Record plan and impact map before source changes.
- [x] Add and capture deterministic red tests.
- [x] Introduce the minimal injectable command seam.
- [x] Gate real mutation tests and use unique accounts.
- [x] Document deterministic and host-live lanes.
- [x] Update logs/indexes and run focused, full, and host-live verification.

## Risks and Mitigations

- Risk: test-only injection changes production command behavior. Mitigation: production default delegates exactly to `exec.CommandContext`; fakes assert full command contract.
- Risk: host-live proof disappears. Mitigation: explicit environment gate, clear skip text, unique-account cleanup, and a documented repeated host command.
