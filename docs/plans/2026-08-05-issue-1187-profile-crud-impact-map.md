# Cross-Surface Impact Map: Issue #1187 profile CRUD

## Task

- Task / issue: #1187 isolated harnessd profile CRUD.
- Plan link: `2026-08-05-issue-1187-profile-crud-plan.md`.
- Owner: Codex.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `runWithSignals` resolves startup environment; `buildHTTPRuntime`
  creates the registry/runner/server; `/v1/profiles/{name}` and deferred profile
  tools consume the paths.
- Owning packages/types/functions and source of truth: `cmd/harnessd/main.go`,
  `cmd/harnessd/runtime_container.go`, `cmd/harnessd/bootstrap_helpers.go`,
  `harness.DefaultRegistryOptions`, `harness.RunnerConfig`, and
  `server.ServerOptions`.
- Callers, consumers, events, and downstream data: agent catalog discovery,
  named-profile runner resolution, and profile HTTP CRUD share the same
  user-mutation directory; profile reads retain project > user > built-in.
- Similar abstractions searched: explicit dir APIs in `internal/profiles`,
  registry `ProfilesDir`, runner `ProfilesDir`, and server project/user/dir
  options.
- Search commands/evidence: `rg -n "ProfilesDir|ProfilesProject|ProfilesUser|HARNESS_PROFILES_DIR|create_profile" cmd internal docs`.
- Duplication/ownership conclusion: path resolution belongs once at daemon
  startup; all consumers receive derived immutable strings.

## Config, API, CLI, and Tools

- User-facing config added or changed: opt-in absolute `HARNESS_PROFILES_DIR`.
- Defaults / fallbacks: omitted override remains `~/.harness/profiles`; project
  reads remain `$HARNESS_WORKSPACE/.harness/profiles`.
- Environment variables, config files, or saved settings touched: startup env
  only; no persisted config/schema change.
- Endpoints, request fields, response fields, or server wiring affected:
  existing profile endpoints change from 501 to configured CRUD when startup
  wiring supplies the directory.
- CLI commands, tools, wire formats, or integrations affected: deferred
  create/update/delete profile tools become discoverable on the same registry.
- Error states / validation changes: relative override fails daemon startup;
  existing built-in, traversal, auth, and validation errors are preserved.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, or ownership changes: none;
  existing compatible TOML files and atomic writers are reused.
- Backward/forward compatibility and versioning: unset environment retains the
  existing path and behavior.
- Partial rollout and mixed-version behavior: opt-in variable is ignored by
  older binaries and can be removed to roll back new wiring.

## Lifecycle, Security, and Reliability

- Concurrency, cancellation, retries, cleanup, and resource ownership: path
  values are set before registry/runner/server construction; existing atomic
  save/delete owns file lifecycle.
- Authentication, authorization, permissions, trust, privacy, and secrets:
  existing `runs:write` endpoint authorization, built-in immutability, strict
  profile-name validation, and path containment remain authoritative.
- Failure modes, recovery, idempotency, and data repair: invalid startup path
  fails closed; no migration or recovery is required.

## Product and Integration Surfaces

- Server/runtime: profile routes, catalog, and named-profile run loading.
- TUI/web/macOS/other clients: no UI semantic change; clients can now use
  already-documented CRUD-backed APIs/tool discovery in configured daemon.
- Provider/model/tool catalog and routing: profile mutation tools are added to
  the runtime catalog only when the resolved user directory is configured.
- External systems and automation: none.
- UX states, keyboard/focus/accessibility/motion: none; no client view change.

## Deployment and Operations

- Deployment/migration order and feature flags: deploy binary, then opt into
  an absolute directory per environment; no migration.
- Logs, metrics, traces, alerts, and support diagnostics: startup error names
  invalid override; acceptance artifact records exact isolated path.
- Rollback triggers and recovery steps: unset variable or revert wiring; TOML
  files are retained and compatible.
- Runbooks and operator docs: profile authoring explains override and default.

## Regression Tests

- Characterization and first expected red test: harnessd real listener test
  requires configured CRUD + runtime tool catalog and initially receives 501 /
  missing tools.
- New acceptance tests required: fake-provider multi-turn create/get/update/get/
  delete/not-found against one temp directory; no real home write.
- Edge, negative, failure, lifecycle, and security tests: relative override,
  default fallback, precedence, built-in protection, traversal, and existing
  atomic write tests.
- Integration/e2e/real-path proof: launch `runWithSignals` with an actual port
  zero listener, then exercise HTTP endpoints over TCP.
- Cross-surface regressions to guard: bootstrap forwards equal paths into
  registry, runner, and server.
- Exact targeted and full commands: `go test ./cmd/harnessd -run Profile -count=1`,
  the same with `-race`, and `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs before code: plan/impact map created before production edits.
- Implementation notes/logs/indexes after code: plan/log indexes and durable
  logs updated with red/green and rollout evidence.
- Training/onboarding/release notes: existing profile-authoring runbook records
  the opt-in isolation procedure.
