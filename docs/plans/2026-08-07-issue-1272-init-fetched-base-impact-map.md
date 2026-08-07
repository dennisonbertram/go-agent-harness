# Cross-Surface Impact Map: Issue #1272

## Task

- Task / issue: #1272 fetched `scripts/init.sh` base resolution.
- Plan link: `2026-08-07-issue-1272-init-fetched-base-plan.md`.
- Owner: bootstrap script and bootstrap-provenance acceptance package.
- Status: implemented locally; review/merge pending.

## Current Ownership, Callers, and Data Flow

- Entry point: `scripts/init.sh` parses `--base-ref`, creates/reuses a linked
  worktree, then writes `.tmp/bootstrap/dev.env`.
- Source of truth: Git remote-tracking refs and verified commit IDs; generated
  env exports carry source and actual worktree provenance to operators.
- Callers: agents/operators invoking `scripts/init.sh`; no runtime consumer
  changes. Search evidence: `rg -n "base-ref|HARNESS_BOOTSTRAP" scripts docs internal`.
- Similar abstraction: existing bootstrap metadata build verification remains
  authoritative and unchanged.

## Config, API, CLI, and Tools

- CLI behavior: existing `--base-ref` accepts the documented branch form plus
  exact SHA, `origin/<branch>`, and explicit `refs/heads/<branch>`.
- No API, endpoint, model/tool catalog, or saved setting changes.
- Error behavior is fail-closed for unknown SHA/ref resolution.

## Persistence and Compatibility

- No schema or migration. Generated dev env gains additive source/head
  provenance exports.
- Reuse remains compatible: an existing task branch is not reset to a newer
  source revision.

## Lifecycle, Security, and Reliability

- No concurrency ownership change; resolving `refs/remotes/origin/...` avoids
  reliance on shared mutable `FETCH_HEAD` during concurrent fetches.
- No auth, secrets, privacy, or permissions impact.
- A new worktree must match the selected immutable commit; reuse records both
  selected source and actual HEAD for diagnosis.

## Product and Integration Surfaces

- Server/runtime, TUI, web, macOS, provider/model routing: none; bootstrap
  builds the same binaries from the selected worktree revision.
- External system: Git `origin` fetch only, already owned by the bootstrap
  workflow.

## Deployment and Operations

- No deployment or feature flag. Operators source the existing env file and
  can inspect the two new revision exports.
- Rollback is reverting this script/test/documentation PR.

## Regression Tests

- Expected red: `go test ./internal/acceptance/bootstrapprovenance -run
  TestInitVerifiesReusedWorktreeAgainstFreshRemoteBaseAndAcceptsExplicitSources -count=1`
  accepted stale reuse before the change.
- New acceptance: default stale-local/newer-origin fixture, reuse provenance,
  explicit remote, SHA, and local ref exact-HEAD cases.
- Commands: focused package test; `./scripts/test-regression.sh` in tmux.

## Documentation and Handoff

- Plan, map, plans index, active plan, engineering log, long-term intent log,
  and logs index updated in this PR.
- PR must say `Closes #1272`; no merge in this slice.
