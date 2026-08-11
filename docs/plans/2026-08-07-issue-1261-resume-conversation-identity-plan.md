# Plan: Issue #1261 Resume Continuation Conversation Identity

## Context

- Governing GitHub issue: #1261.
- Problem: a continuation child run belongs to its source conversation, but the continue response drops that identity. A blank TUI consequently treats the child run ID as a conversation ID and opens a guaranteed-404 conversation stream after a correct reply.
- User impact: `/resume <source-run> <prompt>` reports a false SSE failure and fails the intended durable conversation lifecycle.
- Constraints: preserve fresh-run `conversation_id == run_id`, retain selected conversation ownership, make only additive wire changes, and do not touch #1260 dual-SSE work or scheduler code.

## Scope

- In scope: additive `conversation_id` on `POST /v1/runs/{id}/continue`; TUI response/message plumbing; safe legacy-server lookup; reducer ownership and lifecycle regressions; real 100x30 PTY proof.
- Out of scope: scheduler/callback/cron changes, persistence schema, selected-conversation redesign, and #1260 reconciliation behavior.

## Documentation Contract

- Feature status: in implementation.
- Public docs affected: none; this is a compatible API response expansion and an existing slash-command repair.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: engineering, observational, and system logs plus their index.

## Test Plan (TDD)

- New failing tests to add first: server response must return inherited conversation; client must transport it; missing response field must look up child-run identity; blank-TUI continuation must select the parent endpoint rather than the child endpoint and avoid an SSE warning.
- Existing tests to update: the continuation endpoint and `/resume` command fixtures.
- Regression tests required: normal/race focused TUI and server tests, full regression, and an exact 100x30 PTY transcript with child run distinct from its conversation.

## Cross-Surface Impact Map

See `2026-08-07-issue-1261-resume-conversation-identity-impact-map.md`.

## Implementation Checklist

- [x] Define acceptance criteria in tests and verify structured issue #1261.
- [x] Record current ownership/callers/search evidence and complete impact map.
- [x] Capture expected red tests.
- [x] Implement minimal additive identity plumbing and legacy resolution.
- [x] Run focused normal/race, full regression, and real PTY proof.
- [x] Update durable logs and indexes.
- [ ] Commit, push, and open a closing PR; do not merge.

## Risks and Mitigations

- Risk: an old server omits `conversation_id`, so using the child run ID reproduces the false 404. Mitigation: resolve the accepted child run through its established GET endpoint and fail visibly if no authoritative identity is returned.
- Risk: an incoming continuation identity overwrites an actively selected different conversation. Mitigation: preserve an existing selected conversation unless the new identity agrees, while blank TUI adopts the authoritative returned identity.
