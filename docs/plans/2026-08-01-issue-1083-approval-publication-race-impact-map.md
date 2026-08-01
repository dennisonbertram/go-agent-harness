# Issue #1083: Approval Publication Readiness Impact Map

## Task

- Task / issue: #1083 approval event publication race.
- Plan link: `2026-08-01-issue-1083-approval-publication-race-plan.md`.
- Owner: isolated `codex/issue-1083-approval-publication-race` worktree.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: tool gating in `runner_step_engine.go`; plan-exit gate in `plan_mode.go`.
- Owning source: `ApprovalBroker`, `InMemoryApprovalBroker`, and `checkpointApprovalBroker` in `internal/harness`.
- Flow: runner status -> broker pending registration -> approval-required event -> API/TUI/macOS consumer -> shared broker approve/deny -> blocked wait resumes.
- Search evidence: `rg 'ApprovalBroker|EventToolApprovalRequired|EventPlanApprovalRequired|ErrNoPendingApproval' internal cmd` found two approval-required publishers, both broker implementations, and the shared HTTP resolver.
- Conclusion: a single explicit register-then-wait lifecycle belongs in the existing broker abstraction; no duplicate coordinator.

## Config, API, CLI, and Tools

- User-facing config/defaults/environment: None; existing approval policy and timeout values are retained.
- HTTP/CLI/wire: approve/deny endpoint paths, bodies, statuses, and event payload schemas are unchanged. Their readiness guarantee is strengthened.
- Validation/errors: unknown or already-resolved approvals continue returning `ErrNoPendingApproval`/404; a newly published event cannot legitimately reach that state due to registration lag.

## Persistence and Compatibility

- Checkpoint approval records are created before publication; no schema or migration changes.
- Mixed-version behavior: one process uses its own runner and broker atomically; no cross-process protocol change.
- Existing direct `Ask` users retain the register-and-wait behavior through the compatibility method.

## Lifecycle, Security, and Reliability

- Registration is the happens-before point for external event publication. The waiter owns timeout/cancellation cleanup and consumes a resolution that may arrive before waiting begins, even if scheduling delays `Wait` past the deadline. Resolution and expiry are linearized so only one can report success.
- Authentication/authorization and fail-closed gating are unchanged; only a valid shared-broker resolution unblocks execution.
- Duplicate registrations remain rejected; duplicate/late resolution remains no-pending; checkpoint expiry and in-memory conditional removal remain scoped to the exact pending entry. Checkpoint parent-context cancellation continues to return the cancellation while retaining its durable pending record (pre-existing behavior); this slice neither broadens nor repairs that lifecycle policy.

## Product and Integration Surfaces

- Server/runtime: runner, broker, shared server broker and SSE replay/live delivery are affected.
- TUI/web/macOS: no code changes; all immediate consumers gain a reliable actionable event.
- Provider/model/tool routing: ordinary tool approvals and plan-exit approvals are covered; no catalog or provider changes.
- External automation/UX: None beyond existing approval clients.

## Deployment and Operations

- Deploy as one small baseline bug fix before dependent approval UI work.
- Observability: engineering log records symptom, event-to-registration invariant, red/green commands, and preserved failure semantics.
- Rollback: revert the isolated code commit; no persistent data repair or migration required.
- Runbooks: no operator workflow change.

## Regression Tests

- First red: gated old `Ask` path exposes `EventToolApprovalRequired`/plan event before underlying registration, then immediate resolution returns no-pending.
- New acceptance tests: observable event followed by immediate approve/deny succeeds for tool and plan exit; direct in-memory and checkpoint lifecycle splitting verifies registration readiness.
- Edge/failure/lifecycle/security: timeout, cancellation, duplicate registration, pre-Wait approve/deny, expiry-winner late rejection, concurrent resolution-vs-expiry, selected plan options, denial, exact deadline round-trip, and fail-closed execution remain covered by existing and focused tests.
- Exact commands: targeted harness/server normal, `-race`, and bounded `-count`; full regression, hosted checks, and live harnessd canary are intentionally left to parent promotion.

## Documentation and Handoff

- No public docs change because wire contracts are stable.
- Update this plan, plans index, active plan, engineering log, long-term-thinking log, and logs index only if its meaning changes.
- Handoff includes exact head, red/green commands, uncommitted diff, and residual validation excluded by scope.
