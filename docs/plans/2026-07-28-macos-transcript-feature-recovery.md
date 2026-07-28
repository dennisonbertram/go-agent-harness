# Plan: macOS transcript feature recovery

## Context

- Problem: Removing the persistent transcript status strip also orphaned the session usage label and whole-conversation clipboard formatter.
- User impact: People cannot see session token/cost totals or copy the complete conversation.
- Constraints: Keep the strip removed; re-use the Environment inspector's existing card primitives; keep measurements in the design-token layer; do not run git commands.

## Scope

- In scope: Re-home usage in the Environment inspector, add a transcript-level copy affordance, split the inspector from `ChatView.swift`, and derive the user-message minimum height from type and spacing tokens.
- Out of scope: Restoring a persistent status strip or changing transcript behavior beyond these regressions.

## Documentation Contract

- Feature status: `in implementation`
- Public docs affected: None.
- Spec docs to update before code: This plan and the long-term-thinking success definition.
- Implementation notes to add after code: Engineering log entry describing the regression guard.

## Test Plan (TDD)

- New failing tests to add first: Production-source reachability assertions for the `UsageLabel` and `TranscriptText.plain` call sites.
- Existing tests to update: Token test must verify the user-message height formula instead of a literal.
- Regression tests required: The reachability test fails if either feature has no production caller.

## Implementation Checklist

- [ ] Add failing reachability coverage.
- [ ] Re-home the two user-facing features.
- [ ] Extract Environment inspector types.
- [ ] Derive the transcript row height from design tokens.
- [ ] Format and run the requested build, tests, and lint.

## Risks and Mitigations

- Risk: A visual refactor silently leaves a useful helper unrendered.
- Mitigation: Assert production-source call sites in the UI test target.
