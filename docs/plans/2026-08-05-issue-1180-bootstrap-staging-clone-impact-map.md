# Cross-Surface Impact Map: Issue #1180 bootstrap staging clone

## Task

- Task / issue: #1180 linked-worktree VCS bootstrap provenance.
- Plan link: `2026-08-05-issue-1180-bootstrap-staging-clone-plan.md`.
- Owner: GoCode engineering.
- Status: implemented pending full gate and review.

## Current Ownership, Callers, and Data Flow

- Entry point: `scripts/init.sh` creates a target linked worktree, checks
  target Git state, builds `harnessd`, `harnesscli`, and `coveragegate`, then
  writes `dev.env`.
- Source of truth: target `git rev-parse HEAD` and clean target status; Go
  binary `vcs.*` settings are the independent published-artifact evidence.
- Callers: agents and operators run `scripts/init.sh`; acceptance coverage is
  `internal/acceptance/bootstrapprovenance`.
- Search evidence: `rg -n "buildvcs|bootstrap provenance|bootstrap_build_binary"
  scripts internal/acceptance` identified no other bootstrap publisher.
- Conclusion: an owned ephemeral clone is the smallest reliable bridge between
  Git linked-worktree semantics and Go buildvcs discovery.

## Config, API, CLI, and Tools

- User-facing config: unchanged flags and generated environment variables.
- Defaults/fallbacks: unchanged; no new fallback accepts absent metadata.
- API, server, TUI, macOS, tools: none; the build source directory changes
  before binaries are launched.
- Errors: staging creation/clone/checkout/cleanliness failures are explicit
  initializer failures, preserving fail-closed behavior.

## Persistence and Compatibility

- Schemas and stored state: none.
- Compatibility: output binary paths and metadata contract unchanged.
- Mixed versions: safe; each initializer invocation is self-contained.

## Lifecycle, Security, and Reliability

- Lifecycle: target HEAD/clean state is verified before clone; clone is
  detached at that SHA, checked clean, and removed through EXIT cleanup.
- Security: ambient Git variables remain unset; the compiler runs with all
  Git-discovery overrides removed. No secrets or credentials are added.
- Reliability: candidate files remain in target build dir, are validated with
  `go version -m`, and rename atomically only after clean revision validation.
  Clone or validation failure leaves rejected outputs removed.

## Product and Integration Surfaces

- Server/runtime: binaries have the same exact target revision evidence.
- TUI/web/macOS: no source or wire change; their launchers continue to use the
  generated binary paths.
- Provider/model/tool catalog and routing: none.
- External systems: local Git only; clone deliberately uses no network remote.

## Deployment and Operations

- Deployment order: merge script and test together; no migration or flag.
- Diagnostics: existing explicit `[init] ERROR` messages identify staging or
  artifact metadata rejection.
- Rollback: revert script change; temporary owned staging paths are disposable.
- Runbook: canonical `scripts/init.sh` remains the only bootstrap entrypoint.

## Regression Tests

- Characterization/red: linked target must not be the compiler CWD; fake Go
  accepts a candidate only if the CWD has directory-form `.git`.
- Acceptance: clean target, fetched origin, external Git env isolation, and
  mismatch/rejected artifact paths remain covered.
- Negatives: clone revision/clean state must match; fake wrong metadata still
  fails closed and leaves no harnessd artifact.
- Exact commands: `TMPDIR=/private/tmp go test ./internal/acceptance/bootstrapprovenance -count=1`,
  race equivalent, canonical fresh `scripts/init.sh`, then
  `./scripts/test-regression.sh` after #1175 restores the shared baseline.

## Documentation and Handoff

- Public docs: none.
- Durable records: plan, impact map, active plan, four logs, and indexes.
- Handoff: review the clone source, detached SHA/clean checks, candidate
  validation, and cleanup trap; do not accept a metadata bypass.

## Warning Check

All headings are mapped. Unaffected product surfaces are explicitly none with
rationale above.
