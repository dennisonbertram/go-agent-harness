# Cross-Surface Impact Map Template

Use this template before implementing complex features, bugs, refactors,
architecture, infrastructure, dependency, security, persistence, API, UI, or
operational changes. Provider/model changes always require it.

Keep the artifact to roughly one page and store it in `docs/plans/` with the task name. Link it from the task plan.

## Task

- Task / issue:
- Plan link:
- Owner:
- Status:

## Current Ownership, Callers, and Data Flow

- Entry points:
- Owning packages/types/functions and source of truth:
- Callers, consumers, events, and downstream data:
- Similar abstractions searched:
- Search commands/evidence:
- Duplication/ownership conclusion:

## Config, API, CLI, and Tools

- User-facing config added or changed:
- Defaults / fallbacks:
- Environment variables, config files, or saved settings touched:
- Endpoints, request fields, response fields, or server wiring affected:
- CLI commands, tools, wire formats, or integrations affected:
- Error states / validation changes:

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, or ownership changes:
- Backward/forward compatibility and versioning:
- Partial rollout and mixed-version behavior:

## Lifecycle, Security, and Reliability

- Concurrency, cancellation, retries, cleanup, and resource ownership:
- Authentication, authorization, permissions, trust, privacy, and secrets:
- Failure modes, recovery, idempotency, and data repair:

## Product and Integration Surfaces

- Server/runtime:
- TUI/web/macOS/other clients:
- Provider/model/tool catalog and routing:
- External systems and automation:
- UX states, keyboard/focus/accessibility/motion:

## Deployment and Operations

- Deployment/migration order and feature flags:
- Logs, metrics, traces, alerts, and support diagnostics:
- Rollback triggers and recovery steps:
- Runbooks and operator docs:

## Regression Tests

- Characterization and first expected red test:
- New acceptance tests required:
- Edge, negative, failure, lifecycle, and security tests:
- Integration/e2e/real-path proof:
- Cross-surface regressions to guard:
- Exact targeted and full commands:

## Documentation and Handoff

- Specs/public docs before code:
- Implementation notes/logs/indexes after code:
- Training/onboarding/release notes:

## Warning Check

- A blank heading is a warning that the integration surface may be under-mapped.
- If a section truly has no impact, write `None` and explain why instead of leaving it blank.
