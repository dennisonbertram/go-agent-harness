# Testing Runbook

## Policy

- Tests are required before implementation (strict TDD).
- No commit is allowed without passing tests.
- Tests must verify behavior, edge cases, and failure paths.

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
```

Use `tmux` for long-running test processes.
