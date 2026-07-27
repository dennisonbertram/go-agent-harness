import HarnessKit
import SwiftUI

/// Two-pane shell: conversation left, work surface right (design doc §4).
public struct ContentView: View {
    @State private var session: RunSession
    @State private var selectedActivity: ToolActivity?

    public init(baseURL: URL) {
        _session = State(initialValue: RunSession(baseURL: baseURL))
    }

    public var body: some View {
        HSplitView {
            ConversationPane(session: session, selectedActivity: $selectedActivity)
                .frame(minWidth: 380, idealWidth: 460)
            WorkSurface(activity: selectedActivity)
                .frame(minWidth: 420)
        }
        .frame(minWidth: 900, minHeight: 560)
    }
}

// MARK: - Conversation

private struct ConversationPane: View {
    @Bindable var session: RunSession
    @Binding var selectedActivity: ToolActivity?

    var body: some View {
        VStack(spacing: 0) {
            TranscriptView(
                items: session.transcript.items,
                selectedActivity: $selectedActivity)
            Divider()
            StatusStrip(session: session)
            Composer(session: session)
        }
        .background(.background)
    }
}

private struct TranscriptView: View {
    let items: [TranscriptItem]
    @Binding var selectedActivity: ToolActivity?

    var body: some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 14) {
                    ForEach(items) { item in
                        row(for: item).id(item.id)
                    }
                }
                .padding(16)
                .frame(maxWidth: .infinity, alignment: .leading)
            }
            .onChange(of: items.last?.id) { _, id in
                guard let id else { return }
                withAnimation(.easeOut(duration: 0.15)) {
                    proxy.scrollTo(id, anchor: .bottom)
                }
            }
        }
    }

    @ViewBuilder
    private func row(for item: TranscriptItem) -> some View {
        switch item.kind {
        case .userPrompt(let text):
            UserBubble(text: text)
        case .assistantMessage(let message):
            AssistantBubble(message: message)
        case .thinking(let text):
            ThinkingRow(text: text)
        case .toolActivity(let activity):
            ToolRow(activity: activity, isSelected: selectedActivity?.callID == activity.callID) {
                selectedActivity = activity
            }
        case .error(let message):
            ErrorRow(message: message)
        }
    }
}

private struct UserBubble: View {
    let text: String
    var body: some View {
        HStack {
            Spacer(minLength: 40)
            Text(text)
                .textSelection(.enabled)
                .padding(.horizontal, 12).padding(.vertical, 8)
                .background(Color.accentColor.opacity(0.15), in: .rect(cornerRadius: 10))
        }
    }
}

private struct AssistantBubble: View {
    let message: AssistantMessage
    var body: some View {
        HStack(alignment: .top, spacing: 8) {
            Image(systemName: "sparkle")
                .foregroundStyle(.tint).font(.caption).padding(.top, 3)
            // Markdown so fenced code and emphasis render rather than showing raw.
            Text(.init(message.text))
                .textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
            if message.isStreaming {
                ProgressView().controlSize(.small)
            }
        }
    }
}

private struct ThinkingRow: View {
    let text: String
    @State private var expanded = false
    var body: some View {
        DisclosureGroup("Thinking", isExpanded: $expanded) {
            Text(text)
                .font(.callout.monospaced())
                .foregroundStyle(.secondary)
                .textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .font(.caption)
        .foregroundStyle(.secondary)
    }
}

/// One-line collapsed tool row — the pattern current agentic coding tools converge on.
private struct ToolRow: View {
    let activity: ToolActivity
    let isSelected: Bool
    let onSelect: () -> Void

    var body: some View {
        Button(action: onSelect) {
            HStack(spacing: 8) {
                statusIcon
                Text(activity.tool).font(.callout.weight(.medium))
                Text(summary)
                    .font(.callout).foregroundStyle(.secondary)
                    .lineLimit(1).truncationMode(.middle)
                Spacer()
                if let ms = activity.durationMS, ms > 0 {
                    Text("\(ms)ms").font(.caption).foregroundStyle(.tertiary)
                }
            }
            .padding(.horizontal, 10).padding(.vertical, 7)
            .background(
                isSelected ? Color.accentColor.opacity(0.12) : Color.secondary.opacity(0.07),
                in: .rect(cornerRadius: 8))
        }
        .buttonStyle(.plain)
    }

    private var summary: String {
        activity.arguments.isEmpty ? "" : activity.arguments
    }

    @ViewBuilder
    private var statusIcon: some View {
        switch activity.status {
        case .running: ProgressView().controlSize(.small).scaleEffect(0.7)
        case .completed: Image(systemName: "checkmark.circle.fill").foregroundStyle(.green)
        case .failed: Image(systemName: "xmark.circle.fill").foregroundStyle(.red)
        // Blocked is a policy decision, not a failure — keep it visually distinct.
        case .blocked: Image(systemName: "hand.raised.fill").foregroundStyle(.orange)
        }
    }
}

private struct ErrorRow: View {
    let message: String
    var body: some View {
        HStack(alignment: .top, spacing: 8) {
            Image(systemName: "exclamationmark.triangle.fill").foregroundStyle(.red)
            Text(message).textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .padding(10)
        .background(Color.red.opacity(0.1), in: .rect(cornerRadius: 8))
    }
}

// MARK: - Status + composer

private struct StatusStrip: View {
    @Bindable var session: RunSession

    var body: some View {
        HStack(spacing: 10) {
            if let approval = session.transcript.pendingApproval {
                Image(systemName: "hand.raised.fill").foregroundStyle(.orange)
                Text("Approve `\(approval.tool)`?").font(.callout)
                Spacer()
                Button("Deny") { session.deny() }
                Button("Approve") { session.approve() }
                    .buttonStyle(.borderedProminent)
            } else {
                stateLabel
                Spacer()
                usageLabel
                if session.isBusy {
                    Button("Stop") { session.cancel() }.controlSize(.small)
                }
            }
        }
        .padding(.horizontal, 14).padding(.vertical, 8)
        .background(.quaternary.opacity(0.4))
    }

    @ViewBuilder
    private var stateLabel: some View {
        HStack(spacing: 7) {
            if session.isBusy { ProgressView().controlSize(.small).scaleEffect(0.7) }
            Text(text).font(.callout).foregroundStyle(.secondary)
        }
    }

    private var text: String {
        if let error = session.connectionError { return error }
        switch session.transcript.runState {
        case .idle: return "Ready"
        case .queued: return "Queued"
        case .running: return "Working"
        case .waitingForUser: return "Waiting for you"
        case .cancelling: return "Stopping"
        case .completed: return "Done"
        case .failed: return "Failed"
        case .cancelled: return "Cancelled"
        }
    }

    @ViewBuilder
    private var usageLabel: some View {
        let usage = session.transcript.usage
        if usage.totalTokens > 0 {
            // An unpriced model reports 0; showing "$0.00" would read as free.
            Text(
                usage.costIsKnown
                    ? "\(usage.totalTokens) tok · $\(String(format: "%.4f", usage.costUSD))"
                    : "\(usage.totalTokens) tok · cost n/a"
            )
            .font(.caption).foregroundStyle(.tertiary)
        }
    }
}

private struct Composer: View {
    @Bindable var session: RunSession
    @FocusState private var focused: Bool

    var body: some View {
        HStack(alignment: .bottom, spacing: 8) {
            TextField("Ask the harness to do something…", text: $session.draft, axis: .vertical)
                .textFieldStyle(.plain)
                .lineLimit(1...8)
                .focused($focused)
                .onSubmit { session.submit() }
                .padding(.horizontal, 12).padding(.vertical, 9)
                .background(.quaternary.opacity(0.5), in: .rect(cornerRadius: 10))
                .disabled(session.isBusy)

            Button {
                session.submit()
            } label: {
                Image(systemName: "arrow.up.circle.fill").font(.title2)
            }
            .buttonStyle(.plain)
            .disabled(!session.canSubmit)
        }
        .padding(12)
        .onAppear { focused = true }
    }
}

// MARK: - Work surface

private struct WorkSurface: View {
    let activity: ToolActivity?

    var body: some View {
        Group {
            if let activity {
                ScrollView {
                    VStack(alignment: .leading, spacing: 12) {
                        Text(activity.tool).font(.headline)
                        if !activity.arguments.isEmpty {
                            section("Arguments", activity.arguments)
                        }
                        if !activity.output.isEmpty {
                            section("Output", activity.output)
                        }
                    }
                    .padding(16)
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
            } else {
                VStack(spacing: 8) {
                    Image(systemName: "curlybraces.square")
                        .font(.system(size: 34)).foregroundStyle(.tertiary)
                    Text("Select a tool call to inspect it")
                        .font(.callout).foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .background(.background.secondary)
    }

    private func section(_ title: String, _ body: String) -> some View {
        VStack(alignment: .leading, spacing: 5) {
            Text(title).font(.caption.weight(.semibold)).foregroundStyle(.secondary)
            Text(body)
                .font(.callout.monospaced()).textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(10)
                .background(.quaternary.opacity(0.35), in: .rect(cornerRadius: 8))
        }
    }
}
