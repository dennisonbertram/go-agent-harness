# Plan: Issue #1162 authoritative fake-provider routing

## Context

- Governing GitHub issue: #1162
- Problem: `HARNESS_PROVIDER=fake` builds a fake default provider, but catalog lookup can select a configured real client before that provider; absent fixture models also fail when fallback is disabled.
- User impact: deterministic local API/TUI/GUI acceptance can send prompts to a real provider and incur cost.
- Constraints: preserve catalog metadata, tools, pricing, and normal non-fake routing; no credential or catalog redesign.

## Scope

- In scope: an assembly-owned `RunnerConfig` routing flag which makes an explicit fake override select the runner's fake provider before registry lookup and prevents registry fallback candidates; daemon-assembly regressions for catalog-known and absent fixture models with an explicit OpenAI fallback request.
- Out of scope: disabling catalogs, changing provider credentials, altering ordinary explicit-provider or normal catalog behavior, and persisted migrations.

## Documentation Contract

- Feature status: `implemented`
- Public docs affected: None; this is an operator safety clarification in the local deterministic acceptance runbook.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: engineering, observational, system, and long-term logs.

## Test Plan (TDD)

- New failing tests to add first: daemon assembly starts with `HARNESS_PROVIDER=fake`, a catalog and configured OpenAI client; both `gpt-4.1-mini` and absent `fake-model` complete with `allow_fallback=true` and `fallback_providers:["openai"]`, report fake, and make zero real-client calls. A second daemon regression uses the concrete `fakeprovider.Provider` with a retryable failure and requires the run to fail locally without constructing the requested registry fallback.
- Existing tests to update: `buildRunnerConfig` assembly coverage and deterministic smoke operator note.
- Regression tests required: focused normal/race, `cmd/harnessd` package normal/race, and `./scripts/test-regression.sh`.

## Cross-Surface Impact Map

See `2026-08-04-issue-1162-fake-provider-routing-impact-map.md`.

## Implementation Checklist

- [x] Verify structured issue #1162 and current origin/main search evidence.
- [x] Record plan and cross-surface impact analysis before code.
- [x] Add and preserve failing daemon-assembly regressions.
- [x] Add the minimal authoritative fake-routing flag from harnessd assembly through `RunnerConfig`.
- [x] Prove catalog metadata remains available and the real client is not called.
- [x] Verify normal non-fake catalog routing remains covered.
- [x] Update logs, runbook, and indexes.
- [x] Run focused normal/race and full regression.

## Risks and Mitigations

- Risk: the override accidentally disables model catalog metadata. Mitigation: retain the registry and assert `/v1/models` remains catalog-backed.
- Risk: a request-specified provider bypasses deterministic safety. Mitigation: make the explicit fake mode win before preferred-provider and model resolution.
- Risk: config reload loses the mode. Mitigation: carry it through the existing shared runner-config assembly options.
