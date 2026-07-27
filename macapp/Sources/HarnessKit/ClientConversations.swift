import Foundation

public struct ConversationInfo: Sendable, Decodable, Identifiable, Hashable {
    public let id: String
    public let title: String?
    public let createdAt: Date?
    public let updatedAt: Date?
    public let messageCount: Int?
    public let costUSD: Double?
    public let pinned: Bool?
    public let workspace: String?

    enum CodingKeys: String, CodingKey {
        case id, title, pinned, workspace
        case createdAt = "created_at"
        case updatedAt = "updated_at"
        case messageCount = "message_count"
        case costUSD = "cost_usd"
    }

    public var displayTitle: String {
        if let title, !title.isEmpty { return title }
        return "Untitled conversation"
    }
}

public struct StoredMessage: Sendable, Decodable, Hashable {
    public struct ToolCall: Sendable, Decodable, Hashable {
        public let id: String
        public let name: String
        public let arguments: String?
    }

    public let role: String
    public let content: String?
    public let step: Int?
    /// Present on assistant messages that invoked tools; the matching output
    /// arrives as a following message with role `tool`.
    public let toolCalls: [ToolCall]?

    enum CodingKeys: String, CodingKey {
        case role, content, step
        case toolCalls = "tool_calls"
    }
}

/// A restorable point in a conversation, with the files a restore would touch.
public struct RewindPoint: Sendable, Decodable, Identifiable, Hashable {
    public struct FileSnapshot: Sendable, Decodable, Hashable {
        public let path: String
        public let exists: Bool?
        public let skipped: Bool?
        public let skipReason: String?

        enum CodingKeys: String, CodingKey {
            case path, exists, skipped
            case skipReason = "skip_reason"
        }
    }

    public let id: String
    public let step: Int?
    public let tool: String?
    public let createdAt: Date?
    public let files: [FileSnapshot]?

    enum CodingKeys: String, CodingKey {
        case id, step, tool, files
        case createdAt = "created_at"
    }
}

public struct RewindResult: Sendable, Decodable, Hashable {
    public let filesRestored: Int
    public let messagesTruncated: Int

    enum CodingKeys: String, CodingKey {
        case filesRestored = "files_restored"
        case messagesTruncated = "messages_truncated"
    }
}

public struct ForkResult: Sendable, Decodable, Hashable {
    public let conversationID: String
    public let forkedFrom: String?
    public let messageCount: Int?

    enum CodingKeys: String, CodingKey {
        case conversationID = "conversation_id"
        case forkedFrom = "forked_from"
        case messageCount = "message_count"
    }
}

extension HarnessClient {

    public func conversations(limit: Int = 50, search: String? = nil) async throws
        -> [ConversationInfo]
    {
        struct Response: Decodable { let conversations: [ConversationInfo]? }
        var query = [URLQueryItem(name: "limit", value: String(limit))]
        if let search, !search.isEmpty {
            query.append(URLQueryItem(name: "q", value: search))
        }
        let response: Response = try await get("/v1/conversations/", query: query)
        return response.conversations ?? []
    }

    public func messages(conversationID: String) async throws -> [StoredMessage] {
        struct Response: Decodable { let messages: [StoredMessage]? }
        let response: Response = try await get("/v1/conversations/\(conversationID)/messages")
        return response.messages ?? []
    }

    public func deleteConversation(id: String) async throws {
        try await sendVoid(.delete, "/v1/conversations/\(id)")
    }

    public func rewindPoints(conversationID: String) async throws -> [RewindPoint] {
        struct Response: Decodable { let points: [RewindPoint]? }
        let response: Response = try await get(
            "/v1/conversations/\(conversationID)/rewind-points")
        return response.points ?? []
    }

    /// Restores files and truncates history to `pointID`.
    ///
    /// Destructive. Without `force`, harnessd refuses with `409 rewind_refused`
    /// when a file changed outside the harness — that refusal is a safety
    /// feature and must be surfaced, never auto-retried with `force: true`.
    @discardableResult
    public func rewind(conversationID: String, pointID: String, force: Bool = false) async throws
        -> RewindResult
    {
        struct Body: Encodable {
            let pointID: String
            let force: Bool

            enum CodingKeys: String, CodingKey {
                case pointID = "point_id"
                case force
            }
        }
        return try await send(
            .post, "/v1/conversations/\(conversationID)/rewind",
            body: Body(pointID: pointID, force: force))
    }

    public func fork(conversationID: String) async throws -> ForkResult {
        try await send(.post, "/v1/conversations/\(conversationID)/fork")
    }

    /// Removes the last `count` user prompts, or truncates to `toStep`.
    public func undo(conversationID: String, count: Int? = nil, toStep: Int? = nil) async throws {
        struct Body: Encodable {
            let count: Int?
            let toStep: Int?

            enum CodingKeys: String, CodingKey {
                case count
                case toStep = "to_step"
            }
        }
        try await sendVoid(
            .post, "/v1/conversations/\(conversationID)/undo",
            body: Body(count: count, toStep: toStep))
    }

    /// Exports the transcript as JSONL (`application/x-ndjson`).
    public func exportConversation(id: String) async throws -> Data {
        var request = URLRequest(url: baseURL.appending(path: "/v1/conversations/\(id)/export"))
        request.setValue("application/x-ndjson", forHTTPHeaderField: "Accept")
        authorize(&request)
        let (data, response) = try await session.data(for: request)
        try Self.checkStatus(response, body: data)
        return data
    }
}
