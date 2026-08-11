# Cross-Surface Impact Map: Issue #1165 runtime provenance

## Task

- Task / issue: #1165 acceptance runners can execute a stale dirty `harnessd` binary.
- Plan link: `2026-08-04-issue-1165-runtime-provenance-plan.md`.
- Owner: acceptance runtime.
- Status: implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `scripts/run-bench-smoke.sh` builds/accepts `harnessd`, then
  starts it and submits `POST /v1/runs`.
- Source of truth: executable `go version -m`, not checkout text or artifact
  naming; `git rev-parse HEAD` supplies the requested source revision.
- Callers/consumers: deterministic shell smoke and future acceptance launchers
  source one guard; run artifacts consume JSON provenance.
- Search evidence: `rg -n "go build|harnessd|go version -m|git rev-parse|sha256" scripts docs`.
- Conclusion: centralize validation in a sourceable script before launch.

## Config, API, CLI, and Tools

- Added operator-only controls: optional expected revision/artifact path inputs
  to the guard; no server environment, API, CLI wire format, or tool changes.
- Failure: nonzero exit with a provenance error before executable invocation.

## Persistence and Compatibility

- No schema, migration, cache, or product persistence changes.
- JSON evidence is additive and per acceptance artifact; older launchers remain
  unchanged until explicitly wired.

## Lifecycle, Security, and Reliability

- Validation precedes daemon process creation and provider initialization.
- Runtime metadata and digest are recorded only after validation, preventing
  stale/dirty evidence from looking accepted.
- No retries or fallback: mismatch requires an intentional clean rebuild.

## Product and Integration Surfaces

- Server/runtime: only acceptance process launch order changes.
- TUI/web/macOS: no client code change; their future acceptance runners can
  source the same guard.
- Provider/model/tool catalog: no routing change; the guard reduces accidental
  real-provider egress from stale acceptance executables.
- External systems/UX: none, searched launcher scripts only.

## Deployment and Operations

- Rollout: source the guard in the deterministic smoke launcher first.
- Diagnostics: retain raw `go version -m` and SHA-256 in a JSON artifact.
- Rollback: revert the launcher wiring/guard; never bypass a rejection in-place.
- Runbook: document required clean exact revision and rejection semantics.

## Regression Tests

- First red test: dirty/mismatched fake build info yields no daemon marker.
- Acceptance: clean matching fixture records full build info and SHA-256 before
  controlled daemon invocation.
- Commands: `go test ./internal/acceptance/runtimeprovenance -v`, race variant,
  then `./scripts/test-regression.sh`.

## Documentation and Handoff

- Update plan/log indexes, acceptance runbook, and engineering/observational/
  system/long-term logs after implementation.
