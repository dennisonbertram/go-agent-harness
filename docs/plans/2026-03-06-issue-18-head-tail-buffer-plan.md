# Plan: Issue #18 Head-Tail Buffer for Long Command Output

## Context

- Problem: shell and command helper paths currently buffer full stdout/stderr in memory, which can explode memory and emit oversized payloads on noisy commands.
- User impact: long-running commands can produce huge outputs that degrade run stability and make tool results hard to consume.
- Constraints: preserve current tool JSON contracts, keep deterministic output order, and maintain strict TDD workflow.

## Scope

- In scope:
  - Add bounded head-tail capture for command output.
  - Apply in `bash` foreground execution and background `job_output` reads.
  - Add tests for truncation behavior and middle-elision marker.
- Out of scope:
  - Streaming tool output events.
  - Persistent output storage across process restarts.
  - Protocol-level result schema redesign.

## Test Plan (TDD)

- New failing tests to add first:
  - Foreground command output preserves head and tail, omits middle for oversized output.
  - Background `job_output` returns bounded head-tail output after command completion.
- Existing tests to update:
  - None expected beyond new coverage additions.
- Regression tests required:
  - `go test ./internal/harness/tools -run TestJobManagerOutputHeadTailBuffer`
  - `go test ./internal/harness -run TestBashToolOutputUsesHeadTailBuffer`
  - `./scripts/test-regression.sh`

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Write failing tests first.
- [x] Implement minimal code changes.
- [x] Refactor while tests remain green.
- [x] Update docs and indexes.
- [x] Update engineering/system/observational logs as needed.
- [x] Run full test suite.
- [ ] Merge branch back to `main` after tests pass. (blocked: existing unrelated regression failures)

## Risks and Mitigations

- Risk: output trimming could hide relevant diagnostics.
- Mitigation: preserve both beginning and end of output with explicit omission marker.

- Risk: behavior drift in existing tool output expectations.
- Mitigation: keep field names identical and validate through targeted + regression tests.
