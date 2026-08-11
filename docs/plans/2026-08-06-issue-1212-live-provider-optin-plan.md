# Plan: Issue #1212 live provider fetch opt-in

## Context

- Governing GitHub issue: #1212
- Problem: `TestLiveFetchAgainstRealProviders` is included by ordinary `go test` and currently reaches real OpenAI/OpenRouter endpoints whenever a credential happens to be present.
- User impact: a developer with normal provider credentials can get an external timeout or paid network call from the deterministic repository regression command.
- Constraints: keep a narrow real-smoke command available; do not change provider implementation, credentials, endpoints, retry policy, or acceptance tooling.

## Scope

- In scope: a test-local explicit environment gate, a no-network gate regression, testing-runbook command, and required delivery records.
- Out of scope: production modelstore code; provider catalog or routing; persistent configuration; server, API, CLI, TUI, web, macOS, and deployment behavior.

## Documentation Contract

- Feature status: implemented.
- Public docs affected: None.
- Spec/runbook updated after code: `docs/runbooks/testing.md` documents the explicit live-smoke command and the offline regression guarantee.
- Implementation notes after code: engineering, observational, system, and long-term logs record the boundary and verification evidence.

## Test Plan (TDD)

- First failing test: `TestLiveProviderFetchEnabled` references the test-local gate and proves credentials alone cannot enable a live request while the explicit flag plus a credential can.
- Expected red: `undefined: liveProviderFetchEnabled`.
- Minimal green: the live parent test returns before constructing a fetcher unless `HARNESS_TEST_LIVE_PROVIDERS=1`; each provider subtest still requires its own credential.
- Regression: focused normal/race package tests and `TMPDIR=/private/tmp ./scripts/test-regression.sh`; no real-provider invocation during validation.

## Implementation Checklist

- [x] Verify issue contract and origin/main worktree provenance.
- [x] Record ownership/search evidence and cross-surface impact map.
- [x] Capture the deterministic red gate test (`undefined: liveProviderFetchEnabled`).
- [x] Implement the minimal explicit opt-in gate.
- [x] Document the dedicated live-smoke command.
- [x] Update delivery logs and indexes.
- [x] Run focused normal/race and the full offline regression gate (85.4% total coverage; zero uncovered functions).
- [ ] Commit and push only this issue's files; do not open a PR or merge.

## Risks and Mitigations

- Risk: an operator cannot find the preserved real smoke lane. Mitigation: place the exact flag-plus-credential command in the testing runbook and test comment.
- Risk: a credential-only machine still contacts a provider. Mitigation: no-network unit coverage asserts the two-input gate and the parent checks it before any fetcher call.
- Rollback: revert the isolated test/documentation commit; no state, data, or deployment rollback is required.
