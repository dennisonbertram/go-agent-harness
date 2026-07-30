import HarnessKit
import SwiftUI
import UniformTypeIdentifiers

struct SessionsView: View {
    @Bindable var project: ProjectSession
    @Binding var section: Section
    @State private var search = ""
    @State private var exportError: String?

    var body: some View {
        VStack(spacing: Spacing.none) {
            HStack {
                Image(systemName: "magnifyingglass").foregroundStyle(Theme.foregroundTertiary)
                TextField("Search conversations", text: $search).textFieldStyle(.plain)
                Button("New") {
                    project.newConversation()
                    section = .chat
                }
            }
            .padding(Spacing.inset)
            Divider()

            if project.conversationsLoadState.showsError {
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
                    if project.conversationsLoadState.showsPlaceholder(
                        itemCount: project.conversations.count)
                    {
                        ForEach(0..<Layout.loadingPlaceholderRowCount, id: \.self) { _ in
                            LoadingPlaceholder(height: Layout.conversationRowHeight)
                        }
                    } else {
                        ForEach(filtered) { conversation in
                            ConversationRow(conversation: conversation)
                                .contentShape(.rect)
                                .onTapGesture {
                                    Task {
                                        await project.openConversation(conversation)
                                        section = .chat
                                    }
                                }
                                .contextMenu {
                                    Button("Open") {
                                        Task {
                                            await project.openConversation(conversation)
                                            section = .chat
                                        }
                                    }
                                    Button("Export Transcript…") { export(conversation) }
                                    Divider()
                                    Button("Delete", role: .destructive) { delete(conversation) }
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

    private func delete(_ conversation: ConversationInfo) {
        Task { await project.deleteConversation(conversation) }
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

    // NOTE (issue #951 finding 9): a "Restore anyway" path for the server's
    // `409 rewind_refused` (a file changed outside the harness) still has no
    // UI. Wiring it properly needs `ProjectSession.rewind` to surface that
    // refusal distinctly instead of collapsing every failure into
    // `statusMessage` text — `ProjectSession.swift` is out of scope for this
    // task, so the previously dead `forceNext` toggle (set to `false` on tap,
    // never `true`) is removed rather than left half-wired.

    var body: some View {
        Group {
            if project.rewindPointsLoadState.showsError {
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
                        if project.rewindPointsLoadState.showsPlaceholder(
                            itemCount: project.rewindPoints.count)
                        {
                            ForEach(0..<Layout.loadingPlaceholderRowCount, id: \.self) { _ in
                                LoadingPlaceholder(height: Layout.checkpointRowHeight)
                            }
                        } else {
                            ForEach(project.rewindPoints) { point in
                                CheckpointCard(point: point) {
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
        } message: {
            Text(
                "This overwrites the files in this checkpoint and removes every message after it. It cannot be undone."
            )
        }
    }
}

private struct CheckpointCard: View {
    let point: RewindPoint
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
