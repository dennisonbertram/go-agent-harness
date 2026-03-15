# Issue #14 Grooming: High priority: harden structured file writes for JSON and machine-readable files

## Summary
Models corrupt adjacent structured files when falling back to full-file writes. JSON written with escaped newline sequences (`\n` literals instead of actual newlines), breaking JSON validity.

## Already Addressed?
**PARTIALLY ADDRESSED** — `internal/harness/tools/write.go` already validates JSON at lines 108-122: if the file extension is `.json` and content is not valid JSON, the write is rejected with an error. However, `apply_patch` has no equivalent validation, so model edits via patch can still corrupt JSON files. The issue may persist if models are using `apply_patch` rather than `write`.

## Clarity Assessment
Clear problem description but defers direction: "Review whether better apply_patch support resolves this naturally." Needs clarification on which tool is triggering the corruption in practice.

## Acceptance Criteria
- JSON validation covers both `write` and `apply_patch` code paths
- Invalid structured writes rejected before persisting
- Regression tests for JSON edits via both tools

## Scope
Medium — spans write tool (partially done) and apply_patch tool.

## Blockers
Partial dependency on #13 (apply_patch unified diff) since both involve apply_patch hardening.

## Effort
**Small-Medium** (2-4h) — JSON validation logic exists, just needs to be ported to apply_patch path + regression tests.

## Label Recommendations
Current: none. Recommended: `bug`, `high-priority`

## Recommendation
**needs-clarification** — Confirm which tool/code path causes corruption in practice before implementing. If apply_patch is the culprit, add JSON validation there; if write, the validation exists but may need tuning.
