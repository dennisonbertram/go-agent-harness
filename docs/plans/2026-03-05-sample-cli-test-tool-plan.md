# Plan: Sample CLI Test Tool for Harness Service

## Context

- Problem: There is no lightweight CLI for quickly testing and observing harness runs from terminal.
- User impact: Manual `curl` flows are slower and less ergonomic for repeated testing.
- Constraints:
  - Keep CLI minimal and practical.
  - Support both run creation and event streaming.
  - Follow strict TDD and keep regression gates green.

## Scope

- In scope:
  - Add a small CLI under `cmd/` that connects to harness HTTP API.
  - Support creating a run (`POST /v1/runs`) with prompt/model/system prompt.
  - Support streaming run events (`GET /v1/runs/{id}/events`) and stopping on terminal events.
  - Add unit tests using `httptest` for run creation and SSE streaming.
  - Document usage in README.
- Out of scope:
  - Interactive TUI.
  - Persistent local run history.

## Test Plan (TDD)

- New failing tests to add first:
  - Start-run request sends expected payload and returns run id.
  - SSE event stream parser handles event/data framing and terminal detection.
  - CLI run flow (start + stream) reports completion and exit status.
- Existing tests to update:
  - N/A.
- Regression tests required:
  - API error path handling (non-2xx on create or stream).

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Write failing tests first.
- [x] Implement minimal code changes.
- [x] Refactor while tests remain green.
- [x] Update docs and indexes.
- [x] Update engineering/system/observational logs as needed.
- [x] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: SSE parsing edge cases could lead to hanging CLI behavior.
- Mitigation: stop on explicit terminal events and enforce HTTP status validation.
