# Cross-Surface Impact Map: Issue #1169 bootstrap VCS provenance

## Task

- Task / issue: #1169 clean bootstrap worktrees inherit dirty parent VCS data.
- Plan link: `2026-08-04-issue-1169-bootstrap-vcs-provenance-plan.md`.
- Owner: `scripts/init.sh` bootstrap process.
- Status: implemented; pending independent review and merge.

## Current Ownership, Callers, and Data Flow

- Entry point: `scripts/init.sh <task>` fetches/creates a worktree from the
  fetched origin commit when origin exists, builds three local binaries, then
  emits `dev.env` consumed by acceptance and manual lanes.
- Source of truth: the target worktree's `git rev-parse` values and each
  executable's `go version -m`, not parent checkout state or emitted text.
- Search evidence: `rg -n "go build|buildvcs|GIT_DIR|GIT_WORK_TREE|go version -m"
  scripts internal docs` and the #1165 guard.
- Conclusion: bootstrap must supply target Git metadata to Go and independently
  compare generated metadata to the target's clean HEAD.

## Config, API, CLI, and Tools

- User-facing config: none. Existing flags and generated environment values
  remain unchanged.
- Environment: ambient Git routing variables are intentionally cleared; each
  `go build` gets target-specific `GIT_DIR`/`GIT_WORK_TREE` and `-buildvcs=true`.
- API, CLI protocol, tools, and error schemas: none. Script errors become an
  operator-visible nonzero bootstrap failure.

## Persistence and Compatibility

- No schema, migration, cache, product state, or wire compatibility change.
- Bootstrap outputs are ephemeral. Rejected candidate binaries are removed so
  a future launcher cannot mistake them for accepted artifacts.

## Lifecycle, Security, and Reliability

- Lifecycle: build -> executable build-info validation -> successful bootstrap;
  failures stop before `dev.env` success output or daemon launch.
- Security: correct provenance prevents accidental use of stale/dirty binaries
  that could dispatch a real provider. No secrets or credentials are read.
- Failure modes: missing Git state, stale local base after a successful fetch,
  a dirty target, unsupported/missing VCS metadata, or revision mismatch fails
  closed without a fallback binary.

## Product and Integration Surfaces

- Server/runtime: only the local binary supplied to `harnessd` launch changes.
- TUI/web/macOS: no source or behavior change; all consume the verified binary
  through existing bootstrap environment paths.
- Provider/model/tool catalog and external automation: unchanged; no route or
  credential change. The existing #1165 guard remains unchanged.
- UX/accessibility: none, searched native and CLI launch paths only.

## Deployment and Operations

- Rollout: land the bootstrap change before the next exact-SHA acceptance run.
- Diagnostics: errors name the expected and observed revision/dirty state.
- Rollback: revert the bootstrap helper; never bypass a provenance rejection.
- Runbook: `scripts/init.sh` remains the required worktree entry point.

## Regression Tests

- First red: real Go 1.26 linked child build inherits a dirty parent marker.
- New tests: clean child exact revision, external Git-environment isolation,
  and mismatched output deletion/rejection.
- Commands: `go test ./internal/acceptance/bootstrapprovenance -count=1`, race
  variant, then `./scripts/test-regression.sh` with isolated caches.

## Documentation and Handoff

- No public docs change. Add plan/map plus engineering, observational, system,
  and long-term intent entries; update plan/log indexes and issue evidence.
