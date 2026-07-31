# Plan: Lock the macOS app logo to the D0 mark

## Context

- Governing GitHub issue: #1077
- Problem: `BrandMark` is shared by the macOS project picker and Dock icon, but
  its current ring opening does not match the selected D0 reference.
- User impact: the app identity currently reads as a different mark than the
  user-selected D0 logo.
- Constraints: keep the existing SwiftUI `Shape` source of truth, preserve the
  Dock-icon deferral, and avoid unrelated UI or asset-catalog work.

## Scope

- In scope: D0 ring/chevron geometry and focused GoCodeUI geometry coverage.
- Out of scope: D1/D2/D3 variants, web/TUI branding, new image assets, and
  wordmark redesign.

## Documentation Contract

- Feature status: `implemented` (merge authorized with documented waiver)
- Public docs affected: None — this is an internal native-app identity asset.
- Spec docs to update before code: None — the supplied D0 reference is the
  governing visual spec and is captured in issue #1077.
- Implementation notes to add after code: source comments and this plan.

## Test Plan (TDD)

- New failing test: assert the ring begins and ends at the D0 right-side
  endpoints and that the chevron uses the D0 mirrored points.
- Existing tests to update: extend `DesignTokenTests` with exact D0 geometry.
- Regression tests required: retain scale-independence, non-square-frame, and
  small-size geometry coverage.

## Cross-Surface Impact Map

See `2026-07-31-issue-1077-d0-logo-impact-map.md`.

## Implementation Checklist

- [x] Define acceptance criteria in issue #1077.
- [x] Link a contract-complete structured GitHub issue before implementation.
- [x] Record current architecture, callers, consumers, and source-of-truth search evidence.
- [x] Document feature status and exact contract before code.
- [x] Complete the cross-surface impact map before implementation.
- [x] Write failing tests first and capture the red result (`BrandMark` missing before implementation).
- [x] Implement the minimal D0 geometry change.
- [x] Run focused and complete Swift tests.
- [x] Run the repository regression gate — exact gate failed on the unrelated real-keychain test kills; user authorized a waiver on 2026-07-31.
- [ ] Inspect the native app path where the local environment permits — existing GoCode instances prevented an isolated accessibility observation.
- [ ] Commit only task files and open the closing PR.

## Risks and Mitigations

- Risk: changing the shared shape could alter both the project picker and Dock
  icon together. Mitigation: both consumers intentionally share this source,
  and exact path plus scale tests cover the contract.
- Risk: the SPM app has no asset catalog. Mitigation: preserve runtime
  rasterisation from `BrandTileView` in `GoCodeApp`.
