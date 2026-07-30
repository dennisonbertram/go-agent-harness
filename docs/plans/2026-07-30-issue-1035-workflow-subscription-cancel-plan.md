# Plan: Make Workflow Subscription Cancellation Test Deterministic

## Context

- Governing GitHub issue: #1035.
- Problem: the race gate intermittently expects the first receive from a
  cancelled buffered subscription to report channel closure, even though Go
  delivers already-buffered events before `ok == false`.
- Impact: an otherwise valid full regression run is red and blocks every
  issue-gated merge.
- Constraint: preserve the production `Engine.Subscribe` lifecycle; this is a
  test-model repair, not a runtime redesign.

## Scope

- In scope: make the buffered-event ordering deterministic, drain events
  accepted before cancellation, and require bounded eventual channel closure.
- Out of scope: production subscription/fanout code, channel size, workflow
  execution, persistence, public APIs, and clients.

## Test Plan

- Red: explicitly emit a known event after `Subscribe` and before `cancel`,
  retaining the current single-receive assertion.
- Expected red: the first receive returns the buffered event with `ok == true`.
- Green: drain buffered values until `ok == false`, with a timeout that still
  detects a channel that never closes.
- Stress: focused test under `-race -count=100`, package normal/race tests, and
  complete repository regression gate.

## Cross-Surface Impact Map

- See `2026-07-30-issue-1035-workflow-subscription-cancel-impact-map.md`.

## Implementation Checklist

- [x] Create contract-complete bug #1035.
- [x] Record current ownership and shared-lock evidence.
- [x] Write plan and impact map before code.
- [x] Capture deterministic failing test.
- [x] Implement the minimal test-model repair.
- [x] Run focused stress, package, and full gates.
- [x] Update issue/engineering evidence.
- [ ] Merge through a closing PR.

## Risks and Mitigations

- Risk: merely deleting the assertion would hide a real non-closing channel.
- Mitigation: keep an explicit deadline and require eventual `ok == false`.
- Risk: modifying production behavior to satisfy an invalid test assumption.
- Mitigation: leave `Engine.Subscribe` and `Engine.emit` unchanged.
