import HarnessKit
import SwiftUI

/// One layout envelope prevents the transcript and composer from drifting to
/// different horizontal origins as their internals evolve.
struct ConversationColumn<Content: View>: View {
    @ViewBuilder let content: Content

    var body: some View {
        content
            .frame(maxWidth: Layout.chatContentMaximumWidth)
            .frame(maxWidth: .infinity, alignment: .center)
    }
}

struct ConversationHeader: View {
    @Bindable var project: ProjectSession
    @Bindable var run: RunSession

    var body: some View {
        ConversationColumn {
            HStack(spacing: Spacing.small) {
                Image(systemName: "folder")
                    .font(.system(size: IconSize.detail))
                    .foregroundStyle(Theme.foregroundTertiary)
                    .accessibilityHidden(true)
                Text(title)
                    .font(Typography.body.weight(.medium))
                    .lineLimit(1)
                Spacer(minLength: Spacing.none)
                Menu {
                    Button("New conversation") { project.newConversation() }
                    if run.conversationID != nil {
                        Button("Fork conversation") { Task { await project.fork() } }
                        Button("Undo last turn") { Task { await project.undo() } }
                    }
                } label: {
                    Image(systemName: "ellipsis")
                        .font(.system(size: IconSize.detail))
                }
                .menuStyle(.borderlessButton)
                .help("Conversation actions")
                .accessibilityLabel("Conversation actions")
            }
            .frame(height: Spacing.conversationHeaderHeight)
        }
    }

    private var title: String {
        guard let id = run.conversationID,
            let conversation = project.conversations.first(where: { $0.id == id })
        else { return "New conversation" }
        return conversation.displayTitle
    }
}
