# Plan: Token Counting and Cost Tracking (OpenAI-first, Extensible)

## Context

- Problem: token usage and cost fields exist in types but were not populated end-to-end.
- User impact: clients could not display token burn/cost by turn or at run completion.
- Constraints:
  - Missing provider usage must not fail runs.
  - Pricing defaults must not be hardcoded or bundled by default.
  - Changes must remain additive and backward-compatible.

## Scope

- In scope:
  - Additive usage/cost types and statuses in harness contracts.
  - OpenAI usage parsing + cost computation with pricing resolver integration.
  - New pricing module with file-backed resolver and alias support.
  - Runner accounting accumulation, `usage.delta` events, and run totals persistence.
  - Runtime context live token/cost fields replacing phase-1 placeholder.
  - HTTP/API and tests/docs updates.
- Out of scope:
  - Anthropic adapter implementation.
  - Streaming per-chunk token deltas.
  - Bundled default pricing tables.

## Test Plan (TDD)

- New failing tests to add first:
  - `internal/provider/openai/client_test.go` usage/cost/status mapping.
  - `internal/provider/pricing/catalog_test.go` catalog load + resolve behavior.
  - `internal/harness/runner_test.go` usage/cost accumulation + terminal payloads.
  - `internal/harness/runner_prompt_test.go` runtime context accounting propagation.
  - `internal/server/http_test.go` SSE/GET run contract updates.
  - `internal/systemprompt/engine_test.go` runtime context format changes.
- Existing tests to update:
  - OpenAI client and system prompt runtime context expectations.
- Regression tests required:
  - `go test ./...`
  - `go test ./... -race`
  - `./scripts/test-regression.sh`

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

- Risk: price table drift produces stale cost outputs.
- Mitigation: require explicit catalog path (`HARNESS_PRICING_CATALOG_PATH`) and expose `cost_status`.
- Risk: pointer-based totals in run state could leak mutable references.
- Mitigation: deep-copy totals in `GetRun`.
- Risk: providers without usage could produce ambiguous behavior.
- Mitigation: normalized zero-value accounting with explicit `provider_unreported` status.
