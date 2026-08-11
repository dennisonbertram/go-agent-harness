# Plan: Issue #1169 bootstrap VCS provenance

## Context

- Governing GitHub issue: #1169.
- Problem: Go 1.26 can follow a linked worktree's `.git` indirection to the
  dirty parent checkout and stamp a clean child binary with the wrong revision
  and `vcs.modified=true`.
- User impact: the #1165 acceptance guard correctly blocks that binary, but a
  fresh bootstrap cannot produce a usable, trustworthy runtime.
- Constraints: preserve #1165's fail-closed guard; never synthesize metadata,
  weaken `-buildvcs`, or alter scheduler, provider, API, persistence, or UI
  behavior.

## Scope

- In scope: `scripts/init.sh` Git-environment isolation, target-worktree VCS
  build context, exact post-build validation, rejected-artifact cleanup, and
  deterministic linked-worktree coverage.
- Out of scope: automatic acceptance launch/retry, `harnessd` runtime behavior,
  cron/callback semantics, provider configuration, and client UI behavior.

## Documentation Contract

- Feature status: implemented; pending independent review and merge.
- Public docs affected: none; this is an operator/bootstrap contract.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: all durable logs and indexes.

## Test Plan (TDD)

- First red: a clean disposable linked child, with a dirty parent, builds a
  `harnessd` carrying `vcs.modified=true` under the old bootstrap.
- New tests: child revision/clean-state success; default `main` resolves the
  fetched `origin/main` rather than stale local `main` (with `main` explicitly
  checked out in the bare-origin fixture); inherited external Git
  environment cannot redirect bootstrap; missing/mismatched build metadata
  rejects and removes the candidate executable.
- Regression tests: focused normal/race and `./scripts/test-regression.sh`.

## Implementation Checklist

- [x] Create a contract-complete issue and inspect the existing guard.
- [x] Map bootstrap ownership and write deterministic red coverage.
- [x] Resolve an origin-backed base to the fetched commit and pin every
  bootstrap build to the child `GIT_DIR` and `GIT_WORK_TREE` with explicit
  `-buildvcs=true`.
- [x] Validate each output using its own `go version -m`; reject/remove an
  absent, dirty, or mismatched output.
- [x] Update plans, logs, and indexes.
- [x] Run focused normal/race and the complete regression gate.
- [ ] Commit, push one closing PR, and obtain independent review.

## Risks and Mitigations

- Risk: ambient Git variables direct setup or the Go tool to another checkout.
  Mitigation: clear ambient Git routing variables, discover the child metadata
  after `cd`, and pass exactly that metadata only to `go build`.
- Risk: a generated binary lacks valid build metadata. Mitigation: delete the
  candidate and fail before emitting a successful bootstrap result.
