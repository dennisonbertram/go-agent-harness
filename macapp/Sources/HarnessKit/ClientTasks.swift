import Foundation

/// Forward-compatible task kind from the server's background-work union.
public enum TaskType: Sendable, Hashable, Decodable {
    case subagent, cron, callback, bashJob
    case unknown(String)

    public init(from decoder: Decoder) throws {
        self.init(rawValue: try decoder.singleValueContainer().decode(String.self))
    }

    public init(rawValue: String) {
        switch rawValue {
        case "subagent": self = .subagent
        case "cron": self = .cron
        case "callback": self = .callback
        case "bash_job": self = .bashJob
        default: self = .unknown(rawValue)
        }
    }

    public var rawValue: String {
        switch self {
        case .subagent: return "subagent"
        case .cron: return "cron"
        case .callback: return "callback"
        case .bashJob: return "bash_job"
        case .unknown(let value): return value
        }
    }
}

/// Forward-compatible lifecycle state. Task-specific states deliberately stay
/// distinct from `RunStatus`: a callback can be pending while no run exists.
public enum TaskStatus: Sendable, Hashable, Decodable {
    case active, paused, pending, retryWait, dispatching, started, fired, canceled, running,
        completed, failed, skipped, exited
    case unknown(String)

    public init(from decoder: Decoder) throws {
        self.init(rawValue: try decoder.singleValueContainer().decode(String.self))
    }

    public init(rawValue: String) {
        switch rawValue {
        case "active": self = .active
        case "paused": self = .paused
        case "pending": self = .pending
        case "retry_wait": self = .retryWait
        case "dispatching": self = .dispatching
        case "started": self = .started
        case "fired": self = .fired
        case "canceled", "cancelled": self = .canceled
        case "running": self = .running
        case "completed": self = .completed
        case "failed": self = .failed
        case "skipped": self = .skipped
        case "exited": self = .exited
        default: self = .unknown(rawValue)
        }
    }

    public var rawValue: String {
        switch self {
        case .active: return "active"
        case .paused: return "paused"
        case .pending: return "pending"
        case .retryWait: return "retry_wait"
        case .dispatching: return "dispatching"
        case .started: return "started"
        case .fired: return "fired"
        case .canceled: return "canceled"
        case .running: return "running"
        case .completed: return "completed"
        case .failed: return "failed"
        case .skipped: return "skipped"
        case .exited: return "exited"
        case .unknown(let value): return value
        }
    }
}

/// An action advertised by the server for one task row. Unknown actions are
/// retained for diagnostics but never rendered as an executable control.
public enum TaskAction: Sendable, Hashable, Decodable {
    case cancel, delete, pause, resume
    case unknown(String)

    public init(from decoder: Decoder) throws {
        self.init(rawValue: try decoder.singleValueContainer().decode(String.self))
    }

    public init(rawValue: String) {
        switch rawValue {
        case "cancel": self = .cancel
        case "delete": self = .delete
        case "pause": self = .pause
        case "resume": self = .resume
        default: self = .unknown(rawValue)
        }
    }

    public var rawValue: String {
        switch self {
        case .cancel: return "cancel"
        case .delete: return "delete"
        case .pause: return "pause"
        case .resume: return "resume"
        case .unknown(let value): return value
        }
    }
}

/// A piece of background work the daemon knows about: a subagent, cron job, or
/// delayed callback. Scheduled lifecycle fields are optional so clients remain
/// compatible with older additive task payloads.
public struct TaskInfo: Sendable, Decodable, Identifiable, Hashable {
    public let id: String
    public let type: TaskType
    public let status: TaskStatus
    public let label: String
    public let startedAt: Date?
    public let ageSeconds: Int?
    public let actions: [TaskAction]?
    public let conversationID: String?
    public let nextRunAt: Date?
    public let lastRunAt: Date?
    public let firesAt: Date?
    public let lastExecutionStatus: TaskStatus?
    public let runID: String?
    public let attempt: Int?
    public let nextAttemptAt: Date?
    public let lastError: String?
    /// Opaque server version token used for cron action compare-and-swap.
    /// Keep this wire value as text: decoding and reformatting it as `Date`
    /// can lose RFC3339 nanoseconds and make a fresh row look stale.
    public let updatedAtVersion: String?

    public init(
        id: String, type: TaskType, status: TaskStatus, label: String,
        startedAt: Date? = nil, ageSeconds: Int? = nil, actions: [TaskAction]? = nil,
        conversationID: String? = nil, nextRunAt: Date? = nil, lastRunAt: Date? = nil,
        firesAt: Date? = nil, lastExecutionStatus: TaskStatus? = nil,
        runID: String? = nil, attempt: Int? = nil, nextAttemptAt: Date? = nil,
        lastError: String? = nil, updatedAtVersion: String? = nil
    ) {
        self.id = id
        self.type = type
        self.status = status
        self.label = label
        self.startedAt = startedAt
        self.ageSeconds = ageSeconds
        self.actions = actions
        self.conversationID = conversationID
        self.nextRunAt = nextRunAt
        self.lastRunAt = lastRunAt
        self.firesAt = firesAt
        self.lastExecutionStatus = lastExecutionStatus
        self.runID = runID
        self.attempt = attempt
        self.nextAttemptAt = nextAttemptAt
        self.lastError = lastError
        self.updatedAtVersion = updatedAtVersion
    }

    enum CodingKeys: String, CodingKey {
        case id, type, status, label, actions
        case startedAt = "started_at"
        case ageSeconds = "age_seconds"
        case conversationID = "conversation_id"
        case nextRunAt = "next_run_at"
        case lastRunAt = "last_run_at"
        case firesAt = "fires_at"
        case lastExecutionStatus = "last_execution_status"
        case runID = "run_id"
        case attempt
        case nextAttemptAt = "next_attempt_at"
        case lastError = "last_error"
        case updatedAtVersion = "updated_at"
    }

    public var isCancellable: Bool { actions?.contains(.cancel) ?? false }
}

public struct TodoItem: Sendable, Decodable, Identifiable, Hashable {
    public let id: String?
    public let text: String
    public let status: String
    /// This item's position in the decoded array, stamped by `todos(runID:)`
    /// right after decoding. Used only as a fallback identity: `stableID`
    /// used to be `id ?? text`, so two todos with identical text and no
    /// server id collided into one `ForEach` identity (#951 finding 6).
    var index: Int = 0

    enum CodingKeys: String, CodingKey { case id, text, status }

    public var isDone: Bool { status == "completed" || status == "done" }
}

extension TodoItem {
    public var stableID: String { id ?? "\(index):\(text)" }
}

/// A run as listed by the dashboard.
public struct RunSummaryInfo: Sendable, Decodable, Identifiable, Hashable {
    public let id: String
    public let status: String?
    public let model: String?
    public let prompt: String?
    public let conversationID: String?
    public let createdAt: Date?

    enum CodingKeys: String, CodingKey {
        case id, status, model, prompt
        case conversationID = "conversation_id"
        case createdAt = "created_at"
    }
}

extension HarnessClient {
    public func tasks() async throws -> [TaskInfo] {
        struct Response: Decodable { let tasks: [TaskInfo]? }
        let response: Response = try await get("/v1/tasks")
        return response.tasks ?? []
    }

    /// Each task control calls a type-specific, server-authorized endpoint.
    /// The caller must follow the request with `tasks()` rather than mutate a
    /// local row optimistically because allowed actions can have gone stale.
    public func pauseCron(id: String, expectedUpdatedAt: String? = nil) async throws {
        try await cronAction(
            .post, "/v1/cron/jobs/\(id)/pause", expectedUpdatedAt: expectedUpdatedAt)
    }

    public func resumeCron(id: String, expectedUpdatedAt: String? = nil) async throws {
        try await cronAction(
            .post, "/v1/cron/jobs/\(id)/resume", expectedUpdatedAt: expectedUpdatedAt)
    }

    public func deleteCron(id: String, expectedUpdatedAt: String? = nil) async throws {
        try await cronAction(.delete, "/v1/cron/jobs/\(id)", expectedUpdatedAt: expectedUpdatedAt)
    }

    public func cancelCallback(id: String) async throws {
        try await sendVoid(.post, "/v1/callbacks/\(id)/cancel")
    }

    /// Preserves an empty request body for older additive task payloads, while
    /// current task rows send their observed version as a CAS fence.
    private func cronAction(_ method: Method, _ path: String, expectedUpdatedAt: String?)
        async throws
    {
        if let expectedUpdatedAt {
            try await sendVoid(
                method, path, body: TaskActionVersion(expectedUpdatedAt: expectedUpdatedAt))
        } else {
            try await sendVoid(method, path)
        }
    }

    public func todos(runID: String) async throws -> [TodoItem] {
        struct Response: Decodable { let todos: [TodoItem]? }
        let response: Response = try await get("/v1/runs/\(runID)/todos")
        // Stamp array position for `stableID`'s no-server-id fallback — see
        // `TodoItem.index`.
        return (response.todos ?? []).enumerated().map { index, item in
            var item = item
            item.index = index
            return item
        }
    }

    /// Lists runs for the dashboard.
    ///
    /// Returns nil — not an error — when the daemon has no run store, which is
    /// a normal configuration rather than a failure.
    public func runs(conversationID: String? = nil) async throws -> [RunSummaryInfo]? {
        struct Response: Decodable { let runs: [RunSummaryInfo]? }
        var query: [URLQueryItem] = []
        if let conversationID {
            query.append(URLQueryItem(name: "conversation_id", value: conversationID))
        }
        do {
            let response: Response = try await get("/v1/runs", query: query)
            return response.runs ?? []
        } catch let error as HarnessError where error.statusCode == 501 {
            return nil
        }
    }
}

private struct TaskActionVersion: Encodable {
    let expectedUpdatedAt: String

    enum CodingKeys: String, CodingKey { case expectedUpdatedAt = "expected_updated_at" }
}
