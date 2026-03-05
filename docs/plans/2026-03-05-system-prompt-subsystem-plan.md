# Plan: Modular System Prompt Subsystem With Intent Routing and Runtime Injection

## Context

- Problem: System prompt behavior was mostly a single string and hard to evolve by model, intent, and runtime context.
- User impact: Prompt customization and specialization were difficult to manage, audit, and test.
- Constraints:
  - Keep `system_prompt` override compatibility.
  - Preserve run/event contracts and existing hook pipeline semantics.
  - Enforce strict TDD and regression gate.

## Scope

- In scope:
  - Add `internal/systemprompt/` engine with YAML catalog + file-backed prompt assets.
  - Add prompt assets under `prompts/`.
  - Add run request fields for intent/profile/extensions.
  - Add runtime context system message per turn.
  - Emit `prompt.resolved` and `prompt.warning` events.
  - Add tests for catalog/engine/matcher and runner/server/cli integration.
- Out of scope:
  - Claude Skills runtime integration.
  - Live usage/cost injection (phase 2).

## Test Plan (TDD)

- New failing tests added first:
  - `internal/systemprompt/catalog_test.go`
  - `internal/systemprompt/matcher_test.go`
  - `internal/systemprompt/engine_test.go`
  - `internal/harness/runner_prompt_test.go`
  - `internal/server/http_prompt_test.go`
  - `cmd/harnesscli/main_prompt_test.go`
- Existing tests updated:
  - none required for behavior lock; existing suites validated compatibility.
- Regression tests required:
  - `go test ./...`
  - `go test ./... -race`
  - `./scripts/test-regression.sh`

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Write failing tests first.
- [x] Implement modular code changes.
- [x] Refactor while tests remain green.
- [x] Update docs and indexes.
- [x] Update engineering/system/observational/long-term logs.
- [x] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: Prompt file path resolution breaks startup in non-root working directories.
- Mitigation: add upward-search fallback for `prompts/catalog.yaml` and env override `HARNESS_PROMPTS_DIR`.
- Risk: Unknown extension IDs silently degrade behavior.
- Mitigation: strict validation failure (`invalid_request`) for unknown intent/profile/behavior/talent.
- Risk: Runtime context may grow transcript size unexpectedly.
- Mitigation: keep runtime block ephemeral and rebuild each turn instead of persisting it in history.
