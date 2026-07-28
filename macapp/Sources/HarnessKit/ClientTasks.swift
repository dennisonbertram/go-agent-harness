import Foundation

/// A piece of background work the daemon knows about: a subagent, cron job, or
/// pending delayed callback.
public struct TaskInfo: Sendable, Decodable, Identifiable, Hashable {
    public let id: String
    public let type: String
    public let status: String
    public let label: String
    public let startedAt: Date?
    public let ageSeconds: Int?
    public let actions: [String]?

    enum CodingKeys: String, CodingKey {
        case id, type, status, label, actions
        case startedAt = "started_at"
        case ageSeconds = "age_seconds"
    }

    public var isCancellable: Bool { actions?.contains("cancel") ?? false }
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
