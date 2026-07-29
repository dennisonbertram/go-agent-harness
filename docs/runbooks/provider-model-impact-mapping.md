# Provider/Model Impact Mapping Runbook

## Policy

Before implementation starts, any task that touches provider/model flows must
include the general one-page cross-surface impact map. Provider/model work must
explicitly cover config, server API, CLI/tools, TUI and other clients, routing
and catalogs, credentials/security, persistence, deployment, observability,
compatibility, and regression tests.

This requirement applies to feature work and bugfixes that change provider selection, model routing, gateway behavior, API-key management, model catalogs, or server/client provider plumbing.

## Why This Exists

Recent feature history showed a repeated pattern: the core feature landed first, then follow-up commits were needed for adjacent wiring, navigation, or regression coverage. The impact map forces those surfaces to be checked before merge.

## Required Artifact

1. Copy [`docs/plans/IMPACT_MAP_TEMPLATE.md`](../plans/IMPACT_MAP_TEMPLATE.md) to a task-specific file in `docs/plans/`.
2. Link the impact map from the task plan.
3. Fill every heading before implementation starts.
4. Update the impact map if the design changes during implementation.

## Rules

- Keep the artifact to roughly one page.
- Do not leave any required heading blank.
- If a surface is truly unaffected, write `None` with a short justification.
- Treat a blank heading as a warning that the change surface is probably incomplete.

## Review Checklist

- `Ownership/data flow`: Did you trace config through registry/routing to every caller?
- `Config/API/CLI`: Did you check env vars, defaults, request/response shapes, validation, and tools?
- `Clients`: Did you check TUI, macOS, web, automation, saved state, and keyboard flow?
- `Security/operations`: Did you check credentials, logging, deployment, observability, and rollback?
- `Regression tests`: Did you name the acceptance and cross-surface coverage needed to keep the feature from drifting?
