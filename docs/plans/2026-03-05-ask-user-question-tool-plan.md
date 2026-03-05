# Plan: Claude-Compatible AskUserQuestion Tool (Server + Runner)

## Context

- Problem: The harness currently lacks a first-class way for tool calls to pause execution and collect structured user input.
- User impact: Frontends cannot reliably support mid-run clarification workflows compatible with Claude Agent SDK behavior.
- Constraints:
  - Strict TDD and regression gate enforcement.
  - Preserve default CLI behavior (non-interactive) in this iteration.
  - Keep tool and API contracts deterministic and JSON-schema driven.

## Scope

- In scope:
  - Add `AskUserQuestion` tool contract and handler with Claude-compatible schema.
  - Add in-memory broker to manage pending questions and answer submission.
  - Add `waiting_for_user` run status and waiting/resumed run events.
  - Add `GET/POST /v1/runs/{runID}/input` endpoints.
  - Add timeout configuration via `HARNESS_ASK_USER_TIMEOUT_SECONDS`.
  - Add comprehensive tests across tool, broker, runner, and HTTP layers.
  - Update required docs and logs.
- Out of scope:
  - Interactive CLI prompting.
  - Persistent question state across process restarts.

## Test Plan (TDD)

- New failing tests to add first:
  - `internal/harness/tools/ask_user_question_test.go`
  - `internal/harness/ask_user_broker_test.go`
  - New runner waiting/resume/timeout tests in `internal/harness/runner_test.go`
  - New input endpoint tests in `internal/server/http_test.go`
- Existing tests to update:
  - `internal/harness/tools/catalog_test.go`
  - `internal/harness/tools_contract_test.go`
  - `cmd/harnessd/main_test.go`
- Regression tests required:
  - `go test ./...`
  - `go test ./... -race`
  - `./scripts/test-regression.sh`

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Write failing tests first (new test coverage added for tool/broker/runner/http).
- [x] Implement minimal code changes.
- [x] Refactor while tests remain green.
- [x] Update docs and indexes.
- [x] Update engineering/system/observational/long-term logs.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: Run status can get stuck in waiting state on non-timeout failures.
- Mitigation: Explicit status reset to `running` for non-timeout tool errors.
- Risk: Invalid answer payloads could accidentally clear pending questions.
- Mitigation: Broker validates and preserves pending state on invalid submissions.
- Risk: Contract drift versus Claude-compatible payload shape.
- Mitigation: Use exact `questions` + `answers` result schema and enforce constraints.
