# Critical Context

## Project Intent

Build an MVP with strong engineering discipline: strict TDD, security best practices, DRY code, and clear operational documentation.

## Working Model

- Every change starts with a structured GitHub issue, then a documented plan and
  checklist linked to that issue.
- Every non-minor change starts with a cross-surface impact map covering current
  ownership/callers, data flow, config/API/CLI, persistence, lifecycle,
  security, product clients, provider/model/tool catalogs, deployment,
  compatibility, tests, and documentation. Unaffected surfaces require a
  searched, explicit rationale.
- Minor work still uses an issue and PR and is strictly documentation-only.
- Every implementation is test-first.
- Every bug gets a regression test and log entry.
- Every doc folder is indexed and maintained.
- When uncertain, default decisions to command intent and user intent from `docs/logs/long-term-thinking-log.md`.

## Contributor Guidance

- Use plain-language summaries when communicating to non-specialists.
- Keep technical detail in code, tests, and internal docs.
- Keep onboarding artifacts current so new agents can execute quickly.
