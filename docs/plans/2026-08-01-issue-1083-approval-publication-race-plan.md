# Issue #1083: Approval Publication Readiness

## Context

- Governing GitHub issue: #1083, `[Bug]: Approval event can precede broker registration`.
- Problem: the runner publishes `tool.approval_required` and `plan.approval_required` before the configured broker records the corresponding pending approval. A client that posts an immediate approval or denial can receive `ErrNoPendingApproval` (HTTP 404).
- User impact: API, TUI, and macOS clients can show an actionable prompt which rejects the first valid user action and leaves a run waiting.
- Constraints: preserve approval policy, endpoint and event schemas, timeout/cancellation cleanup, duplicate resolution, fail-closed tool gates, and checkpoint durability. No client retry, sleep, UI redesign, or parallel broker.

## Scope

- In scope: split approval registration from blocking wait in the existing broker abstraction; register before either approval-required event; update in-memory and checkpoint-backed implementations; add deterministic tool and plan-exit immediate-resolution regressions.
- Out of scope: HTTP handler behavior, approval policy changes, UI changes, checkpoint schema changes, and full-regression/hosted/live-server validation in this isolated slice.

## Documentation Contract

- Feature status: implemented; uncommitted focused verification complete, pending parent promotion gates.
- Public docs affected: none; event and HTTP schemas are unchanged.
- Spec docs to update before code: this plan and the linked impact map.
- Implementation notes to add after code: engineering log, active-plan tracker, and indexes.

## Test Plan (TDD)

- New failing tests first: a gated broker proves that the old `Ask`-only runner ordering lets an observable approval-required event produce an immediate `ErrNoPendingApproval`; the same test requires immediate approve and deny to resolve tool and plan-exit paths after registration is separated. Review regressions delay `Wait` beyond the deadline after a successful approve/deny and require that committed decision to win, while a parsed SSE deadline must equal the registered timestamp exactly.
- Existing tests to retain: in-memory and checkpoint broker lifecycle, duplicate, timeout, cancellation, runner approval, plan-mode semantics, HTTP handlers, and E2E round-trip coverage.
- Regression tests required: direct broker tests for both implementations plus real runner event-to-immediate endpoint behavior for ordinary tools and plan exit, normal, race, and bounded repetition.

## Cross-Surface Impact Map

See [2026-08-01-issue-1083-approval-publication-race-impact-map.md](2026-08-01-issue-1083-approval-publication-race-impact-map.md).

## Implementation Checklist

- [x] Verify issue #1083 contract and current source ownership/callers.
- [x] Create dedicated worktree from refreshed `origin/main`.
- [x] Record plan and impact map before code.
- [x] Add deterministic red regressions.
- [x] Split broker registration from wait and update both implementations.
- [x] Register before tool and plan approval publication.
- [x] Preserve timeout, cancellation, duplicate-resolution, and authorization behavior.
- [x] Update logs/indexes and record red/green evidence.
- [x] Run focused normal, race, and repetition checks only.

## Risks and Mitigations

- Risk: a decision can arrive after registration but before wait begins. Mitigation: retain the existing buffered/ durable resolution mechanism and make wait consume the registered entry/record.
- Risk: moving registration changes timeout or cancellation ownership. Mitigation: emit and enforce the exact registered deadline while retaining conditional cleanup/expiry ownership in its wait handle.
- Risk: expiry and HTTP resolution race after the deadline. Mitigation: linearize them under the in-memory broker mutex or checkpoint store's atomic `ResolvePending`, then return the winning outcome consistently.
- Risk: third-party broker implementations drift. Mitigation: make the readiness lifecycle an explicit required `ApprovalBroker` contract and compile every in-repository implementation.
