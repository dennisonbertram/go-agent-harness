# Issue #13: Make apply_patch compatible with unified diff payloads

## Summary

Added standard unified diff format support to the `apply_patch` tool. Previously, the tool only accepted a custom `*** Begin Patch` / `*** End Patch` format; it rejected standard `--- a/file` / `+++ b/file` diffs with "path is required" errors. Models consistently produce standard unified diff output, so this broke common editing workflows.

## Root cause

The `parseUnifiedPatch` function required the custom format and returned an error if the patch didn't start with `*** Begin Patch`. Since `applyUnifiedPatch` was the only entry point for the `patch` argument, any standard unified diff payload would fail.

## Changes

### `internal/harness/tools/core/apply_patch.go`

- Added `isStandardUnifiedDiff(patch string) bool` — detects format by checking if the trimmed patch starts with `--- `.
- Updated `applyUnifiedPatch` to dispatch to `parseStandardUnifiedDiff` when the format is detected, falling back to the existing `parseUnifiedPatch` for the custom format.
- Added `parseStandardUnifiedDiff(patch string) ([]unifiedPatchFile, error)` — parses standard unified diff into the existing `unifiedPatchFile` / `unifiedPatchHunk` representation. Handles:
  - Single and multiple files in one patch
  - `--- /dev/null` → file creation (kind: "add")
  - `+++ /dev/null` → file deletion (kind: "delete")
  - `a/` and `b/` prefix stripping (git convention)
  - Multiple hunks per file
  - `\\ No newline at end of file` markers (skipped)
  - Trailing empty line sentinel from `strings.Split` (not treated as a blank context line)
- Added `parseStdDiffPath(raw string) string` — extracts path from `--- `/`+++ ` headers.
- Added `parseStdDiffHunk(lines []string, start int) (unifiedPatchHunk, int, error)` — reads hunk content until the next `@@ ` or `--- ` header.

### `internal/harness/tools/apply_patch.go`

Same changes as above (this is the legacy non-core copy of the tool; both copies are kept in sync).

### `internal/harness/tools/core/core_test.go`

Added 7 new tests (TDD — written before implementation):

- `TestApplyPatchTool_Handler_StandardUnifiedDiff` — basic update via standard diff
- `TestApplyPatchTool_Handler_StandardUnifiedDiff_MultipleHunks` — two hunks in one file
- `TestApplyPatchTool_Handler_StandardUnifiedDiff_NewFile` — create new file via `--- /dev/null`
- `TestApplyPatchTool_Handler_StandardUnifiedDiff_MultipleFiles` — patch two files in one diff
- `TestParseStandardUnifiedDiff_BasicHunk` — unit test for parser: basic update
- `TestParseStandardUnifiedDiff_NewFile` — unit test for parser: `--- /dev/null` detection
- `TestParseStandardUnifiedDiff_DeleteFile` — unit test for parser: `+++ /dev/null` detection

### `internal/harness/tools/descriptions/apply_patch.md`

Updated the tool description to:
- Document standard unified diff as the recommended patch format
- Provide a concrete standard diff example
- Keep the custom `*** Begin Patch` format documented as an alternative
- Remove the misleading "Do NOT use standard git unified diff syntax" instruction

## Test results

```
go test ./internal/harness/tools/core/... -race
ok  go-agent-harness/internal/harness/tools/core  1.2s

go test ./... -race
ok  go-agent-harness/cmd/harnesscli
ok  go-agent-harness/cmd/harnessd
ok  go-agent-harness/internal/harness
ok  go-agent-harness/internal/harness/tools
ok  go-agent-harness/internal/harness/tools/core
ok  go-agent-harness/internal/harness/tools/descriptions
ok  go-agent-harness/internal/server
FAIL go-agent-harness/demo-cli [build failed] (pre-existing)
```

## Acceptance criteria met

- A model can submit a standard unified patch payload without a separate `path` field — YES
- Existing structured `apply_patch` calls continue to work — YES (all existing tests pass)
- Terminal Bench patch-style tasks no longer fail immediately on `path is required` — YES (standard diffs are now accepted)
