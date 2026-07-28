import SwiftUI

/// Card background shared by every grouped content box (issue #951 finding 15:
/// `SectionBox` and `CheckpointCard` hand-built this identically, `DiffView`
/// built a near-identical variant at a different opacity/radius).
///
/// Used to be one translucent `.quaternary` material at a tunable opacity —
/// which is exactly the pattern `.ux/design-baseline.md` §8 identifies as the
/// root cause of GoCode's 13-level, warm-tinted surface ramp: stacking
/// opacities of the same material over the same background caps how far
/// apart two surfaces can read, however the opacity is tuned. `surfaceElevated`
/// is an opaque, explicit level instead, so `opacity` no longer has anything
/// to parameterize.
struct CardStyle: ViewModifier {
    var cornerRadius: CGFloat = 10

    func body(content: Content) -> some View {
        content.background(Theme.surfaceElevated, in: .rect(cornerRadius: cornerRadius))
    }
}

extension View {
    func cardStyle(cornerRadius: CGFloat = 10) -> some View {
        modifier(CardStyle(cornerRadius: cornerRadius))
    }
}

/// Caption-weight, tertiary-colored metadata row shared by list-row summaries
/// (`TaskRow`, `RunRow`, `ConversationRow`, `CheckpointCard`), which each
/// hand-built the same `HStack` + `.font(.caption).foregroundStyle(.secondary)`.
struct MetadataRow<Content: View>: View {
    var spacing: CGFloat = 6
    @ViewBuilder var content: Content

    var body: some View {
        HStack(spacing: spacing) { content }
            .font(.caption).foregroundStyle(Theme.foregroundTertiary)
    }
}
