# Plan: Issue #1173 durable run replay

## Context

- Governing GitHub issue: #1173.
- Problem: TUI `/replay run_*` sent a durable ID to rollout-file simulation and failed when capture was unavailable.
- User impact: completed runs advertised by `/runs` could not be replayed.
- Constraints: preserve rollout simulation; no GUI, fallback-policy, or provider-schema redesign.

## Scope

- In scope: authenticated durable replay endpoint, TUI ID dispatch, tenant and terminal gates, tests and evidence.
- Out of scope: rollout simulation semantics and provider fallback reconstruction.

## Test Plan (TDD)

- Red: durable completed run ID must return a distinct 202 replay run preserving conversation/scope/model/effective provider.
- Negative: unknown, nonterminal, cross-tenant, and missing-write-scope paths remain typed and safe.
- TUI: bare ID emits `RunStartedMsg`; `.jsonl` remains simulation.

## Implementation Checklist

- [x] Add durable endpoint and TUI classifier.
- [x] Add focused durable server regression and TUI command regression.
- [x] Complete focused normal/race and repository regression gates.
- [ ] Record fake-provider PTY acceptance evidence before PR handoff.

## Risks and Mitigations

- Risk: file-path simulation is accidentally redefined. Mitigation: only syntactically bare `run_*` uses the new route.
- Risk: cross-tenant replay leaks data. Mitigation: existing write-scope and tenant gates execute before source resolution.
