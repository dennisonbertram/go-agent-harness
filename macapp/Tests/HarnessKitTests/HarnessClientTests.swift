import Foundation
import Testing

@testable import HarnessKit

/// Intercepts URLSession traffic so client behaviour can be tested without a
/// live harnessd. `URLProtocol` is the stdlib seam for this; the client itself
/// has no injected transport abstraction.
final class StubURLProtocol: URLProtocol, @unchecked Sendable {
    struct Response: Sendable {
        var status: Int = 200
        var headers: [String: String] = ["Content-Type": "application/json"]
        /// Body chunks delivered in sequence, so SSE streaming can be exercised.
        var chunks: [Data] = []
    }

    nonisolated(unsafe) private static var handler: (@Sendable (URLRequest) -> Response)?
    nonisolated(unsafe) private static var recorded: [URLRequest] = []
    private static let lock = NSLock()

    static func set(_ handler: @escaping @Sendable (URLRequest) -> Response) {
        lock.withLock {
            self.handler = handler
            self.recorded = []
        }
    }

    static var requests: [URLRequest] {
        lock.withLock { recorded }
    }

    /// `URLProtocol` strips the body from `request`, so capture it separately.
    nonisolated(unsafe) private static var bodies: [String: Data] = [:]
    static func body(forPath path: String) -> Data? {
        lock.withLock { bodies[path] }
    }

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        let request = self.request
        Self.lock.withLock {
            Self.recorded.append(request)
            if let path = request.url?.path {
                Self.bodies[path] = request.httpBodyData
            }
        }
        let response = Self.lock.withLock { Self.handler }?(request) ?? Response()

        let http = HTTPURLResponse(
            url: request.url!, statusCode: response.status,
            httpVersion: "HTTP/1.1", headerFields: response.headers)!
        client?.urlProtocol(self, didReceive: http, cacheStoragePolicy: .notAllowed)
        for chunk in response.chunks {
            client?.urlProtocol(self, didLoad: chunk)
        }
        client?.urlProtocolDidFinishLoading(self)
    }

    override func stopLoading() {}
}

extension URLRequest {
    /// `httpBody` is nil for stream-backed bodies; read the stream instead.
    fileprivate var httpBodyData: Data? {
        if let body = httpBody { return body }
        guard let stream = httpBodyStream else { return nil }
        stream.open()
        defer { stream.close() }
        var data = Data()
        let size = 4096
        let buffer = UnsafeMutablePointer<UInt8>.allocate(capacity: size)
        defer { buffer.deallocate() }
        while stream.hasBytesAvailable {
            let read = stream.read(buffer, maxLength: size)
            if read <= 0 { break }
            data.append(buffer, count: read)
        }
        return data
    }
}

private func makeClient(token: String? = nil) -> HarnessClient {
    let config = URLSessionConfiguration.ephemeral
    config.protocolClasses = [StubURLProtocol.self]
    return HarnessClient(
        baseURL: URL(string: "http://127.0.0.1:8899")!,
        token: token,
        session: URLSession(configuration: config))
}

/// `StubURLProtocol` is registered by type and therefore shares state across
/// tests, so every test touching it must run serially — `.serialized` on this
/// outer suite covers the nested suites too.
@Suite("HarnessClient", .serialized)
struct HarnessClientSuite {

    @Suite("run control")
    struct HarnessClientTests {

        @Test("starts a run and returns the server-minted run id")
        func startsRun() async throws {
            StubURLProtocol.set { _ in
                .init(
                    status: 202,
                    chunks: [Data(#"{"run_id":"run_abc","status":"queued"}"#.utf8)])
            }

            let client = makeClient()
            let started = try await client.startRun(.init(prompt: "list the workspace"))

            #expect(started.runID == "run_abc")
            #expect(started.status == .queued)

            let request = try #require(StubURLProtocol.requests.first)
            #expect(request.httpMethod == "POST")
            #expect(request.url?.path == "/v1/runs")

            let body = try #require(StubURLProtocol.body(forPath: "/v1/runs"))
            let json = try #require(
                try JSONSerialization.jsonObject(with: body) as? [String: Any])
            #expect(json["prompt"] as? String == "list the workspace")
            // Optional fields must be omitted, not sent as null — harnessd
            // validates presence for several of them.
            #expect(json["model"] == nil)
            #expect(json["plan_mode"] == nil)
        }

        @Test("sends run options the TUI exposes")
        func sendsRunOptions() async throws {
            StubURLProtocol.set { _ in
                .init(status: 202, chunks: [Data(#"{"run_id":"r","status":"queued"}"#.utf8)])
            }

            // NOTE: there is no per-run `workspace` field on harness.RunRequest —
            // the workspace root is process-level (`HARNESS_WORKSPACE`), so the app
            // runs one harnessd per project. `extra_dirs` is how a run is granted
            // access beyond that root (the TUI's /add-dir).
            var request = HarnessClient.StartRunRequest(prompt: "hi")
            request.model = "gpt-5"
            request.planMode = true
            request.extraDirs = ["/tmp/other"]
            request.allowFallback = true
            request.conversationID = "conv_1"
            _ = try await makeClient().startRun(request)

            let body = try #require(StubURLProtocol.body(forPath: "/v1/runs"))
            let json = try #require(try JSONSerialization.jsonObject(with: body) as? [String: Any])
            #expect(json["model"] as? String == "gpt-5")
            #expect(json["plan_mode"] as? Bool == true)
            #expect(json["extra_dirs"] as? [String] == ["/tmp/other"])
            #expect(json["allow_fallback"] as? Bool == true)
            #expect(json["conversation_id"] as? String == "conv_1")
            #expect(json["workspace"] == nil)
        }

        @Test("authenticates with a bearer token when configured")
        func sendsBearerToken() async throws {
            StubURLProtocol.set { _ in
                .init(status: 202, chunks: [Data(#"{"run_id":"r","status":"queued"}"#.utf8)])
            }
            _ = try await makeClient(token: "secret-key").startRun(.init(prompt: "x"))

            let request = try #require(StubURLProtocol.requests.first)
            #expect(request.value(forHTTPHeaderField: "Authorization") == "Bearer secret-key")
        }

        @Test("omits the Authorization header when no token is configured")
        func omitsTokenWhenAbsent() async throws {
            StubURLProtocol.set { _ in
                .init(status: 202, chunks: [Data(#"{"run_id":"r","status":"queued"}"#.utf8)])
            }
            _ = try await makeClient().startRun(.init(prompt: "x"))

            let request = try #require(StubURLProtocol.requests.first)
            #expect(request.value(forHTTPHeaderField: "Authorization") == nil)
        }

        /// harnessd returns structured errors; surfacing the server's message is
        /// the difference between "something went wrong" and "prompt is required".
        @Test("surfaces the server's structured error message")
        func surfacesServerError() async throws {
            StubURLProtocol.set { _ in
                .init(
                    status: 400,
                    chunks: [
                        Data(
                            #"{"error":{"code":"invalid_request","message":"prompt is required"}}"#
                                .utf8)
                    ])
            }

            await #expect(throws: HarnessError.self) {
                _ = try await makeClient().startRun(.init(prompt: ""))
            }

            do {
                _ = try await makeClient().startRun(.init(prompt: ""))
            } catch let error as HarnessError {
                #expect(error.code == "invalid_request")
                #expect(error.message == "prompt is required")
                #expect(error.statusCode == 400)
            }
        }

        /// `URL.appending(path:)` percent-encodes `?`, so folding a query string
        /// into the path yields `/v1/conversations/%3Flimit=10` and a 404. Queries
        /// must travel as real query items.
        @Test("sends query parameters as a query string, not inside the path")
        func sendsQueryAsQueryString() async throws {
            StubURLProtocol.set { _ in
                .init(status: 200, chunks: [Data(#"{"conversations":[]}"#.utf8)])
            }
            _ = try await makeClient().conversations(limit: 10, search: "hello world")

            let request = try #require(StubURLProtocol.requests.first)
            #expect(request.url?.path == "/v1/conversations/")
            let query = try #require(request.url?.query)
            #expect(query.contains("limit=10"))
            #expect(query.contains("q=hello%20world") || query.contains("q=hello+world"))
            #expect(request.url?.absoluteString.contains("%3F") == false)
        }

        @Test("approve, deny, cancel and steer hit the documented endpoints")
        func runControlEndpoints() async throws {
            StubURLProtocol.set { _ in .init(status: 200, chunks: [Data("{}".utf8)]) }
            let client = makeClient()

            try await client.cancel(runID: "run_1")
            try await client.approve(runID: "run_1", option: "allow-always")
            try await client.deny(runID: "run_1")
            try await client.steer(runID: "run_1", prompt: "focus on tests")

            let paths = StubURLProtocol.requests.compactMap(\.url?.path)
            #expect(
                paths == [
                    "/v1/runs/run_1/cancel",
                    "/v1/runs/run_1/approve",
                    "/v1/runs/run_1/deny",
                    "/v1/runs/run_1/steer",
                ])
            #expect(StubURLProtocol.requests.allSatisfy { $0.httpMethod == "POST" })

            let steer = try #require(StubURLProtocol.body(forPath: "/v1/runs/run_1/steer"))
            let json = try #require(try JSONSerialization.jsonObject(with: steer) as? [String: Any])
            #expect(json["prompt"] as? String == "focus on tests")
        }
    }

    @Suite("event stream")
    struct HarnessClientStreamTests {

        @Test("streams decoded events and stops after a terminal event")
        func streamsUntilTerminal() async throws {
            let frames = """
                id: run_1:0
                event: run.started
                data: {"id":"run_1:0","run_id":"run_1","type":"run.started","payload":{}}

                id: run_1:1
                event: assistant.message
                data: {"id":"run_1:1","run_id":"run_1","type":"assistant.message","payload":{"content":"hello"}}

                id: run_1:2
                event: run.completed
                data: {"id":"run_1:2","run_id":"run_1","type":"run.completed","payload":{}}


                """

            // Deliberately split mid-frame to prove the stream reassembles chunks.
            let bytes = Array(frames.utf8)
            let mid = bytes.count / 2
            StubURLProtocol.set { _ in
                .init(
                    status: 200,
                    headers: ["Content-Type": "text/event-stream"],
                    chunks: [Data(bytes[..<mid]), Data(bytes[mid...])])
            }

            var received: [HarnessEventType] = []
            for try await event in makeClient().events(runID: "run_1") {
                received.append(event.type)
            }

            #expect(received == [.runStarted, .assistantMessage, .runCompleted])
        }

        @Test("resumes from the last seen event id")
        func resumesFromLastEventID() async throws {
            StubURLProtocol.set { _ in
                .init(status: 200, headers: ["Content-Type": "text/event-stream"], chunks: [])
            }

            for try await _ in makeClient().events(runID: "run_1", lastEventID: "run_1:7") {}

            let request = try #require(StubURLProtocol.requests.first)
            #expect(request.value(forHTTPHeaderField: "Last-Event-ID") == "run_1:7")
            #expect(request.url?.path == "/v1/runs/run_1/events")
        }

        /// Regression for finding 14: a malformed frame used to be silently
        /// dropped via `try?`, and if it had been terminal-looking it could
        /// never have surfaced *why* the stream produced fewer events than
        /// expected. The fix logs and continues, so the well-formed frames on
        /// either side of a bad one must still both arrive.
        @Test("skips a malformed frame but still delivers the events around it")
        func skipsMalformedFrameAndContinues() async throws {
            let frames = """
                id: run_1:0
                event: run.started
                data: {"id":"run_1:0","run_id":"run_1","type":"run.started","payload":{}}

                id: run_1:1
                event: assistant.message
                data: { this is not valid json

                id: run_1:2
                event: run.completed
                data: {"id":"run_1:2","run_id":"run_1","type":"run.completed","payload":{}}


                """
            StubURLProtocol.set { _ in
                .init(
                    status: 200,
                    headers: ["Content-Type": "text/event-stream"],
                    chunks: [Data(frames.utf8)])
            }

            var received: [HarnessEventType] = []
            for try await event in makeClient().events(runID: "run_1") {
                received.append(event.type)
            }

            #expect(received == [.runStarted, .runCompleted])
        }
    }

    @Suite("conversation event stream")
    struct HarnessClientConversationStreamTests {

        /// Issue #950: a delayed callback (or cron run) can fire on a
        /// conversation after the run that scheduled it already reached a
        /// terminal event. Unlike `events(runID:)`, the conversation stream
        /// must keep delivering events from later runs past an earlier run's
        /// `run.completed` -- that terminal event ends one run, not the
        /// conversation.
        @Test("delivers events across multiple runs and does not stop at a terminal event")
        func deliversEventsAcrossRuns() async throws {
            let frames = """
                id: run_1:0
                event: run.started
                data: {"id":"run_1:0","run_id":"run_1","type":"run.started","payload":{}}

                id: run_1:1
                event: run.completed
                data: {"id":"run_1:1","run_id":"run_1","type":"run.completed","payload":{}}

                id: run_2:0
                event: assistant.message
                data: {"id":"run_2:0","run_id":"run_2","type":"assistant.message","payload":{"content":"callback fired"}}


                """
            StubURLProtocol.set { _ in
                .init(
                    status: 200,
                    headers: ["Content-Type": "text/event-stream"],
                    chunks: [Data(frames.utf8)])
            }

            var received: [HarnessEventType] = []
            for try await event in makeClient().conversationEvents(conversationID: "conv_1") {
                received.append(event.type)
            }

            // If the client closed on `run.completed` like `events(runID:)`
            // does, `run_2`'s assistant message would never arrive.
            #expect(received == [.runStarted, .runCompleted, .assistantMessage])

            let request = try #require(StubURLProtocol.requests.first)
            #expect(request.url?.path == "/v1/conversations/conv_1/events")
        }

        @Test("resumes the conversation stream from the last seen event id")
        func resumesFromLastEventID() async throws {
            StubURLProtocol.set { _ in
                .init(status: 200, headers: ["Content-Type": "text/event-stream"], chunks: [])
            }

            for try await _ in makeClient().conversationEvents(
                conversationID: "conv_1", lastEventID: "run_1:7")
            {}

            let request = try #require(StubURLProtocol.requests.first)
            #expect(request.value(forHTTPHeaderField: "Last-Event-ID") == "run_1:7")
            #expect(request.url?.path == "/v1/conversations/conv_1/events")
        }
    }

    @Suite("todos")
    struct HarnessClientTodoTests {

        /// The server can omit `id` entirely; `stableID` used to fall back to
        /// raw text, so two todos with the same text collided into one
        /// `ForEach` identity. `todos(runID:)` is where the fix stamps each
        /// item's array position (#951 finding 6).
        @Test("todos(runID:) gives duplicate-text, id-less items distinct stableIDs")
        func todosWithDuplicateTextDontCollide() async throws {
            StubURLProtocol.set { _ in
                .init(
                    status: 200,
                    chunks: [
                        Data(
                            #"{"todos":[{"text":"write tests","status":"pending"},{"text":"write tests","status":"pending"}]}"#
                                .utf8)
                    ])
            }
            let todos = try await makeClient().todos(runID: "run_1")
            #expect(todos.count == 2)
            #expect(Set(todos.map(\.stableID)).count == 2)
        }

        /// Regression angle: when the server does send an id, it must still
        /// win over the position stamp — the fix only fills a gap, it must not
        /// override real identity.
        @Test("todos(runID:) still prefers a server-provided id")
        func todosPreferServerID() async throws {
            StubURLProtocol.set { _ in
                .init(
                    status: 200,
                    chunks: [
                        Data(
                            #"{"todos":[{"id":"todo_1","text":"write tests","status":"pending"}]}"#
                                .utf8)
                    ])
            }
            let todos = try await makeClient().todos(runID: "run_1")
            #expect(todos.first?.stableID == "todo_1")
        }

        /// Regression angle distinct from the two above: a realistic mixed
        /// batch — some items with a real id, some without, and duplicate
        /// text among the id-less ones — must still come out fully unique.
        @Test("todos(runID:) keeps every stableID unique in a mixed batch")
        func todosMixedBatchStaysUnique() async throws {
            StubURLProtocol.set { _ in
                .init(
                    status: 200,
                    chunks: [
                        Data(
                            #"""
                            {"todos":[
                                {"id":"todo_1","text":"write tests","status":"done"},
                                {"text":"write docs","status":"pending"},
                                {"text":"write docs","status":"pending"}
                            ]}
                            """#
                            .utf8)
                    ])
            }
            let todos = try await makeClient().todos(runID: "run_1")
            #expect(todos.count == 3)
            #expect(Set(todos.map(\.stableID)).count == 3)
        }
    }

}
