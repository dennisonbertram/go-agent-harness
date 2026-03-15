# Issue #41 Grooming: Migrate all tool descriptions to //go:embed markdown files

## Summary
Move all inline tool description strings to embedded `.md` files in `internal/harness/tools/descriptions/`, using the `descriptions.Load()` pattern.

## Already Addressed?
**SUBSTANTIALLY COMPLETE (~70%)** — The `//go:embed` pattern is implemented and working. `internal/harness/tools/descriptions/` contains 51 `.md` files. The cron tools and many others are already migrated. However, approximately 15-25 tools still use inline description strings rather than `descriptions.Load()`.

## Clarity Assessment
Very clear. Reference implementation exists and the pattern is proven.

## Acceptance Criteria
- All tools use `descriptions.Load("tool_name")` for the Description field
- All `.md` files exist in `internal/harness/tools/descriptions/`
- Build passes, tests pass
- No inline description strings remain

## Scope
Mechanical refactoring — atomic per tool, can be batched.

## Blockers
None.

## Effort
**Medium** (2-3h remaining) — ~8 minutes per tool for the remaining ~15-25 tools. Good beginner contribution.

## Label Recommendations
Current: none. Recommended: `chore`, `good-first-issue`

## Recommendation
**well-specified** — Ready to implement. Identify remaining tools with inline descriptions, migrate in batches of 5-6, verify build + tests after each batch.
