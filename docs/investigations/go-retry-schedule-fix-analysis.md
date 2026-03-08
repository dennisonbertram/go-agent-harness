# go-retry-schedule-fix Task Analysis

## Problem

The `go-retry-schedule-fix` terminal bench task fails every run.

## Root Cause

The test oracle (`tests/test_task.py`) used **source-code pattern matching** instead of behavioral testing. It checked for very specific code patterns like:

- `time.Duration(i+1)*base`
- `current := base` + `current += base`
- `for i := 1; i <= attempts; i++` + `time.Duration(i)*base`

An LLM agent can produce many valid implementations that fix the bug correctly but use different variable names, loop structures, or capping idioms. The pattern-matching oracle rejected these valid solutions.

Additionally, the task instruction was ambiguous: "increasing by one base interval each step" could be interpreted as additive (linear) or multiplicative. The Go test file (`retry_test.go`) clearly expects linear progression (base, 2*base, 3*base...) but the English phrasing left room for confusion.

## The Task

- **Bug**: `retry.go` uses `time.Duration(i)*base` which produces delays `[0, base, 2*base, ...]` -- the first delay is zero.
- **Fix**: First delay should be `base`, then `2*base`, etc., capped at 30s.
- **Existing Go tests** (`retry_test.go`) already comprehensively validate the correct behavior.

## Changes Made

### 1. Fixed test oracle (`tests/test_task.py`)

Replaced brittle source-pattern matching with three robust checks:

1. **`test_go_tests_pass`** -- Runs `go test -v ./...` which exercises the existing `retry_test.go` suite. This is the primary correctness check and is implementation-agnostic.
2. **`test_retry_first_delay_is_not_zero`** -- Verifies the agent didn't leave the code unchanged (the original `time.Duration(i)*base` pattern with `i := 0`).
3. **`test_retry_has_thirty_second_cap`** -- Verifies the code mentions `30` somewhere (the cap value).

### 2. Clarified task instruction (`task.yaml`)

- Changed "increasing by one base interval each step" to explicit "base, 2*base, 3*base, etc."
- Added hint to run `go test ./...` to verify the fix.

## Why This Works

The `retry_test.go` file already contains excellent behavioral tests:
- `TestScheduleReturnsMonotonicRetryDelays`: Checks `Schedule(5s, 3)` returns `[5s, 10s, 15s]`
- `TestScheduleRejectsNonPositiveInputs`: Checks nil returns for invalid inputs
- `TestScheduleCapsDelaysAtThirtySeconds`: Checks `Schedule(12s, 4)` returns `[12s, 24s, 30s, 30s]`

By running these Go tests as the oracle, any correct implementation passes regardless of code style.
