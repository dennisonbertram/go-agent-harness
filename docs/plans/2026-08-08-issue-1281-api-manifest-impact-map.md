# Impact Map: Issue #1281 API acceptance manifest contract

## Scope and Ownership

- Task: #1281; plan: `2026-08-08-issue-1281-api-manifest-plan.md`.
- Ownership/data flow: `cmd/acceptance-api-sse` decodes
  `apisserunner.Manifest`; `BuildCoverageReport` compares its rows against the
  runtime `/v1/tools` inventory compiled by `internal/acceptance/inventory`.
- Search evidence: `rg -n "apisserunner|Manifest|coverage" cmd internal docs`
  found the runner as the sole API report consumer and the inventory package as
  the sole catalog/hash validator.

## Cross-Surface Analysis

- API/CLI: additive `daemon_source_sha` manifest field and mandatory
  `-provenance` lifecycle artifact; reports serialize the accepted daemon
  source and command digest alongside the inventory hash. No public harness
  route, tool schema, or provider contract changes.
- Harness/persistence/lifecycle: no production store, scheduler, callback,
  cron, retry, or process lifecycle changes. The CLI consumes the existing
  lifecycle `provenance.json` and the already-public inventory endpoint.
- TUI/native GUI: unaffected; #1088/#1089 retain all rendered-client proof.
- Provider/tool catalog: mappings bind exact ID, owner, condition, and whole
  inventory hash. Resolver N/A items require an `unavailable` disposition and
  rationale; available exclusions require an explicit rationale.
- Security/operations: no credentials in the manifest. The report rejects a
  bare operator URL: lifecycle source/address/path/digest provenance must
  match before live inventory is trusted. A mapping is not a passing case:
  absent `Cases` are still reported missing and the CLI exits 1.

## Verification and Rollback

- Tests: checked-in manifest parsing/counting, source-mismatch-before-report
  rejection, lifecycle-artifact CLI load, partial mapping rejection,
  owner/condition drift rejection, existing stale hash/missing-case/N/A tests,
  focused race, and `./scripts/test-regression.sh`.
- Rollback: one additive revert; no migration or data repair.
