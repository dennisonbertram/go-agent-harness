# Plan: macOS Codex visual gauntlet — round 6

## Context

- Problem: Four visual deltas remain after the transcript's earlier neutralisation pass.
- User impact: A blue selected rail, compressed type, full-width authored prompts, and a separate toolbar band make the app recognisably unlike Codex.
- Constraints: Put all visual values in the macOS design-system layer; do not grow `ChatView.swift` with reusable layout behavior; do not run Git commands.

## Scope

- In scope: Neutral selected-rail tokens, shared typography scale, content-hugging prompt width, and rail safe-area surface continuity.
- Out of scope: Tool-row, metadata, environment inspector, and message-surface changes already accepted in earlier rounds.

## Documentation Contract

- Feature status: implemented
- Public docs affected: None; this is an internal visual calibration.
- Spec docs to update before code: This plan and the long-term thinking log.
- Implementation notes to add after code: Token and regression-test coverage is recorded in the engineering changes.

## Test Plan (TDD)

- New failing tests to add first: Pin selected-row semantics, body size, user-message cap, and the derived 45.5pt row height.
- Existing tests to update: Extend design-token and production-call-site tests.
- Regression tests required: The existing macapp Swift suite covers the token layer and production references.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Document the visual contract before code.
- [x] Implement semantic token and layout changes.
- [ ] Run the full Swift build and test suite (blocked by sandbox-owned Xcode cache).
- [x] Run Swift formatting and strict formatting lint.
