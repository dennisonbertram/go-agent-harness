# Plan: Issue #987 issue-driven engineering workflow

## Context

- Problem: The repository's Markdown issue templates are advisory and a pull
  request can currently merge without proving that the implementation was
  scoped from a structured issue or that adjacent surfaces were inspected.
- User impact: Unscoped agent changes can duplicate architecture, miss callers
  and client surfaces, create inconsistent behavior, and accumulate avoidable
  technical debt.
- Governing issue: [#987](https://github.com/dennisonbertram/go-code/issues/987)
- Constraints:
  - Keep confidential vulnerability reports out of public issues.
  - Minor work remains issue-driven and is limited to small documentation-only
    changes that cannot alter runtime behavior.
  - Per user direction, pilot the process through Claude/agent instructions and
    review before adding GitHub branch protection or a required status check.

## Scope

- In scope:
  - Required Issue Forms for engineering changes, bugs, epics, research, and
    minor documentation changes.
  - A pull request template that carries issue acceptance criteria, impact
    analysis, TDD evidence, and rollout evidence through review.
  - Agent, planning, triage, testing, and worktree policy updates.
  - Repository drift tests for the forms, PR template, private-security route,
    and agent rules.
- Out of scope:
  - Runtime harness behavior.
  - Rewriting the existing backlog.
  - Treating form completion as a substitute for code review.
  - Branch protection, a required GitHub status check, or live PR-content
    validation during this pilot.

## Documentation Contract

- Feature status: `implemented`
- Public docs affected: none.
- Spec docs to update before code:
  - this plan
  - `docs/runbooks/issue-driven-development.md`
  - `docs/runbooks/issue-triage.md`
- Implementation notes to add after code:
  - `docs/logs/engineering-log.md`
  - `docs/logs/system-log.md`
  - relevant folder indexes

## Test Plan (TDD)

- New failing tests to add first:
  - exactly five YAML Issue Forms and no legacy Markdown templates;
  - valid YAML, stable unique IDs, required fields, exact Work type markers,
    and exhaustive contract labels for each form;
  - blank-issue disablement and private vulnerability routing;
  - PR template scope, impact, architecture, TDD, verification, rollout,
    documentation, closing-link, and checklist sections;
  - matching issue-first policy language in `AGENTS.md` and `CLAUDE.md`.
- Existing tests to update: none expected.
- Regression tests required:
  - `go test ./internal/quality/repostructure -count=1`
  - `go test ./internal/... ./cmd/...`
  - `make test-e2e`
  - `python3 scripts/test_terminal_bench_artifacts.py`
  - `./scripts/test-regression.sh`

## Cross-Surface Impact Map

- Call sites and data flow:
  - GitHub issue selector -> required Issue Form -> issue body -> agent plan and
    implementation -> PR template reconciliation -> review.
  - Repository process test -> form/PR/agent policy files -> drift failure.
- Config, API, and CLI: no harness runtime config, API, CLI, or GitHub API
  automation changes.
- Persistence and schemas: no product persistence or schema changes.
- Concurrency and lifecycle: none.
- Security and privacy:
  - route confidential reports to GitHub private vulnerability reporting;
  - never put credentials, exploit details, or private user data into public
    Issue Forms.
- Product clients and UI: no TUI, server, API, web, macOS, or provider/model
  behavior changes.
- Deployment, observability, and rollback:
  - no deployed product behavior changes;
  - observe Claude/agent compliance in subsequent issues and PRs;
  - rollback is a normal revert of the templates/instructions if the process is
    counterproductive.
- Compatibility:
  - legacy issues do not satisfy the new contract until their bodies are
    upgraded;
  - epics cannot be the sole shipping issue for a PR;
  - research PRs must remain documentation-only;
  - minor PRs must remain documentation-only, touch at most five files, and
    change at most 150 lines.

## Implementation Checklist

- [x] Create governing Issue #987 before implementation.
- [x] Define acceptance criteria and the full impact map.
- [x] Add repository process tests and confirm the expected red.
- [x] Replace Markdown templates with five required Issue Forms.
- [x] Add the exhaustive pull request template.
- [x] Update agent rules and contributor runbooks.
- [x] Update logs and indexes.
- [x] Run targeted, fast, end-to-end, and full regression verification.
- [ ] Open a PR that closes #987 and reconcile it against the new template.
- [ ] Squash-merge after existing CI and local regression are green.

## Risks and Mitigations

- Risk: The minor template becomes a loophole.
  - Mitigation: define documentation-only paths and file/line caps in the form,
    agent rules, PR checklist, and review runbook; any prohibited surface uses
    the full change or bug form.
- Risk: Claude ignores policy because GitHub does not block the merge.
  - Mitigation: repeat the stop condition in both agent entrypoints, pin it with
    repository tests, and observe real follow-on work before deciding whether a
    technical gate is warranted.
- Risk: Required forms encourage checkbox compliance without engineering
  judgment.
  - Mitigation: require concrete call-site/search evidence, unaffected-surface
    rationales, exact red/green commands, scope reconciliation, and human review.
