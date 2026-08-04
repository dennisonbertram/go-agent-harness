# Plan: Issue #1165 acceptance runtime provenance guard

## Context

- Governing GitHub issue: #1165.
- Problem: an acceptance artifact claimed clean source `3ffc3d764`, while its
  executable's `go version -m` metadata named dirty revision `7f8b2c92557b`.
- User impact: a stale executable can dispatch a real provider request and make
  an acceptance failure or pass untrustworthy.
- Constraints: reject before daemon startup or prompt dispatch; do not alter
  provider routing, scheduler behavior, credentials, API, or persisted data.

## Scope

- In scope: a reusable shell guard for acceptance launchers, exact clean VCS
  metadata validation from `go version -m`, SHA-256 and build-info artifacts,
  and deterministic launch-order regression coverage.
- Out of scope: production `harnessd` behavior, provider configuration, cron
  semantics, GUI behavior, and automatic recovery/rebuild after rejection.

## Documentation Contract

- Feature status: `implemented`.
- Public docs affected: none; this is an operator acceptance-runbook contract.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: acceptance runbook and durable logs.

## Test Plan (TDD)

- New failing test: a fake stale/dirty `go version -m` result causes the guarded
  smoke launch to exit before its fake daemon can create its start marker.
- Existing tests to update: none.
- Regression tests required: clean matching build info emits a JSON artifact
  with the SHA-256; mismatch and dirty cases emit no artifact/start marker.

## Implementation Checklist

- [x] Define acceptance criteria and current-source evidence.
- [x] Complete cross-surface impact map.
- [x] Write failing tests first.
- [x] Implement the minimal reusable guard and wire the smoke launcher.
- [x] Update runbook, durable logs, and indexes.
- [x] Run focused, race, and repository regression gates.

## Risks and Mitigations

- Risk: a malformed or absent build-info record is treated as trustworthy.
- Mitigation: require `vcs=git`, full expected revision, and explicit
  `vcs.modified=false`; reject every absence or mismatch.
