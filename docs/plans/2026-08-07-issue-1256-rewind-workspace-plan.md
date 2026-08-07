# Plan: Issue #1256 trusted workspace metadata for rewind

## Context

- Governing GitHub issue: #1256.
- Problem: mutating-tool rewind points can be captured before the conversation
  row stores the configured workspace; default conversations store no workspace
  at terminal persistence, so a fork inherits an empty untrusted owner and the
  destructive route correctly returns 404.
- User impact: a valid TUI rewind point cannot restore its own safe pre-image.
- Constraints: persist only the canonical `RunnerConfig.WorkspaceBaseOptions.RepoPath`;
  never infer a restore root from daemon CWD, a client request, or a point.

## Scope

- In scope: immediate conversation metadata persistence before a rewind point
  can be captured; default and named tenant metadata; fork/HTTP/TUI regression
  coverage and a 30x100 real PTY proof.
- Out of scope: schema migration, client-provided workspace paths, CWD fallback,
  and any relaxation of the empty-workspace 404 boundary.

## Documentation Contract

- Feature status: in implementation.
- Public docs affected: none; the existing session-rewind runbook remains
  accurate because its destructive API and confirmation semantics do not change.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: engineering, observational, system,
  and long-term-thinking logs plus their index.

## Test Plan (TDD)

- New failing tests first: a runner-owned mutating tool captures a point only
  after the conversation owner contains the canonical workspace for both
  default and named tenants; a server fork restores its own file; an empty
  owner remains 404 without altering either workspace file.
- Existing tests to update: focused runner/server rewind tests and the PTY
  acceptance fixture.
- Regression tests required: focused normal/race, full `./scripts/test-regression.sh`,
  and a 30x100 test-owned PTY `/rewind safe-point confirm` proof.

## Cross-Surface Impact Map

See `2026-08-07-issue-1256-rewind-workspace-impact-map.md`.

## Implementation Checklist

- [x] Define acceptance criteria in #1256.
- [x] Link a contract-complete structured GitHub issue before implementation.
- [x] Record current ownership and source-of-truth search evidence in impact map.
- [x] Document feature status and exact contract before code.
- [x] Complete and reconcile the cross-surface impact map before implementation.
- [ ] Add characterization and failing regressions.
- [ ] Persist the canonical workspace before rewind capture.
- [ ] Preserve fork inheritance and strict 404 behavior.
- [ ] Run focused, full, and real PTY verification.
- [ ] Update durable logs and indexes.

## Risks and Mitigations

- Risk: a late metadata write makes a listed rewind point unusable. Mitigation:
  write trusted metadata before the capture call and test the ordering.
- Risk: a fallback broadens destructive file access. Mitigation: use only the
  Runner-configured path and retain the existing empty-owner 404 test.
- Rollback: revert the early canonical metadata handoff; the existing strict
  route rejection remains in place.
