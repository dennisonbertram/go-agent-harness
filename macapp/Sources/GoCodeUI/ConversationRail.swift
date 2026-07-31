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

            // Each header is gated on its own rows. Gating both on "any
            // conversations at all" printed a "Pinned" heading with nothing
            // under it for every project that has none — which is most of them.
            if !pinnedConversations.isEmpty {
                RailSectionHeader("Pinned")
                ForEach(pinnedConversations) { conversation in
                    ConversationRailRow(
                        conversation: conversation, project: project, section: $section)
                }
            }

            if !unpinnedConversations.isEmpty {
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
        // `.task` fires once, so a rail that appeared before the project had
        // any saved conversation stayed empty for the whole session — in a new
        // project that is every conversation the user starts, and it only
        // filled in after a relaunch. A run finishing is the point at which
        // the conversation is definitely persisted and worth re-reading.
        .onChange(of: project.run?.isBusy) { _, busy in
            guard busy == false else { return }
            Task { await project.refreshConversations() }
        }
    }

    private var conversations: [ConversationInfo] {
        project.conversations.sorted { lhs, rhs in
            (lhs.updatedAt ?? lhs.createdAt ?? .distantPast)
                > (rhs.updatedAt ?? rhs.createdAt ?? .distantPast)
        }
    }

    private var pinnedConversations: [ConversationInfo] {
        conversations.filter { $0.pinned == true }
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
                    // Body, not detail. The reference sets its sidebar very
                    // slightly *larger* than its transcript body; this was 24%
                    // smaller, which read as a footnote rather than as the
                    // list the sidebar exists to be.
                    .font(Typography.body)
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
