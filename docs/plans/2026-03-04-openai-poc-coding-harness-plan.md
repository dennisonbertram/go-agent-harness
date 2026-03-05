# Plan: OpenAI-Powered Go Coding Harness POC

## Context

- Problem: The repository has process scaffolding but no runnable coding harness implementation.
- User impact: Without a working service and event stream, GUI/TUI integration cannot start.
- Constraints:
  - Keep implementation as an MVP proof-of-concept with a small, safe tool surface.
  - Build as a service/server with explicit event emission.
  - Follow strict TDD workflow and update project logs/docs.

## Scope

- In scope:
  - Build a Go HTTP service that accepts harness runs and streams run events.
  - Implement an OpenAI provider adapter using chat-completions tool-calling.
  - Implement a minimal coding-oriented tool set (`list_files`, `read_file`, `run_go_test`).
  - Add tests for tool behavior, event publishing, and run loop tool-calling flow.
  - Document setup and API usage in README and logs.
- Out of scope:
  - Multi-provider support.
  - Persistent run/session store.
  - Web GUI/TUI implementation.

## Test Plan (TDD)

- New failing tests to add first:
  - Event broker subscription receives published run lifecycle events in order.
  - File tools enforce workspace boundaries and return expected data/errors.
  - Run loop handles model tool-calls, executes tools, and completes with assistant output.
  - HTTP SSE endpoint streams emitted events for a run.
- Existing tests to update: N/A (no existing implementation tests).
- Regression tests required:
  - Tool-argument validation failure paths.
  - OpenAI provider malformed response handling.

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

- Risk: Provider API shape drift causes runtime failures.
- Mitigation: Keep provider adapter isolated and covered by contract tests against mocked API responses.

- Risk: Unsafe tool execution in coding harness context.
- Mitigation: Restrict tool capabilities to workspace-scoped file reads/listing and bounded `go test` command execution with timeouts.
