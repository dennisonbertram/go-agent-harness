# Plan: Preserve Source-Workflow Process Exit Errors

## Context

- Governing GitHub issue: #1064.
- Problem: `SourceManager.runSourceWorkflow` captures both `stdin.Close` and
  `cmd.Wait` failures but returns the cleanup error first, masking a non-zero
  child exit and its bounded stderr.
- User impact: workflow diagnostics can report `broken pipe` instead of the
  actionable child-process failure, and the scheduling-dependent result makes
  the hosted fast gate intermittent.
- Constraints: timeout and protocol errors keep their existing precedence;
  process exit precedes stdin-close cleanup; a close-only failure remains
  visible; no sleeps, retries, protocol changes, or unbounded output.

## Scope

- In scope: deterministic outcome-arbitration coverage and the minimal
  precedence repair in `internal/workflow/source.go`.
- Out of scope: workflow protocol changes, process-supervision redesign,
  ignoring pipe errors generally, API or persistence changes, and unrelated
  workflow failures.

## Documentation Contract

- Feature status: implemented and fully verified locally; hosted checks pending.
- Public docs affected: none; this changes diagnostic precedence only.
- Spec docs before code: this plan and its linked impact map.
- Implementation notes after code: engineering, observational, system, and
  long-term logs plus the plans index.

## Test Plan (TDD)

- First red: add a table-driven test for a deterministic outcome-arbitration
  seam where `waitErr` and a synthetic broken-pipe `closeErr` coexist; require
  the returned error to name the workflow exit and bounded stderr.
- Controls: timeout precedes all teardown failures; protocol error precedes
  wait/close failures; wait error precedes close error; close-only failure is
  returned; nil result after clean exit remains an error; success is unchanged.
- Focused stress: normal and race at `-count=100`.
- Package gates: complete `internal/workflow` normal and race.
- Repository gate: unchanged foreground non-TTY
  `./scripts/test-regression.sh`.
- Hosted gates: required PR checks, including `test-fast` and `test-race`.

## Cross-Surface Impact Map

- See `2026-07-31-issue-1064-workflow-exit-precedence-impact-map.md`.

## Implementation Checklist

- [x] Link contract-complete bug #1064 before implementation.
- [x] Record current ownership, callers, sources of truth, and search evidence.
- [x] Write this plan and the impact map before code.
- [x] Add and confirm the deterministic failing regression.
- [x] Implement the minimal precedence repair.
- [x] Confirm focused stress, package, and repository gates.
- [x] Update logs and documentation status.
- [ ] Open one closing PR and request `@codex` review without merging.
- [ ] Confirm hosted checks are green.

## Risks and Mitigations

- Risk: reordering errors could hide timeout or protocol failures.
- Mitigation: pin the full precedence table, with timeout and protocol controls.
- Risk: extracting arbitration could change successful result behavior.
- Mitigation: pin successful and clean-exit-without-result cases.
- Risk: stderr diagnostics could become unbounded or disappear.
- Mitigation: preserve `boundedString(..., maxWorkflowStderrBytes)` and assert
  both inclusion and truncation behavior through existing and new tests.

## Verification Evidence

- Semantic red:
  `go test ./internal/workflow -run
  '^TestResolveSourceWorkflowOutcomePrecedence/process_exit_precedes_stdin_close_and_includes_stderr$'
  -count=1` returned `broken pipe` instead of the expected process-exit error.
- Semantic green: the same command passed after restoring process-exit
  precedence.
- Focused stress: deterministic arbitration normal/race `-count=100` passed;
  the real child-exit path plus arbitration normal/race `-count=100` passed.
- Package: `go test ./internal/workflow/... -count=1` and
  `go test -race ./internal/workflow/... -count=1` passed.
- Repository: unchanged foreground non-TTY `./scripts/test-regression.sh`
  passed normal, race, and coverage
  (`total=85.6%`, `zero-functions=0`).
