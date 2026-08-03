import Foundation
import HarnessKit
import Testing

@testable import GoCodeUI

private final class SubmissionHandleStub: URLProtocol, @unchecked Sendable {
    struct Response: Sendable {
        var status = 200
        var headers = ["Content-Type": "application/json"]
        var body = Data()
        var delay: TimeInterval = 0
        var neverFinishes = false
    }

    nonisolated(unsafe) private static var handler: (@Sendable (URLRequest) -> Response)?
    nonisolated(unsafe) private static var requests: [URLRequest] = []
    private static let lock = NSLock()

    static func set(_ handler: @escaping @Sendable (URLRequest) -> Response) {
        lock.withLock { self.handler = handler }
    }

    static func reset() {
        lock.withLock {
            handler = nil
            requests = []
        }
    }
    static func paths() -> [String] { lock.withLock { requests.compactMap(\.url?.path) } }

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        let request = self.request
        let response = Self.lock.withLock {
            Self.requests.append(request)
            return Self.handler?(request) ?? Response()
        }
        if response.delay > 0 {
            DispatchQueue.global().asyncAfter(deadline: .now() + response.delay) {
                self.deliver(response, request: request)
            }
        } else {
            deliver(response, request: request)
        }
    }

    private func deliver(_ response: Response, request: URLRequest) {
        let http = HTTPURLResponse(
            url: request.url!, statusCode: response.status, httpVersion: "HTTP/1.1",
            headerFields: response.headers)!
        client?.urlProtocol(self, didReceive: http, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: response.body)
        if !response.neverFinishes { client?.urlProtocolDidFinishLoading(self) }
    }

    override func stopLoading() {}
}

@Suite("RunSubmission ownership", .serialized)
@MainActor
struct RunSubmissionTests {
    private func makeSession() -> RunSession {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [SubmissionHandleStub.self]
        return RunSession(
            client: HarnessClient(
                baseURL: URL(string: "http://127.0.0.1:8896")!,
                session: URLSession(configuration: configuration)))
    }

    private func event(_ id: String, _ runID: String, _ type: String, payload: String = "{}") throws
        -> HarnessEvent
    {
        try HarnessEvent(
            frame: SSEFrame(
                id: id, event: type,
                data:
                    #"{"id":"\#(id)","run_id":"\#(runID)","type":"\#(type)","payload":\#(payload)}"#
            ))
    }

    private func wait(timeout: Duration = .seconds(2), for condition: () -> Bool) async throws {
        let deadline = ContinuousClock.now.advanced(by: timeout)
        while ContinuousClock.now < deadline {
            if condition() { return }
            try await Task.sleep(for: .milliseconds(10))
        }
        Issue.record("timed out waiting for condition")
    }

    @Test("a captured steer action never falls through to a later submit")
    func capturedSteerNeverSubmits() {
        let action = ComposerAction.capture(canSteer: true, runID: "run_a")
        var steeredRunID: String?
        var submitted = false

        action.perform(
            canSubmit: true,
            steer: { steeredRunID = $0 },
            submit: { submitted = true })

        #expect(steeredRunID == "run_a")
        #expect(!submitted)
    }

    @Test("a scheduled B before A start response never becomes A's handle")
    func scheduledRunBeforeStartResponseDoesNotBecomeSubmission() async throws {
        SubmissionHandleStub.reset()
        SubmissionHandleStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                return .init(
                    status: 202, body: Data(#"{"run_id":"run_a","status":"queued"}"#.utf8),
                    delay: 0.15)
            case ("GET", "/v1/runs/run_a/events"), ("GET", "/v1/conversations/conversation/events"):
                return .init(headers: ["Content-Type": "text/event-stream"], neverFinishes: true)
            default: return .init()
            }
        }
        let session = makeSession()
        session.load(messages: [], conversationID: "conversation")
        session.draft = "run A"
        let submission = try #require(session.submit())

        try await session.applyConversationEvent(
            event("run_b:0", "run_b", "run.started"), conversationID: "conversation")
        #expect(session.currentRunID == "run_b")

        try await wait { submission.runID == "run_a" }
        #expect(submission.runID == "run_a")
        #expect(submission.state == .started("run_a"))
        let bActionPaths = Set(SubmissionHandleStub.paths()).intersection([
            "/v1/runs/run_b/cancel", "/v1/runs/run_b/approve", "/v1/runs/run_b/deny",
            "/v1/runs/run_b/input", "/v1/runs/run_b/steer",
        ])
        #expect(bActionPaths.isEmpty)
        session.reset()
    }

    @Test("a B that replaces captured A displaces A without a B cancel")
    func replacementAfterCaptureAbortsSubmissionWithoutBAction() async throws {
        SubmissionHandleStub.reset()
        SubmissionHandleStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                return .init(status: 202, body: Data(#"{"run_id":"run_a","status":"queued"}"#.utf8))
            case ("GET", "/v1/runs/run_a/events"), ("GET", "/v1/conversations/conversation/events"):
                return .init(headers: ["Content-Type": "text/event-stream"], neverFinishes: true)
            default: return .init()
            }
        }
        let session = makeSession()
        session.load(messages: [], conversationID: "conversation")
        session.draft = "run A"
        let submission = try #require(session.submit())
        try await wait { submission.runID == "run_a" && session.currentRunID == "run_a" }

        try await session.applyConversationEvent(
            event("run_b:0", "run_b", "run.started"), conversationID: "conversation")
        #expect(
            session.currentRunID == "run_a", "timestamp-less B cannot displace provisional local A")
        try await session.applyConversationEvent(
            event("run_b:1", "run_b", "run.started"), conversationID: "conversation")
        // A local run remains provisional until timestamped evidence. Model an
        // authoritative scheduled continuation by using a timestamped B frame.
        let earlyB = try HarnessEvent(
            frame: SSEFrame(
                id: "run_b:2", event: "run.started",
                data:
                    #"{"id":"run_b:2","run_id":"run_b","type":"run.started","timestamp":"2026-08-03T20:00:00Z","payload":{}}"#
            ))
        await session.applyConversationEvent(earlyB, conversationID: "conversation")
        #expect(session.currentRunID == "run_a", "provisional local A intentionally resists replay")

        // A current local A transitions out of provisional ownership when it
        // receives its own timestamped lifecycle evidence; only a later B is
        // permitted to replace it.
        let startedA = try HarnessEvent(
            frame: SSEFrame(
                id: "run_a:0", event: "run.started",
                data:
                    #"{"id":"run_a:0","run_id":"run_a","type":"run.started","timestamp":"2026-08-03T20:00:01Z","payload":{}}"#
            ))
        await session.applyConversationEvent(startedA, conversationID: "conversation")
        let laterB = try HarnessEvent(
            frame: SSEFrame(
                id: "run_b:3", event: "run.started",
                data:
                    #"{"id":"run_b:3","run_id":"run_b","type":"run.started","timestamp":"2026-08-03T20:00:02Z","payload":{}}"#
            ))
        await session.applyConversationEvent(laterB, conversationID: "conversation")
        #expect(session.currentRunID == "run_b")
        #expect(submission.isDisplaced)

        // A's late terminal remains useful A-only evidence but must not erase
        // the displacement outcome that tells ToolWalk to stop before B.
        submission.apply(try event("run_a:late", "run_a", "run.completed"))
        #expect(submission.isDisplaced)

        session.cancelTimedOutRun(expectedRunID: submission.runID)
        try await Task.sleep(for: .milliseconds(50))
        #expect(!SubmissionHandleStub.paths().contains("/v1/runs/run_b/cancel"))
        session.reset()
    }

    @Test("A-only handle retains A terminal transcript and never reads B")
    func submissionRetainsItsOwnTerminalTranscript() async throws {
        let submission = RunSubmission(prompt: "run A")
        submission.markStarted(runID: "run_a")
        submission.apply(
            try event(
                "run_a:0", "run_a", "assistant.message", payload: #"{"content":"A replied"}"#))
        submission.apply(try event("run_a:1", "run_a", "run.completed"))
        // B's event cannot enter A's handle because `apply` is run-id scoped.
        submission.apply(
            try event(
                "run_b:0", "run_b", "assistant.message", payload: #"{"content":"B replied"}"#))
        #expect(submission.runID == "run_a")
        #expect(submission.transcript.runState == .completed)
        #expect(
            submission.transcript.items.contains { item in
                if case .assistantMessage(let message) = item.kind {
                    return message.text == "A replied"
                }
                return false
            })
    }

    @Test("start failure and reset leave no resurrected submission")
    func startFailureAndResetAreSubmissionLocal() async throws {
        SubmissionHandleStub.reset()
        SubmissionHandleStub.set { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs" else {
                return .init(headers: ["Content-Type": "text/event-stream"], neverFinishes: true)
            }
            return .init(status: 503, body: Data(#"{"error":"unavailable"}"#.utf8))
        }
        let failedSession = makeSession()
        failedSession.draft = "fail"
        let failed = try #require(failedSession.submit())
        try await wait { failed.failure != nil }
        #expect(failed.runID == nil)
        #expect(failedSession.currentRunID == nil)

        SubmissionHandleStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                return .init(
                    status: 202, body: Data(#"{"run_id":"late_a","status":"queued"}"#.utf8),
                    delay: 0.15)
            default:
                return .init(headers: ["Content-Type": "text/event-stream"], neverFinishes: true)
            }
        }
        let resetSession = makeSession()
        resetSession.draft = "reset"
        let reset = try #require(resetSession.submit())
        resetSession.reset()
        try await wait { reset.isDisplaced }
        try await Task.sleep(for: .milliseconds(200))
        #expect(reset.runID == nil)
        #expect(resetSession.currentRunID == nil)
    }
}
