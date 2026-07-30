import Foundation
import HarnessKit
import Testing

@testable import GoCodeUI

/// Minimal HTTP+SSE stub scoped to this file's tests, keyed on HTTP method
/// *and* path (not path alone), because `GET /v1/runs/{id}/input`
/// (`pendingInput`) and `POST /v1/runs/{id}/input` (`answerInput`) share a
/// path and must be scripted independently.
private final class RunControlStub: URLProtocol, @unchecked Sendable {
    struct Response: Sendable {
        var status: Int = 200
        var headers: [String: String] = ["Content-Type": "application/json"]
        var body: Data = Data()
        /// Simulates a connection that never terminates. Used for the run's
        /// (and conversation's) `/events` stream so `RunSession.currentRunID`
        /// stays set for the duration of a test instead of clearing the
        /// moment an empty stream finishes normally.
        var neverFinishes = false
    }

    nonisolated(unsafe) private static var handler: (@Sendable (URLRequest) -> Response)?
    nonisolated(unsafe) private static var recorded: [URLRequest] = []
    private static let lock = NSLock()

    static func set(_ handler: @escaping @Sendable (URLRequest) -> Response) {
        lock.withLock { self.handler = handler }
    }

    static func reset() {
        lock.withLock {
            handler = nil
            recorded = []
        }
    }

    static func requests(matching path: String) -> [URLRequest] {
        lock.withLock { recorded.filter { $0.url?.path == path } }
    }

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        let request = self.request
        let response = Self.lock.withLock {
            Self.recorded.append(request)
            return Self.handler?(request) ?? Response()
        }
        let http = HTTPURLResponse(
            url: request.url!, statusCode: response.status,
            httpVersion: "HTTP/1.1", headerFields: response.headers)!
        client?.urlProtocol(self, didReceive: http, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: response.body)
        if !response.neverFinishes {
            client?.urlProtocolDidFinishLoading(self)
        }
    }

    override func stopLoading() {}
}

/// Exercises the fix for #994 (F3): `RunSession.cancel/approve/deny/answer`
/// discarded their server acknowledgement with `try?`, so a rejected or
/// failed call left the UI asserting an action had succeeded when it had
/// not. These tests drive `RunSession` directly against the stub above, the
/// same shape `RunSessionConversationStreamTests` uses.
@Suite("RunSession control acknowledgements", .serialized)
@MainActor
struct RunControlAckTests {

    private func makeSession() -> RunSession {
        let config = URLSessionConfiguration.ephemeral
        config.protocolClasses = [RunControlStub.self]
        let client = HarnessClient(
            baseURL: URL(string: "http://127.0.0.1:8897")!,
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

    /// Starts a run and waits for `currentRunID` to be set, with its events
    /// streams left open (never a terminal event) so the run stays "current"
    /// for the rest of the test -- cancel/approve/deny all guard on
    /// `currentRunID`. `extra` answers every other path the test needs
    /// (cancel/approve/deny).
    private func startBusyRun(
        _ session: RunSession, extra: @escaping @Sendable (URLRequest) -> RunControlStub.Response
    ) async throws {
        RunControlStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                return .init(status: 202, body: Data(#"{"run_id":"run_1","status":"queued"}"#.utf8))
            case ("GET", "/v1/runs/run_1/events"), ("GET", "/v1/conversations/run_1/events"):
                return .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"], neverFinishes: true
                )
            default:
                return extra(request)
            }
        }
        session.draft = "hi"
        session.submit()
        try await wait { session.currentRunID == "run_1" }
    }

    @Test("a failed approve surfaces via connectionError -- core regression")
    func approveFailureSurfaces() async throws {
        RunControlStub.reset()
        let session = makeSession()
        try await startBusyRun(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/approve" else {
                return .init()
            }
            return .init(
                status: 500,
                body: Data(
                    #"{"error":{"code":"internal_error","message":"approve rejected"}}"#.utf8))
        }

        session.approve()
        try await wait { session.connectionError != nil }
        #expect(session.connectionError == "approve rejected")

        session.reset()
    }

    @Test("a failed deny surfaces via connectionError")
    func denyFailureSurfaces() async throws {
        RunControlStub.reset()
        let session = makeSession()
        try await startBusyRun(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/deny" else {
                return .init()
            }
            return .init(
                status: 500,
                body: Data(#"{"error":{"code":"internal_error","message":"deny rejected"}}"#.utf8))
        }

        session.deny()
        try await wait { session.connectionError != nil }
        #expect(session.connectionError == "deny rejected")

        session.reset()
    }

    @Test("answerInput succeeding clears pendingQuestions")
    func answerSuccessClearsPendingQuestions() async throws {
        RunControlStub.reset()
        let session = makeSession()
        let promptJSON =
            #"{"run_id":"run_1","call_id":"call_1","questions":[{"question":"Continue?"}]}"#
        RunControlStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                return .init(status: 202, body: Data(#"{"run_id":"run_1","status":"queued"}"#.utf8))
            case ("GET", "/v1/conversations/run_1/events"):
                return .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"], neverFinishes: true
                )
            case ("GET", "/v1/runs/run_1/events"):
                let frame = """
                    id: run_1:0
                    event: run.waiting_for_user
                    data: {"id":"run_1:0","run_id":"run_1","type":"run.waiting_for_user","payload":{}}


                    """
                return .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"],
                    body: Data(frame.utf8), neverFinishes: true)
            case ("GET", "/v1/runs/run_1/input"):
                return .init(status: 200, body: Data(promptJSON.utf8))
            case ("POST", "/v1/runs/run_1/input"):
                return .init(status: 200)
            default:
                return .init()
            }
        }

        session.draft = "hi"
        session.submit()
        try await wait { session.pendingQuestions != nil }
        let questionID = try #require(session.pendingQuestions?.questions.first?.id)

        session.answer([questionID: "yes"])
        try await wait { session.pendingQuestions == nil }
        #expect(session.connectionError == nil)

        session.reset()
    }

    @Test("a rejected answerInput keeps pendingQuestions and surfaces the error -- core regression")
    func answerFailureKeepsPendingQuestions() async throws {
        RunControlStub.reset()
        let session = makeSession()
        let promptJSON =
            #"{"run_id":"run_1","call_id":"call_1","questions":[{"question":"Continue?"}]}"#
        RunControlStub.set { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/v1/runs"):
                return .init(status: 202, body: Data(#"{"run_id":"run_1","status":"queued"}"#.utf8))
            case ("GET", "/v1/conversations/run_1/events"):
                return .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"], neverFinishes: true
                )
            case ("GET", "/v1/runs/run_1/events"):
                let frame = """
                    id: run_1:0
                    event: run.waiting_for_user
                    data: {"id":"run_1:0","run_id":"run_1","type":"run.waiting_for_user","payload":{}}


                    """
                return .init(
                    status: 200, headers: ["Content-Type": "text/event-stream"],
                    body: Data(frame.utf8), neverFinishes: true)
            case ("GET", "/v1/runs/run_1/input"):
                return .init(status: 200, body: Data(promptJSON.utf8))
            case ("POST", "/v1/runs/run_1/input"):
                return .init(
                    status: 409,
                    body: Data(
                        #"{"error":{"code":"no_pending_input","message":"already answered"}}"#.utf8)
                )
            default:
                return .init()
            }
        }

        session.draft = "hi"
        session.submit()
        try await wait { session.pendingQuestions != nil }
        let questionID = try #require(session.pendingQuestions?.questions.first?.id)

        session.answer([questionID: "yes"])
        try await wait { session.connectionError != nil }
        #expect(
            session.pendingQuestions != nil, "a rejected answer must not clear the pending question"
        )

        session.reset()
    }

    @Test(
        "a failed cancel surfaces via connectionError and a second press stays cooperative -- core regression"
    )
    func cancelFailureStaysCooperative() async throws {
        RunControlStub.reset()
        let session = makeSession()
        try await startBusyRun(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/cancel" else {
                return .init()
            }
            return .init(
                status: 500,
                body: Data(#"{"error":{"code":"internal_error","message":"cancel rejected"}}"#.utf8)
            )
        }

        session.cancel()
        try await wait { session.connectionError != nil }
        #expect(session.transcript.runState != .cancelled)

        session.cancel()
        try await wait { RunControlStub.requests(matching: "/v1/runs/run_1/cancel").count == 2 }
        #expect(
            session.transcript.runState != .cancelled,
            "a failed first cancel must not let the second press force-abandon the stream locally"
        )

        session.reset()
    }

    @Test("cancel succeeding leaves no connectionError, and a second press marks cancelled")
    func cancelSuccessThenSecondPressCancels() async throws {
        RunControlStub.reset()
        let session = makeSession()
        try await startBusyRun(session) { request in
            guard request.httpMethod == "POST", request.url?.path == "/v1/runs/run_1/cancel" else {
                return .init()
            }
            return .init(status: 200)
        }

        session.cancel()
        try await wait { RunControlStub.requests(matching: "/v1/runs/run_1/cancel").count == 1 }
        try await Task.sleep(for: .milliseconds(50))
        #expect(session.connectionError == nil)

        session.cancel()
        try await wait { session.transcript.runState == .cancelled }
        #expect(
            RunControlStub.requests(matching: "/v1/runs/run_1/cancel").count == 1,
            "the second press must abandon locally, not call cancel again")

        session.reset()
    }
}
