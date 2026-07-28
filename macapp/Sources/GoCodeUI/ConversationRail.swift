import HarnessKit
import SwiftUI

/// The rail is a compact view of the same saved conversations exposed by the
/// Sessions screen. Navigation remains available without displacing the list.
struct ConversationRail: View {
    @Binding var section: Section
    @Bindable var project: ProjectSession
    let onClose: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.tight) {
            RailRow(section: $section, item: .chat)

            if !conversations.isEmpty {
                RailSectionHeader("Pinned")
                ForEach(conversations.filter { $0.pinned == true }) { conversation in
                    ConversationRailRow(
                        conversation: conversation, project: project, section: $section)
                }

                RailSectionHeader("Projects")
                ForEach(unpinnedConversations) { conversation in
                    ConversationRailRow(
                        conversation: conversation, project: project, section: $section)
                }
            }

            Spacer(minLength: Spacing.none)

            HStack(spacing: Spacing.tight) {
                RailRow(section: $section, item: .activity, compact: true)
                RailRow(section: $section, item: .sessions, compact: true)
                RailRow(section: $section, item: .checkpoints, compact: true)
                RailRow(section: $section, item: .settings, compact: true)
                Button(action: onClose) {
                    Image(systemName: "xmark.circle")
                        .font(.system(size: IconSize.standard))
                }
                .buttonStyle(.plain)
                .help("Close project and stop its server")
                .accessibilityLabel("Close project and stop its server")
            }
            .foregroundStyle(Theme.foregroundSecondary)
        }
        .padding(.vertical, Spacing.comfortable)
        .padding(.horizontal, Spacing.standard)
        .frame(width: Layout.railWidth)
        .background(Theme.surface.ignoresSafeArea(.container, edges: .top))
        .task { await project.refreshConversations() }
    }

    private var conversations: [ConversationInfo] {
        project.conversations.sorted { lhs, rhs in
            (lhs.updatedAt ?? lhs.createdAt ?? .distantPast)
                > (rhs.updatedAt ?? rhs.createdAt ?? .distantPast)
        }
    }

    private var unpinnedConversations: [ConversationInfo] {
        conversations.filter { $0.pinned != true }
    }
}

private struct ConversationRailRow: View {
    let conversation: ConversationInfo
    @Bindable var project: ProjectSession
    @Binding var section: Section

    private var isActive: Bool { project.run?.conversationID == conversation.id }

    var body: some View {
        Button {
            Task { await project.openConversation(conversation) }
            section = .chat
        } label: {
            HStack(spacing: Spacing.small) {
                Circle()
                    .fill(isActive ? Theme.foreground : Theme.foregroundQuaternary)
                    .frame(width: IconSize.rule, height: IconSize.rule)
                    .accessibilityHidden(true)
                Image(systemName: "bubble.left")
                    .font(.system(size: IconSize.detail))
                    .foregroundStyle(Theme.foregroundQuaternary)
                    .accessibilityHidden(true)
                Text(conversation.displayTitle)
                    .font(Typography.detail)
                    .lineLimit(1)
                Spacer(minLength: Spacing.none)
                if conversation.messageCount ?? 0 > 0 {
                    Image(systemName: "chevron.right")
                        .font(.system(size: IconSize.rule))
                        .foregroundStyle(Theme.foregroundQuaternary)
                        .accessibilityHidden(true)
                }
            }
            .padding(.leading, Spacing.inset)
            .padding(.trailing, Spacing.comfortable)
            .frame(height: Layout.railRowHeight, alignment: .leading)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(
                isActive ? Theme.selectedRowSurface : .clear,
                in: .rect(cornerRadius: CornerRadius.control)
            )
            .foregroundStyle(isActive ? Theme.selectedRowForeground : Theme.foregroundSecondary)
            .contentShape(.rect)
        }
        .buttonStyle(.plain)
        .help(conversation.displayTitle)
        .accessibilityLabel("Open \(conversation.displayTitle)")
    }
}
