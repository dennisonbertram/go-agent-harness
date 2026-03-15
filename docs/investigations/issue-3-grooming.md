# Issue #3 Grooming: Make max steps tunable per-run, default to unlimited

## Summary
Current 8-step default is too low for real coding tasks (need 20-50+). Add per-run override via POST body, change default to unlimited, keep env var as fallback.

## Evaluation
- **Clarity**: Very clear — problem is well-defined, proposed changes are concrete
- **Acceptance Criteria**: Explicit and testable — default behavior, per-run override, env var fallback
- **Scope**: Atomic — focused change to request validation and step loop condition
- **Blockers**: None
- **Effort**: small — mostly RunRequest validation and runner config plumbing

## Recommended Labels
well-specified, small

## Missing Clarifications
- Should `max_steps: 0` in request override env default, or inherit it?

## Notes
- Runner already has `effectiveMaxSteps` computed from env var — simple to add request param override
- Safety mechanisms (cost ceiling, idle detection, cancellation) are separate issues — out of scope here
