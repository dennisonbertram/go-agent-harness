import HarnessKit
import SwiftUI
import UniformTypeIdentifiers

struct SessionsView: View {
    @Bindable var project: ProjectSession
    @Binding var section: Section
    @State private var search = ""
    @State private var exportError: String?
    @State private var deleteConfirmation: DestructiveConfirmation?

    var body: some View {
        VStack(spacing: Spacing.none) {
            HStack {
                Image(systemName: "magnifyingglass").foregroundStyle(Theme.foregroundTertiary)
                TextField("Search conversations", text: $search).textFieldStyle(.plain)
                Button("New") {
                    project.newConversation()
                    section = .chat
                }
                .disabled(project.conversationActionDisabledReason != nil)
                .help(project.conversationActionDisabledReason ?? "Start a new conversation")
                .accessibilityHint(project.conversationActionDisabledReason ?? "")
            }
            .padding(Spacing.inset)
            Divider()

            if project.conversationsLoadState.showsBlockingError(
                itemCount: project.conversations.count)
            {
                CollectionErrorState(
                    message: project.conversationsLoadState.errorMessage ?? ""
                ) {
                    Task { await project.refreshConversations() }
                }
            } else if project.conversationsLoadState.showsEmptyState(
                itemCount: project.conversations.count
            ) {
                EmptyState(
                    icon: "clock.arrow.circlepath",
                    title: "No saved conversations",
                    // Persistence is optional server-side, so absence is not an error.
                    detail:
                        "Conversations appear here once the server has a conversation store configured."
                )
            } else {
                List {
                    if project.conversationsLoadState.showsRefreshError(
                        itemCount: project.conversations.count)
                    {
                        CollectionRefreshErrorState(
                            message: project.conversationsLoadState.errorMessage ?? ""
                        ) {
                            Task { await project.refreshConversations() }
                        }
                    }
                    if project.conversationsLoadState.showsPlaceholder(
                        itemCount: project.conversations.count)
                    {
                        ForEach(0..<Layout.loadingPlaceholderRowCount, id: \.self) { _ in
                            LoadingPlaceholder(height: Layout.conversationRowHeight)
                        }
                    } else {
                        ForEach(filtered) { conversation in
                            Button {
                                Task {
                                    await project.openConversation(conversation)
                                    section = .chat
                                }
                            } label: {
                                ConversationRow(conversation: conversation)
                            }
                            .buttonStyle(.plain)
                            .accessibilityLabel(accessibilityLabel(for: conversation))
                            .disabled(project.conversationActionDisabledReason != nil)
                            .help(
                                project.conversationActionDisabledReason
                                    ?? "Open \(conversation.displayTitle)"
                            )
                            .accessibilityHint(project.conversationActionDisabledReason ?? "")
                            .contextMenu {
                                Button("Open") {
                                    Task {
                                        await project.openConversation(conversation)
                                        section = .chat
                                    }
                                }
                                .disabled(project.conversationActionDisabledReason != nil)
                                .accessibilityHint(
                                    project.conversationActionDisabledReason ?? "")
                                Button("Export Transcript…") { export(conversation) }
                                Divider()
                                Button("Delete", role: .destructive) {
                                    confirmDelete(conversation)
                                }
                                .disabled(
                                    project.run?.conversationID == conversation.id
                                        && project.conversationActionDisabledReason != nil
                                )
                                .accessibilityHint(
                                    project.run?.conversationID == conversation.id
                                        ? project.conversationActionDisabledReason ?? "" : "")
                            }
                        }
                    }
                }
                .listStyle(.inset)
            }
        }
        .task { await project.refreshConversations() }
        .alert(
            "Export failed",
            isPresented: .init(get: { exportError != nil }, set: { if !$0 { exportError = nil } })
        ) {
            Button("OK", role: .cancel) { exportError = nil }
        } message: {
            Text(exportError ?? "")
        }
        .destructiveConfirmation($deleteConfirmation)
    }

    private var filtered: [ConversationInfo] {
        guard !search.isEmpty else { return project.conversations }
        return project.conversations.filter {
            $0.displayTitle.localizedCaseInsensitiveContains(search)
        }
    }

    private func export(_ conversation: ConversationInfo) {
        let panel = NSSavePanel()
        panel.nameFieldStringValue = "\(conversation.displayTitle).jsonl"
        // The export is NDJSON, not JSON — declare a type that matches the
        // ".jsonl" extension we actually write instead of claiming `.json`.
        panel.allowedContentTypes = [
            UTType(filenameExtension: "jsonl", conformingTo: .plainText) ?? .plainText
        ]
        guard panel.runModal() == .OK, let url = panel.url else { return }
        // Call the client directly with the id being exported — `openConversation`
        // would replace whatever the user is currently looking at in chat just
        // to prime state, for an export that never needed to touch it.
        guard let client = project.harnessClient else { return }
        Task {
            do {
                let data = try await client.exportConversation(id: conversation.id)
                try data.write(to: url)
            } catch {
                exportError = error.localizedDescription
            }
        }
    }

    /// The row's icon and metadata already read visually; VoiceOver needs the
    /// same facts named explicitly rather than reading the SF Symbol pin
    /// glyph and a bare title (R9).
    private func accessibilityLabel(for conversation: ConversationInfo) -> String {
        var label = conversation.displayTitle
        if let count = conversation.messageCount {
            label += ", \(count) \(count == 1 ? "message" : "messages")"
        }
        if conversation.pinned == true { label += ", pinned" }
        return label
    }

    /// States what will be lost before it is lost (R6) rather than deleting
    /// the moment the menu item is tapped.
    private func confirmDelete(_ conversation: ConversationInfo) {
        deleteConfirmation = DestructiveConfirmation(
            title: "Delete conversation?",
            message: DeletePreview.message(for: conversation),
            confirmLabel: "Delete"
        ) {
            Task { await project.deleteConversation(conversation) }
        }
    }
}

private struct ConversationRow: View {
    let conversation: ConversationInfo

    var body: some View {
        HStack(spacing: Spacing.comfortable) {
            if conversation.pinned == true {
                Image(systemName: "pin.fill").font(Typography.caption).foregroundStyle(.orange)
            }
            VStack(alignment: .leading, spacing: 3) {
                Text(conversation.displayTitle).lineLimit(1)
                MetadataRow(spacing: Spacing.standard) {
                    if let date = conversation.updatedAt ?? conversation.createdAt {
                        Text(date.formatted(date: .abbreviated, time: .shortened))
                    }
                    if let count = conversation.messageCount {
                        Text("\(count) messages")
                    }
                    if let cost = conversation.costUSD, cost > 0 {
                        Text("$\(String(format: "%.4f", cost))")
                    }
                }
            }
            Spacer()
        }
        .padding(.vertical, Spacing.compact)
    }
}

/// Checkpoint cards — the legible form of the TUI's `/rewind <point-id> confirm`.
struct CheckpointsView: View {
    @Bindable var project: ProjectSession
    @State private var confirming: RewindPoint?

    var body: some View {
        Group {
            if project.rewindPointsLoadState.showsBlockingError(
                itemCount: project.rewindPoints.count)
            {
                CollectionErrorState(
                    message: project.rewindPointsLoadState.errorMessage ?? ""
                ) {
                    Task { await project.refreshRewindPoints() }
                }
            } else if project.rewindPointsLoadState.showsEmptyState(
                itemCount: project.rewindPoints.count)
            {
                EmptyState(
                    icon: "arrow.uturn.backward.circle",
                    title: "No checkpoints yet",
                    detail:
                        "A checkpoint is recorded whenever the agent changes files, so you can restore them."
                )
            } else {
                ScrollView {
                    VStack(alignment: .leading, spacing: Spacing.comfortable) {
                        if project.rewindPointsLoadState.showsRefreshError(
                            itemCount: project.rewindPoints.count)
                        {
                            CollectionRefreshErrorState(
                                message: project.rewindPointsLoadState.errorMessage ?? ""
                            ) {
                                Task { await project.refreshRewindPoints() }
                            }
                        }
                        if project.rewindPointsLoadState.showsPlaceholder(
                            itemCount: project.rewindPoints.count)
                        {
                            ForEach(0..<Layout.loadingPlaceholderRowCount, id: \.self) { _ in
                                LoadingPlaceholder(height: Layout.checkpointRowHeight)
                            }
                        } else {
                            ForEach(project.rewindPoints) { point in
                                CheckpointCard(
                                    point: point,
                                    disabledReason: project.conversationActionDisabledReason
                                ) {
                                    confirming = point
                                }
                            }
                        }
                    }
                    .padding(Spacing.large)
                }
            }
        }
        .task { await project.refreshRewindPoints() }
        .alert(
            "Restore this checkpoint?",
            isPresented: .init(
                get: { confirming != nil }, set: { if !$0 { confirming = nil } })
        ) {
            Button("Cancel", role: .cancel) { confirming = nil }
            Button("Restore", role: .destructive) {
                if let point = confirming {
                    Task { await project.rewind(to: point) }
                }
                confirming = nil
            }
            .disabled(project.conversationActionDisabledReason != nil)
            .accessibilityHint(project.conversationActionDisabledReason ?? "")
        } message: {
            Text(
                "This overwrites the files in this checkpoint and removes every message after it. It cannot be undone."
            )
        }
        .destructiveConfirmation(forceRewindConfirmation)
    }

    /// The distinct second confirmation for the server's `409 rewind_refused`
    /// refusal (R7, KTD-6): worded more severely than the ordinary restore
    /// above -- it names the fact that a file changed outside the harness,
    /// quotes the server's own message, and its confirm label reads "Restore
    /// Anyway" rather than "Restore" so the two cannot be confused. Declining
    /// (the binding's setter firing with `nil`) only clears the refusal; a
    /// refusal is never auto-retried with the forcing flag.
    private var forceRewindConfirmation: Binding<DestructiveConfirmation?> {
        Binding(
            get: {
                guard let refusal = project.rewindRefusal else { return nil }
                return DestructiveConfirmation(
                    title: "This checkpoint changed outside the harness",
                    message:
                        "\(refusal.message) Restoring anyway overwrites it with the checkpoint's version. It cannot be undone.",
                    confirmLabel: "Restore Anyway"
                ) {
                    Task { await project.rewind(to: refusal.point, force: true) }
                }
            },
            set: { newValue in
                if newValue == nil { project.dismissRewindRefusal() }
            }
        )
    }
}

private struct CheckpointCard: View {
    let point: RewindPoint
    let disabledReason: String?
    let onRestore: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.standard) {
            HStack {
                Image(systemName: "clock.arrow.circlepath").foregroundStyle(.tint)
                VStack(alignment: .leading, spacing: Spacing.tight) {
                    Text(point.tool.map { "Before \($0)" } ?? "Checkpoint")
                        .font(Typography.body.weight(.medium))
                    if let date = point.createdAt {
                        MetadataRow { Text(date.formatted(date: .abbreviated, time: .standard)) }
                    }
                }
                Spacer()
                Button("Restore", action: onRestore)
                    .disabled(disabledReason != nil)
                    .help(disabledReason ?? "Restore this checkpoint")
                    .accessibilityHint(disabledReason ?? "")
            }

            if let files = point.files, !files.isEmpty {
                // Say exactly which files a restore would overwrite.
                VStack(alignment: .leading, spacing: 3) {
                    ForEach(files, id: \.path) { file in
                        HStack(spacing: Spacing.small) {
                            Image(systemName: file.skipped == true ? "minus.circle" : "doc")
                                .font(Typography.detail).foregroundStyle(Theme.foregroundTertiary)
                            Text(file.path).font(Typography.codeCaption).lineLimit(1)
                                .truncationMode(.middle)
                            if let reason = file.skipReason, !reason.isEmpty {
                                Text("· \(reason)").font(Typography.detail).foregroundStyle(
                                    Theme.foregroundQuaternary)
                            }
                        }
                    }
                }
            }
        }
        .padding(Spacing.inset)
        .cardStyle()
    }
}

struct EmptyState: View {
    let icon: String
    let title: String
    let detail: String

    var body: some View {
        VStack(spacing: Spacing.standard) {
            Image(systemName: icon).font(.system(size: IconSize.emptyState)).foregroundStyle(
                Theme.foregroundQuaternary)
            Text(title).font(Typography.body.weight(.medium))
            Text(detail)
                .font(Typography.caption).foregroundStyle(Theme.foregroundTertiary)
                .multilineTextAlignment(.center).frame(maxWidth: Layout.emptyStateTextWidth)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}
