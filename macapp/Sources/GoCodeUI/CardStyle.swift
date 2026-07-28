import SwiftUI

/// Card background shared by every grouped content box (issue #951 finding 15:
/// `SectionBox` and `CheckpointCard` hand-built this identically, `DiffView`
/// built a near-identical variant at a different opacity/radius).
struct CardStyle: ViewModifier {
    var cornerRadius: CGFloat = 10
    var opacity: Double = 0.3

    func body(content: Content) -> some View {
        content.background(.quaternary.opacity(opacity), in: .rect(cornerRadius: cornerRadius))
    }
}

extension View {
    func cardStyle(cornerRadius: CGFloat = 10, opacity: Double = 0.3) -> some View {
        modifier(CardStyle(cornerRadius: cornerRadius, opacity: opacity))
    }
}

/// Caption-weight, secondary-colored metadata row shared by list-row
/// summaries (`TaskRow`, `RunRow`, `ConversationRow`, `CheckpointCard`), which
/// each hand-built the same `HStack` + `.font(.caption).foregroundStyle(.secondary)`.
struct MetadataRow<Content: View>: View {
    var spacing: CGFloat = 6
    @ViewBuilder var content: Content

    var body: some View {
        HStack(spacing: spacing) { content }
            .font(.caption).foregroundStyle(.secondary)
    }
}
