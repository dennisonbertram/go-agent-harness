# Plan: Issue #556 Git Diff MaxBytes Fixture

## Context

- Problem: `TestGitDiffTool_MaxBytes` depended on whatever uncommitted diff existed in the repository checkout.
- User impact: clean checkouts and CI could fail because `git diff` returned empty output and never exercised truncation.
- Constraints: keep the fix scoped to the flaky test, follow strict TDD, and avoid unrelated working-tree changes.

## Scope

- In scope: replace the ambient repository diff dependency with a temporary git repository fixture containing a known tracked-file modification.
- Out of scope: changing `GitDiffTool` behavior, changing git command execution, or broadening git tool coverage.

## Documentation Contract

- Feature status: `implemented`
- Public docs affected: none.
- Spec docs to update before code: none; this is a test flake fix.
- Implementation notes to add after code: engineering log entry with validation evidence.

## Test Plan (TDD)

- New failing tests to add first: tighten `TestGitDiffTool_MaxBytes` so it controls its own diff fixture instead of relying on the workspace checkout.
- Existing tests to update: `internal/harness/tools/core/git_test.go::TestGitDiffTool_MaxBytes`.
- Regression tests required: targeted `go test ./internal/harness/tools/core -run TestGitDiffTool_MaxBytes -count=1`, followed by the package test and regression gate.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Document feature status and exact contract before code.
- [x] Write or tighten the failing test first.
- [x] Implement minimal test fixture changes.
- [x] Run targeted package tests.
- [x] Stabilize the Docker-backed workspace test exposed by the regression gate by avoiding fixed container names and cleaning up provisioned containers.
- [ ] Run the repo regression gate to completion without blockers.
- [x] Update engineering log with final validation.
- [x] Update GitHub issue workpad.
- [ ] Create branch/commit/PR when git ref writes and GitHub publishing are available.

## Risks and Mitigations

- Risk: local git availability could vary.
- Mitigation: the test skips when `git` is unavailable, matching nearby git-test patterns.

- Risk: local Go cache writes can fail outside the workspace sandbox.
- Mitigation: run validation with repo-local `TMPDIR` and `GOCACHE`.

- Risk: the local sandbox can block tests that need localhost listeners before the full regression gate reaches the GH-556-relevant packages.
- Mitigation: keep the GH-556 fix locally validated with focused/package tests and report the sandbox bind restriction as a remaining blocker instead of marking the issue complete.
