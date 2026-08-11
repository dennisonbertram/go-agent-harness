# Cross-Surface Impact Map: Issue #1198 skill-directory isolation

## Task

- Task / issue: #1198, honor isolated `HARNESS_SKILLS_DIR` across harnessd.
- Plan link: `2026-08-05-issue-1198-skills-dir-plan.md`.
- Owner: harnessd runtime.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry point: `cmd/harnessd/main.go:runWithSignalsWithDeps` resolves global
  startup paths and builds loader, watcher, registry, and HTTP runtime.
- Source of truth: loader `GlobalDir`, registry `SkillsDir`, watcher global
  watched directory, and `goWorkflowSkillDirs` currently independently derive
  `filepath.Join(globalDir, "skills")`.
- Consumers: `/v1/skills`, `skill`/`create_skill`/`verify_skill`, watcher
  reload, Go workflow skill-bundle discovery; plugin and workspace skill dirs
  remain independent.
- Search evidence: `rg "GlobalDir:.*skills|SkillsDir:|globalSkillsDir" cmd/harnessd internal/harness`.
- Conclusion: one resolved `globalSkillsDir` must replace every global-root
  derivation, avoiding partial isolated execution.

## Config, API, CLI, and Tools

- Config: `HARNESS_SKILLS_DIR` gains its actual documented contract: trimmed,
  absolute override only.
- Defaults: unset/blank falls back to `$HARNESS_GLOBAL_DIR/skills`.
- API/tools: existing GET/verify routes and create/verify/skill tool schemas
  are unchanged; their backing directory changes consistently.
- Error state: relative override returns startup error before listener startup.

## Persistence and Compatibility

- No schemas, migrations, or cached-format changes.
- Existing deployments without the variable retain path and precedence.
- Mixed versions: only the new binary honors the override; rollout should
  restart all relevant daemon instances together where isolation matters.

## Lifecycle, Security, and Reliability

- Watcher and loader share one immutable path for process lifetime; reload is
  still registry-owned.
- Security: reject relative input rather than rebasing it, preventing CWD
  dependent writes; tests prove no default-global/home write under override.
- Failure/recovery: invalid startup leaves no listener; unset rollback restores
  legacy root without data conversion.

## Product and Integration Surfaces

- Server/runtime: affected as above.
- TUI: `loadTUISkills` has a direct local slash-command catalog consumer and
  now mirrors the override/fail-closed contract; macOS has no direct skill-path
  handling and continues through API behavior.
- Provider/model: none; fake-provider is acceptance-only.
- External automation: Go workflow skill directories now match authored skill
  root; plugin/workspace directories are unchanged.
- UX: existing responses/errors unchanged except fail-closed startup diagnostic.

## Deployment and Operations

- Deploy by setting an absolute path before daemon start; verify `/v1/skills`
  and watcher reload against that path.
- Logs: startup error identifies invalid setting through wrapping; watcher logs
  retain existing wording.
- Rollback: unset variable/revert wiring; no cleanup or migration required.
- Runbooks: environment, skills, and workflow path references updated.

## Regression Tests

- First red: absolute override skill is missing from `/v1/skills` on current
  main; relative override currently starts instead of failing.
- Acceptance: fake-provider multi-message SSE creates, lists, verifies, and
  watches a temporary override skill without default-root write.
- Edge cases: whitespace trimming, unset fallback, invalid relative startup,
  watcher-created skill, registry/tool catalog availability.
- Commands: focused normal/race `go test ./cmd/harnessd`; then
  `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Documentation and Handoff

- Public docs: environment reference, skills integration/concepts, workflow SDK.
- Durable logs/indexes: engineering, observational, system, long-term-thinking.
- No training or release-note artifact beyond PR acceptance evidence.
