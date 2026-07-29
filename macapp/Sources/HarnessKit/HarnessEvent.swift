import Foundation

/// A harnessd run-stream event type.
///
/// Named cases cover the events the app renders or reacts to. Every other
/// server event decodes to `.other` with its wire name intact, so a harnessd
/// that emits new event types stays forward-compatible with an older app rather
/// than dropping frames.
public enum HarnessEventType: Sendable, Hashable {
    // Run lifecycle
    case runQueued, runStarted, runResumed
    case runCompleted, runFailed, runCancelled
    case runWaitingForUser, runCostLimitReached
    case runStepStarted, runStepCompleted
    case maxTurnsExhausted

    // Resolution
    case providerResolved, promptResolved, promptWarning

    // Model turns
    case llmTurnRequested, llmTurnCompleted
    case assistantMessage, assistantMessageDelta, assistantThinkingDelta

    // Tools
    case toolCallStarted, toolCallDelta, toolCallCompleted, toolCallBlocked
    case toolOutputDelta
    /// A background job finished. It routinely outlives the run that started
    /// it, so this is the only signal the UI gets that it ever completed.
    case backgroundJobCompleted
    case toolApprovalRequired, toolApprovalGranted, toolApprovalDenied

    // Plan mode
    case planApprovalRequired, planApprovalGranted, planApprovalDenied

    // Ambient state the UI surfaces
    case usageDelta, todosUpdated, steeringReceived
    case contextWindowWarning, autoCompactStarted, autoCompactCompleted
    case memoryObserveStarted, memoryObserveCompleted
    case taskCompleted, spawnAgentStarted, spawnAgentCompleted
    case errorContext

    /// Any event this client does not model, carrying its wire name.
    case other(String)

    /// Run-stream events after which no further events arrive.
    public var isTerminal: Bool {
        switch self {
        case .runCompleted, .runFailed, .runCancelled: return true
        default: return false
        }
    }

    /// True when the run is blocked awaiting a human decision.
    public var requiresUserDecision: Bool {
        switch self {
        case .toolApprovalRequired, .planApprovalRequired, .runWaitingForUser: return true
        default: return false
        }
    }
}

extension HarnessEventType: RawRepresentable {
    private static let byName: [String: HarnessEventType] = {
        var map: [String: HarnessEventType] = [:]
        for value in known { map[value.rawValue] = value }
        return map
    }()

    private static let known: [HarnessEventType] = [
        .runQueued, .runStarted, .runResumed, .runCompleted, .runFailed, .runCancelled,
        .runWaitingForUser, .runCostLimitReached, .runStepStarted, .runStepCompleted,
        .maxTurnsExhausted, .providerResolved, .promptResolved, .promptWarning,
        .llmTurnRequested, .llmTurnCompleted, .assistantMessage, .assistantMessageDelta,
        .assistantThinkingDelta, .toolCallStarted, .toolCallDelta, .toolCallCompleted,
        .backgroundJobCompleted,
        .toolCallBlocked, .toolOutputDelta, .toolApprovalRequired, .toolApprovalGranted,
        .toolApprovalDenied, .planApprovalRequired, .planApprovalGranted, .planApprovalDenied,
        .usageDelta, .todosUpdated, .steeringReceived, .contextWindowWarning,
        .autoCompactStarted, .autoCompactCompleted, .memoryObserveStarted,
        .memoryObserveCompleted, .taskCompleted, .spawnAgentStarted, .spawnAgentCompleted,
        .errorContext,
    ]

    public init(rawValue: String) {
        self = Self.byName[rawValue] ?? .other(rawValue)
    }

    public var rawValue: String {
        switch self {
        case .runQueued: return "run.queued"
        case .runStarted: return "run.started"
        case .runResumed: return "run.resumed"
        case .runCompleted: return "run.completed"
        case .runFailed: return "run.failed"
        case .runCancelled: return "run.cancelled"
        case .runWaitingForUser: return "run.waiting_for_user"
        case .runCostLimitReached: return "run.cost_limit_reached"
        case .runStepStarted: return "run.step.started"
        case .runStepCompleted: return "run.step.completed"
        case .maxTurnsExhausted: return "max_turns.exhausted"
        case .providerResolved: return "provider.resolved"
        case .promptResolved: return "prompt.resolved"
        case .promptWarning: return "prompt.warning"
        case .llmTurnRequested: return "llm.turn.requested"
        case .llmTurnCompleted: return "llm.turn.completed"
        case .assistantMessage: return "assistant.message"
        case .assistantMessageDelta: return "assistant.message.delta"
        case .assistantThinkingDelta: return "assistant.thinking.delta"
        case .toolCallStarted: return "tool.call.started"
        case .toolCallDelta: return "tool.call.delta"
        case .toolCallCompleted: return "tool.call.completed"
        case .toolCallBlocked: return "tool.call.blocked"
        case .toolOutputDelta: return "tool.output.delta"
        case .backgroundJobCompleted: return "job.completed"
        case .toolApprovalRequired: return "tool.approval_required"
        case .toolApprovalGranted: return "tool.approval_granted"
        case .toolApprovalDenied: return "tool.approval_denied"
        case .planApprovalRequired: return "plan.approval_required"
        case .planApprovalGranted: return "plan.approval_granted"
        case .planApprovalDenied: return "plan.approval_denied"
        case .usageDelta: return "usage.delta"
        case .todosUpdated: return "todos.updated"
        case .steeringReceived: return "steering.received"
        case .contextWindowWarning: return "context.window.warning"
        case .autoCompactStarted: return "auto_compact.started"
        case .autoCompactCompleted: return "auto_compact.completed"
        case .memoryObserveStarted: return "memory.observe.started"
        case .memoryObserveCompleted: return "memory.observe.completed"
        case .taskCompleted: return "task.completed"
        case .spawnAgentStarted: return "spawn_agent.started"
        case .spawnAgentCompleted: return "spawn_agent.completed"
        case .errorContext: return "error.context"
        case .other(let name): return name
        }
    }
}

/// A decoded harnessd run event.
public struct HarnessEvent: Sendable, Hashable {
    /// Monotonic stream id (`<run id>:<sequence>`), replayed via `Last-Event-ID`.
    public let id: String
    public let runID: String
    public let type: HarnessEventType
    public let timestamp: Date?
    public let payload: [String: JSONValue]

    private struct Envelope: Decodable {
        let id: String?
        let runID: String?
        let type: String
        let timestamp: String?
        let payload: [String: JSONValue]?

        enum CodingKeys: String, CodingKey {
            case id
            case runID = "run_id"
            case type, timestamp, payload
        }
    }

    /// Decodes the JSON envelope carried in an SSE frame's `data:` field.
    ///
    /// The frame's `event:` name and the envelope's `type` always agree in
    /// practice; the envelope wins because it is what harnessd persists.
    public init(frame: SSEFrame) throws {
        let envelope = try JSONDecoder().decode(Envelope.self, from: Data(frame.data.utf8))
        self.id = envelope.id ?? frame.id
        self.runID = envelope.runID ?? ""
        self.type = HarnessEventType(rawValue: envelope.type)
        self.timestamp = envelope.timestamp.flatMap(Self.parseTimestamp)
        self.payload = envelope.payload ?? [:]
    }

    // `Date.ISO8601FormatStyle` is a Sendable value type, unlike
    // `ISO8601DateFormatter`, so these can safely be shared globals.
    private static let iso8601Fractional = Date.ISO8601FormatStyle(includingFractionalSeconds: true)
    private static let iso8601Whole = Date.ISO8601FormatStyle(includingFractionalSeconds: false)

    /// harnessd stamps events RFC3339 with fractional seconds, but not every
    /// producer includes them, so fall back to whole seconds.
    static func parseTimestamp(_ value: String) -> Date? {
        (try? iso8601Fractional.parse(value)) ?? (try? iso8601Whole.parse(value))
    }
}
