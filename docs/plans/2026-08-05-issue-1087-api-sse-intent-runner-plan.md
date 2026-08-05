# Plan: Issue #1201 API/SSE evidence-runner foundation

## Context

- Governing GitHub issue: #1201 (child of #1087).
- Problem: the hash-bound #1086 inventory proves catalog completeness but does not execute API/SSE cases, retain raw lifecycle evidence, or independently probe durable state.
- User impact: operators need a deterministic, fixture-safe proof that a tool intent—not merely a completed tool event—succeeded over the production HTTP boundary.
- Constraints: exact `93bfc883` baseline, isolated real daemon fixtures, fake provider by default, no product route/schema changes, no PTY/native/cron convergence claims.

## Scope

- In scope: registry-derived API case completeness, ordered start/continue HTTP actions, raw SSE capture, terminal and independent probe evidence, cleanup and no-mutation failure records.
- Out of scope: PTY/native GUI, live credentials except an opt-in documented subset, product fixes discovered by a case, and #1010 scheduling convergence.

## Documentation Contract

- Feature status: `in implementation`.
- Public docs affected: none.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: acceptance runbook, durable logs, and indexes.

## Test Plan (TDD)

- New failing tests to add first: a real HTTP/SSE fixture proves that a complete registry-derived plan records run/conversation/event identities, exact ordered start/continue actions, raw SSE and terminal probes; a missing case fails before dispatch; a rejected action records no mutation.
- Existing tests to update: inventory case/evidence contract remains the validator.
- Regression tests required: focused normal/race acceptance package, `cmd/harnessd` real-daemon fixture, and `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Cross-Surface Impact Map

See `2026-08-05-issue-1087-api-sse-intent-runner-impact-map.md`.

## Implementation Checklist

- [x] Verify #1201, parent #1087, #1086, and #1090 boundaries and current architecture.
- [x] Capture a green exact-head regression baseline.
- [x] Write plan and impact map before implementation.
- [ ] Observe focused red runner tests.
- [ ] Implement the minimal API/SSE executor and evidence artifact writer.
- [ ] Add fixture-safe intent cases (one profile lifecycle case is green; all
  remaining eligible families still need positive intent plans).
- [x] Add live-inventory-derived no-mutation rejection coverage for every
  current available API item.
- [ ] Run focused normal/race and full regression.
- [ ] Update docs, logs, indexes, issue, and draft PR (`Closes #1201`; never close parent #1087 from this foundation PR).

## Risks and Mitigations

- Risk: a tool event is mistaken for an intended effect. Mitigation: `inventory.ValidateEvidence` requires a separately verified typed probe and cleanup.
- Risk: dynamic/external tools are silently skipped. Mitigation: preserve inventory N/A resolver records and reject missing available cases.
- Risk: fixture state leaks. Mitigation: unique roots, explicit cleanup checks, raw failure artifacts, and no use of real credentials by default.
