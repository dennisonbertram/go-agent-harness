import Foundation

/// A structured error returned by harnessd, or a transport-level failure.
public struct HarnessError: Error, Sendable, Hashable {
    public let code: String
    public let message: String
    public let statusCode: Int

    public init(code: String, message: String, statusCode: Int) {
        self.code = code
        self.message = message
        self.statusCode = statusCode
    }
}

extension HarnessError: LocalizedError {
    public var errorDescription: String? { message }
}

public enum RunStatus: String, Sendable, Codable {
    case queued, running, completed, failed, cancelled, cancelling
    case unknown

    public init(from decoder: Decoder) throws {
        let raw = try decoder.singleValueContainer().decode(String.self)
        self = RunStatus(rawValue: raw) ?? .unknown
    }
}

/// Client for the harnessd HTTP + SSE API.
///
/// One instance addresses one harnessd process. Because the workspace root is
/// process-level (`HARNESS_WORKSPACE`) rather than per-run, the app runs one
/// harnessd — and therefore one client — per open project.
public final class HarnessClient: Sendable {
    public let baseURL: URL
    private let token: String?
    private let session: URLSession

    public init(baseURL: URL, token: String? = nil, session: URLSession = .shared) {
        self.baseURL = baseURL
        self.token = token
        self.session = session
    }

    // MARK: - Run control

    public struct StartRunRequest: Sendable, Encodable {
        public var prompt: String
        public var model: String?
        public var conversationID: String?
        public var planMode: Bool?
        public var planFile: String?
        public var extraDirs: [String]?
        public var providerName: String?
        public var allowFallback: Bool?
        public var systemPrompt: String?
        public var maxSteps: Int?
        public var maxCostUSD: Double?
        public var reasoningEffort: String?
        public var allowedTools: [String]?
        public var deniedTools: [String]?
        public var workspaceType: String?
        public var profile: String?

        public init(prompt: String) {
            self.prompt = prompt
        }

        // Explicit keys mirror harness.RunRequest's json tags. Optionals encode
        // only when set: harnessd treats several fields as present-or-absent.
        enum CodingKeys: String, CodingKey {
            case prompt, model
            case conversationID = "conversation_id"
            case planMode = "plan_mode"
            case planFile = "plan_file"
            case extraDirs = "extra_dirs"
            case providerName = "provider_name"
            case allowFallback = "allow_fallback"
            case systemPrompt = "system_prompt"
            case maxSteps = "max_steps"
            case maxCostUSD = "max_cost_usd"
            case reasoningEffort = "reasoning_effort"
            case allowedTools = "allowed_tools"
            case deniedTools = "denied_tools"
            case workspaceType = "workspace_type"
            case profile
        }
    }

    public struct StartedRun: Sendable, Decodable {
        public let runID: String
        public let status: RunStatus

        enum CodingKeys: String, CodingKey {
            case runID = "run_id"
            case status
        }
    }

    @discardableResult
    public func startRun(_ request: StartRunRequest) async throws -> StartedRun {
        try await send(.post, "/v1/runs", body: request)
    }

    public func cancel(runID: String) async throws {
        try await sendIgnoringBody(.post, "/v1/runs/\(runID)/cancel")
    }

    /// Approves a pending tool call, or selects an option when exiting plan mode.
    public func approve(runID: String, option: String? = nil) async throws {
        struct Body: Encodable { let option: String }
        if let option {
            try await sendIgnoringBody(
                .post, "/v1/runs/\(runID)/approve", body: Body(option: option))
        } else {
            try await sendIgnoringBody(.post, "/v1/runs/\(runID)/approve")
        }
    }

    public func deny(runID: String) async throws {
        try await sendIgnoringBody(.post, "/v1/runs/\(runID)/deny")
    }

    /// Injects a steering message applied at the run's next step boundary.
    public func steer(runID: String, prompt: String) async throws {
        struct Body: Encodable { let prompt: String }
        try await sendIgnoringBody(.post, "/v1/runs/\(runID)/steer", body: Body(prompt: prompt))
    }

    /// Answers a pending AskUserQuestion, keyed by question id.
    public func answerInput(runID: String, answers: [String: String]) async throws {
        struct Body: Encodable { let answers: [String: String] }
        try await sendIgnoringBody(.post, "/v1/runs/\(runID)/input", body: Body(answers: answers))
    }

    // MARK: - Event stream

    /// Streams a run's events, finishing after a terminal event.
    ///
    /// Pass `lastEventID` to resume a dropped stream: harnessd replays from
    /// that sequence number so no event is lost or duplicated.
    public func events(
        runID: String, lastEventID: String? = nil
    ) -> AsyncThrowingStream<HarnessEvent, Error> {
        var request = URLRequest(url: baseURL.appending(path: "/v1/runs/\(runID)/events"))
        request.setValue("text/event-stream", forHTTPHeaderField: "Accept")
        if let lastEventID {
            request.setValue(lastEventID, forHTTPHeaderField: "Last-Event-ID")
        }
        authorize(&request)

        let session = self.session
        let streamRequest = request
        return AsyncThrowingStream {
            (continuation: AsyncThrowingStream<HarnessEvent, Error>.Continuation) in
            let task = Task { @Sendable in
                do {
                    let (bytes, response) = try await session.bytes(for: streamRequest)
                    try Self.checkStatus(response, body: Data())

                    var parser = SSEParser()
                    // `bytes` yields one byte at a time; batch them so the
                    // parser is not called per byte on a busy stream.
                    var buffer = Data()
                    buffer.reserveCapacity(4096)

                    for try await byte in bytes {
                        buffer.append(byte)
                        guard byte == UInt8(ascii: "\n") else { continue }
                        let frames = parser.consume(buffer)
                        buffer.removeAll(keepingCapacity: true)
                        for frame in frames {
                            guard let event = try? HarnessEvent(frame: frame) else { continue }
                            continuation.yield(event)
                            if event.type.isTerminal {
                                continuation.finish()
                                return
                            }
                        }
                    }
                    continuation.finish()
                } catch is CancellationError {
                    continuation.finish()
                } catch {
                    continuation.finish(throwing: error)
                }
            }
            continuation.onTermination = { _ in task.cancel() }
        }
    }

    // MARK: - Transport

    private enum Method: String {
        case get = "GET"
        case post = "POST"
        case put = "PUT"
        case delete = "DELETE"
    }

    private func authorize(_ request: inout URLRequest) {
        if let token {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
    }

    private func makeRequest(_ method: Method, _ path: String, body: (any Encodable)?) throws
        -> URLRequest
    {
        var request = URLRequest(url: baseURL.appending(path: path))
        request.httpMethod = method.rawValue
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        authorize(&request)
        if let body {
            let encoder = JSONEncoder()
            request.httpBody = try encoder.encode(body)
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }
        return request
    }

    private func send<Response: Decodable>(
        _ method: Method, _ path: String, body: (any Encodable)? = nil
    ) async throws -> Response {
        let (data, response) = try await session.data(
            for: try makeRequest(method, path, body: body))
        try Self.checkStatus(response, body: data)
        return try JSONDecoder().decode(Response.self, from: data)
    }

    private func sendIgnoringBody(
        _ method: Method, _ path: String, body: (any Encodable)? = nil
    ) async throws {
        let (data, response) = try await session.data(
            for: try makeRequest(method, path, body: body))
        try Self.checkStatus(response, body: data)
    }

    private struct ErrorEnvelope: Decodable {
        struct Detail: Decodable {
            let code: String?
            let message: String?
        }
        let error: Detail
    }

    private static func checkStatus(_ response: URLResponse, body: Data) throws {
        guard let http = response as? HTTPURLResponse else { return }
        guard !(200..<300).contains(http.statusCode) else { return }

        // harnessd wraps failures as {"error":{"code","message"}}; fall back to
        // the raw body so a proxy's HTML error is still legible.
        if let envelope = try? JSONDecoder().decode(ErrorEnvelope.self, from: body) {
            throw HarnessError(
                code: envelope.error.code ?? "unknown",
                message: envelope.error.message ?? "request failed",
                statusCode: http.statusCode)
        }
        let text = String(decoding: body.prefix(512), as: UTF8.self)
        throw HarnessError(
            code: "http_\(http.statusCode)",
            message: text.isEmpty ? "HTTP \(http.statusCode)" : text,
            statusCode: http.statusCode)
    }
}
