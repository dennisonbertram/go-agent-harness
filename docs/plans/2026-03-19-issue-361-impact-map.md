# Provider/Model Impact Map

## Task

- Task / issue: `#361` golden-path deployment profile and smoke suite
- Plan link: `docs/plans/2026-03-19-issue-361-golden-path-smoke-plan.md`
- Owner: Codex
- Status: in progress

## Config

- User-facing config added or changed:
  - `harnessd --profile full` must resolve to a real built-in config profile in-repo.
  - Golden-path smoke env contract explicitly enables run and conversation persistence.
- Defaults / fallbacks:
  - Built-in config profile fallback is used when a named profile is not found in the user profile directory.
  - Smoke path remains overridable through `HARNESS_SMOKE_*` env vars.
- Environment variables, config files, or saved settings touched:
  - `HARNESS_RUN_DB`
  - `HARNESS_CONVERSATION_DB`
  - `HARNESS_WORKSPACE`
  - `HARNESS_MODEL_CATALOG_PATH`
  - provider key env vars already supported after `#362`
- Migration / backward-compatibility notes:
  - Existing user-local profiles still win; builtin fallback only fixes the broken documented `full` path when no local file exists.

## Server API

- Endpoints, request fields, response fields, or server wiring affected:
  - Smoke/regression path covers `/healthz`, `/v1/providers`, `/v1/models`, `/v1/runs`, `/v1/runs/{id}`, `/v1/runs/{id}/events`, and persistence-backed readback endpoints.
- Provider/model resolution or registry changes:
  - None to runtime provider routing; this issue consumes the `#362` bootstrap behavior.
- Error states / validation changes:
  - Named profile startup should no longer fail for the documented builtin `full` path when no user-local profile file is present.

## TUI State

- Slash commands, overlays, selection state, routing, or status bar changes:
  - None, harness-only work.
- Persisted client state or local config changes:
  - None.
- Keyboard/navigation implications:
  - None.

## Regression Tests

- New acceptance tests required:
  - Builtin config profile fallback test for `full`.
  - Harness startup integration covering `--profile full` with run and conversation persistence enabled.
- Existing tests to update:
  - Smoke contract/docs and any tests assuming named profiles only exist in `~/.harness/profiles`.
- Cross-surface regressions to guard:
  - Smoke script must not regress back to a startup path that cannot resolve its own profile.
  - Persistence-backed run/conversation readback must remain wired in the documented golden path.

## Warning Check

- No blank sections remain. TUI state is intentionally `None` because this issue does not touch client surfaces.
