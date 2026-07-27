import HarnessKit
import SwiftUI
import UniformTypeIdentifiers

struct SessionsView: View {
    @Bindable var project: ProjectSession
    @Binding var section: Section
    @State private var search = ""

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Image(systemName: "magnifyingglass").foregroundStyle(.secondary)
                TextField("Search conversations", text: $search).textFieldStyle(.plain)
                Button("New") {
                    project.newConversation()
                    section = .chat
                }
            }
            .padding(12)
            Divider()

            if project.conversations.isEmpty {
                EmptyState(
                    icon: "clock.arrow.circlepath",
                    title: "No saved conversations",
                    // Persistence is optional server-side, so absence is not an error.
                    detail:
                        "Conversations appear here once the server has a conversation store configured."
                )
            } else {
                List(filtered) { conversation in
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
                .listStyle(.inset)
            }
        }
        .task { await project.refreshConversations() }
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
        panel.allowedContentTypes = [UTType.json]
        guard panel.runModal() == .OK, let url = panel.url else { return }
        Task {
            await project.openConversation(conversation)
            await project.exportTranscript(to: url)
        }
    }

    private func delete(_ conversation: ConversationInfo) {
        Task { await project.deleteConversation(conversation) }
    }
}

private struct ConversationRow: View {
    let conversation: ConversationInfo

    var body: some View {
        HStack(spacing: 10) {
            if conversation.pinned == true {
                Image(systemName: "pin.fill").font(.caption).foregroundStyle(.orange)
            }
            VStack(alignment: .leading, spacing: 3) {
                Text(conversation.displayTitle).lineLimit(1)
                HStack(spacing: 8) {
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
                .font(.caption).foregroundStyle(.secondary)
            }
            Spacer()
        }
        .padding(.vertical, 4)
    }
}

/// Checkpoint cards — the legible form of the TUI's `/rewind <point-id> confirm`.
struct CheckpointsView: View {
    @Bindable var project: ProjectSession
    @State private var confirming: RewindPoint?
    @State private var forceNext = false

    var body: some View {
        Group {
            if project.rewindPoints.isEmpty {
                EmptyState(
                    icon: "arrow.uturn.backward.circle",
                    title: "No checkpoints yet",
                    detail:
                        "A checkpoint is recorded whenever the agent changes files, so you can restore them."
                )
            } else {
                ScrollView {
                    VStack(alignment: .leading, spacing: 10) {
                        ForEach(project.rewindPoints) { point in
                            CheckpointCard(point: point) {
                                forceNext = false
                                confirming = point
                            }
                        }
                    }
                    .padding(16)
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
                    Task { await project.rewind(to: point, force: forceNext) }
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
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Image(systemName: "clock.arrow.circlepath").foregroundStyle(.tint)
                VStack(alignment: .leading, spacing: 2) {
                    Text(point.tool.map { "Before \($0)" } ?? "Checkpoint")
                        .font(.callout.weight(.medium))
                    if let date = point.createdAt {
                        Text(date.formatted(date: .abbreviated, time: .standard))
                            .font(.caption).foregroundStyle(.secondary)
                    }
                }
                Spacer()
                Button("Restore", action: onRestore)
            }

            if let files = point.files, !files.isEmpty {
                // Say exactly which files a restore would overwrite.
                VStack(alignment: .leading, spacing: 3) {
                    ForEach(files, id: \.path) { file in
                        HStack(spacing: 6) {
                            Image(systemName: file.skipped == true ? "minus.circle" : "doc")
                                .font(.caption2).foregroundStyle(.secondary)
                            Text(file.path).font(.caption.monospaced()).lineLimit(1)
                                .truncationMode(.middle)
                            if let reason = file.skipReason, !reason.isEmpty {
                                Text("· \(reason)").font(.caption2).foregroundStyle(.tertiary)
                            }
                        }
                    }
                }
            }
        }
        .padding(12)
        .background(.quaternary.opacity(0.3), in: .rect(cornerRadius: 10))
    }
}

struct EmptyState: View {
    let icon: String
    let title: String
    let detail: String

    var body: some View {
        VStack(spacing: 8) {
            Image(systemName: icon).font(.system(size: 30)).foregroundStyle(.tertiary)
            Text(title).font(.callout.weight(.medium))
            Text(detail)
                .font(.caption).foregroundStyle(.secondary)
                .multilineTextAlignment(.center).frame(maxWidth: 320)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}
