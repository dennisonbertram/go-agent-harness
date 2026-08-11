# Plan: Issue #1188 selected TUI profile policy

## Context

- Governing GitHub issue: #1188.
- Problem: ordinary TUI runs transmit `profile`, but Runner currently consumes it only for workspace isolation and MCP setup. The picker therefore advertises controls it does not enforce.
- User impact: selecting a restricted profile must change the very next ordinary conversation turn, without changing startup or subagent profile semantics.
- Constraints: explicit request model/budget/prompt remains authoritative; profile safety restrictions must not be widened by a request. No profile CRUD or schema redesign.

## Scope

- In scope: compose a selected profile into ordinary `Runner.StartRun` requests: model, budgets, prompt, tool allowlist, profile permission restrictions, workspace isolation, and profile MCP. Preserve request precedence only where it cannot widen a profile restriction.
- Out of scope: profile CRUD (#1187), startup profile composition, subagent request composition, and command-level `allowed_commands` enforcement. The authoring runbook explicitly labels that existing parsed field unsupported for ordinary selected runs rather than claiming a security control.

## Documentation Contract

- Feature status: implemented locally; full regression in progress.
- Public docs affected: profile authoring precedence semantics.
- Spec docs to update before code: this plan and impact map.
- Implementation notes after code: four durable logs and indexes.

## Test Plan (TDD)

- First red: selected profile defaults are present in ordinary run state/provider request, while a request override wins only for non-safety fields.
- Regression tests: profile tool allowlist and denied tool categories survive a second same-conversation turn; explicit broad tool input is intersected with the selected profile; startup/subagent coverage remains unchanged.
- Acceptance: fake-provider TUI/API multi-turn run selects a user profile, receives two normal messages in one conversation, and confirms the provider never sees a blocked tool.

## Cross-Surface Impact Map

See `2026-08-05-issue-1188-selected-profile-policy-impact-map.md`.

## Implementation Checklist

- [x] Verify #1188 architecture evidence and issue contract.
- [x] Record plan and impact map before source changes.
- [x] Add expected-red ordinary profile policy tests.
- [x] Add the minimal profile-to-request composition boundary.
- [x] Add fake-provider/TUI multi-turn acceptance evidence.
- [x] Update authoring semantics, logs, and indexes.
- [ ] Run targeted normal/race and full regression with `TMPDIR=/private/tmp`.

## Risks and Mitigations

- Risk: an explicit request broadens a selected profile or a newly registered tool evades a stale name denylist. Mitigation: treat profile allow/deny/permission restrictions as upper bounds and enforce file/network policy by registry action at offer and dispatch, with actual side-effect tests.
- Risk: profile loading differs from existing isolation/MCP paths. Mitigation: reuse `RunnerConfig.ProfilesDir` and the existing profile loader.
- Rollout: applies only when `profile` is explicitly named. Rollback is reverting this composition; no stored data/migration is involved.
