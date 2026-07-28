import Foundation
import Testing

@testable import GoCodeUI

/// WCAG relative luminance / contrast ratio, computed from an sRGB level
/// 0–255. Used to turn the baseline's measured contrast numbers (Codex
/// 17.8:1, old GoCode 10.8:1) into an assertion instead of a read-the-hex
/// judgement call.
private func relativeLuminance(_ level: Int) -> Double {
    let c = Double(level) / 255
    let linear = c <= 0.03928 ? c / 12.92 : pow((c + 0.055) / 1.055, 2.4)
    return linear
}

private func contrastRatio(_ a: Int, _ b: Int) -> Double {
    let l1 = max(relativeLuminance(a), relativeLuminance(b))
    let l2 = min(relativeLuminance(a), relativeLuminance(b))
    return (l1 + 0.05) / (l2 + 0.05)
}

@Suite("Theme tokens — dark ramp separation and neutrality")
struct ThemeTests {

    /// Codex keeps the transcript, composer, and body-text extents on one
    /// 883pt column even when the window grows. Keeping that constraint in
    /// the layout vocabulary prevents either chat surface from reclaiming
    /// the full pane width later.
    @Test("chat content width stays capped at the measured Codex column")
    func chatContentWidthMatchesCodex() {
        #expect(Layout.chatContentMaximumWidth == 883)
    }

    /// Baseline gap #1: GoCode's six surfaces spanned only 13 grey levels
    /// (40→53). Codex's five span 27 (24→51). A palette that doesn't clear a
    /// wide span is the same bug restated with new names.
    @Test("dark surface ramp spans at least 25 grey levels, matching Codex's measured 27")
    func darkSurfaceSpanMatchesCodex() {
        let span = Theme.surfaceHighestLevel.dark.level - Theme.backgroundLevel.dark.level
        #expect(span >= 25)
    }

    /// Baseline gap #1: every GoCode surface measured `R = B + 3` — a warm
    /// tint inherited from `.quaternary`. Codex's surfaces are all neutral
    /// (R=G=B). Every surface token must be neutral in dark appearance.
    @Test("every dark surface token is neutral (R=G=B), not warm-tinted")
    func darkSurfacesAreNeutral() {
        #expect(Theme.backgroundLevel.dark.isNeutral)
        #expect(Theme.surfaceLevel.dark.isNeutral)
        #expect(Theme.surfaceElevatedLevel.dark.isNeutral)
        #expect(Theme.surfaceHighestLevel.dark.isNeutral)
    }

    /// Baseline gap #4: GoCode had 2 foreground levels where Codex has 5.
    /// The task requires a 4-level minimum.
    @Test("dark foreground ramp has 4 distinct rungs")
    func darkForegroundHasFourDistinctRungs() {
        let levels = Set([
            Theme.foregroundLevel.dark.level,
            Theme.foregroundSecondaryLevel.dark.level,
            Theme.foregroundTertiaryLevel.dark.level,
            Theme.foregroundQuaternaryLevel.dark.level,
        ])
        #expect(levels.count == 4)
    }

    /// Baseline §8: GoCode's body contrast measured 10.8:1 against Codex's
    /// 17.8:1. Target near Codex's number, not just "passes AAA" (10.8:1
    /// already passes AAA — the gap is the *margin*, which is what leaves
    /// room for the foreground ramp below primary to actually read as a
    /// ramp).
    @Test("dark body-text contrast lands near Codex's measured 17.8:1")
    func darkBodyContrastNearCodex() {
        let ratio = contrastRatio(
            Theme.foregroundLevel.dark.level, Theme.backgroundLevel.dark.level)
        #expect(ratio >= 17.0)
    }

    /// Constraint: "Keep a light appearance that still works. Do not ship
    /// something only legible in dark mode." A light appearance whose
    /// background is darker than its own foreground is not a light
    /// appearance — it's the dark ramp with a different name.
    @Test("light background is brighter than light foreground")
    func lightAppearanceIsActuallyLight() {
        #expect(Theme.backgroundLevel.light.level > Theme.foregroundLevel.light.level + 50)
    }

    // MARK: - Regression

    /// Different angle from `darkSurfaceSpanMatchesCodex`: that test only
    /// compares the two endpoints, so it would still pass if `surface` and
    /// `surfaceElevated` were swapped (or collapsed onto each other) as long
    /// as the outer span held. This checks every adjacent pair is in strict
    /// ascending order, which is what actually makes each surface read as
    /// "one step more elevated" than the last — the property the whole
    /// layered-ramp remedy depends on, not just the total distance covered.
    @Test("dark surface ramp is strictly ascending, not just wide")
    func darkSurfaceRampIsStrictlyOrdered() {
        let levels = [
            Theme.backgroundLevel.dark.level,
            Theme.surfaceLevel.dark.level,
            Theme.surfaceElevatedLevel.dark.level,
            Theme.surfaceHighestLevel.dark.level,
        ]
        #expect(levels == levels.sorted())
        #expect(Set(levels).count == levels.count)
    }

    /// Different angle from `darkForegroundHasFourDistinctRungs`: that test
    /// only checks the four values are *distinct*, so it would still pass if
    /// two rungs were accidentally swapped (e.g. tertiary ending up lighter
    /// than secondary) — four distinct numbers in the wrong order still
    /// count as 4. This checks the actual hierarchy: primary reads strongest,
    /// each rung after it strictly dimmer, in dark appearance, and the same
    /// hierarchy mirrored (strictly the other direction) in light appearance,
    /// which is the property call sites are actually relying on when they
    /// pick `foregroundSecondary` over `foregroundTertiary` for "one step
    /// less important than body text."
    @Test("foreground ramp hierarchy holds in both appearances, not just distinctness")
    func foregroundRampHierarchyHoldsInBothAppearances() {
        let dark = [
            Theme.foregroundLevel.dark.level,
            Theme.foregroundSecondaryLevel.dark.level,
            Theme.foregroundTertiaryLevel.dark.level,
            Theme.foregroundQuaternaryLevel.dark.level,
        ]
        #expect(dark == dark.sorted(by: >))

        let light = [
            Theme.foregroundLevel.light.level,
            Theme.foregroundSecondaryLevel.light.level,
            Theme.foregroundTertiaryLevel.light.level,
            Theme.foregroundQuaternaryLevel.light.level,
        ]
        #expect(light == light.sorted())
    }
}
