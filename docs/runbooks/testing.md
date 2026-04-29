# Testing Runbook

## Policy

- Tests are required before implementation (strict TDD).
- No commit is allowed without passing tests.
- Tests must verify behavior, edge cases, and failure paths.
- Regression suite must enforce coverage gates to prevent silent test erosion.
- No structural refactor is allowed without characterization coverage for the seam being changed.
- Every bug discovered during implementation must add a permanent regression test before the fix is considered complete.

## Anti-Ghost-Feature Rule

- README and operator-facing docs may describe only implemented and test-covered behavior.
- Planned work belongs in plan/spec docs only, never in public route lists or feature overviews.
- Implementation notes are written only after code and tests land.
- If implementation scope changes, update the spec before writing more code.

## Minimum Test Quality Bar

- Each new feature has at least one acceptance-level test.
- Include edge case tests and negative-path tests.
- Avoid trivial assertions that do not validate user-visible behavior.
- Avoid tests that only replicate implementation internals.

## Workflow

1. Write test(s) first.
2. Run test(s) and verify failure is expected.
3. Implement minimal code to pass tests.
4. Run full suite.
5. Commit only after all tests pass.

## Architectural Change Protocol

1. Capture the current regression baseline for the packages affected by the change.
2. Add or tighten characterization tests for the current seam before refactoring.
3. Add new failing tests for the new capability.
4. Implement the smallest change that turns the new tests green.
5. Add regression coverage for any bug found during the slice.
6. Run targeted package tests during the red-green-refactor loop.
7. Run `./scripts/test-regression.sh` at stage-complete boundaries.

## Common Commands (Go)

```bash
go test ./...
go test ./... -race
go test ./... -coverprofile=coverage.out
./scripts/test-regression.sh
```

Use `tmux` for long-running test processes.

## Autoresearch Loop

Use the `autoresearch` prompt profile when you want the harness to search for a useful regression or characterization test instead of running a one-off manual investigation.

### One-Shot

```bash
./scripts/autoresearch-run.sh --target "internal/harness.Runner.SubmitInput" --max-steps 50
```

### Loop

```bash
tmux new-session -d -s autoresearch './scripts/autoresearch-loop.sh --iterations 3 --max-steps 50'
```

The loop writes markdown reports and raw logs under `.tmp/autoresearch/` by default. It starts with the highest-risk seams from `docs/investigations/test-coverage-gaps.md`, runs each target with a 50-step budget unless overridden, and picks a narrower validation command for each target before falling back to `./scripts/test-regression.sh`.

## Regression Gate (Required Before Merge)

Use the repository regression script:

```bash
./scripts/test-regression.sh
```

The script enforces:

- `go test ./...`
- `go test ./... -race`
- Coverage profile generation
- Coverage gate checks:
  - Minimum total statement coverage (default `80.0%`)
  - No function with `0.0%` execution coverage

## GitHub CI Shape

- Pull requests run the fast GitHub gate: `go test ./internal/... ./cmd/...`
- The full regression suite stays in GitHub, but no longer blocks every PR:
  - on pushes to `main`
  - on the nightly scheduled run
  - on manual `workflow_dispatch`
- Before merge/release confidence-sensitive changes, still run `./scripts/test-regression.sh` locally or via GitHub Actions.

Optional overrides:

```bash
MIN_TOTAL_COVERAGE=82.5 COVERPROFILE_PATH=coverage.out ./scripts/test-regression.sh
```
