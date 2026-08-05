# Plan: Issue #1180 clean linked-worktree bootstrap provenance

## Context

- Governing GitHub issue: #1180.
- Problem: Go 1.26.4 emits no VCS metadata when `go build -buildvcs=true` runs
  in a linked worktree whose `.git` is an indirection file, causing the
  existing fail-closed bootstrap guard to reject a clean fresh worktree.
- User impact: the canonical `scripts/init.sh` workflow cannot create
  verifiable local harness binaries, blocking issue-scoped implementation and
  end-to-end validation.
- Constraints: preserve target cleanliness and exact-revision checks; do not
  synthesize metadata, accept missing metadata, or alter cron/callback paths.

## Scope

- In scope: create and clean up an ephemeral local clone at the already
  verified target revision; compile there; retain candidate validation and
  atomic publication into the target build directory; regressions and durable
  process documentation.
- Out of scope: changing Go, disabling `-buildvcs`, modifying Git repository
  state, runtime task behavior, API/TUI/macOS clients, or scheduled work.

## Documentation Contract

- Feature status: implemented pending review and hosted verification.
- Public docs affected: none; this is an internal developer bootstrap contract.
- Spec docs to update before code: this plan and impact map.
- Implementation notes to add after code: four durable logs and folder indexes.

## Test Plan (TDD)

- New failing test: a linked-worktree fixture provides a fake Go compiler that
  accepts only when the build CWD has a directory-form `.git`; the old target
  build fails it, while an isolated clone passes it.
- Existing tests: clean linked target metadata, fetched `origin/main`,
  inherited external Git environment, and mismatched metadata rejection.
- Regression tests: focused normal/race bootstrap acceptance, real fresh
  initializer proof, and repository regression after the known independent
  macOS temporary-path baseline is repaired.

## Cross-Surface Impact Map

See `2026-08-05-issue-1180-bootstrap-staging-clone-impact-map.md`.

## Implementation Checklist

- [x] Record exact clean target preconditions before staging.
- [x] Add a red linked-worktree directory-form `.git` regression.
- [x] Build from an exact-revision clean staging clone with Git environment removed.
- [x] Preserve candidate metadata validation and atomic target publication.
- [x] Ensure staging cleanup on success and failure.
- [x] Update durable logs and indexes.
- [x] Run focused normal/race tests.
- [x] Run accepted full regression with the documented canonical temporary-directory isolation.
- [ ] Receive independent cheap review, push PR, and merge after checks pass.

## Risks and Mitigations

- Risk: staging clone compiles a different revision. Mitigation: target HEAD
  and clean status are checked first; clone is detached at the target SHA and
  checked clean before any build.
- Risk: unverified output is exposed. Mitigation: every package builds to a
  candidate, validates `go version -m`, and only then renames atomically.
- Risk: leaked temporary repository. Mitigation: one owned `mktemp` directory
  under target `.tmp/bootstrap` is removed through an EXIT trap.
- Rollout: use canonical `scripts/init.sh`; no migration or flag. Roll back by
  reverting the script and deleting only newly created staging directories;
  rejected candidates remain removed by the existing guard.
