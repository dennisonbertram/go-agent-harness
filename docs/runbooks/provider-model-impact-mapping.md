# Provider/Model Impact Mapping Runbook

## Policy

Before implementation starts, any task that touches provider/model flows must include a one-page impact map covering:

- `Config`
- `Server API`
- `TUI state`
- `Regression tests`

This requirement applies to feature work and bugfixes that change provider selection, model routing, gateway behavior, API-key management, model catalogs, or server/client provider plumbing.

## Why This Exists

Recent feature history showed a repeated pattern: the core feature landed first, then follow-up commits were needed for adjacent wiring, navigation, or regression coverage. The impact map forces those surfaces to be checked before merge.

## Required Artifact

1. Copy [`docs/plans/IMPACT_MAP_TEMPLATE.md`](../plans/IMPACT_MAP_TEMPLATE.md) to a task-specific file in `docs/plans/`.
2. Link the impact map from the task plan.
3. Fill all four headings before implementation starts.
4. Update the impact map if the design changes during implementation.

## Rules

- Keep the artifact to roughly one page.
- Do not leave any required heading blank.
- If a surface is truly unaffected, write `None` with a short justification.
- Treat a blank heading as a warning that the change surface is probably incomplete.

## Review Checklist

- `Config`: Did you check env vars, config files, defaults, and persistence?
- `Server API`: Did you check endpoints, request/response shapes, validation, and provider wiring?
- `TUI state`: Did you check overlays, routing, status indicators, saved state, and keyboard flow?
- `Regression tests`: Did you name the acceptance/regression coverage needed to keep the feature from drifting?
