# Testing Runbook

## Policy

- Tests are required before implementation (strict TDD).
- No commit is allowed without passing tests.
- Tests must verify behavior, edge cases, and failure paths.
- Regression suite must enforce coverage gates to prevent silent test erosion.

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

## Common Commands (Go)

```bash
go test ./...
go test ./... -race
go test ./... -coverprofile=coverage.out
./scripts/test-regression.sh
```

Use `tmux` for long-running test processes.

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
