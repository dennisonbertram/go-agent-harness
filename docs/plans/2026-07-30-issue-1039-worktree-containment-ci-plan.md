# Plan: Synchronize Worktree Containment Assertions with Cleanup

## Context

- Governing GitHub issue: #1039.
- Problem: the containment test consumes a buffered `tool.call.completed`
  event after the runner may already have completed its next turn and removed
  the worktree.
- User impact: a false-negative Linux fast gate blocks the cron/callback GUI
  merge chain despite valid worktree routing.
- Constraint: keep the real bash tool and every containment assertion; do not
  change production workspace lifetime to satisfy a test-only race.

## Scope

- In scope: add a test-provider handshake that holds the terminal provider
  turn until the subscriber has inspected the live worktree.
- Out of scope: production runner, event, workspace, tool, API, and client
  behavior.

## Documentation Contract

- Feature status: implemented; merge pending.
- Public docs affected: none; no user-facing contract changes.
- Implementation evidence: engineering and long-term logs, this plan, impact
  map, and plans index.

## Test Plan (TDD)

- Red: deterministically let the terminal provider turn return before the
  subscriber examines `tool.call.completed`; prove the worktree is absent.
- Green: block that terminal turn on a bounded test-only release, perform the
  existing filesystem assertions, then release cleanup.
- Stress: focused normal/race tests at `-count=100`, harness package tests, and
  the full repository regression gate.

## Cross-Surface Impact Map

- See `2026-07-30-issue-1039-worktree-containment-ci-impact-map.md`.

## Implementation Checklist

- [x] Create contract-complete bug #1039.
- [x] Record ownership and lifecycle search evidence.
- [x] Write plan and impact map before code.
- [x] Capture deterministic red evidence.
- [x] Implement the minimal test handshake.
- [x] Run focused, race, package, and full gates.
- [ ] Merge through a closing PR.

## Risks and Mitigations

- Risk: a blocking fixture could deadlock the test.
- Mitigation: both provider wait and subscriber wait have bounded timeouts.
- Risk: the repair could weaken the containment contract.
- Mitigation: retain real bash execution, exact resolved cwd, in-worktree file,
  no daemon-cwd leak, and successful completion assertions.
