# Plan: Issue #1141 callback deadline-release fixture

## Context

- Governing GitHub issue: #1141.
- Problem: three callback tests used a 40 ms lease and inferred deadline cancellation from a heartbeat-only channel. Under load the deadline can cancel admission before the heartbeat calls `ExtendLease`, leaving that channel unclosed despite correct runtime behavior.
- User impact: flaky baseline checks obscure durable callback guarantees needed by API, TUI, and native transcript continuation work.
- Constraints: test and documentation only; do not alter callback behavior.

## Scope

- In scope: deterministic gates for the three deadline-release tests and retained/stronger durable state assertions.
- Out of scope: callback policy, scheduler behavior, public API, persistence, TUI, and native GUI changes.

## Documentation Contract

- Feature status: implemented test-fixture repair.
- Public docs affected: none; runtime behavior is unchanged.
- Spec docs to update before code: this plan and impact map.
- Implementation notes to add after code: engineering, observational, system, and long-term-thinking logs plus their indexes.

## Test Plan (TDD)

- First red evidence: hosted #1138 fast CI reported the 40 ms fixture timing failure; the stronger tests replace that non-causal observation.
- New/updated tests: wait in order for admitted entry, heartbeat renewal entry, deadline gate, and cancellation-aware starter cancellation before release.
- Regression tests required: preserve same-manager retry, safe reason, and attempt-cap terminal state including exact run identity/cleared ownership.

## Cross-Surface Impact Map

`2026-08-03-issue-1141-callback-deadline-fixture-impact-map.md`.

## Implementation Checklist

- [x] Verify the structured issue and repository owners/search evidence.
- [x] Write plan and impact map before fixture changes.
- [x] Replace timing assumptions with causal test gates.
- [x] Retain durable-state assertions and strengthen the attempt-bound state.
- [x] Run focused normal/race x20 and full regression.
- [x] Update final evidence and create the reviewable PR.

## Risks and Mitigations

- Risk: a longer lease hides a runtime deadline bug.
- Mitigation: each test now observes the actual deadline signal and admission cancellation before unblocking; no pass is based on elapsed sleep alone.
