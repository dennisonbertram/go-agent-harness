# Issue #14 — Harden Structured File Writes for JSON Files

## Summary

Added JSON content validation to the `write` tool so that writing malformed JSON to a `.json` file is rejected before the file is touched. This prevents the corruption pattern observed in Terminal Bench where the model fell back to a full-file write and produced escaped newlines or unclosed braces that broke downstream JSON consumers.

## Problem

The `write` tool accepted any string content for any file, including `.json` files. When the model generated malformed JSON (e.g. unclosed braces, literal `\n` sequences outside quoted values), the file was overwritten with the invalid content, silently breaking machine-readable consumers.

Evidence: `deploy/targets.json` was rewritten with escaped newline sequences during the `staging-deploy-docs` Terminal Bench run.

## Implementation

### Files Changed

- `internal/harness/tools/core/write.go` — added `strings` import + JSON validation guard
- `internal/harness/tools/write.go` — same guard for the legacy catalog path
- `internal/harness/tools/descriptions/write.md` — documented the JSON validation behavior
- `internal/harness/tools/core/core_test.go` — 5 new TDD tests
- `internal/harness/tools/coverage_boost_test.go` — 1 new regression test for the legacy path

### Validation Logic

Before writing any file whose path ends in `.json` (case-insensitive) and `append=false`, the tool calls `json.Valid()` on the content. If it returns false, the tool returns a structured tool result with:

```json
{
  "error": {
    "code": "invalid_json",
    "path": "deploy/targets.json",
    "message": "content is not valid JSON; the file was not written. Fix the JSON and retry."
  }
}
```

The file is not written. The error is a structured tool result (not a Go `error`), so the LLM sees the rejection and can retry with corrected content.

`append=true` writes bypass validation because appending partial JSON fragments is a legitimate pattern (e.g., JSONL streams).

### Test Coverage

| Test | Coverage |
|------|----------|
| `TestWriteTool_Handler_ValidJSON` | Valid JSON to `.json` succeeds and file is written |
| `TestWriteTool_Handler_InvalidJSON` | Malformed JSON returns `invalid_json` error, file not written |
| `TestWriteTool_Handler_InvalidJSON_EscapedNewlines` | Truncated JSON (missing closing brace) is rejected |
| `TestWriteTool_Handler_NonJSONExtension` | Non-JSON files bypass validation entirely |
| `TestWriteTool_Handler_JSONArray` | JSON arrays are accepted as valid content |
| `TestWriteJSONValidation` (legacy) | Same checks against the legacy `BuildCatalog` write tool |

## Success Criteria Met

- The harness cannot write an invalid JSON file to disk — the write is rejected and the LLM receives actionable error feedback.
- Non-JSON files are unaffected.
- All tests pass under `-race`.
