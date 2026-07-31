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
                // Primary, not tertiary. The reference draws this icon at
                // full white — it belongs to the title beside it. Ours sat two
                // rungs down while the overflow menu next to it sat two rungs
                // up, so the row's hierarchy was inverted at both ends.
                Image(systemName: "folder")
                    .font(.system(size: IconSize.detail))
                    .foregroundStyle(Theme.foreground)
                    .accessibilityHidden(true)
                Text(title)
                    .font(Typography.body.weight(.medium))
                    // Primary, matching the folder icon beside it. Setting the
                    // icon to white and leaving the title a rung down made the
                    // decoration brighter than the label it decorates.
                    .foregroundStyle(Theme.foreground)
                    .lineLimit(1)
                Spacer(minLength: Spacing.none)
                Menu {
                    Button("New conversation") { project.newConversation() }
                    if run.conversationID != nil {
                        Button("Fork conversation") { Task { await project.fork() } }
                        Button("Undo last turn") { Task { await project.undo() } }
                    }
                } label: {
                    // Subtle: an overflow menu should be findable when looked
                    // for and invisible otherwise. This was brighter than the
                    // title's own folder icon.
                    Image(systemName: "ellipsis")
                        .font(.system(size: IconSize.detail))
                        .foregroundStyle(Theme.foregroundSubtle)
                }
                .menuStyle(.borderlessButton)
                // Tint on the Menu, not on its label: .borderlessButton
                // re-tints the label it is given, so styling the Image alone
                // left the overflow brighter than the title's own folder icon.
                .tint(Theme.foregroundSubtle)
                .help("Conversation actions")
                .accessibilityLabel("Conversation actions")
            }
            .frame(height: Spacing.conversationHeaderHeight)
        }
        // The reference is a compartmented app: rules cut header from
        // transcript, transcript from composer, sidebar from account. With
        // none of them the window reads as one flat plane with a floating box.
        .overlay(alignment: .bottom) {
            Rectangle()
                .fill(Theme.rule)
                .frame(height: Spacing.hairline)
        }
    }

    private var title: String {
        guard let id = run.conversationID,
            let conversation = project.conversations.first(where: { $0.id == id })
        else { return "New conversation" }
        return conversation.displayTitle
    }
}
