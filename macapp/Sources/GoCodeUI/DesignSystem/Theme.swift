import AppKit
import SwiftUI

/// A single grey value shared between the two system appearances. Kept as
/// plain 0–255 RGB rather than a `Color` so `ThemeTests` can assert on the
/// exact numbers (span, neutrality, ramp order) without resolving a dynamic
/// `NSColor` at runtime — the `Color` tokens below are derived from the same
/// values, so a regression here is a regression there too.
struct RGB: Equatable {
    let r: Int
    let g: Int
    let b: Int
    /// Codex's surfaces measure R=G=B; GoCode's old `.quaternary` ramp
    /// measured R=B+3 on every surface (`.ux/design-baseline.md` §8). This is
    /// the check that catches a hue creeping back in.
    var isNeutral: Bool { r == g && g == b }
    /// Mean of the channels — equal to any channel when `isNeutral`, and the
    /// perceptual grey level used for span/ordering comparisons either way.
    var level: Int { (r + g + b) / 3 }
    var unitWhite: Double { Double(level) / 255 }
}

struct GreyLevel {
    let dark: RGB
    let light: RGB
}

/// Semantic color tokens for GoCodeUI.
///
/// Before this file existed, every surface in the app was the same
/// `.quaternary` system material at a different opacity over the window
/// background (`AppShell.swift`'s old `.background(.quaternary.opacity(0.25))`
/// / `0.35`, `CardStyle.swift`'s `.background(.quaternary.opacity(opacity))`).
/// Stacking flat opacities of one material over one background caps how far
/// apart two surfaces can ever read, however many opacities you try: six
/// surfaces measured only 13 grey levels apart (40→53), and `.quaternary`
/// itself carries a slight warm tint on macOS, so every one of them measured
/// `R = B + 3` instead of neutral (`.ux/design-baseline.md` §8).
///
/// These tokens are opaque, explicit, neutral RGB levels instead, chosen to
/// land near the reference points measured from Codex (ChatGPT.app) in that
/// baseline — a 24→51 span across four surface tiers, and a foreground ramp
/// with more than two usable rungs. They are not pixel-exact copies of
/// Codex's numbers: this is GoCode's own palette, sized to the same kind of
/// separation Codex has, not a clone of it.
enum Theme {

    // MARK: - Surfaces
    // Dark values are a 27-level span (24→51), matching Codex's measured
    // 24→34→36→45→51 (Codex's middle rung, 36, is a state-specific bubble
    // fill this app doesn't need a fifth surface token for). Light values
    // do not mirror the dark numbers (255−x would make elevated surfaces
    // *darker* than the page, which reads backwards in a light appearance):
    // they keep the same "further from paper white = further back"
    // relationship the dark ramp expresses in the other direction.

    /// The window's own background — the furthest-back surface. Nothing
    /// should render directly on this except the transcript/list content
    /// itself.
    static let backgroundLevel = GreyLevel(
        dark: RGB(r: 24, g: 24, b: 24), light: RGB(r: 246, g: 246, b: 246))

    /// One step up: rails, side panels, footer/status bars — chrome that sits
    /// beside primary content rather than on top of it.
    static let surfaceLevel = GreyLevel(
        dark: RGB(r: 34, g: 34, b: 34), light: RGB(r: 250, g: 250, b: 250))

    /// Two steps up: cards, input fields, popups, code blocks — anything that
    /// reads as its own control or container floating over `surface`.
    static let surfaceElevatedLevel = GreyLevel(
        dark: RGB(r: 45, g: 45, b: 45), light: RGB(r: 255, g: 255, b: 255))

    /// The frontmost neutral tier: selected/active rows and small tags that
    /// need to read as in front of an already-elevated surface.
    static let surfaceHighestLevel = GreyLevel(
        dark: RGB(r: 51, g: 51, b: 51), light: RGB(r: 235, g: 235, b: 235))

    // MARK: - Foreground
    // Four rungs (the task's stated minimum; Codex measures five —
    // 255/222/163/116/97 — a fifth can slot in later without breaking this
    // span). Light values are `255 − dark`, so each rung keeps the same
    // *distance from its appearance's extreme* — same relative contrast
    // step — instead of the two appearances drifting to different ratios.

    /// Primary body text — what you read. Baseline measured Codex's body
    /// contrast at 17.8:1 against GoCode's old 10.8:1; matching Codex's own
    /// black/white extremes is how that gap closes.
    static let foregroundLevel = GreyLevel(
        dark: RGB(r: 255, g: 255, b: 255), light: RGB(r: 0, g: 0, b: 0))

    /// Secondary — labels that matter but are not the thing you're reading:
    /// status words, model names, sidebar-equivalent labels.
    static let foregroundSecondaryLevel = GreyLevel(
        dark: RGB(r: 222, g: 222, b: 222), light: RGB(r: 33, g: 33, b: 33))

    /// Tertiary — metadata: dates, counts, durations, captions.
    static let foregroundTertiaryLevel = GreyLevel(
        dark: RGB(r: 163, g: 163, b: 163), light: RGB(r: 92, g: 92, b: 92))

    /// Quaternary — the dimmest still-legible text: placeholders, section
    /// headers, empty-state icons.
    static let foregroundQuaternaryLevel = GreyLevel(
        dark: RGB(r: 116, g: 116, b: 116), light: RGB(r: 139, g: 139, b: 139))

    // MARK: - Color tokens
    // Built from an appearance-aware `NSColor` so one `Color` constant tracks
    // the system's current appearance instead of the app needing an
    // `@Environment(\.colorScheme)` read at every call site.

    private static func color(_ level: GreyLevel) -> Color {
        Color(
            NSColor(name: nil) { appearance in
                let isDark = appearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua
                let unit = isDark ? level.dark.unitWhite : level.light.unitWhite
                return NSColor(white: unit, alpha: 1)
            })
    }

    static let background = color(backgroundLevel)
    static let surface = color(surfaceLevel)
    static let surfaceElevated = color(surfaceElevatedLevel)
    static let surfaceHighest = color(surfaceHighestLevel)

    static let foreground = color(foregroundLevel)
    static let foregroundSecondary = color(foregroundSecondaryLevel)
    static let foregroundTertiary = color(foregroundTertiaryLevel)
    static let foregroundQuaternary = color(foregroundQuaternaryLevel)

    /// Hairline dividers for boundaries between surfaces close enough in
    /// value that a boundary would not otherwise read (baseline §8: adjacent
    /// GoCode surfaces used to differ by only 2–3 levels — below the
    /// threshold where a value jump alone reads as a seam).
    static let separator = foreground.opacity(0.12)

    /// Unchanged from the system tint. The baseline's accent-related gap
    /// (§3, §10 — GoCode spends its one saturated hue on message ownership
    /// rather than run state) is a separate remedy from this task's palette
    /// fix; this token exists so call sites read `Theme.accent` instead of a
    /// bare `Color.accentColor` scattered across eight files.
    static let accent = Color.accentColor
}
