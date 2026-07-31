import Foundation

public enum RunState: Sendable, Hashable {
    case idle, queued, running, waitingForUser
    case cancelling, completed, failed, cancelled

    /// True while the UI should show a spinner and disable the composer.
    public var isActive: Bool {
        switch self {
        case .queued, .running, .waitingForUser, .cancelling: return true
        case .idle, .completed, .failed, .cancelled: return false
        }
    }
}

public enum ToolStatus: Sendable, Hashable {
    case running, completed, failed, blocked
}

/// One tool invocation, rendered as a single collapsible row.
public struct ToolActivity: Sendable, Hashable, Identifiable {
    public var id: String { callID }
    public let callID: String
    public var tool: String
    public var arguments: String
    public var output: String
    public var status: ToolStatus
    public var durationMS: Int?

    init(callID: String, tool: String, arguments: String = "") {
        self.callID = callID
        self.tool = tool
        self.arguments = arguments
        self.output = ""
        self.status = .running
        self.durationMS = nil
    }
}

public struct AssistantMessage: Sendable, Hashable {
    public var text: String
    /// True while deltas are still arriving, so the view can show a caret.
    public var isStreaming: Bool
}

/// A tool call blocked awaiting the operator's decision.
public struct PendingApproval: Sendable, Hashable {
    public let callID: String
    public let tool: String
    public let arguments: String
}

/// A plan awaiting the operator's approval to leave plan mode.
public struct PendingPlan: Sendable, Hashable {
    public struct Approach: Sendable, Hashable, Identifiable {
        public let id: String
        public let label: String
        public let description: String?
    }

    public let plan: String
    /// Approaches parsed out of the plan; approving sends the chosen id.
    public let options: [Approach]
}

public struct UsageTotals: Sendable, Hashable {
    public var promptTokens = 0
    public var completionTokens = 0
    public var totalTokens = 0
    public var costUSD: Double = 0
    /// harnessd's `cost_status`. Only `"available"` means the number is real:
    /// an unpriced model reports 0 with a different status, and rendering that
    /// as "$0.00" would read as free.
    public var costStatus = "pending"

    public var costIsKnown: Bool { costStatus == "available" }
}

public struct TranscriptItem: Sendable, Hashable, Identifiable {
    public enum Kind: Sendable, Hashable {
        case userPrompt(String)
        case assistantMessage(AssistantMessage)
        case thinking(String)
        case toolActivity(ToolActivity)
        case error(String)
        /// A non-fatal warning from the server, such as the run being routed
        /// to a different provider than the model implies. Dropping these made
        /// a substituted provider look like an unexplained failure.
        case notice(String)
        /// Marks where history was folded away, so the transcript does not
        /// silently lose messages.
        case compaction(summary: String, messagesRemoved: Int)
    }

    public let id: UUID
    public var kind: Kind
}

/// Reduces a run's event stream into renderable transcript items.
///
/// Deliberately a plain value type with a synchronous `apply`: the entire
/// streaming behaviour of the app is then testable by replaying a captured
/// stream, with no UI, concurrency, or network involved.
public struct Transcript: Sendable {
    public private(set) var items: [TranscriptItem] = []
    public private(set) var runState: RunState = .idle
    public private(set) var usage = UsageTotals()
    public private(set) var pendingApproval: PendingApproval?
    public private(set) var pendingPlan: PendingPlan?
    public private(set) var lastEventID: String?

    /// The run whose cumulative usage/cost currently owns `usage`. Harnessd
    /// restarts cumulative accounting at zero for every run, while a
    /// conversation transcript survives across runs, so monotonic merging is
    /// valid only inside this identity.
    private var accountingRunID: String?
    /// Set after a local prompt starts a new run but before harnessd reveals
    /// its id. Events from the prior run can still arrive through the
    /// conversation stream during this gap and must not reclaim accounting.
    private var awaitingAccountingRun = false
    private var previousAccountingRunID: String?
    /// A terminal event's cost status is the server's sealed conclusion for
    /// this run. Duplicate conversation streams may deliver older deltas
    /// afterward, but they cannot reclassify that conclusion.
    private var sealedTerminalCostStatus: String?

    /// Index into `items` of the assistant message currently accumulating
    /// deltas, so a new tool row between turns starts a fresh message.
    private var streamingMessageIndex: Int?
    /// Index into `items` for each in-flight tool call.
    private var toolIndexByCallID: [String: Int] = [:]

    public init() {}

    public mutating func appendUserPrompt(_ text: String) {
        items.append(.init(id: UUID(), kind: .userPrompt(text)))
        // A new prompt ends any previous streaming message.
        streamingMessageIndex = nil
        // Usage is per run, not per conversation. Clear it immediately so the
        // queued follow-up never presents the previous run's tokens/cost while
        // waiting for the new run id.
        previousAccountingRunID = accountingRunID
        accountingRunID = nil
        awaitingAccountingRun = true
        resetUsage()
        // Go busy immediately rather than waiting for the server's first event:
        // otherwise the composer stays enabled during the round trip and a
        // second submit can start a duplicate run.
        runState = .queued
    }

    public mutating func apply(_ event: HarnessEvent) {
        lastEventID = event.id
        let payload = event.payload
        let ownsAccounting = prepareAccounting(for: event)
        // Local synthetic terminal events represent transport failure or an
        // operator force-stop for the active run. They carry no accounting,
        // but must still settle the UI state.
        let ownsRunState = ownsAccounting || event.runID == "local"

        switch event.type {
        case .runQueued:
            if ownsRunState { runState = .queued }
        case .runStarted, .runResumed:
            if ownsRunState { runState = .running }
        case .runCompleted:
            if ownsRunState {
                if ownsAccounting { applyTerminalUsage(payload) }
                finishStreaming()
                runState = .completed
            }
        case .runFailed:
            if ownsRunState {
                if ownsAccounting { applyTerminalUsage(payload) }
                finishStreaming()
                runState = .failed
                if let message = payload["error"]?.stringValue, !message.isEmpty {
                    items.append(.init(id: UUID(), kind: .error(message)))
                }
            }
        case .runCancelled:
            if ownsRunState {
                if ownsAccounting { applyTerminalUsage(payload) }
                finishStreaming()
                runState = .cancelled
            }

        case .assistantMessageDelta:
            guard let chunk = payload["content"]?.stringValue, !chunk.isEmpty else { break }
            appendDelta(chunk)

        case .assistantMessage:
            guard let text = payload["content"]?.stringValue else { break }
            // Providers that stream deltas also send the complete text at the
            // end; replace rather than append or the reply is duplicated.
            if let index = streamingMessageIndex {
                items[index].kind = .assistantMessage(.init(text: text, isStreaming: false))
                streamingMessageIndex = nil
            } else if !text.isEmpty {
                items.append(
                    .init(
                        id: UUID(), kind: .assistantMessage(.init(text: text, isStreaming: false))))
            }

        case .assistantThinkingDelta:
            guard let chunk = payload["content"]?.stringValue else { break }
            if case .thinking(let existing) = items.last?.kind {
                items[items.count - 1].kind = .thinking(existing + chunk)
            } else {
                items.append(.init(id: UUID(), kind: .thinking(chunk)))
            }

        case .toolCallStarted:
            guard let callID = payload["call_id"]?.stringValue else { break }
            finishStreaming()
            let activity = ToolActivity(
                callID: callID,
                tool: payload["tool"]?.stringValue ?? "tool",
                arguments: payload["arguments"]?.stringValue ?? "")
            toolIndexByCallID[callID] = items.count
            items.append(.init(id: UUID(), kind: .toolActivity(activity)))

        case .toolOutputDelta:
            guard let chunk = payload["content"]?.stringValue else { break }
            mutateActivity(payload) { $0.output += chunk }

        case .toolCallCompleted:
            mutateActivity(payload) { activity in
                if let output = payload["output"]?.stringValue, activity.output.isEmpty {
                    activity.output = output
                }
                activity.durationMS = payload["duration_ms"]?.intValue
                activity.status = payload["error"]?.stringValue == nil ? .completed : .failed
            }

        case .toolCallBlocked:
            mutateActivity(payload) { $0.status = .blocked }

        // A background job routinely finishes after the run that started it
        // has ended, so this is the only moment the transcript can show that
        // it ran at all. Rendered as a notice rather than folded into a tool
        // row, because by now there is no tool row left to fold it into.
        case .backgroundJobCompleted:
            let command = payload["command"]?.stringValue ?? "background job"
            let output = payload["output"]?.stringValue ?? ""
            let exitCode = payload["exit_code"]?.intValue ?? 0
            let timedOut = payload["timed_out"]?.boolValue ?? false
            var message = timedOut ? "\(command) — timed out" : "\(command) — finished"
            if !timedOut && exitCode != 0 { message += " (exit \(exitCode))" }
            if !output.isEmpty { message += "\n\(output)" }
            items.append(.init(id: UUID(), kind: .notice(message)))

        case .toolApprovalRequired:
            guard let callID = payload["call_id"]?.stringValue else { break }
            pendingApproval = PendingApproval(
                callID: callID,
                tool: payload["tool"]?.stringValue ?? "tool",
                arguments: payload["arguments"]?.stringValue ?? "")
            runState = .waitingForUser

        case .planApprovalRequired:
            pendingPlan = PendingPlan(
                plan: payload["plan"]?.stringValue ?? "",
                options: (payload["options"]?.arrayValue ?? []).compactMap { entry in
                    guard let id = entry["id"]?.stringValue,
                        let label = entry["label"]?.stringValue
                    else { return nil }
                    return PendingPlan.Approach(
                        id: id, label: label, description: entry["description"]?.stringValue)
                })
            runState = .waitingForUser

        case .toolApprovalGranted, .toolApprovalDenied, .planApprovalGranted, .planApprovalDenied:
            pendingApproval = nil
            pendingPlan = nil
            if runState == .waitingForUser { runState = .running }

        case .runWaitingForUser:
            runState = .waitingForUser

        case .promptWarning:
            guard let message = payload["message"]?.stringValue, !message.isEmpty else { break }
            items.append(.init(id: UUID(), kind: .notice(message)))

        case .autoCompactCompleted:
            items.append(
                .init(
                    id: UUID(),
                    kind: .compaction(
                        summary: payload["summary"]?.stringValue ?? "History compacted",
                        messagesRemoved: payload["messages_removed"]?.intValue
                            ?? payload["removed"]?.intValue ?? 0)))

        case .usageDelta:
            if ownsAccounting { applyUsage(payload) }

        default:
            break
        }
    }

    // MARK: - Helpers

    /// Chooses the single run allowed to mutate cumulative accounting.
    /// Duplicate events for that run remain monotonic; late events from an
    /// older run are ignored instead of resetting or inflating the new run.
    private mutating func prepareAccounting(for event: HarnessEvent) -> Bool {
        guard !event.runID.isEmpty else { return accountingRunID == nil }
        if accountingRunID == event.runID { return true }

        let startsRun: Bool
        switch event.type {
        case .runQueued, .runStarted, .runResumed:
            startsRun = true
        default:
            startsRun = false
        }

        if awaitingAccountingRun {
            // Reconnect/resume can begin at any retained event, including a
            // terminal one, rather than replaying `run.started`. The new id is
            // sufficient ownership proof; only the immediately prior id is
            // rejected as a late duplicate.
            guard event.runID != previousAccountingRunID else { return false }
            accountingRunID = event.runID
            awaitingAccountingRun = false
            resetUsage()
            return true
        }

        if accountingRunID == nil {
            accountingRunID = event.runID
            return true
        }

        // A server-started follow-up has no local `appendUserPrompt` boundary.
        // Its queued/started event may claim accounting only after the prior
        // run is terminal; while a run is active, a different id is a late
        // event from another stream and cannot replace the owner.
        guard startsRun, !runState.isActive else { return false }
        previousAccountingRunID = accountingRunID
        accountingRunID = event.runID
        resetUsage()
        return true
    }

    private mutating func appendDelta(_ chunk: String) {
        if let index = streamingMessageIndex,
            case .assistantMessage(var message) = items[index].kind
        {
            message.text += chunk
            items[index].kind = .assistantMessage(message)
        } else {
            items.append(
                .init(id: UUID(), kind: .assistantMessage(.init(text: chunk, isStreaming: true))))
            streamingMessageIndex = items.count - 1
        }
    }

    /// Marks any in-flight streaming message complete so the caret stops.
    private mutating func finishStreaming() {
        guard let index = streamingMessageIndex,
            case .assistantMessage(var message) = items[index].kind
        else {
            streamingMessageIndex = nil
            return
        }
        message.isStreaming = false
        items[index].kind = .assistantMessage(message)
        streamingMessageIndex = nil
    }

    private mutating func mutateActivity(
        _ payload: [String: JSONValue], _ body: (inout ToolActivity) -> Void
    ) {
        guard let callID = payload["call_id"]?.stringValue,
            let index = toolIndexByCallID[callID],
            case .toolActivity(var activity) = items[index].kind
        else { return }
        body(&activity)
        items[index].kind = .toolActivity(activity)
    }

    /// `usage.delta` reports cumulative totals nested under `cumulative_usage`,
    /// with cost as a sibling flat field.
    private mutating func applyUsage(_ payload: [String: JSONValue]) {
        if let totals = payload["cumulative_usage"]?.objectValue {
            mergeUsageTotals(
                promptTokens: totals["prompt_tokens"]?.intValue,
                completionTokens: totals["completion_tokens"]?.intValue,
                totalTokens: totals["total_tokens"]?.intValue)
        }
        if let cost = payload["cumulative_cost_usd"]?.doubleValue {
            usage.costUSD = max(usage.costUSD, cost)
        }
        if let status = payload["cost_status"]?.stringValue {
            if sealedTerminalCostStatus == nil {
                mergeCostStatus(status)
            }
        }
    }

    /// `run.completed` is the server's sealed, authoritative accounting
    /// snapshot. The app consumes the same run through both per-run and
    /// conversation streams, so the terminal event can win the scheduling
    /// race before a duplicate stream's earlier `usage.delta`. Reconcile from
    /// the terminal payload before publishing `.completed`, then keep all
    /// cumulative values monotonic when older duplicate events arrive.
    private mutating func applyTerminalUsage(_ payload: [String: JSONValue]) {
        if let totals = payload["usage_totals"]?.objectValue {
            mergeUsageTotals(
                promptTokens: totals["prompt_tokens_total"]?.intValue,
                completionTokens: totals["completion_tokens_total"]?.intValue,
                totalTokens: totals["total_tokens"]?.intValue)
        }
        if let costs = payload["cost_totals"]?.objectValue {
            if let cost = costs["cost_usd_total"]?.doubleValue {
                usage.costUSD = max(usage.costUSD, cost)
            }
            if let status = costs["cost_status"]?.stringValue {
                usage.costStatus = status
                sealedTerminalCostStatus = status
            }
        }
    }

    private mutating func resetUsage() {
        usage = UsageTotals()
        sealedTerminalCostStatus = nil
    }

    private mutating func mergeUsageTotals(
        promptTokens: Int?, completionTokens: Int?, totalTokens: Int?
    ) {
        if let promptTokens {
            usage.promptTokens = max(usage.promptTokens, promptTokens)
        }
        if let completionTokens {
            usage.completionTokens = max(usage.completionTokens, completionTokens)
        }
        if let totalTokens {
            usage.totalTokens = max(usage.totalTokens, totalTokens)
        }
    }

    private mutating func mergeCostStatus(_ status: String) {
        if status == "available" || usage.costStatus != "available" {
            usage.costStatus = status
        }
    }
}

extension Transcript {
    /// Rebuilds the transcript from persisted history when opening a past
    /// conversation. Tool output arrives as a separate `tool` message, so it is
    /// merged back onto the call it belongs to rather than shown as a stray row.
    public mutating func load(messages: [StoredMessage]) {
        self = Transcript()
        var indexByCallID: [String: Int] = [:]

        for message in messages {
            switch message.role {
            case "user":
                if let content = message.content, !content.isEmpty {
                    items.append(.init(id: UUID(), kind: .userPrompt(content)))
                }
            case "assistant":
                if let content = message.content, !content.isEmpty {
                    items.append(
                        .init(
                            id: UUID(),
                            kind: .assistantMessage(.init(text: content, isStreaming: false))))
                }
                for call in message.toolCalls ?? [] {
                    var activity = ToolActivity(
                        callID: call.id, tool: call.name, arguments: call.arguments ?? "")
                    activity.status = .completed
                    indexByCallID[call.id] = items.count
                    items.append(.init(id: UUID(), kind: .toolActivity(activity)))
                }
            case "tool":
                // Attach to the most recent unfilled call; persisted tool
                // messages do not always carry the call id.
                if let index = indexByCallID.values.sorted().last,
                    case .toolActivity(var activity) = items[index].kind,
                    activity.output.isEmpty
                {
                    activity.output = message.content ?? ""
                    items[index].kind = .toolActivity(activity)
                }
            default:
                break
            }
        }
        runState = .completed
    }

    /// Rebuilds durable rows without turning an authoritative failed or
    /// cancelled terminal event into a successful run. Failure detail exists
    /// only on the event stream, so retain those rows across the persisted
    /// message rebuild as well.
    public mutating func reconcile(messages: [StoredMessage]) {
        let terminalState = runState
        let terminalUsage = usage
        let terminalCostStatus = sealedTerminalCostStatus
        let terminalAccountingRunID = accountingRunID
        let wasAwaitingAccountingRun = awaitingAccountingRun
        let priorAccountingRunID = previousAccountingRunID
        let terminalErrors = items.compactMap { item -> String? in
            if case .error(let message) = item.kind { return message }
            return nil
        }

        load(messages: messages)
        usage = terminalUsage
        sealedTerminalCostStatus = terminalCostStatus
        accountingRunID = terminalAccountingRunID
        awaitingAccountingRun = wasAwaitingAccountingRun
        previousAccountingRunID = priorAccountingRunID

        switch terminalState {
        case .failed:
            runState = .failed
            let loadedErrors = Set(
                items.compactMap { item -> String? in
                    if case .error(let message) = item.kind { return message }
                    return nil
                })
            for message in terminalErrors where !loadedErrors.contains(message) {
                items.append(.init(id: UUID(), kind: .error(message)))
            }
        case .cancelled:
            runState = .cancelled
        default:
            break
        }
    }

    /// Clears everything for a new conversation.
    public mutating func reset() {
        self = Transcript()
    }
}
