# Provider/Model Impact Map Template

Use this template before landing any feature or bugfix that changes provider selection, model routing, gateway behavior, API-key provisioning, model catalogs, or server/client provider plumbing.

Keep the artifact to roughly one page and store it in `docs/plans/` with the task name. Link it from the task plan.

## Task

- Task / issue:
- Plan link:
- Owner:
- Status:

## Config

- User-facing config added or changed:
- Defaults / fallbacks:
- Environment variables, config files, or saved settings touched:
- Migration / backward-compatibility notes:

## Server API

- Endpoints, request fields, response fields, or server wiring affected:
- Provider/model resolution or registry changes:
- Error states / validation changes:

## TUI State

- Slash commands, overlays, selection state, routing, or status bar changes:
- Persisted client state or local config changes:
- Keyboard/navigation implications:

## Regression Tests

- New acceptance tests required:
- Existing tests to update:
- Cross-surface regressions to guard:

## Warning Check

- A blank heading is a warning that the integration surface may be under-mapped.
- If a section truly has no impact, write `None` and explain why instead of leaving it blank.
