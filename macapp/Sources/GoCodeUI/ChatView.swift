import HarnessKit
import SwiftUI

/// Two-pane conversation surface: transcript + composer left, inspector right.
struct ChatView: View {
    @Bindable var project: ProjectSession
    @Bindable var run: RunSession
    @State private var selected: ToolActivity?

    var body: some View {
        HSplitView {
            VStack(spacing: 0) {
                TranscriptView(items: run.transcript.items, selected: $selected)
                Divider()
                if let plan = run.transcript.pendingPlan {
                    PlanApprovalView(plan: plan, run: run)
                } else if let prompt = run.pendingQuestions {
                    AskUserView(prompt: prompt) { run.answer($0) }
                } else if let approval = run.transcript.pendingApproval {
                    ApprovalBar(approval: approval, run: run)
                } else {
                    StatusBar(project: project, run: run)
                }
                Composer(project: project, run: run)
            }
            .frame(minWidth: 400, idealWidth: 520)

            InspectorPane(activity: selected)
                .frame(minWidth: 380)
        }
    }
}

// MARK: - Transcript

struct TranscriptView: View {
    let items: [TranscriptItem]
    @Binding var selected: ToolActivity?
    /// Auto-scroll only while the user is already at the bottom, so scrolling
    /// back to read is not yanked away mid-stream.
    @State private var pinnedToBottom = true

    var body: some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 14) {
                    ForEach(items) { item in
                        row(for: item).id(item.id)
                    }
                    Color.clear.frame(height: 1).id(bottomAnchor)
                }
                .padding(16)
                .frame(maxWidth: .infinity, alignment: .leading)
            }
            .onChange(of: items.last?.id) { _, _ in scrollIfPinned(proxy) }
            .onChange(of: lastItemLength) { _, _ in scrollIfPinned(proxy) }
        }
    }

    private let bottomAnchor = "transcript-bottom"

    /// Streaming mutates the last item in place rather than appending, so the
    /// item id alone does not change as text arrives.
    private var lastItemLength: Int {
        guard case .assistantMessage(let message) = items.last?.kind else { return 0 }
        return message.text.count
    }

    private func scrollIfPinned(_ proxy: ScrollViewProxy) {
        guard pinnedToBottom else { return }
        withAnimation(.easeOut(duration: 0.12)) {
            proxy.scrollTo(bottomAnchor, anchor: .bottom)
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
            ToolRow(activity: activity, isSelected: selected?.callID == activity.callID) {
                selected = activity
            }
        case .error(let message):
            ErrorRow(message: message)
        case .compaction(let summary, let removed):
            CompactionRow(summary: summary, messagesRemoved: removed)
        }
    }
}

/// Collapsed by default, like the TUI's Ctrl+O block: it marks that history was
/// folded without burying the conversation in the summary.
struct CompactionRow: View {
    let summary: String
    let messagesRemoved: Int
    @State private var expanded = false

    var body: some View {
        DisclosureGroup(isExpanded: $expanded) {
            Text(summary)
                .font(.callout).textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.top, 5)
        } label: {
            Label(
                messagesRemoved > 0
                    ? "History compacted — \(messagesRemoved) messages folded"
                    : "History compacted",
                systemImage: "arrow.down.right.and.arrow.up.left")
        }
        .font(.caption)
        .foregroundStyle(.secondary)
        .padding(10)
        .background(.quaternary.opacity(0.3), in: .rect(cornerRadius: 8))
    }
}

struct UserBubble: View {
    let text: String
    var body: some View {
        HStack {
            Spacer(minLength: 48)
            Text(text)
                .textSelection(.enabled)
                .padding(.horizontal, 12).padding(.vertical, 8)
                .background(Color.accentColor.opacity(0.15), in: .rect(cornerRadius: 10))
        }
    }
}

struct AssistantBubble: View {
    let message: AssistantMessage
    var body: some View {
        HStack(alignment: .top, spacing: 8) {
            Image(systemName: "sparkle")
                .foregroundStyle(.tint).font(.caption).padding(.top, 3)
            VStack(alignment: .leading, spacing: 8) {
                ForEach(Array(MarkdownBlock.parse(message.text).enumerated()), id: \.offset) {
                    _, block in
                    switch block {
                    case .text(let body):
                        Text(.init(body)).textSelection(.enabled)
                            .frame(maxWidth: .infinity, alignment: .leading)
                    case .code(let code, let language):
                        CodeBlock(code: code, language: language)
                    }
                }
            }
            if message.isStreaming {
                ProgressView().controlSize(.small)
            }
        }
    }
}

/// Fenced code needs monospace and a copy button; `Text(.init:)` renders
/// markdown but collapses fenced blocks into inline styling.
enum MarkdownBlock {
    case text(String)
    case code(String, String?)

    static func parse(_ markdown: String) -> [MarkdownBlock] {
        var blocks: [MarkdownBlock] = []
        var buffer: [String] = []
        var code: [String] = []
        var language: String?
        var inFence = false

        for line in markdown.components(separatedBy: .newlines) {
            if line.hasPrefix("```") {
                if inFence {
                    blocks.append(.code(code.joined(separator: "\n"), language))
                    code = []
                    language = nil
                    inFence = false
                } else {
                    if !buffer.isEmpty {
                        blocks.append(.text(buffer.joined(separator: "\n")))
                        buffer = []
                    }
                    let tag = line.dropFirst(3).trimmingCharacters(in: .whitespaces)
                    language = tag.isEmpty ? nil : tag
                    inFence = true
                }
                continue
            }
            if inFence { code.append(line) } else { buffer.append(line) }
        }

        // An unterminated fence is normal mid-stream: show it as code anyway.
        if inFence, !code.isEmpty { blocks.append(.code(code.joined(separator: "\n"), language)) }
        if !buffer.isEmpty { blocks.append(.text(buffer.joined(separator: "\n"))) }
        return blocks
    }
}

struct CodeBlock: View {
    let code: String
    let language: String?
    @State private var copied = false

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack {
                Text(language ?? "code").font(.caption2).foregroundStyle(.secondary)
                Spacer()
                Button(copied ? "Copied" : "Copy") {
                    NSPasteboard.general.clearContents()
                    NSPasteboard.general.setString(code, forType: .string)
                    copied = true
                }
                .buttonStyle(.plain).font(.caption2).foregroundStyle(.secondary)
            }
            .padding(.horizontal, 10).padding(.vertical, 5)

            ScrollView(.horizontal) {
                Text(code)
                    .font(.callout.monospaced()).textSelection(.enabled)
                    .padding(.horizontal, 10).padding(.bottom, 9)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
        .background(.quaternary.opacity(0.4), in: .rect(cornerRadius: 8))
    }
}

struct ThinkingRow: View {
    let text: String
    @State private var expanded = false
    var body: some View {
        DisclosureGroup("Thinking", isExpanded: $expanded) {
            Text(text)
                .font(.callout.monospaced()).foregroundStyle(.secondary)
                .textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .font(.caption).foregroundStyle(.secondary)
    }
}

struct ToolRow: View {
    let activity: ToolActivity
    let isSelected: Bool
    let onSelect: () -> Void

    var body: some View {
        Button(action: onSelect) {
            HStack(spacing: 8) {
                statusIcon.frame(width: 15)
                Text(activity.tool).font(.callout.weight(.medium))
                Text(ToolSummary.describe(activity))
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

    @ViewBuilder
    private var statusIcon: some View {
        switch activity.status {
        case .running: ProgressView().controlSize(.small).scaleEffect(0.65)
        case .completed: Image(systemName: "checkmark.circle.fill").foregroundStyle(.green)
        case .failed: Image(systemName: "xmark.circle.fill").foregroundStyle(.red)
        // Blocked is a policy decision, not a failure.
        case .blocked: Image(systemName: "hand.raised.fill").foregroundStyle(.orange)
        }
    }
}

/// Turns raw tool arguments into something scannable: `edit {"path":"a/b.swift"}`
/// reads as `a/b.swift`.
enum ToolSummary {
    static func describe(_ activity: ToolActivity) -> String {
        guard let data = activity.arguments.data(using: .utf8),
            let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any]
        else { return activity.arguments }

        for key in ["path", "file_path", "command", "pattern", "query", "url"] {
            if let value = object[key] as? String, !value.isEmpty { return value }
        }
        return object.keys.sorted().joined(separator: ", ")
    }
}

struct ErrorRow: View {
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

// MARK: - Status, approvals, questions

struct StatusBar: View {
    @Bindable var project: ProjectSession
    @Bindable var run: RunSession

    var body: some View {
        HStack(spacing: 10) {
            if run.isBusy { ProgressView().controlSize(.small).scaleEffect(0.7) }
            Text(label).font(.callout).foregroundStyle(.secondary)
            if let message = project.statusMessage {
                Text("· \(message)").font(.caption).foregroundStyle(.tertiary).lineLimit(1)
            }
            Spacer()
            UsageLabel(usage: run.transcript.usage)
            if run.isBusy {
                Button("Stop") { run.cancel() }.controlSize(.small)
            }
        }
        .padding(.horizontal, 14).padding(.vertical, 7)
        .background(.quaternary.opacity(0.4))
    }

    private var label: String {
        if let error = run.connectionError { return error }
        switch run.transcript.runState {
        case .idle: return run.planMode ? "Plan mode — ready" : "Ready"
        case .queued: return "Starting"
        case .running: return "Working"
        case .waitingForUser: return "Waiting for you"
        case .cancelling: return "Stopping — press Stop again to force"
        case .completed: return "Done"
        case .failed: return "Failed"
        case .cancelled: return "Cancelled"
        }
    }
}

struct UsageLabel: View {
    let usage: UsageTotals
    var body: some View {
        if usage.totalTokens > 0 {
            // An unpriced model reports 0; "$0.00" would read as free.
            Text(
                usage.costIsKnown
                    ? "\(usage.totalTokens) tok · $\(String(format: "%.4f", usage.costUSD))"
                    : "\(usage.totalTokens) tok · cost n/a"
            )
            .font(.caption).foregroundStyle(.tertiary)
        }
    }
}

/// Approvals are the highest-stakes interaction here, so Deny is the plain
/// action and nothing is bound to Return.
struct ApprovalBar: View {
    let approval: PendingApproval
    @Bindable var run: RunSession
    @State private var showArguments = false

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 8) {
                Image(systemName: "hand.raised.fill").foregroundStyle(.orange)
                Text("Allow **\(approval.tool)** to run?")
                Spacer()
                Button(showArguments ? "Hide" : "Details") { showArguments.toggle() }
                    .buttonStyle(.plain).font(.caption).foregroundStyle(.secondary)
                Button("Deny") { run.deny() }
                Button("Allow") { run.approve() }.buttonStyle(.borderedProminent)
            }
            if showArguments {
                ScrollView {
                    Text(approval.arguments)
                        .font(.caption.monospaced()).textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                .frame(maxHeight: 120)
                .padding(8)
                .background(.quaternary.opacity(0.4), in: .rect(cornerRadius: 6))
            }
        }
        .padding(.horizontal, 14).padding(.vertical, 9)
        .background(Color.orange.opacity(0.12))
    }
}

struct AskUserView: View {
    let prompt: AskUserPrompt
    let onAnswer: ([String: String]) -> Void
    @State private var answers: [String: String] = [:]

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Label("The agent needs your input", systemImage: "questionmark.bubble")
                .font(.callout.weight(.medium))

            ForEach(prompt.questions) { question in
                VStack(alignment: .leading, spacing: 6) {
                    Text(question.question).font(.callout)
                    if question.isFreeform {
                        TextField(
                            "Your answer",
                            text: Binding(
                                get: { answers[question.id] ?? "" },
                                set: { answers[question.id] = $0 }))
                    } else {
                        ForEach(question.options ?? [], id: \.label) { option in
                            Button {
                                answers[question.id] = option.label
                            } label: {
                                HStack {
                                    Image(
                                        systemName: answers[question.id] == option.label
                                            ? "largecircle.fill.circle" : "circle")
                                    VStack(alignment: .leading) {
                                        Text(option.label)
                                        if let detail = option.description, !detail.isEmpty {
                                            Text(detail).font(.caption)
                                                .foregroundStyle(.secondary)
                                        }
                                    }
                                    Spacer()
                                }
                            }
                            .buttonStyle(.plain)
                        }
                    }
                }
            }

            HStack {
                if let deadline = prompt.deadlineAt {
                    Text("Answer by \(deadline.formatted(date: .omitted, time: .shortened))")
                        .font(.caption).foregroundStyle(.secondary)
                }
                Spacer()
                Button("Send") { onAnswer(answers) }
                    .buttonStyle(.borderedProminent)
                    .disabled(answers.count < prompt.questions.count)
            }
        }
        .padding(14)
        .background(Color.accentColor.opacity(0.08))
    }
}

// MARK: - Composer

struct Composer: View {
    @Bindable var project: ProjectSession
    @Bindable var run: RunSession
    @FocusState private var focused: Bool
    @State private var mentions: [FileCompletion.Match] = []
    @State private var mentionTask: Task<Void, Never>?

    var body: some View {
        VStack(spacing: 8) {
            if !mentions.isEmpty {
                MentionPopup(matches: mentions) { match in
                    run.draft = MentionQuery.replacing(run.draft, with: match.relativePath)
                    mentions = []
                }
            }
            HStack(alignment: .bottom, spacing: 8) {
                TextField(placeholder, text: $run.draft, axis: .vertical)
                    .textFieldStyle(.plain)
                    .lineLimit(1...10)
                    .focused($focused)
                    .onSubmit(send)
                    .onChange(of: run.draft) { _, text in updateMentions(for: text) }
                    .padding(.horizontal, 12).padding(.vertical, 9)
                    .background(.quaternary.opacity(0.5), in: .rect(cornerRadius: 10))

                Button(action: send) {
                    Image(systemName: run.canSteer ? "arrow.turn.up.right" : "arrow.up.circle.fill")
                        .font(.title2)
                }
                .buttonStyle(.plain)
                .disabled(run.draft.trimmed.isEmpty)
                .help(run.canSteer ? "Steer the running task" : "Send")
            }

            HStack(spacing: 10) {
                ModelChip(project: project)
                Toggle("Plan mode", isOn: $project.planMode)
                    .toggleStyle(.checkbox).font(.caption)
                    .help("Restrict the agent to writing a plan file until you approve it")
                Spacer()
                Button("New") { project.newConversation() }
                    .buttonStyle(.plain).font(.caption).foregroundStyle(.secondary)
            }
        }
        .padding(12)
        .onAppear { focused = true }
    }

    /// Debounced and cancellable: a large repo has hundreds of thousands of
    /// files and the composer must stay responsive while typing.
    private func updateMentions(for text: String) {
        mentionTask?.cancel()
        guard let query = MentionQuery.current(in: text) else {
            mentions = []
            return
        }
        let completion = FileCompletion(roots: [project.workspace])
        mentionTask = Task {
            try? await Task.sleep(for: .milliseconds(120))
            guard !Task.isCancelled else { return }
            let found = await completion.matches(for: query, limit: 8)
            guard !Task.isCancelled else { return }
            mentions = found
        }
    }

    private var placeholder: String {
        run.canSteer ? "Steer the running task…" : "Ask the harness to do something…"
    }

    /// While a run is active the same control steers instead of queueing a
    /// second run, matching the TUI's mid-turn steering.
    private func send() {
        if run.canSteer {
            run.steer()
        } else if run.canSubmit {
            project.submit()
        }
    }
}

struct ModelChip: View {
    @Bindable var project: ProjectSession

    var body: some View {
        Menu {
            Button("Server default") { project.selectedModel = nil }
            Divider()
            ForEach(groupedProviders, id: \.self) { provider in
                Menu(provider) {
                    ForEach(models(for: provider)) { model in
                        Button {
                            project.selectedModel = model.id
                        } label: {
                            // The TUI picker shows neither; both drive the choice.
                            Text(model.priceSummary.map { "\(model.id) — \($0)" } ?? model.id)
                        }
                    }
                }
            }
        } label: {
            Label(project.selectedModel ?? "Server default", systemImage: "cpu")
                .font(.caption)
        }
        .menuStyle(.borderlessButton)
        .fixedSize()
    }

    private var groupedProviders: [String] {
        Array(Set(project.models.map(\.provider))).sorted()
    }

    private func models(for provider: String) -> [ModelInfo] {
        project.models.filter { $0.provider == provider }.sorted { $0.id < $1.id }
    }
}

// MARK: - Inspector

struct InspectorPane: View {
    let activity: ToolActivity?

    var body: some View {
        Group {
            if let activity {
                ScrollView {
                    VStack(alignment: .leading, spacing: 12) {
                        HStack {
                            Text(activity.tool).font(.headline)
                            Spacer()
                            Text(String(describing: activity.status))
                                .font(.caption).foregroundStyle(.secondary)
                        }
                        // An edit carries its before/after text, so it can be
                        // shown as a diff instead of raw JSON arguments.
                        if let edit = ToolEdit(tool: activity.tool, arguments: activity.arguments) {
                            DiffView(edit: edit)
                        } else if !activity.arguments.isEmpty {
                            LabelledCode(title: "Arguments", body: activity.arguments)
                        }
                        if !activity.output.isEmpty {
                            LabelledCode(title: "Output", body: activity.output)
                        }
                    }
                    .padding(16)
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
            } else {
                VStack(spacing: 8) {
                    Image(systemName: "sidebar.right")
                        .font(.system(size: 30)).foregroundStyle(.tertiary)
                    Text("Select a tool call to inspect it")
                        .font(.callout).foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .background(.background.secondary)
    }
}

struct LabelledCode: View {
    let title: String
    let content: String

    init(title: String, body: String) {
        self.title = title
        self.content = body
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 5) {
            Text(title).font(.caption.weight(.semibold)).foregroundStyle(.secondary)
            ScrollView(.horizontal) {
                Text(content)
                    .font(.callout.monospaced()).textSelection(.enabled)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .padding(10)
            .background(.quaternary.opacity(0.35), in: .rect(cornerRadius: 8))
        }
    }
}

/// File suggestions for an in-progress `@mention`.
struct MentionPopup: View {
    let matches: [FileCompletion.Match]
    let onPick: (FileCompletion.Match) -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            ForEach(matches) { match in
                Button {
                    onPick(match)
                } label: {
                    HStack(spacing: 6) {
                        Image(systemName: "doc").font(.caption2).foregroundStyle(.secondary)
                        Text(match.relativePath)
                            .font(.caption.monospaced())
                            .lineLimit(1).truncationMode(.head)
                        Spacer()
                    }
                    .padding(.horizontal, 10).padding(.vertical, 5)
                    .contentShape(.rect)
                }
                .buttonStyle(.plain)
            }
        }
        .background(.quaternary.opacity(0.5), in: .rect(cornerRadius: 8))
    }
}

/// Leaving plan mode: show the plan, then approve with a chosen approach.
struct PlanApprovalView: View {
    let plan: PendingPlan
    @Bindable var run: RunSession
    @State private var selected: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Label("Ready to leave plan mode", systemImage: "list.bullet.clipboard")
                .font(.callout.weight(.medium))

            ScrollView {
                Text(.init(plan.plan))
                    .textSelection(.enabled)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .frame(maxHeight: 220)
            .padding(10)
            .background(.quaternary.opacity(0.35), in: .rect(cornerRadius: 8))

            if !plan.options.isEmpty {
                Text("Approach").font(.caption).foregroundStyle(.secondary)
                ForEach(plan.options) { option in
                    Button {
                        selected = option.id
                    } label: {
                        HStack(alignment: .top, spacing: 7) {
                            Image(
                                systemName: selected == option.id
                                    ? "largecircle.fill.circle" : "circle")
                            VStack(alignment: .leading, spacing: 1) {
                                Text(option.label)
                                if let detail = option.description, !detail.isEmpty {
                                    Text(detail).font(.caption).foregroundStyle(.secondary)
                                }
                            }
                            Spacer()
                        }
                    }
                    .buttonStyle(.plain)
                }
            }

            HStack {
                Spacer()
                Button("Keep Planning") { run.deny() }
                Button("Approve") { run.approve(option: selected) }
                    .buttonStyle(.borderedProminent)
                    // With approaches offered, one must be chosen.
                    .disabled(!plan.options.isEmpty && selected == nil)
            }
        }
        .padding(14)
        .background(Color.accentColor.opacity(0.08))
    }
}
