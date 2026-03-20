# Plan: Issue #361 golden-path deployment profile and smoke suite

## Context

- Problem: the documented `--profile full` golden path is not a real startup contract for `harnessd`, and the shipped smoke script currently fails before boot because the profile cannot be resolved.
- User impact: contributors do not have one reliable local deployment path to validate before merge, and the current smoke script gives false confidence while missing persistence/readback checks.
- Constraints:
  - Strict TDD.
  - Keep the smoke path live-provider based but narrow and deterministic.
  - Do not require extra infrastructure beyond a provider credential and local sqlite files.

## Scope

- In scope:
  - Make the documented `full` startup path resolve consistently in-repo.
  - Define the golden-path env contract for run and conversation persistence.
  - Extend the smoke script to verify the supported path, including persistence readback and a real tool call.
  - Add regression coverage for profile resolution and golden-path startup/persistence wiring.
- Out of scope:
  - CI-enforcing the live smoke path.
  - Adding optional extras like S3 backup, external MCP servers, or third-party search to the golden path.

## Test Plan (TDD)

- New failing tests to add first:
  - Config/bootstrap test proving the `full` profile path can be resolved without a user-local profile file.
  - Harness startup integration test proving the golden-path profile starts with run + conversation persistence enabled and exposes persistence-backed readback.
- Existing tests to update:
  - Smoke script contract if it assumes the old broken profile path or lacks persistence assertions.
- Regression tests required:
  - Regression test for profile fallback/resolution used by `harnessd --profile full`.
  - Regression test for run-store + conversation-store wiring in the golden-path startup path.

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- Create a one-page impact map from `IMPACT_MAP_TEMPLATE.md` covering:
  - Config: yes, profile resolution and golden-path env contract.
  - Server API: yes, smoke and regression checks on health/models/runs/events/persistence endpoints.
  - TUI state: None, this issue is harness-only.
  - Regression tests: yes, config/startup integration plus live smoke entrypoint.

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

- Risk: live smoke behavior remains provider-sensitive and brittle.
- Mitigation: keep the scripted path provider-agnostic where possible and document the exact env/model contract.
