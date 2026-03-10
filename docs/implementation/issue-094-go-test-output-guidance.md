# Issue #94: Agent misinterprets go test output as '1 test passed'

**Date**: 2026-03-09
**Issue**: https://github.com/dennisonbertram/go-agent-harness/issues/94
**Branch**: issue-94-go-test-output-guidance
**Status**: DONE

## Problem

When an agent ran `go test ./internal/skills/...` (without `-v`), Go produced the terse output:

```
ok  go-agent-harness/internal/skills  0.200s
```

The agent interpreted this as "1 test passed" when 74+ tests actually ran. Go intentionally
suppresses individual test names and results when all tests pass and `-v` is not used.

## Root Cause

The bash tool description gave no guidance on Go test output format. The LLM had no
context to interpret the compact `ok <package> <duration>` line correctly, leading it to
infer "1 test passed" instead of understanding this is a package-level pass summary.

## Fix

Added a "INTERPRETING Go TEST OUTPUT" section to `internal/harness/tools/descriptions/bash.md`
that explains:

1. The `ok` line is a **package-level summary** — it does not report how many tests ran.
2. Non-verbose mode suppresses individual test results.
3. Agents must use `go test -v` when they need individual test counts or names.
4. Specific guidance on when to use `-v`: counting tests, identifying which tests
   passed/failed, reporting accurate results to users.

## Files Changed

- `internal/harness/tools/descriptions/bash.md` — added Go test output guidance section
- `internal/harness/tools/descriptions/embed_test.go` — added `TestBashDescriptionContainsGoTestGuidance`

## TDD Process

1. Wrote failing test `TestBashDescriptionContainsGoTestGuidance` that verifies:
   - bash description mentions "go test"
   - bash description mentions "-v" flag
   - bash description mentions "package" to clarify the summary level
2. Confirmed test failed before the fix.
3. Updated `bash.md` with the guidance section.
4. Confirmed test passes after the fix.
5. Ran `go test ./... -race` — all 15 packages pass; pre-existing `demo-cli` build failure only.

## Test Results

```
ok  go-agent-harness/internal/harness/tools/descriptions   0.130s
...all 15 non-demo-cli packages pass with -race...
```

## Impact

Low-risk change: text-only update to an embedded markdown description file. No logic
changes. The guidance is now delivered to the LLM as part of the bash tool's description
on every request, preventing the misinterpretation without any runtime overhead.
