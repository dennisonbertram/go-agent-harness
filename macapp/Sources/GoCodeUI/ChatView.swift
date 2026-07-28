import HarnessKit
import SwiftUI

/// Conversation surface with an on-demand environment card over the transcript.
struct ChatView: View {
    @Bindable var project: ProjectSession
    @Bindable var run: RunSession
    @State private var selected: ToolActivity?
    // Baseline gap #3: a permanent HSplitView pane held 44% of the window's
    // area to show one tool call's JSON. Default it closed and let it open on
    // demand instead, matching Codex's floating, give-the-space-back panel.
    @State private var showInspector = false

    var body: some View {
        ZStack(alignment: .topTrailing) {
            VStack(spacing: Spacing.none) {
                TranscriptView(
                    items: run.transcript.items,
                    statusMessage: project.statusMessage,
                    run: run,
                    selected: $selected
                )
                Divider()
                if let plan = run.transcript.pendingPlan {
                    PlanApprovalView(plan: plan, run: run)
                } else if let prompt = run.pendingQuestions {
                    AskUserView(prompt: prompt) { run.answer($0) }
                } else if let approval = run.transcript.pendingApproval {
                    ApprovalBar(approval: approval, run: run)
                }
                Composer(project: project, run: run)
            }
            .frame(minWidth: Layout.chatMinimumWidth, idealWidth: Layout.chatIdealWidth)

            if showInspector {
                EnvironmentInspector(
                    project: project, activities: toolActivities, selected: $selected
                )
                .padding(Spacing.large)
            }
        }
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Button {
                    showInspector.toggle()
                } label: {
                    Image(systemName: showInspector ? "sidebar.trailing" : "sidebar.right")
                }
                .help(showInspector ? "Hide tool inspector" : "Show tool inspector")
                .accessibilityLabel(showInspector ? "Hide tool inspector" : "Show tool inspector")
            }
        }
        // Clicking a tool call is the reason the inspector exists; open it
        // automatically on selection rather than leaving the click looking
        // like it did nothing because the pane defaults to closed.
        .onChange(of: selected) { _, activity in
            if activity != nil { showInspector = true }
        }
    }

    private var toolActivities: [ToolActivity] {
        run.transcript.items.compactMap { item in
            guard case .toolActivity(let activity) = item.kind else { return nil }
            return activity
        }
    }
}

// MARK: - Transcript

struct TranscriptView: View {
    let items: [TranscriptItem]
    let statusMessage: String?
    @Bindable var run: RunSession
    @Binding var selected: ToolActivity?
    /// Auto-scroll only while the user is already at the bottom, so scrolling
    /// back to read is not yanked away mid-stream.
    @State private var pinnedToBottom = true

    var body: some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: Spacing.large) {
                    ForEach(items) { item in
                        row(for: item).id(item.id)
                    }
                    if run.isBusy {
                        InlineRunStatus(run: run, statusMessage: statusMessage)
                    }
                    Color.clear.frame(height: Spacing.hairline).id(bottomAnchor)
                }
                .padding(Spacing.large)
                .frame(maxWidth: Layout.chatContentMaximumWidth, alignment: .leading)
                .frame(maxWidth: .infinity, alignment: .center)
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
            ToolRow(activity: activity) {
                selected = activity
            }
        case .error(let message):
            ErrorRow(message: message)
        case .notice(let message):
            NoticeRow(message: message)
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
                .font(Typography.body).textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.top, 5)
        } label: {
            Label(
                messagesRemoved > 0
                    ? "History compacted — \(messagesRemoved) messages folded"
                    : "History compacted",
                systemImage: "arrow.down.right.and.arrow.up.left")
        }
        .font(Typography.caption)
        .foregroundStyle(Theme.foregroundTertiary)
        .padding(Spacing.comfortable)
        .compactElevatedSurface()
    }
}

/// Copies one whole message. Dragging cannot do this job: SwiftUI text
/// selection never spans separate `Text` views, so a reply split across
/// paragraphs and code blocks can only be taken whole by a button.
struct CopyMessageButton: View {
    let text: String
    @State private var copied = false

    var body: some View {
        Button {
            NSPasteboard.general.clearContents()
            NSPasteboard.general.setString(text, forType: .string)
            copied = true
            Task {
                try? await Task.sleep(for: .seconds(1.6))
                copied = false
            }
        } label: {
            Image(systemName: copied ? "checkmark" : "doc.on.doc")
                .font(Typography.caption)
                .foregroundStyle(
                    copied ? AnyShapeStyle(.tint) : AnyShapeStyle(Theme.foregroundQuaternary)
                )
                .contentShape(.rect)
        }
        .buttonStyle(.plain)
        .help(copied ? "Copied" : "Copy this message")
        .accessibilityLabel("Copy message")
    }
}

struct UserBubble: View {
    let text: String

    var body: some View {
        Text(text)
            .textSelection(.enabled)
            .padding(.horizontal, Spacing.inset)
            .padding(.vertical, Spacing.standard)
            .frame(minHeight: Layout.userMessageMinimumHeight, alignment: .leading)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(Theme.messageSurface, in: .rect(cornerRadius: CornerRadius.card))
    }
}

struct AssistantBubble: View {
    let message: AssistantMessage

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.standard) {
            ForEach(Array(MarkdownBlock.parse(message.text).enumerated()), id: \.offset) {
                _, block in
                switch block {
                case .paragraph(let body):
                    Text(.init(body)).textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                case .heading(let level, let text):
                    Text(.init(text))
                        .font(MarkdownBlock.headingFont(level)).fontWeight(.semibold)
                        .textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                case .unorderedListItem(let text):
                    MarkdownListRow(marker: "•", text: text)
                case .orderedListItem(let number, let text):
                    MarkdownListRow(marker: "\(number).", text: text)
                case .quote(let text):
                    MarkdownQuoteRow(text: text)
                case .rule:
                    Divider()
                case .code(let code, let language):
                    CodeBlock(code: code, language: language)
                }
            }
            if message.isStreaming {
                ProgressView().controlSize(.small)
            } else {
                CopyMessageButton(text: message.text)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
    }
}

/// `Text(.init:)` only renders *inline* markdown (bold, italic, links, inline
/// code): it has no concept of block structure, so headings/lists/quotes came
/// out as literal `#`/`-`/`>` characters with no line breaks between them.
/// This splits a reply into block-level pieces so each one can pick its own
/// view, while still handing inline markdown for the block's own text back to
/// `Text(.init:)`.
///
/// Tables are out of scope — no attempt is made to detect `|` rows.
/// Nested lists are out of scope too: every list item is flat regardless of
/// leading indentation, to keep this parser (which reruns on every streamed
/// token) cheap and simple.
enum MarkdownBlock: Equatable {
    case paragraph(String)
    case heading(level: Int, text: String)
    case unorderedListItem(String)
    case orderedListItem(number: Int, text: String)
    case quote(String)
    case rule
    case code(String, String?)

    static func headingFont(_ level: Int) -> Font {
        switch level {
        case 1: return .title
        case 2: return .title2
        case 3: return .title3
        case 4: return .headline
        case 5: return .subheadline
        default: return .footnote
        }
    }

    static func parse(_ markdown: String) -> [MarkdownBlock] {
        var blocks: [MarkdownBlock] = []
        var paragraph: [String] = []
        var code: [String] = []
        var language: String?
        var inFence = false

        // A blank line or the start of any other block ends the paragraph
        // that was accumulating; a bare run of plain lines does not.
        func flushParagraph() {
            guard !paragraph.isEmpty else { return }
            // Two trailing spaces is CommonMark's hard line break. Joining
            // with a bare "\n" reads fine here but `Text(.init:)` reflows a
            // soft newline away, which is exactly the collapsing this exists
            // to avoid.
            blocks.append(.paragraph(paragraph.joined(separator: "  \n")))
            paragraph = []
        }

        for line in markdown.components(separatedBy: .newlines) {
            if line.hasPrefix("```") {
                if inFence {
                    blocks.append(.code(code.joined(separator: "\n"), language))
                    code = []
                    language = nil
                    inFence = false
                } else {
                    flushParagraph()
                    let tag = line.dropFirst(3).trimmingCharacters(in: .whitespaces)
                    language = tag.isEmpty ? nil : tag
                    inFence = true
                }
                continue
            }
            if inFence {
                code.append(line)
                continue
            }

            if let block = heading(line) {
                flushParagraph()
                blocks.append(block)
            } else if let block = unorderedItem(line) {
                flushParagraph()
                blocks.append(block)
            } else if let block = orderedItem(line) {
                flushParagraph()
                blocks.append(block)
            } else if let block = blockQuote(line) {
                flushParagraph()
                blocks.append(block)
            } else if isRule(line) {
                flushParagraph()
                blocks.append(.rule)
            } else if line.trimmingCharacters(in: .whitespaces).isEmpty {
                flushParagraph()
            } else {
                paragraph.append(line)
            }
        }

        // An unterminated fence is normal mid-stream: show it as code anyway.
        if inFence, !code.isEmpty { blocks.append(.code(code.joined(separator: "\n"), language)) }
        flushParagraph()
        return blocks
    }

    private static func heading(_ line: String) -> MarkdownBlock? {
        let level = line.prefix(while: { $0 == "#" }).count
        guard (1...6).contains(level) else { return nil }
        let rest = line.dropFirst(level)
        guard rest.hasPrefix(" ") else { return nil }
        return .heading(level: level, text: rest.trimmingCharacters(in: .whitespaces))
    }

    private static func unorderedItem(_ line: String) -> MarkdownBlock? {
        guard let marker = line.first, "-*+".contains(marker) else { return nil }
        let rest = line.dropFirst()
        guard rest.hasPrefix(" ") else { return nil }
        return .unorderedListItem(rest.trimmingCharacters(in: .whitespaces))
    }

    private static func orderedItem(_ line: String) -> MarkdownBlock? {
        let digits = line.prefix(while: \.isNumber)
        guard !digits.isEmpty, let number = Int(digits) else { return nil }
        let rest = line.dropFirst(digits.count)
        guard rest.hasPrefix(". ") else { return nil }
        return .orderedListItem(
            number: number, text: rest.dropFirst(2).trimmingCharacters(in: .whitespaces))
    }

    /// Named apart from the `quote` case itself: a same-named helper with a
    /// matching single-`String` argument shape resolved ambiguously against
    /// the case's own initializer and silently returned the wrong thing.
    private static func blockQuote(_ line: String) -> MarkdownBlock? {
        guard line.hasPrefix(">") else { return nil }
        return .quote(line.dropFirst().trimmingCharacters(in: .whitespaces))
    }

    /// `---`, `***`, `___` (3+ of the same character, nothing else on the
    /// line) is CommonMark's thematic break. Checked before the list-item
    /// tests below would even matter: none of them accept a bare `-`/`*` run
    /// with no following space, so there is no real ambiguity to resolve.
    private static func isRule(_ line: String) -> Bool {
        let trimmed = line.trimmingCharacters(in: .whitespaces)
        guard let marker = trimmed.first, "-*_".contains(marker) else { return false }
        return trimmed.count >= 3 && trimmed.allSatisfy { $0 == marker }
    }
}

/// Hanging indent for list markers: the marker sits in its own fixed-width
/// column so a wrapped second line lands under the text, not under the
/// bullet/number.
struct MarkdownListRow: View {
    let marker: String
    let text: String
    var body: some View {
        HStack(alignment: .top, spacing: Spacing.small) {
            Text(marker).foregroundStyle(Theme.foregroundTertiary).frame(
                minWidth: 18, alignment: .trailing)
            Text(.init(text)).textSelection(.enabled)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

struct MarkdownQuoteRow: View {
    let text: String
    var body: some View {
        HStack(spacing: Spacing.standard) {
            Rectangle().fill(Theme.foregroundQuaternary).frame(width: IconSize.rule)
            Text(.init(text)).foregroundStyle(Theme.foregroundTertiary).textSelection(.enabled)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

struct CodeBlock: View {
    let code: String
    let language: String?
    @State private var copied = false

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.none) {
            HStack {
                Text(language ?? "code").font(Typography.detail).foregroundStyle(
                    Theme.foregroundTertiary)
                Spacer()
                Button(copied ? "Copied" : "Copy") {
                    NSPasteboard.general.clearContents()
                    NSPasteboard.general.setString(code, forType: .string)
                    copied = true
                }
                .buttonStyle(.plain).font(Typography.detail).foregroundStyle(
                    Theme.foregroundTertiary)
            }
            .padding(.horizontal, Spacing.comfortable).padding(.vertical, 5)

            ScrollView(.horizontal) {
                Text(code)
                    .font(Typography.code).textSelection(.enabled)
                    .padding(.horizontal, Spacing.comfortable).padding(.bottom, 9)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
        .compactElevatedSurface()
    }
}

struct ThinkingRow: View {
    let text: String
    @State private var expanded = false
    var body: some View {
        DisclosureGroup("Thinking", isExpanded: $expanded) {
            Text(text)
                .font(Typography.code).foregroundStyle(Theme.foregroundTertiary)
                .textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .font(Typography.caption).foregroundStyle(Theme.foregroundTertiary)
    }
}

struct ToolRow: View {
    let activity: ToolActivity
    let onSelect: () -> Void

    var body: some View {
        Button(action: onSelect) {
            VStack(alignment: .leading, spacing: Spacing.small) {
                HStack(spacing: Spacing.tight) {
                    Text(TranscriptActivityLabel.text(for: activity))
                        .font(Typography.caption)
                        .foregroundStyle(Theme.foregroundTertiary)
                    Text("›")
                        .font(Typography.caption)
                        .foregroundStyle(Theme.foregroundQuaternary)
                    Spacer(minLength: Spacing.none)
                }
                Rectangle()
                    .fill(Theme.separator)
                    .frame(height: Spacing.hairline)
            }
            .contentShape(.rect)
        }
        .buttonStyle(.plain)
        .accessibilityLabel(TranscriptActivityLabel.text(for: activity))
    }
}

/// Tool details live in the optional inspector; the transcript deliberately
/// summarizes work as one neutral line so it keeps the conversation's rhythm.
enum TranscriptActivityLabel {
    static func text(for activity: ToolActivity) -> String {
        switch activity.status {
        case .completed: return completed(durationMS: activity.durationMS)
        case .running: return "Working"
        case .failed: return "Tool failed"
        case .blocked: return "Waiting for approval"
        }
    }

    static func completed(durationMS: Int?) -> String {
        guard let durationMS, durationMS > 0 else { return "Worked" }
        let seconds = Int((Double(durationMS) / 1_000).rounded(.up))
        return "Worked for \(seconds)s"
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
        HStack(alignment: .top, spacing: Spacing.standard) {
            Image(systemName: "exclamationmark.triangle.fill").foregroundStyle(.red)
            VStack(alignment: .leading, spacing: Spacing.compact) {
                Text(message).textSelection(.enabled)
                    .frame(maxWidth: .infinity, alignment: .leading)
                // An error is the message most likely to be pasted elsewhere.
                CopyMessageButton(text: message)
                    .frame(maxWidth: .infinity, alignment: .trailing)
            }
        }
        .padding(Spacing.comfortable)
        .background(
            Color.red.opacity(StateOpacity.feedback), in: .rect(cornerRadius: CornerRadius.control))
    }
}

/// Server warnings that change what ran — most often the model being served by
/// a different provider than the one picked. Styled apart from errors: the run
/// still proceeds.
struct NoticeRow: View {
    let message: String
    var body: some View {
        HStack(alignment: .top, spacing: Spacing.standard) {
            Image(systemName: "info.circle.fill").foregroundStyle(.orange)
            Text(message).font(Typography.body).textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .padding(Spacing.comfortable)
        .background(
            Color.orange.opacity(StateOpacity.feedback),
            in: .rect(cornerRadius: CornerRadius.control))
    }
}

// MARK: - Status, approvals, questions

/// Run state belongs to the transcript while work is active; a permanent
/// footer makes a quiet conversation look like a log viewer.
struct InlineRunStatus: View {
    @Bindable var run: RunSession
    let statusMessage: String?

    var body: some View {
        MetadataRow(spacing: Spacing.standard) {
            if run.isBusy { ProgressView().controlSize(.small).scaleEffect(0.7) }
            Text(label).foregroundStyle(Theme.foregroundSecondary)
            if let statusMessage {
                Text("· \(statusMessage)")
                    .foregroundStyle(Theme.foregroundQuaternary)
                    .lineLimit(1)
            }
            Spacer()
            if run.isBusy {
                Button("Stop") { run.cancel() }.controlSize(.small)
            }
        }
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

/// Flattens a transcript to plain text for the clipboard, so the whole
/// conversation can be pasted into an issue or a message.
enum TranscriptText {
    static func plain(_ items: [TranscriptItem]) -> String {
        items.map(line).joined(separator: "\n\n")
    }

    private static func line(_ item: TranscriptItem) -> String {
        switch item.kind {
        case .userPrompt(let text): return "You: \(text)"
        case .assistantMessage(let message): return message.text
        case .thinking(let text): return "Thinking: \(text)"
        case .toolActivity(let activity):
            return "[\(activity.tool)] \(ToolSummary.describe(activity))"
        case .error(let message): return "Error: \(message)"
        case .notice(let message): return "Note: \(message)"
        case .compaction(let summary, let removed):
            return "History compacted (\(removed) messages folded): \(summary)"
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
            .font(Typography.caption).foregroundStyle(Theme.foregroundQuaternary)
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
        VStack(alignment: .leading, spacing: Spacing.standard) {
            HStack(spacing: Spacing.standard) {
                Image(systemName: "hand.raised.fill").foregroundStyle(.orange)
                Text("Allow **\(approval.tool)** to run?")
                Spacer()
                Button(showArguments ? "Hide" : "Details") { showArguments.toggle() }
                    .buttonStyle(.plain).font(Typography.caption).foregroundStyle(
                        Theme.foregroundTertiary)
                Button("Deny") { run.deny() }
                Button("Allow") { run.approve() }.buttonStyle(.borderedProminent)
            }
            if showArguments {
                ScrollView {
                    Text(approval.arguments)
                        .font(Typography.codeCaption).textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                .frame(maxHeight: 120)
                .padding(Spacing.standard)
                .background(Theme.surfaceElevated, in: .rect(cornerRadius: CornerRadius.code))
            }
        }
        // Same 16pt left inset as the transcript column and the status bar.
        .padding(.horizontal, Spacing.large).padding(.vertical, 9)
        .background(Color.orange.opacity(StateOpacity.emphasis))
    }
}

struct AskUserView: View {
    let prompt: AskUserPrompt
    let onAnswer: ([String: String]) -> Void
    @State private var answers: [String: String] = [:]

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.inset) {
            Label("The agent needs your input", systemImage: "questionmark.bubble")
                .font(Typography.body.weight(.medium))

            ForEach(prompt.questions) { question in
                VStack(alignment: .leading, spacing: Spacing.small) {
                    Text(question.question).font(Typography.body)
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
                                            Text(detail).font(Typography.caption)
                                                .foregroundStyle(Theme.foregroundTertiary)
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
                        .font(Typography.caption).foregroundStyle(Theme.foregroundTertiary)
                }
                Spacer()
                Button("Send") { onAnswer(answers) }
                    .buttonStyle(.borderedProminent)
                    .disabled(answers.count < prompt.questions.count)
            }
        }
        // Same 16pt left inset as the transcript column and the status bar.
        .padding(.horizontal, Spacing.large).padding(.vertical, 14)
        .background(Theme.accent.opacity(StateOpacity.subtle))
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
        VStack(alignment: .leading, spacing: Spacing.standard) {
            if !mentions.isEmpty {
                MentionPopup(matches: mentions) { match in
                    run.draft = MentionQuery.replacing(run.draft, with: match.relativePath)
                    mentions = []
                }
            }

            // One elevated card holds every composer control (baseline gap
            // #2): the field, send button, model picker and plan toggle used
            // to read as three unrelated regions — the field, a right-hand
            // gutter for send, and a separate borderless strip below.
            VStack(alignment: .leading, spacing: Spacing.comfortable) {
                TextField(placeholder, text: $run.draft, axis: .vertical)
                    .textFieldStyle(.plain)
                    .lineLimit(1...10)
                    .focused($focused)
                    .onSubmit(send)
                    .onChange(of: run.draft) { _, text in updateMentions(for: text) }

                HStack(spacing: Spacing.comfortable) {
                    ModelChip(project: project)
                    Toggle("Plan mode", isOn: $project.planMode)
                        .toggleStyle(.checkbox).font(Typography.caption)
                        .help("Restrict the agent to writing a plan file until you approve it")
                    Spacer()
                    Button("New") { project.newConversation() }
                        .buttonStyle(.plain).font(Typography.caption).foregroundStyle(
                            Theme.foregroundTertiary)

                    Button(action: send) {
                        Image(
                            systemName: run.canSteer
                                ? "arrow.turn.up.right" : "arrow.up.circle.fill"
                        )
                        // A title-sized SF Symbol makes its filled disc half
                        // the target control; this restores the send target's
                        // intended visual weight without changing its glyph.
                        .font(.system(size: 34))
                    }
                    .buttonStyle(.plain)
                    .disabled(run.draft.trimmed.isEmpty)
                    .help(run.canSteer ? "Steer the running task" : "Send")
                    .accessibilityLabel(run.canSteer ? "Steer the running task" : "Send message")
                }
            }
            .padding(.horizontal, Spacing.large).padding(.vertical, Spacing.inset)
            // The minimum keeps the idle composer at the reference height
            // while allowing a multi-line draft to grow instead of clipping.
            .frame(minHeight: 117)
            .background(Theme.surfaceElevated, in: .rect(cornerRadius: CornerRadius.composer))
            .frame(maxWidth: Layout.chatContentMaximumWidth)
            .frame(maxWidth: .infinity, alignment: .center)
        }
        .padding(.horizontal, Spacing.large)
        .padding(.top, Spacing.standard)
        // A real bottom inset (was 0 — the old control strip ran flush to the
        // window edge) so the card reads as inset chrome, not a footer.
        .padding(.bottom, Spacing.section)
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
            ForEach(usableProviders, id: \.self) { provider in
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
            // Hiding models without saying so reads as "my model disappeared".
            // One disabled line names the count and the reason.
            if !hiddenSummary.isEmpty {
                Divider()
                Text(hiddenSummary).font(Typography.caption)
            }
        } label: {
            Label(project.selectedModel ?? "Server default", systemImage: "cpu")
                .font(Typography.caption)
        }
        .menuStyle(.borderlessButton)
        .fixedSize()
    }

    /// Provider names that can actually run a model, or nil when the provider
    /// list is unavailable.
    ///
    /// Failing open matters here: `refreshCatalog` swallows a failed fetch into
    /// an empty array, and filtering against that would hide every model and
    /// leave a picker containing nothing but "Server default". An over-full
    /// menu is a worse menu; an empty one looks like the app is broken.
    private var usableProviderNames: Set<String>? {
        guard !project.providers.isEmpty else { return nil }
        return Set(project.providers.filter(\.isUsable).map(\.name))
    }

    /// Only providers whose credentials exist and are not known-broken. A model
    /// the user cannot run has no business being offered: picking it fails the
    /// run, and the failure names a provider rather than the model they chose.
    private var usableProviders: [String] {
        let all = Set(project.models.map(\.provider))
        guard let usable = usableProviderNames else { return all.sorted() }
        return Array(all.intersection(usable)).sorted()
    }

    private func models(for provider: String) -> [ModelInfo] {
        project.models.filter { $0.provider == provider }.sorted { $0.id < $1.id }
    }

    private var hiddenSummary: String {
        guard let usable = usableProviderNames else { return "" }
        let hidden = project.models.filter { !usable.contains($0.provider) }
        guard !hidden.isEmpty else { return "" }

        let broken = project.providers
            .filter { $0.configured && $0.health == "failed" }
            .map(\.name).sorted()
        if !broken.isEmpty {
            return "\(hidden.count) models hidden — \(broken.joined(separator: ", ")) "
                + "\(broken.count == 1 ? "needs" : "need") re-authentication"
        }
        return "\(hidden.count) models hidden — no credentials for their providers"
    }
}

// MARK: - Environment

/// Codex groups environment state by kind. This card deliberately has no
/// height constraint: it occupies only the space its present cards require,
/// instead of reserving a permanent full-height column for an optional detail.
struct EnvironmentInspector: View {
    @Bindable var project: ProjectSession
    let activities: [ToolActivity]
    @Binding var selected: ToolActivity?

    private var subagents: [TaskInfo] { project.tasks.filter { $0.type == "subagent" } }
    private var backgroundTasks: [TaskInfo] { project.tasks.filter { $0.type != "subagent" } }

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.standard) {
            Text("Environment")
                .font(Typography.heading)

            if !project.rewindPoints.isEmpty {
                EnvironmentCard(title: "Changes", icon: "arrow.uturn.backward.circle") {
                    ForEach(project.rewindPoints) { point in
                        CheckpointSummary(point: point)
                    }
                }
            }

            if !subagents.isEmpty {
                EnvironmentCard(title: "Subagents", icon: "person.2") {
                    ForEach(subagents) { task in TaskSummary(task: task) }
                }
            }

            if !backgroundTasks.isEmpty || !activities.isEmpty {
                EnvironmentCard(title: "Background processes", icon: "gearshape.2") {
                    ForEach(backgroundTasks) { task in TaskSummary(task: task) }
                    ForEach(activities) { activity in
                        ToolActivitySummary(
                            activity: activity, isSelected: selected?.id == activity.id
                        ) {
                            selected = activity
                        }
                    }
                    if let selected {
                        Divider()
                        ToolActivityDetail(activity: selected)
                    }
                }
            }

            if project.rewindPoints.isEmpty && subagents.isEmpty && backgroundTasks.isEmpty
                && activities.isEmpty
            {
                EnvironmentCard(title: "Environment", icon: "sidebar.right") {
                    Text("No changes or background work yet.")
                        .font(Typography.caption)
                        .foregroundStyle(Theme.foregroundTertiary)
                }
            }
        }
        .padding(Spacing.inset)
        .frame(width: Layout.inspectorCardWidth, alignment: .leading)
        .fixedSize(horizontal: false, vertical: true)
        .cardStyle()
        .task {
            await project.refreshRewindPoints()
            await project.refreshActivity()
        }
    }
}

/// Reused card chrome keeps each environment kind distinct without making the
/// inspector a visually undifferentiated column again.
private struct EnvironmentCard<Content: View>: View {
    let title: String
    let icon: String
    @ViewBuilder let content: Content

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.small) {
            Label(title, systemImage: icon)
                .font(Typography.caption.weight(.semibold))
                .foregroundStyle(Theme.foregroundSecondary)
            VStack(alignment: .leading, spacing: Spacing.small) { content }
        }
        .padding(Spacing.standard)
        .frame(maxWidth: .infinity, alignment: .leading)
        .compactElevatedSurface()
    }
}

private struct CheckpointSummary: View {
    let point: RewindPoint

    var body: some View {
        HStack(spacing: Spacing.small) {
            Text(point.tool.map { "Before \($0)" } ?? "Checkpoint")
                .font(Typography.caption)
                .lineLimit(1)
            Spacer(minLength: Spacing.none)
            if let date = point.createdAt {
                Text(date.formatted(date: .omitted, time: .shortened))
                    .font(Typography.detail)
                    .foregroundStyle(Theme.foregroundTertiary)
            }
        }
    }
}

private struct TaskSummary: View {
    let task: TaskInfo

    var body: some View {
        HStack(spacing: Spacing.small) {
            Text(task.label).font(Typography.caption).lineLimit(1)
            Spacer(minLength: Spacing.none)
            Text(task.status).font(Typography.detail).foregroundStyle(Theme.foregroundTertiary)
        }
    }
}

private struct ToolActivitySummary: View {
    let activity: ToolActivity
    let isSelected: Bool
    let onSelect: () -> Void

    var body: some View {
        Button(action: onSelect) {
            HStack(spacing: Spacing.small) {
                Text(activity.tool).font(Typography.caption).lineLimit(1)
                Spacer(minLength: Spacing.none)
                Text(String(describing: activity.status))
                    .font(Typography.detail)
                    .foregroundStyle(Theme.foregroundTertiary)
            }
            .padding(Spacing.compact)
            .background(
                isSelected ? Theme.surfaceHighest : .clear,
                in: .rect(cornerRadius: CornerRadius.tag)
            )
            .contentShape(.rect)
        }
        .buttonStyle(.plain)
    }
}

/// The selected activity stays in its kind's card so detail does not cause a
/// second, permanent inspector region to reappear beside the conversation.
private struct ToolActivityDetail: View {
    let activity: ToolActivity

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.small) {
            if let edit = ToolEdit(tool: activity.tool, arguments: activity.arguments) {
                DiffView(edit: edit)
            } else if !activity.arguments.isEmpty {
                LabelledCode(title: "Arguments", body: activity.arguments)
            }
            if !activity.output.isEmpty {
                LabelledCode(title: "Output", body: activity.output)
            }
        }
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
            Text(title).font(Typography.caption.weight(.semibold)).foregroundStyle(
                Theme.foregroundQuaternary)
            ScrollView(.horizontal) {
                Text(content)
                    .font(Typography.code).textSelection(.enabled)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .padding(Spacing.comfortable)
            .compactElevatedSurface()
        }
    }
}

/// File suggestions for an in-progress `@mention`.
struct MentionPopup: View {
    let matches: [FileCompletion.Match]
    let onPick: (FileCompletion.Match) -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.none) {
            ForEach(matches) { match in
                Button {
                    onPick(match)
                } label: {
                    HStack(spacing: Spacing.small) {
                        Image(systemName: "doc").font(Typography.detail).foregroundStyle(
                            Theme.foregroundTertiary)
                        Text(match.relativePath)
                            .font(Typography.codeCaption)
                            .lineLimit(1).truncationMode(.head)
                        Spacer()
                    }
                    .padding(.horizontal, Spacing.comfortable).padding(.vertical, 5)
                    .contentShape(.rect)
                }
                .buttonStyle(.plain)
            }
        }
        .compactElevatedSurface()
    }
}

/// Leaving plan mode: show the plan, then approve with a chosen approach.
struct PlanApprovalView: View {
    let plan: PendingPlan
    @Bindable var run: RunSession
    @State private var selected: String?

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.comfortable) {
            Label("Ready to leave plan mode", systemImage: "list.bullet.clipboard")
                .font(Typography.body.weight(.medium))

            ScrollView {
                Text(.init(plan.plan))
                    .textSelection(.enabled)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .frame(maxHeight: 220)
            .padding(Spacing.comfortable)
            .compactElevatedSurface()

            if !plan.options.isEmpty {
                Text("Approach").font(Typography.caption).foregroundStyle(
                    Theme.foregroundQuaternary)
                ForEach(plan.options) { option in
                    Button {
                        selected = option.id
                    } label: {
                        HStack(alignment: .top, spacing: 7) {
                            Image(
                                systemName: selected == option.id
                                    ? "largecircle.fill.circle" : "circle")
                            VStack(alignment: .leading, spacing: Spacing.hairline) {
                                Text(option.label)
                                if let detail = option.description, !detail.isEmpty {
                                    Text(detail).font(Typography.caption).foregroundStyle(
                                        Theme.foregroundTertiary)
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
        // Same 16pt left inset as the transcript column and the status bar.
        .padding(.horizontal, Spacing.large).padding(.vertical, 14)
        .background(Theme.accent.opacity(StateOpacity.subtle))
    }
}
