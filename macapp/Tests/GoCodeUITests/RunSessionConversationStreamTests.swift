import Foundation
import HarnessKit
import Testing

@testable import GoCodeUI

/// Minimal HTTP+SSE stub scoped to this file's tests.
///
/// Mirrors `HarnessKitTests.StubURLProtocol` (different test target, not
/// exported, so it can't be reused directly). Responses are queued per path:
/// each request to that path pops the next scripted response, repeating the
/// last one once the queue drains, so a test can script a dropped connection
/// followed by a reconnect without hand-rolling per-call state everywhere.
private final class ConversationStreamStub: URLProtocol, @unchecked Sendable {
    struct Response: Sendable {
        var status: Int = 200
        var headers: [String: String] = ["Content-Type": "application/json"]
        var chunks: [Data] = []
    }

    nonisolated(unsafe) private static var handlers: [String: [Response]] = [:]
    nonisolated(unsafe) private static var recorded: [URLRequest] = []
    private static let lock = NSLock()

    static func queue(_ path: String, _ responses: [Response]) {
        lock.withLock { handlers[path] = responses }
    }

    static func reset() {
        lock.withLock {
            handlers = [:]
            recorded = []
        }
    }

    static var requests: [URLRequest] { lock.withLock { recorded } }

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        let request = self.request
        let response: Response = Self.lock.withLock {
            Self.recorded.append(request)
            guard let path = request.url?.path, var queue = Self.handlers[path], !queue.isEmpty
            else { return Response() }
            let next = queue.removeFirst()
            Self.handlers[path] = queue.isEmpty ? [next] : queue
            return next
        }
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

/// Exercises the fix for issue #950: harnessd exposes a conversation-wide SSE
/// stream (`GET /v1/conversations/{id}/events`) so a delayed callback or cron
/// run that fires after the run that scheduled it already ended still reaches
/// the app. These tests drive `RunSession` directly against the stub above,
/// the same layer `RunSessionLiveTests` exercises against a real harnessd.
@Suite("RunSession conversation-wide event stream", .serialized)
@MainActor
struct RunSessionConversationStreamTests {

    private func makeSession() -> RunSession {
        let config = URLSessionConfiguration.ephemeral
        config.protocolClasses = [ConversationStreamStub.self]
        let client = HarnessClient(
            baseURL: URL(string: "http://127.0.0.1:8899")!,
            session: URLSession(configuration: config))
        return RunSession(client: client)
    }

    private func wait(
        timeout: Duration = .seconds(5), for condition: () -> Bool
    ) async throws {
        let deadline = ContinuousClock.now.advanced(by: timeout)
        while ContinuousClock.now < deadline {
            if condition() { return }
            try await Task.sleep(for: .milliseconds(20))
        }
        Issue.record("timed out waiting for condition")
    }

    private func hasAssistantText(_ session: RunSession, _ text: String) -> Bool {
        session.transcript.items.contains {
            if case .assistantMessage(let message) = $0.kind { return message.text == text }
            return false
        }
    }

    /// BT-002: a run this app instance never called `startRun`/`submit()` for
    /// -- exactly the shape of a fired delayed callback -- must still render
    /// in the transcript, because opening a conversation subscribes to every
    /// run on it, not just the ones this client started.
    @Test("renders output from a run this app never started")
    func rendersUnstartedRunEvents() async throws {
        ConversationStreamStub.reset()
        let frames = """
            id: run_cb:0
            event: assistant.message
            data: {"id":"run_cb:0","run_id":"run_cb","type":"assistant.message","payload":{"content":"CALLBACK_SURFACED_IN_UI"}}

            id: run_cb:1
            event: run.completed
            data: {"id":"run_cb:1","run_id":"run_cb","type":"run.completed","payload":{}}


            """
        ConversationStreamStub.queue(
            "/v1/conversations/conv_1/events",
            [
                .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"],
                    chunks: [Data(frames.utf8)])
            ])

        let session = makeSession()
        session.load(messages: [], conversationID: "conv_1")

        try await wait { hasAssistantText(session, "CALLBACK_SURFACED_IN_UI") }

        session.reset()
    }
}
