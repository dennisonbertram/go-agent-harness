# Issue #13 Grooming: High priority: make apply_patch compatible with unified diff payloads

## Summary
The `apply_patch` tool requires a `path` parameter, causing model-generated unified diffs to fail with "path is required" error. Issue proposes accepting raw unified diff payloads where the path is extracted from diff headers.

## Already Addressed?
**PARTIALLY ADDRESSED** — The codebase already supports unified diffs (both standard `--- a/file` and `*** Begin Patch` format) via the `patch` parameter in `internal/harness/tools/apply_patch.go`. Functions `isStandardUnifiedDiff()` and `parseStandardUnifiedDiff()` exist and extract paths from `--- ` headers. However, the tool still requires a `path` field before reaching that logic — models that emit a plain unified diff blob without an explicit `path` field hit the "path is required" error at line 88.

## Clarity Assessment
Clear. Concrete Terminal Bench evidence: models emit unified diffs but don't include a separate `path` field. The fix is to check for a parseable diff before requiring `path`.

## Acceptance Criteria
- Model can submit raw unified diff without explicit `path` parameter
- Tool auto-extracts path from `--- ` headers
- Existing structured calls continue to work
- Regression tests for model-style unified diff payloads

## Scope
Atomic — modify `apply_patch` handler to support multiple field aliases (like write tool's `content`/`new_text`/`new_string`/`text`) and skip `path` requirement when a parseable diff is provided.

## Blockers
None.

## Effort
**Small** (1-2h) — Unified diff parser already exists. Fix is field alias addition + move path-required check after diff detection.

## Label Recommendations
Current: none. Recommended: `bug`, `high-priority`

## Recommendation
**well-specified** — Clear problem, existing parsing infrastructure. Ready to implement.
