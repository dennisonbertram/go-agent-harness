# Issue #147 Grooming: Add user-facing HTTP endpoints for recipe listing and execution

## Summary
Expose recipe discovery and execution via HTTP endpoints so users can trigger recipe-based workflows without an active agent session.

## Already Addressed?
**NOT ADDRESSED** — No `/v1/recipes/*` endpoints. Recipe infrastructure fully exists: `Recipe`, `Step`, `ParameterDef` types, `LoadRecipes()`, `Executor.Execute()` in `internal/harness/tools/recipe/`.

## Clarity Assessment
Excellent — 3 endpoints with schemas and error handling.

## Acceptance Criteria
- list recipes, get recipe details, execute recipe with parameters
- Template substitution via `Substitute()`
- Step failure handling
- ~15 test cases

## Scope
Atomic.

## Blockers
None.

## Effort
**Medium** (3-4 days).

## Label Recommendations
Current: none. Recommended: `enhancement`, `medium`

## Recommendation
**well-specified** — Ready to implement. All recipe infrastructure exists.
