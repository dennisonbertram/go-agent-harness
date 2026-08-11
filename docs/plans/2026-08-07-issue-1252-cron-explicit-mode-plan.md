# Plan: Issue #1252 explicit model-facing cron execution mode

## Context

- Governing GitHub issue: #1252.
- Problem: `cron_create` silently treats omitted `execution_type` as `shell`, so conversational scheduling can produce a successful headless job with no child conversation run.
- User impact: an agent must explicitly select either headless shell work or a visible conversation continuation.
- Constraints: preserve persisted shell jobs and REST/operator APIs; do not coerce commands into prompts or alter callback behavior.

## Scope

- In scope: the model-facing `cron_create` schema, handler validation, description, deterministic core/embedded tests, and exact-source PTY evidence when feasible.
- Out of scope: cron persistence/migrations, REST APIs, operator CLI, existing job behavior, callback behavior, and native GUI implementation.

## Documentation Contract

- Feature status: implemented; final full-regression/PTY evidence pending.
- Public docs affected: embedded `cron_create` model-facing description only.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: engineering log and plan index.

## Test Plan (TDD)

- New failing tests to add first: omitted `execution_type` is rejected and the schema requires it.
- Existing tests to update: explicit shell test inputs that deliberately exercise model-facing `cron_create`.
- Regression tests required: explicit shell retains command/output/no child run; explicit harness retains scoped same-conversation child run and assistant output.

## Implementation Checklist

- [x] Link a contract-complete GitHub issue before implementation.
- [x] Record ownership/search evidence and cross-surface map.
- [x] Capture focused red test evidence.
- [x] Require `execution_type` in schema and handler.
- [x] Update explicit-shell callers and model-facing description.
- [ ] Verify core, embedded, PTY, and full regression evidence.
- [x] Update logs/indexes; open one PR that closes #1252 after final verification.

## Risks and Mitigations

- Risk: accidental REST or persisted-job compatibility break. Mitigation: change only the deferred model-tool boundary; prove explicit shell and existing embedded job paths remain unchanged.
- Risk: a scheduler fire is mistaken for a visible conversation continuation. Mitigation: require nonempty harness child run ID, same conversation scope, assistant output, and PTY rendered/post-fire proof.
