# Cross-Surface Impact Map: D0 app logo

## Task

- Task / issue: Lock the macOS app logo to D0 / #1077
- Plan link: `2026-07-31-issue-1077-d0-logo-plan.md`
- Owner: Codex
- Status: in implementation

## Current Ownership, Callers, and Data Flow

- Entry points: `ProjectPicker` and `GoCodeApp.init()`.
- Owning source: `BrandMark` in `macapp/Sources/GoCodeUI/DesignSystem/BrandMark.swift`.
- Consumers: `BrandMarkView`, `BrandTileView`, `AppShell.swift`, and
  `GoCodeApp.swift`.
- Similar abstractions searched: `rg -n -i "logo|brand|mark|favicon|icon|AppIcon|BrandMark"`;
  no competing native mark was found.
- Conclusion: keep one drawn `Shape`; it is already the shared source of truth.

## Config, API, CLI, and Tools

- None — searched; the mark has no config, API, CLI, tool, or wire contract.

## Persistence and Compatibility

- None — searched; geometry is stateless and no persisted value changes.
- Public `BrandMark`, `BrandMarkView`, and `BrandTileView` initializers remain
  compatible.

## Lifecycle, Security, and Reliability

- Lifecycle: preserve deferred Dock rasterisation so app startup remains fast.
- Security/privacy: None — no data, credentials, permissions, or trust boundary.
- Failure/recovery: if rasterisation fails, existing nil behavior remains; the
  in-app mark still renders directly from the shape.

## Product and Integration Surfaces

- Server/runtime: None — no Go runtime path consumes the mark.
- TUI/web/other clients: None — searched; macOS-only native surface.
- Provider/model/tool catalogs: None — searched; unrelated.
- UX: D0 ring opening at the right center, mirrored inward chevron, current
  decorative accessibility behavior, current light/dark theme colors.

## Deployment and Operations

- None — no migration, feature flag, deployment ordering, metric, alert, or
  runbook change.
- Rollback: revert the isolated commit; no stored state needs recovery.

## Regression Tests

- First red test: `DesignTokenTests.brandMarkUsesD0Geometry`.
- Acceptance: exact D0 ring endpoints and mirrored chevron coordinates at a
  normalized 100x100 square.
- Edge: existing small-size, scale-independence, and non-square-frame tests.
- Real path: build and launch the native app/project picker when available.
- Commands: `swift test --package-path macapp --filter GoCodeUITests.DesignTokenTests`,
  `swift test --package-path macapp`, and `./scripts/test-regression.sh`.

## Documentation and Handoff

- Source comments and this plan carry the geometry rationale.
- No public docs or folder index update is needed because no public doc file is
  changed.

## Warning Check

- All unaffected surfaces are explicitly marked None with search/rationale.
