# Plan: macOS inline loading states

## Context

- Problem: fetched macOS collections use their empty arrays before a request completes, so the UI briefly states that data does not exist. Several conditional controls and full-pane spinners also replace differently sized views mid-flow.
- User impact: operators see false empty states and distracting layout shifts while conversations, activity, and model settings load.
- Constraints: keep loading inline, use existing semantic tokens, respect Reduce Motion, add one reusable skeleton primitive, and do not use Git commands.

## Scope

- In scope: typed collection load states; skeleton rows and guarded empty states for conversations, activity, rewind points, and model settings; fixed-footprint assistant and model-fetch status slots; focused regression tests.
- Out of scope: startup-state presentation, unrelated conditional content, and server/API contract changes.

## Documentation Contract

- Feature status: `implemented` (Swift build/test execution is sandbox-blocked; formatter parsed the source.)
- Public docs affected: None; this is a native presentation correction.
- Spec docs to update before code: this plan and the long-term intent ledger.
- Implementation notes to add after code: loading-state ownership and the regression coverage.

## Test Plan (TDD)

- New failing tests to add first: `CollectionLoadState` permits an empty state only after a successful loaded result.
- Existing tests to update: token tests for the new loading geometry/token roles.
- Regression tests required: loading, failed, and loaded-empty collection states.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Document feature status and exact contract before code.
- [x] Write failing tests first.
- [x] Implement the shared state and skeleton primitive.
- [x] Gate empty states and reserve inline footprints.
- [x] Update implementation logs and indexes.
- [x] Run the requested build, tests, formatter, and strict lint (build/test blocked by Xcode cache sandboxing).

## Risks and Mitigations

- Risk: a fetch failure could be presented as a normal empty collection. Mitigation: only `.loaded` permits empty messaging; failures retain the non-empty loading shell and surface the existing status message.
- Risk: animation can be distracting or inaccessible. Mitigation: the placeholder pulse is disabled when Reduce Motion is enabled.
