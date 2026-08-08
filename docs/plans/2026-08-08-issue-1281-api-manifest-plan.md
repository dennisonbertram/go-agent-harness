# Plan: Issue #1281 API acceptance manifest contract

## Context

- Governing issue: #1281.
- Problem: the live inventory had no reviewed execution-plan mapping; an empty
  hash-correct manifest correctly reported every tool missing, but did not
  assign an owner or safe execution cohort for the remaining matrix work.
- Scope: bind every resolver-derived API item to its reviewed execution cohort.
  This slice does not execute a tool scenario, prove cron/callback convergence,
  or make any GUI/TUI claim.

## Test-First Plan and Evidence

1. Red: a focused test references the absent checked-in manifest and contract
   fields, so the package does not compile on the unmodified baseline.
2. Green: a complete mapping binds each item ID, owner, condition,
   disposition, and cohort. The validator rejects a partial mapping,
   owner/condition drift, stale hash, duplicate/unknown/API-inapplicable rows,
   and invalid unavailable/out-of-scope semantics.
3. Red then green: a daemon-source mismatch fails before any inventory report.
   The checked-in manifest pins `daemon_source_sha`; the CLI requires the
   lifecycle-owned `provenance.json`, rejects relative/noncanonical/missing
   command paths, recomputes SHA-256 from the canonical executable, and binds
   its source SHA, listener address, canonical command identity, and recomputed
   digest to the requested harness URL.
4. The isolated fake-provider daemon's live `/v1/tools` inventory is pinned at
   hash `5230df4123f5c860c4849f5b44ab90f502b2212dec9dd8d1fce5fe2e78fcf1fa`:
   67 available API tools map to cohorts, while zero executable cases means the
   CLI reports `planned=0`, `missing=67`, and exits non-zero.
4. Verify focused normal/race tests and the repository regression gate.

## Rollout and Rollback

- Rollout: consumers must pass both this JSON manifest and the exact
  lifecycle-generated `provenance.json` to `acceptance-api-sse`; source,
  command, listener, live inventory, ownership, or resolver-condition drift
  fails before a report can count coverage.
- Rollback: revert the additive manifest schema, fixture, and documentation.
  Preserve generated acceptance artifacts; do not relabel a mapping as tool
  execution.
